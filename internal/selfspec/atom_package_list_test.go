package selfspec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

// rootModulePackageFixture lays out a §14 root-module atom domain: go.mod at
// the fixture root, a vendored recorder under the declared import path, and
// an atom package OUTSIDE spec/model (internal/dogfood).
func rootModulePackageFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.test/root\n\ngo 1.22\n",
		"manifest.json": `{"self_hosting":true,"self_executing_atoms":true,"parent":null,` +
			`"self_executing_atom_packages":["internal/dogfood"],` +
			`"atom_recorder_import_path":"example.test/root/internal/recorder/canon"}`,
		"internal/recorder/canon/hotamspec.go": vendor.Source(),
		"internal/dogfood/dogfood.go": `package dogfood

type Speaker struct{}

// speaks plainly
func (s Speaker) Speak() string { return "plain" }
`,
		"internal/dogfood/dogfood_test.go": `package dogfood

import (
	"testing"

	hotamspec "example.test/root/internal/recorder/canon"
)

func TestSpeaks(t *testing.T) {
	s := Speaker{}
	hotamspec.Fact(t, s.Speak, "plain")
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

// Without the §14 generalization DiscoverAtoms hardwires the spec/model dir
// filter and never threads the manifest package list onto the source graph,
// so the internal/dogfood atom is invisible (the source index would read the
// absent <specRoot>/spec/go.mod and fail outright).
func TestDiscoverAtomsRootModulePackageList(t *testing.T) {
	root := rootModulePackageFixture(t)
	discovered, err := DiscoverAtoms(root, root, registry.New[ontology.Requirement]())
	if err != nil {
		t.Fatal(err)
	}
	req, ok := discovered.Get("R-speaker-speak")
	if !ok {
		t.Fatalf("atom from the listed non-spec/model package not discovered; got %v", discovered.All())
	}
	if len(req.ImplementedBy) != 1 || req.ImplementedBy[0] != "internal/dogfood/dogfood.go:Speaker.Speak" {
		t.Fatalf("ImplementedBy = %v, want the root-module link", req.ImplementedBy)
	}
	if len(req.VerifiedBy) != 1 || req.VerifiedBy[0] != "internal/dogfood/dogfood_test.go:TestSpeaks" {
		t.Fatalf("VerifiedBy = %v, want the listed package's test", req.VerifiedBy)
	}
	if req.Claim != "Speaks plainly — plain." {
		t.Fatalf("Claim = %q", req.Claim)
	}
}
