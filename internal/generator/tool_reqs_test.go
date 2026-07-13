package generator

import (
	"strings"
	"testing"
)

// TestScanToolRequirements_ProjectsEveryEntryToRequirement enforces
// R-tool-is-its-own-requirement: every row of the toolRequirementData table is
// projected into a STRUCTURAL R-tool-<basename> requirement (id = "R-tool-" +
// basename with _ → -) carrying non-empty claim text and a Canon §-section, and
// every projected id is rendered by BuildToolDerivedSection. A regression that
// broke the id formula, dropped a claim, skipped a row, introduced a duplicate,
// or disconnected the renderer from the scan fails this test.
func TestScanToolRequirements_ProjectsEveryEntryToRequirement(t *testing.T) {
	t.Parallel()
	toolReqs := ScanToolRequirements()

	if len(toolReqs) != len(toolRequirementData) {
		t.Fatalf("ScanToolRequirements returned %d entries, expected %d (one per data row)",
			len(toolReqs), len(toolRequirementData))
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
