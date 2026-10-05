package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestValidateLanguages_LegacyAndExplicitSingleLanguage(t *testing.T) {
	if err := validateLanguages(nil, ""); err != nil {
		t.Fatalf("legacy configuration: %v", err)
	}
	if err := validateLanguages([]string{"ru"}, ""); err != nil {
		t.Fatalf("single supported language without explicit default: %v", err)
	}
}

func TestLoadManifestRejectsInvalidLanguageConfiguration(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"empty language list", `{"languages":[]}`},
		{"duplicate language", `{"languages":["en","en"],"default_language":"en"}`},
		{"unsafe path language", `{"languages":["../ru"]}`},
		{"unsupported catalog", `{"languages":["fr"]}`},
		{"missing multilingual default", `{"languages":["en","ru"]}`},
		{"default outside list", `{"languages":["en"],"default_language":"ru"}`},
		{"noncanonical field spelling", `{"Languages":["en"]}`},
		{"null languages", `{"languages":null}`},
		{"empty default string", `{"languages":["ru"],"default_language":""}`},
		{"null conformance object", `{"conformance":null}`},
		{"null manifest", `null`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "manifest.json")
			if err := os.WriteFile(path, []byte(tc.json), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadManifest(path); err == nil {
				t.Fatalf("LoadManifest accepted invalid language configuration %s", tc.json)
			}
		})
	}
}

func TestConformanceConfig_ExplicitEmptyInventorySurvivesSerialization(t *testing.T) {
	manifest := DomainManifest{
		Conformance: &ontology.ConformanceConfig{Clauses: []ontology.SourceClause{}},
	}
	data, err := marshalManifest(&manifest)
	if err != nil {
		t.Fatalf("marshalManifest: %v", err)
	}
	var decoded DomainManifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if decoded.Conformance == nil || decoded.Conformance.Clauses == nil || len(decoded.Conformance.Clauses) != 0 {
		t.Fatalf("explicit empty clauses declaration was lost: %+v", decoded.Conformance)
	}
	if decoded.Conformance.Profiles != nil || decoded.Conformance.Compositions != nil {
		t.Fatalf("absent conformance fields became explicit: %+v", decoded.Conformance)
	}
}
func TestGraphRoundTrip_PreservesSecondaryClaimText(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	manifest := []byte(`{"languages":["en","ru"],"default_language":"en"}`)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	graph := validGraph()
	graph.Requirements[0].Claim = "English claim"
	graph.Requirements[0].ClaimTexts = ontology.LocalizedText{
		"en": "English claim",
		"ru": "Русское требование",
	}
	if err := WriteGraph(graphPath, graph); err != nil {
		t.Fatalf("WriteGraph: %v", err)
	}
	loaded, err := LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	if loaded.Requirements[0].Claim != "English claim" ||
		loaded.Requirements[0].ClaimTexts["ru"] != "Русское требование" {
		t.Fatalf("secondary-language claim text did not survive graph round-trip: %+v", loaded.Requirements[0])
	}
}
