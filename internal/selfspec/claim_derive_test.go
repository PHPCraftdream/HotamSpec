package selfspec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

// claimDeriveFixtureImplSrc is a tiny real (not merely AST-shaped) impl the
// fixture's verified_by test(s) genuinely exercise -- mirrors
// internal/invariants/spec_md_current_test.go's specMDFixtureImplSrc shape.
const claimDeriveFixtureImplSrc = `package model

type BrdPackage struct {
	Blockers int
}

func (p *BrdPackage) SignOff() error {
	if p.Blockers > 0 {
		return errHasBlockers
	}
	return nil
}

var errHasBlockers = stubErr{}

type stubErr struct{}

func (stubErr) Error() string { return "has blockers" }
`

// claimDeriveFixtureSingleTestSrc is a fixture with exactly ONE verified_by
// test carrying exactly one hotamspec.Scenario -- the trivial n=1 case of
// the concatenation rule (see internal/selfspec/claim_derive.go's own
// package doc comment, point 1: a single verified_by entry is the n=1 case
// of the SAME rule, not a separately coded branch).
func claimDeriveFixtureSingleTestSrc(modulePath string) string {
	return `package model

import (
	"testing"

	"` + modulePath + `/hotamspec"
)

func TestBrdPackage_SignOff_RejectsBlockers(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-claim-derive-single", "sign-off is rejected while a blocker is outstanding")
	p := &BrdPackage{Blockers: 1}
	err := p.SignOff()
	s.Then("sign-off returns an error", err != nil)
}
`
}

// claimDeriveFixtureMultiTestSrc mirrors the REAL pilot precedent this
// task's brief cites (PRAT-hotam/domains/prat/spec/model/brd_package_test.go's
// R-brd-integrity-zero-blockers): TWO separate Test* functions, each
// constructing its OWN hotamspec.Scenario with a distinct description,
// listed against the SAME requirement via TWO verified_by entries.
func claimDeriveFixtureMultiTestSrc(modulePath string) string {
	return `package model

import (
	"testing"

	"` + modulePath + `/hotamspec"
)

func TestBrdPackage_SignOff_RejectsBlockers(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-claim-derive-multi", "sign-off is rejected while a blocker is outstanding")
	p := &BrdPackage{Blockers: 1}
	err := p.SignOff()
	s.Then("sign-off returns an error", err != nil)
}

func TestBrdPackage_SignOff_AllowsZeroBlockers(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-claim-derive-multi", "zero blockers alone is sufficient for sign-off")
	p := &BrdPackage{Blockers: 0}
	err := p.SignOff()
	s.Then("sign-off succeeds", err == nil)
}
`
}

// claimDeriveFixturePlainTestSrc is a genuine, passing Go test that does NOT
// use hotamspec at all (no scenario narrated) -- used to prove a
// non-narrating verified_by entry contributes nothing to the derived Claim
// without aborting derivation for the whole requirement.
const claimDeriveFixturePlainTestSrc = `package model

import "testing"

func TestBrdPackage_SignOff_Plain(t *testing.T) {
	p := &BrdPackage{Blockers: 0}
	if err := p.SignOff(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
`

