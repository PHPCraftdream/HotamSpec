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

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

type GateResult struct {
	Confident bool
	NodeIDs   []string
	Reason    string
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
			return selectFromRequirement(targetAnchor, r.EnforcedBy)
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

func selectFromRequirement(targetAnchor string, enforcedBy []string) GateResult {
	if len(enforcedBy) == 0 {
		return GateResult{
			Confident: false,
			Reason: fmt.Sprintf(
				"target %q has an empty enforced_by list — no targeted enforcer is known; fail-closed to full suite.",
				targetAnchor),
		}
	}

	scan, err := buildScan()
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
// internal/gate). It is the root for the Test*-name half of resolution:
// mechanism #1 (an enforced_by entry that is a literal Test* function name)
// matches a Go test function ANYWHERE under internal/**/*_test.go, because a
// real test enforcer in internal/proposal or internal/generator is just as
// load-bearing as one in internal/invariants. Mechanism #2 (the check_*
// literal -> tests map) stays scoped to internal/invariants via
// defaultInvariantsDir, because check_* string literals appear as real
// enforcer references only there; elsewhere (gate_test.go, ontology/query
// fixtures) they appear as TEST FIXTURE DATA, and widening the check_ scan to
// those would make fake names like "check_full" /
// "check_nonexistent_fake_check" falsely resolve. See resolveOne.
func defaultInternalRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Join(filepath.Dir(file), "..")
}

// buildScan constructs the resolver's combined view: the check_ literal map
// comes from internal/invariants only (scanTestDir), and the Test* function
// name set comes from ALL internal/**/*_test.go (walkTestFuncs). This split
// is what makes mechanism #1 repo-wide while keeping mechanism #2 free of
// fixture pollution — see defaultInternalRoot.
func buildScan() (*testScan, error) {
	invScan, err := scanTestDir(defaultInvariantsDir())
	if err != nil {
		return nil, err
	}
	funcSet := make(map[string]struct{})
	if err := walkTestFuncs(defaultInternalRoot(), funcSet); err != nil {
		return nil, err
	}
	return &testScan{checkToTests: invScan.checkToTests, testFuncs: funcSet}, nil
}

// TestFuncNames returns the set of every top-level Test* function name found
// under internal/**/*_test.go — the Test*-name half (mechanism #1) of the
// enforced_by resolver that selectFromRequirement / resolveOne use. It is the
// shared resolution primitive that check_enforced_by_resolvable reuses, so the
// two consumers (gate's targeted test selection and the invariant's
// resolvability audit) can never drift on what counts as a real Go test
// enforcer. The check_* half is NOT included here: each consumer answers it
// differently (gate via the literal map from internal/invariants, the
// invariant via its own All registry), so only the genuinely shared Test*
// name set is exposed.
func TestFuncNames() (map[string]struct{}, error) {
	funcSet := make(map[string]struct{})
	if err := walkTestFuncs(defaultInternalRoot(), funcSet); err != nil {
		return nil, err
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
