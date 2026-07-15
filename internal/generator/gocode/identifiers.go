// Package gocode implements `hotam gen-code`'s model generator: it turns
// EntityType nodes from a domain graph into Go structs, state enums, and
// Validate() methods. See docs/GEN-CODE-CONTRACT.md for the full contract
// this package is bound by — in particular §4 (identifiers) and §2
// (EntityType -> Go mapping).
package gocode

import (
	"fmt"
	"strings"
)

// abbreviations is the GEN-CODE-CONTRACT.md §4.2 table verbatim: Russian
// abbreviations transliterated letter-for-letter to uppercase Latin (NOT
// translated). Keys are lowercased for case-insensitive matching against
// graph-name parts; the contract's examples ("фт", "отт") are lowercase.
var abbreviations = map[string]string{
	"фт":  "FT",
	"отт": "OTT",
	"иб":  "IB",
	"пм":  "PM",
	"рп":  "RP",
	"ба":  "BA",
}

// abbreviationLatinForms is the set of §4.2 abbreviation VALUES, lowercased,
// used to recognize a graph-name part that is already spelled in Latin as
// the abbreviation itself (e.g. field name "исходный_номер_ott" carries the
// part "ott" — the lowercase Latin spelling of "ОТТ" — rather than the
// Cyrillic form). Contract §4.3's worked example renders this part as "OTT"
// (uppercase, matching the abbreviation's canonical casing), not as a
// generic title-cased passthrough word ("Ott") — the author wrote the
// abbreviation directly in Latin instead of Cyrillic, so it is still the
// abbreviation, not an ordinary ASCII word like "gate" or "dto".
var abbreviationLatinForms = func() map[string]string {
	out := make(map[string]string, len(abbreviations))
	for _, latin := range abbreviations {
		out[strings.ToLower(latin)] = latin
	}
	return out
}()

// glossary is the GEN-CODE-CONTRACT.md §4.1 table verbatim: a curated
// Russian -> English word translation dictionary. Keys are lowercased.
var glossary = map[string]string{
	"варианты":        "variants",
	"версия":          "version",
	"волны":           "waves",
	"вопрос":          "question",
	"горизонт":        "horizon",
	"записи":          "records",
	"ид":              "id",
	"артефакта":       "artifact",
	"исходный":        "source",
	"номер":           "number",
	"кластер":         "cluster",
	"критический":     "critical",
	"путь":            "path",
	"ссылка":          "reference",
	"область":         "area",
	"ограничения":     "constraints",
	"резюме":          "summary",
	"главный":         "main",
	"риск":            "risk",
	"рекомендация":    "recommendation",
	"решение":         "decision",
	"решено":          "decided",
	"рёбра":           "edges",
	"сложность":       "complexity",
	"текст":           "text",
	"узлы":            "nodes",
	"на":              "at",
	"пересмотрен":     "revised",
	"пересмотреть":    "revise",
	"повторно":        "again",
	"утверждён":       "approved",
	"утвердить":       "approve",
	"черновик":        "draft",
	"вернуть":         "return",
	"доработку":       "rework",
	"декомпозировать": "decompose",
	"после":           "after",
	"зафиксировать":   "record",
	"представить":     "present",
	"пройти":          "pass",
	"составить":       "compile",
	"уточнить":        "clarify",
	"и":               "and",
}

// UnknownTermError is returned when a name part cannot be resolved via the
// abbreviation table (§4.2) or the glossary (§4.1), and is not already
// ASCII/Latin passthrough. Per contract §4 step 6, this must surface as a
// loud, explicit error — never a silent fallback to transliteration or a
// guessed translation.
type UnknownTermError struct {
	// Term is the exact unrecognized part (as it appeared in the graph name,
	// lowercased for lookup but reported here in its original form).
	Term string
	// Source is the full original graph-name being translated (e.g. the
	// EntityType.slug or field.name), for locating the offending name.
	Source string
}

func (e *UnknownTermError) Error() string {
	return fmt.Sprintf("gocode: unrecognized identifier part %q in graph name %q — not in GEN-CODE-CONTRACT.md §4.1 glossary or §4.2 abbreviation table", e.Term, e.Source)
}

// isASCII reports whether s consists entirely of ASCII characters (letters,
// digits, or otherwise) — a proxy for "already Latin", eligible for
// passthrough per contract §4 step 4.
func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// splitParts breaks a composite graph name into parts on `_` and `-`, per
// contract §4 step 1. Empty parts (from leading/trailing/doubled separators)
// are dropped.
func splitParts(name string) []string {
	fields := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-'
	})
	return fields
}

// resolvedPart is one graph-name part after resolution, carrying both the
// resolved Go-identifier fragment and whether it originated from the
// abbreviation table (which must stay all-uppercase when title-casing, since
// step 5 says abbreviations transliterate verbatim, not re-cased).
type resolvedPart struct {
	text          string
	isAbbrev      bool
	isPassthrough bool
}

