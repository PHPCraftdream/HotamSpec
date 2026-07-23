package graphfacts

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func reqWithGates(id string, gates ...ontology.GateSignoff) ontology.Requirement {
	return ontology.Requirement{ID: id, GateSignoffs: gates}
}

func TestGateSignoffTally_DedupsLastEntryPerRequirementPerStage(t *testing.T) {
	t.Parallel()
	order := []string{"P-G0", "P-G1"}
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			// R-1: DEFERRED at P-G1, later superseded by SIGNED at P-G1 —
			// must count ONCE, as SIGNED (the last entry), never as both.
			reqWithGates("R-1",
				ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateDeferred, DeferredReason: "blocked"},
				ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateSigned},
			),
			// R-2: SIGNED at P-G1, only entry.
			reqWithGates("R-2", ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateSigned}),
			// R-3: DEFERRED at P-G1, only entry (still deferred, never superseded).
			reqWithGates("R-3", ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateDeferred, DeferredReason: "blocked"}),
			// R-4: signoff at a DIFFERENT stage (P-G0) — must not count toward P-G1.
			reqWithGates("R-4", ontology.GateSignoff{Stage: "P-G0", State: ontology.GateSignoffStateSigned}),
			// R-5: no gate signoffs at all.
			reqWithGates("R-5"),
		},
	}
	tally := GateSignoffTally(g, order, "P-G1", "")
	if tally.Signed != 2 {
		t.Errorf("Signed = %d, want 2 (R-1's LAST entry + R-2)", tally.Signed)
	}
	if tally.Deferred != 1 {
		t.Errorf("Deferred = %d, want 1 (R-3 only — R-1's DEFERRED was superseded)", tally.Deferred)
	}
}

func TestGateSignoffTally_UnknownStageTalliesZero(t *testing.T) {
	t.Parallel()
	order := []string{"P-G0"}
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			reqWithGates("R-1", ontology.GateSignoff{Stage: "P-G0", State: ontology.GateSignoffStateSigned}),
		},
	}
	tally := GateSignoffTally(g, order, "P-G99", "")
	if tally.Signed != 0 || tally.Deferred != 0 {
		t.Errorf("expected (0,0) for an undeclared stage, got %+v", tally)
	}
}

func TestGateFrontier_ReturnsFurthestTouchedStage(t *testing.T) {
	t.Parallel()
	order := []string{"P-G0", "P-G1", "P-G2"}
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			reqWithGates("R-1", ontology.GateSignoff{Stage: "P-G0", State: ontology.GateSignoffStateSigned}),
			reqWithGates("R-2", ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateSigned}),
			reqWithGates("R-3", ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateDeferred, DeferredReason: "x"}),
		},
	}
	stage, tally, ok := GateFrontier(g, order)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if stage != "P-G1" {
		t.Errorf("stage = %q, want P-G1 (furthest touched)", stage)
	}
	if tally.Signed != 1 || tally.Deferred != 1 {
		t.Errorf("tally = %+v, want {Signed:1 Deferred:1}", tally)
	}
}

func TestGateFrontier_NotOkWhenNoSignoffs(t *testing.T) {
	t.Parallel()
	order := []string{"P-G0", "P-G1"}
	g := &ontology.Graph{Requirements: []ontology.Requirement{reqWithGates("R-1")}}
	_, _, ok := GateFrontier(g, order)
	if ok {
		t.Error("expected ok=false when no Requirement carries a matching GateSignoff")
	}
}

func TestGateFrontier_NotOkWhenOrderNil(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			reqWithGates("R-1", ontology.GateSignoff{Stage: "P-G0", State: ontology.GateSignoffStateSigned}),
		},
	}
	_, _, ok := GateFrontier(g, nil)
	if ok {
		t.Error("expected ok=false when order is nil (no declared gate_stage_order)")
	}
}

