package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// confrontOverlapClaim is a candidate whose distinctive tokens
// (check_typed_anchors / validate / ent / prefix / entityinstance) strongly
// overlap R-entity-typed-anchors in the real graph — verified to score 6 with
// 6 shared tokens (well above the MinLexicalOverlapTokens bar). Used to drive
// the FLAGGED confront-at-gate path deterministically.
const confrontOverlapClaim = "check_typed_anchors shall validate the ENT- prefix for EntityInstance nodes"

// confrontClearClaim uses nonsense tokens absent from any graph claim, so it
// is deterministically CLEAR (zero shared distinctive tokens).
const confrontClearClaim = "zzqx wumbo frobnicator splines reticulate totally novel"

// reqProposalJSON builds a minimal valid Requirement proposal JSON for the
// confront-at-gate subprocess tests.
func reqProposalJSON(id, claim string) string {
	return `{
		"kind": "Requirement", "id": "` + id + `",
		"claim": "` + claim + `",
		"owner": "framework-author", "status": "DRAFT", "why": "confront-at-gate coverage"
	}`
}

// TestCmdApplyProposal_ConfrontAtGate_RunsAndNeverBlocks proves the confront-
// at-gate check runs (its report header is in stdout) for a DIRECT
// `hotam apply-proposal <file>` invocation AND that the apply STILL SUCCEEDS
// even when the claim overlaps a SETTLED requirement (confront is advisory,
// never a gate — R-ai-presents-not-decides).
func TestCmdApplyProposal_ConfrontAtGate_RunsAndNeverBlocks(t *testing.T) {
	if testing.Short() {
		t.Skip("confront-gate e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	proposalPath := filepath.Join(t.TempDir(), "p.json")
	writeBatchProposal(t, filepath.Dir(proposalPath), filepath.Base(proposalPath),
		reqProposalJSON("R-apply-confront-gate", confrontOverlapClaim))

	out, err := exec.Command(binPath, "apply-proposal", proposalPath,
		"--domain", domainDir, "--today", "2026-07-13").CombinedOutput()
	if err != nil {
		t.Fatalf("apply-proposal must succeed (confront never blocks), got: %v\n%s", err, out)
	}
	// The confront-at-gate report must be visible in the apply output.
	if !strings.Contains(string(out), "CONFRONT check") {
		t.Errorf("output missing the confront report header (confront-at-gate did not run):\n%s", out)
	}
	// The overlap is real (score 6) — a DUPLICATE warning must have rendered.
	if !strings.Contains(string(out), "possible DUPLICATE") {
		t.Errorf("output missing the DUPLICATE warning for an overlapping claim:\n%s", out)
	}
	// ...and the apply must still have proceeded regardless.
	if !strings.Contains(string(out), "applied Requirement R-apply-confront-gate") {
		t.Errorf("output missing the apply confirmation (confront blocked the apply):\n%s", out)
	}
}

// TestCmdLand_ConfrontAtGate_RunsAndNeverBlocks is the land single-file
// counterpart: `hotam land <file>` (DIRECT invocation) prints the confront
// report and still lands (0 violations) even when the claim overlaps.
func TestCmdLand_ConfrontAtGate_RunsAndNeverBlocks(t *testing.T) {
	if testing.Short() {
		t.Skip("confront-gate e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	proposalPath := filepath.Join(t.TempDir(), "p.json")
	writeBatchProposal(t, filepath.Dir(proposalPath), filepath.Base(proposalPath),
		reqProposalJSON("R-land-confront-gate", confrontOverlapClaim))

	out, err := exec.Command(binPath, "land", proposalPath,
		"--domain", domainDir, "--today", "2026-07-13").CombinedOutput()
	if err != nil {
		t.Fatalf("land must succeed (confront never blocks), got: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "CONFRONT check") {
		t.Errorf("output missing the confront report header (confront-at-gate did not run):\n%s", out)
	}
	if !strings.Contains(string(out), "possible DUPLICATE") {
		t.Errorf("output missing the DUPLICATE warning for an overlapping claim:\n%s", out)
	}
	if !strings.Contains(string(out), "landed: graph applied, docs regenerated, 0 violations") {
		t.Errorf("output missing the land confirmation (confront blocked the land):\n%s", out)
	}
}

// TestCmdPropose_Land_PrintsExactlyOneConfrontReport is the NON-REGRESSION
// guard for the double-confront bug this task's design explicitly avoids.
// `hotam propose --land` runs its OWN confront inside runPropose (propose.go,
// task #124) BEFORE calling landProposalFile -> landProposalValue — neither of
// which runs a confront. So exactly ONE confront report must print, not two.
// This asserts the count directly rather than trusting the design reasoning.
func TestCmdPropose_Land_PrintsExactlyOneConfrontReport(t *testing.T) {
	if testing.Short() {
		t.Skip("confront-gate e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	outPath := filepath.Join(t.TempDir(), "draft.json")

	out, err := exec.Command(binPath, "propose", "requirement",
		"--id", "R-propose-land-once",
		"--claim", confrontOverlapClaim,
		"--owner", "framework-author", "--status", "DRAFT",
		"--why", "double-confront non-regression",
		"--domain", domainDir, "--out", outPath,
		"--today", "2026-07-13", "--land").CombinedOutput()
	if err != nil {
		t.Fatalf("propose --land failed: %v\n%s", err, out)
	}
	// formatConfrontReport emits exactly one "CONFRONT check" header per
	// report; count occurrences to detect a double-print.
	n := strings.Count(string(out), "CONFRONT check")
	if n != 1 {
		t.Errorf("propose --land printed %d confront reports, want exactly 1 (double-confront regression):\n%s", n, out)
	}
	if !strings.Contains(string(out), "landed: graph applied, docs regenerated, 0 violations") {
		t.Errorf("output missing the land confirmation:\n%s", out)
	}
}

// TestCmdLand_Batch_ConfrontSummary_AllClear proves the batch summary renders
// the single "N/N clear" line (not per-item verbosity) when every proposal is
// clear, and that the batch still applies.
func TestCmdLand_Batch_ConfrontSummary_AllClear(t *testing.T) {
	if testing.Short() {
		t.Skip("confront-gate e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	batchDir := t.TempDir()
	writeBatchProposal(t, batchDir, "01.json", reqProposalJSON("R-batch-clear-1", "zzqx wumbo frobnicator one"))
	writeBatchProposal(t, batchDir, "02.json", reqProposalJSON("R-batch-clear-2", "splines reticulate novel two"))
	writeBatchProposal(t, batchDir, "03.json", reqProposalJSON("R-batch-clear-3", "wumbo frobnicator three novel"))

	out, err := exec.Command(binPath, "land", "--batch", batchDir,
		"--domain", domainDir, "--today", "2026-07-13").CombinedOutput()
	if err != nil {
		t.Fatalf("land --batch (all clear) must succeed, got: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "confront batch: 3/3 proposals clear") {
		t.Errorf("output missing the all-clear batch summary line:\n%s", out)
	}
	if !strings.Contains(string(out), "no overlap detected") {
		t.Errorf("output missing the 'no overlap detected' verdict:\n%s", out)
	}
	// A clear batch must NOT render per-item digests.
	if strings.Contains(string(out), "flagged for possible overlap") {
		t.Errorf("all-clear batch unexpectedly rendered a flagged digest:\n%s", out)
	}
	if !strings.Contains(string(out), "applied batch of 3 proposals") {
		t.Errorf("output missing the batch apply confirmation:\n%s", out)
	}
}

// TestCmdLand_Batch_ConfrontSummary_FlaggedMix proves the batch summary renders
// the SHORT digest (count + one line per flagged proposal) for a MIX of clear
// and flagged proposals, not N full confront reports. The flagged proposal's
// claim strongly overlaps R-entity-typed-anchors (score 6); the clear one uses
// nonsense tokens. Both still apply (advisory, never blocks).
func TestCmdLand_Batch_ConfrontSummary_FlaggedMix(t *testing.T) {
	if testing.Short() {
		t.Skip("confront-gate e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	batchDir := t.TempDir()
	writeBatchProposal(t, batchDir, "01-clear.json", reqProposalJSON("R-batch-mix-clear", confrontClearClaim))
	writeBatchProposal(t, batchDir, "02-flagged.json", reqProposalJSON("R-batch-mix-flagged", confrontOverlapClaim))

	out, err := exec.Command(binPath, "land", "--batch", batchDir,
		"--domain", domainDir, "--today", "2026-07-13").CombinedOutput()
	if err != nil {
		t.Fatalf("land --batch (mixed) must succeed (confront never blocks), got: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "confront batch: 1/2 proposals flagged for possible overlap") {
		t.Errorf("output missing the flagged-mix summary line:\n%s", out)
	}
	// Only the flagged proposal's anchor appears in the digest.
	if !strings.Contains(string(out), "R-batch-mix-flagged") {
		t.Errorf("output missing the flagged proposal's anchor in the digest:\n%s", out)
	}
	// The clear proposal must NOT appear as a digest line (only flagged ones do).
	if strings.Contains(string(out), "R-batch-mix-clear") {
		t.Errorf("clear proposal anchor appeared in the digest (only flagged ones should):\n%s", out)
	}
	// The top-hit id must be surfaced (one-line digest carries it).
	if !strings.Contains(string(out), "R-entity-typed-anchors") {
		t.Errorf("output missing the top-hit id in the flagged digest:\n%s", out)
	}
	// The summarized digest must NOT render a full per-hit confront report
	// (no "CONFRONT check" header — that is the single-file report shape).
	if strings.Contains(string(out), "CONFRONT check") {
		t.Errorf("batch mode rendered a full confront report instead of the short digest:\n%s", out)
	}
	if !strings.Contains(string(out), "applied batch of 2 proposals") {
		t.Errorf("output missing the batch apply confirmation:\n%s", out)
	}
}

// TestCmdApplyProposal_Batch_ConfrontSummary runs the SAME summary check
// against the apply-proposal --batch path (not just land --batch), proving the
// shared confrontBatchSummary helper is wired into both batch commands.
func TestCmdApplyProposal_Batch_ConfrontSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("confront-gate e2e: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	domainDir := copySelfDomain(t)
	batchDir := t.TempDir()
	writeBatchProposal(t, batchDir, "01-clear.json", reqProposalJSON("R-apply-batch-clear", confrontClearClaim))
	writeBatchProposal(t, batchDir, "02-flagged.json", reqProposalJSON("R-apply-batch-flagged", confrontOverlapClaim))

	out, err := exec.Command(binPath, "apply-proposal", "--batch", batchDir,
		"--domain", domainDir, "--today", "2026-07-13").CombinedOutput()
	if err != nil {
		t.Fatalf("apply-proposal --batch must succeed (confront never blocks), got: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "confront batch: 1/2 proposals flagged for possible overlap") {
		t.Errorf("output missing the flagged-mix summary line:\n%s", out)
	}
	if !strings.Contains(string(out), "applied batch of 2 proposals") {
		t.Errorf("output missing the batch apply confirmation:\n%s", out)
	}
}
