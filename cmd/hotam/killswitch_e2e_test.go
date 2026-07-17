package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

// hashModuleInputsForTest reproduces internal/gate's UNEXPORTED
// hashPackageInputs bit-for-bit (same field order, same SHA-256 write
// sequence: "go.mod\n"+contents, then "go.sum\n"+contents if present, then
// every *.go file under moduleRoot -- skipping .git/vendor -- in SORTED
// relative-path order, each written as "<relpath>\n"+contents) so this test
// can compute the EXACT on-disk cache path an attacker would compute, purely
// OFFLINE from source files, without ever running `hotam` or importing the
// unexported function from a different package. This is deliberately a
// SEPARATE, independent reimplementation (not a call into internal/gate) --
// the whole point of the forge attack this test reproduces is that the
// attacker has NO privileged access to the engine's internals, only its
// published algorithm (readable from this very source file), so proving the
// attack requires an independent computation of the same hash, not a
// shortcut through the package under test.
func hashModuleInputsForTest(t *testing.T, moduleRoot string) string {
	t.Helper()
	h := sha256.New()
	for _, modFile := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(moduleRoot, modFile))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatalf("read %s: %v", modFile, err)
		}
		h.Write([]byte(modFile + "\n"))
		h.Write(data)
	}
	var relPaths []string
	fileData := map[string][]byte{}
	walkErr := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		rel, relErr := filepath.Rel(moduleRoot, path)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		relPaths = append(relPaths, relSlash)
		fileData[relSlash] = data
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", moduleRoot, walkErr)
	}
	sort.Strings(relPaths)
	for _, rel := range relPaths {
		h.Write([]byte(rel + "\n"))
		h.Write(fileData[rel])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TestKillswitch_ForgedVerdictCacheFile_DoesNotSilenceRedDomain is the REAL
// oracle for @fh's ROOT finding (severity HIGH, the one that made the
// marker-file and env-var kill-switches above look tame by comparison): the
// cache key (hash of every *.go file + go.mod/go.sum under the owning
// module, computed by hashPackageInputs) was a PURE FUNCTION of files
// already on disk -- an attacker never needed to run `hotam`, guess a nonce,
// or race a marker file. They could compute the EXACT verdict-cache path
// OFFLINE from source alone and hand-write a fabricated PASS verdict to it
// BEFORE ever invoking `hotam all-violations` against a genuinely red
// domain, and the (now-removed) loadDiskCache hit would read the forged
// verdict back as if a real `go test` had produced it -- worse than every
// env-var-shaped kill-switch above, because the forged artifact carried the
// VERDICT ITSELF, not merely a "skip real proof" signal.
//
// This test computes the REAL path a real attacker would compute (via
// hashModuleInputsForTest above, an independent reimplementation of the
// exact algorithm, not a shortcut through internal/gate), writes a forged
// {"passed":true} verdict there BEFORE gutting the implementation and
// BEFORE ever invoking hotam, and asserts the forge has ZERO effect: since
// the disk verdict cache (diskCacheDir/loadDiskCache/storeDiskCache) has
// been REMOVED entirely (NEW-3, gate/test_exec.go), there is no code path
// left that would ever read this file back, no matter how precisely its
// name matches what the old mechanism would have computed.
func TestKillswitch_ForgedVerdictCacheFile_DoesNotSilenceRedDomain(t *testing.T) {
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

	// The attacker's offline reconnaissance: the fixture domain's own go.mod
	// module root is domainDir (killswitchFixtureDomain writes "module
	// prat-spec" there) -- compute the EXACT hash the (now-removed)
	// hashPackageInputs would have computed for this module's CURRENT
	// (still-passing) source, before any mutation.
	moduleHash := hashModuleInputsForTest(t, domainDir)
	testName := "TestNewRisk_RejectsMissingOwner"
	legacyCacheDir := filepath.Join(os.TempDir(), "hotam-verified-by-cache")
	forgedVerdictPath := filepath.Join(legacyCacheDir, moduleHash+"__"+testName+".json")

	if err := os.MkdirAll(legacyCacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll legacy cache dir: %v", err)
	}
	forgedPayload, err := json.Marshal(map[string]any{
		"passed":         true,
		"compile_failed": false,
		"output":         "PASS (forged -- this process never ran go test)",
	})
	if err != nil {
		t.Fatalf("marshal forged payload: %v", err)
	}
	if err := os.WriteFile(forgedVerdictPath, forgedPayload, 0o644); err != nil {
		t.Fatalf("write forged verdict file: %v", err)
	}
	t.Cleanup(func() { os.Remove(forgedVerdictPath) })

	// NOW flip the domain red -- exactly @fh's Probe C shape -- AFTER the
	// forged verdict is already in place, mirroring the real attack
	// timeline: forge first (computable purely offline, no need to observe
	// any hotam run at all), gut the implementation second, then invoke
	// all-violations expecting the forged "clean" verdict to be believed.
	if err := os.WriteFile(implPath, []byte(guttedRiskImplSrc), 0o644); err != nil {
		t.Fatalf("write gutted risk.go: %v", err)
	}

	cmd := exec.Command(binPath, "all-violations", "--domain", domainDir)
	cmd.Dir = workDir
	cmd.Env = clearedEnv // no env-var attack layered in -- this is the disk-cache forge in isolation.
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("KILL-SWITCH NOT CLOSED: a forged on-disk verdict file at the real computed cache path silenced a genuinely red domain (exited 0):\n%s", out)
	}
	if strings.Contains(string(out), "0 violations") {
		t.Fatalf("KILL-SWITCH NOT CLOSED: forged verdict cache file produced a silent \"0 violations\" on a red domain:\n%s", out)
	}
	if !strings.Contains(string(out), "R-risk-owner-required") {
		t.Errorf("expected the violation to name R-risk-owner-required, got:\n%s", out)
	}
}

