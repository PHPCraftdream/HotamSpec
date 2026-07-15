// lifecycle_test_gen.go renders lifecycle_test.go: table-driven Go tests
// that exercise every generated transition method from lifecycle.go. Per
// GEN-CODE-CONTRACT.md §5 ("мутационная проверка"), these tests must be
// able to fail — a rendered test suite that only asserts the happy path
// cannot catch a mirror that silently drifted from the graph, so this file
// also asserts the negative space: calling an event from a state it is not
// declared from must return a *WrongStateError and must not mutate State.
package gocode

import (
	"fmt"
	"sort"
	"strings"
)

// legalCase is one declared (src, event) -> dst transition, rendered as a
// "should succeed" table row.
type legalCase struct {
	entity     *entityModel
	transition transitionModel
}

// illegalCase is one (state, event) pair that is NOT declared as a
// transition on the entity — rendered as a "should fail, state unchanged"
// table row. event is the already-kebab-cased event value (contract §1.1):
// the only form allowed into a generated string literal/test name.
type illegalCase struct {
	entity     *entityModel
	state      stateModel
	methodName string
	event      string
}

// buildIllegalCases computes the negative-space test cases for one entity
// model: for every declared event, at least one state it is not declared
// from (if any such state exists); plus, for every terminal-kind state, one
// illegal case per event NOT already declared from that state (so "any
// event from a terminal state fails" is asserted explicitly per state, not
// just implied by the per-event minimum). Determinism (contract §5): both
// loops iterate m.transitions/m.states in their existing graph-declared
// order, and results are deduplicated by (state, event) preserving first
// occurrence.
func buildIllegalCases(m *entityModel) []illegalCase {
	declared := make(map[string]map[string]bool, len(m.states)) // state.value -> event -> declared
	for _, s := range m.states {
		declared[s.value] = map[string]bool{}
	}
	for _, tr := range m.transitions {
		declared[tr.srcState.value][tr.src.Event] = true
	}

	seen := make(map[[2]string]bool)
	var out []illegalCase

	add := func(state stateModel, tr transitionModel) {
		key := [2]string{state.value, tr.src.Event}
		if seen[key] {
			return
		}
		seen[key] = true
		// Reuses tr.eventValue, already computed once in BuildEntityModel —
		// no independent re-derivation of the same translated event text
		// (contract §1.1/§4.3: one ToKebabCase call per transition event,
		// its result threaded through, not recomputed).
		out = append(out, illegalCase{entity: m, state: state, methodName: tr.methodName, event: tr.eventValue})
	}

	// Minimum coverage: for every declared event, one src state it is NOT
	// declared from (if such a state exists at all).
	for _, tr := range m.transitions {
		for _, s := range m.states {
			if declared[s.value][tr.src.Event] {
				continue
			}
			add(s, tr)
			break
		}
	}

	// Terminal states: every event NOT already declared from that terminal
	// state gets an explicit illegal case, so "any (non-declared) event from
	// terminal fails" is asserted per terminal state, not just per event.
	for _, s := range m.states {
		if !s.src.IsTerminal() {
			continue
		}
		for _, tr := range m.transitions {
			if declared[s.value][tr.src.Event] {
				continue
			}
			add(s, tr)
		}
	}

	return out
}

// RenderLifecycleTestFile renders the full lifecycle_test.go body for a
// domain: the ownership marker, package clause, and one table-driven test
// function per EntityType that has lifecycle transitions, covering both the
// declared transitions (legal cases) and the negative space (illegal
// cases). EntityTypes with zero transitions are skipped, matching
// RenderLifecycleFile.
func RenderLifecycleTestFile(packageName string, models []*entityModel) ([]byte, error) {
	sorted := make([]*entityModel, len(models))
	copy(sorted, models)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].src.Slug < sorted[j].src.Slug })

	var b strings.Builder
	b.WriteString(OwnershipMarker)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "package %s\n\n", packageName)
	b.WriteString("import \"testing\"\n\n")

	any := false
	for _, m := range sorted {
		if len(m.transitions) == 0 {
			continue
		}
		if any {
			b.WriteString("\n")
		}
		any = true
		b.WriteString(renderEntityTransitionTests(m))
	}

	return []byte(b.String()), nil
}

