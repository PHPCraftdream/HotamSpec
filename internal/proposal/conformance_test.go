package proposal

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestProposedRequirement_StrictDecodeRejectsUnknownTypedValueField(t *testing.T) {
	data := `{"id":"R-typed","claim":"claim","owner":"owner","status":"DRAFT","cases":[{"id":"C-typed","input":{"kind":"text","text":"sample","unexpected":"x"}}]}`
	dec := json.NewDecoder(strings.NewReader(data))
	dec.DisallowUnknownFields()
	var p ProposedRequirement
	if err := dec.Decode(&p); err == nil {
		t.Fatal("strict proposal decode accepted an unknown field inside cases.input")
	}
}

func TestProposedRequirement_InvalidSelectionRejected(t *testing.T) {
	p := ProposedRequirement{
		ID: "R-selection", Claim: "claim", Owner: "owner", Status: ontology.StatusDRAFT,
		Cases: []ontology.CaseDefinition{{
			ID: "C-selection",
			Selection: &ontology.SelectionEvidence{
				Matched: []string{"rule-a"}, Selected: "rule-b",
			},
		}},
	}
	if err := p.validate(); err == nil || !strings.Contains(err.Error(), "selection.selected") {
		t.Fatalf("invalid selected branch was not rejected precisely: %v", err)
	}
}

func TestProposedRequirement_SecondaryClaimTextChangeIsAppliedAndRecorded(t *testing.T) {
	graph := &ontology.Graph{
		Languages:       []string{"en", "ru"},
		DefaultLanguage: "ru",
		Requirements: []ontology.Requirement{{
			ID: "R-localized", Claim: "русский текст", Owner: "owner", Status: ontology.StatusDRAFT,
			ClaimTexts: ontology.LocalizedText{"en": "original English", "ru": "русский текст"},
		}},
	}
	p := ProposedRequirement{
		ID: "R-localized", Claim: "русский текст", Owner: "owner", Status: ontology.StatusDRAFT,
		ClaimTexts: ontology.LocalizedText{"en": "revised English", "ru": "русский текст"},
	}
	if err := p.validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := p.mutate(graph, "2026-10-04"); err != nil {
		t.Fatalf("mutate: %v", err)
	}
	got := graph.Requirements[0]
	if got.Claim != "русский текст" || got.ClaimTexts["ru"] != "русский текст" {
		t.Fatalf("default-language claim changed unexpectedly: claim=%q texts=%v", got.Claim, got.ClaimTexts)
	}
	if got.ClaimTexts["en"] != "revised English" {
		t.Fatalf("secondary-language edit was lost: %v", got.ClaimTexts)
	}
	if len(got.History) != 1 || !strings.Contains(got.History[0].Summary, "claim_texts") {
		t.Fatalf("secondary-language edit was not recorded in history: %+v", got.History)
	}
}

func TestCloneGraph_PreservesAndDeepCopiesRuntimeLanguageConfig(t *testing.T) {
	graph := &ontology.Graph{
		Languages: []string{"en", "ru"}, DefaultLanguage: "ru", RenderLanguage: "en",
		Conformance: &ontology.ConformanceConfig{
			Profiles: []ontology.Profile{{ID: "profile-a", Features: []string{"feature-a"}}},
		},
	}
	clone, err := cloneGraph(graph)
	if err != nil {
		t.Fatalf("cloneGraph: %v", err)
	}
	clone.Languages[0] = "ru"
	clone.Conformance.Profiles[0].Features[0] = "changed"
	if graph.Languages[0] != "en" || graph.Conformance.Profiles[0].Features[0] != "feature-a" {
		t.Fatalf("clone aliases runtime configuration: original=%+v", graph)
	}
	if clone.DefaultLanguage != "ru" || clone.RenderLanguage != "en" {
		t.Fatalf("clone lost language selection state: %+v", clone)
	}
}
