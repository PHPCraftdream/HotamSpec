package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func TestCheckTypedAnchorsVariant_OK(t *testing.T) {
	if vs := runCheck(t, "check_typed_anchors_variant", graphWithConflict(heldConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckTypedAnchorsVariant_FiresOnBadPrefix(t *testing.T) {
	bad := heldConflict()
	bad.Variants = []ontology.Variant{
		{ID: "option-1", Behavior: "first"},
		{ID: "V-2", Behavior: "second"},
	}
	vs := runCheck(t, "check_typed_anchors_variant", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID+":option-1") {
		t.Fatalf("expected violation on %s:option-1, got %v", bad.ID, vs)
	}
}

func TestCheckSignoffChosenVariantResolves_OK(t *testing.T) {
	c := heldConflict()
	cv := "V-fast"
	c.Signoff = &ontology.Signoff{DecidedBy: "outsider", ChosenVariant: cv}
	if vs := runCheck(t, "check_signoff_chosen_variant_resolves", graphWithConflict(c, nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckSignoffChosenVariantResolves_OKOnEmptyChosen(t *testing.T) {
	c := heldConflict()
	c.Signoff = &ontology.Signoff{DecidedBy: "outsider", ChosenVariant: ""}
	if vs := runCheck(t, "check_signoff_chosen_variant_resolves", graphWithConflict(c, nil)); len(vs) != 0 {
		t.Fatalf("empty chosen_variant must not fire, got %v", vs)
	}
}

func TestCheckSignoffChosenVariantResolves_FiresOnUnknown(t *testing.T) {
	c := heldConflict()
	c.Signoff = &ontology.Signoff{DecidedBy: "outsider", ChosenVariant: "V-nonexistent"}
	vs := runCheck(t, "check_signoff_chosen_variant_resolves", graphWithConflict(c, nil))
	if !hasViolationFor(vs, c.ID) {
		t.Fatalf("expected violation on %s, got %v", c.ID, vs)
	}
}

func TestCheckSignoffChosenVariantResolves_SilentOnNilSignoff(t *testing.T) {
	c := heldConflict()
	c.Signoff = nil
	if vs := runCheck(t, "check_signoff_chosen_variant_resolves", graphWithConflict(c, nil)); len(vs) != 0 {
		t.Fatalf("nil signoff must not fire, got %v", vs)
	}
}

func TestCheckDecidedConflictCarriesSignoff_OK(t *testing.T) {
	c := decidedConflict()
	c.Signoff = &ontology.Signoff{DecidedBy: "outsider"}
	if vs := runCheck(t, "check_decided_conflict_carries_signoff", graphWithConflict(c, nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckDecidedConflictCarriesSignoff_OKOnNoSignoff(t *testing.T) {
	c := decidedConflict()
	c.Signoff = nil
	if vs := runCheck(t, "check_decided_conflict_carries_signoff", graphWithConflict(c, nil)); len(vs) != 0 {
		t.Fatalf("missing signoff must not fire (SOFT), got %v", vs)
	}
}

func TestCheckDecidedConflictCarriesSignoff_FiresOnMismatch(t *testing.T) {
	c := decidedConflict()
	c.Signoff = &ontology.Signoff{DecidedBy: "sa"}
	vs := runCheck(t, "check_decided_conflict_carries_signoff", graphWithConflict(c, nil))
	if !hasViolationFor(vs, c.ID) {
		t.Fatalf("expected violation on %s for decided_by mismatch, got %v", c.ID, vs)
	}
}

func TestCheckDecidedConflictCarriesSignoff_OKOnHeld(t *testing.T) {
	c := heldConflict()
	c.Signoff = &ontology.Signoff{DecidedBy: "outsider"}
	if vs := runCheck(t, "check_decided_conflict_carries_signoff", graphWithConflict(c, nil)); len(vs) != 0 {
		t.Fatalf("HELD conflict with matching signoff must not fire, got %v", vs)
	}
}

func TestCheckDecidedConflictCarriesSignoff_SilentOnAcknowledged(t *testing.T) {
	c := baseConflict()
	c.Signoff = &ontology.Signoff{DecidedBy: "someone-else"}
	if vs := runCheck(t, "check_decided_conflict_carries_signoff", graphWithConflict(c, nil)); len(vs) != 0 {
		t.Fatalf("ACKNOWLEDGED conflict must not be checked, got %v", vs)
	}
}

func TestCheckTypedAnchorsRequirement_OK(t *testing.T) {
	g := &ontology.Graph{Requirements: []ontology.Requirement{req("R-1", "sa"), req("R-2", "sb")}}
	if vs := runCheck(t, "check_typed_anchors_requirement", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckTypedAnchorsRequirement_FiresOnBadPrefix(t *testing.T) {
	g := &ontology.Graph{Requirements: []ontology.Requirement{{ID: "foo", Claim: "c", Owner: "sa"}}}
	vs := runCheck(t, "check_typed_anchors_requirement", g)
	if !hasViolationFor(vs, "foo") {
		t.Fatalf("expected violation on foo, got %v", vs)
	}
}

func TestCheckTypedAnchorsAssumption_OK(t *testing.T) {
	g := &ontology.Graph{Assumptions: []ontology.Assumption{{ID: "A-1", Statement: "s", Status: ontology.AssumptionHOLDS, Owner: "sa"}}}
	if vs := runCheck(t, "check_typed_anchors_assumption", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckTypedAnchorsAssumption_FiresOnBadPrefix(t *testing.T) {
	g := &ontology.Graph{Assumptions: []ontology.Assumption{{ID: "assum-x", Statement: "s", Status: ontology.AssumptionHOLDS, Owner: "sa"}}}
	vs := runCheck(t, "check_typed_anchors_assumption", g)
	if !hasViolationFor(vs, "assum-x") {
		t.Fatalf("expected violation on assum-x, got %v", vs)
	}
}

func TestCheckTypedAnchorsConflict_OK(t *testing.T) {
	g := &ontology.Graph{Conflicts: []ontology.Conflict{baseConflict()}}
	if vs := runCheck(t, "check_typed_anchors_conflict", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckTypedAnchorsConflict_FiresOnBadPrefix(t *testing.T) {
	bad := baseConflict()
	bad.ID = "CONFLICT-hand-written"
	g := &ontology.Graph{Conflicts: []ontology.Conflict{bad}}
	vs := runCheck(t, "check_typed_anchors_conflict", g)
	if !hasViolationFor(vs, "CONFLICT-hand-written") {
		t.Fatalf("expected violation on CONFLICT-hand-written, got %v", vs)
	}
}

func TestCheckTypedAnchorsOperator_OK(t *testing.T) {
	g := &ontology.Graph{Operators: []ontology.Operator{{ID: "OP-1", Stakeholder: "sa"}}}
	if vs := runCheck(t, "check_typed_anchors_operator", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckTypedAnchorsOperator_FiresOnBadPrefix(t *testing.T) {
	g := &ontology.Graph{Operators: []ontology.Operator{{ID: "operator-1", Stakeholder: "sa"}}}
	vs := runCheck(t, "check_typed_anchors_operator", g)
	if !hasViolationFor(vs, "operator-1") {
		t.Fatalf("expected violation on operator-1, got %v", vs)
	}
}

func TestCheckTypedAnchorsProcess_OK(t *testing.T) {
	g := &ontology.Graph{Processes: []ontology.Process{{ID: "PR-1"}}}
	if vs := runCheck(t, "check_typed_anchors_process", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckTypedAnchorsProcess_FiresOnBadPrefix(t *testing.T) {
	g := &ontology.Graph{Processes: []ontology.Process{{ID: "process-1"}}}
	vs := runCheck(t, "check_typed_anchors_process", g)
	if !hasViolationFor(vs, "process-1") {
		t.Fatalf("expected violation on process-1, got %v", vs)
	}
}

func TestCheckTypedAnchorsGoal_OK(t *testing.T) {
	g := &ontology.Graph{Goals: []ontology.Goal{{ID: "GOAL-1", Owner: "sa"}}}
	if vs := runCheck(t, "check_typed_anchors_goal", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckTypedAnchorsGoal_FiresOnBadPrefix(t *testing.T) {
	g := &ontology.Graph{Goals: []ontology.Goal{{ID: "goal-1", Owner: "sa"}}}
	vs := runCheck(t, "check_typed_anchors_goal", g)
	if !hasViolationFor(vs, "goal-1") {
		t.Fatalf("expected violation on goal-1, got %v", vs)
	}
}

func TestCheckTypedAnchors_OK(t *testing.T) {
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{req("R-1", "sa")},
		Assumptions:  []ontology.Assumption{{ID: "A-1", Statement: "s", Status: ontology.AssumptionHOLDS, Owner: "sa"}},
		Conflicts:    []ontology.Conflict{baseConflict()},
		Operators:    []ontology.Operator{{ID: "OP-1", Stakeholder: "sa"}},
		Processes:    []ontology.Process{{ID: "PR-1"}},
		Goals:        []ontology.Goal{{ID: "GOAL-1", Owner: "sa"}},
	}
	if vs := runCheck(t, "check_typed_anchors", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckTypedAnchors_FiresOnBadRequirement(t *testing.T) {
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{{ID: "foo", Owner: "sa"}},
	}
	vs := runCheck(t, "check_typed_anchors", g)
	if !hasViolationFor(vs, "foo") {
		t.Fatalf("expected delegator to fire on bad requirement id, got %v", vs)
	}
}

func TestCheckTypedAnchors_FiresOnBadOperator(t *testing.T) {
	g := &ontology.Graph{
		Operators: []ontology.Operator{{ID: "operator-1", Stakeholder: "sa"}},
	}
	vs := runCheck(t, "check_typed_anchors", g)
	if !hasViolationFor(vs, "operator-1") {
		t.Fatalf("expected delegator to fire on bad operator id, got %v", vs)
	}
}

func TestCheckTypedAnchors_DoesNotCheckVariants(t *testing.T) {
	c := heldConflict()
	c.Variants = []ontology.Variant{
		{ID: "no-v-prefix-1", Behavior: "first"},
		{ID: "no-v-prefix-2", Behavior: "second"},
	}
	g := &ontology.Graph{Conflicts: []ontology.Conflict{c}}
	if vs := runCheck(t, "check_typed_anchors", g); len(vs) != 0 {
		t.Fatalf("delegator must NOT fold in Variant checks (those are dedicated), got %v", vs)
	}
}
