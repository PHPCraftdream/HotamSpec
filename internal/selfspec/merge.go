package selfspec

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// MergeIntoGraph applies every registered Requirements entry onto g, IN
// PLACE: for each registered requirement ID that exists in g.Requirements,
// the STRUCTURAL fields (everything requirements_pilot.go's codegen wrote —
// Claim, Owner, Status, Why, Assumptions, Relations, Enforcement, EnforcedBy,
// MTag, Enforceability, Summary, CreatedAt, SettledAt, SourceRefs, DeclOrder,
// BlockedOn, ImplementedBy, VerifiedBy) are REPLACED from the registry value,
// while the EVENT fields (History, GateSignoffs, LastReviewedAt,
// ReviewAfter, Evidence) are PASSED THROUGH untouched from the graph's
// existing node.
//
// Phase 0 scope, deliberately narrow:
//   - A registry entry whose ID is absent from g.Requirements is an ERROR —
//     Phase 0 never creates graph nodes, only mirrors ones that already
//     exist (creation is a later phase's concern).
//   - A graph.Requirements entry whose ID is NOT registered is left
//     COMPLETELY untouched — the ~280+ requirements outside the Phase 0
//     pilot subset are unaffected by this function.
//   - Pure and deterministic: same g + same Requirements registry state
//     always produces the same result; MergeIntoGraph itself never reads a
//     clock, a file, or global mutable state beyond the Requirements
//     registry it was built to mirror.
//
// g must be non-nil; its Requirements slice is mutated in place (each
// matched element is replaced with a new ontology.Requirement value built
// from the registry entry plus the old element's event fields).
func MergeIntoGraph(g *ontology.Graph) error {
	if g == nil {
		return fmt.Errorf("selfspec: MergeIntoGraph: nil graph")
	}

	indexByID := make(map[string]int, len(g.Requirements))
	for i, r := range g.Requirements {
		indexByID[r.ID] = i
	}

	for _, id := range registeredIDsSorted() {
		reg, ok := Requirements.Get(id)
		if !ok {
			// unreachable: id came from Requirements itself.
			continue
		}
		idx, found := indexByID[id]
		if !found {
			return fmt.Errorf("selfspec: MergeIntoGraph: registered requirement %q not found in graph — Phase 0 never creates graph nodes, only mirrors existing ones", id)
		}
		existing := g.Requirements[idx]
		merged := *reg // copy: structural fields from the registry
		// Event fields pass through untouched from the graph's existing node.
		merged.LastReviewedAt = existing.LastReviewedAt
		merged.ReviewAfter = existing.ReviewAfter
		merged.Evidence = existing.Evidence
		merged.History = existing.History
		merged.GateSignoffs = existing.GateSignoffs
		g.Requirements[idx] = merged
	}
	return nil
}

// registeredIDsSorted returns every ID currently registered in Requirements,
// in the registry's own registration order (via All(), which is stable and
// duplicate-free by construction — MustRegister panics on a duplicate name).
// Order does not affect MergeIntoGraph's result (each registered ID is
// applied independently, keyed by ID), but iterating deterministically keeps
// error reporting order stable across runs.
func registeredIDsSorted() []string {
	all := Requirements.All()
	ids := make([]string, len(all))
	for i, r := range all {
		ids[i] = r.ID
	}
	return ids
}
