package diagnose

import (
	"strings"
	"testing"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func settledReq(id string) ontology.Requirement {
	return ontology.Requirement{
		ID:             id,
		Owner:          "owner",
		Status:         ontology.StatusSETTLED,
		Enforcement:    ontology.EnforcementPROSE,
		Enforceability: ontology.EnforceabilityENFORCEABLE,
	}
}

func draftReq(id string) ontology.Requirement {
	return ontology.Requirement{
		ID:             id,
		Owner:          "owner",
		Status:         ontology.StatusDRAFT,
		Enforcement:    ontology.EnforcementPROSE,
		Enforceability: ontology.EnforceabilityENFORCEABLE,
	}
}

func rejectedReq(id string) ontology.Requirement {
	return ontology.Requirement{
		ID:             id,
		Owner:          "owner",
		Status:         ontology.StatusREJECTED,
		Enforcement:    ontology.EnforcementPROSE,
		Enforceability: ontology.EnforceabilityENFORCEABLE,
	}
}

func TestReflectDraftOverhang_Fires(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			settledReq("R-1"), settledReq("R-2"), draftReq("R-3"), draftReq("R-4"),
		},
	}
	fs := ReflectDraftOverhang(g)
	if len(fs) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(fs))
	}
	if fs[0].Target != "burn-down" {
		t.Errorf("target: got %q, want burn-down", fs[0].Target)
	}
	if !strings.Contains(fs[0].Imperative, "2 DRAFT vs 2 SETTLED") {
		t.Errorf("imperative missing counts: %q", fs[0].Imperative)
	}
}

func TestReflectDraftOverhang_DoesNotFire(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			settledReq("R-1"), settledReq("R-2"), settledReq("R-3"),
			draftReq("R-4"),
		},
	}
	if fs := ReflectDraftOverhang(g); len(fs) != 0 {
		t.Errorf("expected 0 findings, got %d", len(fs))
	}
}

func TestReflectDraftOverhang_HalfBoundary(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			settledReq("R-1"), settledReq("R-2"), settledReq("R-3"), settledReq("R-4"), settledReq("R-5"),
			draftReq("R-6"), draftReq("R-7"), draftReq("R-8"),
		},
	}
	fs := ReflectDraftOverhang(g)
	if len(fs) != 1 {
		t.Fatalf("5 SETTLED, 3 DRAFT: 3 >= 2.5 should fire, got %d", len(fs))
	}
}

func TestReflectUnenforcedSettled_Fires(t *testing.T) {
	t.Parallel()
	reqs := make([]ontology.Requirement, 7)
	for i := range reqs {
		reqs[i] = settledReq("R-" + string(rune('a'+i)))
	}
	g := &ontology.Graph{Requirements: reqs}
	fs := ReflectUnenforcedSettled(g)
	if len(fs) != 1 {
		t.Fatalf("7 closeable-debt SETTLED should fire, got %d", len(fs))
	}
	if fs[0].Target != "enforcement-gradient" {
		t.Errorf("target: got %q, want enforcement-gradient", fs[0].Target)
	}
}

func TestReflectUnenforcedSettled_DoesNotFire(t *testing.T) {
	t.Parallel()
	reqs := make([]ontology.Requirement, 5)
	for i := range reqs {
		reqs[i] = settledReq("R-" + string(rune('a'+i)))
	}
	g := &ontology.Graph{Requirements: reqs}
	if fs := ReflectUnenforcedSettled(g); len(fs) != 0 {
		t.Errorf("5 closeable-debt should NOT fire (>5), got %d", len(fs))
	}
}

func TestReflectUnenforcedSettled_InherentlyProseDoesNotCount(t *testing.T) {
	t.Parallel()
	reqs := make([]ontology.Requirement, 7)
	for i := range reqs {
		reqs[i] = settledReq("R-" + string(rune('a'+i)))
		reqs[i].Enforceability = ontology.EnforceabilityINHERENTLY_PROSE
	}
	g := &ontology.Graph{Requirements: reqs}
	if fs := ReflectUnenforcedSettled(g); len(fs) != 0 {
		t.Errorf("INHERENTLY_PROSE should not be closeable debt, got %d", len(fs))
	}
}

