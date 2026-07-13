package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	// Project-root marker (R4 existence-only resolution → empty file is correct).
	marker := filepath.Join(dir, ".hotam-spec-project")
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("project-root marker not created: %v", err)
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
