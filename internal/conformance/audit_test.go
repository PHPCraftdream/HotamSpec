package conformance

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/source"
)

func TestAuditFromSourcesUsesOneAuthoritativeSourceSnapshot(t *testing.T) {
	root := t.TempDir()
	contents := []byte("# Clause\nrequire a result\n")
	path := filepath.Join(root, "source.md")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(contents)
	g := &ontology.Graph{}
	g.DomainDir = root
	g.SpecificationSources = []ontology.SpecificationSource{{
		ID: "spec", Path: "source.md", Version: "v1", SHA256: hex.EncodeToString(digest[:]),
	}}
	g.Conformance = &ontology.ConformanceConfig{Clauses: []ontology.SourceClause{{
		ID: "clause", SourceLinks: []ontology.SourceLink{{SourceID: "spec", Anchor: "L1"}},
	}}}
	checks := source.Verify(g)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	snapshot := AuditFromSources(g, nil, checks)
	fresh := Audit(g, nil)
	hasSourceIssue := func(report Report) bool {
		for _, issue := range report.Issues {
			if issue.Code == IssueSourceUnverified {
				return true
			}
		}
		return false
	}
	if hasSourceIssue(snapshot) {
		t.Fatalf("provided verified source snapshot was reread or discarded: %+v", snapshot.Issues)
	}
	if !hasSourceIssue(fresh) {
		t.Fatalf("standalone Audit did not freshly verify its source link after the source disappeared: %+v", fresh.Issues)
	}
}

