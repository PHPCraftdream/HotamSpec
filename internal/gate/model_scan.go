// model_scan.go holds the shared AST model scan BOTH internal/generator's
// MODELS.md/COVERAGE.md rendering (BuildModels/ScanModelLayerCounts) AND
// internal/invariants' model-level discipline gate (check_model_complete,
// PLAN-scenario-generated-spec.md §2 D5 / §3 W2.4) call, so the two layers
// never disagree about which authored Go declarations count as "this
// domain's own object model".
//
// LAYERING (why this lives in internal/gate, not internal/generator, W2.4):
// this scan was originally internal/generator/models.go's unexported
// scanDomainModelFiles/scanSelfHostingModelFiles/parseModelFiles/
// isVendoredRecorderFile suite (task W1.3). Task W2.4 needed a model-level
// check (check_model_complete) living in internal/invariants -- but
// internal/invariants must never import internal/generator (a real import
// cycle: internal/generator -> internal/diagnose -> internal/invariants
// already exists, and internal/generator's own fixture_test.go imports
// internal/invariants directly -- see internal/gate/spec_build.go's own
// LAYERING doc comment for the full reasoning, which this file follows
// verbatim as the established W2.3 precedent). internal/gate is a true leaf
// both internal/generator (models.go's own former self already depended on
// it for SpecRootForGraph/ParseFileColonSymbol) and internal/invariants
// (authored_links.go, scenario_discipline.go, etc.) already depend on
// directly, and it already owns an identical receiverBaseTypeName helper
// (spec_resolver.go) this scan previously DUPLICATED in generator/models.go
// -- moving the scan here, once, lets check_model_complete
// (internal/invariants) and BuildModels/ScanModelLayerCounts
// (internal/generator, now thin wrappers) call the EXACT SAME code without
// either package importing the other, and RETIRES the duplicated
// receiverBaseTypeName copy generator/models.go carried (its own doc comment
// at the time explicitly flagged it as a locally-duplicated mirror of the
// gate helper "duplicated locally rather than exported across the package
// boundary").
//
// This file is read-only over both the graph and the filesystem: it parses
// authored Go source with go/parser purely for a structural inventory (type
// names, field names+types, method signatures, `var Err*` declarations). It
// never executes, type-checks, or mutates anything it reads, and it is not
// an enforcement gate -- that role belongs to the mechanical checks in
// internal/invariants; this scan only inventories declarations.
package gate

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	ontologyvendor "github.com/PHPCraftdream/HotamSpec/internal/ontology/vendor"
	recordervendor "github.com/PHPCraftdream/HotamSpec/internal/recorder/vendor"
	registrydumpvendor "github.com/PHPCraftdream/HotamSpec/internal/registrydump/vendor"
)

// ModelObject is one rendered type declaration: its name, kind (struct/
// interface/other), fields (struct only), and methods (functions with this
// type as receiver, matched by base type name so both pointer and value
// receivers collapse onto the same object).
type ModelObject struct {
	Name    string
	Kind    string // "struct" | "interface" | "type"
	Doc     string
	Fields  []ModelField
	Methods []ModelMethod
}

// ModelField is one declared field of a struct-typed ModelObject (one entry
// per declared name; embedded and multi-name declarations expand).
type ModelField struct {
	Name string
	Typ  string
}

// ModelMethod is one method declared with a ModelObject's type as receiver
// (pointer or value, both collapse onto the same object by base type name).
type ModelMethod struct {
	Receiver  string // e.g. "*Risk" or "Risk"
	Name      string
	Signature string // full "func (r *Risk) Validate(...) error"-shaped rendering
	Doc       string
}

// ModelError is one `var Err... = errors.New(...)`-shaped typed sentinel
// error declaration.
type ModelError struct {
	Name string
	Doc  string
}

// ModelFile is one parsed authored Go source file's extracted inventory,
// keyed by its path relative to the scan root (specRoot for an ordinary
// domain, engineRoot for self-hosting) so callers can group by file and
// sort deterministically.
type ModelFile struct {
	RelPath string
	Pkg     string
	Objects []ModelObject
	Errors  []ModelError
}

