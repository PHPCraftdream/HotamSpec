package hotamontology

// Stakeholder mirrors ontology.Stakeholder (internal/ontology/stakeholder.go)
// field-for-field with identical JSON tags (the engine type carries no Why).
// A code-authority domain declares its requirement owners as
// `var Stakeholders = hotamontology.New[hotamontology.Stakeholder]()` in its
// spec/ package; `hotam sync-domain` APPENDS those absent from graph.json
// (never rewrites or deletes an existing graph stakeholder).
//
// Kept in its own file, not requirement.go, so a domain vendored before this
// type existed stays byte-current (check_ontology_vendor_current): its
// absent stakeholder.go is an honest no-op until it re-runs vendor-ontology.
type Stakeholder struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	DeclOrder int    `json:"decl_order"`
}
