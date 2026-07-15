package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	written, err := initProject(dir, "main", "2026-07-13")
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

	// Base domain graph + docs/gen populated by initDomain + genSpec.
	graphJSON := filepath.Join(dir, "domains", "main", "graph.json")
	if _, err := os.Stat(graphJSON); err != nil {
		t.Errorf("base domain graph.json not created: %v", err)
	}
	reqMD := filepath.Join(dir, "domains", "main", "docs", "gen", "REQUIREMENTS.md")
	if _, err := os.Stat(reqMD); err != nil {
		t.Errorf("docs/gen/REQUIREMENTS.md not created: %v", err)
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

	_, err := initProject(dir, "main", "2026-07-13")
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

	_, err := initProject(dir, "main", "2026-07-13")
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
// default to the SAME gen-spec profile (consumer) in their scaffolded
// manifests. Before the fix, initDomain wrote no gen_profile field at all
// (silently resolving to "full" via ResolveGenProfile's absent-field
// fallback), while initProject wrote "consumer" — two different defaults for
// two onboarding paths with no flag or message explaining the divergence.
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
	if _, err := initProject(projDir, "main", "2026-07-14"); err != nil {
		t.Fatalf("initProject: %v", err)
	}
	projProfile := readManifestGenProfile(t, filepath.Join(projDir, "domains", "main", "manifest.json"))

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
