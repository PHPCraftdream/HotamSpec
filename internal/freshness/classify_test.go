package freshness

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const testToday = "2026-07-12"

func TestClassify_TableDriven(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		lastReviewedAt string
		reviewAfter    string
		want           Status
	}{
		{"both empty is never reviewed", "", "", NeverReviewed},
		{"reviewed once, no next date scheduled is fresh", "2026-01-01", "", Fresh},
		{"review_after exactly one day before today is overdue", "", "2026-07-11", Overdue},
		{"review_after far in the past is overdue", "2020-01-01", "2020-06-01", Overdue},
		{"review_after equal to today is due-soon (boundary, not overdue)", "", "2026-07-12", DueSoon},
		{"review_after 29 days out is due-soon (inside window)", "", "2026-08-10", DueSoon},
		{"review_after 30 days out is fresh (window is exclusive upper bound)", "", "2026-08-11", Fresh},
		{"review_after one day out is due-soon", "", "2026-07-13", DueSoon},
		{"review_after far in the future is fresh", "2026-07-01", "2027-01-01", Fresh},
		{"review_after one day after due-soon boundary is fresh", "", "2026-08-12", Fresh},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := ontology.Requirement{
				ID:             "R-x",
				Status:         ontology.StatusSETTLED,
				LastReviewedAt: tc.lastReviewedAt,
				ReviewAfter:    tc.reviewAfter,
			}
			got := Classify(r, testToday)
			if got != tc.want {
				t.Errorf("Classify(last=%q, review_after=%q, today=%q) = %q, want %q",
					tc.lastReviewedAt, tc.reviewAfter, testToday, got, tc.want)
			}
		})
	}
}

func TestOverdueDays(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		reviewAfter string
		want        int
	}{
		{"empty review_after", "", 0},
		{"review_after equal to today", "2026-07-12", 0},
		{"review_after in the future", "2026-08-01", 0},
		{"review_after one day before today", "2026-07-11", 1},
		{"review_after 10 days before today", "2026-07-02", 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := ontology.Requirement{ReviewAfter: tc.reviewAfter}
			got := OverdueDays(r, testToday)
			if got != tc.want {
				t.Errorf("OverdueDays(review_after=%q, today=%q) = %d, want %d",
					tc.reviewAfter, testToday, got, tc.want)
			}
		})
	}
}

func TestClassify_NonSettledIgnoredByClassifyGraph(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-draft", Status: ontology.StatusDRAFT},
			{ID: "R-open", Status: "OPEN(some question)"},
			{ID: "R-settled-overdue", Status: ontology.StatusSETTLED, ReviewAfter: "2020-01-01"},
		},
	}
	got := ClassifyGraph(g, testToday)
	if len(got) != 1 {
		t.Fatalf("ClassifyGraph len = %d, want 1 (only SETTLED requirements classified); got %+v", len(got), got)
	}
	if got[0].Requirement.ID != "R-settled-overdue" {
		t.Errorf("ClassifyGraph[0].Requirement.ID = %q, want R-settled-overdue", got[0].Requirement.ID)
	}
	if got[0].Status != Overdue {
		t.Errorf("ClassifyGraph[0].Status = %q, want OVERDUE", got[0].Status)
	}
}

func TestClassifyGraph_SortOrder(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-fresh", Status: ontology.StatusSETTLED, ReviewAfter: "2027-01-01"},
			{ID: "R-due-soon", Status: ontology.StatusSETTLED, ReviewAfter: "2026-07-20"},
			{ID: "R-never", Status: ontology.StatusSETTLED},
			{ID: "R-overdue-shallow", Status: ontology.StatusSETTLED, ReviewAfter: "2026-07-10"},
			{ID: "R-overdue-deep", Status: ontology.StatusSETTLED, ReviewAfter: "2026-01-01"},
		},
	}
	got := ClassifyGraph(g, testToday)
	wantOrder := []string{"R-overdue-deep", "R-overdue-shallow", "R-never", "R-due-soon", "R-fresh"}
	if len(got) != len(wantOrder) {
		t.Fatalf("ClassifyGraph len = %d, want %d", len(got), len(wantOrder))
	}
	for i, wantID := range wantOrder {
		if got[i].Requirement.ID != wantID {
			t.Errorf("ClassifyGraph[%d].Requirement.ID = %q, want %q (full order: %v)",
				i, got[i].Requirement.ID, wantID, idsOf(got))
		}
	}
}

func idsOf(cs []Classification) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = string(c.Status) + ":" + c.Requirement.ID
	}
	return out
}
