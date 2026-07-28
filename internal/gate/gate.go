package gate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

type GateResult struct {
	Confident bool     `json:"confident"`
	NodeIDs   []string `json:"node_ids"`
	Reason    string   `json:"reason"`
}

var alwaysRun = []string{
	"TestRegistryComplete_AllViolationsOnRealGraphDoesNotPanic",
}

type testScan struct {
	checkToTests map[string][]string
	testFuncs    map[string]struct{}
}

func SelectTier1(targetAnchor string, g *ontology.Graph) GateResult {
	for _, r := range g.Requirements {
		if r.ID == targetAnchor {
			return selectFromRequirement(targetAnchor, r.EnforcedBy, g.DomainDir)
		}
	}
	for _, c := range g.Conflicts {
		if c.ID == targetAnchor {
			return GateResult{
				Confident: false,
				Reason: fmt.Sprintf(
					"target %q is a Conflict node — Conflict has no per-instance enforced_by; fail-closed to full suite.",
					targetAnchor),
			}
		}
	}
	return GateResult{
		Confident: false,
		Reason: fmt.Sprintf(
			"target %q not found in the current graph (new node, or a target outside Requirement/Conflict) — fail-closed to full suite.",
			targetAnchor),
	}
}

func selectFromRequirement(targetAnchor string, enforcedBy []string, domainDir string) GateResult {
	if len(enforcedBy) == 0 {
		return GateResult{
			Confident: false,
			Reason: fmt.Sprintf(
				"target %q has an empty enforced_by list — no targeted enforcer is known; fail-closed to full suite.",
				targetAnchor),
		}
	}

	scan, err := buildScan(domainDir)
	if err != nil {
		return GateResult{
			Confident: false,
			Reason: fmt.Sprintf(
				"target %q: could not scan internal test tree: %v — fail-closed to full suite.",
				targetAnchor, err),
		}
	}

	nodeSet := make(map[string]struct{}, len(alwaysRun))
	for _, n := range alwaysRun {
		nodeSet[n] = struct{}{}
	}
	var unresolved []string
	for _, entry := range enforcedBy {
		resolved := resolveOne(strings.TrimSpace(entry), scan)
		if resolved == nil {
			unresolved = append(unresolved, entry)
			continue
		}
		for _, n := range resolved {
			nodeSet[n] = struct{}{}
		}
	}

	if len(unresolved) > 0 {
		noun := entryNoun(len(unresolved))
		return GateResult{
			Confident: false,
			Reason: fmt.Sprintf(
				"target %q: %d enforced_by %s could not be resolved to a Go test function (%v) — fail-closed to full suite.",
				targetAnchor, len(unresolved), noun, unresolved),
		}
	}

	nodeIDs := keysSorted(nodeSet)
	return GateResult{
		Confident: true,
		NodeIDs:   nodeIDs,
		Reason: fmt.Sprintf(
			"target %q: resolved %d enforced_by %s to %d Go test function(s) (plus %d always-run).",
			targetAnchor, len(enforcedBy), entryNoun(len(enforcedBy)), len(nodeIDs), len(alwaysRun)),
	}
}

func resolveOne(entry string, scan *testScan) []string {
	if _, ok := scan.testFuncs[entry]; ok {
		return []string{entry}
	}
	if strings.HasPrefix(entry, "check_") {
		hits := scan.checkToTests[entry]
		if len(hits) == 0 {
			return nil
		}
		return append([]string(nil), hits...)
	}
	return nil
}

func BuildCheckToTestsMap(testsDir string) (map[string][]string, error) {
	scan, err := scanTestDir(testsDir)
	if err != nil {
		return nil, err
	}
	return scan.checkToTests, nil
}

func scanTestDir(testsDir string) (*testScan, error) {
	entries, err := os.ReadDir(testsDir)
	if err != nil {
		return nil, err
	}
	scan := &testScan{
		checkToTests: make(map[string][]string),
		testFuncs:    make(map[string]struct{}),
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(testsDir, entry.Name())
		if err := scanTestFile(path, scan); err != nil {
			return nil, err
		}
	}
	for k := range scan.checkToTests {
		sort.Strings(scan.checkToTests[k])
	}
	return scan, nil
}

func scanTestFile(path string, scan *testScan) error {
	parsed, err := parseTestFileCached(path)
	if err != nil {
		return err
	}
	for _, name := range parsed.testFuncs {
		scan.testFuncs[name] = struct{}{}
	}
	for _, hit := range parsed.checkHits {
		addUnique(scan.checkToTests, hit.checkName, hit.testName)
	}
	return nil
}

// checkHit is one (check_* string literal -> enclosing Test* function name)
// association found inside a single _test.go file, in first-seen order
// within that file -- the same association scanTestFile used to compute
// inline via ast.Inspect before extraction was pulled out into
// parseTestFileCached so its result could be cached.
type checkHit struct {
	checkName string
	testName  string
}

