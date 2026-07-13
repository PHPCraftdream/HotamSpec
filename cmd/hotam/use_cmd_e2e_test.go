package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// TestExternal_Use_SwitchesActiveDomain is the end-to-end proof for
// `hotam use <domain-name>`: it builds the real binary, scaffolds a project
// with TWO domains, runs `hotam use second` from inside the project, and
// confirms (a) the marker file now records {"active_domain": "second"} and
// (b) a subsequent bare `hotam status` (no --domain) targets domains/second via
// resolveDomain's tier-3 marker resolution.
//
// It reuses the package's shared test helpers (repoRootForTest,
// buildSharedHotamBinary, filteredEnv, isInsideForTest) defined in the other
// _test.go files in this package.
func TestExternal_Use_SwitchesActiveDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("external e2e: builds a real binary + spawns child processes; skipped in -short")
	}
	// Not t.Parallel(): mutates TMP/TEMP (process-global) via t.Setenv.

	// Clean temp roots outside both the repo and C:\Users\Computer (same
	// marker-isolation discipline as TestExternal_InitProject).
	cleanTmp := filepath.FromSlash("D:/ai_dev/_clean_tmp")
	if st, err := os.Stat(cleanTmp); err != nil || !st.IsDir() {
		t.Skipf("clean tmp root %s unavailable (%v) — required for marker-isolation on this host", cleanTmp, err)
	}
	t.Setenv("TMP", cleanTmp)
	t.Setenv("TEMP", cleanTmp)

	repoRoot := repoRootForTest(t)
	binPath := buildSharedHotamBinary(t)
	if isInsideForTest(filepath.Dir(binPath), repoRoot) {
		t.Fatalf("test invariant broken: binary built inside repo root")
	}

	workDir, err := os.MkdirTemp("", "hotam-use-work-")
	if err != nil {
		t.Fatalf("MkdirTemp workDir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(workDir) })
	if isInsideForTest(workDir, repoRoot) {
		t.Fatalf("test invariant broken: workDir inside repo root")
	}

	clearedEnv := filteredEnv(t, "HOTAM_SPEC_PROJECT_ROOT", "HOTAM_SPEC_DOMAINS_ROOT", "HOTAM_DOMAIN")

	runAt := func(cwd string, args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command(binPath, args...)
		cmd.Dir = cwd
		cmd.Env = clearedEnv
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	projDir := filepath.Join(workDir, "myproject")
	secondDomainDir := filepath.Join(projDir, "domains", "second")

	// (1) init-project scaffolds the project with the default "main" domain and
	// records main as the active-domain preference in the marker.
	if out, err := runAt(workDir, "init-project", projDir, "--today", "2026-07-13"); err != nil {
		t.Fatalf("init-project failed: %v\nOUTPUT:\n%s", err, out)
	}

	// (2) Scaffold a second bare domain via `hotam init <dir>/domains/second`.
	if out, err := runAt(workDir, "init", secondDomainDir, "--name", "second"); err != nil {
		t.Fatalf("init second domain failed: %v\nOUTPUT:\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(secondDomainDir, "graph.json")); err != nil {
		t.Fatalf("second domain graph.json not created: %v", err)
	}

	// (3) Sanity: the marker still records the scaffolded default ("main").
	markerPath := filepath.Join(projDir, paths.MarkerFilename)
	if name, ok := paths.ReadActiveDomain(markerPath); !ok || name != "main" {
		t.Fatalf("after init-project, marker active_domain should be main, got name=%q ok=%v", name, ok)
	}

	// (4) `hotam use second` from inside the project switches the preference.
	useOut, useErr := runAt(projDir, "use", "second")
	if useErr != nil {
		t.Fatalf("hotam use second failed: %v\nOUTPUT:\n%s", useErr, useOut)
	}
	if !strings.Contains(useOut, `active domain set to "second"`) {
		t.Errorf("hotam use output missing confirmation:\n%s", useOut)
	}

	// (5) The marker now records second.
	if name, ok := paths.ReadActiveDomain(markerPath); !ok || name != "second" {
		t.Errorf("after `hotam use second`, marker active_domain should be second, got name=%q ok=%v", name, ok)
	}

	// (6) A bare `hotam status` (no --domain) now targets domains/second via
	// resolveDomain tier-3 marker resolution, and reports it on stderr.
	statusOut, statusErr := runAt(projDir, "status")
	if statusErr != nil {
		t.Fatalf("bare hotam status after `use second` failed: %v\nOUTPUT:\n%s", statusErr, statusOut)
	}
	if !strings.Contains(statusOut, "resolved domain: second") {
		t.Errorf("bare status should report tier-3 resolution to second, got:\n%s", statusOut)
	}
	if strings.Contains(statusOut, "resolved domain: main") {
		t.Errorf("bare status should no longer target main after `use second`, got:\n%s", statusOut)
	}
}

// TestExternal_Use_RefusesWhenDomainMissing asserts `hotam use` refuses to
// point the active domain at a non-existent domain (no graph.json under
// domains/<name>), so it can never silently point the active domain at nothing.
func TestExternal_Use_RefusesWhenDomainMissing(t *testing.T) {
	if testing.Short() {
		t.Skip("external e2e: builds a real binary + spawns child processes; skipped in -short")
	}
	cleanTmp := filepath.FromSlash("D:/ai_dev/_clean_tmp")
	if st, err := os.Stat(cleanTmp); err != nil || !st.IsDir() {
		t.Skipf("clean tmp root %s unavailable (%v) — required for marker-isolation on this host", cleanTmp, err)
	}
	t.Setenv("TMP", cleanTmp)
	t.Setenv("TEMP", cleanTmp)

	repoRoot := repoRootForTest(t)
	binPath := buildSharedHotamBinary(t)
	workDir, err := os.MkdirTemp("", "hotam-use-refuse-")
	if err != nil {
		t.Fatalf("MkdirTemp workDir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(workDir) })
	if isInsideForTest(workDir, repoRoot) {
		t.Fatalf("test invariant broken: workDir inside repo root")
	}

	clearedEnv := filteredEnv(t, "HOTAM_SPEC_PROJECT_ROOT", "HOTAM_SPEC_DOMAINS_ROOT", "HOTAM_DOMAIN")
	runAt := func(cwd string, args ...string) (string, error) {
		cmd := exec.Command(binPath, args...)
		cmd.Dir = cwd
		cmd.Env = clearedEnv
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	projDir := filepath.Join(workDir, "myproject")
	if out, err := runAt(workDir, "init-project", projDir, "--today", "2026-07-13"); err != nil {
		t.Fatalf("init-project failed: %v\nOUTPUT:\n%s", err, out)
	}

	out, err := runAt(projDir, "use", "does-not-exist")
	if err == nil {
		t.Fatalf("hotam use of a non-existent domain should refuse, got success:\n%s", out)
	}
	if !strings.Contains(out, "not found") || !strings.Contains(out, "does-not-exist") {
		t.Errorf("refusal should name the missing domain, got:\n%s", out)
	}
}
