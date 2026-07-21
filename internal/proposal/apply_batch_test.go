package proposal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// testConflictChecker is a test-local ConflictChecker, functionally
// equivalent to cmd/hotam's batchConflictChecker (which cannot be imported
// here — cmd/hotam imports internal/proposal, so importing it back would be
// a real import cycle). Test files ARE excluded from
// R-core-periphery-import-ratchet (TestCorePeriphery_ImportRatchet only
// scans non-test .go files), so this file may import internal/diagnose
// directly.
func testConflictChecker(g *ontology.Graph, claim string) error {
	result := diagnose.Confront(g, claim)
	for _, h := range result.Settled {
		if diagnose.IsBlockingHit(h) {
			return fmt.Errorf(
				"semantically contradicts SETTLED requirement %s: %q "+
					"(opposite-marker signal; shared tokens: [%s])",
				h.ID, h.Claim, strings.Join(h.Shared, ", "))
		}
	}
	return nil
}

// testProvenanceChecker is a test-local ProvenanceChecker, functionally
// equivalent to cmd/hotam's batchProvenanceChecker's core check (it always
// enforces the gate — no manifest.json opt-in flag to resolve here, since
// this package cannot import internal/loader for it without a needless
// dependency for a test double). It simulates the post-merge result via
// SimulateRequirementResult and requires non-empty SourceRefs,
// LastReviewedAt, and ReviewAfter for a SETTLED result, matching
// cmd/hotam/provenance_gate.go's provenanceCheck field-by-field.
func testProvenanceChecker(g *ontology.Graph, today string, p ProposedRequirement) error {
	if p.Status != ontology.StatusSETTLED {
		return nil
	}
	result, err := SimulateRequirementResult(g, today, p)
	if err != nil {
		return err
	}
	var missing []string
	if len(result.SourceRefs) == 0 {
		missing = append(missing, "source_refs")
	}
	if result.LastReviewedAt == "" {
		missing = append(missing, "last_reviewed_at")
	}
	if result.ReviewAfter == "" {
		missing = append(missing, "review_after")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("missing provenance on %s: %s", p.ID, strings.Join(missing, ", "))
}

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
	if err := ApplyBatch(path, today, batch, nil, nil); err != nil {
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
	if err := ApplyBatch(path, today, batch, nil, nil); err != nil {
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
	err := ApplyBatch(path, today, batch, nil, nil)
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
	err := ApplyBatch(path, today, batch, nil, nil)
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
	if err := ApplyBatch(path, today, nil, nil, nil); err == nil {
		t.Fatal("expected error for empty batch")
	}
}

// TestApplyBatch_SemanticConflict_PreExistingRefusesWholeBatch proves the
// R10-a batch semantic-conflict gate: a batch item whose claim carries an
// opposite marker against a PRE-EXISTING SETTLED requirement aborts the
// ENTIRE batch — including the benign first item that would have applied fine
// standalone — and the graph on disk is byte-identical to its pre-batch state.
func TestApplyBatch_SemanticConflict_PreExistingRefusesWholeBatch(t *testing.T) {
	t.Parallel()
	// Graph with a SETTLED requirement carrying "always encrypt".
	g := baseGraph()
	g.Requirements = append(g.Requirements, ontology.Requirement{
		ID:             "R-enc-always",
		Claim:          "export service must always encrypt records",
		Owner:          "sa",
		Status:         ontology.StatusSETTLED,
		Why:            "seed for batch gate test",
		Enforcement:    ontology.EnforcementPROSE,
		Enforceability: ontology.EnforceabilityENFORCEABLE,
	})
	path := writeTempGraph(t, g)
	before := readFile(t, path)

	batch := []Proposal{
		// A benign first item that would apply fine standalone.
		ProposedRequirement{ID: "R-ok-benign", Claim: "a benign requirement about tea grading", Owner: "sa", Status: ontology.StatusDRAFT},
		// Contradicts R-enc-always (opposite marker always/never + topical "encrypt").
		ProposedRequirement{ID: "R-enc-never", Claim: "export service must never encrypt records", Owner: "sb", Status: ontology.StatusSETTLED},
	}
	err := ApplyBatch(path, today, batch, testConflictChecker, nil)
	if err == nil {
		t.Fatal("expected ApplyBatch to refuse the contradicting batch item")
	}
	if !containsString(err.Error(), "R-enc-always") {
		t.Errorf("error must name the conflicting anchor R-enc-always:\n%s", err.Error())
	}
	if !containsString(err.Error(), "batch proposal 2 of 2") {
		t.Errorf("error must identify batch proposal 2 as the failure:\n%s", err.Error())
	}

	// The whole batch must be refused: graph byte-identical, benign item NOT landed.
	after := readFile(t, path)
	if before != after {
		t.Fatal("graph on disk changed despite batch refusal — batch must be all-or-nothing")
	}
	rg := reload(t, path)
	if _, ok := findReq(rg, "R-ok-benign"); ok {
		t.Error("R-ok-benign landed despite batch refusal — the benign first item must be rolled back")
	}
}

// TestApplyBatch_SemanticConflict_WithinBatchRefusesWholeBatch proves the
// "checked against previous items of the same batch" claim: two requirements
// that contradict EACH OTHER WITHIN the batch (neither exists in the graph
// beforehand) must also be refused. Proposal 1 ("always encrypt") applies to
// the rolling in-memory graph fine; proposal 2 ("never encrypt") is then
// confronted against that rolling graph — which now contains proposal 1 — and
// the blocking hit aborts the whole batch.
func TestApplyBatch_SemanticConflict_WithinBatchRefusesWholeBatch(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	before := readFile(t, path)

	batch := []Proposal{
		// First item: "always encrypt" — no conflict against the base graph.
		ProposedRequirement{ID: "R-bat-always", Claim: "export service must always encrypt records", Owner: "sa", Status: ontology.StatusSETTLED},
		// Second item: "never encrypt" — contradicts the FIRST item, now in the rolling graph.
		ProposedRequirement{ID: "R-bat-never", Claim: "export service must never encrypt records", Owner: "sb", Status: ontology.StatusSETTLED},
	}
	err := ApplyBatch(path, today, batch, testConflictChecker, nil)
	if err == nil {
		t.Fatal("expected ApplyBatch to refuse the within-batch contradiction")
	}
	if !containsString(err.Error(), "R-bat-always") {
		t.Errorf("error must name the earlier same-batch anchor R-bat-always (proves rolling-graph check):\n%s", err.Error())
	}
	if !containsString(err.Error(), "batch proposal 2 of 2") {
		t.Errorf("error must identify batch proposal 2 as the failure:\n%s", err.Error())
	}

	after := readFile(t, path)
	if before != after {
		t.Fatal("graph changed despite within-batch contradiction refusal — batch must be all-or-nothing")
	}
	rg := reload(t, path)
	if _, ok := findReq(rg, "R-bat-always"); ok {
		t.Error("R-bat-always landed despite batch refusal — even the first item must be rolled back")
	}
}

// TestApplyBatch_NoConflict_StillSucceeds is the regression guard: a batch
// with NO semantic conflicts must still land exactly as before the batch gate
// was added. This would FAIL if the gate were too aggressive (false positive
// on ordinary related-but-not-contradicting requirements).
func TestApplyBatch_NoConflict_StillSucceeds(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())

	batch := []Proposal{
		// Two requirements that share topical vocabulary but AGREE (no opposite
		// marker) — the gate must not fire on mere relatedness.
		ProposedRequirement{ID: "R-encrypt-rules-a", Claim: "export service must encrypt records at rest", Owner: "sa", Status: ontology.StatusDRAFT},
		ProposedRequirement{ID: "R-encrypt-rules-b", Claim: "export service must encrypt records in transit", Owner: "sb", Status: ontology.StatusDRAFT},
	}
	if err := ApplyBatch(path, today, batch, testConflictChecker, nil); err != nil {
		t.Fatalf("non-conflicting batch must succeed (gate too aggressive?): %v", err)
	}
	g := reload(t, path)
	for _, id := range []string{"R-encrypt-rules-a", "R-encrypt-rules-b"} {
		if _, ok := findReq(g, id); !ok {
			t.Errorf("%s missing after non-conflicting batch", id)
		}
	}
}

// TestApplyBatch_ProvenanceChecker_RefusesIncompleteProvenance proves batch
// parity for the provenance gate (task #158): a batch item with a SETTLED
// status but zero provenance fields, checked via an injected
// ProvenanceChecker, aborts the ENTIRE batch atomically — including the
// benign first item — same shape as the semantic-conflict batch tests above.
func TestApplyBatch_ProvenanceChecker_RefusesIncompleteProvenance(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())
	before := readFile(t, path)

	batch := []Proposal{
		ProposedRequirement{ID: "R-prov-benign", Claim: "a benign draft requirement", Owner: "sa", Status: ontology.StatusDRAFT},
		ProposedRequirement{ID: "R-prov-bare", Claim: "a bare settled requirement with no provenance", Owner: "sb", Status: ontology.StatusSETTLED},
	}
	err := ApplyBatch(path, today, batch, nil, testProvenanceChecker)
	if err == nil {
		t.Fatal("expected ApplyBatch to refuse the bare SETTLED batch item")
	}
	if !containsString(err.Error(), "source_refs") {
		t.Errorf("error must name missing source_refs:\n%s", err.Error())
	}
	if !containsString(err.Error(), "batch proposal 2 of 2") {
		t.Errorf("error must identify batch proposal 2 as the failure:\n%s", err.Error())
	}

	after := readFile(t, path)
	if before != after {
		t.Fatal("graph on disk changed despite batch provenance refusal — batch must be all-or-nothing")
	}
	rg := reload(t, path)
	if _, ok := findReq(rg, "R-prov-benign"); ok {
		t.Error("R-prov-benign landed despite batch refusal — the benign first item must be rolled back")
	}
}

