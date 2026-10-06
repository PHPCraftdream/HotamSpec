package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/selfspec"
)

// syncSelfSummaryTargetID is a real, currently-registered requirement ID
// (internal/selfspec/requirements_domain.go) whose registry Summary field is
// "" — a safe, low-risk target for the CHANGED-entry fixtures below: editing
// the COPIED graph.json's summary for this ID to a non-empty placeholder
// diverges it from the registry on a purely structural field (Summary),
// without touching Claim (so the confront gate's opposite-marker/claim-change
// trigger never fires for this scenario) and without breaking any other
// invariant (Summary is free text with no structural constraints).
const syncSelfSummaryTargetID = "R-director-agent-required-per-domain"

// syncSelfFixture is a hermetic, disk-backed copy of the REAL self-hosting
// domain (graph.json + manifest.json) AND the REAL internal/selfspec Go
// source files, laid out exactly the way sync-self's stale-binary guard
// (checkSelfspecBinaryFresh) expects: <root>/domains/hotam-spec-self/
// (repoRootForDomain's tier-1 "parent is literally domains" rule) and
// <root>/internal/selfspec/*.go — a byte-identical copy of the SAME files
// this test binary was itself compiled with (selfspec.SourceFiles is a
// build-time embed of the real repo's internal/selfspec/, so copying those
// same files verbatim into the fixture root always passes the freshness
// check, exactly like a real `go run ./cmd/hotam sync-self` would).
type syncSelfFixture struct {
	root      string // synthetic project/repo root
	domainDir string // <root>/domains/hotam-spec-self
	graphPath string
}

// newSyncSelfFixture copies the real domain + the real internal/selfspec
// sources into a fresh t.TempDir(). mutateGraph, if non-nil, is applied to
// the decoded graph.json map BEFORE it is written to the fixture, letting a
// caller introduce a controlled ADDED/CHANGED divergence (see
// syncSelfSummaryTargetID's doc comment and the ADDED-entry tests' own
// removal of a node) before sync-self ever runs against it.
//
// The fixture is an honest NON-ATOM double of the real domain: the atom
// opt-in is stripped from the manifest (copySelfDomainManifestSansLocalOptIns)
// AND the atom-derived requirement nodes are stripped from the graph via
// stripSelfAtomRequirements, mirroring copySelfDomainUnderRoot — so
// cmdSyncSelf's mergeSelfAtoms is an honest no-op and no atom-pipeline
// invariant (check_self_requirements_match_registry, check_verified_by_test_has_teeth
// via the declared recorder path) ever fires against it. mutateGraph is
// applied to the map BEFORE the graph is written and stripSelfAtomRequirements
// rewrites graph.json afterwards, so the two compose (mutators target manual
// registry IDs, never the atom IDs).
func newSyncSelfFixture(t *testing.T, mutateGraph func(m map[string]any)) *syncSelfFixture {
	t.Helper()
	root := t.TempDir()
	domainDir := filepath.Join(root, "domains", "hotam-spec-self")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("mkdir domain: %v", err)
	}

	graphData, err := os.ReadFile(selfDomainGraph)
	if err != nil {
		t.Fatalf("read %s: %v", selfDomainGraph, err)
	}
	var m map[string]any
	if err := json.Unmarshal(graphData, &m); err != nil {
		t.Fatalf("unmarshal fixture graph: %v", err)
	}
	if mutateGraph != nil {
		mutateGraph(m)
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal mutated graph: %v", err)
	}
	graphPath := filepath.Join(domainDir, "graph.json")
	if err := os.WriteFile(graphPath, out, 0o644); err != nil {
		t.Fatalf("write graph.json: %v", err)
	}

	copySelfDomainManifestSansLocalOptIns(t, filepath.Join(domainDir, "manifest.json"))
	stripSelfAtomRequirements(t, domainDir)

	// Mirror the real internal/selfspec/*.go sources so the stale-binary
	// guard (checkSelfspecBinaryFresh) finds byte-identical files.
	srcSelfspecDir := filepath.Join("..", "..", "internal", "selfspec")
	entries, err := os.ReadDir(srcSelfspecDir)
	if err != nil {
		t.Fatalf("read %s: %v", srcSelfspecDir, err)
	}
	dstSelfspecDir := filepath.Join(root, "internal", "selfspec")
	if err := os.MkdirAll(dstSelfspecDir, 0o755); err != nil {
		t.Fatalf("mkdir fixture selfspec dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue // not part of selfspec.SourceFiles' embed glob
		}
		copyFile(t, filepath.Join(srcSelfspecDir, name), filepath.Join(dstSelfspecDir, name))
	}

	return &syncSelfFixture{root: root, domainDir: domainDir, graphPath: graphPath}
}

