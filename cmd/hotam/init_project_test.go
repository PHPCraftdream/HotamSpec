package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// TestInitProject_ScaffoldFromCleanDir exercises initProject (the testable
// helper under cmdInitProject) end-to-end against a clean temp dir: it must
// scaffold the base domain, write the project-root marker, and render the root
// crystal + docs/gen via the existing initDomain + genSpec primitives — and
// report every written path in order.
func TestInitProject_ScaffoldFromCleanDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	written, err := initProject(dir, "main", "2026-07-13", false, true, "owner")
	if err != nil {
		t.Fatalf("initProject: %v", err)
	}

	// Project-root marker exists and records the active domain (R4 resolution
	// is existence-only, so the JSON payload is additive and never breaks it).
	marker := filepath.Join(dir, ".hotam-spec-project")
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("project-root marker not created: %v", err)
	}
	if name, ok := paths.ReadActiveDomain(marker); !ok || name != "main" {
		t.Errorf("initProject should record active_domain=main in the marker, got name=%q ok=%v", name, ok)
	}

	// Root crystal: CLAUDE.md + AGENTS.md + GEMINI.md at the project root.
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not created at project root: %v", name, err)
		}
	}

	// Base domain graph is created by initDomain.
	graphJSON := filepath.Join(dir, "domains", "main", "graph.json")
	if _, err := os.Stat(graphJSON); err != nil {
		t.Errorf("base domain graph.json not created: %v", err)
	}
	// Task #364: initDomain no longer auto-seeds a Stakeholder + Requirement
	// — the base domain is genuinely empty, so genSpec (called internally by
	// initProject) withholds REQUIREMENTS.md (and every other content-gated
	// docs/gen/ projection) entirely, rather than rendering it with a calm
	// "no content yet" placeholder.
	reqMD := filepath.Join(dir, "domains", "main", "docs", "gen", "REQUIREMENTS.md")
	if _, err := os.Stat(reqMD); !os.IsNotExist(err) {
		t.Errorf("docs/gen/REQUIREMENTS.md must NOT be created for a genuinely empty base domain, stat err=%v", err)
	}

	// The written list must be non-empty and surface both the marker and the
	// root crystal path (so callers can report exactly what landed on disk).
	if len(written) == 0 {
		t.Fatal("initProject returned an empty written list")
	}
	foundMarker, foundClaude := false, false
	for _, p := range written {
		s := filepath.ToSlash(p)
		if strings.HasSuffix(s, "/.hotam-spec-project") {
			foundMarker = true
		}
		if strings.HasSuffix(s, "/CLAUDE.md") {
			foundClaude = true
		}
	}
	if !foundMarker {
		t.Error("written list omits the project-root marker path")
	}
	if !foundClaude {
		t.Error("written list omits the root CLAUDE.md path")
	}
}

// TestInitProject_RefusesWhenMarkerExists mirrors initDomain's overwrite
// discipline: a project already bootstrapped (marker present) must never be
// silently re-scaffolded.
func TestInitProject_RefusesWhenMarkerExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	marker := filepath.Join(dir, ".hotam-spec-project")
	if err := os.WriteFile(marker, []byte{}, 0o644); err != nil {
		t.Fatalf("seed marker: %v", err)
	}

	_, err := initProject(dir, "main", "2026-07-13", false, true, "owner")
	if err == nil {
		t.Fatal("expected a refusal error when the marker exists, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("refusal error should mention 'already exists', got: %v", err)
	}
}

// TestInitProject_RefusesWhenClaudeMDExists guards the second overwrite point:
// a project root already holding a crystal must not have it silently overwritten
// by gen-spec. The marker is absent here so this specifically exercises the
// CLAUDE.md guard (the marker check runs first and must pass through).
func TestInitProject_RefusesWhenClaudeMDExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte("# pre-existing crystal"), 0o644); err != nil {
		t.Fatalf("seed CLAUDE.md: %v", err)
	}

	_, err := initProject(dir, "main", "2026-07-13", false, true, "owner")
	if err == nil {
		t.Fatal("expected a refusal error when CLAUDE.md exists, got nil")
	}
	if !strings.Contains(err.Error(), "CLAUDE.md") || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("refusal error should name CLAUDE.md already exists, got: %v", err)
	}
}

