package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestSpecFreshnessChecksIndexAndEveryShard(t *testing.T) {
	root := t.TempDir()
	g := &ontology.Graph{
		DomainDir:          root,
		SelfExecutingAtoms: true,
		Requirements: []ontology.Requirement{
			{ID: "REQ-MODEL", ImplementedBy: []string{"model/model.go:Model"}},
			{ID: "REQ-RELATIONS", ImplementedBy: []string{"relations/relations.go:Relations"}},
		},
	}
	documents, err := gate.BuildSpecDocumentsFromRows(g, map[string]gate.SpecRow{})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "docs", "gen", "spec")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("model.md", documents["spec/model.md"])
	if got := compareSpecDocuments(g, []byte(documents["SPEC.md"]), documents); len(got) != 1 || got[0].ID != filepath.Join(dir, "relations.md") {
		t.Fatalf("missing shard not reported: %v", got)
	}
	write("relations.md", "stale relations")
	if got := compareSpecDocuments(g, []byte(documents["SPEC.md"]), documents); len(got) != 1 || got[0].ID != filepath.Join(dir, "relations.md") {
		t.Fatalf("stale shard not reported: %v", got)
	}
	write("relations.md", documents["spec/relations.md"])
	if got := compareSpecDocuments(g, []byte("stale index"), documents); len(got) != 1 || got[0].ID != filepath.Join(root, "docs", "gen", "SPEC.md") {
		t.Fatalf("stale index not reported: %v", got)
	}
	if got := compareSpecDocuments(g, []byte(documents["SPEC.md"]), documents); len(got) != 0 {
		t.Fatalf("fresh documents rejected: %v", got)
	}
	write("authored.md", "operator-authored shard-like document")
	if got := compareSpecDocuments(g, []byte(documents["SPEC.md"]), documents); len(got) != 0 {
		t.Fatalf("authored markdown was classified as an obsolete generated shard: %v", got)
	}
	write("old.md", documents["spec/model.md"])
	if got := compareSpecDocuments(g, []byte(documents["SPEC.md"]), documents); len(got) != 1 || got[0].ID != filepath.Join(dir, "old.md") {
		t.Fatalf("obsolete generated shard not reported: %v", got)
	}
}
