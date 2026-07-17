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

// authoredRiskModelSrc is a genuinely-passing fixture: NewRisk actually
// rejects a missing owner (not just AST-shaped to look like it does), so
// authoredRiskTestGoodSrc below is a REAL passing proof once
// check_verified_by_test_passes (internal/gate.RunVerifiedByTest, @fh
// finding F1) actually compiles and runs it -- not merely a test whose body
// LOOKS like it asserts something, which is all the AST-only checks above
// this check in the pipeline (resolvable/has-teeth/no-skip) can verify. An
// earlier revision of this fixture had NewRisk unconditionally succeed
// (`return &Risk{Owner: owner}, nil` with no validation at all) while
// authoredRiskTestGoodSrc asserted the opposite -- exactly the F1 failure
// shape (a red proof coexisting with every AST-only check reporting green)
// -- and check_verified_by_test_passes correctly flagged it the first time
// this fixture was ever actually executed instead of just AST-inspected,
// which is the bug this whole remediation exists to catch.
const authoredRiskModelSrc = `package model

import "errors"

type Risk struct {
	Owner string
}

func NewRisk(owner string) (*Risk, error) {
	if owner == "" {
		return nil, errors.New("owner is required")
	}
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
// entire mechanical checking pipeline (every registered invariant) with
// zero violations, which is exactly the guarantee the authored-spec pilot
// (#224) needs before it can land its first authored-only ENFORCED
// requirement. The fixture carries a real go.mod at domainDir (mirroring
// PLAN-authored-spec-discipline.md's "prat-spec" module convention for an
// ordinary domain's spec/ tree) so check_verified_by_test_passes
// (internal/gate's RunVerifiedByTest, @fh finding F1) can actually compile
// and run TestNewRisk_RejectsMissingOwner via `go test`, not just resolve it
// by AST shape -- without a real module this check would report an
// infrastructure violation ("no go.mod found"), which would defeat the
// point of this being the FULL-sweep zero-violations acceptance test.
func TestAuthoredOnlyEnforcedRequirement_PassesFullAllViolations(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk.go", authoredRiskModelSrc)
	if err := os.WriteFile(filepath.Join(domainDir, "go.mod"), []byte("module prat-spec\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
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

// authoredUnrelatedArithmeticTestSrc is a REAL, structurally-perfect test --
// it resolves, has teeth (a genuine t.Fatalf assertion), is not vacuous, is
// not t.Skip -- that proves something true but has NOTHING to do with Risk
// OWNERSHIP VALIDATION SEMANTICS: it calls NewRisk (so a real coverage-proof
// run genuinely executes the symbol's lines -- check_scenario_executes_impl,
// task W2.2, PLAN-scenario-generated-spec.md §2 D3, closes exactly the
// narrower case where a cited test never touches the implemented_by symbol
// AT ALL) but then asserts a completely unrelated arithmetic fact instead of
// anything about the returned Risk or its owner. It exists to demonstrate
// R-structural-floor-vs-mirror-audit's own claim in its FULL, still-standing
// form even after coverage-proof: the engine's mechanical checks -- now
// including "did this run touch the symbol's lines" -- still cannot and do
// not verify that a cited test's ASSERTIONS semantically prove the citing
// requirement's claim, only that it structurally resolves, has real
// assertions, passes, AND executes the right lines. A test that dutifully
// calls the right function and then asserts something irrelevant about an
// unrelated value is the residual gap only a human/LLM mirror audit can
// close -- coverage-proof narrows the forgeable surface (a test that never
// even CALLS the implementation can no longer hide behind AST-only checks)
// without eliminating the semantic-adequacy gap entirely.
const authoredUnrelatedArithmeticTestSrc = `package model

import "testing"

