package diagnose

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestEmptyContentCalmBanner enforces R-empty-content-calm-banner: running
// diagnosis (DiagnoseSignals / TopAction) over a completely empty graph must
// produce no error, no panic, and a calm/quiet result rather than crashing or
// emitting noisy spurious output.
//
// EXACT RULE (mechanically checked), two halves:
//  1. DiagnoseSignals(&ontology.Graph{}) returns without panicking and yields
//     zero signals (a quiet result — nothing to report).
//  2. TopAction(&ontology.Graph{}) returns the calm "none — graph clean" signal,
//     not an error string and not a noisy imperative.
//
// Together these are the engine-level calm-banner guarantee that `hotam
// what-now` renders over an empty-but-present graph.
//
// Discrimination: see TestEmptyContentCalmBanner_NotNoisyOnContent — a graph
// with real content must produce a non-calm TopAction, proving the calm result
// is specific to emptiness rather than a hardcoded constant.
func TestEmptyContentCalmBanner(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}

	// Half 1: DiagnoseSignals must not panic and must be quiet.
	signals := safeDiagnoseSignals(t, g)
	if len(signals) != 0 {
		t.Fatalf("R-empty-content-calm-banner: empty graph must yield zero diagnose signals (calm/quiet), got %d: %v",
			len(signals), signals)
	}

	// Half 2: TopAction must render the calm banner, not an error.
	const calm = "none — graph clean"
	if ta := TopAction(g); ta != calm {
		t.Errorf("R-empty-content-calm-banner: TopAction(empty) = %q, want calm %q", ta, calm)
	}
}

// safeDiagnoseSignals runs DiagnoseSignals and fails the test if it panics,
// making the no-panic half of the calm-banner guarantee explicit.
func safeDiagnoseSignals(t *testing.T, g *ontology.Graph) (signals []Signal) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("R-empty-content-calm-banner: DiagnoseSignals panicked over the graph: %v", r)
		}
	}()
	return DiagnoseSignals(g)
}

// TestEmptyContentCalmBanner_NotNoisyOnContent is the non-vacuity control: a
// graph carrying real content must yield a non-calm TopAction, proving the calm
// "none — graph clean" result is specific to emptiness, not a constant returned
// regardless of input.
func TestEmptyContentCalmBanner_NotNoisyOnContent(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Conflicts: []ontology.Conflict{{
			ID:        "C-1",
			Axis:      "cost-vs-flexibility",
			Context:   "scenario",
			Steward:   "steward",
			Lifecycle: ontology.ConflictDETECTED,
			Members:   []string{"R-1", "R-2"},
		}},
		Requirements: []ontology.Requirement{settledReq("R-1"), settledReq("R-2")},
	}
	ta := TopAction(g)
	if ta == "" {
		t.Fatalf("R-empty-content-calm-banner non-vacuity: TopAction over content must be non-empty")
	}
	if ta == "none — graph clean" {
		t.Fatalf("R-empty-content-calm-banner non-vacuity: a graph with a stalled conflict must NOT render the calm banner, got %q", ta)
	}
}
