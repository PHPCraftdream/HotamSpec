package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestPublicationEnforcersUseOwnFieldTriggers(t *testing.T) {
	legacy := &ontology.Graph{SelfExecutingAtoms: true, Discipline: loader.DisciplineFull}
	if got := checkLanguageBundleComplete(legacy); len(got) != 0 {
		t.Fatalf("legacy opt-ins activated language parity: %v", got)
	}
	if hasConformanceAuditTrigger(legacy) {
		t.Fatal("legacy atom/discipline opt-ins activated conformance audit")
	}

	ruleCases := &ontology.Graph{}
	ruleCases.Conformance = &ontology.ConformanceConfig{RuleCases: true}
	if !hasConformanceAuditTrigger(ruleCases) {
		t.Fatal("explicit conformance.rule_cases did not activate the conformance audit")
	}

	declaredCases := &ontology.Graph{}
	requirement := ontology.Requirement{}
	requirement.Cases = make([]ontology.CaseDefinition, 1)
	declaredCases.Requirements = append(declaredCases.Requirements, requirement)
	if !hasConformanceAuditTrigger(declaredCases) {
		t.Fatal("explicit requirement cases did not activate the conformance audit")
	}
}
