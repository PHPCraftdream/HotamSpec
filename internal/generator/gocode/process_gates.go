// process_gates.go renders GEN-CODE-CONTRACT.md's stage-6 output: one
// COMPOSITE gate function per gated ontology.Process.Step —
// "Gate<Step>Complete(...)" — requiring EVERY relevant driven EntityType of
// that step to be in its own required lifecycle state simultaneously.
//
// This closes a coverage gap pipeline.go's per-field gates (stage 5) cannot
// close on their own: stage 5 renders one gate per kind:reference EDGE
// between two EntityTypes (e.g. fr-graph -> fr-registry), never a single
// function asserting "every artifact this whole stage/gate needs is ready at
// once" — which is exactly what a composite requirement like
// R-gate-pg1-planning-approved actually claims ("реестр, граф, порядок,
// P-G1-R pass, forecast_v1, ... review" — a conjunction of SEVEN conditions,
// not one pairwise edge). Before this file, that composite requirement's only
// atom was a single field-presence check on ONE of those seven artifacts
// (ImplementationOrder.GraphDependencies) — technically an atom, but not a
// mirror of the requirement's own ALL-of-these-together shape (the same
// "atom found != claim covered" gap GEN-CODE-CONTRACT.md §3.1 already legislates
// against for single-field claims, here at the composite-gate level instead).
//
// # Step -> relevant-entity resolution (the honest, deliberately-scoped part)
//
// ontology.Process.DrivesEntities lives at the WHOLE-PROCESS level (contract
// §Process: "a Lifecycle + ordered Steps + roles_required + drives_entities"
// — one flat slice, no per-Step sub-field binding an entity to exactly one
// step). Resolving "which of these driven entities does THIS step actually
// require" without extending the ontology (a bigger, cross-domain decision
// deliberately out of scope for this pilot task — see this file's own doc
// footer for the honest write-up) uses the SAME anchor-correlation discipline
// pipeline.go's resolvePreciseGateState already established for pairwise
// gates, applied here to step<->entity binding instead of
// referencer<->referenced-state binding:
//
//  1. R-anchor citation: every typed R-anchor token in step.Why (the SAME
//     idAnchorPattern shape used throughout this package, filtered to ids
//     starting with "R-") is searched for, verbatim, inside each candidate
//     EntityType's OWN why text. An EntityType whose why text cites the exact
//     requirement id this step's own why text also cites is independently
//     tied to that step by a real graph-authored citation, not by a
//     coincidental word match (mirrors resolvePreciseGateState's rule 2:
//     "token survives in referencer.src.Why").
//  2. Short gate-token citation: prat's own step why-text convention embeds a
//     short "P-G<N>" token (e.g. "P-G1", extracted here by
//     shortGateTokenPattern) identifying which Planning gate this step
//     enacts. When that SAME short token appears in a candidate EntityType's
//     own why text, OR in exactly one of that EntityType's own lifecycle
//     STATE why texts, the entity (or, for a state-level hit, that PRECISE
//     state) is bound to this step. A state-level hit additionally sets the
//     precise required state for that entity in this gate (never falling
//     back to "any terminal state" once a step-specific state has been
//     named) — this is deliberately the same precision fix contract §2.1
//     already made for pairwise pipeline gates (forecast_v2 vs forecast_v3
//     must never satisfy each other's gate), applied here so a composite
//     gate cannot be satisfied by, say, forecast sitting in v3 when the step
//     it gates actually names v1.
//
// Both signals are searched independently; an entity is relevant to a step if
// EITHER signal ties it there. Verified against the real prat domain
// (docs/coverage-vs-source.md-adjacent audit, this task): every one of the
// five REAL Planning gate steps (source-ready/planning-approved/brd-scope/
// brd-approved/solution-ready) resolves a non-empty, precise entity set this
// way — no step needed the "fall back to the whole Process.DrivesEntities
// list" escape hatch buildProcessGateEntities' own doc comment describes, but
// that escape hatch still exists and is exercised (and tested) for the
// honest case where a future domain's step why-text does not carry either
// signal.
//
// # Which steps get a composite gate
//
// Only steps whose own why text cites at least one R-anchor (signal 1's
// token list is non-empty) are gates in the methodology's own sense — a step
// with zero R-anchor citations (e.g. prat's "execution-handoff", a pure
// stage-4 HANDOFF per its own why text, not one of the five Planning gates
// P-G0..P-G4) is not rendered a composite gate function at all. This mirrors
// pipeline.go's own "no real edge, no gate" discipline: a synthetic
// always-true gate for a step that documents no actual gate condition would
// be a vacuous check, exactly what contract §0 forbids ("явно и громко
// отказать [молчать], а не молча угадать" — here, simply not generating
// anything is the honest choice, not guessing a gate that was never claimed).
package gocode

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// shortGateTokenPattern matches prat's short Planning-gate token convention
// ("P-G0".."P-G4", or any "P-G<digits>" shape a future domain's steps might
// use) inside a step's own why text. Domain-agnostic in the DIGIT (no
// hardcoded "0".."4"), but the literal "P-G" prefix is prat's own authored
// convention, not a framework-wide one — a domain whose steps never embed
// this shape simply never benefits from signal 2 (see
// buildProcessGateEntities' doc comment), same honest-gap discipline as
// resolvePreciseGateState's own "referencer never repeats the token" case.
var shortGateTokenPattern = regexp.MustCompile(`\bP-G\d+\b`)

