package query

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/freshness"
)

func TestBrief_RequirementIncludesContextAndFreshness(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	const today = "2026-07-13"

	b, err := Brief(g, "R-alpha", today)
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if b.Kind != KindRequirement {
		t.Fatalf("Kind = %q, want %q", b.Kind, KindRequirement)
	}
	if b.ID != "R-alpha" {
		t.Fatalf("ID = %q, want R-alpha", b.ID)
	}
	if b.Requirement == nil {
		t.Fatal("Requirement card is nil")
	}
	if b.Requirement.Claim == "" {
		t.Error("Requirement card has empty claim")
	}

	// Freshness must match what freshness.Classify independently computes.
	wantStatus := freshness.Classify(g.Requirements[0], today)
	if b.Freshness == nil {
		t.Fatal("Freshness is nil for a Requirement anchor")
	}
	if b.Freshness.Status != string(wantStatus) {
		t.Errorf("Freshness.Status = %q, want %q", b.Freshness.Status, wantStatus)
	}
	// R-alpha has review_after=2026-08-01, today=2026-07-13 → DUE-SOON
	// (within the 30-day window). Assert the exact value to pin the behavior.
	if b.Freshness.Status != string(freshness.DueSoon) {
		t.Errorf("expected DUE-SOON, got %q", b.Freshness.Status)
	}

	wantOverdue := freshness.OverdueDays(g.Requirements[0], today)
	if b.Freshness.OverdueDays != wantOverdue {
		t.Errorf("OverdueDays = %d, want %d", b.Freshness.OverdueDays, wantOverdue)
	}
}

func TestBrief_RequirementNeighborsAssumptionsConflicts(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	b, err := Brief(g, "R-alpha", "2026-07-13")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}

	// Neighbors must include outgoing refines->R-beta.
	var foundRefines bool
	for _, n := range b.Neighbors {
		if n.ID == "R-beta" && n.RelKind == "refines" {
			foundRefines = true
		}
	}
	if !foundRefines {
		t.Errorf("expected refines->R-beta in Neighbors, got %v", b.Neighbors)
	}

	// Assumptions must be the full cards (2 for R-alpha).
	if len(b.Assumptions) != 2 {
		t.Errorf("expected 2 full assumption cards, got %d", len(b.Assumptions))
	}

	// Conflicts must include C-ab.
	if len(b.Conflicts) != 1 || b.Conflicts[0].ID != "C-ab" {
		t.Errorf("expected member of C-ab, got %v", b.Conflicts)
	}

	// SharedAssumptionWith must include R-beta (shares A-shared).
	var sharesWithBeta bool
	for _, s := range b.SharedAssumptionWith {
		if s.ID == "R-beta" {
			sharesWithBeta = true
		}
	}
	if !sharesWithBeta {
		t.Errorf("expected R-beta in SharedAssumptionWith, got %v", b.SharedAssumptionWith)
	}
}

func TestBrief_RequirementMatchesContext(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	b, err := Brief(g, "R-alpha", "2026-07-13")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	cc, err := Context(g, "R-alpha")
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	// Brief's enrichment fields must be identical to Context's.
	if len(b.Neighbors) != len(cc.Relations) {
		t.Errorf("Neighbors len %d != Context.Relations len %d", len(b.Neighbors), len(cc.Relations))
	}
	if len(b.Assumptions) != len(cc.Assumptions) {
		t.Errorf("Assumptions len %d != Context.Assumptions len %d", len(b.Assumptions), len(cc.Assumptions))
	}
	if len(b.Conflicts) != len(cc.Conflicts) {
		t.Errorf("Conflicts len %d != Context.Conflicts len %d", len(b.Conflicts), len(cc.Conflicts))
	}
	if len(b.SharedAssumptionWith) != len(cc.SharedAssumptionWith) {
		t.Errorf("SharedAssumptionWith len %d != Context len %d", len(b.SharedAssumptionWith), len(cc.SharedAssumptionWith))
	}
}

func TestBrief_RequirementOverdue(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	// today past review_after (2026-08-01) → OVERDUE
	b, err := Brief(g, "R-alpha", "2026-09-01")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if b.Freshness.Status != string(freshness.Overdue) {
		t.Errorf("expected OVERDUE, got %q", b.Freshness.Status)
	}
	if b.Freshness.OverdueDays <= 0 {
		t.Errorf("expected positive OverdueDays for OVERDUE, got %d", b.Freshness.OverdueDays)
	}
}

