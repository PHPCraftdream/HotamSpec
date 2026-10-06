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
	Claim:                    "every explicitly configured language has a byte-fresh complete rendered document view, including all promised SPEC indexes/shards, shared framework docs, and localized boot crystals; the comparison renders are fed the SAME publication-flavored violation snapshot (invariants.PublicationViolationsFromPhaseOne) genSpec writes with; evidence/finding views are included when the shared evidence report is materialized.",
	Rule:                     "IF Graph.Languages is explicitly nonempty, render the whole view set from one graph/config and shared SPEC/evidence snapshots, feeding every comparative render the publication-flavored phase-one snapshot (invariants.PublicationViolationsFromPhaseOne of this pass's phase-1 result) so an unrelated, transiently stale on-disk projection never makes a re-run-identical output read as stale; compare every expected output against the canonical inventory. Evidence/finding output remains opt-in to its shared report store; legacy domains retain existing opt-ins.",
	Why:                      "Checking only the legacy SPEC.md filename misses stale secondary locales and shards. An explicit language set owns every rendered view without silently making a separate evidence write mandatory. The comparison must use the publication flavor (review finding P3-13): genSpec writes localized views from that filtered snapshot, so the freshness comparison rendered from the unfiltered phase-1 list would flag false staleness whenever any other projection on disk is transiently stale.",
	PostProcessCheck:         checkLanguageOutputsCurrentUnwired,
	PostProcessCheckAsOf:     checkLanguageOutputsCurrentUnwiredAsOf,
})
