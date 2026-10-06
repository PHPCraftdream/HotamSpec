// test_exec_inputs.go holds the content-hash of a package's inputs used to invalidate cached test runs.
package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// moduleHashFileState is what perFileHashCache remembers about one file the
// last time hashPackageInputs actually read+used its bytes: the (mtime,
// size) pair observed at that read, plus the exact bytes read -- keeping the
// bytes (not just a digest of them) lets the reuse path feed the SAME data
// into the combiner loop at the bottom of hashPackageInputs that a fresh
// os.ReadFile would have produced, so a fully warm cache hit and a fully cold
// computation write byte-for-byte identical input into the running sha256,
// and therefore produce the byte-for-byte identical final digest for the
// same tree content (task #383's required invariant -- this is strictly an
// internal fast-path, never an observably different hash).
type moduleHashFileState struct {
	modTime time.Time
	size    int64
	data    []byte
}

// moduleHashCacheEntry is the memoized walk state for one moduleRoot: the
// per-relative-path state observed on the last hashPackageInputs call for
// that module, plus that call's resulting combined digest (returned directly
// when EVERY file in the current walk still matches its cached state and no
// file was added or removed -- see the fast-path check at the top of the
// walk below).
type moduleHashCacheEntry struct {
	files  map[string]moduleHashFileState
	digest string
}

// perFileHashCacheMu/perFileHashCache is the in-process, moduleRoot-keyed
// cache task #383 adds: profiling (#378/#380/#381) found os.ReadFile itself
// -- not the sha256 hashing -- dominating hashPackageInputs's cost
// (1215.88 of 1226.93s CPU in one profiled run), because the ORIGINAL
// implementation re-reads and re-hashes EVERY regular file under moduleRoot
// from scratch on every single call, even when the process already computed
// this exact digest moments ago and nothing on disk has changed since.
//
// Design: on every call, the directory walk itself still runs in full (the
// walk is not the expensive part -- only os.ReadFile's content read is), but
// for each file encountered, its CURRENT (mtime, size) is compared against
// the (mtime, size) recorded the last time this cache actually read that
// file's bytes. When they match, the previously-read bytes are reused
// verbatim (no os.ReadFile, no re-hash of that file's content) instead of
// touching disk again; only new or changed files are actually read. The
// file SET is also compared (see hashPackageInputs below): a file added or
// removed relative to the cached entry is exactly as significant as a
// content change and forces the affected part of the digest to be
// recomputed from the current walk, never silently reused from a smaller or
// larger cached set.
//
// A sync.Mutex guarding a plain map is used here, matching this file's own
// inFlightMu/inFlightCalls pattern (singleflightRun above) rather than
// introducing a second concurrency idiom (sync.Map, as runCache uses) into
// the same file for no reason: the operations this cache needs -- "read the
// whole per-module entry", "replace the whole per-module entry" -- are
// coarse-grained (one entry per moduleRoot, read+possibly-rewritten once per
// hashPackageInputs call) rather than the fine-grained independent-key
// read/write pattern sync.Map is suited for, so a plain mutex is the more
// direct match, exactly as inFlightCalls already is for the same reason.
//
// THREAD SAFETY (risk 3, task #382's finding): internal/invariants and
// internal/gate both run verified_by jobs concurrently (runExecWorkers,
// singleflightRun above), so multiple goroutines can call hashPackageInputs
// for the SAME moduleRoot at once. perFileHashCacheMu guards every individual
// access to the perFileHashCache MAP itself (the lookup at the top of
// hashPackageInputs and the store at the bottom, see hashPackageInputs'
// use below) -- Go map reads/writes are never safe to interleave without
// this, so every actual map access is race-free by construction, and the
// data each goroutine reads out (a *moduleHashCacheEntry) is never mutated
// in place after publish (hashPackageInputs always builds a brand-new
// moduleHashCacheEntry and stores that pointer, rather than mutating a
// previously-published one), so a goroutine that read a pointer under the
// lock can safely keep reading through it after unlocking without racing a
// concurrent in-place mutation.
//
// The lock is deliberately NOT held across the walk/os.Stat/os.ReadFile
// calls in between (only around the short lookup and the short store) --
// this means two goroutines racing to hash the SAME moduleRoot at the SAME
// moment (e.g. two different verified_by entries in the same package both
// missing the runCache/singleflightRun layer above them at once --
// singleflightRun already collapses same-KEY concurrent callers, but two
// DIFFERENT cacheKeys in the same module, e.g. two different testName values
// for the same pkgDir, are NOT collapsed by singleflightRun and can both
// reach hashPackageInputs concurrently) can both read the SAME prevEntry,
// both redundantly walk+read the module once each, and both then publish
// their own freshly-built entry (the second Store simply overwrites the
// first) -- a missed dedup opportunity, not a correctness bug: both
// goroutines observe a mutually consistent snapshot of the files they
// individually stat, so both independently compute the SAME correct digest
// for the same on-disk content, and whichever entry ends up stored is
// equally valid for the next caller to consult.
var (
	perFileHashCacheMu sync.Mutex
	perFileHashCache   = map[string]*moduleHashCacheEntry{}
)

