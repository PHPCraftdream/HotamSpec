package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// writeCoverageFixture writes a minimal Go module (go.mod + one model file
// under spec/model/ + one test file under spec/model/) so
// check_scenario_executes_impl has a real module to run `go test
// -coverprofile` against -- mirrors writeAuthoredSpecFixture (authored_links_test.go)
// but additionally writes go.mod, since RunVerifiedByTestRecording needs a
// real module root to compute -coverpkg from (gate.ImportPathForFile reads
// go.mod's own "module " directive).
func writeCoverageFixture(t *testing.T, modulePath, implSrc, testSrc string) string {
	t.Helper()
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}
	modelDir := filepath.Join(tmp, "spec", "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "risk.go"), []byte(implSrc), 0o644); err != nil {
		t.Fatalf("WriteFile risk.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "risk_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatalf("WriteFile risk_test.go: %v", err)
	}
	return tmp
}

// coverageFixtureImplSrc is the SAME shape as authoredRiskModelSrc
// (authored_links_test.go) -- a real, non-trivial NewRisk with a genuine
// branch (reject empty owner / accept non-empty owner) so a real coverage
// run has something meaningful to prove was or was not touched.
const coverageFixtureImplSrc = `package model

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
`

// coverageFixtureRealTestSrc is the FORGE-PROBE's NEGATIVE control: a
// verified_by test that actually calls NewRisk and asserts something about
// its behavior -- check_scenario_executes_impl MUST NOT fire a violation for
// this pairing (real coverage of the symbol's lines exists).
const coverageFixtureRealTestSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	r, err := NewRisk("")
	if err == nil {
		t.Fatalf("expected error for missing owner, got risk=%v", r)
	}
}
`

// coverageFixtureForgedTestSrc is the FORGE-PROBE's POSITIVE control (the
// core value of task W2.2, per its own instructions): a verified_by test
// that is structurally perfect -- resolves, has real teeth (a genuine
// t.Fatalf), is not vacuous, is not skipped, and PASSES when run -- yet
// NEVER calls NewRisk at all. Every check that predates check_scenario_executes_impl
// (resolvable/has-teeth/no-skip/passes) reports this pairing clean;
// check_scenario_executes_impl MUST be the one check that turns red for it,
// because a real coverage profile from actually running this test contains
// zero covered lines anywhere inside NewRisk's own declaration range.
const coverageFixtureForgedTestSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	if 2+2 != 4 {
		t.Fatalf("arithmetic is broken")
	}
}
`

