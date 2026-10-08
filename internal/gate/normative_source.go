package gate

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// normativeTextSource retains source positions for diagnostics, not identity.
// Its reference is the declared file:symbol, independent of source line ranges.
type normativeTextSource struct {
	texts  ontology.LocalizedText
	source AtomSource
}

func (index *AtomSourceIndex) indexNormativeText(reference string, texts ontology.LocalizedText, source AtomSource) {
	index.normativeTexts[reference] = append(index.normativeTexts[reference], normativeTextSource{texts: texts, source: source})
}

// ResolveNormativeText resolves a documented typed constant in this invocation's
// AST snapshot. It never reads Markdown or reparses files at publication time.
// Returned maps are owned by the caller; no fallback language is introduced.
func (index *AtomSourceIndex) ResolveNormativeText(reference string) (ontology.LocalizedText, error) {
	if index == nil {
		return nil, fmt.Errorf("normative text %q: source snapshot is absent", reference)
	}
	if reference == "" || strings.TrimSpace(reference) != reference {
		return nil, fmt.Errorf("normative text reference %q is empty or has surrounding whitespace", reference)
	}
	entries := index.normativeTexts[reference]
	if len(entries) != 1 {
		return nil, fmt.Errorf("normative text %q resolves to %d documented typed constants in the source snapshot", reference, len(entries))
	}
	entry := entries[0]
	languages := index.claimLanguages()
	for _, language := range languages {
		if strings.TrimSpace(entry.texts[language]) == "" {
			return nil, atomPhraseError(entry.source, language, "normative text has no non-empty translation")
		}
	}
	out := make(ontology.LocalizedText, len(entry.texts))
	for _, language := range languages {
		// Reuse the fence-aware include policy for active nested directives.
		expanded, err := index.expandNormativePhrase("include: "+reference, language)
		if err != nil {
			return nil, atomPhraseError(entry.source, language, err.Error())
		}
		out[language] = expanded
	}
	return out, nil
}

// ResolveNormativeText exposes document resolution on the same execution
// snapshot that owns source indexing and evidence, rather than a second cache.
func (snapshot *AtomExecutionSnapshot) ResolveNormativeText(reference string) (ontology.LocalizedText, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("normative text %q: execution snapshot is absent", reference)
	}
	if snapshot.SourceErr != nil {
		return nil, fmt.Errorf("normative text %q: source snapshot: %w", reference, snapshot.SourceErr)
	}
	return snapshot.SourceIndex.ResolveNormativeText(reference)
}