// TestGateSignoffTally_RunFilteredDedupsWithinRunOnly proves the run
// parameter both (a) restricts the tally to ONE pipeline_run's signoffs and
// (b) still applies the last-entry dedup rule WITHIN that run — never
// leaking a later entry from a DIFFERENT run into the dedup computation.
func TestGateSignoffTally_RunFilteredDedupsWithinRunOnly(t *testing.T) {
	t.Parallel()
	order := []string{"P-G1"}
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			// R-1: DEFERRED then SIGNED, both in run "run-a" — dedup within
			// run-a must resolve to SIGNED (the last entry in that run).
			reqWithGates("R-1",
				ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateDeferred, DeferredReason: "x", PipelineRun: "run-a"},
				ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-a"},
			),
			// R-2: SIGNED in run-a, but DEFERRED in a LATER run-b entry — when
			// filtering to run-a only, run-b's entry must be invisible, so
			// R-2 tallies SIGNED (run-a's own last/only entry), not DEFERRED.
			reqWithGates("R-2",
				ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-a"},
				ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateDeferred, DeferredReason: "y", PipelineRun: "run-b"},
			),
			// R-3: only a run-b entry — invisible when filtering to run-a.
			reqWithGates("R-3", ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-b"}),
		},
	}

	tallyA := GateSignoffTally(g, order, "P-G1", "run-a")
	if tallyA.Signed != 2 || tallyA.Deferred != 0 {
		t.Errorf("run-a tally = %+v, want {Signed:2 Deferred:0} (R-1 dedup'd to SIGNED, R-2's run-a entry is SIGNED, R-3 invisible)", tallyA)
	}

	tallyB := GateSignoffTally(g, order, "P-G1", "run-b")
	if tallyB.Signed != 1 || tallyB.Deferred != 1 {
		t.Errorf("run-b tally = %+v, want {Signed:1 Deferred:1} (R-2's run-b entry DEFERRED, R-3's SIGNED, R-1 invisible)", tallyB)
	}

	// Empty run string still tallies across ALL runs (backward compat) —
	// R-1 dedups to SIGNED across its own two run-a entries; R-2 dedups to
	// its LAST appended entry overall (run-b's DEFERRED, since it was
	// appended after the run-a entry); R-3 is SIGNED.
	tallyAll := GateSignoffTally(g, order, "P-G1", "")
	if tallyAll.Signed != 2 || tallyAll.Deferred != 1 {
		t.Errorf("all-runs tally = %+v, want {Signed:2 Deferred:1}", tallyAll)
	}
}

// TestPipelineRunsAtStage_ReturnsDistinctSortedRuns proves the distinct-run
// reader the multi-run ambiguity guard relies on.
func TestPipelineRunsAtStage_ReturnsDistinctSortedRuns(t *testing.T) {
	t.Parallel()
	order := []string{"P-G1"}
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			reqWithGates("R-1", ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-b"}),
			reqWithGates("R-2", ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-a"}),
			reqWithGates("R-3", ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-a"}),
			// Different stage — must not appear.
			reqWithGates("R-4", ontology.GateSignoff{Stage: "P-G0", State: ontology.GateSignoffStateSigned, PipelineRun: "run-c"}),
		},
	}
	runs := PipelineRunsAtStage(g, order, "P-G1")
	if len(runs) != 2 || runs[0] != "run-a" || runs[1] != "run-b" {
		t.Errorf("PipelineRunsAtStage = %v, want [run-a run-b] (sorted, deduped, P-G0's run-c excluded)", runs)
	}
}

// TestPipelineRunsAtStage_SingleRunReturnsOne proves the common, unambiguous
// case: one distinct run recorded at the stage.
func TestPipelineRunsAtStage_SingleRunReturnsOne(t *testing.T) {
	t.Parallel()
	order := []string{"P-G1"}
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			reqWithGates("R-1", ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateSigned, PipelineRun: "run-a"}),
			reqWithGates("R-2", ontology.GateSignoff{Stage: "P-G1", State: ontology.GateSignoffStateDeferred, DeferredReason: "x", PipelineRun: "run-a"}),
		},
	}
	runs := PipelineRunsAtStage(g, order, "P-G1")
	if len(runs) != 1 || runs[0] != "run-a" {
		t.Errorf("PipelineRunsAtStage = %v, want [run-a]", runs)
	}
}

// TestCohortCount_BasicFilter proves CohortCount is a trivial counted
// filter: it counts exactly the Requirements for which member(r) is true.
func TestCohortCount_BasicFilter(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", Status: ontology.StatusSETTLED},
			{ID: "R-2", Status: ontology.StatusSETTLED},
			{ID: "R-3", Status: ontology.StatusDRAFT},
			{ID: "R-4", Status: ontology.StatusREJECTED},
		},
	}
	count := CohortCount(g, func(r ontology.Requirement) bool { return r.Status == ontology.StatusSETTLED })
	if count != 2 {
		t.Errorf("CohortCount(SETTLED) = %d, want 2", count)
	}
}

// TestCohortCount_EmptyGraphIsZero proves the trivial empty-input case.
func TestCohortCount_EmptyGraphIsZero(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	count := CohortCount(g, func(r ontology.Requirement) bool { return true })
	if count != 0 {
		t.Errorf("CohortCount on empty graph = %d, want 0", count)
	}
}

