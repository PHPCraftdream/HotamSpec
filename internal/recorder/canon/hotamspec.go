// Package hotamspec is the CANONICAL source of the scenario-recorder API
// authored verified_by tests use to narrate a requirement's proof in the
// domain's own words (PLAN-scenario-generated-spec.md §1/§2 D1, task W1.1).
//
// THIS FILE IS THE CANON. It is never imported directly by a consumer
// domain's spec/ module (that would require a cross-module `replace`,
// forbidden per NEW-2-bis -- see internal/gate/test_exec.go's
// hashPackageInputs NEW-2 doc comment for the sibling precedent on why
// cross-module coupling is treated as a structural hazard in this engine).
// Instead this file is VENDORED -- copied byte-for-byte, banner-stamped
// "do not edit", into each consumer spec/ module as its own single-file
// `hotamspec` package (internal/generator/recorder_vendor.go's
// VendoredRecorderSource does the copying; cmd/hotam's `gen-spec` writes the
// vendored copy to <domainDir>/spec/hotamspec/hotamspec.go, mirroring the
// existing crystal-vendoring precedent in claudemd_static.go for markdown,
// applied here to Go source instead). internal/invariants/recorder_check.go's
// check_recorder_current invariant sha256-compares the vendored copy against
// this canonical file (post banner-strip) and fires a violation on drift.
//
// API SHAPE (PLAN §2 D1 sketch, confirmed against the pilot's actual
// authored-test style -- domains/prat/spec/model/brd_package_test.go /
// forecast_test.go -- both currently plain `func Test*(t *testing.T)` using
// bare t.Fatalf/t.Errorf, NOT any assertion library):
//
//	s := hotamspec.Scenario(t, "R-brd-integrity-zero-blockers", "BRD sign-off requires zero blockers")
//	s.Given("a BRD package with one outstanding blocker", "rule_id", "ac-orphan")
//	err := p.SignOffP_G3()
//	s.When("SignOffP_G3 is called")
//	s.Then("sign-off is rejected with ErrBrdHasBlockers", errors.Is(err, ErrBrdHasBlockers))
//	s.Value("blocker_count", p.BlockerCount())
//
// A plain `go test` run is PURE ASSERTS: Then still calls t.Errorf/t.Fatalf
// exactly like the hand-rolled tests it replaces (so an author who deletes
// every hotamspec.* call except Then loses nothing -- the recorder is a
// strict superset of "the test still asserts", never a replacement asserter
// bolted on top of a separately-passing test). No artifact is written in
// this mode: the Scenario only starts writing an artifact when the ENGINE
// (never the author, never the test itself) sets an env var requesting one
// -- that record-mode wiring is W1.2's job (test_exec.go), NOT this file's;
// this file only defines the recording MECHANISM (Steps accumulate, Render
// produces canonical bytes) that a future env-gated writer in W1.2 can call.
// The env var name and the actual write-on-request wiring are intentionally
// NOT present here yet, so this task cannot accidentally pre-empt W1.2's own
// design decisions about where/how the artifact lands on disk.
//
// DETERMINISM (mandatory: this becomes the source for byte-identical SPEC.md
// generation in W1.3): every Step's rendered form is a pure function of its
// own fields, values are canonically rendered (renderValue below), Given/
// Value keyed maps are impossible by construction (kv pairs are stored as an
// ORDERED slice, never a map, so there is no map-iteration order to sort
// away), and no wall-clock time, random number, pointer address, or other
// non-reproducible quantity is ever captured into a Step. Callers who pass a
// pointer/struct value to Given/Value get its %v/%q rendering, which for the
// float/map/pointer-address hazards this package explicitly avoids (see
// renderValue) is stable across runs on the same inputs.
package hotamspec

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"testing"
)

// T is the minimal subset of *testing.T the Scenario needs. Scenario is
// constructed with a real *testing.T in every real test; this interface
// exists only so this package's OWN tests can substitute a fake recorder
// without depending on testing.T's full surface, and so a future non-testing
// driver (unlikely, but not foreclosed) is not structurally prevented.
type T interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// StepKind classifies one recorded Step.
type StepKind string

const (
	// StepGiven records a precondition: a fact about the world the scenario
	// starts from, with zero or more supporting key/value facts.
	StepGiven StepKind = "given"
	// StepWhen records the action under test: the one thing the scenario
	// does that the Then steps judge.
	StepWhen StepKind = "when"
	// StepThen records an assertion: a claim that must hold, judged via the
	// underlying *testing.T exactly like a hand-written t.Errorf/t.Fatalf
	// would (see Then's doc comment for the pass/fail contract).
	StepThen StepKind = "then"
	// StepValue records a bare fact captured for narration (e.g. an
	// intermediate value worth showing in the generated prose) without
	// itself asserting anything.
	StepValue StepKind = "value"
)

// kv is one ordered key/value pair attached to a Step. A slice, never a map:
// map iteration order is randomized per Go process (deliberately, since
// Go 1), so ANY map-typed field on Step would make Render's output
// non-deterministic run-to-run -- exactly the hazard PLAN-scenario-generated
// -spec.md §5 names first ("Детерминизм записанных значений (map-order,
// float-формат, адреса)"). Keeping kv as an explicit slice sidesteps the
// hazard structurally: there is no map to accidentally range over.
type kv struct {
	key      string
	rendered string
}

