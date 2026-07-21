// scenario_coverage.go holds check_scenario_executes_impl
// (PLAN-scenario-generated-spec.md §2 D3, task W2.2): the COVERAGE-PROOF
// gate that closes the one hole every other authored-link check
// (authored_links.go, scenario_discipline.go) deliberately leaves open --
// none of them ever confirm the verified_by test's run actually touched the
// implemented_by symbol's OWN lines. A verified_by test can resolve, have
// teeth, avoid an unconditional skip, narrate a hotamspec scenario, AND pass
// when run, while still asserting a TAUTOLOGY or exercising a completely
// unrelated symbol -- every AST-only signal (HasTeeth/HasScenario) and even
// the execution signal (check_verified_by_test_passes, authored_links.go)
// stay green for that forged pairing, because none of them ever asks "did
// this SPECIFIC run cover THESE SPECIFIC lines". This check asks exactly
// that, mechanically, via a real coverage profile from the SAME kind of run
// RunVerifiedByTestRecording already performs for W1.2/W1.3's artifact
// pipeline.
package invariants

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/loader"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// checkScenarioExecutesImpl is the coverage-proof gate. SCOPE: only the
// AUTHORED path -- a requirement with a non-empty implemented_by AND a
// non-empty verified_by (the same pre-filter checkEnforcedRequiresEnforcerOrAuthoredLink's
// authored half and checkSettledRequiresScenario's authored branch both use).
// A requirement on the ENGINE path (enforced_by, no implemented_by/
// verified_by pair) is untouched -- this is a NO-OP for it, exactly the same
// honesty boundary every check in authored_links.go already documents for an
// empty implemented_by/verified_by (that file's own header comment). Applies
// regardless of Status/Enforcement/discipline: an authored implemented_by+
// verified_by pair claims "embodied HERE, proven HERE" the moment both are
// written, independent of whether the domain has opted into discipline:full
// (checkSettledRequiresScenario's OPT-IN gate) or whether the requirement
// happens to be ENFORCED yet -- a forged pairing is exactly as false on a
// SETTLED-but-not-yet-ENFORCED requirement as on an ENFORCED one.
//
// F1 GATE (task W7.2, @fx finding F1): the coverage-proof above is scoped to
// implemented_by entries that resolve to a function/method (a coverable
// range). A type-only entry (struct/interface declaration) deliberately has
// no coverable range (ResolveSpecSymbolRange returns found=false), so it
// produces no coverage job. Before F1, a SETTLED discipline:full requirement
// whose ENTIRE implemented_by set was type-only produced zero jobs and passed
// this gate having proven ZERO execution -- a silent bypass of the
// coverage-proof the same requirement's discipline:full promise demands. The
// F1 gate closes this: when such a requirement's implemented_by set contains
// at least one type-only entry AND zero coverable entries, this check fires
// a violation directly (no coverage run needed -- there is nothing to run
// coverage against). Scoped to SETTLED + discipline:full because only those
// requirements have made the one-way promise that their implementation is
// fully scenario-proven; a requirement citing a type for documentation in a
// soft-discipline domain has not. Does NOT fire when ALL entries are
// unresolvable (checkImplementedBySymbolResolvable's violation) -- the
// type-only distinction (ResolveSpecSymbol's SpecSymbolType) is what
// separates F1's concern from the unresolvable case.
//
// SEMANTICS -- why "each implemented_by symbol covered by AT LEAST ONE
// verified_by test" rather than an index-paired 1:1 correspondence: task
// instructions and every existing authored-link check (parseSpecEntries,
// checkVerifiedByNoUnrelatedReuse's own doc comment) treat implemented_by and
// verified_by as two INDEPENDENT lists -- there is no positional convention
// anywhere in this codebase that verified_by[i] proves implemented_by[i]
// specifically (a requirement can, and in the real pilot domain does, name
// TWO implemented_by methods proven collectively by TWO verified_by tests
// that do not individually map 1:1 -- see domains/prat's R-forecast-three-
// versions: implemented_by=[RequireComplete, RecordVersion], verified_by=
// [TestRequireComplete_NeedsAllThreeVersions, TestRecordVersion_
// RejectsDuplicateRecording] -- each verified_by test in that pair actually
// happens to cover ITS matching implemented_by method, but nothing in the
// graph schema promises that alignment, and a requirement could equally
// legitimately have ONE broad verified_by test that exercises BOTH
// implemented_by methods together). The only semantics that does not invent
// an unstated convention is: EVERY implemented_by symbol must be covered by
// AT LEAST ONE of the requirement's verified_by tests (existential over the
// verified_by set, universal over the implemented_by set) -- this is exactly
// "every embodiment claim is proven by something in the proof set", the
// weakest claim consistent with both lists being independent, and it still
// catches the forge this task's own adversarial probe targets: a
// verified_by test that covers NEITHER (or a DIFFERENT, unrelated) symbol
// leaves at least one implemented_by entry with zero covering test, which
// fires.
//
// COST: real, per requirement in scope, this check runs EVERY verified_by
// test of that requirement in RunVerifiedByTestRecording's record-mode
// (a real `go test -coverprofile` subprocess) at least once -- there is no
// cheaper AST-only substitute for "did this run touch these lines", by
// construction (that is the whole point of a coverage PROOF over an AST
// guess). This mirrors checkVerifiedByTestPasses's own already-accepted cost
// (authored_links.go's PERFORMANCE doc comment): that check already pays for
// one real `go test` subprocess per verified_by entry in EVERY all-violations
// call. This check does NOT reuse checkVerifiedByTestPasses's run, because
// that run uses PLAIN RunVerifiedByTest (no -coverprofile, no -coverpkg --
// adding coverage flags to every verified_by run unconditionally would
// slow down the always-on pass/fail gate for domains that never ask for
// coverage-proof at all); instead it pays its OWN, separate recording-mode
// subprocess cost, bounded by the SAME worker pool shape
// (runVerifiedByTestJobs/runExecWorkers) and de-duplicated within one
// AllViolations call via coverageRunCache below so a verified_by test cited
// by several implemented_by symbols (or several requirements, per
// checkVerifiedByNoUnrelatedReuse's own reuse-detector) is only actually
// RUN once, not once per symbol it happens to be checked against.
func checkScenarioExecutesImpl(g *ontology.Graph) []Violation {
	specRoot := gate.SpecRootForGraph(g)
	var out []Violation

	type job struct {
		reqID   string
		implRaw string
		symRng  gate.SymbolRange
		imports string
		tests   []specFileEntry
	}
	var jobs []job

	for _, r := range g.Requirements {
		if len(r.ImplementedBy) == 0 || len(r.VerifiedBy) == 0 {
			continue
		}
		implEntries := parseSpecEntries(r.ImplementedBy)
		testEntries := eligibleVerifiedByEntries(specRoot, g.SelfHosting, r.VerifiedBy)
		if len(testEntries) == 0 {
			// No verified_by entry resolves to a real, non-vacuous, non-skipped
			// test at all -- that is checkVerifiedByTestResolvable/
			// checkVerifiedByTestHasTeeth/checkVerifiedByTestNoSkip's violation
			// to report, not this check's. Nothing to run coverage against.
			continue
		}
		// F1 (task W7.2, @fx finding F1): track whether ANY implemented_by
		// entry of this requirement resolved to a coverable function/method
		// range (a job this check can actually run coverage against). A
		// requirement whose ENTIRE implemented_by set consists of type-only
		// declarations (struct/interface -- ResolveSpecSymbolRange returns
		// found=false for them deliberately, since a type has no executable
		// lines a test could cover) produces ZERO jobs and would otherwise
		// pass this coverage-proof gate having proven ZERO execution -- the
		// exact silent bypass F1 targets. typeOnlyCount distinguishes a
		// type-only entry (resolves as SpecSymbolType -- the resolver intended
		// a real citation, it just has nothing coverable) from an unresolvable
		// entry (SpecSymbolNone -- checkImplementedBySymbolResolvable's
		// violation, not this check's), so this fix only fires when the
		// requirement has at least one type-only entry AND no coverable entry
		// at all (the all-type/all-uncoverable case), never for a legitimate
		// mixed citation (type for context + method for the real logic, where
		// the method's range does get covered and a job IS created).
		var coverableCount int
		var typeOnlyEntries []string
		for _, ie := range implEntries {
			if !ie.ok {
				// Malformed implemented_by shape -- checkImplementedBySymbolResolvable's
				// violation, not this check's.
				continue
			}
			if ok, _ := gate.EntryWithinSpecScope(specRoot, ie.file, g.SelfHosting); !ok {
				continue
			}
			rng, found, err := gate.ResolveSpecSymbolRange(specRoot, ie.file, ie.symbol)
			if err != nil {
				// Parse error -- checkImplementedBySymbolResolvable's violation
				// to report, not this check's.
				continue
			}
			if !found {
				// Resolves to a type (no coverable range) or is unresolvable.
				// Distinguish the two: a type-only entry (SpecSymbolType) is
				// this check's F1 concern; an unresolvable entry
				// (SpecSymbolNone) is checkImplementedBySymbolResolvable's.
				if symRes, symErr := gate.ResolveSpecSymbol(specRoot, ie.file, ie.symbol); symErr == nil && symRes.Kind == gate.SpecSymbolType {
					typeOnlyEntries = append(typeOnlyEntries, ie.raw)
				}
				continue
			}
			coverableCount++
			// The Go MODULE root that owns rng.File is NOT necessarily specRoot:
			// for an ordinary (non-self-hosting) domain, specRoot is domainDir
			// (e.g. "domains/prat"), but the actual go.mod authored spec/ lives
			// under (e.g. "domains/prat/spec/go.mod", module "prat-spec" per
			// PLAN-authored-spec-discipline.md §8) is one level DEEPER --
			// gate.ModuleRoot walks up from the resolved file itself to find the
			// nearest go.mod, which is the only reliable way to find it (a
			// self-hosting domain's specRoot IS already the engine's own module
			// root, so this walk is a no-op there -- ModuleRoot finds the exact
			// same go.mod either way).
			moduleRoot, mrOK := gate.ModuleRoot(filepath.Dir(rng.File))
			if !mrOK {
				out = append(out, Violation{
					Check: "check_scenario_executes_impl",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"implemented_by entry %q: could not find a go.mod owning %s -- cannot compute an import path for coverage matching",
						ie.raw, rng.File),
				})
				continue
			}
			importPath, impErr := gate.ImportPathForFile(moduleRoot, rng.File)
			if impErr != nil {
				out = append(out, Violation{
					Check: "check_scenario_executes_impl",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"implemented_by entry %q: could not compute an import path for coverage matching: %v",
						ie.raw, impErr),
				})
				continue
			}
			jobs = append(jobs, job{reqID: r.ID, implRaw: ie.raw, symRng: rng, imports: importPath, tests: testEntries})
		}
		// F1 gate: a SETTLED requirement in a discipline:full domain whose
		// implemented_by set produced ZERO coverable symbols (no function/
		// method range a coverage run could touch) AND has at least one
		// type-only entry claims to be "implemented" yet nothing in that
		// claim is even theoretically checkable by this coverage-proof gate.
		// This is the all-type/all-uncoverable bypass: the requirement passes
		// every other authored-link gate (resolvable, has teeth, passes,
		// scenario-narrated) while this check -- the one gate whose entire
		// purpose is to prove execution -- silently proves nothing. Scoped to
		// SETTLED + discipline:full because only those requirements have made
		// the explicit one-way promise that their implementation is fully
		// scenario-proven; a requirement in a soft-discipline domain citing a
		// type for documentation has not made that promise. typeOnlyEntries >
		// 0 (rather than coverableCount == 0 alone) ensures this does NOT fire
		// for a requirement whose entries are all unresolvable
		// (checkImplementedBySymbolResolvable's violation, not this check's).
		if coverableCount == 0 && len(typeOnlyEntries) > 0 &&
			r.Status == ontology.StatusSETTLED && g.Discipline == loader.DisciplineFull {
			out = append(out, Violation{
				Check: "check_scenario_executes_impl",
				ID:    r.ID,
				Message: fmt.Sprintf(
					"implemented_by set (%s) contains only type declarations with no executable lines -- a SETTLED requirement "+
						"in a discipline:full domain claims to be implemented, but none of its implemented_by entries resolve to a "+
						"function/method this coverage-proof gate could ever prove was executed; add a real function/method entry "+
						"(the implementation's actual logic) to implemented_by, or cite the type alongside a method that the "+
						"verified_by test genuinely exercises",
					strings.Join(typeOnlyEntries, ", ")),
			})
		}
	}

	if len(jobs) == 0 {
		return out
	}

	cache := &coverageRunCache{}
	sem := make(chan struct{}, runExecWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			v := scenarioExecutesImplViolation(specRoot, j.reqID, j.implRaw, j.symRng, j.imports, j.tests, cache)
			if v != nil {
				mu.Lock()
				out = append(out, *v)
				mu.Unlock()
			}
		}(j)
	}
	wg.Wait()

	return out
}

