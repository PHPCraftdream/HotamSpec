package gate

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// testExecTimeoutEnv, when set in the process's environment to a Go
// time.ParseDuration string (e.g. "120s", "3m"), overrides the default
// per-test execution timeout (defaultTestExecTimeout) that bounds BOTH
// (a) the wall-clock budget a verified_by test's compiled binary gets to RUN
// once it has a globalExecSlots slot (the execCtx runGoTest/
// runGoTestRecording mint for cmd.Run), and (b) the budget the RunVerifiedBy
// Test caller's own ctx gives to the steps that PRECEDE execution -- waiting
// on an in-flight compile (compileSingleflight) and waiting on a
// globalExecSlots slot. Unset or unparseable → defaultTestExecTimeout.
//
// Exists because the ORIGINAL fixed 60s budget was sized for an unloaded box
// and was observed (tasks #350/#340/#341-342 verifications) to spuriously
// expire under heavy parallel load: a full `go test ./...` run fans out many
// t.Parallel() tests, several of which EACH spawn `hotam`/`go test`
// subprocesses that in turn call RunVerifiedByTest; the resulting CPU + Go
// build-cache contention pushed individual subprocess `go test` invocations
// past 60s (especially the slot-wait + cold-compile portion), producing
// non-deterministic Err results that (correctly) are NOT memoized in runCache
// (RunVerifiedByTest only caches result.Err==nil outcomes) -- which made
// AllViolations return DIFFERENT violation counts across two calls inside the
// SAME process (e.g. TestBuildStatusReport_MatchesOnRealDomain calls
// buildStatusReport -> AllViolations AND AllViolations separately, and a
// timeout in one call but not the other mismatched ViolationCount). Making the
// budget both (1) larger by default and (2) operator-tunable lets a loaded CI
// runner or a heavy local `go test ./...` extend it, restoring determinism,
// while a genuine infinite-loop test still surfaces within the bound.
const testExecTimeoutEnv = "HOTAM_VERIFIED_BY_EXEC_TIMEOUT"

// defaultTestExecTimeout is the per-test execution timeout used when
// testExecTimeoutEnv is unset or unparseable. Deliberately EQUAL to
// compileTimeout (180s): under heavy parallel load a compiled test binary's
// wall-clock execution can approach cold-compile time due to CPU contention
// from the outer `go test ./...` fan-out, so the execution bound needs the
// same headroom the compile bound already grants (see compileTimeout's doc
// comment for the load rationale). A genuine hang still surfaces within this
// bound; a loaded run no longer spuriously times out. Operator-tunable via
// testExecTimeoutEnv when even 180s is too tight (or too loose) for a given
// host.
const defaultTestExecTimeout = 180 * time.Second

// testExecTimeout returns the per-test execution timeout, honoring
// testExecTimeoutEnv when set to a parseable positive Go duration, otherwise
// defaultTestExecTimeout. Read fresh on every RunVerifiedByTest /
// RunVerifiedByTestRecording invocation (NOT process-cached) so an operator
// or a test can change it between calls if needed (e.g. a test that wants to
// exercise the timeout-error path sets a tiny value for one call).
func testExecTimeout() time.Duration {
	if v := os.Getenv(testExecTimeoutEnv); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return defaultTestExecTimeout
}

