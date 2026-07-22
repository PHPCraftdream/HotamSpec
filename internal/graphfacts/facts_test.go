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
	tally := GateSignoffTally(g, order, "P-G1")
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
	tally := GateSignoffTally(g, order, "P-G99")
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
