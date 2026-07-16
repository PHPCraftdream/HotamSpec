package gate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
}

// recursionGuardEnv is the environment variable RunVerifiedByTest sets to
// "1" on every `go test` child process it spawns, and checks for on its OWN
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
// that finds the marker already set knows it is nested and reports Skipped
// instead of spawning yet another generation.
const recursionGuardEnv = "HOTAM_VERIFIED_BY_EXEC_GUARD"

// inRecursionGuard reports whether the CURRENT process is already running
// inside a `go test` subprocess spawned by RunVerifiedByTest (see
// recursionGuardEnv).
func inRecursionGuard() bool {
	return os.Getenv(recursionGuardEnv) == "1"
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
	cmd.Env = append(os.Environ(), recursionGuardEnv+"=1")
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
func RunVerifiedByTest(specRoot, file, testName string) TestRunResult {
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

// hashPackageInputs computes a single SHA-256 digest over every file that
// can affect `go test`'s verdict for a package: go.mod and go.sum (if
// present) at moduleRoot, plus every *.go file directly inside pkgDir
// (non-recursive -- Go packages are single-directory; a sub-package is a
// separate compilation unit with its own hash). File CONTENTS are hashed,
// not mtimes, so a touch-without-edit (e.g. a checkout that resets mtimes)
// never forces a spurious re-run, and a content-identical rewrite never
// causes a false cache miss.
func hashPackageInputs(moduleRoot, pkgDir string) (string, error) {
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

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return "", err
	}
	var goFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".go") {
			goFiles = append(goFiles, name)
		}
	}
	sort.Strings(goFiles)
	for _, name := range goFiles {
		data, err := os.ReadFile(filepath.Join(pkgDir, name))
		if err != nil {
			return "", err
		}
		h.Write([]byte(name + "\n"))
		h.Write(data)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
