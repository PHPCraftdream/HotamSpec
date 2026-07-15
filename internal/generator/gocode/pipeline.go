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

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// pipelineGateModel is the resolved, Go-identifier-shaped view of one
// pipeline gate: a referencer EntityType's kind:reference field whose
// ref_target resolves to another EntityType of the SAME domain. Built once
// (BuildPipelineGateModels) so the gate function renderer (RenderPipelineFile)
// and its test renderer (RenderPipelineTestFile) never re-derive the
// referencer/referenced identifiers BuildEntityModel already computed — the
// same single-source discipline every other *Model type in this package
// follows (contract §0).
//
// preciseState is set (contract §2.1) when the SETTLED requirement corpus
// names one CONCRETE state of referenced for THIS SPECIFIC referencer (see
// resolvePreciseGateState) — the rendered gate then compares
// referenced.State against preciseState.constant instead of accepting any
// terminal state. It is nil (the pre-§2.1 default) whenever no such
// unambiguous, referencer-bound signal is found; terminalStates then remains
// the only structural signal available, and RequiresTerminal is generated as
// before.
type pipelineGateModel struct {
	referencer     *entityModel
	field          fieldModel
	referenced     *entityModel
	funcName       string       // Gate<Referencer><Field>Requires<Referenced>Terminal, or ...Requires<Referenced>_<State> when preciseState is set
	terminalStates []stateModel // referenced.states with IsTerminal() == true, graph order
	preciseState   *stateModel  // contract §2.1: the one referenced.state this referencer's claim names, if unambiguous
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
//
// requirements is the FULL domain requirement corpus (any status; only
// SETTLED ones are used, contract §2.1/§3) — passed so each gate can search
// for a precise-state signal (resolvePreciseGateState). Callers that have no
// requirement corpus available (isolated unit fixtures) may pass nil; every
// gate then simply falls back to the pre-§2.1 general RequiresTerminal
// behavior, unchanged.
func BuildPipelineGateModels(models []*entityModel, requirements []ontology.Requirement) ([]*pipelineGateModel, error) {
	bySlug := make(map[string]*entityModel, len(models))
	for _, em := range models {
		bySlug[em.src.Slug] = em
	}

	var settledClaims []string
	for _, r := range requirements {
		if r.Status == ontology.StatusSETTLED {
			settledClaims = append(settledClaims, r.Claim)
		}
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

			preciseState := resolvePreciseGateState(referencer, referenced, settledClaims)

			var funcName string
			if preciseState != nil {
				funcName = fmt.Sprintf("Gate%s%sRequires%s_%s", referencer.structName, f.fieldName, referenced.structName, preciseState.constant)
			} else {
				funcName = fmt.Sprintf("Gate%s%sRequires%sTerminal", referencer.structName, f.fieldName, referenced.structName)
			}
			out = append(out, &pipelineGateModel{
				referencer:     referencer,
				field:          f,
				referenced:     referenced,
				funcName:       funcName,
				terminalStates: terminalStates,
				preciseState:   preciseState,
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

// findPipelineGate looks up the one pipelineGateModel (if any) rendered for
// referencer's field named fieldName — the same referencer/field pair
// BuildPipelineGateModels above resolved (kind:reference, ref_target
// resolving to another EntityType of this domain). Used by
// resolveScopedFieldMatches/requirements.go (contract §2.1 field-atom
// integration, task #209) to find whether a claim-matched reference field
// already has a precise-state (or general-terminal) pipeline gate to mirror
// in its own field-atom sub-test, without requiring requirements.go to
// re-derive or re-scan the referencer/ref_target resolution
// BuildPipelineGateModels already performs (contract §0 one source of
// truth). Returns nil when no gate was built for this exact referencer+field
// pair (e.g. the field is not kind:reference, or its ref_target does not
// resolve in this domain).
func findPipelineGate(gates []*pipelineGateModel, referencer *entityModel, fieldName string) *pipelineGateModel {
	for _, g := range gates {
		if g.referencer.structName == referencer.structName && g.field.fieldName == fieldName {
			return g
		}
	}
	return nil
}

// resolvePreciseGateState implements GEN-CODE-CONTRACT.md §2.1: it looks for
// a SINGLE unambiguous "this referencer's claim names this exact state of
// referenced" signal, deterministically, without LLM-guessing — the same
// discipline resolveGateAnchorCorrelate (requirements.go) already applies to
// gate/order requirement atoms, applied here to pipeline-gate edges instead.
//
// The search has two parts, both required to agree:
//
//  1. Domain-wide: for each of referenced's own lifecycle states, build the
//     candidate token "<referenced.slug>_<state.name>" (also tried with a
//     "-" separator, since graph authors are not consistent) and check
//     whether that literal token appears (case-insensitively) in ANY SETTLED
//     requirement's claim anywhere in the domain. A state whose token is
//     never claimed anywhere is not a candidate at all.
//
//  2. Referencer-bound: among the domain-wide candidates from (1), keep only
//     the ones whose token ALSO appears in referencer's OWN EntityType.why
//     text. why is exactly the field stewards use to cite the SETTLED
//     requirement(s) that justify this specific EntityType/edge (see
//     RenderAuditFile's R-speak-by-reference discipline for why text) — a
//     token surviving in referencer.src.Why is therefore independently tied
//     to THIS referencer, not merely "mentioned somewhere in the domain" (a
//     forecast_v3 hit in sdr-package.why does not make sdr-package.прогноз's
//     gate accidentally precise-v2, because "forecast_v2" is never a
//     substring of sdr-package's own why text). This is the same anchor-
//     correlation IDEA resolveGateAnchorCorrelate already uses (search
//     EntityType.why for a real correlate), narrowed from "any EntityType in
//     the domain" to "this specific referencer" because that is what
//     distinguishes two different referencers pointing at the same
//     referenced EntityType (brd-package vs sdr-package both -> forecast).
//
// If step 2 leaves exactly one state, that state is the precise gate target.
// If it leaves zero or more than one (genuinely ambiguous, or the referencer
// simply never names a specific version), resolvePreciseGateState returns
// nil and the caller keeps the general RequiresTerminal gate — contract
// §2.1's explicit "не гадай" (do not guess) instruction.
//
// Known limitation (honestly documented, not silently papered over): this
// heuristic's referencer-binding signal is referencer.src.Why, a single free
// -text field. It works for the real prat domain because prat's stewards
// already write why as "R-<id> requires ... forecast_v2 ... as a condition
// of gate P-Gn" — but a referencer whose why text does not itself quote the
// concrete state token (e.g. why merely says "see R-gate-pg3-brd-approved"
// without repeating "forecast_v2") will not get a precise gate today, even
// if a human reading both texts together could resolve it. That gap is the
// same kind of honest gap contract §3's closing row already tolerates
// elsewhere, not a silent misclassification: RequiresTerminal remains
// correct-but-coarser fallback behavior in that case, never a wrong gate.
func resolvePreciseGateState(referencer, referenced *entityModel, settledClaims []string) *stateModel {
	referencerWhy := strings.ToLower(referencer.src.Why)
	referencedSlug := strings.ToLower(referenced.src.Slug)

	var matched *stateModel
	for i := range referenced.states {
		s := referenced.states[i]
		stateName := strings.ToLower(s.src.Name)
		tokenUnderscore := referencedSlug + "_" + stateName
		tokenHyphen := referencedSlug + "-" + stateName

		inDomainClaims := false
		for _, claim := range settledClaims {
			lc := strings.ToLower(claim)
			if strings.Contains(lc, tokenUnderscore) || strings.Contains(lc, tokenHyphen) {
				inDomainClaims = true
				break
			}
		}
		if !inDomainClaims {
			continue
		}

		if !strings.Contains(referencerWhy, tokenUnderscore) && !strings.Contains(referencerWhy, tokenHyphen) {
			continue
		}

		if matched != nil {
			// Two different states of referenced both claimed AND both tied
			// to this same referencer's why text - genuinely ambiguous per
			// contract §2.1 ("все совпадения должны указывать на ОДНО и то
			// же состояние, иначе неоднозначность"). Refuse to guess.
			return nil
		}
		matchedCopy := s
		matched = &matchedCopy
	}

	return matched
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
// that entity's State satisfies the gate — GEN-CODE-CONTRACT.md §0's
// "artifact not accepted until predecessor terminal" made literal, OR
// (contract §2.1, when g.preciseState is set) the stricter "artifact not
// accepted until predecessor is in this EXACT state" — never a later
// terminal state standing in for it. The referencer/field are only used to
// compose the function name and the error message (contract §0 mirror: the
// message should let a human/LLM auditor see which graph edge this gate
// enforces without re-deriving anything), never as a parameter — the gate is
// a pure predicate over the REFERENCED entity's own state, callable
// regardless of which referencer instance is asking.
func renderPipelineGateFunc(g *pipelineGateModel) string {
	if g.preciseState != nil {
		return renderPrecisePipelineGateFunc(g)
	}

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

// renderPrecisePipelineGateFunc renders the contract §2.1 precise-state
// variant: referenced.State must equal g.preciseState.constant exactly - a
// later terminal state of referenced (e.g. forecast's v3) does NOT satisfy a
// gate that names an earlier, non-terminal state (e.g. forecast's v2,
// brd-package's real requirement) - the exact structural bug this stage
// fixes (a general RequiresTerminal gate wrongly accepting v3 where the
// claim names v2).
func renderPrecisePipelineGateFunc(g *pipelineGateModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// %s gates GEN-CODE-CONTRACT.md section 2.1's precise-state pipeline\n", g.funcName)
	fmt.Fprintf(&b, "// mirror: %s.%s references %s (kind:reference, ref_target=%q) - the SETTLED\n",
		g.referencer.structName, g.field.fieldName, g.referenced.structName, g.referenced.src.Slug)
	fmt.Fprintf(&b, "// requirement corpus names %s's own %q state specifically for this\n",
		g.referenced.structName, g.preciseState.src.Name)
	fmt.Fprintf(&b, "// referencer (see requirements_audit.md), so the artifact must not be\n")
	b.WriteString("// accepted by the next pipeline stage until it is in EXACTLY that state -\n")
	b.WriteString("// a later terminal state of the same entity does not satisfy this gate.\n")
	fmt.Fprintf(&b, "func %s(referenced *%s) error {\n", g.funcName, g.referenced.structName)
	fmt.Fprintf(&b, "\tif referenced.State != %s {\n", g.preciseState.constant)
	fmt.Fprintf(&b, "\t\treturn fmt.Errorf(%q, referenced.State)\n",
		fmt.Sprintf("%s: %s.%s requires %s to be in state %q, got state %%q",
			g.funcName, g.referencer.structName, g.field.fieldName, g.referenced.structName, g.preciseState.value))
	b.WriteString("\t}\n")
	b.WriteString("\treturn nil\n")
	b.WriteString("}\n")

	return b.String()
}
