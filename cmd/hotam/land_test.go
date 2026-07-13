package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/proposal"
)

// TestCmdLand_AppliesRegeneratesAndVerifies is the e2e test for `hotam land`
// (TaskList P1-4): a valid proposal lands against a domain fixture and the
// command must (1) exit 0, (2) leave graph.json containing the new node,
// and (3) leave docs/gen/*.md re-rendered so they actually describe the
// post-apply graph — this is exactly the gap internal/proposal/apply.go
// leaves open on its own (it writes graph.json + graph.lock but never
// touches docs/gen), which is the bug this command exists to close.
//
// copySelfDomain (main_test.go) supplies the fixture: a real, invariant-
// clean graph.json+manifest.json pair already used throughout this package
// for exactly this purpose, so land has something non-trivial to apply
// against without hand-building a synthetic graph that would need to
// satisfy all ~47 invariants from scratch.
func TestCmdLand_AppliesRegeneratesAndVerifies(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	genDir := filepath.Join(domainDir, "docs", "gen")

	// copySelfDomain only copies graph.json + manifest.json (main_test.go);
	// docs/gen/ does not exist yet, which itself proves any content found
	// there after land ran was produced BY land's gen-spec step, not
	// inherited from the fixture.
	reqPath := filepath.Join(genDir, "REQUIREMENTS.md")
	if _, err := os.Stat(reqPath); !os.IsNotExist(err) {
		t.Fatalf("expected docs/gen/REQUIREMENTS.md absent before land, stat err = %v", err)
	}

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-e2e-smoke",
		"claim": "hotam land applies a proposal and regenerates docs in one step",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "e2e coverage for the land pipeline"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal fixture: %v", err)
	}

	err := cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-12",
		proposalPath,
	})
	if err != nil {
		t.Fatalf("cmdLand: %v", err)
	}

	// graph.json (source of truth, sibling of manifest.json — NOT
	// docs/gen/graph.json) must contain the newly applied node.
	graphData, err := os.ReadFile(graphPathForDomain(domainDir))
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	if !strings.Contains(string(graphData), "R-land-e2e-smoke") {
		t.Error("graph.json does not contain R-land-e2e-smoke after land")
	}

	// docs/gen/REQUIREMENTS.md must be freshly rendered and reflect the new
	// node — this is the specific drift TaskList P1-4 exists to close.
	after, err := os.ReadFile(reqPath)
	if err != nil {
		t.Fatalf("read post-land REQUIREMENTS.md: %v", err)
	}
	if !strings.Contains(string(after), "R-land-e2e-smoke") {
		t.Error("docs/gen/REQUIREMENTS.md was not regenerated with the new requirement after land")
	}

	// docs/gen/graph.json (the rendered copy gen-spec writes) must also be
	// current, not just the domain-root graph.json apply-proposal wrote.
	genGraphData, err := os.ReadFile(filepath.Join(genDir, "graph.json"))
	if err != nil {
		t.Fatalf("read docs/gen/graph.json: %v", err)
	}
	if !strings.Contains(string(genGraphData), "R-land-e2e-smoke") {
		t.Error("docs/gen/graph.json was not regenerated after land")
	}
}

// TestCmdLand_MissingRequiredFlags mirrors apply-proposal's own flag
// validation (land shares the same --domain/--today contract for step 1).
func TestCmdLand_MissingRequiredFlags(t *testing.T) {
	t.Parallel()

	t.Run("no proposal arg", func(t *testing.T) {
		t.Parallel()
		err := cmdLand([]string{"--domain", "/tmp/d", "--today", "2026-07-12"})
		if err == nil {
			t.Fatal("expected error when no proposal file given")
		}
	})
	t.Run("missing domain", func(t *testing.T) {
		t.Parallel()
		err := cmdLand([]string{"--today", "2026-07-12", "proposal.json"})
		if err == nil {
			t.Fatal("expected error when --domain missing")
		}
	})
	t.Run("missing today", func(t *testing.T) {
		t.Parallel()
		err := cmdLand([]string{"--domain", "/tmp/d", "proposal.json"})
		if err == nil {
			t.Fatal("expected error when --today missing")
		}
	})
}

// TestCmdLand_InvalidProposalAppliesNothing proves step (a) failing stops
// the pipeline before gen-spec or all-violations run — an unparsable
// proposal must not silently regenerate docs from an unmodified graph and
// report success.
func TestCmdLand_InvalidProposalAppliesNothing(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)

	proposalPath := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(proposalPath, []byte(`{"kind":"Bogus"}`), 0o644); err != nil {
		t.Fatalf("write proposal fixture: %v", err)
	}

	err := cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-12",
		proposalPath,
	})
	if err == nil {
		t.Fatal("expected error for unknown proposal kind")
	}
	if !strings.Contains(err.Error(), "apply step failed") {
		t.Errorf("error = %q, want it to identify the apply step as the failure point", err.Error())
	}
}

