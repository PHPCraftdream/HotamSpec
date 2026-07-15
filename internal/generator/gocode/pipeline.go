// pipeline.go renders pipeline_test.go's non-test half: the pipeline GATE
// functions themselves (GEN-CODE-CONTRACT.md §2's kind:reference row,
// "future inter-entity resolution" — this is that resolution, stage 5). A
// gate function exists for every EntityType.fields[] entry whose kind is
// "reference" AND whose ref_target resolves to another EntityType's slug in
// the SAME domain (a real structural pipeline edge, e.g.
// fr-graph.входной_реестр -> fr-registry) — never for a ref_target that does
// not resolve in this domain's graph (e.g. sdr-package.feature_lead ->
// "Stakeholder", a cross-graph/role reference with no EntityType of that
// slug here), which stays the existing §2/§6 TODO-comment, not a gate.
//
// See docs/GEN-CODE-CONTRACT.md §0 (mirror principle): "artifact not
// accepted until predecessor terminal" is exactly the pipeline shape this
// package's other renderers already give one entity at a time (entities.go's
// Validate(), lifecycle.go's transition-state checks) — a gate function is
// the same mirror discipline applied ACROSS two EntityTypes joined by a real
// typed reference field, using terminal-ness (stateModel already resolved
// via ontology.State.IsTerminal(), which covers both StateKindTerminal and
// StateKindQuiescent — the SAME definition lifecycle_test_gen.go's
// buildIllegalCases already uses for "terminal", so a gate's notion of
// "done" can never silently disagree with the lifecycle layer's own).
package gocode

import (
	"fmt"
	"strings"
)

// pipelineGateModel is the resolved, Go-identifier-shaped view of one
// pipeline gate: a referencer EntityType's kind:reference field whose
// ref_target resolves to another EntityType of the SAME domain. Built once
// (BuildPipelineGateModels) so the gate function renderer (RenderPipelineFile)
// and its test renderer (RenderPipelineTestFile) never re-derive the
// referencer/referenced identifiers BuildEntityModel already computed — the
// same single-source discipline every other *Model type in this package
// follows (contract §0).
type pipelineGateModel struct {
	referencer     *entityModel
	field          fieldModel
	referenced     *entityModel
	funcName       string       // Gate<Referencer><Field>Requires<Referenced>Terminal
	terminalStates []stateModel // referenced.states with IsTerminal() == true, graph order
}

// BuildPipelineGateModels resolves every real inter-EntityType pipeline gate
// in the domain: for each entityModel's kind:reference field whose
// ref_target matches another entityModel's original graph slug (case-
// sensitive, the same slug BuildEntityModel already read off
// ontology.EntityType.Slug), one pipelineGateModel is produced. Fields whose
// ref_target does not resolve to any entityModel's slug in models are
// skipped entirely — that is the existing §2/§6 "honest TODO, not an error"
// case, unchanged by this stage. Order: referencer entities in the same
// sorted (by slug) order models is passed in, and within one entity, fields
// in that entity's own graph-declared field order (fieldModel already
// preserves this, models.go) — both already-deterministic orders, so no
// additional sort is needed beyond what BuildEntityModel/models already
// guarantee (contract §5).
func BuildPipelineGateModels(models []*entityModel) ([]*pipelineGateModel, error) {
	bySlug := make(map[string]*entityModel, len(models))
	for _, em := range models {
		bySlug[em.src.Slug] = em
	}

	var out []*pipelineGateModel
	for _, referencer := range models {
		for _, f := range referencer.fields {
			if f.src.Kind != "reference" || f.src.RefTarget == "" {
				continue
			}
			referenced, ok := bySlug[f.src.RefTarget]
			if !ok {
				// ref_target does not resolve to an EntityType of this domain
				// (e.g. "Stakeholder") - contract §2/§6's existing honest TODO
				// comment already covers this in entities.go; no gate here.
				continue
			}

			var terminalStates []stateModel
			for _, s := range referenced.states {
				if s.src.IsTerminal() {
					terminalStates = append(terminalStates, s)
				}
			}
			if len(terminalStates) == 0 {
				return nil, fmt.Errorf("gocode: pipeline gate %s.%s -> %s: referenced EntityType %q has no terminal (or quiescent) lifecycle state to gate on",
					referencer.structName, f.fieldName, referenced.structName, referenced.src.Slug)
			}

			funcName := fmt.Sprintf("Gate%s%sRequires%sTerminal", referencer.structName, f.fieldName, referenced.structName)
			out = append(out, &pipelineGateModel{
				referencer:     referencer,
				field:          f,
				referenced:     referenced,
				funcName:       funcName,
				terminalStates: terminalStates,
			})
		}
	}

	byFuncName := make(map[string]struct{}, len(out))
	for _, g := range out {
		if _, dup := byFuncName[g.funcName]; dup {
			return nil, fmt.Errorf("gocode: two pipeline gates both resolve to Go function %q", g.funcName)
		}
		byFuncName[g.funcName] = struct{}{}
	}

	return out, nil
}