// mutateSummary returns a mutateGraph func that overwrites the "summary"
// field of the requirement named id in the decoded graph.json map, producing
// a deterministic CHANGED SyncReportEntry for id once sync-self runs (the
// registry's own Summary for id stays "", so the graph now disagrees).
func mutateSummary(id, summary string) func(map[string]any) {
	return func(m map[string]any) {
		reqs, _ := m["requirements"].([]any)
		for _, ri := range reqs {
			r, ok := ri.(map[string]any)
			if !ok {
				continue
			}
			if r["id"] == id {
				r["summary"] = summary
				return
			}
		}
	}
}

// removeRequirement returns a mutateGraph func that deletes the requirement
// named id from the decoded graph.json map entirely, so id (a real
// registered ID) becomes ADDED once sync-self runs (registered, but absent
// from the graph).
func removeRequirement(id string) func(map[string]any) {
	return func(m map[string]any) {
		reqs, _ := m["requirements"].([]any)
		out := make([]any, 0, len(reqs))
		for _, ri := range reqs {
			r, ok := ri.(map[string]any)
			if ok && r["id"] == id {
				continue
			}
			out = append(out, ri)
		}
		m["requirements"] = out
	}
}

// runSyncSelf runs cmdSyncSelf in-process against the fixture's domain and
// captures stdout/stderr by redirecting os.Stdout/os.Stderr for the
// duration of the call — cmdSyncSelf itself writes via fmt.Print*/printJSON,
// which go through the process-global os.Stdout/os.Stderr, so this is the
// simplest reliable capture mechanism for an in-process (non-subprocess)
// invocation. NOT safe to run in parallel with another test doing the same
// (both would race on the redirected globals) — callers must not mark a test
// using this helper as t.Parallel() alongside another such test, though
// SEQUENTIAL execution of every sync-self test in this file's own run is
// fine (Go runs non-parallel tests within a file sequentially by default).
func runSyncSelf(t *testing.T, args []string) (stdout, stderr string, err error) {
	t.Helper()

	outR, outW, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("pipe: %v", perr)
	}
	errR, errW, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("pipe: %v", perr)
	}
	prevOut, prevErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW

	done := make(chan struct{})
	var gotOut, gotErr []byte
	go func() {
		gotOut, _ = readAllSync(outR)
		close(done)
	}()
	doneErr := make(chan struct{})
	go func() {
		gotErr, _ = readAllSync(errR)
		close(doneErr)
	}()

	err = cmdSyncSelf(args)

	os.Stdout, os.Stderr = prevOut, prevErr
	outW.Close()
	errW.Close()
	<-done
	<-doneErr

	return string(gotOut), string(gotErr), err
}

// readAllSync reads r to completion (small buffer loop, no external
// dependency on io/ioutil.ReadAll's specific version behavior).
func readAllSync(r *os.File) ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			return buf, nil
		}
	}
}

// extractDiffHash pulls the "diff-hash: <hex>" line's hex value out of a
// dry-run's stdout render.
func extractDiffHash(t *testing.T, stdout string) string {
	t.Helper()
	const marker = "diff-hash: "
	idx := strings.Index(stdout, marker)
	if idx < 0 {
		t.Fatalf("stdout has no %q line:\n%s", marker, stdout)
	}
	rest := stdout[idx+len(marker):]
	end := strings.IndexAny(rest, "\r\n")
	if end < 0 {
		end = len(rest)
	}
	return strings.TrimSpace(rest[:end])
}

