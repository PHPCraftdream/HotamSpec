package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// writeAuthoredSpecFixture writes a single Go source file under
// tmp/<rel> (rel already spec/-prefixed, matching the domain-relative
// convention implemented_by/verified_by entries use) and returns tmp as the
// domain directory (g.DomainDir).
func writeAuthoredSpecFixture(t *testing.T, rel, content string) string {
	t.Helper()
	tmp := t.TempDir()
	full := filepath.Join(tmp, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return tmp
}

const authoredRiskModelSrc = `package model

type Risk struct {
	Owner string
}

func NewRisk(owner string) (*Risk, error) {
	return &Risk{Owner: owner}, nil
}

func (r *Risk) Validate() error {
	return nil
}
`

const authoredRiskTestGoodSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	r, err := NewRisk("")
	if err == nil {
		t.Fatalf("expected error for missing owner, got risk=%v", r)
	}
}
`

const authoredRiskTestVacuousSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	t.Log("no structural atom")
}
`

const authoredRiskTestSkipSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	t.Skip("not ready yet")
}
`

func reqWithLinks(rid, owner string, implementedBy, verifiedBy []string) ontology.Requirement {
	r := req(rid, owner)
	r.ImplementedBy = implementedBy
	r.VerifiedBy = verifiedBy
	return r
}

// --- check_implemented_by_symbol_resolvable --------------------------------

func TestCheckImplementedBySymbolResolvable_OK(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk.go", authoredRiskModelSrc)
	r := reqWithLinks("R-1", "sa", []string{"spec/model/risk.go:NewRisk"}, nil)
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_implemented_by_symbol_resolvable", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckImplementedBySymbolResolvable_NoOpWhenEmpty(t *testing.T) {
	t.Parallel()
	r := req("R-1", "sa")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_implemented_by_symbol_resolvable", g); len(vs) != 0 {
		t.Fatalf("expected no violations for empty implemented_by (field optional), got %v", vs)
	}
}

func TestCheckImplementedBySymbolResolvable_FiresOnMalformedEntry(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk.go", authoredRiskModelSrc)
	r := reqWithLinks("R-1", "sa", []string{"not-shaped-like-file-colon-symbol"}, nil)
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_implemented_by_symbol_resolvable", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation for malformed implemented_by entry, got %v", vs)
	}
}

// TestCheckImplementedBySymbolResolvable_MUTATION_StalenessRoundTrip is the
// break->fix mutation proof: an implemented_by entry pointing at a symbol
// that exists passes; deleting the symbol (simulating a rename that orphans
// the reference) makes the check fire; restoring the symbol makes it pass
// again.
func TestCheckImplementedBySymbolResolvable_MUTATION_StalenessRoundTrip(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk.go", authoredRiskModelSrc)
	path := filepath.Join(domainDir, "spec", "model", "risk.go")
	r := reqWithLinks("R-1", "sa", []string{"spec/model/risk.go:NewRisk"}, nil)
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}

	if vs := runCheck(t, "check_implemented_by_symbol_resolvable", g); len(vs) != 0 {
		t.Fatalf("INTACT: expected no violations, got %v", vs)
	}

	orphaned := `package model

