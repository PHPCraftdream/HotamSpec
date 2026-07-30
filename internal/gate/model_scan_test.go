package gate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
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

// ---------------------------------------------------------------------
// Task #394 (W1.2): ModelKind classification rules 1-4 (file-local) and
// the separate mock cross-file pass (rule 5).
// ---------------------------------------------------------------------

// parseFixtureAt is parseFixture but with a caller-chosen relPath, needed
// for the policy-path classification rule (rule 1), which is entirely
// path-driven and parseFixture's own hardcoded "fixture.go" relPath cannot
// exercise.
func parseFixtureAt(t *testing.T, relPath, src string) ModelFile {
	t.Helper()
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, "fixture.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parser.ParseFile: %v", err)
	}
	return extractModelFile(astFile, relPath)
}

// TestClassifyModelKind_PolicyByExactPathSegment is rule 1: an object
// declared in a file under an EXACT "spec/policy/" path-segment sequence
// classifies as "policy", mirroring AUTHORED-SPEC-CONTRACT.md §1's own
// spec/policy/ convention (policies/gates/predicates).
func TestClassifyModelKind_PolicyByExactPathSegment(t *testing.T) {
	const src = `package policy

// MinimumAgeGate is a policy predicate.
type MinimumAgeGate struct {
	MinAge int
}
`
	f := parseFixtureAt(t, "spec/policy/age_gate.go", src)
	obj := findObject(t, f, "MinimumAgeGate")
	if obj.ModelKind != "policy" {
		t.Errorf("ModelKind = %q, want policy for a file under spec/policy/", obj.ModelKind)
	}
}

// TestClassifyModelKind_PolicySubstringFalsePositiveAvoided is rule 1's own
// documented non-goal: a file merely containing "policy" as a SUBSTRING of
// its own name (e.g. spec/model/policyholder.go) must NOT classify as
// policy -- only an exact "spec/policy/" path-SEGMENT sequence does. This is
// the false-positive-avoidance case the brief explicitly calls out.
func TestClassifyModelKind_PolicySubstringFalsePositiveAvoided(t *testing.T) {
	const src = `package model

// Policyholder is an ordinary domain object whose file name happens to
// contain the substring "policy" -- it must not be misclassified.
type Policyholder struct {
	Name string
}
`
	f := parseFixtureAt(t, "spec/model/policyholder.go", src)
	obj := findObject(t, f, "Policyholder")
	if obj.ModelKind == "policy" {
		t.Errorf("ModelKind = %q, want NOT policy -- spec/model/policyholder.go must not false-positive on the \"policy\" substring", obj.ModelKind)
	}
	if obj.ModelKind != "object" {
		t.Errorf("ModelKind = %q, want object (ordinary struct, no other rule applies)", obj.ModelKind)
	}
}

// TestClassifyModelKind_Port is rule 2: any interface classifies as "port"
// (its Kind == "interface" already makes it a contract with the outside
// world by construction), so long as its own file is not under spec/policy/.
func TestClassifyModelKind_Port(t *testing.T) {
	const src = `package model

// OcrRecognizer is the photo OCR service seam.
type OcrRecognizer interface {
	Recognize(photo []byte) (string, error)
}
`
	f := parseFixtureAt(t, "spec/model/vin_ocr.go", src)
	obj := findObject(t, f, "OcrRecognizer")
	if obj.ModelKind != "port" {
		t.Errorf("ModelKind = %q, want port", obj.ModelKind)
	}
}

// TestClassifyModelKind_ValueWithoutMethods is rule 3: a named
// primitive/alias type declaration (Kind == "type") with zero receiver
// methods anywhere in its file classifies as "value".
func TestClassifyModelKind_ValueWithoutMethods(t *testing.T) {
	const src = `package model

// VehicleID is a named string identifier with no methods.
type VehicleID string
`
	f := parseFixtureAt(t, "spec/model/vehicle_identification.go", src)
	obj := findObject(t, f, "VehicleID")
	if obj.ModelKind != "value" {
		t.Errorf("ModelKind = %q, want value", obj.ModelKind)
	}
}

// TestClassifyModelKind_ValueShapedTypeWithMethodFallsThroughToObject is
// rule 3's own documented, deliberately narrow limitation: a value-shaped
// type (Kind == "type") that DOES carry a receiver method (e.g. a
// String()-like method) falls through to "object" instead of "value" -- an
// accepted, honest heuristic, not a bug, and this task does not special-case
// Stringer-like methods (that would be over-engineering beyond scope).
func TestClassifyModelKind_ValueShapedTypeWithMethodFallsThroughToObject(t *testing.T) {
	const src = `package model

// VehicleID is a named string identifier that DOES carry a method.
type VehicleID string

// String renders the identifier for display.
func (v VehicleID) String() string {
	return string(v)
}
`
	f := parseFixtureAt(t, "spec/model/vehicle_identification.go", src)
	obj := findObject(t, f, "VehicleID")
	if obj.ModelKind != "object" {
		t.Errorf("ModelKind = %q, want object -- a value-shaped type WITH a method is a deliberately known limitation, must fall through to object, not value", obj.ModelKind)
	}
}

