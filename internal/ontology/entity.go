package ontology

var EntityFieldKinds = map[string]struct{}{
	"string":    {},
	"number":    {},
	"enum":      {},
	"reference": {},
	"state":     {},
}

type EntityField struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Required  bool   `json:"required"`
	RefTarget string `json:"ref_target"`
}

type EntityType struct {
	Slug        string         `json:"slug"`
	Description string         `json:"description"`
	Lifecycle   Lifecycle      `json:"lifecycle"`
	Fields      []EntityField  `json:"fields"`
	Why         string         `json:"why"`
	DeclOrder   int            `json:"decl_order"`
	History     []HistoryEntry `json:"history"`
}

type EntityInstance struct {
	ID          string      `json:"id"`
	EntityType  string      `json:"entity_type"`
	State       string      `json:"state"`
	FieldValues [][2]string `json:"field_values"`
	DeclOrder   int         `json:"decl_order"`
}

func (e EntityInstance) FieldValue(name string) (string, bool) {
	for _, fv := range e.FieldValues {
		if fv[0] == name {
			return fv[1], true
		}
	}
	return "", false
}
