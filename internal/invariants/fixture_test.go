package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

var demoAxes = []ontology.Axis{
	{Slug: "cost-vs-flexibility", Description: "cost vs flexibility"},
}

var (
	sOut = ontology.Stakeholder{ID: "outsider", Name: "Outsider", Domain: "x"}
	sA   = ontology.Stakeholder{ID: "sa", Name: "A", Domain: "x"}
	sB   = ontology.Stakeholder{ID: "sb", Name: "B", Domain: "x"}
)

func req(rid, owner string) ontology.Requirement {
	return ontology.Requirement{
		ID:             rid,
		Claim:          "claim " + rid,
		Owner:          owner,
		Status:         ontology.StatusSETTLED,
		Enforcement:    ontology.EnforcementPROSE,
		Enforceability: ontology.EnforceabilityENFORCEABLE,
	}
}

func reqStatus(rid, owner, status string) ontology.Requirement {
	return ontology.Requirement{
		ID:             rid,
		Claim:          "claim " + rid,
		Owner:          owner,
		Status:         status,
		Enforcement:    ontology.EnforcementPROSE,
		Enforceability: ontology.EnforceabilityENFORCEABLE,
	}
}

func variant(vid, behavior string) ontology.Variant {
	return ontology.Variant{ID: vid, Behavior: behavior}
}

func decidedConflict() ontology.Conflict {
	c := baseConflict()
	c.Lifecycle = "DECIDED(resolver chose option A)"
	c.DecidedBy = "outsider"
	return c
}

func heldConflict() ontology.Conflict {
	c := baseConflict()
	c.Lifecycle = "HELD(awaiting more data)"
	c.DecidedBy = "outsider"
	c.Variants = []ontology.Variant{
		variant("V-fast", "ship now"),
		variant("V-safe", "add tests first"),
	}
	return c
}

func baseConflict() ontology.Conflict {
	axis := "cost-vs-flexibility"
	context := "some shared scenario"
	return ontology.Conflict{
		ID:        ontology.ConflictIdentity(axis, context),
		Axis:      axis,
		Context:   context,
		Members:   []string{"R-1", "R-2"},
		Resolver:  "outsider",
		Lifecycle: "ACKNOWLEDGED",
	}
}

func graphWithConflict(c ontology.Conflict, reqs []ontology.Requirement, assumptions ...ontology.Assumption) *ontology.Graph {
	if reqs == nil {
		reqs = []ontology.Requirement{req("R-1", "sa"), req("R-2", "sb")}
	}
	return &ontology.Graph{
		Axes:         demoAxes,
		Stakeholders: []ontology.Stakeholder{sOut, sA, sB},
		Assumptions:  assumptions,
		Requirements: reqs,
		Conflicts:    []ontology.Conflict{c},
	}
}

func runCheck(t *testing.T, name string, g *ontology.Graph) []Violation {
	t.Helper()
	inv, ok := All.Get(name)
	if !ok {
		t.Fatalf("invariant %q not registered", name)
	}
	return inv.Check(g)
}

func hasViolationFor(vs []Violation, id string) bool {
	for _, v := range vs {
		if v.ID == id {
			return true
		}
	}
	return false
}

func reqEnforced(rid, owner string, enforcedBy ...string) ontology.Requirement {
	r := req(rid, owner)
	r.Enforcement = ontology.EnforcementENFORCED
	r.EnforcedBy = enforcedBy
	return r
}

func op(id, stakeholder, lifecycle string) ontology.Operator {
	return ontology.Operator{ID: id, Stakeholder: stakeholder, Lifecycle: lifecycle}
}

func goal(id, owner, lifecycle string) ontology.Goal {
	return ontology.Goal{ID: id, Owner: owner, Lifecycle: lifecycle}
}

func process(id string) ontology.Process {
	return ontology.Process{ID: id, Lifecycle: ontology.ProcessLifecycle}
}

func step(name, role, invokes string) ontology.Step {
	return ontology.Step{Name: name, RequiresRole: role, Invokes: invokes}
}

func simpleLifecycle(slug string) ontology.Lifecycle {
	return ontology.Lifecycle{
		Slug: slug,
		States: []ontology.State{
			{Name: "INIT", Kind: ontology.StateKindInitial},
			{Name: "ACTIVE", Kind: ontology.StateKindNormal},
			{Name: "DONE", Kind: ontology.StateKindQuiescent},
		},
		Transitions: []ontology.Transition{
			{Src: "INIT", Dst: "ACTIVE", Event: "activate"},
			{Src: "ACTIVE", Dst: "DONE", Event: "finish"},
		},
	}
}

func entityType(slug string) ontology.EntityType {
	return ontology.EntityType{Slug: slug, Lifecycle: simpleLifecycle("lc-" + slug)}
}

func entityInstance(id, et, state string) ontology.EntityInstance {
	return ontology.EntityInstance{ID: id, EntityType: et, State: state}
}
