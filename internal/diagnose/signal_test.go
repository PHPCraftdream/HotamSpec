package diagnose

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func TestDiagnoseSignals_EmptyGraph(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	signals := DiagnoseSignals(g)
	if len(signals) != 0 {
		t.Errorf("empty graph should have 0 signals, got %d", len(signals))
	}
}

func TestTopAction_EmptyGraph(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	got := TopAction(g)
	want := "none — graph clean"
	if got != want {
		t.Errorf("TopAction(empty): got %q, want %q", got, want)
	}
}

func TestDiagnoseSignals_ReflectionBeatsStructure(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			settledReq("R-1"), settledReq("R-2"), settledReq("R-3"),
			settledReq("R-4"), settledReq("R-5"), settledReq("R-6"),
			settledReq("R-7"),
		},
		Conflicts: []ontology.Conflict{{
			ID:        "C-nosteward",
			Axis:      "cost-vs-flexibility",
			Context:   "scenario",
			Lifecycle: ontology.ConflictDETECTED,
			Members:   []string{"R-1", "R-2"},
		}},
	}
	signals := DiagnoseSignals(g)
	if len(signals) == 0 {
		t.Fatal("expected signals, got none")
	}
	if signals[0].Priority != PReflection {
		t.Errorf("top priority: got P%d, want P%d (REFLECTION)", signals[0].Priority, PReflection)
	}
}

func TestDiagnoseSignals_PriorityOrder(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-dead", Status: ontology.AssumptionDEAD, Statement: "dead claim"},
		},
		Requirements: []ontology.Requirement{
			{
				ID:          "R-drift",
				Status:      ontology.StatusSETTLED,
				Enforcement: ontology.EnforcementPROSE,
				Assumptions: []string{"A-dead"},
			},
			{
				ID:     "R-open",
				Owner:  "owner",
				Status: "OPEN(what now?)",
			},
		},
		Conflicts: []ontology.Conflict{{
			ID:        "C-stalled",
			Axis:      "cost-vs-flexibility",
			Context:   "scenario",
			Steward:   "steward",
			Lifecycle: ontology.ConflictDETECTED,
			Members:   []string{"R-drift", "R-open"},
		}},
	}
	signals := DiagnoseSignals(g)
	for i := 1; i < len(signals); i++ {
		if signals[i].Priority < signals[i-1].Priority {
			t.Errorf("signal %d (P%d) out of order after signal %d (P%d)",
				i, signals[i].Priority, i-1, signals[i-1].Priority)
		}
	}
}

func TestDiagnoseSignals_StableSortByKey(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{
			{
				ID:        "C-zeta",
				Axis:      "cost-vs-flexibility",
				Context:   "zeta",
				Steward:   "steward",
				Lifecycle: ontology.ConflictDETECTED,
				Members:   []string{"R-1", "R-2"},
			},
			{
				ID:        "C-alpha",
				Axis:      "cost-vs-flexibility",
				Context:   "alpha",
				Steward:   "steward",
				Lifecycle: ontology.ConflictACKNOWLEDGED,
				Members:   []string{"R-1", "R-2"},
			},
		},
		Requirements: []ontology.Requirement{
			settledReq("R-1"), settledReq("R-2"),
		},
	}
	signals := DiagnoseSignals(g)
	if len(signals) < 2 {
		t.Fatal("expected at least 2 signals")
	}
	var conflictSignals []Signal
	for _, s := range signals {
		if s.Priority == PConflictStalled {
			conflictSignals = append(conflictSignals, s)
		}
	}
	if len(conflictSignals) != 2 {
		t.Fatalf("expected 2 CONFLICT_STALLED signals, got %d", len(conflictSignals))
	}
	if conflictSignals[0].Target != "C-alpha" {
		t.Errorf("expected C-alpha first (sorted by target), got %q", conflictSignals[0].Target)
	}
	if conflictSignals[1].Target != "C-zeta" {
		t.Errorf("expected C-zeta second, got %q", conflictSignals[1].Target)
	}
}