func TestArithmetic_TwoPlusTwoIsFour(t *testing.T) {
	r, err := NewRisk("someone")
	if err != nil {
		t.Fatalf("NewRisk(%q) unexpected error: %v", "someone", err)
	}
	_ = r
	if 2+2 != 4 {
		t.Fatalf("expected 4")
	}
}
`

// TestStructuralFloorDoesNotCatchSemanticMismatch is the carrier test for
// R-structural-floor-vs-mirror-audit's own claim text: the engine holds the
// STRUCTURAL floor (symbol/test resolves, has teeth, not vacuous, not
// skipped, not orphaned, not suspiciously reused) while a SEPARATE mirror
// audit -- a human or LLM reading (requirement claim, code+test) together --
// is the only thing that can certify the cited test SEMANTICALLY proves the
// requirement's claim; "the engine's structural checks shall NEVER be
// represented, documented, or relied upon as a substitute for the mirror
// audit's semantic certification."
//
// This test proves that boundary operationally, not just in prose: it wires
// a requirement whose CLAIM is about Risk ownership validation to a
// verified_by test (authoredUnrelatedArithmeticTestSrc) that is genuinely
// unrelated IN ITS ASSERTIONS -- it calls NewRisk (so it DOES execute the
// implemented_by symbol's own lines, satisfying task W2.2's coverage-proof,
// check_scenario_executes_impl) but then asserts a bare arithmetic fact that
// has nothing to do with Risk ownership -- yet passes EVERY mechanical check
// the engine has, INCLUDING coverage-proof (resolves, has teeth, not
// vacuous, not skipped, not reused, actually passes, actually executes the
// symbol's lines), and consequently the FULL AllViolations sweep reports
// ZERO violations for this obviously-wrong pairing. That residual gap -- 0
// mechanical violations for a SpecLink no mirror audit would ever accept,
// even once the engine can also mechanically prove the test executed the
// right lines -- is exactly what the requirement's claim asserts exists and
// must never be papered over by encoding lexical/keyword heuristics into the
// graph invariant layer. (An EARLIER revision of this fixture used a test
// that never called NewRisk at all -- pure `if 2+2 != 4` with no call into
// the implementation -- and task W2.2's coverage-proof correctly started
// rejecting THAT shape, exactly as its own forge-probe requires: a cited
// test that never touches the implemented_by symbol's lines is no longer a
// gap only a mirror audit can catch, it is now a mechanical violation. The
// fixture was revised, deliberately and visibly in this comment, to call the
// real symbol while still asserting something semantically irrelevant, so
// the still-standing part of the thesis remains provable.) If a future change
// made the engine start rejecting THIS narrower fixture too (e.g. by adding
// a claim-vs-test keyword-similarity check, or an assertion-target dataflow
// check), THIS test would need to be revisited -- deliberately, as a
// recorded decision to fold more semantic judgment into the mechanical
// layer -- not silently, which is exactly the property this test is here to
// guard.
func TestStructuralFloorDoesNotCatchSemanticMismatch(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk.go", authoredRiskModelSrc)
	if err := os.WriteFile(filepath.Join(domainDir, "go.mod"), []byte("module prat-spec\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	testPath := filepath.Join(domainDir, "spec", "model", "arithmetic_test.go")
	if err := os.WriteFile(testPath, []byte(authoredUnrelatedArithmeticTestSrc), 0o644); err != nil {
		t.Fatalf("WriteFile arithmetic_test.go: %v", err)
	}

	r := reqWithLinks(
		"R-semantic-mismatch-fixture", "sa",
		[]string{"spec/model/risk.go:NewRisk"},
		[]string{"spec/model/arithmetic_test.go:TestArithmetic_TwoPlusTwoIsFour"},
	)
	r.Claim = "Risk creation MUST reject a missing owner."
	r.Enforcement = ontology.EnforcementENFORCED
	r.Status = ontology.StatusSETTLED
	r.Enforceability = ontology.EnforceabilityENFORCEABLE

	g := &ontology.Graph{
		DomainDir:    domainDir,
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{r},
	}

	vs := AllViolations(g)
	if len(vs) != 0 {
		t.Fatalf("expected the FULL AllViolations sweep to report 0 violations for a semantically-unrelated "+
			"but structurally-perfect verified_by test -- that gap IS R-structural-floor-vs-mirror-audit's own "+
			"claim (structural checks cannot and must not substitute for a semantic mirror audit); got %d: %v",
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

// --- F2 remediation (@fh Probe D): scope-escape reproduction at the check
// level ------------------------------------------------------------------

// TestCheckImplementedBySymbolResolvable_ParentTraversal_Fires is the direct
// check-level reproduction of @fh Probe D: a non-self-hosting domain (prat-
// shaped: DomainDir with a spec/ tree, SelfHosting=false) declaring
// implemented_by "../HotamSpec/internal/ontology/lifecycle.go:Lifecycle"
// used to resolve cleanly (0 violations) because the resolver just joined
// the path onto domainDir with no scope check. It must now fire.
func TestCheckImplementedBySymbolResolvable_ParentTraversal_Fires(t *testing.T) {
	t.Parallel()
	// domainDir/spec/model/risk.go exists (a normal pilot-shaped domain);
	// the escape reference is deliberately a sibling of domainDir, mimicking
	// "../HotamSpec/internal/..." reaching outside the domain root.
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk.go", authoredRiskModelSrc)
	outsideDir := filepath.Dir(domainDir)
	if err := os.MkdirAll(filepath.Join(outsideDir, "internal", "ontology"), 0o755); err != nil {
		t.Fatalf("MkdirAll outside engine dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "internal", "ontology", "lifecycle.go"), []byte(`package ontology

