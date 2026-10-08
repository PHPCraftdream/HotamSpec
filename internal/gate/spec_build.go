// spec_build.go holds the data-collection AND rendering machinery behind
// docs/gen/SPEC.md: the generated NORMATIVE TEXT projection
// PLAN-scenario-generated-spec.md §2 D2/§3 W1.3 names — the successor stage
// to the authored-spec discipline's own projections (traceability.go/
// models.go/coverage.go, internal/generator): a requirement's claim (still
// the short AUTHORED intent from graph.json, D2 — never invented here)
// followed by the GENERATED prose narrative of its verified_by scenario(s),
// rendered from the ACTUAL Given/When/Then/Value steps a real, passing
// `go test` run just recorded via internal/recorder/canon's hotamspec API
// (PLAN-scenario-generated-spec.md §1's "text incarnates the actually-run
// test", never a second, independently-writable source of truth).
//
// LAYERING: collection and rendering live here so internal/invariants can
// consume SPEC freshness without importing internal/generator and closing its
// existing diagnose -> invariants dependency cycle. This package remains a
// leaf over loader, ontology, localization, and docbundle; generator and CLI
// entry points share the same rows/renderers through their wrappers.
//
// Unlike traceability.go/models.go/coverage.go, SPEC includes real execution
// evidence. Legacy scenario graphs record each verified_by test through
// RunVerifiedByTestRecording; self-executing atom graphs record each distinct
// source package once through RunAtomPackageRecording, preserving per-test and
// per-subtest verdicts/artifacts in one invocation-local snapshot.
//
// This file is read-only over the graph and re-EXECUTES the domain's own
// authored code via `go test` purely to observe what a real scenario
// narrated; it never mutates the graph, never writes to the domain's spec/
// tree. check_spec_md_current (internal/invariants/spec_md_current.go, W2.3)
// is the mechanical staleness gate that consumes this file's output; this
// file itself only renders what a fresh run reports NOW.
package gate