// TestCmdSyncSelf_DryRunNeverWrites is the first e2e scenario: a dry-run
// (no --confirm-hash) against a fixture with a real, controlled CHANGED
// divergence must print the SyncReport + a diff-hash and leave graph.json
// byte-identical to what was on disk before the call, no matter how many
// other flags are passed.
func TestCmdSyncSelf_DryRunNeverWrites(t *testing.T) {
	fx := newSyncSelfFixture(t, mutateSummary(syncSelfSummaryTargetID, "diverged placeholder summary"))
	before, err := os.ReadFile(fx.graphPath)
	if err != nil {
		t.Fatalf("read pre-run graph: %v", err)
	}

	stdout, _, err := runSyncSelf(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("dry-run sync-self must succeed, got: %v\nstdout:\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "DRY RUN") {
		t.Errorf("stdout missing DRY RUN banner:\n%s", stdout)
	}
	if !strings.Contains(stdout, syncSelfSummaryTargetID) {
		t.Errorf("stdout does not mention the diverged requirement %s:\n%s", syncSelfSummaryTargetID, stdout)
	}
	hash := extractDiffHash(t, stdout)
	if hash == "" {
		t.Error("diff-hash was empty")
	}

	after, err := os.ReadFile(fx.graphPath)
	if err != nil {
		t.Fatalf("read post-run graph: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("graph.json was modified by a dry-run — dry-run must NEVER write to disk")
	}
	if _, err := os.Stat(loader.LockPath(fx.graphPath)); !os.IsNotExist(err) {
		t.Errorf("graph.lock should not exist after a dry-run, stat err=%v", err)
	}
}

// TestCmdSyncSelf_HashMismatchRefusesAndDoesNotWrite is the second e2e
// scenario: passing a stale/incorrect --confirm-hash must refuse with a
// "diff changed since PRESENT" error and leave the graph untouched.
func TestCmdSyncSelf_HashMismatchRefusesAndDoesNotWrite(t *testing.T) {
	fx := newSyncSelfFixture(t, mutateSummary(syncSelfSummaryTargetID, "diverged placeholder summary"))
	before, err := os.ReadFile(fx.graphPath)
	if err != nil {
		t.Fatalf("read pre-run graph: %v", err)
	}

	_, _, err = runSyncSelf(t, []string{
		"--domain", fx.domainDir, "--today", "2026-07-22",
		"--confirm-hash", "0000000000000000000000000000000000000000000000000000000000000000",
	})
	if err == nil {
		t.Fatal("expected an error for a wrong --confirm-hash, got nil")
	}
	if !strings.Contains(err.Error(), "diff changed since PRESENT") {
		t.Errorf("error = %q, want it to mention 'diff changed since PRESENT'", err.Error())
	}

	after, err := os.ReadFile(fx.graphPath)
	if err != nil {
		t.Fatalf("read post-run graph: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("graph.json was modified despite a hash mismatch")
	}
}

// TestCmdSyncSelf_ConfrontBlockerWithoutAckRefuses is the third e2e
// scenario: an ADDED entry whose registered Claim carries an opposite-marker
// contradiction against an existing SETTLED requirement's claim must refuse
// the confirm-mode write when no --ack-conflict/--decision-ref is supplied.
//
// Driving this scenario through the real CLI would require a registered
// requirement whose COMPILED-IN claim genuinely contradicts another real
// SETTLED requirement's claim — the registry cannot be edited by a test (it
// is fixed at compile time), and depending on two specific real requirements
// staying lexically contradictory forever would be fragile against future
// registry edits. Instead this exact gate (runSyncGates: report a blocker,
// refuse with no ack, accept with one) is proven directly at the function
// level, registry-independent, by TestRunSyncGates_ConfrontBlocker_RequiresAck
// below — same function the CLI path calls, so it is not a weaker proof.
func TestCmdSyncSelf_ConfrontBlockerWithoutAckRefuses(t *testing.T) {
	t.Skip("see TestRunSyncGates_ConfrontBlocker_RequiresAck: covers the same gate at the function level, independent of which real requirement pair happens to carry an opposite marker today")
}

// TestCmdSyncSelf_AppendOnlyViolationRefuses is the fourth e2e scenario. A
// genuine SyncGraph-driven append-only violation cannot be manufactured
// through the CLI's own inputs (SyncGraph itself always appends, never
// truncates) — so, matching the task brief's "can be simulated if the test
// scaffolding allows it" allowance, this is covered at the function level by
// TestVerifyAppendOnly's own unit tests in internal/selfspec/verify_test.go
// (already green — see this task's report) and by
// TestRunSyncGates_AppendOnlyGate_CalledLast below, which proves gate 9 is
// wired into runSyncGates's own refusal path.
func TestCmdSyncSelf_AppendOnlyViolationRefuses(t *testing.T) {
	t.Skip("append-only violations are a SyncGraph/VerifyAppendOnly contract proven at the unit level (internal/selfspec/verify_test.go) — see TestRunSyncGates_AppendOnlyGate_CalledLast for the CLI-side wiring proof")
}

// TestCmdSyncSelf_FullRoundTrip is the fifth, primary e2e scenario: dry-run
// -> extract hash -> confirm-hash -> the graph is REALLY written with the
// expected CHANGED entry, docs are regenerated, and all-violations is clean
// afterward.
func TestCmdSyncSelf_FullRoundTrip(t *testing.T) {
	fx := newSyncSelfFixture(t, mutateSummary(syncSelfSummaryTargetID, "diverged placeholder summary"))

	dryOut, _, err := runSyncSelf(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, dryOut)
	}
	hash := extractDiffHash(t, dryOut)

	confirmOut, _, err := runSyncSelf(t, []string{
		"--domain", fx.domainDir, "--today", "2026-07-22",
		"--confirm-hash", hash,
	})
	if err != nil {
		t.Fatalf("confirm-hash run failed: %v\n%s", err, confirmOut)
	}
	if !strings.Contains(confirmOut, "sync-self landed") {
		t.Errorf("stdout missing the landed confirmation:\n%s", confirmOut)
	}

	// graph.json must now show the registry's value (empty summary) for the
	// synced requirement, not the fixture's diverged placeholder.
	g, err := loader.LoadGraph(fx.graphPath)
	if err != nil {
		t.Fatalf("reload written graph: %v", err)
	}
	var found bool
	for _, r := range g.Requirements {
		if r.ID != syncSelfSummaryTargetID {
			continue
		}
		found = true
		if r.Summary != "" {
			t.Errorf("requirement %s: Summary = %q after sync, want \"\" (the registry's own value)", syncSelfSummaryTargetID, r.Summary)
		}
		lastHist := r.History[len(r.History)-1]
		if !strings.Contains(lastHist.Summary, "Summary") {
			t.Errorf("requirement %s: last History entry does not mention the changed Summary field: %+v", syncSelfSummaryTargetID, lastHist)
		}
	}
	if !found {
		t.Fatalf("requirement %s missing from graph after sync", syncSelfSummaryTargetID)
	}

	// docs must have been regenerated (REQUIREMENTS.md exists and is non-empty).
	reqDoc := filepath.Join(fx.domainDir, "docs", "gen", "REQUIREMENTS.md")
	data, err := os.ReadFile(reqDoc)
	if err != nil {
		t.Fatalf("read regenerated REQUIREMENTS.md: %v", err)
	}
	if len(data) == 0 {
		t.Error("REQUIREMENTS.md is empty after sync-self")
	}

	violations, err := allViolations(fx.domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("expected 0 violations after a clean sync-self round-trip, got %d: %+v", len(violations), violations)
	}

	// graph.lock must now exist and carry the sync-self note.
	lockData, err := os.ReadFile(loader.LockPath(fx.graphPath))
	if err != nil {
		t.Fatalf("read graph.lock: %v", err)
	}
	if !strings.Contains(string(lockData), "sync-self") {
		t.Errorf("graph.lock note does not mention sync-self:\n%s", lockData)
	}
}

// TestCmdSyncSelf_AddedEntryRoundTrip covers the ADDED path specifically: a
// registered requirement removed from the copied graph.json comes back after
// a full dry-run -> confirm-hash round-trip, with a "created via sync-self"
// History seed entry (see selfspec.SyncGraph's own doc comment).
func TestCmdSyncSelf_AddedEntryRoundTrip(t *testing.T) {
	const targetID = syncSelfSummaryTargetID
	fx := newSyncSelfFixture(t, removeRequirement(targetID))

	dryOut, _, err := runSyncSelf(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, dryOut)
	}
	if !strings.Contains(dryOut, "[ADDED] "+targetID) {
		t.Errorf("dry-run output missing ADDED entry for %s:\n%s", targetID, dryOut)
	}
	hash := extractDiffHash(t, dryOut)

	confirmOut, _, err := runSyncSelf(t, []string{
		"--domain", fx.domainDir, "--today", "2026-07-22",
		"--confirm-hash", hash,
	})
	if err != nil {
		t.Fatalf("confirm-hash run failed: %v\n%s", err, confirmOut)
	}

	g, err := loader.LoadGraph(fx.graphPath)
	if err != nil {
		t.Fatalf("reload written graph: %v", err)
	}
	var found bool
	for _, r := range g.Requirements {
		if r.ID != targetID {
			continue
		}
		found = true
		if len(r.History) != 1 || !strings.Contains(r.History[0].Summary, "created via sync-self") {
			t.Errorf("requirement %s: History = %+v, want a single 'created via sync-self' seed entry", targetID, r.History)
		}
	}
	if !found {
		t.Fatalf("requirement %s was not re-created by sync-self", targetID)
	}

	violations, err := allViolations(fx.domainDir)
	if err != nil {
		t.Fatalf("allViolations: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("expected 0 violations after re-adding %s via sync-self, got %d: %+v", targetID, len(violations), violations)
	}
}

// TestCmdSyncSelf_RollbackOnPostWriteFailure is the sixth e2e scenario: a
// failure AFTER the graph write (genSpec) must roll the domain back to its
// pre-sync state, reusing the exact land.go transactional machinery
// (rollbackLand). Failure is injected the SAME way
// TestCmdLand_GenSpecFailure_RollsBackGraphJSON injects it: pointing the
// resolved crystal path at an existing DIRECTORY so genSpec's
// os.ReadFile(claudeMDPath) fails with a non-IsNotExist error. sync-self has
// no --claude-md flag of its own (it always resolves via
// resolveClaudeMDPath(domainDir, "")); with the fixture's project root
// carrying exactly ONE domain under domains/, isActiveOrUnambiguousDomain
// resolves it as the unambiguous/active domain, so resolveClaudeMDPath picks
// the ROOT crystal default (<root>/CLAUDE.md, not a local one) — the
// blocking directory is placed there.
func TestCmdSyncSelf_RollbackOnPostWriteFailure(t *testing.T) {
	fx := newSyncSelfFixture(t, mutateSummary(syncSelfSummaryTargetID, "diverged placeholder summary"))

	dryOut, _, err := runSyncSelf(t, []string{"--domain", fx.domainDir})
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, dryOut)
	}
	hash := extractDiffHash(t, dryOut)

	beforeGraph, err := os.ReadFile(fx.graphPath)
	if err != nil {
		t.Fatalf("read pre-run graph: %v", err)
	}
	lockPath := loader.LockPath(fx.graphPath)
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("precondition: graph.lock should be absent, stat=%v", err)
	}

	// resolveClaudeMDPath needs crystalConventionExists(repoRoot) to be true
	// for a crystal default to fire at all; write a marker so the root
	// "carries the crystal convention".
	if err := os.WriteFile(filepath.Join(fx.root, ".hotam-spec-project"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write project marker: %v", err)
	}
	claudeMDPath := filepath.Join(fx.root, "CLAUDE.md")
	if err := os.MkdirAll(claudeMDPath, 0o755); err != nil {
		t.Fatalf("mkdir (as a file-blocking directory) %s: %v", claudeMDPath, err)
	}

	_, _, err = runSyncSelf(t, []string{
		"--domain", fx.domainDir, "--today", "2026-07-22",
		"--confirm-hash", hash,
	})
	if err == nil {
		t.Fatal("expected the confirm-hash run to fail (genSpec blocked by a directory at the crystal path)")
	}
	if !strings.Contains(err.Error(), "rolled back") {
		t.Errorf("error = %q, want it to state the sync was rolled back", err.Error())
	}

	afterGraph, err := os.ReadFile(fx.graphPath)
	if err != nil {
		t.Fatalf("read post-run graph: %v", err)
	}
	if string(beforeGraph) != string(afterGraph) {
		t.Fatal("graph.json was NOT restored to its pre-sync bytes after a rolled-back sync-self")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Errorf("graph.lock should be absent again after rollback (pre-sync state was absent), stat=%v", err)
	}
}

