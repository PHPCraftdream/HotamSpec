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
	// ModelSymbol is an OPTIONAL "file:Symbol"-shaped reference (the same
	// shape/parser as Requirement.ImplementedBy, gate.ParseFileColonSymbol)
	// naming the Go type in the domain's authored spec/model/ tree that this
	// EntityType corresponds to. Empty means "no Go type yet, or this
	// EntityType has no 1:1 Go counterpart" -- a calm, expected, honest-no-op
	// default, mirroring every other optional link field in this codebase
	// (BlockedOn/ImplementedBy/VerifiedBy on Requirement).
	//
	// ONE-DIRECTIONAL by design: EntityType names its own Go symbol, but the
	// Go side carries no back-reference to the EntityType (no MODELS.md
	// annotation pointing here). This field is a graph-level reference to
	// code that is already authored elsewhere; it is NEVER a generation
	// target and NEVER generates the named Go symbol -- the methodology's
	// authority for Go code stays exactly where R-authored-spec-projections-
	// are-derived and docs/AUTHORED-SPEC-CONTRACT.md §9 already put it: Go
	// code is authored by hand, graph nodes are projections of it, never the
	// other way around. See R-entity-type-realized-by-go-symbol-never-generated.
	ModelSymbol string `json:"model_symbol,omitempty"`
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