func TestReflectOverBudgetOperators_NodeCountFires(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: make([]ontology.Requirement, 20),
		Conflicts:    make([]ontology.Conflict, 20),
		Assumptions:  make([]ontology.Assumption, 20),
		Operators: []ontology.Operator{{
			ID:            "OP-test",
			ContextBudget: ontology.ContextBudget{Limit: 30, Measure: ontology.BudgetMeasureNODE_COUNT},
		}},
	}
	fs := ReflectOverBudgetOperators(g)
	if len(fs) != 1 {
		t.Fatalf("60 nodes > 30 limit should fire, got %d", len(fs))
	}
	if fs[0].Target != "OP-test" {
		t.Errorf("target: got %q, want OP-test", fs[0].Target)
	}
	if !strings.Contains(fs[0].Imperative, "nodes (NODE_COUNT measure)") {
		t.Errorf("imperative missing NODE_COUNT unit: %q", fs[0].Imperative)
	}
}

func TestReflectOverBudgetOperators_UnderBudget(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{settledReq("R-1")},
		Operators: []ontology.Operator{{
			ID:            "OP-test",
			ContextBudget: ontology.ContextBudget{Limit: 100, Measure: ontology.BudgetMeasureNODE_COUNT},
		}},
	}
	if fs := ReflectOverBudgetOperators(g); len(fs) != 0 {
		t.Errorf("1 node < 100 limit should not fire, got %d", len(fs))
	}
}

func TestReflectOverBudgetOperators_CrystalCharsSkipped(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: make([]ontology.Requirement, 100),
		Conflicts:    make([]ontology.Conflict, 100),
		Assumptions:  make([]ontology.Assumption, 100),
		Operators: []ontology.Operator{{
			ID:            "OP-crystal",
			ContextBudget: ontology.ContextBudget{Limit: 10, Measure: ontology.BudgetMeasureCRYSTAL_CHARS},
		}},
	}
	if fs := ReflectOverBudgetOperators(g); len(fs) != 0 {
		t.Errorf("CRYSTAL_CHARS operator should be skipped, got %d", len(fs))
	}
}

func TestReflectOverBudgetOperators_ZeroLimitUnbounded(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: make([]ontology.Requirement, 100),
		Operators: []ontology.Operator{{
			ID:            "OP-unbounded",
			ContextBudget: ontology.ContextBudget{Limit: 0, Measure: ontology.BudgetMeasureNODE_COUNT},
		}},
	}
	if fs := ReflectOverBudgetOperators(g); len(fs) != 0 {
		t.Errorf("limit 0 = unbounded, should not fire, got %d", len(fs))
	}
}

func TestReflectDeadAssumptionOnEnforcer_Fires(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-dead", Status: ontology.AssumptionDEAD},
		},
		Requirements: []ontology.Requirement{
			{
				ID:          "R-enf",
				Status:      ontology.StatusSETTLED,
				Enforcement: ontology.EnforcementENFORCED,
				Assumptions: []string{"A-dead"},
			},
		},
	}
	fs := ReflectDeadAssumptionOnEnforcer(g)
	if len(fs) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(fs))
	}
	if fs[0].Target != "R-enf" {
		t.Errorf("target: got %q, want R-enf", fs[0].Target)
	}
	if !strings.Contains(fs[0].Imperative, "A-dead") {
		t.Errorf("imperative missing dead assumption id: %q", fs[0].Imperative)
	}
}

func TestReflectDeadAssumptionOnEnforcer_ProseNotEnforced(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-dead", Status: ontology.AssumptionDEAD},
		},
		Requirements: []ontology.Requirement{
			{
				ID:          "R-prose",
				Status:      ontology.StatusSETTLED,
				Enforcement: ontology.EnforcementPROSE,
				Assumptions: []string{"A-dead"},
			},
		},
	}
	if fs := ReflectDeadAssumptionOnEnforcer(g); len(fs) != 0 {
		t.Errorf("PROSE requirement on dead assumption should not fire, got %d", len(fs))
	}
}

