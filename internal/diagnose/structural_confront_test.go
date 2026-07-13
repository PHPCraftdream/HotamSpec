package diagnose

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// structuralFixtureGraph builds a small graph with two SETTLED requirements
// (R-alpha, R-beta) sharing assumption A-shared with no mediating Conflict
// node — the precondition for a latent shared-assumption cluster — plus one
// requirement (R-gamma) resting only on an unrelated assumption A-other. A
// single conflict C-xy on axis "latency-vs-completeness" provides the axis
// anchor for co-reference tests. All assumption reference counts stay well
// under GenericAssumptionThreshold (8) so the frequency filter never
// suppresses the signal.
func structuralFixtureGraph() *ontology.Graph {
	return &ontology.Graph{
		Axes: []ontology.Axis{
			{Slug: "latency-vs-completeness", Description: "speed vs full check"},
		},
		Assumptions: []ontology.Assumption{
			{ID: "A-shared", Statement: "one customer per account", Status: ontology.AssumptionHOLDS, Owner: "finance"},
			{ID: "A-other", Statement: "api version three", Status: ontology.AssumptionHOLDS, Owner: "platform"},
		},
		Requirements: []ontology.Requirement{
			{ID: "R-alpha", Claim: "alpha settles fast", Owner: "finance", Status: ontology.StatusSETTLED, Assumptions: []string{"A-shared"}},
			{ID: "R-beta", Claim: "beta checks fully", Owner: "finance", Status: ontology.StatusSETTLED, Assumptions: []string{"A-shared"}},
			{ID: "R-gamma", Claim: "gamma reports quietly", Owner: "platform", Status: ontology.StatusSETTLED, Assumptions: []string{"A-other"}},
		},
		Conflicts: []ontology.Conflict{
			{ID: "C-xy", Axis: "latency-vs-completeness", Context: "peak load", Members: []string{"R-alpha", "R-beta"}, Steward: "platform", Lifecycle: ontology.ConflictDETECTED},
		},
	}
}

// TestStructuralConfrontForRequirement_JoinsCluster proves the positive
// shared-assumption signal: a candidate naming A-shared (already shared by
// R-alpha and R-beta with no mediating Conflict) must be flagged as joining
// that latent cluster.
func TestStructuralConfrontForRequirement_JoinsCluster(t *testing.T) {
	t.Parallel()
	g := structuralFixtureGraph()
	res := StructuralConfrontForRequirement(g, []string{"A-shared"})

	if res.Clear {
		t.Fatalf("candidate sharing A-shared must be flagged, got Clear=true; result=%+v", res)
	}
	if len(res.SharedAssumptionHits) == 0 {
		t.Fatalf("expected shared-assumption hits, got none; result=%+v", res)
	}
	// The cluster must name both existing members AND the candidate label.
	for _, c := range res.SharedAssumptionHits {
		assertNoSentinelLeak(t, c)
		if !stringSliceContains(c.Members, "(candidate)") {
			t.Errorf("expected (candidate) in Members %v", c.Members)
		}
		if !stringSliceContains(c.Members, "R-alpha") {
			t.Errorf("expected R-alpha in Members %v (candidate joined its cluster)", c.Members)
		}
	}
	// A Requirement candidate has no Axis — axis hits must be empty.
	if len(res.AxisCoReferenceHits) != 0 {
		t.Errorf("Requirement candidate must have empty axis hits, got %d", len(res.AxisCoReferenceHits))
	}
}

// TestStructuralConfrontForRequirement_NoExistingReferenceIsClear proves the
// negative / non-vacuity signal: a candidate naming an assumption that NO
// existing requirement references forms no pair with anything and must NOT be
// flagged. (A candidate naming A-other WOULD correctly pair with R-gamma, which
// also references A-other — that is a true positive, not a false negative; the
// clean negative requires an assumption absent from every existing requirement.)
func TestStructuralConfrontForRequirement_NoExistingReferenceIsClear(t *testing.T) {
	t.Parallel()
	g := structuralFixtureGraph()
	// A-nonexistent is referenced by zero existing requirements — no pair can form.
	res := StructuralConfrontForRequirement(g, []string{"A-nonexistent"})
	if !res.Clear {
		t.Fatalf("candidate naming an unreferenced assumption must be clear (no pair forms), got hits: %+v", res.SharedAssumptionHits)
	}
	if len(res.SharedAssumptionHits) != 0 {
		t.Errorf("expected zero shared-assumption hits, got %d", len(res.SharedAssumptionHits))
	}
}

// TestStructuralConfrontForRequirement_EmptyAssumptionsIsClear proves a
// candidate with no assumptions at all produces a clean, non-nil result.
func TestStructuralConfrontForRequirement_EmptyAssumptionsIsClear(t *testing.T) {
	t.Parallel()
	g := structuralFixtureGraph()
	res := StructuralConfrontForRequirement(g, nil)
	if !res.Clear {
		t.Fatalf("candidate with no assumptions must be clear, got hits: %+v", res.SharedAssumptionHits)
	}
	if res.SharedAssumptionHits == nil {
		t.Error("SharedAssumptionHits must be non-nil even when empty (JSON shape stability)")
	}
	if res.AxisCoReferenceHits == nil {
		t.Error("AxisCoReferenceHits must be non-nil even when empty (JSON shape stability)")
	}
}

