package gate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// compile_cache.go implements the BINARY-level compile cache that sits
// UNDERNEATH runGoTest/runGoTestRecording's existing verdict cache
// (runCache/singleflightRun) and RunVerifiedByTestRecording's deliberate
// lack of verdict memoization. It exists to close the same-package
// recompile penalty the verdict cache cannot help with: two DIFFERENT
// verified_by tests living in the SAME package (e.g. two requirements
// whose verified_by entries both point at internal/ontology/graph_smoke_
// test.go but at different TestXxx functions) have DIFFERENT verdict-cache
// keys (pkgDir+testName differs), so without this layer each one drove
// its OWN full `go test -run "^TestXxx$" ./pkg` subprocess invocation,
// recompiling+relinking the ENTIRE package's test binary from scratch
// every time, even when the previous call compiled exactly the same
// package seconds earlier.
//
// The fix is the standard Go pattern: compile the package's test binary
// ONCE via `go test -c -o <path> [-coverpkg=<pattern>] <pkgPattern>`,
// then invoke that compiled binary directly, repeatedly, with
// `<binary> -test.run "^TestName$" [-test.coverprofile=<path>]` -- NO
// recompilation on each invocation, just process startup + the actual
// test execution. This preserves the EXACT same per-test isolation the
// two functions already rely on for correctness: each invocation still
// runs exactly one named test, in its own process, with its own env vars
// for the recursion guard / HOTAM_RECORD_DIR. Only HOW the binary gets
// built changes.
//
// LAYERING vs. runCache/singleflightRun (the verdict cache):
//
//   - runCache memoizes the VERDICT (Passed/CompileFailed/Err) of one
//     (pkgDir, testName) pair, with content-hash invalidation -- a higher
//     layer that decides whether runGoTest gets called AT ALL. It is
//     UNCHANGED by this file. A caller whose verdict is cache-hit never
//     reaches runGoTest, and therefore never reaches this compile cache.
//   - THIS file memoizes the COMPILED ARTIFACT (the .test binary path)
//     keyed by (moduleRoot, pkgPattern, coverPkgPattern) -- a lower,
//     complementary layer. It activates only when runGoTest IS reached
//     (verdict cache miss) and serves the binary for the EXECUTION step.
//
// The two layers are independent: a verdict-cache hit short-circuits
// before this layer is consulted; a verdict-cache miss reaches this
// layer, which may itself be a compile-cache hit (cheap) or miss (one
// `go test -c`). RunVerifiedByTestRecording, which has NO verdict cache
// by design (every call is meant to produce a fresh run), benefits from
// this layer directly: two recording calls for two tests in the same
// package now share one compile.
//
// INVALIDATION (no content hash in the key, deliberately): unlike the
// verdict cache, this cache's key does NOT carry a whole-module content
// hash. Within ONE production run (one `hotam all-violations` / `hotam
// gen-spec --spec` invocation) the codebase does not recompile source
// between calls -- there is no mid-run edit to invalidate against, so a
// content-hash check would be pure cost (hashPackageInputs walks EVERY
// file under moduleRoot on every call) for zero benefit. The cache is
// instead PROCESS-LIFETIME only: ResetRunCacheForTest (the test-reset
// helper that already resets the verdict cache for the mutation tests)
// resets this cache too, and CleanupCompileCache (the end-of-run hook
// cmd/hotam's top-level command handlers defer) removes every compiled
// binary from disk. A test that mutates source mid-process
// (TestRunVerifiedByTest_MUTATION_*) calls ResetRunCacheForTest before
// its second pass, exactly as it already does for the verdict cache --
// see ResetRunCacheForTest's updated doc comment.

