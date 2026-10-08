package hotamspec

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakeT is a minimal T double so this package's own tests can inspect
// pass/fail without depending on testing.T's full surface (and, for the
// Given-odd-args case, without actually crashing the real go test run via a
// nested t.Fatalf on the outer *testing.T).
type fakeT struct {
	errors []string
	fatal  string
	fatal_ bool
	helped int
}

func (f *fakeT) Helper() { f.helped++ }
func (f *fakeT) Errorf(format string, args ...any) {
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}
func (f *fakeT) Fatalf(format string, args ...any) {
	f.fatal = fmt.Sprintf(format, args...)
	f.fatal_ = true
}

// fakeRecordT extends fakeT with the recordT surface (Cleanup/Name/Failed),
// so record-mode's write-on-Cleanup path can be exercised directly -- with
// full control over WHEN Cleanup fires and WHAT Failed() reports -- without
// running a genuinely failing real *testing.T subtest, which would bubble a
// FAIL up into this package's own test-suite verdict (a real t.Run("x", ...)
// that calls t.Errorf inside makes the OUTER test fail too; that is exactly
// right for production code but wrong for a unit test that is deliberately
// exercising the "verdict=fail" branch on purpose).
type fakeRecordT struct {
	fakeT
	name      string
	cleanups  []func()
	failedVal bool
}

func (f *fakeRecordT) Name() string      { return f.name }
func (f *fakeRecordT) Failed() bool      { return f.failedVal || len(f.errors) > 0 || f.fatal_ }
func (f *fakeRecordT) Cleanup(fn func()) { f.cleanups = append(f.cleanups, fn) }

// runCleanups invokes every registered Cleanup in LIFO order, mirroring Go's
// own testing.T.Cleanup contract.
func (f *fakeRecordT) runCleanups() {
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		f.cleanups[i]()
	}
}

func TestScenario_GivenWhenThen_HappyPath(t *testing.T) {
	ft := &fakeT{}
	s := NewScenario(ft, "R-example", "example scenario")
	if s.ReqID() != "R-example" || s.Title() != "example scenario" {
		t.Fatalf("ReqID/Title = %q/%q, want R-example/example scenario", s.ReqID(), s.Title())
	}

	s.Given("a fresh counter", "start", 0)
	s.When("the counter is incremented")
	ok := s.Then("the counter is now 1", 1 == 1)
	if !ok {
		t.Fatalf("Then returned false for a true condition")
	}
	s.Value("final_count", 1)

	if len(ft.errors) != 0 || ft.fatal_ {
		t.Fatalf("fakeT recorded a failure for an all-true scenario: errors=%v fatal=%q", ft.errors, ft.fatal)
	}

	steps := s.Steps()
	if len(steps) != 4 {
		t.Fatalf("Steps() len = %d, want 4", len(steps))
	}
	if steps[0].Kind != StepGiven || steps[0].Values[0].Key != "start" || steps[0].Values[0].Value != "0" {
		t.Fatalf("Given step = %+v, want Kind=given Values=[{start 0}]", steps[0])
	}
	if steps[1].Kind != StepWhen {
		t.Fatalf("When step Kind = %q, want when", steps[1].Kind)
	}
	if steps[2].Kind != StepThen || !steps[2].Passed {
		t.Fatalf("Then step = %+v, want Kind=then Passed=true", steps[2])
	}
	if steps[3].Kind != StepValue || steps[3].Values[0].Key != "final_count" || steps[3].Values[0].Value != "1" {
		t.Fatalf("Value step = %+v, want Kind=value Values=[{final_count 1}]", steps[3])
	}
}

func TestScenario_Then_RecordsFailureViaErrorf(t *testing.T) {
	ft := &fakeT{}
	s := NewScenario(ft, "R-example", "example scenario")
	ok := s.Then("this must fail", false)
	if ok {
		t.Fatalf("Then returned true for a false condition")
	}
	if len(ft.errors) != 1 {
		t.Fatalf("fakeT.errors len = %d, want 1 (Then must call Errorf on a false condition)", len(ft.errors))
	}
	steps := s.Steps()
	if len(steps) != 1 || steps[0].Passed {
		t.Fatalf("Steps() = %+v, want one Then step with Passed=false", steps)
	}
}

func TestScenario_Given_OddKVPairsFailsFatally(t *testing.T) {
	ft := &fakeT{}
	s := NewScenario(ft, "R-example", "example scenario")
	s.Given("bad call", "only_key")
	if !ft.fatal_ {
		t.Fatalf("Given with an odd-length kv list did not call Fatalf")
	}
}

func TestScenario_Steps_ReturnsDefensiveCopy(t *testing.T) {
	ft := &fakeT{}
	s := NewScenario(ft, "R-example", "example scenario")
	s.Given("one fact", "k", "v")
	steps := s.Steps()
	steps[0].Desc = "mutated"
	again := s.Steps()
	if again[0].Desc != "one fact" {
		t.Fatalf("Steps() leaked internal state: second call = %q, want unaffected %q", again[0].Desc, "one fact")
	}
}

func TestRenderValue_Determinism(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want string
	}{
		{"nil", nil, "<nil>"},
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"bool", true, "true"},
		{"float64_shortest", 1.0 / 3.0, "0.3333333333333333"},
		{"float32", float32(1.5), "1.5"},
		{"error", errors.New("boom"), "boom"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := renderValue(c.v)
			if got != c.want {
				t.Errorf("renderValue(%v) = %q, want %q", c.v, got, c.want)
			}
		})
	}
}

func TestRenderValue_MapIsSortedByKey(t *testing.T) {
	m := map[string]int{"zebra": 1, "apple": 2, "mango": 3}
	// Run several times: if renderValue ever depended on Go's randomized map
	// iteration order, at least one of these repeats would eventually differ.
	first := renderValue(m)
	for i := 0; i < 20; i++ {
		got := renderValue(m)
		if got != first {
			t.Fatalf("renderValue(map) not deterministic across calls: %q vs %q", first, got)
		}
	}
	want := "map[apple:2 mango:3 zebra:1]"
	if first != want {
		t.Errorf("renderValue(map) = %q, want %q", first, want)
	}
}