// parsedTestFile is everything scanTestFile/collectTestFuncNames actually
// consume out of one _test.go file's AST: the set of top-level Test*
// function names (sorted for a deterministic, order-independent cache
// value) and every check_* string-literal association scanTestFile's
// mechanism #2 needs. Caching THIS extracted shape, rather than the raw
// *ast.File, is deliberate: it is small, immutable, trivially safe to share
// across goroutines without any risk of a caller mutating a cached AST node
// out from under another reader (go/ast nodes are mutable Go values with no
// synchronization of their own), and it is exactly what both consumers need
// -- collectTestFuncNames only ever wanted testFuncs, so caching the fuller
// scanTestFile shape and letting collectTestFuncNames read only the
// testFuncs field costs it nothing extra.
type parsedTestFile struct {
	testFuncs []string
	checkHits []checkHit
}

// testFileParseCacheEntry is one memoized parseTestFileCached result, valid
// only for the EXACT (mtime, size) pair observed on disk at parse time --
// see parseTestFileCached for the invalidation check. mtime+size (rather
// than a content hash) mirrors the same cheap-stat-first design runCache's
// sibling caches in this package already use for compiled-binary and
// verdict invalidation (compile_cache.go/test_exec.go): a full content hash
// would be strictly more precise (immune to a same-size-same-mtime
// coincidence) but requires reading the whole file, defeating the point of
// caching the parse in the first place -- an os.Stat is the cheap
// pre-check, exactly like the existing siblings' hashPackageInputs step
// which the resolver still has to pay per-call regardless of a stat-level
// pre-check.
type testFileParseCacheEntry struct {
	modTime time.Time
	size    int64
	parsed  parsedTestFile
}

// testFileParseCache is a process-lifetime, in-memory cache of
// parseTestFileCached results, keyed by the file's ABSOLUTE path -- see
// parseTestFileCached's doc comment for why an absolute path (not the raw
// path argument, which may be relative and ambiguous across different
// callers' working directories) is the cache key, and why a sync.Map (the
// same concurrency-safe primitive runCache already uses in test_exec.go,
// chosen here for stylistic consistency with that sibling cache rather than
// a mutex-protected map) is safe for the concurrent walkTestFuncs/
// filepath.WalkDir callers that can reach this cache from multiple
// goroutines... walkTestFuncs itself is currently sequential
// (filepath.WalkDir's own callback is single-goroutine), but buildScan's
// two independent scans (scanTestDir + walkTestFuncs over two roots) are
// still both funneled through this single shared map, and a future caller
// (or a test exercising concurrent access directly, see
// TestParseTestFileCached_ConcurrentAccess_NoRace) may call
// parseTestFileCached from multiple goroutines at once, so the cache is
// built race-safe from the start rather than retrofitted later.
var testFileParseCache sync.Map // absolute path (string) -> testFileParseCacheEntry

// ResetTestFileParseCacheForTest clears the in-memory AST-parse cache.
// Test-only helper, mirroring ResetRunCacheForTest's shape in
// test_exec.go, exported so a test that mutates a _test.go file on disk and
// wants to force a fresh stat+parse (rather than relying on the mtime/size
// check alone, e.g. to isolate cache behavior from filesystem timestamp
// resolution) can reset cache state between assertions.
func ResetTestFileParseCacheForTest() {
	testFileParseCache = sync.Map{}
}