// compileCacheKey identifies one unit of COMPILATION: the (owning module,
// package pattern, coverage-package pattern) triple. coverPkgPattern is
// part of the key because Go bakes the -coverpkg instrumentation into the
// binary at COMPILE time, not run time -- a binary compiled without
// -coverpkg is NOT equivalent to one compiled with -coverpkg=A, and two
// binaries compiled with different -coverpkg values are NOT equivalent
// either. Conflating any of these would silently produce either no
// coverage profile or a profile instrumented over the wrong package.
type compileCacheKey struct {
	moduleRoot      string
	pkgPattern      string
	coverPkgPattern string
}

// compiledBinary is the cached outcome of ONE `go test -c` invocation:
// either the absolute path to a successfully-compiled .test binary (the
// cache-hit case the execution step invokes directly), OR a recorded
// compile failure (CompileFailed=true, with the captured output) so a
// broken package does not get retried on every subsequent call within
// the same process. err is non-nil only for transient infrastructure
// failures (the `go` binary itself missing, a timeout, a failure to
// create the tmp dir); such results are NOT stored in the cache, mirroring
// RunVerifiedByTest's own verdict-cache policy (only cache results that
// genuinely reflect the package's content, not transient host state).
type compiledBinary struct {
	path          string
	compileFailed bool
	output        string
	err           error
}

var (
	// compileTmpDir is the per-process tmp directory holding every
	// compiled .test binary this cache ever produced. Lazily created on
	// first use (compileTmpOnce), removed in full by CleanupCompileCache.
	// Mirrors RunVerifiedByTestRecording's own per-call
	// os.MkdirTemp("", "hotam-record-") convention (a tmp-scoped artifact
	// directory that never leaks past its owner's lifetime), just at the
	// per-process scale of the compile cache instead of the per-call
	// scale of one record dir.
	compileTmpDir  string
	compileTmpOnce sync.Once
	compileTmpErr  error

	// compileCache is the in-memory (moduleRoot, pkgPattern,
	// coverPkgPattern) -> *compiledBinary map. A sync.Map (not a
	// mutex+map) to mirror runCache's exact shape and to stay lock-free
	// on the hot read path (cache hits, the common case once warmed).
	compileCache sync.Map

	// compileSingleflightMu + compileInFlight mirror inFlightMu +
	// inFlightCalls's pattern (singleflightRun): at most one goroutine
	// actually compiles for a given key; concurrent callers for the same
	// key block on the SAME in-flight compile and reuse its result,
	// rather than each spawning its own `go test -c` subprocess. Without
	// this, N workers in check_verified_by_test_passes's pool reaching
	// the same cold key at once would each independently miss the cache
	// and each spawn their own compile -- exactly the thundering-herd
	// shape singleflightRun already prevents for the verdict layer.
	compileSingleflightMu sync.Mutex
	compileInFlight       = map[compileCacheKey]*compileInFlightCall{}

	// compileInvocations is a process-wide count of how many real
	// `go test -c` subprocesses this cache has actually spawned (cache
	// MISSES -- hits do not bump it). Exposed via CompileInvocationCount
	// for the cache-correctness tests (proving N tests in the same
	// package compile exactly ONCE, not N times).
	compileInvocations int64
)

// compileInFlightCall is one in-progress compile that other goroutines
// with the SAME key can wait on instead of starting their own. Mirrors
// inFlightCall's shape (without the hash field -- this cache's key has no
// content-hash component, see the file doc comment).
type compileInFlightCall struct {
	done chan struct{}
	bin  *compiledBinary
}

// CompileInvocationCount returns the number of real `go test -c`
// subprocess invocations this cache has performed process-wide (a cache
// MISS counter, never bumped on a hit). Test-only surface: production
// code never reads this; it exists so the cache-correctness tests can
// assert "two tests in the same package compile exactly once" without
// relying on wall-clock timing (which is flaky on a loaded CI host).
func CompileInvocationCount() int64 {
	return atomic.LoadInt64(&compileInvocations)
}

