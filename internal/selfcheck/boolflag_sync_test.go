package selfcheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestBoolFlagNames_SyncedWithRegistrations closes the "comment, not test" gap
// flagged by the wave-5 review around cmd/hotam/main.go's boolFlagNames map.
//
// boolFlagNames (cmd/hotam/main.go) is the flat, package-wide registry of flag
// names that take NO value (registered via fs.Bool somewhere in cmd/hotam), so
// reorderFlagsFirst — a pre-processing step over raw os.Args that runs BEFORE
// subcommand dispatch, with no per-subcommand *flag.FlagSet in scope yet —
// never swallows the following token as a boolean flag's "value". isBoolFlag
// consults this map to decide whether a "--<name>" token consumes the next
// positional argument.
//
// Until this test landed, the invariant "boolFlagNames mirrors every real
// fs.Bool(...) registration in cmd/hotam/*.go" was enforced ONLY by a hand-
// written "KEEP THIS LIST IN SYNC" comment above the map. A future editor who
// adds fs.Bool("verbose", false, ...) to a subcommand's FlagSet without adding
// "verbose" to boolFlagNames reintroduces the EXACT bug fixed in task #107: the
// new boolean flag would wrongly eat the next positional token as its "value"
// (because reorderFlagsFirst only treats a flag as value-less when isBoolFlag
// says so). This test replaces that comment with a structural check.
//
// EXACT RULE (mechanically checked, BOTH directions):
//
//	FORWARD  — every flag name found via a real .Bool("<name>", ...) call in any
//	           NON-TEST .go file under cmd/hotam/ is present as a key in
//	           boolFlagNames. A registration with no matching key is the live
//	           bug (task #107 regression).
//	REVERSE  — every key in boolFlagNames corresponds to at least one real
//	           .Bool("<name>", ...) registration. A stale/orphaned entry left in
//	           the map after its registration is removed is also real drift
//	           (harmless to argv parsing today, but it lies about the package's
//	           actual boolean flags and would mask a future re-registration).
//
// This test scans NON-TEST source only (so this scanner's own *_test.go file is
// excluded, as is every *_test.go). Because cmd/hotam is package main, the map
// literal's keys are extracted by parsing main.go's AST (not by importing the
// runtime value, which Go forbids for package main) — the same self-contained,
// no-cross-package-coupling structural style as switchCaseStrings/stringLiterals
// elsewhere in this package.
//
// Discrimination / non-vacuity: see TestBoolFlagNames_DetectorNonVacuous.
func TestBoolFlagNames_SyncedWithRegistrations(t *testing.T) {
	t.Parallel()

	// FORWARD scan: every real .Bool("<name>", ...) registration across the
	// cmd/hotam package (non-test source).
	files := collectGoFiles(t, []string{"cmd/hotam"}, false /* non-test */, false /* no testdata */)
	registered := map[string][]string{} // flag name -> files that register it
	for _, f := range files {
		for _, name := range detectBoolFlagRegistrations(f.ast) {
			registered[name] = append(registered[name], relPath(t, f.path))
		}
	}

	// Extract the declared keys of the boolFlagNames map literal from main.go.
	mainPath := filepath.Join(repoRoot(t), "cmd", "hotam", "main.go")
	fset := token.NewFileSet()
	mainFile, err := parser.ParseFile(fset, mainPath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", mainPath, err)
	}
	keys := mapLiteralStringKeys(mainFile, "boolFlagNames")
	if len(keys) == 0 {
		t.Fatalf("mapLiteralStringKeys found no keys for boolFlagNames in %s — the map literal shape changed and this test is now blind", relPath(t, mainPath))
	}
	declared := map[string]bool{}
	for _, k := range keys {
		declared[k] = true
	}

	// FORWARD: every real registration must have a matching declared key.
	for name, where := range registered {
		if !declared[name] {
			t.Errorf("boolFlagNames drift (FORWARD): flag %q is registered via .Bool(%q, ...) in %s but is missing from boolFlagNames in cmd/hotam/main.go — reorderFlagsFirst would wrongly swallow the next positional token as this flag's value (the bug fixed in task #107). Add %q to boolFlagNames.",
				name, name, strings.Join(where, ", "), name)
		}
	}

	// REVERSE: every declared key must correspond to at least one real
	// registration (no stale/orphaned entries).
	for name := range declared {
		if _, ok := registered[name]; !ok {
			t.Errorf("boolFlagNames drift (REVERSE): %q is declared in boolFlagNames (cmd/hotam/main.go) but no .Bool(%q, ...) registration exists in any non-test cmd/hotam/*.go file — a stale/orphaned entry.", name, name)
		}
	}
}

