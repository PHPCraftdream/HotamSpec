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

// PublicationViolationsFromPhaseOne filters an already-computed phase-one
// (ordinary-check) violation list down to the publication snapshot's
// candidate set with includeSpec == false — the same filter
// publicationCandidates(false) applies when collecting, but as a pure
// filter over violations the caller already holds, so a caller that needs
// the publication flavor a second time (e.g. a post-process check
// rendering a projection for comparison) does not re-run the ordinary
// checks, including the expensive check_spec_md_current test execution.
// Violations whose check is a registered ComparesOnDiskProjection invariant
// are dropped, EXCEPT check_spec_md_current, mirroring
// publicationCandidates(false) exactly.
//
// SINGLE FLAVOR SELECTOR: this function is THE one shared entry point for
// every comparative (fresh-render) projection in the codebase — genSpec's
// write path (via PriorToPostProcessViolationsForPublication*) AND every
// freshness check's comparative render (check_domain_claude_md_current,
// check_agent_context_md_current, check_language_outputs_current) must feed
// the SAME publication flavor into their renders. A check that renders from
// the unfiltered phase-one list instead reports false staleness whenever an
// unrelated on-disk projection is transiently stale, even though a re-run
// would rewrite the file byte-identically (review finding P3-13).
func PublicationViolationsFromPhaseOne(prior []Violation) []Violation {
	drop := make(map[string]bool, len(All.All()))
	for _, inv := range All.All() {
		if inv.ComparesOnDiskProjection && inv.Name != "check_spec_md_current" {
			drop[inv.Name] = true
		}
	}
	filtered := make([]Violation, 0, len(prior))
	for _, v := range prior {
		if !drop[v.Check] {
			filtered = append(filtered, v)
		}
	}
	return filtered
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