// Step is one recorded moment in a Scenario's narrative -- a Given
// precondition, the When action, a Then assertion (with its outcome), or a
// bare Value capture. Exported so a future artifact writer (W1.2) and SPEC.md
// generator (W1.3) can walk s.Steps() without this package growing a second,
// parallel serialization path.
type Step struct {
	Kind StepKind
	Desc string
	// Values is Given/Value's ordered key/value payload -- always in the
	// order the caller passed them (see kv's doc comment for why this is a
	// slice, not a map). Empty for When/Then.
	Values []Fact
	// Passed is only meaningful for StepThen: whether the asserted condition
	// held. Zero value (false) for every other Kind.
	Passed bool
}

// Fact is one exported (key, canonically-rendered value) pair from a Given
// or Value step.
type Fact struct {
	Key   string
	Value string
}

// Scenario narrates one verified_by test's proof, in Given/When/Then/Value
// steps, while asserting through the underlying T exactly as a hand-written
// test would. Construct with Scenario(t, reqID, title); call Given/When/
// Then/Value as the test proceeds.
type Scenario struct {
	t     T
	reqID string
	title string
	steps []Step
}

// NewScenario starts a new Scenario bound to t, narrating the proof of
// requirement reqID under the human-readable title. reqID is expected to be
// an R-anchor (e.g. "R-brd-integrity-zero-blockers") matching the
// requirement this test's verified_by entry is cited from, but this package
// does not itself validate that shape or cross-check it against a graph --
// that linkage is the mirror audit's job (PLAN-authored-spec-discipline.md
// §6's HONESTY BOUNDARY), not a mechanical property this recorder enforces.
//
// Named NewScenario (not the shorter Scenario used in the package doc
// comment's illustrative sketch) because Scenario is also this file's
// exported TYPE name -- Go does not allow a function and a type to share one
// identifier in the same package, so the constructor takes the New-prefixed
// form used throughout this codebase's own conventions (see
// model.NewBrdPackage / model.NewForecast in the pilot's authored spec/, the
// exact style this recorder is designed to sit alongside).
func NewScenario(t T, reqID, title string) *Scenario {
	t.Helper()
	return &Scenario{t: t, reqID: reqID, title: title}
}

// ReqID returns the requirement anchor this Scenario narrates.
func (s *Scenario) ReqID() string { return s.reqID }

// Title returns the scenario's human-readable title.
func (s *Scenario) Title() string { return s.title }

// Steps returns the recorded steps in call order -- a defensive copy, so a
// caller (e.g. a future artifact writer) cannot mutate the Scenario's own
// internal slice.
func (s *Scenario) Steps() []Step {
	out := make([]Step, len(s.steps))
	copy(out, s.steps)
	return out
}

// Given records a precondition the scenario starts from. kv is an optional,
// even-length list of alternating key, value, key, value, ... pairs (the
// same convention log/slog uses) -- e.g.
// s.Given("a BRD package with one outstanding blocker", "rule_id", "ac-orphan").
// An odd-length kv is a caller bug: Given reports it via t.Fatalf immediately
// (fail loudly at the call site, not silently drop the dangling key) rather
// than guessing which half of the pair was meant.
func (s *Scenario) Given(desc string, kvPairs ...any) {
	s.t.Helper()
	facts, ok := pairsToFacts(kvPairs)
	if !ok {
		s.t.Fatalf("hotamspec: Given(%q, ...) called with an odd number of key/value arguments (%d) -- pass alternating key, value pairs", desc, len(kvPairs))
		return
	}
	s.steps = append(s.steps, Step{Kind: StepGiven, Desc: desc, Values: facts})
}

// When records the action under test -- the one thing the scenario does
// that the subsequent Then steps judge. When does not itself take a
// value/error to record: the pilot's own authored-test style
// (brd_package_test.go/forecast_test.go) calls the real method directly
// (err := p.SignOffP_G3()) and asserts on its result via a plain `if`, so
// When's job is purely narrative -- naming what just happened -- while the
// actual value flows into the following Then's condition exactly as it
// already does in a hand-written test. This keeps When from becoming a
// second, parallel calling convention alongside the direct method call the
// test already makes.
func (s *Scenario) When(desc string) {
	s.steps = append(s.steps, Step{Kind: StepWhen, Desc: desc})
}

