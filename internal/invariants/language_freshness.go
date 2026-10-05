package invariants

import (
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkLanguageOutputsCurrentUnwired is the invariant-package fallback. The
// real renderer comparison is wired by cmd/hotam, which can import generator
// without introducing the generator <-> invariants import cycle.
func checkLanguageOutputsCurrentUnwired(*ontology.Graph, []Violation) []Violation { return nil }

func checkLanguageOutputsCurrentUnwiredAsOf(g *ontology.Graph, prior []Violation, _ string) []Violation {
	return checkLanguageOutputsCurrentUnwired(g, prior)
}

var _ = All.MustRegister("check_language_outputs_current", Invariant{
	Name:                     "check_language_outputs_current",
	ComparesOnDiskProjection: true,
	Canon:                    methodology.Domain,
	Claim:                    "every explicitly configured language has a byte-fresh complete rendered document view, including all promised SPEC indexes/shards, shared framework docs, and localized boot crystals; evidence/finding views are included when the shared evidence report is materialized.",
	Rule:                     "IF Graph.Languages is explicitly nonempty, render the whole view set from one graph/config and shared SPEC/evidence snapshots; compare every expected output against the canonical inventory. Evidence/finding output remains opt-in to its shared report store; legacy domains retain existing opt-ins.",
	Why:                      "Checking only the legacy SPEC.md filename misses stale secondary locales and shards. An explicit language set owns every rendered view without silently making a separate evidence write mandatory.",
	PostProcessCheck:         checkLanguageOutputsCurrentUnwired,
	PostProcessCheckAsOf:     checkLanguageOutputsCurrentUnwiredAsOf,
})
