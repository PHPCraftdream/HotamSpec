// pipeline_test_gen.go renders pipeline_test.go's table-driven test half
// (see pipeline.go for the gate-function half both are combined into by
// GenerateGoCode/CLI — this file only renders the *testing.T body). Per
// GEN-CODE-CONTRACT.md §5's mutational-proof requirement for this stage: a
// gate function that stops comparing referenced.State against its terminal
// states (e.g. "always return nil") must turn the "not yet terminal" sub-test
// red — the sub-tests below exist to make exactly that regression visible.
package gocode

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// pipelinePath is one shortest sequence of already-generated transition
// methods (lifecycle.go) that walks an entityModel from its initial state to
// one of its terminal (or quiescent) states, resolved once per referenced
// entityModel so RenderPipelineTestFile never re-derives it per gate (a
// referenced EntityType may be the target of more than one gate, e.g.
// "forecast" is referenced by both brd-package.прогноз and
// sdr-package.прогноз - the walk is computed once and reused for both).
type pipelinePath struct {
	methods []string // transitionModel.methodName, in call order
	dst     stateModel
}

// shortestPathToTerminal performs a breadth-first search over m's declared
// transitions (lifecycle.go's own transition table - never a hand-authored
// path) from m's initial state to the nearest terminal (or quiescent) state,
// per contract §5's mutational requirement that the "reaches terminal"
// sub-test call REAL generated transition methods, not assert the terminal
// state directly. Deterministic: at each BFS layer, m.transitions is walked
// in its existing graph-declared order (the same order renderEntityTransitions
// emits the methods in), so two BuildPipelineGateModels runs on an unchanged
// graph produce byte-identical output (contract §5 determinism).
func shortestPathToTerminal(m *entityModel) (*pipelinePath, error) {
	return shortestPathToPredicate(m, func(s stateModel) bool { return s.src.IsTerminal() })
}

// shortestPathToState performs the same deterministic BFS as
// shortestPathToTerminal, but to one SPECIFIC named state rather than any
// terminal state - used by contract §2.1's precise-state gate test
// rendering, where the "gate accepts" sub-test must walk the referenced
// entity to EXACTLY the precise state the gate names (e.g. forecast's v2),
// never past it to a later terminal state (v3) that would defeat the whole
// point of the precise gate.
func shortestPathToState(m *entityModel, target stateModel) (*pipelinePath, error) {
	return shortestPathToPredicate(m, func(s stateModel) bool { return s.value == target.value })
}

// shortestPathToPredicate is the shared BFS walk shortestPathToTerminal and
// shortestPathToState both specialize (one source of truth for the walk
// itself, contract §0) - it returns the shortest sequence of already-
// generated transition methods from m's initial state to the nearest state
// satisfying isTarget.
func shortestPathToPredicate(m *entityModel, isTarget func(stateModel) bool) (*pipelinePath, error) {
	initial, err := m.initialState()
	if err != nil {
		return nil, err
	}
	if isTarget(initial) {
		return &pipelinePath{dst: initial}, nil
	}

	type queueItem struct {
		state   stateModel
		methods []string
	}
	visited := map[string]bool{initial.value: true}
	queue := []queueItem{{state: initial}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, tr := range m.transitions {
			if tr.srcState.value != cur.state.value {
				continue
			}
			if visited[tr.dstState.value] {
				continue
			}
			nextMethods := append(append([]string{}, cur.methods...), tr.methodName)
			if isTarget(tr.dstState) {
				return &pipelinePath{methods: nextMethods, dst: tr.dstState}, nil
			}
			visited[tr.dstState.value] = true
			queue = append(queue, queueItem{state: tr.dstState, methods: nextMethods})
		}
	}

	return nil, fmt.Errorf("gocode: EntityType %q has no declared transition path from its initial state (%s) that reaches the target state",
		m.src.Slug, initial.value)
}