// readManifestGenProfile reads a domain's manifest.json and returns its
// gen_profile value (empty string when the field is absent — matching
// loader.ResolveGenProfile's absent-field semantics for test readability).
func readManifestGenProfile(t *testing.T, manifestPath string) string {
	t.Helper()
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest %s: %v", manifestPath, err)
	}
	var m struct {
		GenProfile string `json:"gen_profile"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal manifest %s: %v", manifestPath, err)
	}
	return m.GenProfile
}

// TestInitAndInitProject_DefaultToSameGenProfile proves the R8-e fix: bare
// `hotam init` (initDomain) and `hotam init-project` (initProject) both
// default to the SAME gen-spec profile (consumer). initDomain writes the
// gen_profile field directly; initProject now names the "atoms" profile and
// the consumer gen profile comes from its expansion at load — asserted here
// through internal/loader.LoadManifest, not raw JSON.
func TestInitAndInitProject_DefaultToSameGenProfile(t *testing.T) {
	t.Parallel()

	// (1) initDomain (bare `hotam init`).
	initDir := t.TempDir()
	if _, err := initDomain(initDir, "bare", "2026-07-13"); err != nil {
		t.Fatalf("initDomain: %v", err)
	}
	bareProfile := readManifestGenProfile(t, filepath.Join(initDir, "manifest.json"))

	// (2) initProject (`hotam init-project`).
	projDir := t.TempDir()
	if _, err := initProject(projDir, "main", "2026-07-14", false, true, "owner"); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	projManifestPath := filepath.Join(projDir, "domains", "main", "manifest.json")
	projLoaded, err := loader.LoadManifest(projManifestPath)
	if err != nil {
		t.Fatalf("LoadManifest %s: %v", projManifestPath, err)
	}
	projProfile := projLoaded.GenProfile

	// (3) Both must be "consumer" and equal.
	if bareProfile != "consumer" {
		t.Errorf("initDomain manifest gen_profile = %q, want \"consumer\"", bareProfile)
	}
	if projProfile != "consumer" {
		t.Errorf("initProject manifest gen_profile = %q, want \"consumer\"", projProfile)
	}
	if bareProfile != projProfile {
		t.Errorf("init and init-project default to different profiles: init=%q init-project=%q", bareProfile, projProfile)
	}
}

// TestInitProject_ManifestAtomsProfile pins the P1-3 init-project contract:
// the default scaffold writes "profile": "atoms" (not the scattered
// discipline/requirements_authority/self_executing_atoms/gen_profile flags),
// and loading the manifest through internal/loader expands it to the
// born-obligated flag set. Composing --require-provenance must not disturb
// the profile.
func TestInitProject_ManifestAtomsProfile(t *testing.T) {
	t.Parallel()

	projDir := t.TempDir()
	if _, err := initProject(projDir, "main", "2026-07-14", false, true, "owner"); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	manifestPath := filepath.Join(projDir, "domains", "main", "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var raw struct {
		Profile               string `json:"profile"`
		Discipline            string `json:"discipline"`
		RequirementsAuthority string `json:"requirements_authority"`
		SelfExecutingAtoms    bool   `json:"self_executing_atoms"`
		GenProfile            string `json:"gen_profile"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if raw.Profile != loader.ProfileAtoms {
		t.Errorf("manifest profile = %q, want \"atoms\"; manifest:\n%s", raw.Profile, data)
	}
	for name, got := range map[string]string{
		"discipline":             raw.Discipline,
		"requirements_authority": raw.RequirementsAuthority,
		"gen_profile":            raw.GenProfile,
	} {
		if got != "" {
			t.Errorf("manifest must not carry scattered flag %s (got %q); the atoms profile supplies it", name, got)
		}
	}
	if raw.SelfExecutingAtoms {
		t.Errorf("manifest must not carry scattered self_executing_atoms flag; the atoms profile supplies it")
	}

	// Load-level: profile expands to the full atoms flag set.
	loaded, err := loader.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if loaded.Discipline != loader.DisciplineFull || loaded.RequirementsAuthority != loader.RequirementsAuthorityCode || !loaded.SelfExecutingAtoms || loaded.GenProfile != loader.GenProfileConsumer {
		t.Errorf("atoms profile expansion mismatch: discipline=%q authority=%q selfExecutingAtoms=%v genProfile=%q", loaded.Discipline, loaded.RequirementsAuthority, loaded.SelfExecutingAtoms, loaded.GenProfile)
	}

	// --discipline "" escape hatch stays profile-free.
	softDir := t.TempDir()
	if _, err := initProject(softDir, "soft", "2026-07-14", false, false, "owner"); err != nil {
		t.Fatalf("initProject --discipline empty: %v", err)
	}
	softData, err := os.ReadFile(filepath.Join(softDir, "domains", "soft", "manifest.json"))
	if err != nil {
		t.Fatalf("read soft manifest: %v", err)
	}
	if strings.Contains(string(softData), "\"profile\"") {
		t.Errorf("--discipline \"\" manifest must not carry a profile field:\n%s", softData)
	}
}

