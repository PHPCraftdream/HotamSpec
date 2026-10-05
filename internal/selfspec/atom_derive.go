package selfspec

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"github.com/PHPCraftdream/HotamSpec/internal/registry"
)

// DiscoverAtoms runs only for the explicit `self_executing_atoms` trigger and
// records each authored model package once. Rule/case artifacts additionally
// require `conformance.rule_cases`; that permission never enables scanning.
// Registry values provide identity, lifecycle, and authored metadata overrides.
func DiscoverAtoms(specRoot string, overrides *registry.Registry[ontology.Requirement]) (*registry.Registry[ontology.Requirement], error) {
	manifest, err := loader.LoadManifest(filepath.Join(specRoot, "manifest.json"))
	if err != nil {
		return nil, err
	}
	if !manifest.SelfExecutingAtoms {
		return overrides, nil
	}
	sourceGraph := &ontology.Graph{
		DomainDir: specRoot, SelfHosting: manifest.SelfHosting,
		SelfExecutingAtoms: true,
		Languages:          manifest.Languages, DefaultLanguage: manifest.DefaultLanguage,
		Conformance: manifest.Conformance,
	}
	snapshot, err := gate.CollectAtomExecutionSnapshot(sourceGraph)
	if err != nil {
		return nil, err
	}
	return discoverAtomsFromSnapshot(specRoot, manifest, overrides, snapshot)
}

// DiscoverAtomsFromSnapshot projects recorder output already collected for
// this invocation; it never starts another package run.
func DiscoverAtomsFromSnapshot(specRoot string, overrides *registry.Registry[ontology.Requirement], snapshot *gate.AtomExecutionSnapshot) (*registry.Registry[ontology.Requirement], error) {
	manifest, err := loader.LoadManifest(filepath.Join(specRoot, "manifest.json"))
	if err != nil {
		return nil, err
	}
	return discoverAtomsFromSnapshot(specRoot, manifest, overrides, snapshot)
}

