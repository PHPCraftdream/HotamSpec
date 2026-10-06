package invariants

import (
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// CrystalReaderCheckNames is the canonical set of invariant names that read
// generated crystal content and participate in gen-spec's fixpoint.
var CrystalReaderCheckNames = []string{"check_orientation_faq_answered", "check_operator_within_budget"}

// CrystalReaderViolations evaluates the publication checks that read the
// crystal or root CLAUDE.md against their current on-disk contents.
func CrystalReaderViolations(g *ontology.Graph) []Violation {
	return withGraphInvocation(g, func(view *ontology.Graph) []Violation {
		var candidates []Invariant
		for _, name := range CrystalReaderCheckNames {
			inv, ok := All.Get(name)
			if ok && inv.Check != nil {
				candidates = append(candidates, *inv)
			}
		}
		return runViolations(view, candidates, time.Now().Format("2006-01-02"))
	})
}