import (
	"encoding/json"
	"fmt"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// specArtifact is one parsed hotamspec.Artifact (internal/recorder/canon's
// JSON shape) read back from a RunVerifiedByTestRecording call -- this
// package's own local decode target so this file does not need to import
// internal/recorder/canon (gate already avoids that coupling for the same
// reason, see RecordedArtifact's doc comment: canon is vendored into
// consumer domains, never imported cross-module by the engine).
type specArtifact struct {
	ReqID        string                   `json:"req_id"`
	Test         string                   `json:"test"`
	Title        string                   `json:"title"`
	Steps        []specArtifactStep       `json:"steps"`
	Verdict      string                   `json:"verdict"`
	Mode         string                   `json:"mode,omitempty"`
	Observations []specObservation        `json:"observations,omitempty"`
	Case         *ontology.CaseDefinition `json:"-"`
}

type specArtifactStep struct {
	Kind         string                 `json:"kind"`
	Desc         string                 `json:"desc"`
	Values       []specArtifactKV       `json:"values,omitempty"`
	Passed       bool                   `json:"passed,omitempty"`
	Subject      string                 `json:"subject,omitempty"`
	Value        string                 `json:"value,omitempty"`
	Texts        ontology.LocalizedText `json:"-"`
	Observations []specObservation      `json:"observations,omitempty"`
}

type specArtifactKV struct {
	Key   string `json:"k"`
	Value string `json:"v"`
}

type specObservation struct {
	Name        string                  `json:"name"`
	Passed      bool                    `json:"passed"`
	RawInput    *ontology.ObservedValue `json:"raw_input,omitempty"`
	RawActual   *ontology.ObservedValue `json:"raw_actual,omitempty"`
	RawExpected *ontology.ObservedValue `json:"raw_expected,omitempty"`
}

// specTestOutcome separates passing, narratable artifacts from failed siblings.
// A package-level failure must not erase an independently passing subtest.
type specTestOutcome struct {
	entry           string
	artifacts       []specArtifact
	failedArtifacts []specArtifact
	problem         string
	sourceError     error
	passed          bool
}

// SpecRow is one rendered requirement section: its own id/claim (the D2
// short authored intent, taken verbatim from the graph, never invented
// here) plus every verified_by entry's recording outcome.
type SpecRow struct {
	req            ontology.Requirement
	outcomes       []specTestOutcome
	sourceError    error
	normativeTexts ontology.LocalizedText
	document       *specDocumentSnapshot
}

// ScenarioVerdict is one requirement's REAL (executed, `--spec`-gated)
// scenario-narrative outcome, derived from the same SpecRow/specTestOutcome
// data BuildSpecFromRows itself renders narratives from (see
// ScenarioVerdictsFromRows).
//
// HISTORICAL NOTE (task #317, superseded by task #370/RAC3-B's audit): this
// type was originally meant to feed an optional "verdict" sub-column on
// internal/generator.BuildTraceability/BuildCoverage (PLAN-scenario-
// generated-spec.md §3 W1.4) -- but task #317 deliberately REMOVED that
// overlay (see internal/generator/scenario_traceability_test.go's
// TestBuildTraceability_ModeIndependent_.../
// TestBuildCoverage_ModeIndependent_...) to keep those two docs pure,
// mode-independent functions of the graph plus a cheap AST scan, so `hotam
// land`'s routine (non---spec) regeneration can never flip their content --
// see cmd/hotam/gen_spec.go's own comment on specRows for the current,
// authoritative wiring (SPEC.md only). Neither BuildTraceability nor
// BuildCoverage has taken a verdicts parameter since; this type and
// ScenarioVerdictsFromRows are kept only because generator.ScenarioVerdict
// re-exports them as part of this package's public surface and a test
// exercises them directly to prove they have zero effect on generator
// output -- task #370's own RequirementState (internal/selfspec/
// requirement_state.go) is the CURRENT, correct home for "is this
// requirement's verified_by evidence real, passing, and fresh" as a
// general-purpose, execution-based predicate; a caller that genuinely needs
// a real per-requirement pass/fail+freshness verdict should use that instead
// of this narrower, SPEC.md-era shape.
type ScenarioVerdict struct {
	// Narrated is true when at least one verified_by entry produced at
	// least one hotamspec.Artifact with verdict "pass" -- the same bar
	// BuildSpecFromRows's own narratedCount uses.
	Narrated bool
	// AllEntriesPass is true when EVERY verified_by entry on the
	// requirement resolved to a currently-passing `go test` run (whether or
	// not it narrated a scenario) -- i.e. specTestOutcome.passed is true for
	// every entry. Deliberately independent of whether an entry narrated: a
	// PLAIN (non-scenario) verified_by test that genuinely passes still
	// counts as "passing" here (see specTestOutcome.passed's own doc
	// comment -- zero-trust finding: conflating "did not narrate" with "did
	// not pass" produced a false failure signal in TRACEABILITY.md's
	// verdict column for a requirement whose second verified_by entry was
	// simply a passing plain test). A requirement can have Narrated==true
	// and AllEntriesPass==false if it carries more than one verified_by
	// entry and one of them GENUINELY fails/errors/is skipped; callers that
	// want a single pass/fail verdict should treat
	// Narrated && AllEntriesPass as the strict "fully proven, fully
	// narrated" state.
	AllEntriesPass bool
}

// CollectSpecRows shares one explicitly closed invocation snapshot for scenario,
// atom, case and document graphs, covering every declared test reference.
func CollectSpecRows(g *ontology.Graph) map[string]SpecRow {
	if g == nil || (g.IsEmpty() && !hasSpecDocument(g)) {
		return map[string]SpecRow{}
	}
	session := NewExecutionSession()
	defer func() {
		if err := session.Close(); err != nil {
			panic(err)
		}
	}()
	snapshot, err := session.CollectAtomExecutionSnapshotForPackages(g, nil)
	if err != nil {
		panic(err)
	}
	return CollectSpecRowsFromSnapshot(g, snapshot)
}

// CollectSpecRowsFromSnapshot projects SPEC outcomes without executing tests.
func CollectSpecRowsFromSnapshot(g *ontology.Graph, snapshot *AtomExecutionSnapshot) map[string]SpecRow {
	rows := make(map[string]SpecRow, len(g.Requirements))
	if g.IsEmpty() && !hasSpecDocument(g) || snapshot == nil {
		return rows
	}
	var snapshotError error
	if g.SelfExecutingAtoms || hasSpecDocument(g) {
		snapshotError = snapshot.SourceErr
		if snapshotError == nil {
			snapshotError = snapshot.DiscoveryErr
		}
	}
	for _, requirement := range g.Requirements {
		if requirement.Status == ontology.StatusREJECTED {
			continue
		}
		references := specTestReferences(requirement)
		if len(references) == 0 && !hasSpecDocument(g) {
			continue
		}
		row := SpecRow{req: requirement, sourceError: snapshotError}
		if requirement.AtomKind == "rule" && snapshot.SourceIndex != nil {
			for _, reference := range requirement.ImplementedBy {
				if sources := snapshot.SourceIndex.byLink[reference]; len(sources) == 1 {
					row.normativeTexts = sources[0].Phrases
					break
				}
			}
		}
		for _, entry := range references {
			file, test, ok := ParseFileColonSymbol(strings.TrimSpace(entry))
			if !ok {
				row.outcomes = append(row.outcomes, specTestOutcome{entry: entry, problem: "malformed test or case reference"})
				continue
			}
			if snapshotError != nil {
				row.outcomes = append(row.outcomes, specTestOutcome{entry: entry, problem: snapshotError.Error()})
				continue
			}
			packageDir := snapshotPackageDir(file)
			result, found := snapshot.PackageRuns[packageDir]
			if !found {
				problem := "verified_by package was not present in the shared execution snapshot"
				row.outcomes = append(row.outcomes, specTestOutcome{entry: entry, problem: problem})
				continue
			}
			if g.SelfExecutingAtoms {
				outcome := atomRecordingOutcome(snapshot.SourceIndex, requirement, entry, test, result)
				if outcome.sourceError != nil && row.sourceError == nil {
					row.sourceError = outcome.sourceError
				}
				row.outcomes = append(row.outcomes, outcome)
			} else {
				row.outcomes = append(row.outcomes, scenarioRecordingOutcomeFromPackage(g, requirement, entry, test, result))
			}
		}
		rows[requirement.ID] = row
	}
	if hasSpecDocument(g) {
		rows[specDocumentRowKey] = SpecRow{document: collectSpecDocumentSnapshot(g, snapshot, rows)}
	}
	return rows
}

func snapshotFileInPackages(file string, packages []string) bool {
	if len(packages) == 0 {
		return true
	}
	file = filepath.ToSlash(filepath.Clean(filepath.FromSlash(file)))
	for _, pkg := range packages {
		pkg = strings.TrimSuffix(filepath.ToSlash(filepath.Clean(filepath.FromSlash(pkg))), "/")
		if file == pkg || strings.HasPrefix(file, pkg+"/") {
			return true
		}
	}
	return false
}

func addSnapshotTestReference(snapshot *AtomExecutionSnapshot, reference string) {
	file, test, ok := ParseFileColonSymbol(strings.TrimSpace(reference))
	if !ok {
		return
	}
	file = filepath.ToSlash(filepath.Clean(filepath.FromSlash(file)))
	dir := snapshotPackageDir(file)
	if old, exists := snapshot.PackageFiles[dir]; !exists || file < old {
		snapshot.PackageFiles[dir] = file
	}
	rootTest := strings.SplitN(test, "/", 2)[0]
	if rootTest == "" {
		return
	}
	if snapshot.TestFiles[dir] == nil {
		snapshot.TestFiles[dir] = make(map[string]string)
	}
	if old, exists := snapshot.TestFiles[dir][rootTest]; !exists || file < old {
		snapshot.TestFiles[dir][rootTest] = file
	}
}

func snapshotPackageDir(file string) string {
	return filepath.ToSlash(filepath.Dir(filepath.FromSlash(file)))
}

func sortedSnapshotDirs(files map[string]string) []string {
	dirs := make([]string, 0, len(files))
	for dir := range files {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

// discoverSnapshotAtomTests finds executable test files for the snapshot.
// With an empty packages list it walks the consumer spec/model tree; with the
// root-module list (§14) it walks each listed package directory instead.
func discoverSnapshotAtomTests(specRoot string, packages []string, snapshot *AtomExecutionSnapshot) error {
	walkRoots := []string{filepath.Join(specRoot, "spec", "model")}
	if len(packages) > 0 {
		walkRoots = walkRoots[:0]
		for _, pkg := range packages {
			walkRoots = append(walkRoots, filepath.Join(specRoot, filepath.FromSlash(pkg)))
		}
	}
	fileSet := token.NewFileSet()
	hasRuleCaseOption := false
	visit := func(walkRoot string) error {
		return filepath.WalkDir(walkRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relative, err := filepath.Rel(specRoot, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			dir := snapshotPackageDir(relative)
			file, err := parser.ParseFile(fileSet, path, nil, 0)
			if err != nil {
				return err
			}
			testingAliases := testingPackageAliases(file)
			recorderAliases := make(map[string]bool)
			recorderImportPath := ""
			if snapshot.SourceIndex != nil {
				recorderImportPath = snapshot.SourceIndex.RecorderImportPath
				recorderAliases = recorderPackageAliases(file, snapshot.SourceIndex.RecorderImportPath)
			}
			for _, declaration := range file.Decls {
				fn, ok := declaration.(*ast.FuncDecl)
				if !ok || !isSnapshotTest(fn, testingAliases) {
					continue
				}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					function := call.Fun
					if indexed, ok := function.(*ast.IndexExpr); ok {
						function = indexed.X
					}
					selector, ok := function.(*ast.SelectorExpr)
					if !ok || selector.Sel.Name != "WithCase" {
						return true
					}
					qualifier, ok := selector.X.(*ast.Ident)
					if ok && recorderAliases[qualifier.Name] {
						hasRuleCaseOption = true
					}
					return true
				})
				if !testBodyHasAtomCalls(fn.Body, file, recorderImportPath) {
					continue
				}
				if previous, exists := snapshot.TestFiles[dir][fn.Name.Name]; exists && previous != relative {
					return fmt.Errorf("duplicate test %s in %s and %s", fn.Name.Name, previous, relative)
				}
				addSnapshotTestReference(snapshot, relative+":"+fn.Name.Name)
			}
			return nil
		})
	}
	for _, walkRoot := range walkRoots {
		if err := visit(walkRoot); err != nil {
			return err
		}
	}
	if hasRuleCaseOption && snapshot.SourceIndex != nil && !snapshot.SourceIndex.ruleCases {
		return fmt.Errorf("hotamspec.WithCase requires conformance.rule_cases")
	}
	return nil
}

func recorderPackageAliases(file *ast.File, recorderImportPath string) map[string]bool {
	aliases := make(map[string]bool)
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil || path != recorderImportPath {
			continue
		}
		name := filepath.Base(path)
		if imported.Name != nil {
			name = imported.Name.Name
		}
		if name != "_" && name != "." {
			aliases[name] = true
		}
	}
	return aliases
}

