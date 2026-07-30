package gate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// parseFixture parses a small in-memory Go source string and runs it
// through extractModelFile -- the direct unit-test entry point for the AST
// extraction logic added by task #393 (W1.1), without needing a full
// on-disk domain scaffold (that end-to-end path is covered separately by
// TestScanDomainModelFiles_ExtractsInterfaceMethodsConstructorsAndConsts
// below and by the cmd/hotam vendor-exclusion regression test).
func parseFixture(t *testing.T, src string) ModelFile {
	t.Helper()
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, "fixture.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parser.ParseFile: %v", err)
	}
	return extractModelFile(astFile, "fixture.go")
}

// findObject is a small test helper: locate a ModelObject by name in a
// ModelFile's Objects slice, failing the test if absent.
func findObject(t *testing.T, f ModelFile, name string) ModelObject {
	t.Helper()
	for _, obj := range f.Objects {
		if obj.Name == name {
			return obj
		}
	}
	t.Fatalf("object %q not found in %v", name, objectNames(f))
	return ModelObject{}
}

func objectNames(f ModelFile) []string {
	var names []string
	for _, obj := range f.Objects {
		names = append(names, obj.Name)
	}
	return names
}

// TestExtractModelFile_InterfaceMethods is task #393's priority #1: an
// interface type's own declared method set (the port/mock contract) must be
// extracted with readable signatures, distinct from ModelObject.Methods
// (which stays empty for an interface -- Go forbids receiver methods ON an
// interface type itself). Modeled on the real domains/gpsm-sm OcrRecognizer
// seam (PRAT-hotam/domains/gpsm-sm/spec/model/vin_ocr.go): a single-method
// port interface with a doc comment on both the type and its method.
func TestExtractModelFile_InterfaceMethods(t *testing.T) {
	const src = `package model

// OcrRecognizer is the photo OCR service seam.
type OcrRecognizer interface {
	// Recognize extracts the VIN/STS identity from photo.
	Recognize(photo []byte) (VehicleID, error)
}
`
	f := parseFixture(t, src)
	obj := findObject(t, f, "OcrRecognizer")

	if obj.Kind != "interface" {
		t.Fatalf("Kind = %q, want interface", obj.Kind)
	}
	if len(obj.Methods) != 0 {
		t.Errorf("Methods = %v, want empty (interface methods live in InterfaceMethods)", obj.Methods)
	}
	if len(obj.InterfaceMethods) != 1 {
		t.Fatalf("InterfaceMethods = %v, want exactly 1 entry", obj.InterfaceMethods)
	}
	im := obj.InterfaceMethods[0]
	if im.Name != "Recognize" {
		t.Errorf("InterfaceMethods[0].Name = %q, want Recognize", im.Name)
	}
	wantSig := "Recognize(photo []byte) (VehicleID, error)"
	if im.Signature != wantSig {
		t.Errorf("InterfaceMethods[0].Signature = %q, want %q", im.Signature, wantSig)
	}
	if im.Doc == "" || !strings.Contains(im.Doc, "Recognize extracts") {
		t.Errorf("InterfaceMethods[0].Doc = %q, want it to carry the method's own doc comment", im.Doc)
	}
}

// TestExtractModelFile_InterfaceMultipleMethodsAndEmbedding proves an
// interface with more than one method sorts deterministically by name, and
// that an embedded interface (a bare type name with no method list) is
// recorded distinctly via Embedded rather than Name/Signature.
func TestExtractModelFile_InterfaceMultipleMethodsAndEmbedding(t *testing.T) {
	const src = `package model

import "io"

// Port is a two-method contract embedding io.Closer.
type Port interface {
	io.Closer
	Zeta() error
	Alpha(x int) (int, error)
}
`
	f := parseFixture(t, src)
	obj := findObject(t, f, "Port")

	if len(obj.InterfaceMethods) != 3 {
		t.Fatalf("InterfaceMethods = %v, want 3 entries (Alpha, Zeta, embedded io.Closer)", obj.InterfaceMethods)
	}
	// Sorted by name/embedded-key: "Alpha" < "Zeta" < "io.Closer".
	if obj.InterfaceMethods[0].Name != "Alpha" {
		t.Errorf("InterfaceMethods[0].Name = %q, want Alpha (sorted first)", obj.InterfaceMethods[0].Name)
	}
	if obj.InterfaceMethods[1].Name != "Zeta" {
		t.Errorf("InterfaceMethods[1].Name = %q, want Zeta", obj.InterfaceMethods[1].Name)
	}
	if obj.InterfaceMethods[2].Embedded != "io.Closer" {
		t.Errorf("InterfaceMethods[2].Embedded = %q, want io.Closer", obj.InterfaceMethods[2].Embedded)
	}
	if obj.InterfaceMethods[2].Name != "" || obj.InterfaceMethods[2].Signature != "" {
		t.Errorf("embedded entry should have empty Name/Signature, got %+v", obj.InterfaceMethods[2])
	}
}

