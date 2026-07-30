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
// interface/other), fields (struct only), methods (functions with this
// type as receiver, matched by base type name so both pointer and value
// receivers collapse onto the same object, PLUS -- for an interface kind --
// the interface's own declared method set, InterfaceMethods), and any
// exported typed const values declared against this type (Consts, e.g. a
// `type Status string; const ( StatusA Status = "a"; ... )` enum group).
type ModelObject struct {
	Name    string
	Kind    string // "struct" | "interface" | "type"
	Doc     string
	Fields  []ModelField
	Methods []ModelMethod
	// InterfaceMethods holds this object's own method set when Kind ==
	// "interface" -- the contract an interface type DECLARES, as opposed to
	// Methods (functions authored elsewhere with this type as receiver,
	// which a plain interface type never has since Go forbids methods on an
	// interface type itself). Populated only for Kind == "interface"; nil
	// otherwise. This is priority #1 of task #393 (W1.1): an interface is a
	// port/mock contract with the outside world, and before this field
	// existed MODELS.md showed the interface's NAME with no visible
	// methods, i.e. an empty-looking port.
	InterfaceMethods []ModelInterfaceMethod
	// Consts holds every exported `const` value declared with this object's
	// own type (e.g. `const StatusActive Status = "active"` when Name ==
	// "Status") -- an enum-value group. Populated for any Kind (typically
	// "type" for a named string/int alias); nil when no such const exists.
	Consts []ModelConst
	// ModelKind is task #394's (W1.2) semantic classification of WHAT ROLE
	// this object plays in the domain's object model, layered on top of Kind
	// (which only records the Go-syntactic shape: struct/interface/type).
	// The plan's own words: "Классификация object / value / port / mock /
	// policy без изменения их исполняемой природы" -- this is a read-only
	// label for navigation/visibility in MODELS.md, never a behavior change;
	// nothing about how any of this code actually executes is touched.
	// Vocabulary, in the priority order extractModelFile applies the rules
	// (see classifyModelKind and scanDomainMockFiles for the exact rules):
	//
	//   - "object" -- the DEFAULT/fallback. An ordinary aggregate or service
	//     with fields and/or methods -- the common case, and also what every
	//     object classified before this task's ModelKind field existed
	//     implicitly was. Also the honest fallback for a struct/type this
	//     scan cannot otherwise place (see "value" below for the one
	//     deliberately narrow exception carved out of it).
	//   - "value" -- a named primitive/alias declaration (Kind == "type",
	//     e.g. `type VehicleID string`) that has NO receiver methods
	//     anywhere in its file. A value type that DOES carry a method (e.g.
	//     a `String()`/`Validate()` receiver) falls back to "object" instead
	//     of "value" -- this is a deliberately narrow, honest heuristic, not
	//     an attempt to special-case Stringer-like methods (which would be
	//     over-engineering beyond what this classification needs to prove).
	//   - "port" -- Kind == "interface" (an interface's method set already
	//     makes it a contract with the outside world by construction).
	//   - "mock" -- an unexported struct declared in a `_test.go` file, in
	//     the SAME package as (but a different file from) a "port" interface
	//     it implements, whose own method set is a full superset of that
	//     port's required method names. NEVER derived from rules 1-4 above
	//     (which only ever look at ONE file) -- mocks require the separate
	//     cross-file matching pass in scanDomainMockFiles, since knowing
	//     "does this struct satisfy that interface" needs both files at
	//     once. A mock is not a stand-in or a hack: per the plan, "моки — не
	//     временная подмена, а правильные исполняемые суррогаты внешнего
	//     мира" (mocks are legitimate executable surrogates for the outside
	//     world) -- this classification exists so a mock becomes VISIBLE in
	//     MODELS.md next to the port contract it stands in for, not so it
	//     can be treated as lesser.
	//   - "policy" -- the object's own file lives under a `spec/policy/`
	//     directory (an exact path-segment match, `.../spec/policy/...`,
	//     never a substring match against the whole relPath -- see
	//     classifyModelKind's own doc comment for why substring matching
	//     would false-positive). Mirrors docs/AUTHORED-SPEC-CONTRACT.md §1's
	//     own already-established `spec/policy/` convention: "политики/
	//     гейты/предикаты" (policies/gates/predicates).
	//
	// Checked BEFORE "port"/"value" in classifyModelKind's own priority
	// order (policy's path-based rule can match an interface or a bare type
	// declared inside spec/policy/ just as easily as a struct, and a policy
	// classification is more specific/intentional than the generic
	// port/value fallback would be).
	ModelKind string
}

