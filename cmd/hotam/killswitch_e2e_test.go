package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This file reproduces @fh's second adversarial re-review of NEW-1
// end-to-end, through the REAL compiled `hotam` binary (never in-process),
// exactly the way an external attacker would invoke it:
//
//	HOTAM_VERIFIED_BY_EXEC_GUARD=<anything> hotam all-violations --domain <red-domain>
//
// The FIRST fix (marker-vouched-nonce, internal/gate's guardMarkerPath under
// os.TempDir()) was itself broken: the marker lived at a predictable,
// world-writable path, so an attacker could pick their own guard value,
// self-create the matching marker file, and the kill-switch worked anyway.
// The fix under test here is root-cause: cmd/hotam's main()
// unconditionally clears HOTAM_VERIFIED_BY_EXEC_GUARD from its own process
// environment before any subcommand dispatch (see main.go's
// clearInheritedVerifiedByGuard), so no value an external actor exports can
// ever survive to reach internal/gate.RunVerifiedByTest's guard check at a
// top-level `hotam` process's own call graph root.

// killswitchFixtureDomain builds a real, on-disk Hotam-Spec domain (a go.mod
// module at domainDir, one ENFORCED Requirement whose implemented_by points
// at spec/model/risk.go:NewRisk and whose verified_by points at
// spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner) via the REAL
// hotam binary's own init/apply-proposal commands -- not by hand-writing
// graph.json -- so this test exercises the exact same code path a real
// adopter would. Returns the domain directory and the path to risk.go (the
// implementation file the caller mutates to flip the domain red).
func killswitchFixtureDomain(t *testing.T, binPath, workDir string, env []string) (domainDir, implPath string) {
	t.Helper()

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(binPath, args...)
		cmd.Dir = workDir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("hotam %s failed: %v\nOUTPUT:\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}

	domainDir = filepath.Join(workDir, "ks-domain")
	run("init", domainDir, "--name", "ks")

	// A real Go module under the domain dir, exactly what
	// PLAN-authored-spec-discipline.md's "prat-spec" convention and
	// internal/invariants' own authored_links_test.go fixtures use, so
	// gate.RunVerifiedByTest can actually `go test` it.
	if err := os.WriteFile(filepath.Join(domainDir, "go.mod"), []byte("module prat-spec\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	modelDir := filepath.Join(domainDir, "spec", "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("mkdir spec/model: %v", err)
	}
	implPath = filepath.Join(modelDir, "risk.go")
	// GENUINELY passing implementation to start: NewRisk really rejects a
	// missing owner.
	const passingImpl = `package model

import "errors"

type Risk struct {
	Owner string
}

func NewRisk(owner string) (*Risk, error) {
	if owner == "" {
		return nil, errors.New("owner is required")
	}
	return &Risk{Owner: owner}, nil
}
`
	if err := os.WriteFile(implPath, []byte(passingImpl), 0o644); err != nil {
		t.Fatalf("write risk.go: %v", err)
	}
	const testSrc = `package model

import "testing"

func TestNewRisk_RejectsMissingOwner(t *testing.T) {
	r, err := NewRisk("")
	if err == nil {
		t.Fatalf("expected error for missing owner, got risk=%v", r)
	}
}
`
	if err := os.WriteFile(filepath.Join(modelDir, "risk_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatalf("write risk_test.go: %v", err)
	}

	reqProposal := filepath.Join(workDir, "r-risk.json")
	writeJSONFile(t, reqProposal, map[string]any{
		"kind":           "Requirement",
		"id":             "R-risk-owner-required",
		"claim":          "Risk creation MUST reject a missing owner.",
		"owner":          "owner",
		"status":         "SETTLED",
		"why":            "kill-switch e2e fixture",
		"enforcement":    "ENFORCED",
		"enforceability": "ENFORCEABLE",
		"implemented_by": []string{"spec/model/risk.go:NewRisk"},
		"verified_by":    []string{"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"},
	})
	landOut := run("land", reqProposal, "--domain", domainDir, "--today", "2026-07-12")
	if !strings.Contains(landOut, "0 violations") {
		t.Fatalf("fixture landing was not clean:\n%s", landOut)
	}

	return domainDir, implPath
}

// guttedRiskImplSrc is the SAME shape as @fh's Probe C / F1: an
// implementation rewritten to unconditionally succeed, which flips
// TestNewRisk_RejectsMissingOwner from PASS to FAIL when actually executed,
// while every AST-only structural check stays green (the symbol still
// resolves, the test still has a real assertion, nothing is skipped).
const guttedRiskImplSrc = `package model

type Risk struct {
	Owner string
}

func NewRisk(owner string) (*Risk, error) {
	return &Risk{Owner: owner}, nil
}
`

// TestKillswitch_ExternalGuardEnv_DoesNotSilenceRedDomain reproduces @fh's
// literal kill-switch shape: an external actor exports
// HOTAM_VERIFIED_BY_EXEC_GUARD (to an arbitrary value -- "1", exactly as
// originally reported) BEFORE invoking `hotam all-violations` directly
// against a domain whose verified_by test is genuinely red. Pre-fix (the
// ORIGINAL guard, trusting mere env presence) this made the TOP-level
// process believe itself already nested, Skip every verified_by check, and
// report "0 violations -- graph clean" on a gutted tree. Post-fix,
// cmd/hotam's main() unsets the inherited env before all-violations ever
// runs, so RunVerifiedByTest actually executes the test and the violation
// is reported.
func TestKillswitch_ExternalGuardEnv_DoesNotSilenceRedDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("kill-switch e2e: builds/spawns real binaries; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	workDir, err := os.MkdirTemp("", "hotam-ks-work-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(workDir) })

	clearedEnv := filteredEnv(t, "HOTAM_SPEC_PROJECT_ROOT", "HOTAM_SPEC_DOMAINS_ROOT", "HOTAM_VERIFIED_BY_EXEC_GUARD")
	domainDir, implPath := killswitchFixtureDomain(t, binPath, workDir, clearedEnv)

	// Flip the domain red: gut the implementation exactly like @fh's Probe C.
	if err := os.WriteFile(implPath, []byte(guttedRiskImplSrc), 0o644); err != nil {
		t.Fatalf("write gutted risk.go: %v", err)
	}

	// The attack: export the guard env to an ARBITRARY attacker-chosen value
	// ("1", the literal original report shape) and invoke all-violations on
	// the now-red domain directly -- no ancestor `hotam` process, no genuine
	// RunVerifiedByTest-spawned lineage.
	attackEnv := append(append([]string{}, clearedEnv...), "HOTAM_VERIFIED_BY_EXEC_GUARD=1")
	cmd := exec.Command(binPath, "all-violations", "--domain", domainDir)
	cmd.Dir = workDir
	cmd.Env = attackEnv
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("KILL-SWITCH NOT CLOSED: `HOTAM_VERIFIED_BY_EXEC_GUARD=1 hotam all-violations` exited 0 (silent clean) on a genuinely red domain:\n%s", out)
	}
	if strings.Contains(string(out), "0 violations") {
		t.Fatalf("KILL-SWITCH NOT CLOSED: attacker-set guard env produced a silent \"0 violations\" on a red domain:\n%s", out)
	}
	if !strings.Contains(string(out), "R-risk-owner-required") {
		t.Errorf("expected the violation to name R-risk-owner-required, got:\n%s", out)
	}
}

