package gate

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestNormativeExamplesRetainFailureAfterPassingCaseIdentity(t *testing.T) {
	requirement := ontology.Requirement{ID: "R-count", Claim: "Count preserves its input.", AtomKind: "rule"}
	input := &ontology.ObservedValue{Kind: "integer", Integer: "7"}
	expected := &ontology.ObservedValue{Kind: "integer", Integer: "7"}
	actual := &ontology.ObservedValue{Kind: "integer", Integer: "9"}
	caseDef := &ontology.CaseDefinition{ID: "same-case"}
	row := SpecRow{req: requirement, outcomes: []specTestOutcome{{
		artifacts:       []specArtifact{{Verdict: "pass", Case: caseDef, Observations: []specObservation{{Name: "count", Passed: true, RawInput: input, RawExpected: expected, RawActual: expected}}}},
		failedArtifacts: []specArtifact{{Verdict: "fail", Case: caseDef, Observations: []specObservation{{Name: "count", Passed: false, RawInput: input, RawExpected: expected, RawActual: actual}}}},
	}}}
	lines, err := renderNormativeRequirement(&ontology.Graph{}, row, "")
	if err != nil {
		t.Fatal(err)
	}
	if document := strings.Join(lines, "\n"); !strings.Contains(document, ReadableObservedValue(actual)) {
		t.Fatalf("a passing execution hid a later discrepancy: %s", document)
	}
}

func TestNormativeBodyPreservesGrammarAndParagraphsAcrossLocales(t *testing.T) {
	root := writeAtomPipelineFixture(t, map[string]string{
		"manifest.json": `{"self_hosting":false}`,
		"spec/model/rule.go": `package model
type Box struct{}
// >>>>> lang=en
// Integer grammar
//
// A sign MUST precede the digits.
//
//     integer = ["+" | "-"], digit, {digit};
// >>>>> lang=ru
// Грамматика целого
//
// Знак MUST предшествовать цифрам.
//
//     integer = ["+" | "-"], digit, {digit};
// >>>>> lang=zh
// 整数语法
//
// 符号 MUST 位于数字之前。
//
//     integer = ["+" | "-"], digit, {digit};
func (Box) Rule() int { return 7 }
`,
	})
	graph := &ontology.Graph{DomainDir: root, Languages: []string{"en", "ru", "zh"}, DefaultLanguage: "en", Conformance: &ontology.ConformanceConfig{RuleCases: true}}
	index, err := NewAtomSourceIndexForGraph(graph)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := index.DeriveClaims(AtomArtifact{ReqID: "R-grammar", Mode: "rule", Verdict: "pass", Case: &AtomCaseContext{ID: "signed"}, Steps: []AtomStep{{Subject: "example.test/pipeline/model.Box.Rule", Value: "7"}}})
	if err != nil {
		t.Fatal(err)
	}
	source, err := index.Resolve("example.test/pipeline/model.Box.Rule")
	if err != nil {
		t.Fatal(err)
	}
	requirement := ontology.Requirement{ID: "R-grammar", Claim: claims["en"], ClaimTexts: claims, AtomKind: "rule"}
	for _, language := range graph.Languages {
		lines, err := renderNormativeRequirement(graph, SpecRow{req: requirement, normativeTexts: source.Phrases}, language)
		if err != nil {
			t.Fatal(err)
		}
		document := strings.Join(lines, "\n")
		if !strings.Contains(document, "\n\n    integer = [\"+\" | \"-\"], digit, {digit};\n") {
			t.Fatalf("%s grammar lost its block structure or delimiters: %s", language, document)
		}
	}
}
