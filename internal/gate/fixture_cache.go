package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// fixture_cache.go implements a STABLE, content-addressed directory for
// test-fixture Go modules, as a drop-in alternative to writing the same
// fixture tree into a fresh t.TempDir() on every test run.
//
// WHY THIS EXISTS (task #376, continuing #374's finding): the compiler
// embeds the fixture's ABSOLUTE SOURCE PATH as a literal argument on the
// `compile.exe ... -pack <path>/impl.go <path>/impl_test.go` command line
// (verified empirically in .scratch/task374-fixtureA-build.log vs
// task374-fixtureB-build.log: byte-identical fixture CONTENT, written to two
// different temp directories, produced two different -buildid values and
// two different cache entries). Go's build cache keys an action's result by
// (among other inputs) that exact command line, so a `t.TempDir()`-rooted
// fixture -- fresh, unique path on every `go test` PROCESS -- is a
// guaranteed cache MISS no matter how many times the identical fixture
// content is compiled, across the whole life of the machine. Writing the
// SAME content to a STABLE path lets two SEPARATE `go test` process
// invocations (the common case: the same test re-run, or two different
// tests that happen to build an identical fixture) share one Go build-cache
// entry.
//
// DESIGN, one paragraph per correctness question the task brief raised:
//
//  1. CONCURRENT WRITERS (two `go test` processes racing to populate the
//     same content-hash path at once): never write directly at the
//     published path. Build the full fixture tree in a PRIVATE, per-attempt
//     tmp directory first (os.MkdirTemp, unique by construction), write a
//     completeness marker file inside it LAST, then attempt a single
//     os.Rename of that tmp directory onto the published hash path. Two
//     racing writers each do this independently; exactly one os.Rename can
//     win (POSIX: atomic replace since the target does not yet exist so
//     there is nothing to replace; Windows: os.Rename onto an *existing*
//     directory fails outright -- verified empirically, "Access is denied"
//     -- rather than corrupting either side). The loser's os.Rename fails,
//     it discards (os.RemoveAll) its own private tmp copy, and both writers
//     converge on using the SAME published directory the winner created.
//     No lock file, no mutex across processes -- the filesystem rename
//     operation itself is the arbiter.
//
//  2. STALE / PARTIAL WRITE FROM A CRASHED PRIOR RUN: because the
//     completeness marker is written INSIDE the private tmp directory
//     BEFORE the rename, and the rename is the ONLY operation that makes
//     the content visible at the published path, there is no state in which
//     a reader can observe a published directory that is missing files --
//     either the rename happened (marker + every fixture file are already
//     on disk, atomically, as one directory-entry swap) or it did not
//     (nothing was ever published, the tmp directory is simply abandoned
//     under its own throwaway name and never contends for the published
//     path again). A process that crashes mid-write leaves an orphaned
//     `<hash>.tmp-*` sibling directory, never a half-written `<hash>`
//     directory -- so a SUBSEQUENT caller either finds a genuinely complete
//     `<hash>` directory (safe to reuse) or does not find one at all (falls
//     through to building its own tmp copy, exactly the cold-cache path).
//     EnsureContentAddressedFixture defensively re-checks for the marker
//     file even on what looks like a pre-existing hit, so a directory that
//     manually got created without ever going through this function's
//     publish step (e.g. hand-inspection during debugging) is never trusted
//     as complete.
//
//  3. UNBOUNDED GROWTH: unlike t.TempDir(), nothing removes these
//     directories automatically -- t.TempDir()'s whole point (per-test
//     cleanup) is deliberately given up here in exchange for cross-process
//     reuse, so SOME retention policy is required or the cache directory
//     grows forever. Accepted, explicit policy: best-effort ATIME/MTIME
//     eviction, run opportunistically (never blocking the caller's own
//     publish) on a small random fraction of calls, deleting any published
//     fixture directory whose completeness marker is older than
//     fixtureCacheMaxAge (7 days). This is deliberately coarse (age-based,
//     not size-based, not LRU) and deliberately best-effort (a failed prune
//     attempt -- e.g. another process mid-read -- is silently skipped, never
//     surfaced as a test failure): the cache directory holds only
//     regeneratable fixture content (never anything a human authored by
//     hand), so the worst case of a bad prune is a future cache miss, never
//     data loss. A resolver who wants stricter bounds (a size cap, an
//     explicit `hotam` subcommand to prune on demand) can layer that on
//     later; this function does not block that path, it just does not
//     implement it yet -- flagged explicitly here rather than silently
//     assumed away.
//
//  4. NO WEAKENED ASSERTIONS: this function's return contract is IDENTICAL
//     to t.TempDir()'s from the caller's point of view -- an absolute path
//     to a directory containing exactly the requested file tree, ready to
//     be used as `cmd.Dir`/a module root. Callers do not change what they
//     assert; only WHERE the bytes happen to live on disk changes. The
//     hash is computed over the EXACT file set + content the caller passes,
//     so any change to fixture content (even one byte) yields a different
//     path, never a stale hit.
const fixtureCacheDirName = "hotamspec-fixture-cache"

