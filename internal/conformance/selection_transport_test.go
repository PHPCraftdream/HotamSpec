package conformance

import (
	"encoding/json"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestAuditEmptyMatchedSelectionRemainsReadable(t *testing.T) {
	selection := &ontology.SelectionEvidence{Matched: []string{}, Selected: ""}
	definition := ontology.CaseDefinition{ID: "none", Test: "spec/model/rule_test.go:TestNone", AtomIDs: []string{"R-rule"}, Selection: selection}
	g := &ontology.Graph{Requirements: []ontology.Requirement{{ID: "R-rule", AtomKind: "rule", Cases: []ontology.CaseDefinition{definition}}}, Conformance: &ontology.ConformanceConfig{RuleCases: true}}
	report := Audit(g, []Execution{{CaseID: "none", AtomID: "R-rule", Test: definition.Test, Verdict: "pass", Selection: selection, CaseMetadata: &definition, Comparisons: []Comparison{{Passed: true}}}})
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("a recorded no-match selection became unreadable: %v", err)
	}
	if len(decoded.Cases) != 1 || len(decoded.Cases[0].Executions) != 1 {
		t.Fatalf("case execution lost: %+v", decoded.Cases)
	}
	actual := decoded.Cases[0].Executions[0].Selection
	if actual == nil || actual.Matched == nil || len(actual.Matched) != 0 || actual.Selected != "" {
		t.Fatalf("explicit no-match state changed: %+v", actual)
	}
}

func TestSharedCaseHasOnePriorityDiscrepancy(t *testing.T) {
	selection := &ontology.SelectionEvidence{Matched: []string{"R-high", "R-low"}, Selected: "R-low"}
	definition := ontology.CaseDefinition{ID: "overlap", Test: "spec/model/rule_test.go:TestOverlap", AtomIDs: []string{"R-high", "R-low"}, Profile: "core", Operation: "decode", Selection: selection}
	higher := ontology.Requirement{ID: "R-high", AtomKind: "rule", Cases: []ontology.CaseDefinition{definition}, Precedence: []ontology.PrecedenceLink{{Target: "R-low", Scope: "decode-errors"}}}
	lower := ontology.Requirement{ID: "R-low", AtomKind: "rule", Cases: []ontology.CaseDefinition{definition}}
	g := &ontology.Graph{Requirements: []ontology.Requirement{higher, lower}, Conformance: &ontology.ConformanceConfig{RuleCases: true, Profiles: []ontology.Profile{{ID: "core", Operations: []string{"decode"}}}}}
	executions := []Execution{
		{CaseID: "overlap", AtomID: "R-high", Test: definition.Test, Profile: "core", Operation: "decode", Verdict: "pass", Selection: selection, CaseMetadata: &definition, Comparisons: []Comparison{{Passed: true}}},
		{CaseID: "overlap", AtomID: "R-low", Test: definition.Test, Profile: "core", Operation: "decode", Verdict: "pass", Selection: selection, CaseMetadata: &definition, Comparisons: []Comparison{{Passed: true}}},
	}
	report := Audit(g, executions)
	priorityIssues := 0
	for _, issue := range report.Issues {
		if issue.Code == IssuePrecedenceDiscrepancy {
			priorityIssues++
		}
	}
	if priorityIssues != 1 || report.Discrepancies != 1 {
		t.Fatalf("one shared selection was counted once per property atom: %+v", report)
	}
}
