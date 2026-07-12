package query

import "strings"

// AnchorKind classifies a node id by its typed-anchor prefix
// (R-anchor-everything / R-anchor-taxonomy: R-/C-/A- are the anchors this
// package resolves).
type AnchorKind string

const (
	KindRequirement AnchorKind = "Requirement"
	KindConflict    AnchorKind = "Conflict"
	KindAssumption  AnchorKind = "Assumption"
	KindUnknown     AnchorKind = "Unknown"
)

// ClassifyAnchor guesses a node's kind from its id prefix. It is a hint
// only — Resolve below always confirms against the graph before trusting it.
func ClassifyAnchor(id string) AnchorKind {
	switch {
	case strings.HasPrefix(id, "R-"):
		return KindRequirement
	case strings.HasPrefix(id, "C-"):
		return KindConflict
	case strings.HasPrefix(id, "A-"):
		return KindAssumption
	default:
		return KindUnknown
	}
}
