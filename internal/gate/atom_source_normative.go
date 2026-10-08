package gate

import (
	"fmt"
	"strings"
)

// Includes refer only to constants in this invocation's parsed source snapshot.
func (index *AtomSourceIndex) expandNormativeText() error {
	for _, sources := range []map[string][]AtomSource{index.full, index.short, index.byLink} {
		for key, entries := range sources {
			for position := range entries {
				source := &entries[position]
				for language, text := range source.Phrases {
					if !strings.Contains(text, "include:") {
						continue
					}
					expanded, err := index.expandNormativePhrase(text, language)
					if err != nil {
						return atomPhraseError(*source, language, err.Error())
					}
					source.Phrases[language] = expanded
				}
				if text, ok := source.Phrases[index.primaryLanguage()]; ok {
					source.Phrase = text
				}
			}
			sources[key] = entries
		}
	}
	return nil
}

func (index *AtomSourceIndex) expandNormativePhrase(text, language string) (string, error) {
	lines := strings.Split(text, "\n")
	var fence markdownFence
	for position, line := range lines {
		if fence.consume(line) || !strings.HasPrefix(line, "include:") {
			continue
		}
		reference := strings.TrimSpace(strings.TrimPrefix(line, "include:"))
		translations, ok := index.fragments[reference]
		if !ok {
			return "", fmt.Errorf("normative text include %q does not identify a documented constant in the source snapshot", reference)
		}
		fragment, ok := translations[language]
		if !ok || strings.TrimSpace(fragment) == "" {
			return "", fmt.Errorf("normative text include %q has no text for language %q", reference, language)
		}
		for fragmentPosition, fragmentLine := range strings.Split(fragment, "\n") {
			if !fence.consume(fragmentLine) && strings.HasPrefix(fragmentLine, "include:") {
				return "", fmt.Errorf("normative text include %q must not contain nested includes: %q at fragment text line %d", reference, strings.TrimSpace(strings.TrimPrefix(fragmentLine, "include:")), fragmentPosition+1)
			}
		}
		lines[position] = fragment
	}
	return strings.Join(lines, "\n"), nil
}
