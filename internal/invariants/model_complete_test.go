package invariants

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// --- check_model_complete ---------------------------------------------------
//
// Fixture shapes deliberately mirror scenario_discipline_test.go's own
// writeScenarioDisciplineFixture / authoredRiskModelSrc /
// authoredRiskTestScenarioSrc conventions (same domain layout:
// spec/model/risk.go + spec/model/risk_test.go + spec/hotamspec/hotamspec.go
// recorder stub when a scenario test is in play), so the W2.4 check's tests
// read as a natural extension of W2.1's -- reusing its scenarioRecorderStubSrc
// and authoredRiskModelSrc verbatim. The one new fixture surface is
// twoMethodModelSrc + its two-test companion, for the partial-model D5 case
// (one cited method scenario-covered, another not) that is the check's
// reason for existing over W2.1's per-requirement framing.

// writeModelCompleteFixture writes a model file, a test file, and (when the
// test file calls hotamspec.NewScenario) the local hotamspec recorder stub
// gate.SpecTestResult.HasScenario detects, plus a manifest carrying the
// given discipline value. Returns the domain directory. Mirrors
// writeScenarioDisciplineFixture exactly except the model + test source are
// parameters (this check needs a 2-method fixture for its partial-model
// case, which the shared helper's hardcoded authoredRiskModelSrc cannot
// express).
func writeModelCompleteFixture(t *testing.T, modelSrc, testSrc, discipline string) string {
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
	writeInto("spec/model/risk.go", modelSrc)
	writeInto("spec/model/risk_test.go", testSrc)
	if strings.Contains(testSrc, "hotamspec.NewScenario") {
		writeInto("spec/hotamspec/hotamspec.go", scenarioRecorderStubSrc)
	}
	writeInto("manifest.json", `{"discipline": "`+discipline+`"}`)
	writeInto("graph.json", `{"schema_version":3}`)
	return tmp
}

// twoMethodModelSrc is a model object (Risk) with TWO exported methods
// (Validate, Score) -- the minimal surface needed to express D5's
// partial-model case (one cited method scenario-covered, the other not) in
// a single domain fixture. authoredRiskModelSrc carries only one exported
// method (Validate) plus a top-level constructor function (NewRisk), which
// cannot express "one method covered, another dangling".
const twoMethodModelSrc = `package model

type Risk struct {
	Owner string
}

func (r *Risk) Validate() error { return nil }
func (r *Risk) Score() int      { return 0 }
`

// twoMethodScenarioTestSrc carries a scenario-narrated test for Validate
// and a PLAIN (non-scenario) test for Score in the SAME file, so the
// partial-model fixture can cite Validate under a scenario test and Score
// under a plain test simultaneously.
const twoMethodScenarioTestSrc = `package model

import (
	"testing"

	"example.com/fixture/spec/hotamspec"
)

func TestValidate_Scenario(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-validate", "validate is a real scenario")
	r := &Risk{}
	s.Then("validate runs", r.Validate() == nil)
}

func TestScore_Plain(t *testing.T) {
	r := &Risk{}
	if r.Score() != 0 {
		t.Fatalf("score not zero")
	}
}
`

// twoMethodAllScenarioTestSrc is the GREEN companion to
// twoMethodScenarioTestSrc: BOTH Validate and Score exercised under their
// own hotamspec scenario, so a model whose every cited method is covered
// fires no violation.
const twoMethodAllScenarioTestSrc = `package model

import (
	"testing"

	"example.com/fixture/spec/hotamspec"
)

func TestValidate_Scenario(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-validate", "validate is a real scenario")
	r := &Risk{}
	s.Then("validate runs", r.Validate() == nil)
}

func TestScore_Scenario(t *testing.T) {
	s := hotamspec.NewScenario(t, "R-score", "score is a real scenario")
	r := &Risk{}
	s.Then("score is zero", r.Score() == 0)
}
`

