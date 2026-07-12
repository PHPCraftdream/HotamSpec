package invariants

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func TestLifecycleWellformedIssues_OK(t *testing.T) {
	t.Parallel()
	lc := ontology.Lifecycle{
		Slug: "ok-lc",
		States: []ontology.State{
			{Name: "A", Kind: ontology.StateKindInitial},
			{Name: "B", Kind: ontology.StateKindNormal},
			{Name: "C", Kind: ontology.StateKindQuiescent},
		},
		Transitions: []ontology.Transition{
			{Src: "A", Dst: "B", Event: "go"},
			{Src: "B", Dst: "C", Event: "done"},
		},
	}
	if issues := lifecycleWellformedIssues(lc); len(issues) != 0 {
		t.Fatalf("expected no issues for a well-formed lifecycle, got %v", issues)
	}
}

func TestLifecycleWellformedIssues_FiresOnEmptyStates(t *testing.T) {
	t.Parallel()
	lc := ontology.Lifecycle{Slug: "empty-lc"}
	if issues := lifecycleWellformedIssues(lc); len(issues) == 0 {
		t.Fatalf("expected an issue for a lifecycle with no states, got %v", issues)
	}
}

func TestLifecycleWellformedIssues_FiresOnMultipleInitials(t *testing.T) {
	t.Parallel()
	lc := ontology.Lifecycle{
		Slug: "multi-init",
		States: []ontology.State{
			{Name: "A", Kind: ontology.StateKindInitial},
			{Name: "B", Kind: ontology.StateKindInitial},
			{Name: "C", Kind: ontology.StateKindQuiescent},
		},
		Transitions: []ontology.Transition{{Src: "A", Dst: "C", Event: "go"}},
	}
	issues := lifecycleWellformedIssues(lc)
	found := false
	for _, s := range issues {
		if strings.Contains(s, "expected exactly 1 INITIAL state") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a multiple-initial issue, got %v", issues)
	}
}

func TestLifecycleWellformedIssues_FiresOnDanglingTransition(t *testing.T) {
	t.Parallel()
	lc := ontology.Lifecycle{
		Slug: "dangling",
		States: []ontology.State{
			{Name: "A", Kind: ontology.StateKindInitial},
			{Name: "C", Kind: ontology.StateKindQuiescent},
		},
		Transitions: []ontology.Transition{{Src: "A", Dst: "GHOST", Event: "go"}},
	}
	issues := lifecycleWellformedIssues(lc)
	found := false
	for _, s := range issues {
		if strings.Contains(s, "unknown dst") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an unknown-dst issue, got %v", issues)
	}
}

func TestLifecycleWellformedIssues_FiresOnNoReachableTerminal(t *testing.T) {
	t.Parallel()
	lc := ontology.Lifecycle{
		Slug: "no-terminal",
		States: []ontology.State{
			{Name: "A", Kind: ontology.StateKindInitial},
			{Name: "B", Kind: ontology.StateKindNormal},
		},
		Transitions: []ontology.Transition{{Src: "A", Dst: "B", Event: "go"}},
	}
	issues := lifecycleWellformedIssues(lc)
	found := false
	for _, s := range issues {
		if strings.Contains(s, "no terminal/quiescent state reachable") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a no-reachable-terminal issue, got %v", issues)
	}
}

func TestLifecycleWellformedIssues_CyclicSkipsTerminalCheck(t *testing.T) {
	t.Parallel()
	lc := ontology.Lifecycle{
		Slug: "cyclic-no-terminal",
		States: []ontology.State{
			{Name: "A", Kind: ontology.StateKindInitial},
			{Name: "B", Kind: ontology.StateKindNormal},
		},
		Transitions: []ontology.Transition{
			{Src: "A", Dst: "B", Event: "go"},
			{Src: "B", Dst: "A", Event: "loop"},
		},
		Cyclic: true,
	}
	if issues := lifecycleWellformedIssues(lc); len(issues) != 0 {
		t.Fatalf("cyclic lifecycle must skip the terminal-reachability check, got %v", issues)
	}
}

func TestCheckRequirementStatusInLifecycle_OK(t *testing.T) {
	t.Parallel()
	reqs := []ontology.Requirement{
		reqStatus("R-1", "sa", ontology.StatusSETTLED),
		reqStatus("R-2", "sb", ontology.StatusDRAFT),
		reqStatus("R-3", "sa", "OPEN(question?)"),
		reqStatus("R-4", "sb", ontology.StatusREJECTED),
	}
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA, sB}, Requirements: reqs}
	if vs := runCheck(t, "check_requirement_status_in_lifecycle", g); len(vs) != 0 {
		t.Fatalf("expected no violations for canonical statuses, got %v", vs)
	}
}

func TestCheckRequirementStatusInLifecycle_FiresOnBogusStatus(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{reqStatus("R-1", "sa", "BOGUS")}}
	vs := runCheck(t, "check_requirement_status_in_lifecycle", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation on R-1 for bogus status, got %v", vs)
	}
}

