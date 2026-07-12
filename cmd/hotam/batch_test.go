package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBatchProposal(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestCmdLand_Batch_AppliesRegeneratesAndVerifies is the e2e test for
// `hotam land --batch <dir>` (TaskList P0-1): a directory of 3 valid
// proposals must (1) exit 0, (2) leave graph.json containing all 3 new
// nodes, and (3) regenerate docs/gen exactly once so the rendered docs
// reflect the post-batch graph.
func TestCmdLand_Batch_AppliesRegeneratesAndVerifies(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	genDir := filepath.Join(domainDir, "docs", "gen")

	// docs/gen/ does not exist before land — proves any content there after
	// land was produced BY land's single gen-spec step.
	reqPath := filepath.Join(genDir, "REQUIREMENTS.md")
	if _, err := os.Stat(reqPath); !os.IsNotExist(err) {
		t.Fatalf("expected docs/gen/REQUIREMENTS.md absent before land, stat err = %v", err)
	}

	batchDir := t.TempDir()
	writeBatchProposal(t, batchDir, "01-add-r1.json", `{
		"kind": "Requirement", "id": "R-batch-e2e-1",
		"claim": "batch proposal one", "owner": "framework-author",
		"status": "DRAFT", "why": "e2e batch coverage"
	}`)
	writeBatchProposal(t, batchDir, "02-add-r2.json", `{
		"kind": "Requirement", "id": "R-batch-e2e-2",
		"claim": "batch proposal two", "owner": "framework-author",
		"status": "DRAFT", "why": "e2e batch coverage"
	}`)
	writeBatchProposal(t, batchDir, "03-add-r3.json", `{
		"kind": "Requirement", "id": "R-batch-e2e-3",
		"claim": "batch proposal three", "owner": "framework-author",
		"status": "DRAFT", "why": "e2e batch coverage"
	}`)

	err := cmdLand([]string{
		"--batch", batchDir,
		"--domain", domainDir,
		"--today", "2026-07-12",
	})
	if err != nil {
		t.Fatalf("cmdLand batch: %v", err)
	}

	// graph.json must contain all 3 new nodes.
	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	for _, want := range []string{"R-batch-e2e-1", "R-batch-e2e-2", "R-batch-e2e-3"} {
		if !strings.Contains(string(graphData), want) {
			t.Errorf("graph.json missing %q after batch land", want)
		}
	}

	// docs/gen/REQUIREMENTS.md must be freshly rendered from the post-batch
	// graph — regenerated exactly once, not once per proposal.
	after, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("read post-land REQUIREMENTS.md: %v", err)
	}
	for _, want := range []string{"R-batch-e2e-1", "R-batch-e2e-2", "R-batch-e2e-3"} {
		if !strings.Contains(string(after), want) {
			t.Errorf("docs/gen/REQUIREMENTS.md missing %q — not regenerated with the full batch", want)
		}
	}
}

// TestCmdLand_Batch_InvalidNth_AppliesNothing proves batch atomicity at the
// CLI level: when proposal N (N>1) in the directory references a nonexistent
// anchor, land must exit non-zero, leave graph.json byte-identical to its
// pre-batch state, and NOT regenerate docs/gen (nothing landed).
func TestCmdLand_Batch_InvalidNth_AppliesNothing(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	genDir := filepath.Join(domainDir, "docs", "gen")
	gp := graphPathForDomain(domainDir)

	before, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph.json before: %v", err)
	}

	batchDir := t.TempDir()
	writeBatchProposal(t, batchDir, "01-valid.json", `{
		"kind": "Requirement", "id": "R-batch-should-not-land",
		"claim": "valid first proposal", "owner": "framework-author",
		"status": "DRAFT", "why": "must be rolled back by atomicity"
	}`)
	// proposal 2 references a nonexistent requirement — mutate fails.
	writeBatchProposal(t, batchDir, "02-ghost.json", `{
		"kind": "Rejection", "requirement_id": "R-ghost-nonexistent",
		"reason": "this anchor does not exist"
	}`)

	err = cmdLand([]string{
		"--batch", batchDir,
		"--domain", domainDir,
		"--today", "2026-07-12",
	})
	if err == nil {
		t.Fatal("expected error for invalid batch proposal 2")
	}
	if !strings.Contains(err.Error(), "apply step failed") {
		t.Errorf("error = %q, want it to identify the apply step as the failure point", err.Error())
	}

	// graph.json must be byte-identical.
	after, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph.json after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("graph.json changed despite batch failure — batch must be all-or-nothing")
	}
	if strings.Contains(string(after), "R-batch-should-not-land") {
		t.Error("proposal 1 landed despite batch failure")
	}

	// docs/gen must NOT have been regenerated (the apply step failed before
	// gen-spec ran).
	if _, err := os.Stat(filepath.Join(genDir, "REQUIREMENTS.md")); err == nil {
		t.Error("docs/gen/REQUIREMENTS.md was regenerated despite apply failure — gen-spec must not run on a failed batch")
	}
}

