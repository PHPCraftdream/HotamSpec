package selfspec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/paths"
)

// frozenAspectsBaseline mirrors testdata/frozen_aspects_baseline.json.
type frozenAspectsBaseline struct {
	Files              map[string]string `json:"files"`
	FrozenToolsPlanned []string          `json:"frozen_tools_planned"`
}

// TestFrozenAspectsSnapshot is the frozen-aspects snapshot guard R-speculative-aspects-frozen names as its missing enforcer.
func TestFrozenAspectsSnapshot(t *testing.T) {
	root, ok := paths.ProjectRoot()
	if !ok {
		t.Fatal("paths.ProjectRoot() did not resolve the engine repository root")
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "frozen_aspects_baseline.json"))
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	var baseline frozenAspectsBaseline
	if err := json.Unmarshal(raw, &baseline); err != nil {
		t.Fatalf("parse baseline: %v", err)
	}
	files := make([]string, 0, len(baseline.Files))
	for f := range baseline.Files {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("frozen file %s unreadable: %v", rel, err)
		}
		sum := sha256.Sum256(data)
		got := hex.EncodeToString(sum[:])
		if got != baseline.Files[rel] {
			t.Errorf("FROZEN ASPECT EDITED: %s changed since the baseline (got sha256 %s, baseline %s). The Entity aspect / multi-domain federation / sub-agent recursion surface is frozen by R-speculative-aspects-frozen; unfreeze consciously via a resolver decision first, then update internal/selfspec/testdata/frozen_aspects_baseline.json in the same change.", rel, got, baseline.Files[rel])
		}
	}
	for _, name := range baseline.FrozenToolsPlanned {
		tool, ok := methodology.Tools.Get(name)
		if !ok {
			t.Errorf("frozen tool %q vanished from the methodology.Tools registry — the frozen surface drifted (R-speculative-aspects-frozen)", name)
			continue
		}
		if tool.Status != methodology.Planned {
			t.Errorf("frozen tool %q has status %q, want %q: implementing a frozen aspect requires a resolver unfreeze decision (R-speculative-aspects-frozen)", name, tool.Status, methodology.Planned)
		}
	}
}
