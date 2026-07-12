package generator

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/invariants"
)

// TestFixtureGraphHasNoViolations guards testdata/fixture-graph.json's
// well-formedness: the fixture is meant to exercise real template branches
// through a legitimate graph, not a broken one. If a future edit to the
// fixture introduces a structural violation (bad conflict identity, dangling
// ref, HELD without decided_by, etc.), this test — not a downstream
// byte-identity diff — should be the one that fails and explains why.
func TestFixtureGraphHasNoViolations(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	violations := invariants.AllViolations(g)
	for _, v := range violations {
		t.Errorf("fixture graph violation: check=%s id=%s msg=%s", v.Check, v.ID, v.Message)
	}
}
