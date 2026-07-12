package proposal

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

// TestApplyBatch_ThreeValid_AppliesAll proves a batch of 3+ valid proposals
// of different kinds lands in a single atomic write: all nodes appear after
// ApplyBatch returns.
func TestApplyBatch_ThreeValid_AppliesAll(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	batch := []Proposal{
		ProposedRequirement{ID: "R-b1", Claim: "batch one", Owner: "sa", Status: ontology.StatusDRAFT},
		ProposedRequirement{ID: "R-b2", Claim: "batch two", Owner: "sb", Status: ontology.StatusDRAFT},
		ProposedAxis{Slug: "speed-vs-quality", Description: "d"},
	}
	if err := ApplyBatch(path, today, batch); err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}
	g := reload(t, path)
	if _, ok := findReq(g, "R-b1"); !ok {
		t.Error("R-b1 missing after batch")
	}
	if _, ok := findReq(g, "R-b2"); !ok {
		t.Error("R-b2 missing after batch")
	}
	found := false
	for _, a := range g.Axes {
		if a.Slug == "speed-vs-quality" {
			found = true
		}
	}
	if !found {
		t.Error("axis speed-vs-quality missing after batch")
	}
}

// TestApplyBatch_SecondDependsOnFirst proves the rolling in-memory baseline:
// proposal 2 references a requirement that only EXISTS because proposal 1
// just created it in the same batch. This is only possible because ApplyBatch
// applies proposal 1 to the in-memory graph before validating/mutating
// proposal 2 — exactly like two sequential single Applies would.
func TestApplyBatch_SecondDependsOnFirst(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	batch := []Proposal{
		ProposedRequirement{ID: "R-batch-dep", Claim: "created then rejected", Owner: "sa", Status: ontology.StatusDRAFT},
		ProposedRejection{RequirementID: "R-batch-dep", Reason: "rejected in same batch"},
	}
	if err := ApplyBatch(path, today, batch); err != nil {
		t.Fatalf("ApplyBatch: %v", err)
	}
	g := reload(t, path)
	r, ok := findReq(g, "R-batch-dep")
	if !ok {
		t.Fatal("R-batch-dep missing — proposal 1 must have landed")
	}
	if r.Status != ontology.StatusREJECTED {
		t.Errorf("Status = %q, want REJECTED (proposal 2 must see proposal 1's addition)", r.Status)
	}
}

// TestApplyBatch_NthInvalid_GraphUnchanged is the atomicity guarantee: when
// proposal N (N>1) fails (here: ProposedRejection on a nonexistent anchor),
// the graph on disk must be byte-identical to its pre-batch state and the
// valid proposals before it must NOT have landed.
func TestApplyBatch_NthInvalid_GraphUnchanged(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	before := readFile(t, path)

	batch := []Proposal{
		ProposedRequirement{ID: "R-ok", Claim: "valid first", Owner: "sa", Status: ontology.StatusDRAFT},
		ProposedRejection{RequirementID: "R-does-not-exist", Reason: "ghost anchor"},
	}
	err := ApplyBatch(path, today, batch)
	if err == nil {
		t.Fatal("expected error for nonexistent requirement_id in proposal 2")
	}
	if !containsString(err.Error(), "batch proposal 2 of 2") {
		t.Errorf("error = %q, want it to identify proposal 2 as the failure", err.Error())
	}

	after := readFile(t, path)
	if before != after {
		t.Fatal("graph on disk changed despite batch failure — batch must be all-or-nothing")
	}
	// The valid proposal 1 must NOT have landed.
	g := reload(t, path)
	if _, ok := findReq(g, "R-ok"); ok {
		t.Error("R-ok landed despite batch failure — proposal 1 must be rolled back")
	}
}

// TestApplyBatch_InvariantViolation_GraphUnchanged proves that a proposal
// which would introduce a new invariant violation (not a mutation error)
// also aborts the whole batch atomically, mirroring Apply's own guard.
func TestApplyBatch_InvariantViolation_GraphUnchanged(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	before := readFile(t, path)

	// "not-r-prefixed" violates check_typed_anchors (R-prefix discipline).
	batch := []Proposal{
		ProposedRequirement{ID: "R-ok", Claim: "valid", Owner: "sa", Status: ontology.StatusDRAFT},
		ProposedRequirement{ID: "not-r-prefixed", Claim: "bad", Owner: "sa", Status: ontology.StatusDRAFT},
	}
	err := ApplyBatch(path, today, batch)
	if err == nil {
		t.Fatal("expected invariant-violation error for proposal 2")
	}
	after := readFile(t, path)
	if before != after {
		t.Fatal("graph on disk changed despite invariant-violation failure")
	}
}

// TestApplyBatch_EmptyBatchFails — an empty batch is a likely caller mistake
// (empty dir, glob matched nothing); reject explicitly rather than silently
// rewriting the graph with no changes.
func TestApplyBatch_EmptyBatchFails(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	if err := ApplyBatch(path, today, nil); err == nil {
		t.Fatal("expected error for empty batch")
	}
}
