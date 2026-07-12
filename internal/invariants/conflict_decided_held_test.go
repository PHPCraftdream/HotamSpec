package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func TestCheckDecidedHasRationaleOrDerived_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_decided_has_rationale_or_derived", graphWithConflict(decidedConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckDecidedHasRationaleOrDerived_OKWithDerived(t *testing.T) {
	t.Parallel()
	bad := decidedConflict()
	bad.Lifecycle = "DECIDED()"
	bad.Derived = []string{"R-3"}
	g := graphWithConflict(bad, []ontology.Requirement{
		req("R-1", "sa"), req("R-2", "sb"), req("R-3", "outsider"),
	})
	if vs := runCheck(t, "check_decided_has_rationale_or_derived", g); len(vs) != 0 {
		t.Fatalf("derived requirement must satisfy the check, got %v", vs)
	}
}

func TestCheckDecidedHasRationaleOrDerived_FiresOnBareDecided(t *testing.T) {
	t.Parallel()
	bad := decidedConflict()
	bad.Lifecycle = "DECIDED"
	vs := runCheck(t, "check_decided_has_rationale_or_derived", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckDecidedHasRationaleOrDerived_FiresOnEmptyParens(t *testing.T) {
	t.Parallel()
	bad := decidedConflict()
	bad.Lifecycle = "DECIDED()"
	vs := runCheck(t, "check_decided_has_rationale_or_derived", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s for empty rationale parens, got %v", bad.ID, vs)
	}
}

func TestCheckDecidedHasRationaleOrDerived_SilentOnAcknowledged(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_decided_has_rationale_or_derived", graphWithConflict(baseConflict(), nil)); len(vs) != 0 {
		t.Fatalf("ACKNOWLEDGED conflict must not be checked, got %v", vs)
	}
}

func TestCheckDecidedHasNonemptyDecidedBy_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_decided_has_nonempty_decided_by", graphWithConflict(decidedConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckDecidedHasNonemptyDecidedBy_FiresOnEmpty(t *testing.T) {
	t.Parallel()
	bad := decidedConflict()
	bad.DecidedBy = ""
	vs := runCheck(t, "check_decided_has_nonempty_decided_by", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckDecidedByIsKnownStakeholder_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_decided_by_is_known_stakeholder", graphWithConflict(decidedConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckDecidedByIsKnownStakeholder_FiresOnUnknown(t *testing.T) {
	t.Parallel()
	bad := decidedConflict()
	bad.DecidedBy = "ghost"
	vs := runCheck(t, "check_decided_by_is_known_stakeholder", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckDecidedByNotMemberOwner_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_decided_by_not_member_owner", graphWithConflict(decidedConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckDecidedByNotMemberOwner_FiresWhenDeciderOwnsMember(t *testing.T) {
	t.Parallel()
	bad := decidedConflict()
	bad.DecidedBy = "sa"
	vs := runCheck(t, "check_decided_by_not_member_owner", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckHeldHasMinTwoVariants_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_held_has_min_two_variants", graphWithConflict(heldConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckHeldHasMinTwoVariants_FiresOnSingleVariant(t *testing.T) {
	t.Parallel()
	bad := heldConflict()
	bad.Variants = []ontology.Variant{variant("V-only", "only option")}
	vs := runCheck(t, "check_held_has_min_two_variants", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckHeldHasMinTwoVariants_FiresOnDuplicateVariantIDs(t *testing.T) {
	t.Parallel()
	bad := heldConflict()
	bad.Variants = []ontology.Variant{
		variant("V-dup", "first"),
		variant("V-dup", "second"),
	}
	vs := runCheck(t, "check_held_has_min_two_variants", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s for duplicate variant ids, got %v", bad.ID, vs)
	}
}

func TestCheckHeldHasMinTwoVariants_SilentOnDecided(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_held_has_min_two_variants", graphWithConflict(decidedConflict(), nil)); len(vs) != 0 {
		t.Fatalf("DECIDED conflict must not be checked, got %v", vs)
	}
}

func TestCheckHeldHasNonemptyDecidedBy_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_held_has_nonempty_decided_by", graphWithConflict(heldConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckHeldHasNonemptyDecidedBy_FiresOnEmpty(t *testing.T) {
	t.Parallel()
	bad := heldConflict()
	bad.DecidedBy = ""
	vs := runCheck(t, "check_held_has_nonempty_decided_by", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckHeldByIsKnownStakeholder_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_held_by_is_known_stakeholder", graphWithConflict(heldConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckHeldByIsKnownStakeholder_FiresOnUnknown(t *testing.T) {
	t.Parallel()
	bad := heldConflict()
	bad.DecidedBy = "ghost"
	vs := runCheck(t, "check_held_by_is_known_stakeholder", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckHeldByNotMemberOwner_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_held_by_not_member_owner", graphWithConflict(heldConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckHeldByNotMemberOwner_FiresWhenHolderOwnsMember(t *testing.T) {
	t.Parallel()
	bad := heldConflict()
	bad.DecidedBy = "sa"
	vs := runCheck(t, "check_held_by_not_member_owner", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckHeldHasDecidedBy_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_held_has_decided_by", graphWithConflict(heldConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckHeldHasDecidedBy_FiresOnEmptyDecidedBy(t *testing.T) {
	t.Parallel()
	bad := heldConflict()
	bad.DecidedBy = ""
	vs := runCheck(t, "check_held_has_decided_by", graphWithConflict(bad, nil))
	if len(vs) == 0 {
		t.Fatalf("expected >=1 violation, got %v", vs)
	}
}

func TestCheckDecidedHasDecidedBy_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_decided_has_decided_by", graphWithConflict(decidedConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckDecidedHasDecidedBy_FiresWhenDeciderOwnsMember(t *testing.T) {
	t.Parallel()
	bad := decidedConflict()
	bad.DecidedBy = "sa"
	vs := runCheck(t, "check_decided_has_decided_by", graphWithConflict(bad, nil))
	if len(vs) == 0 {
		t.Fatalf("expected >=1 violation, got %v", vs)
	}
}
