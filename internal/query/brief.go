package query

import (
	"github.com/PHPCraftdream/HotamSpec/internal/freshness"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// FreshnessInfo is the freshness classification of a Requirement anchor,
// present on a BriefCard only when Kind == KindRequirement. It carries the
// OVERDUE/DUE-SOON/NEVER-REVIEWED/FRESH status (internal/freshness.Classify)
// and the overdue day count (internal/freshness.OverdueDays), which is 0 for
// non-OVERDUE statuses.
type FreshnessInfo struct {
	Status      string `json:"status"`
	OverdueDays int    `json:"overdue_days"`
}

// BriefCard is the single-call aggregation of everything an agent needs to
// fully orient on one anchor: the full card (claim/why/status/enforcement or
// the Conflict/Assumption equivalent), its one-hop graph neighborhood
// (relations, assumptions with their own liveness, conflicts with axes,
// shared-assumption peers), and — for a Requirement anchor only, where
// freshness is a meaningful concept — its OVERDUE/DUE-SOON/NEVER-REVIEWED/
// FRESH classification. Replaces the 3-4 separate round-trips `hotam req
// show` + `hotam req context` + `hotam req related` + `hotam due` previously
// required to answer "what do I need to know about this anchor right now".
//
// JSON shape is self-describing: the `kind` field discriminates which sub-
// fields are populated. For a Requirement anchor: `requirement`, `neighbors`,
// `assumptions`, `conflicts`, `shared_assumption_with`, and `freshness` are
// set. For a Conflict anchor: `conflict` and `neighbors`. For an Assumption
// anchor: `assumption` and `neighbors`.
type BriefCard struct {
	Kind AnchorKind `json:"kind"`
	ID   string     `json:"id"`

	// The full anchor card — exactly one is non-nil depending on Kind.
	Requirement *RequirementCard `json:"requirement,omitempty"`
	Conflict    *ConflictCard    `json:"conflict,omitempty"`
	Assumption  *AssumptionCard  `json:"assumption,omitempty"`

	// Neighbors is the one-hop graph neighborhood. For a Requirement this
	// is Context's Relations (outgoing + incoming relation edges). For a
	// Conflict this is Related's output (members, shared_assumption,
	// derived). For an Assumption this is Related's output (assumed_by,
	// shared_assumption_of).
	Neighbors []NeighborRef `json:"neighbors"`

	// Requirement-only enrichment from Context: the full text of every
	// assumption the requirement rests on (with each assumption's own
	// liveness Status), the full card of every conflict it is a member of
	// (carrying Axis), and peers that share at least one assumption. Absent
	// (omitted in JSON) for Conflict/Assumption anchors.
	Assumptions          []AssumptionCard `json:"assumptions,omitempty"`
	Conflicts            []ConflictCard   `json:"conflicts,omitempty"`
	SharedAssumptionWith []NeighborRef    `json:"shared_assumption_with,omitempty"`

	// Freshness is the Requirement anchor's freshness classification. It is
	// nil (omitted in JSON) for Conflict/Assumption anchors — freshness is
	// not a Conflict/Assumption concept in this codebase.
	Freshness *FreshnessInfo `json:"freshness,omitempty"`
}

// Brief builds the full single-call orientation card for any anchor
// (Requirement, Conflict, or Assumption). today (YYYY-MM-DD) drives the
// freshness classification for a Requirement anchor; it is ignored for
// Conflict/Assumption anchors.
func Brief(g *ontology.Graph, id, today string) (BriefCard, error) {
	kind := ClassifyAnchor(id)
	switch kind {
	case KindRequirement:
		return briefRequirement(g, id, today)
	case KindConflict:
		return briefConflict(g, id)
	case KindAssumption:
		return briefAssumption(g, id)
	}
	// Unrecognized prefix: try each table in turn, mirroring Show/Related.
	if _, err := ShowRequirement(g, id); err == nil {
		return briefRequirement(g, id, today)
	}
	if _, err := ShowConflict(g, id); err == nil {
		return briefConflict(g, id)
	}
	if _, err := ShowAssumption(g, id); err == nil {
		return briefAssumption(g, id)
	}
	return BriefCard{}, &ErrNotFound{ID: id}
}

// briefRequirement is a thin wrapper over the existing Context function:
// it calls Context unchanged and layers the freshness classification on top.
func briefRequirement(g *ontology.Graph, id, today string) (BriefCard, error) {
	cc, err := Context(g, id)
	if err != nil {
		return BriefCard{}, err
	}
	r, ok := ontology.RequirementByID(g, id)
	if !ok {
		// Context succeeded so the requirement exists; this is unreachable.
		return BriefCard{}, &ErrNotFound{ID: id}
	}
	return BriefCard{
		Kind:                 KindRequirement,
		ID:                   id,
		Requirement:          &cc.Requirement,
		Neighbors:            cc.Relations,
		Assumptions:          cc.Assumptions,
		Conflicts:            cc.Conflicts,
		SharedAssumptionWith: cc.SharedAssumptionWith,
		Freshness: &FreshnessInfo{
			Status:      string(freshness.Classify(r, today)),
			OverdueDays: freshness.OverdueDays(r, today),
		},
	}, nil
}

// briefConflict composes the brief from ShowConflict (the full ConflictCard,
// which carries Axis) + Related (the neighbor list: members, shared_assumption,
// derived).
func briefConflict(g *ontology.Graph, id string) (BriefCard, error) {
	card, err := ShowConflict(g, id)
	if err != nil {
		return BriefCard{}, err
	}
	neighbors, err := Related(g, id)
	if err != nil {
		return BriefCard{}, err
	}
	return BriefCard{
		Kind:      KindConflict,
		ID:        id,
		Conflict:  &card,
		Neighbors: neighbors,
	}, nil
}

// briefAssumption composes the brief from ShowAssumption (the full
// AssumptionCard) + Related (the neighbor list: assumed_by,
// shared_assumption_of).
func briefAssumption(g *ontology.Graph, id string) (BriefCard, error) {
	card, err := ShowAssumption(g, id)
	if err != nil {
		return BriefCard{}, err
	}
	neighbors, err := Related(g, id)
	if err != nil {
		return BriefCard{}, err
	}
	return BriefCard{
		Kind:       KindAssumption,
		ID:         id,
		Assumption: &card,
		Neighbors:  neighbors,
	}, nil
}