func discoverAtomsFromSnapshot(specRoot string, manifest *loader.DomainManifest, overrides *registry.Registry[ontology.Requirement], snapshot *gate.AtomExecutionSnapshot) (*registry.Registry[ontology.Requirement], error) {
	if !manifest.SelfExecutingAtoms {
		return overrides, nil
	}
	if snapshot == nil {
		return nil, fmt.Errorf("atom discovery requires an execution snapshot")
	}
	if snapshot.SourceErr != nil {
		return nil, snapshot.SourceErr
	}
	if snapshot.DiscoveryErr != nil {
		return nil, snapshot.DiscoveryErr
	}
	sources := snapshot.SourceIndex
	if sources == nil {
		return nil, fmt.Errorf("atom discovery snapshot has no source index")
	}
	defaults := loader.AtomDefaults{Owner: manifest.Director, Status: "SETTLED"}
	if manifest.AtomDefaults != nil {
		defaults = *manifest.AtomDefaults
		if defaults.Owner == "" {
			defaults.Owner = manifest.Director
		}
		if defaults.Status == "" {
			defaults.Status = "SETTLED"
		}
	}
	packages := map[string]string{}
	tests := map[string]map[string]string{}
	expectedTests := map[string]bool{}
	recordedTests := map[string]bool{}
	// proofOrder captures the authored narrative order of Fact/Holds calls:
	// test files keep their package order, TestXxx functions follow source
	// order, and proofs inside a test follow call order. A Holds relation
	// sits at its own call site, so evidence Facts nested in its arguments
	// follow it.
	type proofSite struct {
		method string
		offset int
	}
	type proofSeq struct {
		fnOffset int
		sites    []proofSite
	}
	proofOrder := map[string]proofSeq{}
	hasRuleCaseOption := false
	dirs := make([]string, 0, len(snapshot.TestFiles))
	for dir := range snapshot.TestFiles {
		if dir == "spec/model" || strings.HasPrefix(dir, "spec/model/") {
			dirs = append(dirs, dir)
		}
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		file := snapshot.PackageFiles[dir]
		if file == "" {
			return nil, fmt.Errorf("atom package %s has test references but no package file", dir)
		}
		packages[dir] = file
		tests[dir] = snapshot.TestFiles[dir]
		testFiles := make([]string, 0, len(tests[dir]))
		seenFiles := make(map[string]bool, len(tests[dir]))
		for _, testFile := range tests[dir] {
			if !seenFiles[testFile] {
				testFiles = append(testFiles, testFile)
				seenFiles[testFile] = true
			}
		}
		sort.Strings(testFiles)
		for _, testFile := range testFiles {
			fset := token.NewFileSet()
			path := filepath.Join(specRoot, filepath.FromSlash(testFile))
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return nil, err
			}
			recorderAliases := map[string]bool{}
			testingAliases := map[string]bool{}
			for _, imp := range f.Imports {
				value, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return nil, err
				}
				name := filepath.Base(value)
				if imp.Name != nil {
					name = imp.Name.Name
				}
				if value == sources.RecorderImportPath {
					recorderAliases[name] = true
				}
				if value == "testing" {
					testingAliases[name] = true
				}
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || !isAtomTest(fn, testingAliases) {
					continue
				}
				if mapped := tests[dir][fn.Name.Name]; mapped != testFile {
					return nil, fmt.Errorf("snapshot test map for %s is inconsistent at %s", fn.Name.Name, testFile)
				}
				seq := proofSeq{fnOffset: fset.Position(fn.Pos()).Offset}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					fun := call.Fun
					if indexed, ok := fun.(*ast.IndexExpr); ok {
						fun = indexed.X
					}
					sel, ok := fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					qualifier, ok := sel.X.(*ast.Ident)
					if !ok || !recorderAliases[qualifier.Name] {
						return true
					}
					if sel.Sel.Name == "WithCase" {
						hasRuleCaseOption = true
					}
					if sel.Sel.Name == "Fact" || sel.Sel.Name == "Holds" {
						expectedTests[testFile+":"+fn.Name.Name] = true
						if len(call.Args) > 1 {
							subject := ast.Unparen(call.Args[1])
							if subjectSel, ok := subject.(*ast.SelectorExpr); ok {
								seq.sites = append(seq.sites, proofSite{
									method: subjectSel.Sel.Name,
									offset: fset.Position(call.Pos()).Offset,
								})
							}
						}
					}
					return true
				})
				proofOrder[testFile+":"+fn.Name.Name] = seq
			}
		}
	}
	if hasRuleCaseOption && (manifest.Conformance == nil || !manifest.Conformance.RuleCases) {
		return nil, fmt.Errorf("hotamspec.WithCase requires conformance.rule_cases")
	}
	type discovered struct {
		r        ontology.Requirement
		position string
	}
	var atoms []discovered
	byID := map[string]int{}
	usedOverrides := map[string]bool{}
	evidenceLinks := map[string][]string{}
	caseByID := map[string]ontology.CaseDefinition{}
	caseAtoms := map[string]map[string]bool{}
	for _, dir := range dirs {
		result, found := snapshot.PackageRuns[dir]
		if !found {
			return nil, fmt.Errorf("atom package %s was not recorded in the shared snapshot", dir)
		}
		if result.Skipped || result.Err != nil || result.CompileFailed || !result.Passed {
			return nil, fmt.Errorf("atom recording %s failed or skipped: %+v", packages[dir], result.TestRunResult)
		}
		for _, raw := range result.Artifacts {
			a, err := gate.DecodeAtomArtifact(raw.RawJSON)
			if err != nil {
				return nil, err
			}
			if a.Mode == "" {
				continue
			}
			if a.Mode == "rule" && (manifest.Conformance == nil || !manifest.Conformance.RuleCases) {
				return nil, fmt.Errorf("rule atom %s requires conformance.rule_cases", a.ReqID)
			}
			claimTexts, err := sources.DeriveClaims(a)
			if err != nil {
				return nil, err
			}
			primaryLanguage := manifest.DefaultLanguage
			if primaryLanguage == "" && len(manifest.Languages) == 1 {
				primaryLanguage = manifest.Languages[0]
			}
			claim, ok := claimTexts[primaryLanguage]
			if !ok {
				return nil, fmt.Errorf("atom %s has no primary claim for language %q", a.ReqID, primaryLanguage)
			}
			test := a.Test
			if test == "" {
				return nil, fmt.Errorf("atom %s artifact omits executed test identity", a.ReqID)
			}
			rootTest := strings.SplitN(test, "/", 2)[0]
			file := tests[dir][rootTest]
			if file == "" {
				return nil, fmt.Errorf("atom %s has unresolved test %q", a.ReqID, test)
			}
			recordedTests[file+":"+rootTest] = true
			links := make([]string, 0, len(a.Steps))
			seen := map[string]bool{}
			position := ""
			for _, step := range a.Steps {
				source, err := sources.Resolve(step.Subject)
				if err != nil {
					return nil, err
				}
				link := source.Link()
				if !seen[link] {
					links = append(links, link)
					seen[link] = true
				}
				if position == "" {
					// Narrative order: the atom sits at its first proof call
					// site (for Holds -- at the Holds call itself, before its
					// nested evidence Facts), keyed by test file, TestXxx
					// source order, and call order. Falls back to the test
					// declaration when the subject is not statically resolvable.
					method := source.Symbol
					if i := strings.LastIndex(method, "."); i >= 0 {
						method = method[i+1:]
					}
					seq := proofOrder[file+":"+rootTest]
					callOffset := seq.fnOffset
					for _, site := range seq.sites {
						if site.method == method {
							callOffset = site.offset
							break
						}
					}
					position = fmt.Sprintf("%s:%09d:%09d", file, seq.fnOffset, callOffset)
				}
			}
			r := ontology.Requirement{
				ID: a.ReqID, Claim: claim, Owner: defaults.Owner, Status: defaults.Status,
				Why: defaults.Why, CreatedAt: defaults.CreatedAt, SettledAt: defaults.SettledAt,
				ImplementedBy: links, VerifiedBy: []string{file + ":" + rootTest},
				Enforcement: "ENFORCED", Enforceability: "ENFORCEABLE",
			}
			if len(manifest.Languages) > 0 {
				r.ClaimTexts = claimTexts
			}
			if a.Mode == "rule" {
				r.AtomKind = "rule"
				caseDef, err := a.CaseDefinition(file + ":" + test)
				if err != nil {
					return nil, err
				}
				caseByID[caseDef.ID], err = registerCaseDescriptor(caseByID, *caseDef)
				if err != nil {
					return nil, err
				}
				if caseAtoms[caseDef.ID] == nil {
					caseAtoms[caseDef.ID] = map[string]bool{}
				}
				r.Cases = []ontology.CaseDefinition{*caseDef}
			}
			var matches []ontology.Requirement
			for _, override := range overrides.All() {
				if override.Status == "REJECTED" {
					continue
				}
				match := override.ID == r.ID
				if len(override.ImplementedBy) > 0 {
					match = override.ImplementedBy[0] == links[0]
				}
				if match {
					matches = append(matches, override)
				}
			}
			if len(matches) > 1 {
				return nil, fmt.Errorf("multiple registry overrides for %s", r.ID)
			}
			if len(matches) == 1 {
				o := matches[0]
				usedOverrides[o.ID] = true
				if o.Claim != "" && o.Claim != claim {
					return nil, fmt.Errorf("atom %s explicit Claim conflicts with derived claim", o.ID)
				}
				if o.AtomKind != "" && o.AtomKind != r.AtomKind {
					return nil, fmt.Errorf("atom %s explicit AtomKind conflicts with recorded mode", o.ID)
				}
				if len(o.ClaimTexts) > 0 && !reflect.DeepEqual(o.ClaimTexts, r.ClaimTexts) {
					return nil, fmt.Errorf("atom %s explicit ClaimTexts conflict with derived claims", o.ID)
				}
				r.ID = o.ID
				if a.Mode == "rule" && len(r.Cases) > 0 && len(r.Cases[0].AtomIDs) > 0 && !containsString(r.Cases[0].AtomIDs, r.ID) {
					return nil, fmt.Errorf("case %s does not include overridden atom %s", r.Cases[0].ID, r.ID)
				}
				if o.Owner != "" {
					r.Owner = o.Owner
				}
				if o.Status != "" {
					r.Status = o.Status
				}
				if o.Why != "" {
					r.Why = o.Why
				}
				if o.CreatedAt != "" {
					r.CreatedAt = o.CreatedAt
				}
				if o.SettledAt != "" {
					r.SettledAt = o.SettledAt
				}
				r.Assumptions = o.Assumptions
				r.Relations = o.Relations
				r.SourceRefs = o.SourceRefs
				r.SourceLinks = o.SourceLinks
				r.Coverage = o.Coverage
				r.Summary = o.Summary
				if o.MTag != "" {
					r.MTag = o.MTag
				}
				if o.Enforcement != "" {
					r.Enforcement = o.Enforcement
				}
				if o.Enforceability != "" {
					r.Enforceability = o.Enforceability
				}
				r.EnforcedBy = o.EnforcedBy
				r.ClauseLinks = o.ClauseLinks
				r.Strength = o.Strength
				r.Applicability = o.Applicability
				r.Precedence = o.Precedence
				if len(o.Cases) > 0 {
					r.Cases, err = mergeCaseDefinitions(r.Cases, o.Cases)
					if err != nil {
						return nil, fmt.Errorf("atom %s registry cases: %w", o.ID, err)
					}
				}
			}
			for i := range r.Cases {
				if r.Cases[i].ID == "" {
					continue
				}
				r.Cases[i], err = registerCaseDescriptor(caseByID, r.Cases[i])
				if err != nil {
					return nil, err
				}
				if caseAtoms[r.Cases[i].ID] == nil {
					caseAtoms[r.Cases[i].ID] = map[string]bool{}
				}
				caseAtoms[r.Cases[i].ID][r.ID] = true
			}
			if idx, ok := byID[r.ID]; ok {
				old := &atoms[idx].r
				if old.Claim != r.Claim || !reflect.DeepEqual(old.ClaimTexts, r.ClaimTexts) ||
					old.AtomKind != r.AtomKind || len(old.ImplementedBy) == 0 || len(r.ImplementedBy) == 0 ||
					old.ImplementedBy[0] != r.ImplementedBy[0] {
					return nil, fmt.Errorf("atom %s produced conflicting subjects or legacy executed values", r.ID)
				}
				old.VerifiedBy = appendUnique(old.VerifiedBy, r.VerifiedBy...)
				old.ImplementedBy = appendUnique(old.ImplementedBy, r.ImplementedBy...)
				old.Cases, err = mergeCaseDefinitions(old.Cases, r.Cases)
				if err != nil {
					return nil, fmt.Errorf("atom %s: %w", r.ID, err)
				}
				// The atom keeps the earliest narrative position across all its
				// proving tests, so ordering stays deterministic regardless of
				// artifact file-name order.
				if position < atoms[idx].position {
					atoms[idx].position = position
				}
			} else {
				byID[r.ID] = len(atoms)
				atoms = append(atoms, discovered{r: r, position: position})
			}
			if a.Mode == "holds" || a.Mode == "rule" {
				evidenceLinks[r.ID] = appendUnique(evidenceLinks[r.ID], links[1:]...)
			}
		}
	}
	expectedTestNames := make([]string, 0, len(expectedTests))
	for test := range expectedTests {
		expectedTestNames = append(expectedTestNames, test)
	}
	sort.Strings(expectedTestNames)
	for _, test := range expectedTestNames {
		if !recordedTests[test] {
			return nil, fmt.Errorf("atom test %s produced no recording (skipped or not executed)", test)
		}
	}
	for i := range atoms {
		for j := range atoms[i].r.Cases {
			caseDef := &atoms[i].r.Cases[j]
			if len(caseDef.AtomIDs) != 0 {
				continue
			}
			for id := range caseAtoms[caseDef.ID] {
				caseDef.AtomIDs = append(caseDef.AtomIDs, id)
			}
			sort.Strings(caseDef.AtomIDs)
		}
	}
	// Supporting atoms remain independent requirements. Structural dependency
	// edges connect shared-test proofs without changing authored_links semantics.
	primaryIDs := map[string]string{}
	for _, a := range atoms {
		if len(a.r.ImplementedBy) > 0 {
			primaryIDs[a.r.ImplementedBy[0]] = a.r.ID
		}
	}
	for i := range atoms {
		for _, link := range evidenceLinks[atoms[i].r.ID] {
			target, ok := primaryIDs[link]
			if !ok {
				return nil, fmt.Errorf("atom %s supporting evidence %s has no independently recorded requirement", atoms[i].r.ID, link)
			}
			relation := ontology.Relation{Kind: "depends_on", Target: target}
			found := false
			for _, old := range atoms[i].r.Relations {
				if old == relation {
					found = true
					break
				}
			}
			if !found {
				atoms[i].r.Relations = append(atoms[i].r.Relations, relation)
			}
		}
	}
	sort.SliceStable(atoms, func(i, j int) bool { return atoms[i].position < atoms[j].position })
	out := registry.New[ontology.Requirement]()
	for i, a := range atoms {
		a.r.DeclOrder = i + 1
		out.MustRegister(a.r.ID, a.r)
	}
	for _, o := range overrides.All() {
		if o.Status == "REJECTED" {
			if _, exists := out.Get(o.ID); exists {
				return nil, fmt.Errorf("rejected requirement %s still has an executed atom", o.ID)
			}
			out.MustRegister(o.ID, o)
		} else if !usedOverrides[o.ID] {
			return nil, fmt.Errorf("registry override %s has no executed atom; method renames require explicit REJECTED/replaces", o.ID)
		}
	}
	return out, nil
}

