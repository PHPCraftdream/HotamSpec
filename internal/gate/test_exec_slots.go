// test_exec_slots.go holds the execution semaphore, run-result memoization, and singleflight coordination.
package gate

import (
	"sync"
)

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

// ResetRunCacheForTest clears the in-memory verdict cache, the per-file
// module-hash cache (perFileHashCache, task #383), AND the binary compile
// cache (compile_cache.go), plus resets the compile-invocation counter.
// Test-only helper (exported so internal/invariants' tests, a different
// package, can call it). There is no on-disk/cross-process cache to also
// clear: see the removed-disk-cache history below (NEW-3) -- runCache and
// perFileHashCache (both in-memory only) are the ONLY caches
// RunVerifiedByTest/hashPackageInputs maintain, and both are process-lifetime
// only, so clearing them here is complete. Clearing perFileHashCache is not
// required for CORRECTNESS (a stale per-file entry only ever gates a
// same-value fast-path reuse or falls through to a real re-read -- see
// hashPackageInputs' own doc comment -- it can never make a genuinely
// changed file hash as unchanged except via the documented residual
// mtime-tick/same-size gap), but it is reset here anyway so each test's
// cache-population assertions (e.g.
// TestHashPackageInputs_WarmCacheReusesUnchangedFiles) start from a known
// empty state rather than depending on t.TempDir()'s per-test path
// uniqueness to make leftover entries harmless by coincidence.
//
// The compile cache reset (added by the binary-level compile cache,
// compile_cache.go) is needed for the SAME reason the verdict cache reset
// is: the cache's key does not carry a content hash, so a test that
// MUTATES source mid-process (the existing
// TestRunVerifiedByTest_MUTATION_* tests do exactly this -- they edit
// impl.go / policy/impl.go / threshold.txt between two RunVerifiedByTest
// calls in one test process) would otherwise observe a STALE cached
// binary built from the pre-mutation source on the second call. Folding
// the reset here (rather than asking every mutation test to call a
// SEPARATE resetCompileCacheForTest) keeps every existing mutation test
// correct with zero changes, the same way it already keeps the verdict
// cache correct. The compile cache's file doc comment documents this
// design decision in full.
func ResetRunCacheForTest() {
	runCache = sync.Map{}
	ResetModuleHashCacheForTest()
	resetCompileCacheForTest()
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
