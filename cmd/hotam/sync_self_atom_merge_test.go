package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

// selfAtomMergeFixture lays out a §14 root-module atom domain (go.mod at the
// fixture root, vendored recorder at the declared import path, atom package
// outside spec/model) for mergeSelfAtoms — mirroring sync-domain's
// selfspec.DiscoverAtoms merge fixture shape.
func selfAtomMergeFixture(t *testing.T) string {
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

func selfAtomMergeGraph(root string) *ontology.Graph {
	return &ontology.Graph{
		DomainDir: root, SelfHosting: true, SelfExecutingAtoms: true,
	}
}

// The discovered atom is ADDED to the hand registry; the override is keyed by
// implemented_by[0] (not ID): a hand requirement citing the atom's link under
// a renamed ID wins. A REJECTED hand entry rides through verbatim. Without
// the mergeSelfAtoms generalization, sync-self passes selfspec.Requirements
// straight to SyncGraph and no discovered atom ever joins it.
func TestMergeSelfAtomsAddsDiscoveredAtomAndHonorsImplementedByOverride(t *testing.T) {
	root := selfAtomMergeFixture(t)
	hand := registry.New[ontology.Requirement]()
	hand.MustRegister("R-speaker-renamed", ontology.Requirement{
		ID: "R-speaker-renamed", Claim: "", Owner: "reviewer",
		ImplementedBy: []string{"internal/dogfood/dogfood.go:Speaker.Speak"},
	})
	hand.MustRegister("R-old-atom", ontology.Requirement{
		ID: "R-old-atom", Status: "REJECTED",
		Relations: []ontology.Relation{{Kind: "replaces", Target: "R-speaker-renamed"}},
	})
	merged, err := mergeSelfAtoms(selfAtomMergeGraph(root), hand)
	if err != nil {
		t.Fatal(err)
	}
	atom, ok := merged.Get("R-speaker-renamed")
	if !ok {
		t.Fatalf("discovered atom not merged under the implemented_by-keyed override; got %v", merged.All())
	}
	if !atom.AtomDiscovered {
		t.Fatalf("discovered atom must carry the atom_discovered provenance marker")
	}
	if len(atom.ImplementedBy) != 1 || atom.ImplementedBy[0] != "internal/dogfood/dogfood.go:Speaker.Speak" {
		t.Fatalf("ImplementedBy = %v, want the discovered link", atom.ImplementedBy)
	}
	if atom.Owner != "reviewer" {
		t.Fatalf("Owner = %q, want the hand override", atom.Owner)
	}
	if atom.Claim != "Speaks plainly — plain." {
		t.Fatalf("Claim = %q, want the derived atom claim", atom.Claim)
	}
	rejected, ok := merged.Get("R-old-atom")
	if !ok || rejected.Status != "REJECTED" {
		t.Fatalf("REJECTED hand entry not carried through: %+v", rejected)
	}
}

// Replacing a manual requirement with an atom follows graph append-only: a
// NON-REJECTED requirement present in the BEFORE graph whose ID the merged
// hand+discovered registry no longer carries must be explicitly REJECTED with
// replaces metadata first (mirroring sync-domain). A hand-only entry is NOT
// refused — it is a newly authored requirement sync-self will append to the
// graph.
func TestMergeSelfAtomsRefusesUnmatchedManualRequirement(t *testing.T) {
	root := selfAtomMergeFixture(t)
	hand := registry.New[ontology.Requirement]()
	before := selfAtomMergeGraph(root)
	before.Requirements = []ontology.Requirement{
		{ID: "R-legacy-manual", Claim: "manual", Status: "SETTLED"},
	}
	_, err := mergeSelfAtoms(before, hand)
	if err == nil || !strings.Contains(err.Error(), "REJECTED/replaces") {
		t.Fatalf("graph requirement dropped by the merged registry must be refused with REJECTED/replaces guidance, got %v", err)
	}
}

// A domain without the atom opt-in is an honest no-op.
func TestMergeSelfAtomsNoOpWithoutAtomOptIn(t *testing.T) {
	hand := registry.New[ontology.Requirement]()
	hand.MustRegister("R-manual", ontology.Requirement{ID: "R-manual", Claim: "manual"})
	before := &ontology.Graph{DomainDir: t.TempDir()}
	merged, err := mergeSelfAtoms(before, hand)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := merged.Get("R-manual"); !ok {
		t.Fatal("hand registry not returned unchanged")
	}
}
