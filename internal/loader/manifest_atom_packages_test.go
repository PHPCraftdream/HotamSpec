package loader

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAtomPackagesManifest(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// LoadManifest must enforce the root-module atom opt-in contract (§14):
// clean relative paths, no duplicates, explicit recorder import path.
func TestLoadManifestSelfExecutingAtomPackages(t *testing.T) {
	valid := writeAtomPackagesManifest(t, `{"parent":null,`+
		`"self_executing_atom_packages":["internal/localization"],`+
		`"atom_recorder_import_path":"github.com/PHPCraftdream/HotamSpec/internal/recorder/canon"}`)
	m, err := LoadManifest(valid)
	if err != nil {
		t.Fatalf("valid atom-packages manifest rejected: %v", err)
	}
	if len(m.SelfExecutingAtomPackages) != 1 || m.SelfExecutingAtomPackages[0] != "internal/localization" {
		t.Fatalf("SelfExecutingAtomPackages = %v, want [internal/localization]", m.SelfExecutingAtomPackages)
	}
	if m.AtomRecorderImportPath == "" {
		t.Fatal("AtomRecorderImportPath lost on load")
	}

	cases := []struct{ name, body string }{
		{"missing recorder import path", `{"self_executing_atom_packages":["internal/localization"]}`},
		{"duplicate package", `{"self_executing_atom_packages":["internal/a","internal/a"],"atom_recorder_import_path":"x/y"}`},
		{"parent traversal", `{"self_executing_atom_packages":["../outside"],"atom_recorder_import_path":"x/y"}`},
		{"absolute path", `{"self_executing_atom_packages":["C:/dev/internal/a"],"atom_recorder_import_path":"x/y"}`},
		{"unclean path", `{"self_executing_atom_packages":["internal/./a"],"atom_recorder_import_path":"x/y"}`},
		{"empty entry", `{"self_executing_atom_packages":[""],"atom_recorder_import_path":"x/y"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadManifest(writeAtomPackagesManifest(t, tc.body)); err == nil {
				t.Fatalf("%s: manifest accepted, want rejection", tc.name)
			}
		})
	}
}

func TestValidateSelfExecutingAtomPackagesConsumerUnchanged(t *testing.T) {
	// Absent keys: honest no-op, no recorder import path demanded.
	if err := validateSelfExecutingAtomPackages(nil, ""); err != nil {
		t.Fatalf("absent atom packages must be a no-op: %v", err)
	}
	if _, err := LoadManifest(writeAtomPackagesManifest(t, `{"parent":null}`)); err != nil {
		t.Fatalf("plain consumer manifest rejected: %v", err)
	}
}
