package gate

import (
	"fmt"
	"github.com/PHPCraftdream/HotamSpec/internal/localization"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"go/ast"
	"go/token"
	"strings"
	"unicode"
)

// atom_source_phrases.go parses localized doc-comment phrases (including value-atom value docs) from atom source files.
type atomDocLine struct {
	text string
	pos  token.Position
}

func atomCommentLines(group *ast.CommentGroup, fs *token.FileSet) []atomDocLine {
	if group == nil {
		return nil
	}
	var lines []atomDocLine
	for _, comment := range group.List {
		text := comment.Text
		switch {
		case strings.HasPrefix(text, "//"):
			text = strings.TrimPrefix(text, "//")
			text = strings.TrimPrefix(text, " ")
			lines = append(lines, atomDocLine{text: text, pos: fs.Position(comment.Pos())})
		case strings.HasPrefix(text, "/*"):
			text = strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/")
			base := fs.Position(comment.Pos())
			for offset, line := range strings.Split(text, "\n") {
				line = strings.TrimSpace(line)
				line = strings.TrimPrefix(line, "*")
				line = strings.TrimSpace(line)
				position := base
				position.Line += offset
				lines = append(lines, atomDocLine{text: line, pos: position})
			}
		}
	}
	return lines
}

func atomDocError(source AtomSource, line int, language, reason string) error {
	where := source.DocPosition
	if where.Filename == "" {
		where = source.Position
	}
	if line > 0 {
		where.Line = line
	}
	if where.Filename == "" {
		where.Filename = source.File
	}
	if language == "" {
		return fmt.Errorf("%s:%d: %s: %s", where.Filename, where.Line, source.Symbol, reason)
	}
	return fmt.Errorf("%s:%d: %s language %q: %s", where.Filename, where.Line, source.Symbol, language, reason)
}

func atomPhraseError(source AtomSource, language, reason string) error {
	position, ok := source.PhrasePositions[language]
	if !ok {
		position = source.PhrasePositions[""]
	}
	if position.Filename != "" {
		source.DocPosition = position
	}
	return atomDocError(source, position.Line, language, reason)
}

func parseAtomPhrases(group *ast.CommentGroup, fs *token.FileSet, source AtomSource, languages []string) (ontology.LocalizedText, string, map[string]token.Position, ontology.LocalizedText, map[string]token.Position, error) {
	if group == nil {
		return nil, "", nil, nil, nil, nil
	}
	docText := strings.TrimSpace(strings.Split(group.Text(), "\n")[0])
	lines := atomCommentLines(group, fs)
	hasMarker := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line.text), ">>>>>") {
			hasMarker = true
			break
		}
	}
	if len(languages) < 2 {
		if hasMarker {
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line.text), ">>>>>") {
					return nil, "", nil, nil, nil, atomDocError(source, line.pos.Line, "", "language markers require a multilingual languages configuration")
				}
			}
		}
		language := ""
		if len(languages) == 1 {
			language = languages[0]
		}
		phrases, positions, notPhrases, notPositions, err := plainPhrases(lines, source, docText, language)
		if err != nil {
			return nil, "", nil, nil, nil, err
		}
		return phrases, docText, positions, notPhrases, notPositions, nil
	}
	if !hasMarker {
		phrases, positions, notPhrases, notPositions, err := plainPhrases(lines, source, docText, "")
		if err != nil {
			return nil, "", nil, nil, nil, err
		}
		return phrases, docText, positions, notPhrases, notPositions, nil
	}

	declared := make(map[string]bool, len(languages))
	for _, language := range languages {
		declared[language] = true
	}
	phrases := make(ontology.LocalizedText, len(languages))
	positions := make(map[string]token.Position, len(languages))
	notPhrases := make(ontology.LocalizedText, len(languages))
	notPositions := make(map[string]token.Position, len(languages))
	currentLanguage := ""
	markerLine := 0
	var block []atomDocLine
	finishBlock := func() error {
		if currentLanguage == "" {
			return nil
		}
		var content []string
		var phrasePosition token.Position
		var notPhrase string
		var notPosition token.Position
		endedParagraph := false
		for _, line := range block {
			text := strings.TrimSpace(line.text)
			if text == "" {
				if len(content) > 0 {
					endedParagraph = true
				}
				continue
			}
			if strings.HasPrefix(line.text, "not:") {
				if notPosition.Filename != "" {
					return atomDocError(source, line.pos.Line, currentLanguage, "duplicate `not:` negation phrase")
				}
				if phrasePosition.Filename == "" {
					return atomDocError(source, line.pos.Line, currentLanguage, "`not:` negation phrase must follow the main phrase")
				}
				not := strings.TrimSpace(strings.TrimPrefix(line.text, "not:"))
				if not == "" {
					return atomDocError(source, line.pos.Line, currentLanguage, "empty `not:` negation phrase")
				}
				notPhrase, notPosition = not, line.pos
				continue
			}
			if endedParagraph || notPosition.Filename != "" {
				return atomDocError(source, line.pos.Line, currentLanguage, "language block must contain one short phrase")
			}
			if phrasePosition.Filename == "" {
				phrasePosition = line.pos
			}
			content = append(content, text)
		}
		phrase := strings.Join(content, " ")
		if phrase == "" {
			return atomDocError(source, markerLine, currentLanguage, "language block is empty")
		}
		phrases[currentLanguage] = phrase
		positions[currentLanguage] = phrasePosition
		if notPhrase != "" {
			notPhrases[currentLanguage], notPositions[currentLanguage] = notPhrase, notPosition
		}
		return nil
	}
	for _, line := range lines {
		text := strings.TrimSpace(line.text)
		if strings.HasPrefix(text, ">>>>>") {
			if err := finishBlock(); err != nil {
				return nil, "", nil, nil, nil, err
			}
			if !strings.HasPrefix(text, ">>>>> lang=") {
				return nil, "", nil, nil, nil, atomDocError(source, line.pos.Line, "", "invalid marker; expected exactly `>>>>> lang=<code>`")
			}
			language := strings.TrimPrefix(text, ">>>>> lang=")
			if language == "" || strings.TrimSpace(language) != language || strings.ContainsAny(language, " \t/\\<>") {
				return nil, "", nil, nil, nil, atomDocError(source, line.pos.Line, language, "invalid language code in marker")
			}
			if !declared[language] {
				return nil, "", nil, nil, nil, atomDocError(source, line.pos.Line, language, "language marker is not declared in manifest")
			}
			if _, duplicate := phrases[language]; duplicate || currentLanguage == language {
				return nil, "", nil, nil, nil, atomDocError(source, line.pos.Line, language, "duplicate language block")
			}
			currentLanguage, markerLine, block = language, line.pos.Line, nil
			continue
		}
		if currentLanguage == "" {
			if text != "" {
				return nil, "", nil, nil, nil, atomDocError(source, line.pos.Line, "", "text outside language blocks")
			}
			continue
		}
		block = append(block, line)
	}
	if err := finishBlock(); err != nil {
		return nil, "", nil, nil, nil, err
	}
	for _, language := range languages {
		if _, ok := phrases[language]; !ok {
			return nil, "", nil, nil, nil, atomDocError(source, source.DocPosition.Line, language, "missing language block")
		}
	}
	if len(phrases) != len(languages) {
		return nil, "", nil, nil, nil, atomDocError(source, source.DocPosition.Line, "", "language blocks do not match declared languages")
	}
	return phrases, phrases[languages[0]], positions, notPhrases, notPositions, nil
}

