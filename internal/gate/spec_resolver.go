package gate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// SpecSymbolKind classifies what an implemented_by entry resolved to.
type SpecSymbolKind int

const (
	// SpecSymbolNone means the symbol was not found in the named file.
	SpecSymbolNone SpecSymbolKind = iota
	// SpecSymbolFunc is a top-level function declaration (fn.Recv == nil).
	SpecSymbolFunc
	// SpecSymbolMethod is a method declaration (fn.Recv != nil).
	SpecSymbolMethod
	// SpecSymbolType is a type declaration (struct/interface/alias/etc).
	SpecSymbolType
)

// SpecSymbolResult is the outcome of resolving one implemented_by entry
// (file:symbol) against a single named file in a domain's authored spec/
// tree.
type SpecSymbolResult struct {
	Kind SpecSymbolKind
}

// Found reports whether the symbol was located.
func (r SpecSymbolResult) Found() bool { return r.Kind != SpecSymbolNone }

// SpecTestResult is the outcome of resolving one verified_by entry
// (file:test) against a single named file in a domain's authored spec/
// tree.
type SpecTestResult struct {
	Found bool
	// HasTeeth is true when the test body contains at least one real
	// assertion or exercising construct (t.Error/t.Fatal/t.Errorf/t.Fatalf/
	// require.*/assert.*/an if-statement), as opposed to a body that is
	// empty or contains only t.Log/t.Logf calls.
	HasTeeth bool
	// HasSkip is true when the test body contains a top-level call to
	// t.Skip/t.Skipf (unconditionally reachable at the top level of the
	// function body -- not nested inside an if, which would make it
	// conditional and therefore not a blanket "this test proves nothing"
	// escape hatch).
	HasSkip bool
}

// ResolveSpecSymbol parses the domain-relative file (joined onto specRoot,
// which callers pass as filepath.Join(domainDir, "spec") or equivalent) and
// looks for a declaration matching symbol.
//
// Convention for `implemented_by` symbol names (file:symbol):
//   - A bare identifier ("NewRisk", "Validate") matches EITHER a top-level
//     function OR a method with that name, on any receiver type. This is the
//     common case: authored code rarely has a same-named function and method
//     colliding in one file, and requiring the receiver type to be spelled
//     out for every method would make implemented_by entries needlessly
//     verbose for the normal case.
//   - A dotted identifier ("Risk.Validate") matches a method named "Validate"
//     declared on a receiver whose base type name is "Risk" (pointer or
//     value receiver both count -- "(r *Risk) Validate" and "(r Risk)
//     Validate" both match "Risk.Validate"). Use this form when a file
//     declares more than one method with the same name on different
//     receiver types (rare, but the qualified form disambiguates it).
//   - A bare identifier can also match a type declaration (struct/interface/
//     alias) with that name -- this covers implemented_by entries that point
//     at the model type itself ("spec/model/risk.go:Risk") rather than a
//     constructor or method.
func ResolveSpecSymbol(specRoot, file, symbol string) (SpecSymbolResult, error) {
	path := filepath.Join(specRoot, filepath.FromSlash(file))
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return SpecSymbolResult{}, fmt.Errorf("parse %s: %w", path, err)
	}

	wantType, wantName, qualified := splitQualifiedSymbol(symbol)

	for _, decl := range astFile.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				// Top-level function. Only matches an unqualified name.
				if !qualified && d.Name.Name == wantName {
					return SpecSymbolResult{Kind: SpecSymbolFunc}, nil
				}
				continue
			}
			// Method. Matches a bare name (any receiver) or a qualified
			// "Type.Method" name (receiver base type must match).
			if d.Name.Name != wantName {
				continue
			}
			if !qualified {
				return SpecSymbolResult{Kind: SpecSymbolMethod}, nil
			}
			if receiverBaseTypeName(d.Recv) == wantType {
				return SpecSymbolResult{Kind: SpecSymbolMethod}, nil
			}
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			if qualified {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if ts.Name.Name == wantName {
					return SpecSymbolResult{Kind: SpecSymbolType}, nil
				}
			}
		}
	}
	return SpecSymbolResult{Kind: SpecSymbolNone}, nil
}

// splitQualifiedSymbol splits a symbol name of the form "Type.Method" into
// its two parts. If symbol contains no ".", qualified is false and name is
// the whole symbol.
func splitQualifiedSymbol(symbol string) (typeName, name string, qualified bool) {
	if idx := strings.LastIndex(symbol, "."); idx >= 0 {
		return symbol[:idx], symbol[idx+1:], true
	}
	return "", symbol, false
}