// parseTestFileCached returns the extracted Test*-name set and check_*
// literal associations for the _test.go file at path, either from the
// in-memory cache (when a previous parse of the exact same absolute path is
// still valid for the file's CURRENT mtime+size) or by parsing it fresh via
// go/parser and updating the cache entry.
//
// SAFETY OF CACHING WITHOUT INVALIDATING MID-PROCESS (see task brief): both
// call sites reachable from this cache -- scanTestFile (via scanTestDir,
// called only from BuildCheckToTestsMap/buildScan) and collectTestFuncNames
// (via walkTestFuncs, called from buildScan/TestFuncNames) -- are PURELY
// READ-ONLY directory walks over this repository's own internal/ and cmd/
// trees (see testFuncRoots/defaultInvariantsDir). Neither of gate.go's own
// two callers of these walks (buildScan, from SelectTier1/
// selectFromRequirement -- reached by `hotam gate` and by `hotam land`'s
// default T1 selector; and the package-external callers gate.TestFuncNames
// (internal/invariants/enforcement.go, AllViolations's enforced_by
// resolvability check) and gate.SelectTier1 (cmd/hotam/gate_cmd.go, the
// `hotam gate` subcommand)) ever WRITE to a _test.go file as part of the
// same call chain that scans it: `hotam gate`/`hotam land`'s T1 selector
// only ever READS the enforced_by tuple and the on-disk test tree to
// resolve it, never regenerating or rewriting _test.go sources itself, and
// AllViolations' enforced_by-resolvability check is likewise read-only over
// the same tree. This is a materially different risk shape from
// hashPackageInputs' own module-wide content hash (task #383's target):
// `hotam land`'s pipeline explicitly documents (see hashPackageInputs'
// cache-invalidation comment in test_exec.go) a real in-process
// write-then-rehash sequence against graph.json/graph.lock/generated docs
// WITHIN one `hotam land` run -- but that sequence never touches _test.go
// files, and no code path in this repository was found (via a grep for
// every call site of collectTestFuncNames/scanTestFile plus every
// PACKAGE-EXTERNAL caller of the two exported entry points that reach them)
// that writes a _test.go file and then, in the SAME process, re-scans that
// exact file expecting to observe the write. The mtime+size check below is
// still applied on every call (not skipped) as defense in depth: if a
// FUTURE caller ever does add such a sequence, or if an external process
// (an editor, `go generate`, a human) mutates a _test.go file between two
// `hotam` invocations that share a long-lived process (unusual for a CLI,
// but not assumed away here), the cache still detects the change and
// reparses rather than silently serving stale results forever.
func parseTestFileCached(path string) (parsedTestFile, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		// Fall back to the raw path as the cache key rather than failing the
		// whole scan outright -- filepath.Abs only fails when os.Getwd()
		// fails (an exotic environment issue), and the raw path is still a
		// perfectly usable (if less canonical) cache key in that case.
		absPath = path
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		return parsedTestFile{}, statErr
	}

	if cached, ok := testFileParseCache.Load(absPath); ok {
		entry := cached.(testFileParseCacheEntry)
		if entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
			return entry.parsed, nil
		}
		// mtime/size changed since this entry was cached -- fall through and
		// reparse; the entry below overwrites the stale one.
	}

	parsed, err := parseTestFileUncached(path)
	if err != nil {
		return parsedTestFile{}, err
	}
	testFileParseCache.Store(absPath, testFileParseCacheEntry{
		modTime: info.ModTime(),
		size:    info.Size(),
		parsed:  parsed,
	})
	return parsed, nil
}

// parseTestFileUncached does the actual go/parser.ParseFile call and AST
// walk -- the expensive step parseTestFileCached exists to avoid repeating
// for an unchanged file. Extraction logic (which decls count as a top-level
// Test* function, which string literals count as check_* associations) is
// UNCHANGED from the pre-cache implementation: this is a pure refactor of
// scanTestFile's original body into a cacheable, side-effect-free function
// that returns its findings instead of mutating a shared *testScan
// in place.
func parseTestFileUncached(path string) (parsedTestFile, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return parsedTestFile{}, err
	}
	var parsed parsedTestFile
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		if !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		parsed.testFuncs = append(parsed.testFuncs, fn.Name.Name)
		if fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if strings.HasPrefix(val, "check_") {
				parsed.checkHits = append(parsed.checkHits, checkHit{checkName: val, testName: fn.Name.Name})
			}
			return true
		})
	}
	return parsed, nil
}

func addUnique(m map[string][]string, key, val string) {
	for _, existing := range m[key] {
		if existing == val {
			return
		}
	}
	m[key] = append(m[key], val)
}

func defaultInvariantsDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Join(filepath.Dir(file), "..", "invariants")
}

// defaultInternalRoot returns the internal/ directory (the parent of
// internal/gate). It is one of two roots for the Test*-name half of
// resolution: mechanism #1 (an enforced_by entry that is a literal Test*
// function name) matches a Go test function ANYWHERE under
// internal/**/*_test.go OR cmd/**/*_test.go (see defaultCmdRoot and
// testFuncRoots), because a real test enforcer in internal/proposal,
// internal/generator, or cmd/hotam is just as load-bearing regardless of
// which directory it happens to live in. Mechanism #2 (the check_* literal ->
// tests map) stays scoped to internal/invariants via defaultInvariantsDir,
// because check_* string literals appear as real enforcer references only
// there; elsewhere (gate_test.go, ontology/query fixtures, cmd/hotam tests)
// they appear as TEST FIXTURE DATA, and widening the check_ scan to those
// would make fake names like "check_full" / "check_nonexistent_fake_check"
// falsely resolve. See resolveOne.
func defaultInternalRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Join(filepath.Dir(file), "..")
}

