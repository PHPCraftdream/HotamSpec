package ontology

type Graph struct {
	Axes         []Axis           `json:"axes"`
	Stakeholders []Stakeholder    `json:"stakeholders"`
	Assumptions  []Assumption     `json:"assumptions"`
	Requirements []Requirement    `json:"requirements"`
	Conflicts    []Conflict       `json:"conflicts"`
	Operators    []Operator       `json:"operators"`
	Processes    []Process        `json:"processes"`
	Goals        []Goal           `json:"goals"`
	EntityTypes  []EntityType     `json:"entity_types"`
	Entities     []EntityInstance `json:"entities"`
	SelfHosting  bool             `json:"self_hosting"`
}

func (g *Graph) IsEmpty() bool {
	return len(g.Axes) == 0 &&
		len(g.Stakeholders) == 0 &&
		len(g.Assumptions) == 0 &&
		len(g.Requirements) == 0 &&
		len(g.Conflicts) == 0 &&
		len(g.Operators) == 0 &&
		len(g.Processes) == 0 &&
		len(g.Goals) == 0 &&
		len(g.EntityTypes) == 0 &&
		len(g.Entities) == 0
}

func AxisSlugs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Axes))
	for _, a := range g.Axes {
		out[a.Slug] = struct{}{}
	}
	return out
}

func StakeholderIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Stakeholders))
	for _, s := range g.Stakeholders {
		out[s.ID] = struct{}{}
	}
	return out
}

func AssumptionIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Assumptions))
	for _, a := range g.Assumptions {
		out[a.ID] = struct{}{}
	}
	return out
}

func RequirementIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Requirements))
	for _, r := range g.Requirements {
		out[r.ID] = struct{}{}
	}
	return out
}

func OperatorIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Operators))
	for _, op := range g.Operators {
		out[op.ID] = struct{}{}
	}
	return out
}

func ProcessIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Processes))
	for _, p := range g.Processes {
		out[p.ID] = struct{}{}
	}
	return out
}

func GoalIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Goals))
	for _, go_ := range g.Goals {
		out[go_.ID] = struct{}{}
	}
	return out
}

func EntityTypeSlugs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.EntityTypes))
	for _, et := range g.EntityTypes {
		out[et.Slug] = struct{}{}
	}
	return out
}

func EntityIDs(g *Graph) map[string]struct{} {
	out := make(map[string]struct{}, len(g.Entities))
	for _, e := range g.Entities {
		out[e.ID] = struct{}{}
	}
	return out
}

func RequirementByID(g *Graph, rid string) (Requirement, bool) {
	for _, r := range g.Requirements {
		if r.ID == rid {
			return r, true
		}
	}
	return Requirement{}, false
}

func AssumptionByID(g *Graph, aid string) (Assumption, bool) {
	for _, a := range g.Assumptions {
		if a.ID == aid {
			return a, true
		}
	}
	return Assumption{}, false
}
