// model_complete.go holds check_model_complete
// (PLAN-scenario-generated-spec.md §2 D5, task W2.4): the MODEL-LEVEL
// completeness gate for the scenario-generated-spec discipline. D5 frames
// authored-spec layering (general -> specific: models -> fields -> methods ->
// scenario tests) NOT as a commit-ordering rule (the gate cannot see git
// history) but as a final-STATE COMPLETENESS property over each authored
// model object: a model bound to the discipline is COMPLETE only when every
// one of its EXPORTED methods that is CITED as an implemented_by entry of
// some SETTLED requirement has at least one scenario-narrated verified_by
// test behind it. A model that is half-bound -- one cited method already
// proven by a hotamspec scenario, another cited method still carried only by
// a plain (non-narrating) test -- is exactly the "incomplete object" D5
// names, and this check surfaces it as a MODEL-centric diagnostic (which
// object, which method, what is missing), not merely as a per-requirement
// scenario-absence violation (which check_settled_requires_scenario already
// reports separately, at the requirement granularity).
package invariants

import (
	"fmt"
	"go/token"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkModelComplete is the model-level completeness gate. SCOPE: only a
// domain whose manifest.json declares discipline:"full" (g.Discipline ==
// loader.DisciplineFull) -- identical opt-in boundary to
// check_settled_requires_scenario (W2.1). A domain that has not opted in
// (every domain today; this wave deliberately does not flip discipline:full
// on any real domain) contributes ZERO violations from this check,
// regardless of how bare or partial its models' cited methods are.
//
// MODEL COMPLETENESS DEFINITION (D5): for every SETTLED requirement citing
// an implemented_by entry that resolves to an EXPORTED METHOD of one of the
// domain's authored model objects (Go struct/type in spec/, as inventoried
// by gate.ScanAuthoredModels -- the SAME scan BuildModels/ScanModelLayerCounts
// render from, excluding the vendored recorder), that method is
// "scenario-complete" iff AT LEAST ONE requirement that cites it has at
// least one verified_by entry resolving to a test whose body calls
// hotamspec.NewScenario(...) (gate.ResolveSpecTest's HasScenario -- the SAME
// AST-only signal check_settled_requires_scenario and coverage.go's
// computeScenarioRatchet use). A model object is COMPLETE iff every one of
// its cited exported methods is scenario-complete; if ANY cited method has
// no scenario behind it, the model is INCOMPLETE and this check fires one
// violation naming the object, every uncovered method, and the requirement
// IDs that cite each.
//
// RELATION TO check_settled_requires_scenario (W2.1) and
// check_scenario_executes_impl (W2.2): this check is deliberately a
// REGROUPING, not a re-implementation, of those two. W2.1 asks "does each
// SETTLED requirement (authored path) have ANY scenario-narrated
// verified_by?" at the REQUIREMENT granularity; W2.2 asks "is each
// implemented_by symbol actually executed by a covering verified_by test?"
// (scenario OR plain) via a real coverage run. THIS check asks the
// orthogonal, MODEL-centric question W2.1's per-requirement framing cannot
// surface: "is this MODEL OBJECT half-built -- one method scenario-proven,
// another cited-but-narratively-dangling?" It reuses W2.1's exact
// anyVerifiedByEntryHasScenario signal (no new coverage run, no second AST
// pass over verified_by) and gate.ScanAuthoredModels' object inventory (no
// second spec/ walk), so the three checks share one scenario-detection
// logic and one model-scan choke point. The genuinely new value is the
// model-grouped diagnostic: a steward building models general->specific
// sees "model Risk is incomplete: method Validate has no scenario", not
// three unrelated per-requirement violations that happen to share an
// object they would have to reconstruct by hand.
//
// MULTI-METHOD, SINGLE REQUIREMENT (honest collapse): a requirement that
// cites TWO implemented_by methods ([O.m1, O.m2]) and carries ONE
// scenario-narrated verified_by test marks BOTH m1 and m2
// scenario-complete here -- the requirement-level signal is "this
// requirement has a scenario", attributed to every method it cites. This
// is NOT a gap: W2.1 is satisfied for that requirement, and W2.2's
// coverage-proof independently guarantees the scenario test actually
// executes BOTH m1 and m2's declaration lines, so both methods genuinely
// ARE scenario-proven via that one test. The gap this check catches that
// W2.1 frames only per-requirement is the CROSS-REQUIREMENT case: when
// DIFFERENT requirements cite different methods of the same model and one
// of those requirements lacks a scenario -- the model is then a
// half-bound object, and a model-centric diagnostic is the actionable
// view.
//
// HONEST NO-OP for a model with no cited method: a model object none of
// whose exported methods is cited as implemented_by by ANY SETTLED
// requirement is simply not in this check's scope (it has no
// discipline-binding citation to be incomplete about) -- it contributes no
// violation, mirroring W2.1/W2.2's identical "no implemented_by, nothing
// to check" boundary for an unbound symbol.
func checkModelComplete(g *ontology.Graph) []Violation {
	if g.Discipline != loader.DisciplineFull {
		// Soft discipline (the default, and every domain in this wave) --
		// honest no-op. See doc comment above and loader.DisciplineFull.
		return nil
	}

	files, err := gate.ScanAuthoredModels(g)
	if err != nil {
		// A scan failure (unreadable spec/ tree) is not this check's
		// violation to diagnose -- it would already surface as a
		// generation error via BuildModels/ScanModelLayerCounts. Honest
		// no-op here, mirroring those callers' own scanErr branch.
		return nil
	}
	if len(files) == 0 {
		// No authored models at all -- nothing to be incomplete. Honest
		// no-op (a discipline:full domain with zero models is
		// gpsm-sm-shaped: its SETTLED requirements' authored-path
		// obligation is W2.1's to report, not this model-level check's).
		return nil
	}

	specRoot := gate.SpecRootForGraph(g)

	// citedMethod maps an object name -> cited method name -> aggregated
	// info across every SETTLED requirement citing that (object, method)
	// pair. anyScenario is the OR of every citing requirement's
	// anyVerifiedByEntryHasScenario: if ANY citing requirement carries a
	// scenario-narrated verified_by test, the method is scenario-complete.
	type citedMethodAgg struct {
		file        string // domain-relative file of the implemented_by citation (diagnostic)
		reqIDs      []string
		anyScenario bool
	}
	cited := map[string]map[string]*citedMethodAgg{}

	for _, r := range g.Requirements {
		if r.Status != ontology.StatusSETTLED {
			// Only SETTLED requirements carry the discipline:full
			// authored-path obligation (mirrors W2.1's own SETTLED filter:
			// a DRAFT requirement's exploration cannot make a model
			// "incomplete" before it has even settled).
			continue
		}
		reqHasScenario := anyVerifiedByEntryHasScenario(specRoot, g.SelfHosting, r.VerifiedBy)
		for _, ie := range parseSpecEntries(r.ImplementedBy) {
			if !ie.ok {
				// Malformed implemented_by shape -- checkImplementedBySymbolResolvable's
				// violation, not this check's.
				continue
			}
			if ok, _ := gate.EntryWithinSpecScope(specRoot, ie.file, g.SelfHosting); !ok {
				// Out-of-scope citation -- checkImplementedBySymbolResolvable's
				// violation, not this check's.
				continue
			}
			objName, methodName, found := matchCitedExportedMethod(files, ie.file, ie.symbol)
			if !found {
				// Not an exported method of a scanned model object (a
				// top-level function, a type-only citation, an unexported
				// method, or a citation into a file the scan did not
				// parse) -- not this MODEL-level check's scope. Either it
				// is another check's violation to report, or it is a
				// legitimately non-model carrier (a package-level
				// constructor function, for instance).
				continue
			}
			bucket := cited[objName]
			if bucket == nil {
				bucket = map[string]*citedMethodAgg{}
				cited[objName] = bucket
			}
			agg := bucket[methodName]
			if agg == nil {
				agg = &citedMethodAgg{file: ie.file}
				bucket[methodName] = agg
			}
			agg.reqIDs = append(agg.reqIDs, r.ID)
			if reqHasScenario {
				agg.anyScenario = true
			}
		}
	}

	if len(cited) == 0 {
		return nil
	}

	// Deterministic output: objects sorted by name, methods within an
	// object sorted by name, requirement IDs de-duplicated + sorted.
	objNames := make([]string, 0, len(cited))
	for n := range cited {
		objNames = append(objNames, n)
	}
	sort.Strings(objNames)

	var out []Violation
	for _, objName := range objNames {
		bucket := cited[objName]
		methodNames := make([]string, 0, len(bucket))
		for m := range bucket {
			methodNames = append(methodNames, m)
		}
		sort.Strings(methodNames)

		var uncovered []string
		for _, m := range methodNames {
			agg := bucket[m]
			if agg.anyScenario {
				continue
			}
			reqIDs := dedupSortedStrings(agg.reqIDs)
			uncovered = append(uncovered, fmt.Sprintf(
				"%s (cited by %s as implemented_by in %s; none of those requirements has a "+
					"verified_by entry resolving to a test calling hotamspec.NewScenario)",
				m, strings.Join(reqIDs, ", "), agg.file))
		}
		if len(uncovered) == 0 {
			continue
		}
		out = append(out, Violation{
			Check: "check_model_complete",
			ID:    objName,
			Message: fmt.Sprintf(
				"model %s is incomplete (PLAN-scenario-generated-spec.md §2 D5): %d of its cited "+
					"exported implemented_by method(s) have no scenario-narrated verified_by test behind them -- "+
					"%s; bring each uncovered method under a hotamspec scenario (add a scenario-narrated "+
					"verified_by test to a citing requirement, or drop the implemented_by citation if the "+
					"method is not actually a discipline carrier)",
				objName, len(uncovered), strings.Join(uncovered, "; ")),
		})
	}
	return out
}

// matchCitedExportedMethod resolves a cited implemented_by "file:symbol"
// entry against the scanned authored-model inventory and reports, when the
// citation names an EXPORTED METHOD of one of the file's model objects, the
// owning object's name and the method name. Returns found=false for
// anything that is not an exported method of a scanned model object -- a
// top-level function, a type declaration, an unexported method, or a
// citation into a file the scan did not parse (e.g. an out-of-scope path).
//
// Matching rules mirror gate.ResolveSpecSymbol's own documented convention
// for implemented_by symbol names (spec_resolver.go):
//   - a QUALIFIED "Type.Method" symbol matches an object named "Type" in the
//     named file that declares an exported method "Method";
//   - a BARE "Method" symbol matches the FIRST object in the named file
//     (declaration order) that declares an exported method "Method" --
//     matching ResolveSpecSymbol's "any receiver" semantics for bare names
//     (authored spec code rarely has two same-named exported methods on
//     different receivers in one file; the qualified form exists for that
//     rare ambiguity).
//
// fileRel is the citation's domain-relative file path (forward-slash, as
// authored); each scanned ModelFile.RelPath is root-relative forward-slash
// -- both are normalized via filepath.ToSlash(filepath.Clean(...)) before
// comparison so platform separator/cleaning differences cannot cause a
// spurious mismatch.
func matchCitedExportedMethod(files []gate.ModelFile, fileRel, symbol string) (objName, methodName string, found bool) {
	wantFile := normalizeRelPath(fileRel)
	wantType, wantName, qualified := splitQualifiedCitationSymbol(symbol)

	for _, f := range files {
		if normalizeRelPath(f.RelPath) != wantFile {
			continue
		}
		for _, obj := range f.Objects {
			if qualified && obj.Name != wantType {
				continue
			}
			for _, m := range obj.Methods {
				if m.Name != wantName {
					continue
				}
				if !token.IsExported(m.Name) {
					// Unexported methods are out of scope for this check
					// (D5: "ЭКСПОРТИРУЕМЫЙ метод"). Keep scanning -- a
					// different object in the same file may declare an
					// exported method of the same name.
					continue
				}
				return obj.Name, m.Name, true
			}
		}
		return "", "", false
	}
	return "", "", false
}

// splitQualifiedCitationSymbol splits an implemented_by symbol of the form
// "Type.Method" into its two parts (mirroring gate.splitQualifiedSymbol,
// which is unexported and lives in spec_resolver.go -- not re-exported to
// avoid widening gate's surface for this one caller). If symbol contains no
// ".", qualified is false and name is the whole symbol.
func splitQualifiedCitationSymbol(symbol string) (typeName, name string, qualified bool) {
	if idx := strings.LastIndex(symbol, "."); idx >= 0 {
		return symbol[:idx], symbol[idx+1:], true
	}
	return "", symbol, false
}

// normalizeRelPath cleans a domain-relative path and forces forward slashes
// so an authored implemented_by file ("spec/model/risk.go") compares equal
// to a scanned ModelFile.RelPath regardless of OS separator or minor
// cleaning differences.
func normalizeRelPath(p string) string {
	return filepath.ToSlash(filepath.Clean(p))
}

// dedupSortedStrings returns the sorted, de-duplicated copy of in -- used
// so a method cited by the same requirement via two implemented_by entries
// (or cited once each by two requirements) renders its citing-requirement
// list deterministically and without repeats.
func dedupSortedStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

var _ = All.MustRegister("check_model_complete", Invariant{
	Name:  "check_model_complete",
	Canon: methodology.Domain,
	Claim: "in a discipline:full domain, every authored model object that has at least one exported method cited as " +
		"implemented_by by a SETTLED requirement is COMPLETE: each such cited method is backed by at least one " +
		"scenario-narrated verified_by test (a test calling hotamspec.NewScenario) on some requirement citing it; " +
		"a domain without discipline:full is an honest no-op, and a model with no cited method is out of scope.",
	Rule: "IF g.Discipline == loader.DisciplineFull (the domain's manifest.json declares \"discipline\": \"full\"), THEN " +
		"compute the domain's authored model inventory via gate.ScanAuthoredModels (the SAME scan BuildModels/" +
		"ScanModelLayerCounts render from, excluding the vendored recorder). For every Requirement with status == " +
		"SETTLED whose implemented_by entries are parsed via parseSpecEntries and in-scope via gate.EntryWithinSpecScope, " +
		"resolve each entry against the scanned inventory (matchCitedExportedMethod): a QUALIFIED \"Type.Method\" " +
		"symbol matches an object named Type declaring an exported method Method in the named file; a BARE \"Method\" " +
		"symbol matches the first object in the named file declaring an exported method Method. For each (object, " +
		"method) cited by at least one SETTLED requirement, the method is scenario-complete iff AT LEAST ONE citing " +
		"requirement has any verified_by entry resolving (gate.ResolveSpecTest) to a real Test* function whose body " +
		"calls hotamspec.NewScenario(...) anywhere (gate.SpecTestResult.HasScenario) -- the SAME signal " +
		"check_settled_requires_scenario's anyVerifiedByEntryHasScenario already computes. A model object is COMPLETE " +
		"iff EVERY one of its cited exported methods is scenario-complete; otherwise this check fires ONE violation " +
		"naming the object, every uncovered method, and the requirement IDs citing each. IF g.Discipline is NOT " +
		"loader.DisciplineFull, this check is a pure HONEST NO-OP: zero violations regardless of model state. A model " +
		"with no cited exported method is also a NO-OP for that model (no discipline-binding citation to be incomplete).",
	Why: "PLAN-scenario-generated-spec.md §2 D5 (task W2.4): authored-spec layering (general -> specific: models -> " +
		"fields -> methods -> scenario tests) is enforced as a final-STATE COMPLETENESS property, not a commit-ordering " +
		"rule (the gate cannot see git history). check_settled_requires_scenario (W2.1) already demands a scenario per " +
		"SETTLED requirement at the REQUIREMENT granularity, and check_scenario_executes_impl (W2.2) already proves each " +
		"implemented_by symbol is really executed by a covering test -- but NEITHER surfaces the MODEL-centric view D5 " +
		"names: an object half-bound to the discipline (one cited method scenario-proven, another cited-but-" +
		"narratively-dangling) reads as a complete object in every per-requirement view, even though it is exactly the " +
		"kind of incomplete general->specific progression D5 polices. This check regroups the SAME W2.1 scenario signal " +
		"and the SAME gate.ScanAuthoredModels inventory (no new coverage run, no second spec/ walk) by OWNING OBJECT, " +
		"so a steward building models general->specific sees 'model Risk is incomplete: method Validate has no scenario' " +
		"-- an actionable, object-centered diagnostic -- instead of reconstructing the half-bound object by hand from " +
		"scattered per-requirement violations. SCOPE: only a discipline:full domain's SETTLED requirements' cited " +
		"EXPORTED methods -- every domain today is an honest no-op (this wave deliberately flips discipline:full on no " +
		"real domain), so prat/gpsm-sm/hotam-spec-self see zero new violations from this check landing. RELATION to " +
		"W2.1/W2.2: deliberately a REGROUPING (reuses anyVerifiedByEntryHasScenario + ScanAuthoredModels verbatim), " +
		"not a re-implementation -- the single, shared scenario-detection logic and the single, shared model-scan choke " +
		"point both hold; the new value is the model-grouped diagnostic and the per-method coverage attribution, " +
		"neither of which W2.1's requirement-level framing provides.",
	Check: checkModelComplete,
})
