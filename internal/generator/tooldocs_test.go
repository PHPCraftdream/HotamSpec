package generator

import (
	"os"
	"strings"
	"testing"
)

// TestBuildToolDocsIndex_FullProfileByteIdenticalToFixture is the byte-identity
// contract for the full-profile (consumer == false) INDEX.md render: the
// profile-threading change (task #144 / R8-a) must leave the full-profile
// output EXACTLY as before — the consumer gating only diverges the Planned
// section's rendering when consumer == true. The golden fixture was captured
// from the pre-change BuildToolDocsIndex() output against the real tool
// registry, so any drift in the full-profile path surfaces here immediately.
func TestBuildToolDocsIndex_FullProfileByteIdenticalToFixture(t *testing.T) {
	t.Parallel()
	got := BuildToolDocsIndex(false)
	want, err := os.ReadFile("testdata/fixture/tools-INDEX.md")
	if err != nil {
		t.Fatalf("read reference fixture: %v", err)
	}
	diffReport(t, "tools/INDEX.md (full profile)", got, string(want))
}

// TestBuildToolDocsIndex_ConsumerPlannedSectionHasNoMarkdownLinks enforces the
// core fix of task #144 (R8-a): under the consumer profile, genSpec skips
// writing per-tool `.md` pages for all Planned tools (the toolIsImplemented
// filter in gen_spec.go), so the INDEX's Planned section must NOT emit
// markdown links `[...](....md)` — those would be dead links to files that
// were never written. The command names render as plain backtick code spans
// instead. Before the fix, every one of the 27 Planned tools shipped a dead
// `](<cmd>.md)` link.
//
// It also confirms the Planned section's intro sentence no longer references
// `internal/methodology/tools_data.go` (a framework SOURCE FILE path that
// does not exist in an external consumer's project) under the consumer
// profile — the same class of misleading cross-reference the review flagged
// across the generated docs.
func TestBuildToolDocsIndex_ConsumerPlannedSectionHasNoMarkdownLinks(t *testing.T) {
	t.Parallel()
	got := BuildToolDocsIndex(true)

	idx := strings.Index(got, "## Planned")
	if idx < 0 {
		t.Fatalf("consumer INDEX.md missing the Planned section header")
	}
	plannedSection := got[idx:]

	if strings.Contains(plannedSection, "](") {
		t.Errorf("consumer Planned section must not contain markdown links ](...), but it does:\n%s", plannedSection)
	}

	// The backtick code span `hotam <name>` must still be present (the names
	// are listed, just not linked) — confirms the tools are still enumerated.
	if !strings.Contains(plannedSection, "`hotam ") {
		t.Errorf("consumer Planned section must still list tool names as backtick code spans, but none found:\n%s", plannedSection)
	}

	if strings.Contains(got, "internal/methodology/tools_data.go") {
		t.Errorf("consumer INDEX.md must not reference the framework source file internal/methodology/tools_data.go")
	}

	// Implemented tools' links are UNAFFECTED — their pages are always written
	// in both profiles, so the Implemented section still carries real links.
	implIdx := strings.Index(got, "## Implemented")
	if implIdx < 0 {
		t.Fatalf("consumer INDEX.md missing the Implemented section header")
	}
	implSection := got[implIdx:idx]
	if !strings.Contains(implSection, "](") {
		t.Errorf("consumer Implemented section must still carry markdown links to per-tool pages (they are always written):\n%s", implSection)
	}
}
