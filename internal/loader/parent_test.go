package loader

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveParent covers every manifest-state branch of the parent
// resolver, including the distinction that makes ResolveParent unlike its
// sibling ResolveDiscipline/ResolveGenProfile readers: "key absent from an
// EXISTING manifest" (ManifestExists=true, Declared=false, the
// check_project_parent_declared violation case) MUST be distinguishable both
// from "key present with JSON null" (ManifestExists=true, Declared=true,
// Value="") and from "manifest.json does not exist at all"
// (ManifestExists=false, the honest-no-op case matching every sibling
// resolver's own missing-manifest default). Mirrors profile_test.go's table
// shape (missing manifest / present-but-no-field / explicit values /
// malformed JSON), adding the parent-specific cases plus the non-string-value
// malformed case.
func TestResolveParent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		manifest   string // content written to manifest.json; "" = no file
		wantExists bool
		wantDecl   bool
		wantVal    string
	}{
		{
			name:       "missing manifest → ManifestExists=false (honest no-op, not a violation)",
			manifest:   "",
			wantExists: false,
			wantDecl:   false,
			wantVal:    "",
		},
		{
			name:       "manifest present but no parent key → ManifestExists=true, Declared=false (the D6 violation case)",
			manifest:   `{"self_hosting": false}` + "\n",
			wantExists: true,
			wantDecl:   false,
			wantVal:    "",
		},
		{
			name:       "malformed JSON → ManifestExists=true (the file is there, just unparseable), Declared=false",
			manifest:   `{ this is not valid json`,
			wantExists: true,
			wantDecl:   false,
			wantVal:    "",
		},
		{
			name:       "parent: null → declared root (ManifestExists=true, Declared=true, Value=\"\")",
			manifest:   `{"self_hosting": true, "parent": null}` + "\n",
			wantExists: true,
			wantDecl:   true,
			wantVal:    "",
		},
		{
			name:       "parent: explicit empty string → declared (Value=\"\", same rendering as null)",
			manifest:   `{"parent": ""}` + "\n",
			wantExists: true,
			wantDecl:   true,
			wantVal:    "",
		},
		{
			name:       "parent: non-empty string → declared child (Declared=true, Value=name)",
			manifest:   `{"parent": "hotam-spec-self"}` + "\n",
			wantExists: true,
			wantDecl:   true,
			wantVal:    "hotam-spec-self",
		},
		{
			name:       "parent: non-string value (number) → malformed, not declared",
			manifest:   `{"parent": 42}` + "\n",
			wantExists: true,
			wantDecl:   false,
			wantVal:    "",
		},
		{
			name:       "parent: non-string value (bool) → malformed, not declared",
			manifest:   `{"parent": true}` + "\n",
			wantExists: true,
			wantDecl:   false,
			wantVal:    "",
		},
		{
			name:       "parent: non-string value (object) → malformed, not declared",
			manifest:   `{"parent": {"name": "x"}}` + "\n",
			wantExists: true,
			wantDecl:   false,
			wantVal:    "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			graphPath := filepath.Join(dir, "graph.json")
			// graph.json content is irrelevant — ResolveParent reads only
			// manifest.json from the same directory.
			if err := os.WriteFile(graphPath, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.manifest != "" {
				if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(tc.manifest), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := ResolveParent(graphPath)
			if got.ManifestExists != tc.wantExists {
				t.Errorf("ResolveParent().ManifestExists = %v, want %v", got.ManifestExists, tc.wantExists)
			}
			if got.Declared != tc.wantDecl {
				t.Errorf("ResolveParent().Declared = %v, want %v", got.Declared, tc.wantDecl)
			}
			if got.Value != tc.wantVal {
				t.Errorf("ResolveParent().Value = %q, want %q", got.Value, tc.wantVal)
			}
		})
	}
}

// TestResolveParent_MissingManifestIsHonestNoOp mirrors
// TestResolveDiscipline_MissingManifestIsHonestNoOp: a graphPath with no
// sibling manifest.json at all resolves to ManifestExists=false — the honest
// no-op case check_project_parent_declared bails on, NOT a violation (a
// domain that never had a manifest.json was never given the chance to
// declare parent in the first place).
func TestResolveParent_MissingManifestIsHonestNoOp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json") // no manifest.json ever written
	got := ResolveParent(graphPath)
	if got.ManifestExists {
		t.Fatalf("expected ManifestExists=false when manifest.json is absent, got true (Declared=%v Value=%q)", got.Declared, got.Value)
	}
	if got.Declared {
		t.Fatalf("expected Declared=false when manifest.json is absent, got Declared=true Value=%q", got.Value)
	}
}