// TestCheckModelComplete_NoOpWithoutDisciplineFull is the central
// backward-compatibility guarantee: a domain with NO discipline field (or
// any value other than "full") sees zero violations from this check no
// matter how partial its models' cited methods are -- exactly the
// prat/gpsm-sm shape that must NOT go red from this wave landing.
func TestCheckModelComplete_NoOpWithoutDisciplineFull(t *testing.T) {
	t.Parallel()
	domainDir := writeModelCompleteFixture(t, authoredRiskModelSrc, authoredRiskTestGoodSrc, "")
	r := reqWithLinks("R-1", "sa",
		[]string{"spec/model/risk.go:Risk.Validate"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Status = ontology.StatusSETTLED
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if g.Discipline != "" {
		t.Fatalf("test setup: expected empty Discipline, got %q", g.Discipline)
	}
	if vs := runCheck(t, "check_model_complete", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a non-discipline:full domain, got %v", vs)
	}
}

// TestCheckModelComplete_NoOpForUnrecognizedDisciplineValue proves a
// typo'd/future discipline value is treated identically to absent --
// loader.ResolveDiscipline's own "unrecognized == absent" rule.
func TestCheckModelComplete_NoOpForUnrecognizedDisciplineValue(t *testing.T) {
	t.Parallel()
	domainDir := writeModelCompleteFixture(t, authoredRiskModelSrc, authoredRiskTestGoodSrc, "FULL") // wrong case
	r := reqWithLinks("R-1", "sa",
		[]string{"spec/model/risk.go:Risk.Validate"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Status = ontology.StatusSETTLED
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if g.Discipline != "" {
		t.Fatalf("test setup: expected empty Discipline for unrecognized value, got %q", g.Discipline)
	}
	if vs := runCheck(t, "check_model_complete", g); len(vs) != 0 {
		t.Fatalf("expected no violations for an unrecognized discipline value, got %v", vs)
	}
}

// TestCheckModelComplete_FiresForCitedMethodWithoutScenario is the RED case:
// discipline:full, a SETTLED requirement citing Risk.Validate as
// implemented_by, with a PLAIN (non-scenario) verified_by test -- the
// method is cited but no requirement narrates a scenario for it, so the
// model Risk is incomplete and the violation names both the object and the
// uncovered method.
func TestCheckModelComplete_FiresForCitedMethodWithoutScenario(t *testing.T) {
	t.Parallel()
	domainDir := writeModelCompleteFixture(t, authoredRiskModelSrc, authoredRiskTestGoodSrc, "full")
	r := reqWithLinks("R-1", "sa",
		[]string{"spec/model/risk.go:Risk.Validate"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Status = ontology.StatusSETTLED
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if g.Discipline != loader.DisciplineFull {
		t.Fatalf("test setup: expected DisciplineFull, got %q", g.Discipline)
	}
	vs := runCheck(t, "check_model_complete", g)
	if !hasViolationFor(vs, "Risk") {
		t.Fatalf("expected a violation naming model Risk (cited method Validate has no scenario), got %v", vs)
	}
	// The message must name BOTH the object and the uncovered method, per
	// D5's actionable-diagnostic mandate.
	var v Violation
	for _, candidate := range vs {
		if candidate.ID == "Risk" {
			v = candidate
			break
		}
	}
	if !strings.Contains(v.Message, "Risk") || !strings.Contains(v.Message, "Validate") {
		t.Fatalf("violation message must name object Risk and uncovered method Validate, got: %s", v.Message)
	}
}

// TestCheckModelComplete_GreenForCitedMethodWithScenario is the GREEN case:
// same citation, but the verified_by test's body genuinely calls
// hotamspec.NewScenario(...) -- the method is scenario-complete, the model
// is complete, no violation.
func TestCheckModelComplete_GreenForCitedMethodWithScenario(t *testing.T) {
	t.Parallel()
	domainDir := writeModelCompleteFixture(t, authoredRiskModelSrc, authoredRiskTestScenarioSrc, "full")
	r := reqWithLinks("R-1", "sa",
		[]string{"spec/model/risk.go:Risk.Validate"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Status = ontology.StatusSETTLED
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if vs := runCheck(t, "check_model_complete", g); len(vs) != 0 {
		t.Fatalf("expected no violations when the cited method is under a scenario test, got %v", vs)
	}
}

// TestCheckModelComplete_NoOpForFunctionCitation proves a citation that
// resolves to a TOP-LEVEL FUNCTION (NewRisk) rather than a method of a
// scanned model object is out of scope for this MODEL-level check -- a
// function is not an object's method, so it cannot make a model
// "incomplete". This is the boundary that keeps the check focused on
// D5's model-completeness question, not on every implemented_by carrier.
func TestCheckModelComplete_NoOpForFunctionCitation(t *testing.T) {
	t.Parallel()
	domainDir := writeModelCompleteFixture(t, authoredRiskModelSrc, authoredRiskTestGoodSrc, "full")
	r := reqWithLinks("R-1", "sa",
		[]string{"spec/model/risk.go:NewRisk"}, // top-level function, not a method
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Status = ontology.StatusSETTLED
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if vs := runCheck(t, "check_model_complete", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a function (non-method) citation, got %v", vs)
	}
}

// TestCheckModelComplete_NoOpForModelWithNoCitedMethod proves a model none
// of whose exported methods is cited as implemented_by is simply out of
// scope -- a SETTLED requirement with no implemented_by at all cannot make
// any model incomplete (there is no discipline-binding citation). Its
// authored-path obligation is W2.1's to report, not this model-level
// check's.
func TestCheckModelComplete_NoOpForModelWithNoCitedMethod(t *testing.T) {
	t.Parallel()
	domainDir := writeModelCompleteFixture(t, authoredRiskModelSrc, authoredRiskTestGoodSrc, "full")
	bare := settledReq("R-1", "sa") // no implemented_by, no verified_by
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{bare})
	if vs := runCheck(t, "check_model_complete", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a model with no cited method, got %v", vs)
	}
}

// TestCheckModelComplete_SkipsNonSettled proves the gate only applies to
// SETTLED requirements' citations -- a DRAFT requirement citing a method
// cannot make a model incomplete before it has settled (legitimate roadmap
// exploration).
func TestCheckModelComplete_SkipsNonSettled(t *testing.T) {
	t.Parallel()
	domainDir := writeModelCompleteFixture(t, authoredRiskModelSrc, authoredRiskTestGoodSrc, "full")
	r := reqWithLinks("R-1", "sa",
		[]string{"spec/model/risk.go:Risk.Validate"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Status = ontology.StatusDRAFT
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if vs := runCheck(t, "check_model_complete", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a DRAFT requirement's method citation, got %v", vs)
	}
}

// TestCheckModelComplete_BareMethodNameMatches proves a BARE implemented_by
// symbol ("Validate", no "Risk." qualifier) still matches the method on
// the scanned Risk object -- gate.ResolveSpecSymbol's documented "any
// receiver" semantics for bare names, which matchCitedExportedMethod
// mirrors so a citation authored in the common bare form is not silently
// dropped.
func TestCheckModelComplete_BareMethodNameMatches(t *testing.T) {
	t.Parallel()
	domainDir := writeModelCompleteFixture(t, authoredRiskModelSrc, authoredRiskTestGoodSrc, "full")
	r := reqWithLinks("R-1", "sa",
		[]string{"spec/model/risk.go:Validate"}, // bare form, no Type. qualifier
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Status = ontology.StatusSETTLED
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	vs := runCheck(t, "check_model_complete", g)
	if !hasViolationFor(vs, "Risk") {
		t.Fatalf("expected a violation for bare-form citation of Validate, got %v", vs)
	}
}

// TestCheckModelComplete_PartialModelIsTheD5Case is the precise scenario
// D5 exists to surface and W2.1's per-requirement framing cannot: ONE
// model object (Risk) with TWO exported methods (Validate, Score), each
// cited by a DIFFERENT SETTLED requirement -- R-validate carries a
// scenario-narrated verified_by test (Validate scenario-complete), R-score
// carries only a PLAIN test (Score NOT scenario-complete). The model is
// INCOMPLETE; the violation names Risk and EXACTLY the uncovered method
// Score (Validate must NOT be listed -- it IS covered).
func TestCheckModelComplete_PartialModelIsTheD5Case(t *testing.T) {
	t.Parallel()
	domainDir := writeModelCompleteFixture(t, twoMethodModelSrc, twoMethodScenarioTestSrc, "full")

	rValidate := reqWithLinks("R-validate", "sa",
		[]string{"spec/model/risk.go:Risk.Validate"},
		[]string{"spec/model/risk_test.go:TestValidate_Scenario"})
	rValidate.Status = ontology.StatusSETTLED

	rScore := reqWithLinks("R-score", "sa",
		[]string{"spec/model/risk.go:Risk.Score"},
		[]string{"spec/model/risk_test.go:TestScore_Plain"})
	rScore.Status = ontology.StatusSETTLED

	g := graphForDiscipline(t, domainDir, []ontology.Requirement{rValidate, rScore})
	vs := runCheck(t, "check_model_complete", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly one violation (for the single incomplete model Risk), got %d: %v", len(vs), vs)
	}
	v := vs[0]
	if v.ID != "Risk" {
		t.Fatalf("violation ID must be the model object name Risk, got %q (msg: %s)", v.ID, v.Message)
	}
	if !strings.Contains(v.Message, "Score") {
		t.Fatalf("violation must name the uncovered method Score, got: %s", v.Message)
	}
	if strings.Contains(v.Message, "Validate (") {
		// Validate IS scenario-covered (R-validate carries a scenario test)
		// and must NOT appear in the uncovered list. Guard against the
		// substring "Validate (" (the diagnostic's per-method format) so a
		// legitimate mention of Validate elsewhere (e.g. in prose) is not a
		// false failure -- but the diagnostic format literally renders
		// uncovered methods as "Validate (cited by ...", so its presence
		// means Validate was wrongly flagged uncovered.
		t.Fatalf("violation must NOT list Validate as uncovered (it IS scenario-covered), got: %s", v.Message)
	}
	if !strings.Contains(v.Message, "R-score") {
		t.Fatalf("violation must name the citing requirement R-score, got: %s", v.Message)
	}
}

// TestCheckModelComplete_FullModelIsGreen proves the partial-model fixture
// goes GREEN the moment the second method also gains a scenario -- i.e.
// the D5 red is genuinely about the missing scenario, not about the mere
// presence of two methods on one object.
func TestCheckModelComplete_FullModelIsGreen(t *testing.T) {
	t.Parallel()
	domainDir := writeModelCompleteFixture(t, twoMethodModelSrc, twoMethodAllScenarioTestSrc, "full")

	rValidate := reqWithLinks("R-validate", "sa",
		[]string{"spec/model/risk.go:Risk.Validate"},
		[]string{"spec/model/risk_test.go:TestValidate_Scenario"})
	rValidate.Status = ontology.StatusSETTLED

	rScore := reqWithLinks("R-score", "sa",
		[]string{"spec/model/risk.go:Risk.Score"},
		[]string{"spec/model/risk_test.go:TestScore_Scenario"})
	rScore.Status = ontology.StatusSETTLED

	g := graphForDiscipline(t, domainDir, []ontology.Requirement{rValidate, rScore})
	if vs := runCheck(t, "check_model_complete", g); len(vs) != 0 {
		t.Fatalf("expected no violations when every cited method has a scenario, got %v", vs)
	}
}

// TestCheckModelComplete_CrossRequirementOR proves the OR semantics: a
// method cited by TWO requirements is scenario-complete if AT LEAST ONE of
// them carries a scenario-narrated verified_by test, even if the other
// cites only a plain test. This is the honest "any citing requirement
// narrates" attribution -- the same requirement-level signal W2.1 uses,
// grouped to the method.
func TestCheckModelComplete_CrossRequirementOR(t *testing.T) {
	t.Parallel()
	domainDir := writeModelCompleteFixture(t, twoMethodModelSrc, twoMethodScenarioTestSrc, "full")

	// R-with-scenario cites Validate AND has a scenario test.
	rWith := reqWithLinks("R-with-scenario", "sa",
		[]string{"spec/model/risk.go:Risk.Validate"},
		[]string{"spec/model/risk_test.go:TestValidate_Scenario"})
	rWith.Status = ontology.StatusSETTLED

	// R-plain-only ALSO cites Validate but carries only a plain test.
	// Validate is still scenario-complete via R-with-scenario.
	rPlain := reqWithLinks("R-plain-only", "sa",
		[]string{"spec/model/risk.go:Risk.Validate"},
		[]string{"spec/model/risk_test.go:TestScore_Plain"})
	rPlain.Status = ontology.StatusSETTLED

	g := graphForDiscipline(t, domainDir, []ontology.Requirement{rWith, rPlain})
	if vs := runCheck(t, "check_model_complete", g); len(vs) != 0 {
		t.Fatalf("expected no violations: Validate is scenario-complete via R-with-scenario even though R-plain-only has only a plain test, got %v", vs)
	}
}

// TestCheckModelComplete_MUTATION_FlipDisciplineRoundTrip is the mutation
// probe the task's own verification step calls for (W2.1's established
// soft -> discipline:full -> red -> add-scenario -> green shape): the SAME
// partial citation is clean under soft discipline, goes red the moment the
// domain's own manifest.json is mutated to discipline:full, and goes green
// again once a genuine scenario is added -- proving the check reacts to
// BOTH inputs (discipline opt-in AND scenario presence), not just one.
func TestCheckModelComplete_MUTATION_FlipDisciplineRoundTrip(t *testing.T) {
	t.Parallel()
	// Stage 1: soft discipline -- partial model, plain test only, clean.
	domainDir := writeModelCompleteFixture(t, authoredRiskModelSrc, authoredRiskTestGoodSrc, "")
	r := reqWithLinks("R-1", "sa",
		[]string{"spec/model/risk.go:Risk.Validate"},
		[]string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"})
	r.Status = ontology.StatusSETTLED
	g := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if g.Discipline != "" {
		t.Fatalf("stage 1 setup: expected empty Discipline, got %q", g.Discipline)
	}
	if vs := runCheck(t, "check_model_complete", g); len(vs) != 0 {
		t.Fatalf("stage 1 (soft): expected no violations, got %v", vs)
	}

	// Stage 2: mutate the SAME domain's manifest to discipline:full -- red.
	manifestPath := filepath.Join(domainDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"discipline": "full"}`), 0o644); err != nil {
		t.Fatalf("stage 2 mutate manifest: %v", err)
	}
	g2 := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if g2.Discipline != loader.DisciplineFull {
		t.Fatalf("stage 2 setup: expected DisciplineFull after flip, got %q", g2.Discipline)
	}
	vs2 := runCheck(t, "check_model_complete", g2)
	if !hasViolationFor(vs2, "Risk") {
		t.Fatalf("stage 2 (discipline:full, plain test only): expected a violation naming Risk, got %v", vs2)
	}

	// Stage 3: add a genuine scenario test (rewrite risk_test.go + drop the
	// recorder stub) -- green.
	testPath := filepath.Join(domainDir, filepath.FromSlash("spec/model/risk_test.go"))
	if err := os.WriteFile(testPath, []byte(authoredRiskTestScenarioSrc), 0o644); err != nil {
		t.Fatalf("stage 3 rewrite test: %v", err)
	}
	stubPath := filepath.Join(domainDir, filepath.FromSlash("spec/hotamspec/hotamspec.go"))
	if err := os.MkdirAll(filepath.Dir(stubPath), 0o755); err != nil {
		t.Fatalf("stage 3 mkdir hotamspec: %v", err)
	}
	if err := os.WriteFile(stubPath, []byte(scenarioRecorderStubSrc), 0o644); err != nil {
		t.Fatalf("stage 3 write recorder stub: %v", err)
	}
	g3 := graphForDiscipline(t, domainDir, []ontology.Requirement{r})
	if vs3 := runCheck(t, "check_model_complete", g3); len(vs3) != 0 {
		t.Fatalf("stage 3 (discipline:full + scenario): expected no violations, got %v", vs3)
	}
}
