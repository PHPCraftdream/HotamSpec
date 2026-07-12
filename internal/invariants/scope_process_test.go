package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func TestCheckScopedNodeHasSinglePresenter_TwoOverlappingOpsResolvePresenter(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA, sB},
		Requirements: []ontology.Requirement{req("R-1", "sa")},
		Operators: []ontology.Operator{
			{ID: "OP-1", Stakeholder: "sa", Lifecycle: "ACTIVE", Scope: []string{"R-"}},
			{ID: "OP-2", Stakeholder: "sb", Lifecycle: "ACTIVE", Scope: []string{"R-"}},
		},
	}
	if vs := runCheck(t, "check_scoped_node_has_single_presenter", g); len(vs) != 0 {
		t.Fatalf("expected no violations: presenter is deterministically resolvable, got %v", vs)
	}
}

func TestCheckScopedNodeHasSinglePresenter_AlwaysGreenWhenFewerThanTwoScopedOps(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Operators: []ontology.Operator{
			{ID: "OP-1", Stakeholder: "sa", Lifecycle: "ACTIVE", Scope: []string{"R-"}},
		},
	}
	if vs := runCheck(t, "check_scoped_node_has_single_presenter", g); len(vs) != 0 {
		t.Fatalf("fewer than 2 scoped operators means no overlap; expected no violations, got %v", vs)
	}
}

func TestCheckProcessLifecycleWellformed_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Processes: []ontology.Process{process("PR-1")}}
	if vs := runCheck(t, "check_process_lifecycle_wellformed", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a well-formed Process lifecycle, got %v", vs)
	}
}

func TestCheckProcessLifecycleWellformed_FiresOnMalformedLifecycle(t *testing.T) {
	t.Parallel()
	p := process("PR-1")
	p.Lifecycle = ontology.Lifecycle{Slug: "bad"}
	g := &ontology.Graph{Processes: []ontology.Process{p}}
	vs := runCheck(t, "check_process_lifecycle_wellformed", g)
	if !hasViolationFor(vs, "PR-1") {
		t.Fatalf("expected violation on PR-1 for malformed lifecycle, got %v", vs)
	}
}

func TestCheckProcessDrivesExistingEntities_OK(t *testing.T) {
	t.Parallel()
	p := process("PR-1")
	p.DrivesEntities = []string{"thing"}
	g := &ontology.Graph{
		Processes:   []ontology.Process{p},
		EntityTypes: []ontology.EntityType{entityType("thing")},
	}
	if vs := runCheck(t, "check_process_drives_existing_entities", g); len(vs) != 0 {
		t.Fatalf("expected no violations for declared slug, got %v", vs)
	}
}

func TestCheckProcessDrivesExistingEntities_FiresOnUndeclaredSlug(t *testing.T) {
	t.Parallel()
	p := process("PR-1")
	p.DrivesEntities = []string{"ghost"}
	g := &ontology.Graph{Processes: []ontology.Process{p}}
	vs := runCheck(t, "check_process_drives_existing_entities", g)
	if !hasViolationFor(vs, "PR-1") {
		t.Fatalf("expected violation on PR-1 for undeclared slug, got %v", vs)
	}
}

func TestCheckStepInvokesKnownTransition_OK(t *testing.T) {
	t.Parallel()
	p := process("PR-1")
	p.Steps = []ontology.Step{step("do", "", "thing.activate")}
	g := &ontology.Graph{
		Processes:   []ontology.Process{p},
		EntityTypes: []ontology.EntityType{entityType("thing")},
	}
	if vs := runCheck(t, "check_step_invokes_known_transition", g); len(vs) != 0 {
		t.Fatalf("expected no violations for valid invokes, got %v", vs)
	}
}

func TestCheckStepInvokesKnownTransition_FiresOnMissingDot(t *testing.T) {
	t.Parallel()
	p := process("PR-1")
	p.Steps = []ontology.Step{step("do", "", "noformat")}
	g := &ontology.Graph{Processes: []ontology.Process{p}}
	vs := runCheck(t, "check_step_invokes_known_transition", g)
	if !hasViolationFor(vs, "PR-1") {
		t.Fatalf("expected violation for missing dot format, got %v", vs)
	}
}