// defaultCmdRoot returns the cmd/ directory (a sibling of internal/, both
// children of the repo root). It is the second root for the Test*-name half
// of resolution (mechanism #1) — see defaultInternalRoot's doc comment for
// why widening mechanism #1 to include cmd/ is safe: mechanism #1 matches
// real `func Test<Name>(...)` declarations via Go AST parsing, never string
// literals, so cmd/hotam's *_test.go files (which contain no check_*-shaped
// string fixture data — verified during the widening that added this
// function) carry no fixture-pollution risk analogous to mechanism #2's.
// Mechanism #2 is NOT widened to cmd/ for that same reason stated the other
// way around: its risk model (string-literal matching) DOES apply there in
// principle, so it stays scoped to internal/invariants only.
func defaultCmdRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "cmd")
}

// testFuncRoots returns every root directory mechanism #1 (Test*-name
// resolution) walks: internal/ and cmd/ (HotamSpec's own tree — the only
// real roots the engine can independently verify by scanning its own
// checkout). Both buildScan and TestFuncNames call this so the two
// consumers can never drift on which roots count.
//
// A domain's own directory tree is deliberately NOT scanned here: the
// engine only trusts Go test functions it can find under its own internal/
// and cmd/ trees (plus, in the future, an authored spec/ root), never a
// generated or domain-local tree it cannot independently verify was
// produced honestly. See docs history for task #214 (which had widened this
// to a consumer domain's gen/go output) and its reversal (gen-code and the
// gen/go trust shift were removed entirely — the resolver no longer has any
// branch that trusts generated code).
func testFuncRoots() []string {
	return []string{defaultInternalRoot(), defaultCmdRoot()}
}

// buildScan constructs the resolver's combined view: the check_ literal map
// comes from internal/invariants only (scanTestDir), and the Test* function
// name set comes from ALL internal/**/*_test.go and cmd/**/*_test.go
// (walkTestFuncs over testFuncRoots). This split is what makes mechanism #1
// repo-wide while keeping mechanism #2 free of fixture pollution — see
// defaultInternalRoot and defaultCmdRoot.
func buildScan(domainDir string) (*testScan, error) {
	invScan, err := scanTestDir(defaultInvariantsDir())
	if err != nil {
		return nil, err
	}
	funcSet := make(map[string]struct{})
	for _, root := range testFuncRoots() {
		if err := walkTestFuncs(root, funcSet); err != nil {
			return nil, err
		}
	}
	return &testScan{checkToTests: invScan.checkToTests, testFuncs: funcSet}, nil
}

// TestFuncNames returns the set of every top-level Test* function name found
// under internal/**/*_test.go and cmd/**/*_test.go. This is the Test*-name
// half (mechanism #1) of the enforced_by resolver that selectFromRequirement
// / resolveOne use. It is the shared resolution primitive that
// check_enforced_by_resolvable reuses, so the two consumers (gate's targeted
// test selection and the invariant's resolvability audit) can never drift on
// what counts as a real Go test enforcer. The check_* half is NOT included
// here: each consumer answers it differently (gate via the literal map from
// internal/invariants, the invariant via its own All registry), so only the
// genuinely shared Test* name set is exposed.
//
// domainDir is accepted for backward API compatibility with callers that
// resolve a --domain path (ontology.Graph.DomainDir) but is otherwise
// unused: the resolver scans only HotamSpec's own internal/ and cmd/ trees,
// never a domain-local or generated tree (see testFuncRoots).
func TestFuncNames(domainDir string) (map[string]struct{}, error) {
	funcSet := make(map[string]struct{})
	for _, root := range testFuncRoots() {
		if err := walkTestFuncs(root, funcSet); err != nil {
			return nil, err
		}
	}
	return funcSet, nil
}

// walkTestFuncs walks root recursively and collects every top-level Test*
// function name from *_test.go files into funcSet. It deliberately does NOT
// collect check_ string literals — those are handled per-directory by
// scanTestFile (scoped to invariants) to avoid fixture pollution.
func walkTestFuncs(root string, funcSet map[string]struct{}) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		return collectTestFuncNames(path, funcSet)
	})
}

// collectTestFuncNames parses one test file and adds its top-level Test*
// function names to funcSet (the Test*-only half of scanTestFile, without the
// check_ literal collection that scanTestFile also performs). Goes through
// the same parseTestFileCached cache scanTestFile uses -- when walkTestFuncs'
// internal/ root walk and scanTestDir's invariants/-scoped walk both reach
// the SAME file (true today for every file under internal/invariants/, since
// defaultInvariantsDir() is itself a subdirectory of defaultInternalRoot(),
// so buildScan's two scans overlap there on every call), the second scan
// reuses the first's cached parse instead of paying go/parser.ParseFile
// again for a file whose content has not changed since.
func collectTestFuncNames(path string, funcSet map[string]struct{}) error {
	parsed, err := parseTestFileCached(path)
	if err != nil {
		return err
	}
	for _, name := range parsed.testFuncs {
		funcSet[name] = struct{}{}
	}
	return nil
}

func keysSorted(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func entryNoun(n int) string {
	if n == 1 {
		return "entry"
	}
	return "entries"
}
