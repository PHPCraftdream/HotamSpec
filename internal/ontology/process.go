package ontology

const (
	TargetKindGraphProperty = "GRAPH_PROPERTY"
	TargetKindBusinessState = "BUSINESS_STATE"
	TargetKindEntityState   = "ENTITY_STATE"
)

var TargetKinds = map[string]struct{}{
	TargetKindGraphProperty: {},
	TargetKindBusinessState: {},
	TargetKindEntityState:   {},
}

type Step struct {
	Name         string `json:"name"`
	RequiresRole string `json:"requires_role"`
	Invokes      string `json:"invokes"`
	Why          string `json:"why"`
}

type Process struct {
	ID             string         `json:"id"`
	Lifecycle      Lifecycle      `json:"lifecycle"`
	Steps          []Step         `json:"steps"`
	RolesRequired  []string       `json:"roles_required"`
	DrivesEntities []string       `json:"drives_entities"`
	Why            string         `json:"why"`
	DeclOrder      int            `json:"decl_order"`
	History        []HistoryEntry `json:"history"`
}

type TargetState struct {
	Kind      string `json:"kind"`
	Predicate string `json:"predicate"`
	Target    string `json:"target"`
}

type Goal struct {
	ID          string      `json:"id"`
	Owner       string      `json:"owner"`
	TargetState TargetState `json:"target_state"`
	Lifecycle   string      `json:"lifecycle"`
	Why         string      `json:"why"`
	DeclOrder   int         `json:"decl_order"`
}

var ProcessLifecycle = Lifecycle{
	Slug: "process-lifecycle",
	States: []State{
		{Name: "READY", Kind: StateKindInitial},
		{Name: "RUNNING", Kind: StateKindNormal},
		{Name: "BLOCKED", Kind: StateKindNormal},
		{Name: "DONE", Kind: StateKindQuiescent},
		{Name: "ABANDONED", Kind: StateKindQuiescent},
	},
	Transitions: []Transition{
		{Src: "READY", Dst: "RUNNING", Event: "start"},
		{Src: "RUNNING", Dst: "BLOCKED", Event: "block"},
		{Src: "RUNNING", Dst: "DONE", Event: "complete"},
		{Src: "RUNNING", Dst: "ABANDONED", Event: "abandon"},
		{Src: "BLOCKED", Dst: "RUNNING", Event: "unblock"},
		{Src: "BLOCKED", Dst: "ABANDONED", Event: "abandon"},
	},
	Cyclic: false,
}

var GoalLifecycle = Lifecycle{
	Slug: "goal-lifecycle",
	States: []State{
		{Name: "ACTIVE", Kind: StateKindInitial},
		{Name: "MET", Kind: StateKindQuiescent},
		{Name: "ABANDONED", Kind: StateKindQuiescent},
	},
	Transitions: []Transition{
		{Src: "ACTIVE", Dst: "MET", Event: "target-reached"},
		{Src: "ACTIVE", Dst: "ABANDONED", Event: "abandon"},
	},
	Cyclic: false,
}