// TestCmdLand_GenSpecFailure_RollsBackGraphJSON proves the transactional
// rollback (R-land-is-transactional): when step (b) genSpec fails AFTER step
// (a) apply already wrote a new graph.json, land must restore the pre-land
// graph.json + graph.lock rather than leave a new graph next to stale docs.
//
// Failure injection: --claude-md pointed at an existing DIRECTORY makes
// genSpec's os.ReadFile(claudeMDPath) return a non-IsNotExist error
// (cross-platform: "Incorrect function." on Windows, EISDIR on Unix —
// verified empirically), so genSpec aborts right after apply succeeded.
//
// This test would FAIL if the rollback were removed: graph.json would still
// hold the new node, byte-differ from the pre-land baseline.
func TestCmdLand_GenSpecFailure_RollsBackGraphJSON(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	genDir := filepath.Join(domainDir, "docs", "gen")
	gp := graphPathForDomain(domainDir)
	lp := loader.LockPath(gp)

	// Pre-land baseline: render docs once so there is a concrete pre-land
	// state to compare the rolled-back domain against.
	if _, err := genSpec(domainDir, "", "2026-07-12"); err != nil {
		t.Fatalf("baseline genSpec: %v", err)
	}
	baselineGraph, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read baseline graph: %v", err)
	}
	baselineReqs, err := os.ReadFile(filepath.Join(genDir, "REQUIREMENTS.md"))
	if err != nil {
		t.Fatalf("read baseline REQUIREMENTS.md: %v", err)
	}

	// copySelfDomain copies graph.json + manifest but NOT graph.lock, so the
	// pre-land lock is absent — rollback must REMOVE the lock apply created,
	// not leave a stray one behind.
	if _, err := os.Stat(lp); !os.IsNotExist(err) {
		t.Fatalf("precondition: graph.lock should be absent before land, stat=%v", err)
	}

	// A valid proposal that would land cleanly on the happy path.
	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-rollback-target",
		"claim": "must NOT survive a rolled-back land",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "rollback coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	// --claude-md as a directory forces genSpec to fail after apply succeeded.
	claudeMDDir := t.TempDir()

	err = cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-13",
		"--claude-md", claudeMDDir,
		proposalPath,
	})
	if err == nil {
		t.Fatal("expected land to fail when genSpec fails, got nil")
	}
	if !strings.Contains(err.Error(), "rolled back") {
		t.Errorf("error = %q, want it to state the land was rolled back", err.Error())
	}

	// graph.json must be byte-identical to the pre-land baseline — the core
	// rollback guarantee. FAILS if rollbackLand does not restore the file.
	after, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read post-land graph: %v", err)
	}
	if string(after) != string(baselineGraph) {
		t.Fatalf("graph.json was NOT restored to pre-land bytes after rolled-back land (len before=%d after=%d)", len(baselineGraph), len(after))
	}
	if strings.Contains(string(after), "R-land-rollback-target") {
		t.Error("graph.json contains the rolled-back proposal's node — restore failed")
	}

	// graph.lock must be absent again (pre-land state was absent).
	if _, err := os.Stat(lp); !os.IsNotExist(err) {
		t.Errorf("graph.lock should be absent after rollback (pre-land state), stat=%v", err)
	}

	// Re-running gen-spec standalone (clean --claude-md) must regenerate docs
	// identical to the pre-land baseline — proving graph + docs are mutually
	// consistent with NO permanent drift. FAILS if graph.json were not
	// restored: a graph still carrying the new node would regenerate docs
	// that mention it.
	if _, err := genSpec(domainDir, "", "2026-07-12"); err != nil {
		t.Fatalf("post-land standalone genSpec: %v", err)
	}
	regenReqs, err := os.ReadFile(filepath.Join(genDir, "REQUIREMENTS.md"))
	if err != nil {
		t.Fatalf("read regenerated REQUIREMENTS.md: %v", err)
	}
	if string(regenReqs) != string(baselineReqs) {
		t.Fatal("standalone gen-spec after rollback produced docs differing from pre-land baseline — permanent drift")
	}
}