// RenderPipelineTestFile renders pipeline_test.go's *testing.T body: one
// table-driven Test<Gate> function per pipelineGateModel, covering (a) the
// referenced entity fresh from New<Referenced>() (a non-terminal state by
// construction - New<Referenced>() always starts at the initial state, and
// StateKindInitial/StateKindTerminal/StateKindQuiescent are mutually
// exclusive kinds on one ontology.State, models.go's BuildEntityModel), which
// must make the gate return a non-nil error, and (b) the same entity walked
// via its own generated transition methods (shortestPathToTerminal) to a
// terminal/quiescent state, which must make the gate return nil.
func RenderPipelineTestFile(packageName string, gates []*pipelineGateModel) ([]byte, error) {
	var b strings.Builder
	b.WriteString(OwnershipMarker)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "package %s\n\n", packageName)

	if len(gates) == 0 {
		// No "import \"testing\"" here: with zero gates, zero Test<Gate>
		// functions are rendered below, so nothing would use the "testing"
		// package - an unconditional import would fail `go build`/`go vet`
		// with an unused-import error despite still parsing as syntactically
		// valid Go (contract §5's compile/test requirement, same reasoning
		// as RenderPipelineFile's matching fix).
		b.WriteString("// No pipeline gate functions were generated for this domain (see pipeline_test.go's\n")
		b.WriteString("// gate-function half) - nothing to table-test here.\n")
		return []byte(b.String()), nil
	}

	b.WriteString("import \"testing\"\n\n")

	// Resolve each distinct referenced entity's path-to-terminal exactly
	// once (contract §0 "one source" - a referenced entity targeted by
	// multiple gates gets one shared path, not N independently-recomputed
	// ones), keyed by struct name for determinism.
	//
	// A gate with a precise state (contract §2.1) additionally needs its OWN
	// path to that specific state - keyed by gate func name rather than
	// referenced struct name, since two gates on the SAME referenced entity
	// (brd-package.прогноз and sdr-package.прогноз, both -> forecast) can
	// name DIFFERENT precise states (v2 vs v3), so the precise path cannot be
	// shared the way the terminal path is.
	paths := make(map[string]*pipelinePath, len(gates))
	var referencedNames []string
	precisePaths := make(map[string]*pipelinePath, len(gates))
	for _, g := range gates {
		name := g.referenced.structName
		if _, ok := paths[name]; !ok {
			p, err := shortestPathToTerminal(g.referenced)
			if err != nil {
				return nil, err
			}
			paths[name] = p
			referencedNames = append(referencedNames, name)
		}
		if g.preciseState != nil {
			pp, err := shortestPathToState(g.referenced, *g.preciseState)
			if err != nil {
				return nil, err
			}
			precisePaths[g.funcName] = pp
		}
	}
	sort.Strings(referencedNames)

	for i, g := range gates {
		if i != 0 {
			b.WriteString("\n")
		}
		b.WriteString(renderPipelineGateTest(g, paths[g.referenced.structName], precisePaths[g.funcName]))
	}

	return []byte(b.String()), nil
}

// renderPipelineGateTest renders one Test<Gate> function. For a general
// RequiresTerminal gate (precisePath == nil): a not-yet-terminal sub-test
// (fresh New<Referenced>(), gate must error) and a reaches-terminal sub-test
// (New<Referenced>() walked via path.methods, gate must return nil). For a
// contract §2.1 precise-state gate (precisePath != nil), a THIRD sub-test is
// added: reaching the referenced entity's full terminal state (path, which
// may walk PAST the precise state to a later one) must still make the gate
// return an error - the exact behavior distinguishing a precise gate from
// the general one (a later terminal state must never silently satisfy a gate
// that names an earlier, specific state). All sub-tests call the REAL
// generated gate function and the REAL generated transition methods - no
// re-implementation of either (contract §0 mirror principle).
func renderPipelineGateTest(g *pipelineGateModel, path *pipelinePath, precisePath *pipelinePath) string {
	var b strings.Builder
	funcName := "Test" + g.funcName
	fmt.Fprintf(&b, "// Atom: %s.%s -> %s (kind:reference, ref_target=%s) - see requirements_audit.md\n",
		g.referencer.entitySlug, g.field.fieldName, g.referenced.entitySlug, g.referenced.entitySlug)
	fmt.Fprintf(&b, "func %s(t *testing.T) {\n", funcName)

	if g.preciseState != nil {
		writePreciseGateSubTests(&b, g, precisePath, path)
	} else {
		writeGeneralGateSubTests(&b, g, path)
	}

	b.WriteString("}\n")
	return b.String()
}

// writeGeneralGateSubTests renders the pre-§2.1 "not yet terminal" /
// "reaches terminal" sub-test pair for a general RequiresTerminal gate.
func writeGeneralGateSubTests(b *strings.Builder, g *pipelineGateModel, path *pipelinePath) {
	// --- not yet terminal: fresh referenced entity, gate must reject ---
	b.WriteString("\tt.Run(\"not yet terminal\", func(t *testing.T) {\n")
	fmt.Fprintf(b, "\t\treferenced := New%s()\n", g.referenced.structName)
	fmt.Fprintf(b, "\t\tif err := %s(referenced); err == nil {\n", g.funcName)
	fmt.Fprintf(b, "\t\t\tt.Fatalf(%s, referenced.State)\n",
		strconv.Quote(fmt.Sprintf("%s: expected an error while %s is not terminal, got nil (state %%q)", g.funcName, g.referenced.structName)))
	b.WriteString("\t\t}\n")
	b.WriteString("\t})\n\n")

	// --- reaches terminal: walk the real transition methods, gate must
	// accept ---
	b.WriteString("\tt.Run(\"reaches terminal\", func(t *testing.T) {\n")
	fmt.Fprintf(b, "\t\treferenced := New%s()\n", g.referenced.structName)
	writeTransitionWalk(b, g.referenced.structName, path.methods)
	fmt.Fprintf(b, "\t\tif err := %s(referenced); err != nil {\n", g.funcName)
	fmt.Fprintf(b, "\t\t\tt.Fatalf(%s, err)\n",
		strconv.Quote(fmt.Sprintf("%s: expected nil once %s is terminal, got %%v", g.funcName, g.referenced.structName)))
	b.WriteString("\t\t}\n")
	b.WriteString("\t})\n")
}