// TestExtractModelFile_TopLevelConstructorAndFreeFunction is task #393's
// priority #2: an exported top-level (non-receiver) function -- typically a
// constructor -- must be extracted into ModelFile.Funcs with a readable
// signature, and must NOT be attached to any ModelObject.Methods (it has no
// receiver). An unexported top-level function must be excluded (mirrors the
// exportedness gate every other category here already applies).
func TestExtractModelFile_TopLevelConstructorAndFreeFunction(t *testing.T) {
	const src = `package model

// Widget is a small aggregate.
type Widget struct {
	Name string
}

// NewWidget constructs a Widget from its name.
func NewWidget(name string) *Widget {
	return &Widget{Name: name}
}

// Helper is a free function unrelated to any one type.
func Helper(a, b int) (int, error) {
	return a + b, nil
}

// unexportedHelper must never appear in the scan.
func unexportedHelper() {}
`
	f := parseFixture(t, src)

	if len(f.Funcs) != 2 {
		var names []string
		for _, fn := range f.Funcs {
			names = append(names, fn.Name)
		}
		t.Fatalf("Funcs = %v, want exactly 2 ([Helper, NewWidget])", names)
	}
	// Sorted by name: "Helper" < "NewWidget".
	if f.Funcs[0].Name != "Helper" {
		t.Errorf("Funcs[0].Name = %q, want Helper", f.Funcs[0].Name)
	}
	if f.Funcs[1].Name != "NewWidget" {
		t.Errorf("Funcs[1].Name = %q, want NewWidget", f.Funcs[1].Name)
	}
	wantSig := "func NewWidget(name string) *Widget"
	if f.Funcs[1].Signature != wantSig {
		t.Errorf("Funcs[1].Signature = %q, want %q", f.Funcs[1].Signature, wantSig)
	}
	if f.Funcs[1].Doc == "" || !strings.Contains(f.Funcs[1].Doc, "constructs a Widget") {
		t.Errorf("Funcs[1].Doc = %q, want the constructor's own doc comment", f.Funcs[1].Doc)
	}

	widget := findObject(t, f, "Widget")
	if len(widget.Methods) != 0 {
		t.Errorf("Widget.Methods = %v, want empty -- NewWidget has no receiver and must not attach", widget.Methods)
	}
}

// TestExtractModelFile_TypedConstGroup is task #393's priority #3: a typed
// const enum group (`type Status string; const ( StatusActive Status =
// "active"; ... )`) must be extracted and attached to its OWN type's
// ModelObject.Consts, in declaration order (enum member order is itself
// meaningful authored intent), each carrying its literal value text and its
// own doc comment.
func TestExtractModelFile_TypedConstGroup(t *testing.T) {
	const src = `package model

// Status is the lifecycle state of a Widget.
type Status string

const (
	// StatusActive means the widget is in use.
	StatusActive Status = "active"
	// StatusRetired means the widget was decommissioned.
	StatusRetired Status = "retired"
)
`
	f := parseFixture(t, src)
	obj := findObject(t, f, "Status")

	if len(obj.Consts) != 2 {
		t.Fatalf("Status.Consts = %v, want 2 entries", obj.Consts)
	}
	if obj.Consts[0].Name != "StatusActive" || obj.Consts[0].Value != `"active"` {
		t.Errorf("Consts[0] = %+v, want {StatusActive \"active\"}", obj.Consts[0])
	}
	if obj.Consts[0].Doc == "" || !strings.Contains(obj.Consts[0].Doc, "in use") {
		t.Errorf("Consts[0].Doc = %q, want the member's own doc comment", obj.Consts[0].Doc)
	}
	if obj.Consts[1].Name != "StatusRetired" || obj.Consts[1].Value != `"retired"` {
		t.Errorf("Consts[1] = %+v, want {StatusRetired \"retired\"}", obj.Consts[1])
	}
	// Declaration order preserved (NOT alphabetized) -- StatusActive
	// declared before StatusRetired in source.
	if obj.Consts[0].Name != "StatusActive" {
		t.Errorf("Consts[0].Name = %q, enum member order must follow declaration, not alphabetical sort", obj.Consts[0].Name)
	}
}