func appendUnique(dst []string, values ...string) []string {
	for _, value := range values {
		found := false
		for _, old := range dst {
			if old == value {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, value)
		}
	}
	return dst
}

func mergeCaseDescriptor(existing, incoming ontology.CaseDefinition) (ontology.CaseDefinition, error) {
	if existing.ID == "" || incoming.ID == "" || existing.ID != incoming.ID {
		return ontology.CaseDefinition{}, fmt.Errorf("case descriptor IDs do not match")
	}
	out := existing
	mergeString := func(name string, dst *string, src string) error {
		if *dst != "" && src != "" && *dst != src {
			return fmt.Errorf("case %s has conflicting %s", existing.ID, name)
		}
		if *dst == "" {
			*dst = src
		}
		return nil
	}
	if err := mergeString("test", &out.Test, incoming.Test); err != nil {
		return ontology.CaseDefinition{}, err
	}
	if err := mergeString("profile", &out.Profile, incoming.Profile); err != nil {
		return ontology.CaseDefinition{}, err
	}
	if err := mergeString("target", &out.Target, incoming.Target); err != nil {
		return ontology.CaseDefinition{}, err
	}
	if err := mergeString("operation", &out.Operation, incoming.Operation); err != nil {
		return ontology.CaseDefinition{}, err
	}
	if err := mergeString("producer", &out.Producer, incoming.Producer); err != nil {
		return ontology.CaseDefinition{}, err
	}
	mergeSlice := func(name string, dst any, src any, dstEmpty bool, srcEmpty bool) error {
		if !dstEmpty && !srcEmpty && !reflect.DeepEqual(dst, src) {
			return fmt.Errorf("case %s has conflicting %s", existing.ID, name)
		}
		return nil
	}
	if err := mergeSlice("atom_ids", out.AtomIDs, incoming.AtomIDs, len(out.AtomIDs) == 0, len(incoming.AtomIDs) == 0); err != nil {
		return ontology.CaseDefinition{}, err
	}
	if len(out.AtomIDs) == 0 {
		out.AtomIDs = append([]string(nil), incoming.AtomIDs...)
	}
	if err := mergeSlice("fixtures", out.Fixtures, incoming.Fixtures, len(out.Fixtures) == 0, len(incoming.Fixtures) == 0); err != nil {
		return ontology.CaseDefinition{}, err
	}
	if len(out.Fixtures) == 0 {
		out.Fixtures = append([]ontology.FixtureRef(nil), incoming.Fixtures...)
	}
	if err := mergeSlice("conditions", out.Conditions, incoming.Conditions, len(out.Conditions) == 0, len(incoming.Conditions) == 0); err != nil {
		return ontology.CaseDefinition{}, err
	}
	if len(out.Conditions) == 0 {
		out.Conditions = append([]ontology.ConditionEvidence(nil), incoming.Conditions...)
	}
	if err := mergeSlice("sides", out.Sides, incoming.Sides, len(out.Sides) == 0, len(incoming.Sides) == 0); err != nil {
		return ontology.CaseDefinition{}, err
	}
	if len(out.Sides) == 0 {
		out.Sides = append([]string(nil), incoming.Sides...)
	}
	if out.Input != nil && incoming.Input != nil && !ontology.EqualObservedValues(out.Input, incoming.Input) {
		return ontology.CaseDefinition{}, fmt.Errorf("case %s has conflicting input", existing.ID)
	}
	if out.Input == nil && incoming.Input != nil {
		copy := *incoming.Input
		out.Input = &copy
	}
	if out.Expected != nil && incoming.Expected != nil && !ontology.EqualObservedValues(out.Expected, incoming.Expected) {
		return ontology.CaseDefinition{}, fmt.Errorf("case %s has conflicting expected value", existing.ID)
	}
	if out.Expected == nil && incoming.Expected != nil {
		copy := *incoming.Expected
		out.Expected = &copy
	}
	if out.Selection != nil && incoming.Selection != nil && !reflect.DeepEqual(*out.Selection, *incoming.Selection) {
		return ontology.CaseDefinition{}, fmt.Errorf("case %s has conflicting selection", existing.ID)
	}
	if out.Selection == nil && incoming.Selection != nil {
		copy := *incoming.Selection
		out.Selection = &copy
	}
	return out, nil
}