// RenderPipelineFile renders the full pipeline_test.go GATE-FUNCTION half
// (see RenderPipelineTestBody for the table-driven test half — both halves
// live in the single pipeline_test.go file per contract §1's layout, so the
// gate functions are directly next to the tests exercising them, matching
// how lifecycle.go's methods and lifecycle_test.go's cases are two separate
// FILES but this stage's contract §1 lists only one pipeline_test.go path).
// gates is expected pre-sorted deterministically by BuildPipelineGateModels'
// own construction order (referencer slug order, then field order) - this
// renderer does not re-sort, so callers must pass BuildPipelineGateModels'
// direct output.
func RenderPipelineFile(packageName string, gates []*pipelineGateModel) ([]byte, error) {
	var b strings.Builder
	b.WriteString(OwnershipMarker)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "package %s\n\n", packageName)

	if len(gates) == 0 {
		// No "import \"fmt\"" here: every gate function's body is the only
		// user of "fmt" (fmt.Errorf), so with zero gates there is nothing to
		// import - an unconditional import would leave this half of
		// pipeline_test.go with an unused import, which fails `go
		// build`/`go vet` even though it still PARSES as syntactically valid
		// Go (contract §5's "Компилируемость и проходимость" requires more
		// than parsing).
		b.WriteString("// No kind:reference field in this domain resolves its ref_target to another\n")
		b.WriteString("// EntityType of this same domain, so no pipeline gate functions are generated.\n")
		return []byte(b.String()), nil
	}

	b.WriteString("import \"fmt\"\n\n")
	for i, g := range gates {
		if i != 0 {
			b.WriteString("\n")
		}
		b.WriteString(renderPipelineGateFunc(g))
	}

	return []byte(b.String()), nil
}

// renderPipelineGateFunc renders one gate predicate: it accepts the
// REFERENCED entity (the pipeline predecessor) and returns an error unless
// that entity's State is one of its own declared terminal (or quiescent)
// lifecycle states — GEN-CODE-CONTRACT.md §0's "artifact not accepted until
// predecessor terminal" made literal. The referencer/field are only used to
// compose the function name and the error message (contract §0 mirror: the
// message should let a human/LLM auditor see which graph edge this gate
// enforces without re-deriving anything), never as a parameter — the gate is
// a pure predicate over the REFERENCED entity's own state, callable
// regardless of which referencer instance is asking.
func renderPipelineGateFunc(g *pipelineGateModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// %s gates GEN-CODE-CONTRACT.md section 0's pipeline mirror: %s.%s\n",
		g.funcName, g.referencer.structName, g.field.fieldName)
	fmt.Fprintf(&b, "// references %s (kind:reference, ref_target=%q) - the referenced %s\n",
		g.referenced.structName, g.referenced.src.Slug, g.referenced.structName)
	b.WriteString("// artifact must not be accepted by the next pipeline stage until it is in one\n")
	b.WriteString("// of its own terminal (or quiescent) lifecycle states.\n")
	fmt.Fprintf(&b, "func %s(referenced *%s) error {\n", g.funcName, g.referenced.structName)
	fmt.Fprintf(&b, "\tswitch referenced.State {\n")
	names := make([]string, len(g.terminalStates))
	for i, s := range g.terminalStates {
		names[i] = s.constant
	}
	fmt.Fprintf(&b, "\tcase %s:\n", strings.Join(names, ", "))
	b.WriteString("\t\treturn nil\n")
	b.WriteString("\tdefault:\n")
	fmt.Fprintf(&b, "\t\treturn fmt.Errorf(%q, referenced.State)\n",
		fmt.Sprintf("%s: %s.%s requires %s to be terminal, got state %%q",
			g.funcName, g.referencer.structName, g.field.fieldName, g.referenced.structName))
	b.WriteString("\t}\n")
	b.WriteString("}\n")

	return b.String()
}