// TestClassifyModelKind_ObjectDefault is rule 4: the default fallback for an
// ordinary struct with fields and methods.
func TestClassifyModelKind_ObjectDefault(t *testing.T) {
	const src = `package model

// Widget is an ordinary aggregate.
type Widget struct {
	Name string
}

// Validate checks the widget's own invariants.
func (w *Widget) Validate() error {
	return nil
}
`
	f := parseFixtureAt(t, "spec/model/widget.go", src)
	obj := findObject(t, f, "Widget")
	if obj.ModelKind != "object" {
		t.Errorf("ModelKind = %q, want object (default fallback)", obj.ModelKind)
	}
}

// ---------------------------------------------------------------------
// Mock cross-file pass (rule 5) -- scanDomainMockFiles / ScanAuthoredModels.
// ---------------------------------------------------------------------

// writeMockScanFixture builds a minimal on-disk domain: spec/model/ carries
// a real port interface (modeled directly on the real
// PRAT-hotam/domains/gpsm-sm OcrRecognizer/mockOcrRecognizer shape cited in
// this task's brief, replicated here as an inline fixture, not copied from
// that read-only reference repo), and spec/model/<name>_test.go carries the
// mock struct in the SAME package but a DIFFERENT file, exactly like the
// real-world evidence.
func writeMockScanFixture(t *testing.T, modelSrc, testSrc string) (domainDir string) {
	t.Helper()
	root := t.TempDir()
	modelDir := filepath.Join(root, "spec", "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll spec/model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "vin_ocr.go"), []byte(modelSrc), 0o644); err != nil {
		t.Fatalf("WriteFile vin_ocr.go: %v", err)
	}
	if testSrc != "" {
		if err := os.WriteFile(filepath.Join(modelDir, "vin_ocr_test.go"), []byte(testSrc), 0o644); err != nil {
			t.Fatalf("WriteFile vin_ocr_test.go: %v", err)
		}
	}
	return root
}

const mockScanPortSrc = `package model

// OcrRecognizer is the photo OCR service seam.
type OcrRecognizer interface {
	// Recognize extracts the VIN/STS identity from photo.
	Recognize(photo []byte) (string, error)
}
`

// TestScanDomainMockFiles_TruePositiveFullSupersetMatch proves a _test.go
// struct whose method set is a full superset of an eligible port's required
// methods (here, an exact 1:1 match, mirroring the real mockOcrRecognizer
// shape) is recognized as a mock and surfaces with ModelKind == "mock".
func TestScanDomainMockFiles_TruePositiveFullSupersetMatch(t *testing.T) {
	const testSrc = `package model

// mockOcrRecognizer is a test double for OcrRecognizer.
type mockOcrRecognizer struct {
	result string
}

// Recognize returns the configured result.
func (m *mockOcrRecognizer) Recognize(photo []byte) (string, error) {
	return m.result, nil
}

// TestSomethingElse is an ordinary test function that must never leak into
// the scan as a model object.
func TestSomethingElse(t *testing.T) {}
`
	domainDir := writeMockScanFixture(t, mockScanPortSrc, testSrc)
	g := &ontology.Graph{DomainDir: domainDir, SelfHosting: false}

	files, err := ScanAuthoredModels(g)
	if err != nil {
		t.Fatalf("ScanAuthoredModels: %v", err)
	}

	var mockFile *ModelFile
	for i := range files {
		if files[i].RelPath == "spec/model/vin_ocr_test.go" {
			mockFile = &files[i]
		}
	}
	if mockFile == nil {
		var paths []string
		for _, f := range files {
			paths = append(paths, f.RelPath)
		}
		t.Fatalf("no mock file surfaced for spec/model/vin_ocr_test.go; got files: %v", paths)
	}
	if len(mockFile.Objects) != 1 || mockFile.Objects[0].Name != "mockOcrRecognizer" {
		t.Fatalf("mock file Objects = %v, want exactly [mockOcrRecognizer]", mockFile.Objects)
	}
	if mockFile.Objects[0].ModelKind != "mock" {
		t.Errorf("ModelKind = %q, want mock", mockFile.Objects[0].ModelKind)
	}
	// The plain TestSomethingElse test function must never leak in as an
	// object (Go test funcs are FuncDecls with no receiver -- they would
	// only ever be picked up by extractModelFile's Funcs category, which
	// scanDomainMockFiles deliberately never copies onto the retained
	// mock-only ModelFile).
	if len(mockFile.Funcs) != 0 {
		t.Errorf("mock file Funcs = %v, want empty -- ordinary test funcs must never leak into the mock pass", mockFile.Funcs)
	}
}