// plainPhrases splits an unmarked doc into the main phrase and an optional
// following `not:` negation line.
func plainPhrases(lines []atomDocLine, source AtomSource, docText, language string) (ontology.LocalizedText, map[string]token.Position, ontology.LocalizedText, map[string]token.Position, error) {
	positions := make(map[string]token.Position, 1)
	notPositions := make(map[string]token.Position, 1)
	var notPhrase string
	phraseSeen := false
	for _, line := range lines {
		text := strings.TrimSpace(line.text)
		if text == "" {
			continue
		}
		if !phraseSeen {
			if strings.HasPrefix(line.text, "not:") {
				return nil, nil, nil, nil, atomDocError(source, line.pos.Line, language, "`not:` negation phrase must follow the main phrase")
			}
			phraseSeen = true
			positions[language] = line.pos
			continue
		}
		if !strings.HasPrefix(line.text, "not:") {
			continue
		}
		if notPhrase != "" {
			return nil, nil, nil, nil, atomDocError(source, line.pos.Line, language, "duplicate `not:` negation phrase")
		}
		not := strings.TrimSpace(strings.TrimPrefix(line.text, "not:"))
		if not == "" {
			return nil, nil, nil, nil, atomDocError(source, line.pos.Line, language, "empty `not:` negation phrase")
		}
		notPhrase, notPositions[language] = not, line.pos
	}
	phrases := ontology.LocalizedText{language: docText}
	if notPhrase == "" {
		return phrases, positions, nil, nil, nil
	}
	return phrases, positions, ontology.LocalizedText{language: notPhrase}, notPositions, nil
}
func atomConstantError(file string, line int, name, language, reason string) error {
	if language != "" {
		return fmt.Errorf("%s:%d: constant %s language %q: %s", file, line, name, language, reason)
	}
	return fmt.Errorf("%s:%d: constant %s: %s", file, line, name, reason)
}

