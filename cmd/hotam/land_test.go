package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
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
	if _, _, err := genSpec(domainDir, "", "2026-07-12", "", false); err != nil {
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

	// A valid proposal that would land cleanly on the happy path. The claim
	// text deliberately avoids the MUST/MUST-NOT/NEVER/ALWAYS/ONLY reserved
	// tokens (R-...'s TRANSLATE-step embedding convention) so this fixture's
	// throwaway prose never collides with the semantic opposite-marker gate
	// against real SETTLED requirements that happen to use those tokens —
	// this test exercises rollback plumbing, not the semantic gate.
	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-rollback-target",
		"claim": "should disappear after a rolled-back land",
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
	if _, _, err := genSpec(domainDir, "", "2026-07-12", "", false); err != nil {
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
	if _, _, err := genSpec(domainDir, "", "2026-07-12", "", false); err != nil {
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
	if _, _, err := genSpec(domainDir, "", "2026-07-12", "", false); err != nil {
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

// crystalDebtLine extracts the LIVE-STATE debt line (the one carrying "SETTLED
// ENFORCED") from a rendered crystal or live-state.md body. Returns "" when no
// such line is present. Used to prove a regenerated crystal reflects the
// post-apply graph (its debt line differs from the pre-apply baseline).
func crystalDebtLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "SETTLED ENFORCED") {
			return line
		}
	}
	return ""
}

// TestResolveClaudeMDPath unit-tests the helper directly across every branch:
// explicit override, CLAUDE.md-present, marker-present (no CLAUDE.md), and the
// no-convention negative. Each case uses a domains-parented domainDir so
// repoRootForDomain tier-1 resolves to a test-controlled root (no CWD leak).
func TestResolveClaudeMDPath(t *testing.T) {
	t.Parallel()

	t.Run("explicit_wins", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		domainDir := filepath.Join(root, "domains", "d")
		if err := os.MkdirAll(domainDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Even with a CLAUDE.md at root, explicit must win.
		if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := resolveClaudeMDPath(domainDir, "/operator/override/CLAUDE.md"); got != "/operator/override/CLAUDE.md" {
			t.Errorf("explicit non-empty path must win; got %q", got)
		}
	})

	t.Run("claude_md_present", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		domainDir := filepath.Join(root, "domains", "d")
		if err := os.MkdirAll(domainDir, 0o755); err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(root, "CLAUDE.md")
		if err := os.WriteFile(want, []byte("existing crystal"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := resolveClaudeMDPath(domainDir, ""); got != want {
			t.Errorf("CLAUDE.md present: got %q, want %q", got, want)
		}
	})

	t.Run("marker_present_no_claude_md", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		domainDir := filepath.Join(root, "domains", "d")
		if err := os.MkdirAll(domainDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Marker exists but no CLAUDE.md yet — still returns the candidate
		// path so land can bootstrap the crystal where a project convention
		// already exists.
		if err := os.WriteFile(filepath.Join(root, paths.MarkerFilename), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(root, "CLAUDE.md")
		if got := resolveClaudeMDPath(domainDir, ""); got != want {
			t.Errorf("marker present: got %q, want %q", got, want)
		}
	})

	t.Run("no_convention_returns_empty", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		domainDir := filepath.Join(root, "domains", "d")
		if err := os.MkdirAll(domainDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// No CLAUDE.md, no marker → no auto-write.
		if got := resolveClaudeMDPath(domainDir, ""); got != "" {
			t.Errorf("no convention: got %q, want \"\"", got)
		}
	})

	// Task A2: a non-active consumer domain under <repoRoot>/domains/, in a
	// project that DOES carry the crystal convention (root CLAUDE.md or
	// marker), defaults to its OWN local crystal at <domainDir>/CLAUDE.md —
	// not the root crystal (which would hijack the active domain's identity)
	// and not silence.
	t.Run("non_active_consumer_domain_gets_local_crystal", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		// Two domains: "active" (named in the marker) and "consumer" (the
		// non-active one being resolved).
		activeDir := filepath.Join(root, "domains", "active")
		consumerDir := filepath.Join(root, "domains", "consumer")
		for _, d := range []string{activeDir, consumerDir} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		// Convention: root CLAUDE.md + marker naming "active" (NOT consumer).
		if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("root"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := paths.WriteActiveDomain(filepath.Join(root, paths.MarkerFilename), "active"); err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(consumerDir, "CLAUDE.md")
		if got := resolveClaudeMDPath(consumerDir, ""); got != want {
			t.Errorf("non-active consumer domain: got %q, want local crystal %q", got, want)
		}
	})

	// Task A2 conservative boundary: a consumer domain under domains/ in a
	// project with NO crystal convention (no root CLAUDE.md, no marker) gets
	// no auto-write — the same gate the root crystal always respected, so a
	// bare test fixture or un-adopted project stays crystal-free.
	t.Run("consumer_domain_no_convention_returns_empty", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		consumerDir := filepath.Join(root, "domains", "consumer")
		if err := os.MkdirAll(consumerDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// No root CLAUDE.md, no marker.
		if got := resolveClaudeMDPath(consumerDir, ""); got != "" {
			t.Errorf("consumer domain without convention: got %q, want \"\"", got)
		}
	})
}

// TestCmdLand_AutoCrystal_WhenProjectRootHasClaudeMD is the positive case for
// the fix (review-6 R6-d): `hotam land` WITHOUT --claude-md, against a domain
// whose project root already carries a root CLAUDE.md, must auto-regenerate
// CLAUDE.md/AGENTS.md/GEMINI.md — closing the gap where docs/gen/*.md was fresh
// but the boot crystal an agent reads was stale. Without the fix,
// resolveClaudeMDPath does not exist and the stale sentinel bytes survive land
// untouched (this test FAILS on the pre-fix code).
func TestCmdLand_AutoCrystal_WhenProjectRootHasClaudeMD(t *testing.T) {
	t.Parallel()
	projectRoot, domainDir := copySelfDomainUnderRoot(t)

	// Pre-create a STALE root crystal so (a) resolveClaudeMDPath detects the
	// project-root convention and (b) we can prove land OVERWRITES it.
	stale := []byte("STALE-CRYSTAL-BASELINE\n")
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		if err := os.WriteFile(filepath.Join(projectRoot, name), stale, 0o644); err != nil {
			t.Fatalf("write stale %s: %v", name, err)
		}
	}

	// Capture the pre-apply debt line from docs/gen (rendered WITHOUT touching
	// the crystal) so the freshness assertion below is robust to whatever the
	// fixture's current DRAFT count is.
	if _, _, err := genSpec(domainDir, "", "2026-07-14", "", false); err != nil {
		t.Fatalf("baseline genSpec: %v", err)
	}
	baselineLS, err := os.ReadFile(filepath.Join(domainDir, "docs", "gen", "live-state.md"))
	if err != nil {
		t.Fatalf("read baseline live-state.md: %v", err)
	}
	baselineDebt := crystalDebtLine(string(baselineLS))

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-auto-crystal",
		"claim": "land without --claude-md must auto-regenerate the root crystal",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "auto-crystal coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	if err := cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-14",
		proposalPath,
	}); err != nil {
		t.Fatalf("cmdLand: %v", err)
	}

	claude, err := os.ReadFile(filepath.Join(projectRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read post-land CLAUDE.md: %v", err)
	}
	// (a) Overwritten — no longer the stale sentinel.
	if string(claude) == string(stale) {
		t.Fatal("CLAUDE.md was NOT regenerated — still the stale baseline")
	}
	// (b) A real render.
	if !strings.Contains(string(claude), "# CLAUDE.md — Hotam-Spec framework") {
		t.Errorf("CLAUDE.md does not carry the crystal header — not a real render")
	}
	// (c) FRESH — the debt line changed because a DRAFT requirement was added.
	if postDebt := crystalDebtLine(string(claude)); postDebt == "" {
		t.Errorf("CLAUDE.md has no LIVE-STATE debt line")
	} else if postDebt == baselineDebt {
		t.Errorf("CLAUDE.md debt line unchanged from baseline — crystal does not reflect the applied proposal:\nbaseline: %s\npost:     %s", baselineDebt, postDebt)
	}

	// AGENTS.md and GEMINI.md must be byte-identical to CLAUDE.md (same render
	// fanned out together).
	for _, name := range []string{"AGENTS.md", "GEMINI.md"} {
		got, err := os.ReadFile(filepath.Join(projectRoot, name))
		if err != nil {
			t.Fatalf("read post-land %s: %v", name, err)
		}
		if string(got) != string(claude) {
			t.Errorf("%s differs from CLAUDE.md — crystal fan-out wrote non-identical content", name)
		}
	}
}

// TestCmdLand_NoAutoCrystal_WhenNoProjectRootConvention is the negative case:
// a domain NOT linked to any project-root convention (no CLAUDE.md, no
// .hotam-spec-project marker at the resolved root) must NOT spontaneously
// create a crystal when --claude-md is omitted. Guards against over-eager
// writes into bare/isolated domains. Without the fix this passes trivially;
// it must KEEP passing with the fix.
func TestCmdLand_NoAutoCrystal_WhenNoProjectRootConvention(t *testing.T) {
	t.Parallel()
	projectRoot, domainDir := copySelfDomainUnderRoot(t)
	// Deliberately create NO CLAUDE.md and NO marker at projectRoot.

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-no-crystal",
		"claim": "no crystal must appear without a project-root convention",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "negative auto-crystal coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	if err := cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-14",
		proposalPath,
	}); err != nil {
		t.Fatalf("cmdLand: %v", err)
	}

	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		if _, err := os.Stat(filepath.Join(projectRoot, name)); err == nil {
			t.Errorf("%s was spontaneously created at the project root — resolveClaudeMDPath should return \"\" with no crystal/marker convention", name)
		}
	}
}

