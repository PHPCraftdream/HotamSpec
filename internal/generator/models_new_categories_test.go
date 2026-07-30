package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// newCategoriesModelSrc is a small domain-authored fixture exercising every
// extraction category task #393 (W1.1) adds: an interface (OcrRecognizer,
// modeled on the real domains/gpsm-sm port -- PRAT-hotam/domains/gpsm-sm/
// spec/model/vin_ocr.go), a typed const enum group (Status), an untyped
// top-level const (MaxRetries), a struct with a tagged field, and a
// top-level constructor (NewWidget).
const newCategoriesModelSrc = `package model

// Status is the lifecycle state of a Widget.
type Status string

const (
	// StatusActive means the widget is in use.
	StatusActive Status = "active"
	// StatusRetired means the widget was decommissioned.
	StatusRetired Status = "retired"
)

// MaxRetries bounds retry attempts (untyped top-level const).
const MaxRetries = 3

// OcrRecognizer is a port seam -- interface S2 in the domain object map.
type OcrRecognizer interface {
	// Recognize extracts an identity from photo bytes.
	Recognize(photo []byte) (string, error)
}

// Widget is the domain's own authored model.
type Widget struct {
	// Name is the widget's display name.
	Name string ` + "`json:\"name,omitempty\" validate:\"required\"`" + `
	State Status
}

// NewWidget constructs a Widget from its name.
func NewWidget(name string) *Widget {
	return &Widget{Name: name, State: StatusActive}
}
`

// writeNewCategoriesFixture builds a minimal domain directory whose spec/
// tree is ONLY newCategoriesModelSrc (no vendored files involved -- that
// exclusion is covered separately by models_vendor_exclusion_test.go and
// cmd/hotam/model_scan_vendor_exclusion_test.go).
func writeNewCategoriesFixture(t *testing.T) (domainDir string) {
	t.Helper()
	root := t.TempDir()
	modelDir := filepath.Join(root, "spec", "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll spec/model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "widget.go"), []byte(newCategoriesModelSrc), 0o644); err != nil {
		t.Fatalf("WriteFile widget.go: %v", err)
	}
	return root
}

// TestBuildModels_RendersInterfaceMethodsConstructorsConstsAndTags is task
// #393's (W1.1) end-to-end rendering proof: MODELS.md must show an
// interface's own method set (priority #1 -- previously an interface
// rendered as a bare name with no visible contract), a top-level
// constructor, a typed const enum group attached to its own type, an
// untyped top-level const, and a struct field's raw tag text.
func TestBuildModels_RendersInterfaceMethodsConstructorsConstsAndTags(t *testing.T) {
	domainDir := writeNewCategoriesFixture(t)
	g := &ontology.Graph{
		DomainDir:   domainDir,
		SelfHosting: false,
		Stakeholders: []ontology.Stakeholder{
			{ID: "fixture-owner", Name: "Fixture Owner", DeclOrder: 1},
		},
	}

	got := BuildModels(g)

	// Priority #1: interface method signature visible, not just the bare name.
	if !strings.Contains(got, "OcrRecognizer") {
		t.Fatalf("MODELS.md missing interface OcrRecognizer:\n%s", got)
	}
	if !strings.Contains(got, "Recognize(photo []byte) (string, error)") {
		t.Errorf("MODELS.md missing interface method signature:\n%s", got)
	}
	if !strings.Contains(got, "Interface methods:") {
		t.Errorf("MODELS.md missing an 'Interface methods:' section:\n%s", got)
	}

	// Priority #2: top-level constructor visible.
	if !strings.Contains(got, "func NewWidget(name string) *Widget") {
		t.Errorf("MODELS.md missing constructor signature:\n%s", got)
	}
	if !strings.Contains(got, "Functions:") {
		t.Errorf("MODELS.md missing a 'Functions:' section:\n%s", got)
	}

	// Priority #3: typed const enum group attached to its own type.
	if !strings.Contains(got, "StatusActive") || !strings.Contains(got, `"active"`) {
		t.Errorf("MODELS.md missing typed const StatusActive:\n%s", got)
	}
	if !strings.Contains(got, "StatusRetired") {
		t.Errorf("MODELS.md missing typed const StatusRetired:\n%s", got)
	}
	// Untyped top-level const.
	if !strings.Contains(got, "MaxRetries") || !strings.Contains(got, "Constants:") {
		t.Errorf("MODELS.md missing untyped top-level const MaxRetries:\n%s", got)
	}

	// Priority #5: struct tag text preserved verbatim.
	if !strings.Contains(got, `json:"name,omitempty" validate:"required"`) {
		t.Errorf("MODELS.md missing raw struct tag text:\n%s", got)
	}
}