type Risk struct {
	Owner string
}
`
	if err := os.WriteFile(path, []byte(orphaned), 0o644); err != nil {
		t.Fatalf("WriteFile mutation: %v", err)
	}
	vs := runCheck(t, "check_implemented_by_symbol_resolvable", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("BROKEN: expected violation after removing NewRisk (staleness), got %v", vs)
	}

	if err := os.WriteFile(path, []byte(authoredRiskModelSrc), 0o644); err != nil {
		t.Fatalf("WriteFile restore: %v", err)
	}
	if vs := runCheck(t, "check_implemented_by_symbol_resolvable", g); len(vs) != 0 {
		t.Fatalf("RESTORED: expected no violations after restoring NewRisk, got %v", vs)
	}
}

// TestCheckImplementedBySymbolResolvable_MethodResolves proves methods
// (fn.Recv != nil) resolve for implemented_by -- unlike the engine-side
// Test*-scan machinery, which deliberately skips methods.
func TestCheckImplementedBySymbolResolvable_MethodResolves(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk.go", authoredRiskModelSrc)
	r := reqWithLinks("R-1", "sa", []string{"spec/model/risk.go:Risk.Validate"}, nil)
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_implemented_by_symbol_resolvable", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a real method reference, got %v", vs)
	}
}

// --- check_verified_by_test_resolvable -------------------------------------

func TestCheckVerifiedByTestResolvable_OK(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", authoredRiskTestGoodSrc)
	r := reqWithLinks("R-1", "sa", nil, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_verified_by_test_resolvable", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckVerifiedByTestResolvable_NoOpWhenEmpty(t *testing.T) {
	t.Parallel()
	r := req("R-1", "sa")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_verified_by_test_resolvable", g); len(vs) != 0 {
		t.Fatalf("expected no violations for empty verified_by (field optional), got %v", vs)
	}
}

// TestCheckVerifiedByTestResolvable_MUTATION_StalenessRoundTrip is the
// break->fix proof: renaming the test out from under the reference fires
// the check; restoring it clears the violation.
func TestCheckVerifiedByTestResolvable_MUTATION_StalenessRoundTrip(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", authoredRiskTestGoodSrc)
	path := filepath.Join(domainDir, "spec", "model", "risk_test.go")
	r := reqWithLinks("R-1", "sa", nil, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}

	if vs := runCheck(t, "check_verified_by_test_resolvable", g); len(vs) != 0 {
		t.Fatalf("INTACT: expected no violations, got %v", vs)
	}

	renamed := `package model

import "testing"