// TestCmdLand_ExplicitClaudeMD_OverridesAutoDetect proves the operator
// override still wins: even when a root CLAUDE.md exists (so auto-detect WOULD
// fire), an explicit --claude-md points land at a DIFFERENT path and leaves the
// auto-detected crystal untouched. Without resolveClaudeMDPath's explicit-wins
// branch this would write to the wrong place.
func TestCmdLand_ExplicitClaudeMD_OverridesAutoDetect(t *testing.T) {
	t.Parallel()
	projectRoot, domainDir := copySelfDomainUnderRoot(t)

	// Pre-create CLAUDE.md at projectRoot so auto-detect is armed.
	stale := []byte("STALE-AUTO-DETECT-TARGET\n")
	if err := os.WriteFile(filepath.Join(projectRoot, "CLAUDE.md"), stale, 0o644); err != nil {
		t.Fatalf("write stale CLAUDE.md: %v", err)
	}

	// Explicit --claude-md points ELSEWHERE.
	otherDir := t.TempDir()
	explicit := filepath.Join(otherDir, "CLAUDE.md")

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-override",
		"claim": "explicit --claude-md must override auto-detection",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "override coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	if err := cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-14",
		"--claude-md", explicit,
		proposalPath,
	}); err != nil {
		t.Fatalf("cmdLand: %v", err)
	}

	// The EXPLICIT path got the real render.
	got, err := os.ReadFile(explicit)
	if err != nil {
		t.Fatalf("read explicit CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(got), "# CLAUDE.md — Hotam-Spec framework") {
		t.Errorf("explicit --claude-md path was not written with a real render")
	}
	for _, name := range []string{"AGENTS.md", "GEMINI.md"} {
		if _, err := os.Stat(filepath.Join(otherDir, name)); err != nil {
			t.Errorf("%s not written alongside explicit CLAUDE.md: %v", name, err)
		}
	}

	// The auto-detected projectRoot/CLAUDE.md must be UNTOUCHED (still stale).
	projClaude, err := os.ReadFile(filepath.Join(projectRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read projectRoot CLAUDE.md: %v", err)
	}
	if string(projClaude) != string(stale) {
		t.Errorf("projectRoot/CLAUDE.md was modified despite explicit --claude-md override (len stale=%d got=%d)", len(stale), len(projClaude))
	}
}

// TestCmdLand_AutoCrystal_IdempotentAcrossGenspec guards against
// self-referential drift in the rune-count fixpoint: landing (which
// auto-writes the crystal via genSpec) and then re-running genSpec on the same
// auto-detected path must produce byte-identical crystal bytes. This project
// has hit rune-count fixpoint drift before (see ComputeCrystalCharCountFixpoint
// in gen_spec.go); a regression would make two consecutive renders differ.
func TestCmdLand_AutoCrystal_IdempotentAcrossGenspec(t *testing.T) {
	t.Parallel()
	projectRoot, domainDir := copySelfDomainUnderRoot(t)
	// Seed a crystal so resolveClaudeMDPath arms the auto-write.
	if err := os.WriteFile(filepath.Join(projectRoot, "CLAUDE.md"), []byte("seed"), 0o644); err != nil {
		t.Fatalf("write seed CLAUDE.md: %v", err)
	}

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-idempotent",
		"claim": "auto-written crystal must be stable across consecutive gen-spec runs",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "fixpoint idempotency coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	if err := cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-14",
		proposalPath,
	}); err != nil {
		t.Fatalf("cmdLand: %v", err)
	}

	first, err := os.ReadFile(filepath.Join(projectRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read first CLAUDE.md: %v", err)
	}

	// Re-run genSpec on the SAME auto-detected path; the graph is unchanged
	// since land, so the crystal must converge to the identical fixpoint.
	resolved := resolveClaudeMDPath(domainDir, "")
	if resolved == "" {
		t.Fatal("resolveClaudeMDPath returned empty despite seeded CLAUDE.md")
	}
	if _, _, err := genSpec(domainDir, resolved, "2026-07-14", "", false); err != nil {
		t.Fatalf("second genSpec: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(projectRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read second CLAUDE.md: %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("auto-written crystal is NOT byte-identical across two gen-spec runs — rune-count fixpoint drift (len first=%d second=%d)", len(first), len(second))
	}
}

// addSecondDomain scaffolds a second domain directory named domainName under
// <projectRoot>/domains/, copying the same self-domain graph+manifest fixture
// copySelfDomainUnderRoot uses. It returns the new domain's directory. Used
// by the R7-b active-domain-awareness tests to build a genuine multi-domain
// project root (copySelfDomainUnderRoot alone only ever produces one domain
// under domains/, which is not enough to exercise the "2+ domains, must
// match the active one" branch of resolveClaudeMDPath).
func addSecondDomain(t *testing.T, projectRoot, domainName string) string {
	t.Helper()
	domainDir := filepath.Join(projectRoot, "domains", domainName)
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir second domain: %v", err)
	}
	copyFile(t, selfDomainGraph, filepath.Join(domainDir, "graph.json"))
	copyFile(t, selfDomainManifest, filepath.Join(domainDir, "manifest.json"))
	return domainDir
}

