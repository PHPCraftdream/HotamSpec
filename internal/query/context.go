package query

import (
	"sort"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// NeighborRef is one edge out of (or into) an anchor: the neighbor's id
// and the relation that connects it, e.g. {ID: "R-x", RelKind: "refines"}
// or {ID: "R-y", RelKind: "refines(in)"} for a reverse reference.
type NeighborRef struct {
	ID      string `json:"id"`
	RelKind string `json:"rel_kind"`
}

// ContextCard is a Requirement plus everything one hop away: outgoing and
// incoming Relations, the full text of every Assumption it rests on,
// Conflicts where it is a member, and other Requirements that share at
// least one Assumption with it (the latent-connector view,
// R-latent-connectors-cluster-by-assumption, made explicit per-node).
type ContextCard struct {
	Requirement          RequirementCard  `json:"requirement"`
	Relations            []NeighborRef    `json:"relations"`
	Assumptions          []AssumptionCard `json:"assumptions"`
	Conflicts            []ConflictCard   `json:"conflicts"`
	SharedAssumptionWith []NeighborRef    `json:"shared_assumption_with"`
}

// Context builds the one-hop neighborhood of a Requirement anchor.
func Context(g *ontology.Graph, id string) (ContextCard, error) {
	card, err := ShowRequirement(g, id)
	if err != nil {
		return ContextCard{}, err
	}
	idx := ontology.BuildIndex(g)

	var relations []NeighborRef
	for _, rel := range card.Relations {
		relations = append(relations, NeighborRef{ID: rel.Target, RelKind: rel.Kind})
	}
	for _, in := range idx.RelationsToRequirement[id] {
		if in.SourceID == id {
			continue
		}
		relations = append(relations, NeighborRef{ID: in.SourceID, RelKind: in.Kind + "(in)"})
	}
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].RelKind != relations[j].RelKind {
			return relations[i].RelKind < relations[j].RelKind
		}
		return relations[i].ID < relations[j].ID
	})

	assumptions := make([]AssumptionCard, 0, len(card.Assumptions))
	for _, aid := range card.Assumptions {
		if a, ok := idx.AssumptionByID[aid]; ok {
			assumptions = append(assumptions, assumptionToCard(a))
		}
	}
	sort.Slice(assumptions, func(i, j int) bool { return assumptions[i].ID < assumptions[j].ID })

	var conflicts []ConflictCard
	for _, c := range g.Conflicts {
		for _, m := range c.Members {
			if m == id {
				conflicts = append(conflicts, conflictToCard(c))
				break
			}
		}
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].ID < conflicts[j].ID })

	ownSet := make(map[string]struct{}, len(card.Assumptions))
	for _, aid := range card.Assumptions {
		ownSet[aid] = struct{}{}
	}
	var shared []NeighborRef
	for _, r := range g.Requirements {
		if r.ID == id {
			continue
		}
		var sharedIDs []string
		for _, aid := range r.Assumptions {
			if _, ok := ownSet[aid]; ok {
				sharedIDs = append(sharedIDs, aid)
			}
		}
		sort.Strings(sharedIDs)
		for _, aid := range sharedIDs {
			shared = append(shared, NeighborRef{ID: r.ID, RelKind: "shares_assumption:" + aid})
		}
	}
	sort.Slice(shared, func(i, j int) bool {
		if shared[i].ID != shared[j].ID {
			return shared[i].ID < shared[j].ID
		}
		return shared[i].RelKind < shared[j].RelKind
	})

	// Relations/Conflicts/SharedAssumptionWith are array-typed JSON fields
	// (`hotam req context --json`) built incrementally above by append
	// inside conditional loops that may never fire (a leaf requirement with
	// no relations/conflicts/shared-assumption peers) — normalize any that
	// stayed nil to an empty slice so they marshal to `[]`, not `null`.
	if relations == nil {
		relations = []NeighborRef{}
	}
	if conflicts == nil {
		conflicts = []ConflictCard{}
	}
	if shared == nil {
		shared = []NeighborRef{}
	}

	return ContextCard{
		Requirement:          card,
		Relations:            relations,
		Assumptions:          assumptions,
		Conflicts:            conflicts,
		SharedAssumptionWith: shared,
	}, nil
}