func TestNewRisk_RejectsMissingOwnerRenamed(t *testing.T) {
	r, err := NewRisk("")
	if err == nil {
		t.Fatalf("expected error, got %v", r)
	}
}
`
	if err := os.WriteFile(path, []byte(renamed), 0o644); err != nil {
		t.Fatalf("WriteFile mutation: %v", err)
	}
	vs := runCheck(t, "check_verified_by_test_resolvable", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("BROKEN: expected violation after renaming the test (staleness), got %v", vs)
	}

	if err := os.WriteFile(path, []byte(authoredRiskTestGoodSrc), 0o644); err != nil {
		t.Fatalf("WriteFile restore: %v", err)
	}
	if vs := runCheck(t, "check_verified_by_test_resolvable", g); len(vs) != 0 {
		t.Fatalf("RESTORED: expected no violations after restoring the test, got %v", vs)
	}
}

func TestCheckVerifiedByTestResolvable_FiresOnNonTestPrefixedName(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", authoredRiskTestGoodSrc)
	r := reqWithLinks("R-1", "sa", nil, []string{"spec/model/risk_test.go:NotATestFunc"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_verified_by_test_resolvable", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation for a non-Test*-shaped verified_by name, got %v", vs)
	}
}

// --- check_verified_by_test_has_teeth --------------------------------------

func TestCheckVerifiedByTestHasTeeth_OK(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", authoredRiskTestGoodSrc)
	r := reqWithLinks("R-1", "sa", nil, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_verified_by_test_has_teeth", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a real assertion, got %v", vs)
	}
}

// TestCheckVerifiedByTestHasTeeth_MUTATION_VacuousRoundTrip is the
// break->fix proof for the ENFORCED-gate anti-vacuousness PROHIBITION (the
// real successor to the honest no-op checkEnforcedByTestHasTeeth): a
// t.Log-only body fires; replacing it with a real assertion clears it.
func TestCheckVerifiedByTestHasTeeth_MUTATION_VacuousRoundTrip(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", authoredRiskTestVacuousSrc)
	path := filepath.Join(domainDir, "spec", "model", "risk_test.go")
	r := reqWithLinks("R-1", "sa", nil, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}

	vs := runCheck(t, "check_verified_by_test_has_teeth", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("BROKEN: expected violation for t.Log-only test body, got %v", vs)
	}

	if err := os.WriteFile(path, []byte(authoredRiskTestGoodSrc), 0o644); err != nil {
		t.Fatalf("WriteFile fix: %v", err)
	}
	if vs := runCheck(t, "check_verified_by_test_has_teeth", g); len(vs) != 0 {
		t.Fatalf("FIXED: expected no violations after adding a real assertion, got %v", vs)
	}
}

func TestCheckVerifiedByTestHasTeeth_SkipsUnresolvedEntries(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", authoredRiskTestGoodSrc)
	r := reqWithLinks("R-1", "sa", nil, []string{"spec/model/risk_test.go:TestDoesNotExist"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	// check_verified_by_test_resolvable is the one that should fire here, not
	// has_teeth (which only judges a body that was actually found).
	if vs := runCheck(t, "check_verified_by_test_has_teeth", g); len(vs) != 0 {
		t.Fatalf("expected has_teeth to no-op on an unresolved entry (that's resolvable's job), got %v", vs)
	}
}

// --- check_verified_by_test_no_skip -----------------------------------------

func TestCheckVerifiedByTestNoSkip_OK(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", authoredRiskTestGoodSrc)
	r := reqWithLinks("R-1", "sa", nil, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_verified_by_test_no_skip", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a test without t.Skip, got %v", vs)
	}
}

// TestCheckVerifiedByTestNoSkip_MUTATION_SkipRoundTrip is the break->fix
// proof: adding an unconditional top-level t.Skip fires; removing it clears
// the violation.
func TestCheckVerifiedByTestNoSkip_MUTATION_SkipRoundTrip(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", authoredRiskTestSkipSrc)
	path := filepath.Join(domainDir, "spec", "model", "risk_test.go")
	r := reqWithLinks("R-1", "sa", nil, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}

	vs := runCheck(t, "check_verified_by_test_no_skip", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("BROKEN: expected violation for unconditional top-level t.Skip, got %v", vs)
	}

	if err := os.WriteFile(path, []byte(authoredRiskTestGoodSrc), 0o644); err != nil {
		t.Fatalf("WriteFile fix: %v", err)
	}
	if vs := runCheck(t, "check_verified_by_test_no_skip", g); len(vs) != 0 {
		t.Fatalf("FIXED: expected no violations after removing t.Skip, got %v", vs)
	}
}

func TestCheckVerifiedByTestNoSkip_ConditionalSkipDoesNotFire(t *testing.T) {
	t.Parallel()
	conditional := `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	r, err := NewRisk("")
	if err == nil {
		t.Fatalf("expected error, got %v", r)
	}
}
`
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", conditional)
	r := reqWithLinks("R-1", "sa", nil, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_verified_by_test_no_skip", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a runtime-conditional skip (normal Go idiom), got %v", vs)
	}
}

// --- check_verified_by_no_unrelated_reuse -----------------------------------

func TestCheckVerifiedByNoUnrelatedReuse_OKSingleCitation(t *testing.T) {
	t.Parallel()
	r := reqWithLinks("R-1", "sa", nil, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_verified_by_no_unrelated_reuse", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a single citation, got %v", vs)
	}
}

// TestCheckVerifiedByNoUnrelatedReuse_MUTATION_UnrelatedReuseRoundTrip is the
// break->fix mutation proof for the reuse-detector: the same verified_by
// entry cited by two UNRELATED requirements fires on both; linking them via
// a Relation clears the violation; removing the second citation entirely
// also clears it.
func TestCheckVerifiedByNoUnrelatedReuse_MUTATION_UnrelatedReuseRoundTrip(t *testing.T) {
	t.Parallel()
	entry := "spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"
	r1 := reqWithLinks("R-1", "sa", nil, []string{entry})
	r2 := reqWithLinks("R-2", "sb", nil, []string{entry})
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA, sB}, Requirements: []ontology.Requirement{r1, r2}}

	vs := runCheck(t, "check_verified_by_no_unrelated_reuse", g)
	if !hasViolationFor(vs, "R-1") || !hasViolationFor(vs, "R-2") {
		t.Fatalf("BROKEN: expected violations on both R-1 and R-2 for unrelated shared verified_by, got %v", vs)
	}

	// Fix path 1: explicitly relate them -- the shared test is now legitimate.
	g.Requirements[0].Relations = []ontology.Relation{{Kind: "refines", Target: "R-2"}}
	if vs := runCheck(t, "check_verified_by_no_unrelated_reuse", g); len(vs) != 0 {
		t.Fatalf("FIXED (related): expected no violations once R-1 refines R-2, got %v", vs)
	}

	// Fix path 2: unrelate them again, but remove the second citation --
	// reuse-detector must clear because there is no longer a shared entry.
	g.Requirements[0].Relations = nil
	g.Requirements[1].VerifiedBy = nil
	if vs := runCheck(t, "check_verified_by_no_unrelated_reuse", g); len(vs) != 0 {
		t.Fatalf("FIXED (no longer shared): expected no violations, got %v", vs)
	}
}

func TestCheckVerifiedByNoUnrelatedReuse_RelationTargetDirectionInsensitive(t *testing.T) {
	t.Parallel()
	entry := "spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"
	r1 := reqWithLinks("R-1", "sa", nil, []string{entry})
	r2 := reqWithLinks("R-2", "sb", nil, []string{entry})
	// Relation points the OTHER direction (R-2 -> R-1); must still count as
	// related since relatedPairIndex is symmetric.
	r2.Relations = []ontology.Relation{{Kind: "depends_on", Target: "R-1"}}
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA, sB}, Requirements: []ontology.Requirement{r1, r2}}
	if vs := runCheck(t, "check_verified_by_no_unrelated_reuse", g); len(vs) != 0 {
		t.Fatalf("expected no violations when related in the reverse direction, got %v", vs)
	}
}

// --- check_enforced_requires_enforcer_or_authored_link (disjunctive gate) --

func TestCheckEnforcedRequiresEnforcerOrAuthoredLink_OKEngineMechanism(t *testing.T) {
	t.Parallel()
	r := reqEnforced("R-1", "sa", "check_typed_anchors")
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_enforced_requires_enforcer_or_authored_link", g); len(vs) != 0 {
		t.Fatalf("expected no violations via the engine mechanism (enforced_by), got %v", vs)
	}
}

func TestCheckEnforcedRequiresEnforcerOrAuthoredLink_OKAuthoredMechanism(t *testing.T) {
	t.Parallel()
	r := reqWithLinks("R-1", "sa", []string{"spec/model/risk.go:NewRisk"}, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Enforcement = ontology.EnforcementENFORCED
	r.Status = ontology.StatusSETTLED
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_enforced_requires_enforcer_or_authored_link", g); len(vs) != 0 {
		t.Fatalf("expected no violations via the authored mechanism (implemented_by+verified_by), got %v", vs)
	}
}

// TestCheckEnforcedRequiresEnforcerOrAuthoredLink_MUTATION_NeitherMechanismFires
// is the break->fix proof for the disjunctive gate itself: an ENFORCED
// requirement with NEITHER mechanism fires; adding only implemented_by is
// still not enough (must have BOTH implemented_by and verified_by for the
// authored path); adding verified_by too clears it.
func TestCheckEnforcedRequiresEnforcerOrAuthoredLink_MUTATION_NeitherMechanismFires(t *testing.T) {
	t.Parallel()
	r := req("R-1", "sa")
	r.Enforcement = ontology.EnforcementENFORCED
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}

	vs := runCheck(t, "check_enforced_requires_enforcer_or_authored_link", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("BROKEN (neither): expected violation for ENFORCED with neither mechanism, got %v", vs)
	}

	g.Requirements[0].ImplementedBy = []string{"spec/model/risk.go:NewRisk"}
	vs2 := runCheck(t, "check_enforced_requires_enforcer_or_authored_link", g)
	if !hasViolationFor(vs2, "R-1") {
		t.Fatalf("STILL BROKEN (implemented_by only): expected violation -- authored path needs BOTH fields, got %v", vs2)
	}

	g.Requirements[0].VerifiedBy = []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"}
	if vs3 := runCheck(t, "check_enforced_requires_enforcer_or_authored_link", g); len(vs3) != 0 {
		t.Fatalf("FIXED (both fields): expected no violations once both implemented_by and verified_by are set, got %v", vs3)
	}
}

func TestCheckEnforcedRequiresEnforcerOrAuthoredLink_SkipsNonSettledOrNonEnforced(t *testing.T) {
	t.Parallel()
	r1 := reqStatus("R-1", "sa", ontology.StatusDRAFT)
	r1.Enforcement = ontology.EnforcementENFORCED
	r2 := req("R-2", "sb")
	r2.Enforcement = ontology.EnforcementPROSE
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA, sB}, Requirements: []ontology.Requirement{r1, r2}}
	if vs := runCheck(t, "check_enforced_requires_enforcer_or_authored_link", g); len(vs) != 0 {
		t.Fatalf("non-SETTLED or non-ENFORCED requirements must be skipped, got %v", vs)
	}
}

// --- Real-domain regressions: 4 domains must stay at 0 for these checks ----

const hotamDevGraphPath = "../../domains/hotam-dev/graph.json"

func TestAuthoredLinkChecks_RealHotamSpecSelfGraph_ZeroViolations(t *testing.T) {
	t.Parallel()
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", domainGraphPath, err)
	}
	for _, name := range authoredLinkCheckNames {
		if vs := runCheck(t, name, g); len(vs) != 0 {
			t.Errorf("%s: expected 0 violations on hotam-spec-self, got %d: %v", name, len(vs), vs)
		}
	}
}

func TestAuthoredLinkChecks_RealHotamDevGraph_ZeroViolations(t *testing.T) {
	t.Parallel()
	g, err := loader.LoadGraph(hotamDevGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", hotamDevGraphPath, err)
	}
	for _, name := range authoredLinkCheckNames {
		if vs := runCheck(t, name, g); len(vs) != 0 {
			t.Errorf("%s: expected 0 violations on hotam-dev, got %d: %v", name, len(vs), vs)
		}
	}
}

var authoredLinkCheckNames = []string{
	"check_implemented_by_symbol_resolvable",
	"check_verified_by_test_resolvable",
	"check_verified_by_test_has_teeth",
	"check_verified_by_test_no_skip",
	"check_verified_by_no_unrelated_reuse",
	"check_enforced_requires_enforcer_or_authored_link",
}

// --- check_enforced_names_invariant (the OLD engine-only check) must also
// be authored-aware -- this is the regression the coordinator's review
// caught: check_enforced_names_invariant (enforcement.go) and the new
// check_enforced_requires_enforcer_or_authored_link (authored_links.go) are
// the SAME disjunction stated twice (non-emptiness half / resolvability
// half). Before this fix, an authored-only ENFORCED requirement (enforced_by
// empty, implemented_by+verified_by both set -- the entire point of the
// authored path) satisfied the NEW gate but still tripped the OLD check,
// which only knew about enforced_by. Latent today (no domain has authored
// spec/ yet), but would have broken the very first authored-only ENFORCED
// requirement task #224's pilot lands.

// TestCheckEnforcedNamesInvariant_OKAuthoredOnlyMechanism is the direct
// regression test: enforced_by empty, implemented_by+verified_by both set ->
// NO violation from the OLD check (it must recognize the authored path, not
// just the engine path).
func TestCheckEnforcedNamesInvariant_OKAuthoredOnlyMechanism(t *testing.T) {
	t.Parallel()
	r := reqWithLinks("R-1", "sa", []string{"spec/model/risk.go:NewRisk"}, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Enforcement = ontology.EnforcementENFORCED
	r.Status = ontology.StatusSETTLED
	// EnforcedBy deliberately left empty -- authored-only ENFORCED requirement.
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_enforced_names_invariant", g); len(vs) != 0 {
		t.Fatalf("BROKEN (pre-fix behavior): an authored-only ENFORCED requirement (enforced_by empty, "+
			"implemented_by+verified_by both set) must NOT trip check_enforced_names_invariant -- got %v", vs)
	}
}

// TestCheckEnforcedNamesInvariant_FiresWhenAuthoredPathAlsoEmpty is the
// negative-half regression: with NEITHER mechanism present (enforced_by
// empty AND authored path incomplete/empty), the OLD check must still fire
// -- proving the fix narrowed the exemption to the authored path specifically,
// it did not just silence the check.
func TestCheckEnforcedNamesInvariant_FiresWhenAuthoredPathAlsoEmpty(t *testing.T) {
	t.Parallel()
	// Case 1: both implemented_by and verified_by empty (already covered by
	// TestCheckEnforcedNamesInvariant_FiresOnEnforcedWithEmptyEnforcedBy, but
	// asserted here again for locality with the authored-path fix).
	r1 := req("R-1", "sa")
	r1.Enforcement = ontology.EnforcementENFORCED
	g1 := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r1}}
	vs1 := runCheck(t, "check_enforced_names_invariant", g1)
	if !hasViolationFor(vs1, "R-1") {
		t.Fatalf("expected violation on R-1: neither enforced_by nor the authored path is present, got %v", vs1)
	}

	// Case 2: implemented_by set but verified_by empty -- authored path is
	// INCOMPLETE (both fields are required), so this must still fire.
	r2 := reqWithLinks("R-2", "sb", []string{"spec/model/risk.go:NewRisk"}, nil)
	r2.Enforcement = ontology.EnforcementENFORCED
	r2.Status = ontology.StatusSETTLED
	g2 := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sB}, Requirements: []ontology.Requirement{r2}}
	vs2 := runCheck(t, "check_enforced_names_invariant", g2)
	if !hasViolationFor(vs2, "R-2") {
		t.Fatalf("expected violation on R-2: implemented_by alone does not satisfy the authored path (verified_by also required), got %v", vs2)
	}
}

// TestAuthoredOnlyEnforcedRequirement_PassesFullAllViolations is the
// coordinator's acceptance test: build a domain fixture with a REAL
// authored-only ENFORCED requirement (implemented_by + verified_by pointing
// at real spec/model/*.go + *_test.go files written under g.DomainDir,
// enforced_by deliberately empty) and run the FULL invariants.AllViolations
// sweep against it -- not just the new gate, not just
// check_enforced_names_invariant in isolation, but every registered
// invariant. This proves an authored-only ENFORCED requirement is not
// merely tolerated by the disjunctive gate in isolation, but survives the
// entire mechanical checking pipeline (all 94 registered invariants) with
// zero violations, which is exactly the guarantee the authored-spec pilot
// (#224) needs before it can land its first authored-only ENFORCED
// requirement.
func TestAuthoredOnlyEnforcedRequirement_PassesFullAllViolations(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk.go", authoredRiskModelSrc)
	// writeAuthoredSpecFixture already created domainDir/spec/model/risk.go;
	// add the sibling test file to the same directory.
	testPath := filepath.Join(domainDir, "spec", "model", "risk_test.go")
	if err := os.WriteFile(testPath, []byte(authoredRiskTestGoodSrc), 0o644); err != nil {
		t.Fatalf("WriteFile risk_test.go: %v", err)
	}

	r := reqWithLinks(
		"R-authored-only-fixture", "sa",
		[]string{"spec/model/risk.go:NewRisk"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	)
	r.Enforcement = ontology.EnforcementENFORCED
	r.Status = ontology.StatusSETTLED
	r.Enforceability = ontology.EnforceabilityENFORCEABLE
	// EnforcedBy deliberately left empty: this requirement is ENFORCED
	// EXCLUSIVELY via the authored path (implemented_by + verified_by),
	// which is the entire scenario the pilot (#224) will create.

	g := &ontology.Graph{
		DomainDir:    domainDir,
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{r},
	}

	vs := AllViolations(g)
	if len(vs) != 0 {
		t.Fatalf("expected 0 violations from the FULL AllViolations sweep on an authored-only ENFORCED "+
			"requirement (real spec/model/risk.go + risk_test.go fixtures under g.DomainDir), got %d: %v",
			len(vs), vs)
	}
}

// --- self-hosting recursion (PLAN-authored-spec-discipline.md §9) ---------

const selfHostingEngineFooSrc = `package internal

