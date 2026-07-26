package selfspec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// requirementStateFixtureReq mirrors claimDeriveFixtureReq (claim_derive_test.go)
// but additionally accepts an explicit Claim, since RequirementState's
// PROVEN/STALE split depends on whether Claim already matches a fresh
// re-derivation.
func requirementStateFixtureReq(id, claim string, verifiedBy []string) ontology.Requirement {
	r := syncFixtureReq(id)
	r.Claim = claim
	r.VerifiedBy = verifiedBy
	r.Enforceability = ontology.EnforceabilityENFORCEABLE
	return r
}

// requirementStateFailingTestSrc is a genuine, COMPILING Go test that always
// fails its own assertion -- used to drive RequirementState into FAILING
// (as opposed to a compile error, which would be an infra-adjacent case
// gate.TestRunResult.CompileFailed also covers, exercised separately below).
const requirementStateFailingTestSrc = `package model

import "testing"

func TestBrdPackage_SignOff_AlwaysFails(t *testing.T) {
	t.Fatalf("this test always fails, by design")
}
`

// requirementStateCompileFailTestSrc references an undefined symbol so the
// package fails to BUILD -- proves RequirementState treats CompileFailed the
// same as a real test failure (FAILING), not as UNVERIFIED.
const requirementStateCompileFailTestSrc = `package model

import "testing"

func TestBrdPackage_SignOff_DoesNotCompile(t *testing.T) {
	_ = thisSymbolDoesNotExistAnywhere
	t.Fatalf("unreachable")
}
`

// TestRequirementState_NoCarrier proves a requirement with zero verified_by
// entries is NO_CARRIER -- nothing to prove or disprove.
func TestRequirementState_NoCarrier(t *testing.T) {
	r := requirementStateFixtureReq("R-state-no-carrier", "some claim", nil)
	got := RequirementState(r, t.TempDir(), false)
	if got != "NO_CARRIER" {
		t.Fatalf("RequirementState = %q, want NO_CARRIER", got)
	}
	if _, ok := ontology.RequirementProofLifecycle.Matches(got); !ok {
		t.Fatalf("%q is not a valid state in RequirementProofLifecycle", got)
	}
}

// TestRequirementState_UnverifiedWhenEntryUnresolvable proves a declared but
// malformed/unresolvable verified_by entry (no test ever actually runs)
// yields UNVERIFIED, not NO_CARRIER and not FAILING.
func TestRequirementState_UnverifiedWhenEntryUnresolvable(t *testing.T) {
	r := requirementStateFixtureReq("R-state-unverified", "some claim",
		[]string{"model/does_not_exist_test.go:TestNoSuchTest"})
	got := RequirementState(r, t.TempDir(), false)
	if got != "UNVERIFIED" {
		t.Fatalf("RequirementState = %q, want UNVERIFIED", got)
	}
}

// TestRequirementState_UnverifiedWhenEntryMalformed proves a syntactically
// malformed verified_by entry (no ":" separator) also yields UNVERIFIED --
// ParseFileColonSymbol fails, so the entry is skipped, contributing no
// verdict, exactly like deriveClaimFromVerifiedBy's own skip.
func TestRequirementState_UnverifiedWhenEntryMalformed(t *testing.T) {
	r := requirementStateFixtureReq("R-state-malformed", "some claim",
		[]string{"not-a-file-colon-symbol-entry"})
	got := RequirementState(r, t.TempDir(), false)
	if got != "UNVERIFIED" {
		t.Fatalf("RequirementState = %q, want UNVERIFIED", got)
	}
}

// TestRequirementState_FailingWhenTestFails proves a verified_by entry that
// resolves, compiles, and runs but does NOT pass yields FAILING.
func TestRequirementState_FailingWhenTestFails(t *testing.T) {
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"impl_test.go": requirementStateFailingTestSrc,
	})
	r := requirementStateFixtureReq("R-state-failing", "some claim",
		[]string{"model/impl_test.go:TestBrdPackage_SignOff_AlwaysFails"})
	got := RequirementState(r, root, false)
	if got != "FAILING" {
		t.Fatalf("RequirementState = %q, want FAILING", got)
	}
}

