package gate

import (
	"encoding/json"
	"fmt"
	"github.com/PHPCraftdream/HotamSpec/internal/localization"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
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
	ClauseIDs  []string                     `json:"clause_ids,omitempty"`
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
	caseDef.ClauseIDs = slices.Clone(a.Case.ClauseIDs)
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
	// PackagePath is the module-relative Go import path of the file that
	// declares the method; PackageImports maps import names to import paths.
	PackagePath     string
	PackageImports  map[string]string
	Phrases         ontology.LocalizedText
	PhrasePositions map[string]token.Position
	NotPhrases      ontology.LocalizedText
	NotPositions    map[string]token.Position
	Method          *ast.FuncDecl
	Position        token.Position
	DocPosition     token.Position
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
	constants          map[string]map[string]map[string]atomValueConstant
	fragments          map[string]ontology.LocalizedText
	normativeTexts     map[string][]normativeTextSource
}

func NewAtomSourceIndex(specRoot string) (*AtomSourceIndex, error) {
	return newAtomSourceIndex(specRoot, atomSourceOptions{})
}

// atomSourceOptions selects the atom source layout. The zero value is the
// consumer layout (spec/go.mod + spec/model); packages selects the root-module
// layout (§14): root go.mod, explicit recorder import path from the manifest,
// and the listed package directories instead of spec/model.
type atomSourceOptions struct {
	languages          []string
	defaultLanguage    string
	ruleCases          bool
	packages           []string
	recorderImportPath string
	// parseSource borrows invocation-owned ASTs when supplied by a session.
	parseSource func(path string) (*token.FileSet, *ast.File, error)
}

// NewAtomSourceIndexForGraph resolves sources using the graph's invocation-local
// language and conformance configuration. It never changes graph/process state.
func NewAtomSourceIndexForGraph(g *ontology.Graph) (*AtomSourceIndex, error) {
	if g == nil {
		return nil, fmt.Errorf("atom source graph is nil")
	}
	return newAtomSourceIndex(SpecRootForGraph(g), atomSourceOptions{
		languages:          g.Languages,
		defaultLanguage:    g.DefaultLanguage,
		ruleCases:          g.Conformance != nil && g.Conformance.RuleCases,
		packages:           g.SelfExecutingAtomPackages,
		recorderImportPath: g.AtomRecorderImportPath,
	})
}

