package hotamspec

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	want := []Fact{{"z", "1"}, {"a", "2"}, {"m", "3"}}
	if len(facts) != len(want) {
		t.Fatalf("facts len = %d, want %d", len(facts), len(want))
	}
	for i, f := range facts {
		if f != want[i] {
			t.Errorf("facts[%d] = %+v, want %+v (order must be call order, not sorted)", i, f, want[i])
		}
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
