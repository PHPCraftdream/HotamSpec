package repohygiene

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// Machine-path patterns: private absolute Windows paths of a developer's
// machine must not leak into tracked files (review 2026-10-05, P0-1).
var (
	reWinHome   = regexp.MustCompile(`(?i)C:[/\\]Users[/\\][^\s"'\x60(),;]+`)
	reDevDrives = regexp.MustCompile(`(?i)[Dd]:[/\\](dev|ai_dev|system_artefact)(?:$|[^A-Za-z0-9_-])`)
)

// allowSuffix lists tracked files exempt from the scan:
//   - internal/generator/relpath_test.go: synthetic Windows roots (C:\proj and
//     a dev-drive project root) that never existed on any real machine;
//   - internal/loader/testdata: synthetic graph fixture;
//   - domains/hotam-spec-self/graph.json and docs/gen/**: FROZEN signed-off
//     graph history and its generated projections — hand-editing them would
//     break check_graph_lock_pins_graph_json; their evidence strings record
//     the pre-rename repo location as historical fact;
//   - proposals/**: frozen applied-proposal records (same evidence history);
//   - docs/reviews/2026-10-05-framework-review.md: the review that reported
//     this finding, quoting the pattern shapes themselves.
var allowSuffix = []string{
	"internal/generator/relpath_test.go",
	"internal/loader/testdata/",
	"domains/hotam-spec-self/graph.json",
	"domains/hotam-spec-self/docs/gen/",
	"domains/hotam-spec-self/proposals/",
	"proposals/",
	"docs/reviews/2026-10-05-framework-review.md",
}

var binExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".ico": true, ".pdf": true, ".zip": true, ".gz": true, ".exe": true,
}

func allowed(path string) bool {
	for _, s := range allowSuffix {
		if strings.HasSuffix(path, s) || strings.HasPrefix(path, s) {
			return true
		}
	}
	return false
}

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("git unavailable or not a repo: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestNoPrivateMachinePathsInTrackedFiles(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	for _, name := range bytes.Split(out, []byte{0}) {
		path := string(name)
		if path == "" || allowed(path) {
			continue
		}
		if binExt[strings.ToLower(filepath.Ext(path))] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("%s: read: %v", path, err)
			continue
		}
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			continue // binary
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, re := range []*regexp.Regexp{reWinHome, reDevDrives} {
				if m := re.FindString(line); m != "" {
					t.Errorf("%s:%d: private machine path %q", path, i+1, m)
				}
			}
		}
	}
}
