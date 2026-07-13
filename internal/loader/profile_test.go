package loader

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveGenProfile covers every manifest-state branch of the profile
// resolver: missing manifest, present-but-no-field, explicit consumer/full,
// malformed JSON, and an unrecognized value — all must degrade gracefully to
// "full" except the two explicit recognized values. Mirrors the
// resolveSelfHosting tolerance precedent (missing file / malformed JSON →
// default).
func TestResolveGenProfile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		manifest string // content written to manifest.json; "" = no file
		want     string
	}{
		{
			name:     "missing manifest defaults to full",
			manifest: "",
			want:     GenProfileFull,
		},
		{
			name:     "present but no gen_profile field defaults to full",
			manifest: `{"self_hosting": false}` + "\n",
			want:     GenProfileFull,
		},
		{
			name:     "explicit consumer",
			manifest: `{"self_hosting": false, "gen_profile": "consumer"}` + "\n",
			want:     GenProfileConsumer,
		},
		{
			name:     "explicit full",
			manifest: `{"self_hosting": true, "gen_profile": "full"}` + "\n",
			want:     GenProfileFull,
		},
		{
			name:     "malformed JSON degrades to full",
			manifest: `{ this is not valid json`,
			want:     GenProfileFull,
		},
		{
			name:     "unrecognized value degrades to full",
			manifest: `{"gen_profile": "consmer"}` + "\n",
			want:     GenProfileFull,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			graphPath := filepath.Join(dir, "graph.json")
			// graph.json content is irrelevant — ResolveGenProfile reads only
			// manifest.json from the same directory.
			if err := os.WriteFile(graphPath, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.manifest != "" {
				if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(tc.manifest), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := ResolveGenProfile(graphPath)
			if got != tc.want {
				t.Errorf("ResolveGenProfile() = %q, want %q", got, tc.want)
			}
		})
	}
}