// TestCmdLand_NoAutoCrystal_WhenLandingDomainIsNotActive is the R7-b
// regression test (review-7 finding: task #134's resolveClaudeMDPath auto-
// wrote the root crystal for ANY domain linked to a convention-carrying
// project root, never checking whether that domain was the marker's recorded
// active_domain — so landing a non-active domain silently hijacked the root
// crystal's identity). Two domains exist under one project root; the marker
// records "hotam-spec-self" as active; landing the OTHER domain ("second")
// without --claude-md must leave the root CLAUDE.md byte-identical to its
// pre-land content. This test FAILS on the pre-fix code (which does not
// consult the active domain at all).
//
// Task A2 extends the assertion: landing a non-active consumer domain must
// now write its OWN LOCAL crystal at <secondDomainDir>/CLAUDE.md (the
// systematic default that previously required --claude-md), NOT hijack the
// root crystal. The positive local-crystal assertions at the end prove the
// new default fires; the root-untouched assertions above prove it is not a
// root hijack.
func TestCmdLand_NoAutoCrystal_WhenLandingDomainIsNotActive(t *testing.T) {
	t.Parallel()
	// copySelfDomainUnderRoot scaffolds the FIRST domain as
	// "hotam-spec-self"; that is the name recorded as active_domain below.
	// The SECOND domain, "second", is the one actually landed here.
	projectRoot, _ := copySelfDomainUnderRoot(t)
	secondDomainDir := addSecondDomain(t, projectRoot, "second")

	// Root crystal convention: a real CLAUDE.md, plus a marker recording
	// hotam-spec-self (not "second") as the active domain.
	baseline := []byte("BASELINE-ACTIVE-DOMAIN-IS-HOTAM-SPEC-SELF\n")
	if err := os.WriteFile(filepath.Join(projectRoot, "CLAUDE.md"), baseline, 0o644); err != nil {
		t.Fatalf("write baseline CLAUDE.md: %v", err)
	}
	markerPath := filepath.Join(projectRoot, paths.MarkerFilename)
	if err := paths.WriteActiveDomain(markerPath, "hotam-spec-self"); err != nil {
		t.Fatalf("write active-domain marker: %v", err)
	}

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-non-active-domain",
		"claim": "landing a non-active domain must not hijack the root crystal",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "R7-b regression coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	// Land into "second" — NOT the marker's active_domain — without
	// --claude-md.
	if err := cmdLand([]string{
		"--domain", secondDomainDir,
		"--today", "2026-07-14",
		proposalPath,
	}); err != nil {
		t.Fatalf("cmdLand: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(projectRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read post-land CLAUDE.md: %v", err)
	}
	if string(got) != string(baseline) {
		t.Fatalf("root CLAUDE.md was hijacked by landing the NON-active domain — got %d bytes, want unchanged baseline (%d bytes):\n%s", len(got), len(baseline), string(got))
	}

	// Sanity: the OTHER two fan-out files must also be untouched (never even
	// created, since this project root started with only CLAUDE.md).
	for _, name := range []string{"AGENTS.md", "GEMINI.md"} {
		if _, err := os.Stat(filepath.Join(projectRoot, name)); err == nil {
			t.Errorf("%s was spontaneously created at the project root while landing the non-active domain", name)
		}
	}

	// Task A2: landing a non-active consumer domain must now write its OWN
	// LOCAL crystal at <domainDir>/CLAUDE.md (+ AGENTS.md + GEMINI.md), the
	// systematic default that previously required a one-off --claude-md.
	// The root crystal staying untouched (asserted above) proves this is NOT
	// a root hijack — the local crystal writes alongside the domain it
	// speaks for.
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		p := filepath.Join(secondDomainDir, name)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("expected local crystal %s for the non-active domain, not written: %v", p, err)
			continue
		}
		if len(b) == 0 {
			t.Errorf("local crystal %s was written but is empty", p)
		}
	}
}