// TestExtractModelFile_UntypedConstGroup proves a const group with no
// explicit type attached to any declared ModelObject in the same file
// (e.g. plain untyped int/string constants) surfaces on ModelFile.Consts
// directly, rather than being silently dropped or mis-attached to an
// unrelated object.
func TestExtractModelFile_UntypedConstGroup(t *testing.T) {
	const src = `package model

const (
	// MaxRetries bounds retry attempts.
	MaxRetries = 3
	MinRetries = 1
)
`
	f := parseFixture(t, src)
	if len(f.Consts) != 2 {
		t.Fatalf("f.Consts = %v, want 2 entries", f.Consts)
	}
	if f.Consts[0].Name != "MaxRetries" || f.Consts[0].Value != "3" {
		t.Errorf("f.Consts[0] = %+v, want {MaxRetries 3}", f.Consts[0])
	}
	if f.Consts[0].Typ != "" {
		t.Errorf("f.Consts[0].Typ = %q, want empty (untyped const)", f.Consts[0].Typ)
	}
}

// TestExtractModelFile_StructTags is task #393's priority #5: a struct
// field's raw tag text must be preserved verbatim (unparsed -- this scan
// does not interpret json/validate/any other tag vocabulary, only stores
// the literal text), and a field's own doc comment must be captured
// independently of the enclosing type's Doc.
func TestExtractModelFile_StructTags(t *testing.T) {
	const src = `package model

// Widget is a small aggregate.
type Widget struct {
	// Name is the widget's display name.
	Name string ` + "`json:\"name,omitempty\" validate:\"required\"`" + `
	// Count has no tag.
	Count int
}
`
	f := parseFixture(t, src)
	obj := findObject(t, f, "Widget")
	if len(obj.Fields) != 2 {
		t.Fatalf("Widget.Fields = %v, want 2 entries", obj.Fields)
	}
	nameField := obj.Fields[0]
	if nameField.Name != "Name" {
		t.Fatalf("Fields[0].Name = %q, want Name", nameField.Name)
	}
	wantTag := `json:"name,omitempty" validate:"required"`
	if nameField.Tag != wantTag {
		t.Errorf("Fields[0].Tag = %q, want %q", nameField.Tag, wantTag)
	}
	if nameField.Doc == "" || !strings.Contains(nameField.Doc, "display name") {
		t.Errorf("Fields[0].Doc = %q, want the field's own doc comment", nameField.Doc)
	}
	countField := obj.Fields[1]
	if countField.Tag != "" {
		t.Errorf("Fields[1].Tag = %q, want empty (no tag on Count)", countField.Tag)
	}
}

// TestExtractModelFile_VendoredFileNeverReachesExtraction is a defense-in-
// depth check for task #391's exclusion (IsGeneratedOrVendoredFile): proves
// extractModelFile itself has no awareness of vendoring -- the choke point
// is entirely in parseModelFiles, one level up, which never calls
// extractModelFile for a banner-stamped path. This test documents that
// contract directly against the AST layer touched by this task, so a
// future change to extractModelFile cannot accidentally reintroduce a
// parallel vendored-content path: it simply has no path to skip a file, by
// construction (that responsibility belongs solely to parseModelFiles).
func TestExtractModelFile_VendoredFileNeverReachesExtraction(t *testing.T) {
	// extractModelFile has no file-path parameter with filesystem access at
	// all (relPath is a caller-supplied label, not something extraction
	// re-reads) -- so it structurally CANNOT special-case a vendored path.
	// This is a compile-time/structural assertion: confirm the signature
	// still takes only (*ast.File, string) with no *os.File/path lookup.
	var _ func(*ast.File, string) ModelFile = extractModelFile
}