func testingPackageAliases(file *ast.File) map[string]bool {
	aliases := make(map[string]bool)
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil || path != "testing" {
			continue
		}
		name := filepath.Base(path)
		if imported.Name != nil {
			name = imported.Name.Name
		}
		if name != "_" && name != "." {
			aliases[name] = true
		}
	}
	return aliases
}

func isSnapshotTest(fn *ast.FuncDecl, testingAliases map[string]bool) bool {
	if fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") ||
		fn.Type.TypeParams != nil || (fn.Type.Results != nil && len(fn.Type.Results.List) != 0) {
		return false
	}
	suffix := strings.TrimPrefix(fn.Name.Name, "Test")
	firstRune, _ := utf8.DecodeRuneInString(suffix)
	if unicode.IsLower(firstRune) {
		return false
	}
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 || len(fn.Type.Params.List[0].Names) > 1 {
		return false
	}
	pointer, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr)
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

// ScenarioVerdictsFromRows reduces a CollectSpecRows result to the
// requirement-keyed ScenarioVerdict map internal/generator's
// BuildTraceability/BuildCoverage accept, without touching the disk or
// spawning any further subprocess -- pure post-processing of data already
// collected.
func ScenarioVerdictsFromRows(rows map[string]SpecRow) map[string]ScenarioVerdict {
	verdicts := make(map[string]ScenarioVerdict, len(rows))
	for id, row := range rows {
		if id == specDocumentRowKey {
			continue
		}
		narrated := false
		allPass := true
		for _, o := range row.outcomes {
			if len(o.artifacts) > 0 {
				narrated = true
			}
			// AllEntriesPass reflects whether the test GENUINELY passed
			// (o.passed), not whether it narrated (o.problem != "" is also
			// true for a plain, passing, non-scenario test -- see
			// specTestOutcome.passed's doc comment for the zero-trust finding
			// this fixes).
			if !o.passed {
				allPass = false
			}
		}
		verdicts[id] = ScenarioVerdict{Narrated: narrated, AllEntriesPass: allPass}
	}
	return verdicts
}

