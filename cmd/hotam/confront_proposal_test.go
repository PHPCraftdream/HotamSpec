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
// an assumption id that a candidate WOULD form a latent pair on if it named
// that assumption: the assumption must be referenced by at least one
// non-REJECTED requirement, AND its reference count + 1 (the candidate's
// addition) must stay under GenericAssumptionThreshold so latentPairRecords
// still treats it as "specific" and the pair (candidate, existing-requirement)
// fires. This mirrors the structural confront's own detection exactly — the
// test cannot rot when a cluster dissolves or a reference count drifts, and
// skips cleanly when no suitable assumption exists.
func pickRealCandidateAssumption(t *testing.T, graphPath string) string {
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
				return aID
			}
		}
	}
	t.Skip("real domain has no assumption a candidate could pair on; cannot exercise the structural confront path")
	return ""
}

// reqProposalWithAssumptions builds a minimal valid Requirement proposal JSON
// whose Assumptions field names the given assumption id — designed to trigger
// the structural shared-assumption-cluster check. The claim is deliberately
// unique nonsense so the LEXICAL check stays clear and the STRUCTURAL section
// is the sole signal (making the assertions unambiguous).
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

// TestCmdConfront_Proposal_StructuralFiresSharedAssumption is the positive
// end-to-end proof: `hotam confront --proposal <file>` against a real domain
// copy, where the proposal names an assumption already shared by 2+ existing
// requirements (found via ontology.LatentConnectorClusters), must surface BOTH
// the lexical CONFRONT report AND the structural shared-assumption section —
// and must never leak the raw __CANDIDATE__ sentinel into the output.
func TestCmdConfront_Proposal_StructuralFiresSharedAssumption(t *testing.T) {
	if testing.Short() {
		t.Skip("confront proposal e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	assumptionID := pickRealCandidateAssumption(t, filepath.Join(domainDir, "graph.json"))

	proposalPath := filepath.Join(t.TempDir(), "req.json")
	writeBatchProposal(t, filepath.Dir(proposalPath), filepath.Base(proposalPath),
		reqProposalWithAssumptions("R-structural-probe", assumptionID))

	out, err := exec.Command(binPath, "confront", "--proposal", proposalPath,
		"--domain", domainDir).CombinedOutput()
	if err != nil {
		t.Fatalf("hotam confront --proposal failed: %v\n%s", err, out)
	}
	text := string(out)

	// The lexical section must be present.
	if !strings.Contains(text, "CONFRONT check") {
		t.Errorf("output missing the lexical CONFRONT report header:\n%s", text)
	}
	// The structural section must be present with a shared-assumption hit.
	if !strings.Contains(text, "structural confront") {
		t.Errorf("output missing the structural confront header:\n%s", text)
	}
	if !strings.Contains(text, "shared-assumption cluster") {
		t.Errorf("output missing the shared-assumption cluster hit:\n%s", text)
	}
	if !strings.Contains(text, "(candidate)") {
		t.Errorf("output missing the (candidate) label in structural members:\n%s", text)
	}
	// The raw sentinel must NEVER appear in user-facing output.
	if strings.Contains(text, candidateSentinelLeakToken) {
		t.Errorf("output leaked the raw sentinel %q:\n%s", candidateSentinelLeakToken, text)
	}
}

// TestCmdConfront_Proposal_JSONShape verifies the --json contract: the output
// is a confrontProposalEnvelope with a "lexical" ConfrontResult and a
// "structural" StructuralConfrontResult whose shared_assumption_hits is
// non-empty and whose axis_co_reference_hits is an empty array (not null) for
// a Requirement candidate (no Axis field).
func TestCmdConfront_Proposal_JSONShape(t *testing.T) {
	if testing.Short() {
		t.Skip("confront proposal e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	assumptionID := pickRealCandidateAssumption(t, filepath.Join(domainDir, "graph.json"))

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

	// Lexical side: the unique nonsense claim should be clear (no overlap).
	if !env.Lexical.Clear {
		t.Errorf("lexical Clear=false for a unique-nonsense claim; settled hits: %v", env.Lexical.Settled)
	}

	// Structural side: shared-assumption hits must be non-empty.
	if env.Structural.Clear {
		t.Errorf("structural Clear=true; expected shared-assumption hits for a candidate naming %q", assumptionID)
	}
	if len(env.Structural.SharedAssumptionHits) == 0 {
		t.Errorf("structural shared_assumption_hits is empty; expected at least one hit")
	}
	// A Requirement candidate has no Axis — axis hits must be an empty array.
	if len(env.Structural.AxisCoReferenceHits) != 0 {
		t.Errorf("Requirement candidate axis_co_reference_hits must be empty, got %d hits", len(env.Structural.AxisCoReferenceHits))
	}

	// No sentinel leak in the JSON either.
	raw := string(out)
	if strings.Contains(raw, candidateSentinelLeakToken) {
		t.Errorf("JSON output leaked the raw sentinel %q", candidateSentinelLeakToken)
	}
}

// TestCmdConfront_Proposal_NoAssumptionsIsStructurallyClear proves a
// Requirement proposal with NO assumptions produces a structural section that
// is explicitly clear (the "no structural tension detected" verdict renders,
// not silence), with empty hit slices.
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

	out, err := exec.Command(binPath, "confront", "--proposal", proposalPath,
		"--domain", domainDir).CombinedOutput()
	if err != nil {
		t.Fatalf("hotam confront --proposal failed: %v\n%s", err, out)
	}
	text := string(out)

	if !strings.Contains(text, "no structural tension detected") {
		t.Errorf("output missing the explicit 'no structural tension' verdict for a no-assumptions proposal:\n%s", text)
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
