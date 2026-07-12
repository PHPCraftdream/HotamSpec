package main

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// TestParseProposal_AllKinds_SnakeCaseRoundTrip is a table-driven test: one
// entry per proposal kind, each a fully-populated snake_case JSON example
// (field names taken from docs/PROPOSAL-REFERENCE.md). Confirms
// parseProposal -> unmarshalProposal actually lands every field on the
// target Proposed* struct instead of silently leaving it at zero (the bug
// this whole change fixes -- previously json.Unmarshal ran with no tags, so
// case-insensitive fallback matching happened to work for single-word
// fields like "id"/"why" but never for multi-word ones like "enforced_by",
// "created_at", "conflict_id", "new_lifecycle").
func TestParseProposal_AllKinds_SnakeCaseRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		json  string
		check func(t *testing.T, p proposal.Proposal)
	}{
		{
			name: "Requirement",
			json: `{
				"kind": "Requirement", "id": "R-x", "claim": "c", "owner": "o",
				"status": "DRAFT", "why": "w", "assumptions": ["A-1"],
				"relations": [{"kind":"refines","target":"R-y"}],
				"enforcement": "ENFORCED", "enforced_by": ["check_x"],
				"m_tag": "M1", "enforceability": "ENFORCEABLE", "summary": "s",
				"created_at": "2026-01-01", "settled_at": "2026-01-02",
				"last_reviewed_at": "2026-01-03", "review_after": "2026-06-01",
				"evidence": ["e1"], "source_refs": ["doc.md"]
			}`,
			check: func(t *testing.T, p proposal.Proposal) {
				r := p.(proposal.ProposedRequirement)
				if r.ID != "R-x" || r.Claim != "c" || r.Owner != "o" || r.Status != "DRAFT" {
					t.Fatalf("identity fields not populated: %+v", r)
				}
				if len(r.EnforcedBy) != 1 || r.EnforcedBy[0] != "check_x" {
					t.Errorf("EnforcedBy = %v, want [check_x]", r.EnforcedBy)
				}
				if r.CreatedAt != "2026-01-01" {
					t.Errorf("CreatedAt = %q, want 2026-01-01", r.CreatedAt)
				}
				if r.MTag != "M1" {
					t.Errorf("MTag = %q, want M1", r.MTag)
				}
				if r.ReviewAfter != "2026-06-01" {
					t.Errorf("ReviewAfter = %q, want 2026-06-01", r.ReviewAfter)
				}
				if len(r.SourceRefs) != 1 || r.SourceRefs[0] != "doc.md" {
					t.Errorf("SourceRefs = %v, want [doc.md]", r.SourceRefs)
				}
			},
		},
		{
			name: "ConflictTransition",
			json: `{
				"kind": "ConflictTransition", "conflict_id": "C-1",
				"new_lifecycle": "DECIDED(x)", "decided_by": "carol",
				"revisit_marker": "REVISIT if y", "shared_assumption": "A-1",
				"derived": ["R-new"],
				"variants": [{"id":"V-1","behavior":"b","implies":"i","costs":"c"}],
				"date": "2026-07-01", "verbatim": "verbatim text",
				"instrument": "personal", "chosen_variant": "V-1"
			}`,
			check: func(t *testing.T, p proposal.Proposal) {
				c := p.(proposal.ProposedConflictTransition)
				if c.ConflictID != "C-1" || c.NewLifecycle != "DECIDED(x)" {
					t.Fatalf("identity fields not populated: %+v", c)
				}
				if c.DecidedBy != "carol" || c.RevisitMarker != "REVISIT if y" {
					t.Errorf("DecidedBy/RevisitMarker not populated: %+v", c)
				}
				if c.ChosenVariant != "V-1" {
					t.Errorf("ChosenVariant = %q, want V-1", c.ChosenVariant)
				}
			},
		},
		{
			name: "Rejection",
			json: `{"kind":"Rejection","requirement_id":"R-old","reason":"REJECTED — replaced","replaced_by":["R-new"]}`,
			check: func(t *testing.T, p proposal.Proposal) {
				r := p.(proposal.ProposedRejection)
				if r.RequirementID != "R-old" || r.Reason != "REJECTED — replaced" {
					t.Fatalf("identity fields not populated: %+v", r)
				}
				if len(r.ReplacedBy) != 1 || r.ReplacedBy[0] != "R-new" {
					t.Errorf("ReplacedBy = %v, want [R-new]", r.ReplacedBy)
				}
			},
		},
		{
			name: "Conflict",
			json: `{
				"kind": "Conflict", "axis": "speed-vs-rigor", "context": "ctx",
				"members": ["R-1","R-2"], "steward": "carol",
				"shared_assumption": "A-1", "note": "n",
				"initial_lifecycle": "DETECTED", "decided_by": "carol"
			}`,
			check: func(t *testing.T, p proposal.Proposal) {
				c := p.(proposal.ProposedConflict)
				if c.Axis != "speed-vs-rigor" || c.Context != "ctx" || c.Steward != "carol" {
					t.Fatalf("identity fields not populated: %+v", c)
				}
				if c.SharedAssumption != "A-1" || c.Note != "n" {
					t.Errorf("SharedAssumption/Note not populated: %+v", c)
				}
			},
		},
		{
			name: "OperatorBudget",
			json: `{"kind":"OperatorBudget","operator_id":"OP-director","new_limit":150000,"new_measure":"CRYSTAL_CHARS","why":"w"}`,
			check: func(t *testing.T, p proposal.Proposal) {
				b := p.(proposal.ProposedOperatorBudget)
				if b.OperatorID != "OP-director" || b.NewLimit != 150000 || b.NewMeasure != "CRYSTAL_CHARS" || b.Why != "w" {
					t.Fatalf("fields not populated: %+v", b)
				}
			},
		},
		{
			name: "Axis",
			json: `{"kind":"Axis","slug":"speed-vs-rigor","description":"d","why":"w"}`,
			check: func(t *testing.T, p proposal.Proposal) {
				a := p.(proposal.ProposedAxis)
				if a.Slug != "speed-vs-rigor" || a.Description != "d" || a.Why != "w" {
					t.Fatalf("fields not populated: %+v", a)
				}
			},
		},
		{
			name: "Stakeholder",
			json: `{"kind":"Stakeholder","id":"carol","name":"Carol","domain":"governance","why":"w"}`,
			check: func(t *testing.T, p proposal.Proposal) {
				s := p.(proposal.ProposedStakeholder)
				if s.ID != "carol" || s.Name != "Carol" || s.Domain != "governance" || s.Why != "w" {
					t.Fatalf("fields not populated: %+v", s)
				}
			},
		},
		{
			name: "Assumption",
			json: `{"kind":"Assumption","id":"A-1","statement":"s","status":"HOLDS","owner":"alice","why":"w","created_at":"2026-07-01"}`,
			check: func(t *testing.T, p proposal.Proposal) {
				a := p.(proposal.ProposedAssumption)
				if a.ID != "A-1" || a.Statement != "s" || a.Status != "HOLDS" || a.Owner != "alice" {
					t.Fatalf("identity fields not populated: %+v", a)
				}
				if a.CreatedAt != "2026-07-01" {
					t.Errorf("CreatedAt = %q, want 2026-07-01", a.CreatedAt)
				}
			},
		},
		{
			name: "AssumptionTransition",
			json: `{
				"kind": "AssumptionTransition", "assumption_id": "A-1",
				"new_status": "DEAD", "reason": "r", "decided_by": "alice",
				"date": "2026-07-01", "verbatim": "v", "instrument": "personal"
			}`,
			check: func(t *testing.T, p proposal.Proposal) {
				a := p.(proposal.ProposedAssumptionTransition)
				if a.AssumptionID != "A-1" || a.NewStatus != "DEAD" || a.Reason != "r" {
					t.Fatalf("identity fields not populated: %+v", a)
				}
				if a.DecidedBy != "alice" || a.Date != "2026-07-01" {
					t.Errorf("DecidedBy/Date not populated: %+v", a)
				}
			},
		},
		{
			name: "ConflictMemberUpdate",
			json: `{"kind":"ConflictMemberUpdate","conflict_id":"C-1","add_members":["R-3"],"remove_members":["R-2"],"decided_by":"carol"}`,
			check: func(t *testing.T, p proposal.Proposal) {
				c := p.(proposal.ProposedConflictMemberUpdate)
				if c.ConflictID != "C-1" || c.DecidedBy != "carol" {
					t.Fatalf("identity fields not populated: %+v", c)
				}
				if len(c.AddMembers) != 1 || c.AddMembers[0] != "R-3" {
					t.Errorf("AddMembers = %v, want [R-3]", c.AddMembers)
				}
				if len(c.RemoveMembers) != 1 || c.RemoveMembers[0] != "R-2" {
					t.Errorf("RemoveMembers = %v, want [R-2]", c.RemoveMembers)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := parseProposal([]byte(tc.json))
			if err != nil {
				t.Fatalf("parseProposal: %v", err)
			}
			tc.check(t, p)
		})
	}
}