func TestAuditFindsUndecomposedSidesMethodsAndMissingExecution(t *testing.T) {
	root := t.TempDir()
	sourceBytes := []byte("# Clause\nrequire a result\n")
	if err := os.WriteFile(filepath.Join(root, "source.md"), sourceBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(sourceBytes)
	g := &ontology.Graph{}
	g.DomainDir = root
	g.SpecificationSources = []ontology.SpecificationSource{{ID: "spec", Path: "source.md", Version: "v1", SHA256: hex.EncodeToString(digest[:])}}
	g.Conformance = &ontology.ConformanceConfig{
		RuleCases: true,
		Clauses: []ontology.SourceClause{{
			ID: "clause-result", SourceLinks: []ontology.SourceLink{{SourceID: "spec", Anchor: "1"}},
			Sides: []string{"success", "failure"}, Strength: "MUST",
		}},
	}
	atom := ontology.Requirement{ID: "R-result", AtomKind: "rule"}
	atom.ClauseLinks = []ontology.ClauseLink{{ClauseID: "clause-result", Side: "success"}}
	atom.Cases = []ontology.CaseDefinition{{ID: "case-success", AtomIDs: []string{"R-result"}, Sides: []string{"success"}}}
	g.Requirements = []ontology.Requirement{atom}

	report := Audit(g, nil)
	found := map[string]bool{}
	for _, issue := range report.Issues {
		found[issue.Code] = true
	}
	for _, code := range []string{IssueClauseMissingSide, IssueAtomMissingMethod, IssueCaseMissingExecution} {
		if !found[code] {
			t.Errorf("audit omitted %s: %+v", code, report.Issues)
		}
	}
	if report.CasesDeclared != 1 || report.CasesExecuted != 0 || !report.HasBlockingIssues() {
		t.Fatalf("structural inventory counts/disposition = %+v, want one declared unexecuted case with no fabricated discrepancy", report)
	}
}

func TestAuditDoesNotLetProfileApplicabilityHideObservedFailure(t *testing.T) {
	g := &ontology.Graph{}
	g.Conformance = &ontology.ConformanceConfig{
		RuleCases: true,
		Profiles:  []ontology.Profile{{ID: "base", Features: []string{"text"}}},
	}
	atom := ontology.Requirement{ID: "R-rule", AtomKind: "rule"}
	atom.Applicability = &ontology.Applicability{Profiles: []string{"extended"}}
	atom.Cases = []ontology.CaseDefinition{{ID: "case-1", AtomIDs: []string{"R-rule"}, Profile: "base", Target: "direct-sdk", Producer: "sdk"}}
	g.Requirements = []ontology.Requirement{atom}
	failed := Execution{
		CaseID: "case-1", AtomID: "R-rule", Verdict: "fail", Profile: "base",
		Target: "direct-sdk", Producer: "sdk",
		Comparisons: []Comparison{{Name: "value", Passed: false}},
	}
	report := Audit(g, []Execution{failed})
	if len(report.Cases) != 1 || report.Cases[0].Status != "discrepancy" || report.Cases[0].Qualification != "not_applicable" {
		t.Fatalf("profile condition masked the observed failure: %+v", report.Cases)
	}
	if !report.HasBlockingIssues() || report.Discrepancies != 1 {
		t.Fatalf("observed mismatch was not classified as a blocking discrepancy: %+v", report)
	}
}

func TestAuditChecksScopedCycleAndObservedCompetingSelection(t *testing.T) {
	g := &ontology.Graph{}
	g.Conformance = &ontology.ConformanceConfig{RuleCases: true, Profiles: []ontology.Profile{{ID: "p", Operations: []string{"parse"}}}}
	higher := ontology.Requirement{ID: "R-higher"}
	higher.Precedence = []ontology.PrecedenceLink{{Target: "R-lower", Scope: "parse"}}
	higher.Cases = []ontology.CaseDefinition{{ID: "overlap", AtomIDs: []string{"R-higher"}, Profile: "p", Operation: "parse"}}
	lower := ontology.Requirement{ID: "R-lower"}
	lower.Precedence = []ontology.PrecedenceLink{{Target: "R-higher", Scope: "parse"}}
	g.Requirements = []ontology.Requirement{higher, lower}
	execution := Execution{
		CaseID: "overlap", AtomID: "R-higher", Profile: "p", Operation: "parse", Verdict: "pass",
		Selection: &ontology.SelectionEvidence{Matched: []string{"R-higher", "R-lower"}, Selected: "R-lower"},
	}
	report := Audit(g, []Execution{execution})
	found := map[string]bool{}
	for _, issue := range report.Issues {
		found[issue.Code] = true
	}
	if !found[IssuePrecedenceCycle] || !found[IssuePrecedenceDiscrepancy] {
		t.Fatalf("structural cycle/observed competing-selection mismatch missing: %+v", report.Issues)
	}
}

func TestAuditSeparatesDirectAndComposedTargetsAndTracksFingerprints(t *testing.T) {
	g := &ontology.Graph{}
	g.Conformance = &ontology.ConformanceConfig{
		RuleCases: true,
		Profiles:  []ontology.Profile{{ID: "p", Operations: []string{"parse"}}},
		Compositions: []ontology.Composition{{
			ID: "composed-app", Components: []ontology.Component{{ID: "adapter", Role: "adapter", Version: "v1", SHA256: "declared"}},
		}},
	}
	atom := ontology.Requirement{ID: "R-value", AtomKind: "rule"}
	atom.Cases = []ontology.CaseDefinition{
		{ID: "direct-case", AtomIDs: []string{"R-value"}, Profile: "p", Operation: "parse", Target: "direct-sdk", Producer: "sdk"},
		{ID: "composed-case", AtomIDs: []string{"R-value"}, Profile: "p", Operation: "parse", Target: "composed-app", Producer: "adapter"},
	}
	g.Requirements = []ontology.Requirement{atom}
	executions := []Execution{
		{CaseID: "direct-case", AtomID: "R-value", Profile: "p", Operation: "parse", Target: "direct-sdk", Producer: "sdk", Verdict: "pass"},
		{CaseID: "composed-case", AtomID: "R-value", Profile: "p", Operation: "parse", Target: "composed-app", Producer: "composed-producer", Verdict: "pass", Components: []ontology.Component{{ID: "adapter", Role: "adapter", Version: "v2", SHA256: "measured", Measured: true}}},
	}
	report := Audit(g, executions)
	if len(report.Cases) != 2 || report.Cases[0].Target == report.Cases[1].Target {
		t.Fatalf("direct SDK and composed target were collapsed: %+v", report.Cases)
	}
	var changed CompositionAssessment
	for _, composition := range report.Compositions {
		if composition.ID == "composed-app" {
			changed = composition
		}
	}
	if changed.Status != "changed" || changed.MeasuredFingerprint == "" || len(changed.ObservedComponents) != 1 {
		t.Fatalf("measured composition change was lost: %+v", changed)
	}
	if report.Cases[0].ProfileFingerprint == report.Cases[1].ProfileFingerprint {
		t.Fatalf("target selection fingerprint did not distinguish direct/composed: %+v", report.Cases)
	}
	if len(report.Issues) == 0 {
		t.Fatal("declared/measured producer or composition changes were not reported")
	}

	var oldCase string
	for _, assessment := range report.Cases {
		if assessment.ID == "direct-case" {
			oldCase = assessment.CaseFingerprint
		}
	}
	atom.Cases[0].Expected = &ontology.ObservedValue{Kind: "text", Text: stringPtr("different")}
	g.Requirements[0] = atom
	changedReport := Audit(g, executions)
	for _, assessment := range changedReport.Cases {
		if assessment.ID == "direct-case" && assessment.CaseFingerprint == oldCase {
			t.Fatal("changed case expectation retained the old review fingerprint")
		}
	}
}

func TestAuditKeepsSharedCaseOracleSeparateFromPropertyExpectations(t *testing.T) {
	input := &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "YQA="}
	caseExpected := &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "YQA="}
	definition := ontology.CaseDefinition{
		ID: "case-shared-oracle", Test: "spec/model/shared_test.go:TestShared",
		AtomIDs: []string{"R-bytes", "R-length"}, Input: input, Expected: caseExpected,
	}
	g := &ontology.Graph{}
	g.Conformance = &ontology.ConformanceConfig{RuleCases: true}
	g.Requirements = []ontology.Requirement{
		{ID: "R-bytes", AtomKind: "rule", ImplementedBy: []string{"model.Bytes"}, Cases: []ontology.CaseDefinition{definition}},
		{ID: "R-length", AtomKind: "rule", ImplementedBy: []string{"model.Length"}, Cases: []ontology.CaseDefinition{definition}},
	}
	bytesMetadata, lengthMetadata := definition, definition
	report := Audit(g, []Execution{
		{
			CaseID: definition.ID, AtomID: "R-bytes", Test: definition.Test, Verdict: "pass",
			CaseMetadata: &bytesMetadata,
			Comparisons: []Comparison{{
				Name: "raw-bytes", Passed: true,
				Actual:   &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "YQA="},
				Expected: &ontology.ObservedValue{Kind: "bytes", Encoding: "base64", Bytes: "YQA="},
			}},
		},
		{
			CaseID: definition.ID, AtomID: "R-length", Test: definition.Test, Verdict: "pass",
			CaseMetadata: &lengthMetadata,
			Comparisons: []Comparison{{
				Name: "length", Passed: true,
				Actual:   &ontology.ObservedValue{Kind: "integer", Integer: "2"},
				Expected: &ontology.ObservedValue{Kind: "integer", Integer: "2"},
			}},
		},
	})
	if len(report.Cases) != 2 {
		t.Fatalf("case was not assessed for both property atoms: %+v", report.Cases)
	}
	for _, assessment := range report.Cases {
		if assessment.Status != "verified_case" || len(assessment.Executions) != 1 ||
			len(assessment.Executions[0].Comparisons) != 1 {
			t.Fatalf("property-specific comparison was not retained as a passing case: %+v", assessment)
		}
		propertyExpected := assessment.Executions[0].Comparisons[0].Expected
		if assessment.AtomID == "R-bytes" && (propertyExpected == nil || propertyExpected.Kind != "bytes") {
			t.Fatalf("bytes observation lost its property expectation: %+v", assessment)
		}
		if assessment.AtomID == "R-length" && (propertyExpected == nil || propertyExpected.Kind != "integer") {
			t.Fatalf("length observation was replaced with the shared case expectation: %+v", assessment)
		}
		if !ontology.EqualObservedValues(assessment.Expected, caseExpected) {
			t.Fatalf("global case oracle changed in property assessment: %+v", assessment)
		}
	}
	if report.HasBlockingIssues() || report.Discrepancies != 0 {
		t.Fatalf("property-level expectations were confused with the shared case oracle: %+v", report)
	}
	for _, issue := range report.Issues {
		if issue.Code == IssueCaseMetadataConflict {
			t.Fatalf("shared case oracle was treated as conflicting metadata: %+v", issue)
		}
	}
}