func TestReflectDeadAssumptionOnEnforcer_NoDeadAssumptions(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-holds", Status: ontology.AssumptionHOLDS},
		},
		Requirements: []ontology.Requirement{
			{
				ID:          "R-enf",
				Status:      ontology.StatusSETTLED,
				Enforcement: ontology.EnforcementENFORCED,
				Assumptions: []string{"A-holds"},
			},
		},
	}
	if fs := ReflectDeadAssumptionOnEnforcer(g); len(fs) != 0 {
		t.Errorf("no dead assumptions should not fire, got %d", len(fs))
	}
}

func TestReflectDerivedButUnbuilt_FiresAbsent(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{{
			ID:        "C-1",
			Lifecycle: "DECIDED(rationale)",
			Derived:   []string{"R-absent"},
		}},
	}
	fs := ReflectDerivedButUnbuilt(g)
	if len(fs) != 1 {
		t.Fatalf("absent derived should fire, got %d", len(fs))
	}
	if fs[0].Target != "R-absent" {
		t.Errorf("target: got %q, want R-absent", fs[0].Target)
	}
}

func TestReflectDerivedButUnbuilt_FiresDraft(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{{
			ID:        "C-1",
			Lifecycle: "DECIDED(rationale)",
			Derived:   []string{"R-draft"},
		}},
		Requirements: []ontology.Requirement{draftReq("R-draft")},
	}
	fs := ReflectDerivedButUnbuilt(g)
	if len(fs) != 1 {
		t.Fatalf("DRAFT derived should fire, got %d", len(fs))
	}
}

func TestReflectDerivedButUnbuilt_DoesNotFireSettled(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{{
			ID:        "C-1",
			Lifecycle: "DECIDED(rationale)",
			Derived:   []string{"R-settled"},
		}},
		Requirements: []ontology.Requirement{settledReq("R-settled")},
	}
	if fs := ReflectDerivedButUnbuilt(g); len(fs) != 0 {
		t.Errorf("SETTLED derived should not fire, got %d", len(fs))
	}
}

func TestReflectDerivedButUnbuilt_NonDecidedSkipped(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{{
			ID:        "C-1",
			Lifecycle: "DETECTED",
			Derived:   []string{"R-absent"},
		}},
	}
	if fs := ReflectDerivedButUnbuilt(g); len(fs) != 0 {
		t.Errorf("DETECTED conflict should be skipped, got %d", len(fs))
	}
}

func TestReflectImplementsDecay_Fires(t *testing.T) {
	t.Parallel()
	old := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-old", Status: ontology.AssumptionIMPLEMENTS, CreatedAt: old},
		},
	}
	fs := ReflectImplementsDecay(g)
	if len(fs) != 1 {
		t.Fatalf("1-year-old IMPLEMENTS should fire, got %d", len(fs))
	}
	if fs[0].Target != "A-old" {
		t.Errorf("target: got %q, want A-old", fs[0].Target)
	}
}

func TestReflectImplementsDecay_DoesNotFireRecent(t *testing.T) {
	t.Parallel()
	recent := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-recent", Status: ontology.AssumptionIMPLEMENTS, CreatedAt: recent},
		},
	}
	if fs := ReflectImplementsDecay(g); len(fs) != 0 {
		t.Errorf("3-day-old IMPLEMENTS should not fire, got %d", len(fs))
	}
}

func TestReflectImplementsDecay_NoDateSkipped(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-nodate", Status: ontology.AssumptionIMPLEMENTS},
		},
	}
	if fs := ReflectImplementsDecay(g); len(fs) != 0 {
		t.Errorf("no-date IMPLEMENTS should be skipped, got %d", len(fs))
	}
}

func TestReflectImplementsDecay_NonImplementsSkipped(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-holds", Status: ontology.AssumptionHOLDS, CreatedAt: "2020-01-01"},
		},
	}
	if fs := ReflectImplementsDecay(g); len(fs) != 0 {
		t.Errorf("HOLDS should be skipped, got %d", len(fs))
	}
}

