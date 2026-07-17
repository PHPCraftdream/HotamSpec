package gate

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	// report a violation for it.
	Skipped bool
	// InfraWarning is a non-fatal, non-empty diagnostic string set when
	// RunVerifiedByTest observed something suspicious about its OWN
	// execution environment that a caller may want to surface even though it
	// did not stop the test from actually running (NEW-1, @fh adversarial
	// re-review): specifically, a non-empty recursionGuardEnv present WITHOUT
	// the corroborating go-test-child signal (guardWasUntrustedExternal) --
	// i.e. an external actor exported HOTAM_VERIFIED_BY_EXEC_GUARD before
	// invoking hotam directly, which this process correctly did NOT honor as
	// proof of nesting (inRecursionGuard returned false, so the test below
	// still actually ran), but the attempt itself is worth reporting rather
	// than silently ignoring, so a caller (or an operator inspecting
	// all-violations output) can see that someone tried to pull the
	// kill-switch. InfraWarning is orthogonal to Err: it never prevents the
	// real test run and is set independently of Passed/CompileFailed/Skipped.
	InfraWarning string
}

// recursionGuardEnv is the environment variable RunVerifiedByTest sets, on
// every `go test` child process it spawns, to a per-process unguessable nonce
// (see guardNonce / NEW-1 below -- NOT a fixed literal like "1"), and checks
// for on its OWN process before spawning another. Self-hosting domains make
// recursion a structural certainty, not an edge case: domains/hotam-spec-
// self's graph names its OWN engine test files as verified_by targets (e.g.
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
// that finds a TRUSTED marker already set (see inRecursionGuard) knows it is
// nested and reports Skipped instead of spawning yet another generation.
const recursionGuardEnv = "HOTAM_VERIFIED_BY_EXEC_GUARD"

// NEW-1 (@fh adversarial re-review): the ORIGINAL guard trusted the mere
// PRESENCE of recursionGuardEnv="1" in the process's ambient environment as
// proof "I am a nested child RunVerifiedByTest itself spawned". That is a
// universal kill-switch any external actor can pull: `HOTAM_VERIFIED_BY_EXEC_
// GUARD=1 hotam all-violations` makes the TOP-level (non-nested) process
// itself believe it is already inside a guarded subprocess, so
// RunVerifiedByTest returns Skipped for every single verified_by entry --
// check_verified_by_test_passes reports zero violations no matter how broken
// the underlying implementations are, on a gutted tree, silently. The literal
// value "1" is guessable and requires no knowledge of anything this process
// generated, so no process can tell a genuine nested child from an outside
// forgery just by checking "is the var set". Two candidate corroborating
// signals were tried and rejected before landing on the one below:
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
// Fix actually used: a genuine nested child is distinguished from an
// external forgery by POSSESSION of an unguessable, freshly-minted
// crypto/rand nonce, corroborated by a MARKER FILE only the minting process
// itself could have written, in a location an external attacker exporting a
// guessed/fixed env value cannot predict or race:
//
//   - guardNonce() mints a fresh 32-byte (256-bit) crypto/rand hex token
//     exactly once per process (sync.Once), the first time it is needed
//     (either to check "am I nested" or to spawn a child). It is NEVER read
//     from the environment.
//   - Before this process spawns a `go test` child (runGoTest), it writes a
//     MARKER FILE at guardMarkerPath(nonce) -- inside diskCacheDir(), a
//     location already used as this engine's own shared scratch space -- and
//     ONLY THEN sets recursionGuardEnv to that same nonce in the child's
//     environment. The marker file's existence is what makes the nonce
//     "vouched for": an external actor who merely `export`s
//     HOTAM_VERIFIED_BY_EXEC_GUARD=<anything> before invoking hotam directly
//     would need to ALSO have independently guessed a 256-bit random hex
//     string AND pre-created a same-named marker file in this engine's own
//     scratch directory -- computationally infeasible, and not what a naive
//     `HOTAM_VERIFIED_BY_EXEC_GUARD=1 hotam ...` kill-switch attempt (@fh's
//     literal reproduction) does or could do.
//   - inRecursionGuard checks BOTH: recursionGuardEnv is non-empty, AND a
//     marker file exists on disk at guardMarkerPath(that exact value). Only
//     when both hold does this process treat itself as genuinely nested and
//     report Skipped. A bare "1" (or any value with no corresponding marker
//     file) fails the second check and is NOT honored as recursion --
//     RunVerifiedByTest runs the test for real and records an InfraWarning
//     instead of silently behaving as if nothing were amiss. See
//     guardWasUntrustedExternal.
//   - Marker files are cheap, small (empty), and self-cleaning is
//     unnecessary for correctness: a stale marker for an old nonce a future
//     process might coincidentally regenerate is astronomically unlikely
//     (256 bits of entropy) to collide, so leftover marker files from past
//     runs are harmless clutter, not a correctness or security hazard.
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