func TestRenderValue_PointerDereferencesNotAddress(t *testing.T) {
	n := 7
	got := renderValue(&n)
	if got != "7" {
		t.Errorf("renderValue(&n) = %q, want dereferenced \"7\" (never a 0x address)", got)
	}
	var nilPtr *int
	if got := renderValue(nilPtr); got != "<nil>" {
		t.Errorf("renderValue(nil *int) = %q, want <nil>", got)
	}
}

func TestPairsToFacts_PreservesOrderNotSorted(t *testing.T) {
	facts, ok := pairsToFacts([]any{"z", 1, "a", 2, "m", 3})
	if !ok {
		t.Fatalf("pairsToFacts returned ok=false for a valid even-length list")
	}
	want := []ValueFact{{"z", "1"}, {"a", "2"}, {"m", "3"}}
	if len(facts) != len(want) {
		t.Fatalf("facts len = %d, want %d", len(facts), len(want))
	}
	for i, f := range facts {
		if f != want[i] {
			t.Errorf("facts[%d] = %+v, want %+v (order must be call order, not sorted)", i, f, want[i])
		}
	}
}

// TestScenario_Eq_PassingRecordsGotValue proves Eq's core contract: on a
// passing comparison no error is reported, the recorded step is a StepThen
// with Passed=true, and its Desc embeds renderValue(got) -- the value from
// EXECUTION, not the author's hand. The desc assertion is deliberately
// written against got: asserting the rendered want instead would pass even
// if Eq rendered want by mistake, which is exactly the author-lies-in-text
// hazard Eq exists to close.
func TestScenario_Eq_PassingRecordsGotValue(t *testing.T) {
	ft := &fakeT{}
	s := NewScenario(ft, "R-example", "example scenario")
	ok := s.Eq("birth year is", 1987, 1987)
	if !ok {
		t.Fatalf("Eq returned false for equal values")
	}
	if len(ft.errors) != 0 || ft.fatal_ {
		t.Fatalf("fakeT recorded a failure for an equal Eq: errors=%v fatal=%q", ft.errors, ft.fatal)
	}
	steps := s.Steps()
	if len(steps) != 1 {
		t.Fatalf("Steps() len = %d, want 1", len(steps))
	}
	if steps[0].Kind != StepThen || !steps[0].Passed {
		t.Fatalf("Eq step = %+v, want Kind=then Passed=true", steps[0])
	}
	if want := "birth year is 1987"; steps[0].Desc != want {
		t.Errorf("Eq step Desc = %q, want %q (rendered got, not want-literal prose)", steps[0].Desc, want)
	}
}

// TestScenario_Eq_FailingReportsErrorfWithGotAndWant proves the failing
// branch: Eq returns false, calls t.Errorf exactly once (non-fatal, no
// Fatalf), and the message names the label plus BOTH renderings. The
// recorded desc still shows got (the executed value), with Passed=false.
func TestScenario_Eq_FailingReportsErrorfWithGotAndWant(t *testing.T) {
	ft := &fakeT{}
	s := NewScenario(ft, "R-example", "example scenario")
	ok := s.Eq("born", 1987, 1990)
	if ok {
		t.Fatalf("Eq returned true for unequal values")
	}
	if len(ft.errors) != 1 {
		t.Fatalf("fakeT.errors len = %d, want 1 (Eq must call Errorf on mismatch)", len(ft.errors))
	}
	if ft.fatal_ {
		t.Errorf("Eq must be non-fatal (Errorf, not Fatalf), got fatal=%q", ft.fatal)
	}
	msg := ft.errors[0]
	for _, part := range []string{"born", "1987", "1990"} {
		if !strings.Contains(msg, part) {
			t.Errorf("Eq error message %q missing %q", msg, part)
		}
	}
	steps := s.Steps()
	if len(steps) != 1 || steps[0].Passed {
		t.Fatalf("Steps() = %+v, want one Then step with Passed=false", steps)
	}
	if want := "born 1987"; steps[0].Desc != want {
		t.Errorf("Eq step Desc = %q, want %q (desc must show got even on failure)", steps[0].Desc, want)
	}
}

// TestScenario_Eq_CanonicalRendering proves got/want go through renderValue:
// float uses the shortest round-trippable form, maps are key-sorted, pointers
// are dereferenced (never a 0x address) -- and rendering BOTH sides through
// the same canonical form makes map-literal inequality (same content, Go's
// randomized iteration) not a false mismatch.
func TestScenario_Eq_CanonicalRendering(t *testing.T) {
	n := 7
	m := map[string]int{"zebra": 1, "apple": 2}
	cases := []struct {
		name  string
		got   any
		want  any
		label string
		desc  string
	}{
		{"float_shortest", 1.0 / 3.0, 1.0 / 3.0, "ratio", "ratio 0.3333333333333333"},
		{"float32", float32(1.5), float32(1.5), "scale", "scale 1.5"},
		{"map_sorted", m, map[string]int{"apple": 2, "zebra": 1}, "counts", "counts map[apple:2 zebra:1]"},
		{"pointer_deref", &n, 7, "depth", "depth 7"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ft := &fakeT{}
			s := NewScenario(ft, "R-example", "example scenario")
			if !s.Eq(c.label, c.got, c.want) {
				t.Fatalf("Eq(%v, %v) returned false", c.got, c.want)
			}
			if len(ft.errors) != 0 {
				t.Fatalf("unexpected errors: %v", ft.errors)
			}
			if got := s.Steps()[0].Desc; got != c.desc {
				t.Errorf("Eq step Desc = %q, want %q", got, c.desc)
			}
		})
	}
}

// TestScenario_Eq_DescIsGotNotWant is the anti-self-deception check: if Eq
// rendered want (instead of got) into the desc, the failing-case assertion
// "born 1987" would fail -- i.e. the desc contract above is load-bearing,
// not vacuous.
func TestScenario_Eq_DescIsGotNotWant(t *testing.T) {
	ft := &fakeT{}
	s := NewScenario(ft, "R-example", "example scenario")
	s.Eq("born", 1987, 1990)
	if got := s.Steps()[0].Desc; got == "born 1990" {
		t.Errorf("Eq step Desc = %q -- rendered want instead of got", got)
	}
}

type birthYear int
type kindStr string
type pointA struct{ X, Y int }
type pointB struct{ X, Y int }

