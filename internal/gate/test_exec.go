package gate

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// TestRunResult is the outcome of actually compiling and executing one
// verified_by test via `go test`, as opposed to SpecTestResult (gate's
// AST-only resolver) which only proves the test FUNCTION EXISTS and has the
// right shape. TestRunResult is the missing "does it actually pass" half
// PLAN-authored-spec-discipline.md §6 names ("verified_by тест существует и
// РЕАЛЬНО ЗАПУСКАЕТСЯ") and @fh finding F1 (Probe C) proved was never
// checked: gutting a requirement's real implementation (e.g. Forecast.
// RequireComplete returning nil unconditionally) makes `go test` red while
// every AST-only check above (resolvable / has-teeth / no-skip) stays green,
// because none of them ever runs the test.
type TestRunResult struct {
	// Passed is true only when `go test -run '^<name>$'` exited 0 AND its
	// output does not contain a "FAIL" line for this package (belt-and-
	// braces: exit code alone is the authoritative signal, FAIL-scanning is
	// a defense against a wrapped/shimmed `go` that swallows exit codes).
	Passed bool
	// CompileFailed is true when the package failed to build (syntax error,
	// undefined symbol, etc.) -- distinguished from a real test failure so
	// the violation message can say "does not compile" instead of "fails".
	CompileFailed bool
	// Output is the combined stdout+stderr from the go test invocation,
	// trimmed to a bounded size so a violation message never balloons.
	Output string
	// Err is a non-nil error only for an INFRASTRUCTURE failure (go binary
	// not found, module root not resolvable, timeout) -- distinct from the
	// test itself legitimately failing (Passed=false, Err=nil).
	Err error
	// Skipped is true when RunVerifiedByTest declined to spawn a subprocess
	// at all because the RECURSION GUARD (recursionGuardEnv) detected this
	// process is ALREADY running inside a `go test` invocation that
	// RunVerifiedByTest itself spawned -- see the guard's doc comment for
	// why this is structurally necessary for self-hosting domains (the
	// engine modeling itself), not an optional optimization. A Skipped
	// result is NEITHER Passed NOR a failure: the caller MUST treat it as
	// "unproven at this nesting level, proven at the outer level" and NOT
	// report a violation for it. Skipped==true ALWAYS carries a non-empty
	// InfraWarning (see below) -- a honored guard is never silent.
	Skipped bool
	// InfraWarning is a non-fatal, non-empty diagnostic string set whenever
	// RunVerifiedByTest wants to surface something about its OWN execution
	// that a caller may want to know about, without treating it as a failure.
	// Set unconditionally whenever Skipped is true (NEW-1, @fh's second
	// adversarial re-review, "honored-skip must not be silent" -- a clean
	// "0 violations" run must never look identical to one where entries were
	// silently deferred to an outer process; see RunVerifiedByTest's guard
	// branch). InfraWarning is orthogonal to Err: it never prevents the real
	// test run (when one happens) and is set independently of
	// Passed/CompileFailed.
	InfraWarning string
}

// recursionGuardEnv is the environment variable RunVerifiedByTest sets, on
// every `go test` child process it spawns, to a per-process unguessable nonce
// (see guardNonce -- NOT a fixed literal like "1"), and checks for on its OWN
// process before spawning another. Self-hosting domains make recursion a
// structural certainty, not an edge case: domains/hotam-spec-self's graph
// names its OWN engine test files as verified_by targets (e.g.
// "internal/ontology/graph_smoke_test.go:TestConflictPredicates"), and
// several of the engine's own package test suites (internal/invariants,
// internal/generator, cmd/hotam -- anywhere a _test.go file calls
// invariants.AllViolations against a real domain graph) exercise
// checkVerifiedByTestPasses themselves. Without a guard, running `go test
// ./internal/invariants/...` reaches a test that calls AllViolations on the
// real graph, which calls RunVerifiedByTest, which spawns ANOTHER `go test
// ./internal/invariants/...` -- itself containing the same test, which
// recurses again, unbounded. The guard breaks the cycle at depth 1: the
// outer (non-nested) process actually runs and proves the test; any process
// that finds recursionGuardEnv already set (see inRecursionGuard) knows it is
// nested and reports Skipped instead of spawning yet another generation.
const recursionGuardEnv = "HOTAM_VERIFIED_BY_EXEC_GUARD"

// ClearInheritedRecursionGuard unsets recursionGuardEnv in this process's own
// environment. Exported ONLY for cmd/hotam's main() to call, unconditionally,
// before any subcommand dispatch -- see recursionGuardEnv's doc comment (the
// NEW-1 fix) for why a top-level CLI process must never honor an INHERITED
// value of this variable: it is the root-cause half of the fix, the
// inRecursionGuard side is the other half, and callers other than a genuine
// CLI entry point have no legitimate reason to call this.
func ClearInheritedRecursionGuard() {
	os.Unsetenv(recursionGuardEnv)
}

