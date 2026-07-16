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

// TestReflectUnenforcedSettled_FeatureBlockedDoesNotTripP0 is the core
// regression guard for the burn-down split: a graph whose closeable debt is
// overwhelmingly FEATURE-BLOCKED (BlockedOn set) but has FEW closeable-now items
// must NOT trip the P0 enforcement-gradient signal. Before the split, all 7
// would have counted as one undifferentiated band and fired at >5; now only the
// 1 closeable-now item counts, which is <= 5, so the P0 signal stays silent.
// The feature-blocked items are surfaced by the separate Advisory finding, not
// here.
func TestReflectUnenforcedSettled_FeatureBlockedDoesNotTripP0(t *testing.T) {
	t.Parallel()
	reqs := make([]ontology.Requirement, 0, 8)
	// 1 closeable-now item (no blocker) — below the >5 P0 threshold.
	reqs = append(reqs, settledReq("R-now"))
	// 7 feature-blocked items — must NOT count toward the P0 signal.
	for i := 0; i < 7; i++ {
		r := settledReq("R-blk-" + string(rune('a'+i)))
		r.BlockedOn = "blocked on a Planned tool"
		reqs = append(reqs, r)
	}
	g := &ontology.Graph{Requirements: reqs}
	if fs := ReflectUnenforcedSettled(g); len(fs) != 0 {
		t.Errorf("feature-blocked debt must NOT trip the P0 signal (only 1 closeable-now <= 5), got %d findings", len(fs))
	}
}

func TestReflectUnenforcedSettled_CloseableNowCounts(t *testing.T) {
	t.Parallel()
	// 6 closeable-now items (no blocker) — just over the >5 threshold.
	reqs := make([]ontology.Requirement, 0, 13)
	for i := 0; i < 6; i++ {
		reqs = append(reqs, settledReq("R-now-"+string(rune('a'+i))))
	}
	// 7 feature-blocked items mixed in — must not change the closeable-now count.
	for i := 0; i < 7; i++ {
		r := settledReq("R-blk-" + string(rune('a'+i)))
		r.BlockedOn = "blocked on a Planned tool"
		reqs = append(reqs, r)
	}
	g := &ontology.Graph{Requirements: reqs}
	fs := ReflectUnenforcedSettled(g)
	if len(fs) != 1 {
		t.Fatalf("6 closeable-now should fire exactly once, got %d", len(fs))
	}
	if !strings.Contains(fs[0].Imperative, "6 SETTLED requirements are closeable now") {
		t.Errorf("P0 message should name the closeable-now count of 6, got: %q", fs[0].Imperative)
	}
}

func TestReflectFeatureBlockedDebt_FiresAdvisory(t *testing.T) {
	t.Parallel()
	reqs := make([]ontology.Requirement, 0, 8)
	reqs = append(reqs, settledReq("R-now")) // closeable-now, not feature-blocked
	for i := 0; i < 7; i++ {
		r := settledReq("R-blk-" + string(rune('a'+i)))
		r.BlockedOn = "blocked on a Planned tool"
		reqs = append(reqs, r)
	}
	g := &ontology.Graph{Requirements: reqs}
	fs := ReflectFeatureBlockedDebt(g)
	if len(fs) != 1 {
		t.Fatalf("7 feature-blocked items should fire once, got %d", len(fs))
	}
	if fs[0].Condition != "reflect_feature_blocked_debt" {
		t.Errorf("condition: got %q, want reflect_feature_blocked_debt", fs[0].Condition)
	}
	if !fs[0].Advisory {
		t.Error("feature-blocked finding must be Advisory (routes to P7, not P0)")
	}
	if !strings.Contains(fs[0].Imperative, "7 SETTLED requirements are feature-blocked debt") {
		t.Errorf("advisory message should name the count of 7, got: %q", fs[0].Imperative)
	}
	if !strings.Contains(fs[0].Imperative, "c1-roadmap-debt-triage.md") {
		t.Errorf("advisory should point at the triage doc, got: %q", fs[0].Imperative)
	}
}