// Then asserts cond and records the outcome. On cond==false it reports a
// failure via t.Errorf(desc) -- non-fatal, exactly like a hand-written
// `t.Errorf(...)` in the pilot's existing tests (brd_package_test.go uses
// t.Fatalf for setup-invariant violations but the assertion pattern
// throughout is "compute, then judge with a plain if + t.Errorf/t.Fatalf");
// Then intentionally uses the non-fatal Errorf form (not Fatalf) so multiple
// Then steps in one Scenario all get a chance to report, matching Go's own
// t.Error-over-t.Fatal convention for "more than one thing worth telling the
// caller about in one test run" (see TestForecastVersion_String_
// MatchesClaimNaming's t.Errorf-in-a-loop precedent in forecast_test.go). A
// caller who needs fatal-on-failure semantics can call t.Fatal itself
// immediately after inspecting Then's own return (bool, so this is
// possible) instead of Then trying to guess when fatal is appropriate.
func (s *Scenario) Then(desc string, cond bool) bool {
	s.t.Helper()
	if !cond {
		s.t.Errorf("hotamspec: Then(%q) failed for %s (%s)", desc, s.reqID, s.title)
	}
	s.steps = append(s.steps, Step{Kind: StepThen, Desc: desc, Passed: cond})
	return cond
}

// Value records a bare fact for narration -- an intermediate or final value
// worth showing in the generated prose -- without itself asserting
// anything. v is canonically rendered via renderValue at call time (not
// lazily), so what gets narrated is exactly the value AS OF this call, never
// re-evaluated later.
func (s *Scenario) Value(key string, v any) {
	s.steps = append(s.steps, Step{Kind: StepValue, Values: []Fact{{Key: key, Value: renderValue(v)}}})
}

// pairsToFacts converts an alternating key,value,... list into an ordered
// []Fact, canonically rendering each value via renderValue. Returns
// ok=false if kvPairs has odd length (a caller bug -- see Given's doc
// comment for why this is a hard failure, not a silent drop). A non-string
// key is rendered via fmt.Sprintf("%v", ...) rather than rejected outright,
// since Go's log/slog accepts this too and rejecting it would make Given
// pickier than the convention it deliberately mirrors -- but the common,
// expected case is a string literal key.
func pairsToFacts(kvPairs []any) ([]Fact, bool) {
	if len(kvPairs)%2 != 0 {
		return nil, false
	}
	facts := make([]Fact, 0, len(kvPairs)/2)
	for i := 0; i < len(kvPairs); i += 2 {
		key := fmt.Sprintf("%v", kvPairs[i])
		facts = append(facts, Fact{Key: key, Value: renderValue(kvPairs[i+1])})
	}
	return facts, true
}

// renderValue canonically renders v to a deterministic string, closing the
// three hazards PLAN-scenario-generated-spec.md §5 names by name
// (map-order, float-format, addresses):
//
//   - map-order: a map[K]V value has its keys sorted (by their %v
//     rendering) before rendering "k1:v1, k2:v2, ..." -- Go's own %v on a
//     map already sorts keys as of Go 1.12+ for BUILT-IN fmt verbs, but this
//     function does not rely on that fmt-internal behavior remaining true
//     forever; it sorts explicitly via reflection so the guarantee lives in
//     THIS package's own contract, not an incidental fmt implementation
//     detail this package happens to depend on.
//   - float format: a float32/float64 is rendered via strconv.FormatFloat
//     with the 'g' verb and -1 precision (shortest round-trippable decimal
//     representation) rather than %v's default %g-with-implementation-
//     chosen-precision, so the same float64 value renders identically
//     regardless of which Go version or platform produced it.
//   - addresses: a pointer is DEREFERENCED and its pointee rendered
//     recursively (never rendered as "0xc000...", which changes every
//     process run and would make two otherwise-identical scenario runs
//     produce different artifact bytes); a nil pointer renders as the
//     literal "<nil>". error values render via .Error() (not %v's default,
//     which for a *fmt.wrapError etc. already calls Error() but this makes
//     the choice explicit rather than incidental).
//
// Anything else (string, bool, int*, uint*, a struct/slice with no pointer/
// float inside) falls through to fmt.Sprintf("%v", v), which is already
// deterministic for those kinds.
func renderValue(v any) string {
	if v == nil {
		return "<nil>"
	}
	if err, ok := v.(error); ok {
		return err.Error()
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr:
		if rv.IsNil() {
			return "<nil>"
		}
		return renderValue(rv.Elem().Interface())
	case reflect.Float32:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 32)
	case reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 64)
	case reflect.Map:
		return renderMap(rv)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// renderMap renders a reflect.Value of Kind Map deterministically: every key
// is rendered via renderValue, the (renderedKey, renderedValue) pairs are
// sorted by renderedKey, then joined as "k1:v1, k2:v2, ...". Sorting by the
// RENDERED key string (not the raw key, which may not even be orderable --
// e.g. a struct key) is what makes this total for any map key type Go
// allows (comparable, but not necessarily ordered).
func renderMap(rv reflect.Value) string {
	type entry struct{ k, v string }
	entries := make([]entry, 0, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		entries = append(entries, entry{
			k: renderValue(iter.Key().Interface()),
			v: renderValue(iter.Value().Interface()),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].k < entries[j].k })
	out := "map["
	for i, e := range entries {
		if i > 0 {
			out += " "
		}
		out += e.k + ":" + e.v
	}
	out += "]"
	return out
}

// compile-time assertion that *testing.T satisfies T -- if the standard
// library ever changes Helper/Errorf/Fatalf's signatures this file fails to
// build instead of failing mysteriously at a real call site.
var _ T = (*testing.T)(nil)