// NEW-1 (@fh adversarial re-review, twice): the ORIGINAL guard trusted the
// mere PRESENCE of recursionGuardEnv="1" in the process's ambient environment
// as proof "I am a nested child RunVerifiedByTest itself spawned" -- a
// universal kill-switch any external actor could pull:
// `HOTAM_VERIFIED_BY_EXEC_GUARD=1 hotam all-violations` made the TOP-level
// (non-nested) process itself believe it was already inside a guarded
// subprocess, so RunVerifiedByTest returned Skipped for every verified_by
// entry -- check_verified_by_test_passes reported zero violations no matter
// how broken the underlying implementations were, on a gutted tree, silently.
//
// A FIRST fix (marker-vouched-nonce: a random per-process nonce, corroborated
// by a marker file written to the (since-removed) disk verdict cache's
// directory before the nonce was ever placed in a child's environment) was
// shipped and then broken by re-review: the
// marker lived at a PREDICTABLE, WORLD-WRITABLE path
// (os.TempDir()/hotam-verified-by-cache/guard-<value>.marker). An attacker
// does not need to guess this process's nonce at all -- they pick their OWN
// value X, write guard-X.marker themselves, then export
// HOTAM_VERIFIED_BY_EXEC_GUARD=X before invoking hotam directly. The
// corroborating secret (the marker) was stored in the open, right next to the
// exact env var being verified, so the "vouching" bought zero real
// protection -- and legitimate runs never cleaned up their markers, leaving a
// permanent, passively replayable kill-switch value for anyone who inspected
// os.TempDir() once.
//
// Two candidate corroborating signals, tried before either the marker
// approach or the fix actually used below, were rejected:
//
//   - argv[0]/invocation-shape sniffing ("is my own binary a `go test`-built
//     .test binary") FAILS: the OUTERMOST `go test ./internal/invariants/...`
//     invocation that legitimately reaches RunVerifiedByTest for the FIRST
//     time (not nested at all) is ITSELF a `go test`-compiled binary -- e.g.
//     this very package's own test suite. There is no way to distinguish "I
//     am the genuine outer go-test run" from "I am a nested go-test run" by
//     inspecting only this process's own argv[0]/build-path shape; both look
//     identical.
//   - direct os.Getppid() PID matching FAILS: `go test` always interposes
//     the `go` tool as an intermediary process between whatever spawned it
//     (runGoTest's exec.Command) and the compiled test binary that actually
//     runs the tests, so the running test binary's immediate parent PID is
//     never the original spawning hotam process's PID -- there is no cheap,
//     portable way to walk the full ancestor chain (especially on Windows)
//     to verify true lineage.
//
// FIX ACTUALLY USED (root-cause, not a corroborating-secret patch): a
// process-local, unguessable, in-memory-only signal cannot itself be forged
// from OUTSIDE the process by exporting an env var, no matter what value is
// chosen or what files an attacker can pre-create on disk -- so instead of
// trying to make the env var itself unforgeable (impossible: any child
// process can read+re-export any env var its parent had), the TOP-LEVEL CLI
// entry point (cmd/hotam's main(), see its own doc comment) unconditionally
// clears recursionGuardEnv from its OWN process environment BEFORE any
// subcommand runs. This is sound because a top-level `hotam` invocation is BY
// DEFINITION the root of any hotam-managed recursion -- it can never be a
// legitimate nested child (legitimate children are `go test`-spawned test
// binaries, which never go through cmd/hotam's main() at all -- see
// runGoTest). An external `HOTAM_VERIFIED_BY_EXEC_GUARD=<anything> hotam
// all-violations` therefore has its forged value wiped before
// RunVerifiedByTest is ever reached: the CLI process runs its own
// verified_by tests for real, with no defense the attacker can construct
// (no nonce to guess, no marker to race -- there is no corroborating check
// left to fool, because the untrusted input is discarded outright, not
// verified). Legitimate recursion is unaffected: runGoTest still mints a
// fresh crypto/rand nonce (guardNonce) and passes it ONLY to the `go test`
// child it itself spawns (cmd.Env) -- that child is a go-test binary, not a
// cmd/hotam CLI process, so main()'s Unsetenv never runs for it, and
// inRecursionGuard (below) trusts the env var's mere presence again, exactly
// as originally designed, because by the time ANY process can observe a
// non-empty recursionGuardEnv, main() has already guaranteed it was not
// inherited from outside a genuine RunVerifiedByTest-spawned lineage. No
// marker file, no disk state, no corroborating secret to leak or replay.
var (
	processGuardOnce  sync.Once
	processGuardNonce string
)

// newGuardNonce generates a fresh, unguessable 32-byte hex token via
// crypto/rand -- deliberately NOT a predictable value (a counter, a PID, a
// timestamp) since the whole point is that no external actor can guess it in
// advance and pre-seed the environment (or the marker-file directory) with a
// matching value.
func newGuardNonce() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing is effectively unheard-of on a real OS; fall
		// back to a fixed sentinel rather than panicking -- worst case this
		// degrades to a deterministic-but-still-per-process value, never a
		// crash. A fallback value never gets a marker file written for a
		// mismatched nonce from a DIFFERENT process, so this does not
		// silently reopen the kill-switch hole: it only affects a single
		// process's own children, which still go through the same
		// mint-then-mark-then-pass-down sequence.
		return "hotam-guard-fallback-nonce"
	}
	return hex.EncodeToString(buf)
}

// guardNonce returns this process's OWN freshly-minted guard token, minting
// it (crypto/rand) exactly once, the first time any caller in this process
// asks for it -- NEVER read from the environment, precisely so an external
// actor exporting recursionGuardEnv before this process even starts cannot
// influence what this process considers "its own" token.
func guardNonce() string {
	processGuardOnce.Do(func() {
		processGuardNonce = newGuardNonce()
	})
	return processGuardNonce
}

// inRecursionGuard reports whether the CURRENT process should treat itself as
// ALREADY running inside a `go test` subprocess RunVerifiedByTest itself
// spawned. True whenever recursionGuardEnv is non-empty.
//
// This is a deliberately simple presence check -- no marker file, no
// corroborating secret -- because the hard problem ("can an external actor
// forge this") is solved UPSTREAM, not here: cmd/hotam's main() (the only
// process type that could ever observe an INHERITED, attacker-controlled
// value of this env var before RunVerifiedByTest first runs) unconditionally
// clears recursionGuardEnv at CLI entry, before any subcommand executes. By
// the time inRecursionGuard runs inside a `hotam` process, any externally
// forged value has already been wiped; the only way this process can observe
// a non-empty value is if IT is a `go test` child that runGoTest itself
// spawned with a freshly minted nonce (see recursionGuardEnv's doc comment
// for the full NEW-1 history and why a marker-file corroboration scheme was
// tried, found forgeable via a predictable world-writable path, and removed
// in favor of this root-cause fix).
func inRecursionGuard() bool {
	return os.Getenv(recursionGuardEnv) != ""
}

// globalExecSlots is a PROCESS-WIDE (not per-call, not per-invariant-run)
// bounded semaphore limiting how many `go test` subprocesses runGoTest may
// have in flight AT ONCE from this process. checkVerifiedByTestPasses's own
// runExecWorkers cap (internal/invariants/authored_links.go) only bounds
// concurrency WITHIN one AllViolations call; it does nothing to stop TWO
// DIFFERENT goroutines/tests -- e.g. several t.Parallel() tests in
// cmd/hotam's own suite, each spawning a real `hotam` binary subprocess that
// in turn calls RunVerifiedByTest -- from independently deciding to run
// several workers each, at the same time, for a combined dozen-plus
// simultaneous `go test` invocations that thrash the machine (each is a
// full Go compile, not free). globalExecSlots is shared across every call
// site in this process (including nested guarded calls, which is harmless:
// a Skipped result never reaches here) so the TOTAL number of concurrent
// `go test` children this process spawns is bounded regardless of how many
// independent callers are asking. Kept aligned with runExecWorkers (2, not
// a larger number): observed on a heavily-loaded shared dev machine that a
// higher cap here let RunVerifiedByTest's own subprocess fan-out starve
// unrelated goroutines (e.g. check_enforced_by_resolvable's repo-wide
// filepath.WalkDir, a PRE-EXISTING check unrelated to verified_by
// execution) of OS scheduling time under go test -race, occasionally
// pushing a whole package past its -timeout budget.
var globalExecSlots = make(chan struct{}, 2)