// writeClaimDeriveFixtureModule builds a standalone temp Go module (go.mod +
// vendored REAL hotamspec recorder + model/impl.go + one or more test files)
// at the module root -- mirrors spec_md_current_test.go's
// writeSpecMDFixtureModule exactly, parameterized over which test source(s)
// to write so this one helper serves the single-test, multi-test, and
// plain-test fixture shapes below.
func writeClaimDeriveFixtureModule(t *testing.T, testFiles map[string]string) (moduleRoot string) {
	t.Helper()
	root := t.TempDir()
	modulePath := "example.com/claimderive"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	modelDir := filepath.Join(root, "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "impl.go"), []byte(claimDeriveFixtureImplSrc), 0o644); err != nil {
		t.Fatalf("WriteFile impl.go: %v", err)
	}
	for name, content := range testFiles {
		if err := os.WriteFile(filepath.Join(modelDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	hotamspecDir := filepath.Join(root, "hotamspec")
	if err := os.MkdirAll(hotamspecDir, 0o755); err != nil {
		t.Fatalf("MkdirAll hotamspec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hotamspecDir, "hotamspec.go"), []byte(recordervendor.BodyForHash()), 0o644); err != nil {
		t.Fatalf("WriteFile vendored hotamspec.go: %v", err)
	}
	// modulePath is embedded via the caller's own test-source template
	// (claimDeriveFixtureSingleTestSrc etc. take modulePath as a parameter);
	// this function itself only ever writes what it is handed.
	_ = modulePath
	return root
}

func claimDeriveFixtureReq(id string, verifiedBy []string, enforceability string) ontology.Requirement {
	r := syncFixtureReq(id)
	r.VerifiedBy = verifiedBy
	r.Enforceability = enforceability
	return r
}

// TestDeriveClaimsFromScenarios_SingleVerifiedByEntry proves the trivial n=1
// case: one verified_by entry, one hotamspec scenario -- Claim becomes
// exactly that scenario's recorded title.
func TestDeriveClaimsFromScenarios_SingleVerifiedByEntry(t *testing.T) {
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"impl_test.go": claimDeriveFixtureSingleTestSrc("example.com/claimderive"),
	})
	reg := registry.New[ontology.Requirement]()
	r := claimDeriveFixtureReq("R-claim-derive-single", []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}, ontology.EnforceabilityENFORCEABLE)
	reg.MustRegister(r.ID, r)

	changed := DeriveClaimsFromScenarios(reg, root, false, loader.DisciplineFull)
	if len(changed) != 1 || changed[0] != r.ID {
		t.Fatalf("expected exactly %s to be reported changed, got %v", r.ID, changed)
	}
	got, _ := reg.Get(r.ID)
	want := "sign-off is rejected while a blocker is outstanding"
	if got.Claim != want {
		t.Fatalf("Claim = %q, want %q", got.Claim, want)
	}
}

// TestDeriveClaimsFromScenarios_MultiVerifiedByEntryConcatenatesInOrder
// proves the resolver-settled multi-test rule (task brief's "Вопрос 1"): TWO
// verified_by entries -> Claim is the concatenation of both scenario titles,
// space-joined, in verified_by's own declared order -- mirroring the real
// pilot precedent (PRAT-hotam R-brd-integrity-zero-blockers).
func TestDeriveClaimsFromScenarios_MultiVerifiedByEntryConcatenatesInOrder(t *testing.T) {
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"impl_test.go": claimDeriveFixtureMultiTestSrc("example.com/claimderive"),
	})
	reg := registry.New[ontology.Requirement]()
	r := claimDeriveFixtureReq("R-claim-derive-multi", []string{
		"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers",
		"model/impl_test.go:TestBrdPackage_SignOff_AllowsZeroBlockers",
	}, ontology.EnforceabilityENFORCEABLE)
	reg.MustRegister(r.ID, r)

	DeriveClaimsFromScenarios(reg, root, false, loader.DisciplineFull)
	got, _ := reg.Get(r.ID)
	want := "sign-off is rejected while a blocker is outstanding" + ClaimDerivationSeparator + "zero blockers alone is sufficient for sign-off"
	if got.Claim != want {
		t.Fatalf("Claim = %q, want %q", got.Claim, want)
	}

	// Reversing verified_by's declared order must reverse the concatenation
	// order too -- proves the function follows verified_by's OWN order, not
	// some other (e.g. alphabetical) ordering.
	reg2 := registry.New[ontology.Requirement]()
	r2 := claimDeriveFixtureReq("R-claim-derive-multi", []string{
		"model/impl_test.go:TestBrdPackage_SignOff_AllowsZeroBlockers",
		"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers",
	}, ontology.EnforceabilityENFORCEABLE)
	reg2.MustRegister(r2.ID, r2)
	DeriveClaimsFromScenarios(reg2, root, false, loader.DisciplineFull)
	got2, _ := reg2.Get(r2.ID)
	wantReversed := "zero blockers alone is sufficient for sign-off" + ClaimDerivationSeparator + "sign-off is rejected while a blocker is outstanding"
	if got2.Claim != wantReversed {
		t.Fatalf("reversed order: Claim = %q, want %q", got2.Claim, wantReversed)
	}
}

// TestDeriveClaimsFromScenarios_NonFullDisciplineIsNoOp proves the domain-
// level gate: a domain that has not declared discipline:"full" sees this
// function do nothing at all, regardless of how many verified_by entries a
// requirement carries.
func TestDeriveClaimsFromScenarios_NonFullDisciplineIsNoOp(t *testing.T) {
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"impl_test.go": claimDeriveFixtureSingleTestSrc("example.com/claimderive"),
	})
	reg := registry.New[ontology.Requirement]()
	r := claimDeriveFixtureReq("R-claim-derive-single", []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}, ontology.EnforceabilityENFORCEABLE)
	originalClaim := r.Claim
	reg.MustRegister(r.ID, r)

	for _, discipline := range []string{"", "FULL", "soft"} {
		changed := DeriveClaimsFromScenarios(reg, root, false, discipline)
		if len(changed) != 0 {
			t.Fatalf("discipline=%q: expected no changes, got %v", discipline, changed)
		}
		got, _ := reg.Get(r.ID)
		if got.Claim != originalClaim {
			t.Fatalf("discipline=%q: Claim was mutated to %q, want unchanged %q", discipline, got.Claim, originalClaim)
		}
	}
}

