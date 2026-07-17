package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
)

// vendorExclusionDomainModelSrc is a small, genuinely domain-authored model
// file -- 1 object, 2 fields, 1 method -- the ONLY thing BuildModels/
// ScanModelLayerCounts must count for this fixture.
const vendorExclusionDomainModelSrc = `package model

// Widget is the domain's own authored model -- the only object this fixture
// wants counted.
type Widget struct {
	Name  string
	Count int
}

// IsReady reports whether the widget has a name.
func (w *Widget) IsReady() bool {
	return w.Name != ""
}
`

// writeVendorExclusionFixture builds a domain directory shaped like a real
// authored spec/ tree that has ALSO been through `hotam vendor-recorder`:
// spec/model/widget.go (the real domain model above) plus
// spec/hotamspec/hotamspec.go (a byte-for-byte vendored copy of the
// recorder, via recordervendor.Source() -- the SAME function `hotam
// vendor-recorder` itself calls, so this fixture is not a hand-approximated
// stand-in for the real banner, it IS the real banner). Returns the domain
// directory (DomainDir for a non-self-hosting graph).
func writeVendorExclusionFixture(t *testing.T) (domainDir string) {
	t.Helper()
	root := t.TempDir()

	modelDir := filepath.Join(root, "spec", "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll spec/model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "widget.go"), []byte(vendorExclusionDomainModelSrc), 0o644); err != nil {
		t.Fatalf("WriteFile widget.go: %v", err)
	}

	hotamspecDir := filepath.Join(root, "spec", "hotamspec")
	if err := os.MkdirAll(hotamspecDir, 0o755); err != nil {
		t.Fatalf("MkdirAll spec/hotamspec: %v", err)
	}
	// recordervendor.Source() is the EXACT function `hotam vendor-recorder`
	// calls to produce a vendored copy on disk (cmd/hotam/vendor_recorder.go)
	// -- using it here (rather than a hand-typed banner) means this fixture
	// can never drift out of sync with what the real tool actually writes.
	if err := os.WriteFile(filepath.Join(hotamspecDir, "hotamspec.go"), []byte(recordervendor.Source()), 0o644); err != nil {
		t.Fatalf("WriteFile vendored hotamspec.go: %v", err)
	}

	return root
}

// TestScanDomainModelFiles_ExcludesVendoredRecorder is the zero-trust
// regression test for the defect a coordinator review found: MODELS.md/
// COVERAGE.md's go/ast scan of spec/ used to sweep up the VENDORED
// hotamspec recorder copy (spec/hotamspec/hotamspec.go) alongside the
// domain's own authored models, so a pilot's COVERAGE.md drifted from
// "3 files / 6 objects / 11 fields / 16 methods" to "4 / 14 / 32 / 24"
// purely because the recorder's own types (Scenario, Artifact, Step,
// StepKind, Fact, ...) got counted as domain object-model surface. The
// recorder is engine machinery vendored for a Go-module-boundary reason
// (PLAN-scenario-generated-spec.md §2 D1), never a domain model -- this
// test proves BuildModels/ScanModelLayerCounts now see ONLY the real
// domain model (Widget: 1 object, 2 fields, 1 method) and the vendored
// recorder file is excluded entirely (not even listed as an empty file).
func TestScanDomainModelFiles_ExcludesVendoredRecorder(t *testing.T) {
	domainDir := writeVendorExclusionFixture(t)
	g := &ontology.Graph{
		DomainDir:   domainDir,
		SelfHosting: false,
		Requirements: []ontology.Requirement{
			{
				ID:             "R-widget-ready-needs-name",
				Claim:          "A Widget is ready ONLY once it has a name.",
				Owner:          "spec-author",
				Status:         ontology.StatusSETTLED,
				Enforcement:    ontology.EnforcementENFORCED,
				Enforceability: ontology.EnforceabilityENFORCEABLE,
				ImplementedBy:  []string{"spec/model/widget.go:Widget.IsReady"},
			},
		},
	}

	got := BuildModels(g)

	if !strings.Contains(got, "Widget") {
		t.Errorf("BuildModels output missing the real domain model Widget:\n%s", got)
	}
	for _, forbidden := range []string{"Scenario", "Artifact", "StepKind", "hotamspec.go"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("BuildModels output leaked vendored-recorder content %q -- the recorder must be excluded entirely:\n%s", forbidden, got)
		}
	}

	counts, err := ScanModelLayerCounts(g)
	if err != nil {
		t.Fatalf("ScanModelLayerCounts: %v", err)
	}
	if counts.Files != 1 {
		t.Errorf("ScanModelLayerCounts.Files = %d, want 1 (widget.go only, hotamspec.go excluded)", counts.Files)
	}
	if counts.Objects != 1 {
		t.Errorf("ScanModelLayerCounts.Objects = %d, want 1 (Widget only)", counts.Objects)
	}
	if counts.Fields != 2 {
		t.Errorf("ScanModelLayerCounts.Fields = %d, want 2 (Widget.Name, Widget.Count)", counts.Fields)
	}
	if counts.Methods != 1 {
		t.Errorf("ScanModelLayerCounts.Methods = %d, want 1 (Widget.IsReady)", counts.Methods)
	}
}

// TestIsVendoredRecorderFile_DetectsRealVendoredCopy proves the detector
// itself: a real vendored copy (recordervendor.Source(), byte-for-byte what
// `hotam vendor-recorder` writes) is recognized, an ordinary domain file
// with unrelated content is not, and the check survives being placed under
// a RENAMED directory (not literally "spec/hotamspec/") -- the banner is
// the load-bearing signal, not the path.
func TestIsVendoredRecorderFile_DetectsRealVendoredCopy(t *testing.T) {
	dir := t.TempDir()

	vendoredPath := filepath.Join(dir, "renamed_dir_recorder.go")
	if err := os.WriteFile(vendoredPath, []byte(recordervendor.Source()), 0o644); err != nil {
		t.Fatalf("WriteFile vendored copy: %v", err)
	}
	if !isVendoredRecorderFile(vendoredPath) {
		t.Errorf("isVendoredRecorderFile(%q) = false, want true for a real vendored copy under a renamed path", vendoredPath)
	}

	domainPath := filepath.Join(dir, "widget.go")
	if err := os.WriteFile(domainPath, []byte(vendorExclusionDomainModelSrc), 0o644); err != nil {
		t.Fatalf("WriteFile domain model: %v", err)
	}
	if isVendoredRecorderFile(domainPath) {
		t.Errorf("isVendoredRecorderFile(%q) = true, want false for an ordinary domain-authored file", domainPath)
	}
}
