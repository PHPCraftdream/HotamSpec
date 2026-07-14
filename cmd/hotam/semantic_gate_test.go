package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// setupGateTestDomain scaffolds a clean, invariant-valid minimal domain (via
// initDomain — one seed stakeholder "owner" + one seed SETTLED requirement)
// placed under a domains/<name> parent so resolveClaudeMDPath does not
// auto-write any crystal. The seed requirement carries no opposite markers,
// so the first test requirement can be landed without triggering the gate.
func setupGateTestDomain(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	domainDir := filepath.Join(root, "domains", "gate-test")
	if _, err := initDomain(domainDir, "gate-test"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	return domainDir
}

// writeReqProposalJSON writes a minimal valid Requirement proposal JSON to a
// temp file and returns its path. The claim and id are interpolated directly
// (callers must keep them JSON-safe — no quotes or backslashes).
func writeReqProposalJSON(t *testing.T, id, claim string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), id+".json")
	jsonStr := `{
		"kind": "Requirement",
		"id": "` + id + `",
		"claim": "` + claim + `",
		"owner": "owner",
		"status": "SETTLED",
		"why": "semantic-conflict gate test"
	}`
	if err := os.WriteFile(path, []byte(jsonStr), 0o644); err != nil {
		t.Fatalf("write proposal %s: %v", path, err)
	}
	return path
}

// addTestConflict creates a valid Conflict node in the domain's graph via the
// proposal pipeline (steward stakeholder + axis + conflict), returning the
// Conflict ID. members must already exist as requirements in the graph.
func addTestConflict(t *testing.T, graphPath, today string, members []string) string {
	t.Helper()
	if err := proposal.Apply(graphPath, today, proposal.ProposedStakeholder{
		ID: "steward-gate", Name: "Gate Steward", Domain: "gate-test",
	}); err != nil {
		t.Fatalf("add steward: %v", err)
	}
	if err := proposal.Apply(graphPath, today, proposal.ProposedAxis{
		Slug: "security", Description: "security tension axis",
	}); err != nil {
		t.Fatalf("add axis: %v", err)
	}
	pc := proposal.ProposedConflict{
		Axis:    "security",
		Context: "encrypt export tension",
		Members: members,
		Steward: "steward-gate",
	}
	if err := proposal.Apply(graphPath, today, pc); err != nil {
		t.Fatalf("add conflict: %v", err)
	}
	return ontology.ConflictIdentity("security", "encrypt export tension")
}

// TestSemanticConflictGate_RefusesOppositeMarkerConflict is the EXACT review
// scenario: two SETTLED requirements whose claims carry opposite markers
// ("always encrypt" vs "never encrypt"). The second one's land (no ack flags)
// must REFUSE with a clear message naming the first anchor and suggesting both
// remediation paths. The graph must remain untouched (no R-gate-never).
func TestSemanticConflictGate_RefusesOppositeMarkerConflict(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	gp := graphPathForDomain(domainDir)

	// Land the first SETTLED requirement — no conflict exists yet.
	first := writeReqProposalJSON(t, "R-gate-always",
		"export service must always encrypt records")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", first}); err != nil {
		t.Fatalf("land first requirement: %v", err)
	}

	// Try to land the contradicting requirement — must refuse.
	second := writeReqProposalJSON(t, "R-gate-never",
		"export service must never encrypt records")
	err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", second})
	if err == nil {
		t.Fatal("expected gate to refuse the contradicting requirement, got nil error")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "R-gate-always") {
		t.Errorf("error must name the conflicting anchor R-gate-always:\n%s", errStr)
	}
	if !strings.Contains(errStr, "refusing to land") {
		t.Errorf("error must state the refusal:\n%s", errStr)
	}
	if !strings.Contains(errStr, "--ack-conflict") {
		t.Errorf("error must suggest --ack-conflict:\n%s", errStr)
	}
	if !strings.Contains(errStr, "--decision-ref") {
		t.Errorf("error must suggest --decision-ref:\n%s", errStr)
	}

	// The graph must NOT contain the refused requirement.
	graphData, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	if strings.Contains(string(graphData), "R-gate-never") {
		t.Error("graph must not contain the refused requirement — gate left the graph untouched")
	}
}

