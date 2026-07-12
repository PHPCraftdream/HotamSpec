package proposal

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestProposedReviewMark_JSONRoundTrip proves the snake_case json tags on
// ProposedReviewMark actually populate every field, mirroring the guard in
// types_json_test.go for the other Proposed* structs.
func TestProposedReviewMark_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"requirement_id": "R-1",
		"reviewed_at": "2026-07-12",
		"review_after": "2027-01-12",
		"evidence": ["docs/audit-2026-07.md"]
	}`)
	var p ProposedReviewMark
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.RequirementID != "R-1" {
		t.Errorf("RequirementID = %q, want R-1", p.RequirementID)
	}
	if p.ReviewedAt != "2026-07-12" {
		t.Errorf("ReviewedAt = %q, want 2026-07-12", p.ReviewedAt)
	}
	if p.ReviewAfter != "2027-01-12" {
		t.Errorf("ReviewAfter = %q, want 2027-01-12", p.ReviewAfter)
	}
	if len(p.Evidence) != 1 || p.Evidence[0] != "docs/audit-2026-07.md" {
		t.Errorf("Evidence = %v, want [docs/audit-2026-07.md]", p.Evidence)
	}
	if p.Kind() != KindReviewMark {
		t.Errorf("Kind() = %q, want %q", p.Kind(), KindReviewMark)
	}
	if p.TargetAnchor() != "R-1" {
		t.Errorf("TargetAnchor() = %q, want R-1", p.TargetAnchor())
	}
}

// TestProposedReviewMark_UnmarshalRejectsUnknownFields exercises the same
// strict-decode path apply_proposal.go's unmarshalProposal uses (via
// DisallowUnknownFields), guarding against a stale/mistyped field name
// silently no-op'ing instead of failing loudly.
func TestProposedReviewMark_UnmarshalRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	dec := json.NewDecoder(strings.NewReader(`{"requirement_id":"R-1","reviewed_at":"2026-07-12","bogus_field":"x"}`))
	dec.DisallowUnknownFields()
	var p ProposedReviewMark
	if err := dec.Decode(&p); err == nil {
		t.Fatal("expected strict decode to reject unknown field bogus_field")
	}
}

func TestApply_ReviewMark_StampsLastReviewedAt(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	before := reload(t, path)
	r1Before, ok := findReq(before, "R-1")
	if !ok {
		t.Fatalf("R-1 missing before apply")
	}
	if r1Before.LastReviewedAt != "" {
		t.Fatalf("fixture precondition: R-1.LastReviewedAt should start empty, got %q", r1Before.LastReviewedAt)
	}

	p := ProposedReviewMark{
		RequirementID: "R-1",
		ReviewedAt:    today,
		ReviewAfter:   "2027-01-12",
		Evidence:      []string{"docs/audit-2026-07.md"},
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	after := reload(t, path)
	r1After, ok := findReq(after, "R-1")
	if !ok {
		t.Fatalf("R-1 missing after apply")
	}
	if r1After.LastReviewedAt != today {
		t.Errorf("LastReviewedAt = %q, want %q", r1After.LastReviewedAt, today)
	}
	if r1After.ReviewAfter != "2027-01-12" {
		t.Errorf("ReviewAfter = %q, want 2027-01-12", r1After.ReviewAfter)
	}
	if len(r1After.Evidence) != 1 || r1After.Evidence[0] != "docs/audit-2026-07.md" {
		t.Errorf("Evidence = %v, want [docs/audit-2026-07.md]", r1After.Evidence)
	}
	// Content fields must be untouched by a review mark.
	if r1After.Claim != r1Before.Claim {
		t.Errorf("Claim changed by a ReviewMark: got %q, want unchanged %q", r1After.Claim, r1Before.Claim)
	}
	if r1After.Status != r1Before.Status {
		t.Errorf("Status changed by a ReviewMark: got %q, want unchanged %q", r1After.Status, r1Before.Status)
	}
	if len(r1After.History) != 1 {
		t.Fatalf("History len = %d, want 1 derived entry", len(r1After.History))
	}
	if r1After.History[0].At != today {
		t.Errorf("History[0].At = %q, want %q", r1After.History[0].At, today)
	}
}

func TestApply_ReviewMark_DefaultsReviewedAtToToday(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedReviewMark{
		RequirementID: "R-2",
		ReviewAfter:   "2027-06-01",
	}
	if err := Apply(path, today, p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	g := reload(t, path)
	r, ok := findReq(g, "R-2")
	if !ok {
		t.Fatalf("R-2 missing")
	}
	if r.LastReviewedAt != today {
		t.Errorf("LastReviewedAt = %q, want writer-time default %q", r.LastReviewedAt, today)
	}
}

func TestApply_ReviewMark_UnknownRequirementFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedReviewMark{RequirementID: "R-does-not-exist", ReviewedAt: today}
	assertApplyFails(t, path, p, "not found")
}

func TestApply_ReviewMark_EmptyProposalFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedReviewMark{RequirementID: "R-1"}
	assertApplyFails(t, path, p, "no-op")
}

func TestApply_ReviewMark_MissingRequirementIDFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	p := ProposedReviewMark{ReviewedAt: today}
	assertApplyFails(t, path, p, "requirement_id")
}