// CleanupCompileCache is the end-of-run hook the top-level command
// handlers that drive verified_by execution (cmd/hotam's cmdAllViolations
// and cmdGenSpec, the two CLI entry points whose call graph reaches
// RunVerifiedByTest/RunVerifiedByTestRecording against a real domain
// graph) call via defer to ensure no compiled .test binary survives past
// the run that created it. Mirrors the spirit of
// RunVerifiedByTestRecording's own per-call `defer os.RemoveAll(record
// Dir)`, just scoped to the WHOLE process's compile cache instead of
// one record dir. Idempotent and safe to call when no compile ever
// happened (e.g. a clean `hotam gen-spec` run without --spec, which
// never reaches this layer at all).
//
// Bounded by b014a63's no-disk-cache-of-verdicts discipline: this cache
// holds COMPILED ARTIFACTS only (binaries that reproduce verbatim what
// `go test` would build anyway), never VERDICTS -- so it cannot be
// forged into reporting a fake PASS the way the removed disk verdict
// cache could. A stale or hand-placed binary at the cache path cannot
// fabricate a verdict either: runGoTest/runGoTestRecording still
// actually EXECUTE the binary and read its REAL exit code, the same as
// they executed `go test`'s in-process build before; only the
// compile+link work is shared. The worst a tampered binary could do is
// run the wrong test binary, which `go test` itself would have built --
// and within one process run (the cache's entire lifetime) no source
// mutation happens, so "the cached binary" and "what `go test` would
// build now" are byte-identical by construction.
func CleanupCompileCache() {
	compileCache.Range(func(k, v any) bool {
		compileCache.Delete(k)
		return true
	})
	removeCompileTmpDir()
}

// resetCompileCacheForTest is the test-only reset called by
// ResetRunCacheForTest (which the existing mutation tests already call to
// reset the verdict cache). Clears the in-memory cache, drops any
// compiled binaries from disk, and resets the invocation counter -- so a
// test that mutates source mid-process and re-invokes RunVerifiedByTest
// observes a fresh compile rather than a stale cached binary built from
// the pre-mutation source.
func resetCompileCacheForTest() {
	compileCache.Range(func(k, v any) bool {
		compileCache.Delete(k)
		return true
	})
	removeCompileTmpDir()
	atomic.StoreInt64(&compileInvocations, 0)
}

// removeCompileTmpDir deletes compileTmpDir (if created) and resets the
// sync.Once so a subsequent compile re-creates a fresh dir. Safe to call
// when compileTmpDir was never created (compileTmpOnce never fired).
func removeCompileTmpDir() {
	d := compileTmpDir
	compileTmpDir = ""
	compileTmpOnce = sync.Once{}
	if d != "" {
		_ = os.RemoveAll(d)
	}
}

// invalidateCompileCacheForModule drops EVERY compile cache entry whose
// key carries the given moduleRoot, and removes the corresponding compiled
// .test binaries from disk. Called by RunVerifiedByTest on a verdict-cache
// hash mismatch (its existing content-hash mechanism -- hashPackageInputs
// hashes the WHOLE module, so a hash change means ANY cached binary for
// that module could be stale, not just the one for the named package).
//
// This is the compile cache's ONLY invalidation path, and it is FREE in
// production: source never changes within one CLI run (no mid-run edits),
// so RunVerifiedByTest's hash always matches its verdict-cache entry and
// this function is never reached. It exists solely so the existing
// mutation tests (which DO edit source mid-process between two
// RunVerifiedByTest calls) observe a fresh compile on the second call
// rather than a stale cached binary built from the pre-mutation source --
// the same correctness property the verdict cache's hash check already
// provides at its layer, mirrored here at the compile layer.
//
// RunVerifiedByTestRecording has NO content hash and therefore does NOT
// call this function: no existing test mutates source between two
// recording calls (the recording tests use fresh fixtures per test), and
// production never mutates mid-run, so a stale recording-path binary is
// never observable within one process. See the file doc comment.
func invalidateCompileCacheForModule(moduleRoot string) {
	cleaned := filepath.Clean(moduleRoot)
	var toDelete []compileCacheKey
	compileCache.Range(func(k, v any) bool {
		key := k.(compileCacheKey)
		if key.moduleRoot == cleaned {
			toDelete = append(toDelete, key)
			if bin, ok := v.(*compiledBinary); ok && bin.path != "" {
				_ = os.Remove(bin.path)
			}
		}
		return true
	})
	for _, k := range toDelete {
		compileCache.Delete(k)
	}
}

