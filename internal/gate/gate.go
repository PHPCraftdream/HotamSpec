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
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return err
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		if !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		scan.testFuncs[fn.Name.Name] = struct{}{}
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
				addUnique(scan.checkToTests, val, fn.Name.Name)
			}
			return true
		})
	}
	return nil
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

// genGoRoot returns the domain's gen-code Go output directory
// (<domainDir>/gen/go — the fixed convention documented in
// docs/GEN-CODE-CONTRACT.md §1) and whether it actually exists on disk.
// domainDir is the same --domain path every caller already resolves (see
// ontology.Graph.DomainDir, populated by the loader); no new manifest field
// or CLI flag is needed since the subpath is deterministic. A domain that
// never ran `hotam gen-code` simply has nothing to scan here — that is not
// an error, just an empty (false) result, so buildScan/TestFuncNames treat a
// missing gen/go the same as a domain with zero generated tests.
func genGoRoot(domainDir string) (string, bool) {
	if domainDir == "" {
		return "", false
	}
	root := filepath.Join(domainDir, "gen", "go")
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return root, true
}

// testFuncRoots returns every root directory mechanism #1 (Test*-name
// resolution) walks: internal/ and cmd/ (HotamSpec's own tree, always
// scanned) plus, when domainDir is non-empty and its gen/go output exists,
// <domainDir>/gen/go (a consumer domain's generated test tree — see
// genGoRoot). Both buildScan and TestFuncNames call this so the two
// consumers can never drift on which roots count.
//
// Widening mechanism #1 to a consumer domain's gen/go is a deliberate,
// steward-accepted trust shift (task #214, following the finding in task
// #197): the engine can no longer independently verify from its OWN tree
// alone that an ENFORCED requirement in a consumer domain has a real
// enforcer — it now also trusts that whatever *_test.go files live under
// gen/go were produced by the deterministic `hotam gen-code` generator, not
// hand-written by an agent. That trust is the same one already placed in
// gen-code itself (tests are generated from the graph, not authored by
// hand), so widening the SCAN here does not introduce a new trust boundary,
// only extends the existing one to cover resolution. Mechanism #2 (the
// check_* literal -> tests map) is deliberately NOT widened this way — see
// defaultInternalRoot's doc comment for why that risk model differs.
func testFuncRoots(domainDir string) []string {
	roots := []string{defaultInternalRoot(), defaultCmdRoot()}
	if genGo, ok := genGoRoot(domainDir); ok {
		roots = append(roots, genGo)
	}
	return roots
}

// buildScan constructs the resolver's combined view: the check_ literal map
// comes from internal/invariants only (scanTestDir), and the Test* function
// name set comes from ALL internal/**/*_test.go, cmd/**/*_test.go, and (when
// present) <domainDir>/gen/go/**/*_test.go (walkTestFuncs over
// testFuncRoots). This split is what makes mechanism #1 repo-wide (and now
// consumer-domain-wide) while keeping mechanism #2 free of fixture
// pollution — see defaultInternalRoot, defaultCmdRoot, and genGoRoot.
func buildScan(domainDir string) (*testScan, error) {
	invScan, err := scanTestDir(defaultInvariantsDir())
	if err != nil {
		return nil, err
	}
	funcSet := make(map[string]struct{})
	for _, root := range testFuncRoots(domainDir) {
		if err := walkTestFuncs(root, funcSet); err != nil {
			return nil, err
		}
	}
	return &testScan{checkToTests: invScan.checkToTests, testFuncs: funcSet}, nil
}

// TestFuncNames returns the set of every top-level Test* function name found
// under internal/**/*_test.go, cmd/**/*_test.go, and — when domainDir is
// non-empty and has a gen/go output directory — that domain's
// gen/go/**/*_test.go (see genGoRoot). This is the Test*-name half
// (mechanism #1) of the enforced_by resolver that selectFromRequirement /
// resolveOne use. It is the shared resolution primitive that
// check_enforced_by_resolvable reuses, so the two consumers (gate's targeted
// test selection and the invariant's resolvability audit) can never drift on
// what counts as a real Go test enforcer. The check_* half is NOT included
// here: each consumer answers it differently (gate via the literal map from
// internal/invariants, the invariant via its own All registry), so only the
// genuinely shared Test* name set is exposed.
//
// domainDir is the resolved --domain path (ontology.Graph.DomainDir); pass
// "" to scan only HotamSpec's own internal/ and cmd/ trees (e.g. when no
// domain context is available), which preserves the pre-task-#214 behavior
// byte-for-byte.
func TestFuncNames(domainDir string) (map[string]struct{}, error) {
	funcSet := make(map[string]struct{})
	for _, root := range testFuncRoots(domainDir) {
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
// check_ literal collection that scanTestFile also performs).
func collectTestFuncNames(path string, funcSet map[string]struct{}) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return err
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		if !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		funcSet[fn.Name.Name] = struct{}{}
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