func Bar() int {
	return 42
}
`

const selfHostingEngineFooTestSrc = `package internal

import "testing"

func TestBar(t *testing.T) {
	if Bar() != 42 {
		t.Fatalf("expected 42")
	}
}
`

// writeSelfHostingFixture builds a temp "engine repo" -- go.mod at the root,
// internal/foo.go + internal/foo_test.go directly under it (mirroring the
// real engine's internal/ontology/lifecycle.go + lifecycle_test.go relative
// to HotamSpec's own go.mod), and a domain directory at
// domains/<domainName>/ with manifest.json{self_hosting:true} + graph.json
// -- matching domains/hotam-spec-self's real on-disk shape. Returns the
// domain directory (g.DomainDir).
func writeSelfHostingFixture(t *testing.T, domainName string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/engine\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	internalDir := filepath.Join(root, "internal")
	if err := os.MkdirAll(internalDir, 0o755); err != nil {
		t.Fatalf("MkdirAll internal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(internalDir, "foo.go"), []byte(selfHostingEngineFooSrc), 0o644); err != nil {
		t.Fatalf("WriteFile foo.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(internalDir, "foo_test.go"), []byte(selfHostingEngineFooTestSrc), 0o644); err != nil {
		t.Fatalf("WriteFile foo_test.go: %v", err)
	}
	domainDir := filepath.Join(root, "domains", domainName)
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("MkdirAll domainDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(`{"self_hosting": true}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "graph.json"), []byte(`{"schema_version":3}`), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}
	return domainDir
}