// mockAndPortModelSrc is task #394's (W1.2) fixture: a single-method port
// interface, modeled directly on the real PRAT-hotam/domains/gpsm-sm
// OcrRecognizer seam this task's brief cites (replicated inline, not copied
// from that read-only reference repo).
const mockAndPortModelSrc = `package model

// OcrRecognizer is the photo OCR service seam.
type OcrRecognizer interface {
	// Recognize extracts the VIN/STS identity from photo.
	Recognize(photo []byte) (string, error)
}
`

// mockAndPortTestSrc is the SAME package's _test.go file: the mock struct
// (mockOcrRecognizer, unexported, matching OcrRecognizer's method set) PLUS
// an ordinary test function (TestRecognize_Placeholder) that must never leak
// into MODELS.md's rendering.
const mockAndPortTestSrc = `package model

// mockOcrRecognizer is a test double for OcrRecognizer.
type mockOcrRecognizer struct {
	result string
}

// Recognize returns the configured result.
func (m *mockOcrRecognizer) Recognize(photo []byte) (string, error) {
	return m.result, nil
}

// TestRecognize_Placeholder is an ordinary test function that must never
// leak into MODELS.md's rendering -- the mock pass surfaces ONLY matched
// mock objects, never a file's other test content.
func TestRecognize_Placeholder(t *testing.T) {}
`

// writeMockAndPortFixture builds a minimal domain directory whose spec/model/
// carries a port interface (vin_ocr.go) and its own _test.go file declaring
// the matching mock (vin_ocr_test.go) alongside an ordinary test function.
func writeMockAndPortFixture(t *testing.T) (domainDir string) {
	t.Helper()
	root := t.TempDir()
	modelDir := filepath.Join(root, "spec", "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll spec/model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "vin_ocr.go"), []byte(mockAndPortModelSrc), 0o644); err != nil {
		t.Fatalf("WriteFile vin_ocr.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "vin_ocr_test.go"), []byte(mockAndPortTestSrc), 0o644); err != nil {
		t.Fatalf("WriteFile vin_ocr_test.go: %v", err)
	}
	return root
}

// TestBuildModels_RendersMockAndPortWithModelKindSuffixNoTestLeakage is task
// #394's (W1.2) end-to-end rendering proof: a port interface renders with
// the "(interface, port)" heading suffix, its matching mock renders with
// "(struct, mock)", AND the test file's OTHER content (the plain
// TestRecognize_Placeholder test function alongside the mock struct in the
// same fixture file) does NOT leak into the rendering.
func TestBuildModels_RendersMockAndPortWithModelKindSuffixNoTestLeakage(t *testing.T) {
	domainDir := writeMockAndPortFixture(t)
	g := &ontology.Graph{
		DomainDir:   domainDir,
		SelfHosting: false,
		Stakeholders: []ontology.Stakeholder{
			{ID: "fixture-owner", Name: "Fixture Owner", DeclOrder: 1},
		},
	}

	got := BuildModels(g)

	if !strings.Contains(got, "### `OcrRecognizer` (interface, port)") {
		t.Errorf("MODELS.md missing port heading suffix:\n%s", got)
	}
	if !strings.Contains(got, "### `mockOcrRecognizer` (struct, mock)") {
		t.Errorf("MODELS.md missing mock heading suffix:\n%s", got)
	}
	if !strings.Contains(got, "spec/model/vin_ocr_test.go") {
		t.Errorf("MODELS.md missing the mock's own test-file section header:\n%s", got)
	}

	// The plain test function must never leak in -- neither as a "Functions:"
	// entry nor anywhere else in the rendered output.
	if strings.Contains(got, "TestRecognize_Placeholder") {
		t.Errorf("MODELS.md leaked the ordinary test function TestRecognize_Placeholder:\n%s", got)
	}
}
