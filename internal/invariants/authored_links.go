package invariants

import (
	"fmt"
	"strings"
	"sync"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// This file holds the MECHANICAL checks for the authored-spec discipline
// (PLAN-authored-spec-discipline.md §6): implemented_by / verified_by are
// path-qualified references INTO A DOMAIN'S OWN authored spec/ tree (never
// HotamSpec's own internal/ or cmd/ -- that is the enforced_by/EnforcedBy
// mechanism in enforcement.go). Unlike EnforcedBy's engine-wide scan, every
// implemented_by/verified_by entry names its OWN file, so resolution is a
// single targeted parse of that one file -- no repo-wide walk.
//
// HONESTY BOUNDARY (§6): these checks hold the STRUCTURAL floor only --
// the symbol/test named really exists, really is a Go test, is not
// t.Log-only, does not t.Skip, and is not suspiciously reused across
// unrelated requirements. They do NOT and CANNOT verify that the test
// SEMANTICALLY proves the requirement's claim -- that remains a mirror
// audit performed by a human or an LLM reading the pair (requirement text,
// authored code+test) without machinery. Do not extend these checks with
// lexical/semantic heuristics about "is this test about this requirement";
// that judgment call belongs to the mirror audit, never to the graph
// invariant layer.
//
// All checks below are NO-OP for a requirement with an empty
// implemented_by/verified_by (both fields are optional -- see
// ontology.Requirement doc comments) -- authored spec/ does not exist yet in
// any domain (task #224 is the pilot), so on every domain graph.json today
// these checks contribute zero violations.

// specFileEntry is one parsed "file:symbol" or "file:test" reference.
type specFileEntry struct {
	raw    string
	file   string
	symbol string
	ok     bool // false if the entry could not even be split into file:symbol
}

func parseSpecEntries(raw []string) []specFileEntry {
	out := make([]specFileEntry, 0, len(raw))
	for _, entry := range raw {
		trimmed := strings.TrimSpace(entry)
		file, symbol, ok := gate.ParseFileColonSymbol(trimmed)
		out = append(out, specFileEntry{raw: trimmed, file: file, symbol: symbol, ok: ok})
	}
	return out
}

// checkImplementedBySymbolResolvable is the existence/staleness check for
// implemented_by: every entry must split into file:symbol and the symbol
// must really be declared (function, method, or type) in the named file
// under the domain's spec/ tree. Applies to every requirement with a
// non-empty implemented_by, regardless of enforcement level or status --
// a stale implemented_by reference is a lie about "where this is embodied"
// the moment it is written, not just once ENFORCED.
func checkImplementedBySymbolResolvable(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		if len(r.ImplementedBy) == 0 {
			continue
		}
		for _, e := range parseSpecEntries(r.ImplementedBy) {
			if !e.ok {
				out = append(out, Violation{
					Check: "check_implemented_by_symbol_resolvable",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"implemented_by entry %q is not shaped like file:symbol (expected e.g. \"spec/model/risk.go:NewRisk\")",
						e.raw),
				})
				continue
			}
			specRoot := gate.SpecRootForGraph(g)
			if ok, reason := gate.EntryWithinSpecScope(specRoot, e.file, g.SelfHosting); !ok {
				out = append(out, Violation{
					Check: "check_implemented_by_symbol_resolvable",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"implemented_by entry %q: %s -- references must stay inside the domain's own authored scope",
						e.raw, reason),
				})
				continue
			}
			result, err := gate.ResolveSpecSymbol(specRoot, e.file, e.symbol)
			if err != nil {
				out = append(out, Violation{
					Check: "check_implemented_by_symbol_resolvable",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"implemented_by entry %q could not be resolved: %v",
						e.raw, err),
				})
				continue
			}
			if !result.Found() {
				out = append(out, Violation{
					Check: "check_implemented_by_symbol_resolvable",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"implemented_by entry %q: symbol %q not found in %q (function, method, or type declaration expected) -- stale or never-written reference",
						e.raw, e.symbol, e.file),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_implemented_by_symbol_resolvable", Invariant{
	Name:  "check_implemented_by_symbol_resolvable",
	Canon: methodology.Requirement,
	Claim: "every non-empty implemented_by entry resolves to a real function, method, or type declaration in the named authored spec/ file.",
	Rule: "each Requirement.implemented_by entry MUST be shaped \"file:symbol\" and the symbol MUST be a real declaration -- a top-level " +
		"function, a method (bare name matches any receiver; \"Type.Method\" qualifies the receiver's base type), or a type -- found by " +
		"parsing exactly the named file under the domain's spec root (gate.SpecRootForGraph(g): g.DomainDir joined with the entry's " +
		"file path, which is already spec/-prefixed per PLAN-authored-spec-discipline.md §4 -- OR, for a self-hosting domain " +
		"(g.SelfHosting, e.g. domains/hotam-spec-self), the engine repository root found by walking up from g.DomainDir to the nearest " +
		"go.mod, per PLAN-authored-spec-discipline.md §9's recursion: engine-facing requirements name paths like " +
		"\"internal/ontology/lifecycle.go:Lifecycle\" relative to the engine root, not to domainDir). Applies to EVERY requirement with a " +
		"non-empty implemented_by, independent of enforcement or status: a stale symbol reference is false the moment it is written.",
	Why: "implemented_by claims \"this requirement is embodied HERE\"; if the named symbol does not exist the claim is unverifiable -- " +
		"either a typo, a symbol that was renamed/deleted after the reference was written (staleness), or a reference that was never real. " +
		"This is the authored-spec counterpart of check_enforced_by_resolvable, using a single targeted file parse (gate.ResolveSpecSymbol) " +
		"instead of a repo-wide scan, because every entry names its own file.",
	Check: checkImplementedBySymbolResolvable,
})