// TestScenario_Eq_TypeStrictness: Eq requires identical dynamic type (after
// pointer deref) plus identical rendering; untyped-constant convenience
// converts want to got's same-family named type only when lossless.
func TestScenario_Eq_TypeStrictness(t *testing.T) {
	n := 5
	cases := []struct {
		name      string
		got, want any
		ok        bool
	}{
		{"int_vs_float_literal_fails", 1, 1.0, false},
		{"float_vs_int_literal_fails", 1.0, 1, false},
		{"int_vs_int_passes", 1, 1, true},
		{"named_int_vs_int_literal_passes", birthYear(1987), 1987, true},
		{"named_int_vs_int_literal_mismatch_fails", birthYear(1987), 1990, false},
		{"named_string_vs_string_passes", kindStr("a"), "a", true},
		{"named_string_vs_int_fails", kindStr("1"), 1, false},
		{"int64_vs_int_literal_passes", int64(7), 7, true},
		{"uint8_overflow_literal_fails", uint8(44), 300, false},
		{"uint_negative_literal_fails", uint(1), -1, false},
		{"named_int_vs_float_literal_fails", birthYear(1), 1.0, false},
		{"typed_named_vs_other_named_fails", birthYear(1), kindStr("1"), false},
		{"struct_same_type_passes", pointA{1, 2}, pointA{1, 2}, true},
		{"struct_mismatched_types_fail", pointA{1, 2}, pointB{1, 2}, false},
		{"pointer_deref_passes", &n, 5, true},
		{"nil_nil_passes", nil, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ft := &fakeT{}
			s := NewScenario(ft, "R-example", "example")
			if got := s.Eq("v", c.got, c.want); got != c.ok {
				t.Fatalf("Eq(%#v, %#v) = %v, want %v (errors=%v)", c.got, c.want, got, c.ok, ft.errors)
			}
			if c.ok != (len(ft.errors) == 0) {
				t.Fatalf("errors=%v, ok=%v", ft.errors, c.ok)
			}
			if st := s.Steps()[0]; st.Passed != c.ok {
				t.Fatalf("step Passed=%v, want %v", st.Passed, c.ok)
			}
		})
	}
}

// TestWithWhen_RendersLikeExplicitWhen: a default When yields exactly the
// steps an explicit When placed before the first Then would.
func TestWithWhen_RendersLikeExplicitWhen(t *testing.T) {
	explicit := NewScenario(&fakeT{}, "R-example", "example")
	explicit.Given("g", "k", 1)
	explicit.When("init")
	explicit.Then("a", true)
	explicit.Eq("n", 3, 3)

	def := NewScenario(&fakeT{}, "R-example", "example", WithWhen("init"))
	def.Given("g", "k", 1)
	def.Then("a", true)
	def.Eq("n", 3, 3)

	if !reflect.DeepEqual(explicit.Steps(), def.Steps()) {
		t.Fatalf("WithWhen steps differ:\nexplicit=%+v\ndefault =%+v", explicit.Steps(), def.Steps())
	}
	if n := len(def.Steps()); n != 4 {
		t.Fatalf("steps = %d, want 4", n)
	}
}

// TestWithWhen_ExplicitWhenWins: explicit When suppresses the default (no
// duplicate), and WithWhen inserts only once.
func TestWithWhen_ExplicitWhenWins(t *testing.T) {
	s := NewScenario(&fakeT{}, "R-example", "example", WithWhen("default"))
	s.When("explicit")
	s.Then("a", true)
	s.Then("b", true)
	var whens []string
	for _, st := range s.Steps() {
		if st.Kind == StepWhen {
			whens = append(whens, st.Desc)
		}
	}
	if !reflect.DeepEqual(whens, []string{"explicit"}) {
		t.Fatalf("whens = %v, want [explicit]", whens)
	}
}

// TestWithWhen_NotAppliedWithoutOption: plain scenarios record no When.
func TestWithWhen_NotAppliedWithoutOption(t *testing.T) {
	s := NewScenario(&fakeT{}, "R-example", "example")
	s.Then("a", true)
	if len(s.Steps()) != 1 {
		t.Fatalf("steps = %+v, want only the Then", s.Steps())
	}
}

// TestNewScenario_NoRecordDirEnv_WritesNoArtifact proves record-mode is
// strictly opt-in: with RecordDirEnv unset (the default, what a plain `go
// test` run always sees), NewScenario must not write anything to disk even
// though the scenario runs to completion normally.
func TestNewScenario_NoRecordDirEnv_WritesNoArtifact(t *testing.T) {
	os.Unsetenv(RecordDirEnv)
	dir := t.TempDir()

	ft := &fakeRecordT{name: "TestFakeNoRecordDir"}
	s := NewScenario(ft, "R-example", "no record dir")
	s.Given("a fact", "k", "v")
	s.When("something happens")
	s.Then("it holds", true)
	ft.runCleanups()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no files written to %s with %s unset, got %v", dir, RecordDirEnv, entries)
	}
	if len(ft.cleanups) != 0 {
		t.Fatalf("expected NewScenario to register no Cleanup when %s is unset, got %d", RecordDirEnv, len(ft.cleanups))
	}
}

// TestNewScenario_RecordMode_WritesCanonicalArtifact proves the core
// record-mode contract: with RecordDirEnv set, a Scenario's Cleanup writes
// <dir>/<reqID>__<TestName>.json holding the expected Artifact shape,
// verdict "pass" for an all-true scenario.
func TestNewScenario_RecordMode_WritesCanonicalArtifact(t *testing.T) {
	dir := t.TempDir()
	os.Setenv(RecordDirEnv, dir)
	t.Cleanup(func() { os.Unsetenv(RecordDirEnv) })

	ft := &fakeRecordT{name: "TestInnerHappy"}
	s := NewScenario(ft, "R-example-record", "record mode happy path")
	s.Given("a fresh counter", "start", 0)
	s.When("the counter is incremented")
	s.Then("the counter is now 1", 1 == 1)
	s.Value("final_count", 1)
	ft.runCleanups()

	wantPath := filepath.Join(dir, "R-example-record__TestInnerHappy.json")
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("expected artifact at %s, got error: %v", wantPath, err)
	}

	var art Artifact
	if err := json.Unmarshal(data, &art); err != nil {
		t.Fatalf("artifact is not valid JSON: %v\n%s", err, data)
	}
	if art.ReqID != "R-example-record" {
		t.Errorf("ReqID = %q, want R-example-record", art.ReqID)
	}
	if art.Title != "record mode happy path" {
		t.Errorf("Title = %q, want %q", art.Title, "record mode happy path")
	}
	if art.Verdict != "pass" {
		t.Errorf("Verdict = %q, want pass", art.Verdict)
	}
	if len(art.Steps) != 4 {
		t.Fatalf("Steps len = %d, want 4: %+v", len(art.Steps), art.Steps)
	}
	if art.Steps[0].Kind != StepGiven || len(art.Steps[0].Values) != 1 || art.Steps[0].Values[0] != (ArtifactFact{"start", "0"}) {
		t.Errorf("Steps[0] = %+v, want Given start=0", art.Steps[0])
	}
	if art.Steps[2].Kind != StepThen || !art.Steps[2].Passed {
		t.Errorf("Steps[2] = %+v, want Then Passed=true", art.Steps[2])
	}
}