// writePreciseGateSubTests renders the contract §2.1 three-sub-test set for
// a precise-state gate: not-yet-reached (fresh entity, must error),
// reaches-exact-state (walked to preciseState via precisePath, must accept),
// and reaches-later-terminal (walked all the way to terminalPath.dst, must
// STILL error unless that terminal state happens to equal preciseState -
// the mutational distinction between a precise gate and the general one).
func writePreciseGateSubTests(b *strings.Builder, g *pipelineGateModel, precisePath, terminalPath *pipelinePath) {
	stateName := g.preciseState.src.Name

	// --- not yet at the precise state: fresh referenced entity, gate must
	// reject ---
	b.WriteString("\tt.Run(\"not yet at required state\", func(t *testing.T) {\n")
	fmt.Fprintf(b, "\t\treferenced := New%s()\n", g.referenced.structName)
	fmt.Fprintf(b, "\t\tif err := %s(referenced); err == nil {\n", g.funcName)
	fmt.Fprintf(b, "\t\t\tt.Fatalf(%s, referenced.State)\n",
		strconv.Quote(fmt.Sprintf("%s: expected an error while %s is not in state %s, got nil (state %%q)", g.funcName, g.referenced.structName, stateName)))
	b.WriteString("\t\t}\n")
	b.WriteString("\t})\n\n")

	// --- reaches exactly the precise state: gate must accept ---
	b.WriteString("\tt.Run(\"reaches required state\", func(t *testing.T) {\n")
	fmt.Fprintf(b, "\t\treferenced := New%s()\n", g.referenced.structName)
	writeTransitionWalk(b, g.referenced.structName, precisePath.methods)
	fmt.Fprintf(b, "\t\tif err := %s(referenced); err != nil {\n", g.funcName)
	fmt.Fprintf(b, "\t\t\tt.Fatalf(%s, err)\n",
		strconv.Quote(fmt.Sprintf("%s: expected nil once %s reaches state %s, got %%v", g.funcName, g.referenced.structName, stateName)))
	b.WriteString("\t\t}\n")
	b.WriteString("\t})\n")

	// --- reaches a LATER terminal state (past the precise one): gate must
	// still reject, unless the terminal path happens to coincide with the
	// precise state itself (nothing to distinguish in that degenerate case).
	if terminalPath.dst.value != g.preciseState.value {
		b.WriteString("\n")
		b.WriteString("\tt.Run(\"overshoots to a later terminal state\", func(t *testing.T) {\n")
		fmt.Fprintf(b, "\t\treferenced := New%s()\n", g.referenced.structName)
		writeTransitionWalk(b, g.referenced.structName, terminalPath.methods)
		fmt.Fprintf(b, "\t\tif err := %s(referenced); err == nil {\n", g.funcName)
		fmt.Fprintf(b, "\t\t\tt.Fatalf(%s, referenced.State)\n",
			strconv.Quote(fmt.Sprintf("%s: expected an error once %s overshoots state %s to a later terminal state, got nil (state %%q)", g.funcName, g.referenced.structName, stateName)))
		b.WriteString("\t\t}\n")
		b.WriteString("\t})\n")
	}
}

// writeTransitionWalk renders one call per transition method name in
// methods, each checked for a nil error, against the local `referenced`
// variable both gate-test-rendering functions above declare. Shared so the
// general and precise sub-test renderers never diverge on how a transition
// walk is checked (contract §0).
func writeTransitionWalk(b *strings.Builder, referencedStructName string, methods []string) {
	writeTransitionWalkVar(b, "referenced", referencedStructName, methods)
}

// writeTransitionWalkVar is writeTransitionWalk generalized to an arbitrary
// local variable name: process_gates_test_gen.go's composite gate tests
// declare one local per required entity (named after that entity's own
// camelCase slug, e.g. "frRegistry", "forecast" — never the fixed
// "referenced" pipeline_test_gen.go's single-entity gate tests use), so the
// variable name driving each generated `<varName>.<Method>()` call and its
// `t.Fatalf` message must be a parameter, not a hardcoded literal. Both
// callers render IDENTICAL per-call shape (contract §0: one source of truth
// for "how is a transition walk checked", not two independently-maintained
// copies) — writeTransitionWalk is now a thin varName="referenced" wrapper
// over this function, not a second implementation.
func writeTransitionWalkVar(b *strings.Builder, varName, structName string, methods []string) {
	for _, methodName := range methods {
		fmt.Fprintf(b, "\t\tif err := %s.%s(); err != nil {\n", varName, methodName)
		fmt.Fprintf(b, "\t\t\tt.Fatalf(%s, err)\n",
			strconv.Quote(fmt.Sprintf("%s: %s(): %%v", structName, methodName)))
		b.WriteString("\t\t}\n")
	}
}
