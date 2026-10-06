package selfspec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// TestProjectIdentityHotamSpec pins the checkable core of R-project-name-hotam-spec.
func TestProjectIdentityHotamSpec(t *testing.T) {
	root, ok := paths.ProjectRoot()
	if !ok {
		t.Fatal("paths.ProjectRoot() did not resolve the engine repository root")
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	const wantModule = "module github.com/PHPCraftdream/HotamSpec"
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == wantModule {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("go.mod module path drifted from the deliberate %q (R-project-name-hotam-spec, commit 4325ac8)", wantModule)
	}
	for _, dir := range []string{filepath.Join("domains", "hotam-spec-self"), filepath.Join("cmd", "hotam")} {
		if info, err := os.Stat(filepath.Join(root, dir)); err != nil || !info.IsDir() {
			t.Errorf("kebab-case artifact %s missing or not a directory (R-project-name-hotam-spec)", dir)
		}
	}
}
