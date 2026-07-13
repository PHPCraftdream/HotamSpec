package generator

import (
	"os"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

// TestScanToolRequirements_ProjectsEveryEntryToRequirement enforces
// R-tool-is-its-own-requirement: every entry in the methodology.Tools registry
// is projected into a STRUCTURAL R-tool-<basename> requirement (id = "R-tool-" +
// basename with _ → -) carrying non-empty claim text and a Canon §-section, and
// every projected id is rendered by BuildToolDerivedSection. A regression that
// broke the id formula, dropped a claim, skipped a row, introduced a duplicate,
// or disconnected the renderer from the registry fails this test.
func TestScanToolRequirements_ProjectsEveryEntryToRequirement(t *testing.T) {
	t.Parallel()
	toolReqs := ScanToolRequirements()

	if len(toolReqs) != len(methodology.Tools.All()) {
		t.Fatalf("ScanToolRequirements returned %d entries, expected %d (one per registry tool)",
			len(toolReqs), len(methodology.Tools.All()))
	}

	rendered := BuildToolDerivedSection()
	seen := make(map[string]bool)

	for _, tr := range toolReqs {
		wantID := "R-tool-" + strings.ReplaceAll(tr.Basename, "_", "-")
		if tr.ID != wantID {
			t.Errorf("basename %q projected to id %q, want %q", tr.Basename, tr.ID, wantID)
		}
		if strings.TrimSpace(tr.Claim) == "" {
			t.Errorf("projected requirement %q has empty claim text (must carry the tool's claim)", tr.ID)
		}
		if strings.TrimSpace(tr.CanonSection) == "" {
			t.Errorf("projected requirement %q has empty CanonSection", tr.ID)
		}
		if seen[tr.Basename] {
			t.Errorf("duplicate basename %q in projection (dedup integrity)", tr.Basename)
		}
		seen[tr.Basename] = true
		// every projected id must render as a bold header in BuildToolDerivedSection
		if !strings.Contains(rendered, "**"+tr.ID+"**") {
			t.Errorf("projected requirement %q is not rendered by BuildToolDerivedSection (renderer/scan drift)", tr.ID)
		}
	}
}

// TestScanToolRequirements_MatchesRegistryCount guards the single-source-of-truth
// invariant: the scan's output count must equal the registry's entry count, so a
// second hand-maintained list cannot drift back in. It also asserts the stale
// `hotam_req` basename (a former entry that named a non-existent command,
// superseded by `req`) never resurfaces in the projection.
func TestScanToolRequirements_MatchesRegistryCount(t *testing.T) {
	t.Parallel()
	toolReqs := ScanToolRequirements()
	want := len(methodology.Tools.All())
	if len(toolReqs) != want {
		t.Fatalf("ScanToolRequirements returned %d entries, expected %d (= len(methodology.Tools.All()))",
			len(toolReqs), want)
	}
	for _, tr := range toolReqs {
		if tr.Basename == "hotam_req" {
			t.Errorf("stale basename %q present in projection — it names a command that does not exist in the registry", tr.Basename)
		}
	}
}

// mainGoPath is the repo-root-relative path to the CLI dispatcher, resolved
// against this test file's own package directory (internal/generator) the same
// way byteidentical_test.go's domainGraphPath constant ("../../domains/...")
// resolves repo-relative sources.
const mainGoPath = "../../cmd/hotam/main.go"

// TestToolRegistry_ImplementedCommandsWiredInCLI is the three-way drift guard
// (registry ↔ CLI wiring ↔ generated docs): every Tool in methodology.Tools
// with Status == Implemented must have its Command (underscore form) — converted
// to hyphen form — present as a `case` in cmd/hotam/main.go's dispatch switch.
// It reads the dispatcher's source text at test time (cmd/hotam is package main,
// so it cannot be imported without a cycle) and does a plain substring match — a
// smoke guard, not a full parser.
func TestToolRegistry_ImplementedCommandsWiredInCLI(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(mainGoPath)
	if err != nil {
		t.Fatalf("read %s: %v", mainGoPath, err)
	}
	dispatch := string(src)
	for _, tool := range methodology.Tools.All() {
		if tool.Status != methodology.Implemented {
			continue
		}
		hyphenated := strings.ReplaceAll(tool.Command, "_", "-")
		needle := `case "` + hyphenated + `"`
		if !strings.Contains(dispatch, needle) {
			t.Errorf("Implemented tool %q: hyphenated name %q not wired as %s in %s",
				tool.Command, hyphenated, needle, mainGoPath)
		}
	}
}