// fixtureCacheCompleteMarker is the sentinel file written LAST inside a
// fixture's private tmp directory (before the publish rename) and checked
// FIRST by any reader before trusting a `<hash>` directory as a genuine,
// fully-written cache hit. Its mere presence is the entire completeness
// contract -- see design point 2 above.
const fixtureCacheCompleteMarker = ".hotamspec-fixture-complete"

// fixtureCacheMaxAge bounds how long a published fixture directory survives
// before it becomes eligible for best-effort pruning -- see design point 3
// above. 7 days comfortably outlives any single work session while still
// keeping the cache from growing without bound across weeks of use.
const fixtureCacheMaxAge = 7 * 24 * time.Hour

// fixturePruneChance is the probability (1-in-N calls) that
// EnsureContentAddressedFixture also attempts an opportunistic prune sweep
// of the whole cache directory. Kept low so pruning's own filesystem walk
// never dominates the cost of what is meant to be a cheap path lookup on
// the common (already-published) case.
const fixturePruneChance = 20

// FixtureCacheRoot returns the stable parent directory every content-
// addressed fixture is published under: <os.TempDir()>/hotamspec-fixture-
// cache/. Exported so tests can inspect or clean it directly without
// duplicating the join.
func FixtureCacheRoot() string {
	return filepath.Join(os.TempDir(), fixtureCacheDirName)
}

// EnsureContentAddressedFixture publishes files (a map of SLASH-separated
// relative path -> content) under a stable directory keyed by the sha256 of
// the whole tree's content, and returns that directory's absolute path.
// Concurrent callers (including callers in SEPARATE OS processes) publishing
// the IDENTICAL file set converge on the SAME returned path; callers with
// even slightly different content get a different path, never a collision.
//
// onFail is invoked (with t.Fatalf semantics expected from the caller) only
// for genuine infrastructure errors (cannot create a tmp dir, cannot write a
// file) -- a losing writer in the concurrent-publish race is NOT a failure
// (see design point 1) and never reaches onFail.
func EnsureContentAddressedFixture(files map[string][]byte, onFail func(format string, args ...any)) (root string) {
	hash := hashFixtureTree(files)
	published := filepath.Join(FixtureCacheRoot(), hash)

	if fixtureCachePublishedComplete(published) {
		touchFixtureCacheHit(published)
		maybePruneFixtureCache()
		return published
	}

	if err := os.MkdirAll(FixtureCacheRoot(), 0o755); err != nil {
		onFail("EnsureContentAddressedFixture: could not create fixture cache root %s: %v", FixtureCacheRoot(), err)
		return published
	}

	tmpDir, err := os.MkdirTemp(FixtureCacheRoot(), hash+".tmp-")
	if err != nil {
		onFail("EnsureContentAddressedFixture: could not create private tmp dir under %s: %v", FixtureCacheRoot(), err)
		return published
	}
	// If publish (the rename below) succeeds, tmpDir no longer exists under
	// its own name (it WAS the rename source) so this cleanup is a no-op. If
	// publish loses the race or errors, this reclaims our private copy so
	// it never lingers as an orphaned .tmp-* sibling.
	defer os.RemoveAll(tmpDir)

	if err := writeFixtureTree(tmpDir, files); err != nil {
		onFail("EnsureContentAddressedFixture: could not write fixture tree into %s: %v", tmpDir, err)
		return published
	}
	// The completeness marker is written LAST, after every fixture file is
	// already flushed to tmpDir -- see design point 2: only a directory that
	// reaches this line ever becomes a rename source, so a reader can never
	// observe a published directory missing files.
	markerPath := filepath.Join(tmpDir, fixtureCacheCompleteMarker)
	if err := os.WriteFile(markerPath, []byte(hash+"\n"), 0o644); err != nil {
		onFail("EnsureContentAddressedFixture: could not write completeness marker in %s: %v", tmpDir, err)
		return published
	}

	if err := os.Rename(tmpDir, published); err != nil {
		// Rename failing here is the EXPECTED shape of losing a concurrent
		// publish race (see design point 1): another writer -- this process
		// earlier, or a different process entirely -- already published
		// `published` first. Trust it ONLY if it is genuinely complete;
		// otherwise this is a real, unexpected infra problem worth
		// surfacing (e.g. permissions), not a race to silently paper over.
		if fixtureCachePublishedComplete(published) {
			return published
		}
		onFail("EnsureContentAddressedFixture: rename %s -> %s failed and %s is not a complete published fixture: %v", tmpDir, published, published, err)
		return published
	}

	maybePruneFixtureCache()
	return published
}