func renderEntityTransitionTests(m *entityModel) string {
	var b strings.Builder
	funcName := "Test" + m.structName + "_LifecycleTransitions"
	fmt.Fprintf(&b, "func %s(t *testing.T) {\n", funcName)

	// --- legal cases: declared transition succeeds and lands on dst ---
	b.WriteString("\tt.Run(\"legal transitions\", func(t *testing.T) {\n")
	b.WriteString("\t\tcases := []struct {\n")
	b.WriteString("\t\t\tname  string\n")
	fmt.Fprintf(&b, "\t\t\tfrom  %s\n", m.stateType)
	fmt.Fprintf(&b, "\t\t\tcall  func(*%s) error\n", m.structName)
	fmt.Fprintf(&b, "\t\t\twant  %s\n", m.stateType)
	b.WriteString("\t\t}{\n")
	for _, tr := range m.transitions {
		// name: references the same eventConst lifecycle.go's WrongStateError
		// construction uses (not a re-literal-ized copy of the kebab string)
		// — one named Go constant is the sole source of truth for this
		// transition's translated event text across both generated files.
		fmt.Fprintf(&b, "\t\t\t{name: %s, from: %s, call: (*%s).%s, want: %s},\n",
			tr.eventConst, tr.srcState.constant, m.structName, tr.methodName, tr.dstState.constant)
	}
	b.WriteString("\t\t}\n")
	b.WriteString("\t\tfor _, tc := range cases {\n")
	b.WriteString("\t\t\tt.Run(tc.name, func(t *testing.T) {\n")
	fmt.Fprintf(&b, "\t\t\t\tx := &%s{State: tc.from}\n", m.structName)
	b.WriteString("\t\t\t\tif err := tc.call(x); err != nil {\n")
	b.WriteString("\t\t\t\t\tt.Fatalf(\"%s: unexpected error: %v\", tc.name, err)\n")
	b.WriteString("\t\t\t\t}\n")
	b.WriteString("\t\t\t\tif x.State != tc.want {\n")
	b.WriteString("\t\t\t\t\tt.Fatalf(\"%s: state = %v, want %v\", tc.name, x.State, tc.want)\n")
	b.WriteString("\t\t\t\t}\n")
	b.WriteString("\t\t\t})\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t})\n\n")

	// --- illegal cases: undeclared (state, event) pair fails, state
	// unchanged ---
	illegal := buildIllegalCases(m)
	b.WriteString("\tt.Run(\"illegal transitions\", func(t *testing.T) {\n")
	b.WriteString("\t\tcases := []struct {\n")
	b.WriteString("\t\t\tname string\n")
	fmt.Fprintf(&b, "\t\t\tfrom %s\n", m.stateType)
	fmt.Fprintf(&b, "\t\t\tcall func(*%s) error\n", m.structName)
	b.WriteString("\t\t}{\n")
	for _, ic := range illegal {
		caseName := fmt.Sprintf("%s from %s", ic.event, ic.state.value)
		fmt.Fprintf(&b, "\t\t\t{name: %q, from: %s, call: (*%s).%s},\n",
			caseName, ic.state.constant, m.structName, ic.methodName)
	}
	b.WriteString("\t\t}\n")
	b.WriteString("\t\tfor _, tc := range cases {\n")
	b.WriteString("\t\t\tt.Run(tc.name, func(t *testing.T) {\n")
	fmt.Fprintf(&b, "\t\t\t\tx := &%s{State: tc.from}\n", m.structName)
	b.WriteString("\t\t\t\terr := tc.call(x)\n")
	b.WriteString("\t\t\t\tif err == nil {\n")
	b.WriteString("\t\t\t\t\tt.Fatalf(\"%s: expected error, got nil\", tc.name)\n")
	b.WriteString("\t\t\t\t}\n")
	b.WriteString("\t\t\t\tif x.State != tc.from {\n")
	b.WriteString("\t\t\t\t\tt.Fatalf(\"%s: state mutated on illegal transition: got %v, want unchanged %v\", tc.name, x.State, tc.from)\n")
	b.WriteString("\t\t\t\t}\n")
	b.WriteString("\t\t\t})\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t})\n")

	b.WriteString("}\n")
	return b.String()
}