// runGoTest invokes `go test -run '^<testName>$' <pkgPattern>` with cwd set
// to moduleRoot (the directory containing the go.mod that owns pkgDir), and
// classifies the result. pkgPattern is the "./..."-relative import pattern
// for the package directory (e.g. "./internal/ontology/" for a self-hosting
// entry, or "./model/" for an authored spec/ package) -- callers compute it
// via relativePackagePattern. The spawned process carries recursionGuardEnv
// so IT (or anything it in turn runs) knows not to spawn a further nested
// invocation -- see recursionGuardEnv's doc comment. Acquires a
// globalExecSlots slot before spawning and releases it after the subprocess
// exits, so this process never has more than a small bounded number of `go
// test` children running concurrently no matter how many independent
// callers invoke it at once.
func runGoTest(ctx context.Context, moduleRoot, pkgPattern, testName string) TestRunResult {
	select {
	case globalExecSlots <- struct{}{}:
	case <-ctx.Done():
		return TestRunResult{Err: fmt.Errorf("go test for %s in %s: %w (timed out waiting for an execution slot)", testName, pkgPattern, ctx.Err())}
	}
	defer func() { <-globalExecSlots }()

	runPattern := "^" + testName + "$"
	cmd := exec.CommandContext(ctx, "go", "test", "-run", runPattern, "-count=1", pkgPattern)
	cmd.Dir = moduleRoot
	// Carry THIS process's own freshly-minted guard nonce (never a fixed
	// literal -- see guardNonce's / recursionGuardEnv's doc comments) so the
	// spawned `go test` child, and anything it in turn runs, can recognize it
	// is nested. No marker file is written: the child this env var reaches is
	// always a `go test`-compiled binary, never a cmd/hotam CLI process (that
	// only ever clears this var, see main()'s doc comment), so there is no
	// forgery surface left for a marker to defend against.
	nonce := guardNonce()
	cmd.Env = append(os.Environ(), recursionGuardEnv+"="+nonce)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	output := boundOutput(buf.String())

	if ctx.Err() == context.DeadlineExceeded {
		return TestRunResult{
			Output: output,
			Err:    fmt.Errorf("go test timed out running %s in %s: %w", runPattern, pkgPattern, ctx.Err()),
		}
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !isExitError(err, &exitErr) {
			// go binary missing, cwd invalid, etc. -- infrastructure failure,
			// not a test verdict.
			return TestRunResult{Output: output, Err: fmt.Errorf("could not run go test: %w", err)}
		}
		compileFailed := looksLikeCompileFailure(output)
		return TestRunResult{Passed: false, CompileFailed: compileFailed, Output: output}
	}
	// Exit 0. Still scan for a "FAIL" line as belt-and-braces (a wrapped `go`
	// or a test harness oddity could theoretically exit 0 with FAIL text);
	// the exit code is authoritative for the common case.
	if strings.Contains(output, "\nFAIL") || strings.HasPrefix(output, "FAIL") {
		return TestRunResult{Passed: false, Output: output}
	}
	return TestRunResult{Passed: true, Output: output}
}

// RecordedArtifact is one hotamspec.Artifact read back, in memory, from a
// record-mode `go test` run -- the engine-side mirror of
// internal/recorder/canon's Artifact JSON shape (PLAN-scenario-generated-
// spec.md §2 D1/§3 W1.2). Kept as this package's own struct (not importing
// the canon package directly) so gate never depends on recorder/canon --
// the two packages are deliberately independent: canon is VENDORED (copied)
// into a consumer domain's own spec/ module and compiled there, while gate
// only ever reads the JSON bytes that vendored copy wrote back out of a tmp
// directory. RawJSON preserves the artifact's exact on-disk bytes (the
// canonical, byte-identical-across-runs form the recorder itself already
// guarantees) so a caller that wants to hash or persist the artifact
// verbatim (e.g. a future SPEC.md generator, W1.3) never has to re-marshal
// through a second, potentially non-identical encoding path.
type RecordedArtifact struct {
	// FileName is the artifact's file name as the recorder wrote it
	// (<reqID>__<TestName>.json), useful for diagnostics and for a caller
	// that wants to correlate an artifact back to which requirement/test
	// produced it without re-parsing RawJSON.
	FileName string
	// RawJSON is the exact bytes read back from disk -- the canonical
	// encoding hotamspec.Scenario.writeArtifact produced, byte-identical
	// across repeated runs of the same scenario (see
	// PLAN-scenario-generated-spec.md §2 D1's determinism requirement).
	RawJSON []byte
}

// RecordingResult is the outcome of RunVerifiedByTestRecording: everything
// TestRunResult already reports (the test's own pass/fail verdict), plus the
// canonical scenario artifact(s) the test wrote in record-mode and the raw
// coverage profile bytes proving which lines of the implemented_by symbol's
// package this SAME test run actually executed (PLAN-scenario-generated-
// spec.md §2 D3 -- consumed by W2.2's coverage-proof gate, but collected
// here, in this one `go test` invocation, per the task's "one run gives
// asserts + artifact + coverprofile" contract).
type RecordingResult struct {
	TestRunResult
	// Artifacts holds every canonical JSON scenario artifact found in the
	// record dir after the run -- ordinarily exactly one (a verified_by test
	// that constructs a single hotamspec.Scenario), but a test that
	// constructs more than one Scenario (e.g. several sub-cases, each with
	// its own NewScenario call) writes one artifact per Scenario, and all of
	// them are returned here. Empty (never nil vs non-nil distinguished
	// beyond len==0) when the test does not use hotamspec at all, or was not
	// reached (Skipped/Err).
	Artifacts []RecordedArtifact
	// CoverProfile is the raw bytes of the `go test -coverprofile` output
	// file for this run (Go's own text coverage-profile format: a "mode:"
	// header line followed by one "file:startLine.startCol,endLine.endCol
	// numStmt count" line per counted statement block) -- nil when coverage
	// collection was not requested, the run never reached execution
	// (Skipped/Err/CompileFailed), or go test produced no profile (can
	// happen for a package with zero coverable statements).
	CoverProfile []byte
}