func TestCheckRequirementHistoryWellformed_OK(t *testing.T) {
	t.Parallel()
	r := req("R-1", "sa")
	r.History = []ontology.HistoryEntry{
		{At: "2024-01-01T00:00:00Z", Summary: "created"},
		{At: "2024-02-01T00:00:00Z", Summary: "updated"},
	}
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	if vs := runCheck(t, "check_requirement_history_wellformed", g); len(vs) != 0 {
		t.Fatalf("expected no violations for well-formed history, got %v", vs)
	}
}

func TestCheckRequirementHistoryWellformed_FiresOnEmptyAt(t *testing.T) {
	t.Parallel()
	r := req("R-1", "sa")
	r.History = []ontology.HistoryEntry{{At: "", Summary: "no stamp"}}
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_requirement_history_wellformed", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation on R-1 for empty `at`, got %v", vs)
	}
}

func TestCheckRequirementHistoryWellformed_FiresOnEmptySummary(t *testing.T) {
	t.Parallel()
	r := req("R-1", "sa")
	r.History = []ontology.HistoryEntry{{At: "2024-01-01T00:00:00Z", Summary: ""}}
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_requirement_history_wellformed", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation on R-1 for empty summary, got %v", vs)
	}
}

func TestCheckRequirementHistoryWellformed_FiresOnBackwardsStamp(t *testing.T) {
	t.Parallel()
	r := req("R-1", "sa")
	r.History = []ontology.HistoryEntry{
		{At: "2024-02-01T00:00:00Z", Summary: "later"},
		{At: "2024-01-01T00:00:00Z", Summary: "earlier"},
	}
	g := &ontology.Graph{Stakeholders: []ontology.Stakeholder{sA}, Requirements: []ontology.Requirement{r}}
	vs := runCheck(t, "check_requirement_history_wellformed", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation on R-1 for non-monotonic stamps, got %v", vs)
	}
}

func TestCheckConflictLifecycleInLifecycle_OK(t *testing.T) {
	t.Parallel()
	for _, lc := range []string{ontology.ConflictDETECTED, ontology.ConflictACKNOWLEDGED, "DECIDED(chose A)", "REVISIT_WHEN(x>1)", "HELD(awaiting)"} {
		c := baseConflict()
		c.Lifecycle = lc
		g := graphWithConflict(c, nil)
		if vs := runCheck(t, "check_conflict_lifecycle_in_lifecycle", g); len(vs) != 0 {
			t.Fatalf("expected no violations for lifecycle %q, got %v", lc, vs)
		}
	}
}

func TestCheckConflictLifecycleInLifecycle_FiresOnBogus(t *testing.T) {
	t.Parallel()
	c := baseConflict()
	c.Lifecycle = "BOGUS"
	g := graphWithConflict(c, nil)
	vs := runCheck(t, "check_conflict_lifecycle_in_lifecycle", g)
	if !hasViolationFor(vs, c.ID) {
		t.Fatalf("expected violation on conflict for bogus lifecycle, got %v", vs)
	}
}

func TestCheckOperatorLifecycleInLifecycle_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Operators: []ontology.Operator{
			op("OP-1", "sa", "ACTIVE"),
			op("OP-2", "sa", "SATURATED"),
		},
	}
	if vs := runCheck(t, "check_operator_lifecycle_in_lifecycle", g); len(vs) != 0 {
		t.Fatalf("expected no violations for canonical operator lifecycles, got %v", vs)
	}
}

func TestCheckOperatorLifecycleInLifecycle_FiresOnBogus(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Operators:    []ontology.Operator{op("OP-1", "sa", "BOGUS")},
	}
	vs := runCheck(t, "check_operator_lifecycle_in_lifecycle", g)
	if !hasViolationFor(vs, "OP-1") {
		t.Fatalf("expected violation on OP-1 for bogus lifecycle, got %v", vs)
	}
}

func TestCheckGoalLifecycleInLifecycle_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Goals:       []ontology.Goal{goal("GOAL-1", "sa", "ACTIVE"), goal("GOAL-2", "sa", "MET")},
	}
	if vs := runCheck(t, "check_goal_lifecycle_in_lifecycle", g); len(vs) != 0 {
		t.Fatalf("expected no violations for canonical goal lifecycles, got %v", vs)
	}
}

func TestCheckGoalLifecycleInLifecycle_FiresOnBogus(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Goals:        []ontology.Goal{goal("GOAL-1", "sa", "BOGUS")},
	}
	vs := runCheck(t, "check_goal_lifecycle_in_lifecycle", g)
	if !hasViolationFor(vs, "GOAL-1") {
		t.Fatalf("expected violation on GOAL-1 for bogus lifecycle, got %v", vs)
	}
}

func TestCheckStatusInLifecycle_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA, sB},
		Requirements: []ontology.Requirement{req("R-1", "sa")},
		Conflicts:    []ontology.Conflict{baseConflict()},
		Operators:    []ontology.Operator{op("OP-1", "sa", "ACTIVE")},
		Goals:        []ontology.Goal{goal("GOAL-1", "sa", "ACTIVE")},
	}
	if vs := runCheck(t, "check_status_in_lifecycle", g); len(vs) != 0 {
		t.Fatalf("expected no violations when all statuses are canonical, got %v", vs)
	}
}

