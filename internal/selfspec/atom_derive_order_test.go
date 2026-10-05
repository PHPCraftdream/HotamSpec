package selfspec

import (
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

// Model split across two files whose alphabetical order (age.go before
// way.go) contradicts the authored test narrative (Way is proven first).
// The projection must follow the test order: package order, then TestXxx
// source order, then Fact call order inside each test.
func atomOrderFixture(t *testing.T) string {
	t.Helper()
	return atomSourceFixture(t, map[string]string{
		"manifest.json":               plainAtomManifest,
		"spec/hotamspec/hotamspec.go": vendor.Source(),
		"spec/model/human.go": `package model

type Human struct{}
`,
		"spec/model/age.go": `package model

// age of the human
func (h Human) Age() int { return 30 }
`,
		"spec/model/way.go": `package model

// way of the human
func (h Human) Way() string { return "practice" }
`,
		"spec/model/human_test.go": `package model

import (
	"testing"

	"example.test/domain/hotamspec"
)

func TestWay(t *testing.T) {
	h := Human{}
	hotamspec.Fact(t, h.Way, "practice")
}

func TestAge(t *testing.T) {
	h := Human{}
	hotamspec.Fact(t, h.Age, 30)
}
`,
	})
}

func TestDiscoverAtomsFollowsTestNarrativeOrder(t *testing.T) {
	root := atomOrderFixture(t)
	discovered, err := DiscoverAtoms(root, registry.New[ontology.Requirement]())
	if err != nil {
		t.Fatal(err)
	}
	requirements := discovered.All()
	if len(requirements) != 2 {
		t.Fatalf("discovered %d atoms, want both model methods", len(requirements))
	}
	// Old behavior sorted by model file name and method position, which put
	// Age (age.go) first; the authored narrative proves Way first.
	if requirements[0].ID != "R-human-way" || requirements[1].ID != "R-human-age" {
		t.Fatalf("narrative order = [%s, %s], want Way before Age", requirements[0].ID, requirements[1].ID)
	}
	if requirements[0].DeclOrder != 1 || requirements[1].DeclOrder != 2 {
		t.Fatalf("DeclOrder = [%d, %d], want [1, 2]", requirements[0].DeclOrder, requirements[1].DeclOrder)
	}
}

func TestDiscoverAtomsNarrativeOrderStableAcrossRuns(t *testing.T) {
	root := atomOrderFixture(t)
	first, err := DiscoverAtoms(root, registry.New[ontology.Requirement]())
	if err != nil {
		t.Fatal(err)
	}
	second, err := DiscoverAtoms(root, registry.New[ontology.Requirement]())
	if err != nil {
		t.Fatal(err)
	}
	a, b := first.All(), second.All()
	if len(a) != len(b) {
		t.Fatalf("run produced %d then %d atoms", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].DeclOrder != b[i].DeclOrder {
			t.Fatalf("atom %d unstable: %s/%d then %s/%d", i, a[i].ID, a[i].DeclOrder, b[i].ID, b[i].DeclOrder)
		}
	}
}
