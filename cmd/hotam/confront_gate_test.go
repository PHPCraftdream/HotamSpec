package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/diagnose"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// reqProposalJSON builds a minimal valid Requirement proposal JSON for the
// confront-at-gate subprocess tests.
func reqProposalJSON(id, claim string) string {
	proposal := struct {
		Kind   string `json:"kind"`
		ID     string `json:"id"`
		Claim  string `json:"claim"`
		Owner  string `json:"owner"`
		Status string `json:"status"`
		Why    string `json:"why"`
	}{
		Kind: "Requirement", ID: id, Claim: claim,
		Owner: "framework-author", Status: "DRAFT", Why: "confront-at-gate coverage",
	}
	encoded, err := json.Marshal(proposal)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// TestCmdApplyProposal_LexicalDuplicatePersists verifies that a lexical
// self-match does not prevent apply-proposal from persisting the proposal.
func TestCmdApplyProposal_LexicalDuplicatePersists(t *testing.T) {
	if testing.Short() {
		t.Skip("confront-gate e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copyNonSelfHostingDomain(t)
	claim, _ := pickRealSettledClaim(t, graphPathForDomain(domainDir))
	proposalPath := filepath.Join(t.TempDir(), "p.json")
	writeBatchProposal(t, filepath.Dir(proposalPath), filepath.Base(proposalPath),
		reqProposalJSON("R-apply-confront-gate", claim))

	out, err := exec.Command(binPath, "apply-proposal", proposalPath,
		"--domain", domainDir, "--today", "2026-07-13").CombinedOutput()
	if err != nil {
		t.Fatalf("apply-proposal must persist a lexically matching proposal, got: %v\n%s", err, out)
	}
	assertDraftRequirementPersisted(t, domainDir, "R-apply-confront-gate")
}

// TestCmdLand_LexicalDuplicatePersists verifies that a lexical self-match
// does not prevent land from persisting the proposal.
func TestCmdLand_LexicalDuplicatePersists(t *testing.T) {
	if testing.Short() {
		t.Skip("confront-gate e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copyNonSelfHostingDomain(t)
	claim, _ := pickRealSettledClaim(t, graphPathForDomain(domainDir))
	proposalPath := filepath.Join(t.TempDir(), "p.json")
	writeBatchProposal(t, filepath.Dir(proposalPath), filepath.Base(proposalPath),
		reqProposalJSON("R-land-confront-gate", claim))

	out, err := exec.Command(binPath, "land", proposalPath,
		"--domain", domainDir, "--today", "2026-07-13").CombinedOutput()
	if err != nil {
		t.Fatalf("land must persist a lexically matching proposal, got: %v\n%s", err, out)
	}
	assertDraftRequirementPersisted(t, domainDir, "R-land-confront-gate")
}

// assertDraftRequirementPersisted checks the actual graph state after a CLI
// operation rather than relying on its confirmation text.
func assertDraftRequirementPersisted(t *testing.T, domainDir, id string) {
	t.Helper()
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("load persisted graph: %v", err)
	}
	for _, r := range g.Requirements {
		if r.ID != id {
			continue
		}
		if r.Status != ontology.StatusDRAFT {
			t.Fatalf("%s status = %q, want %q", id, r.Status, ontology.StatusDRAFT)
		}
		return
	}
	t.Fatalf("persisted graph does not contain requirement %s", id)
}

// pickClearAndMatchedClaims proves the batch inputs exercise both lexical
// outcomes against the graph before either proposal is applied.
func pickClearAndMatchedClaims(t *testing.T, domainDir string) (clearClaim, matchedClaim string) {
	t.Helper()
	g, err := loadDomainGraph(domainDir)
	if err != nil {
		t.Fatalf("load starting graph: %v", err)
	}
	clearClaim = "zzqx wumbo frobnicator splines reticulate totally novel"
	if result := diagnose.Confront(g, clearClaim); !result.Clear {
		t.Fatalf("clear batch candidate unexpectedly has lexical hits: %+v", result)
	}
	matchedClaim, _ = pickRealSettledClaim(t, graphPathForDomain(domainDir))
	return clearClaim, matchedClaim
}

// TestCmdLand_Batch_LexicalDuplicatePersists checks that a mixed clear/matched
// batch lands both proposals.
func TestCmdLand_Batch_LexicalDuplicatePersists(t *testing.T) {
	if testing.Short() {
		t.Skip("confront-gate e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copyNonSelfHostingDomain(t)
	clearClaim, matchedClaim := pickClearAndMatchedClaims(t, domainDir)
	batchDir := t.TempDir()
	writeBatchProposal(t, batchDir, "01-clear.json",
		reqProposalJSON("R-land-batch-clear", clearClaim))
	writeBatchProposal(t, batchDir, "02-matched.json",
		reqProposalJSON("R-land-batch-matched", matchedClaim))

	out, err := exec.Command(binPath, "land", "--batch", batchDir,
		"--domain", domainDir, "--today", "2026-07-13").CombinedOutput()
	if err != nil {
		t.Fatalf("land --batch must persist clear and lexically matched proposals, got: %v\n%s", err, out)
	}
	assertDraftRequirementPersisted(t, domainDir, "R-land-batch-clear")
	assertDraftRequirementPersisted(t, domainDir, "R-land-batch-matched")
}

// TestCmdApplyProposal_Batch_LexicalDuplicatePersists checks the same batch
// behavior through apply-proposal's entry point.
func TestCmdApplyProposal_Batch_LexicalDuplicatePersists(t *testing.T) {
	if testing.Short() {
		t.Skip("confront-gate e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copyNonSelfHostingDomain(t)
	clearClaim, matchedClaim := pickClearAndMatchedClaims(t, domainDir)
	batchDir := t.TempDir()
	writeBatchProposal(t, batchDir, "01-clear.json",
		reqProposalJSON("R-apply-batch-clear", clearClaim))
	writeBatchProposal(t, batchDir, "02-matched.json",
		reqProposalJSON("R-apply-batch-matched", matchedClaim))

	out, err := exec.Command(binPath, "apply-proposal", "--batch", batchDir,
		"--domain", domainDir, "--today", "2026-07-13").CombinedOutput()
	if err != nil {
		t.Fatalf("apply-proposal --batch must persist clear and lexically matched proposals, got: %v\n%s", err, out)
	}
	assertDraftRequirementPersisted(t, domainDir, "R-apply-batch-clear")
	assertDraftRequirementPersisted(t, domainDir, "R-apply-batch-matched")
}
