package generator

import (
	"errors"
	"strings"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/localization"
)

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

func TestBuildToolDocsLocalizedTranslatesRegistryDescriptionWithoutChangingCommandData(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		language   string
		translated string
	}{
		{language: "ru", translated: "Пересоздаёт docs/gen/*.md"},
		{language: "zh", translated: "根据可执行模型"},
	} {
		docs, err := BuildToolDocsLocalized(tc.language, false)
		if err != nil {
			t.Fatalf("BuildToolDocsLocalized(%q): %v", tc.language, err)
		}
		doc := docs["gen_spec"]
		for _, want := range []string{"# gen_spec", tc.translated, "--today YYYY-MM-DD", "internal/generator + internal/ontology"} {
			if !strings.Contains(doc, want) {
				t.Errorf("%s tool page does not preserve translated registry purpose and command data %q:\n%s", tc.language, want, doc)
			}
		}
		if strings.Contains(doc, "Regenerates docs/gen/*.md") {
			t.Errorf("%s tool page retained the English purpose instead of its exact locale view", tc.language)
		}
	}
}

func TestBuildToolDocsLocalizedRefusesUnsupportedLocale(t *testing.T) {
	t.Parallel()
	_, err := BuildToolDocsLocalized("fr", false)
	var missing *localization.MissingTranslation
	if !errors.As(err, &missing) || missing.Language != "fr" {
		t.Fatalf("BuildToolDocsLocalized(fr) error = %#v, want typed missing-translation refusal", err)
	}
}
