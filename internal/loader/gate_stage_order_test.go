package loader

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestResolveGateStageOrder covers every manifest-state branch of the
// gate_stage_order resolver: missing manifest, present-but-no-field, a
// well-formed list, malformed JSON, and the per-entry honest-no-op drop
// (a blank string entry). Mirrors TestResolveOrientationFAQ's exact table
// shape in orientation_faq_test.go.
func TestResolveGateStageOrder(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		manifest string // content written to manifest.json; "" = no file
		want     []string
	}{
		{
			name:     "missing manifest is honest no-op",
			manifest: "",
			want:     nil,
		},
		{
			name:     "present but no gate_stage_order field is honest no-op",
			manifest: `{"purpose": "x", "parent": null}` + "\n",
			want:     nil,
		},
		{
			name:     "gate_stage_order present but empty list",
			manifest: `{"gate_stage_order": []}` + "\n",
			want:     nil,
		},
		{
			name:     "well-formed stage order parsed in declared sequence",
			manifest: `{"gate_stage_order": ["P-G0", "P-G1", "P-G2", "P-G3", "P-G4"]}` + "\n",
			want:     []string{"P-G0", "P-G1", "P-G2", "P-G3", "P-G4"},
		},
		{
			name:     "malformed JSON is honest no-op",
			manifest: `{ this is not valid json`,
			want:     nil,
		},
		{
			name:     "blank entries are dropped, rest kept",
			manifest: `{"gate_stage_order": ["P-G0", "", "P-G1"]}` + "\n",
			want:     []string{"P-G0", "P-G1"},
		},
		{
			name:     "all-blank list collapses to nil",
			manifest: `{"gate_stage_order": ["", ""]}` + "\n",
			want:     nil,
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
			got := ResolveGateStageOrder(graphPath)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ResolveGateStageOrder = %#v, want %#v", got, tc.want)
			}
		})
	}
}
