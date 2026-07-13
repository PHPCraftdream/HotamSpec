package ontology

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEntityTypesAreDeclarative enforces R-entity-is-declarative: the framework
// supplies no built-in EntityType values — all entity types are pure
// business-domain declarations that live only in domains/*/graph.json, never
// hardcoded in Go.
//
// EXACT RULE (mechanically checked), two halves:
//
//  1. EMPTY-GRAPH HALF: a freshly constructed Graph (zero value) has an empty
//     EntityTypes slice — the framework injects no default entity type.
//  2. SOURCE HALF: across every NON-TEST .go file under internal/ and
//     cmd/hotam/, no package-level `var` declaration binds an EntityType value
//     (or slice of EntityType) with concrete content. A package-level var whose
//     declared type is EntityType/[]EntityType (qualified or unqualified) with
//     an initializer, OR whose initializer is a populated composite literal
//     constructing EntityType/[]EntityType, is a framework-supplied built-in
//     entity type and fails this check.
//
// Discrimination: see TestEntityTypesAreDeclarative_DetectsViolation.
func TestEntityTypesAreDeclarative(t *testing.T) {
	t.Parallel()

	// Half 1: zero-value graph has no entity types.
	var g Graph
	if len(g.EntityTypes) != 0 {
		t.Fatalf("R-entity-is-declarative: a zero-value Graph must default to an empty EntityTypes slice, got %d",
			len(g.EntityTypes))
	}

	// Half 2: no package-level EntityType var bindings in framework source.
	for _, f := range collectFrameworkGoFiles(t) {
		for _, name := range entityTypeValueBindings(f.ast) {
			t.Errorf("R-entity-is-declarative: package-level EntityType var %q in %s — the framework must supply no built-in EntityType values (entity types are domain-only declarations)",
				name, relRepoPath(t, f.path))
		}
	}
}

// TestEntityTypesAreDeclarative_DetectsViolation is the non-vacuity control: the
// detector must flag every shape of a package-level built-in EntityType binding
// (typed-with-initializer, unqualified single, qualified slice) and must NOT
// flag a map-typed var. If this passes, the main test's predicate genuinely
// catches built-in entity types rather than trivially succeeding.
func TestEntityTypesAreDeclarative_DetectsViolation(t *testing.T) {
	t.Parallel()
	src := `package synth
import "github.com/PHPCraftdream/HotamSpec/internal/ontology"
var BuiltinTypes = []ontology.EntityType{{Slug: "feature"}}
var Single = ontology.EntityType{Slug: "feature"}
var Typed EntityType = EntityType{Slug: "feature"}
var UntypedEmpty ontology.EntityType
var FieldMap = map[string]struct{}{}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synth.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	got := map[string]bool{}
	for _, name := range entityTypeValueBindings(file) {
		got[name] = true
	}
	for _, want := range []string{"BuiltinTypes", "Single", "Typed"} {
		if !got[want] {
			t.Errorf("R-entity-is-declarative non-vacuity: detector failed to flag package-level EntityType var %q, got %v",
				want, got)
		}
	}
	// UntypedEmpty is a typed declaration with NO initializer (zero value, no
	// concrete content) — it must NOT be flagged as a built-in value.
	if got["UntypedEmpty"] {
		t.Errorf("R-entity-is-declarative non-vacuity: detector must NOT flag a typed-but-uninitialized EntityType var, got %v", got)
	}
	if got["FieldMap"] {
		t.Errorf("R-entity-is-declarative non-vacuity: detector must NOT flag a map-typed var, got %v", got)
	}
}

// entityTypeValueBindings returns the names of package-level `var` bindings in
// the file that supply a built-in EntityType value (or slice of them) with
// concrete content. EntityType is a struct, so it cannot appear as a `const`;
// only `var` is inspected.
func entityTypeValueBindings(file *ast.File) []string {
	var offenders []string
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
			if valueSpecBindsEntityType(vs) {
				for _, n := range vs.Names {
					offenders = append(offenders, n.Name)
				}
			}
		}
	}
	return offenders
}

// valueSpecBindsEntityType reports whether a package-level ValueSpec binds an
// EntityType value with concrete content: either a typed declaration
// (EntityType / []EntityType, qualified or unqualified) that HAS an
// initializer, or an untyped declaration whose initializer is a populated
// composite literal constructing EntityType / []EntityType.
func valueSpecBindsEntityType(vs *ast.ValueSpec) bool {
	if exprReferencesEntityType(vs.Type, true) && len(vs.Values) > 0 {
		return true
	}
	for _, val := range vs.Values {
		if compositeLiteralOfEntityType(val) {
			return true
		}
	}
	return false
}

// compositeLiteralOfEntityType reports whether expr is a populated composite
// literal constructing an EntityType or []EntityType (qualified or unqualified).
func compositeLiteralOfEntityType(expr ast.Expr) bool {
	cl, ok := expr.(*ast.CompositeLit)
	if !ok || len(cl.Elts) == 0 {
		return false
	}
	return exprReferencesEntityType(cl.Type, true)
}

// exprReferencesEntityType reports whether expr is a reference to EntityType
// (qualified *ast.SelectorExpr or unqualified *ast.Ident), or — when allowSlice
// is true — a []EntityType array type whose element references EntityType.
func exprReferencesEntityType(expr ast.Expr, allowSlice bool) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name == "EntityType"
	case *ast.SelectorExpr:
		return e.Sel.Name == "EntityType"
	case *ast.ArrayType:
		return allowSlice && exprReferencesEntityType(e.Elt, false)
	}
	return false
}

// fwFile is a parsed framework .go file with its absolute path.
type fwFile struct {
	path string
	ast  *ast.File
}

// collectFrameworkGoFiles walks internal/ and cmd/hotam/ (resolved against the
// repo root) and returns the parsed NON-TEST .go files.
func collectFrameworkGoFiles(t *testing.T) []fwFile {
	t.Helper()
	root := repoRoot(t)
	fset := token.NewFileSet()
	var out []fwFile
	for _, r := range []string{"internal", "cmd/hotam"} {
		absRoot := filepath.Join(root, filepath.Clean(r))
		filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				t.Fatalf("walk %s: %v", path, err)
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			parsed, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if perr != nil {
				t.Fatalf("parse %s: %v", path, perr)
			}
			out = append(out, fwFile{path: path, ast: parsed})
			return nil
		})
	}
	if len(out) == 0 {
		t.Fatalf("collectFrameworkGoFiles: no non-test .go files found under internal/ or cmd/hotam/")
	}
	return out
}

// repoRoot returns the repository root (the directory holding go.mod) by
// walking up from the test's working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("repoRoot: could not find go.mod above the working directory")
	return ""
}

// relRepoPath returns a path relative to the repo root for readable diagnostics.
func relRepoPath(t *testing.T, path string) string {
	t.Helper()
	rel, err := filepath.Rel(repoRoot(t), path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
