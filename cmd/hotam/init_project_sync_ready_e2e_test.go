package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestExternal_InitProjectSyncReady is the end-to-end proof of the ZERO
// manual steps contract between init-project and the first sync-domain:
// after `hotam init-project` alone (no hand-written spec/stakeholders.go, no
// hand-added manifest atom_defaults, no re-run of vendor-ontology or
// scaffold-registrydump), authoring one atom in spec/model and running the
// standard two-command projection must succeed — the first dry-run is "all
// clear" (check_no_dangling_requirement_owner never fires), the confirmed
// run lands 1 requirement + 1 stakeholder into the graph, and
// all-violations stays clean. Modeled on TestExternal_InitProjectBornObligated.
func TestExternal_InitProjectSyncReady(t *testing.T) {
	if testing.Short() {
		t.Skip("external e2e: builds a real binary + spawns child processes; skipped in -short")
	}

	repoRoot := repoRootForTest(t)
	binPath := buildSharedHotamBinary(t)
	if isInsideForTest(filepath.Dir(binPath), repoRoot) {
		t.Fatalf("test invariant broken: binary built inside repo root")
	}

	workDir, err := os.MkdirTemp("", "hotam-initproject-syncready-")
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

	projDir := filepath.Join(workDir, "syncproject")
	domainDir := filepath.Join(projDir, "domains", "main")
	specDir := filepath.Join(domainDir, "spec")

	// (1) init-project with DEFAULTS only — the whole point is that no
	// follow-up command or hand-edit is needed afterward.
	today := time.Now().Format("2006-01-02")
	out, err := runAt(workDir, "init-project", projDir, "--today", today)
	if err != nil {
		t.Fatalf("init-project failed: %v\nOUTPUT:\n%s", err, out)
	}

	// (2) Author the first atom (QUICKSTART-CONSUMER Part 1 step 3 verbatim).
	modelDir := filepath.Join(specDir, "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("mkdir spec/model: %v", err)
	}
	humanGo := `package model

type BirthYear int

type Human struct{ Born BirthYear }

func Init() Human { return Human{Born: 1987} }

// Birth year
func (h Human) BirthYear() BirthYear { return h.Born }
`
	humanTestGo := `package model

import (
	"testing"

	"main-spec/hotamspec"
)

func TestBirthYear(t *testing.T) {
	hotamspec.Fact(t, Init().BirthYear, BirthYear(1987))
}
`
	for name, src := range map[string]string{"human.go": humanGo, "human_test.go": humanTestGo} {
		if err := os.WriteFile(filepath.Join(modelDir, name), []byte(src), 0o644); err != nil {
			t.Fatalf("write spec/model/%s: %v", name, err)
		}
	}

	// (3) The scaffolded spec/ module (including the seeded stakeholders.go)
	// must compile and pass as-is.
	testCmd := exec.Command("go", "test", "./...")
	testCmd.Dir = specDir
	testCmd.Env = clearedEnv
	if testOut, err := testCmd.CombinedOutput(); err != nil {
		t.Fatalf("go test ./... in scaffolded spec/ failed: %v\n%s", err, testOut)
	}

	// (4) First sync-domain dry-run: must be all-clear — the seeded owner
	// (stakeholders.go + atom_defaults + envelope registrydump) means
	// check_no_dangling_requirement_owner never fires. Capture the diff-hash
	// for the confirm step.
	dryOut, err := runAt(workDir, "sync-domain", "--domain", domainDir)
	if err != nil {
		t.Fatalf("first sync-domain (dry run) failed: %v\nOUTPUT:\n%s", err, dryOut)
	}
	if !strings.Contains(dryOut, "DRY RUN") {
		t.Errorf("sync-domain output missing DRY RUN marker:\n%s", dryOut)
	}
	if !strings.Contains(dryOut, "[ADDED] R-human-birth-year") {
		t.Errorf("sync-domain dry run missing [ADDED] R-human-birth-year:\n%s", dryOut)
	}
	// Gate preview is all-clear (each gate prints "clear"; the gate that
	// would otherwise fire is named check_no_dangling_requirement_owner).
	if !strings.Contains(dryOut, "[8 pre/post-violations] clear") {
		t.Errorf("sync-domain dry run gate preview not clear (must pass with zero manual steps):\n%s", dryOut)
	}
	if strings.Contains(dryOut, "check_no_dangling_requirement_owner") {
		t.Errorf("check_no_dangling_requirement_owner fired despite init-project's seeded owner:\n%s", dryOut)
	}
	diffHashRe := regexp.MustCompile(`(?m)^diff-hash: ([0-9a-f]+)$`)
	m := diffHashRe.FindStringSubmatch(dryOut)
	if m == nil {
		t.Fatalf("sync-domain dry run printed no diff-hash line:\n%s", dryOut)
	}
	diffHash := m[1]

	// (5) Confirm with the hash: 1 added requirement, 1 stakeholder added,
	// 0 violations.
	confirmOut, err := runAt(workDir, "sync-domain", "--domain", domainDir, "--today", today, "--confirm-hash", diffHash)
	if err != nil {
		t.Fatalf("sync-domain --confirm-hash failed: %v\nOUTPUT:\n%s", err, confirmOut)
	}
	if !strings.Contains(confirmOut, "1 added requirement(s), 1 stakeholder(s) added") {
		t.Errorf("confirmed sync-domain must add 1 requirement + 1 stakeholder:\n%s", confirmOut)
	}
	if !strings.Contains(confirmOut, "0 violations") {
		t.Errorf("confirmed sync-domain must report 0 violations:\n%s", confirmOut)
	}

	// (6) Post-landing health check stays clean.
	avOut, err := runAt(workDir, "all-violations", "--domain", domainDir)
	if err != nil {
		t.Fatalf("all-violations after sync failed: %v\nOUTPUT:\n%s", err, avOut)
	}
	if !strings.Contains(avOut, "0 violations") {
		t.Errorf("all-violations after sync must report 0 violations:\n%s", avOut)
	}
}
