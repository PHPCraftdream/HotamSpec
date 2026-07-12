package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func chdirAndRestore(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})
}

func makeDir(t *testing.T, path ...string) string {
	t.Helper()
	full := filepath.Join(path...)
	if err := os.MkdirAll(full, 0755); err != nil {
		t.Fatal(err)
	}
	return full
}

func TestProjectRoot_R1_EnvProjectRootSet(t *testing.T) {
	target := makeDir(t, t.TempDir(), "my-project")
	t.Setenv(EnvProjectRoot, target)
	chdirAndRestore(t, t.TempDir())
	got, ok := ProjectRoot()
	if !ok {
		t.Fatal("R1 env var pointing at an existing dir must resolve")
	}
	if got != target {
		t.Fatalf("R1 resolved = %q, want %q", got, target)
	}
}

func TestProjectRoot_R1_NonexistentSkipped(t *testing.T) {
	t.Setenv(EnvProjectRoot, filepath.Join(t.TempDir(), "does-not-exist"))
	empty := makeDir(t, t.TempDir(), "empty-cwd")
	chdirAndRestore(t, empty)
	got, ok := ProjectRoot()
	if ok {
		t.Fatalf("a non-existent R1 env value must not resolve, got %q", got)
	}
}

func TestProjectRoot_R1_FileNotDirSkipped(t *testing.T) {
	parent := t.TempDir()
	notADir := filepath.Join(parent, "not-a-dir.txt")
	writeFile(t, parent, "not-a-dir.txt", "hello")
	t.Setenv(EnvProjectRoot, notADir)
	chdirAndRestore(t, makeDir(t, parent, "empty-cwd"))
	got, ok := ProjectRoot()
	if ok {
		t.Fatalf("an R1 env value pointing at a file (not a dir) must not resolve, got %q", got)
	}
}

func TestProjectRoot_R2_EnvDomainsRootParent(t *testing.T) {
	project := makeDir(t, t.TempDir(), "consumer")
	domains := makeDir(t, project, "domains")
	t.Setenv(EnvDomainsRoot, domains)
	chdirAndRestore(t, t.TempDir())
	got, ok := ProjectRoot()
	if !ok {
		t.Fatal("R2 env var pointing at an existing domains dir must resolve")
	}
	if got != project {
		t.Fatalf("R2 resolved = %q, want project parent %q", got, project)
	}
}

func TestProjectRoot_R3_MarkerInParentDir(t *testing.T) {
	root := t.TempDir()
	makeDir(t, root, "domains")
	sub := makeDir(t, root, "packages", "myapp")
	chdirAndRestore(t, sub)
	got, ok := ProjectRoot()
	if !ok {
		t.Fatal("a RELIABLE marker in a parent of CWD must resolve via bottom-up search")
	}
	if got != root {
		t.Fatalf("R3 parent-marker resolved = %q, want %q", got, root)
	}
}

func TestProjectRoot_R4_MarkerFileInCWD(t *testing.T) {
	cwd := makeDir(t, t.TempDir(), "a", "b")
	writeFile(t, cwd, MarkerFilename, "")
	chdirAndRestore(t, cwd)
	got, ok := ProjectRoot()
	if !ok {
		t.Fatal("an R4 marker file in CWD must resolve")
	}
	if got != cwd {
		t.Fatalf("R4 resolved = %q, want %q", got, cwd)
	}
}

func TestProjectRoot_R4_MarkerFileInParent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, MarkerFilename, "")
	sub := makeDir(t, root, "sub", "deep")
	chdirAndRestore(t, sub)
	got, ok := ProjectRoot()
	if !ok {
		t.Fatal("an R4 marker file in a parent must resolve via bottom-up search")
	}
	if got != root {
		t.Fatalf("R4 parent-marker resolved = %q, want %q", got, root)
	}
}

func TestProjectRoot_Priority_R1BeatsR3(t *testing.T) {
	cwd := t.TempDir()
	makeDir(t, cwd, "domains")
	r1Target := makeDir(t, t.TempDir(), "explicit-r1")
	t.Setenv(EnvProjectRoot, r1Target)
	chdirAndRestore(t, cwd)
	got, ok := ProjectRoot()
	if !ok {
		t.Fatal("expected resolution with both R1 and R3 active")
	}
	if got != r1Target {
		t.Fatalf("R1 must win over R3: got %q, want %q", got, r1Target)
	}
}

func TestProjectRoot_Priority_R2BeatsR3(t *testing.T) {
	cwd := t.TempDir()
	makeDir(t, cwd, "domains")
	r2Project := makeDir(t, t.TempDir(), "r2-consumer")
	r2Domains := makeDir(t, r2Project, "domains")
	t.Setenv(EnvDomainsRoot, r2Domains)
	chdirAndRestore(t, cwd)
	got, ok := ProjectRoot()
	if !ok {
		t.Fatal("expected resolution with both R2 and R3 active")
	}
	if got != r2Project {
		t.Fatalf("R2 must win over R3: got %q, want %q", got, r2Project)
	}
}

func TestProjectRoot_Priority_NativeMarkerBeatsPyproject(t *testing.T) {
	// Variant-A demotion: a Go-native marker (domains/) at a HIGHER level must
	// win over a legacy pyproject.toml[tool.hotam-spec] marker at a CLOSER level.
	// Before the demotion, R3 treated pyproject as a RELIABLE marker and would
	// resolve to `mid` (the pyproject dir); now R3 (native-only) skips it and
	// resolves to `root` (the domains/ dir).
	root := t.TempDir()
	makeDir(t, root, "domains")
	mid := makeDir(t, root, "mid")
	makeDir(t, mid, "somewhere") // divergent pyproject project_root target
	writeFile(t, mid, "pyproject.toml", "[tool.hotam-spec]\nproject_root = \"somewhere\"\n")
	leaf := makeDir(t, mid, "leaf")
	chdirAndRestore(t, leaf)
	got, ok := ProjectRoot()
	if !ok {
		t.Fatal("expected resolution with both native and legacy pyproject markers present")
	}
	if got != root {
		t.Fatalf("native marker (domains/) must win over legacy pyproject: got %q, want root %q", got, root)
	}
}

func TestProjectRoot_NoMarkersOutsideRepoResolvesFalse(t *testing.T) {
	empty := makeDir(t, t.TempDir(), "no-markers")
	chdirAndRestore(t, empty)
	got, ok := ProjectRoot()
	if ok {
		t.Fatalf("an empty dir outside the framework repo must NOT resolve (R6 must not guess), got %q", got)
	}
}