// TestCmdInit_ProfileFlagOverridesDefault proves the --profile flag on
// `hotam init` works in both directions: --profile full overrides
// initDomain's consumer default, and the bare default (no flag) writes
// consumer. This ensures an operator who needs the heavier
// framework-self-hosting doc set can still get it from bare `hotam init`.
//
// cmdInit receives args AFTER main.go's reorderFlagsFirst has moved flags
// ahead of positional arguments (Go's stdlib flag stops at the first non-flag
// token), so these in-process calls pass flags first to mirror that.
func TestCmdInit_ProfileFlagOverridesDefault(t *testing.T) {
	t.Parallel()

	// --profile full overrides the consumer default.
	dirFull := t.TempDir()
	domainFull := filepath.Join(dirFull, "mydomain")
	if err := cmdInit([]string{"--name", "mydomain", "--profile", "full", domainFull}); err != nil {
		t.Fatalf("cmdInit --profile full: %v", err)
	}
	if got := readManifestGenProfile(t, filepath.Join(domainFull, "manifest.json")); got != "full" {
		t.Errorf("cmdInit --profile full: manifest gen_profile = %q, want \"full\"", got)
	}

	// No flag → consumer default (initDomain's own default).
	dirDefault := t.TempDir()
	domainDefault := filepath.Join(dirDefault, "mydomain")
	if err := cmdInit([]string{"--name", "mydomain", domainDefault}); err != nil {
		t.Fatalf("cmdInit (default): %v", err)
	}
	if got := readManifestGenProfile(t, filepath.Join(domainDefault, "manifest.json")); got != "consumer" {
		t.Errorf("cmdInit (default): manifest gen_profile = %q, want \"consumer\"", got)
	}

	// --profile consumer is an explicit no-op (same as default).
	dirConsumer := t.TempDir()
	domainConsumer := filepath.Join(dirConsumer, "mydomain")
	if err := cmdInit([]string{"--name", "mydomain", "--profile", "consumer", domainConsumer}); err != nil {
		t.Fatalf("cmdInit --profile consumer: %v", err)
	}
	if got := readManifestGenProfile(t, filepath.Join(domainConsumer, "manifest.json")); got != "consumer" {
		t.Errorf("cmdInit --profile consumer: manifest gen_profile = %q, want \"consumer\"", got)
	}

	// Invalid value is rejected.
	dirBad := t.TempDir()
	if err := cmdInit([]string{"--profile", "bogus", filepath.Join(dirBad, "x")}); err == nil {
		t.Fatal("cmdInit --profile bogus should return an error, got nil")
	}
}