// TestSemanticConflictGate_AckConflictSucceeds proves that supplying
// --ack-conflict <existing-C-id> overrides the refusal and the requirement
// lands successfully.
func TestSemanticConflictGate_AckConflictSucceeds(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	gp := graphPathForDomain(domainDir)

	// Land the first SETTLED requirement.
	first := writeReqProposalJSON(t, "R-gate-always-2",
		"export service must always encrypt records")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", first}); err != nil {
		t.Fatalf("land first: %v", err)
	}

	// Create a valid Conflict node in the graph.
	conflictID := addTestConflict(t, gp, "2026-07-14",
		[]string{"R-gate-always-2", "R-domain-exists"})

	// Land the contradicting requirement WITH --ack-conflict — must succeed.
	second := writeReqProposalJSON(t, "R-gate-never-2",
		"export service must never encrypt records")
	err := cmdLand([]string{
		"--domain", domainDir, "--today", "2026-07-14",
		"--ack-conflict", conflictID,
		second,
	})
	if err != nil {
		t.Fatalf("land with --ack-conflict should succeed: %v", err)
	}

	// The graph must contain the acked requirement.
	graphData, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	if !strings.Contains(string(graphData), "R-gate-never-2") {
		t.Error("graph must contain the ack-conflict-landed requirement")
	}

	// The acked requirement's History must carry the audit trail.
	g, err := loader.LoadGraph(gp)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	r, ok := ontology.RequirementByID(g, "R-gate-never-2")
	if !ok {
		t.Fatal("R-gate-never-2 not found in graph")
	}
	found := false
	for _, h := range r.History {
		if strings.Contains(h.Summary, conflictID) {
			found = true
		}
	}
	if !found {
		t.Errorf("History must reference the acked Conflict %s; got history: %v", conflictID, r.History)
	}
}

// TestSemanticConflictGate_DecisionRefSucceedsAndPersists proves that
// --decision-ref <text> overrides the refusal, the requirement lands, and the
// decision-ref text is persisted in the requirement's History.
func TestSemanticConflictGate_DecisionRefSucceedsAndPersists(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	gp := graphPathForDomain(domainDir)

	// Land the first SETTLED requirement.
	first := writeReqProposalJSON(t, "R-gate-always-3",
		"export service must always encrypt records")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", first}); err != nil {
		t.Fatalf("land first: %v", err)
	}

	// Land the contradicting requirement WITH --decision-ref — must succeed.
	const ref = "ticket-BUG-42: steward decided both apply to different export contexts"
	second := writeReqProposalJSON(t, "R-gate-never-3",
		"export service must never encrypt records")
	err := cmdLand([]string{
		"--domain", domainDir, "--today", "2026-07-14",
		"--decision-ref", ref,
		second,
	})
	if err != nil {
		t.Fatalf("land with --decision-ref should succeed: %v", err)
	}

	// The requirement must exist and its History must carry the decision-ref.
	g, err := loader.LoadGraph(gp)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	r, ok := ontology.RequirementByID(g, "R-gate-never-3")
	if !ok {
		t.Fatal("R-gate-never-3 not found in graph")
	}
	found := false
	for _, h := range r.History {
		if strings.Contains(h.Summary, "BUG-42") {
			found = true
		}
	}
	if !found {
		t.Errorf("History must carry the decision-ref audit trail; got history: %v", r.History)
	}
}