// touchFixtureCacheHit bumps the completeness marker's mtime to "now" on a
// cache HIT, so a fixture that keeps getting reused (e.g. a test suite run
// daily over weeks) keeps surviving maybePruneFixtureCache's age-based
// sweep -- pruning is meant to reclaim fixtures that stopped being asked
// for, not ones still in active rotation. Best-effort: a failure (e.g. a
// permissions hiccup, or a concurrent pruner mid-delete) is silently
// ignored -- worst case this particular hit does not extend the fixture's
// lifetime and a future call pays one avoidable recompile, never a
// correctness problem.
func touchFixtureCacheHit(published string) {
	now := time.Now()
	_ = os.Chtimes(filepath.Join(published, fixtureCacheCompleteMarker), now, now)
}

// fixtureCachePublishedComplete reports whether dir is a published fixture
// directory that carries the completeness marker -- the ONLY signal this
// package trusts to treat a `<hash>` directory as safe to reuse without
// rewriting it (design point 2). A `<hash>` directory that exists but lacks
// the marker (which this function's own publish path can never produce, but
// a human poking around under os.TempDir() by hand still theoretically
// could) is treated as absent, not as a hit.
func fixtureCachePublishedComplete(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, fixtureCacheCompleteMarker))
	return err == nil
}

// hashFixtureTree computes a sha256 over files' relative paths AND content,
// sorted by path for a deterministic hash regardless of map iteration order.
// The path is included in the hash (not just content) so two fixtures whose
// concatenated bytes happen to collide under different file layouts are
// still distinguished; a length-prefix-free "path\n" + "len(content)\n" +
// content framing per entry avoids any ambiguity from a path or a content
// blob that itself happens to contain a delimiter byte sequence.
func hashFixtureTree(files map[string][]byte) string {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	h := sha256.New()
	for _, p := range paths {
		content := files[p]
		fmt.Fprintf(h, "path:%s\nlen:%d\n", p, len(content))
		h.Write(content)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// writeFixtureTree writes every entry of files (slash-separated relative
// path -> content) under root, creating parent directories as needed.
func writeFixtureTree(root string, files map[string][]byte) error {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, rel := range paths {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("MkdirAll %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, files[rel], 0o644); err != nil {
			return fmt.Errorf("WriteFile %s: %w", full, err)
		}
	}
	return nil
}

// maybePruneFixtureCache runs an opportunistic, best-effort eviction sweep
// with probability 1/fixturePruneChance -- see design point 3. Every failure
// mode inside the sweep is swallowed (a prune is a housekeeping nicety, not
// a correctness requirement of the fixture-cache contract): a directory this
// process cannot remove today (permissions, a concurrent reader) is simply
// left for a future sweep to retry.
func maybePruneFixtureCache() {
	if pruneRandN(fixturePruneChance) != 0 {
		return
	}
	root := FixtureCacheRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-fixtureCacheMaxAge)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		markerPath := filepath.Join(root, e.Name(), fixtureCacheCompleteMarker)
		info, err := os.Stat(markerPath)
		if err != nil {
			// No marker: either an in-flight private tmp dir (name carries
			// ".tmp-", never trusted) or a stray/incomplete directory this
			// package's own publish path never leaves behind. Skip it --
			// only mtime-eligible, MARKED-complete directories are ever
			// pruned automatically, so an in-flight writer's private tmp
			// directory is never at risk of being pruned out from under it.
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(root, e.Name()))
	}
}

// pruneRandN returns a pseudo-random int in [0, n) using a package-level
// math/rand source. This gate has no security relevance (it only decides
// whether a housekeeping sweep runs on a given call, never anything about
// which fixture path is trusted or published), so math/rand's weaker
// guarantees are an acceptable, simpler choice than crypto/rand here.
func pruneRandN(n int) int {
	return rand.Intn(n)
}