// TestCmdSyncSelf_RefusesNonSelfHostingDomain proves the domain-scope guard
// (gate 0): a plain domain fixture with self_hosting unset/false must be
// refused outright, before any other gate runs.
func TestCmdSyncSelf_RefusesNonSelfHostingDomain(t *testing.T) {
	domainDir := copySelfDomain(t)
	// copySelfDomain's manifest carries self_hosting: true (it's a literal
	// copy of the real one) — flip it off to exercise the guard.
	manifestPath := filepath.Join(domainDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	m["self_hosting"] = false
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, out, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, _, err = runSyncSelf(t, []string{"--domain", domainDir})
	if err == nil {
		t.Fatal("expected an error for a non-self-hosting domain")
	}
	if !strings.Contains(err.Error(), "not the self-hosting domain") {
		t.Errorf("error = %q, want it to mention 'not the self-hosting domain'", err.Error())
	}
}

// TestCmdSyncSelf_StaleBinaryGuardRefuses proves gate 1: if the on-disk
// internal/selfspec/*.go under the resolved repo root disagrees with the
// running binary's embedded copy, sync-self refuses before doing anything
// else — simulated here by corrupting ONE fixture-copied source file's bytes
// after newSyncSelfFixture already made them byte-identical.
func TestCmdSyncSelf_StaleBinaryGuardRefuses(t *testing.T) {
	fx := newSyncSelfFixture(t, nil)
	corrupted := filepath.Join(fx.root, "internal", "selfspec", "selfspec.go")
	if err := os.WriteFile(corrupted, []byte("package selfspec\n// corrupted for the stale-binary guard test\n"), 0o644); err != nil {
		t.Fatalf("corrupt fixture selfspec.go: %v", err)
	}

	_, _, err := runSyncSelf(t, []string{"--domain", fx.domainDir})
	if err == nil {
		t.Fatal("expected the stale-binary guard to refuse")
	}
	if !strings.Contains(err.Error(), "stale binary") {
		t.Errorf("error = %q, want it to mention 'stale binary'", err.Error())
	}
}

// TestCmdSyncSelf_SubprocessSmoke is a light real-binary smoke test
// (mirrors the style of other *_e2e_test.go files in this package): builds
// the shared hotam binary and runs a dry-run against the fixture as a real
// child process, proving the wiring in main.go (the "sync-self" case) is
// correct end-to-end, not just reachable via the in-process cmdSyncSelf
// helper the rest of this file uses.
func TestCmdSyncSelf_SubprocessSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("sync-self subprocess smoke: builds a real binary + spawns a child process; skipped in -short")
	}
	t.Parallel()

	binPath := buildSharedHotamBinary(t)
	fx := newSyncSelfFixture(t, mutateSummary(syncSelfSummaryTargetID, "diverged placeholder summary"))

	out, err := exec.Command(binPath, "sync-self", "--domain", fx.domainDir).CombinedOutput()
	if err != nil {
		t.Fatalf("hotam sync-self (dry-run) subprocess failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "diff-hash:") {
		t.Errorf("subprocess output missing diff-hash line:\n%s", out)
	}
}