// TestKillswitch_ForgedMarkerFile_DoesNotRestoreKillSwitch reproduces @fh's
// SECOND finding against the FIRST (now-removed) fix: even after the
// marker-vouched-nonce scheme existed, an attacker could defeat it by
// picking their OWN guard value, self-creating a matching marker file at the
// scheme's predictable, world-writable path
// (os.TempDir()/hotam-verified-by-cache/guard-<value>.marker), and exporting
// that SAME value before invoking hotam directly. This test proves that
// attempting exactly that shape TODAY -- against the current mechanism,
// which no longer even has a marker-file concept -- still does not silence a
// red domain: creating an arbitrary file at the legacy predictable path
// (best-effort, the mechanism may not even read it any more) plus exporting
// its "value" as the guard env must still result in a reported violation,
// never a silent clean.
func TestKillswitch_ForgedMarkerFile_DoesNotRestoreKillSwitch(t *testing.T) {
	if testing.Short() {
		t.Skip("kill-switch e2e: builds/spawns real binaries; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	workDir, err := os.MkdirTemp("", "hotam-ks-work-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(workDir) })

	clearedEnv := filteredEnv(t, "HOTAM_SPEC_PROJECT_ROOT", "HOTAM_SPEC_DOMAINS_ROOT", "HOTAM_VERIFIED_BY_EXEC_GUARD")
	domainDir, implPath := killswitchFixtureDomain(t, binPath, workDir, clearedEnv)

	if err := os.WriteFile(implPath, []byte(guttedRiskImplSrc), 0o644); err != nil {
		t.Fatalf("write gutted risk.go: %v", err)
	}

	// Attacker picks their own forged value and self-creates the legacy
	// marker file the FIRST (removed) fix used to consult, at the exact
	// predictable, world-writable path it lived at.
	forged := "attacker-chosen-forged-value-12345"
	legacyMarkerDir := filepath.Join(os.TempDir(), "hotam-verified-by-cache")
	if err := os.MkdirAll(legacyMarkerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll legacy marker dir: %v", err)
	}
	legacyMarkerPath := filepath.Join(legacyMarkerDir, "guard-"+forged+".marker")
	if err := os.WriteFile(legacyMarkerPath, []byte{}, 0o644); err != nil {
		t.Fatalf("write forged marker file: %v", err)
	}
	t.Cleanup(func() { os.Remove(legacyMarkerPath) })

	attackEnv := append(append([]string{}, clearedEnv...), "HOTAM_VERIFIED_BY_EXEC_GUARD="+forged)
	cmd := exec.Command(binPath, "all-violations", "--domain", domainDir)
	cmd.Dir = workDir
	cmd.Env = attackEnv
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("KILL-SWITCH NOT CLOSED: forged marker file + matching guard env exited 0 (silent clean) on a red domain:\n%s", out)
	}
	if strings.Contains(string(out), "0 violations") {
		t.Fatalf("KILL-SWITCH NOT CLOSED: forged marker file restored a silent \"0 violations\" on a red domain:\n%s", out)
	}
	if !strings.Contains(string(out), "R-risk-owner-required") {
		t.Errorf("expected the violation to name R-risk-owner-required, got:\n%s", out)
	}
}

