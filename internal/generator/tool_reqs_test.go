package generator

import (
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
)

// TestScanToolRequirements_ProjectsEveryEntryToRequirement pins the 1:1
// tool→requirement projection contract of R-tool-is-its-own-requirement:
// every methodology.Tools entry projects to exactly one R-tool-<basename>
// carrying that tool's own Claim text, with no filtering and no second
// hand-maintained table.
func TestScanToolRequirements_ProjectsEveryEntryToRequirement(t *testing.T) {
	tools := methodology.Tools.All()
	if len(tools) == 0 {
		t.Fatal("methodology.Tools registry is empty: nothing to project (R-tool-is-its-own-requirement)")
	}
	projected := ScanToolRequirements()
	if len(projected) != len(tools) {
		t.Fatalf("projection is not 1:1: registry carries %d tools, ScanToolRequirements returned %d (a filter crept into the projection)", len(tools), len(projected))
	}
	byID := make(map[string]ToolRequirement, len(projected))
	for _, tr := range projected {
		byID[tr.ID] = tr
	}
	for _, tool := range tools {
		wantID := "R-tool-" + strings.ReplaceAll(tool.Command, "_", "-")
		tr, ok := byID[wantID]
		if !ok {
			t.Errorf("tool %q has no projected R-tool requirement %q", tool.Command, wantID)
			continue
		}
		if tr.Basename != tool.Command {
			t.Errorf("projected basename %q != tool command %q", tr.Basename, tool.Command)
		}
		if tr.Claim != tool.Claim {
			t.Errorf("projected claim for %q is not the tool's own Claim text (a second hand-maintained claim table is forbidden by R-tool-is-its-own-requirement)", tool.Command)
		}
		if wantCanon := strings.TrimPrefix(tool.Canon, "§"); tr.CanonSection != wantCanon {
			t.Errorf("projected canon section %q != tool canon %q with § stripped", tr.CanonSection, tool.Canon)
		}
		if tr.Enforcer != tool.Enforcer {
			t.Errorf("projected enforcer %q != tool enforcer %q", tr.Enforcer, tool.Enforcer)
		}
		delete(byID, wantID)
	}
	for id := range byID {
		t.Errorf("projected requirement %q has no backing registry entry (a stale or hand-added projection)", id)
	}
	for i := 1; i < len(projected); i++ {
		if projected[i-1].Basename > projected[i].Basename {
			t.Fatalf("projection is not sorted by basename: %q > %q", projected[i-1].Basename, projected[i].Basename)
		}
	}
	section := BuildToolDerivedSection()
	for _, tr := range projected {
		if !strings.Contains(section, tr.ID) {
			t.Errorf("rendered tool-derived section does not contain %q — the projection never reaches FRAMEWORK-INVARIANTS.md", tr.ID)
		}
	}
}