// eligibleVerifiedByEntries mirrors collectVerifiedByTestJobs' own
// eligibility filter (authored_links.go): out-of-scope, unresolvable,
// no-teeth, or skipped entries are excluded -- those are OTHER checks'
// violations to report, not this one's, and a test that never even runs for
// real reasons has no coverage profile worth asking about.
func eligibleVerifiedByEntries(specRoot string, selfHosting bool, verifiedBy []string) []specFileEntry {
	var out []specFileEntry
	for _, e := range parseSpecEntries(verifiedBy) {
		if !e.ok || !strings.HasPrefix(e.symbol, "Test") {
			continue
		}
		if ok, _ := gate.EntryWithinSpecScope(specRoot, e.file, selfHosting); !ok {
			continue
		}
		resolved, err := gate.ResolveSpecTest(specRoot, e.file, e.symbol)
		if err != nil || !resolved.Found {
			continue
		}
		if !resolved.HasTeeth || resolved.HasSkip {
			continue
		}
		out = append(out, e)
	}
	return out
}

// coverageRunCache de-duplicates RunVerifiedByTestRecording invocations
// WITHIN one checkScenarioExecutesImpl call: several implemented_by symbols
// (even across different requirements, since checkVerifiedByNoUnrelatedReuse
// already flags -- separately -- a verified_by entry shared by unrelated
// requirements) can end up asking "does test X's coverage profile, recorded
// against coverPkgFile Y, cover MY symbol" for the SAME (X, Y) pair; without
// this cache each such repeat would spawn its own redundant `go test
// -coverprofile` subprocess for byte-identical work. Keyed by (test file,
// test name, coverPkgFile) -- NOT reusing gate's own process-lifetime runCache
// (that cache is PLAIN RunVerifiedByTest results, no coverage profile, and is
// deliberately not taught to carry a second result shape per
// RunVerifiedByTestRecording's own doc comment) and deliberately scoped to
// ONE checkScenarioExecutesImpl call (a plain map + mutex, not a package-level
// var) since a coverage profile recorded for one AllViolations call has no
// reason to survive into the next -- content could have changed between
// calls in the same process (e.g. cmd/hotam's own long-lived test-harness
// callers), and this check's cost is already accepted as "pay it every real
// run" per the PERFORMANCE doc comment above.
type coverageRunCache struct {
	mu      sync.Mutex
	entries map[coverageRunKey]*coverageRunEntry
}

