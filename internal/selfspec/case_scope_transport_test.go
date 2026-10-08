package selfspec

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestCaseDescriptorScopeMergePreservesExplicitEvidenceOnlyAndRejectsConflicts(t *testing.T) {
	for _, scenario := range []struct {
		name         string
		existing     []string
		incoming     []string
		wantNil      bool
		wantConflict bool
	}{
		{"legacy absent", nil, nil, true, false},
		{"adopt explicit empty", nil, []string{}, false, false},
		{"retain explicit empty", []string{}, nil, false, false},
		{"empty is not broad scope", []string{}, []string{"clause"}, false, true},
		{"broad scope is not empty", []string{"clause"}, []string{}, false, true},
		{"different concrete clauses", []string{"first"}, []string{"second"}, false, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			merged, err := mergeCaseDescriptor(
				ontology.CaseDefinition{ID: "same", ClauseIDs: scenario.existing},
				ontology.CaseDefinition{ID: "same", ClauseIDs: scenario.incoming},
			)
			if scenario.wantConflict {
				if err == nil || !strings.Contains(err.Error(), "clause_ids") {
					t.Fatalf("conflicting proof scopes merged into one normative witness: scope=%v/%v err=%v", scenario.existing, scenario.incoming, err)
				}
				return
			}
			if err != nil || (merged.ClauseIDs == nil) != scenario.wantNil || len(merged.ClauseIDs) != 0 {
				t.Fatalf("absence/evidence-only scope precedence changed: scope=%v/%v merged=%+v err=%v", scenario.existing, scenario.incoming, merged, err)
			}
		})
	}
}

func TestCaseDescriptorAdoptedScopeAndOrderedOracleDoNotAliasIncomingRecording(t *testing.T) {
	text := "authored"
	incoming := ontology.CaseDefinition{
		ID: "same", ClauseIDs: []string{"first"},
		Input:    &ontology.ObservedValue{Kind: "array", Items: []ontology.ObservedValue{{Kind: "bytes", Encoding: "base64", Bytes: "/wA="}}},
		Expected: &ontology.ObservedValue{Kind: "array", Items: []ontology.ObservedValue{{Kind: "object", Fields: map[string]ontology.ObservedValue{"kept": {Kind: "text", Text: &text}}}}},
	}
	merged, err := mergeCaseDescriptor(ontology.CaseDefinition{ID: "same"}, incoming)
	if err != nil {
		t.Fatal(err)
	}
	incoming.ClauseIDs[0] = "second"
	incoming.Input.Items[0].Bytes = "AA=="
	incoming.Expected.Items[0].Fields["kept"] = ontology.ObservedValue{Kind: "null"}
	if merged.ClauseIDs[0] != "first" || merged.Input.Items[0].Bytes != "/wA=" || merged.Expected.Items[0].Fields["kept"].Kind != "text" {
		t.Fatalf("later observation mutation rewrote persisted clause scope/input/oracle: %+v", merged)
	}
}
