package gate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// writeRootModuleAtomFixture lays out a self-hosting-style root module:
// go.mod at the root, an atom package under internal/, a manifest declaring
// the root-module atom opt-in (docs/AUTHORED-SPEC-CONTRACT.md §14).
func writeRootModuleAtomFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.test/root\n\ngo 1.22\n",
		"manifest.json": `{"self_executing_atoms":true,"parent":null,` +
			`"self_executing_atom_packages":["internal/dogfood"],` +
			`"atom_recorder_import_path":"example.test/root/internal/recorder/canon"}`,
		"internal/dogfood/dogfood.go": `package dogfood

type Speaker struct{}

// speaks plainly
func (s Speaker) Speak() string { return "plain" }
`,
		"internal/dogfood/dogfood_test.go": `package dogfood

import "testing"

func TestSpeaks(t *testing.T) {
	if (Speaker{}).Speak() != "plain" {
		t.Fatal("unexpected speak")
	}
}
`,
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func rootModuleAtomGraph(root string) *ontology.Graph {
	return &ontology.Graph{
		DomainDir:                 root,
		SelfHosting:               true,
		SelfExecutingAtoms:        true,
		SelfExecutingAtomPackages: []string{"internal/dogfood"},
		AtomRecorderImportPath:    "example.test/root/internal/recorder/canon",
	}
}

// Without the root-module layout the index reads <specRoot>/spec/go.mod
// (absent here) and walks spec/model (absent), so both halves must fail.
func TestCollectAtomExecutionSnapshotForPackagesFiltersReferences(t *testing.T) {
	root := writeRootModuleAtomFixture(t)
	files := map[string]string{
		"internal/other/other_test.go": "package other\nimport \"testing\"\nfunc TestOther(t *testing.T) {}\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	graph := rootModuleAtomGraph(root)
	graph.SelfExecutingAtoms = false // isolate requirement-reference collection from atom discovery
	graph.Requirements = []ontology.Requirement{{
		Status:     ontology.StatusSETTLED,
		VerifiedBy: []string{"internal/dogfood/dogfood_test.go:TestSpeaks", "internal/other/other_test.go:TestOther"},
	}}
	filtered, err := CollectAtomExecutionSnapshotForPackages(graph, []string{"internal/dogfood"})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.PackageFiles) != 1 || filtered.PackageFiles["internal/dogfood"] == "" {
		t.Fatalf("filtered PackageFiles = %#v", filtered.PackageFiles)
	}
	if len(filtered.PackageRuns) != 1 || filtered.PackageRuns["internal/dogfood"].Err != nil {
		t.Fatalf("filtered PackageRuns = %#v", filtered.PackageRuns)
	}
	all, err := CollectAtomExecutionSnapshotForPackages(graph, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all.PackageFiles) != 2 || all.PackageFiles["internal/dogfood"] == "" || all.PackageFiles["internal/other"] == "" {
		t.Fatalf("unfiltered PackageFiles = %#v", all.PackageFiles)
	}
	if len(all.PackageRuns) != 2 {
		t.Fatalf("unfiltered PackageRuns keys = %#v", all.PackageRuns)
	}
}

func TestCollectSpecRowsUsesLegacyFallbackOutsideDeclaredAtomPackages(t *testing.T) {
	root := writeRootModuleAtomFixture(t)
	other := filepath.Join(root, "internal", "other", "other_test.go")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("package other\nimport \"testing\"\nfunc TestOther(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	graph := rootModuleAtomGraph(root)
	graph.Requirements = []ontology.Requirement{{
		ID: "REQ-OUTSIDE", Status: ontology.StatusSETTLED,
		ImplementedBy: []string{"internal/dogfood/dogfood.go:Speaker.Speak"},
		VerifiedBy:    []string{"internal/other/other_test.go:TestOther"},
	}}

	rows := CollectSpecRows(graph)
	row, ok := rows["REQ-OUTSIDE"]
	if !ok || len(row.outcomes) != 1 {
		t.Fatalf("CollectSpecRows outcome = %#v, want one outcome", row.outcomes)
	}
	if row.outcomes[0].problem == "verified_by package was not present in the shared execution snapshot" {
		t.Fatalf("legacy fallback was not used: %#v", row.outcomes[0])
	}
}

func TestNewAtomSourceIndexForGraphRootModulePackages(t *testing.T) {
	root := writeRootModuleAtomFixture(t)
	index, err := NewAtomSourceIndexForGraph(rootModuleAtomGraph(root))
	if err != nil {
		t.Fatal(err)
	}
	if index.RecorderImportPath != "example.test/root/internal/recorder/canon" {
		t.Fatalf("RecorderImportPath = %q, want manifest value", index.RecorderImportPath)
	}
	// §14: implemented_by links stay file:symbol relative to the module root.
	source, err := index.ResolveLink("internal/dogfood/dogfood.go:Speaker.Speak")
	if err != nil {
		t.Fatalf("link in listed package does not resolve: %v", err)
	}
	full, err := index.Resolve("example.test/root/internal/dogfood.Speaker.Speak")
	if err != nil {
		t.Fatalf("full subject in listed package does not resolve: %v", err)
	}
	if full.PackagePath != source.PackagePath {
		t.Fatalf("full/short subjects disagree: %q vs %q", full.PackagePath, source.PackagePath)
	}
	if source.PackagePath != "example.test/root/internal/dogfood" {
		t.Fatalf("PackagePath = %q, want root-module path", source.PackagePath)
	}
	if source.Phrase != "speaks plainly" {
		t.Fatalf("Phrase = %q, want source doc phrase", source.Phrase)
	}
}

func TestNewAtomSourceIndexForGraphRootModulePackagesRequiresRecorderPath(t *testing.T) {
	root := writeRootModuleAtomFixture(t)
	graph := rootModuleAtomGraph(root)
	graph.AtomRecorderImportPath = ""
	if _, err := NewAtomSourceIndexForGraph(graph); err == nil {
		t.Fatal("index without an explicit recorder import path must fail")
	}
}

func TestDiscoverSnapshotAtomTestsFindsRootPackageTests(t *testing.T) {
	root := writeRootModuleAtomFixture(t)
	snapshot := &AtomExecutionSnapshot{
		PackageFiles: make(map[string]string),
		PackageRuns:  make(map[string]RecordingResult),
		TestFiles:    make(map[string]map[string]string),
	}
	if err := discoverSnapshotAtomTests(root, []string{"internal/dogfood"}, snapshot); err != nil {
		t.Fatal(err)
	}
	if got := snapshot.TestFiles["internal/dogfood"]["TestSpeaks"]; got != "internal/dogfood/dogfood_test.go" {
		t.Fatalf("TestFiles entry = %q, want the listed package's test", got)
	}
	if got := snapshot.PackageFiles["internal/dogfood"]; got != "internal/dogfood/dogfood_test.go" {
		t.Fatalf("PackageFiles entry = %q, want the listed package's test", got)
	}
}