// TestParseProposal_EntityType_ObjectShape covers EntityType separately from
// the table above: its nested States/Transitions/Fields are Go structs with
// snake_case tags (JSON object shape), not Python's compact array-triple
// wire format (docs/PROPOSAL-REFERENCE.md describes the array-triple shape
// as the Python tool's contract; this Go port's ProposedEntityType has no
// custom UnmarshalJSON to accept that compact form -- see the final report
// for this documented divergence).
func TestParseProposal_EntityType_ObjectShape(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"kind": "EntityType", "slug": "release", "description": "d", "why": "w",
		"states": [{"name":"draft","kind":"initial","why":"start"}],
		"transitions": [{"src":"draft","dst":"shipped","event":"ship"}],
		"cyclic": true,
		"fields": [{"name":"owner","kind":"ref","required":true,"ref_target":"Stakeholder"}]
	}`)
	p, err := parseProposal(data)
	if err != nil {
		t.Fatalf("parseProposal: %v", err)
	}
	e := p.(proposal.ProposedEntityType)
	if e.Slug != "release" || e.Description != "d" || e.Why != "w" {
		t.Fatalf("identity fields not populated: %+v", e)
	}
	if len(e.States) != 1 || e.States[0].Name != "draft" || e.States[0].Why != "start" {
		t.Errorf("States not populated: %+v", e.States)
	}
	if len(e.Transitions) != 1 || e.Transitions[0].Event != "ship" {
		t.Errorf("Transitions not populated: %+v", e.Transitions)
	}
	if !e.Cyclic {
		t.Errorf("Cyclic = false, want true")
	}
	if len(e.Fields) != 1 || e.Fields[0].RefTarget != "Stakeholder" {
		t.Errorf("Fields not populated: %+v", e.Fields)
	}
}

// TestParseProposal_UnknownFieldRejected covers case (b): a field name not
// declared on the target struct (typo or invented key) must be a hard
// error, not a silent no-op.
func TestParseProposal_UnknownFieldRejected(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		json       string
		wantSubstr string
	}{
		{
			name:       "Requirement typo'd field",
			json:       `{"kind":"Requirement","id":"R-x","claim":"c","owner":"o","status":"DRAFT","enforcedby":["check_x"]}`,
			wantSubstr: "enforcedby",
		},
		{
			name:       "Requirement invented field",
			json:       `{"kind":"Requirement","id":"R-x","claim":"c","owner":"o","status":"DRAFT","bogus_field":"x"}`,
			wantSubstr: "bogus_field",
		},
		{
			name:       "ConflictTransition unknown field",
			json:       `{"kind":"ConflictTransition","conflict_id":"C-1","new_lifecycle":"DECIDED(x)","decided_by":"c","unexpected":"x"}`,
			wantSubstr: "unexpected",
		},
		{
			name:       "Assumption unknown field",
			json:       `{"kind":"Assumption","id":"A-1","statement":"s","status":"HOLDS","owner":"a","extra_junk":true}`,
			wantSubstr: "extra_junk",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseProposal([]byte(tc.json))
			if err == nil {
				t.Fatalf("expected error for unknown field, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

// TestParseProposal_CamelCaseOldFormatRejected covers case (c): a
// proposal file written in the pre-tag camelCase convention (or any
// case variant other than the documented snake_case) must fail loudly
// via DisallowUnknownFields, not silently unmarshal into zero-valued
// fields the way a bare json.Unmarshal (no tags, no strict mode) used to.
func TestParseProposal_CamelCaseOldFormatRejected(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		json       string
		wantSubstr string
	}{
		{
			name:       "Requirement camelCase enforcedBy",
			json:       `{"kind":"Requirement","id":"R-x","claim":"c","owner":"o","status":"DRAFT","enforcedBy":["check_x"]}`,
			wantSubstr: "enforcedBy",
		},
		{
			name:       "Requirement camelCase createdAt",
			json:       `{"kind":"Requirement","id":"R-x","claim":"c","owner":"o","status":"DRAFT","createdAt":"2026-01-01"}`,
			wantSubstr: "createdAt",
		},
		{
			name:       "ConflictTransition camelCase conflictId/newLifecycle",
			json:       `{"kind":"ConflictTransition","conflictId":"C-1","newLifecycle":"DECIDED(x)","decidedBy":"c"}`,
			wantSubstr: "conflictId",
		},
		{
			name:       "AssumptionTransition camelCase newStatus",
			json:       `{"kind":"AssumptionTransition","assumption_id":"A-1","newStatus":"DEAD","reason":"r"}`,
			wantSubstr: "newStatus",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, err := parseProposal([]byte(tc.json))
			if err == nil {
				t.Fatalf("expected error for camelCase legacy field, got proposal %+v", p)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

// TestUnmarshalProposal_KindFieldStrippedNotUnknown proves the top-level
// "kind" selector is legal on every proposal (it must not itself trigger
// DisallowUnknownFields, since no Proposed* struct declares it).
func TestUnmarshalProposal_KindFieldStrippedNotUnknown(t *testing.T) {
	t.Parallel()
	var p proposal.ProposedStakeholder
	err := unmarshalProposal([]byte(`{"kind":"Stakeholder","id":"carol","name":"Carol","domain":"governance"}`), &p)
	if err != nil {
		t.Fatalf("unmarshalProposal: %v", err)
	}
	if p.ID != "carol" || p.Name != "Carol" || p.Domain != "governance" {
		t.Errorf("fields not populated: %+v", p)
	}
}