func TestCheckStatusInLifecycle_DelegatesAndFires(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{reqStatus("R-1", "sa", "BOGUS")},
		Operators:    []ontology.Operator{op("OP-1", "sa", "ALSO_BOGUS")},
	}
	vs := runCheck(t, "check_status_in_lifecycle", g)
	if len(vs) < 2 {
		t.Fatalf("check_status_in_lifecycle (delegator) must surface violations from its sub-checks, got %v", vs)
	}
}

func TestCheckCanonicalLifecyclesWellformed_FrameworkSelfTest(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	if vs := runCheck(t, "check_canonical_lifecycles_wellformed", g); len(vs) != 0 {
		t.Fatalf("the framework's own canonical lifecycles MUST be well-formed (self-application), got %v", vs)
	}
}

func TestCheckOperatorStewardNotSelf_OK(t *testing.T) {
	t.Parallel()
	c := baseConflict()
	c.Members = []string{"R-1", "R-2"}
	c.Steward = "OP-out"
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut, sA, sB},
		Requirements: []ontology.Requirement{req("R-1", "sa"), req("R-2", "sb")},
		Conflicts:    []ontology.Conflict{c},
		Operators: []ontology.Operator{
			op("OP-1", "sa", "ACTIVE"),
			op("OP-2", "sb", "ACTIVE"),
			op("OP-out", "outsider", "ACTIVE"),
		},
	}
	if vs := runCheck(t, "check_operator_steward_not_self", g); len(vs) != 0 {
		t.Fatalf("operator outside the member-owners may steward; expected no violations, got %v", vs)
	}
}

func TestCheckOperatorStewardNotSelf_FiresWhenStewardOwnsMember(t *testing.T) {
	t.Parallel()
	c := baseConflict()
	c.Members = []string{"R-1", "R-2"}
	c.Steward = "OP-1"
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut, sA, sB},
		Requirements: []ontology.Requirement{req("R-1", "sa"), req("R-2", "sb")},
		Conflicts:    []ontology.Conflict{c},
		Operators: []ontology.Operator{
			op("OP-1", "sa", "ACTIVE"),
			op("OP-2", "sb", "ACTIVE"),
		},
	}
	vs := runCheck(t, "check_operator_steward_not_self", g)
	if !hasViolationFor(vs, c.ID) {
		t.Fatalf("expected violation when operator stewards a conflict its stakeholder owns a member of, got %v", vs)
	}
}

func TestCheckOperatorWithinBudget_OKUnderLimit(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{req("R-1", "sa")},
		Operators: []ontology.Operator{
			{ID: "OP-1", Stakeholder: "sa", Lifecycle: "ACTIVE", ContextBudget: ontology.ContextBudget{Limit: 100, Measure: ontology.BudgetMeasureNODE_COUNT}},
		},
	}
	if vs := runCheck(t, "check_operator_within_budget", g); len(vs) != 0 {
		t.Fatalf("operator under its NODE_COUNT budget must not fire, got %v", vs)
	}
}

func TestCheckOperatorWithinBudget_FiresOverNodeCount(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{req("R-1", "sa"), req("R-2", "sa"), req("R-3", "sa")},
		Operators: []ontology.Operator{
			{ID: "OP-1", Stakeholder: "sa", Lifecycle: "ACTIVE", ContextBudget: ontology.ContextBudget{Limit: 1, Measure: ontology.BudgetMeasureNODE_COUNT}},
		},
	}
	vs := runCheck(t, "check_operator_within_budget", g)
	if !hasViolationFor(vs, "OP-1") {
		t.Fatalf("expected violation on OP-1 for NODE_COUNT over budget, got %v", vs)
	}
}

func TestCheckOperatorWithinBudget_SkipsUnboundedLimit(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Requirements: []ontology.Requirement{req("R-1", "sa")},
		Operators: []ontology.Operator{
			{ID: "OP-1", Stakeholder: "sa", Lifecycle: "ACTIVE", ContextBudget: ontology.ContextBudget{Limit: 0, Measure: ontology.BudgetMeasureNODE_COUNT}},
		},
	}
	if vs := runCheck(t, "check_operator_within_budget", g); len(vs) != 0 {
		t.Fatalf("limit <= 0 means unbounded; the check must be skipped, got %v", vs)
	}
}

func TestCheckOperatorWithinBudget_CrystalCharsLargeLimit(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Operators: []ontology.Operator{
			{ID: "OP-1", Stakeholder: "sa", Lifecycle: "ACTIVE", ContextBudget: ontology.ContextBudget{Limit: 1_000_000_000, Measure: ontology.BudgetMeasureCRYSTAL_CHARS}},
		},
	}
	if vs := runCheck(t, "check_operator_within_budget", g); len(vs) != 0 {
		t.Fatalf("CRYSTAL_CHARS with a very large limit must not fire regardless of CLAUDE.md, got %v", vs)
	}
}
