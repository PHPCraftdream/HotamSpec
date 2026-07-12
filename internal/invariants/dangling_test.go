package invariants

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestCheckNoDanglingAssumptionOwner_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Assumptions:  []ontology.Assumption{{ID: "A-1", Statement: "x", Status: ontology.AssumptionHOLDS, Owner: "sa"}},
	}
	if vs := runCheck(t, "check_no_dangling_assumption_owner", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckNoDanglingAssumptionOwner_FiresOnUnknownOwner(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Assumptions:  []ontology.Assumption{{ID: "A-1", Statement: "x", Status: ontology.AssumptionHOLDS, Owner: "ghost"}},
	}
	vs := runCheck(t, "check_no_dangling_assumption_owner", g)
	if !hasViolationFor(vs, "A-1") {
		t.Fatalf("expected violation on A-1, got %v", vs)
	}
}

func TestCheckAssumptionStatusValid_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-1", Statement: "x", Status: ontology.AssumptionHOLDS, Owner: "sa"},
			{ID: "A-2", Statement: "y", Status: ontology.AssumptionIMPLEMENTS, Owner: "sa"},
		},
	}
	if vs := runCheck(t, "check_assumption_status_valid", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckAssumptionStatusValid_FiresOnBogusStatus(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{{ID: "A-1", Statement: "x", Status: "BOGUS", Owner: "sa"}},
	}
	vs := runCheck(t, "check_assumption_status_valid", g)
	if !hasViolationFor(vs, "A-1") {
		t.Fatalf("expected violation on A-1, got %v", vs)
	}
}

func TestCheckNoDanglingRequirementOwner_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA, sB},
		Requirements: []ontology.Requirement{req("R-1", "sa"), req("R-2", "sb")},
	}
	if vs := runCheck(t, "check_no_dangling_requirement_owner", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckNoDanglingRequirementOwner_FiresOnUnknownOwner(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA, sB},
		Requirements: []ontology.Requirement{req("R-1", "sa"), req("R-2", "no_such_owner")},
	}
	vs := runCheck(t, "check_no_dangling_requirement_owner", g)
	if !hasViolationFor(vs, "R-2") {
		t.Fatalf("expected violation on R-2, got %v", vs)
	}
}

func TestCheckNoDanglingRequirementAssumptions_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{{ID: "A-1", Statement: "x", Status: ontology.AssumptionHOLDS, Owner: "sa"}},
		Requirements: []ontology.Requirement{
			{ID: "R-1", Claim: "c", Owner: "sa", Status: ontology.StatusSETTLED, Assumptions: []string{"A-1"}},
		},
	}
	if vs := runCheck(t, "check_no_dangling_requirement_assumptions", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckNoDanglingRequirementAssumptions_FiresOnMissing(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", Claim: "c", Owner: "sa", Status: ontology.StatusSETTLED, Assumptions: []string{"A-ghost"}},
		},
	}
	vs := runCheck(t, "check_no_dangling_requirement_assumptions", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation on R-1, got %v", vs)
	}
}

func TestCheckNoDanglingRequirementRelations_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", Claim: "c", Owner: "sa", Status: ontology.StatusSETTLED, Relations: []ontology.Relation{{Kind: "refines", Target: "R-2"}}},
			req("R-2", "sb"),
		},
	}
	if vs := runCheck(t, "check_no_dangling_requirement_relations", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckNoDanglingRequirementRelations_FiresOnBadKindAndTarget(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", Claim: "c", Owner: "sa", Status: ontology.StatusSETTLED, Relations: []ontology.Relation{{Kind: "unknown-kind", Target: "R-missing"}}},
		},
	}
	vs := runCheck(t, "check_no_dangling_requirement_relations", g)
	if !hasViolationFor(vs, "R-1") || len(vs) < 2 {
		t.Fatalf("expected >=2 violations on R-1 (kind+target), got %v", vs)
	}
}