// RunVerifiedByTestRecording is the RECORD-MODE sibling of RunVerifiedByTest
// (PLAN-scenario-generated-spec.md §2 D1/D3, task W1.2): ONE `go test`
// invocation that simultaneously (a) asserts normally (Then still calls
// t.Errorf exactly as plain mode), (b) writes a canonical hotamspec.Scenario
// JSON artifact per PLAN §2 D1 (via internal/recorder/canon's env-gated
// t.Cleanup -- see that package's RecordDirEnv), and (c) collects a Go
// coverage profile over coverPkgFile's OWN package (the implemented_by
// symbol's package -- PLAN §2 D3's coverage-proof input, consumed by a later
// gate, W2.2, but the profile is captured here because it can only ever be
// produced by actually re-running the test, and re-running it a SECOND time
// just to add -coverprofile would defeat the whole "one run proves
// everything" point of this task).
//
// specRoot/file/testName identify the verified_by test exactly as
// RunVerifiedByTest's own parameters do. coverPkgFile is a file living in the
// PACKAGE coverage should be measured over -- ordinarily the implemented_by
// entry's own file (e.g. "model/brd_package.go") -- so -coverpkg targets
// exactly that package's import path; pass "" to skip coverage collection
// entirely (a plain record-mode run with no coverprofile).
//
// Deliberately DOES NOT read or write the runCache/singleflightRun in-memory
// verdict cache RunVerifiedByTest maintains: a recording run's whole point is
// to produce FRESH artifacts and a fresh coverage profile for THIS call, so
// memoizing it under the same cache used by the boolean-verdict fast path
// would either (a) let a plain RunVerifiedByTest call silently reuse a
// recording run's cached TestRunResult without ever having asked for
// artifacts (harmless but confusing), or (b) let a recording call reuse a
// PLAIN cached result and return stale/absent artifacts for a test that
// really did just run with recording requested -- neither is worth the
// complexity of teaching one cache to carry two different result shapes.
// Every call to this function spawns its OWN real `go test` subprocess --
// unlike RunVerifiedByTest there is no in-memory memoization or singleflight
// collapsing here at all (a caller invoking this twice for the same test
// gets two real subprocess runs), which is deliberately simple rather than
// teaching one cache/singleflight pair to carry two different result shapes;
// callers that need to avoid redundant recording runs are expected to call
// this at most once per (file, test) per invocation, the same way a
// generator (W1.3) would only ever record a given verified_by entry once per
// `hotam gen-spec` run. There is no PERSISTENT disk cache of any kind here
// either way (holding b014a63's fix): the tmp directory this function
// creates is deleted before it returns, on every return path, success or
// failure.
func RunVerifiedByTestRecording(specRoot, file, testName, coverPkgFile string) RecordingResult {
	if inRecursionGuard() {
		return RecordingResult{TestRunResult: TestRunResult{
			Skipped: true,
			InfraWarning: fmt.Sprintf(
				"%s recursion guard honored -- this process did not execute %s itself (record-mode); it is skipped at this nesting level and must be proven PASSING by the outer, non-nested process that set this guard",
				recursionGuardEnv, testName),
		}}
	}

	path := filepath.Join(specRoot, filepath.FromSlash(file))
	pkgDir := filepath.Dir(path)
	absPkgDir, err := filepath.Abs(pkgDir)
	if err != nil {
		return RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("could not resolve package directory for %s: %w", path, err)}}
	}

	moduleRoot, ok := ModuleRoot(absPkgDir)
	if !ok {
		return RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("no go.mod found walking up from %s -- cannot determine which Go module owns %s", absPkgDir, path)}}
	}

	pattern, err := relativePackagePattern(moduleRoot, path)
	if err != nil {
		return RecordingResult{TestRunResult: TestRunResult{Err: err}}
	}

	var coverPkgPattern string
	if coverPkgFile != "" {
		coverAbs, err := filepath.Abs(filepath.Join(specRoot, filepath.FromSlash(coverPkgFile)))
		if err != nil {
			return RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("could not resolve coverage package file %s: %w", coverPkgFile, err)}}
		}
		coverPkgPattern, err = relativePackagePattern(moduleRoot, coverAbs)
		if err != nil {
			return RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("could not compute -coverpkg pattern for %s: %w", coverPkgFile, err)}}
		}
	}

	recordDir, err := os.MkdirTemp("", "hotam-record-")
	if err != nil {
		return RecordingResult{TestRunResult: TestRunResult{Err: fmt.Errorf("could not create per-run record tmp dir: %w", err)}}
	}
	defer os.RemoveAll(recordDir)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	runResult, coverProfile := runGoTestRecording(ctx, moduleRoot, pattern, testName, recordDir, coverPkgPattern)

	artifacts, artErr := readArtifacts(recordDir)
	if artErr != nil && runResult.Err == nil {
		runResult.Err = fmt.Errorf("record-mode run completed but artifacts could not be read back from %s: %w", recordDir, artErr)
	}

	return RecordingResult{
		TestRunResult: runResult,
		Artifacts:     artifacts,
		CoverProfile:  coverProfile,
	}
}

// runGoTestRecording is runGoTest's record-mode sibling: same recursion-guard
// nonce, same globalExecSlots bound, same PASS/FAIL/CompileFailed
// classification -- but additionally sets hotamspec.RecordDirEnv
// ("HOTAM_RECORD_DIR") to recordDir on the child process's environment, and,
// when coverPkgPattern is non-empty, passes -coverprofile (written inside
// recordDir, never a shared/predictable path) and -coverpkg so the SAME
// subprocess also produces a coverage profile over the implemented_by
// symbol's package. Returns the classified TestRunResult plus the raw
// coverprofile bytes (nil if coverage was not requested or the file was
// never produced).
func runGoTestRecording(ctx context.Context, moduleRoot, pkgPattern, testName, recordDir, coverPkgPattern string) (TestRunResult, []byte) {
	select {
	case globalExecSlots <- struct{}{}:
	case <-ctx.Done():
		return TestRunResult{Err: fmt.Errorf("go test (record-mode) for %s in %s: %w (timed out waiting for an execution slot)", testName, pkgPattern, ctx.Err())}, nil
	}
	defer func() { <-globalExecSlots }()

	runPattern := "^" + testName + "$"
	args := []string{"test", "-run", runPattern, "-count=1"}
	var coverProfilePath string
	if coverPkgPattern != "" {
		coverProfilePath = filepath.Join(recordDir, "cover.out")
		args = append(args, "-coverprofile="+coverProfilePath, "-coverpkg="+coverPkgPattern)
	}
	args = append(args, pkgPattern)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleRoot
	nonce := guardNonce()
	cmd.Env = append(os.Environ(),
		recursionGuardEnv+"="+nonce,
		recordDirEnvName+"="+recordDir,
	)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	output := boundOutput(buf.String())

	var coverProfile []byte
	if coverProfilePath != "" {
		if data, readErr := os.ReadFile(coverProfilePath); readErr == nil {
			coverProfile = data
		}
		// A missing coverprofile (e.g. the run failed before any test ran at
		// all, or a package with zero coverable statements) is not itself an
		// error worth surfacing here -- TestRunResult's own Passed/
		// CompileFailed/Err already carries whatever went wrong with the run
		// itself; CoverProfile simply stays nil.
	}

	if ctx.Err() == context.DeadlineExceeded {
		return TestRunResult{
			Output: output,
			Err:    fmt.Errorf("go test (record-mode) timed out running %s in %s: %w", runPattern, pkgPattern, ctx.Err()),
		}, coverProfile
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !isExitError(err, &exitErr) {
			return TestRunResult{Output: output, Err: fmt.Errorf("could not run go test (record-mode): %w", err)}, coverProfile
		}
		compileFailed := looksLikeCompileFailure(output)
		return TestRunResult{Passed: false, CompileFailed: compileFailed, Output: output}, coverProfile
	}
	if strings.Contains(output, "\nFAIL") || strings.HasPrefix(output, "FAIL") {
		return TestRunResult{Passed: false, Output: output}, coverProfile
	}
	return TestRunResult{Passed: true, Output: output}, coverProfile
}