// TestSemanticConflictGate_AckConflictNonExistentFails proves that a typo'd
// --ack-conflict ID (no such Conflict node) does NOT silently bypass the gate.
func TestSemanticConflictGate_AckConflictNonExistentFails(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)

	first := writeReqProposalJSON(t, "R-gate-always-4",
		"export service must always encrypt records")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", first}); err != nil {
		t.Fatalf("land first: %v", err)
	}

	second := writeReqProposalJSON(t, "R-gate-never-4",
		"export service must never encrypt records")
	err := cmdLand([]string{
		"--domain", domainDir, "--today", "2026-07-14",
		"--ack-conflict", "C-doesnotexist",
		second,
	})
	if err == nil {
		t.Fatal("expected error for non-existent --ack-conflict ID")
	}
	if !strings.Contains(err.Error(), "C-doesnotexist") {
		t.Errorf("error must name the invalid conflict id:\n%s", err.Error())
	}
}

// TestSemanticConflictGate_NormalLandUnaffected is the regression guard: a
// completely benign requirement (no opposite markers, no meaningful overlap)
// must land exactly as before. This would FAIL if the gate were too aggressive.
func TestSemanticConflictGate_NormalLandUnaffected(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	gp := graphPathForDomain(domainDir)

	proposalPath := filepath.Join(t.TempDir(), "normal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-gate-benign-normal-land",
		"claim": "a completely benign requirement about tea quality grading",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "regression guard for the semantic-conflict gate"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", proposalPath})
	if err != nil {
		t.Fatalf("normal non-conflicting land must succeed (gate too aggressive?): %v", err)
	}

	graphData, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	if !strings.Contains(string(graphData), "R-gate-benign-normal-land") {
		t.Error("graph must contain the benign requirement after a normal land")
	}
}

// TestSemanticConflictGate_ProposeLandPath tests that the gate fires on the
// `hotam propose requirement --land` path too (not just `hotam land <file>`),
// proving the gate applies consistently through the shared landProposalValue
// pipeline without duplicated logic.
func TestSemanticConflictGate_ProposeLandPath(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)

	// Land the first via propose --land — should succeed (no conflict yet).
	out1 := filepath.Join(t.TempDir(), "p1.json")
	if err := cmdPropose([]string{
		"requirement",
		"--id", "R-prop-gate-always",
		"--claim", "export service must always encrypt records",
		"--owner", "owner", "--status", "SETTLED",
		"--domain", domainDir, "--today", "2026-07-14",
		"--out", out1, "--land",
	}); err != nil {
		t.Fatalf("first propose --land: %v", err)
	}

	// Second via propose --land — should REFUSE.
	out2 := filepath.Join(t.TempDir(), "p2.json")
	err := cmdPropose([]string{
		"requirement",
		"--id", "R-prop-gate-never",
		"--claim", "export service must never encrypt records",
		"--owner", "owner", "--status", "SETTLED",
		"--domain", domainDir, "--today", "2026-07-14",
		"--out", out2, "--land",
	})
	if err == nil {
		t.Fatal("expected propose --land to refuse the contradicting requirement")
	}
	if !strings.Contains(err.Error(), "R-prop-gate-always") {
		t.Errorf("error must name the conflicting anchor:\n%s", err.Error())
	}

	// Third via propose --land WITH --decision-ref — should succeed.
	out3 := filepath.Join(t.TempDir(), "p3.json")
	if err := cmdPropose([]string{
		"requirement",
		"--id", "R-prop-gate-never-ack",
		"--claim", "export service must never encrypt records",
		"--owner", "owner", "--status", "SETTLED",
		"--domain", domainDir, "--today", "2026-07-14",
		"--out", out3,
		"--decision-ref", "meeting-2026-07-14",
		"--land",
	}); err != nil {
		t.Fatalf("propose --land with --decision-ref should succeed: %v", err)
	}
}