// TestDeriveClaimsFromScenarios_InherentlyProseExemptionIsUntouched proves
// the requirement-level exemption: a requirement tagged
// Enforceability=INHERENTLY_PROSE is never a derivation candidate even under
// discipline:"full", mirroring checkSettledRequiresScenario's own exemption.
func TestDeriveClaimsFromScenarios_InherentlyProseExemptionIsUntouched(t *testing.T) {
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"impl_test.go": claimDeriveFixtureSingleTestSrc("example.com/claimderive"),
	})
	reg := registry.New[ontology.Requirement]()
	r := claimDeriveFixtureReq("R-claim-derive-single", []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}, ontology.EnforceabilityINHERENTLY_PROSE)
	originalClaim := r.Claim
	reg.MustRegister(r.ID, r)

	changed := DeriveClaimsFromScenarios(reg, root, false, loader.DisciplineFull)
	if len(changed) != 0 {
		t.Fatalf("expected no changes for an INHERENTLY_PROSE requirement, got %v", changed)
	}
	got, _ := reg.Get(r.ID)
	if got.Claim != originalClaim {
		t.Fatalf("Claim was mutated to %q, want unchanged %q", got.Claim, originalClaim)
	}
}

// TestDeriveClaimsFromScenarios_NoVerifiedByIsUntouched proves a requirement
// with zero verified_by entries is left alone (nothing to derive from).
func TestDeriveClaimsFromScenarios_NoVerifiedByIsUntouched(t *testing.T) {
	reg := registry.New[ontology.Requirement]()
	r := claimDeriveFixtureReq("R-claim-derive-none", nil, ontology.EnforceabilityENFORCEABLE)
	originalClaim := r.Claim
	reg.MustRegister(r.ID, r)

	changed := DeriveClaimsFromScenarios(reg, t.TempDir(), false, loader.DisciplineFull)
	if len(changed) != 0 {
		t.Fatalf("expected no changes for a requirement with no verified_by entries, got %v", changed)
	}
	got, _ := reg.Get(r.ID)
	if got.Claim != originalClaim {
		t.Fatalf("Claim was mutated to %q, want unchanged %q", got.Claim, originalClaim)
	}
}

// TestDeriveClaimsFromScenarios_PlainNonScenarioEntryContributesNothing
// proves a verified_by entry that genuinely passes but records no
// hotamspec scenario at all contributes nothing to the concatenation,
// without aborting derivation for entries that DO narrate.
func TestDeriveClaimsFromScenarios_PlainNonScenarioEntryContributesNothing(t *testing.T) {
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"scenario_test.go": claimDeriveFixtureSingleTestSrc("example.com/claimderive"),
		"plain_test.go":    claimDeriveFixturePlainTestSrc,
	})
	reg := registry.New[ontology.Requirement]()
	r := claimDeriveFixtureReq("R-claim-derive-single", []string{
		"model/scenario_test.go:TestBrdPackage_SignOff_RejectsBlockers",
		"model/plain_test.go:TestBrdPackage_SignOff_Plain",
	}, ontology.EnforceabilityENFORCEABLE)
	reg.MustRegister(r.ID, r)

	DeriveClaimsFromScenarios(reg, root, false, loader.DisciplineFull)
	got, _ := reg.Get(r.ID)
	want := "sign-off is rejected while a blocker is outstanding"
	if got.Claim != want {
		t.Fatalf("Claim = %q, want %q (the plain non-scenario entry must contribute nothing)", got.Claim, want)
	}
}

// TestDeriveClaimsFromScenarios_UnchangedIDsNotReported proves the
// changed-IDs return value only names requirements whose Claim actually
// moved -- calling derivation twice in a row (idempotent, unchanged source)
// reports zero changes the second time.
func TestDeriveClaimsFromScenarios_UnchangedIDsNotReported(t *testing.T) {
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"impl_test.go": claimDeriveFixtureSingleTestSrc("example.com/claimderive"),
	})
	reg := registry.New[ontology.Requirement]()
	r := claimDeriveFixtureReq("R-claim-derive-single", []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}, ontology.EnforceabilityENFORCEABLE)
	reg.MustRegister(r.ID, r)

	first := DeriveClaimsFromScenarios(reg, root, false, loader.DisciplineFull)
	if len(first) != 1 {
		t.Fatalf("first pass: expected 1 change, got %v", first)
	}
	second := DeriveClaimsFromScenarios(reg, root, false, loader.DisciplineFull)
	if len(second) != 0 {
		t.Fatalf("second pass (idempotent): expected 0 changes, got %v", second)
	}
}

// TestRequirementInClaimDerivationScope_Table exercises the scope predicate
// directly across its four corner cases.
func TestRequirementInClaimDerivationScope_Table(t *testing.T) {
	cases := []struct {
		name           string
		enforceability string
		verifiedBy     []string
		want           bool
	}{
		{"enforceable+verifiedBy", ontology.EnforceabilityENFORCEABLE, []string{"a:b"}, true},
		{"enforceable+noVerifiedBy", ontology.EnforceabilityENFORCEABLE, nil, false},
		{"inherentlyProse+verifiedBy", ontology.EnforceabilityINHERENTLY_PROSE, []string{"a:b"}, false},
		{"inherentlyProse+noVerifiedBy", ontology.EnforceabilityINHERENTLY_PROSE, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := claimDeriveFixtureReq("R-x", tc.verifiedBy, tc.enforceability)
			if got := RequirementInClaimDerivationScope(r); got != tc.want {
				t.Fatalf("RequirementInClaimDerivationScope(%+v) = %v, want %v", r, got, tc.want)
			}
		})
	}
}