func TestBrief_RequirementNeverReviewed(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	// R-beta has no review_after and no last_reviewed_at → NEVER-REVIEWED
	b, err := Brief(g, "R-beta", "2026-07-13")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if b.Freshness.Status != string(freshness.NeverReviewed) {
		t.Errorf("expected NEVER-REVIEWED, got %q", b.Freshness.Status)
	}
}

func TestBrief_RequirementFreshnessNoDrift(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	// Assert Brief's freshness exactly equals freshness.Classify for every
	// requirement and several dates — proving no drift between the two
	// independent computations.
	dates := []string{"2026-01-01", "2026-07-13", "2026-08-01", "2026-09-15", "2027-01-01"}
	for _, r := range g.Requirements {
		for _, today := range dates {
			wantStatus := freshness.Classify(r, today)
			wantOverdue := freshness.OverdueDays(r, today)
			b, err := Brief(g, r.ID, today)
			if err != nil {
				t.Fatalf("Brief(%s, %s): %v", r.ID, today, err)
			}
			if b.Freshness.Status != string(wantStatus) {
				t.Errorf("Brief(%s,%s) Status = %q, want %q", r.ID, today, b.Freshness.Status, wantStatus)
			}
			if b.Freshness.OverdueDays != wantOverdue {
				t.Errorf("Brief(%s,%s) OverdueDays = %d, want %d", r.ID, today, b.Freshness.OverdueDays, wantOverdue)
			}
		}
	}
}

func TestBrief_Conflict(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	b, err := Brief(g, "C-ab", "2026-07-13")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if b.Kind != KindConflict {
		t.Fatalf("Kind = %q, want %q", b.Kind, KindConflict)
	}
	if b.Conflict == nil {
		t.Fatal("Conflict card is nil")
	}
	if b.Conflict.Axis != "test-axis" {
		t.Errorf("Axis = %q, want test-axis", b.Conflict.Axis)
	}
	if b.Freshness != nil {
		t.Error("Freshness must be nil for a Conflict anchor")
	}
	if b.Requirement != nil {
		t.Error("Requirement must be nil for a Conflict anchor")
	}
	if b.Assumption != nil {
		t.Error("Assumption must be nil for a Conflict anchor")
	}
	// Neighbors must include both members + shared_assumption.
	var foundMember, foundShared bool
	for _, n := range b.Neighbors {
		if n.RelKind == "member" {
			foundMember = true
		}
		if n.RelKind == "shared_assumption" {
			foundShared = true
		}
	}
	if !foundMember {
		t.Errorf("expected member neighbor in %v", b.Neighbors)
	}
	if !foundShared {
		t.Errorf("expected shared_assumption neighbor in %v", b.Neighbors)
	}
}

func TestBrief_Assumption(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	b, err := Brief(g, "A-shared", "2026-07-13")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if b.Kind != KindAssumption {
		t.Fatalf("Kind = %q, want %q", b.Kind, KindAssumption)
	}
	if b.Assumption == nil {
		t.Fatal("Assumption card is nil")
	}
	if b.Assumption.Statement == "" {
		t.Error("Assumption card has empty statement")
	}
	if b.Freshness != nil {
		t.Error("Freshness must be nil for an Assumption anchor")
	}
	// Neighbors must include assumed_by entries (R-alpha and R-beta both
	// assume A-shared).
	var foundAssumedBy bool
	for _, n := range b.Neighbors {
		if n.RelKind == "assumed_by" {
			foundAssumedBy = true
		}
	}
	if !foundAssumedBy {
		t.Errorf("expected assumed_by neighbor in %v", b.Neighbors)
	}
}

func TestBrief_NotFound(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	_, err := Brief(g, "R-does-not-exist", "2026-07-13")
	if err == nil {
		t.Fatal("expected error for unknown anchor")
	}
	if _, ok := err.(*ErrNotFound); !ok {
		t.Errorf("expected *ErrNotFound, got %T: %v", err, err)
	}
}

func TestBrief_NotFoundUnrecognizedPrefix(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	_, err := Brief(g, "X-bogus", "2026-07-13")
	if err == nil {
		t.Fatal("expected error for unrecognized prefix")
	}
	if _, ok := err.(*ErrNotFound); !ok {
		t.Errorf("expected *ErrNotFound, got %T: %v", err, err)
	}
}

func TestBrief_NeighborsNeverNil(t *testing.T) {
	t.Parallel()
	g := fixtureGraph()
	// Even a requirement with no relations should have non-nil Neighbors.
	b, err := Brief(g, "R-gamma", "2026-07-13")
	if err != nil {
		t.Fatalf("Brief: %v", err)
	}
	if b.Neighbors == nil {
		t.Error("Neighbors must be non-nil (normalized to [])")
	}
}