// TestNewScenario_RecordMode_VerdictFailWhenThenFails proves verdict tracks
// the test's real Failed() state, not an independently-tracked notion of
// success: a Scenario whose Then step fails must record verdict "fail".
func TestNewScenario_RecordMode_VerdictFailWhenThenFails(t *testing.T) {
	dir := t.TempDir()
	os.Setenv(RecordDirEnv, dir)
	t.Cleanup(func() { os.Unsetenv(RecordDirEnv) })

	ft := &fakeRecordT{name: "TestInnerFailing"}
	s := NewScenario(ft, "R-example-record-fail", "record mode failing path")
	s.Given("a doomed precondition", "k", "v")
	s.When("the action runs")
	s.Then("this assertion is false on purpose", false)
	ft.runCleanups()

	wantPath := filepath.Join(dir, "R-example-record-fail__TestInnerFailing.json")
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("expected artifact at %s, got error: %v", wantPath, err)
	}
	var art Artifact
	if err := json.Unmarshal(data, &art); err != nil {
		t.Fatalf("artifact is not valid JSON: %v\n%s", err, data)
	}
	if art.Verdict != "fail" {
		t.Errorf("Verdict = %q, want fail (the Then step failed)", art.Verdict)
	}
}

// TestNewScenario_RecordMode_DeterministicAcrossRuns proves the byte-identical
// contract PLAN-scenario-generated-spec.md §2 D1 demands: running the
// identical scenario twice (two separate record dirs, two separate fakeRecordT
// instances sharing the same test Name()) must produce byte-identical
// artifact content.
func TestNewScenario_RecordMode_DeterministicAcrossRuns(t *testing.T) {
	run := func() []byte {
		dir := t.TempDir()
		os.Setenv(RecordDirEnv, dir)
		defer os.Unsetenv(RecordDirEnv)

		ft := &fakeRecordT{name: "TestInnerDeterministic"}
		s := NewScenario(ft, "R-determinism", "deterministic scenario")
		s.Given("inputs", "a", 1, "b", 2.5, "m", map[string]int{"z": 1, "a": 2})
		s.When("computed")
		s.Then("result holds", true)
		s.Value("out", 42)
		ft.runCleanups()

		path := filepath.Join(dir, "R-determinism__TestInnerDeterministic.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		return data
	}

	first := run()
	second := run()
	if string(first) != string(second) {
		t.Fatalf("record-mode artifact not byte-identical across two runs:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestArtifactFileName_SanitizesSubtestSlashes proves artifactFileName turns
// a subtest's "/"-separated Name() into a filesystem-safe file name (Windows
// rejects "/" in a path component outright).
func TestArtifactFileName_SanitizesSubtestSlashes(t *testing.T) {
	got := artifactFileName("R-x", "TestFoo/case_1")
	want := "R-x__TestFoo_case_1.json"
	if got != want {
		t.Errorf("artifactFileName = %q, want %q", got, want)
	}
}

type atomProbe struct{ calls int }

func (p *atomProbe) BirthYear() int { p.calls++; return 1987 }
func (p *atomProbe) Related() bool  { p.calls++; return false }

type genericAtom[T any] struct{ value T }

func (p genericAtom[T]) Value() T         { return p.value }
func (p *genericAtom[T]) PointerValue() T { return p.value }
func atomTopLevel() int                   { return 1 }

func TestAtomMethodIdentityAndRejection(t *testing.T) {
	p := &atomProbe{}
	g := genericAtom[int]{value: 7}
	for _, tc := range []struct {
		method any
		suffix string
	}{
		{p.BirthYear, ".atomProbe.BirthYear"},
		{g.Value, ".genericAtom.Value"},
		{g.PointerValue, ".genericAtom.PointerValue"},
	} {
		subject, err := methodSubject(tc.method)
		if err != nil || !strings.HasSuffix(subject, tc.suffix) || strings.ContainsAny(subject, "*[]()") {
			t.Fatalf("subject = %q, err = %v", subject, err)
		}
	}
	captured := 1987
	for _, method := range []any{atomTopLevel, func() int { return 1 }, func() int { return captured }, (func() int)(nil)} {
		if subject, err := methodSubject(method); err == nil {
			t.Fatalf("invalid method accepted as %q", subject)
		}
	}
	ft := &fakeT{}
	Fact(ft, atomTopLevel, 1)
	if !ft.fatal_ {
		t.Fatal("top-level function must fail the test")
	}
}

func TestAtomsExecuteOnceAndKeepDistinctArtifacts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(RecordDirEnv, dir)
	ft := &fakeRecordT{name: "TestAtoms"}
	p := &atomProbe{}
	first := Fact(ft, p.BirthYear, 1987)
	second := Fact(ft, p.BirthYear, 1987)
	relation := Holds(ft, p.Related, Expect(false), first, second)
	if !relation.Passed || p.calls != 3 {
		t.Fatalf("evidence = %+v, calls = %d", relation, p.calls)
	}
	ft.runCleanups()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("artifacts = %d, want 3", len(entries))
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var art Artifact
		if err := json.Unmarshal(data, &art); err != nil {
			t.Fatal(err)
		}
		if art.Verdict != "pass" {
			t.Fatalf("artifact = %+v", art)
		}
		if art.Mode == "holds" {
			if len(art.Steps) != 3 || art.Steps[0].Value != "false" || art.Steps[1].Subject != first.Subject || art.Steps[1].Value != "1987" {
				t.Fatalf("relation steps = %+v", art.Steps)
			}
		}
	}
}

func TestFailedFactCannotProducePassingRelation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(RecordDirEnv, dir)
	ft := &fakeRecordT{name: "TestMismatch"}
	p := &atomProbe{}
	e := Fact(ft, p.BirthYear, 1988)
	r := Holds(ft, p.Related, Expect(false), e)
	if e.Passed || r.Passed || p.calls != 2 {
		t.Fatalf("fact=%+v relation=%+v calls=%d", e, r, p.calls)
	}
	ft.runCleanups()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var art Artifact
		if err := json.Unmarshal(data, &art); err != nil {
			t.Fatal(err)
		}
		if art.Verdict != "fail" {
			t.Fatalf("failed assertion produced %+v", art)
		}
	}
}