// TestApplyBatch_ProvenanceChecker_CompleteProvenanceSucceeds is the
// regression guard: a SETTLED batch item WITH complete provenance must still
// land via the injected ProvenanceChecker.
func TestApplyBatch_ProvenanceChecker_CompleteProvenanceSucceeds(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())

	batch := []Proposal{
		ProposedRequirement{
			ID: "R-prov-complete", Claim: "a settled requirement with full provenance",
			Owner: "sa", Status: ontology.StatusSETTLED,
			SourceRefs:     []string{"https://example.com/source"},
			LastReviewedAt: today,
			ReviewAfter:    "2027-01-01",
			Evidence:       []string{"resolver review"},
		},
	}
	if err := ApplyBatch(path, today, batch, nil, testProvenanceChecker); err != nil {
		t.Fatalf("complete-provenance batch must succeed: %v", err)
	}
	g := reload(t, path)
	if _, ok := findReq(g, "R-prov-complete"); !ok {
		t.Error("R-prov-complete missing after complete-provenance batch")
	}
}

// TestApplyBatch_ProvenanceChecker_UpdatePreservingProvenanceSucceeds is the
// CREATE-vs-UPDATE batch proof: land a complete-provenance requirement first,
// then an UPDATE-only-claim proposal (within the SAME batch, via the rolling
// graph) that omits provenance fields must still succeed because the
// simulated post-merge result still carries the earlier provenance.
func TestApplyBatch_ProvenanceChecker_UpdatePreservingProvenanceSucceeds(t *testing.T) {
	t.Parallel()
	path := writeTempGraph(t, baseGraph())

	batch := []Proposal{
		ProposedRequirement{
			ID: "R-prov-rolling", Claim: "original claim with provenance",
			Owner: "sa", Status: ontology.StatusSETTLED,
			SourceRefs:     []string{"https://example.com/rolling"},
			LastReviewedAt: today,
			ReviewAfter:    "2027-01-01",
			Evidence:       []string{"resolver review"},
		},
		// UPDATE within the same batch: only claim changes, provenance omitted.
		ProposedRequirement{
			ID: "R-prov-rolling", Claim: "updated claim, provenance omitted",
			Owner: "sa", Status: ontology.StatusSETTLED,
		},
	}
	if err := ApplyBatch(path, today, batch, nil, testProvenanceChecker); err != nil {
		t.Fatalf("UPDATE preserving provenance via rolling graph must succeed: %v", err)
	}
	g := reload(t, path)
	r, ok := findReq(g, "R-prov-rolling")
	if !ok {
		t.Fatal("R-prov-rolling missing after batch")
	}
	if r.Claim != "updated claim, provenance omitted" {
		t.Errorf("Claim = %q, want the update to have applied", r.Claim)
	}
	if len(r.SourceRefs) != 1 || r.SourceRefs[0] != "https://example.com/rolling" {
		t.Errorf("SourceRefs = %v, want preserved from the earlier batch item", r.SourceRefs)
	}
}
