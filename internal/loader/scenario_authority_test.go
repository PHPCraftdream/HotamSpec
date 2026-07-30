package loader

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveScenarioAuthorityQuality covers every manifest-state branch of
// resolveScenarioAuthorityQuality (task #397/W1.5): missing manifest,
// present-but-no-field, explicit "quality", malformed JSON, and an
// unrecognized value — all must degrade gracefully to false (absent) except
// the single recognized literal "quality". Mirrors
// TestResolvePublicSurfaceAuthorityLinked's table-driven shape
// (public_surface_authority_test.go).
func TestResolveScenarioAuthorityQuality(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		manifest string // content written to manifest.json; "" = no file
		want     bool
	}{
		{
			name:     "missing manifest defaults to false",
			manifest: "",
			want:     false,
		},
		{
			name:     "present but no scenario_authority field defaults to false",
			manifest: `{"discipline": "full"}` + "\n",
			want:     false,
		},
		{
			name:     "explicit quality",
			manifest: `{"discipline": "full", "scenario_authority": "quality"}` + "\n",
			want:     true,
		},
		{
			name:     "explicit but unrelated value (informal) defaults to false",
			manifest: `{"scenario_authority": "informal"}` + "\n",
			want:     false,
		},
		{
			name:     "malformed JSON degrades to false",
			manifest: `{ this is not valid json`,
			want:     false,
		},
		{
			name:     "unrecognized value degrades to false",
			manifest: `{"scenario_authority": "qualty"}` + "\n",
			want:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			graphPath := filepath.Join(dir, "graph.json")
			// graph.json content is irrelevant —
			// resolveScenarioAuthorityQuality reads only manifest.json from
			// the same directory.
			if err := os.WriteFile(graphPath, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.manifest != "" {
				if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(tc.manifest), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := resolveScenarioAuthorityQuality(graphPath)
			if got != tc.want {
				t.Errorf("resolveScenarioAuthorityQuality() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLoadGraph_ScenarioAuthorityQuality proves the field is correctly wired
// into LoadGraph's Graph{...} literal — g.ScenarioAuthorityQuality reflects
// the manifest's live scenario_authority value end-to-end, mirroring
// LoadGraph's existing SelfHosting/ClaimAuthorityScenario/
// PublicSurfaceAuthorityLinked wiring.
func TestLoadGraph_ScenarioAuthorityQuality(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(graphPath, []byte(`{"schema_version":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"),
		[]byte(`{"discipline": "full", "scenario_authority": "quality"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g, err := LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	if !g.ScenarioAuthorityQuality {
		t.Errorf("g.ScenarioAuthorityQuality = false, want true")
	}
}

// TestLoadGraph_ScenarioAuthorityQuality_AbsentDefaultsFalse proves the
// absent-key default holds through the full LoadGraph path, not just the
// bare resolver.
func TestLoadGraph_ScenarioAuthorityQuality_AbsentDefaultsFalse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(graphPath, []byte(`{"schema_version":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"),
		[]byte(`{"discipline": "full"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g, err := LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	if g.ScenarioAuthorityQuality {
		t.Errorf("g.ScenarioAuthorityQuality = true, want false (key absent)")
	}
}