type Lifecycle struct{}
`), 0o644); err != nil {
		t.Fatalf("WriteFile outside lifecycle.go: %v", err)
	}

	escapeRel := "../" + filepath.Base(outsideDir) + "/internal/ontology/lifecycle.go:Lifecycle"
	// Simpler, and matching the review's literal probe text more closely:
	// climb straight out of domainDir via "..".
	escapeRel = "../internal/ontology/lifecycle.go:Lifecycle"

	r := reqWithLinks("R-probe-d", "sa", []string{escapeRel}, nil)
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}

	vs := runCheck(t, "check_implemented_by_symbol_resolvable", g)
	if !hasViolationFor(vs, "R-probe-d") {
		t.Fatalf("BROKEN CASE (Probe D): expected a violation for a \"..\"-escaping implemented_by entry, got %v", vs)
	}
}

// TestCheckImplementedBySymbolResolvable_LegitimateSpecReference_StillOK is
// the companion proof that the F2 scope gate does NOT break the legitimate
// pilot shape: a normal "spec/model/risk.go:NewRisk" entry keeps resolving
// to zero violations.
func TestCheckImplementedBySymbolResolvable_LegitimateSpecReference_StillOK(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk.go", authoredRiskModelSrc)
	r := reqWithLinks("R-legit", "sa", []string{"spec/model/risk.go:NewRisk"}, nil)
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_implemented_by_symbol_resolvable", g); len(vs) != 0 {
		t.Fatalf("expected legitimate spec/ implemented_by entry to still resolve with 0 violations, got %v", vs)
	}
}

// TestCheckVerifiedByTestResolvable_ParentTraversal_Fires is the verified_by
// counterpart of the Probe D reproduction.
func TestCheckVerifiedByTestResolvable_ParentTraversal_Fires(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk.go", authoredRiskModelSrc)
	outsideDir := filepath.Dir(domainDir)
	if err := os.MkdirAll(filepath.Join(outsideDir, "internal", "ontology"), 0o755); err != nil {
		t.Fatalf("MkdirAll outside engine dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "internal", "ontology", "lifecycle_test.go"), []byte(`package ontology

import "testing"

func TestLifecycle(t *testing.T) {
	t.Fatalf("should never resolve from outside the domain scope")
}
`), 0o644); err != nil {
		t.Fatalf("WriteFile outside lifecycle_test.go: %v", err)
	}

	r := reqWithLinks("R-probe-d-verified", "sa", nil, []string{"../internal/ontology/lifecycle_test.go:TestLifecycle"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}

	vs := runCheck(t, "check_verified_by_test_resolvable", g)
	if !hasViolationFor(vs, "R-probe-d-verified") {
		t.Fatalf("BROKEN CASE (Probe D): expected a violation for a \"..\"-escaping verified_by entry, got %v", vs)
	}
}

// TestCheckImplementedBySymbolResolvable_SelfHosting_ParentTraversal_Fires
// proves the self-hosting path is equally hardened: an implemented_by entry
// that climbs out of the engine repository root via ".." must fire even
// though self-hosting has no required "spec/" prefix.
func TestCheckImplementedBySymbolResolvable_SelfHosting_ParentTraversal_Fires(t *testing.T) {
	t.Parallel()
	domainDir := writeSelfHostingFixture(t, "hotam-spec-self")
	engineRoot := filepath.Dir(filepath.Dir(domainDir))
	outsideDir := filepath.Dir(engineRoot)
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("MkdirAll outside engine root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "evil.go"), []byte(`package evil