func TestDiagnoseSignals_AdvisoryRoutedLowest(t *testing.T) {
	t.Parallel()
	r := rejectedReq("R-old")
	r.Why = "REJECTED — REPLACES R-new"
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{r},
		Conflicts: []ontology.Conflict{{
			ID:        "C-stalled",
			Axis:      "cost-vs-flexibility",
			Context:   "scenario",
			Steward:   "steward",
			Lifecycle: ontology.ConflictDETECTED,
			Members:   []string{"R-old"},
		}},
	}
	signals := DiagnoseSignals(g)
	var advisory, stalled int
	for _, s := range signals {
		if s.Priority == PAdvisory {
			advisory++
		}
		if s.Priority == PConflictStalled {
			stalled++
		}
	}
	if advisory == 0 {
		t.Error("advisory finding should route to PAdvisory band")
	}
	if stalled == 0 {
		t.Error("conflict stalled should produce PConflictStalled signal")
	}
	for i := 1; i < len(signals); i++ {
		if signals[i].Priority < signals[i-1].Priority {
			t.Errorf("signal %d (P%d) out of order after signal %d (P%d)",
				i, signals[i].Priority, i-1, signals[i-1].Priority)
			break
		}
	}
	if signals[len(signals)-1].Priority != PAdvisory {
		t.Errorf("last signal should be PAdvisory, got P%d", signals[len(signals)-1].Priority)
	}
}

func TestDiagnoseSignals_P2DriftFalloutReqsAndConflicts(t *testing.T) {
	t.Parallel()
	sharedAssumption := "A-dead"
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: sharedAssumption, Status: ontology.AssumptionDEAD, Statement: "dead claim"},
		},
		Requirements: []ontology.Requirement{
			{
				ID:          "R-rests",
				Status:      ontology.StatusSETTLED,
				Enforcement: ontology.EnforcementPROSE,
				Assumptions: []string{sharedAssumption},
			},
		},
		Conflicts: []ontology.Conflict{{
			ID:               "C-rests",
			Axis:             "cost-vs-flexibility",
			Context:          "scenario",
			Steward:          "steward",
			Lifecycle:        "DECIDED(r)",
			SharedAssumption: &sharedAssumption,
			Members:          []string{"R-rests"},
		}},
	}
	signals := DiagnoseSignals(g)
	var driftCount int
	for _, s := range signals {
		if s.Priority == PDriftFallout {
			driftCount++
		}
	}
	if driftCount != 2 {
		t.Errorf("expected 2 DRIFT_FALLOUT signals (req + conflict), got %d", driftCount)
	}
}

func TestDiagnoseSignals_P4OpenQuestionExtracted(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-q", Owner: "owner-q", Status: "OPEN(which path?)"},
		},
	}
	signals := DiagnoseSignals(g)
	var openSignals []Signal
	for _, s := range signals {
		if s.Priority == POpenItem && s.Target == "R-q" {
			openSignals = append(openSignals, s)
		}
	}
	if len(openSignals) != 1 {
		t.Fatalf("expected 1 OPEN_ITEM signal for R-q, got %d (total %d)", len(openSignals), len(signals))
	}
	if openSignals[0].Target != "R-q" {
		t.Errorf("target: got %q, want R-q", openSignals[0].Target)
	}
}

func TestDiagnoseSignals_P4OpenNoQuestionFallback(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-noq", Owner: "owner", Status: "OPEN()"},
		},
	}
	signals := DiagnoseSignals(g)
	for _, s := range signals {
		if s.Priority == POpenItem && s.Target == "R-noq" {
			if s.Message != "OPEN requirement 'R-noq' (owner 'owner') awaits a decision: (no question stated)" {
				t.Errorf("fallback question message: %q", s.Message)
			}
			return
		}
	}
	t.Error("expected an OPEN_ITEM signal for R-noq")
}

func TestDiagnoseSignals_P4HeldVariants(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{{
			ID:        "C-held",
			Axis:      "cost-vs-flexibility",
			Context:   "scenario",
			Steward:   "steward",
			Lifecycle: "HELD(reason)",
			DecidedBy: "steward",
			Members:   []string{"R-1", "R-2"},
			Variants: []ontology.Variant{
				{ID: "V-a", Behavior: "a"},
				{ID: "V-b", Behavior: "b"},
			},
		}},
		Requirements: []ontology.Requirement{settledReq("R-1"), settledReq("R-2")},
	}
	signals := DiagnoseSignals(g)
	var variantCount int
	for _, s := range signals {
		if s.Priority == POpenItem {
			variantCount++
		}
	}
	if variantCount != 2 {
		t.Errorf("expected 2 OPEN_ITEM variant signals, got %d", variantCount)
	}
}

