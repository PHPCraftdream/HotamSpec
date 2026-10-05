package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProfileFixture writes a minimal graph.json plus the given manifest.json
// content into a temp domain directory and returns the graph.json path — the
// same fixture shape the Resolve* tests use, so the LoadGraph/probe paths can
// be exercised against a real on-disk domain.
func writeProfileFixture(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(graphPath, []byte(`{"schema_version": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return graphPath
}

// TestLoadManifest_ProfileAtomsExpansion proves the atoms profile materializes
// all four bundled flag defaults into the typed manifest when none of them is
// explicitly declared.
func TestLoadManifest_ProfileAtomsExpansion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(`{"profile": "atoms"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Profile != ProfileAtoms {
		t.Errorf("Profile = %q, want %q", m.Profile, ProfileAtoms)
	}
	if m.Discipline != DisciplineFull {
		t.Errorf("Discipline = %q, want %q", m.Discipline, DisciplineFull)
	}
	if m.RequirementsAuthority != RequirementsAuthorityCode {
		t.Errorf("RequirementsAuthority = %q, want %q", m.RequirementsAuthority, RequirementsAuthorityCode)
	}
	if m.GenProfile != GenProfileConsumer {
		t.Errorf("GenProfile = %q, want %q", m.GenProfile, GenProfileConsumer)
	}
	if !m.SelfExecutingAtoms {
		t.Errorf("SelfExecutingAtoms = false, want true")
	}
}

// TestLoadManifest_ProfileExplicitFlagWins proves an explicitly-set flag
// overrides the profile's bundled default — per-flag, leaving every other
// profile default intact.
func TestLoadManifest_ProfileExplicitFlagWins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(`{"profile": "atoms", "gen_profile": "full"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.GenProfile != GenProfileFull {
		t.Errorf("GenProfile = %q, want explicit %q to win", m.GenProfile, GenProfileFull)
	}
	if m.Discipline != DisciplineFull || m.RequirementsAuthority != RequirementsAuthorityCode || !m.SelfExecutingAtoms {
		t.Errorf("non-overridden profile defaults must still apply: %+v", m)
	}
}

// TestLoadManifest_UnknownProfileIsError proves an unrecognized profile name
// fails the load, with the known names in the message.
func TestLoadManifest_UnknownProfileIsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(`{"profile": "atomz"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("LoadManifest with unknown profile: want error, got nil")
	}
	for _, name := range []string{"atomz", ProfileAtoms} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q must mention %q", err.Error(), name)
		}
	}
}

// TestLoadManifest_NoProfileFieldUnchanged proves a manifest without the
// "profile" key loads with exactly the pre-profile resolved values (all
// zero-value defaults) — the compatibility contract for every existing
// manifest.
func TestLoadManifest_NoProfileFieldUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(`{"self_hosting": false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Profile != "" || m.Discipline != "" || m.RequirementsAuthority != "" || m.GenProfile != "" || m.SelfExecutingAtoms {
		t.Errorf("profile-less manifest must load unchanged, got %+v", m)
	}
}

// TestProfileExpansion_VisibleToResolveProbes proves the expansion reaches the
// direct manifest.json probes the runtime actually reads through —
// ResolveDiscipline, resolveRequirementsAuthorityCode, and ResolveGenProfile —
// not just the typed DomainManifest object.
func TestProfileExpansion_VisibleToResolveProbes(t *testing.T) {
	t.Parallel()
	graphPath := writeProfileFixture(t, `{"profile": "atoms"}`+"\n")
	if got := ResolveDiscipline(graphPath); got != DisciplineFull {
		t.Errorf("ResolveDiscipline = %q, want %q", got, DisciplineFull)
	}
	if got := ResolveGenProfile(graphPath); got != GenProfileConsumer {
		t.Errorf("ResolveGenProfile = %q, want %q", got, GenProfileConsumer)
	}
	if !resolveRequirementsAuthorityCode(graphPath) {
		t.Errorf("resolveRequirementsAuthorityCode = false, want true")
	}
}

// TestProfileExpansion_ExplicitFlagWinsInProbes mirrors the typed-path
// explicit-override test on the probe path.
func TestProfileExpansion_ExplicitFlagWinsInProbes(t *testing.T) {
	t.Parallel()
	graphPath := writeProfileFixture(t, `{"profile": "atoms", "gen_profile": "full"}`+"\n")
	if got := ResolveGenProfile(graphPath); got != GenProfileFull {
		t.Errorf("ResolveGenProfile = %q, want explicit %q to win", got, GenProfileFull)
	}
	if got := ResolveDiscipline(graphPath); got != DisciplineFull {
		t.Errorf("ResolveDiscipline = %q, want %q (untouched default)", got, DisciplineFull)
	}
}

// TestProfileExpansion_UnknownProfileProbesDegrade proves the tolerant probe
// contract survives an unknown profile name on the probe path (the hard load
// error is LoadManifest's job, tested above); probes stay honest no-ops.
func TestProfileExpansion_UnknownProfileProbesDegrade(t *testing.T) {
	t.Parallel()
	graphPath := writeProfileFixture(t, `{"profile": "atomz"}`+"\n")
	if got := ResolveDiscipline(graphPath); got != "" {
		t.Errorf("ResolveDiscipline = %q, want \"\"", got)
	}
	if got := ResolveGenProfile(graphPath); got != GenProfileFull {
		t.Errorf("ResolveGenProfile = %q, want %q", got, GenProfileFull)
	}
	if resolveRequirementsAuthorityCode(graphPath) {
		t.Errorf("resolveRequirementsAuthorityCode = true, want false")
	}
}

// TestProfileJSONDefault_ExplicitFlagBlocksDefault pins the shared helper's
// explicit-wins rule directly, including the unknown-profile and
// no-profile soft no-ops.
func TestProfileJSONDefault_ExplicitFlagBlocksDefault(t *testing.T) {
	t.Parallel()
	explicit := []byte(`{"profile": "atoms", "self_executing_atoms": false}`)
	if _, ok := profileJSONDefault(explicit, "self_executing_atoms"); ok {
		t.Error("explicitly-set flag must block the profile default")
	}
	if d, ok := profileJSONDefault([]byte(`{"profile": "atoms"}`), "self_executing_atoms"); !ok || d != "true" {
		t.Errorf("profileJSONDefault(atoms, self_executing_atoms) = %q, %v; want \"true\", true", d, ok)
	}
	if _, ok := profileJSONDefault([]byte(`{}`), "discipline"); ok {
		t.Error("no profile must be a soft no-op")
	}
	if _, ok := profileJSONDefault([]byte(`{"profile": "atomz"}`), "discipline"); ok {
		t.Error("unknown profile must be a soft no-op on the probe path")
	}
	if _, ok := profileJSONDefault([]byte(`not json`), "discipline"); ok {
		t.Error("malformed JSON must be a soft no-op")
	}
}

// TestProfileAtoms_RoundTripMarshal pins that the profile field carries
// omitempty (a profile-less manifest marshals without the key — the byte-
// identity requirement for every existing manifest) and that a profile-bearing
// manifest round-trips the field.
func TestProfileAtoms_RoundTripMarshal(t *testing.T) {
	t.Parallel()
	noProfile, err := json.Marshal(&DomainManifest{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(noProfile), "profile") {
		t.Errorf("empty manifest marshaled %s; must omit the profile key", noProfile)
	}
	withProfile, err := json.Marshal(&DomainManifest{Profile: ProfileAtoms})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(withProfile), `"profile":"atoms"`) {
		t.Errorf("marshaled %s; must carry the profile key", withProfile)
	}
}
