package query

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// RequirementCard is the full agent-facing card for one Requirement: every
// field an agent would otherwise have had to dig out of graph.json or
// docs/gen/REQUIREMENTS.md by hand, including the freshness fields
// (R-requirement-freshness-fields).
type RequirementCard struct {
	ID             string                  `json:"id"`
	Kind           AnchorKind              `json:"kind"`
	Claim          string                  `json:"claim"`
	Why            string                  `json:"why"`
	Owner          string                  `json:"owner"`
	Status         string                  `json:"status"`
	Enforcement    string                  `json:"enforcement"`
	Enforceability string                  `json:"enforceability"`
	EnforcedBy     []string                `json:"enforced_by"`
	ImplementedBy  []string                `json:"implemented_by"`
	VerifiedBy     []string                `json:"verified_by"`
	Assumptions    []string                `json:"assumptions"`
	Relations      []ontology.Relation     `json:"relations"`
	Summary        string                  `json:"summary"`
	MTag           string                  `json:"m_tag"`
	CreatedAt      string                  `json:"created_at"`
	SettledAt      string                  `json:"settled_at"`
	LastReviewedAt string                  `json:"last_reviewed_at"`
	ReviewAfter    string                  `json:"review_after"`
	Evidence       []string                `json:"evidence"`
	SourceRefs     []string                `json:"source_refs"`
	History        []ontology.HistoryEntry `json:"history"`
}

// ConflictCard is the full agent-facing card for one Conflict node.
type ConflictCard struct {
	ID               string             `json:"id"`
	Kind             AnchorKind         `json:"kind"`
	Axis             string             `json:"axis"`
	Context          string             `json:"context"`
	Members          []string           `json:"members"`
	Steward          string             `json:"steward"`
	Lifecycle        string             `json:"lifecycle"`
	SharedAssumption *string            `json:"shared_assumption"`
	Derived          []string           `json:"derived"`
	RevisitMarker    string             `json:"revisit_marker"`
	DecidedBy        string             `json:"decided_by"`
	Variants         []ontology.Variant `json:"variants"`
	Signoff          *ontology.Signoff  `json:"signoff"`
	CreatedAt        string             `json:"created_at"`
	DecidedAt        string             `json:"decided_at"`
}

// AssumptionCard is the full agent-facing card for one Assumption node.
type AssumptionCard struct {
	ID           string            `json:"id"`
	Kind         AnchorKind        `json:"kind"`
	Statement    string            `json:"statement"`
	Status       string            `json:"status"`
	Owner        string            `json:"owner"`
	MachineCheck *string           `json:"machine_check"`
	Signoff      *ontology.Signoff `json:"signoff"`
	CreatedAt    string            `json:"created_at"`
	DecidedAt    string            `json:"decided_at"`
}

// ErrNotFound is returned by Show* lookups when the anchor id does not
// exist in the graph under the requested (or any recognized) kind.
type ErrNotFound struct {
	ID string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("anchor %q not found in graph", e.ID)
}

func ShowRequirement(g *ontology.Graph, id string) (RequirementCard, error) {
	r, ok := ontology.RequirementByID(g, id)
	if !ok {
		return RequirementCard{}, &ErrNotFound{ID: id}
	}
	return requirementToCard(r), nil
}

func requirementToCard(r ontology.Requirement) RequirementCard {
	return RequirementCard{
		ID:             r.ID,
		Kind:           KindRequirement,
		Claim:          r.Claim,
		Why:            r.Why,
		Owner:          r.Owner,
		Status:         r.Status,
		Enforcement:    r.Enforcement,
		Enforceability: r.Enforceability,
		EnforcedBy:     nonNilStrings(r.EnforcedBy),
		ImplementedBy:  nonNilStrings(r.ImplementedBy),
		VerifiedBy:     nonNilStrings(r.VerifiedBy),
		Assumptions:    nonNilStrings(r.Assumptions),
		Relations:      nonNilRelations(r.Relations),
		Summary:        r.Summary,
		MTag:           r.MTag,
		CreatedAt:      r.CreatedAt,
		SettledAt:      r.SettledAt,
		LastReviewedAt: r.LastReviewedAt,
		ReviewAfter:    r.ReviewAfter,
		Evidence:       nonNilStrings(r.Evidence),
		SourceRefs:     nonNilStrings(r.SourceRefs),
		History:        nonNilHistory(r.History),
	}
}

