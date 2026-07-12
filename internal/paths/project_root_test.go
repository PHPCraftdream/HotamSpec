package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestHasMarkerReliable(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "domains"), 0755); err != nil {
		t.Fatal(err)
	}
	if !hasMarker(dir) {
		t.Fatal("a single RELIABLE marker (domains/) should match")
	}
}

func TestHasMarkerSecondaryAloneFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", "# project")
	if hasMarker(dir) {
		t.Fatal("a lone SECONDARY marker must NOT match")
	}
}

func TestHasMarkerSecondaryPair(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", "# project")
	if err := os.Mkdir(filepath.Join(dir, ".claude"), 0755); err != nil {
		t.Fatal(err)
	}
	if !hasMarker(dir) {
		t.Fatal("two SECONDARY markers should match")
	}
}

func TestHasMarkerPyprojectTable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[tool.hotam-spec]\nproject_root = \".\"\n")
	if !hasMarker(dir) {
		t.Fatal("pyproject.toml with [tool.hotam-spec] should match as RELIABLE")
	}
}

func TestHasMarkerForeignPyprojectFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[project]\nname = \"other\"\n")
	if hasMarker(dir) {
		t.Fatal("a generic pyproject.toml must NOT match")
	}
}

func TestResolvePyprojectProjectRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "subproject")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "pyproject.toml", "[tool.hotam-spec]\nproject_root = \"subproject\"\n")
	got, ok := resolvePyproject(root, 1)
	if !ok {
		t.Fatal("resolvePyproject should find project_root")
	}
	if got != target {
		t.Fatalf("resolved = %q, want %q", got, target)
	}
}

func TestProjectRootSelfHostingFallback(t *testing.T) {
	root, ok := ProjectRoot()
	if !ok {
		t.Skip("no project root resolvable in this environment")
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolved root %q has no go.mod: %v", root, err)
	}
}