// parseAtomValueDoc parses a constant's doc comment for value translations.
// Markers follow the phrase syntax; `>>>>> lang=*` marks a verbatim value that
// is never translated. A plain doc yields no translations: in a multilingual
// domain a matched string constant without them is rejected at derive time.
func parseAtomValueDoc(name, file string, docLine int, lines []atomDocLine, languages []string) (ontology.LocalizedText, bool, error) {
	fail := func(line int, language, reason string) error {
		return atomConstantError(file, line, name, language, reason)
	}
	hasMarker := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line.text), ">>>>>") {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		return nil, false, nil
	}
	if len(languages) < 2 {
		return nil, false, fail(docLine, "", "language markers require a multilingual languages configuration")
	}
	declared := make(map[string]bool, len(languages))
	for _, language := range languages {
		declared[language] = true
	}
	translations := ontology.LocalizedText{}
	verbatim := false
	current := ""
	markerLine := 0
	var block []string
	finishBlock := func() error {
		if current == "" {
			return nil
		}
		if len(block) == 0 {
			return fail(markerLine, current, "language block is empty")
		}
		translations[current] = strings.Join(block, " ")
		return nil
	}
	for _, line := range lines {
		text := strings.TrimSpace(line.text)
		if !strings.HasPrefix(text, ">>>>>") {
			if current == "" {
				if text != "" {
					return nil, false, fail(line.pos.Line, "", "text outside language blocks")
				}
				continue
			}
			if text == "" {
				continue
			}
			if strings.HasPrefix(line.text, "not:") {
				return nil, false, fail(line.pos.Line, current, "`not:` negation phrases do not apply to constant values")
			}
			block = append(block, text)
			continue
		}
		if err := finishBlock(); err != nil {
			return nil, false, err
		}
		if !strings.HasPrefix(text, ">>>>> lang=") {
			return nil, false, fail(line.pos.Line, "", "invalid marker; expected exactly `>>>>> lang=<code>`")
		}
		language := strings.TrimPrefix(text, ">>>>> lang=")
		if language == "*" {
			if verbatim || len(translations) > 0 || current != "" {
				return nil, false, fail(line.pos.Line, "*", "verbatim `lang=*` marker must be the only language block")
			}
			verbatim, markerLine, current, block = true, line.pos.Line, "", nil
			continue
		}
		if verbatim {
			return nil, false, fail(line.pos.Line, language, "verbatim `lang=*` marker must be the only language block")
		}
		if language == "" || strings.TrimSpace(language) != language || strings.ContainsAny(language, " \t/\\<>") {
			return nil, false, fail(line.pos.Line, language, "invalid language code in marker")
		}
		if !declared[language] {
			return nil, false, fail(line.pos.Line, language, "language marker is not declared in manifest")
		}
		if _, duplicate := translations[language]; duplicate || current == language {
			return nil, false, fail(line.pos.Line, language, "duplicate language block")
		}
		current, markerLine, block = language, line.pos.Line, nil
	}
	if err := finishBlock(); err != nil {
		return nil, false, err
	}
	if verbatim {
		return nil, true, nil
	}
	for _, language := range languages {
		if _, ok := translations[language]; !ok {
			return nil, false, fail(docLine, language, "missing language block")
		}
	}
	if len(translations) != len(languages) {
		return nil, false, fail(docLine, "", "language blocks do not match declared languages")
	}
	return translations, false, nil
}
func normalizedAtomPhrase(phrase, language string) (string, error) {
	punctuation := ".!?;:… "
	if language == "zh" {
		punctuation = "。！？；：… "
	}
	phrase = strings.TrimRight(strings.TrimSpace(phrase), punctuation)
	if phrase == "" {
		return "", fmt.Errorf("atom method has no doc phrase")
	}
	if language == "zh" {
		return phrase, nil
	}
	r := []rune(phrase)
	r[0] = unicode.ToUpper(r[0])
	return string(r), nil
}

func atomText(language, template string, args ...any) (string, error) {
	if language == "" {
		return fmt.Sprintf(template, args...), nil
	}
	localized, err := localization.Lookup(language, template)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(localized, args...), nil
}

func atomPhraseText(language, phrase, notPhrase, value string, hasValue, boolMethod bool) (string, error) {
	normalized, err := normalizedAtomPhrase(phrase, language)
	if err != nil {
		return "", err
	}
	if hasValue && boolMethod && value == "true" {
		if language == "" {
			return normalized + ".", nil
		}
		return atomText(language, "%s.", normalized)
	}
	if hasValue && boolMethod && value == "false" {
		not, err := normalizedAtomPhrase(notPhrase, language)
		if err != nil {
			return "", fmt.Errorf("bool atom evaluated to false without a `not:` negation phrase")
		}
		if language == "" {
			return not + ".", nil
		}
		return atomText(language, "%s.", not)
	}
	if hasValue {
		if language == "" {
			return normalized + " — " + value + ".", nil
		}
		return atomText(language, "%s — %s.", normalized, value)
	}
	if language == "" {
		return normalized + ".", nil
	}
	return atomText(language, "%s.", normalized)
}

var legacyAtomLanguages = [...]string{""}
