package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func TestCheckEntityTypeLifecycleWellformed_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{EntityTypes: []ontology.EntityType{entityType("thing")}}
	if vs := runCheck(t, "check_entity_type_lifecycle_wellformed", g); len(vs) != 0 {
		t.Fatalf("expected no violations for a well-formed EntityType lifecycle, got %v", vs)
	}
}

func TestCheckEntityTypeLifecycleWellformed_FiresOnMalformedLifecycle(t *testing.T) {
	t.Parallel()
	et := entityType("thing")
	et.Lifecycle = ontology.Lifecycle{Slug: "bad"}
	g := &ontology.Graph{EntityTypes: []ontology.EntityType{et}}
	vs := runCheck(t, "check_entity_type_lifecycle_wellformed", g)
	if !hasViolationFor(vs, "thing") {
		t.Fatalf("expected violation on thing for malformed lifecycle, got %v", vs)
	}
}

func TestCheckTransitionGuardAssumptionResolves_OK(t *testing.T) {
	t.Parallel()
	ga := "A-1"
	et := entityType("thing")
	et.Lifecycle.Transitions[0].GuardAssumption = &ga
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		EntityTypes:  []ontology.EntityType{et},
		Assumptions:  []ontology.Assumption{{ID: "A-1", Status: ontology.AssumptionHOLDS, Owner: "sa"}},
	}
	if vs := runCheck(t, "check_transition_guard_assumption_resolves", g); len(vs) != 0 {
		t.Fatalf("expected no violations for resolving guard_assumption, got %v", vs)
	}
}

func TestCheckTransitionGuardAssumptionResolves_FiresOnDangling(t *testing.T) {
	t.Parallel()
	ga := "A-ghost"
	et := entityType("thing")
	et.Lifecycle.Transitions[0].GuardAssumption = &ga
	g := &ontology.Graph{
		EntityTypes: []ontology.EntityType{et},
	}
	vs := runCheck(t, "check_transition_guard_assumption_resolves", g)
	if !hasViolationFor(vs, "thing") {
		t.Fatalf("expected violation for dangling guard_assumption, got %v", vs)
	}
}

func TestCheckTransitionGuardAssumptionResolves_SkipsEmptyGuardAssumption(t *testing.T) {
	t.Parallel()
	ga := ""
	et := entityType("thing")
	et.Lifecycle.Transitions[0].GuardAssumption = &ga
	g := &ontology.Graph{EntityTypes: []ontology.EntityType{et}}
	if vs := runCheck(t, "check_transition_guard_assumption_resolves", g); len(vs) != 0 {
		t.Fatalf("empty guard_assumption must be skipped, got %v", vs)
	}
}

func TestCheckAssumptionMachineChecksSyntactic_OKNonEmptyFormula(t *testing.T) {
	t.Parallel()
	mc := "len(graph.requirements) < 100"
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Assumptions: []ontology.Assumption{
			{ID: "A-1", Status: ontology.AssumptionHOLDS, Owner: "sa", MachineCheck: &mc},
		},
	}
	if vs := runCheck(t, "check_assumption_machine_checks_syntactic", g); len(vs) != 0 {
		t.Fatalf("expected no violations for non-empty machine_check, got %v", vs)
	}
}

func TestCheckAssumptionMachineChecksSyntactic_OKNilMachineCheckSkipped(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Assumptions: []ontology.Assumption{
			{ID: "A-1", Status: ontology.AssumptionHOLDS, Owner: "sa"},
		},
	}
	if vs := runCheck(t, "check_assumption_machine_checks_syntactic", g); len(vs) != 0 {
		t.Fatalf("nil machine_check must be skipped, got %v", vs)
	}
}

func TestCheckAssumptionMachineChecksSyntactic_FiresOnEmptyMarker(t *testing.T) {
	t.Parallel()
	mc := "   "
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		Assumptions: []ontology.Assumption{
			{ID: "A-1", Status: ontology.AssumptionHOLDS, Owner: "sa", MachineCheck: &mc},
		},
	}
	vs := runCheck(t, "check_assumption_machine_checks_syntactic", g)
	if !hasViolationFor(vs, "A-1") {
		t.Fatalf("expected violation for empty machine_check marker, got %v", vs)
	}
}

func TestCheckEntityInstanceStateInLifecycle_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		EntityTypes: []ontology.EntityType{entityType("thing")},
		Entities:    []ontology.EntityInstance{entityInstance("ENT-thing-1", "thing", "ACTIVE")},
	}
	if vs := runCheck(t, "check_entity_instance_state_in_lifecycle", g); len(vs) != 0 {
		t.Fatalf("expected no violations for valid state, got %v", vs)
	}
}

