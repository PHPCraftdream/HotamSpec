package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestCodeProjectionLanguageTransitionsRequireFullyValidAfterState(t *testing.T) {
	for _, transition := range []struct {
		name                        string
		before, after               []string
		beforeDefault, afterDefault string
	}{
		{"add", []string{"en", "ru"}, []string{"en", "ru", "zh"}, "en", "en"},
		{"remove", []string{"en", "ru", "zh"}, []string{"en", "ru"}, "en", "en"},
		{"default", []string{"en", "ru"}, []string{"en", "ru"}, "en", "ru"},
	} {
		t.Run(transition.name, func(t *testing.T) {
			dir := t.TempDir()
			graphPath := filepath.Join(dir, "graph.json")
			graph := validGraph()
			graph.Languages, graph.DefaultLanguage = transition.before, transition.beforeDefault
			for i := range graph.Requirements {
				texts := ontology.LocalizedText{}
				for _, language := range transition.before {
					texts[language] = language + ": " + graph.Requirements[i].ID
				}
				graph.Requirements[i].ClaimTexts = texts
				graph.Requirements[i].Claim = texts[transition.beforeDefault]
			}
			if err := WriteGraph(graphPath, graph); err != nil {
				t.Fatal(err)
			}
			manifest, err := json.Marshal(DomainManifest{
				RequirementsAuthority: RequirementsAuthorityCode,
				Languages:             transition.after, DefaultLanguage: transition.afterDefault,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadGraph(graphPath); err == nil {
				t.Fatal("ordinary reader accepted stale metadata against the new manifest")
			}
			projected, err := LoadGraphForCodeProjection(graphPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateGraph(projected); err == nil {
				t.Fatal("preprojection metadata was treated as a publishable graph")
			}
			for i := range projected.Requirements {
				texts := ontology.LocalizedText{}
				for _, language := range transition.after {
					texts[language] = language + ": " + projected.Requirements[i].ID
				}
				projected.Requirements[i].ClaimTexts = texts
				projected.Requirements[i].Claim = texts[transition.afterDefault]
			}
			if err := ValidateGraph(projected); err != nil {
				t.Fatal(err)
			}
			if err := WriteGraph(graphPath, projected); err != nil {
				t.Fatal(err)
			}
			loaded, err := LoadGraph(graphPath)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Requirements[0].Claim != loaded.Requirements[0].ClaimTexts[transition.afterDefault] {
				t.Fatal("persisted projection lost exact primary-language claim parity")
			}
		})
	}
}

func TestCodeProjectionCannotBypassAuthorityOrCoreGraphValidation(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	if err := WriteGraph(graphPath, validGraph()); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"self_hosting":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGraph(graphPath); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGraphForCodeProjection(graphPath); err == nil {
		t.Fatal("projection mode was available without code authority")
	}
	if err := os.WriteFile(manifestPath, []byte(`{"self_hosting":false,"requirements_authority":"code"}`), 0600); err != nil {
		t.Fatal(err)
	}
	graph := validGraph()
	graph.Requirements[0].ID = ""
	data, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(graphPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGraphForCodeProjection(graphPath); err == nil {
		t.Fatal("projection mode bypassed unchanged requirement identity validation")
	}
}
