package gate

import (
	"testing"
)

// TestMatchCitedSymbol_ReceiverMethodQualified proves a QUALIFIED "Type.Method"
// citation resolves to a receiver method of the named object.
func TestMatchCitedSymbol_ReceiverMethodQualified(t *testing.T) {
	t.Parallel()
	const src = `package model

type Risk struct{}

func (r *Risk) Validate() error { return nil }
func (r *Risk) Score() int      { return 0 }
`
	f := parseFixture(t, src)
	kind, objName, symbolName, found := MatchCitedSymbol([]ModelFile{f}, "fixture.go", "Risk.Validate")
	if !found {
		t.Fatalf("expected found=true for Risk.Validate")
	}
	if kind != SymbolKindMethod {
		t.Errorf("kind = %v, want SymbolKindMethod", kind)
	}
	if objName != "Risk" {
		t.Errorf("objName = %q, want Risk", objName)
	}
	if symbolName != "Validate" {
		t.Errorf("symbolName = %q, want Validate", symbolName)
	}
}

// TestMatchCitedSymbol_ReceiverMethodBare proves a BARE "Method" citation
// resolves to the first object in the file that declares it.
func TestMatchCitedSymbol_ReceiverMethodBare(t *testing.T) {
	t.Parallel()
	const src = `package model

type Risk struct{}

func (r *Risk) Validate() error { return nil }
`
	f := parseFixture(t, src)
	kind, objName, symbolName, found := MatchCitedSymbol([]ModelFile{f}, "fixture.go", "Validate")
	if !found {
		t.Fatalf("expected found=true for bare Validate")
	}
	if kind != SymbolKindMethod {
		t.Errorf("kind = %v, want SymbolKindMethod", kind)
	}
	if objName != "Risk" {
		t.Errorf("objName = %q, want Risk", objName)
	}
	if symbolName != "Validate" {
		t.Errorf("symbolName = %q, want Validate", symbolName)
	}
}

// TestMatchCitedSymbol_InterfaceMethod proves a "Type.Method" citation naming
// an interface resolves to the interface's own declared method
// (SymbolKindInterfaceMethod).
func TestMatchCitedSymbol_InterfaceMethod(t *testing.T) {
	t.Parallel()
	const src = `package model

type Recognizer interface {
	Recognize(photo []byte) (string, error)
}
`
	f := parseFixture(t, src)
	kind, objName, symbolName, found := MatchCitedSymbol([]ModelFile{f}, "fixture.go", "Recognizer.Recognize")
	if !found {
		t.Fatalf("expected found=true for Recognizer.Recognize")
	}
	if kind != SymbolKindInterfaceMethod {
		t.Errorf("kind = %v, want SymbolKindInterfaceMethod", kind)
	}
	if objName != "Recognizer" {
		t.Errorf("objName = %q, want Recognizer", objName)
	}
	if symbolName != "Recognize" {
		t.Errorf("symbolName = %q, want Recognize", symbolName)
	}
}

// TestMatchCitedSymbol_TopLevelFunc proves a BARE citation naming an exported
// top-level function resolves to SymbolKindFunc.
func TestMatchCitedSymbol_TopLevelFunc(t *testing.T) {
	t.Parallel()
	const src = `package model

func NewRisk(owner string) (*Risk, error) {
	return &Risk{Owner: owner}, nil
}

type Risk struct{ Owner string }
`
	f := parseFixture(t, src)
	kind, objName, symbolName, found := MatchCitedSymbol([]ModelFile{f}, "fixture.go", "NewRisk")
	if !found {
		t.Fatalf("expected found=true for NewRisk")
	}
	if kind != SymbolKindFunc {
		t.Errorf("kind = %v, want SymbolKindFunc", kind)
	}
	if objName != "" {
		t.Errorf("objName = %q, want empty (func has no owning object)", objName)
	}
	if symbolName != "NewRisk" {
		t.Errorf("symbolName = %q, want NewRisk", symbolName)
	}
}