// TestCheckScenarioExecutesImpl_ForgedTest_NeverCallsImpl_Fires is the
// FORGE-PROBE task W2.2's own instructions require: a verified_by test that
// is green and structurally perfect but never once calls the implemented_by
// symbol must make check_scenario_executes_impl fire, mechanically proving
// the coverage-proof gate -- not merely the AST/pass-fail checks that predate
// it -- is what closes this forge vector.
func TestCheckScenarioExecutesImpl_ForgedTest_NeverCallsImpl_Fires(t *testing.T) {
	t.Parallel()
	domainDir := writeCoverageFixture(t, "forgedmod", coverageFixtureImplSrc, coverageFixtureForgedTestSrc)

	r := reqWithLinks(
		"R-forged-coverage-fixture", "sa",
		[]string{"spec/model/risk.go:NewRisk"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	)
	r.Enforcement = ontology.EnforcementENFORCED
	r.Status = ontology.StatusSETTLED
	r.Enforceability = ontology.EnforceabilityENFORCEABLE

	g := &ontology.Graph{
		DomainDir:    domainDir,
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{r},
	}

	vs := runCheck(t, "check_scenario_executes_impl", g)
	if !hasViolationFor(vs, "R-forged-coverage-fixture") {
		t.Fatalf("expected check_scenario_executes_impl to fire for a verified_by test that never calls the "+
			"implemented_by symbol (forge probe) -- got %d violations: %+v", len(vs), vs)
	}
}

// TestCheckScenarioExecutesImpl_RealTest_CallsImpl_DoesNotFire is the
// NEGATIVE control proving the same check does NOT fire for a genuine test
// that actually calls and exercises the implemented_by symbol -- the forge
// probe above is only meaningful if this pairing (real coverage) stays
// clean; otherwise the check would be trivially "always red" rather than
// actually discriminating covered from uncovered.
func TestCheckScenarioExecutesImpl_RealTest_CallsImpl_DoesNotFire(t *testing.T) {
	t.Parallel()
	domainDir := writeCoverageFixture(t, "realmod", coverageFixtureImplSrc, coverageFixtureRealTestSrc)

	r := reqWithLinks(
		"R-real-coverage-fixture", "sa",
		[]string{"spec/model/risk.go:NewRisk"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	)
	r.Enforcement = ontology.EnforcementENFORCED
	r.Status = ontology.StatusSETTLED
	r.Enforceability = ontology.EnforceabilityENFORCEABLE

	g := &ontology.Graph{
		DomainDir:    domainDir,
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{r},
	}

	vs := runCheck(t, "check_scenario_executes_impl", g)
	if hasViolationFor(vs, "R-real-coverage-fixture") {
		t.Fatalf("expected check_scenario_executes_impl to NOT fire for a verified_by test that genuinely calls and "+
			"exercises the implemented_by symbol -- got violations: %+v", vs)
	}
}

// TestCheckScenarioExecutesImpl_NoImplementedBy_NoOp proves the engine-path /
// empty-field NO-OP boundary: a requirement with no implemented_by (or no
// verified_by) contributes zero violations from this check, regardless of
// anything else about it -- the same honesty boundary every authored-link
// check in authored_links.go already documents.
func TestCheckScenarioExecutesImpl_NoImplementedBy_NoOp(t *testing.T) {
	t.Parallel()
	r := req("R-engine-path-only", "sa")
	r.EnforcedBy = []string{"check_something_else"}
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{r},
	}
	vs := runCheck(t, "check_scenario_executes_impl", g)
	if len(vs) != 0 {
		t.Fatalf("expected 0 violations for a requirement with no implemented_by/verified_by (engine path only), got %+v", vs)
	}
}

// TestCheckScenarioExecutesImpl_UnresolvableImplementedBy_NoOp proves this
// check defers to checkImplementedBySymbolResolvable for a stale/unresolvable
// implemented_by entry rather than double-reporting or panicking on it.
func TestCheckScenarioExecutesImpl_UnresolvableImplementedBy_NoOp(t *testing.T) {
	t.Parallel()
	domainDir := writeCoverageFixture(t, "unresolvedmod", coverageFixtureImplSrc, coverageFixtureRealTestSrc)

	r := reqWithLinks(
		"R-unresolvable-impl", "sa",
		[]string{"spec/model/risk.go:DoesNotExist"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	)
	g := &ontology.Graph{
		DomainDir:    domainDir,
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{r},
	}
	vs := runCheck(t, "check_scenario_executes_impl", g)
	if hasViolationFor(vs, "R-unresolvable-impl") {
		t.Fatalf("expected check_scenario_executes_impl to defer to checkImplementedBySymbolResolvable for an "+
			"unresolvable implemented_by entry, not report its own violation -- got %+v", vs)
	}
}

// --- F1: type-only implemented_by bypass (task W7.2, @fx finding F1) -------
//
// A SETTLED requirement in a discipline:full domain whose ENTIRE implemented_by
// set consists of type declarations (struct/interface -- no coverable range)
// must NOT pass the coverage-proof gate having proven ZERO execution. Before
// F1, ResolveSpecSymbolRange returned found=false for type-only entries and
// check_scenario_executes_impl silently `continue`d with no violation --
// meaning such a requirement passed the coverage-proof gate (the one gate
// whose entire purpose is to prove execution) having proven nothing at all.

// writeCoverageFixtureWithDiscipline extends writeCoverageFixture with a
// manifest.json carrying the given discipline value + a graph.json, so the
// fixture's resolved discipline (loader.ResolveDiscipline) matches what a
// real discipline:full domain would load. Mirrors scenario_discipline_test.go's
// writeScenarioDisciplineFixture convention applied to the coverage fixture.
func writeCoverageFixtureWithDiscipline(t *testing.T, modulePath, implSrc, testSrc, discipline string) string {
	t.Helper()
	domainDir := writeCoverageFixture(t, modulePath, implSrc, testSrc)
	manifest := `{"discipline": "` + discipline + `"}` + "\n"
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "graph.json"), []byte(`{"schema_version":3}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile graph.json: %v", err)
	}
	return domainDir
}

// coverageGraphForDiscipline builds a graph with Discipline resolved from the
// fixture's own manifest.json (mirrors graphForDiscipline from
// scenario_discipline_test.go), so the check sees the same shape a real
// `hotam all-violations` run against a discipline:full domain would.
func coverageGraphForDiscipline(t *testing.T, domainDir string, reqs []ontology.Requirement) *ontology.Graph {
	t.Helper()
	graphPath := filepath.Join(domainDir, "graph.json")
	return &ontology.Graph{
		DomainDir:    domainDir,
		Discipline:   loader.ResolveDiscipline(graphPath),
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: reqs,
	}
}

// TestCheckScenarioExecutesImpl_F1_TypeOnlyImplementedBy_Fires is the EXPLOIT
// case F1 targets: a SETTLED requirement in a discipline:full domain whose
// implemented_by cites ONLY a type declaration (spec/model/risk.go:Risk) and a
// verified_by test that never meaningfully touches it. Before F1, this passed
// the coverage-proof gate silently (ResolveSpecSymbolRange returns found=false
// for a type, the loop continued with no job, no violation). After F1, this
// must fire -- the requirement claims to be implemented, but nothing in that
// claim is even theoretically checkable by this coverage-proof gate.
func TestCheckScenarioExecutesImpl_F1_TypeOnlyImplementedBy_Fires(t *testing.T) {
	t.Parallel()
	domainDir := writeCoverageFixtureWithDiscipline(t, "typeonlymod",
		coverageFixtureImplSrc, coverageFixtureForgedTestSrc, "full")

	r := reqWithLinks(
		"R-f1-type-only-exploit", "sa",
		[]string{"spec/model/risk.go:Risk"}, // type-only -- no coverable range
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	)
	r.Status = ontology.StatusSETTLED

	g := coverageGraphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if g.Discipline != loader.DisciplineFull {
		t.Fatalf("test setup: expected Discipline=%q, got %q", loader.DisciplineFull, g.Discipline)
	}
	vs := runCheck(t, "check_scenario_executes_impl", g)
	if !hasViolationFor(vs, "R-f1-type-only-exploit") {
		t.Fatalf("F1: expected check_scenario_executes_impl to fire for a SETTLED discipline:full requirement "+
			"with type-only implemented_by (the exploit case) -- got %d violations: %+v", len(vs), vs)
	}
}

// TestCheckScenarioExecutesImpl_F1_MixedTypeAndMethod_DoesNotFire is the
// LEGITIMATE mixed-citation case F1 must NOT break: implemented_by cites BOTH
// a type (for context) AND a method (the real logic), where the method IS
// covered by the verified_by test. coverableCount > 0 (the method's range
// resolves), so the F1 gate does not fire, and the real coverage run proves
// the method was executed. This is the non-regression case for the mixed
// citation pattern.
func TestCheckScenarioExecutesImpl_F1_MixedTypeAndMethod_DoesNotFire(t *testing.T) {
	t.Parallel()
	domainDir := writeCoverageFixtureWithDiscipline(t, "mixedtypemethodmod",
		coverageFixtureImplSrc, coverageFixtureRealTestSrc, "full")

	r := reqWithLinks(
		"R-f1-mixed-citation", "sa",
		[]string{"spec/model/risk.go:Risk", "spec/model/risk.go:NewRisk"}, // type + method
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	)
	r.Status = ontology.StatusSETTLED

	g := coverageGraphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if g.Discipline != loader.DisciplineFull {
		t.Fatalf("test setup: expected Discipline=%q, got %q", loader.DisciplineFull, g.Discipline)
	}
	vs := runCheck(t, "check_scenario_executes_impl", g)
	if hasViolationFor(vs, "R-f1-mixed-citation") {
		t.Fatalf("F1: expected check_scenario_executes_impl to NOT fire for a mixed citation (type + method, "+
			"method covered) -- the legitimate case must stay green, got violations: %+v", vs)
	}
}

// TestCheckScenarioExecutesImpl_F1_TypeOnly_SoftDiscipline_NoOp proves F1 does
// NOT fire for a soft-discipline domain (discipline not "full"): a requirement
// citing only a type in a domain that has not opted into the one-way
// discipline:full promise has not made the claim F1 polices. This is the
// backward-compatibility guarantee -- existing soft-discipline domains must
// see zero new violations from F1.
func TestCheckScenarioExecutesImpl_F1_TypeOnly_SoftDiscipline_NoOp(t *testing.T) {
	t.Parallel()
	domainDir := writeCoverageFixtureWithDiscipline(t, "softdiscmod",
		coverageFixtureImplSrc, coverageFixtureForgedTestSrc, "") // no discipline

	r := reqWithLinks(
		"R-f1-soft-discipline", "sa",
		[]string{"spec/model/risk.go:Risk"}, // type-only
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	)
	r.Status = ontology.StatusSETTLED

	g := coverageGraphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if g.Discipline == loader.DisciplineFull {
		t.Fatalf("test setup: expected soft discipline, got %q", g.Discipline)
	}
	vs := runCheck(t, "check_scenario_executes_impl", g)
	if hasViolationFor(vs, "R-f1-soft-discipline") {
		t.Fatalf("F1: expected no violation for a type-only citation in a soft-discipline domain "+
			"(backward compatibility), got: %+v", vs)
	}
}