// TestCmdInitProject_RequireProvenanceFlag proves R12-b's init-project half:
// `hotam init-project <dir> --require-provenance` writes
// "require_provenance": true into the SCAFFOLDED BASE DOMAIN's manifest.json
// (domains/<name>/manifest.json), not just some project-root file — matching
// where internal/loader.ResolveRequireProvenance actually looks (next to
// graph.json).
func TestCmdInitProject_RequireProvenanceFlag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := cmdInitProject([]string{"--require-provenance", dir}); err != nil {
		t.Fatalf("cmdInitProject --require-provenance: %v", err)
	}

	manifestPath := filepath.Join(dir, "domains", defaultInitProjectDomain, "manifest.json")
	got, present := readManifestRequireProvenance(t, manifestPath)
	if !present {
		t.Fatalf("base domain manifest.json has no require_provenance field after --require-provenance")
	}
	if !got {
		t.Errorf("base domain manifest.json require_provenance = false, want true")
	}

	// gen_profile must resolve to consumer (via the atoms profile), proving
	// the require_provenance write did not disturb the profile.
	loaded, err := loader.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest %s: %v", manifestPath, err)
	}
	if loaded.GenProfile != "consumer" {
		t.Errorf("base domain manifest gen_profile = %q after --require-provenance, want \"consumer\" (must not be clobbered)", loaded.GenProfile)
	}
}

// TestCmdInitProject_RequireProvenanceDefaultOff confirms omitting
// --require-provenance on init-project leaves the base domain's manifest
// exactly as before this task: no require_provenance field — zero behavior
// change for the existing default onboarding path.
func TestCmdInitProject_RequireProvenanceDefaultOff(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := cmdInitProject([]string{dir}); err != nil {
		t.Fatalf("cmdInitProject (default): %v", err)
	}

	manifestPath := filepath.Join(dir, "domains", defaultInitProjectDomain, "manifest.json")
	if _, present := readManifestRequireProvenance(t, manifestPath); present {
		t.Errorf("default cmdInitProject (no --require-provenance) wrote a require_provenance field; want it absent")
	}
}