// BuildSpec renders a single selected view from one fresh CollectSpecRows
// snapshot. Multilingual publication callers use BuildSpecDocumentsFromRows
// with a shared rows map so each language consumes identical evidence.
//
// BuildSpec itself is the convenient execution entry point; callers that also
// need other projections from the same snapshot should collect once and pass
// those rows to each renderer. No renderer persists verdicts or executes
// methods while changing language.

// firstImplementedByFile identifies the primary source package for a focused
// reading view. Test ownership is considered separately when no source exists.
func firstImplementedByFile(r ontology.Requirement) string {
	if len(r.ImplementedBy) == 0 {
		return ""
	}
	file, _, ok := ParseFileColonSymbol(strings.TrimSpace(r.ImplementedBy[0]))
	if !ok {
		return ""
	}
	return file
}

func scenarioRecordingOutcomeFromPackage(g *ontology.Graph, req ontology.Requirement, entry, test string, result RecordingResult) specTestOutcome {
	out := specTestOutcome{entry: entry}
	switch {
	case result.Skipped:
		out.problem = result.InfraWarning
		return out
	case result.Err != nil:
		out.problem = result.Err.Error()
		return out
	case result.CompileFailed:
		out.problem = "package does not compile"
		return out
	}
	executed, allPass := atomTestVerdict(result, test)
	out.passed = executed && allPass
	if !executed {
		if req.AtomKind == "rule" {
			out.problem = "declared rule case test was not executed"
		} else {
			out.problem = "test passes but recorded no hotamspec scenario (plain go test, no narrative to render)"
		}
		return out
	}
	var artifactProblem string
	for _, recorded := range result.Artifacts {
		var art specArtifact
		if json.Unmarshal(recorded.RawJSON, &art) != nil || (art.Test != test && !strings.HasPrefix(art.Test, test+"/")) {
			continue
		}
		if art.Mode == "" {
			if art.ReqID != req.ID {
				continue
			}
		} else {
			atom, err := DecodeAtomArtifact(recorded.RawJSON)
			if err != nil {
				artifactProblem = err.Error()
				continue
			}
			if atom.Mode != "rule" || req.AtomKind != "rule" ||
				g.Conformance == nil || !g.Conformance.RuleCases {
				if art.ReqID == req.ID {
					artifactProblem = fmt.Sprintf("atom %s requires self_executing_atoms and conformance.rule_cases discovery", art.ReqID)
				}
				continue
			}
			caseDef, err := atomCaseDefinition(atom, entry)
			if err != nil {
				artifactProblem = err.Error()
				continue
			}
			if art.ReqID != req.ID && !caseListsAtom(caseDef, req.ID) {
				continue
			}
			if caseDef, err = reconcileRecordedCase(req.Cases, *caseDef); err != nil {
				artifactProblem = err.Error()
				continue
			}
			art.Case = caseDef
		}
		if art.Verdict == "pass" {
			out.artifacts = append(out.artifacts, art)
		} else {
			out.failedArtifacts = append(out.failedArtifacts, art)
		}
	}
	if artifactProblem != "" {
		out.problem = artifactProblem
	}
	if !allPass && len(out.failedArtifacts) == 0 && out.problem == "" {
		out.problem = "test does not currently pass"
	}
	if len(out.artifacts) == 0 && len(out.failedArtifacts) == 0 && out.problem == "" {
		if req.AtomKind == "rule" {
			out.problem = "test passes but recorded no matching rule case evidence"
		} else {
			out.problem = "test passes but recorded no hotamspec scenario (plain go test, no narrative to render)"
		}
	}
	return out
}