// TestSelfHostingLinks_ImplementedByAndVerifiedBy_Resolve proves the
// self-hosting recursion at the CHECK level (not just gate.SpecRoot in
// isolation): a requirement on a graph with SelfHosting=true whose
// implemented_by/verified_by name engine-relative paths ("internal/foo.go:
// Bar", "internal/foo_test.go:TestBar") resolves cleanly through both
// check_implemented_by_symbol_resolvable and check_verified_by_test_resolvable
// -- zero violations -- because the checks now call
// gate.SpecRootForGraph(g), which walks up from g.DomainDir to the engine's
// go.mod for a self-hosting graph instead of joining onto domainDir.
func TestSelfHostingLinks_ImplementedByAndVerifiedBy_Resolve(t *testing.T) {
	t.Parallel()
	domainDir := writeSelfHostingFixture(t, "hotam-spec-self")

	r := reqWithLinks(
		"R-self-hosting-fixture", "sa",
		[]string{"internal/foo.go:Bar"},
		[]string{"internal/foo_test.go:TestBar"},
	)
	g := &ontology.Graph{
		DomainDir:    domainDir,
		SelfHosting:  true,
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{r},
	}

	if vs := runCheck(t, "check_implemented_by_symbol_resolvable", g); len(vs) != 0 {
		t.Fatalf("expected implemented_by internal/foo.go:Bar to resolve against the engine root for a "+
			"self-hosting domain, got violations: %v", vs)
	}
	if vs := runCheck(t, "check_verified_by_test_resolvable", g); len(vs) != 0 {
		t.Fatalf("expected verified_by internal/foo_test.go:TestBar to resolve against the engine root for a "+
			"self-hosting domain, got violations: %v", vs)
	}
}

