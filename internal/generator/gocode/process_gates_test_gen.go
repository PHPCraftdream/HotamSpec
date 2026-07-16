// process_gates_test_gen.go renders the *testing.T half of stage 6's
// composite process-step gates (see process_gates.go for the gate-function
// half both are combined into by GenerateAllFromGraph/the CLI). Mirrors
// pipeline_test_gen.go's own mutational-proof discipline (GEN-CODE-CONTRACT.md
// §5): a composite gate that stops checking one of its N required entities
// (e.g. "always accept the risk-registry parameter regardless of state") must
// turn a sub-test red - each required entity gets its own "held back" sub-test
// proving the gate rejects when JUST THAT ONE entity is not yet ready, even
// with every other required entity already fully satisfied. That is the
// mutation this composite gate exists to catch: a single-condition regression
// hiding behind N-1 other conditions that still pass.
package gocode

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// RenderProcessGatesTestFile renders the *testing.T body for every
// processStepGateModel: one Test<Gate> function per composite gate, with one
// "<entity> not ready" sub-test per required entity (gate must reject when
// that one entity is fresh/not-yet-at-its-required-state, even with every
// OTHER required entity already walked to ITS required state) plus one "all
// ready" sub-test (gate must accept once every required entity has reached
// its own required state).
func RenderProcessGatesTestFile(packageName string, gates []*processStepGateModel) ([]byte, error) {
	var b strings.Builder
	b.WriteString(OwnershipMarker)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "package %s\n\n", packageName)

	if len(gates) == 0 {
		b.WriteString("// No composite process-step gate functions were generated for this domain\n")
		b.WriteString("// (see the gate-function half) - nothing to table-test here.\n")
		return []byte(b.String()), nil
	}

	b.WriteString("import \"testing\"\n\n")

	for i, g := range gates {
		if i != 0 {
			b.WriteString("\n")
		}
		src, err := renderProcessStepGateTest(g)
		if err != nil {
			return nil, err
		}
		b.WriteString(src)
	}

	return []byte(b.String()), nil
}

// processGateEntityPath resolves the "ready" path for one processGateEntity:
// the shortest generated-transition-method walk to its precise required
// state (if set) or to any of its own terminal states otherwise - reusing
// shortestPathToState/shortestPathToTerminal (pipeline_test_gen.go) exactly,
// never a second BFS implementation.
func processGateEntityPath(e processGateEntity) (*pipelinePath, error) {
	if e.preciseState != nil {
		return shortestPathToState(e.entity, *e.preciseState)
	}
	return shortestPathToTerminal(e.entity)
}

// entitySatisfiesRequirement reports whether stateVal (a state's own kebab
// value) already satisfies processGateEntity e's own gate requirement -
// either "equals the precise required state" or "is one of the entity's own
// terminal/quiescent states", matching exactly what the rendered gate
// function itself checks (process_gates.go's renderProcessStepGateFunc).
func entitySatisfiesRequirement(e processGateEntity, stateVal string) bool {
	if e.preciseState != nil {
		return stateVal == e.preciseState.value
	}
	for _, s := range e.terminal {
		if s.value == stateVal {
			return true
		}
	}
	return false
}

// processGateEntityNotReadyPath resolves a walk to a state of e's own entity
// that does NOT satisfy e's gate requirement — needed for the "not ready"
// mutational sub-test (renderProcessStepGateTest). A fresh New<Entity>() is
// NOT always "not ready": on the real prat domain, forecast's own initial
// state IS v1 (the exact state planning-approved's gate requires) and
// brd-package's own initial state IS scope (the exact state brd-scope's gate
// requires) - a "not ready" sub-test that merely constructs a fresh entity
// would be VACUOUSLY green in that case (the gate never gets exercised
// against a genuinely failing input), not a real mutational proof. This
// function performs the SAME shortestPathToPredicate BFS
// shortestPathToTerminal/shortestPathToState already use (pipeline_test_gen.go
// — no second implementation), searching instead for the nearest state that
// does NOT satisfy e's own requirement. Returns (nil, false) when the entity's
// OWN fresh initial state already fails to satisfy the requirement (the
// common case — no walk needed, construct-and-check is already a genuine
// negative test) or (path, true) when a walk is needed because the initial
// state trivially satisfies the requirement.
func processGateEntityNotReadyPath(e processGateEntity) (*pipelinePath, bool, error) {
	initial, err := e.entity.initialState()
	if err != nil {
		return nil, false, err
	}
	if !entitySatisfiesRequirement(e, initial.value) {
		return nil, false, nil
	}
	p, err := shortestPathToPredicate(e.entity, func(s stateModel) bool {
		return !entitySatisfiesRequirement(e, s.value)
	})
	if err != nil {
		return nil, false, fmt.Errorf("gocode: process-step gate entity %s: fresh initial state %q already satisfies its own gate requirement, and no OTHER reachable state fails it either — cannot render a genuine \"not ready\" mutational sub-test: %w", e.entity.src.Slug, initial.value, err)
	}
	return p, true, nil
}