// TestSemanticConflictGate_HadConflictReturnValue directly exercises the
// semanticConflictGate (hadConflict bool, err error) contract introduced to
// fix the false-positive ack-history bug. The return value must reflect
// whether a real conflict signal (blockers) was found — INDEPENDENT of ack
// flags — so the caller can gate appendAckHistory correctly:
//
//   - no conflict + no ack   → hadConflict=false, err=nil
//   - no conflict + ack      → hadConflict=false, err=nil  (ack alone must NOT
//     imply a conflict; this is the core of the regression)
//   - real conflict + no ack → hadConflict=true,  err!=nil (refusal)
//   - real conflict + ack    → hadConflict=true,  err=nil  (ack overrides)
func TestSemanticConflictGate_HadConflictReturnValue(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)

	// Seed a SETTLED requirement carrying "always encrypt" so a contradicting
	// claim with "never encrypt" trips the opposite-marker signal.
	first := writeReqProposalJSON(t, "R-gate-had-conflict-seed",
		"export service must always encrypt records")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", first}); err != nil {
		t.Fatalf("land seed: %v", err)
	}

	const conflictingClaim = "export service must never encrypt records"
	const benignClaim = "a completely benign requirement about tea quality grading"

	// Case 1: no conflict, no ack → no conflict, no error.
	hc, err := semanticConflictGate(domainDir, proposal.ProposedRequirement{
		ID: "R-benign-1", Claim: benignClaim, Owner: "owner", Status: "SETTLED", Why: "t",
	}, landAckOptions{})
	if err != nil {
		t.Fatalf("benign claim, no ack: expected no error, got %v", err)
	}
	if hc {
		t.Error("benign claim, no ack: expected hadConflict=false, got true")
	}

	// Case 2: no conflict, but ack supplied → STILL hadConflict=false. This is
	// the regression core: ack flags must not fabricate a conflict.
	hc, err = semanticConflictGate(domainDir, proposal.ProposedRequirement{
		ID: "R-benign-2", Claim: benignClaim, Owner: "owner", Status: "SETTLED", Why: "t",
	}, landAckOptions{DecisionRef: "no conflict here"})
	if err != nil {
		t.Fatalf("benign claim, ack: expected no error, got %v", err)
	}
	if hc {
		t.Error("benign claim, ack: expected hadConflict=false, got true (ack must not imply conflict)")
	}

	// Case 3: real conflict, no ack → hadConflict=true, error (refusal).
	hc, err = semanticConflictGate(domainDir, proposal.ProposedRequirement{
		ID: "R-never-3", Claim: conflictingClaim, Owner: "owner", Status: "SETTLED", Why: "t",
	}, landAckOptions{})
	if err == nil {
		t.Fatal("conflicting claim, no ack: expected refusal error, got nil")
	}
	if !hc {
		t.Error("conflicting claim, no ack: expected hadConflict=true, got false")
	}

	// Case 4: real conflict, ack supplied → hadConflict=true, no error.
	hc, err = semanticConflictGate(domainDir, proposal.ProposedRequirement{
		ID: "R-never-4", Claim: conflictingClaim, Owner: "owner", Status: "SETTLED", Why: "t",
	}, landAckOptions{DecisionRef: "decided both apply"})
	if err != nil {
		t.Fatalf("conflicting claim, ack: expected no error, got %v", err)
	}
	if !hc {
		t.Error("conflicting claim, ack: expected hadConflict=true, got false")
	}
}

