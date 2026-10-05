package ontology

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateConformanceRejectsInvalidRenderSelectionAndMissingPrimary(t *testing.T) {
	g := &Graph{
		Languages:       []string{"en", "ru"},
		DefaultLanguage: "ru",
		RenderLanguage:  "fr",
		Requirements: []Requirement{
			{
				ID: "R-localized", Claim: "English only",
				ClaimTexts: LocalizedText{"en": "English only"},
			},
			{
				ID: "R-wrong-primary", Claim: "English only",
				ClaimTexts: LocalizedText{"en": "English only", "ru": "Другое русское требование"},
			},
		},
	}
	issues := ValidateConformance(g)
	var invalidSelection, missingPrimary, wrongPrimary bool
	for _, issue := range issues {
		invalidSelection = invalidSelection || (issue.ID == "render_language" && strings.Contains(issue.Message, "not declared"))
		missingPrimary = missingPrimary || (issue.ID == "R-localized" && strings.Contains(issue.Message, "ru"))
		wrongPrimary = wrongPrimary || (issue.ID == "R-wrong-primary" && strings.Contains(issue.Message, "does not exactly match"))
	}
	if !invalidSelection {
		t.Fatalf("invalid render language was not rejected: %+v", issues)
	}
	if !missingPrimary {
		t.Fatalf("missing default-language text was not rejected: %+v", issues)
	}
	if !wrongPrimary {
		t.Fatalf("Claim was not required to match the default language without fallback: %+v", issues)
	}
}
func TestValidateConformanceRejectsConflictingCaseAndScopedPrecedenceCycle(t *testing.T) {
	g := &Graph{
		Conformance: &ConformanceConfig{RuleCases: true},
		Requirements: []Requirement{
			{
				ID: "R-first", AtomKind: "rule",
				Cases: []CaseDefinition{{
					ID: "C-shared", Test: "spec/rules_test.go:TestRule", AtomIDs: []string{"R-first", "R-second"},
					Operation: "parse", Producer: "direct",
				}},
				Precedence: []PrecedenceLink{{Target: "R-second", Scope: "parse"}},
			},
			{
				ID: "R-second", AtomKind: "rule",
				Cases: []CaseDefinition{{
					ID: "C-shared", Test: "spec/rules_test.go:TestRule", AtomIDs: []string{"R-first", "R-second"},
					Operation: "parse", Producer: "adapter",
				}},
				Precedence: []PrecedenceLink{{Target: "R-first", Scope: "parse"}},
			},
		},
	}
	issues := ValidateConformance(g)
	var caseConflict, cycle bool
	for _, issue := range issues {
		caseConflict = caseConflict || (issue.ID == "C-shared" && strings.Contains(issue.Message, "conflicting metadata"))
		cycle = cycle || (issue.Kind == "precedence" && strings.Contains(issue.Message, "cycle"))
	}
	if !caseConflict || !cycle {
		t.Fatalf("expected case metadata collision and scoped precedence cycle, got %+v", issues)
	}
}

func TestValidateConformanceRejectsSelectionOutsideMatchedRules(t *testing.T) {
	graph := &Graph{
		Conformance: &ConformanceConfig{RuleCases: true},
		Requirements: []Requirement{{
			ID: "R-selection", AtomKind: "rule",
			Cases: []CaseDefinition{{
				ID: "C-selection", Test: "spec/rules_test.go:TestSelection", AtomIDs: []string{"R-selection"},
				Selection: &SelectionEvidence{Matched: []string{"R-selection"}, Selected: "R-other"},
			}},
		}},
	}
	issues := ValidateConformance(graph)
	for _, issue := range issues {
		if issue.Kind == "selection" {
			return
		}
	}
	t.Fatalf("case selection outside matched branches was not rejected: %+v", issues)
}