// processStepGateModel is the resolved, Go-identifier-shaped view of one
// gated Process.Step: the composite Gate<Step>Complete function requiring
// every one of its resolved processGateEntity entries to be in its own
// required state.
type processStepGateModel struct {
	process       ontology.Process
	step          ontology.Step
	funcName      string // Gate<Process><Step>Complete
	requirementID string // the R-gate-pg<n>-<step> requirement this composite gate mirrors, "" if none found
	entities      []processGateEntity
	// wholeProcessFallback is true when NEITHER anchor-citation nor
	// short-token signal resolved any entity for this step, so this gate
	// falls back to the entire Process.DrivesEntities set — the honest,
	// explicitly-documented weaker case (see buildProcessGateEntities' doc
	// comment). Surfaced on the model (not just decided silently inside the
	// builder) so the audit renderer can state this limitation plainly for
	// this exact gate, per this task's explicit instruction not to claim
	// precision the resolution does not actually have.
	wholeProcessFallback bool
}

// processGateEntity is one EntityType this step's composite gate requires,
// plus which lifecycle state (precise, or "any terminal") it must be in.
type processGateEntity struct {
	entity       *entityModel
	preciseState *stateModel  // set when signal 2's state-level hit resolved a specific state
	terminal     []stateModel // used when preciseState is nil: entity's own terminal/quiescent states
	// signal names which resolution signal tied this entity to the step, for
	// the audit render (never silently merged into one undifferentiated
	// "relevant" bucket - contract §0 mirror principle: a human/LLM auditor
	// should see WHY this entity is here, not just THAT it is).
	signal processGateSignal
}

type processGateSignal int

const (
	processGateSignalAnchor processGateSignal = iota // signal 1: step why cites an R-anchor also cited by entity.why
	processGateSignalToken                           // signal 2: step's short P-G<n> token found in entity.why (general)
	processGateSignalState                           // signal 2, state-level: short token found in one specific state.why (precise)
	processGateSignalFallback
)

