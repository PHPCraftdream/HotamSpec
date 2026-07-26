// Package hotamontology is the CANONICAL source of the minimal vendored
// Requirement mirror + generic Registry a consumer domain's own spec/ Go
// module imports directly by file (task #365, RAC2 Phase A: "Go-code-only
// authority" for consumer-domain requirements, mirroring hotam-spec-self's
// own self-hosting shape -- internal/selfspec/requirements_*.go +
// `hotam sync-self`).
//
// THIS FILE IS THE CANON. It is never imported directly by a consumer
// domain's spec/ module (that would require a cross-module `replace`,
// forbidden per NEW-2-bis -- see internal/gate/test_exec.go's
// hashPackageInputs NEW-2 doc comment, and internal/recorder/canon/
// hotamspec.go's identical doc comment for the sibling precedent this
// package copies 1-in-1). Instead this file is VENDORED -- copied
// byte-for-byte, banner-stamped "do not edit", into each consumer spec/
// module as its own single-file `hotamontology` package
// (internal/ontology/vendor's Source does the banner-stamping; cmd/hotam's
// `vendor-ontology` command writes the vendored copy to
// <domainDir>/spec/hotamontology/requirement.go).
// internal/invariants/ontology_vendor_check.go's check_ontology_vendor_current
// invariant sha256-compares the vendored copy against this canonical file
// (post banner-strip) and fires a violation on drift.
//
// SCOPE: Requirement here is a MINIMAL, JSON-tag-identical mirror of ONLY the
// STRUCTURAL fields of internal/ontology.Requirement (internal/ontology/
// requirement.go) -- the fields internal/selfspec/merge.go's own doc comment
// already names "structural" (replaced wholesale from a registry entry) in
// contrast to the "event" fields it passes through untouched from the
// existing graph node (History, GateSignoffs, LastReviewedAt, ReviewAfter,
// Evidence -- deliberately NOT mirrored here; a consumer domain authors
// identity/claim/owner/status/links in Go code, it never authors its own
// review timestamps or history log entries in code). Why/Enforceability's
// sibling event-adjacent fields (LastReviewedAt, ReviewAfter, Evidence,
// History, GateSignoffs) stay engine-side, populated only by the graph
// itself over time -- exactly the same split MergeIntoGraph already
// mechanizes for hotam-spec-self's own self-hosting path.
//
// No methods, no behavior -- pure form, so a consumer domain's separate Go
// module can import this file directly (copied in, not a go.mod dependency
// on the engine) without pulling in anything internal/ontology itself does
// (Graph traversal, Lifecycle, validation, ...).
package hotamontology

// Relation mirrors ontology.Relation (internal/ontology/requirement.go) --
// same fields, same JSON tags, byte-identical wire shape.
type Relation struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
}

// Requirement mirrors ONLY the structural fields of ontology.Requirement
// (internal/ontology/requirement.go) -- see this file's package doc comment
// for exactly which fields those are and why the event fields (History,
// GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are excluded. Field
// order and JSON tags match the source exactly, field-for-field, so a value
// of this type round-trips through JSON identically to the corresponding
// subset of an ontology.Requirement.
type Requirement struct {
	ID             string     `json:"id"`
	Claim          string     `json:"claim"`
	Owner          string     `json:"owner"`
	Status         string     `json:"status"`
	Relations      []Relation `json:"relations"`
	Assumptions    []string   `json:"assumptions"`
	Enforcement    string     `json:"enforcement"`
	EnforcedBy     []string   `json:"enforced_by"`
	Enforceability string     `json:"enforceability"`
	MTag           string     `json:"m_tag"`
	Summary        string     `json:"summary"`
	CreatedAt      string     `json:"created_at"`
	SettledAt      string     `json:"settled_at"`
	BlockedOn      string     `json:"blocked_on,omitempty"`
	ImplementedBy  []string   `json:"implemented_by,omitempty"`
	VerifiedBy     []string   `json:"verified_by,omitempty"`
	SourceRefs     []string   `json:"source_refs"`
	DeclOrder      int        `json:"decl_order"`
}
