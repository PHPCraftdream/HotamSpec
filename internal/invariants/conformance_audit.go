package invariants

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/conformance"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func checkConformanceAudit(g *ontology.Graph) []Violation {
	if g == nil || !hasConformanceAuditTrigger(g) {
		return nil
	}
	var violations []Violation
	for _, issue := range ontology.ValidateConformance(g) {
		violations = append(violations, Violation{
			Check:   "check_conformance_audit",
			ID:      issue.ID,
			Message: fmt.Sprintf("conformance declaration %s (%s): %s", issue.ID, issue.Kind, issue.Message),
		})
	}
	report, err := collectInvocationEvidence(g)
	if err != nil {
		violations = append(violations, Violation{
			Check:   "check_conformance_audit",
			ID:      g.DomainDir,
			Message: fmt.Sprintf("collect case executions for conformance audit: %v", err),
		})
	} else {
		for _, issue := range report.Conformance.Issues {
			if issue.Disposition != "violation" {
				continue
			}
			violations = append(violations, Violation{
				Check:   "check_conformance_audit",
				ID:      conformanceIssueID(issue),
				Message: fmt.Sprintf("conformance audit %s: %s", issue.Code, issue.Message),
			})
		}
		if report.Conformance.HasBlockingIssues() && len(violations) == 0 {
			violations = append(violations, Violation{
				Check:   "check_conformance_audit",
				ID:      g.DomainDir,
				Message: "conformance audit reported blocking issues without individual issue details",
			})
		}
	}
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].ID != violations[j].ID {
			return violations[i].ID < violations[j].ID
		}
		return violations[i].Message < violations[j].Message
	})
	return violations
}

func conformanceIssueID(issue conformance.Issue) string {
	parts := []string{issue.Code}
	for _, part := range []string{issue.ClauseID, issue.AtomID, issue.CaseID, issue.Profile, issue.Target, issue.Producer, issue.Scope, issue.Operation} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ":")
}

func hasConformanceAuditTrigger(g *ontology.Graph) bool {
	if config := g.Conformance; config != nil {
		if config.RuleCases || len(config.Clauses) > 0 || len(config.Profiles) > 0 || len(config.Compositions) > 0 {
			return true
		}
	}
	for _, requirement := range g.Requirements {
		if requirement.AtomKind != "" || len(requirement.Cases) > 0 || len(requirement.ClauseLinks) > 0 ||
			requirement.Strength != "" || requirement.Applicability != nil || len(requirement.Precedence) > 0 {
			return true
		}
	}
	return false
}

// ConformanceAuditRequired reports whether an explicit rule/case or
// conformance-owned field activates check_conformance_audit.
func ConformanceAuditRequired(g *ontology.Graph) bool {
	return g != nil && hasConformanceAuditTrigger(g)
}

var _ = All.MustRegister("check_conformance_audit", Invariant{
	Name:  "check_conformance_audit",
	Canon: methodology.Domain,
	Claim: "explicit conformance declarations, rule/case metadata, scoped precedence, profiles, compositions, and observed executions pass the shared ontology and conformance audits.",
	Rule:  "IF conformance.rule_cases or any explicit conformance inventory/case/strength/applicability/precedence/profile/composition field is present, call ontology.ValidateConformance and the collector-owned conformance.Audit report; no method discovery is enabled by this trigger and no audit qualification is promoted to a pass.",
	Why:   "Explicit rule/case and source-profile obligations need an all-violations/sync/refusal path, while legacy self_executing_atoms and discipline fields alone create no new conformance duties.",
	Check: checkConformanceAudit,
})