// checkVerifiedByTestResolvable is the existence/staleness check for
// verified_by: every entry must split into file:test and the test must
// really be a top-level `func TestXxx(t *testing.T)` declared in the named
// file under the domain's spec/ tree.
func checkVerifiedByTestResolvable(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		if len(r.VerifiedBy) == 0 {
			continue
		}
		for _, e := range parseSpecEntries(r.VerifiedBy) {
			if !e.ok {
				out = append(out, Violation{
					Check: "check_verified_by_test_resolvable",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"verified_by entry %q is not shaped like file:test (expected e.g. \"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner\")",
						e.raw),
				})
				continue
			}
			if !strings.HasPrefix(e.symbol, "Test") {
				out = append(out, Violation{
					Check: "check_verified_by_test_resolvable",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"verified_by entry %q: %q is not Test*-shaped -- go test would never run it",
						e.raw, e.symbol),
				})
				continue
			}
			specRoot := gate.SpecRootForGraph(g)
			if ok, reason := gate.EntryWithinSpecScope(specRoot, e.file, g.SelfHosting); !ok {
				out = append(out, Violation{
					Check: "check_verified_by_test_resolvable",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"verified_by entry %q: %s -- references must stay inside the domain's own authored scope",
						e.raw, reason),
				})
				continue
			}
			result, err := gate.ResolveSpecTest(specRoot, e.file, e.symbol)
			if err != nil {
				out = append(out, Violation{
					Check: "check_verified_by_test_resolvable",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"verified_by entry %q could not be resolved: %v",
						e.raw, err),
				})
				continue
			}
			if !result.Found {
				out = append(out, Violation{
					Check: "check_verified_by_test_resolvable",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"verified_by entry %q: test %q not found in %q (a real func %s(t *testing.T) is required) -- stale or never-written reference",
						e.raw, e.symbol, e.file, e.symbol),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_verified_by_test_resolvable", Invariant{
	Name:  "check_verified_by_test_resolvable",
	Canon: methodology.Requirement,
	Claim: "every non-empty verified_by entry resolves to a real, runnable Test* function in the named authored spec/ file.",
	Rule: "each Requirement.verified_by entry MUST be shaped \"file:test\", the test name MUST be Test*-prefixed, and it MUST be a real top-level " +
		"declaration `func TestXxx(t *testing.T)` found by parsing exactly the named file under the domain's spec root " +
		"(gate.SpecRootForGraph(g) -- domainDir for an ordinary domain, or the engine repository root for a self-hosting domain per " +
		"PLAN-authored-spec-discipline.md §9). Applies to EVERY requirement with a non-empty verified_by, independent of enforcement or status.",
	Why: "verified_by claims \"this requirement is PROVEN here\"; if the named test does not exist, is not Test*-shaped, or does not have the " +
		"real go-test signature, the claim is unverifiable -- go test would never even run it. This is the authored-spec counterpart of " +
		"check_enforced_by_resolvable's Test*-name half, scoped to a single named file via gate.ResolveSpecTest instead of a repo-wide scan.",
	Check: checkVerifiedByTestResolvable,
})

