package selfcheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// TestLaunchDirWriteScope_NoHomeWriteColocation enforces
// R-committed-code-no-home-writes: committed framework Go source shall never
// reference the host home directory co-located with a filesystem-write sink.
// This is the structural/mechanically-checkable half of the original
// R-work-within-launch-dir principle, re-ported to Go from the Python-era AST
// scanner (tests/test_launch_dir_write_scope.py) that was not carried forward
// into the Go port; the residual live-agent-runtime-conduct half remains
// discipline-of-prose at R-work-within-launch-dir.
//
// EXACT RULE (mechanically checked): for EVERY NON-TEST, NON-TESTDATA .go file
// under internal/ and cmd/hotam/, no function (or method) body may contain BOTH
// a home-directory reference AND a filesystem-write sink. The conjunction is
// what makes a hit: a function that merely READS a home path, or merely WRITES
// somewhere, is not flagged — only a function that does both within one body is
// a home-write co-location. "Same function body" (the *ast.FuncDecl body) is the
// co-location unit, matching the spirit of the original Python scanner: it is
// the scope within which a computed home path can flow into a write sink.
//
// HOME-DIRECTORY REFERENCES (any one triggers the home half):
//   - os.UserHomeDir()           — the canonical Go home resolver;
//   - os.Getenv("HOME")          — Unix home via env (Python's expanduser('~') analog);
//   - os.Getenv("USERPROFILE")   — Windows home via env;
//   - a string literal starting with "~/" — shell-style home-rooted path.
//
// FILESYSTEM-WRITE SINKS (any one triggers the write half):
//   - os.WriteFile, os.Create, os.Mkdir, os.MkdirAll, os.Remove, os.RemoveAll,
//     os.Rename, os.Symlink, os.Chmod.
//
// (os.OpenFile is deliberately NOT in the sink list: it is read-or-write and the
// explicit write primitives above cover the unambiguous write case. os/user's
// Current().HomeDir is also out of scope today — this repo imports neither —
// and would only matter under the same conjunction.)
//
// This scan runs over NON-TEST source, so this scanner's own *_test.go file is
// automatically excluded from the scan (as is every *_test.go). The home half
// matches via the local identifier bound to the "os" import (aliasing-aware); a
// file that does not import "os" cannot produce os.* hits, though a "~/" literal
// is still caught.
//
// Discrimination: see TestLaunchDirWriteScope_DetectsColocation.
func TestLaunchDirWriteScope_NoHomeWriteColocation(t *testing.T) {
	t.Parallel()
	files := collectGoFiles(t, frameworkScanRoots, false /* non-test */, false /* no testdata */)
	for _, f := range files {
		for _, hit := range detectHomeWriteColocation(f.path, f.ast) {
			t.Errorf("R-committed-code-no-home-writes: function %s in %s co-locates home-directory reference %q with filesystem-write sink %q — committed framework code must never write into the host home directory (R-work-within-launch-dir)",
				hit.funcName, relPath(t, f.path), hit.homeRef, hit.writeRef)
		}
	}
}