// TestBoolFlagNames_DetectorNonVacuous is the non-vacuity control: synthetic
// source proves detectBoolFlagRegistrations genuinely finds .Bool("<name>", ...)
// calls (and NOT .String(...) calls or non-string-literal args), and that
// mapLiteralStringKeys genuinely extracts the named map's string keys (and
// ignores a same-shaped map bound to a different variable name). Without this,
// a silently-broken detector/extractor would make the main test vacuously green.
func TestBoolFlagNames_DetectorNonVacuous(t *testing.T) {
	t.Parallel()

	// (a) detectBoolFlagRegistrations: must match .Bool("<str>", ...) and reject
	// both a wrong method name and a non-string-literal first argument.
	regSrc := `package synth
import "flag"
func regJSON(fs *flag.FlagSet)   { fs.Bool("json", false, "x") }
func regVerbose(fs *flag.FlagSet) { fs.Bool("verbose", false, "y") }
func notABool(fs *flag.FlagSet)   { fs.String("name", "", "z") }
func nonStrArg(x *flag.FlagSet)   { other := x; other.Bool(42) }
`
	regFile, err := parser.ParseFile(token.NewFileSet(), "synth_reg.go", regSrc, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse synthetic registrations: %v", err)
	}
	got := map[string]int{}
	for _, name := range detectBoolFlagRegistrations(regFile) {
		got[name]++
	}
	if got["json"] != 1 || got["verbose"] != 1 || len(got) != 2 {
		t.Fatalf("detector must find exactly json+verbose (and reject .String(...) / non-string-literal args), got %v — the forward test would be vacuous", got)
	}

	// (b) mapLiteralStringKeys: must extract only the named map's string keys,
	// ignoring a same-shaped map bound to a different variable.
	mapSrc := `package synth
var boolFlagNames = map[string]bool{
	"json":    true,
	"verbose": true,
}
var otherMap = map[string]bool{"ignored": true}
`
	mapFile, err := parser.ParseFile(token.NewFileSet(), "synth_map.go", mapSrc, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse synthetic map: %v", err)
	}
	keys := mapLiteralStringKeys(mapFile, "boolFlagNames")
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}
	if !keySet["json"] || !keySet["verbose"] || len(keySet) != 2 {
		t.Fatalf("mapLiteralStringKeys must extract exactly json+verbose from boolFlagNames (and ignore otherMap), got %v — the reverse test would be vacuous", keySet)
	}
}

// detectBoolFlagRegistrations scans a parsed .go file for <x>.Bool("<name>", ...)
// -shaped call expressions — where the method selector is named "Bool" and the
// first argument is a string literal — and returns every such flag name (a name
// registered N times appears N times). This is the lightweight, no-type-
// resolution heuristic style used by this package's other structural scanners:
// it matches fs.Bool("json", ...) without resolving the receiver type (which is
// a *flag.FlagSet bound to a locally-named variable, conventionally "fs"). A
// .Bool(...) call whose first argument is NOT a string literal is not matched.
func detectBoolFlagRegistrations(file *ast.File) []string {
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Bool" {
			return true
		}
		if name, ok := stringLit(call.Args, 0); ok {
			names = append(names, name)
		}
		return true
	})
	return names
}

// mapLiteralStringKeys finds a top-level `var <varName> = map[...]{...}`
// declaration in the file and returns the unquoted string-literal keys of its
// composite literal (one per *ast.KeyValueExpr whose Key is a string literal).
// It returns nil if no such declaration exists, its value is not a composite
// literal, or its elements have no string-literal keys. This matches main.go's
// `var boolFlagNames = map[string]bool{ "json": true, }` shape. A map literal
// bound to a different variable name is ignored.
func mapLiteralStringKeys(file *ast.File, varName string) []string {
	var keys []string
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			matched := false
			for _, name := range vs.Names {
				if name.Name == varName {
					matched = true
					break
				}
			}
			if !matched || len(vs.Values) == 0 {
				continue
			}
			cl, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, elt := range cl.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				lit, ok := kv.Key.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if v, err := strconv.Unquote(lit.Value); err == nil {
					keys = append(keys, v)
				}
			}
		}
	}
	return keys
}