// TestRollbackLand_RestoresFilesAndRegeneratesDocs unit-tests the shared
// rollback helper directly. It exercises the path where the rollback's own
// genSpec re-render SUCCEEDS (complementing TestCmdLand_GenSpecFailure above,
// where the broken --claude-md makes the re-render fail too) and mirrors the
// step-(c) all-violations rollback trigger: apply succeeded, docs were already
// regenerated from the new graph, then rollback fires and must restore both.
//
// This would FAIL if rollbackLand skipped the genSpec re-run: the docs would
// still reflect the new graph (the direct genSpec below) instead of baseline.
func TestRollbackLand_RestoresFilesAndRegeneratesDocs(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	genDir := filepath.Join(domainDir, "docs", "gen")
	gp := graphPathForDomain(domainDir)
	lp := loader.LockPath(gp)

	// Pre-land baseline.
	if _, err := genSpec(domainDir, "", "2026-07-12"); err != nil {
		t.Fatalf("baseline genSpec: %v", err)
	}
	baselineGraph, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read baseline graph: %v", err)
	}
	baselineReqs, err := os.ReadFile(filepath.Join(genDir, "REQUIREMENTS.md"))
	if err != nil {
		t.Fatalf("read baseline REQUIREMENTS.md: %v", err)
	}

	// Snapshot exactly what cmdLand would capture before step (a): graph.json
	// present, graph.lock absent.
	snap := &graphSnapshot{
		graphBytes:   baselineGraph,
		graphPresent: true,
		lockPresent:  false,
	}

	// Simulate step (a): apply directly (writes a NEW graph.json + graph.lock).
	p, err := parseProposal([]byte(`{
		"kind": "Requirement",
		"id": "R-rollback-helper-target",
		"claim": "applied directly to set up the post-apply state",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "rollback helper coverage"
	}`))
	if err != nil {
		t.Fatalf("parseProposal: %v", err)
	}
	if err := proposal.Apply(gp, "2026-07-13", p); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := os.Stat(lp); err != nil {
		t.Fatalf("postcondition: graph.lock should exist after apply: %v", err)
	}

	// Simulate step (b) having run too: regenerate docs from the NEW graph so
	// the on-disk docs already mention the new node. This makes the genSpec
	// re-run inside rollbackLand non-vacuous.
	if _, err := genSpec(domainDir, "", "2026-07-12"); err != nil {
		t.Fatalf("post-apply genSpec: %v", err)
	}
	newReqs, _ := os.ReadFile(filepath.Join(genDir, "REQUIREMENTS.md"))
	if !strings.Contains(string(newReqs), "R-rollback-helper-target") {
		t.Fatalf("precondition: post-apply docs should mention the new node before rollback")
	}

	// Roll back to the snapshot (clean --claude-md → re-genSpec succeeds).
	if err := rollbackLand(domainDir, snap, "", "2026-07-13"); err != nil {
		t.Fatalf("rollbackLand: %v", err)
	}

	// graph.json restored to baseline bytes.
	after, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read post-rollback graph: %v", err)
	}
	if string(after) != string(baselineGraph) {
		t.Fatal("rollbackLand did not restore graph.json to pre-land bytes")
	}
	if strings.Contains(string(after), "R-rollback-helper-target") {
		t.Error("graph.json still contains the rolled-back node")
	}

	// graph.lock removed (pre-land state was absent).
	if _, err := os.Stat(lp); !os.IsNotExist(err) {
		t.Errorf("graph.lock should be absent after rollback, stat=%v", err)
	}

	// docs regenerated from the restored graph → match baseline. FAILS if
	// rollbackLand skipped the genSpec re-run (docs would still be "new").
	regenReqs, err := os.ReadFile(filepath.Join(genDir, "REQUIREMENTS.md"))
	if err != nil {
		t.Fatalf("read post-rollback REQUIREMENTS.md: %v", err)
	}
	if string(regenReqs) != string(baselineReqs) {
		t.Fatal("docs were not re-rendered to baseline by rollbackLand — genSpec re-run missing")
	}
}

// TestCmdLand_SuccessPathDoesNotRollBack is the regression guard for the
// transactional refactor: the happy path (no failures) must still land the
// new node into BOTH graph.json and the rendered docs, and must NOT trigger a
// spurious rollback (graph.lock stays in place). This would FAIL if the
// refactor accidentally rolled back on success.
func TestCmdLand_SuccessPathDoesNotRollBack(t *testing.T) {
	t.Parallel()
	domainDir := copySelfDomain(t)
	genDir := filepath.Join(domainDir, "docs", "gen")
	gp := graphPathForDomain(domainDir)
	lp := loader.LockPath(gp)

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-success-path",
		"claim": "happy path must still land without rolling back",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "regression guard for the transactional refactor"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	if err := cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-13",
		proposalPath,
	}); err != nil {
		t.Fatalf("cmdLand success path: %v", err)
	}

	graphData, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read graph.json: %v", err)
	}
	if !strings.Contains(string(graphData), "R-land-success-path") {
		t.Error("graph.json missing the new node — success path was wrongly rolled back")
	}
	reqs, err := os.ReadFile(filepath.Join(genDir, "REQUIREMENTS.md"))
	if err != nil {
		t.Fatalf("read REQUIREMENTS.md: %v", err)
	}
	if !strings.Contains(string(reqs), "R-land-success-path") {
		t.Error("docs not regenerated with the new node on the success path")
	}
	// graph.lock must remain — apply wrote it, and the success path must not
	// remove it (only a rollback removes a lock, and only when it was absent
	// pre-land).
	if _, err := os.Stat(lp); err != nil {
		t.Errorf("graph.lock should exist after a successful land, stat=%v", err)
	}
}