// Related returns just the neighbor id+relation-kind list for an anchor —
// the same edges Context computes (outgoing/incoming Relations, plus
// Conflict membership for Requirements and Assumptions), without the full
// card payload of each neighbor. Works for Requirement, Conflict and
// Assumption anchors alike.
func Related(g *ontology.Graph, id string) ([]NeighborRef, error) {
	kind := ClassifyAnchor(id)
	switch kind {
	case KindRequirement:
		return relatedToRequirement(g, id)
	case KindConflict:
		return relatedToConflict(g, id)
	case KindAssumption:
		return relatedToAssumption(g, id)
	}
	// Unrecognized prefix: try each table in turn.
	if _, err := ShowRequirement(g, id); err == nil {
		return relatedToRequirement(g, id)
	}
	if _, err := ShowConflict(g, id); err == nil {
		return relatedToConflict(g, id)
	}
	if _, err := ShowAssumption(g, id); err == nil {
		return relatedToAssumption(g, id)
	}
	return nil, &ErrNotFound{ID: id}
}

func relatedToRequirement(g *ontology.Graph, id string) ([]NeighborRef, error) {
	r, ok := ontology.RequirementByID(g, id)
	if !ok {
		return nil, &ErrNotFound{ID: id}
	}
	var out []NeighborRef
	for _, rel := range r.Relations {
		out = append(out, NeighborRef{ID: rel.Target, RelKind: rel.Kind})
	}
	for _, aid := range r.Assumptions {
		out = append(out, NeighborRef{ID: aid, RelKind: "assumes"})
	}
	for _, other := range g.Requirements {
		if other.ID == id {
			continue
		}
		for _, rel := range other.Relations {
			if rel.Target == id {
				out = append(out, NeighborRef{ID: other.ID, RelKind: rel.Kind + "(in)"})
			}
		}
	}
	for _, c := range g.Conflicts {
		for _, m := range c.Members {
			if m == id {
				out = append(out, NeighborRef{ID: c.ID, RelKind: "conflict_member"})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RelKind != out[j].RelKind {
			return out[i].RelKind < out[j].RelKind
		}
		return out[i].ID < out[j].ID
	})
	// Array-typed JSON field for `hotam req related --json`: a requirement
	// with no relations/assumptions/incoming-refs/conflict-memberships is a
	// real (not exceptional) case — normalize nil to `[]` so it marshals
	// that way instead of `null`.
	if out == nil {
		out = []NeighborRef{}
	}
	return out, nil
}

func relatedToConflict(g *ontology.Graph, id string) ([]NeighborRef, error) {
	c, err := ShowConflict(g, id)
	if err != nil {
		return nil, err
	}
	var out []NeighborRef
	for _, m := range c.Members {
		out = append(out, NeighborRef{ID: m, RelKind: "member"})
	}
	if c.SharedAssumption != nil && *c.SharedAssumption != "" {
		out = append(out, NeighborRef{ID: *c.SharedAssumption, RelKind: "shared_assumption"})
	}
	for _, d := range c.Derived {
		out = append(out, NeighborRef{ID: d, RelKind: "derived"})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RelKind != out[j].RelKind {
			return out[i].RelKind < out[j].RelKind
		}
		return out[i].ID < out[j].ID
	})
	if out == nil {
		out = []NeighborRef{}
	}
	return out, nil
}

func relatedToAssumption(g *ontology.Graph, id string) ([]NeighborRef, error) {
	if _, ok := ontology.AssumptionByID(g, id); !ok {
		return nil, &ErrNotFound{ID: id}
	}
	var out []NeighborRef
	for _, r := range g.Requirements {
		for _, aid := range r.Assumptions {
			if aid == id {
				out = append(out, NeighborRef{ID: r.ID, RelKind: "assumed_by"})
				break
			}
		}
	}
	for _, c := range g.Conflicts {
		if c.SharedAssumption != nil && *c.SharedAssumption == id {
			out = append(out, NeighborRef{ID: c.ID, RelKind: "shared_assumption_of"})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RelKind != out[j].RelKind {
			return out[i].RelKind < out[j].RelKind
		}
		return out[i].ID < out[j].ID
	})
	if out == nil {
		out = []NeighborRef{}
	}
	return out, nil
}