func TestDiagnoseSignals_P4UncertainAgingHighFanOut(t *testing.T) {
	t.Parallel()
	reqs := make([]ontology.Requirement, UncertainAgingMinDependents)
	for i := range reqs {
		reqs[i] = ontology.Requirement{
			ID:          "R-" + string(rune('a'+i)),
			Status:      ontology.StatusSETTLED,
			Enforcement: ontology.EnforcementPROSE,
			Assumptions: []string{"A-unc"},
		}
	}
	g := &ontology.Graph{
		Assumptions: []ontology.Assumption{
			{ID: "A-unc", Status: ontology.AssumptionUNCERTAIN, Statement: "not sure"},
		},
		Requirements: reqs,
	}
	signals := DiagnoseSignals(g)
	found := false
	for _, s := range signals {
		if s.Priority == POpenItem && s.Target == "A-unc" {
			found = true
		}
	}
	if !found {
		t.Error("expected UNCERTAIN-aging P4 signal for high-fan-out assumption")
	}
}

func TestDiagnoseSignals_P4UncertainAgingLowFanOut(t *testing.T) {
	t.Parallel()
	reqs := make([]ontology.Requirement, UncertainAgingMinDependents-1)
	for i := range reqs {
		reqs[i] = ontology.Requirement{
			ID:          "R-" + string(rune('a'+i)),
			Status:      ontology.StatusSETTLED,
			Enforcement: ontology.EnforcementPROSE,
			Assumptions: []string{"A-unc"},
		}
	}
	g := &ontology.Graph{
		Assumptions:  []ontology.Assumption{{ID: "A-unc", Status: ontology.AssumptionUNCERTAIN}},
		Requirements: reqs,
	}
	signals := DiagnoseSignals(g)
	for _, s := range signals {
		if s.Priority == POpenItem && s.Target == "A-unc" {
			t.Error("low fan-out UNCERTAIN should not produce a P4 OPEN_ITEM signal")
		}
	}
}

func TestTopAction_ReturnsTopSignalMessage(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{{
			ID:        "C-1",
			Axis:      "cost-vs-flexibility",
			Context:   "scenario",
			Steward:   "steward",
			Lifecycle: ontology.ConflictDETECTED,
			Members:   []string{"R-1", "R-2"},
		}},
		Requirements: []ontology.Requirement{settledReq("R-1"), settledReq("R-2")},
	}
	ta := TopAction(g)
	if ta == "" {
		t.Error("TopAction should return non-empty message for non-clean graph")
	}
	if ta == "none — graph clean" {
		t.Error("TopAction should not return 'graph clean' when signals exist")
	}
}

func TestDiagnoseSignals_AllSignalsUseDiagnoseSource(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{{
			ID:        "C-1",
			Axis:      "cost-vs-flexibility",
			Context:   "scenario",
			Steward:   "steward",
			Lifecycle: ontology.ConflictDETECTED,
			Members:   []string{"R-1", "R-2"},
		}},
		Requirements: []ontology.Requirement{settledReq("R-1"), settledReq("R-2")},
	}
	signals := DiagnoseSignals(g)
	for _, s := range signals {
		if s.Source != "diagnose" {
			t.Errorf("source: got %q, want diagnose", s.Source)
		}
	}
}

func TestPriorityConstants(t *testing.T) {
	t.Parallel()
	if PReflection != 0 {
		t.Errorf("PReflection: got %d, want 0", PReflection)
	}
	if PStructure != 1 {
		t.Errorf("PStructure: got %d, want 1", PStructure)
	}
	if PDriftFallout != 2 {
		t.Errorf("PDriftFallout: got %d, want 2", PDriftFallout)
	}
	if PConflictStalled != 3 {
		t.Errorf("PConflictStalled: got %d, want 3", PConflictStalled)
	}
	if POpenItem != 4 {
		t.Errorf("POpenItem: got %d, want 4", POpenItem)
	}
	if PLatentConnector != 5 {
		t.Errorf("PLatentConnector: got %d, want 5", PLatentConnector)
	}
	if PRuntime != 6 {
		t.Errorf("PRuntime: got %d, want 6", PRuntime)
	}
	if PAdvisory != 7 {
		t.Errorf("PAdvisory: got %d, want 7", PAdvisory)
	}
}
