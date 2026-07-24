package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCmdApplyProposal_SelfHosting_RequirementRefused proves task #350's
// (RAC-B3) self-hosting lock is reachable through `hotam apply-proposal`
// (single-file path): landing a ProposedRequirement against the REAL
// hotam-spec-self fixture (copySelfDomain, self_hosting: true, unmodified)
// must be refused, name `hotam sync-self`, and leave graph.json untouched.
func TestCmdApplyProposal_SelfHosting_RequirementRefused(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	gp := graphPathForDomain(domainDir)
	before, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph.json before: %v", err)
	}

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-attempted-hand-authored-self-hosting-edit",
		"claim": "an operator tries to hand-author a requirement in the self-hosting domain",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "task #350/RAC-B3 CLI-level coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal fixture: %v", err)
	}

	err = cmdApplyProposal([]string{
		"--domain", domainDir,
		"--today", "2026-07-14",
		proposalPath,
	})
	if err == nil {
		t.Fatal("expected apply-proposal to refuse a Requirement on the self-hosting domain, got nil error")
	}
	if !strings.Contains(err.Error(), "hotam sync-self") {
		t.Errorf("error = %q, want it to name `hotam sync-self`", err.Error())
	}
	if !strings.Contains(err.Error(), "self-hosting") {
		t.Errorf("error = %q, want it to explain the self-hosting lock", err.Error())
	}

	after, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph.json after: %v", err)
	}
	if string(before) != string(after) {
		t.Error("graph.json was mutated despite the self-hosting lock refusing the apply")
	}
}

// TestCmdLand_SelfHosting_RejectionRefused is the `hotam land` counterpart,
// covering ProposedRejection specifically: landing a Rejection against the
// real self-hosting fixture must be refused, and the refusal message must
// mention the 'replaces' relation reminder (see
// errSelfHostingRejectionLocked in internal/proposal/apply.go).
func TestCmdLand_SelfHosting_RejectionRefused(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	gp := graphPathForDomain(domainDir)
	before, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph.json before: %v", err)
	}

	// R-anchor-everything is a real, long-SETTLED requirement in the self
	// domain fixture — any real ID works here since the lock fires before
	// the ID is even looked up in the graph.
	proposalPath := filepath.Join(t.TempDir(), "rejection.json")
	proposalJSON := `{
		"kind": "Rejection",
		"requirement_id": "R-anchor-everything",
		"reason": "an operator tries to hand-author a rejection in the self-hosting domain",
		"replaced_by": ["R-conflict-is-connector-node"]
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal fixture: %v", err)
	}

	err = cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-14",
		proposalPath,
	})
	if err == nil {
		t.Fatal("expected land to refuse a Rejection on the self-hosting domain, got nil error")
	}
	if !strings.Contains(err.Error(), "hotam sync-self") {
		t.Errorf("error = %q, want it to name `hotam sync-self`", err.Error())
	}
	if !strings.Contains(err.Error(), "replaces") {
		t.Errorf("error = %q, want it to mention the 'replaces' relation reminder", err.Error())
	}

	after, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph.json after: %v", err)
	}
	if string(before) != string(after) {
		t.Error("graph.json was mutated despite the self-hosting lock refusing the land")
	}
}

// TestCmdLand_NonSelfHosting_RequirementStillLands is the negative control
// at the CLI level: a copy of the SAME fixture with self_hosting forced to
// false (copyNonSelfHostingDomain) must keep landing Requirement proposals
// exactly as before task #350/RAC-B3 — proving the lock is genuinely gated
// on the manifest flag, not on some other property of the fixture (its
// size, its real anchors, etc).
func TestCmdLand_NonSelfHosting_RequirementStillLands(t *testing.T) {
	t.Parallel()
	domainDir := copyNonSelfHostingDomain(t)

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-non-self-hosting-lands-fine",
		"claim": "a non-self-hosting domain keeps accepting hand-authored requirements",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "task #350/RAC-B3 negative-control CLI coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal fixture: %v", err)
	}

	if err := cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-14",
		proposalPath,
	}); err != nil {
		t.Fatalf("cmdLand on a non-self-hosting domain should succeed, got: %v", err)
	}

	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	if !strings.Contains(string(graphData), "R-non-self-hosting-lands-fine") {
		t.Error("graph.json does not contain the landed requirement on the non-self-hosting domain")
	}
}