func TestReflectImplementsDecay_DecidedAtPrecedence(t *testing.T) {
	t.Parallel()
	veryOld := "2020-01-01"
	recent := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-dec", Status: ontology.AssumptionIMPLEMENTS, CreatedAt: veryOld, DecidedAt: recent},
		},
	}
	if fs := ReflectImplementsDecay(g); len(fs) != 0 {
		t.Errorf("decided_at (recent) should take precedence over old created_at, got %d", len(fs))
	}
}

func TestReflectReplacesEdgeMigration_Fires(t *testing.T) {
	t.Parallel()
	r := rejectedReq("R-old")
	r.Why = "REJECTED — REPLACES R-new"
	g := &ontology.Graph{Requirements: []ontology.Requirement{r}}
	fs := ReflectReplacesEdgeMigration(g)
	if len(fs) != 1 {
		t.Fatalf("prose marker without structural edge should fire, got %d", len(fs))
	}
	if fs[0].Target != "R-old" {
		t.Errorf("target: got %q, want R-old", fs[0].Target)
	}
	if !fs[0].Advisory {
		t.Error("finding should be advisory")
	}
}

func TestReflectReplacesEdgeMigration_DoesNotFireWithEdge(t *testing.T) {
	t.Parallel()
	r1 := rejectedReq("R-old")
	r1.Why = "REJECTED — REPLACES R-new"
	r2 := ontology.Requirement{
		ID:     "R-new",
		Status: ontology.StatusSETTLED,
		Relations: []ontology.Relation{
			{Kind: "replaces", Target: "R-old"},
		},
	}
	g := &ontology.Graph{Requirements: []ontology.Requirement{r1, r2}}
	if fs := ReflectReplacesEdgeMigration(g); len(fs) != 0 {
		t.Errorf("structural replaces edge present should not fire, got %d", len(fs))
	}
}

func TestReflectReplacesEdgeMigration_DoesNotFireWithoutProse(t *testing.T) {
	t.Parallel()
	r := rejectedReq("R-plain")
	r.Why = "discarded, no successor"
	g := &ontology.Graph{Requirements: []ontology.Requirement{r}}
	if fs := ReflectReplacesEdgeMigration(g); len(fs) != 0 {
		t.Errorf("no REPLACES prose marker should not fire, got %d", len(fs))
	}
}

func TestReflectReplacesEdgeMigration_DashVariants(t *testing.T) {
	t.Parallel()
	dashes := []string{
		"REJECTED — REPLACES R-new",
		"REJECTED – REPLACES R-new",
		"REJECTED -- REPLACES R-new",
		"REJECTED- REPLACES R-new",
		"REJECTED - REPLACES R-new",
	}
	for _, why := range dashes {
		r := rejectedReq("R-x")
		r.Why = why
		g := &ontology.Graph{Requirements: []ontology.Requirement{r}}
		if fs := ReflectReplacesEdgeMigration(g); len(fs) != 1 {
			t.Errorf("dash variant %q should fire, got %d", why, len(fs))
		}
	}
}

func TestReflectAllMembersRejected_Fires(t *testing.T) {
	t.Parallel()
	c := ontology.Conflict{
		ID:        "C-ghost",
		Lifecycle: ontology.ConflictDETECTED,
		Members:   []string{"R-a", "R-b"},
	}
	g := &ontology.Graph{
		Conflicts:    []ontology.Conflict{c},
		Requirements: []ontology.Requirement{rejectedReq("R-a"), rejectedReq("R-b")},
	}
	fs := ReflectAllMembersRejected(g)
	if len(fs) != 1 {
		t.Fatalf("DETECTED conflict all-REJECTED should fire, got %d", len(fs))
	}
	if fs[0].Target != "C-ghost" {
		t.Errorf("target: got %q, want C-ghost", fs[0].Target)
	}
	if !fs[0].Advisory {
		t.Error("finding should be advisory")
	}
	if !strings.Contains(fs[0].Imperative, "['R-a', 'R-b']") {
		t.Errorf("imperative missing member list repr: %q", fs[0].Imperative)
	}
}

