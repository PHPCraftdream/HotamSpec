package selfspec

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

// MergeIntoGraph applies every entry of reg onto g, IN PLACE: for each
// registered requirement ID that exists in g.Requirements, the STRUCTURAL
// fields (everything the requirements_<topic>.go codegen wrote — Claim,
// Owner, Status, Why, Assumptions, Relations, Enforcement, EnforcedBy, MTag,
// Enforceability, Summary, CreatedAt, SettledAt, SourceRefs, DeclOrder,
// BlockedOn, ImplementedBy, VerifiedBy, SourceLinks, Coverage, ClaimTexts,
// AtomKind, Cases, ClauseLinks, Strength, Applicability, Precedence) are
// replaced from the registry; event fields (History, GateSignoffs,
// LastReviewedAt, ReviewAfter, Evidence) pass through untouched from the graph.
//
// reg is an explicit parameter (task #366/RAC2 Phase B) rather than this
// package's own package-global Requirements var, so the identical merge
// mechanism can be reused against ANY *registry.Registry[ontology.Requirement]
// — this package's own self-hosting Requirements (via `hotam sync-self`) OR
// a consumer domain's own Go-authored registry, unmarshaled from its
// `spec/registrydump` subprocess output (via `hotam sync-domain`). Every
// caller inside THIS package still passes the package-global Requirements
// explicitly (see cmd/hotam/sync_self.go, a thin wrapper) — this refactor
// changes zero behavior for that caller, only how the registry value
// arrives.
//
// Scope, deliberately narrow (unchanged since Phase 0/RAC-0, task #344):
//   - A registry entry whose ID is absent from g.Requirements is an ERROR —
//     this package never creates graph nodes, only mirrors ones that already
//     exist (creation is a later phase's concern, see task #346/RAC-B).
//   - A graph.Requirements entry whose ID is NOT registered is left
//     COMPLETELY untouched. As of Phase A (task #345) the registry covers
//     all 301 real domains/hotam-spec-self/graph.json requirements, so this
//     case is now rare in practice against that domain — but MergeIntoGraph
//     itself stays a general mechanism: any graph (including a synthetic one
//     in a test, or a future not-yet-landed proposal's node) may contain IDs
//     the registry does not know about, and those are unaffected.
//   - Pure and deterministic: same g + same reg state always produces the
//     same result; MergeIntoGraph itself never reads a clock, a file, or
//     global mutable state beyond the reg registry it was handed.
//
// g must be non-nil; its Requirements slice is mutated in place (each
// matched element is replaced with a new ontology.Requirement value built
// from the registry entry plus the old element's event fields).
func MergeIntoGraph(g *ontology.Graph, reg *registry.Registry[ontology.Requirement]) error {
	if g == nil {
		return fmt.Errorf("selfspec: MergeIntoGraph: nil graph")
	}
	if reg == nil {
		return fmt.Errorf("selfspec: MergeIntoGraph: nil registry")
	}

	indexByID := make(map[string]int, len(g.Requirements))
	for i, r := range g.Requirements {
		indexByID[r.ID] = i
	}

	for _, id := range registeredIDsSorted(reg) {
		entry, ok := reg.Get(id)
		if !ok {
			// unreachable: id came from reg itself.
			continue
		}
		idx, found := indexByID[id]
		if !found {
			return fmt.Errorf("selfspec: MergeIntoGraph: registered requirement %q not found in graph — this package never creates graph nodes, only mirrors existing ones", id)
		}
		existing := g.Requirements[idx]
		merged := ontology.CloneRequirementMetadata(*entry)
		// Event fields pass through untouched from the graph's existing node.
		merged.LastReviewedAt = existing.LastReviewedAt
		merged.ReviewAfter = existing.ReviewAfter
		merged.Evidence = existing.Evidence
		merged.History = existing.History
		merged.GateSignoffs = existing.GateSignoffs
		// Authored in the new multilingual style: claim lives only in
		// claim_texts[default_language]; derive Claim so downstream
		// exact-match validation sees a consistent pair.
		merged = deriveClaimFromDefaultLanguage(merged, g.DefaultLanguage)
		g.Requirements[idx] = merged
	}
	return nil
}

// registeredIDsSorted returns every ID currently registered in reg, in the
// registry's own registration order (via All(), which is stable and
// duplicate-free by construction — MustRegister panics on a duplicate name).
// Order does not affect MergeIntoGraph's/SyncGraph's result (each registered
// ID is applied independently, keyed by ID), but iterating deterministically
// keeps error reporting order stable across runs.
func registeredIDsSorted(reg *registry.Registry[ontology.Requirement]) []string {
	all := reg.All()
	ids := make([]string, len(all))
	for i, r := range all {
		ids[i] = r.ID
	}
	return ids
}
