package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// --- check_settled_requires_scenario ----------------------------------------
//
// Fixture shapes for these tests deliberately mirror authored_links_test.go's
// existing writeAuthoredSpecFixture/authoredRiskModelSrc conventions (same
// domain layout: spec/model/risk.go + spec/model/risk_test.go) so the new
// check's tests read as a natural extension of that file rather than
// reinventing fixture plumbing.

// scenarioTestSrc is a verified_by test written against the hotamspec
// scenario recorder's public shape (mirrors
// internal/gate/spec_resolver_test.go's TestResolveSpecTest_ScenarioThenCountsAsTeeth
// fixture) -- HasScenario requires exactly a package-qualified
// `hotamspec.NewScenario(...)` call, so this fixture declares a same-named
// LOCAL package literally called `hotamspec` (AST-only detection cannot and
// does not resolve real import identity, only the literal selector text --
// see gate.SpecTestResult.HasScenario's own doc comment) with a NewScenario
// function that returns a value exposing a Then method, exactly reproducing
// the shape check_verified_by_test_has_teeth's own scenario fixture uses.
const scenarioRecorderStubSrc = `package hotamspec

type T interface {
	Helper()
	Errorf(format string, args ...any)
}

type Scenario struct{ t T }

func NewScenario(t T, reqID, title string) *Scenario {
	return &Scenario{t: t}
}

func (s *Scenario) Then(desc string, ok bool) {
	s.t.Helper()
	if !ok {
		s.t.Errorf("%s: %s failed", desc, desc)
	}
}
`

const authoredRiskTestScenarioSrc = `package model

import (
	"testing"

	"example.com/fixture/spec/hotamspec"
)

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-1", "reject missing owner")
	r, err := NewRisk("")
	s.Then("rejects a missing owner", err != nil && r == nil)
}
`

// writeScenarioDisciplineFixture writes both the model file and a
// verified_by test file (either the plain non-scenario fixture already
// shared by authored_links_test.go, or the scenario-narrated one above) plus
// a manifest.json carrying the given discipline value, and returns the
// domain directory.
func writeScenarioDisciplineFixture(t *testing.T, testSrc, discipline string) string {
	t.Helper()
	tmp := t.TempDir()
	writeInto := func(rel, content string) {
		full := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", rel, err)
		}
	}
	writeInto("spec/model/risk.go", authoredRiskModelSrc)
	writeInto("spec/model/risk_test.go", testSrc)
	if testSrc == authoredRiskTestScenarioSrc {
		writeInto("spec/hotamspec/hotamspec.go", scenarioRecorderStubSrc)
	}
	manifest := `{"discipline": "` + discipline + `"}`
	writeInto("manifest.json", manifest)
	writeInto("graph.json", `{"schema_version":3}`)
	return tmp
}

// graphForDiscipline builds an ontology.Graph the way loader.LoadGraph would
// populate it (DomainDir + Discipline resolved from the fixture's own
// manifest.json), so the check under test sees exactly the same shape a real
// `hotam all-violations` run would -- not a hand-built graph with the field
// poked in directly, which would not exercise loader.ResolveDiscipline at
// all.
func graphForDiscipline(t *testing.T, domainDir string, reqs []ontology.Requirement) *ontology.Graph {
	t.Helper()
	graphPath := filepath.Join(domainDir, "graph.json")
	return &ontology.Graph{
		DomainDir:    domainDir,
		Discipline:   loader.ResolveDiscipline(graphPath),
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: reqs,
	}
}

func settledReq(rid, owner string) ontology.Requirement {
	r := req(rid, owner)
	r.Status = ontology.StatusSETTLED
	return r
}

// TestCheckSettledRequiresScenario_NoOpWithoutDisciplineFull proves the
// central backward-compatibility guarantee: a domain with NO discipline
// field (or any value other than "full") sees zero violations from this
// check no matter how bare its requirements are -- exactly the pilot's
// prat/gpsm-sm shape (33 SETTLED, 0 models) that must NOT go red from this
// wave landing.
func TestCheckSettledRequiresScenario_NoOpWithoutDisciplineFull(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioDisciplineFixture(t, authoredRiskTestGoodSrc, "")
	bare := settledReq("R-1", "sa") // no enforced_by, no implemented_by, no verified_by at all
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{bare})
	if g.Discipline != "" {
		t.Fatalf("test setup: expected empty Discipline, got %q", g.Discipline)
	}
	if vs := runCheck(t, "check_settled_requires_scenario", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a non-discipline:full domain, got %v", vs)
	}
}