// compileTestBinary returns a *compiledBinary for the given (moduleRoot,
// pkgPattern, coverPkgPattern) triple, compiling it via `go test -c` on
// the first request for that triple and serving the cached result on
// every subsequent request for the same triple. coverPkgPattern is "" for
// the plain (no-coverage) verdict path used by runGoTest; non-empty
// values (the record-mode path, runGoTestRecording) are passed to
// `go test -c` as -coverpkg, which bakes the coverage instrumentation
// into the binary at compile time so that a later
// `-test.coverprofile=<path>` at run time actually instruments the
// intended package.
//
// The caller's ctx governs ONLY cancellation while waiting on an
// in-flight compile of the SAME key (singleflight). The compile step
// itself runs under its own compileTimeout and does NOT acquire
// globalExecSlots -- see doCompileTestBinary's doc comment for the
// concurrency decision.
func compileTestBinary(ctx context.Context, moduleRoot, pkgPattern, coverPkgPattern string) *compiledBinary {
	key := compileCacheKey{
		moduleRoot:      filepath.Clean(moduleRoot),
		pkgPattern:      pkgPattern,
		coverPkgPattern: coverPkgPattern,
	}
	if cached, ok := compileCache.Load(key); ok {
		return cached.(*compiledBinary)
	}
	return compileSingleflight(key, ctx, moduleRoot, pkgPattern, coverPkgPattern)
}

// compileSingleflight ensures at most ONE goroutine actually compiles a
// given key at a time; concurrent callers for the same key block on the
// SAME in-flight compile and receive its result. Mirrors singleflightRun's
// pattern (without the hash-mismatch loop -- this cache's key has no
// content-hash component, so there is no "the hash changed while we
// waited" case to fall through to).
func compileSingleflight(key compileCacheKey, ctx context.Context, moduleRoot, pkgPattern, coverPkgPattern string) *compiledBinary {
	compileSingleflightMu.Lock()
	if existing, ok := compileInFlight[key]; ok {
		compileSingleflightMu.Unlock()
		select {
		case <-existing.done:
			// Compile finished -- return its result (may be success or
			// compileFailed, both are valid cached outcomes).
			return existing.bin
		case <-ctx.Done():
			// Caller's ctx expired while waiting for an in-flight compile.
			// Do NOT return existing.bin (it may still be nil -- the
			// compile has not finished yet, so call.bin has not been
			// assigned). Return a distinct infrastructure error so the
			// caller's runGoTest/runGoTestRecording surfaces a timeout
			// rather than dereferencing a nil *compiledBinary.
			return &compiledBinary{err: fmt.Errorf("go test -c for %s: %w (caller cancelled while waiting for an in-flight compile of the same package)", pkgPattern, ctx.Err())}
		}
	}
	call := &compileInFlightCall{done: make(chan struct{})}
	compileInFlight[key] = call
	compileSingleflightMu.Unlock()

	bin := doCompileTestBinary(ctx, moduleRoot, pkgPattern, coverPkgPattern)
	call.bin = bin

	// Cache only if this is not an infrastructure error (transient --
	// should be retried, not poison the cache for the rest of the
	// process). Both successful compiles AND CompileFailed results
	// genuinely reflect the package's content and are safe to cache.
	if bin.err == nil {
		compileCache.Store(key, bin)
	}

	compileSingleflightMu.Lock()
	if compileInFlight[key] == call {
		delete(compileInFlight, key)
	}
	compileSingleflightMu.Unlock()
	close(call.done)
	return bin
}