// recordDirEnvName is HotamSpec's own copy of internal/recorder/canon's
// RecordDirEnv literal ("HOTAM_RECORD_DIR"). Declared as a separate literal
// here, rather than importing internal/recorder/canon directly, so this
// package (internal/gate, engine-internal) never depends on the recorder
// canon package that gets VENDORED (copied) into consumer domains -- the two
// sides of this contract (the engine setting the env var, the vendored
// recorder reading it) only need to agree on the STRING, not share a Go
// import; a mismatch between the two literals is caught mechanically by
// TestRecordDirEnvName_MatchesCanonLiteral (test_exec_test.go), which reads
// both packages' source at test time to compare the two constants without
// creating a compile-time dependency.
const recordDirEnvName = "HOTAM_RECORD_DIR"

// readArtifacts reads every *.json file directly inside dir (non-recursive --
// hotamspec.Scenario's writer never creates subdirectories) into memory as
// RecordedArtifact values, sorted by file name for a deterministic return
// order (os.ReadDir's own result is already name-sorted, but sorting again
// explicitly here documents the guarantee rather than relying on an incidental
// stdlib behavior this function does not itself control).
func readArtifacts(dir string) ([]RecordedArtifact, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	artifacts := make([]RecordedArtifact, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return artifacts, fmt.Errorf("could not read artifact %s: %w", name, err)
		}
		artifacts = append(artifacts, RecordedArtifact{FileName: name, RawJSON: data})
	}
	return artifacts, nil
}

// isExitError reports whether err is (or wraps) an *exec.ExitError, writing
// it into *target on success. Kept as a named helper so the intent at the
// call site ("this is a normal test-failure exit, not infra breakage") reads
// clearly.
func isExitError(err error, target **exec.ExitError) bool {
	ee, ok := err.(*exec.ExitError)
	if ok {
		*target = ee
	}
	return ok
}

// looksLikeCompileFailure detects the standard `go test` build-failure
// shape ("# <import path>" header followed by compiler diagnostics, or the
// literal "FAIL	<pkg> [build failed]" trailer) so a violation can say
// "does not compile" rather than the more general "test fails" -- Probe C's
// sibling case (a syntax error introduced into a spec file) must be
// reported as a violation, not a panic, and this is what lets the message
// name the real defect.
func looksLikeCompileFailure(output string) bool {
	return strings.Contains(output, "[build failed]") ||
		strings.Contains(output, "build constraints exclude all Go files") ||
		strings.HasPrefix(strings.TrimSpace(output), "#") ||
		strings.Contains(output, "cannot find package") ||
		strings.Contains(output, "no Go files in")
}

const maxOutputBytes = 4000

// boundOutput trims combined go-test output to a bounded size so a
// violation message (and any downstream JSON/log consumer) never carries an
// unbounded blob -- the trailing lines are kept (compiler errors and the
// final FAIL/PASS line live at the end) since the head is usually least
// informative (package name, cache status).
func boundOutput(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxOutputBytes {
		return s
	}
	return "...(truncated)...\n" + s[len(s)-maxOutputBytes:]
}

// relativePackagePattern converts an absolute file path into a
// "./"-relative go test package pattern, relative to moduleRoot: the
// directory portion of file, expressed as "./a/b/" (trailing slash, forward
// slashes, so it works identically whether invoked from Windows or a POSIX
// shell -- `go test` accepts forward-slash import patterns on every OS).
func relativePackagePattern(moduleRoot, file string) (string, error) {
	dir := filepath.Dir(file)
	rel, err := filepath.Rel(moduleRoot, dir)
	if err != nil {
		return "", fmt.Errorf("could not compute package pattern for %s relative to %s: %w", file, moduleRoot, err)
	}
	relSlash := filepath.ToSlash(rel)
	if relSlash == "." {
		return "./", nil
	}
	if strings.HasPrefix(relSlash, "..") {
		return "", fmt.Errorf("package directory %s escapes module root %s", dir, moduleRoot)
	}
	return "./" + relSlash + "/", nil
}

// ModuleRoot walks UP from dir looking for the nearest ancestor directory
// containing go.mod -- the working directory `go test` must be invoked from
// so the package pattern resolves against the right module. Exported (not
// just the package-private walkUpToGoMod used by engineRoot) because
// RunVerifiedByTest needs it for BOTH the self-hosting case (same answer as
// engineRoot) and the ordinary-domain case (a domain's own spec/ tree,
// which per PLAN-authored-spec-discipline.md carries its OWN go.mod, module
// "prat-spec" in the plan's worked example -- a module distinct from and
// unaware of the engine's own go.mod that a naive walk-up from deep inside
// domains/<name>/spec/ would otherwise never find without stopping at the
// FIRST go.mod encountered, which walkUpToGoMod already does).
func ModuleRoot(dir string) (string, bool) {
	return walkUpToGoMod(dir)
}

// cacheKey identifies one (package directory + test name) unit of work.
type cacheKey struct {
	pkgDir   string
	testName string
}

// cacheEntry is a memoized TestRunResult plus the content-hash it was
// computed under -- a cache HIT requires both the key AND the hash to
// match; a changed hash (impl file edited, test file edited, go.mod/go.sum
// edited) is treated as a cache MISS even though the key is unchanged, which
// is what makes Probe C's mutation (an impl-file edit, not a test-file edit)
// actually invalidate the cache instead of returning the stale green result.
type cacheEntry struct {
	hash   string
	result TestRunResult
}

// runCache is a process-lifetime, in-memory memoization of RunVerifiedByTest
// results, keyed by (package directory, test name) with content-hash
// invalidation -- the fast path: a repeat call from the SAME process (e.g.
// BenchmarkAllViolations_RealDomain's b.N loop, TestAllViolations_
// DeterministicOrder's 21x loop) never even touches disk.
var runCache sync.Map // cacheKey -> cacheEntry

// ResetRunCacheForTest clears the in-memory cache only. Test-only helper
// (exported so internal/invariants' tests, a different package, can call
// it). There is no on-disk/cross-process cache to also clear: see the
// removed-disk-cache history below (NEW-3) -- runCache (this in-memory
// sync.Map) is the ONLY verdict cache RunVerifiedByTest maintains, and it is
// process-lifetime only, so clearing it here is complete.
func ResetRunCacheForTest() {
	runCache = sync.Map{}
}