// ScanAuthoredModels runs the SAME source selection BuildModels/ScanModelLayerCounts
// (internal/generator) use to render MODELS.md/COVERAGE.md -- an ordinary
// (non-self-hosting) domain is scanned by walking its authored spec/ tree
// on disk (SpecRootForGraph(g)/spec/.../*.go except *_test.go); a
// self-hosting domain is scanned by resolving the focused engine-file slice
// named by implemented_by/verified_by entries -- and returns the parsed
// ModelFile inventory directly. Returns an empty (nil) slice, not an error,
// when spec/ does not exist yet or no authored links name engine files yet
// (a domain that has not reached founding step 3, PLAN §8) -- a calm,
// expected state, not a scan failure.
//
// Single shared entry point for both generators (rendering) and the
// model-level discipline gate (check_model_complete): everything that needs
// to know "what objects/fields/methods does this domain's authored spec/
// declare, EXCLUDING every known vendored/generated file" funnels through
// here, so MODELS.md/COVERAGE.md and the check_model_complete violation set
// can never disagree about which files count as "this domain's own models"
// (the same IsGeneratedOrVendoredFile choke point every path funnels
// through -- see that function's doc comment for the full registry of
// recognized banners).
func ScanAuthoredModels(g *ontology.Graph) ([]ModelFile, error) {
	if g.SelfHosting {
		return scanSelfHostingModelFiles(g)
	}
	return scanDomainModelFiles(g)
}

// scanDomainModelFiles walks an ordinary (non-self-hosting) domain's
// authored spec/ tree on disk -- SpecRootForGraph(g)/spec and any sibling
// *.go files under it (excluding *_test.go) -- and parses each into a
// ModelFile. Returns an empty (nil) slice, not an error, when spec/ does
// not exist yet (a domain that has not reached founding step 3 yet, PLAN
// §8) -- that is a normal, calm state, not a scan failure.
func scanDomainModelFiles(g *ontology.Graph) ([]ModelFile, error) {
	specRoot := SpecRootForGraph(g)
	specDir := filepath.Join(specRoot, "spec")

	var goFiles []string
	err := filepath.WalkDir(specDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		goFiles = append(goFiles, path)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(goFiles) == 0 {
		return nil, nil
	}

	return parseModelFiles(goFiles, specRoot)
}

// scanSelfHostingModelFiles resolves the focused engine-file slice for a
// self-hosting domain: every distinct file (deduplicated, sorted) named by
// an implemented_by or verified_by entry on this domain's own requirements,
// resolved against SpecRootForGraph(g) -- the same engine-root resolution
// internal/invariants/authored_links.go uses -- so the inventory shows
// exactly the engine types the discipline's own authored links point at,
// never the whole internal/ tree.
func scanSelfHostingModelFiles(g *ontology.Graph) ([]ModelFile, error) {
	specRoot := SpecRootForGraph(g)

	relSet := map[string]struct{}{}
	for _, r := range g.Requirements {
		for _, entry := range append(append([]string{}, r.ImplementedBy...), r.VerifiedBy...) {
			file, _, ok := ParseFileColonSymbol(strings.TrimSpace(entry))
			if !ok {
				continue
			}
			relSet[file] = struct{}{}
		}
	}
	if len(relSet) == 0 {
		return nil, nil
	}

	rels := make([]string, 0, len(relSet))
	for rel := range relSet {
		rels = append(rels, rel)
	}
	sort.Strings(rels)

	paths := make([]string, len(rels))
	for i, rel := range rels {
		paths[i] = filepath.Join(specRoot, filepath.FromSlash(rel))
	}

	return parseModelFiles(paths, specRoot)
}

// parseModelFiles parses each absolute path in paths (deduplicated,
// re-sorted by root-relative slash path for determinism) into a ModelFile,
// skipping any file that fails to parse or no longer exists rather than
// failing the whole scan (a stale implemented_by reference is already
// reported as ORPHANED by TRACEABILITY.md; the scan simply omits it) --
// and skipping any file stamped with a known generated/vendored do-not-edit
// banner (IsGeneratedOrVendoredFile), since such a file is engine machinery
// copied or templated into the domain's spec/ tree for a Go module boundary
// reason (PLAN-scenario-generated-spec.md §2 D1's vendoring contract), never
// a domain-authored model -- see IsGeneratedOrVendoredFile's doc comment for
// why this is the single, shared choke point BOTH scanDomainModelFiles and
// scanSelfHostingModelFiles funnel through, so every caller of
// ScanAuthoredModels never disagrees about which files count as "this
// domain's own models".
func parseModelFiles(paths []string, root string) ([]ModelFile, error) {
	seen := map[string]struct{}{}
	var uniquePaths []string
	for _, p := range paths {
		clean := filepath.Clean(p)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		uniquePaths = append(uniquePaths, clean)
	}

	var files []ModelFile
	fset := token.NewFileSet()
	for _, p := range uniquePaths {
		if IsGeneratedOrVendoredFile(p) {
			continue
		}
		astFile, err := parser.ParseFile(fset, p, nil, parser.ParseComments)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			rel = p
		}
		rel = filepath.ToSlash(rel)
		files = append(files, extractModelFile(astFile, rel))
	}

	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	return files, nil
}