func TestObservedValueDistinguishesAbsentPayloadFromExplicitEmpty(t *testing.T) {
	var missingBytes ObservedValue
	if err := json.Unmarshal([]byte(`{"kind":"bytes","encoding":"base64"}`), &missingBytes); err != nil {
		t.Fatalf("unmarshal absent byte payload: %v", err)
	}
	if err := missingBytes.Validate(); err == nil || !strings.Contains(err.Error(), "present bytes field") {
		t.Fatalf("absent bytes payload was not rejected: %v", err)
	}

	var emptyBytes ObservedValue
	if err := json.Unmarshal([]byte(`{"kind":"bytes","encoding":"base64","bytes":""}`), &emptyBytes); err != nil {
		t.Fatalf("unmarshal explicit empty byte payload: %v", err)
	}
	if err := emptyBytes.Validate(); err != nil {
		t.Fatalf("explicit empty byte payload should be valid: %v", err)
	}
	encodedBytes, err := json.Marshal(emptyBytes)
	if err != nil {
		t.Fatalf("marshal explicit empty byte payload: %v", err)
	}
	var byteRoundTrip ObservedValue
	if err := json.Unmarshal(encodedBytes, &byteRoundTrip); err != nil {
		t.Fatalf("unmarshal marshaled empty bytes: %v", err)
	}
	if err := byteRoundTrip.Validate(); err != nil {
		t.Fatalf("empty byte payload was lost across JSON round-trip: %v", err)
	}

	var emptyObject ObservedValue
	if err := json.Unmarshal([]byte(`{"kind":"object","fields":{}}`), &emptyObject); err != nil {
		t.Fatalf("unmarshal empty object: %v", err)
	}
	if err := emptyObject.Validate(); err != nil {
		t.Fatalf("explicit empty object should be valid: %v", err)
	}
	encodedObject, err := json.Marshal(emptyObject)
	if err != nil {
		t.Fatalf("marshal explicit empty object: %v", err)
	}
	var objectRoundTrip ObservedValue
	if err := json.Unmarshal(encodedObject, &objectRoundTrip); err != nil {
		t.Fatalf("unmarshal marshaled empty object: %v", err)
	}
	if err := objectRoundTrip.Validate(); err != nil {
		t.Fatalf("empty object was lost across JSON round-trip: %v", err)
	}

	var missingBool ObservedValue
	if err := json.Unmarshal([]byte(`{"kind":"bool"}`), &missingBool); err != nil {
		t.Fatalf("unmarshal absent bool payload: %v", err)
	}
	if err := missingBool.Validate(); err == nil || !strings.Contains(err.Error(), "present bool field") {
		t.Fatalf("absent bool payload was not rejected: %v", err)
	}

	var falseValue ObservedValue
	if err := json.Unmarshal([]byte(`{"kind":"bool","bool":false,"scalar_kind":"bool"}`), &falseValue); err != nil {
		t.Fatalf("unmarshal explicit false bool: %v", err)
	}
	if err := falseValue.Validate(); err != nil {
		t.Fatalf("explicit false bool should be valid: %v", err)
	}
	encodedBool, err := json.Marshal(falseValue)
	if err != nil {
		t.Fatalf("marshal explicit false bool: %v", err)
	}
	var boolRoundTrip ObservedValue
	if err := json.Unmarshal(encodedBool, &boolRoundTrip); err != nil {
		t.Fatalf("unmarshal marshaled bool: %v", err)
	}
	if err := boolRoundTrip.Validate(); err != nil {
		t.Fatalf("false bool was lost across JSON round-trip: %v", err)
	}
	if boolRoundTrip.Bool == nil || *boolRoundTrip.Bool {
		t.Fatalf("round-trip bool = %v, want an explicit false", boolRoundTrip.Bool)
	}

	var nullText ObservedValue
	if err := json.Unmarshal([]byte(`{"kind":"text","text":null}`), &nullText); err == nil {
		t.Fatal("typed value decoder accepted a null text payload as absent")
	}
}

func TestValidateConformanceAllowsREJECTEDWithoutFullTranslations(t *testing.T) {
	g := &Graph{
		Languages:       []string{"ru", "en"},
		DefaultLanguage: "ru",
		Requirements: []Requirement{
			// Historical wording only — no claim_texts at all.
			{ID: "R-old", Claim: "Историческая формулировка", Status: StatusREJECTED},
			// Partial claim_texts with a language subset and no primary text:
			// the REJECTED wording is historical, so neither full coverage nor
			// the Claim match is required.
			{
				ID: "R-partial", Claim: "Русская историческая формулировка", Status: StatusREJECTED,
				ClaimTexts: LocalizedText{"ru": "Русская историческая формулировка"},
			},
			{
				ID: "R-mismatch", Claim: "Русская формулировка", Status: StatusREJECTED,
				ClaimTexts: LocalizedText{"en": "English historical wording"},
			},
		},
	}
	var issues = ValidateConformance(g)
	for _, issue := range issues {
		t.Errorf("unexpected issue for REJECTED history: %+v", issue)
	}
}

func TestValidateConformanceStillRequiresClaimTextsForActiveRequirements(t *testing.T) {
	g := &Graph{
		Languages:       []string{"ru", "en"},
		DefaultLanguage: "ru",
		Requirements: []Requirement{
			{ID: "R-active", Claim: "Active", Status: StatusDRAFT},
		},
	}
	found := false
	for _, issue := range ValidateConformance(g) {
		if issue.ID == "R-active" && strings.Contains(issue.Message, "claim_texts is required") {
			found = true
		}
	}
	if !found {
		t.Fatal("active requirement without claim_texts was not rejected in a multilingual domain")
	}
}

func TestValidateConformanceREJECTEDSurvivesDefaultLanguageSwitch(t *testing.T) {
	base := func(defaultLanguage string) *Graph {
		return &Graph{
			Languages:       []string{"ru", "en"},
			DefaultLanguage: defaultLanguage,
			Requirements: []Requirement{
				{ID: "R-old", Claim: "Историческая формулировка", Status: StatusREJECTED},
			},
		}
	}
	for _, dl := range []string{"ru", "en"} {
		if issues := ValidateConformance(base(dl)); len(issues) != 0 {
			t.Errorf("default_language %q: unexpected issues for REJECTED history: %+v", dl, issues)
		}
	}
}
