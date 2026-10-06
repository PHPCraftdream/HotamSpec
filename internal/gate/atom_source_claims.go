package gate

import (
	"fmt"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"strings"
)

// atom_source_claims.go derives requirement claim text from recorded atom artifacts.
func (index *AtomSourceIndex) claimLanguages() []string {
	if len(index.languages) == 0 {
		return legacyAtomLanguages[:]
	}
	return index.languages
}

func (index *AtomSourceIndex) primaryLanguage() string {
	if index.defaultLanguage != "" {
		return index.defaultLanguage
	}
	if len(index.languages) == 1 {
		return index.languages[0]
	}
	return ""
}

// DeriveClaims uses one AST snapshot to derive each declared normative view.
// Contract: rule and holds claims carry only the first (predicate) step's
// authored phrases — evidence methods stay in the artifact and SPEC views.
// Fact claims keep the phrase-plus-executed-value semantics, except bool
// atoms: value "true" renders the bare phrase and "false" renders the
// authored `not:` negation phrase; a false verdict without a `not:` phrase is
// an error, never an auto-generated "не ..." wording. In multilingual domains
// an executed value matching a typed model string constant is translated via
// that constant's language blocks; an untranslated matched string constant is
// an error, while `>>>>> lang=*` marks a verbatim value.
func (index *AtomSourceIndex) DeriveClaims(a AtomArtifact) (ontology.LocalizedText, error) {
	if a.Mode != "fact" && a.Mode != "holds" && a.Mode != "rule" {
		return nil, fmt.Errorf("unknown atom mode %q", a.Mode)
	}
	if a.Verdict != "pass" {
		return nil, fmt.Errorf("atom %s did not pass", a.ReqID)
	}
	if len(a.Steps) == 0 || (a.Mode == "fact" && len(a.Steps) != 1) {
		return nil, fmt.Errorf("atom %s has invalid subject count", a.ReqID)
	}
	if a.Mode == "rule" && !index.ruleCases {
		return nil, fmt.Errorf("rule atom %s requires conformance.rule_cases", a.ReqID)
	}
	if a.Mode == "rule" && (a.Case == nil || strings.TrimSpace(a.Case.ID) == "") {
		return nil, fmt.Errorf("rule atom %s has no case ID", a.ReqID)
	}
	languages := index.claimLanguages()
	claims := make(ontology.LocalizedText, len(languages))
	parts := make(map[string][]string, len(languages))
	seen := map[string]bool{}
	for _, step := range a.Steps {
		if a.Mode != "fact" && len(seen) > 0 {
			break
		}
		if seen[step.Subject] {
			continue
		}
		seen[step.Subject] = true
		source, err := index.Resolve(step.Subject)
		if err != nil {
			return nil, err
		}
		boolMethod := source.returnsBool()
		hasValue := a.Mode != "rule"
		boolValue := boolMethod && (step.Value == "true" || step.Value == "false")
		if hasValue && !boolValue && len(index.languages) > 1 {
			if entry, ok := index.valueConstant(source, step.Value); ok && entry.untranslatedString() {
				where := entry.position
				return nil, fmt.Errorf("%s:%d: constant %s: string constant value without translation in a multilingual domain", where.Filename, where.Line, entry.name)
			}
		}
		for _, language := range languages {
			phrase, ok := source.Phrases[language]
			if !ok || strings.TrimSpace(phrase) == "" {
				return nil, atomPhraseError(source, language, "missing non-empty source phrase")
			}
			notPhrase := strings.TrimSpace(source.NotPhrases[language])
			if boolMethod && step.Value == "false" && notPhrase == "" {
				return nil, atomPhraseError(source, language, "bool atom evaluated to false without a `not:` negation phrase")
			}
			value := step.Value
			if hasValue && !boolValue {
				if entry, ok := index.valueConstant(source, step.Value); ok && !entry.verbatim && len(entry.translations) > 0 {
					value = entry.translations[language]
				}
			}
			part, err := atomPhraseText(language, phrase, notPhrase, value, hasValue, boolMethod)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", source.Link(), err)
			}
			parts[language] = append(parts[language], part)
		}
	}
	for _, language := range languages {
		claims[language] = strings.Join(parts[language], " ")
	}
	primary := claims[index.primaryLanguage()]
	if a.Title != "" && a.Title != primary {
		return nil, fmt.Errorf("atom %s explicit title %q conflicts with derived claim %q", a.ReqID, a.Title, primary)
	}
	return claims, nil
}

// DeriveClaim derives the primary language view. Its selection is explicit:
// graph default language, the sole configured language, or the legacy phrase.
func (index *AtomSourceIndex) DeriveClaim(a AtomArtifact) (string, error) {
	claims, err := index.DeriveClaims(a)
	if err != nil {
		return "", err
	}
	claim, ok := claims[index.primaryLanguage()]
	if !ok {
		return "", fmt.Errorf("atom %s has no primary claim language %q", a.ReqID, index.primaryLanguage())
	}
	return claim, nil
}

func DeriveAtomClaim(specRoot string, a AtomArtifact) (string, error) {
	index, err := NewAtomSourceIndex(specRoot)
	if err != nil {
		return "", err
	}
	return index.DeriveClaim(a)
}
