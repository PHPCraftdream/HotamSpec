package gate

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/PHPCraftdream/HotamSpec/internal/localization"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// AtomStep identifies an executed method and its rendered value.
type AtomStep struct {
	Subject string `json:"subject"`
	Value   string `json:"value"`
}
type AtomArtifact struct {
	ReqID        string                  `json:"req_id"`
	Test         string                  `json:"test"`
	Mode         string                  `json:"mode"`
	Title        string                  `json:"title"`
	Verdict      string                  `json:"verdict"`
	Steps        []AtomStep              `json:"steps"`
	Case         *AtomCaseContext        `json:"case,omitempty"`
	CaseInput    *ontology.ObservedValue `json:"case_input,omitempty"`
	CaseExpected *ontology.ObservedValue `json:"case_expected,omitempty"`
}

// AtomCaseContext mirrors the recorder's case descriptor root. Case Test is
// resolved from the real test identity; CaseInput and CaseExpected are the
// authored WithInput and independent want/Expect projections, never actual.
type AtomCaseContext struct {
	ID         string                       `json:"id"`
	AtomIDs    []string                     `json:"atom_ids,omitempty"`
	Profile    string                       `json:"profile,omitempty"`
	Target     string                       `json:"target,omitempty"`
	Operation  string                       `json:"operation,omitempty"`
	Producer   string                       `json:"producer,omitempty"`
	Fixtures   []ontology.FixtureRef        `json:"fixtures,omitempty"`
	Conditions []ontology.ConditionEvidence `json:"conditions,omitempty"`
	Sides      []string                     `json:"sides,omitempty"`
	Selection  *ontology.SelectionEvidence  `json:"selection,omitempty"`
}

func (a AtomArtifact) CaseDefinition(test string) (*ontology.CaseDefinition, error) {
	if a.Mode != "rule" {
		return nil, nil
	}
	if a.Case == nil || strings.TrimSpace(a.Case.ID) == "" {
		return nil, fmt.Errorf("rule atom %s has no case ID", a.ReqID)
	}
	if strings.TrimSpace(test) == "" {
		return nil, fmt.Errorf("rule atom %s has no executed test address", a.ReqID)
	}
	caseDef := new(ontology.CaseDefinition)
	caseDef.ID = a.Case.ID
	caseDef.Test = test
	caseDef.AtomIDs = append([]string(nil), a.Case.AtomIDs...)
	caseDef.Profile = a.Case.Profile
	caseDef.Target = a.Case.Target
	caseDef.Operation = a.Case.Operation
	caseDef.Producer = a.Case.Producer
	caseDef.Input = a.CaseInput
	caseDef.Expected = a.CaseExpected
	caseDef.Fixtures = append([]ontology.FixtureRef(nil), a.Case.Fixtures...)
	caseDef.Conditions = append([]ontology.ConditionEvidence(nil), a.Case.Conditions...)
	caseDef.Sides = append([]string(nil), a.Case.Sides...)
	caseDef.Selection = a.Case.Selection
	return caseDef, nil
}

type AtomSource struct {
	File, Symbol, Phrase string
	Phrases              ontology.LocalizedText
	PhrasePositions      map[string]token.Position
	NotPhrases           ontology.LocalizedText
	NotPositions         map[string]token.Position
	Method               *ast.FuncDecl
	Position             token.Position
	DocPosition          token.Position
}

func (s AtomSource) Link() string { return s.File + ":" + s.Symbol }
func DecodeAtomArtifact(raw []byte) (AtomArtifact, error) {
	var a AtomArtifact
	err := json.Unmarshal(raw, &a)
	return a, err
}

// AtomSourceIndex is an invocation-local AST snapshot, never a verdict cache.
// Full import paths distinguish recursive packages; abbreviated package names
// are accepted only when they identify exactly one method.
type AtomSourceIndex struct {
	full               map[string][]AtomSource
	short              map[string][]AtomSource
	byLink             map[string][]AtomSource
	RecorderImportPath string
	languages          []string
	defaultLanguage    string
	ruleCases          bool
}

func NewAtomSourceIndex(specRoot string) (*AtomSourceIndex, error) {
	return newAtomSourceIndex(specRoot, nil, "", false)
}

// NewAtomSourceIndexForGraph resolves sources using the graph's invocation-local
// language and conformance configuration. It never changes graph/process state.
func NewAtomSourceIndexForGraph(g *ontology.Graph) (*AtomSourceIndex, error) {
	if g == nil {
		return nil, fmt.Errorf("atom source graph is nil")
	}
	ruleCases := g.Conformance != nil && g.Conformance.RuleCases
	return newAtomSourceIndex(SpecRootForGraph(g), g.Languages, g.DefaultLanguage, ruleCases)
}