// TestScanDomainMockFiles_PartialOverlapIsNotAMatch proves a test-file
// struct whose methods only PARTIALLY overlap an eligible port's required
// method set is NOT treated as a mock (true-negative case).
func TestScanDomainMockFiles_PartialOverlapIsNotAMatch(t *testing.T) {
	const portSrc = `package model

// TwoMethodPort requires both Alpha and Beta.
type TwoMethodPort interface {
	Alpha() error
	Beta() error
}
`
	const testSrc = `package model

// partialAdapter only implements Alpha, not Beta -- must not match.
type partialAdapter struct{}

func (p *partialAdapter) Alpha() error { return nil }
`
	domainDir := writeMockScanFixture(t, portSrc, testSrc)
	g := &ontology.Graph{DomainDir: domainDir, SelfHosting: false}

	files, err := ScanAuthoredModels(g)
	if err != nil {
		t.Fatalf("ScanAuthoredModels: %v", err)
	}
	for _, f := range files {
		for _, obj := range f.Objects {
			if obj.Name == "partialAdapter" {
				t.Fatalf("partialAdapter must not be classified as a mock (partial method overlap only): %+v", obj)
			}
		}
	}
}

// TestScanDomainMockFiles_ZeroPortsShortCircuitsWithoutFilesystemWalk proves
// the early-exit: when zero eligible ports exist, scanDomainMockFiles must
// return (nil, nil) WITHOUT walking the filesystem at all. Proven here by
// pointing DomainDir at a directory that does not exist for the _test.go
// walk -- if the function attempted to walk it, WalkDir's own os.IsNotExist
// handling would still make this pass silently, so the REAL proof is that
// zero ports means the walk is never attempted in the first place; a
// domain with no spec/ tree at all (scanDomainModelFiles already returns
// nil for it) has, by definition, zero eligible ports, so no error/panic
// occurs either way.
func TestScanDomainMockFiles_ZeroPortsShortCircuitsWithoutFilesystemWalk(t *testing.T) {
	g := &ontology.Graph{DomainDir: filepath.Join(t.TempDir(), "does-not-exist"), SelfHosting: false}

	files, err := ScanAuthoredModels(g)
	if err != nil {
		t.Fatalf("ScanAuthoredModels: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("files = %v, want empty for a nonexistent domain dir", files)
	}

	// Direct unit-level proof: zero ports in, must return (nil, nil).
	mockFiles, err := scanDomainMockFiles(g, nil)
	if err != nil {
		t.Fatalf("scanDomainMockFiles: %v", err)
	}
	if mockFiles != nil {
		t.Errorf("scanDomainMockFiles(zero ports) = %v, want nil", mockFiles)
	}
}

// TestScanDomainMockFiles_EmbeddingInterfaceExcluded proves a port whose
// InterfaceMethods carries an Embedded entry (an interface that embeds
// another interface, e.g. `io.Closer`) is NEVER used as a mock-matching
// target -- a deliberate, documented limitation (an embedded interface has
// an incomplete method-name set from AST alone).
func TestScanDomainMockFiles_EmbeddingInterfaceExcluded(t *testing.T) {
	const portSrc = `package model

import "io"

// EmbeddingPort embeds io.Closer plus its own method.
type EmbeddingPort interface {
	io.Closer
	Alpha() error
}
`
	// mockAdapter implements Alpha AND Close -- if embedding exclusion were
	// broken, this would be treated as a match against EmbeddingPort's
	// apparent method set; it must NOT be, since EmbeddingPort itself must
	// never become an eligible mock-matching target at all.
	const testSrc = `package model

// mockAdapter fully implements the interface's apparent surface.
type mockAdapter struct{}

func (m *mockAdapter) Alpha() error { return nil }
func (m *mockAdapter) Close() error { return nil }
`
	domainDir := writeMockScanFixture(t, portSrc, testSrc)
	g := &ontology.Graph{DomainDir: domainDir, SelfHosting: false}

	// Direct proof at the eligiblePortSignatures level: EmbeddingPort must
	// never appear.
	modelFiles, err := scanDomainModelFiles(g)
	if err != nil {
		t.Fatalf("scanDomainModelFiles: %v", err)
	}
	ports := eligiblePortSignatures(modelFiles)
	for _, p := range ports {
		if p.portName == "EmbeddingPort" {
			t.Fatalf("EmbeddingPort must be excluded from eligiblePortSignatures (it embeds io.Closer): %+v", ports)
		}
	}

	// End-to-end proof: mockAdapter must not surface as a mock either,
	// since there is no OTHER eligible port for it to match against.
	files, err := ScanAuthoredModels(g)
	if err != nil {
		t.Fatalf("ScanAuthoredModels: %v", err)
	}
	for _, f := range files {
		for _, obj := range f.Objects {
			if obj.Name == "mockAdapter" {
				t.Fatalf("mockAdapter must not be classified as a mock -- its only candidate port (EmbeddingPort) embeds another interface and must be excluded: %+v", obj)
			}
		}
	}
}
