package conformance

import (
	"slices"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func scopedCaseGraph(scope []string) (*ontology.Graph, Execution) {
	definition := ontology.CaseDefinition{
		ID: "one-case", AtomIDs: []string{"R-rule"}, ClauseIDs: scope,
		Profile: "core", Operation: "decode", Sides: []string{"success"},
	}
	graph := &ontology.Graph{
		Conformance: &ontology.ConformanceConfig{
			RuleCases: true, DocumentSections: []ontology.DocumentSection{},
			Clauses:  []ontology.SourceClause{{ID: "first", Sides: []string{"success"}}, {ID: "second", Sides: []string{"success"}}},
			Profiles: []ontology.Profile{{ID: "core", Operations: []string{"decode"}}},
		},
		Requirements: []ontology.Requirement{{
			ID: "R-rule", AtomKind: "rule", ImplementedBy: []string{"model.Rule"},
			ClauseLinks: []ontology.ClauseLink{{ClauseID: "first", Side: "success"}, {ClauseID: "second", Side: "success"}},
			Cases:       []ontology.CaseDefinition{definition},
		}},
	}
	return graph, Execution{
		CaseID: definition.ID, AtomID: "R-rule", Profile: "core", Operation: "decode", Verdict: "pass",
		CaseMetadata: &definition, Comparisons: []Comparison{{Name: "value", Passed: true}},
	}
}

func clauseAssessment(report Report, id string) ClauseAssessment {
	for _, clause := range report.Clauses {
		if clause.ID == id {
			return clause
		}
	}
	return ClauseAssessment{}
}

func TestCaseDeclaredClauseSubsetCannotWitnessBroadMethodLinks(t *testing.T) {
	graph, execution := scopedCaseGraph([]string{"first"})
	graph.Conformance.Clauses[1].Applicability = &ontology.Applicability{Profiles: []string{"other"}}
	report := AuditFromSources(graph, []Execution{execution}, nil)
	first, second := clauseAssessment(report, "first"), clauseAssessment(report, "second")
	if !slices.Equal(first.WitnessCaseIDs, []string{"one-case"}) || first.Qualification != "" || first.Status != "decomposed" {
		t.Fatalf("explicit applicable subset was not witnessed independently: %+v", first)
	}
	if len(second.WitnessCaseIDs) != 0 || second.Qualification != "unproved" || !slices.Equal(second.MissingSides, []string{"success"}) {
		t.Fatalf("method's broad paragraph links fabricated proof outside case scope: %+v", second)
	}
	if len(report.Cases) != 1 || report.Cases[0].Status != "verified_case" || !slices.Equal(report.Cases[0].ClauseIDs, []string{"first"}) {
		t.Fatalf("unselected clause profile restricted independent explicit proof: %+v", report.Cases)
	}
}

func TestMissingAndEmptyCaseScopesRemainUnprovedWithoutHidingExecution(t *testing.T) {
	for _, scope := range [][]string{nil, {}} {
		graph, execution := scopedCaseGraph(scope)
		report := AuditFromSources(graph, []Execution{execution}, nil)
		if report.CasesExecuted != 1 || len(report.Cases) != 1 || report.Cases[0].Status != "verified_case" {
			t.Fatalf("evidence-only case execution was discarded: %+v", report.Cases)
		}
		for _, clause := range report.Clauses {
			if len(clause.WitnessCaseIDs) != 0 || clause.Qualification != "unproved" {
				t.Fatalf("absent authored scope inferred universal proof: %+v", clause)
			}
		}
	}
	graph, execution := scopedCaseGraph(nil)
	graph.Conformance.DocumentSections = nil
	report := AuditFromSources(graph, []Execution{execution}, nil)
	for _, clause := range report.Clauses {
		if !slices.Equal(clause.WitnessCaseIDs, []string{"one-case"}) || len(clause.MissingSides) != 0 {
			t.Fatalf("legacy non-document domain behavior changed: %+v", clause)
		}
	}
}

func TestUnknownUnlinkedAndMalformedScopesBlockAllWitnesses(t *testing.T) {
	for _, sample := range []struct {
		scope []string
		code  string
	}{
		{[]string{"first", "unknown"}, IssueCaseUnknownClause},
		{[]string{"first", "unlinked"}, IssueCaseUnlinkedClause},
		{[]string{"first", "first"}, IssueCaseInvalidClauseScope},
		{[]string{"first", ""}, IssueCaseInvalidClauseScope},
	} {
		graph, execution := scopedCaseGraph(sample.scope)
		graph.Conformance.Clauses = append(graph.Conformance.Clauses, ontology.SourceClause{ID: "unlinked"})
		report := AuditFromSources(graph, []Execution{execution}, nil)
		found := false
		for _, issue := range report.Issues {
			if issue.Code == sample.code && issue.CaseID == "one-case" && issue.Disposition == DispositionViolation {
				found = true
			}
		}
		if !found || len(clauseAssessment(report, "first").WitnessCaseIDs) != 0 {
			t.Fatalf("invalid scope was ignored or valid subset still witnessed: scope=%v issues=%+v", sample.scope, report.Issues)
		}
	}
}

func TestFailedQualifiedOrInapplicableCasesCannotWitnessClauses(t *testing.T) {
	for _, scenario := range []string{"failed", "unsupported", "unreachable", "unverified", "wrong-profile", "profile-unverified", "scoped-clause-inapplicable", "scope-conflict", "typed-mismatch"} {
		t.Run(scenario, func(t *testing.T) {
			graph, execution := scopedCaseGraph([]string{"first"})
			switch scenario {
			case "failed":
				execution.Comparisons[0].Passed = false
			case "unsupported":
				graph.Requirements[0].Coverage = &ontology.CoverageDeclaration{Status: ontology.CoverageUnsupported, Profile: "core", Rationale: "authored qualification"}
			case "unreachable", "unverified":
				graph.Requirements[0].Coverage = &ontology.CoverageDeclaration{Status: scenario, Profile: "core", Rationale: "authored qualification"}
			case "wrong-profile":
				execution.ObservedProfile = "other"
			case "profile-unverified":
				graph.Conformance.Profiles = nil
			case "scoped-clause-inapplicable":
				graph.Conformance.Clauses[0].Applicability = &ontology.Applicability{Profiles: []string{"other"}}
			case "scope-conflict":
				metadata := *execution.CaseMetadata
				metadata.ClauseIDs = []string{"second"}
				execution.CaseMetadata = &metadata
			case "typed-mismatch":
				execution.Comparisons[0].Actual = &ontology.ObservedValue{Kind: "array", Items: []ontology.ObservedValue{{Kind: "integer", Integer: "1"}, {Kind: "integer", Integer: "2"}}}
				execution.Comparisons[0].Expected = &ontology.ObservedValue{Kind: "array", Items: []ontology.ObservedValue{{Kind: "integer", Integer: "2"}, {Kind: "integer", Integer: "1"}}}
			}
			report := AuditFromSources(graph, []Execution{execution}, nil)
			clause := clauseAssessment(report, "first")
			if len(clause.WitnessCaseIDs) != 0 || clause.Qualification != "unproved" {
				t.Fatalf("%s case became normative runtime proof: clause=%+v cases=%+v", scenario, clause, report.Cases)
			}
			if scenario == "typed-mismatch" && report.Cases[0].Status != "discrepancy" {
				t.Fatalf("producer PASS flag hid ordered typed mismatch: %+v", report.Cases)
			}
		})
	}
}