func Evil() {}
`), 0o644); err != nil {
		t.Fatalf("WriteFile evil.go: %v", err)
	}

	r := reqWithLinks("R-self-hosting-escape", "sa", []string{"../evil.go:Evil"}, nil)
	g := &ontology.Graph{DomainDir: domainDir, SelfHosting: true, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}

	vs := runCheck(t, "check_implemented_by_symbol_resolvable", g)
	if !hasViolationFor(vs, "R-self-hosting-escape") {
		t.Fatalf("BROKEN CASE: expected a violation for a self-hosting \"..\"-escaping implemented_by entry, got %v", vs)
	}
}

// TestAuthoredLinkChecks_RealPratDomain_ZeroViolations_WithLegitimateLinks is
// the acceptance-level proof that the F2 scope gate does not regress the
// real pilot domain: loading ../PRAT-hotam/domains/prat's actual graph.json
// (which carries real "spec/model/*.go" implemented_by/verified_by entries)
// through the resolvable checks still yields zero violations from those two
// checks. Skips (does not fail) if the sibling repository checkout is not
// present in this environment.
func TestAuthoredLinkChecks_RealPratDomain_ZeroViolations_WithLegitimateLinks(t *testing.T) {
	t.Parallel()
	domainDir := findSiblingDomainDir(t, "PRAT-hotam", "prat")
	if domainDir == "" {
		t.Skip("sibling ../PRAT-hotam/domains/prat checkout not present in this environment")
	}
	graphPath := filepath.Join(domainDir, "graph.json")
	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("loader.LoadGraph(%s): %v", graphPath, err)
	}
	for _, name := range []string{"check_implemented_by_symbol_resolvable", "check_verified_by_test_resolvable"} {
		if vs := runCheck(t, name, g); len(vs) != 0 {
			t.Fatalf("expected 0 violations from %s on the real prat domain (legitimate spec/ links), got %v", name, vs)
		}
	}
}

// findSiblingDomainDir looks for <repoRoot>/../<siblingRepo>/domains/<domain>
// relative to the HotamSpec module root (found by walking up from the
// current test file's working directory to go.mod), returning "" if it does
// not exist -- lets acceptance tests against the real sibling checkouts
// degrade to a skip rather than a hard failure in an environment that does
// not have the sibling repository checked out alongside HotamSpec.
func findSiblingDomainDir(t *testing.T, siblingRepo, domain string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	dir := wd
	for {
		if info, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil && !info.IsDir() {
			candidate := filepath.Join(dir, "..", siblingRepo, "domains", domain)
			if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
				return candidate
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// --- F3 remediation (@fh Probe A) at the check level -----------------------

// TestCheckVerifiedByTestHasTeeth_EmptyLoop_Fires is the check-level
// reproduction of @fh Probe A: a verified_by test whose body is only an
// empty loop (no real assertion) used to pass check_verified_by_test_has_teeth
// because any bare for/if/switch counted as "teeth" by shape. It must now
// fire.
func TestCheckVerifiedByTestHasTeeth_EmptyLoop_Fires(t *testing.T) {
	t.Parallel()
	const emptyLoopSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	for i := 0; i < 0; i++ {
	}
}
`
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", emptyLoopSrc)
	r := reqWithLinks("R-probe-a-loop", "sa", nil, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}

	vs := runCheck(t, "check_verified_by_test_has_teeth", g)
	if !hasViolationFor(vs, "R-probe-a-loop") {
		t.Fatalf("BROKEN CASE (Probe A): expected a violation for an empty-loop verified_by test with no assertion, got %v", vs)
	}
}

// TestCheckVerifiedByTestHasTeeth_RealAssertion_StillOK is the companion
// proof that the F3 hardening does not break a real, honestly-asserting
// verified_by test.
func TestCheckVerifiedByTestHasTeeth_RealAssertion_StillOK(t *testing.T) {
	t.Parallel()
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", authoredRiskTestGoodSrc)
	r := reqWithLinks("R-real-assert", "sa", nil, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_verified_by_test_has_teeth", g); len(vs) != 0 {
		t.Fatalf("expected a real-assertion verified_by test to keep passing check_verified_by_test_has_teeth, got %v", vs)
	}
}

