package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProjectRootOrRaise_FailureBuildsDiagnostic drives the failure path of
// ProjectRootOrRaise: from an empty CWD outside the repo with NO env vars set,
// ProjectRoot() resolves false, so ProjectRootOrRaise must return a
// *ProjectRootUnresolved whose Error() embeds the full R1–R6 diagnostic.
// This covers ProjectRootOrRaise, buildDiagnostic (no-env branches), and the
// ProjectRootUnresolved.Error method together.
func TestProjectRootOrRaise_FailureNoEnv(t *testing.T) {
	empty := makeDir(t, t.TempDir(), "no-markers-no-env")
	chdirAndRestore(t, empty)
	t.Setenv(EnvProjectRoot, "")
	t.Setenv(EnvDomainsRoot, "")

	root, err := ProjectRootOrRaise()
	if err == nil {
		t.Fatalf("expected error when no source resolves, got root=%q nil err", root)
	}
	msg := err.Error()
	for _, want := range []string{
		"ProjectRoot() could not resolve",
		"R1 env", "not set",
		"R2 env", "not set",
		"R3 CWD",
		"R4 marker file",
		"R5 pyproject.toml",
		"R6 self-hosting",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("diagnostic missing %q; got:\n%s", want, msg)
		}
	}
}

// TestProjectRootOrRaise_FailureWithBogusEnv sets the R1/R2 env vars to paths
// that do not exist, exercising the buildDiagnostic "NOT a directory/missing"
// branches (vs the "not set" branches above).
func TestProjectRootOrRaise_FailureWithBogusEnv(t *testing.T) {
	empty := makeDir(t, t.TempDir(), "no-markers-bogus-env")
	chdirAndRestore(t, empty)
	t.Setenv(EnvProjectRoot, filepath.Join(t.TempDir(), "no-such-dir"))
	t.Setenv(EnvDomainsRoot, filepath.Join(t.TempDir(), "also-gone"))

	_, err := ProjectRootOrRaise()
	if err == nil {
		t.Fatalf("expected error when env vars point at non-existent paths")
	}
	msg := err.Error()
	if !strings.Contains(msg, "NOT a directory/missing") {
		t.Errorf("diagnostic should flag bogus env values as missing, got:\n%s", msg)
	}
	// both R1 and R2 lines should carry the configured env var names
	if !strings.Contains(msg, EnvProjectRoot) || !strings.Contains(msg, EnvDomainsRoot) {
		t.Errorf("diagnostic should name both env vars, got:\n%s", msg)
	}
}

// TestProjectRootOrRaise_Success exercises the success path: a CWD with a
// RELIABLE native marker resolves and ProjectRootOrRaise returns (root, nil).
func TestProjectRootOrRaise_Success(t *testing.T) {
	root := t.TempDir()
	makeDir(t, root, "domains")
	chdirAndRestore(t, root)

	got, err := ProjectRootOrRaise()
	if err != nil {
		t.Fatalf("expected success with a domains/ marker, got err: %v", err)
	}
	if got != root {
		t.Errorf("resolved = %q, want %q", got, root)
	}
}

// TestIsInside covers all three return branches of the containment predicate,
// including the rel=="." identity case that ProjectRoot()'s R6 fallback never
// reaches on its own (CWD normally differs from the repo root).
func TestIsInside(t *testing.T) {
	t.Parallel()
	parent := filepath.Join(os.TempDir(), "isinside-parent")
	child := filepath.Join(parent, "child")
	sibling := filepath.Join(parent, "other")

	cases := []struct {
		name   string
		child  string
		parent string
		want   bool
	}{
		{"identity", parent, parent, true},                 // rel == "." → true
		{"direct child", child, parent, true},              // relative sub-path → true
		{"sibling not inside", sibling, child, false},      // rel starts with ".." → false
		{"parent not inside child", parent, child, false},  // rel == ".." → false
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := isInside(c.child, c.parent); got != c.want {
				t.Errorf("isInside(%q, %q) = %v, want %v", c.child, c.parent, got, c.want)
			}
		})
	}
}

// TestParseTomlValue covers the quoted-value branch with escape unescaping
// (\" → " and \\ → \), which no existing pyproject fixture exercises.
func TestParseTomlValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{`"plain"`, "plain"},                // simple quoted → unwrapped
		{`"a\"b"`, `a"b`},                   // escaped quote unescaped
		{`"c:\\d"`, `c:\d`},                 // escaped backslash unescaped
		{`bare`, "bare"},                    // unquoted → trimmed as-is
		{`  "spaced"  `, "spaced"},          // surrounding whitespace trimmed
		{`"`, `"`},                          // single quote → not a pair → as-is
	}
	for _, c := range cases {
		c := c
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			if got := parseTomlValue(c.in); got != c.want {
				t.Errorf("parseTomlValue(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestStripTomlComment covers comment stripping inside vs outside quotes.
func TestStripTomlComment(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"key = val # comment", "key = val "},
		{`"url = http://x#y" # c`, `"url = http://x#y" `},
		{"nopound", "nopound"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()
			if got := stripTomlComment(c.in); got != c.want {
				t.Errorf("stripTomlComment(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestResolvePyproject_EmptyProjectRootSkipped: a pyproject with the
// [tool.hotam-spec] table but NO project_root key must NOT resolve (the key
// is what the legacy fallback needs to point at a root).
func TestResolvePyproject_EmptyProjectRootSkipped(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "pyproject.toml", "[tool.hotam-spec]\nname = \"x\"\n")
	if _, ok := resolvePyproject(root, 1); ok {
		t.Errorf("pyproject without project_root key must not resolve")
	}
}

// TestResolvePyproject_TargetIsFileSkipped: project_root pointing at a regular
// file (not a directory) must be rejected by the IsDir guard.
func TestResolvePyproject_TargetIsFileSkipped(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, root, "not-a-dir", "x")
	writeFile(t, root, "pyproject.toml", "[tool.hotam-spec]\nproject_root = \"not-a-dir\"\n")
	if _, ok := resolvePyproject(root, 1); ok {
		t.Errorf("project_root pointing at a file (not a dir) must not resolve")
	}
}