// TestSemanticConflictGate_NoFalseAckHistoryOnNonConflictingLand is the
// end-to-end regression for the R9-a bug: landing a NON-conflicting requirement
// with --decision-ref must succeed (ack flags are an optional override, never
// a requirement) AND must NOT write a false "semantic conflict acknowledged"
// History entry. Before the fix, appendAckHistory fired whenever ackOpts.hasAck()
// was true regardless of whether the gate found a real conflict.
func TestSemanticConflictGate_NoFalseAckHistoryOnNonConflictingLand(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	gp := graphPathForDomain(domainDir)

	// Land a benign requirement WITH --decision-ref. The seed requirement
	// carries no opposite markers, so there is nothing to acknowledge.
	benign := writeReqProposalJSON(t, "R-gate-benign-ack-noconflict",
		"a completely benign requirement about tea quality grading scales")
	if err := cmdLand([]string{
		"--domain", domainDir, "--today", "2026-07-14",
		"--decision-ref", "no real conflict to acknowledge",
		benign,
	}); err != nil {
		t.Fatalf("non-conflicting land with --decision-ref must succeed (ack is an override, not a requirement): %v", err)
	}

	// Read the requirement back from graph.json and inspect its History slice
	// directly: it must contain NO "semantic conflict acknowledged" entry.
	g, err := loader.LoadGraph(gp)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	r, ok := ontology.RequirementByID(g, "R-gate-benign-ack-noconflict")
	if !ok {
		t.Fatal("R-gate-benign-ack-noconflict not found in graph")
	}
	for _, h := range r.History {
		if strings.Contains(h.Summary, "semantic conflict acknowledged") {
			t.Errorf("non-conflicting land must not record a false ack entry, but History has: %q", h.Summary)
		}
	}
}

// TestApplyProposalGate_RefusesOppositeMarkerConflict is the EXACT review-9
// R9-b bypass scenario: a requirement is landed via `hotam land` (gated), then a
// semantically-contradicting requirement is applied via `hotam apply-proposal`
// (single-file, NOT --land, NOT --batch) WITHOUT --ack-conflict /
// --decision-ref. Before the fix this path ignored the gate entirely and
// silently succeeded; after the fix it must REFUSE with the same kind of message
// `land` produces, and leave the graph untouched.
func TestApplyProposalGate_RefusesOppositeMarkerConflict(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	gp := graphPathForDomain(domainDir)

	// Land the first SETTLED requirement via `hotam land` — no conflict yet.
	first := writeReqProposalJSON(t, "R-apply-always",
		"export service must always encrypt records")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", first}); err != nil {
		t.Fatalf("land first requirement: %v", err)
	}

	// Attempt `hotam apply-proposal` (single-file) on the contradicting
	// requirement WITHOUT ack — the review's exact bypass. Must REFUSE.
	second := writeReqProposalJSON(t, "R-apply-never",
		"export service must never encrypt records")
	err := cmdApplyProposal([]string{"--domain", domainDir, "--today", "2026-07-14", second})
	if err == nil {
		t.Fatal("expected apply-proposal gate to refuse the contradicting requirement, got nil error")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "R-apply-always") {
		t.Errorf("error must name the conflicting anchor R-apply-always:\n%s", errStr)
	}
	if !strings.Contains(errStr, "refusing to land") {
		t.Errorf("error must state the refusal:\n%s", errStr)
	}
	if !strings.Contains(errStr, "--ack-conflict") {
		t.Errorf("error must suggest --ack-conflict:\n%s", errStr)
	}
	if !strings.Contains(errStr, "--decision-ref") {
		t.Errorf("error must suggest --decision-ref:\n%s", errStr)
	}

	// The graph must NOT contain the refused requirement.
	graphData, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	if strings.Contains(string(graphData), "R-apply-never") {
		t.Error("graph must not contain the refused requirement — gate left the graph untouched")
	}
}