// TestMatchCitedSymbol_QualifiedCitationDoesNotMatchFunc proves a QUALIFIED
// citation whose Type does not match any object in the file does NOT fall
// through to match a top-level func of the same bare name -- the qualifier
// commits to naming a type.
func TestMatchCitedSymbol_QualifiedCitationDoesNotMatchFunc(t *testing.T) {
	t.Parallel()
	const src = `package model

func NewRisk() *Risk { return nil }

type Risk struct{}
`
	f := parseFixture(t, src)
	_, _, _, found := MatchCitedSymbol([]ModelFile{f}, "fixture.go", "OtherType.NewRisk")
	if found {
		t.Fatalf("a qualified citation whose Type does not exist must NOT match a func")
	}
}

// TestMatchCitedSymbol_UnexportedSymbolNotMatched proves an unexported method
// or func is never matched (only EXPORTED symbols are in scope).
func TestMatchCitedSymbol_UnexportedSymbolNotMatched(t *testing.T) {
	t.Parallel()
	const src = `package model

type Risk struct{}

func (r *Risk) validate() error { return nil }

func newRisk() *Risk { return nil }
`
	f := parseFixture(t, src)
	if _, _, _, found := MatchCitedSymbol([]ModelFile{f}, "fixture.go", "validate"); found {
		t.Errorf("unexported receiver method must not match")
	}
	if _, _, _, found := MatchCitedSymbol([]ModelFile{f}, "fixture.go", "newRisk"); found {
		t.Errorf("unexported func must not match")
	}
}

// TestMatchCitedSymbol_WrongFile proves a citation whose file does not match
// any scanned ModelFile returns found=false.
func TestMatchCitedSymbol_WrongFile(t *testing.T) {
	t.Parallel()
	const src = `package model

type Risk struct{}

func (r *Risk) Validate() error { return nil }
`
	f := parseFixture(t, src)
	if _, _, _, found := MatchCitedSymbol([]ModelFile{f}, "other.go", "Risk.Validate"); found {
		t.Errorf("citation into a file not in the scan must not match")
	}
}

// TestMatchCitedSymbol_TypeOnlyCitationNotMatched proves a citation naming a
// TYPE (not a method/func) returns found=false -- types are out of scope for
// this matcher (they are check_implemented_by_symbol_resolvable's concern via
// gate.ResolveSpecSymbol, not MatchCitedSymbol's).
func TestMatchCitedSymbol_TypeOnlyCitationNotMatched(t *testing.T) {
	t.Parallel()
	const src = `package model

type Risk struct{ Owner string }
`
	f := parseFixture(t, src)
	if _, _, _, found := MatchCitedSymbol([]ModelFile{f}, "fixture.go", "Risk"); found {
		t.Errorf("a type-only citation must not match (only methods/interface-methods/funcs are in scope)")
	}
}

// TestMatchCitedSymbol_PriorityReceiverOverFunc proves a qualified "Type.Method"
// citation resolves to the receiver method even when a top-level func of the
// same name exists (receiver methods take priority in the three-category
// order).
func TestMatchCitedSymbol_PriorityReceiverOverFunc(t *testing.T) {
	t.Parallel()
	const src = `package model

type Risk struct{}

func (r *Risk) Validate() error { return nil }

func Validate() bool { return true }
`
	f := parseFixture(t, src)
	kind, objName, _, found := MatchCitedSymbol([]ModelFile{f}, "fixture.go", "Risk.Validate")
	if !found {
		t.Fatalf("expected found=true for Risk.Validate")
	}
	if kind != SymbolKindMethod {
		t.Errorf("kind = %v, want SymbolKindMethod (receiver takes priority over func)", kind)
	}
	if objName != "Risk" {
		t.Errorf("objName = %q, want Risk", objName)
	}
}

// TestMatchCitedSymbol_PathNormalization proves file matching is
// separator/cleaning-insensitive via normalizeRelPath.
func TestMatchCitedSymbol_PathNormalization(t *testing.T) {
	t.Parallel()
	const src = `package model

type Risk struct{}

func (r *Risk) Validate() error { return nil }
`
	f := parseFixture(t, src)
	f.RelPath = "spec/model/risk.go"
	// Citation uses a path that normalizes to the same thing.
	if _, _, _, found := MatchCitedSymbol([]ModelFile{f}, "spec/./model/risk.go", "Risk.Validate"); !found {
		t.Errorf("citation with redundant ./ should match via normalizeRelPath")
	}
}
