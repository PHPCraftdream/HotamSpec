package hotamspec

import (
	"errors"
	"fmt"
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