// TestCheckSettledRequiresScenario_NoOpForUnrecognizedDisciplineValue proves
// a typo'd/future discipline value is treated identically to absent --
// loader.ResolveDiscipline's own documented "unrecognized == absent" rule --
// so a malformed opt-in can never silently masquerade as a real one.
func TestCheckSettledRequiresScenario_NoOpForUnrecognizedDisciplineValue(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioDisciplineFixture(t, authoredRiskTestGoodSrc, "FULL") // wrong case
	bare := settledReq("R-1", "sa")
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{bare})
	if g.Discipline != "" {
		t.Fatalf("test setup: expected empty Discipline for unrecognized value, got %q", g.Discipline)
	}
	if vs := runCheck(t, "check_settled_requires_scenario", g); len(vs) != 0 {
		t.Fatalf("expected no violations for an unrecognized discipline value, got %v", vs)
	}
}

// TestCheckSettledRequiresScenario_FiresWithNeitherCarrier is the RED case:
// discipline:full, a SETTLED requirement with no enforced_by and no
// implemented_by/verified_by at all.
func TestCheckSettledRequiresScenario_FiresWithNeitherCarrier(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioDisciplineFixture(t, authoredRiskTestGoodSrc, "full")
	bare := settledReq("R-1", "sa")
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{bare})
	if g.Discipline != loader.DisciplineFull {
		t.Fatalf("test setup: expected DisciplineFull, got %q", g.Discipline)
	}
	vs := runCheck(t, "check_settled_requires_scenario", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected a violation for a SETTLED requirement with no carrier at all, got %v", vs)
	}
}

// TestCheckSettledRequiresScenario_GreenWithEnforcedBy is the engine-path
// GREEN case: discipline:full, a SETTLED requirement carrying only
// enforced_by (no implemented_by/verified_by at all) -- the self-hosting
// exemption from the doc comment.
func TestCheckSettledRequiresScenario_GreenWithEnforcedBy(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioDisciplineFixture(t, authoredRiskTestGoodSrc, "full")
	r := settledReq("R-1", "sa")
	r.Enforcement = ontology.EnforcementENFORCED
	r.EnforcedBy = []string{"check_enforced_names_invariant"}
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if vs := runCheck(t, "check_settled_requires_scenario", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a requirement carried by enforced_by (engine path), got %v", vs)
	}
}