// TestCheckVerifiedByTestNoSkip_IfTrueSkip_Fires is the check-level
// reproduction of @fh Probe A's second half: `if true { t.Skip(...) }` used
// to slip past check_verified_by_test_no_skip because the skip was not a
// direct top-level statement. It must now fire.
func TestCheckVerifiedByTestNoSkip_IfTrueSkip_Fires(t *testing.T) {
	t.Parallel()
	const ifTrueSkipSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	if true {
		t.Skip("dressed-up unconditional skip")
	}
}
`
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", ifTrueSkipSrc)
	r := reqWithLinks("R-probe-a-iftrueskip", "sa", nil, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}

	vs := runCheck(t, "check_verified_by_test_no_skip", g)
	if !hasViolationFor(vs, "R-probe-a-iftrueskip") {
		t.Fatalf("BROKEN CASE (Probe A): expected a violation for `if true { t.Skip(...) }`, got %v", vs)
	}
}

// TestCheckVerifiedByTestNoSkip_ConditionalTestingShortSkip_StillOK is the
// companion non-regression proof: the genuine testing.Short() idiom must
// still pass check_verified_by_test_no_skip after the F3 hardening.
func TestCheckVerifiedByTestNoSkip_ConditionalTestingShortSkip_StillOK(t *testing.T) {
	t.Parallel()
	const conditionalSkipSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("slow test")
	}
	r, err := NewRisk("")
	if err == nil {
		t.Fatalf("expected error, got %v", r)
	}
}
`
	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk_test.go", conditionalSkipSrc)
	r := reqWithLinks("R-conditional-skip-ok", "sa", nil, []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_verified_by_test_no_skip", g); len(vs) != 0 {
		t.Fatalf("REGRESSION: expected the testing.Short() idiom to still pass check_verified_by_test_no_skip, got %v", vs)
	}
}

// TestHonoredSkipWarnings_MUTATION_SkipIsVisibleNotSilent is the mutation
// proof for @fh's "honored-skip must not be silent" re-review: with
// gate.RunVerifiedByTest's own recursion guard env var
// (HOTAM_VERIFIED_BY_EXEC_GUARD) set on THIS test process -- simulating
// exactly the state a genuine nested `go test` child RunVerifiedByTest
// itself spawned would observe -- a real, otherwise-passing verified_by
// entry is Skipped rather than actually executed. Before this fix, that
// Skipped result was dropped on the floor entirely: checkVerifiedByTestPasses
// reported zero violations (correct -- Skipped is "unproven here", never a
// false pass or false fail) and NOTHING else in AllViolations' output ever
// mentioned the skip happened, so a run with every entry silently deferred
// looked byte-identical to a run where every entry was genuinely proven.
// This test proves BOTH halves of the fix: the blocking check still reports
// zero violations for the Skipped entry (Skip is not itself a proof
// failure), AND HonoredSkipWarnings now reports a non-empty, visible warning
// naming the requirement -- the skip is no longer invisible.
func TestHonoredSkipWarnings_MUTATION_SkipIsVisibleNotSilent(t *testing.T) {
	// Deliberately NOT t.Parallel(): mutates process-global environment
	// (HOTAM_VERIFIED_BY_EXEC_GUARD) that gate.RunVerifiedByTest reads for
	// EVERY call in this process, including from other tests running
	// concurrently in this same package.
	const guardEnv = "HOTAM_VERIFIED_BY_EXEC_GUARD"
	prev, hadPrev := os.LookupEnv(guardEnv)
	if err := os.Setenv(guardEnv, "test-simulated-nested-go-test-child"); err != nil {
		t.Fatalf("Setenv: %v", err)
	}
	t.Cleanup(func() {
		if hadPrev {
			os.Setenv(guardEnv, prev)
		} else {
			os.Unsetenv(guardEnv)
		}
	})

	domainDir := writeAuthoredSpecFixture(t, "spec/model/risk.go", authoredRiskModelSrc)
	if err := os.WriteFile(filepath.Join(domainDir, "go.mod"), []byte("module prat-spec-honored-skip\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	testPath := filepath.Join(domainDir, "spec", "model", "risk_test.go")
	if err := os.WriteFile(testPath, []byte(authoredRiskTestGoodSrc), 0o644); err != nil {
		t.Fatalf("WriteFile risk_test.go: %v", err)
	}

	r := reqWithLinks(
		"R-honored-skip-visible", "sa",
		[]string{"spec/model/risk.go:NewRisk"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	)
	g := &ontology.Graph{DomainDir: domainDir, Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}

	blocking := runCheck(t, "check_verified_by_test_passes", g)
	if len(blocking) != 0 {
		t.Fatalf("expected NO blocking violation for an honored-Skipped entry (Skip is 'unproven here', never a false failure), got %v", blocking)
	}

	warnings := HonoredSkipWarnings(g)
	if len(warnings) == 0 {
		t.Fatalf("SILENT HONORED SKIP: expected HonoredSkipWarnings to report a non-blocking warning for the Skipped verified_by entry, got none")
	}
	if !hasViolationFor(warnings, "R-honored-skip-visible") {
		t.Fatalf("expected a warning naming R-honored-skip-visible, got %v", warnings)
	}
	for _, w := range warnings {
		if w.Message == "" {
			t.Fatalf("expected a non-empty warning message, got %+v", w)
		}
	}
}