// TestKillswitch_CleanRun_NeverCreatesOnDiskVerdictStore proves the positive
// half of NEW-3's fix: a completely NORMAL, non-adversarial all-violations
// run against a genuinely clean domain must never create ANY file or
// directory under os.TempDir()/hotam-verified-by-cache -- there is no
// world-writable verdict store left to populate, forge, or race, because the
// mechanism that used to write one (storeDiskCache) has been deleted
// outright rather than hardened.
func TestKillswitch_CleanRun_NeverCreatesOnDiskVerdictStore(t *testing.T) {
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

	// An ISOLATED TMPDIR for this test's own subprocess, so this assertion
	// ("no world-writable verdict store gets created") is not confused by
	// unrelated files any other concurrently-running test/process may have
	// left under the machine's real shared os.TempDir().
	isolatedTemp := filepath.Join(workDir, "isolated-tmp")
	if err := os.MkdirAll(isolatedTemp, 0o755); err != nil {
		t.Fatalf("MkdirAll isolated temp dir: %v", err)
	}

	clearedEnv := filteredEnv(t, "HOTAM_SPEC_PROJECT_ROOT", "HOTAM_SPEC_DOMAINS_ROOT", "HOTAM_VERIFIED_BY_EXEC_GUARD", "TMPDIR", "TMP", "TEMP")
	envWithIsolatedTemp := append(append([]string{}, clearedEnv...),
		"TMPDIR="+isolatedTemp, "TMP="+isolatedTemp, "TEMP="+isolatedTemp)

	domainDir, _ := killswitchFixtureDomain(t, binPath, workDir, envWithIsolatedTemp)
	// implementation left untouched -- genuinely clean, ordinary run.

	cmd := exec.Command(binPath, "all-violations", "--domain", domainDir)
	cmd.Dir = workDir
	cmd.Env = envWithIsolatedTemp
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("all-violations on a genuinely clean domain failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "0 violations") {
		t.Fatalf("expected 0 violations on a genuinely clean domain, got:\n%s", out)
	}

	legacyCacheDir := filepath.Join(isolatedTemp, "hotam-verified-by-cache")
	if _, statErr := os.Stat(legacyCacheDir); statErr == nil {
		entries, _ := os.ReadDir(legacyCacheDir)
		t.Fatalf("WORLD-WRITABLE VERDICT STORE RECREATED: %s exists after a normal clean run (entries: %v) -- the removed disk cache must never be written again", legacyCacheDir, entries)
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("unexpected error stat'ing %s: %v", legacyCacheDir, statErr)
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
