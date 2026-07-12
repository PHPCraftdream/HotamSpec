package invariants

import (
	"sync"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func AllViolations(g *ontology.Graph) []Violation {
	all := All.All()
	invs := make([]Invariant, 0, len(all))
	for _, inv := range all {
		if inv.IsDelegator {
			continue
		}
		invs = append(invs, inv)
	}
	results := make([][]Violation, len(invs))
	var wg sync.WaitGroup
	for i, inv := range invs {
		wg.Add(1)
		go func(idx int, in Invariant) {
			defer wg.Done()
			results[idx] = in.Check(g)
		}(i, inv)
	}
	wg.Wait()
	var out []Violation
	for _, r := range results {
		out = append(out, r...)
	}
	return out
}