// TestLaunchDirWriteScope_DetectsColocation is the non-vacuity control: a
// synthetic source carrying a home-ref+write co-location must be flagged, while a
// function with ONLY a home ref and a function with ONLY a write sink must NOT.
// This proves the detector genuinely requires the conjunction rather than
// trivially succeeding or flagging either half alone.
func TestLaunchDirWriteScope_DetectsColocation(t *testing.T) {
	t.Parallel()
	src := `package synth
import "os"
func badUserHomeDir() {
	home, _ := os.UserHomeDir()
	_ = os.WriteFile(home+"/.config/leak", []byte("x"), 0o644)
}
func badGetenvHome() {
	h := os.Getenv("HOME")
	_ = os.MkdirAll(h+"/.cache/x", 0o755)
}
func homeOnly() {
	home, _ := os.UserHomeDir()
	_ = home
}
func writeOnly() {
	_ = os.WriteFile("/tmp/inside-launch-dir", []byte("x"), 0o644)
}
func tildeLiteralOnly() {
	_ = "~/.config"
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synth_home_write.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	hits := detectHomeWriteColocation("synth_home_write.go", file)
	// Exactly two co-locations: badUserHomeDir (UserHomeDir + WriteFile) and
	// badGetenvHome (Getenv("HOME") + MkdirAll). This also proves the Getenv
	// home-ref branch is exercised, not just the UserHomeDir branch.
	if len(hits) != 2 {
		t.Fatalf("detector must flag exactly two co-locations, got %d: %+v — the main test would be vacuous", len(hits), hits)
	}
	flagged := map[string]bool{}
	for _, h := range hits {
		flagged[h.funcName] = true
	}
	if !flagged["badUserHomeDir"] || !flagged["badGetenvHome"] {
		t.Errorf("expected hits on badUserHomeDir and badGetenvHome, got %v", flagged)
	}
	// The conjunction is required: a function with ONLY a home ref (homeOnly,
	// tildeLiteralOnly) and a function with ONLY a write sink (writeOnly) must
	// NOT be flagged.
	for _, name := range []string{"homeOnly", "writeOnly", "tildeLiteralOnly"} {
		if flagged[name] {
			t.Errorf("detector wrongly flagged %q — a function must carry BOTH a home ref and a write sink to be a co-location", name)
		}
	}
}

// homeWriteHit is a detected co-location of a home-dir reference and a
// filesystem-write sink within a single function body.
type homeWriteHit struct {
	path     string
	funcName string
	homeRef  string
	writeRef string
}

// homeRefFuncs are os.* call targets that resolve a home-rooted directory.
var homeRefFuncs = map[string]bool{
	"UserHomeDir": true,
}

// homeEnvKeys are os.Getenv argument values that resolve a home directory.
var homeEnvKeys = map[string]bool{
	"HOME":        true,
	"USERPROFILE": true,
}

// writeSinkFuncs are os.* call targets that mutate the filesystem.
var writeSinkFuncs = map[string]bool{
	"WriteFile": true,
	"Create":    true,
	"Mkdir":     true,
	"MkdirAll":  true,
	"Remove":    true,
	"RemoveAll": true,
	"Rename":    true,
	"Symlink":   true,
	"Chmod":     true,
}

// detectHomeWriteColocation scans a parsed .go file for function bodies that
// contain BOTH a home-directory reference and a filesystem-write sink. It returns
// one hit per offending function. The co-location unit is the *ast.FuncDecl body.
func detectHomeWriteColocation(path string, file *ast.File) []homeWriteHit {
	osLocal := osQualifierName(file)
	var hits []homeWriteHit
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		var homeRefs, writeSinks []string
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if isOSQualifier(sel.X, osLocal) {
						name := sel.Sel.Name
						if homeRefFuncs[name] {
							homeRefs = append(homeRefs, "os."+name+"()")
						}
						if name == "Getenv" {
							if key, ok := stringLit(call.Args, 0); ok && homeEnvKeys[key] {
								homeRefs = append(homeRefs, "os.Getenv(\""+key+"\")")
							}
						}
						if writeSinkFuncs[name] {
							writeSinks = append(writeSinks, "os."+name)
						}
					}
				}
			}
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if v, err := strconv.Unquote(lit.Value); err == nil && strings.HasPrefix(v, "~/") {
					homeRefs = append(homeRefs, "literal \""+v+"\"")
				}
			}
			return true
		})
		if len(homeRefs) > 0 && len(writeSinks) > 0 {
			hits = append(hits, homeWriteHit{
				path:     path,
				funcName: funcDeclName(fn),
				homeRef:  homeRefs[0],
				writeRef: writeSinks[0],
			})
		}
	}
	return hits
}

// osQualifierName returns the local identifier bound to the "os" import in this
// file (aliasing-aware), or "" if "os" is not imported by a usable name. A
// dot-import or blank-import of "os" returns "" since neither is referenceable as
// a selector qualifier in normal use.
func osQualifierName(file *ast.File) string {
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != "os" {
			continue
		}
		if imp.Name != nil {
			name := imp.Name.Name
			if name != "." && name != "_" {
				return name
			}
			return ""
		}
		return "os"
	}
	return ""
}

// isOSQualifier reports whether x is the identifier that qualifies the "os"
// import in this file.
func isOSQualifier(x ast.Expr, osLocal string) bool {
	if osLocal == "" {
		return false
	}
	id, ok := x.(*ast.Ident)
	return ok && id.Name == osLocal
}

// stringLit returns the (unquoted) string-literal value at args[i] and true, if
// that argument is a string literal; otherwise ("", false). i must be in range.
func stringLit(args []ast.Expr, i int) (string, bool) {
	if i < 0 || i >= len(args) {
		return "", false
	}
	lit, ok := args[i].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

// funcDeclName returns a readable name for a function declaration: "Recv.Name"
// for methods (best-effort receiver type), or "Name" for plain functions.
func funcDeclName(fn *ast.FuncDecl) string {
	name := fn.Name.Name
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		switch t := fn.Recv.List[0].Type.(type) {
		case *ast.Ident:
			return t.Name + "." + name
		case *ast.StarExpr:
			if id, ok := t.X.(*ast.Ident); ok {
				return id.Name + "." + name
			}
		}
	}
	return name
}