func newAtomSourceIndex(specRoot string, opts atomSourceOptions) (*AtomSourceIndex, error) {
	specDir := filepath.Join(specRoot, "spec")
	baseDir := specDir
	walkRoots := []string{filepath.Join(specDir, "model")}
	moduleGoMod := filepath.Join(specDir, "go.mod")
	recorderImportPath := ""
	if len(opts.packages) > 0 {
		// Root-module layout: go.mod at the repository root, subjects walked
		// in the listed package directories relative to it, recorder import
		// path taken verbatim from the manifest.
		moduleGoMod = filepath.Join(specRoot, "go.mod")
		baseDir = specRoot
		walkRoots = walkRoots[:0]
		for _, pkg := range opts.packages {
			walkRoots = append(walkRoots, filepath.Join(specRoot, filepath.FromSlash(pkg)))
		}
		recorderImportPath = opts.recorderImportPath
		if recorderImportPath == "" {
			return nil, fmt.Errorf("atom source module: self_executing_atom_packages requires atom_recorder_import_path in the manifest")
		}
	}
	data, err := os.ReadFile(moduleGoMod)
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
	// Consumer layout derives the recorder path from the module.
	if len(opts.packages) == 0 {
		recorderImportPath = modulePath + "/hotamspec"
	}
	languages := opts.languages
	defaultLanguage := opts.defaultLanguage
	ruleCases := opts.ruleCases
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
		RecorderImportPath: recorderImportPath,
		languages:          langs, defaultLanguage: defaultLanguage, ruleCases: ruleCases,
		constants:      map[string]map[string]map[string]atomValueConstant{},
		fragments:      map[string]ontology.LocalizedText{},
		normativeTexts: map[string][]normativeTextSource{},
	}
	parseSource := opts.parseSource
	if parseSource == nil {
		parseSource = func(path string) (*token.FileSet, *ast.File, error) {
			fs := token.NewFileSet()
			file, err := parser.ParseFile(fs, path, nil, parser.ParseComments)
			return fs, file, err
		}
	}
	visit := func(walkRoot string) error {
		return filepath.WalkDir(walkRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fs, f, err := parseSource(path)
			if err != nil {
				return err
			}
			dir, err := filepath.Rel(baseDir, filepath.Dir(path))
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
				if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.CONST {
					for _, spec := range gd.Specs {
						vs, ok := spec.(*ast.ValueSpec)
						if !ok || len(vs.Values) != 1 {
							continue
						}
						typeName := ""
						if id, ok := vs.Type.(*ast.Ident); ok {
							typeName = id.Name
						}
						lit, ok := vs.Values[0].(*ast.BasicLit)
						if typeName == "" || !ok {
							continue
						}
						value := lit.Value
						isString := lit.Kind == token.STRING
						if isString {
							unquoted, err := strconv.Unquote(value)
							if err != nil {
								continue
							}
							value = unquoted
						}
						doc := vs.Doc
						if doc == nil && gd.Lparen == token.NoPos {
							doc = gd.Doc
						}
						name := ""
						if len(vs.Names) > 0 {
							name = vs.Names[0].Name
						}
						translations, verbatim, err := parseAtomValueDoc(name, rel, fs.Position(vs.Pos()).Line, atomCommentLines(doc, fs), langs)
						if err != nil {
							return err
						}
						if !verbatim && len(translations) > 0 {
							fragmentSource := AtomSource{File: rel, Symbol: name, Position: fs.Position(vs.Pos())}
							if doc != nil {
								fragmentSource.DocPosition = fs.Position(doc.Pos())
							}
							texts, _, positions, _, _, err := parseAtomPhrases(doc, fs, fragmentSource, langs)
							if err != nil {
								return err
							}
							fragmentSource.PhrasePositions = positions
							index.fragments[filepath.ToSlash(rel)+":"+name] = texts
							index.indexNormativeText(filepath.ToSlash(rel)+":"+name, texts, fragmentSource)
						}
						if index.constants[packagePath] == nil {
							index.constants[packagePath] = map[string]map[string]atomValueConstant{}
						}
						if index.constants[packagePath][typeName] == nil {
							index.constants[packagePath][typeName] = map[string]atomValueConstant{}
						}
						index.constants[packagePath][typeName][value] = atomValueConstant{name: name, translations: translations, verbatim: verbatim, isString: isString, position: fs.Position(vs.Pos())}
					}
					continue
				}
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
					PackagePath: packagePath,
				}
				if len(f.Imports) > 0 {
					source.PackageImports = map[string]string{}
					for _, imp := range f.Imports {
						path, err := strconv.Unquote(imp.Path.Value)
						if err != nil {
							continue
						}
						name := path[strings.LastIndex(path, "/")+1:]
						if imp.Name != nil && imp.Name.Name != "_" && imp.Name.Name != "." {
							name = imp.Name.Name
						}
						source.PackageImports[name] = path
					}
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
	}
	for _, walkRoot := range walkRoots {
		if err := visit(walkRoot); err != nil {
			return nil, fmt.Errorf("atom sources: %w", err)
		}
	}
	if err := index.expandNormativeText(); err != nil {
		return nil, err
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
// before its package is executed. Non-method links (package-level invariant
// funcs, §14) are skipped: the index only resolves the listed atom packages.
func (index *AtomSourceIndex) ValidatePhraseLanguages(links []string) error {
	for _, link := range links {
		if !atomLinkShaped(link) {
			continue
		}
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