// TestApplyProposalGate_AckConflictSucceedsAndPersists proves that supplying
// --ack-conflict <existing-C-id> through `hotam apply-proposal` overrides the
// refusal, the requirement is applied, and the resulting graph.json carries the
// ack-history audit trail on the applied requirement (read back directly, since
// apply-proposal does not regenerate docs).
func TestApplyProposalGate_AckConflictSucceedsAndPersists(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	gp := graphPathForDomain(domainDir)

	// Land the first SETTLED requirement.
	first := writeReqProposalJSON(t, "R-apply-always-2",
		"export service must always encrypt records")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", first}); err != nil {
		t.Fatalf("land first: %v", err)
	}

	// Create a valid Conflict node in the graph.
	conflictID := addTestConflict(t, gp, "2026-07-14",
		[]string{"R-apply-always-2", "R-domain-exists"})

	// apply-proposal the contradicting requirement WITH --ack-conflict — must succeed.
	second := writeReqProposalJSON(t, "R-apply-never-2",
		"export service must never encrypt records")
	err := cmdApplyProposal([]string{
		"--domain", domainDir, "--today", "2026-07-14",
		"--ack-conflict", conflictID,
		second,
	})
	if err != nil {
		t.Fatalf("apply-proposal with --ack-conflict should succeed: %v", err)
	}

	// The graph must contain the acked requirement.
	graphData, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph: %v", err)
	}
	if !strings.Contains(string(graphData), "R-apply-never-2") {
		t.Error("graph must contain the ack-conflict-applied requirement")
	}

	// The acked requirement's History must carry the audit trail.
	g, err := loader.LoadGraph(gp)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	r, ok := ontology.RequirementByID(g, "R-apply-never-2")
	if !ok {
		t.Fatal("R-apply-never-2 not found in graph")
	}
	found := false
	for _, h := range r.History {
		if strings.Contains(h.Summary, conflictID) {
			found = true
		}
	}
	if !found {
		t.Errorf("History must reference the acked Conflict %s; got history: %v", conflictID, r.History)
	}
}

// TestApplyProposalGate_NoFalseAckHistoryOnNonConflictingApply is the
// false-positive guard carried over to the apply-proposal path (same class of
// check as TestSemanticConflictGate_NoFalseAckHistoryOnNonConflictingLand but
// through cmdApplyProposal instead of cmdLand): applying a NON-conflicting
// requirement with --decision-ref must succeed AND write NO false
// "semantic conflict acknowledged" History entry.
func TestApplyProposalGate_NoFalseAckHistoryOnNonConflictingApply(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	gp := graphPathForDomain(domainDir)

	// apply-proposal a benign requirement WITH --decision-ref. The seed
	// requirement carries no opposite markers, so there is nothing to acknowledge.
	benign := writeReqProposalJSON(t, "R-apply-benign-ack-noconflict",
		"a completely benign requirement about tea quality grading scales")
	if err := cmdApplyProposal([]string{
		"--domain", domainDir, "--today", "2026-07-14",
		"--decision-ref", "no real conflict to acknowledge",
		benign,
	}); err != nil {
		t.Fatalf("non-conflicting apply-proposal with --decision-ref must succeed: %v", err)
	}

	// Read the requirement back and inspect its History slice directly: it must
	// contain NO "semantic conflict acknowledged" entry.
	g, err := loader.LoadGraph(gp)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	r, ok := ontology.RequirementByID(g, "R-apply-benign-ack-noconflict")
	if !ok {
		t.Fatal("R-apply-benign-ack-noconflict not found in graph")
	}
	for _, h := range r.History {
		if strings.Contains(h.Summary, "semantic conflict acknowledged") {
			t.Errorf("non-conflicting apply-proposal must not record a false ack entry, but History has: %q", h.Summary)
		}
	}
}