func TestCheckEntityInstanceStateInLifecycle_FiresOnBogusState(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		EntityTypes: []ontology.EntityType{entityType("thing")},
		Entities:    []ontology.EntityInstance{entityInstance("ENT-thing-1", "thing", "BOGUS")},
	}
	vs := runCheck(t, "check_entity_instance_state_in_lifecycle", g)
	if !hasViolationFor(vs, "ENT-thing-1") {
		t.Fatalf("expected violation for bogus state, got %v", vs)
	}
}

func TestCheckEntityInstanceStateInLifecycle_FiresOnUndeclaredType(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Entities: []ontology.EntityInstance{entityInstance("ENT-thing-1", "ghost", "ACTIVE")},
	}
	vs := runCheck(t, "check_entity_instance_state_in_lifecycle", g)
	if !hasViolationFor(vs, "ENT-thing-1") {
		t.Fatalf("expected violation for undeclared entity_type, got %v", vs)
	}
}

func TestCheckEntityInstanceRequiredFields_OK(t *testing.T) {
	t.Parallel()
	et := entityType("thing")
	et.Fields = []ontology.EntityField{{Name: "name", Kind: "string", Required: true}}
	inst := entityInstance("ENT-thing-1", "thing", "ACTIVE")
	inst.FieldValues = [][2]string{{"name", "widget"}}
	g := &ontology.Graph{
		EntityTypes: []ontology.EntityType{et},
		Entities:    []ontology.EntityInstance{inst},
	}
	if vs := runCheck(t, "check_entity_instance_required_fields", g); len(vs) != 0 {
		t.Fatalf("expected no violations when required field is present, got %v", vs)
	}
}

func TestCheckEntityInstanceRequiredFields_FiresOnMissingRequired(t *testing.T) {
	t.Parallel()
	et := entityType("thing")
	et.Fields = []ontology.EntityField{{Name: "name", Kind: "string", Required: true}}
	g := &ontology.Graph{
		EntityTypes: []ontology.EntityType{et},
		Entities:    []ontology.EntityInstance{entityInstance("ENT-thing-1", "thing", "ACTIVE")},
	}
	vs := runCheck(t, "check_entity_instance_required_fields", g)
	if !hasViolationFor(vs, "ENT-thing-1") {
		t.Fatalf("expected violation for missing required field, got %v", vs)
	}
}

func TestCheckEntityInstanceIdPrefix_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Entities: []ontology.EntityInstance{entityInstance("ENT-thing-1", "thing", "ACTIVE")},
	}
	if vs := runCheck(t, "check_entity_instance_id_prefix", g); len(vs) != 0 {
		t.Fatalf("expected no violations for correct prefix, got %v", vs)
	}
}

func TestCheckEntityInstanceIdPrefix_FiresOnWrongPrefix(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Entities: []ontology.EntityInstance{entityInstance("WRONG-thing-1", "thing", "ACTIVE")},
	}
	vs := runCheck(t, "check_entity_instance_id_prefix", g)
	if !hasViolationFor(vs, "WRONG-thing-1") {
		t.Fatalf("expected violation for wrong prefix, got %v", vs)
	}
}

func TestCheckEntityInstanceRefsResolve_OKStakeholderTarget(t *testing.T) {
	t.Parallel()
	et := entityType("thing")
	et.Fields = []ontology.EntityField{{Name: "owner", Kind: "reference", RefTarget: "stakeholder"}}
	inst := entityInstance("ENT-thing-1", "thing", "ACTIVE")
	inst.FieldValues = [][2]string{{"owner", "sa"}}
	g := &ontology.Graph{
		Stakeholders: []ontology.Stakeholder{sA},
		EntityTypes:  []ontology.EntityType{et},
		Entities:     []ontology.EntityInstance{inst},
	}
	if vs := runCheck(t, "check_entity_instance_refs_resolve", g); len(vs) != 0 {
		t.Fatalf("expected no violations for resolving stakeholder reference, got %v", vs)
	}
}

func TestCheckEntityInstanceRefsResolve_OKEntityTypeTarget(t *testing.T) {
	t.Parallel()
	et := entityType("thing")
	et.Fields = []ontology.EntityField{{Name: "parent", Kind: "reference", RefTarget: "thing"}}
	inst := entityInstance("ENT-thing-1", "thing", "ACTIVE")
	inst.FieldValues = [][2]string{{"parent", "ENT-thing-0"}}
	g := &ontology.Graph{
		EntityTypes: []ontology.EntityType{et},
		Entities: []ontology.EntityInstance{
			entityInstance("ENT-thing-0", "thing", "ACTIVE"),
			inst,
		},
	}
	if vs := runCheck(t, "check_entity_instance_refs_resolve", g); len(vs) != 0 {
		t.Fatalf("expected no violations for resolving entity-type reference, got %v", vs)
	}
}

