package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestSourceMetadataRoundTripsThroughManifestAndGraph(t *testing.T) {
	dir := t.TempDir()
	sources := []ontology.SpecificationSource{{
		ID: "service-spec", Path: "spec/service.md", Version: "2026.1",
		SHA256: strings.Repeat("a", 64),
	}}
	req := ontology.Requirement{
		ID:          "R-source-roundtrip",
		SourceLinks: []ontology.SourceLink{{SourceID: "service-spec", Anchor: "#errors"}},
		Coverage: &ontology.CoverageDeclaration{
			Status: ontology.CoverageUnsupported, Rationale: "not in this edition",
		},
	}
	graphPath := filepath.Join(dir, "graph.json")
	if err := WriteGraph(graphPath, &ontology.Graph{Requirements: []ontology.Requirement{req}}); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(filepath.Join(dir, "manifest.json"), &DomainManifest{SpecificationSources: sources}); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadGraph(graphPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.SpecificationSources, sources) {
		t.Errorf("manifest source projection = %#v, want %#v", loaded.SpecificationSources, sources)
	}
	if len(loaded.Requirements) != 1 || !reflect.DeepEqual(loaded.Requirements[0].SourceLinks, req.SourceLinks) || !reflect.DeepEqual(loaded.Requirements[0].Coverage, req.Coverage) {
		t.Errorf("requirement evidence metadata did not round-trip: %#v", loaded.Requirements)
	}
	graphBytes, err := os.ReadFile(graphPath)
	if err != nil {
		t.Fatal(err)
	}
	var graphFields map[string]json.RawMessage
	if err := json.Unmarshal(graphBytes, &graphFields); err != nil {
		t.Fatal(err)
	}
	if _, exists := graphFields["specification_sources"]; exists {
		t.Errorf("specification_sources must remain manifest-authored, graph fields: %v", graphFields)
	}
}

func TestLoadGraphRejectsMalformedExplicitSourceDeclaration(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(graphPath, []byte(`{"schema_version":3}`), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := `{"self_hosting":false,"parent":null,"specification_sources":"not-an-array"}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGraph(graphPath); err == nil || !strings.Contains(err.Error(), "specification_sources") {
		t.Fatalf("LoadGraph error = %v, want an explicit source-declaration diagnostic", err)
	}
}
