package ontology

type Axis struct {
	Slug        string `json:"slug"`
	Description string `json:"description"`
	DeclOrder   int    `json:"decl_order"`
	// History records every UPDATE this Axis's Description has been through
	// (proposal.ProposedAxis.mutate's UPDATE path), mirroring
	// Requirement.History's shape and purpose: a mechanical trail of WHEN a
	// field changed, distinct from the field's current value. Purely
	// additive and optional (omitempty) -- an Axis that has never been
	// UPDATEd (the CREATE-only era before this field existed) has no
	// History and its JSON is unchanged.
	History []HistoryEntry `json:"history,omitempty"`
}