// NEW-3 (@fh final adversarial re-review, "kill-switch moved from env var to
// cache file"): this package used to also maintain a SHARED, CROSS-PROCESS
// on-disk verdict cache at os.TempDir()/hotam-verified-by-cache/, keyed by
// content-hash (diskCacheDir/loadDiskCache/storeDiskCache, all now REMOVED).
// The stated purpose was a real, legitimate performance win: cmd/hotam's own
// e2e suite spawns dozens of independent `hotam.exe` processes against
// copies of the SAME real self-hosting domain graph, and without a
// cross-process cache each one pays its own cold `go test` compile for the
// same real engine packages at once. But @fh reproduced a FULL OFFLINE
// FORGE against it: the cache key (hashPackageInputs) is a pure function of
// files already on disk, so ANYONE can recompute it WITHOUT ever running
// `hotam` at all, then hand-write {"passed":true,...} to the resulting
// world-writable, predictably-named path
// (os.TempDir()/hotam-verified-by-cache/<sha256>__<TestName>.json) BEFORE
// invoking `hotam all-violations` against a genuinely red domain --
// RunVerifiedByTest's disk-cache-hit branch read the forged verdict back and
// reported "0 violations -- graph clean" without ever actually compiling or
// running anything. This is structurally worse than the NEW-1 env-var
// kill-switch it superficially resembles: an env var could only ever signal
// "skip" (Skipped, never a fabricated PASS), but the disk-cache file carried
// the VERDICT ITSELF -- an attacker did not need to make the engine skip
// proof, they could hand it a fabricated proof directly.
//
// No corroborating-secret patch (a signed cache file, a per-user-writable
// directory, a "trusted path" allowlist) was attempted: NEW-1's own history
// (recursionGuardEnv's doc comment) already demonstrates that pattern fails
// here for the identical reason a marker-vouched-nonce failed there -- any
// corroborating secret stored ALONGSIDE the untrusted value it is meant to
// vouch for, in a location the attacker can also read and write, buys
// nothing. The root-cause fix is the one NEW-1 already established as the
// only sound shape for this class of problem: remove the untrusted shared
// state ENTIRELY rather than trying to make it unforgeable. In production a
// single `hotam all-violations` invocation is one process; there is no
// legitimate cross-process verdict-sharing need to weigh against the forge
// risk. The in-memory runCache (this process only, unreachable from outside
// the process) plus singleflightRun (collapses concurrent in-process
// callers) remain -- they give every real correctness and anti-stampede
// property the disk cache offered WITHIN one process, which is the only
// scope a single `hotam` invocation ever needs. cmd/hotam's own e2e suite
// (which legitimately spawns many separate processes against copies of the
// same graph) is slower without cross-process sharing -- an accepted,
// measured cost of closing a real production forgery hole, not a regression
// in anything a real `hotam all-violations` invocation depends on.