// checkVerifiedByTestHasTeeth is the REAL successor to the honest no-op
// checkEnforcedByTestHasTeeth (enforcement.go) -- for the authored-spec era
// this is no longer an advisory shrug, it is a mechanical PROHIBITION: a
// resolvable verified_by test whose body is empty or contains only
// t.Log/t.Logf calls (no real assertion, no exercising branch) fires a
// violation. Skips entries that failed resolution entirely (that is
// checkVerifiedByTestResolvable's job -- this check only judges the body of
// a test that was actually found).
func checkVerifiedByTestHasTeeth(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		if len(r.VerifiedBy) == 0 {
			continue
		}
		specRoot := gate.SpecRootForGraph(g)
		for _, e := range parseSpecEntries(r.VerifiedBy) {
			if !e.ok || !strings.HasPrefix(e.symbol, "Test") {
				continue
			}
			if ok, _ := gate.EntryWithinSpecScope(specRoot, e.file, g.SelfHosting); !ok {
				// Out-of-scope entries are checkVerifiedByTestResolvable's
				// violation to report -- this check does not resolve (let
				// alone judge the teeth of) a file outside the domain's own
				// authored scope.
				continue
			}
			result, err := gate.ResolveSpecTest(specRoot, e.file, e.symbol)
			if err != nil || !result.Found {
				continue
			}
			if !result.HasTeeth {
				out = append(out, Violation{
					Check: "check_verified_by_test_has_teeth",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"verified_by entry %q: test body has no real assertion or exercising branch (t.Log-only or empty) -- "+
							"a vacuous test proves nothing; add a real t.Error/t.Fatal/require/assert or a conditional check",
						e.raw),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_verified_by_test_has_teeth", Invariant{
	Name:  "check_verified_by_test_has_teeth",
	Canon: methodology.Requirement,
	Claim: "every resolvable verified_by test contains at least one real assertion call, anywhere in its body -- not t.Log-only, not empty, and not a bare control-flow construct with no assertion inside it.",
	Rule: "for each Requirement.verified_by entry that resolves to a real Test* function (checkVerifiedByTestResolvable), the test body MUST " +
		"contain at least one real assertion call, ANYWHERE in the body (top-level or nested inside if/for/switch/range/t.Run branches): a " +
		"call to t.Error/t.Errorf/t.Fatal/t.Fatalf/t.Fail/t.FailNow, or a call on an identifier conventionally used by assertion libraries " +
		"(require.*/assert.*). A body that is empty, contains ONLY t.Log/t.Logf calls, or contains only control-flow (if/for/switch/range) " +
		"with no assertion call anywhere inside it, fails this check -- a bare `for i := 0; i < 0; i++ {}` or an `if` with an empty/assert-free " +
		"body no longer counts as \"teeth\" merely by shape.",
	Why: "this is the authored-spec ENFORCEMENT successor to enforcement.go's checkEnforcedByTestHasTeeth, which was an honest no-op because " +
		"engine-side Test* enforcers are repo-wide and running under `go test` anyway -- vacuous failure there shows up at CI time regardless. " +
		"Authored verified_by tests are different: PLAN-authored-spec-discipline.md §1 documents that under the OLD generator, " +
		"\"t.Log('no structural atom') in 10 of 22 requirements\" was exactly this failure mode wearing an honest-looking always-green test. " +
		"Since a domain's spec/ tree is not necessarily wired into any CI `go test ./...` HotamSpec itself runs, this check is the only " +
		"mechanical backstop against a vacuous verified_by claim -- it MUST be a real prohibition, not a no-op, for ENFORCED to mean anything " +
		"in the authored era. HONESTY BOUNDARY: this proves the test is not HOLLOW; it cannot and does not prove the test is about the right " +
		"requirement -- that remains the mirror audit's job (§6).",
	Check: checkVerifiedByTestHasTeeth,
})

// checkVerifiedByTestNoSkip prohibits a top-level, unconditional t.Skip/
// t.Skipf in a resolvable verified_by test: an always-skipped test proves
// nothing, exactly like a t.Log-only body, but passes checkVerifiedByTestHasTeeth
// if it happens to also contain assertions after the skip (dead code) --
// this is a separate, explicit prohibition rather than folded into "teeth"
// so the violation message names the actual defect precisely.
func checkVerifiedByTestNoSkip(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		if len(r.VerifiedBy) == 0 {
			continue
		}
		specRoot := gate.SpecRootForGraph(g)
		for _, e := range parseSpecEntries(r.VerifiedBy) {
			if !e.ok || !strings.HasPrefix(e.symbol, "Test") {
				continue
			}
			if ok, _ := gate.EntryWithinSpecScope(specRoot, e.file, g.SelfHosting); !ok {
				// Out-of-scope entries are checkVerifiedByTestResolvable's
				// violation to report -- this check does not resolve a file
				// outside the domain's own authored scope.
				continue
			}
			result, err := gate.ResolveSpecTest(specRoot, e.file, e.symbol)
			if err != nil || !result.Found {
				continue
			}
			if result.HasSkip {
				out = append(out, Violation{
					Check: "check_verified_by_test_no_skip",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"verified_by entry %q: test contains an unconditional top-level t.Skip/t.Skipf -- an always-skipped test proves nothing",
						e.raw),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_verified_by_test_no_skip", Invariant{
	Name:  "check_verified_by_test_no_skip",
	Canon: methodology.Requirement,
	Claim: "no resolvable verified_by test contains an unconditional top-level t.Skip/t.Skipf.",
	Rule: "for each Requirement.verified_by entry that resolves to a real Test* function, the test body MUST NOT contain a direct top-level " +
		"statement calling t.Skip or t.Skipf. A skip nested inside a runtime condition (e.g. `if testing.Short() { t.Skip(...) }`) is a normal " +
		"Go idiom and is NOT flagged -- only an unconditional, always-reached top-level skip is prohibited, since that unconditionally defeats " +
		"the test regardless of how it runs.",
	Why: "an unconditionally skipped test is the same lie as a t.Log-only body wearing a green checkmark -- `go test` reports it as passed/" +
		"skipped either way, so ENFORCED via such a test is unverifiable in exactly the way §6's anti-vacuousness rule targets. Split out from " +
		"check_verified_by_test_has_teeth so the violation message names the specific defect (unconditional skip) rather than a generic " +
		"\"no teeth\" message, and so a test with real assertions AFTER a top-level unconditional skip (dead code, still never runs) is still " +
		"caught -- teeth-detection alone would miss that shape since ast.Inspect walks the whole body regardless of reachability.",
	Check: checkVerifiedByTestNoSkip,
})

// checkVerifiedByTestPasses is the EXECUTION half of the verified_by
// discipline (@fh finding F1, Probe C; PLAN-authored-spec-discipline.md §6's
// "verified_by тест существует и РЕАЛЬНО ЗАПУСКАЕТСЯ"): every other
// verified_by check above (resolvable / has-teeth / no-skip) is AST-only --
// none of them ever actually COMPILES or RUNS the named test. That let a
// requirement stay ENFORCED with a verified_by test that fails, or does not
// even compile, as long as the test function's SHAPE looked right: real
// Test*(t *testing.T) signature, a real assertion call somewhere in the
// body, no unconditional skip. Probe C demonstrated this concretely: gutting
// a requirement's implementation (e.g. replacing a validation method's body
// with `return nil`) turns `go test` red for that package while every
// AST-only check above stays green, because they inspect the TEST's source
// text, never the compiled, executed behavior of the package under test.
//
// This check closes that gap: for every verified_by entry that already
// resolved AND has teeth AND has no unconditional skip (the three checks
// above; an entry that already fails one of those is THEIR violation to
// report, not this check's -- no point compiling/running a test already
// known to be structurally hollow), gate.RunVerifiedByTest actually invokes
// `go test -run '^<name>$'` against the owning Go module (the domain's own
// spec/ module for an ordinary domain, or the engine's own module for a
// self-hosting domain -- gate.ModuleRoot resolves which, by walking up from
// the test file to the nearest go.mod) and requires PASS. A non-zero exit,
// a "FAIL" in the output, or a compile failure all fire a violation naming
// the test, the requirement, and a bounded tail of the actual go-test
// output, so the failure is diagnosable from the violation message alone
// without re-running anything by hand.
//
// PERFORMANCE (design decision A, PLAN-authored-spec-discipline.md §6):
// gate.RunVerifiedByTest memoizes results in a process-lifetime cache keyed
// by (package directory, test name) with content-hash invalidation over
// go.mod/go.sum plus every *.go file in the test's own package directory
// (gate.hashPackageInputs) -- so a repeat all-violations call against an
// UNCHANGED spec skips the go test invocation entirely (cache hit), while
// ANY edit to the test file, the implementation file(s) sharing its
// package, or the module's go.mod/go.sum invalidates the cache and forces a
// real re-run. This is what makes Probe C's mutation (an impl-file edit,
// never a test-file edit) actually caught: the impl file lives in the same
// package directory the hash covers, so the very next all-violations call
// after the mutation sees a changed hash and re-executes, even though
// nothing about the verified_by ENTRY ITSELF (file:test string) changed.
//
// A cache MISS is still relatively expensive (a real `go test` subprocess,
// observed 1-10s per distinct package on this repo's own hardware -- see
// internal/gate/test_exec.go's RunVerifiedByTest doc comment), and a single
// domain graph can legitimately name several DIFFERENT packages across its
// verified_by entries (the real domains/hotam-spec-self graph names nine,
// spanning internal/ontology, internal/proposal, internal/loader, etc --
// nine cold, sequential `go test` invocations measured ~90s on this
// machine, which blew past `go test`'s own default 30s per-package
// timeout for a caller like TestRegistryComplete_AllViolationsOnRealGraphDoesNotPanic
// that calls AllViolations once against the real graph). Since each
// RunVerifiedByTest call is independent I/O-bound work (a subprocess, not
// CPU-bound computation), this check fans the runnable entries out across a
// small bounded worker pool (runExecWorkers) rather than running them
// sequentially -- bounded, not unbounded, so a domain with many verified_by
// entries cannot spawn dozens of simultaneous `go test` processes and
// thrash the machine the way N-way unbounded parallelism would.
func checkVerifiedByTestPasses(g *ontology.Graph) []Violation {
	specRoot := gate.SpecRootForGraph(g)
	type job struct {
		reqID string
		entry specFileEntry
	}
	var jobs []job
	for _, r := range g.Requirements {
		if len(r.VerifiedBy) == 0 {
			continue
		}
		for _, e := range parseSpecEntries(r.VerifiedBy) {
			if !e.ok || !strings.HasPrefix(e.symbol, "Test") {
				continue
			}
			if ok, _ := gate.EntryWithinSpecScope(specRoot, e.file, g.SelfHosting); !ok {
				// Out-of-scope entries are checkVerifiedByTestResolvable's
				// violation to report.
				continue
			}
			resolved, err := gate.ResolveSpecTest(specRoot, e.file, e.symbol)
			if err != nil || !resolved.Found {
				// checkVerifiedByTestResolvable's violation to report.
				continue
			}
			if !resolved.HasTeeth || resolved.HasSkip {
				// checkVerifiedByTestHasTeeth / checkVerifiedByTestNoSkip's
				// violation to report -- do not also compile/run a test
				// already known to be structurally hollow or skipped.
				continue
			}
			jobs = append(jobs, job{reqID: r.ID, entry: e})
		}
	}
	if len(jobs) == 0 {
		return nil
	}

	results := make([]*Violation, len(jobs))
	sem := make(chan struct{}, runExecWorkers)
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, j job) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = verifiedByTestPassesViolation(specRoot, j.reqID, j.entry)
		}(i, j)
	}
	wg.Wait()

	var out []Violation
	for _, v := range results {
		if v != nil {
			out = append(out, *v)
		}
	}
	return out
}

// runExecWorkers bounds how many RunVerifiedByTest subprocess calls
// checkVerifiedByTestPasses runs concurrently. A small fixed cap (not
// unbounded, not runtime.NumCPU()-scaled) is deliberate: this is I/O-bound
// subprocess fan-out (each `go test` invocation is its own OS process with
// its own compile step), not CPU-bound work, so tying it to core count would
// both under-use idle cores waiting on I/O and over-subscribe a
// small-core-count CI runner with too many simultaneous `go` toolchain
// invocations (which themselves spawn further child processes). Kept at 2
// (not the original 4) and aligned with gate.globalExecSlots' own cap: this
// process almost never has just ONE checkVerifiedByTestPasses call in
// flight -- cmd/hotam's own e2e test suite runs many t.Parallel() tests that
// EACH independently call AllViolations (directly or via a spawned `hotam`
// subprocess), so the REALISTIC peak concurrency is (number of simultaneous
// callers) x runExecWorkers; observed on a heavily-loaded shared dev machine
// that 4 was enough to starve unrelated goroutines (e.g. the pre-existing
// check_enforced_by_resolvable's repo-wide filepath.WalkDir) of scheduling
// time under go test -race, occasionally pushing a whole package past its
// -timeout budget. 2 trades a little wall-clock time within one
// checkVerifiedByTestPasses call for materially less system-wide thrash.
const runExecWorkers = 2

// verifiedByTestPassesViolation runs one job (a single verified_by entry
// already known to resolve, have teeth, and not be skipped) and returns the
// Violation to report, or nil if it passes. Split out from
// checkVerifiedByTestPasses so the worker goroutine body is a plain
// function call, not an inline closure duplicating this logic per call site.
func verifiedByTestPassesViolation(specRoot, reqID string, e specFileEntry) *Violation {
	run := gate.RunVerifiedByTest(specRoot, e.file, e.symbol)
	if run.Skipped {
		// RunVerifiedByTest's recursion guard fired: this process is already
		// nested inside a `go test` subprocess RunVerifiedByTest itself
		// spawned (structural for self-hosting domains -- see
		// gate.recursionGuardEnv's doc comment). Not provable at THIS
		// nesting level; the outer, non-nested invocation is the one that
		// actually runs and proves it, so this level reports no violation
		// rather than a false pass or a false failure.
		return nil
	}
	if run.Err != nil {
		return &Violation{
			Check: "check_verified_by_test_passes",
			ID:    reqID,
			Message: fmt.Sprintf(
				"verified_by entry %q could not be executed: %v",
				e.raw, run.Err),
		}
	}
	if run.Passed {
		return nil
	}
	if run.CompileFailed {
		return &Violation{
			Check: "check_verified_by_test_passes",
			ID:    reqID,
			Message: fmt.Sprintf(
				"verified_by entry %q: package does not compile -- `go test` build failed for %s, so %s could never run. Output:\n%s",
				e.raw, e.file, e.symbol, run.Output),
		}
	}
	return &Violation{
		Check: "check_verified_by_test_passes",
		ID:    reqID,
		Message: fmt.Sprintf(
			"verified_by entry %q: test %s FAILS when actually run (`go test -run '^%s$'`) -- a red or non-compiling proof does not satisfy verified_by. Output:\n%s",
			e.raw, e.symbol, e.symbol, run.Output),
	}
}

var _ = All.MustRegister("check_verified_by_test_passes", Invariant{
	Name:  "check_verified_by_test_passes",
	Canon: methodology.Requirement,
	Claim: "every resolvable, non-vacuous, non-skipped verified_by test actually PASSES when compiled and executed via `go test`.",
	Rule: "for each Requirement.verified_by entry that resolves to a real Test* function (check_verified_by_test_resolvable), has a real " +
		"assertion (check_verified_by_test_has_teeth), and is not unconditionally skipped (check_verified_by_test_no_skip), the engine MUST " +
		"actually compile and run it: `go test -run '^<name>$'` against the owning Go module (gate.ModuleRoot: the domain's own spec/ module for " +
		"an ordinary domain, or the engine's own module for a self-hosting domain), and it MUST exit 0 with no FAIL in its output. A non-zero " +
		"exit, a FAIL line, or a build/compile failure is a violation naming the test, the requirement, and the actual go-test output.",
	Why: "@fh finding F1 (Probe C): every verified_by check before this one is AST-only -- it inspects the TEST's source text (does the symbol " +
		"exist, does it look like a test, does it call an assertion, is it skipped) but never actually RUNS the test, so a requirement could stay " +
		"ENFORCED with a verified_by test that fails or does not even compile, as long as the test's SHAPE looked right. Probe C demonstrated this " +
		"concretely: gutting an implementation (e.g. a validation method rewritten to unconditionally return nil) turns `go test` red for that " +
		"package while every AST-only check stays green. This check closes that gap by actually executing the test and requiring PASS, memoized " +
		"via gate.RunVerifiedByTest's content-hash cache (design decision A) so an unchanged spec is not re-run on every all-violations call, while " +
		"any edit to the test file, a sibling implementation file in the same package, or the module's go.mod/go.sum invalidates the cache and " +
		"forces a real re-run -- so 'graph clean' now HONESTLY means 'the proofs execute and pass', not merely 'the proofs are shaped like proofs'.",
	Check: checkVerifiedByTestPasses,
})

// checkVerifiedByNoUnrelatedReuse is the reuse-detector (§6): the same
// file:test verified_by entry formally cited by two or more requirements
// that are NOT all mutually related is suspicious -- one authored test is
// being stretched to "prove" multiple business claims it was not written to
// individually exercise. Two requirements are RELATED (and therefore exempt
// from the flag when a whole citing group is pairwise related) if either
// directly names the other in its Relations (refines/depends_on/replaces, in
// either direction) -- a deliberately narrow, STRUCTURAL definition of
// "related" (no lexical/semantic guessing about claim text, per the honesty
// boundary in §6). Concretely: partition the citing requirements into
// connected components under the "related" adjacency; if more than one
// component exists, every citing requirement is part of an unrelated-reuse
// situation and fires. A single citation, or a whole group of citations that
// are all pairwise connected via Relations into ONE component, does not
// fire.
func checkVerifiedByNoUnrelatedReuse(g *ontology.Graph) []Violation {
	entryToReqs := map[string][]string{}
	for _, r := range g.Requirements {
		seenInThisReq := map[string]struct{}{}
		for _, e := range parseSpecEntries(r.VerifiedBy) {
			if !e.ok {
				continue
			}
			if _, dup := seenInThisReq[e.raw]; dup {
				continue
			}
			seenInThisReq[e.raw] = struct{}{}
			entryToReqs[e.raw] = append(entryToReqs[e.raw], r.ID)
		}
	}

	related := relatedPairIndex(g)

	var out []Violation
	for entry, reqIDs := range entryToReqs {
		if len(reqIDs) < 2 {
			continue
		}
		// Partition the citing requirements into connected components under
		// the "related" adjacency (direct Relation edge only). If they all
		// collapse into ONE component, the shared entry is legitimately
		// justified by recorded Relations -- no violation. If MORE THAN ONE
		// component remains, at least two citers share the entry with no
		// recorded relation between their components -- every citer fires.
		groups := partitionByRelatedness(reqIDs, related)
		if len(groups) <= 1 {
			continue
		}
		out = append(out, groupReuseViolations(entry, reqIDs)...)
	}
	return out
}

// relatedPairIndex builds a symmetric set of {reqA,reqB} pairs that are
// directly connected via a Relation (refines/depends_on/replaces) in either
// direction.
func relatedPairIndex(g *ontology.Graph) map[[2]string]struct{} {
	pairs := map[[2]string]struct{}{}
	for _, r := range g.Requirements {
		for _, rel := range r.Relations {
			pairs[orderedPair(r.ID, rel.Target)] = struct{}{}
		}
	}
	return pairs
}

func orderedPair(a, b string) [2]string {
	if a < b {
		return [2]string{a, b}
	}
	return [2]string{b, a}
}

// partitionByRelatedness groups reqIDs (all citing the same verified_by
// entry) into connected components under the "related" adjacency (direct
// Relation edge only -- not transitive beyond the graph's own Relations).
// Each returned group is either a single unrelated requirement or a
// mutually-connected cluster.
func partitionByRelatedness(reqIDs []string, related map[[2]string]struct{}) [][]string {
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		if parent[x] == "" {
			parent[x] = x
		}
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	for _, id := range reqIDs {
		find(id)
	}
	for i := 0; i < len(reqIDs); i++ {
		for j := i + 1; j < len(reqIDs); j++ {
			if _, ok := related[orderedPair(reqIDs[i], reqIDs[j])]; ok {
				union(reqIDs[i], reqIDs[j])
			}
		}
	}
	groups := map[string][]string{}
	for _, id := range reqIDs {
		root := find(id)
		groups[root] = append(groups[root], id)
	}
	out := make([][]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g)
	}
	return out
}