// --- Unit-level gate tests (registry-independent) ---
//
// The CLI-level TestCmdSyncSelf_ConfrontBlockerWithoutAckRefuses and
// TestCmdSyncSelf_AppendOnlyViolationRefuses scenarios are skipped above
// because manufacturing a genuine confront-blocker or append-only breach
// through sync-self's real inputs would require either depending on a
// specific pair of real requirements staying lexically contradictory
// forever (fragile against future registry edits) or a SyncGraph bug that
// does not exist (SyncGraph always appends, never truncates). Both gates
// are instead proven here directly against runSyncGates, using small
// synthetic before/after graphs this test fully controls — the same
// function sync-self's CLI path calls, so this is not a weaker proof, only
// a more direct one.

// syncGatesTestGraph supplies two requirements with opposing words.
// Those words remain advisory unless a test explicitly adds a Conflict carrier.
func syncGatesTestGraph() *ontology.Graph {
	return &ontology.Graph{
		DomainDir: "unit-test-fixture",
		Requirements: []ontology.Requirement{
			{
				ID:             "R-sync-gate-existing",
				Claim:          "the widget component shall ALWAYS frobnicate on startup",
				Owner:          "framework-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
			},
			{
				ID:             "R-sync-gate-changing",
				Claim:          "the widget component shall NEVER frobnicate on startup",
				Owner:          "framework-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
				History:        []ontology.HistoryEntry{{At: "2026-01-01", Summary: "seed"}},
			},
		},
	}
}