// TestCmdLand_AutoCrystal_WhenLandingDomainIsActive is the positive
// counterpart to the test above, in a genuine multi-domain project (not the
// single-domain shortcut): two domains exist under one project root, the
// marker names hotam-spec-self as active, and landing hotam-spec-self itself
// must still auto-write the root crystal exactly as task #134 intended. This
// proves the fix's "matches the active domain" branch actually fires, not
// just the single-domain unambiguous shortcut.
func TestCmdLand_AutoCrystal_WhenLandingDomainIsActive(t *testing.T) {
	t.Parallel()
	projectRoot, activeDomainDir := copySelfDomainUnderRoot(t)
	addSecondDomain(t, projectRoot, "second")

	stale := []byte("STALE-CRYSTAL-BASELINE\n")
	if err := os.WriteFile(filepath.Join(projectRoot, "CLAUDE.md"), stale, 0o644); err != nil {
		t.Fatalf("write stale CLAUDE.md: %v", err)
	}
	markerPath := filepath.Join(projectRoot, paths.MarkerFilename)
	if err := paths.WriteActiveDomain(markerPath, "hotam-spec-self"); err != nil {
		t.Fatalf("write active-domain marker: %v", err)
	}

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-active-domain",
		"claim": "landing the active domain must still auto-write the crystal",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "R7-b positive-case coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	if err := cmdLand([]string{
		"--domain", activeDomainDir,
		"--today", "2026-07-14",
		proposalPath,
	}); err != nil {
		t.Fatalf("cmdLand: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(projectRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read post-land CLAUDE.md: %v", err)
	}
	if string(got) == string(stale) {
		t.Fatal("CLAUDE.md was NOT regenerated when landing the genuinely active domain in a multi-domain project")
	}
	if !strings.Contains(string(got), "# CLAUDE.md — Hotam-Spec framework") {
		t.Errorf("CLAUDE.md does not carry the crystal header — not a real render")
	}
}