func TestHoldsDuplicateExpectationDoesNotExecute(t *testing.T) {
	ft := &fakeT{}
	p := &atomProbe{}
	Holds(ft, p.Related, Expect(false), Expect(true))
	if !ft.fatal_ || p.calls != 0 {
		t.Fatalf("fatal=%t calls=%d", ft.fatal_, p.calls)
	}
}

func (p *atomProbe) NestedResult() ObservedValue[string] {
	p.calls++
	return Observed("", Observe("inner check", "request-17", false, true))
}

func TestFactRecordsNestedFailureAndKeepsEmptyExpectedValue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(RecordDirEnv, dir)
	ft := &fakeRecordT{name: "TestFactRecordsNestedFailureAndKeepsEmptyExpectedValue"}
	probe := &atomProbe{}
	evidence := Fact(
		ft, probe.NestedResult, "",
		WithInput("request-17"),
		WithContext(ArtifactContext{
			Implementation:        "spec/model/result.go:probe.NestedResult",
			ImplementationVersion: "impl-4",
			SpecVersion:           "spec-9",
			Profile:               "linux-amd64",
		}),
	)
	if probe.calls != 1 {
		t.Fatalf("bound method calls = %d, want exactly one", probe.calls)
	}
	if evidence.Passed || len(ft.errors) == 0 {
		t.Fatalf("nested mismatch did not fail the actual assertion: evidence=%+v errors=%v", evidence, ft.errors)
	}
	ft.runCleanups()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("artifacts = %d, want one", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var artifact Artifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Verdict != "fail" || len(artifact.Steps) != 1 {
		t.Fatalf("artifact verdict/steps = %q/%d; want failed single-step evidence", artifact.Verdict, len(artifact.Steps))
	}
	step := artifact.Steps[0]
	if step.Expected == nil || *step.Expected != "" {
		t.Fatalf("empty expected value was lost: %+v", step)
	}
	if step.Input == nil || *step.Input != "request-17" {
		t.Fatalf("input = %v, want request-17", step.Input)
	}
	if step.Context == nil || step.Context.ImplementationVersion != "impl-4" || step.Context.SpecVersion != "spec-9" {
		t.Fatalf("context = %+v", step.Context)
	}
	if len(step.Observations) != 2 ||
		step.Observations[0] != (Observation{Name: "inner check", Input: "request-17", Actual: "false", Expected: "true", Passed: false}) ||
		!step.Observations[1].Passed || step.Observations[1].Expected != "" {
		t.Fatalf("observations lost a nested mismatch or empty expectation: %+v", step.Observations)
	}
}

func TestEqPreservesNilAndEmptyExpectedEvidence(t *testing.T) {
	s := NewScenario(&fakeT{}, "R-nil-empty", "nil and empty comparison")
	if !s.Eq("nil", nil, nil) || !s.Eq("empty", "", "") {
		t.Fatal("equal nil or empty comparison failed")
	}
	steps := s.Steps()
	if len(steps) != 2 || !steps[0].HasExpected || !steps[1].HasExpected {
		t.Fatalf("expected-value presence was lost: %+v", steps)
	}
	if steps[0].Expected != "<nil>" || steps[1].Expected != "" {
		t.Fatalf("nil/empty expected values collapsed: %q / %q", steps[0].Expected, steps[1].Expected)
	}
	if len(steps[0].Observations) != 1 || steps[0].Observations[0].Expected != "<nil>" ||
		len(steps[1].Observations) != 1 || steps[1].Observations[0].Expected != "" {
		t.Fatalf("nil/empty comparison observations = %+v", steps)
	}
}

type recorderTypedProbe struct {
	rawCalls      int
	lengthCalls   int
	numberCalls   int
	textCalls     int
	floatCalls    int
	predicateCall int
	summaryCalls  int
	objectCalls   int
	bytes         []byte
	number        int
	text          string
	float         float64
}

func (p *recorderTypedProbe) RawBytes() []byte {
	p.rawCalls++
	return p.bytes
}

func (p *recorderTypedProbe) Length() int {
	p.lengthCalls++
	return len(p.bytes)
}

func (p *recorderTypedProbe) Number() int {
	p.numberCalls++
	return p.number
}

func (p *recorderTypedProbe) Label() string {
	p.textCalls++
	return p.text
}

func (p *recorderTypedProbe) SignedZero() float64 {
	p.floatCalls++
	return p.float
}

func (p *recorderTypedProbe) Predicate() bool {
	p.predicateCall++
	return true
}

func (p *recorderTypedProbe) Summary() ObservedValue[string] {
	p.summaryCalls++
	return Observed("ready", Observe("inner equality", Text("raw"), false, true))
}

func (p *recorderTypedProbe) ObjectResult() TypedValue {
	p.objectCalls++
	return Object(map[string]TypedValue{
		"kind":  Text("integer"),
		"value": Scalar("integer", int64(0)),
	})
}