type coverageRunKey struct {
	testFile     string
	testName     string
	coverPkgFile string
}

type coverageRunEntry struct {
	once   sync.Once
	result gate.RecordingResult
}

// runOrReuse returns the RunVerifiedByTestRecording result for key, running
// it via runFn exactly once even if multiple goroutines request the same key
// concurrently (sync.Once per entry -- the same single-flight shape
// gate.singleflightRun already establishes as this codebase's pattern for
// collapsing concurrent identical subprocess requests, reimplemented locally
// here since gate's own singleflightRun is unexported and scoped to plain
// RunVerifiedByTest's cacheKey/TestRunResult shape, not RecordingResult).
func (c *coverageRunCache) runOrReuse(key coverageRunKey, runFn func() gate.RecordingResult) gate.RecordingResult {
	c.mu.Lock()
	if c.entries == nil {
		c.entries = map[coverageRunKey]*coverageRunEntry{}
	}
	entry, ok := c.entries[key]
	if !ok {
		entry = &coverageRunEntry{}
		c.entries[key] = entry
	}
	c.mu.Unlock()

	entry.once.Do(func() {
		entry.result = runFn()
	})
	return entry.result
}

// scenarioExecutesImplViolation runs (or reuses, via cache) every test in
// tests against coverPkgFile (the implemented_by symbol's own file) until one
// run's coverage profile covers symRng, or all are exhausted -- returning nil
// the moment ANY covering run is found (existential semantics, see
// checkScenarioExecutesImpl's own doc comment), or a violation naming every
// test tried and why none proved coverage if none did.
func scenarioExecutesImplViolation(specRoot, reqID, implRaw string, symRng gate.SymbolRange, importPath string, tests []specFileEntry, cache *coverageRunCache) *Violation {
	relImplFile, err := filepathRelSlash(specRoot, symRng.File)
	if err != nil {
		return &Violation{
			Check: "check_scenario_executes_impl",
			ID:    reqID,
			Message: fmt.Sprintf(
				"implemented_by entry %q: could not compute a domain-relative coverage-package path: %v",
				implRaw, err),
		}
	}

	var tried []string
	var infra []string
	for _, te := range tests {
		key := coverageRunKey{testFile: te.file, testName: te.symbol, coverPkgFile: relImplFile}
		result := cache.runOrReuse(key, func() gate.RecordingResult {
			return gate.RunVerifiedByTestRecording(specRoot, te.file, te.symbol, relImplFile)
		})
		tried = append(tried, te.raw)

		if result.Skipped {
			// Recursion guard honored -- unproven at this nesting level, same
			// treatment authored_links.go's verifiedByTestPassesViolation gives
			// a Skipped plain run: not a violation at THIS level (the outer,
			// non-nested process is the one that actually proves it), but not
			// silently treated as "covered" either -- record it as an infra
			// note so a caller inspecting a violation message (if one fires
			// from a LATER test in this same loop that is NOT skipped and
			// genuinely fails to cover) can see a skip happened, without this
			// skip alone ever being able to manufacture a false "coverage
			// proven" verdict.
			infra = append(infra, fmt.Sprintf("%s: skipped (recursion guard honored) -- %s", te.raw, result.InfraWarning))
			continue
		}
		if result.Err != nil {
			infra = append(infra, fmt.Sprintf("%s: could not execute -- %v", te.raw, result.Err))
			continue
		}
		if len(result.CoverProfile) == 0 {
			// No profile at all (package had zero coverable statements, or the
			// run never reached execution) -- cannot prove coverage from this
			// test; try the next one.
			continue
		}
		blocks := gate.ParseCoverProfile(result.CoverProfile)
		if gate.SymbolRangeCoveredByProfile(blocks, importPath, symRng.StartLine, symRng.EndLine) {
			return nil
		}
	}

	msg := fmt.Sprintf(
		"implemented_by entry %q (lines %d-%d): none of this requirement's verified_by tests (%s) actually executed a covered "+
			"statement inside this symbol's own declaration range when run with real coverage instrumentation -- a passing, "+
			"non-vacuous test that never once touches the implemented_by symbol's lines does not prove the requirement's claim; "+
			"either the test needs to actually call/exercise %s, or implemented_by/verified_by are mismatched",
		implRaw, symRng.StartLine, symRng.EndLine, strings.Join(tried, ", "), implRaw)
	if len(infra) > 0 {
		msg += fmt.Sprintf(" (additional notes: %s)", strings.Join(infra, "; "))
	}
	return &Violation{
		Check:   "check_scenario_executes_impl",
		ID:      reqID,
		Message: msg,
	}
}

