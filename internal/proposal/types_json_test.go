package proposal

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestProposedStructs_JSONTagsRoundTrip verifies that every Proposed*
// struct's exported fields carry the snake_case json tag documented in
// docs/PROPOSAL-REFERENCE.md, by round-tripping a fully-populated
// snake_case JSON object through json.Unmarshal and asserting every field
// lands non-zero. Before json tags were added, encoding/json fell back to
// case-insensitive matching against the bare Go field name (e.g.
// "enforced_by" would NOT match EnforcedBy without a tag), so a struct
// missing a tag here would leave the corresponding field at its zero value
// silently -- exactly the bug this test guards against regressing.
func TestProposedStructs_JSONTagsRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		json string
		into func() (any, error)
		want any
	}{
		{
			name: "Requirement",
			json: `{
				"id": "R-x", "claim": "c", "owner": "o", "status": "DRAFT",
				"why": "w", "assumptions": ["A-1"],
				"relations": [{"kind":"refines","target":"R-y"}],
				"enforcement": "ENFORCED", "enforced_by": ["check_x"],
				"m_tag": "M1", "enforceability": "ENFORCEABLE",
				"summary": "s", "created_at": "2026-01-01",
				"settled_at": "2026-01-02", "last_reviewed_at": "2026-01-03",
				"review_after": "2026-06-01", "evidence": ["e1"],
				"source_refs": ["doc.md"], "blocked_on": "blocked on Planned tool T"
			}`,
			into: func() (any, error) {
				var p ProposedRequirement
				err := json.Unmarshal([]byte(`{
				"id": "R-x", "claim": "c", "owner": "o", "status": "DRAFT",
				"why": "w", "assumptions": ["A-1"],
				"relations": [{"kind":"refines","target":"R-y"}],
				"enforcement": "ENFORCED", "enforced_by": ["check_x"],
				"m_tag": "M1", "enforceability": "ENFORCEABLE",
				"summary": "s", "created_at": "2026-01-01",
				"settled_at": "2026-01-02", "last_reviewed_at": "2026-01-03",
				"review_after": "2026-06-01", "evidence": ["e1"],
				"source_refs": ["doc.md"], "blocked_on": "blocked on Planned tool T"
			}`), &p)
				return p, err
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.into()
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			assertNoZeroFields(t, got)
		})
	}
}

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
		"chosen_variant": "V-1"
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
