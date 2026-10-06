package selfspec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

// The self-hosting domain's manifest.json sits NEXT TO ITS GRAPH (e.g.
// domains/hotam-spec-self/manifest.json), not at the engine root — SpecRoot
// resolves to the engine root only for Go source indexing. Discovery must
// read the manifest from the domain dir even when the two differ.
func TestDiscoverAtomsReadsManifestFromDomainDir(t *testing.T) {
	root := rootModulePackageFixture(t)
	domainDir := filepath.Join(root, "domains", "selfdomain")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// remove the engine-root manifest: it belongs to the domain dir only
	manifest, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "manifest.json"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	discovered, err := DiscoverAtoms(root, domainDir, registry.New[ontology.Requirement]())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := discovered.Get("R-speaker-speak"); !ok {
		t.Fatalf("atom not discovered when manifest lives in the domain dir; got %v", discovered.All())
	}
}