// RunVerifiedByTest compiles and executes the named verified_by test
// (specRoot/file:testName, in the shape ResolveSpecTest already validated
// exists and is a real func TestXxx(t *testing.T)) via `go test -run`, and
// reports whether it passes. This is the EXECUTION half of the verified_by
// discipline: ResolveSpecTest/testBodyHasTeeth/testBodyHasTopLevelSkip (all
// AST-only, gate/spec_resolver.go) prove the test EXISTS and is not
// trivially vacuous; RunVerifiedByTest is what actually runs it, closing
// @fh finding F1 (Probe C: gutting the implementation the test exercises
// left every AST-only check green because none of them executed the test).
//
// Results are memoized in runCache, keyed by (package directory, test name)
// with content-hash invalidation over: go.mod + go.sum (if present) at the
// resolved module root, and every *.go file's content in the test's own
// package directory (not just the two named files) -- `go test` compiles
// the WHOLE package, so a mutation to any sibling file in that package
// (exactly Probe C's shape: the implementation function lives in a
// different file than the test, both in the same package) must invalidate
// the cache, and hashing the whole directory's file set is the only way to
// guarantee that without having to correctly guess which implemented_by
// entry pairs with which verified_by entry (they are not necessarily on the
// same Requirement, and a package can have more source files than the ones
// any single Requirement names).
func RunVerifiedByTest(specRoot, file, testName string) (out TestRunResult) {
	if inRecursionGuard() {
		// See recursionGuardEnv's doc comment: this process is ALREADY
		// running inside a `go test` subprocess that RunVerifiedByTest
		// itself spawned (a self-hosting domain's own graph names its own
		// engine tests as verified_by targets, and those same engine
		// packages' test suites call AllViolations against that same real
		// graph -- e.g. internal/invariants' own tests exercise
		// AllViolations(domains/hotam-spec-self/graph.json), which is
		// EXACTLY the graph whose verified_by entries point back into
		// internal/invariants -- so without this guard, running `go test
		// ./internal/invariants/...` would spawn a NESTED `go test
		// ./internal/invariants/...`, which itself runs the same tests,
		// which spawn the same nested call again, without bound). Do not
		// spawn a second nested subprocess -- report Skipped so the caller
		// treats this entry as "unproven at THIS nesting level, proven at
		// the outer, non-nested level" rather than either a false PASS or a
		// false violation.
		//
		// NEW-1 (@fh's second re-review, "honored-skip must not be silent"):
		// a Skipped result used to carry no InfraWarning at all -- a clean
		// "0 violations" run gave no visible trace of how many verified_by
		// entries were actually proven at THIS level versus deferred to an
		// outer process. Every honored Skip now stamps a non-empty
		// InfraWarning unconditionally, so a caller inspecting results (or an
		// operator reading all-violations output) can always see that this
		// entry's real proof happened elsewhere, not nowhere.
		return TestRunResult{
			Skipped: true,
			InfraWarning: fmt.Sprintf(
				"%s recursion guard honored -- this process did not execute %s itself; it is skipped at this nesting level and must be proven PASSING by the outer, non-nested process that set this guard",
				recursionGuardEnv, testName),
		}
	}

	path := filepath.Join(specRoot, filepath.FromSlash(file))
	pkgDir := filepath.Dir(path)
	absPkgDir, err := filepath.Abs(pkgDir)
	if err != nil {
		return TestRunResult{Err: fmt.Errorf("could not resolve package directory for %s: %w", path, err)}
	}

	moduleRoot, ok := ModuleRoot(absPkgDir)
	if !ok {
		return TestRunResult{Err: fmt.Errorf("no go.mod found walking up from %s -- cannot determine which Go module owns %s", absPkgDir, path)}
	}

	hash, err := hashPackageInputs(moduleRoot, absPkgDir)
	if err != nil {
		return TestRunResult{Err: fmt.Errorf("could not hash package inputs for %s: %w", absPkgDir, err)}
	}

	key := cacheKey{pkgDir: absPkgDir, testName: testName}
	if cached, ok := runCache.Load(key); ok {
		entry := cached.(cacheEntry)
		if entry.hash == hash {
			return entry.result
		}
	}

	// SINGLEFLIGHT (anti-stampede): several goroutines in THIS process can
	// reach here for the SAME (hash, testName) at once -- checkVerifiedByTestPasses's
	// own worker pool is only 4-wide per call, but cmd/hotam's e2e suite
	// runs many t.Parallel() tests that EACH independently call AllViolations
	// (directly, or via a spawned `hotam.exe` subprocess whose own in-process
	// call graph fans out the same way) against COPIES of the same real
	// self-hosting graph. Without collapsing duplicate in-flight work, EVERY
	// one of those callers independently misses the (still-cold) disk cache
	// at the same moment and spawns its OWN redundant `go test` subprocess --
	// exactly the thundering-herd fan-out observed to push cmd/hotam's own
	// test suite past its `-timeout` budget under machine contention.
	// singleflightForKey ensures only the FIRST caller for a given key
	// actually runs runGoTest; every other concurrent caller for the SAME
	// key blocks on the same result and reuses it, never spawning its own
	// subprocess.
	result := singleflightRun(key, hash, func() TestRunResult {
		pattern, err := relativePackagePattern(moduleRoot, path)
		if err != nil {
			return TestRunResult{Err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		return runGoTest(ctx, moduleRoot, pattern, testName)
	})

	// Only memoize a result that actually reflects the current content (not
	// an infrastructure failure, which should be retried rather than cached
	// -- a transient "go binary not found" or timeout should not poison
	// every subsequent call for the rest of the process's life).
	if result.Err == nil {
		runCache.Store(key, cacheEntry{hash: hash, result: result})
	}
	return result
}

// inFlightCall is one in-progress RunVerifiedByTest execution that other
// goroutines with the SAME (key, hash) can wait on instead of starting their
// own redundant subprocess -- see singleflightRun.
type inFlightCall struct {
	hash   string
	done   chan struct{}
	result TestRunResult
}

// inFlightCalls tracks in-progress calls, keyed by cacheKey (not by hash --
// see singleflightRun's doc comment for why a key can have at most one
// in-flight call at a time even across a hash change mid-flight).
var (
	inFlightMu    sync.Mutex
	inFlightCalls = map[cacheKey]*inFlightCall{}
)

// singleflightRun ensures at most ONE goroutine in this process actually
// executes run() for a given (key, hash) at a time; concurrent callers for
// the same key+hash block on the SAME in-flight call and receive its result
// once it completes, rather than each spawning their own subprocess. A
// concurrent caller for the same KEY but a DIFFERENT hash (a genuine race
// between "read the content" and "someone else is mutating the files right
// now") is deliberately NOT collapsed onto the stale in-flight call -- it
// waits for the in-flight call to finish (so as to not pile on TOP of it),
// then falls through to start its own fresh call for its own hash, since a
// hash mismatch means the content it observed is not what the in-flight
// call is proving.
func singleflightRun(key cacheKey, hash string, run func() TestRunResult) TestRunResult {
	for {
		inFlightMu.Lock()
		existing, inFlight := inFlightCalls[key]
		if inFlight {
			inFlightMu.Unlock()
			<-existing.done
			if existing.hash == hash {
				return existing.result
			}
			// Hash changed while we waited (content mutated concurrently) --
			// loop around: either another in-flight call for the NEW hash is
			// now registered (wait on that one too), or none is and this
			// goroutine will register and run its own below.
			continue
		}
		call := &inFlightCall{hash: hash, done: make(chan struct{})}
		inFlightCalls[key] = call
		inFlightMu.Unlock()

		call.result = run()
		close(call.done)

		inFlightMu.Lock()
		if inFlightCalls[key] == call {
			delete(inFlightCalls, key)
		}
		inFlightMu.Unlock()

		return call.result
	}
}

// hashPackageInputs computes a single SHA-256 digest over every file that can
// affect `go test`'s verdict for the WHOLE MODULE rooted at moduleRoot: go.mod
// and go.sum (if present) at moduleRoot, plus EVERY file anywhere under
// moduleRoot (recursive) -- not just *.go files, see the NEW-4 doc comment
// below. pkgDir is accepted for API/call-site compatibility (existing
// callers, including this package's own tests, pass the test's specific
// package directory) but is otherwise UNUSED for hashing purposes -- see the
// NEW-2 doc comment below for why the cache key intentionally widened from
// "this one package directory" to "the whole owning module".
//
// NEW-2 (@fh adversarial re-review, stale-green across a package boundary):
// the ORIGINAL implementation hashed only pkgDir's own *.go files (the single
// directory containing the named verified_by test) plus go.mod/go.sum. That
// is unsound the moment an authored spec/ tree follows
// PLAN-authored-spec-discipline.md §8's own prescribed layout: spec/model/,
// spec/application/, and spec/policy/ are SEPARATE Go packages (separate
// directories), and a model/ test can transitively depend on policy/ (e.g.
// a model constructor calling a policy/ validation function). `go test
// ./model/` compiles and links the WHOLE DEPENDENCY GRAPH the model package
// imports, so a behavioral change inside policy/ (e.g. a validation
// function's threshold silently gutted) can flip that same `go test` from
// PASS to FAIL -- but pkgDir-only hashing never observes the change, because
// policy/'s files live in a DIFFERENT directory than model/'s. The stale
// disk/in-memory cache entry (keyed on model/'s unchanged hash) is served
// back verbatim: "0 violations -- graph clean" on a tree that would fail if
// actually re-run. @fh reproduced exactly this: warm the cache (green run),
// gut spec/policy/threshold.go's return value, `go test ./model/` now fails
// for real, but `hotam all-violations` still reports clean because the cache
// key never moved.
//
// Fix: widen the cache key from "one package directory's *.go files" to
// "every *.go file anywhere under the OWNING MODULE (moduleRoot down)".
// Two designs were considered:
//
//	(A) Compute the test package's TRANSITIVE IMPORT GRAPH (`go list -deps`
//	    or golang.org/x/tools/go/packages), filter to packages whose import
//	    path falls under the module's own path prefix, and hash only those
//	    directories' *.go files. Precise (a change in an unrelated sibling
//	    package that the test does NOT import would not force a re-run), but
//	    adds a `go list` subprocess call (or a go/packages load) to EVERY
//	    cache-key computation -- itself a real cost paid on every single
//	    RunVerifiedByTest call, cache hit or not, since the hash has to be
//	    computed before the cache can even be consulted -- and ties
//	    correctness to `go list`'s own behavior (build tags, module graph
//	    resolution) being invoked correctly for every domain shape.
//	(B) Hash EVERY *.go file under the whole owning module (moduleRoot
//	    downward), unconditionally, regardless of whether the test's package
//	    actually imports each one. Coarser (a change to a sibling package the
//	    test does NOT depend on also forces a re-run of tests that could not
//	    possibly have been affected), but: (1) it is trivially CORRECT --
//	    guarantees ANY behavioral change anywhere in the module invalidates
//	    EVERY cache entry for that module, closing NEW-2 completely rather
//	    than only for the specific sibling-package shape found so far; (2) it
//	    needs no subprocess, no go/packages dependency, no import-graph
//	    resolution logic to get right per-domain; (3) for a self-hosting
//	    domain (the common real case today -- domains/hotam-spec-self names
//	    the engine's OWN packages as verified_by targets) the owning module IS
//	    the engine repository, and "any engine-side change invalidates every
//	    self-hosting verified_by cache entry" is not overreach, it is exactly
//	    correct: the whole engine is the thing under test. For an authored
//	    domain's OWN spec/ module (small by construction -- a handful of
//	    model/application/policy packages, not a large codebase), the
//	    over-invalidation cost of (B) is bounded and cheap in absolute terms
//	    (hashing a few hundred KB of source is single-digit milliseconds).
//
// CHOSEN: (B), whole-module hash. It is simpler, has no new external-tool
// dependency, and its only real cost -- some cache misses that a precise
// import-graph analysis would have avoided -- is bounded by module size,
// which for every domain shape this engine currently supports (its own
// engine module, or a domain's dedicated spec/ module) is small enough that
// the hash itself stays cheap (this repo's own ~250 *.go files hash in low
// single-digit milliseconds); the correctness guarantee (NOTHING under the
// module can change without invalidating every verified_by cache entry for
// that module) is worth strictly more than the saved cache-hit-rate (A)
// would have bought, and is far simpler to keep correct as the authored-spec
// layout (§8's model/application/policy split, and whatever further
// packages a future domain adds) evolves without this cache-invalidation
// logic having to be re-taught about each new package shape.
//
// Skips VCS/build-cache/tool-state directories: any directory whose name
// starts with "." (the same convention `go build`/`go list` themselves use
// to decide what is NOT part of a module's own package tree -- .git, .github,
// an editor's .vscode, or any other dotted tool-state directory a host
// environment happens to keep at the module root) plus any directory
// literally named "vendor" (a vendored dependency's source never affects
// THIS module's own behavior in a way relevant to re-running ITS tests, and
// vendor trees can be large). This bound is what keeps the walk to the
// module's own authored code -- see the NEW-4-PERF note below for why it
// matters more now that the file-suffix filter (*.go only) is gone: a
// dotted tool-state directory can hold arbitrarily large, frequently
// rewritten, build-irrelevant files (a local agent/editor cache database,
// logs, lockfiles) that a *.go-only walk never touched at all (nothing in
// them ends in ".go") but a some-content-not-touched, all-files walk would
// otherwise read and hash on EVERY hashPackageInputs call -- a real,
// measured perf regression (a single dotted directory holding a ~80MB cache
// file pushed one whole-module hash from single-digit milliseconds to
// dominating a test run) that is not a hypothetical: it was caught by
// running this fix's own mutation tests, plus a full `go test ./...` pass,
// against the live self-hosting repository before landing. File CONTENTS
// are hashed, not mtimes/paths-only, so a touch-without-edit (e.g. a
// checkout that resets mtimes) never forces a spurious re-run, and a
// content-identical rewrite never causes a false cache miss. Relative paths
// (not absolute) are hashed alongside each file's content, with forward
// slashes on every OS, so the digest is stable across machines/checkouts and
// a file rename is still observed as a real content-relevant change.
//
// NEW-4 (@fh final adversarial re-review, "non-.go inputs never invalidate
// the in-memory verdict cache"): the walk used to filter to files whose name
// ends in ".go" ONLY, on the theory that `go test` compiles Go source and
// nothing else. That is unsound for any package whose test verdict also
// depends on a NON-.go file `go test` reads as part of running (not
// compiling): a //go:embed directive pulling in a golden fixture, a
// testdata/ file a test opens directly, or any other on-disk input a test
// function's own logic consults at run time. Such a file can change the
// test's PASS/FAIL verdict exactly as an impl.go edit can, but a
// *.go-suffix-only walk never observes it -- a cache entry keyed on the old
// digest is served back stale (a green verdict for a package that would now,
// if actually re-run, go red). This is the same shape as NEW-2 (a sibling
// package's behavior affecting the verdict without appearing in the hash),
// just for non-.go inputs instead of a different package directory, and gets
// the same fix: widen what gets hashed rather than try to enumerate exactly
// which files a given test happens to read (equivalent to NEW-2's rejected
// option (A) -- precise dependency analysis -- and rejected for the identical
// reasons: it would need per-test static analysis of //go:embed directives
// and file I/O calls to be exhaustive, is easy to get wrong as tests evolve,
// and the coarser whole-tree hash is already cheap for every module shape
// this engine supports).
//
// Fix: hash EVERY regular file under moduleRoot (recursive), not just those
// named *.go -- go.mod/go.sum are still hashed once, up front, by their own
// explicit read (kept as-is, harmless double coverage since the walk below
// also reaches them and hashing the same bytes twice under the same relative
// path is a no-op for cache-key purposes beyond a few extra sha256.Write
// calls). The SAME skip list applies (.git, vendor) -- broadening the file
// SUFFIX filter never broadens which directories are walked.
func hashPackageInputs(moduleRoot, pkgDir string) (string, error) {
	_ = pkgDir // NEW-2: cache key is now the whole module, not one package dir; see doc comment.
	h := sha256.New()

	for _, modFile := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(moduleRoot, modFile))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
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
			name := d.Name()
			if path != moduleRoot && (strings.HasPrefix(name, ".") || name == "vendor") {
				// Dotted directories (.git, .github, .crush, .vscode, any
				// other VCS/editor/tool-state directory a host environment
				// keeps at the module root) and "vendor" are never part of
				// the module's OWN package tree -- see the doc comment above
				// for why the dot-prefix rule (not just a literal ".git"
				// entry) is load-bearing now that every file is hashed, not
				// only *.go ones. The moduleRoot != path guard only matters
				// if moduleRoot itself were ever named starting with "."
				// (never true for a real go.mod-owning directory in
				// practice, but keeps the walk from vacuously skipping its
				// own root on a hypothetical dotted checkout path).
				return filepath.SkipDir
			}
			return nil
		}
		// NEW-4: hash every regular file, not just *.go -- a //go:embed
		// target, testdata/ golden file, or any other non-.go input a test
		// reads at run time can flip its verdict exactly as a .go edit can,
		// and must invalidate the cache the same way. Only the directory
		// skip-list (.git, vendor) bounds the walk now; there is no file-name
		// suffix filter left.
		if !d.Type().IsRegular() {
			// Skip symlinks/devices/etc: os.ReadFile on a non-regular entry
			// either follows a symlink (already reachable via its target
			// path elsewhere in the walk, or intentionally outside the
			// module) or fails outright -- neither is a "file whose content
			// affects go test" in the sense this hash needs to capture.
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
		return "", walkErr
	}

	sort.Strings(relPaths)
	for _, rel := range relPaths {
		h.Write([]byte(rel + "\n"))
		h.Write(fileData[rel])
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