func TestReflectFeatureBlockedDebt_ZeroDoesNotFire(t *testing.T) {
	t.Parallel()
	// Only closeable-now items — no feature-blocked debt at all.
	reqs := make([]ontology.Requirement, 7)
	for i := range reqs {
		reqs[i] = settledReq("R-" + string(rune('a'+i)))
	}
	g := &ontology.Graph{Requirements: reqs}
	if fs := ReflectFeatureBlockedDebt(g); len(fs) != 0 {
		t.Errorf("zero feature-blocked items should not fire, got %d", len(fs))
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

// === ReflectOrphanEntityType (task #203: advisory "orphan detail") ===

func orphanTestProcess(id string, drivesEntities ...string) ontology.Process {
	return ontology.Process{
		ID:             id,
		Lifecycle:      ontology.ProcessLifecycle,
		DrivesEntities: drivesEntities,
	}
}

func orphanTestEntityType(slug string) ontology.EntityType {
	return ontology.EntityType{Slug: slug}
}

func TestReflectOrphanEntityType_NoProcessNodesStaysSilent(t *testing.T) {
	t.Parallel()
	// Zero Process nodes at all: the Process aspect was never opted into, so
	// "orphaned from a Process" is not a meaningful question for this domain
	// — the signal must not fire, no matter how many undriven EntityTypes
	// exist (this is exactly hotam-dev's real shape: 1 EntityType, 0
	// Process nodes).
	g := &ontology.Graph{
		EntityTypes: []ontology.EntityType{orphanTestEntityType("wave")},
	}
	if fs := ReflectOrphanEntityType(g); len(fs) != 0 {
		t.Fatalf("expected silence when g.processes is empty, got %v", fs)
	}
}

func TestReflectOrphanEntityType_NoEntityTypesStaysSilent(t *testing.T) {
	t.Parallel()
	// A Process aspect with zero EntityTypes declared (this is
	// hotam-spec-self's real shape: PR-closed-loop exists, but the domain
	// declares no EntityType at all) — vacuously nothing can be orphaned.
	g := &ontology.Graph{
		Processes: []ontology.Process{orphanTestProcess("PR-1")},
	}
	if fs := ReflectOrphanEntityType(g); len(fs) != 0 {
		t.Fatalf("expected silence when g.entity_types is empty, got %v", fs)
	}
}

func TestReflectOrphanEntityType_AllDrivenStaysSilent(t *testing.T) {
	t.Parallel()
	// A consistent domain: every EntityType is named in SOME Process's
	// drives_entities — the honest "no orphan" case must not fire.
	g := &ontology.Graph{
		Processes: []ontology.Process{
			orphanTestProcess("PR-1", "thing-a"),
			orphanTestProcess("PR-2", "thing-b"),
		},
		EntityTypes: []ontology.EntityType{
			orphanTestEntityType("thing-a"),
			orphanTestEntityType("thing-b"),
		},
	}
	if fs := ReflectOrphanEntityType(g); len(fs) != 0 {
		t.Fatalf("expected silence when every EntityType is driven, got %v", fs)
	}
}

func TestReflectOrphanEntityType_FiresOnUndrivenEntityType(t *testing.T) {
	t.Parallel()
	// A synthetic domain with a real orphan: >=1 Process node exists, but
	// one EntityType is not named by any Process.drives_entities.
	g := &ontology.Graph{
		Processes: []ontology.Process{
			orphanTestProcess("PR-1", "thing-a"),
		},
		EntityTypes: []ontology.EntityType{
			orphanTestEntityType("thing-a"),
			orphanTestEntityType("thing-orphan"),
		},
	}
	fs := ReflectOrphanEntityType(g)
	if len(fs) != 1 {
		t.Fatalf("expected exactly 1 orphan finding, got %d: %v", len(fs), fs)
	}
	if fs[0].Condition != "reflect_orphan_entity_type" {
		t.Errorf("condition: got %q, want reflect_orphan_entity_type", fs[0].Condition)
	}
	if fs[0].Target != "thing-orphan" {
		t.Errorf("target: got %q, want thing-orphan", fs[0].Target)
	}
	if !fs[0].Advisory {
		t.Error("orphan-entity-type finding must be Advisory (never a blocking gate)")
	}
	if !strings.Contains(fs[0].Imperative, "thing-orphan") {
		t.Errorf("imperative should name the orphan slug, got: %q", fs[0].Imperative)
	}
}

func TestReflectOrphanEntityType_MultipleOrphansSortedDeterministic(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Processes: []ontology.Process{orphanTestProcess("PR-1")},
		EntityTypes: []ontology.EntityType{
			orphanTestEntityType("zzz-orphan"),
			orphanTestEntityType("aaa-orphan"),
		},
	}
	fs := ReflectOrphanEntityType(g)
	if len(fs) != 2 {
		t.Fatalf("expected 2 orphan findings, got %d: %v", len(fs), fs)
	}
	if fs[0].Target != "aaa-orphan" || fs[1].Target != "zzz-orphan" {
		t.Fatalf("expected deterministic sorted order aaa-orphan, zzz-orphan; got %s, %s", fs[0].Target, fs[1].Target)
	}
}

func TestReflectOrphanEntityType_RoutesToPAdvisoryInDiagnoseSignals(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Processes:   []ontology.Process{orphanTestProcess("PR-1")},
		EntityTypes: []ontology.EntityType{orphanTestEntityType("thing-orphan")},
	}
	signals := DiagnoseSignals(g, "2026-07-16")
	found := false
	for _, s := range signals {
		if s.Check == "reflect_orphan_entity_type" {
			found = true
			if s.Priority != PAdvisory {
				t.Errorf("orphan-entity-type signal priority = %d, want PAdvisory (%d)", s.Priority, PAdvisory)
			}
		}
	}
	if !found {
		t.Fatal("expected DiagnoseSignals to surface the orphan-entity-type advisory signal")
	}
}