// TestApplyProposalGate_BatchModeNowBlocksConflict is the reversal of the
// pre-R10-a behavior: a semantically-contradicting requirement applied via
// `hotam apply-proposal --batch <dir>` WITHOUT ack must now be REFUSED (the
// batch-mode bypass was closed in R10-a). The graph must be left untouched.
// (Before R10-a this test asserted the opposite — that batch mode succeeded
// despite the conflict — which was the documented bypass this task removed.)
func TestApplyProposalGate_BatchModeNowBlocksConflict(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	gp := graphPathForDomain(domainDir)

	// Land the first SETTLED requirement.
	first := writeReqProposalJSON(t, "R-apply-batch-always",
		"export service must always encrypt records")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", first}); err != nil {
		t.Fatalf("land first: %v", err)
	}

	before, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph before: %v", err)
	}

	// Put the contradicting requirement in a batch dir.
	batchDir := t.TempDir()
	jsonStr := `{
		"kind": "Requirement",
		"id": "R-apply-batch-never",
		"claim": "export service must never encrypt records",
		"owner": "owner",
		"status": "SETTLED",
		"why": "batch-mode gate now blocks test"
	}`
	second := filepath.Join(batchDir, "01-never.json")
	if err := os.WriteFile(second, []byte(jsonStr), 0o644); err != nil {
		t.Fatalf("write batch proposal: %v", err)
	}

	// apply-proposal --batch on the contradicting requirement WITHOUT ack —
	// batch mode now runs the blocking half of the gate, so this must REFUSE.
	err = cmdApplyProposal([]string{
		"--domain", domainDir, "--today", "2026-07-14",
		"--batch", batchDir,
	})
	if err == nil {
		t.Fatal("expected batch mode to refuse the contradicting requirement (R10-a closed the bypass), got nil error")
	}
	if !strings.Contains(err.Error(), "R-apply-batch-always") {
		t.Errorf("error must name the conflicting anchor R-apply-batch-always:\n%s", err.Error())
	}
	if !strings.Contains(err.Error(), "semantically contradicts") {
		t.Errorf("error must state the semantic contradiction:\n%s", err.Error())
	}

	// The graph must be byte-identical (nothing from the batch landed).
	after, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("graph.json changed despite batch refusal — batch must be all-or-nothing")
	}
	if strings.Contains(string(after), "R-apply-batch-never") {
		t.Error("graph must not contain the refused batch requirement")
	}
}

// TestLandGate_BatchModeNowBlocksConflict is the `hotam land --batch <dir>`
// counterpart to TestApplyProposalGate_BatchModeNowBlocksConflict: the batch
// path goes through cmdLandBatch → proposal.ApplyBatch, which now runs the
// blocking half of the semantic-conflict gate. A contradicting batch must be
// refused and the graph left byte-identical.
func TestLandGate_BatchModeNowBlocksConflict(t *testing.T) {
	t.Parallel()
	domainDir := setupGateTestDomain(t)
	gp := graphPathForDomain(domainDir)

	// Land the first SETTLED requirement via `hotam land` (single-file).
	first := writeReqProposalJSON(t, "R-land-batch-always",
		"export service must always encrypt records")
	if err := cmdLand([]string{"--domain", domainDir, "--today", "2026-07-14", first}); err != nil {
		t.Fatalf("land first: %v", err)
	}

	before, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph before: %v", err)
	}

	// Put the contradicting requirement in a batch dir.
	batchDir := t.TempDir()
	jsonStr := `{
		"kind": "Requirement",
		"id": "R-land-batch-never",
		"claim": "export service must never encrypt records",
		"owner": "owner",
		"status": "SETTLED",
		"why": "land-batch gate now blocks test"
	}`
	conflictFile := filepath.Join(batchDir, "01-never.json")
	if err := os.WriteFile(conflictFile, []byte(jsonStr), 0o644); err != nil {
		t.Fatalf("write batch proposal: %v", err)
	}

	// land --batch on the contradicting requirement — must REFUSE.
	err = cmdLand([]string{
		"--domain", domainDir, "--today", "2026-07-14",
		"--batch", batchDir,
	})
	if err == nil {
		t.Fatal("expected land --batch to refuse the contradicting requirement, got nil error")
	}
	if !strings.Contains(err.Error(), "R-land-batch-always") {
		t.Errorf("error must name the conflicting anchor R-land-batch-always:\n%s", err.Error())
	}

	// The graph must be byte-identical (nothing from the batch landed).
	after, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("graph.json changed despite land --batch refusal — batch must be all-or-nothing")
	}
	if strings.Contains(string(after), "R-land-batch-never") {
		t.Error("graph must not contain the refused batch requirement")
	}
}