// TestSelfHostingLinks_MUTATION_RemoveSymbolAndTestFlipsToViolations is the
// break->fix mutation proof at the check level: remove Bar from
// internal/foo.go and TestBar from internal/foo_test.go (simulating engine
// code being renamed/deleted out from under a self-hosting implemented_by/
// verified_by reference) and confirm BOTH checks flip from zero violations
// to firing; restoring the engine files makes them pass again.
func TestSelfHostingLinks_MUTATION_RemoveSymbolAndTestFlipsToViolations(t *testing.T) {
	t.Parallel()
	domainDir := writeSelfHostingFixture(t, "hotam-spec-self")
	// domainDir is <root>/domains/hotam-spec-self -- the engine root is two
	// levels up, exactly as gate.SpecRoot's go.mod walk finds it.
	engineRoot := filepath.Dir(filepath.Dir(domainDir))
	fooPath := filepath.Join(engineRoot, "internal", "foo.go")
	fooTestPath := filepath.Join(engineRoot, "internal", "foo_test.go")

	r := reqWithLinks(
		"R-self-hosting-mutation", "sa",
		[]string{"internal/foo.go:Bar"},
		[]string{"internal/foo_test.go:TestBar"},
	)
	g := &ontology.Graph{
		DomainDir:    domainDir,
		SelfHosting:  true,
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{r},
	}

	if vs := runCheck(t, "check_implemented_by_symbol_resolvable", g); len(vs) != 0 {
		t.Fatalf("INTACT: expected no implemented_by violations, got %v", vs)
	}
	if vs := runCheck(t, "check_verified_by_test_resolvable", g); len(vs) != 0 {
		t.Fatalf("INTACT: expected no verified_by violations, got %v", vs)
	}

	// Mutate: rename the symbol and the test away.
	mutatedFoo := `package internal

func Baz() int {
	return 0
}
`
	if err := os.WriteFile(fooPath, []byte(mutatedFoo), 0o644); err != nil {
		t.Fatalf("WriteFile mutate foo.go: %v", err)
	}
	mutatedFooTest := `package internal

import "testing"

func TestBaz(t *testing.T) {
	if Baz() != 0 {
		t.Fatalf("expected 0")
	}
}
`
	if err := os.WriteFile(fooTestPath, []byte(mutatedFooTest), 0o644); err != nil {
		t.Fatalf("WriteFile mutate foo_test.go: %v", err)
	}

	if vs := runCheck(t, "check_implemented_by_symbol_resolvable", g); !hasViolationFor(vs, "R-self-hosting-mutation") {
		t.Fatalf("BROKEN CASE: expected a violation after removing Bar from the engine file, got %v", vs)
	}
	if vs := runCheck(t, "check_verified_by_test_resolvable", g); !hasViolationFor(vs, "R-self-hosting-mutation") {
		t.Fatalf("BROKEN CASE: expected a violation after removing TestBar from the engine test file, got %v", vs)
	}

	// Fix: restore both engine files.
	if err := os.WriteFile(fooPath, []byte(selfHostingEngineFooSrc), 0o644); err != nil {
		t.Fatalf("WriteFile restore foo.go: %v", err)
	}
	if err := os.WriteFile(fooTestPath, []byte(selfHostingEngineFooTestSrc), 0o644); err != nil {
		t.Fatalf("WriteFile restore foo_test.go: %v", err)
	}
	if vs := runCheck(t, "check_implemented_by_symbol_resolvable", g); len(vs) != 0 {
		t.Fatalf("FIXED CASE: expected no implemented_by violations after restoring the engine file, got %v", vs)
	}
	if vs := runCheck(t, "check_verified_by_test_resolvable", g); len(vs) != 0 {
		t.Fatalf("FIXED CASE: expected no verified_by violations after restoring the engine test file, got %v", vs)
	}
}

// TestNonSelfHostingLinks_StillResolveAgainstDomainDir is the regression
// proof at the check level: an ordinary (non-self-hosting, SelfHosting
// false/zero-value) domain's implemented_by/verified_by entries keep
// resolving against g.DomainDir exactly as before -- the pilot domain (prat,
// spec/model/risk.go) is not affected by the self-hosting engine-root walk.
func TestNonSelfHostingLinks_StillResolveAgainstDomainDir(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk.go", authoredRiskModelSrc)
	r := reqWithLinks("R-non-self-hosting", "sa", []string{"spec/model/risk.go:NewRisk"}, nil)
	g := &ontology.Graph{
		DomainDir: domainDir,
		// SelfHosting deliberately left at its zero value (false).
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{r},
	}
	if vs := runCheck(t, "check_implemented_by_symbol_resolvable", g); len(vs) != 0 {
		t.Fatalf("expected a non-self-hosting domain to still resolve implemented_by against domainDir, got %v", vs)
	}
}
