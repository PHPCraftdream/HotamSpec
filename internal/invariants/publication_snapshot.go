package invariants

import (
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/evidence"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// PriorToPostProcessViolationsForPublication computes the phase-one violation
// snapshot for a generation that will publish its staged document projections.
// Projection checks for files in that staged bundle are omitted because those
// checks describe the on-disk state being replaced, not the state after publish.
// When SPEC is deliberately not regenerated, its existing full freshness gate
// remains in the snapshot; a skipped SPEC bundle can still be stale.
func PriorToPostProcessViolationsForPublication(g *ontology.Graph, includeSpec bool) []Violation {
	return withGraphInvocation(g, func(view *ontology.Graph) []Violation {
		return runViolations(view, publicationCandidates(includeSpec), time.Now().Format("2006-01-02"))
	})
}

// PriorToPostProcessViolationsForPublicationWithEvidence returns the same
// post-publication phase-one snapshot plus the one raw evidence report collected
// in the same invocation scope. A conformance audit already performed during
// phase one and any caller using this returned report share one collection.
func PriorToPostProcessViolationsForPublicationWithEvidence(g *ontology.Graph, includeSpec bool) ([]Violation, evidence.Report, error) {
	var violations []Violation
	var report evidence.Report
	var collectErr error
	withGraphInvocation(g, func(view *ontology.Graph) []Violation {
		violations = runViolations(view, publicationCandidates(includeSpec), time.Now().Format("2006-01-02"))
		report, collectErr = collectInvocationEvidence(view)
		return nil
	})
	return violations, report, collectErr
}

func publicationCandidates(includeSpec bool) []Invariant {
	candidates := make([]Invariant, 0, len(All.All()))
	for _, inv := range All.All() {
		if inv.PostProcessCheck != nil {
			continue
		}
		if inv.ComparesOnDiskProjection {
			if includeSpec || inv.Name != "check_spec_md_current" {
				continue
			}
		}
		candidates = append(candidates, inv)
	}
	return candidates
}