// receiverBaseTypeName extracts the bare type name from a method receiver
// field list, unwrapping a leading pointer ("*Risk" -> "Risk").
func receiverBaseTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// ResolveSpecTest parses the domain-relative file (joined onto specRoot) and
// looks for a top-level test function `func <testName>(t *testing.T)`.
// Returns Found=false if no such function exists in the file (staleness /
// existence check). When found, also reports HasTeeth (anti-vacuousness) and
// HasSkip (no escape-hatch skip) so callers can build the ENFORCED-gate
// prohibition checks without re-parsing.
func ResolveSpecTest(specRoot, file, testName string) (SpecTestResult, error) {
	path := filepath.Join(specRoot, filepath.FromSlash(file))
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return SpecTestResult{}, fmt.Errorf("parse %s: %w", path, err)
	}

	for _, decl := range astFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		if fn.Name.Name != testName {
			continue
		}
		if !strings.HasPrefix(fn.Name.Name, "Test") {
			// Named exactly but not Test*-shaped -- not a real Go test
			// function (go test would never run it).
			continue
		}
		if !isRealTestSignature(fn) {
			continue
		}
		teeth := testBodyHasTeeth(fn.Body)
		skip := testBodyHasTopLevelSkip(fn.Body)
		return SpecTestResult{Found: true, HasTeeth: teeth, HasSkip: skip}, nil
	}
	return SpecTestResult{Found: false}, nil
}

// isRealTestSignature reports whether fn has the shape go test requires:
// func TestXxx(t *testing.T). A function merely named Test* with a
// different signature is not a real test enforcer.
func isRealTestSignature(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}
	param := fn.Type.Params.List[0]
	star, ok := param.Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkgIdent.Name == "testing" && sel.Sel.Name == "T"
}

// testBodyHasTeeth reports whether fn's body contains at least one
// meaningful assertion or exercising construct: a call to t.Error/t.Errorf/
// t.Fatal/t.Fatalf/t.FailNow/t.Fail, a call whose selector name matches a
// common assertion-library pattern (require.*/assert.*), or a control-flow
// construct (if/for/switch) -- anything beyond a flat sequence of t.Log
// calls (or an empty body). This is the anti-vacuousness detector: the
// mechanical successor to the old honest no-op checkEnforcedByTestHasTeeth,
// now applied to verified_by as a structural PROHIBITION for ENFORCED
// (not an advisory no-op). It does not and cannot judge whether the
// assertions are ABOUT the right thing -- only that the test body is not
// hollow.
func testBodyHasTeeth(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch stmt := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.RangeStmt:
			found = true
			return false
		case *ast.CallExpr:
			if isTeethCall(stmt) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// isTeethCall reports whether call is an assertion-shaped call: t.Error*,
// t.Fatal*, t.Fail*, or a call on an identifier/package conventionally used
// by Go assertion libraries (require.*, assert.*) -- e.g. require.NoError,
// assert.Equal. t.Log/t.Logf and other non-assertion calls (helper setup,
// fmt.Sprintf, etc.) do not count.
func isTeethCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	method := sel.Sel.Name
	switch method {
	case "Error", "Errorf", "Fatal", "Fatalf", "FailNow", "Fail":
		return true
	}
	if ident, ok := sel.X.(*ast.Ident); ok {
		switch ident.Name {
		case "require", "assert":
			return true
		}
	}
	return false
}

// testBodyHasTopLevelSkip reports whether fn's body contains a call to
// t.Skip/t.Skipf reachable unconditionally at the TOP LEVEL of the function
// body (a direct statement of the body's *ast.BlockStmt, not nested inside
// an if/for/switch/etc). A skip guarded by a runtime condition (e.g.
// `if testing.Short() { t.Skip(...) }`) is a normal Go idiom and is NOT
// flagged -- only an unconditional top-level skip, which makes the "test"
// unconditionally prove nothing, is treated as the escape hatch this check
// prohibits.
func testBodyHasTopLevelSkip(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	for _, stmt := range body.List {
		exprStmt, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := exprStmt.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if sel.Sel.Name == "Skip" || sel.Sel.Name == "Skipf" {
			return true
		}
	}
	return false
}

// ParseFileColonSymbol splits a "file:symbol" or "file:test" entry (as used
// by implemented_by / verified_by) into its file and symbol/test-name parts.
// Returns ok=false if entry does not contain exactly the expected shape
// (non-empty file, non-empty symbol, separated by the LAST colon -- Windows
// drive-letter colons are not expected here since entries are always
// domain-relative forward-slash paths per PLAN-authored-spec-discipline.md
// §4, but using the last colon keeps this robust regardless).
func ParseFileColonSymbol(entry string) (file, symbol string, ok bool) {
	idx := strings.LastIndex(entry, ":")
	if idx <= 0 || idx == len(entry)-1 {
		return "", "", false
	}
	return entry[:idx], entry[idx+1:], true
}

// SpecRoot returns the authored spec/ root for a domain directory
// (domainDir/spec). Entries in implemented_by/verified_by are documented
// (PLAN-authored-spec-discipline.md §4) as domain-relative paths already
// prefixed with "spec/" (e.g. "spec/model/risk.go:NewRisk"), so this helper
// returns domainDir itself -- callers join the full entry path (which
// already starts with "spec/") onto domainDir, not onto domainDir/spec. It
// exists as a named seam so the convention is documented in one place and
// callers do not have to re-derive it.
func SpecRoot(domainDir string) string {
	return domainDir
}