func registerCaseDescriptor(byID map[string]ontology.CaseDefinition, caseDef ontology.CaseDefinition) (ontology.CaseDefinition, error) {
	if caseDef.ID == "" {
		return ontology.CaseDefinition{}, fmt.Errorf("rule case has empty ID")
	}
	if old, exists := byID[caseDef.ID]; exists {
		merged, err := mergeCaseDescriptor(old, caseDef)
		if err != nil {
			return ontology.CaseDefinition{}, err
		}
		byID[caseDef.ID] = merged
		return merged, nil
	}
	byID[caseDef.ID] = caseDef
	return caseDef, nil
}

func mergeCaseDefinitions(dst, additions []ontology.CaseDefinition) ([]ontology.CaseDefinition, error) {
	out := append([]ontology.CaseDefinition(nil), dst...)
	byID := make(map[string]int, len(out)+len(additions))
	for i, caseDef := range out {
		if caseDef.ID == "" {
			return nil, fmt.Errorf("case definition has empty ID")
		}
		if _, exists := byID[caseDef.ID]; exists {
			return nil, fmt.Errorf("case %s is duplicated in requirement", caseDef.ID)
		}
		byID[caseDef.ID] = i
	}
	for _, caseDef := range additions {
		if caseDef.ID == "" {
			return nil, fmt.Errorf("case definition has empty ID")
		}
		if i, exists := byID[caseDef.ID]; exists {
			merged, err := mergeCaseDescriptor(out[i], caseDef)
			if err != nil {
				return nil, err
			}
			out[i] = merged
			continue
		}
		byID[caseDef.ID] = len(out)
		out = append(out, caseDef)
	}
	return out, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isAtomTest(fn *ast.FuncDecl, testingAliases map[string]bool) bool {
	if fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Type.TypeParams != nil || (fn.Type.Results != nil && len(fn.Type.Results.List) > 0) {
		return false
	}
	suffix := strings.TrimPrefix(fn.Name.Name, "Test")
	if suffix != "" && unicode.IsLower([]rune(suffix)[0]) {
		return false
	}
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}
	param := fn.Type.Params.List[0]
	if len(param.Names) > 1 {
		return false
	}
	pointer, ok := param.Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "T" {
		return false
	}
	qualifier, ok := selector.X.(*ast.Ident)
	return ok && testingAliases[qualifier.Name]
}
