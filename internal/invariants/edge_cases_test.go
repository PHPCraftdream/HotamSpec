package invariants

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func TestRelationKinds_ExcludesConflictsWith(t *testing.T) {
	t.Parallel()
	if _, present := ontology.RelationKinds["conflicts_with"]; present {
		t.Fatalf("conflicts_with must NOT be a RelationKind — a contradiction is a first-class Conflict NODE, never a bare edge between requirements")
	}
}

func TestCheckEntityInstanceIdPrefix_FiresOnMissingTypeSegment(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Entities: []ontology.EntityInstance{entityInstance("ENT-acme", "thing", "ACTIVE")},
	}
	vs := runCheck(t, "check_entity_instance_id_prefix", g)
	if !hasViolationFor(vs, "ENT-acme") {
		t.Fatalf("id with ENT- prefix but missing entity_type slug segment must fire, got %v", vs)
	}
}

func TestCheckEntityFieldKindKnown_AllValidKindsPass(t *testing.T) {
	t.Parallel()
	for kind := range ontology.EntityFieldKinds {
		et := entityType("thing")
		et.Fields = []ontology.EntityField{{Name: "f", Kind: kind}}
		g := &ontology.Graph{EntityTypes: []ontology.EntityType{et}}
		if vs := runCheck(t, "check_entity_field_kind_known", g); len(vs) != 0 {
			t.Fatalf("valid kind %q must not fire, got %v", kind, vs)
		}
	}
}

func TestCheckEntityInstanceIdPrefix_MessageNamesExpectedPrefix(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Entities: []ontology.EntityInstance{entityInstance("X-1", "thing", "ACTIVE")},
	}
	vs := runCheck(t, "check_entity_instance_id_prefix", g)
	if len(vs) == 0 {
		t.Fatalf("expected a violation for wrong prefix X-1")
	}
	if !strings.Contains(vs[0].Message, "ENT-thing-") {
		t.Fatalf("violation message should name the expected prefix ENT-thing-, got %q", vs[0].Message)
	}
}