func newAtomSourceIndex(specRoot string, languages []string, defaultLanguage string, ruleCases bool) (*AtomSourceIndex, error) {
	specDir := filepath.Join(specRoot, "spec")
	data, err := os.ReadFile(filepath.Join(specDir, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("atom source module: %w", err)
	}
	modulePath := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			modulePath = strings.Trim(fields[1], "\"")
			break
		}
	}
	if modulePath == "" {
		return nil, fmt.Errorf("atom source module: go.mod has no module directive")
	}
	langs := append([]string(nil), languages...)
	if len(langs) > 1 && defaultLanguage == "" {
		return nil, fmt.Errorf("atom source languages require a default language")
	}
	for _, language := range langs {
		if !localization.Supported(language) {
			return nil, fmt.Errorf("atom source language %q has no complete service catalog", language)
		}
	}
	if defaultLanguage != "" {
		found := false
		for _, language := range langs {
			found = found || language == defaultLanguage
		}
		if !found {
			return nil, fmt.Errorf("atom source default language %q is not declared", defaultLanguage)
		}
	}
	index := &AtomSourceIndex{
		full: map[string][]AtomSource{}, short: map[string][]AtomSource{},
		byLink:             map[string][]AtomSource{},
		RecorderImportPath: modulePath + "/hotamspec",
		languages:          langs, defaultLanguage: defaultLanguage, ruleCases: ruleCases,
	}
	fs := token.NewFileSet()
	err = filepath.WalkDir(filepath.Join(specDir, "model"), func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fs, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		dir, err := filepath.Rel(specDir, filepath.Dir(path))
		if err != nil {
			return err
		}
		packagePath := modulePath
		if dir != "." {
			packagePath += "/" + filepath.ToSlash(dir)
		}
		rel, err := filepath.Rel(specRoot, path)
		if err != nil {
			return err
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			typ := fn.Recv.List[0].Type
			if p, ok := typ.(*ast.StarExpr); ok {
				typ = p.X
			}
			if p, ok := typ.(*ast.IndexExpr); ok {
				typ = p.X
			}
			if p, ok := typ.(*ast.IndexListExpr); ok {
				typ = p.X
			}
			id, ok := typ.(*ast.Ident)
			if !ok {
				continue
			}
			source := AtomSource{
				File: filepath.ToSlash(rel), Symbol: id.Name + "." + fn.Name.Name,
				Method: fn, Position: fs.Position(fn.Pos()),
			}
			if fn.Doc != nil {
				source.DocPosition = fs.Position(fn.Doc.Pos())
			}
			source.Phrases, source.Phrase, source.PhrasePositions, source.NotPhrases, source.NotPositions, err = parseAtomPhrases(fn.Doc, fs, source, langs)
			if err != nil {
				return err
			}
			primaryLanguage := defaultLanguage
			if primaryLanguage == "" && len(langs) == 1 {
				primaryLanguage = langs[0]
			}
			if primaryLanguage != "" {
				source.Phrase = source.Phrases[primaryLanguage]
			}
			full := packagePath + "." + source.Symbol
			short := f.Name.Name + "." + source.Symbol
			index.full[full] = append(index.full[full], source)
			index.short[short] = append(index.short[short], source)
			link := source.Link()
			index.byLink[link] = append(index.byLink[link], source)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("atom sources: %w", err)
	}
	return index, nil
}
func (index *AtomSourceIndex) Resolve(subject string) (AtomSource, error) {
	var found []AtomSource
	if strings.Contains(subject, "/") {
		found = index.full[subject]
	} else {
		found = index.short[subject]
	}
	if len(found) != 1 {
		return AtomSource{}, fmt.Errorf("atom subject %q resolves to %d methods", subject, len(found))
	}
	return found[0], nil
}

// ResolveLink finds a method by its domain-relative implemented_by link.
func (index *AtomSourceIndex) ResolveLink(link string) (AtomSource, error) {
	found := index.byLink[strings.TrimSpace(link)]
	if len(found) != 1 {
		return AtomSource{}, fmt.Errorf("atom source link %q resolves to %d methods", link, len(found))
	}
	return found[0], nil
}

// ValidatePhraseLanguages checks all source methods used by a requirement
// before its package is executed.
func (index *AtomSourceIndex) ValidatePhraseLanguages(links []string) error {
	for _, link := range links {
		source, err := index.ResolveLink(link)
		if err != nil {
			return err
		}
		for _, language := range index.claimLanguages() {
			if phrase, ok := source.Phrases[language]; !ok || strings.TrimSpace(phrase) == "" {
				return atomPhraseError(source, language, "missing non-empty source phrase")
			}
		}
	}
	return nil
}

// ResolveAtomSubject is convenient for a single lookup. Batch callers should
// build one AtomSourceIndex and reuse Resolve and DeriveClaim.
func ResolveAtomSubject(specRoot, subject string) (AtomSource, error) {
	index, err := NewAtomSourceIndex(specRoot)
	if err != nil {
		return AtomSource{}, err
	}
	return index.Resolve(subject)
}

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

// returnsBool reports whether the subject method returns exactly one bare
// `bool`. Aliases (`type Bit bool`) are intentionally not recognized: typed
// values from the recorder are absent for plain fact/holds steps, so the AST
// return type is the only reliable bool signal.
func (s AtomSource) returnsBool() bool {
	if s.Method == nil || s.Method.Type == nil || s.Method.Type.Results == nil || len(s.Method.Type.Results.List) != 1 {
		return false
	}
	field := s.Method.Type.Results.List[0]
	if len(field.Names) > 1 {
		return false
	}
	ident, ok := field.Type.(*ast.Ident)
	return ok && ident.Name == "bool"
}

// atomPhraseText renders a fact/holds step: a bool method renders the bare
// phrase on "true" and needs an authored `not:` negation on "false", other
// methods render "phrase — value.".
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
// an error, never an auto-generated "не ..." wording.
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
		for _, language := range languages {
			phrase, ok := source.Phrases[language]
			if !ok || strings.TrimSpace(phrase) == "" {
				return nil, atomPhraseError(source, language, "missing non-empty source phrase")
			}
			notPhrase := strings.TrimSpace(source.NotPhrases[language])
			if boolMethod && step.Value == "false" && notPhrase == "" {
				return nil, atomPhraseError(source, language, "bool atom evaluated to false without a `not:` negation phrase")
			}
			part, err := atomPhraseText(language, phrase, notPhrase, step.Value, a.Mode != "rule", boolMethod)
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
