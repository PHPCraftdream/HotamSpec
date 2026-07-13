package invariants

import (
	"reflect"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
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

// TestProjectScopeIsIDProjectionNotCopy enforces R-scope-is-projection: an
// operator's sub-domain is a computed PROJECTION (an id-set view derived by
// prefix match over the shared graph), never a copy of any node. projectScope
// must (a) select only the nodes whose ID matches the prefix tuple, and (b)
// carry ONLY id sets -- it embeds no node field data, so a projection stays an
// ID reference set whose membership is independent of the source nodes' non-ID
// fields. Mutating a requirement's Claim after projecting must not change which
// IDs the projection selected, and the projected ID must still resolve into the
// live (mutated) node rather than a frozen copy -- proving it is a view.
func TestProjectScopeIsIDProjectionNotCopy(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-aa", Claim: "original A"},
			{ID: "R-bb", Claim: "original B"},
		},
		Conflicts: []ontology.Conflict{{ID: "C-aa", Axis: "x", Context: "y"}},
	}
	view := projectScope(g, []string{"R-a"})

	// (a) prefix match: R-aa selected, R-bb excluded; no conflict matches "R-a".
	if _, ok := view.requirementIDs["R-aa"]; !ok {
		t.Errorf("projectScope must select prefix-matching R-aa; got requirementIDs=%v", view.requirementIDs)
	}
	if _, ok := view.requirementIDs["R-bb"]; ok {
		t.Errorf("projectScope must NOT select non-matching R-bb (it is a projection, not a copy of all nodes); got requirementIDs=%v", view.requirementIDs)
	}
	if len(view.conflictIDs) != 0 {
		t.Errorf("no conflict ID matches prefix \"R-a\"; expected empty conflictIDs, got %v", view.conflictIDs)
	}

	// (b) the projection carries ids only -- mutating a source node's non-ID
	// field after projecting leaves the projected id-set unchanged, and the id
	// still resolves into the live (mutated) node, proving no data was copied.
	g.Requirements[0].Claim = "mutated"
	if _, ok := view.requirementIDs["R-aa"]; !ok {
		t.Errorf("mutating a source node's Claim must not evict its projected id (projection references ids, not a copied snapshot)")
	}
	var resolved *ontology.Requirement
	for i := range g.Requirements {
		if g.Requirements[i].ID == "R-aa" {
			resolved = &g.Requirements[i]
		}
	}
	if resolved == nil {
		t.Fatalf("projected id R-aa no longer resolves in the graph")
	}
	if resolved.Claim != "mutated" {
		t.Errorf("projected id resolves into a frozen copy (Claim=%q); a projection must reference the live node (Claim=%q)",
			resolved.Claim, "mutated")
	}
}

// TestScopeOverlapNodeIDs_DeterministicIntersection enforces
// R-scope-overlap-generated: when two operators' scope projections share a node,
// the overlap is computed (never hidden). scopeOverlapNodeIDs(a, b) must return
// the sorted intersection of the two views across BOTH requirement and conflict
// id sets. The overlapping case below shares R-2/R-3 (requirements) and C-2
// (conflict); the disjoint sub-check confirms an empty result is returned (not
// hidden or silently merged) when nothing is shared.
func TestScopeOverlapNodeIDs_DeterministicIntersection(t *testing.T) {
	t.Parallel()
	a := scopeView{
		requirementIDs: map[string]struct{}{"R-1": {}, "R-2": {}, "R-3": {}},
		conflictIDs:    map[string]struct{}{"C-1": {}, "C-2": {}},
	}
	b := scopeView{
		requirementIDs: map[string]struct{}{"R-2": {}, "R-3": {}, "R-4": {}},
		conflictIDs:    map[string]struct{}{"C-2": {}, "C-3": {}},
	}
	got := scopeOverlapNodeIDs(a, b)
	// Sorted union of the intersection: {R-2, R-3} (requirements) ∪ {C-2}
	// (conflicts), sorted lexicographically as one combined id set.
	want := []string{"C-2", "R-2", "R-3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scopeOverlapNodeIDs overlapping = %v, want %v (sorted intersection across req+conflict ids)", got, want)
	}
	if !sortStringSliceIsSorted(got) {
		t.Errorf("scopeOverlapNodeIDs must return a sorted slice (deterministic), got %v", got)
	}

	// Disjoint views: overlap is empty -- nothing is hidden or silently merged.
	disjoint := scopeOverlapNodeIDs(
		scopeView{requirementIDs: map[string]struct{}{"R-1": {}}, conflictIDs: map[string]struct{}{"C-1": {}}},
		scopeView{requirementIDs: map[string]struct{}{"R-2": {}}, conflictIDs: map[string]struct{}{"C-2": {}}},
	)
	if len(disjoint) != 0 {
		t.Errorf("scopeOverlapNodeIDs of disjoint views must be empty, got %v", disjoint)
	}
}

// sortStringSliceIsSorted reports whether s is in ascending lexicographic order.
func sortStringSliceIsSorted(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] > s[i] {
			return false
		}
	}
	return true
}
