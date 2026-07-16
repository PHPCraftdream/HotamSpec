package gate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
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
	// assertion call, anywhere in the body (t.Error/t.Fatal/t.Errorf/
	// t.Fatalf/t.Fail/t.FailNow/require.*/assert.*, whether at the top level
	// or nested inside an if/for/switch/range/t.Run branch), as opposed to a
	// body that is empty, contains only t.Log/t.Logf calls, or contains only
	// control-flow with no assertion call inside it.
	HasTeeth bool
	// HasSkip is true when the test body contains an UNCONDITIONAL call to
	// t.Skip/t.Skipf -- either a direct top-level statement, or nested
	// inside an `if` whose condition is a constant-true expression (so the
	// branch always runs). Not set for a skip nested inside a REAL runtime
	// condition (e.g. `if testing.Short() { t.Skip(...) }`), which is a
	// normal Go idiom and not a blanket "this test proves nothing" escape
	// hatch.
	HasSkip bool
}

// EntryWithinSpecScope enforces the STRUCTURAL FLOOR half of the
// implemented_by/verified_by discipline that resolution alone cannot: the
// referenced file path must stay INSIDE the domain's own authored scope, not
// merely "somewhere parseable on disk". Without this gate, a non-self-hosting
// domain could cite an arbitrary file anywhere in the repository (including
// another domain's spec/, or the engine's own internal/) via a "../" escape,
// and ResolveSpecSymbol/ResolveSpecTest would happily parse and resolve it --
// the mechanical check would report 0 violations for a reference that lies
// about "this is embodied in MY spec/".
//
// The rule (PLAN-authored-spec-discipline.md §6, hardened per adversarial
// review Probe D):
//
//   - non-self-hosting domain: file, once cleaned, MUST be a relative path
//     (no leading "/" or drive letter) that starts with "spec/" (forward-
//     slash form, as every implemented_by/verified_by entry is documented
//     and authored) and MUST NOT climb above specRoot (domainDir) via "..".
//     Concretely: filepath.Clean(file) must not equal ".." and must not
//     start with "../" (or the OS equivalent), AND filepath.Join(specRoot,
//     file) must remain lexically within specRoot.
//   - self-hosting domain: file MUST resolve (after filepath.Join(specRoot,
//     file) + Clean) to a path still inside specRoot (the engine repository
//     root found by SpecRoot/engineRoot) -- "..".-escapes out of the engine
//     repo are rejected the same way, but there is no required "spec/"
//     prefix since self-hosting entries name real engine paths like
//     "internal/ontology/lifecycle.go" (PLAN-authored-spec-discipline.md
//     §9).
//
// This is deliberately a pure path-shape check (no filesystem I/O) so it can
// run before ResolveSpecSymbol/ResolveSpecTest even attempts to parse --
// callers should treat a false return as an immediate violation, not attempt
// resolution at all.
func EntryWithinSpecScope(specRoot, file string, selfHosting bool) (ok bool, reason string) {
	if file == "" {
		return false, "empty file path"
	}
	slashFile := filepath.ToSlash(file)
	if strings.HasPrefix(slashFile, "/") || hasVolumePrefix(slashFile) {
		return false, "must be a relative path, not absolute"
	}
	cleaned := filepath.Clean(filepath.FromSlash(slashFile))
	cleanedSlash := filepath.ToSlash(cleaned)
	if cleanedSlash == ".." || strings.HasPrefix(cleanedSlash, "../") {
		return false, "must not traverse above the domain's spec root via \"..\""
	}
	if !selfHosting {
		if !strings.HasPrefix(cleanedSlash, "spec/") {
			return false, "must be a path under \"spec/\" (non-self-hosting domains authored code lives under domainDir/spec/)"
		}
	}
	// Belt-and-braces: even if the prefix/traversal checks above somehow
	// missed a shape, confirm the joined, cleaned, absolute path is still
	// lexically inside specRoot before allowing resolution to proceed.
	absRoot, err := filepath.Abs(specRoot)
	if err != nil {
		return false, fmt.Sprintf("could not resolve spec root: %v", err)
	}
	joined := filepath.Join(absRoot, cleaned)
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return false, fmt.Sprintf("could not resolve joined path: %v", err)
	}
	rel, err := filepath.Rel(absRoot, absJoined)
	if err != nil {
		return false, fmt.Sprintf("could not compute relative path: %v", err)
	}
	relSlash := filepath.ToSlash(rel)
	if relSlash == ".." || strings.HasPrefix(relSlash, "../") {
		return false, "resolved path escapes the domain's spec root"
	}
	return true, ""
}

