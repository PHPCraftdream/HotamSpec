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

// allowFiles lists exact files carrying frozen historical machine-path evidence
// (signed-off graph history, its generated projections and applied proposals;
// synthetic test roots; the review quoting the patterns). Anything else is scanned.
var allowFiles = map[string]bool{
	"internal/generator/relpath_test.go":                                                                                    true,
	"internal/loader/testdata/hotam-spec-self.graph.json":                                                                   true,
	"domains/hotam-spec-self/graph.json":                                                                                    true,
	"domains/hotam-spec-self/docs/gen/graph.json":                                                                           true,
	"domains/hotam-spec-self/docs/gen/atoms-check.md":                                                                       true,
	"domains/hotam-spec-self/proposals/task226-recursion/001-R-lifecycle-type-exists-authored.json":                         true,
	"domains/hotam-spec-self/proposals/task226-recursion/002-R-conflict-is-connector-node-authored.json":                    true,
	"domains/hotam-spec-self/proposals/task226-recursion/003-R-no-hand-edit-graph-authored.json":                            true,
	"domains/hotam-spec-self/proposals/task226-recursion/004-R-requirement-freshness-fields-authored.json":                  true,
	"domains/hotam-spec-self/proposals/task236-selfexample/001-R-spec-link-embodied-vs-proven-authored.json":                true,
	"domains/hotam-spec-self/proposals/task236-selfexample/002-R-structural-floor-vs-mirror-audit-authored.json":            true,
	"domains/hotam-spec-self/proposals/task236-selfexample/003-R-authored-spec-projections-are-derived-authored.json":       true,
	"domains/hotam-spec-self/proposals/task236-selfexample/004-R-authored-spec-links-mechanically-checked-authored.json":    true,
	"domains/hotam-spec-self/proposals/task236-selfexample/005-R-enforced-requires-enforcer-or-authored-link-authored.json": true,
	"proposals/update-R-authored-spec-links-mechanically-checked-f1.json":                                                   true,
	"proposals/wave9-lexical-fn-validation/02-R-no-hand-edit-graph-runtime-check.json":                                      true,
	"proposals/wave9-lexical-fn-validation/03-R-spec-link-embodied-vs-proven-orthogonal-carrier.json":                       true,
	"proposals/wave9-lexical-fn-validation/04-R-structural-floor-vs-mirror-audit-boundary-carrier.json":                     true,
	"docs/reviews/2026-10-05-framework-review.md":                                                                           true,
}

var binExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
	".ico": true, ".pdf": true, ".zip": true, ".gz": true, ".exe": true,
}

func allowed(path string) bool {
	return allowFiles[path]
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
	tracked := bytes.Split(out, []byte{0})
	// Untracked proposal additions are scanned too, so a new file cannot slip past before staging.
	_ = filepath.WalkDir(filepath.Join(root, "proposals"), func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if rel, relErr := filepath.Rel(root, path); relErr == nil {
				tracked = append(tracked, []byte(filepath.ToSlash(rel)))
			}
		}
		return nil
	})
	for _, name := range tracked {
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
