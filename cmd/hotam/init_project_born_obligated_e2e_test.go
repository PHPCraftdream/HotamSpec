package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestExternal_InitProjectBornObligated is the end-to-end proof for the W6.2
// "no migration window" promise (PLAN-scenario-generated-spec.md §2 D6):
// `hotam init-project <dir>` must scaffold a base domain that is IMMEDIATELY,
// with zero manual follow-up:
//
//   - "discipline": "full" in its manifest.json (unconditional, not gated
//     behind a flag — D6: "у новых проектов нет миграционного окна,
//     обязанность действует с рождения");
//   - "parent": null in its manifest.json (the domain is the root of the
//     new external project, satisfying check_project_parent_declared,
//     internal/invariants/project_parent.go);
//   - equipped with a REAL spec/go.mod and a REAL vendored recorder at
//     spec/hotamspec/hotamspec.go (banner-stamped "DO NOT EDIT", proving it
//     is the genuine vendored copy from vendorRecorder, not an empty/stub
//     placeholder);
//   - clean under a REAL `hotam all-violations` run (0 violations) — the
//     decisive proof that being born under the new, stricter discipline
//     does not itself introduce debt.
//
// This mirrors init_project_e2e_test.go's TestExternal_InitProject shape
// (build the real binary, drive it as a subprocess against a genuinely
// external os.MkdirTemp directory) but is a SEPARATE test focused
// specifically on the born-obligated contract, rather than duplicating that
// test's broader marker/crystal/profile assertions.
func TestExternal_InitProjectBornObligated(t *testing.T) {
	if testing.Short() {
		t.Skip("external e2e: builds a real binary + spawns child processes; skipped in -short")
	}

	repoRoot := repoRootForTest(t)
	binPath := buildSharedHotamBinary(t)
	if isInsideForTest(filepath.Dir(binPath), repoRoot) {
		t.Fatalf("test invariant broken: binary built inside repo root")
	}

	workDir, err := os.MkdirTemp("", "hotam-initproject-bornobligated-")
	if err != nil {
		t.Fatalf("MkdirTemp workDir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(workDir) })
	if isInsideForTest(workDir, repoRoot) {
		t.Fatalf("test invariant broken: workDir %s resolved inside repo root %s", workDir, repoRoot)
	}

	clearedEnv := filteredEnv(t, "HOTAM_SPEC_PROJECT_ROOT", "HOTAM_SPEC_DOMAINS_ROOT")

	runAt := func(cwd string, args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command(binPath, args...)
		cmd.Dir = cwd
		cmd.Env = clearedEnv
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	projDir := filepath.Join(workDir, "bornproject")
	domainDir := filepath.Join(projDir, "domains", "main")

	// (1) hotam init-project: full bootstrap from nothing.
	//
	// --today is pinned to the REAL current date (time.Now()), not a fixed
	// past literal like most other fixtures in this package use, because
	// step (3) below runs `hotam all-violations` in a SEPARATE subprocess
	// immediately afterward, and check_domain_claude_md_current
	// (internal/invariants/claude_md_current.go, wired from
	// cmd/hotam/claude_md_current_wiring.go) has no --today input of its own
	// at all — it always sources today from time.Now() internally (a
	// deliberate design choice: the crystal's LIVE-STATE freshness/OVERDUE
	// signals are supposed to reflect real calendar time). Pinning
	// init-project's OWN --today to a fixed past date while the verification
	// step computes its comparison target against the REAL current date
	// would introduce a spurious day-boundary mismatch unrelated to what
	// this test actually means to prove (the "born obligated, zero
	// violations from birth" contract) — using time.Now() here keeps both
	// subprocess calls on the same calendar day.
	today := time.Now().Format("2006-01-02")
	out, err := runAt(workDir, "init-project", projDir, "--today", today)
	if err != nil {
		t.Fatalf("init-project failed: %v\nOUTPUT:\n%s", err, out)
	}
	if !strings.Contains(out, `initialized project at`) {
		t.Errorf("init-project output missing confirmation:\n%s", out)
	}

	// (2a) manifest.json carries "discipline": "full" — unconditional, no
	// --discipline flag was passed.
	manifestPath := filepath.Join(domainDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest.json not read: %v", err)
	}
	manifestBody := string(manifestData)
	if !strings.Contains(manifestBody, `"discipline": "full"`) {
		t.Errorf("init-project's scaffolded manifest.json must carry \"discipline\": \"full\" unconditionally, got:\n%s", manifestBody)
	}

	// (2b) manifest.json carries "parent": null — this base domain is the
	// root of the new external project.
	if !strings.Contains(manifestBody, `"parent": null`) {
		t.Errorf("init-project's scaffolded manifest.json must carry \"parent\": null (root domain), got:\n%s", manifestBody)
	}

	// (2c) spec/go.mod exists and is a real, minimal Go module.
	specGoModPath := filepath.Join(domainDir, "spec", "go.mod")
	goModData, err := os.ReadFile(specGoModPath)
	if err != nil {
		t.Fatalf("spec/go.mod not created: %v", err)
	}
	goModBody := string(goModData)
	if !strings.Contains(goModBody, "module main-spec") {
		t.Errorf("spec/go.mod missing expected module declaration, got:\n%s", goModBody)
	}
	if !strings.Contains(goModBody, "go 1.25") {
		t.Errorf("spec/go.mod missing expected go directive, got:\n%s", goModBody)
	}

	// (2d) spec/hotamspec/hotamspec.go exists and is genuinely the vendored
	// canonical recorder (banner-stamped "DO NOT EDIT"), not an empty/stub
	// file — grepping for the exact do-not-edit banner text confirms the
	// real vendoring path ran (vendorRecorder), not a placeholder write.
	recorderPath := filepath.Join(domainDir, "spec", "hotamspec", "hotamspec.go")
	recorderData, err := os.ReadFile(recorderPath)
	if err != nil {
		t.Fatalf("spec/hotamspec/hotamspec.go not created: %v", err)
	}
	recorderBody := string(recorderData)
	if !strings.Contains(recorderBody, "DO NOT EDIT") {
		t.Errorf("spec/hotamspec/hotamspec.go missing the vendored do-not-edit banner — not a genuine vendored copy:\n%s", firstLines(recorderBody, 5))
	}
	if !strings.Contains(recorderBody, "package hotamspec") {
		t.Errorf("spec/hotamspec/hotamspec.go missing \"package hotamspec\" — not a real Go source file:\n%s", firstLines(recorderBody, 5))
	}
	if len(recorderBody) < 500 {
		t.Errorf("spec/hotamspec/hotamspec.go is suspiciously short (%d bytes) — expected a real vendored recorder, not a stub", len(recorderBody))
	}

	// (3) THE decisive proof: a REAL `hotam all-violations` run against the
	// freshly-scaffolded domain — born under discipline:full, born with
	// parent declared — is 0 violations, with ZERO manual follow-up beyond
	// the single init-project call above. This is the "no migration window"
	// promise made concrete.
	avOut, err := runAt(workDir, "all-violations", "--domain", domainDir)
	if err != nil {
		t.Fatalf("all-violations against freshly-init-project'd domain failed: %v\nOUTPUT:\n%s", err, avOut)
	}
	if !strings.Contains(avOut, "0 violations") {
		t.Errorf("freshly-init-project'd domain (discipline:full from birth) is not clean:\n%s", avOut)
	}
}

// firstLines returns at most n lines of s, for compact failure messages
// against a potentially large file body.
func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
