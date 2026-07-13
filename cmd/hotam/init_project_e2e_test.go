package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestExternal_InitProject is the end-to-end proof for `hotam init-project`:
// it builds the real binary, runs it as a subprocess from a genuinely external
// temp dir (outside both this repo AND C:\Users\Computer, whose stray
// home-dir markers can mask CWD-resolution bugs — hence TMP/TEMP pointed at a
// clean root), and verifies the full onboarding contract:
//
//   - init-project scaffolds the project marker + base domain + root crystal;
//   - the scaffolded domain is invariant-clean (0 violations);
//   - the project-root marker resolves (paths.ProjectRoot R4) AND records the
//     scaffolded domain as the active-domain preference, so a bare
//     `hotam status` (no --domain) from inside the project SUCCEEDS and
//     targets domains/main (resolveDomain tier-3 marker resolution);
//   - an explicit --domain domains/main (relative, CWD-rooted) works from
//     inside the project, and an explicit absolute --domain works from outside;
//   - a second init-project on the same dir refuses (overwrite guard).
func TestExternal_InitProject(t *testing.T) {
	if testing.Short() {
		t.Skip("external e2e: builds a real binary + spawns child processes; skipped in -short")
	}
	// Not t.Parallel(): this test mutates TMP/TEMP (process-global) via
	// t.Setenv to steer os.MkdirTemp at a clean root, and env-mutating tests
	// are inherently serial.

	// Clean temp roots outside both the repo and C:\Users\Computer, so this
	// developer's stray home-dir markers (.claude/CLAUDE.md/domains) cannot
	// mask a CWD-resolution bug. os.MkdirTemp reads TMP/TEMP via os.TempDir.
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

	workDir, err := os.MkdirTemp("", "hotam-initproject-work-")
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

	// clearedEnv strips HOTAM_SPEC_PROJECT_ROOT / HOTAM_SPEC_DOMAINS_ROOT so the
	// child cannot inherit the outer runner's project-root override. TMP/TEMP
	// are kept (set to cleanTmp above) so any child temp op stays clean too.
	clearedEnv := filteredEnv(t, "HOTAM_SPEC_PROJECT_ROOT", "HOTAM_SPEC_DOMAINS_ROOT")

	// runAt runs the binary at cwd with env, returning combined output + the
	// exec error (nil on exit 0) so failure-asserting steps can inspect both.
	runAt := func(cwd string, args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command(binPath, args...)
		cmd.Dir = cwd
		cmd.Env = clearedEnv
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	projDir := filepath.Join(workDir, "myproject")
	domainDir := filepath.Join(projDir, "domains", "main")

	// (1) hotam init-project: full bootstrap from nothing.
	out, err := runAt(workDir, "init-project", projDir, "--today", "2026-07-13")
	if err != nil {
		t.Fatalf("init-project failed: %v\nOUTPUT:\n%s", err, out)
	}
	if !strings.Contains(out, `initialized project at`) {
		t.Errorf("init-project output missing confirmation:\n%s", out)
	}

	// (2) Project-root marker exists.
	marker := filepath.Join(projDir, ".hotam-spec-project")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("project-root marker not created: %v", err)
	}

	// (3) Root crystal exists at the project root and its DOMAIN-MAP block
	// lists the scaffolded domain (main), proving repoRootForDomain derived
	// the project root from the domains/<name> layout with no extra plumbing.
	claudeMD := filepath.Join(projDir, "CLAUDE.md")
	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("CLAUDE.md not created at project root: %v", err)
	}
	claudeBody := string(data)
	if !strings.Contains(claudeBody, "### main") {
		t.Errorf("CLAUDE.md DOMAIN-MAP does not list the 'main' domain:\n%s", claudeBody)
	}
	if !strings.Contains(claudeBody, "domains/main/") {
		t.Errorf("CLAUDE.md does not reference domains/main/:\n%s", claudeBody)
	}

	// (4) Scaffolded domain is invariant-clean immediately.
	avOut, err := runAt(workDir, "all-violations", "--domain", domainDir)
	if err != nil {
		t.Fatalf("all-violations against scaffolded domain failed: %v\nOUTPUT:\n%s", err, avOut)
	}
	if !strings.Contains(avOut, "0 violations") {
		t.Errorf("scaffolded domain is not clean:\n%s", avOut)
	}

	// (5) ACTIVE-DOMAIN RESOLUTION PROOF (the review-5 fix): run with NO
	// --domain from CWD inside the project. The marker resolves the project
	// root (paths.ProjectRoot R4 finds .hotam-spec-project) AND init-project
	// recorded the scaffolded domain ("main") as the active-domain preference,
	// so resolveDomain's tier-3 marker resolution joins the root with
	// domains/main. So this MUST succeed and print the status pulse — the
	// inverse of the pre-fix behavior, where the hardcoded default
	// "domains/hotam-spec-self" missed. The honesty-over-magic stderr notice
	// ("resolved domain: main (via .hotam-spec-project marker)") fires on the
	// magic-resolution path; CombinedOutput merges it with stdout.
	noDomOut, noDomErr := runAt(projDir, "status")
	if noDomErr != nil {
		t.Fatalf("bare `hotam status` (no --domain) failed inside a domains/main project after the active-domain fix — marker resolution regressed:\n%v\nOUTPUT:\n%s", noDomErr, noDomOut)
	}
	if !strings.Contains(noDomOut, "violations:") {
		t.Errorf("bare `hotam status` output missing the status pulse (violations line):\n%s", noDomOut)
	}
	if !strings.Contains(noDomOut, "resolved domain: main") {
		t.Errorf("bare `hotam status` should report the tier-3 marker resolution on stderr (\"resolved domain: main ...\"), got:\n%s", noDomOut)
	}
	if strings.Contains(noDomOut, "hotam-spec-self") {
		t.Errorf("bare `hotam status` should target domains/main, not the legacy hotam-spec-self default; output references it:\n%s", noDomOut)
	}

	// (6) Explicit relative --domain from CWD inside the project works.
	relOut, err := runAt(projDir, "status", "--domain", filepath.Join("domains", "main"))
	if err != nil {
		t.Errorf("explicit relative --domain status from inside project failed: %v\nOUTPUT:\n%s", err, relOut)
	}
	if !strings.Contains(relOut, "violations:") {
		t.Errorf("status output missing violations line:\n%s", relOut)
	}

	// (7) Explicit absolute --domain from CWD outside the project works
	// (the non-marker path — resolveDomain takes --domain verbatim via
	// filepath.Abs, no project-root resolution involved).
	absOut, err := runAt(workDir, "what-now", "--domain", domainDir)
	if err != nil {
		t.Errorf("explicit absolute --domain what-now from outside project failed: %v\nOUTPUT:\n%s", err, absOut)
	}
	if strings.TrimSpace(absOut) == "" {
		t.Error("what-now produced no output")
	}

	// (8) Second init-project on the same dir must refuse (overwrite guard).
	dupOut, dupErr := runAt(workDir, "init-project", projDir, "--today", "2026-07-13")
	if dupErr == nil {
		t.Fatalf("second init-project unexpectedly succeeded instead of refusing:\n%s", dupOut)
	}
	if !strings.Contains(dupOut, "already exists") {
		t.Errorf("second init-project should refuse with 'already exists', got:\n%s", dupOut)
	}
}
