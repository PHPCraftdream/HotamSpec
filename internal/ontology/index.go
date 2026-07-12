package ontology

import "sort"

// GraphIndex is a read-only snapshot of ByID / reverse-reference lookups
// over a Graph, built once via BuildIndex(g). It exists to replace the
// linear scans (`for _, r := range g.Requirements { if r.ID == id ... }`)
// that were scattered across internal/invariants, internal/diagnose,
// internal/proposal and internal/query — harmless at small graph sizes but
// O(n) per lookup, and O(n^2) when a lookup runs inside another range over
// the graph (e.g. resolving every Conflict.Derived id, or every
// Conflict.Members id, against Requirements).
//
// LIFECYCLE / MUTATION CONTRACT (read this before caching a GraphIndex):
// a GraphIndex is a POINT-IN-TIME snapshot. It is built by copying id ->
// value (or id -> []id) into maps; it holds no pointer back into the
// Graph's slices and is never written to after BuildIndex returns. If the
// Graph is mutated after BuildIndex(g) was called (e.g. proposal.mutate.go
// appending a new Requirement, or editing one in place), the index does
// NOT observe the change — by design, this package does NOT attempt
// mutation-tracking or auto-invalidation. Two supported usage patterns:
//
//  1. Read-only queries (internal/query, internal/diagnose, internal/gate):
//     build the index once right after loader.LoadGraph and use it for the
//     lifetime of that in-memory Graph — nothing under those packages
//     mutates the Graph.
//  2. Mutate-then-recheck (internal/proposal/apply.go: AllViolations
//     before, mutate, AllViolations after): call BuildIndex again AFTER
//     the mutation if the post-mutate pass wants index-backed lookups.
//     Never reuse a pre-mutate index across a mutation boundary.
//
// Iteration over the reverse-index maps (RequirementsByAssumption,
// RelationsToRequirement) is NEVER used to produce output order directly —
// callers that need a stable order must sort the returned id slice
// themselves (map iteration order is intentionally not relied upon
// anywhere in this package).
type GraphIndex struct {
	RequirementByID map[string]Requirement
	ConflictByID    map[string]Conflict
	AssumptionByID  map[string]Assumption
	StakeholderByID map[string]Stakeholder

	// RequirementsByAssumption maps an Assumption id to the ids of every
	// Requirement whose Assumptions list cites it (reverse of
	// Requirement.Assumptions). Insertion order follows g.Requirements
	// order; sort before presenting if a different order is required.
	RequirementsByAssumption map[string][]string

	// RelationsToRequirement maps a Requirement id (the relation TARGET)
	// to every {SourceID, Kind} edge whose Relation.Target names it —
	// i.e. "who points at R-x". Reverse of Requirement.Relations.
	RelationsToRequirement map[string][]IncomingRelation
}

// IncomingRelation is one reverse-relation edge: SourceID is the
// Requirement whose Relations list carries the edge, Kind is the
// Relation.Kind value (e.g. "depends_on", "refines", "replaces").
type IncomingRelation struct {
	SourceID string
	Kind     string
}

// BuildIndex builds a GraphIndex by making a single pass over each of
// g.Requirements, g.Conflicts, g.Assumptions and g.Stakeholders. See the
// GraphIndex doc-comment for the read-only / rebuild-after-mutate contract.
func BuildIndex(g *Graph) *GraphIndex {
	idx := &GraphIndex{
		RequirementByID:          make(map[string]Requirement, len(g.Requirements)),
		ConflictByID:             make(map[string]Conflict, len(g.Conflicts)),
		AssumptionByID:           make(map[string]Assumption, len(g.Assumptions)),
		StakeholderByID:          make(map[string]Stakeholder, len(g.Stakeholders)),
		RequirementsByAssumption: make(map[string][]string),
		RelationsToRequirement:   make(map[string][]IncomingRelation),
	}
	for _, r := range g.Requirements {
		idx.RequirementByID[r.ID] = r
		for _, aid := range r.Assumptions {
			idx.RequirementsByAssumption[aid] = append(idx.RequirementsByAssumption[aid], r.ID)
		}
		for _, rel := range r.Relations {
			idx.RelationsToRequirement[rel.Target] = append(idx.RelationsToRequirement[rel.Target], IncomingRelation{SourceID: r.ID, Kind: rel.Kind})
		}
	}
	for _, c := range g.Conflicts {
		idx.ConflictByID[c.ID] = c
	}
	for _, a := range g.Assumptions {
		idx.AssumptionByID[a.ID] = a
	}
	for _, s := range g.Stakeholders {
		idx.StakeholderByID[s.ID] = s
	}
	return idx
}

// SortedRequirementIDs returns a sorted copy of ids — a small helper for
// callers that pull an id slice out of one of the reverse-index maps above
// and need deterministic order (map-derived slices already follow
// insertion order in this package, but callers combining several such
// slices, or wanting an explicit sort, should use this instead of trusting
// range order).
func SortedRequirementIDs(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}