// guardMarkerPath returns the on-disk path this process (or an ancestor)
// writes to VOUCH for a guard nonce it minted, before ever placing that nonce
// into a spawned child's environment -- see the NEW-1 block comment above.
// Lives under diskCacheDir() (already this engine's own shared scratch
// space), namespaced with a "guard-" prefix distinct from the verified-by
// result cache files diskCacheFileName produces, so the two never collide.
func guardMarkerPath(nonce string) string {
	return filepath.Join(diskCacheDir(), "guard-"+nonce+".marker")
}

// writeGuardMarker vouches for nonce by writing an (empty) marker file at
// guardMarkerPath(nonce), creating diskCacheDir() first if needed. Called
// exactly once per process, before that process's own nonce is ever placed
// into a child's environment (runGoTest) -- see the NEW-1 block comment.
// Errors are swallowed deliberately: if the marker cannot be written (e.g. a
// read-only temp dir), the resulting child will simply fail the marker check
// and run for real instead of Skipping -- a safe, non-silent failure mode
// (an extra, harmless re-run) rather than a hazard, so this must never panic
// or propagate an error that would abort the real work RunVerifiedByTest is
// trying to do.
func writeGuardMarker(nonce string) {
	dir := diskCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(guardMarkerPath(nonce), []byte{}, 0o644)
}

// guardMarkerExists reports whether nonce was vouched for by a
// writeGuardMarker call from SOME process (this one or an ancestor) --
// the corroborating signal inRecursionGuard requires before trusting a
// non-empty recursionGuardEnv as genuine nesting.
func guardMarkerExists(nonce string) bool {
	if nonce == "" {
		return false
	}
	_, err := os.Stat(guardMarkerPath(nonce))
	return err == nil
}

// inRecursionGuard reports whether the CURRENT process should treat itself as
// ALREADY running inside a `go test` subprocess RunVerifiedByTest itself
// spawned. True only when recursionGuardEnv is non-empty AND a marker file
// vouching for that exact value exists on disk (guardMarkerExists) -- a
// non-empty recursionGuardEnv WITHOUT a corresponding marker is untrusted (an
// external actor's `export HOTAM_VERIFIED_BY_EXEC_GUARD=1` before invoking
// the hotam binary directly has no way to also predict a 256-bit nonce AND
// pre-create its marker file), so this process runs its own verified_by
// checks for real rather than silently Skipping every one of them. See
// guardWasUntrustedExternal for the caller-facing signal RunVerifiedByTest
// uses to additionally warn (rather than stay silent) when an untrusted
// guard-env was observed and ignored.
func inRecursionGuard() bool {
	v := os.Getenv(recursionGuardEnv)
	if v == "" {
		return false
	}
	return guardMarkerExists(v)
}