// TestRequirementState_FailingWhenCompileFails proves a verified_by entry
// whose package fails to BUILD is also FAILING (CompileFailed counts as not
// passing), not UNVERIFIED -- a real red bar, not merely "no verdict yet".
func TestRequirementState_FailingWhenCompileFails(t *testing.T) {
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"impl_test.go": requirementStateCompileFailTestSrc,
	})
	r := requirementStateFixtureReq("R-state-compilefail", "some claim",
		[]string{"model/impl_test.go:TestBrdPackage_SignOff_DoesNotCompile"})
	got := RequirementState(r, root, false)
	if got != "FAILING" {
		t.Fatalf("RequirementState = %q, want FAILING", got)
	}
}

// TestRequirementState_ProvenWhenClaimMatchesFreshScenario proves the
// healthy terminal case: verified_by passes AND resolves a scenario, and the
// committed Claim already equals that scenario's title -> PROVEN.
func TestRequirementState_ProvenWhenClaimMatchesFreshScenario(t *testing.T) {
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"impl_test.go": claimDeriveFixtureSingleTestSrc("example.com/claimderive"),
	})
	claim := "sign-off is rejected while a blocker is outstanding"
	r := requirementStateFixtureReq("R-state-proven", claim,
		[]string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"})
	got := RequirementState(r, root, false)
	if got != "PROVEN" {
		t.Fatalf("RequirementState = %q, want PROVEN", got)
	}
}

// TestRequirementState_ProvenWhenNoScenarioNarrated proves a passing,
// NON-narrating verified_by test (no hotamspec.NewScenario call at all) is
// also PROVEN: there is no scenario text to drift against, so passing
// evidence alone is enough -- mirrors check_claim_matches_scenario's own
// "cannot prove drift either way" skip, but on the positive (proven) side.
func TestRequirementState_ProvenWhenNoScenarioNarrated(t *testing.T) {
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"impl_test.go": claimDeriveFixturePlainTestSrc,
	})
	r := requirementStateFixtureReq("R-state-plain-proven", "a hand-authored claim, never derived",
		[]string{"model/impl_test.go:TestBrdPackage_SignOff_Plain"})
	got := RequirementState(r, root, false)
	if got != "PROVEN" {
		t.Fatalf("RequirementState = %q, want PROVEN", got)
	}
}

// TestRequirementState_StaleWhenClaimDrifted proves the drift case: the
// verified_by test passes and narrates a scenario, but the committed Claim
// disagrees with the CURRENT scenario title -> STALE, mirroring
// check_claim_matches_scenario's own RED case exactly.
func TestRequirementState_StaleWhenClaimDrifted(t *testing.T) {
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"impl_test.go": claimDeriveFixtureSingleTestSrc("example.com/claimderive"),
	})
	r := requirementStateFixtureReq("R-state-stale", "a stale claim that no longer matches the scenario",
		[]string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"})
	got := RequirementState(r, root, false)
	if got != "STALE" {
		t.Fatalf("RequirementState = %q, want STALE", got)
	}
}

// TestRequirementState_MultiVerifiedByAllMustPass proves that when a
// requirement carries MULTIPLE verified_by entries, ALL must pass for the
// overall verdict to reach PROVEN/STALE -- one failing entry alongside a
// passing one is FAILING, not a partial credit state.
func TestRequirementState_MultiVerifiedByAllMustPass(t *testing.T) {
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"scenario_test.go": claimDeriveFixtureSingleTestSrc("example.com/claimderive"),
		"failing_test.go":  requirementStateFailingTestSrc,
	})
	r := requirementStateFixtureReq("R-state-multi-mixed", "some claim", []string{
		"model/scenario_test.go:TestBrdPackage_SignOff_RejectsBlockers",
		"model/failing_test.go:TestBrdPackage_SignOff_AlwaysFails",
	})
	got := RequirementState(r, root, false)
	if got != "FAILING" {
		t.Fatalf("RequirementState = %q, want FAILING (one of two verified_by entries fails)", got)
	}
}