// knownGeneratedBannerFirstLines is the registry every
// IsGeneratedOrVendoredFile check runs against: the exact first line of
// EVERY known do-not-edit banner this engine stamps onto a file it writes
// into a consumer domain's spec/ tree, one entry per distinct banner
// producer. Each entry is DERIVED from that producer's own exported Banner
// constant (never a hand-copied literal), so a future wording change to any
// one of those banners cannot silently desync this registry from the marker
// it is meant to recognize -- the same non-duplication discipline the
// original single-purpose vendoredRecorderBannerFirstLine (now folded into
// this slice) already followed, generalized to N producers instead of one.
//
// Adding a FOURTH generated/vendored file kind later (a future `hotam
// vendor-*`/`hotam scaffold-*` command) is a one-line addition here --
// append that producer's own Banner first line -- never a new, parallel
// isVendoredXFile function: this is the single unified choke point every
// generated/vendored marker funnels through, so ScanAuthoredModels'
// definition of "domain-authored" never has to be re-taught per producer at
// each of MODELS.md/COVERAGE.md/check_model_complete separately.
//
// Current producers (task #391, W0.4 -- zero-trust finding: only the
// recorder had this exclusion; the ontology-vendor mirror
// (spec/hotamontology/, `hotam vendor-ontology`, task #365) and the
// registrydump scaffold (spec/registrydump/, `hotam scaffold-registrydump`,
// task #367) had none, so their own exported types -- Relation, Requirement,
// Registry -- would leak into MODELS.md/COVERAGE.md/check_model_complete as
// soon as a consumer domain adopted requirements_authority:"code"):
//
//   - internal/recorder/vendor.Banner       -- `hotam vendor-recorder`   (spec/hotamspec/)
//   - internal/ontology/vendor.Banner       -- `hotam vendor-ontology`   (spec/hotamontology/)
//   - internal/registrydump/vendor.Banner   -- `hotam scaffold-registrydump` (spec/registrydump/)
var knownGeneratedBannerFirstLines = []string{
	strings.SplitN(recordervendor.Banner, "\n", 2)[0],
	strings.SplitN(ontologyvendor.Banner, "\n", 2)[0],
	strings.SplitN(registrydumpvendor.Banner, "\n", 2)[0],
}

