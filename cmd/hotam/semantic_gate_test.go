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