// guardWasUntrustedExternal reports whether recursionGuardEnv was observed
// non-empty on this process WITHOUT a corresponding marker file
// (guardMarkerExists) -- exactly the external-forgery shape NEW-1 targets:
// `HOTAM_VERIFIED_BY_EXEC_GUARD=<anything> hotam all-violations` invoked
// directly, with no genuine ancestor RunVerifiedByTest call ever having
// minted and vouched for that value. RunVerifiedByTest uses this to attach an
// infra-warning to its result instead of staying silent, so the caller can
// surface "an external guard-env was present but NOT honored as recursion"
// rather than behaving identically to the case where no guard-env was
// present at all.
func guardWasUntrustedExternal() bool {
	v := os.Getenv(recursionGuardEnv)
	return v != "" && !guardMarkerExists(v)
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
	// literal -- see guardNonce's doc comment / NEW-1) so the spawned `go
	// test` child, and anything it in turn runs, can recognize it is nested.
	// writeGuardMarker vouches for the nonce on disk BEFORE it is ever placed
	// into the child's environment, which is what lets the child's
	// inRecursionGuard trust it (guardMarkerExists) instead of trusting the
	// env var's mere presence.
	nonce := guardNonce()
	writeGuardMarker(nonce)
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
// it). Deliberately does NOT wipe diskCacheDir: that directory is SHARED,
// CROSS-PROCESS state (os.TempDir()/hotam-verified-by-cache/) that other
// concurrent processes on the same machine -- another test binary, a real
// `hotam` invocation, potentially another agent's session entirely on a
// shared dev box -- may be reading or writing at the same moment; nuking it
// out from under them would be a correctness hazard for THEM, not a
// convenience for this process. It is also unnecessary for THIS process's
// own correctness: every disk-cache entry is content-hash-keyed (hash +
// testName), and every fixture in this package's tests that shares source
// CONTENT (e.g. passingImplSrc/passingTestSrc reused across several tests)
// also embeds a DIFFERENT module path in its own go.mod -- which
// hashPackageInputs hashes as part of the key -- so distinct fixtures never
// collide on the same disk-cache file even without clearing it between
// tests.
func ResetRunCacheForTest() {
	runCache = sync.Map{}
}

// diskCacheDir is the SHARED, CROSS-PROCESS on-disk cache location:
// os.TempDir()/hotam-verified-by-cache/. This is what makes design decision
// A (PLAN-authored-spec-discipline.md §6) actually deliver "an unchanged
// spec is not re-run" across process boundaries, not just within one
// process's lifetime: cmd/hotam's own e2e test suite builds ONE shared
// `hotam` binary (testbinary_test.go's buildSharedHotamBinary) and spawns it
// as a SEPARATE OS process from dozens of t.Parallel() tests, each against
// its own copy of the SAME real self-hosting domain graph (copySelfDomain --
// identical graph.json content, so identical verified_by entries resolving
// against the SAME real engine packages via gate.engineRoot's CWD-fallback
// walk). Without a cross-process cache, every one of those independently
// spawned `hotam.exe` processes pays its own COLD `go test` compile for the
// same real engine packages, all at once -- observed to blow past `go
// test`'s own default 30s per-package timeout for the cmd/hotam suite under
// load. A shared disk cache lets the FIRST process to prove a given
// (content hash, test name) write the result once; every other process
// (including nested guarded subprocesses, though those never reach this
// path -- see inRecursionGuard) reads it back without spawning anything.
func diskCacheDir() string {
	return filepath.Join(os.TempDir(), "hotam-verified-by-cache")
}

// diskCacheFileName maps (hash, testName) to a filesystem-safe cache file
// name: the content hash is already a hex SHA-256 (filesystem-safe on every
// OS); testName is a valid Go identifier (enforced upstream by
// ResolveSpecTest's isRealTestSignature/Test*-prefix checks before
// RunVerifiedByTest is ever called), so neither needs further escaping.
func diskCacheFileName(hash, testName string) string {
	return hash + "__" + testName + ".json"
}

// diskCacheEntry is the on-disk JSON shape -- a strict subset of
// TestRunResult (Skipped is deliberately NOT persisted: a Skipped result is
// meaningless outside the recursion-guarded process that produced it, and
// must never be read back as if it were a real verdict for an unguarded
// caller).
type diskCacheEntry struct {
	Passed        bool   `json:"passed"`
	CompileFailed bool   `json:"compile_failed"`
	Output        string `json:"output"`
}

// loadDiskCache reads a cached TestRunResult for (hash, testName) from
// diskCacheDir, if present. Returns ok=false on any error (missing file,
// corrupt JSON, concurrent-write torn read) -- a disk cache miss always
// falls through to a real `go test` run, never blocks or errors out the
// caller; this is a pure performance optimization, and treating any
// uncertainty as a miss keeps it that way.
func loadDiskCache(hash, testName string) (TestRunResult, bool) {
	path := filepath.Join(diskCacheDir(), diskCacheFileName(hash, testName))
	data, err := os.ReadFile(path)
	if err != nil {
		return TestRunResult{}, false
	}
	var entry diskCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return TestRunResult{}, false
	}
	return TestRunResult{Passed: entry.Passed, CompileFailed: entry.CompileFailed, Output: entry.Output}, true
}