// stepRAnchors extracts every R-anchor token (idAnchorPattern's shape,
// filtered to ids beginning "R-") from a step's own why text, deduplicated
// and sorted for determinism.
func stepRAnchors(why string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, tok := range idAnchorPattern.FindAllString(why, -1) {
		if !strings.HasPrefix(tok, "R-") {
			continue
		}
		if _, dup := seen[tok]; dup {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	sort.Strings(out)
	return out
}

// buildProcessGateEntities resolves the relevant-entity set for one Step of
// process p, against the already-built entityModels for p's own
// DrivesEntities slugs (driven, keyed by slug). See this file's package doc
// comment for the full two-signal resolution rule; this function applies it
// entity-by-entity, deterministically (driven is iterated in p.DrivesEntities'
// own declared order).
//
// Honest fallback: if zero entities resolve via either signal, EVERY driven
// entity of the process is returned instead (wholeProcessFallback=true) —
// this function never returns an empty set for a step that legitimately has
// at least one driven entity, because an empty composite gate would silently
// assert "nothing is required", which is a worse lie than "we could not
// narrow this down, so every process artifact is required" (still a real,
// checkable, honestly-labeled gate — just a coarser one, exactly parallel to
// pipeline.go's own RequiresTerminal-vs-precise-state fallback).
func buildProcessGateEntities(step ontology.Step, driven map[string]*entityModel, drivesOrder []string, rAnchors []string) ([]processGateEntity, bool) {
	shortTok := ""
	if m := shortGateTokenPattern.FindString(step.Why); m != "" {
		shortTok = m
	}

	var out []processGateEntity
	for _, slug := range drivesOrder {
		em, ok := driven[slug]
		if !ok {
			continue
		}

		// Signal 1: an R-anchor this step's why cites is ALSO cited in this
		// entity's own why text.
		anchorHit := false
		for _, a := range rAnchors {
			if em.src.Why != "" && strings.Contains(em.src.Why, a) {
				anchorHit = true
				break
			}
		}

		// Signal 2, state-level: the short P-G<n> token appears in EXACTLY
		// ONE of this entity's own lifecycle state why texts - that state
		// becomes the precise requirement (never "any terminal") for this
		// entity in this gate, the same non-guessing discipline
		// resolvePreciseGateState applies ("if step 2 leaves exactly one
		// state ... if it leaves zero or more than one ... returns nil").
		var stateHit *stateModel
		ambiguousState := false
		if shortTok != "" {
			for i := range em.states {
				s := em.states[i]
				if s.src.Why != "" && strings.Contains(s.src.Why, shortTok) {
					if stateHit != nil {
						ambiguousState = true
						break
					}
					sc := s
					stateHit = &sc
				}
			}
		}

		// Signal 2, entity-level: the short token appears in the entity's own
		// why text (not tied to one specific state).
		tokenHit := shortTok != "" && em.src.Why != "" && strings.Contains(em.src.Why, shortTok)

		switch {
		case stateHit != nil && !ambiguousState:
			out = append(out, processGateEntity{entity: em, preciseState: stateHit, signal: processGateSignalState})
		case anchorHit:
			out = append(out, processGateEntity{entity: em, terminal: terminalStatesOf(em), signal: processGateSignalAnchor})
		case tokenHit:
			out = append(out, processGateEntity{entity: em, terminal: terminalStatesOf(em), signal: processGateSignalToken})
		}
	}

	if len(out) > 0 {
		return out, false
	}

	// Honest fallback: nothing resolved via either signal - every driven
	// entity of the process is required, explicitly labeled as the coarser
	// case (wholeProcessFallback=true on the caller's model).
	out = out[:0]
	for _, slug := range drivesOrder {
		em, ok := driven[slug]
		if !ok {
			continue
		}
		out = append(out, processGateEntity{entity: em, terminal: terminalStatesOf(em), signal: processGateSignalFallback})
	}
	return out, true
}

// terminalStatesOf returns m's own terminal (or quiescent) lifecycle states,
// in graph order - the same IsTerminal() definition pipeline.go's
// BuildPipelineGateModels already uses, so "done" can never silently disagree
// between stage 5 and stage 6.
func terminalStatesOf(m *entityModel) []stateModel {
	var out []stateModel
	for _, s := range m.states {
		if s.src.IsTerminal() {
			out = append(out, s)
		}
	}
	return out
}

// findStepRequirementID looks for a SETTLED requirement whose id matches
// prat's own observed "R-gate-pg<n>[<suffix-letter>]-<step-name>" convention
// exactly - the id, after stripping a "R-gate-pg<digits><optional-letters>-"
// prefix, equals step.Name verbatim. This is a deliberately narrow, exact
// match (never substring/fuzzy) - it exists only to let the composite gate's
// audit entry cite the ONE requirement it is the direct mirror of, without
// which requirements_audit.md's "## Process Gates" section could not point
// back to "## Requirements"'s own entry for R-gate-pg1-planning-approved.  A
// step whose name does not appear as any requirement id's suffix this way
// (e.g. prat's own "execution-handoff", which also has zero R-anchors and so
// never reaches this function at all - see buildProcessStepGateModels)
// simply gets "" - no requirement anchor cited, not a guessed one.
var stepGatePrefixPattern = regexp.MustCompile(`^R-gate-pg\d+[a-z]*-`)

func findStepRequirementID(stepName string, settled []ontology.Requirement) string {
	for _, r := range settled {
		if !stepGatePrefixPattern.MatchString(r.ID) {
			continue
		}
		suffix := stepGatePrefixPattern.ReplaceAllString(r.ID, "")
		if suffix == stepName {
			return r.ID
		}
	}
	return ""
}

// BuildProcessStepGateModels resolves every gated Step (a step whose own why
// text cites at least one R-anchor - see this file's package doc comment for
// why an anchor-less step, e.g. a pure handoff, is not a gate) of every
// Process in the domain into a processStepGateModel. entityModels is the
// domain's already-built []*entityModel (keyed by slug internally); settled
// is the domain's SETTLED requirement corpus, used only by
// findStepRequirementID's exact-suffix match (never for entity resolution
// itself - that is entirely the two-signal why-text search above).
func BuildProcessStepGateModels(processes []ontology.Process, entityModels []*entityModel, settled []ontology.Requirement) ([]*processStepGateModel, error) {
	bySlug := make(map[string]*entityModel, len(entityModels))
	for _, em := range entityModels {
		bySlug[em.src.Slug] = em
	}

	var out []*processStepGateModel
	for _, p := range processes {
		driven := make(map[string]*entityModel, len(p.DrivesEntities))
		for _, slug := range p.DrivesEntities {
			if em, ok := bySlug[slug]; ok {
				driven[slug] = em
			}
			// A DrivesEntities slug that does not resolve to any EntityType in
			// this domain is the same honest gap pipeline.go tolerates for an
			// unresolvable ref_target - silently skipped here, not an error:
			// Process.DrivesEntities is a free-text slug list, not itself
			// validated against EntityType existence elsewhere in this
			// generator.
		}

		for _, step := range p.Steps {
			rAnchors := stepRAnchors(step.Why)
			if len(rAnchors) == 0 {
				// Not a gate in the methodology's own sense (see package doc
				// comment) - no composite gate function rendered.
				continue
			}

			procIdent, err := ToPascalCase(p.ID)
			if err != nil {
				return nil, fmt.Errorf("gocode: process %q: %w", p.ID, err)
			}
			stepIdent, err := ToPascalCase(step.Name)
			if err != nil {
				return nil, fmt.Errorf("gocode: process %q step %q: %w", p.ID, step.Name, err)
			}

			entities, fallback := buildProcessGateEntities(step, driven, p.DrivesEntities, rAnchors)
			if len(entities) == 0 {
				// This process declares zero resolvable DrivesEntities at all
				// (either DrivesEntities itself is empty, or none of its slugs
				// resolved to a real EntityType) - nothing to gate on.
				continue
			}

			out = append(out, &processStepGateModel{
				process:              p,
				step:                 step,
				funcName:             fmt.Sprintf("Gate%s%sComplete", procIdent, stepIdent),
				requirementID:        findStepRequirementID(step.Name, settled),
				entities:             entities,
				wholeProcessFallback: fallback,
			})
		}
	}

	byFuncName := make(map[string]struct{}, len(out))
	for _, g := range out {
		if _, dup := byFuncName[g.funcName]; dup {
			return nil, fmt.Errorf("gocode: two process-step gates both resolve to Go function %q", g.funcName)
		}
		byFuncName[g.funcName] = struct{}{}
	}

	return out, nil
}

// paramName returns the Go parameter identifier for one gate entity: the
// entity's own camelCase-translated slug (e.g. "frRegistry"), matching the
// same translation ToCamelCase already gives every other generated local
// identifier in this package - never a hand-rolled abbreviation of the
// struct name.
func (e processGateEntity) paramName() (string, error) {
	return ToCamelCase(e.entity.src.Slug)
}

// RenderProcessGatesFile renders the stage-6 composite-gate functions: one
// Gate<Process><Step>Complete(...) per processStepGateModel, each taking one
// pointer parameter per required entity (in that model's own resolved
// order) and returning the FIRST failing requirement as an error (fail-fast,
// same convention pipeline.go's own gate functions use), or nil once every
// parameter satisfies its own required state.
func RenderProcessGatesFile(packageName string, gates []*processStepGateModel) ([]byte, error) {
	var b strings.Builder
	b.WriteString(OwnershipMarker)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "package %s\n\n", packageName)

	if len(gates) == 0 {
		b.WriteString("// No Process.Step in this domain cites at least one R-anchor in its own\n")
		b.WriteString("// why text (GEN-CODE-CONTRACT.md stage 6), so no composite process-step\n")
		b.WriteString("// gate functions are generated.\n")
		return []byte(b.String()), nil
	}

	b.WriteString("import \"fmt\"\n\n")
	for i, g := range gates {
		if i != 0 {
			b.WriteString("\n")
		}
		src, err := renderProcessStepGateFunc(g)
		if err != nil {
			return nil, err
		}
		b.WriteString(src)
	}

	return []byte(b.String()), nil
}

func renderProcessStepGateFunc(g *processStepGateModel) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "// %s gates GEN-CODE-CONTRACT.md stage 6's composite pipeline mirror:\n", g.funcName)
	fmt.Fprintf(&b, "// Process %q, step %q requires EVERY one of the entities below to be in\n", g.process.ID, g.step.Name)
	b.WriteString("// its own required state SIMULTANEOUSLY - unlike a single pairwise pipeline\n")
	b.WriteString("// gate (pipeline_test.go), this mirrors the requirement's own AND-of-N shape.\n")
	if g.requirementID != "" {
		fmt.Fprintf(&b, "// Atom: %s - see requirements_audit.md\n", g.requirementID)
	}
	if g.wholeProcessFallback {
		b.WriteString("// LIMITATION (honest, not silently narrowed): no per-step entity binding\n")
		b.WriteString("// signal resolved for this step (see requirements_audit.md), so this gate\n")
		b.WriteString("// requires the process's ENTIRE DrivesEntities set, not a step-scoped subset.\n")
	}

	params := make([]string, len(g.entities))
	for i, e := range g.entities {
		pname, err := e.paramName()
		if err != nil {
			return "", err
		}
		params[i] = fmt.Sprintf("%s *%s", pname, e.entity.structName)
	}
	fmt.Fprintf(&b, "func %s(%s) error {\n", g.funcName, strings.Join(params, ", "))

	for _, e := range g.entities {
		pname, err := e.paramName()
		if err != nil {
			return "", err
		}
		if e.preciseState != nil {
			fmt.Fprintf(&b, "\tif %s.State != %s {\n", pname, e.preciseState.constant)
			fmt.Fprintf(&b, "\t\treturn fmt.Errorf(%q, %s.State)\n",
				fmt.Sprintf("%s: step %q requires %s to be in state %q, got state %%q", g.funcName, g.step.Name, e.entity.structName, e.preciseState.value), pname)
			b.WriteString("\t}\n")
		} else {
			names := make([]string, len(e.terminal))
			for j, s := range e.terminal {
				names[j] = s.constant
			}
			fmt.Fprintf(&b, "\tswitch %s.State {\n", pname)
			fmt.Fprintf(&b, "\tcase %s:\n", strings.Join(names, ", "))
			b.WriteString("\t\t// ok\n")
			b.WriteString("\tdefault:\n")
			fmt.Fprintf(&b, "\t\treturn fmt.Errorf(%q, %s.State)\n",
				fmt.Sprintf("%s: step %q requires %s to be terminal, got state %%q", g.funcName, g.step.Name, e.entity.structName), pname)
			b.WriteString("\t}\n")
		}
	}
	b.WriteString("\treturn nil\n")
	b.WriteString("}\n")

	return b.String(), nil
}