// compileTimeout bounds a single `go test -c` invocation. Compiling is
// ordinarily fast (single-digit seconds for any package in any domain
// this engine currently supports), but a hung compiler must not pin the
// whole run. Deliberately INDEPENDENT of (and larger than) the per-test
// execution timeout (60s) the caller's ctx carries: a caller's 60s test
// deadline is meant to bound TEST execution, not compile -- a cold
// compile of a large package legitimately takes longer than running one
// test inside it, and consuming the test deadline on the compile would
// leave only fragments of the budget for the actual test. doCompileTest
// Binary creates its own context from context.Background() with this
// timeout so the caller's per-test ctx cannot prematurely abort the
// compile (a compile that completes is CACHED, so even if the caller's
// ctx expired mid-compile the work is not wasted -- a future caller
// benefits from the cache hit).
const compileTimeout = 180 * time.Second

// doCompileTestBinary performs the actual `go test -c` subprocess,
// classifies the outcome, and returns a *compiledBinary.
//
// CONCURRENCY DECISION: the compile step deliberately does NOT acquire
// globalExecSlots. An earlier version of this code bounded the compile by
// the SAME capacity-2 semaphore runGoTest/runGoTestRecording use for test
// execution, on the theory that `go test -c` is itself a full Go compile
// ("not free", per globalExecSlots's own doc comment). That was wrong:
// acquiring the slot ate into the caller's 60s per-test context budget
// (the SAME ctx that also bounds the subsequent execution step), so under
// heavy parallel load (go test ./... across many packages, each spawning
// subprocesses) a compile that waited N seconds for a slot left only
// 60-N seconds for execution -- and with 2 slots, a queue of compiles
// could push N past 60s, causing the execution step to see an
// already-expired ctx and report a spurious timeout. The Go build cache
// (GOCACHE) already serializes compiles of the same package internally
// and bounds total compile parallelism by GOMAXPROCS, so the host-load
// concern that justified globalExecSlots for EXECUTION (a compiled binary
// running a parallel test can use ALL of GOMAXPROCS) does not apply to
// compilation with the same force. The execution step continues to be
// bounded by globalExecSlots; the compile step is bounded only by the Go
// tool's own internal limits plus the compileTimeout context.
func doCompileTestBinary(ctx context.Context, moduleRoot, pkgPattern, coverPkgPattern string) *compiledBinary {
	if err := ensureCompileTmpDir(); err != nil {
		return &compiledBinary{err: fmt.Errorf("could not create compile-cache tmp dir: %w", err)}
	}

	binaryPath := filepath.Join(compileTmpDir, compileBinaryName(moduleRoot, pkgPattern, coverPkgPattern))

	// The compile runs under its own context (NOT the caller's per-test
	// ctx): see compileTimeout's doc comment.
	compileCtx, cancel := context.WithTimeout(context.Background(), compileTimeout)
	defer cancel()

	args := []string{"test", "-c", "-o", binaryPath, "-count=1"}
	if coverPkgPattern != "" {
		args = append(args, "-coverpkg="+coverPkgPattern)
	}
	args = append(args, pkgPattern)

	cmd := exec.CommandContext(compileCtx, "go", args...)
	cmd.Dir = moduleRoot
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	atomic.AddInt64(&compileInvocations, 1)
	err := cmd.Run()
	output := boundOutput(buf.String())

	if compileCtx.Err() == context.DeadlineExceeded {
		return &compiledBinary{output: output, err: fmt.Errorf("go test -c timed out compiling %s in %s: %w", pkgPattern, moduleRoot, compileCtx.Err())}
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !isExitError(err, &exitErr) {
			// `go` binary missing, cwd invalid, etc. -- infrastructure
			// failure, NOT a verdict. Do not cache (mirrors runGoTest).
			return &compiledBinary{output: output, err: fmt.Errorf("could not run go test -c: %w", err)}
		}
		// `go test -c` exits non-zero ONLY on a build failure -- unlike
		// `go test -run`, it never runs any tests, so a non-zero exit is
		// unambiguously a compile failure (not a test verdict). The
		// captured output (carrying the "# <importpath>" header and
		// compiler diagnostics, or the "[build failed]" trailer) is what
		// callers see in their violation message -- the SAME shape
		// runGoTest/runGoTestRecording's original looksLikeCompileFailure
		// branch already surfaced, just produced from the -c step instead
		// of the -run step. looksLikeCompileFailure is no longer needed
		// as a CLASSIFIER here (the exit code alone is authoritative for
		// `go test -c`), but the OUTPUT bytes are preserved verbatim so
		// the downstream diagnostic is byte-identical.
		return &compiledBinary{compileFailed: true, output: output}
	}
	// Exit 0: the binary should have been produced. Sanity-check it
	// actually exists at the requested -o path -- a misconfigured `go`
	// wrapper that swallows the exit code AND misplaces the artifact
	// would otherwise hand every downstream test-run caller a
	// nonexistent binary, producing a confusing "file not found"
	// infrastructure error instead of the real diagnostic.
	if _, statErr := os.Stat(binaryPath); statErr != nil {
		return &compiledBinary{output: output, err: fmt.Errorf("go test -c reported success but no compiled binary at %s: %w", binaryPath, statErr)}
	}
	return &compiledBinary{path: binaryPath, output: output}
}

