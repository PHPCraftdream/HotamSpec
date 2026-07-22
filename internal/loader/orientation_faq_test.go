package loader

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestResolveOrientationFAQ covers every manifest-state branch of the
// orientation_faq resolver: missing manifest, present-but-no-field, a
// well-formed list, malformed JSON, and the per-entry honest-no-op drops
// (an entry that is not a JSON object, an entry whose Question is empty).
// Mirrors TestResolveGenProfile's tolerance precedent (missing file /
// malformed JSON -> default).
func TestResolveOrientationFAQ(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		manifest string // content written to manifest.json; "" = no file
		want     []OrientationFAQEntry
	}{
		{
			name:     "missing manifest is honest no-op",
			manifest: "",
			want:     nil,
		},
		{
			name:     "present but no orientation_faq field is honest no-op",
			manifest: `{"purpose": "x", "parent": null}` + "\n",
			want:     nil,
		},
		{
			name:     "orientation_faq present but empty list",
			manifest: `{"orientation_faq": []}` + "\n",
			want:     nil,
		},
		{
			name: "well-formed entries parsed with keywords and link",
			manifest: `{"orientation_faq": [
			  {"question": "purpose?", "keywords": ["a", "b"]},
			  {"question": "lifecycle?", "link": "docs/gen/PIPELINE.md"}
			]}` + "\n",
			want: []OrientationFAQEntry{
				{Question: "purpose?", Keywords: []string{"a", "b"}},
				{Question: "lifecycle?", Link: "docs/gen/PIPELINE.md"},
			},
		},
		{
			name:     "malformed JSON is honest no-op",
			manifest: `{ this is not valid json`,
			want:     nil,
		},
		{
			name: "entry with empty Question is dropped (per-entry no-op)",
			manifest: `{"orientation_faq": [
			  {"question": "", "keywords": ["a"]},
			  {"question": "kept?", "keywords": ["b"]}
			]}` + "\n",
			want: []OrientationFAQEntry{
				{Question: "kept?", Keywords: []string{"b"}},
			},
		},
		{
			name: "malformed entry (not a JSON object) is dropped, rest kept",
			manifest: `{"orientation_faq": [
			  "not-an-object",
			  {"question": "kept?", "link": "x.md"}
			]}` + "\n",
			want: []OrientationFAQEntry{
				{Question: "kept?", Link: "x.md"},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			graphPath := filepath.Join(dir, "graph.json")
			if tc.manifest != "" {
				manifestPath := filepath.Join(dir, "manifest.json")
				if err := os.WriteFile(manifestPath, []byte(tc.manifest), 0o644); err != nil {
					t.Fatalf("WriteFile manifest: %v", err)
				}
			}
			got := ResolveOrientationFAQ(graphPath)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ResolveOrientationFAQ = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestResolveOrientationFAQDiagnostic_MalformedManifestDeclaresIntent covers
// the diagnostic signal the invariant layer's fail-closed malformed-manifest
// check depends on: a manifest that fails to parse as JSON, but whose raw
// bytes contain the literal "orientation_faq" field name, must set
// ManifestDeclaresIntent even though Entries stays nil (the tolerant
// contract every other resolver shares is preserved).
func TestResolveOrientationFAQDiagnostic_MalformedManifestDeclaresIntent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	manifest := `{
  "purpose": "x",
  "orientation_faq": [
    {"question": "q", "keywords": ["a"]},
  ]
}
`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest: %v", err)
	}
	diag := ResolveOrientationFAQDiagnostic(graphPath)
	if diag.Entries != nil {
		t.Errorf("expected nil Entries for a malformed manifest, got %#v", diag.Entries)
	}
	if !diag.ManifestExists {
		t.Error("expected ManifestExists true")
	}
	if diag.ManifestParsed {
		t.Error("expected ManifestParsed false for malformed JSON")
	}
	if !diag.ManifestDeclaresIntent {
		t.Error("expected ManifestDeclaresIntent true when raw bytes contain \"orientation_faq\"")
	}
}

// TestResolveOrientationFAQDiagnostic_MalformedManifestNoIntent covers the
// negative: a malformed manifest that never mentions "orientation_faq" at
// all must NOT set ManifestDeclaresIntent, so the invariant layer can still
// treat it as an honest no-op.
func TestResolveOrientationFAQDiagnostic_MalformedManifestNoIntent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	manifest := `{ this is not valid json, no mention of the field`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest: %v", err)
	}
	diag := ResolveOrientationFAQDiagnostic(graphPath)
	if diag.ManifestDeclaresIntent {
		t.Error("expected ManifestDeclaresIntent false when raw bytes never mention \"orientation_faq\"")
	}
}

// TestResolveOrientationFAQDiagnostic_DroppedEntriesReported covers the
// diagnostic signal the invariant layer's fail-closed dropped-entry check
// depends on: Dropped must name every raw entry that failed per-entry
// validation, with its index and reason, while Entries keeps its existing
// tolerant (shortened) contract.
func TestResolveOrientationFAQDiagnostic_DroppedEntriesReported(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	manifest := `{"orientation_faq": [
	  "not-an-object",
	  {"question": "", "keywords": ["a"]},
	  {"question": "kept?", "keywords": ["b"]}
	]}
`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile manifest: %v", err)
	}
	diag := ResolveOrientationFAQDiagnostic(graphPath)
	if len(diag.Entries) != 1 || diag.Entries[0].Question != "kept?" {
		t.Fatalf("expected Entries to keep only the well-formed entry, got %#v", diag.Entries)
	}
	if len(diag.Dropped) != 2 {
		t.Fatalf("expected 2 dropped entries, got %d: %#v", len(diag.Dropped), diag.Dropped)
	}
	if diag.Dropped[0].Index != 0 || diag.Dropped[1].Index != 1 {
		t.Errorf("expected dropped indices [0, 1], got [%d, %d]", diag.Dropped[0].Index, diag.Dropped[1].Index)
	}
}
