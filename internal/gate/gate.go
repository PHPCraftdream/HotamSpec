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

	testsDir := defaultInvariantsDir()
	scan, err := scanTestDir(testsDir)
	if err != nil {
		return GateResult{
			Confident: false,
			Reason: fmt.Sprintf(
				"target %q: could not scan test directory %q: %v — fail-closed to full suite.",
				targetAnchor, testsDir, err),
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
