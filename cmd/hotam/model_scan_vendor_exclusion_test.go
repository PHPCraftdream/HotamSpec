package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/generator"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// modelScanFixtureDomainModelSrc is a small, genuinely domain-authored model
// file the fixture below plants alongside the REAL `hotam vendor-ontology` +
// `hotam scaffold-registrydump` output -- the only file
// gate.ScanAuthoredModels must count from this fixture.
//
// Its Widget type deliberately sits in spec/model/registry.go (not
// widget.go): task #391's brief specifically calls out that a genuinely
// domain-authored file whose NAME merely resembles a vendored one (here,
// "registry.go" echoing the vendored spec/hotamontology/registry.go) must
// NOT be excluded -- only a file carrying an ACTUAL known banner as its
// first line is. Naming this fixture file registry.go is the direct proof
// of that non-goal.
const modelScanFixtureDomainModelSrc = `package model

// Widget is the domain's own authored model -- the only object this fixture
// wants counted. It deliberately lives in a file named registry.go (see this
// file's own doc comment) to prove a same-named-but-not-banner-stamped file
// is never excluded by name/path alone.
type Widget struct {
	Name  string
	Count int
}

// IsReady reports whether the widget has a name.
func (w *Widget) IsReady() bool {
	return w.Name != ""
}
`

// writeModelScanVendorExclusionFixture builds a domain directory that has
// gone through BOTH real vendoring/scaffolding steps this task closes the
// exclusion gap for -- `hotam vendor-ontology` (task #365,
// spec/hotamontology/requirement.go + registry.go) and
// `hotam scaffold-registrydump` (task #367, spec/registrydump/main.go) --
// PLUS a real domain-authored model file at spec/model/registry.go. Modeled
// directly on newSyncDomainFixture (sync_domain_test.go)'s spec/ scaffolding
// sequence, minus the requirements.go/sync-domain-specific pieces this test
// does not need: it only needs the files ScanAuthoredModels walks, not a
// working registrydump subprocess.
func writeModelScanVendorExclusionFixture(t *testing.T) (domainDir string) {
	t.Helper()
	_, domainDir = initDomainUnderRoot(t, "fixture-model-scan", "2026-07-30")

	specDir := filepath.Join(domainDir, "spec")
	const modulePath = "hotamspec-fixture-model-scan"
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "go.mod"), []byte("module "+modulePath+"\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write spec/go.mod: %v", err)
	}

	// Real vendor-ontology writer (task #365) -- spec/hotamontology/{requirement,registry}.go.
	if _, err := vendorOntology(domainDir); err != nil {
		t.Fatalf("vendorOntology: %v", err)
	}

	// Real scaffold-registrydump writer (task #367) -- spec/registrydump/main.go.
	// Requires the vendored hotamontology package to already exist (just written above).
	if _, err := scaffoldRegistrydump(domainDir); err != nil {
		t.Fatalf("scaffoldRegistrydump: %v", err)
	}

	// Real domain-authored model, deliberately named registry.go -- see
	// modelScanFixtureDomainModelSrc's own doc comment for why.
	modelDir := filepath.Join(specDir, "model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("mkdir spec/model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "registry.go"), []byte(modelScanFixtureDomainModelSrc), 0o644); err != nil {
		t.Fatalf("write spec/model/registry.go: %v", err)
	}

	return domainDir
}

