package evidence

import (
	"github.com/PHPCraftdream/HotamSpec/internal/conformance"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/source"
)

func conformanceFindings(g *ontology.Graph, report Report, existing []Finding, checks []source.Check) []Finding {
	var findings []Finding
	for _, issue := range report.Conformance.Issues {
		if issue.Code == conformance.IssueCaseDiscrepancy && hasCaseDiscrepancy(existing, issue.CaseID, issue.AtomID) {
			continue
		}
		finding := Finding{
			RequirementID: issue.AtomID, ClauseID: issue.ClauseID, CaseID: issue.CaseID,
			Test: "", Subject: "conformance audit", Kind: conformanceFindingKind(issue.Code),
			ReviewStatus: ReviewUnreviewed, Disposition: issue.Disposition,
		}
		if result := findReportRequirement(&report, issue.AtomID); result != nil {
			finding.Claim = result.Claim
			finding.ClaimTexts = cloneClaimTexts(result.ClaimTexts)
			finding.SourceLinks = append([]ontology.SourceLink(nil), result.SourceLinks...)
			if finding.Profile == "" {
				finding.Profile = result.Profile
			}
		}
		if requirement := requirementByID(g, issue.AtomID); requirement != nil {
			if finding.Claim == "" {
				finding.Claim = requirement.Claim
				finding.ClaimTexts = cloneClaimTexts(requirement.ClaimTexts)
			}
			if len(finding.SourceLinks) == 0 {
				finding.SourceLinks = append([]ontology.SourceLink(nil), requirement.SourceLinks...)
			}
		}
		if clause := sourceClauseByID(g, issue.ClauseID); clause != nil {
			if len(finding.SourceLinks) == 0 {
				finding.SourceLinks = append([]ontology.SourceLink(nil), clause.SourceLinks...)
			}
		}
		assessment := findCaseAssessment(report.Conformance.Cases, issue.CaseID, issue.AtomID)
		if assessment != nil {
			finding.Test = assessment.Test
			if finding.Profile == "" {
				finding.Profile = assessment.Profile
			}
			if finding.Operation == "" {
				finding.Operation = assessment.Operation
			}
			if finding.Target == "" {
				finding.Target = assessment.Target
			}
			if finding.Producer == "" {
				finding.Producer = assessment.Producer
			}
			finding.Context = declaredAssessmentContext(g, assessment)
			for _, execution := range assessment.Executions {
				if issue.Producer != "" && execution.Producer != issue.Producer {
					continue
				}
				finding.Context.Profile = execution.ObservedProfile
				finding.Context.Operation = execution.Operation
				finding.Context.Target = execution.Target
				finding.Context.Producer = execution.Producer
				finding.Context.MeasuredComponents = normalizeComponents(execution.Components)
				break
			}
		}
		finding.ID = findingID(finding, matchingSources(checks, sourceIDsFromLinks(finding.SourceLinks)))
		findings = append(findings, finding)
	}
	return findings
}

func declaredAssessmentContext(g *ontology.Graph, assessment *conformance.CaseAssessment) Context {
	return Context{
		Profile: assessment.Profile, ProfileDetails: assessment.ProfileDetails,
		Operation: assessment.Operation, Target: assessment.Target,
		Producer: assessment.Producer, Composition: compositionByTarget(g, assessment.Target),
		ProfileFingerprint:  assessment.ProfileFingerprint,
		ProducerFingerprint: assessment.ProducerFingerprint,
		CaseFingerprint:     assessment.CaseFingerprint,
	}
}

// Missing executions retain declarations, never fabricated measured context.
func attachDeclaredCaseContexts(g *ontology.Graph, report Report, findings []Finding, checks []source.Check) {
	for i := range findings {
		finding := &findings[i]
		if finding.CaseID == "" || !contextEmpty(finding.Context) {
			continue
		}
		assessment := findCaseAssessment(report.Conformance.Cases, finding.CaseID, finding.RequirementID)
		if assessment == nil {
			continue
		}
		finding.Profile = assessment.Profile
		finding.Context = declaredAssessmentContext(g, assessment)
		finding.ID = findingID(*finding, matchingSources(checks, sourceIDsFromLinks(finding.SourceLinks)))
	}
}

func conformanceFindingKind(code string) string {
	switch code {
	case conformance.IssueCaseDiscrepancy, conformance.IssuePrecedenceDiscrepancy:
		return FindingDiscrepancy
	case conformance.IssueSourceUnverified:
		return FindingSourceDrift
	case conformance.IssueCaseMissingExecution, conformance.IssueCaseUnverified,
		conformance.IssueCaseUnknownExecution, conformance.IssuePrecedenceUnwitnessed,
		conformance.IssueCompositionUnmeasured:
		return FindingExecutionUnavailable
	default:
		return FindingConformance
	}
}

func hasCaseDiscrepancy(findings []Finding, caseID, requirementID string) bool {
	for _, finding := range findings {
		if finding.CaseID != caseID || finding.RequirementID != requirementID {
			continue
		}
		if finding.Kind == FindingDiscrepancy || finding.Kind == FindingTestFailure {
			return true
		}
	}
	return false
}

func sourceClauseByID(g *ontology.Graph, id string) *ontology.SourceClause {
	if g == nil || g.Conformance == nil {
		return nil
	}
	for i := range g.Conformance.Clauses {
		if g.Conformance.Clauses[i].ID == id {
			return &g.Conformance.Clauses[i]
		}
	}
	return nil
}

func findCaseAssessment(cases []conformance.CaseAssessment, caseID, atomID string) *conformance.CaseAssessment {
	for i := range cases {
		if cases[i].ID == caseID && (atomID == "" || cases[i].AtomID == atomID) {
			return &cases[i]
		}
	}
	return nil
}
