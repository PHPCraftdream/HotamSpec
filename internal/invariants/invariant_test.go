package invariants

import "testing"

var batchANames = []string{
	"check_no_dangling_assumption_owner",
	"check_assumption_status_valid",
	"check_no_dangling_requirement_owner",
	"check_no_dangling_requirement_assumptions",
	"check_no_dangling_requirement_relations",
	"check_no_dangling_conflict_refs",
	"check_no_dangling_operator_refs",
	"check_no_dangling_ids",
	"check_doc_reader_resolves_to_stakeholder",
	"check_conflict_has_axis",
	"check_conflict_has_context",
	"check_conflict_has_steward",
	"check_conflict_has_axis_context_steward",
	"check_conflict_min_two_members",
	"check_constituting_not_in_unresolved_conflict",
	"check_axis_in_registry",
	"check_conflict_id_matches_identity",
	"check_steward_not_a_member_owner",
	"check_open_has_question",
}

func TestBatchAInvariantsRegistered(t *testing.T) {
	t.Parallel()
	all := All.All()
	if len(all) < len(batchANames) {
		t.Fatalf("expected at least %d registered invariants, got %d", len(batchANames), len(all))
	}
	seen := map[string]bool{}
	for _, inv := range all {
		seen[inv.Name] = true
	}
	for _, name := range batchANames {
		if !seen[name] {
			t.Errorf("expected invariant %q to be registered", name)
		}
	}
}

func TestRegisteredInvariantsHaveCanon(t *testing.T) {
	t.Parallel()
	for _, inv := range All.All() {
		if inv.Canon == nil {
			t.Errorf("invariant %q has nil Canon section", inv.Name)
		}
		if inv.Check == nil {
			t.Errorf("invariant %q has nil Check function", inv.Name)
		}
	}
}