func caseListsAtom(caseDef *ontology.CaseDefinition, atomID string) bool {
	for _, id := range caseDef.AtomIDs {
		if id == atomID {
			return true
		}
	}
	return false
}

func atomTestVerdict(result RecordingResult, test string) (executed, passed bool) {
	passed = true
	for _, verdict := range result.TestVerdicts {
		if verdict.Test != test && !strings.HasPrefix(verdict.Test, test+"/") {
			continue
		}
		executed = true
		if verdict.Verdict != "pass" {
			passed = false
		}
	}
	return executed, passed
}

func atomRecordingOutcome(sourceIndex *AtomSourceIndex, req ontology.Requirement, entry, test string, result RecordingResult) specTestOutcome {
	out := specTestOutcome{entry: entry}
	switch {
	case result.Skipped:
		out.problem = result.InfraWarning
		return out
	case result.Err != nil:
		out.problem = result.Err.Error()
		return out
	case result.CompileFailed:
		out.problem = "package does not compile"
		return out
	}
	executed, allPass := atomTestVerdict(result, test)
	out.passed = executed && allPass
	if !executed {
		out.problem = "verified_by test was not executed"
		return out
	}
	var artifactProblem string
	for _, recorded := range result.Artifacts {
		var art specArtifact
		if json.Unmarshal(recorded.RawJSON, &art) != nil || (art.Test != test && !strings.HasPrefix(art.Test, test+"/")) {
			continue
		}
		matches := art.ReqID == req.ID
		if !matches && art.Mode != "" && len(art.Steps) > 0 {
			source, err := sourceIndex.Resolve(art.Steps[0].Subject)
			if err == nil {
				matches = len(req.ImplementedBy) > 0 && req.ImplementedBy[0] == source.Link()
			}
		}
		if !matches {
			continue
		}
		for _, step := range art.Steps {
			art.Observations = append(art.Observations, step.Observations...)
		}
		var atom AtomArtifact
		if art.Mode != "" {
			var err error
			atom, err = DecodeAtomArtifact(recorded.RawJSON)
			if err != nil {
				artifactProblem = err.Error()
				continue
			}
			if atom.Mode == "rule" && !sourceIndex.ruleCases {
				artifactProblem = fmt.Sprintf("rule atom %s requires conformance.rule_cases", atom.ReqID)
				continue
			}
			if atom.Mode == "rule" {
				caseDef, err := atomCaseDefinition(atom, entry)
				if err != nil {
					artifactProblem = err.Error()
					continue
				}
				if caseDef, err = reconcileRecordedCase(req.Cases, *caseDef); err != nil {
					artifactProblem = err.Error()
					continue
				}
				art.Case = caseDef
			}
		}
		if art.Verdict == "pass" {
			if art.Mode != "" {
				steps, err := humanizeAtomSteps(sourceIndex, atom)
				if err != nil {
					out.sourceError = err
					return out
				}
				art.Title = steps[0].Desc
				art.Steps = steps
			}
			out.artifacts = append(out.artifacts, art)
		} else {
			// A failed atom is a diagnostic: when its text cannot be derived (e.g. a
			// false bool without a `not:` phrase) it keeps its recorded steps instead
			// of blocking the whole SPEC. Passing atoms stay strict above.
			if art.Mode != "" {
				if steps, err := humanizeAtomSteps(sourceIndex, atom); err == nil {
					art.Steps = steps
				}
			}
			out.failedArtifacts = append(out.failedArtifacts, art)
		}
	}
	if artifactProblem != "" {
		out.problem = artifactProblem
	}
	if !allPass && len(out.failedArtifacts) == 0 && out.problem == "" {
		out.problem = "verified_by test or a nested subtest failed"
	}
	if len(out.artifacts) == 0 && len(out.failedArtifacts) == 0 && out.problem == "" {
		out.problem = "verified_by test passed but recorded no matching atom evidence"
	}
	return out
}