// runGoTest invokes the named test against a pre-compiled test binary
// (compileTestBinary, the cache layer that compiles `go test -c` ONCE per
// (moduleRoot, pkgPattern, coverPkgPattern) triple and reuses it across
// every test in the same package), and classifies the result. pkgPattern
// is the "./..."-relative import pattern for the package directory (e.g.
// "./internal/ontology/" for a self-hosting entry, or "./model/" for an
// authored spec/ package) -- callers compute it via
// relativePackagePattern. The spawned process carries recursionGuardEnv
// so IT (or anything it in turn runs) knows not to spawn a further nested
// invocation -- see recursionGuardEnv's doc comment. Acquires a
// globalExecSlots slot before spawning and releases it after the subprocess
// exits, so this process never has more than a small bounded number of
// test-execution children running concurrently no matter how many
// independent callers invoke it at once. The compile step
// (compileTestBinary, on a cache miss) is ALSO bounded by globalExecSlots
// for the same host-load reason -- see doCompileTestBinary's doc comment.
func runGoTest(ctx context.Context, moduleRoot, pkgPattern, testName string) TestRunResult {
	// Step 1: get-or-compile the binary for this package (no coverpkg for
	// the plain verdict path -- the caller did not ask for coverage). This
	// is the optimization: across N tests in the same package, the compile
	// happens ONCE; subsequent calls get a cache hit and skip straight to
	// the execution step. A CompileFailed result here is surfaced with the
	// SAME classification runGoTest originally produced inline (via the
	// captured `go test -c` output, byte-identical to what `go test -run`
	// would have printed on the same broken package).
	bin := compileTestBinary(ctx, moduleRoot, pkgPattern, "")
	if bin.err != nil {
		return TestRunResult{Output: bin.output, Err: bin.err}
	}
	if bin.compileFailed {
		return TestRunResult{Passed: false, CompileFailed: true, Output: bin.output}
	}

	// Step 2: invoke the cached binary directly. Same per-test isolation
	// as before -- ONE subprocess per test, in its own process, with its
	// own env. The binary was compiled with the package's whole test
	// entry; -test.run "^TestName$" selects exactly one test out of it.
	//
	// The slot-wait below is bounded by the CALLER's ctx (which also bounds
	// the in-flight compile singleflight wait in compileTestBinary above).
	// The EXECUTION itself (cmd.Run) runs under its OWN freshly-minted
	// execCtx, NOT the caller's ctx, so time spent QUEUING for a slot under
	// heavy parallel load cannot eat the test's execution budget -- the
	// exact structural problem doCompileTestBinary already solved for the
	// COMPILE step (see compile_cache.go's CONCURRENCY DECISION): there too,
	// acquiring globalExecSlots under the caller's ctx ate into the budget
	// for the step that actually does the work, and caused spurious timeouts
	// under load. The same decoupling now applies here for execution's own
	// slot-wait-vs-run split (task #352, FLAKY). execCtx's timeout comes from
	// testExecTimeout (configurable via testExecTimeoutEnv) so a loaded host
	// can extend it without recompiling.
	select {
	case globalExecSlots <- struct{}{}:
	case <-ctx.Done():
		return TestRunResult{Err: fmt.Errorf("go test for %s in %s: %w (timed out waiting for an execution slot)", testName, pkgPattern, ctx.Err())}
	}
	defer func() { <-globalExecSlots }()

	runPattern := "^" + testName + "$"
	execCtx, execCancel := context.WithTimeout(context.Background(), testExecTimeout())
	defer execCancel()
	cmd := exec.CommandContext(execCtx, bin.path,
		"-test.run", runPattern,
		"-test.count", "1",
	)
	// `go test` chdirs into the test's own package directory before
	// running it; a directly-invoked .test binary does not, so set
	// cmd.Dir to the package directory explicitly to preserve testdata/
	// and cwd-relative-path behavior. See packageDirFromPattern's doc.
	cmd.Dir = packageDirFromPattern(moduleRoot, pkgPattern)
	// Carry THIS process's own freshly-minted guard nonce (never a fixed
	// literal -- see guardNonce's / recursionGuardEnv's doc comments) so the
	// spawned test binary, and anything it in turn runs, can recognize it
	// is nested. No marker file is written: the child this env var reaches
	// is always a `go test`-compiled binary, never a cmd/hotam CLI process
	// (that only ever clears this var, see main()'s doc comment), so there
	// is no forgery surface left for a marker to defend against.
	nonce := guardNonce()
	cmd.Env = append(os.Environ(), recursionGuardEnv+"="+nonce)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	output := boundOutput(buf.String())

	if execCtx.Err() == context.DeadlineExceeded {
		return TestRunResult{
			Output: output,
			Err:    fmt.Errorf("go test timed out running %s in %s: %w", runPattern, pkgPattern, execCtx.Err()),
		}
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !isExitError(err, &exitErr) {
			// go binary missing, cwd invalid, etc. -- infrastructure failure,
			// not a test verdict.
			return TestRunResult{Output: output, Err: fmt.Errorf("could not run go test: %w", err)}
		}
		// A directly-invoked .test binary has ALREADY been compiled, so
		// looksLikeCompileFailure(output) should never fire here in
		// practice -- kept as a defensive classifier against a wrapped
		// `go`/binary path that could in theory produce compile-error-
		// shaped output at run time (e.g. a binary that re-checks build
		// constraints on launch). The common case is a plain test
		// failure (CompileFailed=false).
		compileFailed := looksLikeCompileFailure(output)
		return TestRunResult{Passed: false, CompileFailed: compileFailed, Output: output}
	}
	// Exit 0. Still scan for a "FAIL" line as belt-and-braces (a wrapped
	// `go` or a test harness oddity could theoretically exit 0 with FAIL
	// text); the exit code is authoritative for the common case.
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
	// Sync the compile cache against this hash: if the module changed since
	// the last sync (from either execution path), its stale compiled
	// binaries are dropped there. See syncCompileCacheToHash.
	syncCompileCacheToHash(moduleRoot, hash)

	key := cacheKey{pkgDir: absPkgDir, testName: testName}
	if cached, ok := runCache.Load(key); ok {
		entry := cached.(cacheEntry)
		if entry.hash == hash {
			return entry.result
		}
		// Hash mismatch: the module's content changed since this verdict
		// was cached (hashPackageInputs hashes the whole module -- any
		// *.go / non-.go file edit, anywhere under moduleRoot, moves the
		// hash). This branch now only skips the stale VERDICT; compile-cache
		// invalidation happens unconditionally at the syncCompileCacheToHash
		// call above, which fires whenever the module hash changed since the
		// last sync (for any cacheKey) -- a superset of this branch's old
		// trigger, which additionally required THIS key to hold a stale
		// entry. See syncCompileCacheToHash.
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
		// This ctx bounds only the PRE-execution waits an in-flight caller
		// can block on: the compile singleflight wait (compileTestBinary)
		// and the globalExecSlots slot-wait. The actual cmd.Run execution
		// gets its OWN freshly-minted execCtx inside runGoTest (decoupled
		// so slot-wait time cannot eat the run budget -- see runGoTest's
		// Step 2 comment), so this timeout and the run timeout are
		// INDEPENDENT budgets, each sized by testExecTimeout (configurable
		// via testExecTimeoutEnv, task #352).
		ctx, cancel := context.WithTimeout(context.Background(), testExecTimeout())
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
