package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// pickRealCandidateAssumption loads the domain graph at graphPath and returns
// a shared assumption and an existing non-REJECTED requirement that names it.
// Adding the candidate creates a specific latent pair under the same reference
// count threshold used by the structural confront logic.
func pickRealCandidateAssumption(t *testing.T, graphPath string) (string, string) {
	t.Helper()
	g, err := loader.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadGraph(%s): %v", graphPath, err)
	}
	rc := ontology.AssumptionReferenceCounts(g)
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusREJECTED {
			continue
		}
		for _, aID := range r.Assumptions {
			if rc[aID] > 0 && rc[aID]+1 < ontology.GenericAssumptionThreshold {
				return aID, r.ID
			}
		}
	}
	t.Skip("real domain has no assumption a candidate could pair on; cannot exercise the structural confront path")
	return "", ""
}

// reqProposalWithAssumptions builds a minimal valid Requirement proposal JSON
// whose Assumptions field names the given assumption id — designed to trigger
// the structural shared-assumption-cluster check. The claim is deliberately
// unique nonsense so the lexical classification is clear and the structural
// JSON hit is the sole positive signal.
func reqProposalWithAssumptions(id, assumptionID string) string {
	return `{
		"kind": "Requirement", "id": "` + id + `",
		"claim": "zzz qxp wumbo totally novel structural probe candidate",
		"owner": "framework-author", "status": "DRAFT", "why": "structural confront coverage",
		"assumptions": ["` + assumptionID + `"]
	}`
}

// candidateSentinelLeakToken is the raw internal sentinel string from
// internal/diagnose.structural_confront.go that must never reach user-facing
// output. Hardcoded here (not imported — it is unexported) so the test can
// assert the leak guard independently.
const candidateSentinelLeakToken = "__CANDIDATE__"

// TestCmdConfront_Proposal_JSONShape verifies the JSON contract carries the
// actual lexical and structural classifications for a proposal.
func TestCmdConfront_Proposal_JSONShape(t *testing.T) {
	if testing.Short() {
		t.Skip("confront proposal e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	assumptionID, existingRequirementID := pickRealCandidateAssumption(t, filepath.Join(domainDir, "graph.json"))

	proposalPath := filepath.Join(t.TempDir(), "req.json")
	writeBatchProposal(t, filepath.Dir(proposalPath), filepath.Base(proposalPath),
		reqProposalWithAssumptions("R-structural-json", assumptionID))

	cmd := exec.Command(binPath, "confront", "--proposal", proposalPath,
		"--domain", domainDir, "--json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hotam confront --proposal --json failed: %v\n%s", err, out)
	}

	var env confrontProposalEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("parse confront --proposal JSON: %v\nraw:\n%s", err, out)
	}

	// This candidate has no lexical hits; the proposal's structural relationship
	// is the independent classification that should produce a hit.
	if !env.Lexical.Clear || len(env.Lexical.Settled) != 0 || len(env.Lexical.Rejected) != 0 {
		t.Errorf("lexical classification = %+v, want clear with no hits", env.Lexical)
	}
	if env.Structural.Clear {
		t.Errorf("structural classification is clear for a candidate naming %q", assumptionID)
	}
	if len(env.Structural.SharedAssumptionHits) == 0 {
		t.Fatal("structural shared_assumption_hits is empty")
	}

	foundMatchingHit := false
	for _, hit := range env.Structural.SharedAssumptionHits {
		foundCandidate, foundExistingRequirement := false, false
		for _, member := range hit.Members {
			foundCandidate = foundCandidate || member == "(candidate)"
			foundExistingRequirement = foundExistingRequirement || member == existingRequirementID
		}
		if foundCandidate && foundExistingRequirement {
			foundMatchingHit = true
			break
		}
	}
	if !foundMatchingHit {
		t.Errorf("structural hits do not include candidate paired with %q via %q: %+v",
			existingRequirementID, assumptionID, env.Structural.SharedAssumptionHits)
	}
	// A Requirement candidate has no Axis — axis hits must be empty.
	if len(env.Structural.AxisCoReferenceHits) != 0 {
		t.Errorf("Requirement candidate axis_co_reference_hits must be empty, got %d hits", len(env.Structural.AxisCoReferenceHits))
	}
	if strings.Contains(string(out), candidateSentinelLeakToken) {
		t.Errorf("JSON output leaked the raw sentinel %q", candidateSentinelLeakToken)
	}
}

// TestCmdConfront_Proposal_NoAssumptionsIsStructurallyClear verifies that a
// Requirement proposal with no assumptions has a clear structural JSON
// classification and no structural hits.
func TestCmdConfront_Proposal_NoAssumptionsIsStructurallyClear(t *testing.T) {
	if testing.Short() {
		t.Skip("confront proposal e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	proposalPath := filepath.Join(t.TempDir(), "req.json")
	writeBatchProposal(t, filepath.Dir(proposalPath), filepath.Base(proposalPath),
		reqProposalJSON("R-no-assumptions-probe", "zzz qxp wumbo totally novel no assumptions"))

	cmd := exec.Command(binPath, "confront", "--proposal", proposalPath,
		"--domain", domainDir, "--json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hotam confront --proposal --json failed: %v\n%s", err, out)
	}
	var env confrontProposalEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("parse confront --proposal JSON: %v\nraw:\n%s", err, out)
	}
	if !env.Structural.Clear || len(env.Structural.SharedAssumptionHits) != 0 ||
		len(env.Structural.AxisCoReferenceHits) != 0 {
		t.Errorf("no-assumptions structural classification = %+v, want clear with no hits", env.Structural)
	}
}

// TestCmdConfront_Proposal_MutualExclusivityRejectsMix proves --proposal is
// mutually exclusive with the positional text mode: mixing them is a usage
// error (non-zero exit), matching readConfrontCandidate's own "pass either X
// OR Y, not both" discipline.
func TestCmdConfront_Proposal_MutualExclusivityRejectsMix(t *testing.T) {
	if testing.Short() {
		t.Skip("confront proposal e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)

	proposalPath := filepath.Join(t.TempDir(), "req.json")
	writeBatchProposal(t, filepath.Dir(proposalPath), filepath.Base(proposalPath),
		reqProposalJSON("R-mix-test", "whatever"))

	cmd := exec.Command(binPath, "confront", "--proposal", proposalPath,
		"--domain", domainDir, "extra positional text")
	out, _ := cmd.CombinedOutput()
	// Must be a non-zero exit (usage error).
	if cmd.ProcessState.Success() {
		t.Errorf("expected non-zero exit for --proposal + positional mix, got success:\n%s", out)
	}
	if !strings.Contains(string(out), "not a mix") {
		t.Errorf("error output missing the mutual-exclusivity message:\n%s", out)
	}
}