// TestCmdInitProject_DisciplineDefaultFull proves the --discipline flag's
// default (no flag passed) reproduces the pre-flag behavior: the "atoms"
// profile in the manifest (expanding, at load, to discipline:"full" +
// requirements_authority:"code"), spec/go.mod +
// spec/hotamspec/hotamspec.go vendored, and docs/gen/SPEC.md rendered — the
// BORN FULLY OBLIGATED contract (task #273/W6.2) must not regress just
// because the flag now exists.
func TestCmdInitProject_DisciplineDefaultFull(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the full command pipeline against a scaffolded domain; skipped in -short")
	}

	t.Parallel()

	dir := t.TempDir()
	if err := cmdInitProject([]string{dir}); err != nil {
		t.Fatalf("cmdInitProject (default): %v", err)
	}

	domainDir := filepath.Join(dir, "domains", defaultInitProjectDomain)
	manifestPath := filepath.Join(domainDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	if !strings.Contains(string(manifestData), `"profile": "atoms"`) {
		t.Errorf("default cmdInitProject manifest.json missing \"profile\": \"atoms\", got:\n%s", manifestData)
	}
	loaded, err := loader.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if loaded.Discipline != loader.DisciplineFull || loaded.RequirementsAuthority != loader.RequirementsAuthorityCode {
		t.Errorf("atoms profile must resolve to discipline:full + requirements_authority:code, got discipline=%q authority=%q", loaded.Discipline, loaded.RequirementsAuthority)
	}
	for _, rel := range []string{
		filepath.Join("spec", "go.mod"),
		filepath.Join("spec", "hotamspec", "hotamspec.go"),
	} {
		if _, err := os.Stat(filepath.Join(domainDir, rel)); err != nil {
			t.Errorf("default cmdInitProject should scaffold %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(domainDir, "docs", "gen", "SPEC.md")); err != nil {
		t.Errorf("default cmdInitProject should render docs/gen/SPEC.md: %v", err)
	}

	// Requirements-in-code from birth: authority resolved via the atoms
	// profile (asserted above through LoadManifest), ontology mirror +
	// registrydump vendored, empty registry written, and the whole spec/
	// module compiles.
	for _, rel := range []string{
		filepath.Join("spec", "hotamontology", "requirement.go"),
		filepath.Join("spec", "hotamontology", "registry.go"),
		filepath.Join("spec", "registrydump", "main.go"),
		filepath.Join("spec", "requirements.go"),
	} {
		if _, err := os.Stat(filepath.Join(domainDir, rel)); err != nil {
			t.Errorf("default cmdInitProject should scaffold %s: %v", rel, err)
		}
	}
	reqSrc, err := os.ReadFile(filepath.Join(domainDir, "spec", "requirements.go"))
	if err != nil {
		t.Fatalf("read spec/requirements.go: %v", err)
	}
	if !strings.Contains(string(reqSrc), "var Requirements = hotamontology.New[hotamontology.Requirement]()") {
		t.Errorf("spec/requirements.go must declare an empty Requirements registry, got:\n%s", reqSrc)
	}
	build := exec.Command("go", "build", "./...")
	build.Dir = filepath.Join(domainDir, "spec")
	if out, err := build.CombinedOutput(); err != nil {
		t.Errorf("scaffolded spec/ module does not compile: %v\n%s", err, out)
	}
}

// TestCmdInitProject_DisciplineOffOptOut proves `--discipline ""` (the escape
// hatch added after dogfooding a business domain that wanted zero framework
// code before any real content existed): the scaffolded manifest carries no
// "discipline" key, no spec/ tree is created at all, no SPEC.md is rendered,
// and the resulting domain is still `all-violations`-clean — matching a bare
// `hotam init` domain exactly.
func TestCmdInitProject_DisciplineOffOptOut(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := cmdInitProject([]string{"--discipline", "", dir}); err != nil {
		t.Fatalf("cmdInitProject --discipline \"\": %v", err)
	}

	domainDir := filepath.Join(dir, "domains", defaultInitProjectDomain)
	manifestData, err := os.ReadFile(filepath.Join(domainDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	if strings.Contains(string(manifestData), "discipline") {
		t.Errorf("--discipline \"\" manifest.json should carry no discipline key, got:\n%s", manifestData)
	}
	if _, err := os.Stat(filepath.Join(domainDir, "spec")); !os.IsNotExist(err) {
		t.Errorf("--discipline \"\" should not create domains/%s/spec/ at all, stat err = %v", defaultInitProjectDomain, err)
	}
	if _, err := os.Stat(filepath.Join(domainDir, "docs", "gen", "SPEC.md")); !os.IsNotExist(err) {
		t.Errorf("--discipline \"\" should not render docs/gen/SPEC.md, stat err = %v", err)
	}

	if violations, err := allViolations(domainDir); err != nil {
		t.Errorf("all-violations against --discipline \"\" domain failed: %v", err)
	} else if len(violations) != 0 {
		t.Errorf("--discipline \"\" domain is not all-violations clean: %+v", violations)
	}
}

// TestCmdInitProject_DisciplineFlagRejectsBogusValue proves the flag validates
// its value the same way --profile does on cmdInit — an unrecognized string is
// a usage error, not silently treated as either "full" or "".
func TestCmdInitProject_DisciplineFlagRejectsBogusValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := cmdInitProject([]string{"--discipline", "bogus", dir}); err == nil {
		t.Fatal("cmdInitProject --discipline bogus should return an error, got nil")
	}
}

// TestInitProject_SyncReadySeedsOwnerAndEnvelope pins the sync-ready
// onboarding contract: a default init-project scaffold must (a) write
// spec/stakeholders.go declaring the seed owner registry, (b) write the
// manifest atom_defaults (owner/status SETTLED/created_at=settled_at=today)
// that check_no_dangling_requirement_owner reads, and (c) make the FIRST
// scaffoldRegistrydump pick the envelope template (because stakeholders.go
// is written before scaffoldRegistrydump runs), so spec/registrydump/main.go
// already marshals Stakeholders.All() — no manual re-run needed.
func TestInitProject_SyncReadySeedsOwnerAndEnvelope(t *testing.T) {
	if testing.Short() {
		t.Skip("scaffolds a full domain and renders the crystal via genSpec; skipped in -short")
	}

	t.Parallel()

	projDir := t.TempDir()
	written, err := initProject(projDir, "main", "2026-07-14", false, true, "owner")
	if err != nil {
		t.Fatalf("initProject: %v", err)
	}

	domainDir := filepath.Join(projDir, "domains", "main")

	// (a) spec/stakeholders.go is in the written list and declares the
	// Stakeholders registry with the owner registered.
	foundStakeholders := false
	for _, p := range written {
		if strings.HasSuffix(filepath.ToSlash(p), "/spec/stakeholders.go") {
			foundStakeholders = true
		}
	}
	if !foundStakeholders {
		t.Error("written list omits spec/stakeholders.go")
	}
	stkSrc, err := os.ReadFile(filepath.Join(domainDir, "spec", "stakeholders.go"))
	if err != nil {
		t.Fatalf("read spec/stakeholders.go: %v", err)
	}
	stkBody := string(stkSrc)
	for _, want := range []string{
		"package spec",
		"var Stakeholders = hotamontology.New[hotamontology.Stakeholder]()",
		`Stakeholders.MustRegister("owner"`,
	} {
		if !strings.Contains(stkBody, want) {
			t.Errorf("spec/stakeholders.go missing %q, got:\n%s", want, stkBody)
		}
	}

	// (c) the registrydump scaffolded in the SAME run already prints the
	// envelope (Stakeholders.All()) — the decisive ordering assertion.
	dumpSrc, err := os.ReadFile(filepath.Join(domainDir, "spec", "registrydump", "main.go"))
	if err != nil {
		t.Fatalf("read spec/registrydump/main.go: %v", err)
	}
	if !strings.Contains(string(dumpSrc), "Stakeholders.All()") {
		t.Errorf("first-scaffolded registrydump main.go must marshal Stakeholders.All() (envelope), got:\n%s", dumpSrc)
	}

	// (b) manifest atom_defaults present with owner/status/dates, and loads
	// through loader.LoadManifest with the expected AtomDefaults.
	manifestPath := filepath.Join(domainDir, "manifest.json")
	loaded, err := loader.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if loaded.AtomDefaults == nil {
		t.Fatalf("manifest has no atom_defaults after initProject; manifest:\n%s", mustReadFile(t, manifestPath))
	}
	if loaded.AtomDefaults.Owner != "owner" || loaded.AtomDefaults.Status != "SETTLED" {
		t.Errorf("atom_defaults owner/status = %q/%q, want \"owner\"/\"SETTLED\"", loaded.AtomDefaults.Owner, loaded.AtomDefaults.Status)
	}
	if loaded.AtomDefaults.CreatedAt != "2026-07-14" || loaded.AtomDefaults.SettledAt != "2026-07-14" {
		t.Errorf("atom_defaults created_at/settled_at = %q/%q, want both \"2026-07-14\"", loaded.AtomDefaults.CreatedAt, loaded.AtomDefaults.SettledAt)
	}

	// The scaffolded spec/ module still compiles as a whole (stakeholders.go
	// is part of the spec package).
	build := exec.Command("go", "build", "./...")
	build.Dir = filepath.Join(domainDir, "spec")
	if out, err := build.CombinedOutput(); err != nil {
		t.Errorf("scaffolded spec/ module does not compile: %v\n%s", err, out)
	}
}

// TestInitProject_CustomOwnerPropagates proves the --owner flag feeds BOTH
// seeded artifacts: spec/stakeholders.go registers the stakeholder under the
// given id and the manifest atom_defaults names the same owner — a custom id
// like "alice" must never desynchronize the two.
func TestInitProject_CustomOwnerPropagates(t *testing.T) {
	t.Parallel()

	projDir := t.TempDir()
	if _, err := initProject(projDir, "main", "2026-07-14", false, true, "alice"); err != nil {
		t.Fatalf("initProject --owner alice: %v", err)
	}
	domainDir := filepath.Join(projDir, "domains", "main")

	stkSrc, err := os.ReadFile(filepath.Join(domainDir, "spec", "stakeholders.go"))
	if err != nil {
		t.Fatalf("read spec/stakeholders.go: %v", err)
	}
	if !strings.Contains(string(stkSrc), `Stakeholders.MustRegister("alice"`) {
		t.Errorf("spec/stakeholders.go must register the custom owner id \"alice\", got:\n%s", stkSrc)
	}

	loaded, err := loader.LoadManifest(filepath.Join(domainDir, "manifest.json"))
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if loaded.AtomDefaults == nil || loaded.AtomDefaults.Owner != "alice" {
		t.Errorf("atom_defaults owner = %+v, want owner \"alice\"", loaded.AtomDefaults)
	}
}

// mustReadFile is a small helper for failure-message contexts.
func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