// TestCheckSettledRequiresScenario_GreenWithAuthoredScenario is the
// authored-path GREEN case: discipline:full, implemented_by + verified_by
// both set, and the verified_by test's body genuinely calls
// hotamspec.NewScenario(...).
func TestCheckSettledRequiresScenario_GreenWithAuthoredScenario(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioDisciplineFixture(t, authoredRiskTestScenarioSrc, "full")
	r := reqWithLinks("R-1", "sa",
		[]string{"spec/model/risk.go:NewRisk"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Status = ontology.StatusSETTLED
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if vs := runCheck(t, "check_settled_requires_scenario", g); len(vs) != 0 {
		t.Fatalf("expected no violations for an authored+scenario-narrated requirement, got %v", vs)
	}
}

// TestCheckSettledRequiresScenario_FiresWithAuthoredButNoScenario is the RED
// case the task explicitly calls for: implemented_by + verified_by both set,
// but the verified_by test's body is a PLAIN test (no
// hotamspec.NewScenario(...) call) -- authored without a scenario must still
// fire under discipline:full.
func TestCheckSettledRequiresScenario_FiresWithAuthoredButNoScenario(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioDisciplineFixture(t, authoredRiskTestGoodSrc, "full")
	r := reqWithLinks("R-1", "sa",
		[]string{"spec/model/risk.go:NewRisk"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Status = ontology.StatusSETTLED
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	vs := runCheck(t, "check_settled_requires_scenario", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected a violation for authored-without-scenario under discipline:full, got %v", vs)
	}
}

// TestCheckSettledRequiresScenario_SkipsNonSettled proves the gate only
// applies to SETTLED requirements -- a DRAFT requirement with no carrier at
// all must not fire even under discipline:full (roadmap debt is legitimate
// before a requirement is SETTLED).
func TestCheckSettledRequiresScenario_SkipsNonSettled(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioDisciplineFixture(t, authoredRiskTestGoodSrc, "full")
	r := reqStatus("R-1", "sa", ontology.StatusDRAFT)
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if vs := runCheck(t, "check_settled_requires_scenario", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a non-SETTLED requirement, got %v", vs)
	}
}

// TestCheckSettledRequiresScenario_MUTATION_FlipDisciplineFullRoundTrip is
// the mutation probe the task's own verification step calls for: the SAME
// requirement (authored, no scenario) is clean under soft discipline, then
// goes red the moment the domain's own manifest.json is mutated to
// discipline:full, then clean again once a genuine scenario is added --
// proving the check is live (reads the real manifest.json each call), not a
// one-shot flag baked in at fixture-build time.
func TestCheckSettledRequiresScenario_MUTATION_FlipDisciplineFullRoundTrip(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioDisciplineFixture(t, authoredRiskTestGoodSrc, "")
	manifestPath := filepath.Join(domainDir, "manifest.json")
	testPath := filepath.Join(domainDir, "spec", "model", "risk_test.go")
	r := reqWithLinks("R-1", "sa",
		[]string{"spec/model/risk.go:NewRisk"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Status = ontology.StatusSETTLED

	// SOFT: authored-without-scenario, but discipline is not full -- clean.
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if vs := runCheck(t, "check_settled_requires_scenario", g); len(vs) != 0 {
		t.Fatalf("SOFT: expected no violations, got %v", vs)
	}

	// FLIP: manifest.json now declares discipline:full -- same requirement,
	// same test body (still no scenario) -- must go RED.
	if err := os.WriteFile(manifestPath, []byte(`{"discipline": "full"}`), 0o644); err != nil {
		t.Fatalf("WriteFile manifest flip: %v", err)
	}
	g2 := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if g2.Discipline != loader.DisciplineFull {
		t.Fatalf("test setup: expected DisciplineFull after flip, got %q", g2.Discipline)
	}
	vs := runCheck(t, "check_settled_requires_scenario", g2)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("FLIPPED: expected a violation once discipline:full is declared with no scenario, got %v", vs)
	}

	// FIXED: add a real scenario to the verified_by test -- back to clean.
	if err := os.WriteFile(testPath, []byte(authoredRiskTestScenarioSrc), 0o644); err != nil {
		t.Fatalf("WriteFile test fix: %v", err)
	}
	stubPath := filepath.Join(domainDir, "spec", "hotamspec", "hotamspec.go")
	if err := os.MkdirAll(filepath.Dir(stubPath), 0o755); err != nil {
		t.Fatalf("MkdirAll hotamspec stub dir: %v", err)
	}
	if err := os.WriteFile(stubPath, []byte(scenarioRecorderStubSrc), 0o644); err != nil {
		t.Fatalf("WriteFile hotamspec stub: %v", err)
	}
	g3 := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if vs := runCheck(t, "check_settled_requires_scenario", g3); len(vs) != 0 {
		t.Fatalf("FIXED: expected no violations after adding a real scenario, got %v", vs)
	}
}

// TestCheckSettledRequiresScenario_GreenWithInherentlyProse proves the THIRD
// branch (the INHERENTLY_PROSE exemption): a SETTLED requirement with empty
// enforced_by/implemented_by/verified_by in a discipline:full domain produces
// NO violation the moment its Enforceability field is honestly tagged
// INHERENTLY_PROSE -- the residual category no snapshot gate could
// mechanically check (PLAN-scenario-generated-spec.md §2 D4 + §5 risks).
func TestCheckSettledRequiresScenario_GreenWithInherentlyProse(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioDisciplineFixture(t, authoredRiskTestGoodSrc, "full")
	r := settledReq("R-inherently-prose", "sa") // no enforced_by, no implemented_by, no verified_by
	r.Enforceability = ontology.EnforceabilityINHERENTLY_PROSE
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if g.Discipline != loader.DisciplineFull {
		t.Fatalf("test setup: expected DisciplineFull, got %q", g.Discipline)
	}
	if vs := runCheck(t, "check_settled_requires_scenario", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a SETTLED+INHERENTLY_PROSE requirement with no carrier in a discipline:full domain, got %v", vs)
	}
}

// TestCheckSettledRequiresScenario_OrdinaryEnforceableStillFires proves the
// INHERENTLY_PROSE exemption does NOT overreach to ordinary requirements: the
// SAME shape (empty enforced_by/implemented_by/verified_by, same
// discipline:full domain) but Enforceability=ENFORCEABLE (the ordinary
// default) STILL produces a violation. The exemption triggers ONLY on the
// Enforceability field, never on the bare absence of links.
func TestCheckSettledRequiresScenario_OrdinaryEnforceableStillFires(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioDisciplineFixture(t, authoredRiskTestGoodSrc, "full")
	r := settledReq("R-ordinary", "sa") // same empty shape as above ...
	r.Enforceability = ontology.EnforceabilityENFORCEABLE // ... but the ordinary default
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if g.Discipline != loader.DisciplineFull {
		t.Fatalf("test setup: expected DisciplineFull, got %q", g.Discipline)
	}
	vs := runCheck(t, "check_settled_requires_scenario", g)
	if !hasViolationFor(vs, "R-ordinary") {
		t.Fatalf("expected a violation for a SETTLED+ENFORCEABLE requirement with no carrier (exemption must NOT cover the ordinary case), got %v", vs)
	}
}

// TestCheckSettledRequiresScenario_InherentlyProseWithCarrierIsGreenEitherWay
// proves the INHERENTLY_PROSE exemption and the engine-path exemption are
// independent: a SETTLED+INHERENTLY_PROSE requirement that ALSO carries an
// enforced_by is green -- it would have been green WITHOUT the INHERENTLY_PROSE
// tag too (engine path already exempts it), so the INHERENTLY_PROSE branch is
// dormant here. This guards against any future refactor accidentally making
// the two exemption branches mutually exclusive.
func TestCheckSettledRequiresScenario_InherentlyProseWithCarrierIsGreenEitherWay(t *testing.T) {
	t.Parallel()
	domainDir := writeScenarioDisciplineFixture(t, authoredRiskTestGoodSrc, "full")
	r := settledReq("R-prose-with-engine", "sa")
	r.Enforceability = ontology.EnforceabilityINHERENTLY_PROSE
	r.Enforcement = ontology.EnforcementENFORCED
	r.EnforcedBy = []string{"check_enforced_names_invariant"}
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if vs := runCheck(t, "check_settled_requires_scenario", g); len(vs) != 0 {
		t.Fatalf("expected no violations for an INHERENTLY_PROSE requirement that also carries enforced_by, got %v", vs)
	}
}

// TestResolveDiscipline_Absent proves loader.ResolveDiscipline's own
// backward-compatibility contract directly (not merely through the check
// above): a manifest.json with no discipline field at all resolves to "".
func TestResolveDiscipline_Absent(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	manifestPath := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"self_hosting": false}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	graphPath := filepath.Join(tmp, "graph.json")
	if got := loader.ResolveDiscipline(graphPath); got != "" {
		t.Fatalf("expected empty discipline for a manifest with no discipline field, got %q", got)
	}
}

// TestResolveDiscipline_Full proves the recognized opt-in value round-trips.
func TestResolveDiscipline_Full(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	manifestPath := filepath.Join(tmp, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"discipline": "full"}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	graphPath := filepath.Join(tmp, "graph.json")
	if got := loader.ResolveDiscipline(graphPath); got != loader.DisciplineFull {
		t.Fatalf("expected DisciplineFull, got %q", got)
	}
}

// TestResolveDiscipline_MissingManifestIsHonestNoOp proves a graphPath with
// no sibling manifest.json at all (the domainDir-empty/synthetic-fixture
// case every other Resolve* function already tolerates) resolves to "".
func TestResolveDiscipline_MissingManifestIsHonestNoOp(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	graphPath := filepath.Join(tmp, "graph.json") // no manifest.json ever written
	if got := loader.ResolveDiscipline(graphPath); got != "" {
		t.Fatalf("expected empty discipline when manifest.json is absent, got %q", got)
	}
}
