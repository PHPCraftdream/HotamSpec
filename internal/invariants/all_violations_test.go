package invariants

import (
	"reflect"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestAllViolations_EmptyGraphNoViolations(t *testing.T) {
	t.Parallel()
	if vs := AllViolations(&ontology.Graph{}); len(vs) != 0 {
		t.Fatalf("expected no violations on empty graph, got %v", vs)
	}
}

func TestAllViolations_WellFormedGraphNoViolations(t *testing.T) {
	t.Parallel()
	g := graphWithConflict(baseConflict(), nil)
	if vs := AllViolations(g); len(vs) != 0 {
		t.Fatalf("expected no violations on well-formed graph, got %v", vs)
	}
}

func TestAllViolations_DeterministicOrder(t *testing.T) {
	t.Parallel()
	bad := baseConflict()
	bad.Members = []string{"R-1"}
	bad.Resolver = "sa"
	bad.ID = "C-bad"
	g := graphWithConflict(bad, nil)
	first := AllViolations(g)
	for i := 0; i < 20; i++ {
		if got := AllViolations(g); !reflect.DeepEqual(first, got) {
			t.Fatalf("AllViolations order is not deterministic:\nfirst: %v\ngot:   %v", first, got)
		}
	}
	if len(first) == 0 {
		t.Fatalf("expected some violations on the broken graph")
	}
}
