package proposal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const today = "2026-07-12"

func baseGraph() *ontology.Graph {
	axis := "cost-vs-flexibility"
	context := "shared scenario"
	return &ontology.Graph{
		Axes: []ontology.Axis{
			{Slug: axis, Description: "cost vs flexibility"},
		},
		Stakeholders: []ontology.Stakeholder{
			{ID: "outsider", Name: "Outsider", Domain: "x"},
			{ID: "sa", Name: "A", Domain: "x"},
			{ID: "sb", Name: "B", Domain: "x"},
		},
		Assumptions: []ontology.Assumption{
			{ID: "A-base", Statement: "the substrate is stable", Status: ontology.AssumptionHOLDS, Owner: "sa"},
		},
		Requirements: []ontology.Requirement{
			reqFull("R-1", "sa"),
			reqFull("R-2", "sb"),
			reqFull("R-3", "sa"),
		},
		Conflicts: []ontology.Conflict{
			{
				ID:        ontology.ConflictIdentity(axis, context),
				Axis:      axis,
				Context:   context,
				Members:   []string{"R-1", "R-2"},
				Steward:   "outsider",
				Lifecycle: "ACKNOWLEDGED",
			},
		},
	}
}

func reqFull(rid, owner string) ontology.Requirement {
	return ontology.Requirement{
		ID:             rid,
		Claim:          "claim " + rid,
		Owner:          owner,
		Status:         ontology.StatusSETTLED,
		Why:            "why " + rid,
		Enforcement:    ontology.EnforcementPROSE,
		Enforceability: ontology.EnforceabilityENFORCEABLE,
	}
}

func graphWithOperator() *ontology.Graph {
	g := baseGraph()
	g.Operators = append(g.Operators, ontology.Operator{
		ID:            "OP-1",
		Stakeholder:   "outsider",
		Lifecycle:     "ACTIVE",
		ContextBudget: ontology.ContextBudget{Limit: 100, Measure: ontology.BudgetMeasureNODE_COUNT},
	})
	return g
}

func writeTempGraph(t *testing.T, g *ontology.Graph) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")
	if err := loader.WriteGraph(path, g); err != nil {
		t.Fatalf("write graph: %v", err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func reload(t *testing.T, path string) *ontology.Graph {
	t.Helper()
	g, err := loader.LoadGraph(path)
	if err != nil {
		t.Fatalf("reload %s: %v", path, err)
	}
	return g
}

func assertApplyFails(t *testing.T, path string, p Proposal, wantSubstr string) {
	t.Helper()
	before := readFile(t, path)
	err := Apply(path, today, p)
	if err == nil {
		t.Fatalf("Apply %s: expected error, got nil", p.Kind())
	}
	if wantSubstr != "" && !containsString(err.Error(), wantSubstr) {
		t.Errorf("Apply %s error = %q, want substring %q", p.Kind(), err.Error(), wantSubstr)
	}
	after := readFile(t, path)
	if before != after {
		t.Errorf("Apply %s: graph on disk changed despite failure", p.Kind())
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func findReq(g *ontology.Graph, id string) (ontology.Requirement, bool) {
	for _, r := range g.Requirements {
		if r.ID == id {
			return r, true
		}
	}
	return ontology.Requirement{}, false
}

func findConflict(g *ontology.Graph, id string) (ontology.Conflict, bool) {
	for _, c := range g.Conflicts {
		if c.ID == id {
			return c, true
		}
	}
	return ontology.Conflict{}, false
}