func groupReuseViolations(entry string, group []string) []Violation {
	out := make([]Violation, 0, len(group))
	for _, id := range group {
		out = append(out, Violation{
			Check: "check_verified_by_no_unrelated_reuse",
			ID:    id,
			Message: fmt.Sprintf(
				"verified_by entry %q is formally cited by %d unrelated requirements (%s) -- one authored test cannot honestly "+
					"stand as proof for multiple unrelated claims; link them via Relations if they truly share one test, or write a "+
					"dedicated test per requirement",
				entry, len(group), strings.Join(group, ", ")),
		})
	}
	return out
}

var _ = All.MustRegister("check_verified_by_no_unrelated_reuse", Invariant{
	Name:  "check_verified_by_no_unrelated_reuse",
	Canon: methodology.Requirement,
	Claim: "no verified_by entry is formally cited by two or more requirements that are not all mutually related.",
	Rule: "collect, per distinct verified_by entry (file:test string), every requirement citing it (fewer than 2 citers -- trivially fine, skip). " +
		"Partition that set into connected components under a STRICT STRUCTURAL adjacency: two requirements are \"related\" only if one directly " +
		"names the other in its Relations (refines/depends_on/replaces, either direction) -- never by lexical or semantic similarity of claim text " +
		"(per the §6 honesty boundary). If the citers collapse into exactly ONE component (all pairwise connected via recorded Relations), the " +
		"shared entry is exempt. If MORE THAN ONE component remains -- i.e. at least two citers share the entry with no recorded relation chain " +
		"between them -- every citing requirement fires a violation naming the shared entry and the sibling IDs.",
	Why: "an authored test formally attached to several requirements' verified_by with NO recorded relationship between those requirements is a " +
		"red flag that the test is being stretched thin, or that requirements were mechanically bulk-tagged rather than individually proven -- the " +
		"exact failure mode PLAN-authored-spec-discipline.md §1 documents from the old generator era (t.Log placeholders masquerading as coverage " +
		"for many requirements at once). The Relations-based \"related\" definition is deliberately narrow and structural (no claim-text guessing) " +
		"so this stays a mechanical check, not a semantic one -- genuinely related requirements (e.g. a refines-chain that legitimately shares one " +
		"proof) are exempted by explicitly recording that relation in the graph, which is itself an auditable, typed edge; an UNRELATED pair with " +
		"no such edge is exactly what the check is designed to catch.",
	Check: checkVerifiedByNoUnrelatedReuse,
})