// nonNilStrings/nonNilRelations/nonNilHistory/nonNilVariants normalize a nil
// slice to a non-nil empty one. Requirement/Conflict array fields are
// deserialized straight from a domain's graph.json (internal/loader), which
// may legitimately contain `null` or an omitted key for an array field (that
// persisted-state format is not this normalization's concern). But the SAME
// Go nil then flows unchanged into RequirementCard/ConflictCard, the shape
// `hotam req show`/`hotam req context --json` marshal to the CLI's
// machine-readable output — where a `null` array is a footgun for an agent
// consumer (e.g. `for x of null` throws in JS) that a `[]` is not. This is
// the one place that boundary is crossed, so it is the right place to close
// the gap rather than touching the loader or the persisted format.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilRelations(r []ontology.Relation) []ontology.Relation {
	if r == nil {
		return []ontology.Relation{}
	}
	return r
}

func nonNilHistory(h []ontology.HistoryEntry) []ontology.HistoryEntry {
	if h == nil {
		return []ontology.HistoryEntry{}
	}
	return h
}

func nonNilVariants(v []ontology.Variant) []ontology.Variant {
	if v == nil {
		return []ontology.Variant{}
	}
	return v
}

func ShowConflict(g *ontology.Graph, id string) (ConflictCard, error) {
	for _, c := range g.Conflicts {
		if c.ID == id {
			return conflictToCard(c), nil
		}
	}
	return ConflictCard{}, &ErrNotFound{ID: id}
}

func conflictToCard(c ontology.Conflict) ConflictCard {
	return ConflictCard{
		ID:               c.ID,
		Kind:             KindConflict,
		Axis:             c.Axis,
		Context:          c.Context,
		Members:          nonNilStrings(c.Members),
		Steward:          c.Steward,
		Lifecycle:        c.Lifecycle,
		SharedAssumption: c.SharedAssumption,
		Derived:          nonNilStrings(c.Derived),
		RevisitMarker:    c.RevisitMarker,
		DecidedBy:        c.DecidedBy,
		Variants:         nonNilVariants(c.Variants),
		Signoff:          c.Signoff,
		CreatedAt:        c.CreatedAt,
		DecidedAt:        c.DecidedAt,
	}
}

func ShowAssumption(g *ontology.Graph, id string) (AssumptionCard, error) {
	a, ok := ontology.AssumptionByID(g, id)
	if !ok {
		return AssumptionCard{}, &ErrNotFound{ID: id}
	}
	return assumptionToCard(a), nil
}

func assumptionToCard(a ontology.Assumption) AssumptionCard {
	return AssumptionCard{
		ID:           a.ID,
		Kind:         KindAssumption,
		Statement:    a.Statement,
		Status:       a.Status,
		Owner:        a.Owner,
		MachineCheck: a.MachineCheck,
		Signoff:      a.Signoff,
		CreatedAt:    a.CreatedAt,
		DecidedAt:    a.DecidedAt,
	}
}

// Show resolves any anchor id to its card, trying the kind implied by its
// prefix first and falling back to the other two kinds so a caller never
// has to know the type up front — `hotam req show` accepts any anchor.
// The returned value is one of RequirementCard, ConflictCard or
// AssumptionCard.
func Show(g *ontology.Graph, id string) (any, error) {
	kind := ClassifyAnchor(id)
	switch kind {
	case KindRequirement:
		if c, err := ShowRequirement(g, id); err == nil {
			return c, nil
		}
	case KindConflict:
		if c, err := ShowConflict(g, id); err == nil {
			return c, nil
		}
	case KindAssumption:
		if c, err := ShowAssumption(g, id); err == nil {
			return c, nil
		}
	}
	// Fallback: prefix didn't resolve (or was unrecognized) — try every
	// kind before giving up, so a mistyped or unconventional id still
	// resolves if it exists under a different table.
	if c, err := ShowRequirement(g, id); err == nil {
		return c, nil
	}
	if c, err := ShowConflict(g, id); err == nil {
		return c, nil
	}
	if c, err := ShowAssumption(g, id); err == nil {
		return c, nil
	}
	return nil, &ErrNotFound{ID: id}
}
