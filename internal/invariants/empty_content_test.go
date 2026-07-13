package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// TestEmptyContentWellFormed enforces R-empty-content-wellformed: a completely
// empty graph (zero requirements, zero conflicts, zero everything) is a
// legitimate, well-formed starting state — running the full invariants checker
// (AllViolations) over it must return zero violations.
//
// EXACT RULE (mechanically checked): AllViolations(&ontology.Graph{}) returns an
// empty slice. An empty graph has nothing for any per-element invariant to
// violate, so the well-formedness property holds by construction.
//
// Discrimination: see TestEmptyContentWellFormed_CheckerNotNoop — the same
// checker over a graph carrying one malformed entity instance MUST emit at least
// one violation, proving the empty-graph pass is not because AllViolations is a
// no-op.
func TestEmptyContentWellFormed(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{}
	if !g.IsEmpty() {
		t.Fatalf("precondition: graph must be completely empty")
	}
	if vs := AllViolations(g); len(vs) != 0 {
		t.Fatalf("R-empty-content-wellformed: an empty graph must pass all invariants with zero violations, got %d: %v",
			len(vs), vs)
	}
}

// TestEmptyContentWellFormed_CheckerNotNoop is the non-vacuity control: the
// identical checker must fire on a graph carrying a malformed element, proving
// the zero-violation result above is meaningful rather than a trivial no-op.
func TestEmptyContentWellFormed_CheckerNotNoop(t *testing.T) {
	t.Parallel()
	g := &ontology.Graph{
		Entities: []ontology.EntityInstance{entityInstance("ENT-acme", "thing", "ACTIVE")},
	}
	if vs := AllViolations(g); len(vs) == 0 {
		t.Fatalf("R-empty-content-wellformed non-vacuity: a malformed entity instance must produce at least one violation, got none — AllViolations appears to be a no-op over this graph")
	}
}
