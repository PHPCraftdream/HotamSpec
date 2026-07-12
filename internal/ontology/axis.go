package ontology

type Axis struct {
	Slug        string `json:"slug"`
	Description string `json:"description"`
	DeclOrder   int    `json:"decl_order"`
}