func humanizeAtomSteps(index *AtomSourceIndex, atom AtomArtifact) ([]specArtifactStep, error) {
	passing := atom
	if passing.Verdict != "pass" {
		passing.Verdict = "pass"
	}
	claims, err := index.DeriveClaims(passing)
	if err != nil {
		return nil, err
	}
	primary := index.primaryLanguage()
	claim, ok := claims[primary]
	if !ok || strings.TrimSpace(claim) == "" {
		return nil, fmt.Errorf("atom %s has no primary claim text for language %q", atom.ReqID, primary)
	}
	steps := []specArtifactStep{{Kind: "then", Desc: claim, Passed: atom.Verdict == "pass", Texts: claims}}
	if len(atom.Steps) == 0 || (atom.Verdict == "pass" && atom.Mode == "fact") {
		return steps, nil
	}
	evidence := atom.Steps[1:]
	seen := map[string]bool{atom.Steps[0].Subject: true}
	if atom.Verdict != "pass" {
		evidence = atom.Steps
		seen = map[string]bool{}
	}
	for _, observed := range evidence {
		if seen[observed.Subject] {
			continue
		}
		seen[observed.Subject] = true
		localized, err := index.DeriveClaims(AtomArtifact{ReqID: atom.ReqID, Mode: "fact", Verdict: "pass", Steps: []AtomStep{observed}})
		if err != nil {
			return nil, err
		}
		primaryText := localized[primary]
		if strings.TrimSpace(primaryText) == "" {
			return nil, fmt.Errorf("atom evidence has no primary text for language %q", primary)
		}
		steps = append(steps, specArtifactStep{Kind: "given", Desc: primaryText, Subject: observed.Subject, Value: observed.Value, Texts: localized})
	}
	return steps, nil
}

