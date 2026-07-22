package ontology

const (
	AssumptionHOLDS      = "HOLDS"
	AssumptionUNCERTAIN  = "UNCERTAIN"
	AssumptionDEAD       = "DEAD"
	AssumptionIMPLEMENTS = "IMPLEMENTS"
)

var AssumptionStates = map[string]struct{}{
	AssumptionHOLDS:      {},
	AssumptionUNCERTAIN:  {},
	AssumptionDEAD:       {},
	AssumptionIMPLEMENTS: {},
}

type Assumption struct {
	ID           string   `json:"id"`
	Statement    string   `json:"statement"`
	Status       string   `json:"status"`
	Owner        string   `json:"owner"`
	MachineCheck *string  `json:"machine_check"`
	Signoff      *Signoff `json:"signoff"`
	CreatedAt    string   `json:"created_at"`
	DecidedAt    string   `json:"decided_at"`
	DeclOrder    int      `json:"decl_order"`
	// SourceRefs lists supporting provenance references for this assumption
	// (a doc path, a ticket id, a decision record, ...) -- the same
	// free-form, unresolved-by-invariant shape Requirement.SourceRefs
	// already establishes (internal/ontology/requirement.go): no check_*
	// validates these entries resolve to anything real, mirroring
	// Requirement's own precedent rather than inventing a stricter rule for
	// this node type alone. Purely additive and optional (omitempty) -- an
	// Assumption that predates this field has no source_refs and its JSON
	// is unchanged.
	SourceRefs []string `json:"source_refs,omitempty"`
	// History records every REWRITE this Assumption's Statement has been
	// through via proposal.ProposedAssumptionRewrite -- distinct from the
	// existing ProposedAssumptionTransition mechanism, which changes Status
	// and APPENDS a "[STATUS] reason" suffix onto Statement as a side
	// effect of a status decision. A rewrite changes the statement's actual
	// meaning with no status change involved, so without its own History
	// trail it would be silent, unaudited drift of what the assumption
	// even claims -- see ProposedAssumptionRewrite's doc comment in
	// types.go. Purely additive and optional (omitempty) -- an Assumption
	// that has never been rewritten has no History and its JSON is
	// unchanged.
	History []HistoryEntry `json:"history,omitempty"`
}