// TestCmdLand_Batch_UnparseableNth_AppliesNothing covers the other atomicity
// failure mode: a structurally invalid JSON file. loadBatchDir parses ALL
// files before any graph I/O, so a bad-JSON file fails the batch before the
// graph is even loaded.
func TestCmdLand_Batch_UnparseableNth_AppliesNothing(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	gp := graphPathForDomain(domainDir)
	before, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph.json before: %v", err)
	}

	batchDir := t.TempDir()
	writeBatchProposal(t, batchDir, "01-valid.json", `{
		"kind": "Requirement", "id": "R-batch-parse-ok",
		"claim": "valid", "owner": "framework-author",
		"status": "DRAFT"
	}`)
	writeBatchProposal(t, batchDir, "02-broken.json", `{not valid json`)

	err = cmdLand([]string{
		"--batch", batchDir,
		"--domain", domainDir,
		"--today", "2026-07-12",
	})
	if err == nil {
		t.Fatal("expected error for unparseable JSON in batch")
	}

	after, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph.json after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("graph.json changed despite unparseable batch file")
	}
}

// TestCmdLand_Batch_EmptyDirFails — a batch dir with no *.json files is a
// caller mistake; land must report it and change nothing.
func TestCmdLand_Batch_EmptyDirFails(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	batchDir := t.TempDir()

	err := cmdLand([]string{
		"--batch", batchDir,
		"--domain", domainDir,
		"--today", "2026-07-12",
	})
	if err == nil {
		t.Fatal("expected error for empty batch dir")
	}
	if !strings.Contains(err.Error(), "no *.json") {
		t.Errorf("error = %q, want it to mention no *.json files", err.Error())
	}
}

// TestCmdApplyProposal_Batch proves the low-level apply-proposal command
// also supports --batch (applies the graph without regenerating docs).
func TestCmdApplyProposal_Batch(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	batchDir := t.TempDir()

	writeBatchProposal(t, batchDir, "01.json", `{
		"kind": "Requirement", "id": "R-apply-batch-1",
		"claim": "c1", "owner": "framework-author", "status": "DRAFT"
	}`)
	writeBatchProposal(t, batchDir, "02.json", `{
		"kind": "Requirement", "id": "R-apply-batch-2",
		"claim": "c2", "owner": "framework-author", "status": "DRAFT"
	}`)

	err := cmdApplyProposal([]string{
		"--batch", batchDir,
		"--domain", domainDir,
		"--today", "2026-07-12",
	})
	if err != nil {
		t.Fatalf("cmdApplyProposal batch: %v", err)
	}

	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	for _, want := range []string{"R-apply-batch-1", "R-apply-batch-2"} {
		if !strings.Contains(string(graphData), want) {
			t.Errorf("graph.json missing %q after apply-proposal batch", want)
		}
	}

	// apply-proposal must NOT regenerate docs.
	if _, err := os.Stat(filepath.Join(domainDir, "docs", "gen", "REQUIREMENTS.md")); err == nil {
		t.Error("docs/gen was regenerated by apply-proposal — only land regenerates docs")
	}
}

// TestCmdLand_SingleRegression confirms single-proposal mode still works
// unchanged after the --batch branch was added to cmdLand. (The existing
// TestCmdLand_AppliesRegeneratesAndVerifies already covers this; this is a
// focused regression guard for the flag-parsing path specifically.)
func TestCmdLand_SingleRegression(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement", "id": "R-single-regression",
		"claim": "single mode still works", "owner": "framework-author",
		"status": "DRAFT", "why": "regression guard"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	err := cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-12",
		proposalPath,
	})
	if err != nil {
		t.Fatalf("cmdLand single: %v", err)
	}
	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	if !strings.Contains(string(graphData), "R-single-regression") {
		t.Error("graph.json does not contain R-single-regression after single land")
	}
}
