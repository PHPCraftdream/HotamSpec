package diagnose

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
)

const domainGraphPath = "../../domains/hotam-spec-self/graph.json"

func TestDiagnoseSignals_RealGraphNoPanic(t *testing.T) {
	t.Parallel()
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	signals := DiagnoseSignals(g)
	// A signal-free graph is valid: TopAction/what-now report "none — graph
	// clean" when len(signals) == 0. The call itself is the no-panic smoke
	// test; assert sort order only when signals are present.
	for i := 1; i < len(signals); i++ {
		if signals[i].Priority < signals[i-1].Priority {
			t.Errorf("signal %d (P%d) out of order after signal %d (P%d)",
				i, signals[i].Priority, i-1, signals[i-1].Priority)
			break
		}
	}
}

func TestTopAction_RealGraph(t *testing.T) {
	t.Parallel()
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	ta := TopAction(g)
	if ta == "" {
		t.Error("TopAction returned empty string")
	}
	t.Logf("real graph top action: %s", ta)
}

func TestAllFindings_RealGraphNoPanic(t *testing.T) {
	t.Parallel()
	g, err := loader.LoadGraph(domainGraphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	fs := AllFindings(g)
	t.Logf("real graph findings: %d", len(fs))
}