func TestCheckStepInvokesKnownTransition_FiresOnUnknownEntity(t *testing.T) {
	t.Parallel()
	p := process("PR-1")
	p.Steps = []ontology.Step{step("do", "", "ghost.activate")}
	g := &ontology.Graph{Processes: []ontology.Process{p}}
	vs := runCheck(t, "check_step_invokes_known_transition", g)
	if !hasViolationFor(vs, "PR-1") {
		t.Fatalf("expected violation for unknown entity slug, got %v", vs)
	}
}

func TestCheckStepInvokesKnownTransition_FiresOnUnknownEvent(t *testing.T) {
	t.Parallel()
	p := process("PR-1")
	p.Steps = []ontology.Step{step("do", "", "thing.bogus")}
	g := &ontology.Graph{
		Processes:   []ontology.Process{p},
		EntityTypes: []ontology.EntityType{entityType("thing")},
	}
	vs := runCheck(t, "check_step_invokes_known_transition", g)
	if !hasViolationFor(vs, "PR-1") {
		t.Fatalf("expected violation for unknown event, got %v", vs)
	}
}

func TestCheckProcessRolesDeclared_OK(t *testing.T) {
	t.Parallel()
	p := process("PR-1")
	p.RolesRequired = []string{"editor"}
	p.Steps = []ontology.Step{step("do", "editor", "")}
	g := &ontology.Graph{Processes: []ontology.Process{p}}
	if vs := runCheck(t, "check_process_roles_declared", g); len(vs) != 0 {
		t.Fatalf("expected no violations for declared role, got %v", vs)
	}
}

func TestCheckProcessRolesDeclared_FiresOnUndeclaredRole(t *testing.T) {
	t.Parallel()
	p := process("PR-1")
	p.RolesRequired = []string{"editor"}
	p.Steps = []ontology.Step{step("do", "ghost", "")}
	g := &ontology.Graph{Processes: []ontology.Process{p}}
	vs := runCheck(t, "check_process_roles_declared", g)
	if !hasViolationFor(vs, "PR-1") {
		t.Fatalf("expected violation for undeclared role, got %v", vs)
	}
}

func TestCheckGoalTargetKindKnown_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Goals: []ontology.Goal{
			{ID: "GOAL-1", Owner: "OP-1", TargetState: ontology.TargetState{Kind: ontology.TargetKindGraphProperty}, Lifecycle: "ACTIVE"},
			{ID: "GOAL-2", Owner: "OP-1", TargetState: ontology.TargetState{Kind: ontology.TargetKindBusinessState}, Lifecycle: "ACTIVE"},
			{ID: "GOAL-3", Owner: "OP-1", TargetState: ontology.TargetState{Kind: ontology.TargetKindEntityState}, Lifecycle: "ACTIVE"},
		},
	}
	if vs := runCheck(t, "check_goal_target_kind_known", g); len(vs) != 0 {
		t.Fatalf("expected no violations for known kinds, got %v", vs)
	}
}

func TestCheckGoalTargetKindKnown_FiresOnBogusKind(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Goals: []ontology.Goal{
			{ID: "GOAL-1", Owner: "OP-1", TargetState: ontology.TargetState{Kind: "BOGUS"}, Lifecycle: "ACTIVE"},
		},
	}
	vs := runCheck(t, "check_goal_target_kind_known", g)
	if !hasViolationFor(vs, "GOAL-1") {
		t.Fatalf("expected violation for bogus kind, got %v", vs)
	}
}

func TestCheckGoalOwnerIsOperator_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Operators:    []ontology.Operator{op("OP-1", "sa", "ACTIVE")},
		Goals:        []ontology.Goal{goal("GOAL-1", "OP-1", "ACTIVE")},
	}
	if vs := runCheck(t, "check_goal_owner_is_operator", g); len(vs) != 0 {
		t.Fatalf("expected no violations for operator owner, got %v", vs)
	}
}

func TestCheckGoalOwnerIsOperator_FiresOnDanglingOwner(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Goals: []ontology.Goal{goal("GOAL-1", "OP-ghost", "ACTIVE")},
	}
	vs := runCheck(t, "check_goal_owner_is_operator", g)
	if !hasViolationFor(vs, "GOAL-1") {
		t.Fatalf("expected violation for dangling owner, got %v", vs)
	}
}