// hasVolumePrefix reports whether a forward-slash-normalized path starts
// with a Windows drive-letter volume ("C:/...") -- filepath.VolumeName only
// recognizes the OS-native separator form, so on a non-Windows build it
// would miss a "C:/" reference smuggled in with forward slashes; this check
// is separator-agnostic so the scope gate rejects it on every platform.
func hasVolumePrefix(slashPath string) bool {
	if len(slashPath) < 2 {
		return false
	}
	c := slashPath[0]
	isLetter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
	return isLetter && slashPath[1] == ':'
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

// testBodyHasTeeth reports whether fn's body contains at least one REAL
// assertion call, anywhere in the body (including nested inside if/for/
// switch/range/t.Run branches): a call to t.Error/t.Errorf/t.Fatal/
// t.Fatalf/t.FailNow/t.Fail, or a call whose selector name matches a common
// assertion-library pattern (require.*/assert.*). This is the anti-
// vacuousness detector: the mechanical successor to the old honest no-op
// checkEnforcedByTestHasTeeth, now applied to verified_by as a structural
// PROHIBITION for ENFORCED (not an advisory no-op). It does not and cannot
// judge whether the assertions are ABOUT the right thing -- only that the
// test body is not hollow.
//
// Hardened per adversarial review (@fh Probe A): a bare control-flow
// construct (if/for/switch/range) with NO assert call anywhere inside it is
// NOT teeth -- `for i := 0; i < 0; i++ {}` or `if someCondition {}` used to
// satisfy this check by shape alone, letting an always-green test through
// with zero real assertions. The rule is narrowed to "does a real
// assertion-shaped call exist anywhere in the body" -- control-flow no
// longer counts on its own, it is only a container an assertion may (or may
// not) be nested inside.
func testBodyHasTeeth(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok && isTeethCall(call) {
			found = true
			return false
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
// t.Skip/t.Skipf reachable UNCONDITIONALLY: either a direct top-level
// statement of the body's *ast.BlockStmt, or nested inside an `if` whose
// condition is a constant-true expression (so the branch is unconditionally
// taken every time the test runs, the same as if it were not wrapped in an
// `if` at all). A skip guarded by a REAL runtime condition (e.g.
// `if testing.Short() { t.Skip(...) }`) is a normal Go idiom and is NOT
// flagged -- only an unconditional skip (bare, or dressed up behind a
// constant-true condition), which makes the "test" unconditionally prove
// nothing, is treated as the escape hatch this check prohibits.
//
// Hardened per adversarial review (@fh Probe A): `if true { t.Skip(...) }`
// used to slip past detection because the skip call was not a direct
// top-level statement -- it was nested one level inside an `if`. Since the
// condition is a literal `true` (or an equivalent always-true expression,
// see isConstantTrueCond), the branch is unconditionally reached and the
// skip is exactly as unconditional as a bare top-level `t.Skip(...)`.
func testBodyHasTopLevelSkip(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	return blockHasUnconditionalSkip(body.List)
}

// blockHasUnconditionalSkip scans a flat statement list (the top level of a
// function body, or the body of an `if` branch known to be unconditionally
// taken) for a Skip/Skipf call reachable without any REAL runtime
// condition -- either a direct ExprStmt call, or nested inside a further
// `if true { ... }` (recursing so `if true { if true { t.Skip() } }` is
// still caught).
func blockHasUnconditionalSkip(stmts []ast.Stmt) bool {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.ExprStmt:
			call, ok := s.X.(*ast.CallExpr)
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
		case *ast.IfStmt:
			// Only descend into an `if` whose condition is constant-true AND
			// which has no Init statement that could make the condition's
			// truth depend on something evaluated at runtime alongside it --
			// the branch is then unconditionally taken every run, so a skip
			// inside it is exactly as unconditional as a bare top-level
			// skip. Any other condition (a real runtime check, however
			// trivial-looking) is left alone: that is the
			// `testing.Short()`-style idiom this check must NOT flag.
			if s.Init == nil && isConstantTrueCond(s.Cond) {
				if blockHasUnconditionalSkip(s.Body.List) {
					return true
				}
			}
		}
	}
	return false
}

// isConstantTrueCond reports whether cond is a compile-time-constant-true
// boolean expression: the literal identifier `true`, or a parenthesized
// form of it. Deliberately narrow -- this is an anti-evasion check for the
// obvious `if true { t.Skip(...) }` shape, not a general constant-folding
// evaluator; anything that requires actual evaluation (even trivially
// foldable, e.g. `1 == 1`) is left as a real runtime condition and is NOT
// treated as unconditional, keeping the false-positive rate at zero for
// genuine conditional logic.
func isConstantTrueCond(cond ast.Expr) bool {
	switch c := cond.(type) {
	case *ast.Ident:
		return c.Name == "true"
	case *ast.ParenExpr:
		return isConstantTrueCond(c.X)
	default:
		return false
	}
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

// SpecRoot returns the root that implemented_by/verified_by file paths are
// joined onto, for a domain directory.
//
// For an ordinary (non-self-hosting) domain, entries are documented
// (PLAN-authored-spec-discipline.md §4) as domain-relative paths already
// prefixed with "spec/" (e.g. "spec/model/risk.go:NewRisk"), so this helper
// returns domainDir itself -- callers join the full entry path (which
// already starts with "spec/") onto domainDir, not onto domainDir/spec.
//
// For a SELF-HOSTING domain (manifest.json's self_hosting: true, e.g.
// domains/hotam-spec-self) the domain's "code" is not an authored spec/ tree
// under domainDir at all -- it IS the engine itself (internal/, cmd/ at the
// repository root). PLAN-authored-spec-discipline.md §9 documents the
// recursion: an engine-facing requirement's implemented_by/verified_by name
// paths like "internal/ontology/lifecycle.go:Lifecycle", relative to the
// engine repository root, not to domainDir. So when selfHosting is true this
// returns engineRoot(domainDir) instead of domainDir.
//
// It exists as a named seam so the convention is documented in one place and
// callers do not have to re-derive it.
func SpecRoot(domainDir string, selfHosting bool) string {
	if !selfHosting {
		return domainDir
	}
	if root, ok := engineRoot(domainDir); ok {
		return root
	}
	// No go.mod found walking up from domainDir -- fall back to domainDir so
	// resolution still fails cleanly (file not found) rather than panicking
	// or silently resolving somewhere unexpected.
	return domainDir
}

// SpecRootForGraph is SpecRoot applied to a loaded graph's own DomainDir and
// SelfHosting fields -- the convenience form every authored-link check
// (internal/invariants/authored_links.go) actually calls, so each call site
// does not have to repeat `SpecRoot(g.DomainDir, g.SelfHosting)`.
func SpecRootForGraph(g *ontology.Graph) string {
	if g == nil {
		return ""
	}
	return SpecRoot(g.DomainDir, g.SelfHosting)
}

// engineRoot finds the engine repository root using a three-step resolution,
// each step a fallback for when the previous one cannot find go.mod:
//
//  1. Walk UP from domainDir looking for the directory that contains go.mod.
//     This is deliberately robust rather than a hardcoded "two levels up"
//     from domainDir: today domains/hotam-spec-self happens to sit exactly
//     two directories below the repository root, but hardcoding that
//     distance would silently break the moment the domain moved, got nested
//     deeper, or the engine's own layout changed. Walking to go.mod is
//     self-documenting (the marker IS "the directory that owns this Go
//     module"). This is the production path: the real self-hosting domain
//     (domains/hotam-spec-self) lives under the real engine's go.mod, so
//     this step always succeeds there and steps 2-3 never trigger.
//  2. If step 1 finds no go.mod above domainDir (e.g. domainDir is a copy of
//     the self-domain's graph.json/manifest.json under an isolated
//     t.TempDir(), as test fixtures do to avoid mutating the real domain --
//     see cmd/hotam/main_test.go's copySelfDomainUnderRoot -- with no go.mod
//     and no internal/ tree anywhere above it), walk UP from os.Getwd()
//     instead. The hotam binary (and `go test`) is always invoked from
//     inside its own module, so the process's CWD sits under the real
//     engine's go.mod even when the domainDir under test is an isolated
//     copy that is not. This lets internal/-relative implemented_by /
//     verified_by entries resolve against the real engine tree instead of
//     failing to find files that were never copied into the fixture.
//  3. If neither walk finds go.mod, fall back to domainDir unchanged (as
//     before) so resolution still fails cleanly (file not found) rather than
//     panicking or silently resolving somewhere unexpected.
//
// Matches the existing precedent in internal/paths/project_root.go's
// unexported repoRoot() -- duplicated here (rather than imported) to keep
// internal/gate free of a dependency on internal/paths, which resolves the
// CWD-based project root for the running hotam process, a different
// question from "where is the engine root relative to THIS domain
// directory" that self-hosting resolution needs (step 2 above answers a
// narrower version of that same CWD question, only as a fallback).
func engineRoot(domainDir string) (string, bool) {
	if domainDir == "" {
		return "", false
	}
	dir, err := filepath.Abs(domainDir)
	if err != nil {
		return "", false
	}
	if root, ok := walkUpToGoMod(dir); ok {
		return root, true
	}
	if cwd, err := os.Getwd(); err == nil {
		if root, ok := walkUpToGoMod(cwd); ok {
			return root, true
		}
	}
	return "", false
}

// walkUpToGoMod walks UP from start looking for the directory that contains
// go.mod, returning that directory. Returns ok=false if no go.mod is found
// before reaching the filesystem root.
func walkUpToGoMod(start string) (string, bool) {
	dir := start
	for {
		if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