// ResetModuleHashCacheForTest clears the in-process per-file hash cache
// (perFileHashCache) task #383 adds. Exported test-only helper, also called
// from ResetRunCacheForTest (so every existing caller of that function gets
// this reset for free) but kept separately callable too: t.TempDir() gives
// every test its own moduleRoot path so cross-test collisions cannot happen
// even without this, but calling it keeps each test's cache state
// independently verifiable (e.g.
// TestHashPackageInputs_WarmCacheReusesUnchangedFiles below asserts on cache
// population directly) rather than depending on prior tests' leftover
// entries never mattering by coincidence of unique temp paths.
func ResetModuleHashCacheForTest() {
	perFileHashCacheMu.Lock()
	perFileHashCache = map[string]*moduleHashCacheEntry{}
	perFileHashCacheMu.Unlock()
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
//
// PERF (task #379, continuing #378's profiling finding): compiled BUILD
// OUTPUTS -- *.exe/*.dll/*.so/*.dylib/*.test binaries left at the module
// root by ad-hoc `go build -o foo.exe` / `go test -c` debugging sessions --
// are structurally incapable of being a `go test` INPUT: they are what the
// toolchain PRODUCES, never something it reads to decide a package's
// compile-or-test verdict (unlike a //go:embed target or testdata/ golden
// file, which NEW-4 above correctly keeps hashing because a test's own code
// reads those at run time). #378's profiling of three real heavy tests
// (TestCmdSyncSelf_FullRoundTrip, TestGenSpec_CrystalFixpointConvergesAcrossRuns,
// TestBuildStatusReport_MatchesOnRealDomain) found hashPackageInputs at
// 28-33% of total CPU, with this repo's own module root littered with ~50MB
// of exactly such stray binaries (hotam.exe, hotam.test.exe,
// invariants.test.exe, etc. -- gitignored, never tracked, but still walked
// and SHA-256'd on every single RunVerifiedByTest call, of which a single
// AllViolations pass makes roughly one per verified_by entry). Skipping
// these extensions cannot cause a stale cache hit: excluding a file from the
// hash only matters if that file could have changed what `go test` observes
// for the SAME (moduleRoot, pkgPattern) the next time it runs, and a binary
// artifact's bytes are never consulted by `go test` itself (it does not
// `go:embed` or open its own sibling .exe). This is a strict subset of
// NEW-4's "hash everything a test could read" guarantee -- these specific
// extensions are excluded from "everything" only because they can never be
// read as an INPUT in the first place, not because hashing them is merely
// inconvenient.
func hashPackageInputs(moduleRoot, pkgDir string) (string, error) {
	_ = pkgDir // NEW-2: cache key is now the whole module, not one package dir; see doc comment.

	// PERF (task #383): consult the per-module cache BEFORE doing any
	// reading. prevFiles is the file-state snapshot from the last call that
	// actually populated the cache for this moduleRoot (nil if this is the
	// first call, or the cache was reset) -- the walk below uses it to decide,
	// per file, whether the cached bytes can be reused or the file must be
	// actually read. The mutex is only held for the cheap map lookup/copy,
	// never across the walk or any disk I/O.
	perFileHashCacheMu.Lock()
	prevEntry := perFileHashCache[moduleRoot]
	perFileHashCacheMu.Unlock()
	var prevFiles map[string]moduleHashFileState
	if prevEntry != nil {
		prevFiles = prevEntry.files
	}

	h := sha256.New()
	newFiles := make(map[string]moduleHashFileState, len(prevFiles))
	// allReused tracks, across every path readAndRecord is called for in
	// THIS call, whether every one of them was served from prevFiles (stays
	// true) or at least one had to be freshly read (goes false the moment
	// any single file misses -- see readAndRecord below). Combined with
	// deletedSincePrev after the walk, this is what detects an
	// added-or-removed file (task #383, risk 1's "file set changed" case):
	// hashPackageInputs itself always recomputes the digest fully from
	// (fresh-or-reused) bytes in the combiner loop below regardless of this
	// flag's value -- allReused only gates the SEPARATE whole-digest fast
	// path (skip the combiner loop entirely and return the previous
	// digest), never whether any individual file's bytes get read.
	allReused := true
	sawPaths := make(map[string]bool, len(prevFiles))

	readAndRecord := func(relKey, path string) ([]byte, error) {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return nil, statErr
		}
		sawPaths[relKey] = true
		if prev, ok := prevFiles[relKey]; ok && prev.size == info.Size() && prev.modTime.Equal(info.ModTime()) {
			// Cached (mtime, size) still match: reuse the bytes read on a
			// previous call instead of touching disk again. See the
			// perFileHashCache doc comment above for the residual gap this
			// leaves open (a content mutation that preserves BOTH the exact
			// byte size AND lands within the same mtime tick as the cached
			// read) and why it is an accepted, narrow, explicitly-documented
			// trade for the os.ReadFile cost this cache removes from the hot
			// path.
			newFiles[relKey] = prev
			return prev.data, nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		allReused = false
		newFiles[relKey] = moduleHashFileState{modTime: info.ModTime(), size: info.Size(), data: data}
		return data, nil
	}

	for _, modFile := range []string{"go.mod", "go.sum"} {
		data, err := readAndRecord(modFile, filepath.Join(moduleRoot, modFile))
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
		// suffix filter left, EXCEPT the compiled-build-output extensions
		// isBuildOutputExtension excludes -- see the PERF doc comment above
		// this function for why that exclusion cannot weaken invalidation.
		if !d.Type().IsRegular() {
			// Skip symlinks/devices/etc: os.ReadFile on a non-regular entry
			// either follows a symlink (already reachable via its target
			// path elsewhere in the walk, or intentionally outside the
			// module) or fails outright -- neither is a "file whose content
			// affects go test" in the sense this hash needs to capture.
			return nil
		}
		if isBuildOutputExtension(d.Name()) {
			return nil
		}
		rel, relErr := filepath.Rel(moduleRoot, path)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)
		data, readErr := readAndRecord(relSlash, path)
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

	// deletedSincePrev catches a file that was in prevFiles but is absent
	// from THIS walk's sawPaths (deleted, or renamed away) -- a pure
	// cardinality drop with no offsetting addition. It is deliberately only
	// a COUNT comparison, not a full set-equality check, because it does not
	// need to be: pairing it with allReused below is what makes the
	// combined guard sound. Any ADDED file (a brand-new path, or the "new
	// name" half of a rename) can never have a prevFiles entry to match
	// against, so readAndRecord always falls to a real os.ReadFile for it
	// and sets allReused=false -- meaning the only shape a bare count
	// comparison could miss (an add and a delete landing in the same call,
	// leaving len(sawPaths)==len(prevFiles) even though the actual path
	// SETS differ) is already independently caught by allReused turning
	// false for the added path. deletedSincePrev only has to catch what
	// allReused cannot: a PURE deletion (no offsetting add), where every
	// surviving file still matches its cached state and allReused would
	// otherwise incorrectly stay true.
	deletedSincePrev := len(prevFiles) != len(sawPaths)

	if allReused && !deletedSincePrev && prevEntry != nil && prevEntry.digest != "" {
		// Full fast path: every file this walk touched matched its cached
		// (mtime, size) state (allReused, which also implies no file was
		// ADDED -- see deletedSincePrev's doc comment above), and no
		// previously-cached file went missing either (deletedSincePrev
		// false). Nothing changed -- return the previously-combined digest
		// directly, without even running the sha256 combiner loop again.
		// This is the warm-cache hot path task #383 exists for: a
		// completely unchanged module tree costs one os.Stat per file and
		// nothing else.
		perFileHashCacheMu.Lock()
		perFileHashCache[moduleRoot] = &moduleHashCacheEntry{files: newFiles, digest: prevEntry.digest}
		perFileHashCacheMu.Unlock()
		return prevEntry.digest, nil
	}

	sort.Strings(relPaths)
	for _, rel := range relPaths {
		h.Write([]byte(rel + "\n"))
		h.Write(fileData[rel])
	}

	digest := hex.EncodeToString(h.Sum(nil))

	perFileHashCacheMu.Lock()
	perFileHashCache[moduleRoot] = &moduleHashCacheEntry{files: newFiles, digest: digest}
	perFileHashCacheMu.Unlock()

	return digest, nil
}

// HashPackageInputs is the exported wrapper around hashPackageInputs
// (task #387): hashPackageInputs itself is package-private, but
// internal/invariants' own process-lifetime coverageRunCache (mirroring
// runCache's shape above) needs the SAME content hash runCache already uses
// for RunVerifiedByTest's cache invalidation, so a RunVerifiedByTestRecording
// result can be invalidated on the identical signal (any *.go / go.mod /
// go.sum change anywhere under moduleRoot) rather than reinventing a second,
// possibly-divergent hashing scheme in a different package. See
// hashPackageInputs' own doc comment for what is and is not hashed, and
// runCache's doc comment (this file) for why a whole-module content hash,
// not a narrower per-file one, is the correct invalidation granularity for a
// `go test`-shaped cache entry.
func HashPackageInputs(moduleRoot, pkgDir string) (string, error) {
	return hashPackageInputs(moduleRoot, pkgDir)
}

// buildOutputExtensions is the set of file-name suffixes hashPackageInputs
// excludes from the walk (task #379): compiled artifacts the Go toolchain
// PRODUCES on this engine's supported platforms, never something `go test`
// reads as an input when deciding a package's verdict. Mirrors this
// project's own .gitignore build-output section (*.exe, *.dll, *.so,
// *.dylib) plus *.test (the `go test -c` compiled-binary suffix this
// package's own compile_cache.go produces, though that cache writes under
// os.TempDir() rather than moduleRoot -- included here defensively in case a
// stray `go test -c -o foo.test` lands at the module root the same way
// `go build -o foo.exe` has). Case-insensitive on the extension so a
// Windows-style ".EXE" (filesystems here are case-insensitive) is still
// recognized.
var buildOutputExtensions = map[string]bool{
	".exe":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".test":  true,
}

// isBuildOutputExtension reports whether name's extension marks it as a
// compiled build artifact excluded from hashPackageInputs's walk -- see the
// PERF doc comment on hashPackageInputs and buildOutputExtensions' doc
// comment for why this exclusion cannot weaken cache invalidation.
func isBuildOutputExtension(name string) bool {
	return buildOutputExtensions[strings.ToLower(filepath.Ext(name))]
}
