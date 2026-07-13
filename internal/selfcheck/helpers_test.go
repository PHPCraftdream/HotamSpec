package selfcheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// repoRoot returns the repository root (the directory holding go.mod) by
// walking up from the test's working directory. Tests in this package run with
// cwd = internal/selfcheck, so go.mod is two levels up; the walk keeps the
// helper robust against package relocation.
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
	t.Fatalf("repoRoot: could not find go.mod above %s", dir)
	return ""
}

// goFileInfo is a parsed .go file with its absolute path.
type goFileInfo struct {
	path string
	ast  *ast.File
}

// collectGoFiles walks the given root directories (resolved against repoRoot)
// and returns parsed .go files. includeTests controls *_test.go inclusion;
// includeTestdata controls descent into testdata/ subtrees. Any directory
// named "testdata" is always skipped when includeTestdata is false.
func collectGoFiles(t *testing.T, roots []string, includeTests, includeTestdata bool) []goFileInfo {
	t.Helper()
	root := repoRoot(t)
	fset := token.NewFileSet()
	var out []goFileInfo
	for _, r := range roots {
		absRoot := filepath.Join(root, filepath.Clean(r))
		filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				t.Fatalf("walk %s: %v", path, err)
			}
			base := d.Name()
			if d.IsDir() {
				if !includeTestdata && base == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(base, ".go") {
				return nil
			}
			if !includeTests && strings.HasSuffix(base, "_test.go") {
				return nil
			}
			parsed, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if perr != nil {
				t.Fatalf("parse %s: %v", path, perr)
			}
			out = append(out, goFileInfo{path: path, ast: parsed})
			return nil
		})
	}
	if len(out) == 0 {
		t.Fatalf("collectGoFiles: no .go files collected under roots %v", roots)
	}
	return out
}

// relPath returns a path relative to repoRoot for readable diagnostics.
func relPath(t *testing.T, path string) string {
	t.Helper()
	rel, err := filepath.Rel(repoRoot(t), path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// stringLiterals returns every string-literal value appearing in the file
// (unquoted). Comments are deliberately excluded — a comment naming a token is
// not compiled-in business data.
func stringLiterals(file *ast.File) []string {
	var vals []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if uq, err := strconv.Unquote(lit.Value); err == nil {
			vals = append(vals, uq)
		}
		return true
	})
	return vals
}

// importPaths returns every import path in the file (unquoted), including
// parenthesized and single imports.
func importPaths(file *ast.File) []string {
	var paths []string
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		if uq, err := strconv.Unquote(imp.Path.Value); err == nil {
			paths = append(paths, uq)
		}
	}
	return paths
}

// typeDeclNames returns the names of every type declaration in the file
// (type Foo struct{...}, type Foo = X aliases, type Foo interface{...}, etc.).
func typeDeclNames(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok {
				names = append(names, ts.Name.Name)
			}
		}
	}
	return names
}

// switchCaseStrings returns every string literal used as a case value in any
// switch/case clause in the file (e.g. the `case "gen-spec":` arms of main.go's
// command dispatch).
func switchCaseStrings(file *ast.File) []string {
	var cases []string
	ast.Inspect(file, func(n ast.Node) bool {
		cc, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expr := range cc.List {
			lit, ok := expr.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			if uq, err := strconv.Unquote(lit.Value); err == nil {
				cases = append(cases, uq)
			}
		}
		return true
	})
	return cases
}

// graphNodeTypes is the set of ontology node types whose composite literals in
// framework source would constitute embedded graph content.
var graphNodeTypes = map[string]bool{
	"Requirement":    true,
	"Conflict":       true,
	"Assumption":     true,
	"Axis":           true,
	"Stakeholder":    true,
	"Goal":           true,
	"Process":        true,
	"Operator":       true,
	"EntityType":     true,
	"EntityInstance": true,
}

// nodeLitHit is a detected composite literal that constructs a graph node (or a
// slice of them), with the file and type name for diagnostics.
type nodeLitHit struct {
	path     string
	typeName string
}

// findNodeLiterals scans the file for composite literals that construct
// POPULATED graph-node content. It matches these shapes (qualified and
// unqualified), requiring at least one element so that zero-value literals like
// `Requirement{}` (a legitimate empty return value) are NOT matched:
//   - ontology.Requirement{ID: ...} (Type is a *ast.SelectorExpr naming a node type)
//   - Requirement{ID: ...}          (Type is an *ast.Ident naming a node type — the
//     unqualified form used inside internal/ontology itself)
//   - []ontology.Requirement{ {...} } / []Requirement{ {...} }
//     (Type is an *ast.ArrayType whose element is one of the above)
//
// It deliberately does NOT match Requirement{} (empty zero-value), nor
// map[string]ontology.EntityType{} (an empty typed map — *ast.MapType).
func findNodeLiterals(path string, file *ast.File) []nodeLitHit {
	var hits []nodeLitHit
	ast.Inspect(file, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok || len(cl.Elts) == 0 {
			return true
		}
		if name := nodeNameFromExpr(cl.Type); name != "" {
			hits = append(hits, nodeLitHit{path: path, typeName: name})
			return true
		}
		if arr, ok := cl.Type.(*ast.ArrayType); ok {
			if name := nodeNameFromExpr(arr.Elt); name != "" {
				hits = append(hits, nodeLitHit{path: path, typeName: name})
			}
		}
		return true
	})
	return hits
}

// nodeNameFromExpr returns the node-type name if expr is a qualified
// (*ast.SelectorExpr) or unqualified (*ast.Ident) reference to a graph node
// type, else "".
func nodeNameFromExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		if graphNodeTypes[e.Sel.Name] {
			return e.Sel.Name
		}
	case *ast.Ident:
		if graphNodeTypes[e.Name] {
			return e.Name
		}
	}
	return ""
}