func TestWithCaseRecordsRealIdentityAndIndependentTypedEvidence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(RecordDirEnv, dir)
	t.Setenv("HOTAM_RECORD_STDOUT", "")
	input := []byte{0xff, '\r', '\n'}
	expected := []byte{0xfe, '\n'}
	ft := &fakeRecordT{name: "TestWithCaseRecordsRealIdentityAndIndependentTypedEvidence/invalid_utf8"}
	probe := &recorderTypedProbe{bytes: input}
	evidence := Fact(
		ft, probe.RawBytes, Bytes(expected),
		WithInput(input),
		WithCase(CaseContext{
			ID:      "case-invalid-utf8",
			AtomIDs: []string{"R-decoder-byte-boundary"},
			Profile: "strict",
			Target:  "direct-sdk",
			Fixtures: []FixtureRef{{
				ID: "fixture-invalid-utf8", Category: "local", Path: "fixtures/invalid.bin",
				Role: "input", SHA256: "f00d", RawBytes: true,
			}},
			Conditions: []ConditionEvidence{{Name: "input_is_invalid_utf8", Matched: false}},
			Sides:      []string{"input"},
			Selection:  &SelectionEvidence{Matched: []string{}, Selected: ""},
			Operation:  "decode",
			Producer:   "native-sdk",
		}),
		WithContext(ArtifactContext{
			Implementation: "sdk", ImplementationVersion: "2.1", Profile: "strict",
			Target: "direct-sdk", Operation: "decode", Producer: "native-sdk",
			Components: []Component{
				{ID: "z-adapter", Role: "adapter", Version: "a2", Measured: false},
				{ID: "a-core", Role: "core", Version: "c7", SHA256: "beef", Measured: true},
			},
		}),
	)
	if evidence.Passed || probe.rawCalls != 1 || len(ft.errors) == 0 {
		t.Fatalf("case mismatch did not fail exactly one execution: evidence=%+v calls=%d errors=%v", evidence, probe.rawCalls, ft.errors)
	}
	ft.runCleanups()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("artifact entries = %v, err=%v; want one artifact", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	var artifact Artifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Mode != "rule" || artifact.Test != ft.name || artifact.ReqID == "case-invalid-utf8" ||
		!strings.Contains(artifact.ReqID, "raw-bytes") || artifact.Verdict != "fail" {
		t.Fatalf("method/case identity or verdict was conflated: %+v", artifact)
	}
	if artifact.Case == nil || artifact.Case.ID != "case-invalid-utf8" ||
		artifact.Case.Profile != "strict" || artifact.Case.Target != "direct-sdk" ||
		artifact.Case.Operation != "decode" || artifact.Case.Producer != "native-sdk" ||
		len(artifact.Case.Fixtures) != 1 || !artifact.Case.Fixtures[0].RawBytes ||
		len(artifact.Case.Conditions) != 1 || artifact.Case.Conditions[0].Matched ||
		artifact.Case.Selection == nil || artifact.Case.Selection.Matched == nil ||
		artifact.Case.Selection.Selected != "" {
		t.Fatalf("case context lost declared metadata or explicit empty selection: %+v", artifact.Case)
	}
	if artifact.CaseInput == nil || artifact.CaseInput.Kind != "bytes" ||
		artifact.CaseInput.Encoding != "base64" || artifact.CaseInput.Bytes != Bytes(input).Bytes ||
		artifact.CaseExpected == nil || artifact.CaseExpected.Bytes != Bytes(expected).Bytes ||
		artifact.CaseInput.Bytes == artifact.CaseExpected.Bytes {
		t.Fatalf("case input/independent expected payload = %+v / %+v", artifact.CaseInput, artifact.CaseExpected)
	}
	if len(artifact.Steps) != 1 || artifact.Steps[0].Context == nil ||
		artifact.Steps[0].Context.Operation != "decode" ||
		len(artifact.Steps[0].Context.Components) != 2 ||
		artifact.Steps[0].Context.Components[0].ID != "a-core" ||
		artifact.Steps[0].Context.Components[0].Measured != true ||
		artifact.Steps[0].Context.Components[1].ID != "z-adapter" ||
		artifact.Steps[0].Context.Components[1].Measured {
		t.Fatalf("runtime context components were lost, inferred, or not normalized: %+v", artifact.Steps)
	}
	var encoded struct {
		Steps []struct {
			Context struct {
				Components []map[string]json.RawMessage `json:"components"`
			} `json:"context"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(data, &encoded); err != nil {
		t.Fatal(err)
	}
	measured, present := encoded.Steps[0].Context.Components[1]["measured"]
	if !present || string(measured) != "false" {
		t.Fatalf("explicit unmeasured state was omitted from artifact JSON: %s", data)
	}
	observations := artifact.Steps[0].Observations
	if len(observations) != 1 || observations[0].RawInput == nil ||
		observations[0].RawInput.Bytes != Bytes(input).Bytes ||
		observations[0].RawActual == nil || observations[0].RawActual.Bytes != Bytes(input).Bytes ||
		observations[0].RawExpected == nil || observations[0].RawExpected.Bytes != Bytes(expected).Bytes ||
		observations[0].Actual == observations[0].Expected {
		t.Fatalf("typed observation did not retain exact byte mismatch: %+v", observations)
	}
}

func TestTypedComparisonsPreserveScalarAndDiagnosticBoundaries(t *testing.T) {
	t.Setenv(RecordDirEnv, "")
	t.Setenv("HOTAM_RECORD_STDOUT", "")
	probe := &recorderTypedProbe{
		number: 0, text: "0", float: math.Copysign(0, -1),
	}
	ft := &fakeRecordT{name: "TestTypedComparisonsPreserveScalarAndDiagnosticBoundaries"}
	zero := Fact(ft, probe.Number, Integer("0"), WithCase(CaseContext{ID: "case-integer-zero"}))
	scalar := Fact(ft, probe.Number, Scalar("integer", int64(0)), WithCase(CaseContext{ID: "case-scalar-integer-zero"}))
	absent := Fact(ft, probe.Number, TypedValue{Kind: "integer", ScalarKind: "integer"}, WithCase(CaseContext{ID: "case-integer-absent"}))
	stringValue := Fact(ft, probe.Label, Integer("0"), WithCase(CaseContext{ID: "case-integer-versus-string"}))
	signedZero := Fact(ft, probe.SignedZero, Float64Bits(0), WithCase(CaseContext{ID: "case-float-signed-zero"}))
	if !zero.Passed || !scalar.Passed || absent.Passed || stringValue.Passed || signedZero.Passed ||
		probe.numberCalls != 3 || probe.textCalls != 1 || probe.floatCalls != 1 {
		t.Fatalf("typed scalar boundaries or execution counts are wrong: zero=%+v scalar=%+v absent=%+v string=%+v float=%+v calls=%d/%d/%d",
			zero, scalar, absent, stringValue, signedZero, probe.numberCalls, probe.textCalls, probe.floatCalls)
	}
	if zero.CaseExpected == nil || zero.CaseExpected.Integer != "0" ||
		absent.CaseExpected == nil || absent.CaseExpected.Integer != "" ||
		signedZero.Observations[len(signedZero.Observations)-1].RawActual.FloatBits != Float64Bits(probe.float).FloatBits ||
		signedZero.Observations[len(signedZero.Observations)-1].RawExpected.FloatBits != Float64Bits(0).FloatBits {
		t.Fatalf("exact scalar payloads were not retained: zero=%+v absent=%+v float=%+v", zero, absent, signedZero)
	}
	zeroLine := 0
	diagnostic := Observe(
		"diagnostic line presence", Text("input"),
		Diagnostic(DiagnosticValue{Code: "parse"}),
		Diagnostic(DiagnosticValue{Code: "parse", Line: &zeroLine}),
	)
	if diagnostic.Passed || diagnostic.RawInput == nil || diagnostic.RawActual == nil ||
		diagnostic.RawActual.Diagnostic.Line != nil ||
		diagnostic.RawExpected.Diagnostic.Line == nil || *diagnostic.RawExpected.Diagnostic.Line != 0 {
		t.Fatalf("absent diagnostic line collapsed with explicit zero: %+v", diagnostic)
	}
	data, err := json.Marshal(diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Observation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.RawExpected.Diagnostic.Line == nil || *decoded.RawExpected.Diagnostic.Line != 0 ||
		decoded.RawActual.Diagnostic.Line != nil {
		t.Fatalf("diagnostic presence was lost through JSON: %+v", decoded)
	}
	span := ByteSpan{Start: 0, End: 2}
	withSpan := Diagnostic(DiagnosticValue{Code: "parse", Line: &zeroLine, Span: &span})
	spanObservation := Observe("half-open span", Text("bytes"), withSpan, Diagnostic(DiagnosticValue{Code: "parse", Line: &zeroLine}))
	if spanObservation.Passed || spanObservation.RawActual.Diagnostic.Span == nil ||
		spanObservation.RawActual.Diagnostic.Span.Start != 0 ||
		spanObservation.RawActual.Diagnostic.Span.End != 2 ||
		spanObservation.RawExpected.Diagnostic.Span != nil {
		t.Fatalf("producer-supplied half-open byte span was lost: %+v", spanObservation)
	}
	boolPresence := Observe(
		"explicit boolean false", Text("input"),
		TypedValue{Kind: "bool"}, Scalar("bool", false),
	)
	if boolPresence.Passed || boolPresence.RawActual.Bool != nil ||
		boolPresence.RawExpected.Bool == nil || *boolPresence.RawExpected.Bool ||
		boolPresence.RawExpected.Text != nil {
		t.Fatalf("absent bool collapsed with false or used a text payload: %+v", boolPresence)
	}
	boolJSON, err := json.Marshal(boolPresence)
	if err != nil {
		t.Fatal(err)
	}
	var decodedBool Observation
	if err := json.Unmarshal(boolJSON, &decodedBool); err != nil {
		t.Fatal(err)
	}
	if decodedBool.RawExpected.Bool == nil || *decodedBool.RawExpected.Bool {
		t.Fatalf("explicit false disappeared from typed JSON: %s", boolJSON)
	}
}

func TestObjectTypedComparisonKeepsIntegerAndTextFieldsDistinct(t *testing.T) {
	t.Setenv(RecordDirEnv, "")
	t.Setenv("HOTAM_RECORD_STDOUT", "")
	probe := &recorderTypedProbe{}
	ft := &fakeRecordT{name: "TestObjectTypedComparisonKeepsIntegerAndTextFieldsDistinct"}
	evidence := Fact(
		ft, probe.ObjectResult,
		Object(map[string]TypedValue{
			"kind":  Text("integer"),
			"value": Text("0"),
		}),
		WithCase(CaseContext{ID: "case-typed-object"}),
	)
	if evidence.Passed || probe.objectCalls != 1 {
		t.Fatalf("object mismatch was accepted or executed more than once: evidence=%+v calls=%d", evidence, probe.objectCalls)
	}
	observation := evidence.Observations[len(evidence.Observations)-1]
	if observation.RawActual == nil || observation.RawExpected == nil ||
		observation.RawActual.Fields["value"].Kind != "integer" ||
		observation.RawActual.Fields["value"].Integer != "0" ||
		observation.RawExpected.Fields["value"].Kind != "text" ||
		observation.RawExpected.Fields["value"].Text == nil ||
		*observation.RawExpected.Fields["value"].Text != "0" {
		t.Fatalf("nested scalar kinds were flattened: %+v", observation)
	}
}

func TestWithCaseRejectsMissingDuplicateAndUnavailableIdentityBeforeExecution(t *testing.T) {
	probe := &recorderTypedProbe{}
	missing := &fakeRecordT{name: "TestWithCaseMissingID"}
	Fact(missing, probe.Number, Integer("0"), WithCase(CaseContext{}))
	if !missing.fatal_ || probe.numberCalls != 0 {
		t.Fatalf("missing case ID was accepted or method executed: fatal=%v calls=%d", missing.fatal_, probe.numberCalls)
	}
	duplicate := &fakeRecordT{name: "TestWithCaseDuplicate"}
	context := CaseContext{ID: "case-duplicate"}
	Fact(duplicate, probe.Number, Integer("0"), WithCase(context), WithCase(context))
	if !duplicate.fatal_ || probe.numberCalls != 0 {
		t.Fatalf("duplicate case metadata was accepted or method executed: fatal=%v calls=%d", duplicate.fatal_, probe.numberCalls)
	}
	unavailable := &fakeT{}
	Fact(unavailable, probe.Number, Integer("0"), WithCase(CaseContext{ID: "case-no-test-name"}))
	if !unavailable.fatal_ || probe.numberCalls != 0 {
		t.Fatalf("missing real test identity was accepted or method executed: fatal=%v calls=%d", unavailable.fatal_, probe.numberCalls)
	}
	holds := &fakeRecordT{name: "TestHoldsWithCaseMissingID"}
	Holds(holds, probe.Predicate, WithCase(CaseContext{}))
	if !holds.fatal_ || probe.predicateCall != 0 {
		t.Fatalf("Holds missing case ID was accepted or predicate executed: fatal=%v calls=%d", holds.fatal_, probe.predicateCall)
	}
}

func TestHoldsRuleKeepsNestedFailureAndOptions(t *testing.T) {
	t.Setenv(RecordDirEnv, "")
	t.Setenv("HOTAM_RECORD_STDOUT", "")
	ft := &fakeRecordT{name: "TestHoldsRuleKeepsNestedFailureAndOptions"}
	probe := &recorderTypedProbe{number: 0}
	passingSibling := Fact(ft, probe.Number, 0)
	failedSummary := Fact(ft, probe.Summary, "ready")
	result := Holds(
		ft, probe.Predicate, Expect(true),
		WithCase(CaseContext{ID: "case-summary-selection", Operation: "select", Producer: "adapter"}),
		WithInput(Text("request-4")),
		WithContext(ArtifactContext{Target: "composed-app", Operation: "select", Producer: "adapter"}),
		passingSibling, failedSummary,
	)
	if !passingSibling.Passed || failedSummary.Passed || result.Passed ||
		probe.numberCalls != 1 || probe.summaryCalls != 1 || probe.predicateCall != 1 ||
		len(ft.errors) < 2 {
		t.Fatalf("Holds hid a nested mismatch or reran an operation: sibling=%+v summary=%+v result=%+v calls=%d/%d/%d errors=%v",
			passingSibling, failedSummary, result, probe.numberCalls, probe.summaryCalls, probe.predicateCall, ft.errors)
	}
	if result.Case == nil || result.Case.ID != "case-summary-selection" ||
		result.CaseExpected == nil || result.CaseExpected.Kind != "bool" ||
		result.CaseExpected.Bool == nil || !*result.CaseExpected.Bool ||
		result.CaseInput == nil || result.CaseInput.Text == nil || *result.CaseInput.Text != "request-4" ||
		result.Context == nil || result.Context.Target != "composed-app" {
		t.Fatalf("Holds did not propagate independent case/input/context options: %+v", result)
	}
	var sawPassingSibling, sawNestedFailure bool
	for _, observation := range result.Observations {
		if observation.Name == passingSibling.Subject && observation.Passed {
			sawPassingSibling = true
		}
		if observation.Name == "inner equality" && !observation.Passed {
			sawNestedFailure = true
		}
	}
	if !sawPassingSibling || !sawNestedFailure {
		t.Fatalf("supporting evidence lost passing sibling or true nested failure: %+v", result.Observations)
	}
}

func TestWithCaseSharesOracleAcrossDifferentPropertyAtoms(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(RecordDirEnv, dir)
	t.Setenv("HOTAM_RECORD_STDOUT", "")
	input := []byte{'a', 'b', 'c'}
	oracle := Object(map[string]TypedValue{
		"bytes":  Bytes(input),
		"length": Integer("3"),
	})
	context := CaseContext{
		ID: "case-shared-input", Operation: "inspect", Producer: "sdk",
		Expected: &oracle,
	}
	ft := &fakeRecordT{name: "TestWithCaseSharesOracleAcrossDifferentPropertyAtoms/one_input"}
	probe := &recorderTypedProbe{bytes: input}
	byteFact := Fact(ft, probe.RawBytes, Bytes(input), WithInput(input), WithCase(context))
	lengthFact := Fact(ft, probe.Length, Integer("3"), WithInput(input), WithCase(context))
	if !byteFact.Passed || !lengthFact.Passed || probe.rawCalls != 1 || probe.lengthCalls != 1 ||
		byteFact.CaseExpected == nil || lengthFact.CaseExpected == nil ||
		!reflect.DeepEqual(*byteFact.CaseExpected, oracle) ||
		!reflect.DeepEqual(*lengthFact.CaseExpected, oracle) {
		t.Fatalf("shared oracle changed local property results or repeated execution: bytes=%+v length=%+v calls=%d/%d",
			byteFact, lengthFact, probe.rawCalls, probe.lengthCalls)
	}
	byteObservation := byteFact.Observations[len(byteFact.Observations)-1]
	lengthObservation := lengthFact.Observations[len(lengthFact.Observations)-1]
	if byteObservation.RawExpected == nil || byteObservation.RawExpected.Kind != "bytes" ||
		lengthObservation.RawExpected == nil || lengthObservation.RawExpected.Kind != "integer" ||
		lengthObservation.RawExpected.Integer != "3" {
		t.Fatalf("per-property expectations were replaced by the shared oracle: bytes=%+v length=%+v",
			byteObservation, lengthObservation)
	}
	ft.runCleanups()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("artifacts = %v, err=%v; want one per distinct method atom", entries, err)
	}
	seenBytes, seenLength := false, false
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var artifact Artifact
		if err := json.Unmarshal(data, &artifact); err != nil {
			t.Fatal(err)
		}
		if artifact.Mode != "rule" || artifact.Test != ft.name || artifact.Case == nil ||
			artifact.Case.ID != context.ID || artifact.CaseExpected == nil ||
			!equalTypedValues(artifact.CaseExpected, &oracle) {
			t.Fatalf("shared case metadata/oracle differed between property artifacts: %+v", artifact)
		}
		var wire struct {
			Case     json.RawMessage `json:"case"`
			Expected *TypedValue     `json:"case_expected"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			t.Fatal(err)
		}
		var caseFields map[string]json.RawMessage
		if err := json.Unmarshal(wire.Case, &caseFields); err != nil {
			t.Fatal(err)
		}
		if _, hasCaseExpected := caseFields["expected"]; hasCaseExpected {
			t.Fatalf("runtime oracle leaked into the CaseContext JSON object: %s", wire.Case)
		}
		if strings.Contains(artifact.ReqID, "raw-bytes") {
			seenBytes = artifact.Steps[0].Observations[len(artifact.Steps[0].Observations)-1].RawExpected.Kind == "bytes"
		} else if strings.Contains(artifact.ReqID, "length") {
			seenLength = artifact.Steps[0].Observations[len(artifact.Steps[0].Observations)-1].RawExpected.Kind == "integer"
		}
	}
	if !seenBytes || !seenLength {
		t.Fatalf("artifacts did not retain both local expectations: bytes=%v length=%v", seenBytes, seenLength)
	}
}