// TestCmdLand_AutoCrystal_SingleDomainNoMarker is explicit coverage for the
// "single domain under domains/, no active_domain recorded anywhere" branch:
// unlike the pre-existing TestCmdLand_AutoCrystal_WhenProjectRootHasClaudeMD
// (which never writes a marker at all — same shape, but this test names the
// case explicitly and additionally confirms the marker file itself may be
// entirely ABSENT, not just silent), a lone domain under <root>/domains/ is
// unambiguous and must still auto-write even with zero recorded preference.
func TestCmdLand_AutoCrystal_SingleDomainNoMarker(t *testing.T) {
	t.Parallel()
	projectRoot, domainDir := copySelfDomainUnderRoot(t)

	stale := []byte("STALE-SINGLE-DOMAIN-BASELINE\n")
	if err := os.WriteFile(filepath.Join(projectRoot, "CLAUDE.md"), stale, 0o644); err != nil {
		t.Fatalf("write stale CLAUDE.md: %v", err)
	}
	// Deliberately NO marker file at all.
	if _, err := os.Stat(filepath.Join(projectRoot, paths.MarkerFilename)); err == nil {
		t.Fatal("marker unexpectedly present — test setup invariant violated")
	}

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-single-domain-no-marker",
		"claim": "a lone domain under domains/ auto-writes even with no marker",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "R7-b single-domain-unambiguous coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	if err := cmdLand([]string{
		"--domain", domainDir,
		"--today", "2026-07-14",
		proposalPath,
	}); err != nil {
		t.Fatalf("cmdLand: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(projectRoot, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read post-land CLAUDE.md: %v", err)
	}
	if string(got) == string(stale) {
		t.Fatal("CLAUDE.md was NOT regenerated for the lone/unambiguous domain under domains/ with no marker")
	}
}