func TestReflectAllMembersRejected_DoesNotFireDecided(t *testing.T) {
	t.Parallel()
	c := ontology.Conflict{
		ID:        "C-decided",
		Lifecycle: "DECIDED(rationale)",
		Members:   []string{"R-a", "R-b"},
	}
	g := &ontology.Graph{
		Conflicts:    []ontology.Conflict{c},
		Requirements: []ontology.Requirement{rejectedReq("R-a"), rejectedReq("R-b")},
	}
	if fs := ReflectAllMembersRejected(g); len(fs) != 0 {
		t.Errorf("DECIDED conflict should be skipped, got %d", len(fs))
	}
}

func TestReflectAllMembersRejected_DoesNotFireRevisit(t *testing.T) {
	t.Parallel()
	c := ontology.Conflict{
		ID:        "C-revisit",
		Lifecycle: "REVISIT_WHEN(condition)",
		Members:   []string{"R-a", "R-b"},
	}
	g := &ontology.Graph{
		Conflicts:    []ontology.Conflict{c},
		Requirements: []ontology.Requirement{rejectedReq("R-a"), rejectedReq("R-b")},
	}
	if fs := ReflectAllMembersRejected(g); len(fs) != 0 {
		t.Errorf("REVISIT_WHEN conflict should be skipped, got %d", len(fs))
	}
}

func TestReflectAllMembersRejected_DoesNotFireNonRejected(t *testing.T) {
	t.Parallel()
	c := ontology.Conflict{
		ID:        "C-mixed",
		Lifecycle: ontology.ConflictDETECTED,
		Members:   []string{"R-rej", "R-set"},
	}
	g := &ontology.Graph{
		Conflicts:    []ontology.Conflict{c},
		Requirements: []ontology.Requirement{rejectedReq("R-rej"), settledReq("R-set")},
	}
	if fs := ReflectAllMembersRejected(g); len(fs) != 0 {
		t.Errorf("mixed statuses should not fire, got %d", len(fs))
	}
}

func TestAllFindings_OrderFollowsRegistry(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			settledReq("R-1"), settledReq("R-2"), settledReq("R-3"),
			settledReq("R-4"), settledReq("R-5"), settledReq("R-6"),
			settledReq("R-7"),
		},
	}
	fs := AllFindings(g)
	if len(fs) != 1 {
		t.Fatalf("expected only unenforced-settled finding, got %d", len(fs))
	}
	if fs[0].Condition != "reflect_unenforced_settled" {
		t.Errorf("expected reflect_unenforced_settled, got %q", fs[0].Condition)
	}
}

func TestPyRepr_SimpleString(t *testing.T) {
	t.Parallel()
	got := pyRepr("hello world")
	want := "'hello world'"
	if got != want {
		t.Errorf("pyRepr(hello world): got %q, want %q", got, want)
	}
}

func TestPyRepr_ContainsSingleQuote(t *testing.T) {
	t.Parallel()
	got := pyRepr("agent's code")
	want := `"agent's code"`
	if got != want {
		t.Errorf("pyRepr(agent's code): got %q, want %q", got, want)
	}
}

func TestPyRepr_ContainsBothQuotes(t *testing.T) {
	t.Parallel()
	got := pyRepr(`both ' and " here`)
	want := `'both \' and " here'`
	if got != want {
		t.Errorf("pyRepr(both quotes): got %q, want %q", got, want)
	}
}

func TestPyListRepr(t *testing.T) {
	t.Parallel()
	got := pyListRepr([]string{"R-a", "R-b"})
	want := "['R-a', 'R-b']"
	if got != want {
		t.Errorf("pyListRepr: got %q, want %q", got, want)
	}
}

func TestPyListRepr_Empty(t *testing.T) {
	t.Parallel()
	got := pyListRepr([]string{})
	want := "[]"
	if got != want {
		t.Errorf("pyListRepr(empty): got %q, want %q", got, want)
	}
}