// IsGeneratedOrVendoredFile reports whether the Go source file at path
// carries one of knownGeneratedBannerFirstLines as its literal first line --
// i.e. it is a VENDORED or GENERATED copy this engine itself wrote into a
// consumer domain's spec/ tree (the recorder, the ontology mirror, or the
// registrydump scaffold) rather than a domain-authored model. Every caller
// of ScanAuthoredModels must never count such a file's own exported types
// (Scenario/Artifact/Step/StepKind from the recorder; Relation/Requirement/
// Registry from the ontology mirror; the registrydump scaffold's generated
// main) as domain object-model surface -- the original zero-trust review
// finding that motivated this check was a pilot's COVERAGE.md drifting from
// "3 files / 6 objects / 11 fields / 16 methods" to "4 / 14 / 32 / 24"
// purely because ONE vendored file (the recorder) got swept into the same
// spec/ walk as the domain's real model/ files; task #391 generalized the
// fix to every known banner producer, not just that first one.
//
// Detection is by each candidate file's OWN do-not-edit banner -- its first
// line must equal one entry of knownGeneratedBannerFirstLines EXACTLY --
// rather than by package name ("hotamspec"/"hotamontology") or directory
// name ("spec/hotamspec/"/"spec/hotamontology/"/"spec/registrydump/"): a
// package/directory name is a convention a domain could rename, but the
// banner is the file's own generated-marker, stamped by the corresponding
// producer on every run of its own `hotam vendor-*`/`hotam scaffold-*`
// command, so it survives a directory rename and cannot drift out of sync
// with what the writing tool itself writes. Only the FIRST LINE of each
// banner is checked (not a full-banner match) so this stays robust to
// whitespace/line-ending normalization elsewhere in the pipeline without
// weakening the signal -- no ordinary domain-authored file begins with any
// of these exact comment lines by accident (each first line names its own
// specific `hotam ...` command). A genuinely domain-authored file whose NAME
// merely resembles a vendored one (e.g. an authored spec/model/registry.go
// declaring the domain's own Registry type) is unaffected: its first line is
// whatever the author wrote, not one of these banners, so it is scanned
// normally.
//
// A file that cannot be opened, or whose first line cannot be read, is
// treated as NOT generated/vendored (false) -- consistent with
// parseModelFiles' own existing policy of skipping unreadable/unparsable
// files silently rather than failing the whole scan; a real read failure
// surfaces moments later anyway when parser.ParseFile is attempted on the
// same path.
func IsGeneratedOrVendoredFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return false
	}
	firstLine := scanner.Text()
	for _, known := range knownGeneratedBannerFirstLines {
		if firstLine == known {
			return true
		}
	}
	return false
}

// extractModelFile walks one parsed *ast.File's top-level declarations into
// a ModelFile: GenDecl(TYPE) -> ModelObject (struct fields extracted when
// the underlying type is *ast.StructType), FuncDecl with a receiver ->
// attached to the matching ModelObject by base receiver type name,
// GenDecl(VAR) whose spec name starts with "Err" -> ModelError. Everything
// is re-sorted (objects/methods/errors sorted by name; struct fields left
// declaration-ordered since field order is itself meaningful API surface)
// so output is deterministic regardless of source declaration order.
func extractModelFile(astFile *ast.File, relPath string) ModelFile {
	f := ModelFile{RelPath: relPath, Pkg: astFile.Name.Name}

	objByName := map[string]*ModelObject{}
	var objOrder []string

	for _, decl := range astFile.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		switch gd.Tok {
		case token.TYPE:
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !ts.Name.IsExported() {
					continue
				}
				obj := ModelObject{Name: ts.Name.Name, Doc: docText(gd.Doc)}
				if obj.Doc == "" {
					obj.Doc = docText(ts.Doc)
				}
				switch t := ts.Type.(type) {
				case *ast.StructType:
					obj.Kind = "struct"
					obj.Fields = extractFields(t)
				case *ast.InterfaceType:
					obj.Kind = "interface"
				default:
					obj.Kind = "type"
				}
				objByName[obj.Name] = &obj
				objOrder = append(objOrder, obj.Name)
			}
		case token.VAR:
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if !name.IsExported() || !strings.HasPrefix(name.Name, "Err") {
						continue
					}
					doc := docText(gd.Doc)
					if doc == "" && i == 0 {
						doc = docText(vs.Doc)
					}
					f.Errors = append(f.Errors, ModelError{Name: name.Name, Doc: doc})
				}
			}
		}
	}

	for _, decl := range astFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		baseName := receiverBaseTypeName(fn.Recv)
		obj, ok := objByName[baseName]
		if !ok {
			continue
		}
		recvStr := renderReceiverType(fn.Recv.List[0].Type)
		obj.Methods = append(obj.Methods, ModelMethod{
			Receiver:  recvStr,
			Name:      fn.Name.Name,
			Signature: renderFuncSignature(fn, recvStr),
			Doc:       docText(fn.Doc),
		})
	}

	for _, name := range objOrder {
		obj := objByName[name]
		sort.Slice(obj.Methods, func(i, j int) bool { return obj.Methods[i].Name < obj.Methods[j].Name })
		f.Objects = append(f.Objects, *obj)
	}
	sort.Slice(f.Objects, func(i, j int) bool { return f.Objects[i].Name < f.Objects[j].Name })
	sort.Slice(f.Errors, func(i, j int) bool { return f.Errors[i].Name < f.Errors[j].Name })

	return f
}