// TestStructuralConfrontForConflict_AxisCoReference proves the positive
// axis-co-reference signal: a candidate Conflict on an axis already used by an
// existing Conflict (C-xy) must be flagged as co-referencing that tension
// dimension.
func TestStructuralConfrontForConflict_AxisCoReference(t *testing.T) {
	t.Parallel()
	g := structuralFixtureGraph()
	res := StructuralConfrontForConflict(g, "latency-vs-completeness", nil)

	if res.Clear {
		t.Fatalf("candidate on existing axis must be flagged, got Clear=true; result=%+v", res)
	}
	if len(res.AxisCoReferenceHits) == 0 {
		t.Fatalf("expected axis co-reference hits, got none; result=%+v", res)
	}
	for _, c := range res.AxisCoReferenceHits {
		assertNoSentinelLeak(t, c)
		if !stringSliceContains(c.Members, "(candidate)") {
			t.Errorf("expected (candidate) in Members %v", c.Members)
		}
		if !stringSliceContains(c.Members, "C-xy") {
			t.Errorf("expected C-xy in Members %v (candidate co-references its axis)", c.Members)
		}
	}
	// No assumptions passed — shared-assumption hits must be empty.
	if len(res.SharedAssumptionHits) != 0 {
		t.Errorf("conflict candidate with no assumptions must have empty shared hits, got %d", len(res.SharedAssumptionHits))
	}
}

// TestStructuralConfrontForConflict_NovelAxisIsClear proves a candidate
// Conflict on an axis NOT used by any existing Conflict must NOT be flagged.
func TestStructuralConfrontForConflict_NovelAxisIsClear(t *testing.T) {
	t.Parallel()
	g := structuralFixtureGraph()
	res := StructuralConfrontForConflict(g, "cost-vs-quality", nil)
	if !res.Clear {
		t.Fatalf("candidate on a novel axis must be clear, got hits: %+v", res.AxisCoReferenceHits)
	}
}

// TestStructuralConfrontForConflict_WithAssumptionsRunsBothChecks proves a
// Conflict candidate carrying a SharedAssumption triggers BOTH the
// shared-assumption cluster check and the axis-co-reference check in one call.
func TestStructuralConfrontForConflict_WithAssumptionsRunsBothChecks(t *testing.T) {
	t.Parallel()
	g := structuralFixtureGraph()
	res := StructuralConfrontForConflict(g, "latency-vs-completeness", []string{"A-shared"})

	if res.Clear {
		t.Fatalf("candidate with shared assumption + existing axis must be flagged on both sides, got Clear=true; result=%+v", res)
	}
	if len(res.SharedAssumptionHits) == 0 {
		t.Errorf("expected shared-assumption hits for a conflict candidate naming A-shared, got none")
	}
	if len(res.AxisCoReferenceHits) == 0 {
		t.Errorf("expected axis co-reference hits for a conflict candidate on an existing axis, got none")
	}
	for _, c := range res.SharedAssumptionHits {
		assertNoSentinelLeak(t, c)
	}
	for _, c := range res.AxisCoReferenceHits {
		assertNoSentinelLeak(t, c)
	}
}

// TestStructuralConfront_SentinelNeverLeaks is the dedicated negative proof
// that the synthetic-node technique never leaks the raw candidateSentinelID
// string into any user-facing field of any returned Candidate. This runs both
// check functions in their positive (hit-producing) configurations and scans
// every string field.
func TestStructuralConfront_SentinelNeverLeaks(t *testing.T) {
	t.Parallel()
	g := structuralFixtureGraph()

	allResults := []StructuralConfrontResult{
		StructuralConfrontForRequirement(g, []string{"A-shared"}),
		StructuralConfrontForConflict(g, "latency-vs-completeness", []string{"A-shared"}),
	}
	for i, res := range allResults {
		for _, c := range res.SharedAssumptionHits {
			if strings.Contains(c.ID, candidateSentinelID) ||
				strings.Contains(c.Evidence, candidateSentinelID) ||
				strings.Contains(c.Recommendation, candidateSentinelID) {
				t.Errorf("result[%d] shared-assumption hit leaked sentinel: %+v", i, c)
			}
			for _, m := range c.Members {
				if strings.Contains(m, candidateSentinelID) {
					t.Errorf("result[%d] shared-assumption Member leaked sentinel: %q", i, m)
				}
			}
		}
		for _, c := range res.AxisCoReferenceHits {
			if strings.Contains(c.ID, candidateSentinelID) ||
				strings.Contains(c.Evidence, candidateSentinelID) ||
				strings.Contains(c.Recommendation, candidateSentinelID) {
				t.Errorf("result[%d] axis hit leaked sentinel: %+v", i, c)
			}
			for _, m := range c.Members {
				if strings.Contains(m, candidateSentinelID) {
					t.Errorf("result[%d] axis Member leaked sentinel: %q", i, m)
				}
			}
		}
	}
}

// assertNoSentinelLeak fails the test if any user-facing string field of c
// contains the raw sentinel id.
func assertNoSentinelLeak(t *testing.T, c Candidate) {
	t.Helper()
	if strings.Contains(c.ID, candidateSentinelID) {
		t.Errorf("Candidate ID leaked sentinel: %q", c.ID)
	}
	if strings.Contains(c.Evidence, candidateSentinelID) {
		t.Errorf("Candidate Evidence leaked sentinel: %q", c.Evidence)
	}
	if strings.Contains(c.Recommendation, candidateSentinelID) {
		t.Errorf("Candidate Recommendation leaked sentinel: %q", c.Recommendation)
	}
	for _, m := range c.Members {
		if strings.Contains(m, candidateSentinelID) {
			t.Errorf("Candidate Member leaked sentinel: %q", m)
		}
	}
}