func stringPtr(value string) *string { return &value }
func TestAuditRetiredRuleCasesDoNotDecomposeActiveClauseInventory(t *testing.T) {
	g := &ontology.Graph{}
	g.Conformance = &ontology.ConformanceConfig{RuleCases: true}
	g.Conformance.Clauses = []ontology.SourceClause{{
		ID: "active-clause", Sides: []string{"required-condition"},
	}}
	retired := ontology.Requirement{ID: "R-retired", AtomKind: "rule", Status: ontology.StatusREJECTED}
	retired.ClauseLinks = []ontology.ClauseLink{{ClauseID: "active-clause", Side: "required-condition"}}
	retired.Cases = []ontology.CaseDefinition{{ID: "retired-case", AtomIDs: []string{"R-retired"}}}
	g.Requirements = []ontology.Requirement{retired}

	report := Audit(g, nil)
	found := map[string]bool{}
	for _, issue := range report.Issues {
		found[issue.Code] = true
	}
	if !found[IssueClauseMissingDecomposition] || !found[IssueClauseMissingSide] {
		t.Fatalf("retired atom incorrectly covered active inventory: %+v", report.Issues)
	}
	for _, code := range []string{IssueAtomMissingMethod, IssueAtomMissingCases, IssueCaseMissingExecution} {
		if found[code] {
			t.Fatalf("retired rule case created a live %s obligation: %+v", code, report.Issues)
		}
	}
	if report.CasesDeclared != 0 || len(report.Cases) != 0 {
		t.Fatalf("retired case was treated as a live declaration: %+v", report.Cases)
	}
}