// TestScanAuthoredModels_ExcludesVendorOntologyAndRegistrydumpScaffold is
// task #391's (W0.4) key regression test: it proves gate.ScanAuthoredModels
// -- the single scan MODELS.md, COVERAGE.md, and check_model_complete all
// funnel through -- excludes BOTH the vendored ontology mirror
// (spec/hotamontology/, `hotam vendor-ontology`) and the generated
// registrydump scaffold (spec/registrydump/, `hotam scaffold-registrydump`)
// from its authored-model inventory, while still scanning a REAL
// domain-authored file whose name/path merely resembles a vendored one
// (spec/model/registry.go).
//
// Before this task, only the recorder (spec/hotamspec/) had this exclusion
// (internal/gate/model_scan.go's now-retired vendoredRecorderBannerFirstLine
// / IsVendoredRecorderFile, scoped to exactly one banner) -- the ontology
// mirror's own exported types (Relation, Requirement, Registry --
// internal/ontology/canon/requirement.go, registry.go) and anything the
// registrydump scaffold declares would leak into MODELS.md/COVERAGE.md/
// check_model_complete as soon as a consumer domain adopted
// requirements_authority:"code" (R-domain-founded-in-wave-order step 6),
// exactly the "framework boilerplate leaked into the business folder" defect
// class task #367 (the `life` domain, task #364) already caught once this
// same session.
func TestScanAuthoredModels_ExcludesVendorOntologyAndRegistrydumpScaffold(t *testing.T) {
	domainDir := writeModelScanVendorExclusionFixture(t)

	g := &ontology.Graph{
		DomainDir:   domainDir,
		SelfHosting: false,
		// A minimal Stakeholder so g.IsEmpty() is false -- BuildModels below
		// (unlike gate.ScanAuthoredModels, called directly first) short-circuits
		// to an "empty domain" notice for a truly empty graph, which this
		// fixture is not (it has a real, vendored, and scaffolded spec/ tree).
		Stakeholders: []ontology.Stakeholder{
			{ID: "fixture-owner", Name: "Fixture Owner", DeclOrder: 1},
		},
	}

	files, err := gate.ScanAuthoredModels(g)
	if err != nil {
		t.Fatalf("ScanAuthoredModels: %v", err)
	}

	if len(files) != 1 {
		var paths []string
		for _, f := range files {
			paths = append(paths, f.RelPath)
		}
		t.Fatalf("ScanAuthoredModels returned %d file(s), want exactly 1 (spec/model/registry.go only): %v", len(files), paths)
	}
	if files[0].RelPath != "spec/model/registry.go" {
		t.Errorf("ScanAuthoredModels file = %q, want spec/model/registry.go", files[0].RelPath)
	}

	var objNames []string
	for _, obj := range files[0].Objects {
		objNames = append(objNames, obj.Name)
	}
	if len(objNames) != 1 || objNames[0] != "Widget" {
		t.Errorf("ScanAuthoredModels objects = %v, want exactly [Widget] -- the vendored ontology mirror's own Relation/Requirement/Registry types must never appear here", objNames)
	}

	// Belt-and-braces: render MODELS.md itself (the actual consumer surface
	// a business-domain resolver reads) and prove none of the vendored
	// ontology mirror's or registrydump scaffold's own exported symbols leak
	// into it, while the real Widget model is present.
	rendered := generator.BuildModels(g)
	if !strings.Contains(rendered, "Widget") {
		t.Errorf("MODELS.md missing the real domain model Widget:\n%s", rendered)
	}
	for _, forbidden := range []string{"Relation", "hotamontology", "registrydump", "spec.Requirements"} {
		if strings.Contains(rendered, forbidden) {
			t.Errorf("MODELS.md leaked vendored/generated content %q -- vendored ontology mirror and registrydump scaffold must be excluded entirely:\n%s", forbidden, rendered)
		}
	}
}

// TestIsGeneratedOrVendoredFile_RecognizesAllThreeKnownBanners is the direct
// unit test for gate.IsGeneratedOrVendoredFile's unified banner registry: a
// real vendored/generated copy of each of the three known producers
// (recorder, ontology mirror, registrydump scaffold) is recognized, and an
// ordinary domain-authored file -- even one whose name closely echoes a
// vendored file's own name (registry.go) -- is not.
func TestIsGeneratedOrVendoredFile_RecognizesAllThreeKnownBanners(t *testing.T) {
	domainDir := writeModelScanVendorExclusionFixture(t)
	specDir := filepath.Join(domainDir, "spec")

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"vendored ontology requirement.go", filepath.Join(specDir, "hotamontology", "requirement.go"), true},
		{"vendored ontology registry.go", filepath.Join(specDir, "hotamontology", "registry.go"), true},
		{"generated registrydump main.go", filepath.Join(specDir, "registrydump", "main.go"), true},
		{"real domain model registry.go", filepath.Join(specDir, "model", "registry.go"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gate.IsGeneratedOrVendoredFile(tc.path)
			if got != tc.want {
				t.Errorf("IsGeneratedOrVendoredFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
