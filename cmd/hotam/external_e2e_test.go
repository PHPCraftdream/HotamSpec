package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestExternal_InitApplyLandReqWhatNowAllViolations is the "applicability to
// external projects" proof TaskList P1-7 exists to produce (applicability
// score was 3/10: no scaffold command, no e2e evidence the hotam binary
// works outside this repository's own checkout).
//
// Unlike every other test in this package, this one does NOT call cmdInit /
// cmdLand / etc. as Go functions in-process against a t.TempDir() fixture
// copied from domains/hotam-spec-self (see main_test.go's copySelfDomain).
// It instead:
//
//  1. go build's the real hotam binary into a fresh os.MkdirTemp directory
//     that this test asserts is OUTSIDE the repository working tree
//     (t.TempDir() on some platforms resolves under the module's own
//     checkout via a symlinked temp root, so this is verified explicitly
//     rather than assumed).
//  2. runs that binary as a real child process (os/exec), with its working
//     directory set to ANOTHER fresh temp directory that contains none of
//     internal/paths' project-root markers (no domains/, no delegations/,
//     no CLAUDE.md, no .claude/, no tickets/, no pyproject.toml) — so if
//     `hotam ... --domain <abs-path>` ever fell back to
//     paths.ProjectRootOrRaise() instead of honoring an explicit --domain
//     flag verbatim, this test's working directory would make that
//     fallback fail loudly (ProjectRootUnresolved), not silently resolve
//     to this repo's own domains/hotam-spec-self by accident.
//  3. drives init -> apply-proposal (Stakeholder) -> land (Requirement,
//     real ProposedRequirement JSON in snake_case) -> req show / what-now /
//     all-violations -> gen-spec, exactly the sequence
//     docs/QUICKSTART-CONSUMER.md walks a human through by hand, and checks
//     the generated docs/gen output at the end.
//
// This proves R-project-root-not-hardcoded end-to-end: resolveDomain(...)
// (cmd/hotam/common.go) takes an explicit --domain and turns it into an
// absolute path via filepath.Abs alone — it never calls
// paths.ProjectRootOrRaise() unless --domain is EMPTY (see common.go) — so
// external callers who always pass --domain never touch this repo's own
// project-root resolution at all.
func TestExternal_InitApplyLandReqWhatNowAllViolations(t *testing.T) {
	if testing.Short() {
		t.Skip("external e2e: builds a real binary + spawns child processes; skipped in -short")
	}
	t.Parallel()

	repoRoot := repoRootForTest(t)

	// (a) Build the real binary into a temp dir OUTSIDE the repo tree.
	binDir, err := os.MkdirTemp("", "hotam-ext-bin-")
	if err != nil {
		t.Fatalf("MkdirTemp binDir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(binDir) })
	if isInsideForTest(binDir, repoRoot) {
		t.Fatalf("test invariant broken: binDir %s resolved inside repo root %s", binDir, repoRoot)
	}

	binName := "hotam"
	if runtime.GOOS == "windows" {
		binName = "hotam.exe"
	}
	binPath := filepath.Join(binDir, binName)

	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/hotam")
	buildCmd.Dir = repoRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("go build hotam binary failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("built binary missing at %s: %v", binPath, err)
	}

	// (b) A separate temp dir, ALSO outside the repo, that carries NONE of
	// internal/paths' project-root markers — this is the "foreign project"
	// the binary is run from, and doubles as its process working directory
	// (cwd), so a --domain fallback bug would resolve against a directory
	// with no marker at all rather than accidentally against this repo.
	workDir, err := os.MkdirTemp("", "hotam-ext-work-")
	if err != nil {
		t.Fatalf("MkdirTemp workDir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(workDir) })
	if isInsideForTest(workDir, repoRoot) {
		t.Fatalf("test invariant broken: workDir %s resolved inside repo root %s", workDir, repoRoot)
	}
	for _, marker := range []string{"domains", "delegations", "CLAUDE.md", ".claude", "tickets", "pyproject.toml"} {
		if _, err := os.Stat(filepath.Join(workDir, marker)); err == nil {
			t.Fatalf("test invariant broken: workDir unexpectedly contains marker %q", marker)
		}
	}

	domainDir := filepath.Join(workDir, "my-external-project", "domains", "acme")

	run := func(env []string, args ...string) string {
		t.Helper()
		cmd := exec.Command(binPath, args...)
		cmd.Dir = workDir
		if env != nil {
			cmd.Env = env
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("hotam %s failed: %v\nOUTPUT:\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}

	// clearedEnv strips HOTAM_SPEC_PROJECT_ROOT / HOTAM_SPEC_DOMAINS_ROOT
	// (internal/paths.EnvProjectRoot / EnvDomainsRoot) so this test cannot
	// pass only because the outer test-runner's environment happens to
	// point at this repo — every call below must succeed purely from
	// --domain plus the isolated cwd.
	clearedEnv := filteredEnv(t, "HOTAM_SPEC_PROJECT_ROOT", "HOTAM_SPEC_DOMAINS_ROOT")

	// (c) hotam init: scaffold the domain from nothing.
	initOut := run(clearedEnv, "init", domainDir, "--name", "acme")
	if !strings.Contains(initOut, `initialized domain "acme"`) {
		t.Errorf("init output missing confirmation line:\n%s", initOut)
	}
	graphPath := filepath.Join(domainDir, "graph.json")
	if _, err := os.Stat(graphPath); err != nil {
		t.Fatalf("init did not create graph.json: %v", err)
	}

	// A freshly initialized domain must be invariant-clean immediately.
	avOut := run(clearedEnv, "all-violations", "--domain", domainDir)
	if !strings.Contains(avOut, "0 violations") {
		t.Fatalf("freshly initialized domain is not clean:\n%s", avOut)
	}

	// (d) apply-proposal: a real ProposedStakeholder, snake_case JSON,
	// written to a proposal file OUTSIDE the domain dir (as a real operator
	// workflow would keep proposal drafts separate from the graph itself).
	stakeholderProposal := filepath.Join(workDir, "sh-alice.json")
	writeJSONFile(t, stakeholderProposal, map[string]any{
		"kind":   "Stakeholder",
		"id":     "alice",
		"name":   "Alice",
		"domain": "product",
		"why":    "external e2e seed stakeholder",
	})
	applyOut := run(clearedEnv, "apply-proposal", stakeholderProposal, "--domain", domainDir, "--today", "2026-07-12")
	if !strings.Contains(applyOut, "applied Stakeholder alice") {
		t.Errorf("apply-proposal output unexpected:\n%s", applyOut)
	}

	// (e) hotam land: a real ProposedRequirement, snake_case JSON, applied +
	// docs regenerated + re-verified in one step.
	requirementProposal := filepath.Join(workDir, "r-first.json")
	writeJSONFile(t, requirementProposal, map[string]any{
		"kind":           "Requirement",
		"id":             "R-external-first",
		"claim":          "The acme external project shall track its own requirements in Hotam-Spec.",
		"owner":          "alice",
		"status":         "SETTLED",
		"why":            "external e2e proof this graph is usable from a foreign project.",
		"enforcement":    "PROSE",
		"enforceability": "INHERENTLY_PROSE",
	})
	landOut := run(clearedEnv, "land", requirementProposal, "--domain", domainDir, "--today", "2026-07-12")
	if !strings.Contains(landOut, "landed: graph applied, docs regenerated, 0 violations") {
		t.Fatalf("land did not report a clean landing:\n%s", landOut)
	}

	// (f) hotam req show — the newly landed requirement must be readable
	// via --json (machine-checkable, not just a substring match on text
	// output).
	reqShowOut := run(clearedEnv, "req", "show", "R-external-first", "--domain", domainDir, "--json")
	var reqShown struct {
		ID     string `json:"id"`
		Owner  string `json:"owner"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(reqShowOut), &reqShown); err != nil {
		t.Fatalf("req show --json did not parse: %v\noutput:\n%s", err, reqShowOut)
	}
	if reqShown.ID != "R-external-first" || reqShown.Owner != "alice" || reqShown.Status != "SETTLED" {
		t.Errorf("req show --json = %+v, want id=R-external-first owner=alice status=SETTLED", reqShown)
	}

	// (g) hotam what-now must run without error against the foreign domain.
	whatNowOut := run(clearedEnv, "what-now", "--domain", domainDir)
	if strings.TrimSpace(whatNowOut) == "" {
		t.Error("what-now produced no output")
	}

	// (h) hotam all-violations must still report clean after the full
	// sequence.
	avOut2 := run(clearedEnv, "all-violations", "--domain", domainDir)
	if !strings.Contains(avOut2, "0 violations") {
		t.Errorf("domain not clean after land:\n%s", avOut2)
	}

	// (i) hotam gen-spec must have already run as part of land (step e) —
	// verify the rendered docs/gen output actually reflects the new
	// requirement, proving docs and graph did not drift apart.
	genReqPath := filepath.Join(domainDir, "docs", "gen", "REQUIREMENTS.md")
	data, err := os.ReadFile(genReqPath)
	if err != nil {
		t.Fatalf("docs/gen/REQUIREMENTS.md missing after land: %v", err)
	}
	if !strings.Contains(string(data), "R-external-first") {
		t.Errorf("docs/gen/REQUIREMENTS.md does not mention R-external-first:\n%s", data)
	}

	// Explicit hotam gen-spec re-run (idempotent) must also succeed, in
	// case a caller wants to re-render docs standalone (as
	// QUICKSTART-CONSUMER.md's own step 5 does).
	genSpecOut := run(clearedEnv, "gen-spec", "--domain", domainDir)
	if strings.TrimSpace(genSpecOut) == "" {
		t.Error("gen-spec produced no file listing")
	}
}

// repoRootForTest resolves this module's root (the directory containing
// go.mod) via runtime.Caller, exactly like internal/paths.repoRoot's own
// (unexported) fallback — duplicated here rather than imported because
// that helper is intentionally unexported (it's the LAST-resort fallback,
// not a public API), and this test needs the repo root only to (1) run `go
// build` from and (2) assert temp dirs land outside it.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			abs, err := filepath.Abs(dir)
			if err != nil {
				t.Fatalf("filepath.Abs(%s): %v", dir, err)
			}
			return abs
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod walking up from test file")
		}
		dir = parent
	}
}

func isInsideForTest(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// filteredEnv returns the current process environment with the given
// variable names removed, so the child hotam process cannot inherit a
// project-root override from the outer test runner's own environment.
func filteredEnv(t *testing.T, exclude ...string) []string {
	t.Helper()
	excludeSet := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		excludeSet[e] = true
	}
	var out []string
	for _, kv := range os.Environ() {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			out = append(out, kv)
			continue
		}
		key := kv[:eq]
		if excludeSet[key] {
			continue
		}
		out = append(out, kv)
	}
	return out
}