// TestKillswitch_PassiveReplay_DoesNotSilenceRedDomain reproduces @fh's third
// shape: a value in the FORM of a legitimate nonce (a long random-looking
// hex string, as a genuine per-process nonce would look, rather than an
// obviously-attacker-chosen literal like "1") is exported before invoking
// hotam directly -- e.g. one an attacker observed lying around from a past
// legitimate run, or simply guessed the SHAPE of without ever having been
// vouched for by any real RunVerifiedByTest-spawned lineage. This must not
// gain anything over the naive "1" shape: cmd/hotam's main() clears
// HOTAM_VERIFIED_BY_EXEC_GUARD unconditionally regardless of what the
// inherited value looks like, so a nonce-shaped replay is exactly as
// powerless as a trivial one.
func TestKillswitch_PassiveReplay_DoesNotSilenceRedDomain(t *testing.T) {
	if testing.Short() {
		t.Skip("kill-switch e2e: builds/spawns real binaries; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	workDir, err := os.MkdirTemp("", "hotam-ks-work-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(workDir) })

	clearedEnv := filteredEnv(t, "HOTAM_SPEC_PROJECT_ROOT", "HOTAM_SPEC_DOMAINS_ROOT", "HOTAM_VERIFIED_BY_EXEC_GUARD")
	domainDir, implPath := killswitchFixtureDomain(t, binPath, workDir, clearedEnv)

	if err := os.WriteFile(implPath, []byte(guttedRiskImplSrc), 0o644); err != nil {
		t.Fatalf("write gutted risk.go: %v", err)
	}

	// A nonce-SHAPED replay value (64 hex chars, exactly what
	// newGuardNonce's 32-byte crypto/rand token renders as) -- simulating a
	// value an attacker observed from some past legitimate run and is now
	// replaying passively, with no fresh vouching of any kind behind it.
	replayed := strings.Repeat("ab", 32)
	attackEnv := append(append([]string{}, clearedEnv...), "HOTAM_VERIFIED_BY_EXEC_GUARD="+replayed)
	cmd := exec.Command(binPath, "all-violations", "--domain", domainDir)
	cmd.Dir = workDir
	cmd.Env = attackEnv
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("KILL-SWITCH NOT CLOSED: passively-replayed nonce-shaped guard env exited 0 (silent clean) on a red domain:\n%s", out)
	}
	if strings.Contains(string(out), "0 violations") {
		t.Fatalf("KILL-SWITCH NOT CLOSED: passively-replayed nonce-shaped guard env produced a silent \"0 violations\" on a red domain:\n%s", out)
	}
	if !strings.Contains(string(out), "R-risk-owner-required") {
		t.Errorf("expected the violation to name R-risk-owner-required, got:\n%s", out)
	}
}

// TestKillswitch_CleanDomain_StillZeroViolationsWithGuardEnvSet is the
// non-regression control: with the SAME external guard env set, a domain
// that is ACTUALLY clean (implementation never gutted) must still report 0
// violations -- the fix must not turn every all-violations run red
// regardless of content; it must only stop the guard env from SILENCING a
// genuinely red domain.
func TestKillswitch_CleanDomain_StillZeroViolationsWithGuardEnvSet(t *testing.T) {
	if testing.Short() {
		t.Skip("kill-switch e2e: builds/spawns real binaries; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	workDir, err := os.MkdirTemp("", "hotam-ks-work-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(workDir) })

	clearedEnv := filteredEnv(t, "HOTAM_SPEC_PROJECT_ROOT", "HOTAM_SPEC_DOMAINS_ROOT", "HOTAM_VERIFIED_BY_EXEC_GUARD")
	domainDir, _ := killswitchFixtureDomain(t, binPath, workDir, clearedEnv)
	// implementation left untouched -- genuinely clean.

	attackEnv := append(append([]string{}, clearedEnv...), "HOTAM_VERIFIED_BY_EXEC_GUARD=whatever-someone-set")
	cmd := exec.Command(binPath, "all-violations", "--domain", domainDir)
	cmd.Dir = workDir
	cmd.Env = attackEnv
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("all-violations on a genuinely clean domain failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "0 violations") {
		t.Fatalf("expected 0 violations on a genuinely clean domain (real execution, guard env must not force a false violation either), got:\n%s", out)
	}
}
