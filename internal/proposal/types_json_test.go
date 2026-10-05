package proposal

import (
	"encoding/json"
	"reflect"
	"testing"
)

// assertNoZeroFields fails the test if any exported field of v (a struct or
// pointer-to-struct) is the zero value for its type. Used to prove that a
// snake_case JSON payload actually populated every declared field, rather
// than silently leaving fields untouched due to a missing/incorrect tag.
func assertNoZeroFields(t *testing.T, v any) {
	t.Helper()
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		fv := rv.Field(i)
		if fv.IsZero() {
			t.Errorf("field %s.%s is zero-valued after unmarshal (tag=%q); json tag missing or mismatched?",
				rt.Name(), f.Name, f.Tag.Get("json"))
		}
	}
}

func TestProposedRequirement_EnforcedByFieldPopulated(t *testing.T) {
	t.Parallel()
	data := []byte(`{"id":"R-x","claim":"c","owner":"o","status":"DRAFT","enforced_by":["check_requirement_x"],"created_at":"2026-07-01"}`)
	var p ProposedRequirement
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.EnforcedBy) != 1 || p.EnforcedBy[0] != "check_requirement_x" {
		t.Errorf("EnforcedBy = %v, want [check_requirement_x]", p.EnforcedBy)
	}
	if p.CreatedAt != "2026-07-01" {
		t.Errorf("CreatedAt = %q, want 2026-07-01", p.CreatedAt)
	}
}

func TestProposedConflictTransition_SnakeCaseFields(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"conflict_id": "C-1", "new_lifecycle": "DECIDED(x)", "decided_by": "carol",
		"revisit_marker": "REVISIT if y", "shared_assumption": "A-1",
		"derived": ["R-new"], "variants": [{"id":"V-1","behavior":"b","implies":"i","costs":"c"}],
		"date": "2026-07-01", "verbatim": "verbatim text", "instrument": "personal",
		"chosen_variant": "V-1", "source_refs": ["docs/decision.md"]
	}`)
	var p ProposedConflictTransition
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertNoZeroFields(t, p)
}

func TestProposedEntityType_SnakeCaseFields(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"slug": "release", "description": "d", "why": "w",
		"states": [{"name":"draft","kind":"initial","why":"start"}],
		"transitions": [{"src":"draft","dst":"shipped","event":"ship"}],
		"cyclic": true,
		"fields": [{"name":"owner","kind":"ref","required":true,"ref_target":"Stakeholder"}]
	}`)
	var p ProposedEntityType
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Slug != "release" || len(p.States) != 1 || p.States[0].Why != "start" {
		t.Errorf("EntityType not fully populated: %+v", p)
	}
	if len(p.Fields) != 1 || p.Fields[0].RefTarget != "Stakeholder" {
		t.Errorf("Fields not fully populated: %+v", p.Fields)
	}
}

func TestProposedGoal_SnakeCaseFields(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"id": "GOAL-x", "owner": "OP-1",
		"target_state": {"kind":"GRAPH_PROPERTY","predicate":"p","target":"t"},
		"why": "w"
	}`)
	var p ProposedGoal
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertNoZeroFields(t, p)
}

func TestProposedEntityInstance_SnakeCaseFields(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"id": "ENT-feature-flag-1", "entity_type": "feature-flag", "state": "INIT",
		"field_values": [["owner", "sa"]]
	}`)
	var p ProposedEntityInstance
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertNoZeroFields(t, p)
}