func TestCheckEntityInstanceRefsResolve_FiresOnDanglingStakeholderRef(t *testing.T) {
	t.Parallel()
	et := entityType("thing")
	et.Fields = []ontology.EntityField{{Name: "owner", Kind: "reference", RefTarget: "stakeholder"}}
	inst := entityInstance("ENT-thing-1", "thing", "ACTIVE")
	inst.FieldValues = [][2]string{{"owner", "ghost"}}
	g := &ontology.Graph{
		EntityTypes: []ontology.EntityType{et},
		Entities:    []ontology.EntityInstance{inst},
	}
	vs := runCheck(t, "check_entity_instance_refs_resolve", g)
	if !hasViolationFor(vs, "ENT-thing-1") {
		t.Fatalf("expected violation for dangling stakeholder reference, got %v", vs)
	}
}

func TestCheckEntityInstanceRefsResolve_FiresOnDanglingEntityRef(t *testing.T) {
	t.Parallel()
	et := entityType("thing")
	et.Fields = []ontology.EntityField{{Name: "parent", Kind: "reference", RefTarget: "thing"}}
	inst := entityInstance("ENT-thing-1", "thing", "ACTIVE")
	inst.FieldValues = [][2]string{{"parent", "ENT-thing-ghost"}}
	g := &ontology.Graph{
		EntityTypes: []ontology.EntityType{et},
		Entities:    []ontology.EntityInstance{inst},
	}
	vs := runCheck(t, "check_entity_instance_refs_resolve", g)
	if !hasViolationFor(vs, "ENT-thing-1") {
		t.Fatalf("expected violation for dangling entity reference, got %v", vs)
	}
}

func TestCheckEntityFieldKindKnown_OK(t *testing.T) {
	t.Parallel()
	et := entityType("thing")
	et.Fields = []ontology.EntityField{
		{Name: "a", Kind: "string"},
		{Name: "b", Kind: "reference"},
	}
	g := &ontology.Graph{EntityTypes: []ontology.EntityType{et}}
	if vs := runCheck(t, "check_entity_field_kind_known", g); len(vs) != 0 {
		t.Fatalf("expected no violations for known kinds, got %v", vs)
	}
}

func TestCheckEntityFieldKindKnown_FiresOnBogusKind(t *testing.T) {
	t.Parallel()
	et := entityType("thing")
	et.Fields = []ontology.EntityField{{Name: "a", Kind: "bogus"}}
	g := &ontology.Graph{EntityTypes: []ontology.EntityType{et}}
	vs := runCheck(t, "check_entity_field_kind_known", g)
	if !hasViolationFor(vs, "thing") {
		t.Fatalf("expected violation for bogus kind, got %v", vs)
	}
}

func TestCheckTypedAnchorsEntity_OK(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Entities: []ontology.EntityInstance{entityInstance("ENT-thing-1", "thing", "ACTIVE")},
	}
	if vs := runCheck(t, "check_typed_anchors_entity", g); len(vs) != 0 {
		t.Fatalf("expected no violations for ENT- prefix, got %v", vs)
	}
}

func TestCheckTypedAnchorsEntity_FiresOnMissingEntPrefix(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Entities: []ontology.EntityInstance{entityInstance("WRONG-1", "thing", "ACTIVE")},
	}
	vs := runCheck(t, "check_typed_anchors_entity", g)
	if !hasViolationFor(vs, "WRONG-1") {
		t.Fatalf("expected violation for missing ENT- prefix, got %v", vs)
	}
}

func TestCheckEntitiesMdListsAllTypes_NoOpInGoInvariantLayer(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{EntityTypes: []ontology.EntityType{entityType("thing")}}
	if vs := runCheck(t, "check_entities_md_lists_all_types", g); len(vs) != 0 {
		t.Fatalf("filesystem-coherence check is a no-op in the graph-only Go invariant contract, got %v", vs)
	}
}

func TestCheckEntityTypeConstitutionProjection_NoOpInGoInvariantLayer(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{EntityTypes: []ontology.EntityType{entityType("thing")}}
	if vs := runCheck(t, "check_entity_type_constitution_projection", g); len(vs) != 0 {
		t.Fatalf("filesystem-coherence check is a no-op in the graph-only Go invariant contract, got %v", vs)
	}
}