// TestCohortCount_AllMatchOrNoneMatch proves both saturation extremes.
func TestCohortCount_AllMatchOrNoneMatch(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", Status: ontology.StatusSETTLED},
			{ID: "R-2", Status: ontology.StatusSETTLED},
		},
	}
	if count := CohortCount(g, func(r ontology.Requirement) bool { return true }); count != 2 {
		t.Errorf("CohortCount(always-true) = %d, want 2", count)
	}
	if count := CohortCount(g, func(r ontology.Requirement) bool { return false }); count != 0 {
		t.Errorf("CohortCount(always-false) = %d, want 0", count)
	}
}

func TestConflictLifecycleTally(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{
			{ID: "C-1", Lifecycle: ontology.ConflictDECIDEDPrefix},
			{ID: "C-2", Lifecycle: ontology.ConflictDECIDEDPrefix + "_variant_x"},
			{ID: "C-3", Lifecycle: ontology.ConflictHELDPrefix},
			{ID: "C-4", Lifecycle: ontology.ConflictDETECTED},
			{ID: "C-5", Lifecycle: ontology.ConflictACKNOWLEDGED},
		},
	}

	if count, total, err := ConflictLifecycleTally(g, "DECIDED"); err != nil || count != 2 || total != 5 {
		t.Errorf("DECIDED: got (%d,%d,%v), want (2,5,nil)", count, total, err)
	}
	if count, total, err := ConflictLifecycleTally(g, "HELD"); err != nil || count != 1 || total != 5 {
		t.Errorf("HELD: got (%d,%d,%v), want (1,5,nil)", count, total, err)
	}
	if count, total, err := ConflictLifecycleTally(g, "UNRESOLVED"); err != nil || count != 2 || total != 5 {
		t.Errorf("UNRESOLVED: got (%d,%d,%v), want (2,5,nil)", count, total, err)
	}
}

func TestConflictLifecycleTally_UnknownClassFailsClosed(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Conflicts: []ontology.Conflict{{ID: "C-1", Lifecycle: ontology.ConflictDETECTED}}}
	count, total, err := ConflictLifecycleTally(g, "BOGUS")
	if err == nil {
		t.Fatal("expected a non-nil error for an unknown lifecycle_class")
	}
	if count != 0 || total != 1 {
		t.Errorf("got (%d,%d), want (0,1) alongside the error", count, total)
	}
}

func TestRequirementStatusTally_StatusOnly(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementENFORCED},
			{ID: "R-2", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementPROSE},
			{ID: "R-3", Status: ontology.StatusDRAFT, Enforcement: ontology.EnforcementPROSE},
		},
	}
	count, total, err := RequirementStatusTally(g, ontology.StatusSETTLED, "")
	if err != nil || count != 2 || total != 3 {
		t.Errorf("got (%d,%d,%v), want (2,3,nil)", count, total, err)
	}
}

func TestRequirementStatusTally_StatusAndEnforcement(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementENFORCED},
			{ID: "R-2", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementPROSE},
			{ID: "R-3", Status: ontology.StatusDRAFT, Enforcement: ontology.EnforcementENFORCED},
		},
	}
	count, total, err := RequirementStatusTally(g, ontology.StatusSETTLED, ontology.EnforcementENFORCED)
	if err != nil || count != 1 || total != 3 {
		t.Errorf("got (%d,%d,%v), want (1,3,nil)", count, total, err)
	}
}

func TestRequirementStatusTally_OpenPrefixMatch(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-1", Status: "OPEN_BLOCKED"},
			{ID: "R-2", Status: "OPEN"},
			{ID: "R-3", Status: ontology.StatusSETTLED},
		},
	}
	count, total, err := RequirementStatusTally(g, ontology.StatusOPENPrefix, "")
	if err != nil || count != 2 || total != 3 {
		t.Errorf("got (%d,%d,%v), want (2,3,nil) — both OPEN and OPEN_BLOCKED match via IsOpen()", count, total, err)
	}
}

func TestRequirementStatusTally_UnknownStatusFailsClosed(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Requirements: []ontology.Requirement{{ID: "R-1", Status: ontology.StatusSETTLED}}}
	count, total, err := RequirementStatusTally(g, "BOGUS_STATUS", "")
	if err == nil {
		t.Fatal("expected a non-nil error for an unknown status")
	}
	if count != 0 || total != 1 {
		t.Errorf("got (%d,%d), want (0,1) alongside the error", count, total)
	}
}

func TestRequirementStatusTally_UnknownEnforcementFailsClosed(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Requirements: []ontology.Requirement{{ID: "R-1", Status: ontology.StatusSETTLED, Enforcement: ontology.EnforcementENFORCED}}}
	count, total, err := RequirementStatusTally(g, ontology.StatusSETTLED, "BOGUS_ENFORCEMENT")
	if err == nil {
		t.Fatal("expected a non-nil error for an unknown enforcement value")
	}
	if count != 0 || total != 1 {
		t.Errorf("got (%d,%d), want (0,1) alongside the error", count, total)
	}
}
