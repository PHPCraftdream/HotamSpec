package ontology

type Stakeholder struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	DeclOrder int    `json:"decl_order"`
}