// TestCmdLand_AutoCrystal_RepoRootIsDomainDir covers the bare/root-is-the-
// domain tier-3 layout explicitly: domainDir IS repoRoot (no domains/ parent
// at all), so there is nothing to disambiguate and auto-write must still
// fire when a CLAUDE.md/marker convention exists at that same directory.
//
// Reaching repoRootForDomain's tier-3 fallback (return domainDir itself)
// requires BOTH env project-root overrides cleared AND a CWD ancestry with no
// discoverable project-root marker — otherwise tier-2 (paths.ProjectRootOrRaise)
// succeeds and resolves to this repo's OWN root instead (its go.mod-anchored
// R6 fallback finds this checkout unconditionally). Not t.Parallel(): it
// changes the process CWD via chdirAndRestore, exactly like
// TestRepoRootForDomain_NoProjectRootFallsBackToDomainDir (gen_spec_test.go),
// whose isolation pattern this test mirrors.
func TestCmdLand_AutoCrystal_RepoRootIsDomainDir(t *testing.T) {
	// Fixture source paths (selfDomainGraph/selfDomainManifest) are CWD-relative
	// ("../../domains/..."), so they must be copied out BEFORE chdirAndRestore
	// below repoints the process CWD — otherwise copyFile fails to find them
	// from the isolated empty dir.
	root := t.TempDir()
	copyFile(t, selfDomainGraph, filepath.Join(root, "graph.json"))
	copyFile(t, selfDomainManifest, filepath.Join(root, "manifest.json"))

	empty := t.TempDir()
	chdirAndRestore(t, empty)
	t.Setenv(paths.EnvProjectRoot, "")
	t.Setenv(paths.EnvDomainsRoot, "")
	skipIfCwdAncestryNotHermetic(t, empty)

	// Bare layout: graph.json lives directly in root, no domains/ parent, so
	// repoRootForDomain's tier-1 (domains/<name>) check fails and it falls
	// through to tier-3 (return domainDir itself, i.e. repoRoot == domainDir)
	// now that tier-2 is guaranteed to fail by the isolation above.

	stale := []byte("STALE-BARE-ROOT-BASELINE\n")
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), stale, 0o644); err != nil {
		t.Fatalf("write stale CLAUDE.md: %v", err)
	}

	// Sanity: confirm repoRootForDomain really does resolve to domainDir
	// itself for this bare layout, so this test is exercising the intended
	// branch and not silently degenerating into the domains/-parented case.
	if got := repoRootForDomain(root); got != root {
		t.Fatalf("test setup invariant violated: repoRootForDomain(%q) = %q, want == domainDir (bare layout)", root, got)
	}

	proposalPath := filepath.Join(t.TempDir(), "proposal.json")
	proposalJSON := `{
		"kind": "Requirement",
		"id": "R-land-bare-root-is-domain",
		"claim": "a bare root-is-the-domain layout auto-writes with nothing to disambiguate",
		"owner": "framework-author",
		"status": "DRAFT",
		"why": "R7-b repoRoot-equals-domainDir coverage"
	}`
	if err := os.WriteFile(proposalPath, []byte(proposalJSON), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}

	if err := cmdLand([]string{
		"--domain", root,
		"--today", "2026-07-14",
		proposalPath,
	}); err != nil {
		t.Fatalf("cmdLand: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read post-land CLAUDE.md: %v", err)
	}
	if string(got) == string(stale) {
		t.Fatal("CLAUDE.md was NOT regenerated for the bare repoRoot==domainDir layout")
	}
}
