package generator

import (
	"os"
	"strings"
	"testing"
)

// TestBuildRequirements_ConsumerProfileDropsFrameworkNoise proves the R6-j
// cut (task #140): under consumer==true, BuildRequirements skips
// BuildToolDerivedSection() (the ~44 synthetic tool-derived requirements
// describing the FRAMEWORK's own CLI surface) and the "## Methodology
// (generated from module docstrings)" encyclopedia (the full Canon/
// Narrative/Why per §-section, duplicated in full from
// docs/gen/thinking/*.md), replacing both with a short closing section —
// while still carrying the domain's OWN "## Requirement roster" table
// (the one thing an external consumer actually needs).
func TestBuildRequirements_ConsumerProfileDropsFrameworkNoise(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)

	full := BuildRequirements(g, false)
	consumer := BuildRequirements(g, true)

	// Marker strings characteristic of each removed section.
	const toolDerivedMarker = "## Tool-derived requirements" // BuildToolDerivedSection's own heading text
	const methodologyMarker = "## Methodology (generated from module docstrings)"

	if !strings.Contains(full, toolDerivedMarker) {
		t.Errorf("full profile must contain the tool-derived section marker %q — precondition for this test is broken", toolDerivedMarker)
	}
	if !strings.Contains(full, methodologyMarker) {
		t.Errorf("full profile must contain the methodology-encyclopedia heading %q — precondition for this test is broken", methodologyMarker)
	}

	if strings.Contains(consumer, toolDerivedMarker) {
		t.Errorf("consumer profile must NOT contain the tool-derived section marker %q", toolDerivedMarker)
	}
	if strings.Contains(consumer, methodologyMarker) {
		t.Errorf("consumer profile must NOT contain the methodology-encyclopedia heading %q", methodologyMarker)
	}

	// The domain's own requirement roster must survive in BOTH profiles —
	// this task must not accidentally strip the one thing an external
	// consumer actually needs.
	const rosterMarker = "## Requirement roster"
	if !strings.Contains(full, rosterMarker) {
		t.Errorf("full profile must contain %q", rosterMarker)
	}
	if !strings.Contains(consumer, rosterMarker) {
		t.Errorf("consumer profile must contain %q", rosterMarker)
	}

	// Concrete size assertion: consumer must be meaningfully smaller, not
	// merely "not equal". The full profile's dropped sections dwarf the
	// short closing section that replaces them.
	if len(consumer) >= len(full) {
		t.Fatalf("consumer profile (%d bytes) must be smaller than full profile (%d bytes)", len(consumer), len(full))
	}
	// The full fixture output's dropped sections (tool-derived + methodology
	// encyclopedia) dominate even this small ~20-node fixture graph; the
	// consumer cut should remove at least half of it.
	if len(consumer) > len(full)/2 {
		t.Errorf("consumer profile (%d bytes) not meaningfully smaller than full profile (%d bytes) — expected well under half", len(consumer), len(full))
	}

	consumerLines := strings.Count(consumer, "\n")
	fullLines := strings.Count(full, "\n")
	if consumerLines >= fullLines {
		t.Fatalf("consumer profile (%d lines) must have fewer lines than full profile (%d lines)", consumerLines, fullLines)
	}
}

// TestBuildRequirements_FullProfileByteIdenticalToPreChangeFixture pins
// full-profile (consumer==false) output to the exact same golden fixture
// TestBuildRequirements_ByteIdenticalToFixture already checks — restated
// here under its own name so the R6-j task's byte-identity claim is
// independently verifiable without relying on a shared test name.
func TestBuildRequirements_FullProfileByteIdenticalToPreChangeFixture(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)
	got := BuildRequirements(g, false)
	want, err := os.ReadFile("testdata/fixture/REQUIREMENTS.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "REQUIREMENTS.md (full profile)", got, string(want))
}