// storeDiskCache writes result to diskCacheDir under (hash, testName),
// via a temp-file-then-rename so a concurrent reader from ANOTHER process
// never observes a partially-written file (os.Rename is atomic on both
// POSIX and Windows when source and destination are on the same volume,
// which they always are here -- both under diskCacheDir). Errors are
// swallowed (best-effort cache write, same reasoning as loadDiskCache: this
// is a pure performance optimization that must never fail the caller).
func storeDiskCache(hash, testName string, result TestRunResult) {
	dir := diskCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	entry := diskCacheEntry{Passed: result.Passed, CompileFailed: result.CompileFailed, Output: result.Output}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	finalPath := filepath.Join(dir, diskCacheFileName(hash, testName))
	tmp, err := os.CreateTemp(dir, "tmp-*")
	if err != nil {
		return
	}
	tmpPath := tmp.Name()
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		os.Remove(tmpPath)
		return
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
	}
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
		return TestRunResult{Skipped: true}
	}

	// NEW-1: recursionGuardEnv was present but NOT honored as proof of
	// nesting (guardWasUntrustedExternal -- no corroborating go-test-child
	// signal). This process proceeds to actually run the test below (never a
	// silent Skip), but the attempted external guard-env is worth surfacing:
	// prefixed onto whatever InfraWarning the real run below produces (there
	// is none yet at this point, so this sets the initial value).
	if guardWasUntrustedExternal() {
		infraWarning := fmt.Sprintf(
			"%s was set in the environment but NOT honored as a recursion signal (this process shows no evidence of being a go-test-spawned child) -- ignored, the test below actually ran",
			recursionGuardEnv)
		// Stamp the warning onto whichever result this function ends up
		// returning (cache hit, disk-cache hit, or a fresh run) via the
		// named return value -- see the deferred stamp below.
		defer func() {
			if out.InfraWarning == "" {
				out.InfraWarning = infraWarning
			} else {
				out.InfraWarning = infraWarning + "; " + out.InfraWarning
			}
		}()
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

	// Cross-process fallback (diskCacheDir's doc comment): another PROCESS
	// entirely -- e.g. a sibling `hotam.exe` spawned by a different
	// t.Parallel() test in cmd/hotam's own e2e suite, working against its
	// own copy of the same real self-hosting domain graph -- may have
	// already proven this exact (content hash, test name) and written the
	// result to disk. A hit here skips the subprocess spawn entirely,
	// AND populates the in-memory cache so this process's own subsequent
	// calls for the same key are free too.
	if result, ok := loadDiskCache(hash, testName); ok {
		runCache.Store(key, cacheEntry{hash: hash, result: result})
		return result
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
		storeDiskCache(hash, testName, result)
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
// and go.sum (if present) at moduleRoot, plus every *.go file anywhere under
// moduleRoot (recursive). pkgDir is accepted for API/call-site compatibility
// (existing callers, including this package's own tests, pass the test's
// specific package directory) but is otherwise UNUSED for hashing purposes --
// see the NEW-2 doc comment below for why the cache key intentionally widened
// from "this one package directory" to "the whole owning module".
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
// Skips VCS/build-cache directories (.git, and any directory literally named
// "vendor" -- a vendored dependency's source never affects THIS module's own
// behavior in a way relevant to re-running ITS tests, and vendor trees can be
// large) so the walk stays bounded to the module's own authored code. File
// CONTENTS are hashed, not mtimes/paths-only, so a touch-without-edit (e.g. a
// checkout that resets mtimes) never forces a spurious re-run, and a
// content-identical rewrite never causes a false cache miss. Relative paths
// (not absolute) are hashed alongside each file's content, with forward
// slashes on every OS, so the digest is stable across machines/checkouts and
// a file rename is still observed as a real content-relevant change.
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
		return "", walkErr
	}

	sort.Strings(relPaths)
	for _, rel := range relPaths {
		h.Write([]byte(rel + "\n"))
		h.Write(fileData[rel])
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
