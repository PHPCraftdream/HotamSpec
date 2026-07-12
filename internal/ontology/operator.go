package ontology

const (
	BudgetMeasureNODE_COUNT     = "NODE_COUNT"
	BudgetMeasureTOKEN_ESTIMATE = "TOKEN_ESTIMATE"
	BudgetMeasureCOMPLEXITY     = "COMPLEXITY"
	BudgetMeasureCRYSTAL_CHARS  = "CRYSTAL_CHARS"
)

var BudgetMeasures = map[string]struct{}{
	BudgetMeasureNODE_COUNT:     {},
	BudgetMeasureTOKEN_ESTIMATE: {},
	BudgetMeasureCOMPLEXITY:     {},
	BudgetMeasureCRYSTAL_CHARS:  {},
}

type ContextBudget struct {
	Limit   int    `json:"limit"`
	Measure string `json:"measure"`
}

var OperatorLifecycle = Lifecycle{
	Slug: "operator-lifecycle",
	States: []State{
		{Name: "ACTIVE", Kind: StateKindInitial},
		{Name: "SATURATED", Kind: StateKindNormal},
		{Name: "DELEGATED", Kind: StateKindNormal},
		{Name: "RETIRED", Kind: StateKindQuiescent},
	},
	Transitions: []Transition{
		{Src: "ACTIVE", Dst: "SATURATED", Event: "approach-limit"},
		{Src: "SATURATED", Dst: "ACTIVE", Event: "crystallize"},
		{Src: "SATURATED", Dst: "DELEGATED", Event: "spawn-sub-operator"},
		{Src: "ACTIVE", Dst: "RETIRED", Event: "retire"},
		{Src: "DELEGATED", Dst: "ACTIVE", Event: "merge-back"},
	},
	Cyclic: true,
}

type Operator struct {
	ID            string        `json:"id"`
	Stakeholder   string        `json:"stakeholder"`
	Lifecycle     string        `json:"lifecycle"`
	ContextBudget ContextBudget `json:"context_budget"`
	Parent        *string       `json:"parent"`
	Scope         []string      `json:"scope"`
	Why           string        `json:"why"`
	DeclOrder     int           `json:"decl_order"`
}
