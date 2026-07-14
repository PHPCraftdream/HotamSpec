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

	full := BuildRequirements(g, "hotam-spec-self", false)
	consumer := BuildRequirements(g, "hotam-spec-self", true)

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

// TestBuildRequirements_ConsumerClosingSectionDomainPrefixesToolsIndex proves
// the R7-a fix (F2, review-6 @fl follow-up on task #140): the consumer
// closing section's "Implemented commands" pointer must be domain-prefixed
// ("domains/<name>/docs/gen/tools/INDEX.md"), matching the repo-root-relative
// convention every other cross-reference inside a generated docs/gen/*.md
// file follows (see domains/hotam-spec-self/docs/gen/AGENT-CONTEXT.md, whose
// cross-references are all "domains/hotam-spec-self/docs/gen/X.md" even
// though the referencing and referenced files are siblings in the same
// directory) — a bare "docs/gen/tools/INDEX.md" would resolve at the repo
// root, which is never where a domain's generated docs live (see
// cmd/hotam/init_project.go).
func TestBuildRequirements_ConsumerClosingSectionDomainPrefixesToolsIndex(t *testing.T) {
	t.Parallel()
	g := loadFixtureGraph(t)

	const domainName = "some-other-domain"
	consumer := BuildRequirements(g, domainName, true)

	wantPrefixed := "`domains/" + domainName + "/docs/gen/tools/INDEX.md`"
	if !strings.Contains(consumer, wantPrefixed) {
		t.Errorf("consumer closing section must reference %q, got:\n%s", wantPrefixed, consumer)
	}

	// The bare (bug) form must not survive: a literal "docs/gen/tools/INDEX.md"
	// NOT preceded by "domains/<name>/" would resolve to a nonexistent
	// repo-root path.
	bareForm := "`docs/gen/tools/INDEX.md`"
	if strings.Contains(consumer, bareForm) {
		t.Errorf("consumer closing section must NOT reference the bare, non-domain-prefixed form %q", bareForm)
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
	got := BuildRequirements(g, "hotam-spec-self", false)
	want, err := os.ReadFile("testdata/fixture/REQUIREMENTS.md")
	if err != nil {
		t.Fatalf("read reference: %v", err)
	}
	diffReport(t, "REQUIREMENTS.md (full profile)", got, string(want))
}