// checkEnforcedRequiresEnforcerOrAuthoredLink is the DISJUNCTIVE ENFORCED
// gate (PLAN-authored-spec-discipline.md §5/§12): a SETTLED requirement with
// enforcement == ENFORCED is valid iff EITHER the engine-mechanism path
// (enforced_by non-empty) OR the authored-spec path (implemented_by AND
// verified_by both non-empty) holds. Neither path present -> violation, same
// as the existing check_enforced_names_invariant, but that check only knows
// about the enforced_by path; this check is the disjunctive widening that
// also accepts the authored path so a requirement need not carry a
// redundant enforced_by once it has real implemented_by+verified_by.
func checkEnforcedRequiresEnforcerOrAuthoredLink(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		if r.Status != ontology.StatusSETTLED || r.Enforcement != ontology.EnforcementENFORCED {
			continue
		}
		engineMechanism := len(r.EnforcedBy) > 0
		authoredMechanism := len(r.ImplementedBy) > 0 && len(r.VerifiedBy) > 0
		if !engineMechanism && !authoredMechanism {
			out = append(out, Violation{
				Check: "check_enforced_requires_enforcer_or_authored_link",
				ID:    r.ID,
				Message: "enforcement is ENFORCED but neither the engine mechanism (enforced_by non-empty) nor the authored " +
					"mechanism (implemented_by AND verified_by both non-empty) is present -- an ENFORCED requirement must name a " +
					"real enforcer via one of the two paths",
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_enforced_requires_enforcer_or_authored_link", Invariant{
	Name:  "check_enforced_requires_enforcer_or_authored_link",
	Canon: methodology.Requirement,
	Claim: "every SETTLED+ENFORCED requirement has a real enforcer via the engine path (enforced_by) or the authored path (implemented_by AND verified_by).",
	Rule: "for each Requirement with status == SETTLED and enforcement == ENFORCED, AT LEAST ONE of the following MUST hold: (1) enforced_by is " +
		"non-empty (the engine mechanism: a check_* registry name or a repo-wide Test* function name -- see check_enforced_names_invariant / " +
		"check_enforced_by_resolvable), OR (2) implemented_by is non-empty AND verified_by is non-empty (the authored mechanism: a path-qualified " +
		"symbol reference into the domain's spec/ tree plus a path-qualified test reference -- see check_implemented_by_symbol_resolvable / " +
		"check_verified_by_test_resolvable). Having implemented_by without verified_by (or vice versa) does NOT satisfy the authored path -- " +
		"\"embodied somewhere\" without \"proven somewhere\" (or vice versa) is not a real guarantee.",
	Why: "PLAN-authored-spec-discipline.md §5 (steward decision 2026-07-16): a requirement may be SETTLED without code as honest roadmap debt, " +
		"but ENFORCED requires a real noun -- EITHER path is acceptable so a requirement enforced by the engine's own check_*/Test* machinery " +
		"(self-domain requirements about the engine itself) is not forced to fabricate a redundant authored spec/ reference, while a business " +
		"domain's authored-spec requirement is not forced to invent a fake engine-side enforcer. check_enforced_names_invariant alone only " +
		"recognizes the enforced_by half and would wrongly reject a requirement that is legitimately ENFORCED purely via implemented_by+" +
		"verified_by; this check is the widened disjunctive gate that recognizes both.",
	Check: checkEnforcedRequiresEnforcerOrAuthoredLink,
})