// TestRequirementState_MUTATION_TransitionsAcrossLifecycle drives ONE
// requirement fixture through NO_CARRIER -> UNVERIFIED -> FAILING -> STALE
// -> PROVEN by mutating its verified_by/Claim/on-disk test body between
// reads, proving RequirementState is a live re-derivation (never a cached or
// stored value) -- the same mutation-probe shape
// TestCheckClaimMatchesScenario_MUTATION_StaleAfterTitleEditWithoutResync
// (claim_scenario_current_test.go) already established for the drift half
// alone; this test walks the FULL proof lifecycle in one place.
func TestRequirementState_MUTATION_TransitionsAcrossLifecycle(t *testing.T) {
	reqID := "R-state-mutation"

	// 1. NO_CARRIER: no verified_by at all yet.
	r := requirementStateFixtureReq(reqID, "placeholder claim", nil)
	if got := RequirementState(r, t.TempDir(), false); got != "NO_CARRIER" {
		t.Fatalf("step 1 (no carrier): got %q, want NO_CARRIER", got)
	}

	// 2. UNVERIFIED: verified_by declared but the entry points outside any
	// resolvable Go module (no go.mod exists yet anywhere on disk for it) --
	// gate.RunVerifiedByTestRecording returns a non-nil Err (ModuleRoot
	// lookup fails), which RequirementState treats identically to Skipped:
	// no verdict either way, contributing nothing.
	r.VerifiedBy = []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}
	if got := RequirementState(r, t.TempDir(), false); got != "UNVERIFIED" {
		t.Fatalf("step 2 (unverified): got %q, want UNVERIFIED", got)
	}

	// 3. FAILING: a real module now exists and the test file exists, but the
	// test fails.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/statemutation\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	modelDir := filepath.Join(root, "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl_test.go"), []byte(`package model

import "testing"

func TestBrdPackage_SignOff_RejectsBlockers(t *testing.T) {
	t.Fatalf("not implemented yet")
}
`), 0o644); err != nil {
		t.Fatalf("WriteFile impl_test.go (failing): %v", err)
	}
	if got := RequirementState(r, root, false); got != "FAILING" {
		t.Fatalf("step 3 (failing): got %q, want FAILING", got)
	}

	// 4. STALE: the test is rewritten to pass and narrate a scenario, but
	// Claim is not updated to match.
	if err := os.WriteFile(filepath.Join(modelDir, "impl.go"), []byte(claimDeriveFixtureImplSrc), 0o644); err != nil {
		t.Fatalf("WriteFile impl.go: %v", err)
	}
	hotamspecDir := filepath.Join(root, "hotamspec")
	if err := os.MkdirAll(hotamspecDir, 0o755); err != nil {
		t.Fatalf("MkdirAll hotamspec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hotamspecDir, "hotamspec.go"), []byte(recordervendor.BodyForHash()), 0o644); err != nil {
		t.Fatalf("WriteFile vendored hotamspec.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl_test.go"), []byte(claimDeriveFixtureSingleTestSrc("example.com/statemutation")), 0o644); err != nil {
		t.Fatalf("WriteFile impl_test.go (scenario): %v", err)
	}
	// gate's in-memory compiled-binary cache is keyed by (moduleRoot,
	// pkgPattern, coverPkgPattern) with no content-hash for the compile step
	// itself (see internal/gate/compile_cache.go) -- this test recompiles the
	// SAME (root, "./model") pattern repeatedly with different on-disk
	// content, so without a reset a later call would silently reuse an
	// earlier step's stale compiled binary (see claim_scenario_current_test.go's
	// identical mutation-test precedent for this exact gotcha).
	gate.ResetRunCacheForTest()
	if got := RequirementState(r, root, false); got != "STALE" {
		t.Fatalf("step 4 (stale): got %q, want STALE", got)
	}

	// 5. PROVEN: Claim is re-derived (mirroring `hotam sync-domain`) to match
	// the currently-passing scenario.
	fresh, ok := deriveClaimFromVerifiedBy(root, false, r.VerifiedBy)
	if !ok {
		t.Fatalf("step 5 setup: expected a derivable fresh Claim")
	}
	r.Claim = fresh
	if got := RequirementState(r, root, false); got != "PROVEN" {
		t.Fatalf("step 5 (proven): got %q, want PROVEN", got)
	}
}