// ModelInterfaceMethod is one method declared directly in an interface
// type's own method-set list (as opposed to ModelMethod, which is a
// receiver-method authored elsewhere against a concrete type). Embedded
// interfaces (a bare type name with no method list, e.g. `io.Reader`) are
// recorded as a single ModelInterfaceMethod with Embedded set and no
// Signature/Params/Results, matching how the AST actually represents them
// (an *ast.Field with no Names).
type ModelInterfaceMethod struct {
	Name      string
	Signature string // "Recognize(photo []byte) (VehicleID, error)"-shaped rendering, no receiver
	Doc       string
	Embedded  string // set instead of Name/Signature when this entry is an embedded interface, e.g. "io.Reader"
}

// ModelConst is one exported `const` declaration whose own type matches its
// enclosing ModelObject's Name (a typed enum-value member), or -- for a
// plain untyped/primitive const group not attached to any declared type --
// a top-level entry surfaced on ModelFile.Consts instead. Value is the
// literal's source-shaped text (e.g. `"active"`, `3`), not an evaluated
// Go value, consistent with this scan's read-only/non-executing charter.
type ModelConst struct {
	Name  string
	Typ   string // "" for an untyped/inferred const
	Value string
	Doc   string
}

// ModelField is one declared field of a struct-typed ModelObject (one entry
// per declared name; embedded and multi-name declarations expand). Tag
// holds the field's raw struct tag text exactly as written (e.g.
// `json:"name,omitempty" validate:"required"`), unparsed -- this scan does
// not interpret any specific tag vocabulary (encoding/json, validator
// libraries, ...), it only preserves the literal text for rendering. Doc
// holds the field's own doc comment (the comment group immediately above
// the field declaration), independent of the enclosing ModelObject's own
// Doc.
type ModelField struct {
	Name string
	Typ  string
	Tag  string
	Doc  string
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

// ModelFunc is one exported top-level `func` declaration that is NOT a
// receiver method (FuncDecl.Recv == nil) -- a constructor (`func
// NewWidget(...) *Widget`) or any other domain-authored top-level function.
// Kept distinct from ModelObject.Methods (which are always receiver-bound)
// so rendering can label constructors/free functions separately from a
// type's own method set, per task #393 (W1.1) priority #2.
type ModelFunc struct {
	Name      string
	Signature string // "func NewWidget(name string) *Widget"-shaped rendering
	Doc       string
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
	// Funcs holds every exported top-level function that is not a receiver
	// method (constructors and other free functions), sorted by name.
	Funcs []ModelFunc
	// Consts holds every exported top-level `const` declaration whose type
	// does NOT match any ModelObject declared in this same file (an
	// untyped/primitive const group, or one typed against a type declared
	// elsewhere/not itself exported) -- a typed const whose type IS one of
	// this file's own ModelObjects is attached to that object's own Consts
	// field instead (see ModelObject.Consts), so a typed enum group renders
	// next to its type rather than being duplicated here.
	Consts []ModelConst
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
	files, err := scanDomainModelFiles(g)
	if err != nil {
		return nil, err
	}

	// Mock cross-file pass (task #394, W1.2) -- scoped to ordinary domains
	// only, deliberately NOT extended to scanSelfHostingModelFiles above (no
	// real-world evidence of test-file mocks in the engine's own
	// self-hosting file slice; out of scope per this task's own plan text).
	mockFiles, err := scanDomainMockFiles(g, files)
	if err != nil {
		return nil, err
	}
	if len(mockFiles) > 0 {
		files = append(files, mockFiles...)
		sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	}

	return files, nil
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

// mockPortSignature is one eligible port's required method-name set for
// mock-matching (task #394, W1.2 step 1) -- collected from the ALREADY
// file-classified ModelObjects scanDomainModelFiles returned (ModelKind ==
// "port"), before any test file is ever walked.
type mockPortSignature struct {
	portName string
	methods  map[string]struct{}
}

// eligiblePortSignatures collects mockPortSignature entries from files' own
// already-scanned (non-test) ModelObjects: every object with Kind ==
// "interface" and ModelKind == "port" is a CANDIDATE port, but two further
// conditions must hold before it becomes ELIGIBLE for mock-matching:
//
//   - none of its InterfaceMethods entries may have Embedded set (an
//     interface that embeds another interface, e.g. `io.Closer`, has an
//     incomplete method-name set from AST alone -- resolving io.Closer's own
//     methods would need full type-checking this AST-only scanner
//     deliberately does not do; embedding interfaces are excluded from
//     mock-matching entirely, a known, deliberate limitation, not a bug);
//   - its required method-name set (the Name of every InterfaceMethods
//     entry) must be non-empty -- an empty interface (zero methods) is never
//     eligible either, since matching against zero required methods would
//     trivially match every struct in existence.
func eligiblePortSignatures(files []ModelFile) []mockPortSignature {
	var ports []mockPortSignature
	for _, f := range files {
		for _, obj := range f.Objects {
			if obj.Kind != "interface" || obj.ModelKind != "port" {
				continue
			}
			embeds := false
			methods := map[string]struct{}{}
			for _, im := range obj.InterfaceMethods {
				if im.Embedded != "" {
					embeds = true
					break
				}
				methods[im.Name] = struct{}{}
			}
			if embeds || len(methods) == 0 {
				continue
			}
			ports = append(ports, mockPortSignature{portName: obj.Name, methods: methods})
		}
	}
	return ports
}

// scanDomainMockFiles is task #394's (W1.2) mock cross-file pass: it walks
// an ordinary (non-self-hosting) domain's `_test.go` files under the same
// specDir scanDomainModelFiles already computes, looking ONLY for unexported
// structs whose own method set is a full superset of at least one eligible
// port's required method names -- i.e. real mocks, per the real-world
// evidence this task's brief cites (PRAT-hotam/domains/gpsm-sm's
// recordingAdapter/mockOcrRecognizer, each an unexported struct declared in
// a `_test.go` file, same package as but a different file from the
// interface it implements).
//
// This pass exists ONLY to surface mocks -- it is NOT a second general-
// purpose scan of `_test.go` content. Every returned ModelFile carries ONLY
// its matched mock Objects (Funcs/Consts/Errors are always left zero-value
// even if the source test file declares them); a test file with zero
// matched mocks is dropped from the result entirely, so an ordinary
// `_test.go` file with only table-driven test funcs/assertion helpers never
// appears in MODELS.md as a hollow "no exported objects" section.
//
// files is the ALREADY-scanned non-test file list scanDomainModelFiles just
// returned (so this pass reuses that file-local ModelKind classification
// rather than re-deriving port eligibility itself). Returns (nil, nil)
// immediately, without walking the filesystem at all, when zero eligible
// ports exist -- "no ports declared yet" is a calm, expected state for the
// large majority of domains, per this codebase's established convention
// (ScanAuthoredModels' own doc comment), not a scan failure, and walking
// every `_test.go` file just to find nothing worth matching would be a
// wasted directory walk for that common case.
func scanDomainMockFiles(g *ontology.Graph, files []ModelFile) ([]ModelFile, error) {
	ports := eligiblePortSignatures(files)
	if len(ports) == 0 {
		return nil, nil
	}

	specRoot := SpecRootForGraph(g)
	specDir := filepath.Join(specRoot, "spec")

	var testFiles []string
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
		if !strings.HasSuffix(name, "_test.go") {
			return nil
		}
		testFiles = append(testFiles, path)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(testFiles) == 0 {
		return nil, nil
	}

	seen := map[string]struct{}{}
	var uniquePaths []string
	for _, p := range testFiles {
		clean := filepath.Clean(p)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		uniquePaths = append(uniquePaths, clean)
	}
	sort.Strings(uniquePaths)

	var mockFiles []ModelFile
	fset := token.NewFileSet()
	for _, p := range uniquePaths {
		// Same defense-in-depth exclusion every other path in this file
		// applies, even though no real vendored _test.go copy is known to
		// exist today (task #391's IsGeneratedOrVendoredFile choke point).
		if IsGeneratedOrVendoredFile(p) {
			continue
		}
		astFile, err := parser.ParseFile(fset, p, nil, parser.ParseComments)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(specRoot, p)
		if err != nil {
			rel = p
		}
		rel = filepath.ToSlash(rel)

		// extractModelFileForMockScan, not extractModelFile: a real mock is
		// conventionally an UNEXPORTED struct (mockOcrRecognizer,
		// recordingAdapter -- see this function's own doc comment), which
		// extractModelFile's ordinary exported-only gate would otherwise
		// make invisible.
		extracted := extractModelFileForMockScan(astFile, rel)

		var mocks []ModelObject
		for _, obj := range extracted.Objects {
			if matchesAnyPort(obj, ports) {
				obj.ModelKind = "mock"
				mocks = append(mocks, obj)
			}
		}
		if len(mocks) == 0 {
			// No matched mock in this test file: drop it entirely rather
			// than emit an empty "no exported objects" section -- that
			// would be pure noise across every domain's ordinary test
			// helpers/table-driven test funcs.
			continue
		}

		mockFiles = append(mockFiles, ModelFile{
			RelPath: rel,
			Pkg:     extracted.Pkg,
			Objects: mocks,
			// Funcs/Consts/Errors deliberately left zero-value: this pass
			// surfaces ONLY matched mocks, never a second general-purpose
			// scan of a test file's other content (plain test functions,
			// unrelated consts, ...).
		})
	}

	sort.Slice(mockFiles, func(i, j int) bool { return mockFiles[i].RelPath < mockFiles[j].RelPath })
	return mockFiles, nil
}

// matchesAnyPort reports whether obj's own method set (from obj.Methods --
// the receiver methods this same test file declares against it) is a full
// superset of at least one eligible port's required method-name set, i.e.
// obj is a name-only-verified mock of that port. Name-only method-set
// matching (never full go/types signature checking of params/results) is
// the accepted, documented approximation this whole AST-only scanner
// already uses everywhere else (e.g. receiverBaseTypeName's own base-type
// collapsing) -- proving a mock genuinely satisfies an interface's full
// signature would require full type-checking this scan deliberately never
// performs.
func matchesAnyPort(obj ModelObject, ports []mockPortSignature) bool {
	if len(obj.Methods) == 0 {
		return false
	}
	objMethods := map[string]struct{}{}
	for _, m := range obj.Methods {
		objMethods[m.Name] = struct{}{}
	}
	for _, port := range ports {
		if isSuperset(objMethods, port.methods) {
			return true
		}
	}
	return false
}

// isSuperset reports whether super contains every key in sub (sub is empty-
// safe: an empty sub is vacuously a subset of anything, but
// eligiblePortSignatures already guarantees every port.methods it produces
// is non-empty, so that vacuous case never actually arises here).
func isSuperset(super, sub map[string]struct{}) bool {
	for k := range sub {
		if _, ok := super[k]; !ok {
			return false
		}
	}
	return true
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
// the underlying type is *ast.StructType; interface methods extracted when
// the underlying type is *ast.InterfaceType), FuncDecl with a receiver ->
// attached to the matching ModelObject by base receiver type name,
// FuncDecl WITHOUT a receiver -> ModelFunc (constructors/top-level
// functions), GenDecl(VAR) whose spec name starts with "Err" ->
// ModelError, GenDecl(CONST) -> ModelConst, attached to the matching
// ModelObject when the const's own type names one of this file's objects,
// else collected on ModelFile.Consts directly. Everything is re-sorted
// (objects/methods/interface-methods/errors/funcs sorted by name; struct
// fields and const groups left declaration-ordered since that order is
// itself meaningful API surface -- a field list's shape and an enum's
// member order are both authored intent) so output is deterministic
// regardless of source declaration order.
//
// Only EXPORTED top-level type declarations become a ModelObject (see
// extractModelFileImpl's includeUnexportedTypes parameter for the one
// deliberate exception, used solely by scanDomainMockFiles' mock-matching
// pass -- task #394, W1.2). This is a thin, zero-cost wrapper preserving the
// exact pre-#394 signature and behavior, so every existing call site
// (parseModelFiles, and the structural assertion
// TestExtractModelFile_VendoredFileNeverReachesExtraction which pins this
// exact two-arg shape) is entirely unaffected.
func extractModelFile(astFile *ast.File, relPath string) ModelFile {
	return extractModelFileImpl(astFile, relPath, false)
}

// extractModelFileForMockScan is extractModelFile's ONLY other caller-facing
// entry point: identical extraction algorithm (never a forked/duplicated
// parallel implementation -- both wrappers share extractModelFileImpl's one
// body), except top-level type declarations are collected regardless of
// exportedness. This one relaxation exists solely because a real mock is
// conventionally an UNEXPORTED struct (see this task's brief and the real
// PRAT-hotam/domains/gpsm-sm evidence -- mockOcrRecognizer,
// recordingAdapter): extractModelFile's ordinary exported-only gate would
// silently make every real-world mock invisible to scanDomainMockFiles'
// matching pass, defeating this task's entire purpose. Used ONLY by
// scanDomainMockFiles, on `_test.go` files, never on an ordinary domain
// model file -- the general (non-test) scan's own exported-only behavior for
// top-level types is completely unchanged.
func extractModelFileForMockScan(astFile *ast.File, relPath string) ModelFile {
	return extractModelFileImpl(astFile, relPath, true)
}

// extractModelFileImpl is the single shared implementation both
// extractModelFile and extractModelFileForMockScan call -- includeUnexportedTypes
// is the only behavioral difference between the two, gating solely the
// top-level TYPE declaration's own exportedness check (GenDecl(VAR)/
// GenDecl(CONST)/top-level FuncDecl exportedness gates are UNCHANGED either
// way; a mock struct's own methods and any const/var it happens to declare
// still follow the ordinary rules -- only whether the struct ITSELF becomes
// a ModelObject at all is affected).
func extractModelFileImpl(astFile *ast.File, relPath string, includeUnexportedTypes bool) ModelFile {
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
				if !ok || (!includeUnexportedTypes && !ts.Name.IsExported()) {
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
					obj.InterfaceMethods = extractInterfaceMethods(t)
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

	// Second pass for CONST groups: deferred until after every TYPE spec has
	// been collected into objByName, so a typed const referencing a type
	// declared LATER in the same file (or earlier -- declaration order
	// within one file is not otherwise significant) still resolves to its
	// object correctly regardless of which GenDecl comes first in
	// astFile.Decls.
	lastTypeName := "" // iota-style groups can omit the type on later ValueSpecs, inheriting the prior spec's type
	for _, decl := range astFile.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		lastTypeName = ""
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			typeName := ""
			if vs.Type != nil {
				typeName = exprToString(vs.Type)
				lastTypeName = typeName
			} else if len(vs.Values) == 0 {
				// No explicit type and no explicit value: an iota-style
				// continuation line inherits the previous ValueSpec's type.
				typeName = lastTypeName
			}
			for i, name := range vs.Names {
				if !name.IsExported() {
					continue
				}
				doc := docText(gd.Doc)
				if doc == "" && i == 0 {
					doc = docText(vs.Doc)
				}
				value := ""
				if i < len(vs.Values) {
					value = exprToString(vs.Values[i])
				}
				mc := ModelConst{Name: name.Name, Typ: typeName, Value: value, Doc: doc}
				if obj, ok := objByName[typeName]; ok {
					obj.Consts = append(obj.Consts, mc)
				} else {
					f.Consts = append(f.Consts, mc)
				}
			}
		}
	}

	for _, decl := range astFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			// Not a receiver method: a constructor or other top-level
			// function. Only exported ones count as domain-authored public
			// surface, same exportedness gate every other category here
			// applies (types, errors, consts).
			if fn.Name.IsExported() {
				f.Funcs = append(f.Funcs, ModelFunc{
					Name:      fn.Name.Name,
					Signature: renderFuncSignature(fn, ""),
					Doc:       docText(fn.Doc),
				})
			}
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
		sort.Slice(obj.InterfaceMethods, func(i, j int) bool {
			return interfaceMethodSortKey(obj.InterfaceMethods[i]) < interfaceMethodSortKey(obj.InterfaceMethods[j])
		})
		// ModelKind is classified last, after Methods is fully populated --
		// rule 3 ("value") depends on knowing whether ANY receiver method
		// was ever attached to this object, which is only certain once the
		// FuncDecl pass above (which runs after every TYPE spec is
		// collected) has finished.
		obj.ModelKind = classifyModelKind(*obj, relPath)
		f.Objects = append(f.Objects, *obj)
	}
	sort.Slice(f.Objects, func(i, j int) bool { return f.Objects[i].Name < f.Objects[j].Name })
	sort.Slice(f.Errors, func(i, j int) bool { return f.Errors[i].Name < f.Errors[j].Name })
	sort.Slice(f.Funcs, func(i, j int) bool { return f.Funcs[i].Name < f.Funcs[j].Name })

	return f
}

// classifyModelKind implements ModelKind's file-local classification rules
// 1-4 (task #394, W1.2) -- everything except "mock" (rule 5, which requires
// cross-file knowledge and is applied separately by scanDomainMockFiles,
// overriding whatever this function would have returned). Priority order,
// first match wins:
//
//  1. "policy" -- obj's own file lives under a `spec/policy/` directory.
//  2. "port"   -- obj.Kind == "interface".
//  3. "value"  -- obj.Kind == "type" (a named primitive/alias) AND obj has
//     zero Methods.
//  4. "object" -- the default fallback, everything else (ordinary structs,
//     types that DO carry a method, ...).
//
// relPath is expected already root-relative and forward-slash-normalized
// (parseModelFiles does this once via filepath.ToSlash before calling
// extractModelFile), so isUnderSpecPolicyDir can split on "/" directly
// without re-normalizing.
func classifyModelKind(obj ModelObject, relPath string) string {
	if isUnderSpecPolicyDir(relPath) {
		return "policy"
	}
	if obj.Kind == "interface" {
		return "port"
	}
	if obj.Kind == "type" && len(obj.Methods) == 0 {
		return "value"
	}
	return "object"
}

// isUnderSpecPolicyDir reports whether relPath (forward-slash, root-relative)
// contains the exact path-segment sequence ".../spec/policy/..." -- i.e. an
// exact segment equal to "policy" immediately following an exact segment
// equal to "spec". This is deliberately a SEGMENT match, not a substring
// match: strings.Contains(relPath, "policy") would false-positive on a
// hypothetical domain-authored spec/model/policyholder.go (a real object
// model file that merely has "policy" as a substring of its own file name),
// wrongly classifying an ordinary object as a policy. Mirrors
// docs/AUTHORED-SPEC-CONTRACT.md §1's own established spec/policy/
// convention ("политики/гейты/предикаты" -- policies/gates/predicates).
func isUnderSpecPolicyDir(relPath string) bool {
	segments := strings.Split(relPath, "/")
	for i := 0; i+1 < len(segments); i++ {
		if segments[i] == "spec" && segments[i+1] == "policy" {
			return true
		}
	}
	return false
}

// interfaceMethodSortKey gives a stable sort key for a ModelInterfaceMethod:
// its own method Name when set, else its Embedded interface name (an
// embedded interface entry has no Name).
func interfaceMethodSortKey(m ModelInterfaceMethod) string {
	if m.Name != "" {
		return m.Name
	}
	return m.Embedded
}

// extractFields renders a struct type's field list as ModelFields, one per
// declared name (an embedded field or a multi-name single declaration like
// `X, Y int` both expand to one ModelField per name). Unexported fields are
// included too -- the inventory is of the WHOLE shape a domain author wrote,
// not just its public API, since an authored spec/ model commonly keeps its
// invariant-carrying fields unexported on purpose. Tag carries the field's
// raw struct tag text verbatim (e.g. `json:"name,omitempty"`, "" when the
// field has none); Doc carries the field's own doc comment, independent of
// any multi-name sibling on the same declaration line and independent of
// the enclosing type's own Doc.
func extractFields(t *ast.StructType) []ModelField {
	if t.Fields == nil {
		return nil
	}
	var fields []ModelField
	for _, field := range t.Fields.List {
		typeStr := exprToString(field.Type)
		tag := ""
		if field.Tag != nil {
			tag = strings.Trim(field.Tag.Value, "`")
		}
		doc := docText(field.Doc)
		if len(field.Names) == 0 {
			// Embedded field: name is the type's own base identifier.
			fields = append(fields, ModelField{Name: typeStr, Typ: typeStr, Tag: tag, Doc: doc})
			continue
		}
		for _, n := range field.Names {
			fields = append(fields, ModelField{Name: n.Name, Typ: typeStr, Tag: tag, Doc: doc})
		}
	}
	return fields
}

// extractInterfaceMethods renders an interface type's own declared method
// set (the contract it specifies) as ModelInterfaceMethods -- one per
// method (rendered without a receiver, since an interface method has none)
// and one per embedded interface (a bare type name with no method list,
// e.g. `io.Reader`, recorded with Embedded set instead of Name/Signature).
// Unexported interface methods ARE included (mirroring extractFields'
// same "whole authored shape, not just exported surface" policy) since an
// unexported interface method is still part of the contract a same-package
// mock must satisfy.
func extractInterfaceMethods(t *ast.InterfaceType) []ModelInterfaceMethod {
	if t.Methods == nil {
		return nil
	}
	var methods []ModelInterfaceMethod
	for _, field := range t.Methods.List {
		doc := docText(field.Doc)
		if len(field.Names) == 0 {
			// Embedded interface: no name, the type expression IS the
			// embedded interface's own identifier (possibly qualified,
			// e.g. io.Reader, or a union/approximation element in a type
			// set -- exprToString degrades unknown shapes to "?" rather
			// than panicking).
			methods = append(methods, ModelInterfaceMethod{Embedded: exprToString(field.Type), Doc: doc})
			continue
		}
		ft, ok := field.Type.(*ast.FuncType)
		if !ok {
			// A named type-set element (Go 1.18+ generic interface
			// constraint, e.g. `Ordered interface { int | float64 }`) --
			// not a callable method. Record it as an embedded-shaped entry
			// under its own name so the constraint element is still
			// visible rather than silently dropped.
			for _, n := range field.Names {
				methods = append(methods, ModelInterfaceMethod{Embedded: n.Name, Doc: doc})
			}
			continue
		}
		for _, n := range field.Names {
			methods = append(methods, ModelInterfaceMethod{
				Name:      n.Name,
				Signature: n.Name + renderFieldList(ft.Params, true) + renderResultSuffix(ft.Results),
				Doc:       doc,
			})
		}
	}
	return methods
}

// renderReceiverType renders a method receiver's type expression back to
// source-shaped text ("*Risk" / "Risk").
func renderReceiverType(expr ast.Expr) string {
	return exprToString(expr)
}

// renderFuncSignature renders a function or method's full signature the way
// it appears in source: "func (r *Risk) Validate(ctx context.Context)
// error" for a receiver method, "func NewRisk(name string) *Risk" for a
// top-level function (fn.Recv == nil, recvType then unused). Shared by both
// ModelMethod (receiver methods) and ModelFunc (top-level functions) so the
// two categories render results/params identically.
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
	b.WriteString(renderResultSuffix(fn.Type.Results))
	return b.String()
}

// renderResultSuffix renders a function/method/interface-method's result
// field list as the trailing " T" / " (a A, b B)"-shaped suffix that
// follows its parameter list in source (a single unnamed result is
// rendered bare, without parens; anything else keeps parens) -- "" when
// results is nil (no results). Shared by renderFuncSignature (receiver
// methods and top-level functions) and extractInterfaceMethods (interface
// methods, which have no *ast.FuncDecl to hang this on) so all three
// categories render results identically.
func renderResultSuffix(results *ast.FieldList) string {
	if results == nil {
		return ""
	}
	rendered := renderFieldList(results, false)
	if len(results.List) == 1 && len(results.List[0].Names) == 0 {
		return " " + strings.Trim(rendered, "()")
	}
	return " " + rendered
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
