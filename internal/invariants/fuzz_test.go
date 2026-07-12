package invariants

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

type fuzzReader struct {
	data []byte
	pos  int
}

func (r *fuzzReader) byte() byte {
	if r.pos >= len(r.data) {
		return 0
	}
	b := r.data[r.pos]
	r.pos++
	return b
}

func (r *fuzzReader) pick(opts []string) string {
	if len(opts) == 0 {
		return ""
	}
	return opts[int(r.byte())%len(opts)]
}

var fuzzOwners = []string{"alice", "bob", "carol"}
var fuzzStakeholders = []ontology.Stakeholder{
	{ID: "alice", Name: "Alice", Domain: "x"},
	{ID: "bob", Name: "Bob", Domain: "y"},
	{ID: "carol", Name: "Carol", Domain: "z"},
}
var fuzzStatuses = []string{
	"DRAFT",
	"SETTLED",
	"OPEN(which scope?)",
	"REJECTED",
}
var fuzzAxes = []ontology.Axis{
	{Slug: "cost-vs-flexibility", Description: "cost vs flexibility"},
	{Slug: "speed-vs-safety", Description: "speed vs safety"},
}
var fuzzConflictLifecycles = []string{
	"DETECTED",
	"ACKNOWLEDGED",
	"DECIDED(steward chose R-1)",
	"HELD(awaiting data)",
}

func buildFuzzGraph(r *fuzzReader) *ontology.Graph {
	axes := []ontology.Axis{fuzzAxes[int(r.byte())%len(fuzzAxes)]}
	nReqs := int(r.byte())%3 + 2
	reqs := make([]ontology.Requirement, 0, nReqs)
	ownersUsed := map[string]bool{}
	for i := 0; i < nReqs; i++ {
		owner := r.pick(fuzzOwners)
		ownersUsed[owner] = true
		rid := "R-" + indexSuffix(i+1)
		reqs = append(reqs, ontology.Requirement{
			ID:             rid,
			Claim:          "claim " + rid,
			Owner:          owner,
			Status:         r.pick(fuzzStatuses),
			Enforcement:    ontology.EnforcementPROSE,
			Enforceability: ontology.EnforceabilityENFORCEABLE,
		})
	}
	g := &ontology.Graph{
		Axes:         axes,
		Stakeholders: fuzzStakeholders,
		Requirements: reqs,
	}
	if r.byte()%2 == 0 {
		context := "fuzz scenario " + indexSuffix(int(r.byte()))
		members := []string{"R-1", "R-2"}
		if nReqs >= 2 {
			members = []string{reqs[0].ID, reqs[1].ID}
		}
		steward := r.pick(fuzzOwners)
		c := ontology.Conflict{
			ID:        ontology.ConflictIdentity(axes[0].Slug, context),
			Axis:      axes[0].Slug,
			Context:   context,
			Members:   members,
			Steward:   steward,
			Lifecycle: r.pick(fuzzConflictLifecycles),
			DecidedBy: r.pick(fuzzOwners),
		}
		g.Conflicts = []ontology.Conflict{c}
	}
	return g
}

func indexSuffix(n int) string {
	if n <= 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func FuzzAllViolations_NoPanicOnFuzzedGraph(f *testing.F) {
	cases := [][]byte{
		{0},
		{1, 2, 3},
		{5, 5, 5, 5, 5},
		{255, 255, 255, 255, 255, 255},
		{0, 1, 2, 0, 1, 2, 0},
	}
	for _, c := range cases {
		f.Add(c)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		r := &fuzzReader{data: data}
		g := buildFuzzGraph(r)
		_ = AllViolations(g)
	})
}

func FuzzAllViolations_DeterministicOnSameGraph(f *testing.F) {
	cases := [][]byte{
		{0},
		{1, 2, 3, 4},
		{9, 9, 9, 9, 9, 9},
		{0, 0, 0, 0, 0, 0, 0},
	}
	for _, c := range cases {
		f.Add(c)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		r := &fuzzReader{data: data}
		g := buildFuzzGraph(r)
		first := AllViolations(g)
		second := AllViolations(g)
		if !violationsEqual(first, second) {
			t.Fatalf("AllViolations is non-deterministic: first=%v second=%v", first, second)
		}
	})
}

func violationsEqual(a, b []Violation) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
