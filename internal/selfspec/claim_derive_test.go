package selfspec

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
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

// claimDeriveFixtureRepeatedSubscenarioTitleSrc mirrors the REAL,
// resolver-verified bug this task (#389/W0.2) fixes: ONE test function that
// calls hotamspec.NewScenario(t, id, "same title") from INSIDE a t.Run loop
// over several sub-cases, each sub-case sharing the identical narrated
// title -- exactly the shape that made PRAT-hotam's R-gate-pg0-source-ready
// derive a Claim with one sentence repeated four times back to back before
// this fix. A SECOND, differently-titled Scenario follows in its own
// subtest, proving dedup collapses only the genuine repeat and leaves a
// distinct title alone.
func claimDeriveFixtureRepeatedSubscenarioTitleSrc(modulePath string) string {
	return `package model

import (
	"testing"

	"` + modulePath + `/hotamspec"
)

func TestBrdPackage_SignOff_RejectsBlockers(t *testing.T) {
	cases := []int{1, 2, 3, 4}
	for _, blockers := range cases {
		t.Run("", func(t *testing.T) {
			s := hotamspec.NewScenario(t, "R-claim-derive-repeat", "sign-off is rejected while a blocker is outstanding")
			p := &BrdPackage{Blockers: blockers}
			err := p.SignOff()
			s.Then("sign-off returns an error", err != nil)
		})
	}
	t.Run("allows_zero", func(t *testing.T) {
		s := hotamspec.NewScenario(t, "R-claim-derive-repeat", "zero blockers alone is sufficient for sign-off")
		p := &BrdPackage{Blockers: 0}
		err := p.SignOff()
		s.Then("sign-off succeeds", err == nil)
	})
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

// writeClaimDeriveFixtureModule builds a standalone Go module (go.mod +
// vendored REAL hotamspec recorder + model/impl.go + one or more test files)
// at a STABLE, content-hash-keyed path under gate.FixtureCacheRoot() rather
// than a fresh t.TempDir() -- mirrors spec_md_current_test.go's
// writeSpecMDFixtureModule shape, parameterized over which test source(s) to
// write so this one helper serves the single-test, multi-test, and
// plain-test fixture shapes below.
//
// PILOT for task #376: `t.TempDir()` gives every test RUN its own fresh
// absolute path, and the Go compiler bakes that absolute path into its
// `compile.exe ... -pack <path>/impl.go` command line -- Go's build cache
// keys off that exact command line, so byte-identical fixture content
// written to a freshly-rolled temp directory is a guaranteed cache MISS
// across separate `go test` processes (proven empirically in task #374's
// .scratch/task374-fixtureA-build.log vs task374-fixtureB-build.log: same
// bytes, different temp dir, different -buildid / cache entry). Routing
// through gate.EnsureContentAddressedFixture instead publishes this exact
// same file set to a STABLE path keyed by its own content hash, so a SECOND
// `go test` process (e.g. this same test re-run) that builds the identical
// fixture content reuses the FIRST process's Go build-cache entry instead of
// recompiling from scratch. See gate/fixture_cache.go's doc comment for the
// full concurrent-writer / stale-write / retention design this delegates to
// -- this function's own contract (an absolute module-root path containing
// exactly the requested files) is unchanged from the t.TempDir() version;
// only the deterministic-vs-fresh choice of WHERE on disk moved.
func writeClaimDeriveFixtureModule(t *testing.T, testFiles map[string]string) (moduleRoot string) {
	t.Helper()
	modulePath := "example.com/claimderive"

	files := map[string][]byte{
		"go.mod":                 []byte("module " + modulePath + "\n\ngo 1.21\n"),
		"model/impl.go":          []byte(claimDeriveFixtureImplSrc),
		"hotamspec/hotamspec.go": []byte(recordervendor.BodyForHash()),
	}
	for name, content := range testFiles {
		files["model/"+name] = []byte(content)
	}

	return gate.EnsureContentAddressedFixture(files, t.Fatalf)
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
	if testing.Short() {
		t.Skip("claim-derive e2e: discipline=full + ENFORCEABLE + verified_by in scope drives DeriveClaimsFromScenarios into a real go build/go test subprocess via gate.RunVerifiedByTestRecording; skipped in -short")
	}
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
	if testing.Short() {
		t.Skip("claim-derive e2e: discipline=full + ENFORCEABLE + two verified_by entries drives DeriveClaimsFromScenarios into TWO real go build/go test subprocess runs (forward and reversed order) via gate.RunVerifiedByTestRecording; skipped in -short")
	}
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

// TestDeriveClaimsFromScenarios_RepeatedSubscenarioTitleDedupedWithinEntry is
// the RED-then-GREEN regression proof for task #389/W0.2's fix: a SINGLE
// verified_by entry whose test function narrates four sub-cases via
// t.Run-nested hotamspec.NewScenario calls sharing the IDENTICAL title must
// contribute that title to the derived Claim exactly ONCE (first-occurrence
// position), never four times back to back -- reproducing, on a controlled
// fixture, the exact real bug the task brief captured verbatim from
// PRAT-hotam's R-gate-pg0-source-ready (a fresh derivation there repeated
// "SourceReady() only returns nil when columns, scope, and the project
// profile are ALL present" four times in a row before this fix). The
// trailing, genuinely DIFFERENT title from the fixture's fifth subtest must
// still be concatenated afterward -- proving dedup removes only the true
// repeat, not every subsequent title.
func TestDeriveClaimsFromScenarios_RepeatedSubscenarioTitleDedupedWithinEntry(t *testing.T) {
	if testing.Short() {
		t.Skip("claim-derive e2e: discipline=full + ENFORCEABLE + verified_by in scope drives DeriveClaimsFromScenarios into a real go build/go test subprocess via gate.RunVerifiedByTestRecording; skipped in -short")
	}
	root := writeClaimDeriveFixtureModule(t, map[string]string{
		"impl_test.go": claimDeriveFixtureRepeatedSubscenarioTitleSrc("example.com/claimderive"),
	})
	reg := registry.New[ontology.Requirement]()
	r := claimDeriveFixtureReq("R-claim-derive-repeat", []string{"model/impl_test.go:TestBrdPackage_SignOff_RejectsBlockers"}, ontology.EnforceabilityENFORCEABLE)
	reg.MustRegister(r.ID, r)

	changed := DeriveClaimsFromScenarios(reg, root, false, loader.DisciplineFull)
	if len(changed) != 1 || changed[0] != r.ID {
		t.Fatalf("expected exactly %s to be reported changed, got %v", r.ID, changed)
	}
	got, _ := reg.Get(r.ID)
	want := "sign-off is rejected while a blocker is outstanding" + ClaimDerivationSeparator + "zero blockers alone is sufficient for sign-off"
	if got.Claim != want {
		t.Fatalf("Claim = %q, want %q (the repeated sub-case title must contribute only once, in first-occurrence order, followed by the distinct trailing title)", got.Claim, want)
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
	if testing.Short() {
		t.Skip("claim-derive e2e: discipline=full + ENFORCEABLE + two verified_by entries (one narrating, one plain) drives DeriveClaimsFromScenarios into real go build/go test subprocess runs via gate.RunVerifiedByTestRecording; skipped in -short")
	}
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
	if testing.Short() {
		t.Skip("claim-derive e2e: discipline=full + ENFORCEABLE + verified_by in scope drives DeriveClaimsFromScenarios into TWO real go build/go test subprocess runs (first pass + idempotent second pass) via gate.RunVerifiedByTestRecording; skipped in -short")
	}
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