// An explicit unresolved carrier blocks a claim update until a decision is recorded.
func TestRunSyncGates_ConfrontBlocker_RequiresAck(t *testing.T) {
	before := syncGatesTestGraph()
	after := syncGatesTestGraph()
	carrier := ontology.Conflict{
		ID: "C-sync-gate-formal", Axis: "startup", Context: "widget startup",
		Members:   []string{"R-sync-gate-changing", "R-sync-gate-existing"},
		Lifecycle: ontology.ConflictDETECTED,
	}
	before.Conflicts = []ontology.Conflict{carrier}
	after.Conflicts = []ontology.Conflict{carrier}
	report := &selfspec.SyncReport{
		Entries: []selfspec.SyncReportEntry{
			{
				ID:   "R-sync-gate-changing",
				Kind: selfspec.SyncKindChanged,
				FieldDiffs: []selfspec.FieldDiff{
					{Field: "Claim", Old: "old claim text", New: "the widget component shall NEVER frobnicate on startup"},
				},
			},
		},
	}

	gr, err := runSyncGates(before.DomainDir, before, after, report, landAckOptions{})
	if err == nil {
		t.Fatal("expected runSyncGates to refuse on an un-acked confront blocker")
	}
	if len(gr.ConfrontBlockers) != 1 {
		t.Fatalf("ConfrontBlockers = %d, want 1: %+v", len(gr.ConfrontBlockers), gr.ConfrontBlockers)
	}
	if gr.ConfrontBlockers[0].HitID != "R-sync-gate-existing" {
		t.Errorf("blocker HitID = %q, want R-sync-gate-existing", gr.ConfrontBlockers[0].HitID)
	}
	if gr.ConfrontAcked {
		t.Error("ConfrontAcked should be false when no ack was supplied")
	}

	// With --decision-ref supplied, the SAME blocker is found but overridden.
	gr2, err2 := runSyncGates(before.DomainDir, before, after, report, landAckOptions{DecisionRef: "ticket #123"})
	if err2 != nil {
		t.Fatalf("runSyncGates with a decision-ref ack should not refuse on gate 7, got: %v", err2)
	}
	if !gr2.ConfrontAcked {
		t.Error("ConfrontAcked should be true once a decision-ref ack is supplied")
	}
}

