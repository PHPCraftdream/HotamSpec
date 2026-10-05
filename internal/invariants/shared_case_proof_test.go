package invariants

import (
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"testing"
)

func TestSharedCaseJustifiesOnlyItsCompatibleTestProof(t *testing.T) {
	entry := "spec/model/raw_test.go:TestRawBytes"
	first := reqWithLinks("R-bytes", "sa", nil, []string{entry})
	second := reqWithLinks("R-length", "sa", nil, []string{entry})
	first.AtomKind, second.AtomKind = "rule", "rule"
	definition := ontology.CaseDefinition{ID: "fixture-case", Test: entry + "/invalid", AtomIDs: []string{first.ID, second.ID}, Expected: &ontology.ObservedValue{Kind: "integer", Integer: "6"}}
	first.Cases = []ontology.CaseDefinition{definition}
	second.Cases = ontology.CloneCaseDefinitions(first.Cases)
	graph := &ontology.Graph{Conformance: &ontology.ConformanceConfig{RuleCases: true}, Requirements: []ontology.Requirement{first, second}}
	if violations := checkVerifiedByNoUnrelatedReuse(graph); len(violations) != 0 {
		t.Fatalf("explicit shared case was rejected as unrelated proof: %+v", violations)
	}
	graph.Requirements[1].Cases[0].Expected.Integer = "7"
	if violations := checkVerifiedByNoUnrelatedReuse(graph); len(violations) != 2 {
		t.Fatalf("incompatible oracles justified proof reuse: %+v", violations)
	}
	graph.Requirements[1].Cases = ontology.CloneCaseDefinitions(first.Cases)
	graph.Requirements[0].VerifiedBy = append(graph.Requirements[0].VerifiedBy, "spec/model/raw_test.go:TestUnrelated")
	graph.Requirements[1].VerifiedBy = append(graph.Requirements[1].VerifiedBy, "spec/model/raw_test.go:TestUnrelated")
	violations := checkVerifiedByNoUnrelatedReuse(graph)
	if len(violations) != 2 {
		t.Fatalf("shared fixture case leaked justification to a different test: %+v", violations)
	}
	for _, violation := range violations {
		if violation.ID != first.ID && violation.ID != second.ID {
			t.Fatalf("wrong citing atom reported: %+v", violation)
		}
	}
}
