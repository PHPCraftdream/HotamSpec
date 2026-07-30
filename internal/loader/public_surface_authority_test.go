package loader

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolvePublicSurfaceAuthorityLinked covers every manifest-state branch
// of resolvePublicSurfaceAuthorityLinked (task #396/W1.4): missing manifest,
// present-but-no-field, explicit "linked", malformed JSON, and an
// unrecognized value — all must degrade gracefully to false (absent) except
// the single recognized literal "linked". Mirrors TestResolveGenProfile's
// table-driven shape (profile_test.go) and resolveClaimAuthorityScenario's
// own documented tolerance contract (missing file / malformed JSON /
// unrecognized value → false).
func TestResolvePublicSurfaceAuthorityLinked(t *testing.T) {
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
			name:     "present but no public_surface_authority field defaults to false",
			manifest: `{"discipline": "full"}` + "\n",
			want:     false,
		},
		{
			name:     "explicit linked",
			manifest: `{"discipline": "full", "public_surface_authority": "linked"}` + "\n",
			want:     true,
		},
		{
			name:     "explicit but unrelated value (authored) defaults to false",
			manifest: `{"public_surface_authority": "authored"}` + "\n",
			want:     false,
		},
		{
			name:     "malformed JSON degrades to false",
			manifest: `{ this is not valid json`,
			want:     false,
		},
		{
			name:     "unrecognized value degrades to false",
			manifest: `{"public_surface_authority": "lnked"}` + "\n",
			want:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			graphPath := filepath.Join(dir, "graph.json")
			// graph.json content is irrelevant —
			// resolvePublicSurfaceAuthorityLinked reads only manifest.json
			// from the same directory.
			if err := os.WriteFile(graphPath, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.manifest != "" {
				if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(tc.manifest), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := resolvePublicSurfaceAuthorityLinked(graphPath)
			if got != tc.want {
				t.Errorf("resolvePublicSurfaceAuthorityLinked() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLoadGraph_PublicSurfaceAuthorityLinked proves the field is correctly
// wired into LoadGraph's Graph{...} literal — g.PublicSurfaceAuthorityLinked
// reflects the manifest's live public_surface_authority value end-to-end,
// mirroring LoadGraph's existing SelfHosting/RequirementsAuthorityCode/
// ClaimAuthorityScenario wiring.
func TestLoadGraph_PublicSurfaceAuthorityLinked(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(graphPath, []byte(`{"schema_version":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"),
		[]byte(`{"discipline": "full", "public_surface_authority": "linked"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g, err := LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	if !g.PublicSurfaceAuthorityLinked {
		t.Errorf("g.PublicSurfaceAuthorityLinked = false, want true")
	}
}

// TestLoadGraph_PublicSurfaceAuthorityLinked_AbsentDefaultsFalse proves the
// absent-key default holds through the full LoadGraph path, not just the
// bare resolver.
func TestLoadGraph_PublicSurfaceAuthorityLinked_AbsentDefaultsFalse(t *testing.T) {
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
	if g.PublicSurfaceAuthorityLinked {
		t.Errorf("g.PublicSurfaceAuthorityLinked = true, want false (key absent)")
	}
}