// extractFields renders a struct type's field list as ModelFields, one per
// declared name (an embedded field or a multi-name single declaration like
// `X, Y int` both expand to one ModelField per name). Unexported fields are
// included too -- the inventory is of the WHOLE shape a domain author wrote,
// not just its public API, since an authored spec/ model commonly keeps its
// invariant-carrying fields unexported on purpose.
func extractFields(t *ast.StructType) []ModelField {
	if t.Fields == nil {
		return nil
	}
	var fields []ModelField
	for _, field := range t.Fields.List {
		typeStr := exprToString(field.Type)
		if len(field.Names) == 0 {
			// Embedded field: name is the type's own base identifier.
			fields = append(fields, ModelField{Name: typeStr, Typ: typeStr})
			continue
		}
		for _, n := range field.Names {
			fields = append(fields, ModelField{Name: n.Name, Typ: typeStr})
		}
	}
	return fields
}

// renderReceiverType renders a method receiver's type expression back to
// source-shaped text ("*Risk" / "Risk").
func renderReceiverType(expr ast.Expr) string {
	return exprToString(expr)
}

// renderFuncSignature renders a method's full signature the way it appears
// in source: "func (r *Risk) Validate(ctx context.Context) error".
func renderFuncSignature(fn *ast.FuncDecl, recvType string) string {
	var b strings.Builder
	b.WriteString("func ")
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recvName := ""
		if len(fn.Recv.List[0].Names) > 0 {
			recvName = fn.Recv.List[0].Names[0].Name + " "
		}
		b.WriteString("(" + recvName + recvType + ") ")
	}
	b.WriteString(fn.Name.Name)
	b.WriteString(renderFieldList(fn.Type.Params, true))
	if fn.Type.Results != nil {
		results := renderFieldList(fn.Type.Results, false)
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			b.WriteString(" " + strings.Trim(results, "()"))
		} else {
			b.WriteString(" " + results)
		}
	}
	return b.String()
}

// renderFieldList renders a parameter or result field list back to
// source-shaped text, always parenthesized ("(a, b int, c string)").
func renderFieldList(fl *ast.FieldList, _ bool) string {
	if fl == nil || len(fl.List) == 0 {
		return "()"
	}
	var parts []string
	for _, field := range fl.List {
		typeStr := exprToString(field.Type)
		if len(field.Names) == 0 {
			parts = append(parts, typeStr)
			continue
		}
		names := make([]string, len(field.Names))
		for i, n := range field.Names {
			names[i] = n.Name
		}
		parts = append(parts, strings.Join(names, ", ")+" "+typeStr)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// exprToString renders an ast.Expr type expression back to source-shaped
// text for the common shapes found in authored model code (identifiers,
// pointers, selectors, arrays/slices, maps, ellipsis, and simple generic
// index expressions). Anything not recognized falls back to a literal "?"
// rather than panicking, so an unusual type expression degrades to a
// visible placeholder instead of crashing the scan.
func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprToString(e.Elt)
		}
		return "[" + exprToString(e.Len) + "]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	case *ast.Ellipsis:
		return "..." + exprToString(e.Elt)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{}"
	case *ast.FuncType:
		return "func" + renderFieldList(e.Params, true)
	case *ast.ChanType:
		return "chan " + exprToString(e.Value)
	case *ast.IndexExpr:
		return exprToString(e.X) + "[" + exprToString(e.Index) + "]"
	case *ast.BasicLit:
		return e.Value
	default:
		return "?"
	}
}

// docText joins a *ast.CommentGroup's text into a single trimmed line
// (newlines collapsed to spaces) for a compact table/list cell. Returns ""
// for a nil group.
func docText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(cg.Text(), "\n", " "))
}
