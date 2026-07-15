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
	initial, err := m.initialState()
	if err != nil {
		return nil, err
	}
	if initial.src.IsTerminal() {
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
			if tr.dstState.src.IsTerminal() {
				return &pipelinePath{methods: nextMethods, dst: tr.dstState}, nil
			}
			visited[tr.dstState.value] = true
			queue = append(queue, queueItem{state: tr.dstState, methods: nextMethods})
		}
	}

	return nil, fmt.Errorf("gocode: EntityType %q has a terminal/quiescent lifecycle state but no declared transition path from its initial state (%s) reaches one",
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
	paths := make(map[string]*pipelinePath, len(gates))
	var referencedNames []string
	for _, g := range gates {
		name := g.referenced.structName
		if _, ok := paths[name]; ok {
			continue
		}
		p, err := shortestPathToTerminal(g.referenced)
		if err != nil {
			return nil, err
		}
		paths[name] = p
		referencedNames = append(referencedNames, name)
	}
	sort.Strings(referencedNames)

	for i, g := range gates {
		if i != 0 {
			b.WriteString("\n")
		}
		b.WriteString(renderPipelineGateTest(g, paths[g.referenced.structName]))
	}

	return []byte(b.String()), nil
}

// renderPipelineGateTest renders one Test<Gate> function: a not-yet-terminal
// sub-test (fresh New<Referenced>(), gate must error) and a reaches-terminal
// sub-test (New<Referenced>() walked via path.methods, gate must return
// nil). Both sub-tests call the REAL generated gate function and the REAL
// generated transition methods - no re-implementation of either (contract
// §0 mirror principle).
func renderPipelineGateTest(g *pipelineGateModel, path *pipelinePath) string {
	var b strings.Builder
	funcName := "Test" + g.funcName
	fmt.Fprintf(&b, "// Atom: %s.%s -> %s (kind:reference, ref_target=%s) - see requirements_audit.md\n",
		g.referencer.entitySlug, g.field.fieldName, g.referenced.entitySlug, g.referenced.entitySlug)
	fmt.Fprintf(&b, "func %s(t *testing.T) {\n", funcName)

	// --- not yet terminal: fresh referenced entity, gate must reject ---
	b.WriteString("\tt.Run(\"not yet terminal\", func(t *testing.T) {\n")
	fmt.Fprintf(&b, "\t\treferenced := New%s()\n", g.referenced.structName)
	fmt.Fprintf(&b, "\t\tif err := %s(referenced); err == nil {\n", g.funcName)
	fmt.Fprintf(&b, "\t\t\tt.Fatalf(%s, referenced.State)\n",
		strconv.Quote(fmt.Sprintf("%s: expected an error while %s is not terminal, got nil (state %%q)", g.funcName, g.referenced.structName)))
	b.WriteString("\t\t}\n")
	b.WriteString("\t})\n\n")

	// --- reaches terminal: walk the real transition methods, gate must
	// accept ---
	b.WriteString("\tt.Run(\"reaches terminal\", func(t *testing.T) {\n")
	fmt.Fprintf(&b, "\t\treferenced := New%s()\n", g.referenced.structName)
	for _, methodName := range path.methods {
		fmt.Fprintf(&b, "\t\tif err := referenced.%s(); err != nil {\n", methodName)
		fmt.Fprintf(&b, "\t\t\tt.Fatalf(%s, err)\n",
			strconv.Quote(fmt.Sprintf("%s: %s(): %%v", g.referenced.structName, methodName)))
		b.WriteString("\t\t}\n")
	}
	fmt.Fprintf(&b, "\t\tif err := %s(referenced); err != nil {\n", g.funcName)
	fmt.Fprintf(&b, "\t\t\tt.Fatalf(%s, err)\n",
		strconv.Quote(fmt.Sprintf("%s: expected nil once %s is terminal, got %%v", g.funcName, g.referenced.structName)))
	b.WriteString("\t\t}\n")
	b.WriteString("\t})\n")

	b.WriteString("}\n")
	return b.String()
}