// renderProcessStepGateTest renders one Test<Gate> function: for each
// required entity index i, a "<entity> not ready" sub-test constructs every
// OTHER required entity already walked to its own ready state, but leaves
// entity i fresh (New<Entity>()) - the composite gate must still reject.
// A final "all entities ready" sub-test walks every required entity to its
// ready state and asserts the gate accepts.
func renderProcessStepGateTest(g *processStepGateModel) (string, error) {
	paths := make([]*pipelinePath, len(g.entities))
	for i, e := range g.entities {
		p, err := processGateEntityPath(e)
		if err != nil {
			return "", err
		}
		paths[i] = p
	}

	var b strings.Builder
	funcName := "Test" + g.funcName
	fmt.Fprintf(&b, "// Atom: process %s step %s -> %s (composite, GEN-CODE-CONTRACT.md stage 6)", g.process.ID, g.step.Name, g.funcName)
	if g.requirementID != "" {
		fmt.Fprintf(&b, " - see requirements_audit.md#%s", strings.ToLower(g.requirementID))
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "func %s(t *testing.T) {\n", funcName)

	params := make([]string, len(g.entities))
	for i, e := range g.entities {
		pname, err := e.paramName()
		if err != nil {
			return "", err
		}
		params[i] = pname
	}

	for i, e := range g.entities {
		pname := params[i]
		notReadyPath, needsWalk, err := processGateEntityNotReadyPath(e)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "\tt.Run(%s, func(t *testing.T) {\n", strconv.Quote(pname+" not ready"))
		var args []string
		for j, e2 := range g.entities {
			p2 := params[j]
			fmt.Fprintf(&b, "\t\t%s := New%s()\n", p2, e2.entity.structName)
			switch {
			case j == i && needsWalk:
				// This entity's own fresh initial state already satisfies its
				// gate requirement (e.g. forecast's initial state IS v1, the
				// exact state planning-approved requires) - walk it to a
				// DIFFERENT, genuinely-non-satisfying state instead, so this
				// sub-test is a real mutational proof, not a vacuous pass.
				writeTransitionWalkVar(&b, p2, e2.entity.structName, notReadyPath.methods)
			case j != i:
				writeTransitionWalkVar(&b, p2, e2.entity.structName, paths[j].methods)
			}
			args = append(args, p2)
		}
		fmt.Fprintf(&b, "\t\tif err := %s(%s); err == nil {\n", g.funcName, strings.Join(args, ", "))
		fmt.Fprintf(&b, "\t\t\tt.Fatalf(%s, %s.State)\n",
			strconv.Quote(fmt.Sprintf("%s: expected an error while %s is not ready, got nil (state %%q)", g.funcName, pname)), pname)
		b.WriteString("\t\t}\n")
		b.WriteString("\t})\n\n")
	}

	b.WriteString("\tt.Run(\"all entities ready\", func(t *testing.T) {\n")
	var args []string
	for j, e := range g.entities {
		p := params[j]
		fmt.Fprintf(&b, "\t\t%s := New%s()\n", p, e.entity.structName)
		writeTransitionWalkVar(&b, p, e.entity.structName, paths[j].methods)
		args = append(args, p)
	}
	fmt.Fprintf(&b, "\t\tif err := %s(%s); err != nil {\n", g.funcName, strings.Join(args, ", "))
	fmt.Fprintf(&b, "\t\t\tt.Fatalf(%s, err)\n",
		strconv.Quote(fmt.Sprintf("%s: expected nil once every required entity is ready, got %%v", g.funcName)))
	b.WriteString("\t\t}\n")
	b.WriteString("\t})\n")

	b.WriteString("}\n")
	return b.String(), nil
}

// sortedFuncNames is a small debug/test helper mirroring pipeline.go's own
// gateFuncNames-style helper, used by this package's tests to assert
// deterministic ordering without re-deriving the sort elsewhere.
func sortedProcessGateFuncNames(gates []*processStepGateModel) []string {
	out := make([]string, len(gates))
	for i, g := range gates {
		out[i] = g.funcName
	}
	sort.Strings(out)
	return out
}
