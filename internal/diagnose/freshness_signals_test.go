package diagnose

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestFreshnessSignals_NoSettledRequirements(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Requirements: []ontology.Requirement{
			{ID: "R-draft", Status: ontology.StatusDRAFT},
		},
	}
	signals := FreshnessSignals(g, "2026-07-12")
	if len(signals) != 0 {
		t.Errorf("expected 0 freshness signals for a graph with no SETTLED requirements, got %d: %+v", len(signals), signals)
	}
}

func TestFreshnessSignals_SummaryNotPerRequirement(t *testing.T) {
	t.Parallel()
	reqs := make([]ontology.Requirement, 0, 50)
	for i := 0; i < 50; i++ {
		r := settledReq("R-never-" + string(rune('a'+i%26)) + string(rune('0'+i/26)))
		reqs = append(reqs, r)
	}
	g := &ontology.Graph{Requirements: reqs}
	signals := FreshnessSignals(g, "2026-07-12")
	// One summary signal for NEVER-REVIEWED (never one per requirement, or
	// what-now would drown in 50 lines instead of the structural signals
	// that matter more).
	if len(signals) != 1 {
		t.Fatalf("expected exactly 1 summary signal (NEVER-REVIEWED only, no OVERDUE), got %d: %+v", len(signals), signals)
	}
	if !strings.Contains(signals[0].Message, "50") {
		t.Errorf("summary message should mention the count 50, got %q", signals[0].Message)
	}
	if signals[0].Priority != PAdvisory {
		t.Errorf("freshness signal priority = %d, want PAdvisory (%d)", signals[0].Priority, PAdvisory)
	}
}

func TestFreshnessSignals_OverdueAndNeverReviewedBothFire(t *testing.T) {
	t.Parallel()
	overdueReq := settledReq("R-overdue")
	overdueReq.ReviewAfter = "2020-01-01"
	neverReq := settledReq("R-never")
	g := &ontology.Graph{Requirements: []ontology.Requirement{overdueReq, neverReq}}
	signals := FreshnessSignals(g, "2026-07-12")
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals (1 OVERDUE summary + 1 NEVER-REVIEWED summary), got %d: %+v", len(signals), signals)
	}
	var sawOverdue, sawNever bool
	for _, s := range signals {
		if strings.Contains(s.Message, "OVERDUE") {
			sawOverdue = true
		}
		if strings.Contains(s.Message, "NEVER") {
			sawNever = true
		}
	}
	if !sawOverdue || !sawNever {
		t.Errorf("expected both OVERDUE and NEVER-REVIEWED summaries, got %+v", signals)
	}
}

func TestFreshnessSignals_IntegratedIntoDiagnoseSignals(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{Requirements: []ontology.Requirement{settledReq("R-1")}}
	signals := DiagnoseSignals(g)
	found := false
	for _, s := range signals {
		if strings.Contains(s.Message, "NEVER") && strings.Contains(s.Message, "hotam due") {
			found = true
		}
	}
	if !found {
		t.Error("expected DiagnoseSignals to surface a freshness NEVER-REVIEWED advisory for an unreviewed SETTLED requirement")
	}
}
