package gate

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestBuildSpecDocumentsFromRowsRendersLanguageIndexesAndPackageShards(t *testing.T) {
	requirement := ontology.Requirement{
		ID: "R-localized-rule", Claim: "Русская норма.",
		ClaimTexts: ontology.LocalizedText{
			"ru": "Русская норма.",
			"en": "The English rule.",
		},
		ImplementedBy: []string{"spec/model/rule.go:Rule"},
		DeclOrder:     1,
	}
	graph := &ontology.Graph{
		Languages: []string{"ru", "en"}, DefaultLanguage: "ru",
		SelfExecutingAtoms: true, Requirements: []ontology.Requirement{requirement},
	}
	documents, err := BuildSpecDocumentsFromRows(graph, map[string]SpecRow{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"SPEC.ru.md", "SPEC.en.md",
		"spec/ru/model.md", "spec/en/model.md",
	} {
		if _, ok := documents[path]; !ok {
			t.Errorf("localized bundle missing %q; got %v", path, documents)
		}
	}
	if !strings.Contains(documents["spec/ru/model.md"], "Русская норма.") {
		t.Errorf("Russian shard did not use its exact ClaimTexts entry:\n%s", documents["spec/ru/model.md"])
	}
	if !strings.Contains(documents["spec/en/model.md"], "The English rule.") {
		t.Errorf("English shard did not use its exact ClaimTexts entry:\n%s", documents["spec/en/model.md"])
	}
	if !strings.Contains(documents["SPEC.ru.md"], "spec/ru/model.md") || strings.Contains(documents["SPEC.ru.md"], "spec/en/model.md") {
		t.Errorf("Russian index links do not remain in the Russian bundle:\n%s", documents["SPEC.ru.md"])
	}
}

func TestBuildSpecDocumentsDoesNotLeakPrimaryAtomTitleAcrossLocales(t *testing.T) {
	requirement := ontology.Requirement{
		ID: "R-localized-rule", Claim: "Русская норма.",
		ClaimTexts:    ontology.LocalizedText{"ru": "Русская норма.", "en": "The English rule."},
		VerifiedBy:    []string{"spec/model/rule_test.go:TestRule"},
		ImplementedBy: []string{"spec/model/rule.go:Rule"},
	}
	graph := &ontology.Graph{
		Languages: []string{"ru", "en"}, DefaultLanguage: "ru",
		SelfExecutingAtoms: true, Conformance: &ontology.ConformanceConfig{RuleCases: true},
		Requirements: []ontology.Requirement{requirement},
	}
	rows := map[string]SpecRow{
		requirement.ID: SpecRow{
			req: requirement,
			outcomes: []specTestOutcome{{
				entry: requirement.VerifiedBy[0], passed: true,
				artifacts: []specArtifact{{
					ReqID: requirement.ID, Test: "TestRule", Title: requirement.Claim,
					Verdict: "pass", Mode: "rule",
					Steps: []specArtifactStep{{Subject: "Box.Rule", Value: "11"}},
				}},
			}},
		},
	}
	documents, err := BuildSpecDocumentsFromRows(graph, rows)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(documents["spec/en/model.md"], "The English rule.") ||
		strings.Contains(documents["spec/en/model.md"], "**Русская норма.**") {
		t.Errorf("English rule view leaked the primary-language artifact title:\n%s", documents["spec/en/model.md"])
	}
	if !strings.Contains(documents["spec/ru/model.md"], "Русская норма.") ||
		strings.Contains(documents["spec/ru/model.md"], "**The English rule.**") {
		t.Errorf("Russian rule view leaked an English artifact title:\n%s", documents["spec/ru/model.md"])
	}
}

func TestBuildSpecRendersExplicitCaseTestWithoutVerifiedByAsCaseEvidence(t *testing.T) {
	caseDef := ontology.CaseDefinition{
		ID: "case-only", Test: "spec/model/rule_test.go:TestRule",
		AtomIDs: []string{"R-case-only"}, Operation: "decode", Producer: "adapter",
		Expected: &ontology.ObservedValue{Kind: "integer", Integer: "1"},
	}
	requirement := ontology.Requirement{
		ID: "R-case-only", Claim: "Русская норма.", AtomKind: "rule",
		ClaimTexts:    ontology.LocalizedText{"ru": "Русская норма.", "en": "The English rule."},
		Cases:         []ontology.CaseDefinition{caseDef},
		ImplementedBy: []string{"spec/model/rule.go:Rule"},
	}
	graph := &ontology.Graph{
		Languages: []string{"ru", "en"}, DefaultLanguage: "ru",
		Conformance:  &ontology.ConformanceConfig{RuleCases: true},
		Requirements: []ontology.Requirement{requirement},
	}
	rows := map[string]SpecRow{
		requirement.ID: SpecRow{
			req: requirement,
			outcomes: []specTestOutcome{{
				entry: caseDef.Test, passed: true,
				artifacts: []specArtifact{{
					ReqID: requirement.ID, Test: "TestRule", Verdict: "pass", Mode: "rule",
					Case:  &caseDef,
					Steps: []specArtifactStep{{Subject: "Box.Rule", Value: "1"}},
				}},
			}},
		},
	}
	documents, err := BuildSpecDocumentsFromRows(graph, rows)
	if err != nil {
		t.Fatal(err)
	}
	document := documents["spec/en/model.md"]
	if !strings.Contains(document, "The English rule.") || strings.Contains(document, "```json") {
		t.Fatalf("declared case evidence did not produce a readable localized norm:\n%s", document)
	}
}

func TestBuildSpecDocumentsRefusesMissingClaimTranslationAsWholeBundle(t *testing.T) {
	graph := &ontology.Graph{
		Languages: []string{"ru", "en"}, DefaultLanguage: "ru",
		Requirements: []ontology.Requirement{{
			ID: "R-missing-english", Claim: "Русская норма.",
			ClaimTexts: ontology.LocalizedText{"ru": "Русская норма."},
		}},
	}
	documents, err := BuildSpecDocumentsFromRows(graph, map[string]SpecRow{})
	if err == nil {
		t.Fatalf("missing English ClaimTexts silently fell back, documents=%v", documents)
	}
	if documents != nil {
		t.Fatalf("translation failure returned a partial bundle: %v", documents)
	}
}

func TestBuildSpecDocumentsRetainsLegacySingleDocumentLayout(t *testing.T) {
	graph := &ontology.Graph{Requirements: []ontology.Requirement{{ID: "R-legacy", Claim: "A plain legacy claim."}}}
	documents, err := BuildSpecDocumentsFromRows(graph, map[string]SpecRow{})
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 1 || documents["SPEC.md"] == "" {
		t.Fatalf("legacy document layout = %v, want only SPEC.md", documents)
	}
}

func TestBuildSpecDocumentsExplicitSingleLanguageKeepsLegacyAtomShardPath(t *testing.T) {
	requirement := ontology.Requirement{
		ID: "R-single-locale", Claim: "Русская норма.",
		ClaimTexts:    ontology.LocalizedText{"ru": "Русская норма."},
		ImplementedBy: []string{"spec/model/rule.go:Rule"},
	}
	graph := &ontology.Graph{
		Languages: []string{"ru"}, DefaultLanguage: "ru",
		SelfExecutingAtoms: true, Requirements: []ontology.Requirement{requirement},
	}
	documents, err := BuildSpecDocumentsFromRows(graph, map[string]SpecRow{})
	if err != nil {
		t.Fatal(err)
	}
	if documents["SPEC.md"] == "" || documents["spec/model.md"] == "" {
		t.Fatalf("single-language atom layout = %v, want legacy unsuffixed index and shard", documents)
	}
	if _, exists := documents["spec/spec/model.md"]; exists {
		t.Fatalf("spec/ package kept the doubled shard path: %v", documents)
	}
	if _, exists := documents["SPEC.ru.md"]; exists {
		t.Fatalf("single language acquired a suffixed SPEC index: %v", documents)
	}
}

func TestBuildSpecDocumentsRejectsMissingServiceCatalogBeforeBundleReturn(t *testing.T) {
	graph := &ontology.Graph{
		Languages: []string{"xx"}, DefaultLanguage: "xx",
		Requirements: []ontology.Requirement{{ID: "R-unsupported-locale", Claim: "An authored claim."}},
	}
	documents, err := BuildSpecDocumentsFromRows(graph, map[string]SpecRow{})
	if err == nil {
		t.Fatalf("unsupported service locale returned documents: %v", documents)
	}
	if documents != nil {
		t.Fatalf("catalog error returned a partial bundle: %v", documents)
	}
}

func TestBuildSpecDocumentsRejectsShardPathCollisions(t *testing.T) {
	graph := &ontology.Graph{
		Languages: []string{"ru", "en"}, DefaultLanguage: "ru",
		SelfExecutingAtoms: true,
		Requirements: []ontology.Requirement{
			{ID: "R-plain", Claim: "A.", ImplementedBy: []string{"pkg/model/rule.go:Rule"}},
			{ID: "R-spec", Claim: "B.", ImplementedBy: []string{"spec/pkg/model/rule.go:Rule"}},
		},
	}
	documents, err := BuildSpecDocumentsFromRows(graph, map[string]SpecRow{})
	if err == nil {
		t.Fatalf("colliding packages produced a bundle: %v", documents)
	}
	if documents != nil {
		t.Fatalf("collision error returned a partial bundle: %v", documents)
	}
}