func atomCaseDefinition(atom AtomArtifact, entry string) (*ontology.CaseDefinition, error) {
	file, _, ok := ParseFileColonSymbol(strings.TrimSpace(entry))
	if !ok {
		return nil, fmt.Errorf("malformed verified_by entry")
	}
	if strings.TrimSpace(atom.Test) == "" {
		return nil, fmt.Errorf("rule atom %s artifact omits executed test identity", atom.ReqID)
	}
	return atom.CaseDefinition(file + ":" + atom.Test)
}

func reconcileRecordedCase(declared []ontology.CaseDefinition, recorded ontology.CaseDefinition) (*ontology.CaseDefinition, error) {
	for _, candidate := range declared {
		if candidate.ID != recorded.ID {
			continue
		}
		merged, err := mergeRecordedCase(candidate, recorded)
		if err != nil {
			return nil, err
		}
		return &merged, nil
	}
	return &recorded, nil
}

func mergeRecordedCase(declared, recorded ontology.CaseDefinition) (ontology.CaseDefinition, error) {
	if declared.ID != recorded.ID {
		return ontology.CaseDefinition{}, fmt.Errorf("case descriptor ID %q conflicts with recorded case %q", declared.ID, recorded.ID)
	}
	out := recorded
	mergeString := func(field string, saved *string, actual string) error {
		if *saved != "" && actual != "" && *saved != actual {
			return fmt.Errorf("case %s recorded %s conflicts with graph descriptor", declared.ID, field)
		}
		return nil
	}
	for _, field := range []struct {
		name  string
		saved *string
		value string
	}{
		{"test", &out.Test, declared.Test},
		{"profile", &out.Profile, declared.Profile},
		{"target", &out.Target, declared.Target},
		{"operation", &out.Operation, declared.Operation},
		{"producer", &out.Producer, declared.Producer},
	} {
		if err := mergeString(field.name, field.saved, field.value); err != nil {
			return ontology.CaseDefinition{}, err
		}
		if *field.saved == "" {
			*field.saved = field.value
		}
	}
	mergeSlice := func(field string, saved, actual any, savedEmpty, actualEmpty bool) error {
		if !savedEmpty && !actualEmpty && !reflect.DeepEqual(saved, actual) {
			return fmt.Errorf("case %s recorded %s conflicts with graph descriptor", declared.ID, field)
		}
		return nil
	}
	if err := mergeSlice("atom_ids", out.AtomIDs, declared.AtomIDs, len(out.AtomIDs) == 0, len(declared.AtomIDs) == 0); err != nil {
		return ontology.CaseDefinition{}, err
	}
	if len(out.AtomIDs) == 0 {
		out.AtomIDs = append([]string(nil), declared.AtomIDs...)
	}
	if out.ClauseIDs != nil && declared.ClauseIDs != nil && !reflect.DeepEqual(out.ClauseIDs, declared.ClauseIDs) {
		return ontology.CaseDefinition{}, fmt.Errorf("case %s recorded clause_ids conflicts with graph descriptor", declared.ID)
	}
	if out.ClauseIDs == nil && declared.ClauseIDs != nil {
		out.ClauseIDs = make([]string, len(declared.ClauseIDs))
		copy(out.ClauseIDs, declared.ClauseIDs)
	}
	if err := mergeSlice("fixtures", out.Fixtures, declared.Fixtures, len(out.Fixtures) == 0, len(declared.Fixtures) == 0); err != nil {
		return ontology.CaseDefinition{}, err
	}
	if len(out.Fixtures) == 0 {
		out.Fixtures = append([]ontology.FixtureRef(nil), declared.Fixtures...)
	}
	if err := mergeSlice("conditions", out.Conditions, declared.Conditions, len(out.Conditions) == 0, len(declared.Conditions) == 0); err != nil {
		return ontology.CaseDefinition{}, err
	}
	if len(out.Conditions) == 0 {
		out.Conditions = append([]ontology.ConditionEvidence(nil), declared.Conditions...)
	}
	if err := mergeSlice("sides", out.Sides, declared.Sides, len(out.Sides) == 0, len(declared.Sides) == 0); err != nil {
		return ontology.CaseDefinition{}, err
	}
	if len(out.Sides) == 0 {
		out.Sides = append([]string(nil), declared.Sides...)
	}
	if out.Input != nil && declared.Input != nil && !ontology.EqualObservedValues(out.Input, declared.Input) {
		return ontology.CaseDefinition{}, fmt.Errorf("case %s recorded input conflicts with graph descriptor", declared.ID)
	}
	if out.Input == nil && declared.Input != nil {
		out.Input = ontology.CloneObservedValue(declared.Input)
	}
	if out.Expected != nil && declared.Expected != nil && !ontology.EqualObservedValues(out.Expected, declared.Expected) {
		return ontology.CaseDefinition{}, fmt.Errorf("case %s recorded expected value conflicts with graph descriptor", declared.ID)
	}
	if out.Expected == nil && declared.Expected != nil {
		out.Expected = ontology.CloneObservedValue(declared.Expected)
	}
	if out.Selection != nil && declared.Selection != nil && !reflect.DeepEqual(*out.Selection, *declared.Selection) {
		return ontology.CaseDefinition{}, fmt.Errorf("case %s recorded selection conflicts with graph descriptor", declared.ID)
	}
	if out.Selection == nil && declared.Selection != nil {
		copy := *declared.Selection
		out.Selection = &copy
	}
	return out, nil
}

// SpecPackage identifies the source package that owns a requirement.