// TestRunSyncGates_ConfrontBlocker_ExcludesSelfID proves the CHANGED-entry
// self-exclusion rule from the task brief: a requirement whose OWN claim
// changes must never self-block against its own (now-superseded) prior
// value. The fixture graph carries exactly ONE requirement
// (R-sync-gate-flipping) whose PRIOR claim asserted "ALWAYS" and whose NEW
// claim (already applied to `after`, exactly as a real SyncGraph mutation
// would leave it) asserts "NEVER" — a direct self-contradiction against its
// own prior value. Confronting the new claim against `after` necessarily
// re-finds R-sync-gate-flipping itself (it is the only requirement in the
// graph); since h.ID == entry.ID for that hit, it must be excluded, leaving
// zero blockers.
func TestRunSyncGates_ConfrontBlocker_ExcludesSelfID(t *testing.T) {
	before := &ontology.Graph{
		DomainDir: "unit-test-fixture",
		Requirements: []ontology.Requirement{
			{
				ID:             "R-sync-gate-flipping",
				Claim:          "the widget component shall ALWAYS frobnicate on startup",
				Owner:          "framework-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
			},
		},
	}
	after := &ontology.Graph{
		DomainDir: "unit-test-fixture",
		Requirements: []ontology.Requirement{
			{
				ID:             "R-sync-gate-flipping",
				Claim:          "the widget component shall NEVER frobnicate on startup",
				Owner:          "framework-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
				History:        []ontology.HistoryEntry{{At: "2026-01-01", Summary: "field Claim changed"}},
			},
		},
	}
	report := &selfspec.SyncReport{
		Entries: []selfspec.SyncReportEntry{
			{
				ID:   "R-sync-gate-flipping",
				Kind: selfspec.SyncKindChanged,
				FieldDiffs: []selfspec.FieldDiff{
					{Field: "Claim", Old: "the widget component shall ALWAYS frobnicate on startup", New: "the widget component shall NEVER frobnicate on startup"},
				},
			},
		},
	}

	gr, err := runSyncGates(before.DomainDir, before, after, report, landAckOptions{})
	if err != nil {
		t.Fatalf("runSyncGates should not refuse when the only hit is the entry's own ID: %v", err)
	}
	if len(gr.ConfrontBlockers) != 0 {
		t.Errorf("ConfrontBlockers = %+v, want none (self-ID hit must be excluded)", gr.ConfrontBlockers)
	}
}