// ensureCompileTmpDir lazily creates compileTmpDir on first use. Uses
// sync.Once so the first successful creation wins across all concurrent
// callers; a creation failure is recorded in compileTmpErr and surfaced
// to every caller until removeCompileTmpDir resets the Once (which
// ResetRunCacheForTest/CleanupCompileCache do).
func ensureCompileTmpDir() error {
	compileTmpOnce.Do(func() {
		d, err := os.MkdirTemp("", "hotam-compile-")
		if err != nil {
			compileTmpErr = err
			return
		}
		compileTmpDir = d
		compileTmpErr = nil
	})
	return compileTmpErr
}

// compileBinaryName returns a deterministic, filesystem-safe filename for
// one compiled binary inside compileTmpDir, derived from the same triple
// that keys the cache. The pkgPattern's slashes would be illegal in a
// filename on Windows, and two different (pkgPattern, coverPkgPattern)
// pairs could otherwise collide on a short hash, so the full triple is
// hashed (sha256, hex) -- two compiles for the same triple always produce
// the same filename (deterministic, so a cache-hit check could os.Stat
// the path directly), and two compiles for different triples never
// collide.
func compileBinaryName(moduleRoot, pkgPattern, coverPkgPattern string) string {
	h := sha256.New()
	h.Write([]byte(moduleRoot + "\n"))
	h.Write([]byte(pkgPattern + "\n"))
	h.Write([]byte(coverPkgPattern + "\n"))
	return hex.EncodeToString(h.Sum(nil)) + ".test"
}

// packageDirFromPattern converts a "./"-relative package pattern (as
// produced by relativePackagePattern) back into the absolute directory
// of that package, suitable as cmd.Dir for the directly-invoked compiled
// .test binary. `go test` chdirs into the test's own package directory
// before running it (so a test's os.ReadFile of a testdata/ relative
// path resolves to <packageDir>/testdata/, the package's own testdata
// tree -- the standard Go test layout); a directly-invoked .test binary
// does NOT chdir on its own, so the caller must set cmd.Dir to the
// package directory explicitly to preserve that behavior. pkgPattern
// == "./" maps to moduleRoot itself (a test living in the module root
// package).
func packageDirFromPattern(moduleRoot, pkgPattern string) string {
	rel := strings.TrimPrefix(pkgPattern, "./")
	rel = strings.TrimSuffix(rel, "/")
	if rel == "" {
		return moduleRoot
	}
	return filepath.Join(moduleRoot, filepath.FromSlash(rel))
}