func TestCheckNoDanglingConflictRefs_OK(t *testing.T) {
	t.Parallel()
	shared := "A-1"
	g := graphWithConflict(
		ontology.Conflict{
			ID:   ontology.ConflictIdentity("cost-vs-flexibility", "ctx"),
			Axis: "cost-vs-flexibility", Context: "ctx", Members: []string{"R-1", "R-2"},
			Steward: "outsider", Lifecycle: "ACKNOWLEDGED",
			SharedAssumption: &shared, Derived: []string{}, DecidedBy: "outsider",
		},
		[]ontology.Requirement{req("R-1", "sa"), req("R-2", "sb")},
		ontology.Assumption{ID: "A-1", Statement: "x", Status: ontology.AssumptionHOLDS, Owner: "sa"},
	)
	if vs := runCheck(t, "check_no_dangling_conflict_refs", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckNoDanglingConflictRefs_FiresOnAllDanglingRefs(t *testing.T) {
	t.Parallel()
	shared := "A-ghost"
	c := ontology.Conflict{
		ID: "C-bad", Axis: "cost-vs-flexibility", Context: "ctx",
		Members: []string{"R-ghost"}, Steward: "ghost-steward",
		Lifecycle: "ACKNOWLEDGED", SharedAssumption: &shared,
		Derived: []string{"R-derived-ghost"}, DecidedBy: "ghost-decider",
	}
	g := graphWithConflict(c, []ontology.Requirement{req("R-1", "sa")})
	vs := runCheck(t, "check_no_dangling_conflict_refs", g)
	if len(vs) < 5 {
		t.Fatalf("expected >=5 violations (steward/member/shared/derived/decided_by), got %v", vs)
	}
}

func TestCheckNoDanglingOperatorRefs_OK(t *testing.T) {
	t.Parallel()
	parent := "OP-parent"
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Operators: []ontology.Operator{
			{ID: "OP-parent", Stakeholder: "sa", Lifecycle: "ACTIVE"},
			{ID: "OP-child", Stakeholder: "sa", Lifecycle: "ACTIVE", Parent: &parent},
		},
	}
	if vs := runCheck(t, "check_no_dangling_operator_refs", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckNoDanglingOperatorRefs_FiresOnBadStakeholderAndParent(t *testing.T) {
	t.Parallel()
	badParent := "OP-ghost"
	g := &ontology.Graph{
		Operators: []ontology.Operator{
			{ID: "OP-1", Stakeholder: "ghost", Lifecycle: "ACTIVE", Parent: &badParent},
		},
	}
	vs := runCheck(t, "check_no_dangling_operator_refs", g)
	if !hasViolationFor(vs, "OP-1") || len(vs) < 2 {
		t.Fatalf("expected >=2 violations on OP-1, got %v", vs)
	}
}

func TestCheckNoDanglingIDs_OK(t *testing.T) {
	t.Parallel()
	g := graphWithConflict(baseConflict(), nil)
	if vs := runCheck(t, "check_no_dangling_ids", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckNoDanglingIDs_FiresOnDanglingMember(t *testing.T) {
	t.Parallel()
	bad := baseConflict()
	bad.Members = []string{"R-1", "R-ghost"}
	g := graphWithConflict(bad, nil)
	vs := runCheck(t, "check_no_dangling_ids", g)
	found := false
	for _, v := range vs {
		if v.ID == bad.ID && strings.Contains(v.Message, "R-ghost") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a dangling-member violation naming R-ghost, got %v", vs)
	}
}

func TestCheckDocReaderResolvesToStakeholder_NoopEmptyGraph(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_doc_reader_resolves_to_stakeholder", &ontology.Graph{}); len(vs) != 0 {
		t.Fatalf("expected no violations on empty graph, got %v", vs)
	}
}

func TestCheckDocReaderResolvesToStakeholder_NoopAspectNotAdopted(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA, sB}}
	if vs := runCheck(t, "check_doc_reader_resolves_to_stakeholder", g); len(vs) != 0 {
		t.Fatalf("expected no violations (Go graph carries no DOC_READERS binding), got %v", vs)
	}
}
