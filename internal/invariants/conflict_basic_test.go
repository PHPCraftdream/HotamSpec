package invariants

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func TestCheckConflictHasAxis_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_conflict_has_axis", graphWithConflict(baseConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckConflictHasAxis_FiresOnEmpty(t *testing.T) {
	t.Parallel()
	bad := baseConflict()
	bad.Axis = "   "
	bad.ID = "C-manual"
	vs := runCheck(t, "check_conflict_has_axis", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckConflictHasContext_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_conflict_has_context", graphWithConflict(baseConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckConflictHasContext_FiresOnEmpty(t *testing.T) {
	t.Parallel()
	bad := baseConflict()
	bad.Context = ""
	bad.ID = "C-manual"
	vs := runCheck(t, "check_conflict_has_context", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckConflictHasSteward_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_conflict_has_steward", graphWithConflict(baseConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckConflictHasSteward_FiresOnEmpty(t *testing.T) {
	t.Parallel()
	bad := baseConflict()
	bad.Steward = ""
	vs := runCheck(t, "check_conflict_has_steward", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckConflictHasAxisContextSteward_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_conflict_has_axis_context_steward", graphWithConflict(baseConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckConflictHasAxisContextSteward_FiresOnMissingSteward(t *testing.T) {
	t.Parallel()
	bad := baseConflict()
	bad.Steward = ""
	vs := runCheck(t, "check_conflict_has_axis_context_steward", graphWithConflict(bad, nil))
	if len(vs) == 0 {
		t.Fatalf("expected >=1 violation, got %v", vs)
	}
}

func TestCheckConflictMinTwoMembers_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_conflict_min_two_members", graphWithConflict(baseConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckConflictMinTwoMembers_FiresOnSingleMember(t *testing.T) {
	t.Parallel()
	bad := baseConflict()
	bad.Members = []string{"R-1"}
	vs := runCheck(t, "check_conflict_min_two_members", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckConstitutingNotInUnresolvedConflict_SilentForBusinessDomain(t *testing.T) {
	t.Parallel()
	bad := baseConflict()
	bad.Lifecycle = "DETECTED"
	g := graphWithConflict(bad, nil)
	if vs := runCheck(t, "check_constituting_not_in_unresolved_conflict", g); len(vs) != 0 {
		t.Fatalf("business domain (not self-hosting) must not fire, got %v", vs)
	}
}

func TestCheckConstitutingNotInUnresolvedConflict_SilentWhenResolved(t *testing.T) {
	t.Parallel()
	decided := baseConflict()
	decided.Lifecycle = "DECIDED(steward chose R-1)"
	decided.DecidedBy = "outsider"
	g := graphWithConflict(decided, []ontology.Requirement{
		req("R-1", "sa"), req("R-2", "sb"), req(constitutingConvergenceAtom, "sa"),
	})
	g.SelfHosting = true
	if vs := runCheck(t, "check_constituting_not_in_unresolved_conflict", g); len(vs) != 0 {
		t.Fatalf("resolved conflict must not fire, got %v", vs)
	}
}

func TestCheckConstitutingNotInUnresolvedConflict_FiresOnSelfHostDetected(t *testing.T) {
	t.Parallel()
	bad := baseConflict()
	bad.Lifecycle = "DETECTED"
	g := graphWithConflict(bad, []ontology.Requirement{
		req("R-1", "sa"), req("R-2", "sb"), req(constitutingConvergenceAtom, "sa"),
	})
	g.SelfHosting = true
	vs := runCheck(t, "check_constituting_not_in_unresolved_conflict", g)
	if len(vs) != 1 {
		t.Fatalf("expected exactly 1 violation, got %v", vs)
	}
	if !strings.Contains(vs[0].Message, "R-1") || !strings.Contains(vs[0].Message, "R-2") {
		t.Fatalf("violation message should name both members, got %q", vs[0].Message)
	}
}

func TestCheckAxisInRegistry_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_axis_in_registry", graphWithConflict(baseConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckAxisInRegistry_FiresOnUnknownAxis(t *testing.T) {
	t.Parallel()
	bad := baseConflict()
	bad.Axis = "totally-made-up-axis"
	bad.ID = ontology.ConflictIdentity("totally-made-up-axis", "some shared scenario")
	vs := runCheck(t, "check_axis_in_registry", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckAxisInRegistry_FiresOnEmptyVocabulary(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sOut, sA, sB},
		Requirements: []ontology.Requirement{req("R-1", "sa"), req("R-2", "sb")},
		Conflicts:    []ontology.Conflict{baseConflict()},
	}
	vs := runCheck(t, "check_axis_in_registry", g)
	if len(vs) == 0 {
		t.Fatalf("empty axes vocabulary must fire for every conflict with an axis, got %v", vs)
	}
}

func TestCheckConflictIDMatchesIdentity_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_conflict_id_matches_identity", graphWithConflict(baseConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckConflictIDMatchesIdentity_FiresOnMismatch(t *testing.T) {
	t.Parallel()
	bad := baseConflict()
	bad.ID = "C-deadbeef"
	vs := runCheck(t, "check_conflict_id_matches_identity", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, "C-deadbeef") {
		t.Fatalf("expected violation on C-deadbeef, got %v", vs)
	}
}

func TestCheckStewardNotAMemberOwner_OK(t *testing.T) {
	t.Parallel()
	if vs := runCheck(t, "check_steward_not_a_member_owner", graphWithConflict(baseConflict(), nil)); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckStewardNotAMemberOwner_FiresWhenStewardOwnsMember(t *testing.T) {
	t.Parallel()
	bad := baseConflict()
	bad.Steward = "sa"
	vs := runCheck(t, "check_steward_not_a_member_owner", graphWithConflict(bad, nil))
	if !hasViolationFor(vs, bad.ID) {
		t.Fatalf("expected violation on %s, got %v", bad.ID, vs)
	}
}

func TestCheckOpenHasQuestion_OK(t *testing.T) {
	t.Parallel()
	g := graphWithConflict(baseConflict(), []ontology.Requirement{
		reqStatus("R-1", "sa", "OPEN(which scope?)"), req("R-2", "sb"),
	})
	if vs := runCheck(t, "check_open_has_question", g); len(vs) != 0 {
		t.Fatalf("expected no violations, got %v", vs)
	}
}

func TestCheckOpenHasQuestion_FiresOnBareOpen(t *testing.T) {
	t.Parallel()
	g := graphWithConflict(baseConflict(), []ontology.Requirement{
		reqStatus("R-1", "sa", "OPEN"), req("R-2", "sb"),
	})
	vs := runCheck(t, "check_open_has_question", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation on R-1, got %v", vs)
	}
}

func TestCheckOpenHasQuestion_FiresOnEmptyParens(t *testing.T) {
	t.Parallel()
	g := graphWithConflict(baseConflict(), []ontology.Requirement{
		reqStatus("R-1", "sa", "OPEN()"), req("R-2", "sb"),
	})
	vs := runCheck(t, "check_open_has_question", g)
	if !hasViolationFor(vs, "R-1") {
		t.Fatalf("expected violation on R-1, got %v", vs)
	}
}