// filepathRelSlash computes a root-relative, forward-slash path from absPath
// (already an absolute path under root, as gate.ResolveSpecSymbolRange
// guarantees for the File it returns) -- the domain-relative "spec/model/
// forecast.go"-shaped string RunVerifiedByTestRecording's own coverPkgFile
// parameter expects (the same shape implemented_by entries are already
// authored in).
func filepathRelSlash(root, absPath string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

var _ = All.MustRegister("check_scenario_executes_impl", Invariant{
	Name:  "check_scenario_executes_impl",
	Canon: methodology.Requirement,
	Claim: "every implemented_by symbol of an authored-path requirement is actually executed (a real, non-zero coverage hit " +
		"somewhere inside its own declaration lines) by at least one of that requirement's verified_by tests, proven via a " +
		"real `go test -coverprofile` run, not merely inferred from the test's source shape.",
	Rule: "for every Requirement with a non-empty implemented_by AND a non-empty verified_by (the authored path -- an empty " +
		"either field is a NO-OP for this check, same honesty boundary as every other authored-link check), for EACH " +
		"implemented_by entry that resolves to a real function/method declaration (gate.ResolveSpecSymbolRange -- a " +
		"type-only entry has no coverable range and is skipped, and an unresolvable entry is checkImplementedBySymbolResolvable's " +
		"violation, not this check's): run every ELIGIBLE verified_by entry (resolves, has teeth, not skipped -- the same " +
		"eligibility filter checkVerifiedByTestPasses uses) via gate.RunVerifiedByTestRecording with coverPkgFile set to the " +
		"implemented_by symbol's own file, parse the resulting coverage profile (gate.ParseCoverProfile), and check whether " +
		"AT LEAST ONE of those runs produced a covered (count>0) block overlapping the symbol's own [StartLine,EndLine] " +
		"(gate.SymbolRangeCoveredByProfile). If NONE of the requirement's eligible verified_by tests cover ANY line of this " +
		"implemented_by symbol, this check fires a violation naming the symbol, its line range, and every test tried.",
	Why: "PLAN-scenario-generated-spec.md §2 D3: every other authored-link check (authored_links.go, scenario_discipline.go) " +
		"is AST-only or, at most, proves the test PASSES (check_verified_by_test_passes) -- none of them ever confirm the " +
		"passing test actually touched the implemented_by symbol's own lines. A verified_by test that asserts a tautology, " +
		"exercises a completely different symbol, or calls a stub that never reaches the real implementation stays green " +
		"through every one of those checks: resolvable (the symbol/test both exist), has teeth (a real assertion call is " +
		"present, about ANYTHING), not skipped, even narrates a hotamspec scenario (HasScenario only checks the test body " +
		"calls hotamspec.NewScenario, never what it then goes on to exercise), and passes when run (RunVerifiedByTest cares " +
		"only about exit code / FAIL-scanning, not which lines executed). This check closes exactly that gap, mechanically: " +
		"a real coverage profile from the SAME kind of run W1.2's RunVerifiedByTestRecording already performs is the only " +
		"honest way to prove 'this test's assertions are actually ABOUT this code', short of a human/LLM mirror audit " +
		"reading claim-against-test (which remains necessary for semantic adequacy -- this check proves EXECUTION, not " +
		"CORRECTNESS of the assertion; see authored_links.go's HONESTY BOUNDARY doc comment, which this check inherits " +
		"unchanged). SCOPE: only the authored path (implemented_by+verified_by both present) -- an engine-path requirement " +
		"(enforced_by only) has no implemented_by symbol for this check to demand coverage of, so it is an honest NO-OP for " +
		"that requirement, the same shape checkSettledRequiresScenario's engine-path exemption already establishes.",
	Check: checkScenarioExecutesImpl,
})
