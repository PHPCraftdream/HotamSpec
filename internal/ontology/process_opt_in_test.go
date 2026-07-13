package ontology

import "testing"

// TestProcessIsOptIn enforces R-process-opt-in: the Process aspect is opt-in —
// a freshly constructed Graph (zero value) has an empty Processes slice, and no
// framework default Process is materialized.
//
// EXACT RULE (mechanically checked): (&Graph{}).Processes has length 0. The
// Process slice is plain content carried verbatim from a domain's graph.json by
// the loader (internal/loader maps DTO.Processes straight onto Graph.Processes
// with no default injection), so a domain that declares no Process gets none.
// The zero-value guarantee is the core claim: "TensionGraph.processes defaults
// to an empty tuple."
//
// Discrimination: see TestProcessIsOptIn_DetectsPopulated — a graph that
// declares a Process must report a non-empty slice, proving the length check
// discriminates rather than trivially always reporting zero.
func TestProcessIsOptIn(t *testing.T) {
	t.Parallel()
	var g Graph
	if len(g.Processes) != 0 {
		t.Fatalf("R-process-opt-in: a zero-value Graph must default to an empty Processes slice (opt-in), got %d",
			len(g.Processes))
	}
	if !g.IsEmpty() {
		t.Fatalf("R-process-opt-in: a zero-value Graph must be empty by construction")
	}
}

// TestProcessIsOptIn_DetectsPopulated is the non-vacuity control: a Graph that
// explicitly declares a Process must report a non-empty Processes slice, so the
// len==0 assertion above is meaningful.
func TestProcessIsOptIn_DetectsPopulated(t *testing.T) {
	t.Parallel()
	g := Graph{Processes: []Process{{ID: "PR-ship", Lifecycle: ProcessLifecycle}}}
	if len(g.Processes) != 1 {
		t.Fatalf("R-process-opt-in non-vacuity: a graph declaring one process must report len(Processes)==1, got %d",
			len(g.Processes))
	}
}