// resolvePart resolves a single graph-name part per contract §4 steps 2-4:
// abbreviation table first, then glossary, then ASCII passthrough. Returns
// an *UnknownTermError (wrapping the original part and source name) if none
// match.
func resolvePart(part string, source string) (resolvedPart, error) {
	lower := strings.ToLower(part)

	if latin, ok := abbreviations[lower]; ok {
		return resolvedPart{text: latin, isAbbrev: true}, nil
	}
	if english, ok := glossary[lower]; ok {
		return resolvedPart{text: english}, nil
	}
	if latin, ok := abbreviationLatinForms[lower]; ok {
		// Already-Latin spelling of a known abbreviation (e.g. "ott" for
		// "ОТТ") — canonicalize to the abbreviation's uppercase form rather
		// than title-casing it like an ordinary passthrough word.
		return resolvedPart{text: latin, isAbbrev: true}, nil
	}
	if isASCII(part) {
		return resolvedPart{text: part, isPassthrough: true}, nil
	}
	return resolvedPart{}, &UnknownTermError{Term: part, Source: source}
}

// titleCasePassthroughWord upper-cases the first rune of an ASCII
// passthrough word and lower-cases the rest, so a passthrough part like "ac"
// or "gate" joins cleanly into PascalCase/camelCase output (contract §4
// step 4: "приводятся к общей конвенции регистра"). Parts that are already
// all-uppercase acronym-looking ASCII (e.g. "v1", "dto") are treated the
// same way: first letter up, remainder down — matching the plain-word
// convention, since the contract does not carve out a separate rule for
// ASCII acronyms beyond "passthrough, cased to the shared convention".
func titleCaseWord(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	head := strings.ToUpper(string(r[0]))
	tail := strings.ToLower(string(r[1:]))
	return head + tail
}

// resolveParts splits and resolves every part of name, returning an error
// naming the exact unrecognized part (and the original name) on the first
// failure — per contract §4 step 6, no partial/best-effort result is ever
// returned alongside an error.
func resolveParts(name string) ([]resolvedPart, error) {
	parts := splitParts(name)
	out := make([]resolvedPart, 0, len(parts))
	for _, p := range parts {
		rp, err := resolvePart(p, name)
		if err != nil {
			return nil, err
		}
		out = append(out, rp)
	}
	return out, nil
}

// joinParts renders resolved parts in PascalCase (firstUpper=true) or
// camelCase (firstUpper=false), preserving the original word order (contract
// §4 step 5 — never reordered). Abbreviation parts are emitted verbatim
// (already uppercase per §4.2); glossary/passthrough parts are title-cased
// per word, except the very first word of a camelCase identifier, which is
// lower-cased (unless it is itself an abbreviation, which stays uppercase to
// remain visually recognizable, e.g. idFT's "id" lowercases but "FT" does
// not).
func joinParts(parts []resolvedPart, firstUpper bool) string {
	var b strings.Builder
	for i, p := range parts {
		switch {
		case p.isAbbrev:
			b.WriteString(p.text)
		case i == 0 && !firstUpper:
			b.WriteString(strings.ToLower(p.text))
		default:
			b.WriteString(titleCaseWord(p.text))
		}
	}
	return b.String()
}

// ToPascalCase converts a graph name (EntityType.slug, field.name, state
// name, etc.) into an exported Go identifier, per GEN-CODE-CONTRACT.md §4.
// Returns *UnknownTermError if any part of name is not in the abbreviation
// table, the glossary, or ASCII.
func ToPascalCase(name string) (string, error) {
	parts, err := resolveParts(name)
	if err != nil {
		return "", err
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("gocode: empty identifier from graph name %q", name)
	}
	return joinParts(parts, true), nil
}

// ToCamelCase converts a graph name into an unexported Go identifier
// (methods/local variables), per GEN-CODE-CONTRACT.md §4 step 5.
func ToCamelCase(name string) (string, error) {
	parts, err := resolveParts(name)
	if err != nil {
		return "", err
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("gocode: empty identifier from graph name %q", name)
	}
	return joinParts(parts, false), nil
}

// ToKebabCase converts a graph name into a lower-case, hyphen-joined string
// VALUE (not a Go identifier), per GEN-CODE-CONTRACT.md §4.3 and §1.1: used
// for enum-constant string values (lifecycle state names) and event strings
// in errors/tests, so no raw Cyrillic graph name ever reaches a generated
// .go file's string literals. Shares resolveParts with ToPascalCase/
// ToCamelCase — the same glossary/abbreviation lookup, so a change to either
// table moves the identifier and the value in lockstep, and identifier/value
// can never disagree. Unlike identifier casing, ALL parts (including
// abbreviations) are lower-cased here: for a string value, an abbreviation's
// upper-case convention doesn't carry the identifier-recognizability
// rationale steps 2/5 give it.
func ToKebabCase(name string) (string, error) {
	parts, err := resolveParts(name)
	if err != nil {
		return "", err
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("gocode: empty identifier from graph name %q", name)
	}
	words := make([]string, len(parts))
	for i, p := range parts {
		words[i] = strings.ToLower(p.text)
	}
	return strings.Join(words, "-"), nil
}
