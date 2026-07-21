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