// TestRunSyncGates_NewViolation_Refuses proves gate 8: an `after` graph that
// introduces a violation absent from `before` is refused, distinct from and
// checked independently of gate 7.
func TestRunSyncGates_NewViolation_Refuses(t *testing.T) {
	before := &ontology.Graph{DomainDir: "unit-test-fixture"}
	after := &ontology.Graph{
		DomainDir: "unit-test-fixture",
		Requirements: []ontology.Requirement{
			{
				// An empty ID triggers a real structural invariant
				// (dangling/empty-id class of checks) that AllViolations
				// reports regardless of Status — a simple, reliable way to
				// manufacture a NEW violation for this unit test without
				// depending on any specific check's exact name.
				ID:     "",
				Claim:  "a requirement with no id",
				Status: ontology.StatusDRAFT,
			},
		},
	}
	report := &selfspec.SyncReport{Entries: []selfspec.SyncReportEntry{{ID: "R-whatever", Kind: selfspec.SyncKindAdded}}}

	gr, err := runSyncGates(before.DomainDir, before, after, report, landAckOptions{})
	if err == nil {
		t.Fatal("expected runSyncGates to refuse when after introduces a new violation")
	}
	if len(gr.NewViolations) == 0 {
		t.Error("NewViolations should be non-empty")
	}
}

// TestRunSyncGates_AppendOnlyGate_CalledLast proves gate 9: an `after` graph
// whose History for a requirement present in `before` SHRINKS (a direct
// append-only breach) is refused by runSyncGates once gates 7/8 are clean,
// and the error is attributable to the append-only guard specifically.
func TestRunSyncGates_AppendOnlyGate_CalledLast(t *testing.T) {
	before := &ontology.Graph{
		DomainDir: "unit-test-fixture",
		Requirements: []ontology.Requirement{
			{
				ID:             "R-sync-gate-shrink",
				Claim:          "an unrelated claim",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
				History: []ontology.HistoryEntry{
					{At: "2026-01-01", Summary: "first"},
					{At: "2026-01-02", Summary: "second"},
				},
			},
		},
	}
	after := &ontology.Graph{
		DomainDir: "unit-test-fixture",
		Requirements: []ontology.Requirement{
			{
				ID:             "R-sync-gate-shrink",
				Claim:          "an unrelated claim",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementPROSE,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
				History: []ontology.HistoryEntry{
					{At: "2026-01-01", Summary: "first"},
					// second entry dropped: a direct append-only breach.
				},
			},
		},
	}
	report := &selfspec.SyncReport{} // no confront-relevant entries; 7 and 8 stay clean

	gr, err := runSyncGates(before.DomainDir, before, after, report, landAckOptions{})
	if err == nil {
		t.Fatal("expected runSyncGates to refuse on an append-only breach")
	}
	if !gr.AppendOnlyChecked {
		t.Error("AppendOnlyChecked should be true (gate 9 was reached)")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Errorf("error = %q, want it to mention 'append-only'", err.Error())
	}
}
