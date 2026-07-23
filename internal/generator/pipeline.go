// pipeline.go renders docs/gen/PIPELINE.md: the domain-overview projection
// (docs/AUTHORED-SPEC-CONTRACT.md §9, R-domain-overview-projection) that answers
// "how is this whole thing put together, stage by stage?" as well as reading
// the source prose would. Every other docs/gen/*.md file this package builds
// is a confrontation/status doc (REQUIREMENTS/HISTORY/UNENFORCED/TENSIONS —
// what's SETTLED, what's REJECTED, what's still open); PIPELINE.md is the
// first one that renders a coherent whole-domain narrative instead of a
// roster or a diff.
//
// Source data: every ontology.Process node's Steps (name / requires_role /
// why), cross-referenced against the domain's EntityTypes two ways — (1)
// Process.DrivesEntities (the direct, authoritative process->entity edge)
// and (2) EntityType kind:reference fields whose ref_target resolves to
// another EntityType in the same domain — a read-only pass over
// ontology.EntityType, resolved directly from the shared ontology source
// (the authored-spec pivot, task #223b/#228, retired the gen-code generator
// this file's edge-resolution once mirrored; this is now the only resolver
// of that edge, not a parallel read-only twin of one).
package generator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/graphfacts"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// pipelineRefEdge is one resolved kind:reference edge between two EntityTypes
// of the same domain: referencer.field -> referenced.
type pipelineRefEdge struct {
	referencerSlug string
	fieldName      string
	referencedSlug string
}

// resolvePipelineRefEdges scans every EntityType's kind:reference fields and
// keeps only the ones whose ref_target resolves to another EntityType slug
// declared in this SAME domain — a real structural pipeline edge, not a
// cross-graph/role reference (e.g. sdr-package.feature_lead -> "Stakeholder"
// has no EntityType of that slug and is skipped — an honest gap, not an
// error).
func resolvePipelineRefEdges(entityTypes []ontology.EntityType) []pipelineRefEdge {
	bySlug := make(map[string]struct{}, len(entityTypes))
	for _, et := range entityTypes {
		bySlug[et.Slug] = struct{}{}
	}
	var edges []pipelineRefEdge
	for _, et := range entityTypes {
		for _, f := range et.Fields {
			if f.Kind != "reference" || f.RefTarget == "" {
				continue
			}
			if _, ok := bySlug[f.RefTarget]; !ok {
				continue
			}
			edges = append(edges, pipelineRefEdge{
				referencerSlug: et.Slug,
				fieldName:      f.Name,
				referencedSlug: f.RefTarget,
			})
		}
	}
	return edges
}

// pipelineAnchorPattern matches a generic "typed identifier-looking" token in
// free text: a capitalized word followed by one or more hyphen-joined
// segments (e.g. "R-gate-pg3-brd-approved", "P-G3", "P-G1-R") — deliberately
// domain-agnostic, no hardcoded "R-" prefix, so any typed anchor convention a
// domain's authors use surfaces as a citation, not just the framework's own
// R-/C-/A- family.
var pipelineAnchorPattern = regexp.MustCompile(`\b[A-Z][A-Za-z0-9]*(-[A-Za-z0-9]+)+\b`)

// docURLFragmentPattern matches a markdown/doc-link URL FRAGMENT — a `#`
// followed by its heading-slug body (letters, digits, hyphens; markdown
// heading-range slugs commonly double the hyphen for an en-dash separator,
// e.g. "#Planning-gates-P-G0--P-G4"). This is a reference to a SECTION of an
// external document, not a standalone requirement/gate anchor citation — even
// though its slug body happens to be built from real anchor-shaped
// substrings (e.g. "P-G0", "P-G4"), those substrings name the ENDPOINTS of a
// doc-heading range, not a claim that THIS step is gated by each one
// individually. Stripped out before pipelineAnchorPattern runs (see
// renderGateCitations) so no fragment fallout — truncated or otherwise —
// gets cited as if it were its own gate.
var docURLFragmentPattern = regexp.MustCompile(`#[A-Za-z0-9-]+`)

// BuildPipeline renders docs/gen/PIPELINE.md: a stage table (Стадия | Вход |
// Выход | Gate | Кто утверждает) plus a Mermaid flowchart, built from every
// ontology.Process node's Steps. Step names are rendered VERBATIM in the
// methodology author's own language — Cyrillic is legitimate here: this is
// generated markdown under docs/gen/, a projection over the domain's own
// prose, not authored Go source (docs/AUTHORED-SPEC-CONTRACT.md governs
// authored spec/ code, an unrelated concern). A domain with zero Process
// nodes gets an honest, valid, non-empty placeholder — never a blank file or
// a generator error (§Process is an opt-in aspect: "no processes modeled
// yet" is a normal domain shape, not a defect).
//
// gateOrder is the domain's declared gate_stage_order (loader.
// ResolveGateStageOrder), threaded through by the caller so this package
// stays a pure function of its arguments (no filesystem reads of its own) —
// it feeds ONLY renderPipelineLiveState (task #331, R4-process-why): a
// generated "Live state" section rendered from typed carriers
// (gate_signoffs, conflict lifecycle), so a domain's authored Process.Why
// prose can stay durable rationale without also carrying a point-in-time
// status snapshot that inevitably goes stale (nothing regenerates prose).
// nil is the honest no-op — a domain with no declared gate_stage_order and
// no Conflicts renders byte-identically to before this parameter existed.
func BuildPipeline(g *ontology.Graph, domainName string, gateOrder []string) string {
	sourceHint := "from the active domain's `graph.json`"
	if domainName != "" {
		sourceHint = "from `domains/" + domainName + "/graph.json`"
	}

	lines := []string{Banner, ReaderHeaderLine("PIPELINE", g), ""}
	lines = append(lines, "# PIPELINE.md — Domain overview: how this is put together, stage by stage (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines, "> Generated by `hotam gen-spec` "+sourceHint+". Do not hand-edit.")
	lines = append(lines, "")
	lines = append(lines, "Generated from the domain's own `Process` nodes (§Process, the opt-in behavioral aspect: a Lifecycle + ordered Steps + roles_required + drives_entities) and the EntityTypes those processes drive — a whole-domain narrative answering \"how does this work end to end?\" without reading the source prose (docs/AUTHORED-SPEC-CONTRACT.md §9, R-domain-overview-projection).")
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	if liveState := renderPipelineLiveState(g, gateOrder); len(liveState) > 0 {
		lines = append(lines, liveState...)
	}

	if g.IsEmpty() {
		lines = append(lines, EmptyNotice)
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	processes := NarrativeOrder(g.Processes, func(p ontology.Process) int { return p.DeclOrder })
	if len(processes) == 0 {
		lines = append(lines, "_This domain does not model any processes yet (§Process is opt-in — zero `Process` nodes declared in `graph.json`). There is no stage pipeline to render. Once a Process node is landed (`hotam apply-proposal` with a `ProposedProcess`), `hotam gen-spec` will populate this file with its stage table and flow diagram._")
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	entityBySlug := make(map[string]ontology.EntityType, len(g.EntityTypes))
	for _, et := range g.EntityTypes {
		entityBySlug[et.Slug] = et
	}
	reqBySlug := make(map[string]ontology.Requirement, len(g.Requirements))
	for _, r := range g.Requirements {
		reqBySlug[r.ID] = r
	}
	refEdges := resolvePipelineRefEdges(g.EntityTypes)

	for _, p := range processes {
		lines = append(lines, "## Process `"+p.ID+"`")
		lines = append(lines, "")
		if p.Why != "" {
			lines = append(lines, Cell(p.Why))
			lines = append(lines, "")
		}

		if len(p.Steps) == 0 {
			lines = append(lines, "_(no steps declared for this process)_")
			lines = append(lines, "")
			continue
		}

		stageOutputs := make([]string, len(p.Steps))
		for i, step := range p.Steps {
			stageOutputs[i] = stageEntityCell(step, entityBySlug, p)
		}

		lines = append(lines, "### Stages")
		lines = append(lines, "")
		lines = append(lines, "| Стадия | Вход | Выход | Gate | Кто утверждает |")
		lines = append(lines, "|---|---|---|---|---|")
		for i, step := range p.Steps {
			input := "—"
			if i > 0 {
				input = stageOutputs[i-1]
			}
			output := stageOutputs[i]
			gate := renderGateCitations(step.Why, reqBySlug)
			approver := "—"
			if step.RequiresRole != "" {
				approver = "`" + step.RequiresRole + "`"
			}
			stageName := stageNameCell(step)
			lines = append(lines, "| "+stageName+" | "+input+" | "+output+" | "+gate+" | "+approver+" |")
		}
		lines = append(lines, "")

		lines = append(lines, "### Flow")
		lines = append(lines, "")
		lines = append(lines, renderPipelineMermaid(p)...)
		lines = append(lines, "")

		if len(p.DrivesEntities) > 0 {
			lines = append(lines, "### Artifacts moved through this process")
			lines = append(lines, "")
			sortedDrives := append([]string{}, p.DrivesEntities...)
			sort.Strings(sortedDrives)
			drivesSet := make(map[string]struct{}, len(sortedDrives))
			for _, slug := range sortedDrives {
				drivesSet[slug] = struct{}{}
				lines = append(lines, "- "+entityAnchorCell(slug, entityBySlug))
			}
			lines = append(lines, "")

			// Artifact-to-artifact structural dependencies (kind:reference
			// fields whose ref_target resolves to another EntityType of this
			// domain), restricted to edges where BOTH ends are among this
			// process's own driven entities, so this list stays a narrative of
			// THIS process's artifact graph, not the whole domain's.
			var processEdges []pipelineRefEdge
			for _, e := range refEdges {
				_, referencerIn := drivesSet[e.referencerSlug]
				_, referencedIn := drivesSet[e.referencedSlug]
				if referencerIn && referencedIn {
					processEdges = append(processEdges, e)
				}
			}
			if len(processEdges) > 0 {
				sort.Slice(processEdges, func(i, j int) bool {
					if processEdges[i].referencerSlug != processEdges[j].referencerSlug {
						return processEdges[i].referencerSlug < processEdges[j].referencerSlug
					}
					return processEdges[i].fieldName < processEdges[j].fieldName
				})
				lines = append(lines, "**Artifact dependencies** (`kind:reference` fields between the artifacts above):")
				lines = append(lines, "")
				for _, e := range processEdges {
					lines = append(lines, "- "+entityAnchorCell(e.referencerSlug, entityBySlug)+"."+e.fieldName+" → "+entityAnchorCell(e.referencedSlug, entityBySlug))
				}
				lines = append(lines, "")
			}
		}
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

// renderPipelineLiveState renders PIPELINE.md's "Live state" section (task
// #331, R4-process-why): the ONLY point-in-time-status content this file
// carries that is guaranteed fresh on every `hotam gen-spec`, because it is
// computed directly from typed carriers (Requirement.GateSignoffs via
// internal/graphfacts's GateSignoffTally/GateFrontier, and g.Conflicts'
// lifecycle via ConflictLifecycleTally) rather than living as free prose in
// some Process/Step's authored `why` text. Authored `why` prose stays
// durable rationale ("why this stage exists, why this order") — a
// point-in-time claim like "27 of 32 requirements SIGNED as of 2026-07-21"
// belongs HERE, regenerated fresh every run, never frozen into prose that
// nothing re-derives (the exact staleness this task's design consult
// diagnosed in gpsm-sm's own Process.Why).
//
// Deliberately a PURE function of graph state only — no `today`/date
// parameter. Dating this section would recreate the identical
// staleness-snapshot smell being fixed here, and would break `gen-spec`
// idempotency (two consecutive renders of the same graph must be
// byte-identical — CI's byte-reproducibility check, mirrored by this
// package's own TestGenerator_DoubleRegenerateIsIdentical).
//
// Returns nil (the honest no-op — omit the whole section, not an empty
// header) when order is empty AND the graph has no Conflicts: a domain that
// never declared gate_stage_order and carries no Conflicts (e.g. a fresh
// domain, or one whose methodology tracks neither) has nothing live to
// report yet, and rendering an empty/placeholder section would itself be
// the same "claims something that isn't there" smell this section exists to
// avoid.
func renderPipelineLiveState(g *ontology.Graph, order []string) []string {
	if len(order) == 0 && len(g.Conflicts) == 0 {
		return nil
	}

	lines := []string{
		"## Live state (generated from typed carriers — authoritative for \"where are we now\")",
		"",
	}

	if len(order) > 0 {
		// GateFrontier returns the stage NAME of the furthest stage any
		// Requirement has touched (plus its own tally, unused here — this
		// loop recomputes each stage's own tally via GateSignoffTally so
		// every stage up to and including the frontier gets its real
		// number, not just the frontier itself). frontierIdx stays -1 (no
		// Requirement has recorded a single matching GateSignoff yet) when
		// !hasFrontier, so every stage renders "not started" — an honest
		// "declared but never touched" domain state.
		frontierStage, _, hasFrontier := graphfacts.GateFrontier(g, order)
		frontierIdx := -1
		if hasFrontier {
			for i, stage := range order {
				if stage == frontierStage {
					frontierIdx = i
					break
				}
			}
		}
		for i, stage := range order {
			if i > frontierIdx {
				lines = append(lines, "- **"+stage+"** — not started")
				continue
			}
			tally := graphfacts.GateSignoffTally(g, order, stage, "")
			total := tally.Signed + tally.Deferred
			lines = append(lines, fmt.Sprintf("- **%s** — %d/%d SIGNED · %d DEFERRED", stage, tally.Signed, total, tally.Deferred))
		}
	}

	if len(g.Conflicts) > 0 {
		decided, total, _ := graphfacts.ConflictLifecycleTally(g, "DECIDED")
		held, _, _ := graphfacts.ConflictLifecycleTally(g, "HELD")
		unresolved, _, _ := graphfacts.ConflictLifecycleTally(g, "UNRESOLVED")
		lines = append(lines, fmt.Sprintf("- **Conflicts** — %d total: %d DECIDED · %d HELD · %d UNRESOLVED", total, decided, held, unresolved))
	}

	lines = append(lines, "")
	lines = append(lines, "_This section regenerates on every `hotam gen-spec` from `gate_signoffs`/conflict lifecycles — where authored prose below disagrees with it, THIS section is current._")
	lines = append(lines, "")

	return lines
}

// stageNameCell renders one Step's authored name verbatim (the methodology
// author's own language — Cyrillic legitimate here, R-domain-overview-
// projection's own scope note) with its Go-code-adjacent identifier, if any,
// only ever added as a parenthetical ANNOTATION alongside the author's name,
// never replacing it — step.Invokes carries an EntityType.event-shaped
// "<slug>.<event>" identifier when the step is wired to a lifecycle
// transition (see entities.go's processDestinations, which parses the same
// field the same way).
func stageNameCell(step ontology.Step) string {
	name := Cell(step.Name)
	if step.Invokes != "" {
		return name + " (`" + step.Invokes + "`)"
	}
	return name
}

// stageEntityCell renders the "Вход"/"Выход" cell for one Step of process p:
// the EntityType(s) THIS SPECIFIC stage produces/consumes, resolved two
// ways, most-specific first, deterministically (never guessing):
//
//  1. step.Invokes, when it names an EntityType lifecycle event directly
//     ("<slug>.<event>", the same parse entities.go's processDestinations
//     already performs) — the strongest signal, an explicit wiring.
//  2. Referencer-bound correlation: among the process's own DrivesEntities,
//     keep only the EntityType slugs whose token literally appears in THIS
//     step's own `why` text, narrowed from "any claim in the domain" to
//     "this specific step's own why", so a step's cell reflects what that
//     step's own prose actually names, not an undifferentiated dump of
//     every entity the whole process ever touches.
//
// If neither source narrows the stage to a specific EntityType, the cell is
// an honest "—" (a step whose why text does not name any of the process's
// driven entities has no structural signal to report — not "all of them").
func stageEntityCell(step ontology.Step, entityBySlug map[string]ontology.EntityType, p ontology.Process) string {
	if step.Invokes != "" && strings.Contains(step.Invokes, ".") {
		slug := strings.SplitN(step.Invokes, ".", 2)[0]
		if _, ok := entityBySlug[slug]; ok {
			return entityAnchorCell(slug, entityBySlug)
		}
	}

	if len(p.DrivesEntities) == 0 {
		return "—"
	}
	whyLower := strings.ToLower(step.Why)
	var matched []string
	for _, slug := range p.DrivesEntities {
		if strings.Contains(whyLower, strings.ToLower(slug)) {
			matched = append(matched, slug)
		}
	}
	if len(matched) == 0 {
		return "—"
	}
	sort.Strings(matched)
	cells := make([]string, 0, len(matched))
	for _, slug := range matched {
		cells = append(cells, entityAnchorCell(slug, entityBySlug))
	}
	return strings.Join(cells, ", ")
}

// entityAnchorCell renders one EntityType slug as a clickable in-document
// anchor to its own ## heading in ENTITIES.md's sibling section of THIS SAME
// generated file family (docs/gen/ENTITIES.md, written whenever
// EntitiesMDHasContent(g) — see entities.go), falling back to a bare
// backticked slug when the slug does not resolve to a declared EntityType in
// this domain (a DrivesEntities/ref_target entry naming something the domain
// has not (yet) typed as an EntityType — an honest gap, not an error).
func entityAnchorCell(slug string, entityBySlug map[string]ontology.EntityType) string {
	if _, ok := entityBySlug[slug]; ok {
		return "[`" + slug + "`](ENTITIES.md#" + slug + ")"
	}
	return "`" + slug + "`"
}

// renderGateCitations extracts every typed anchor (R-/C-/A-/P-G*-shaped
// token — pipelineAnchorPattern's domain-agnostic shape) from a step's `why`
// text and renders each as a citation, resolving the ones that match a
// requirement ID in this domain's own roster to that requirement's own
// status, so the gate line reads as "a REAL SETTLED gate", not just a
// string. Anchors that do not resolve to a requirement in this domain (a
// gate-and-agent-modes.md prose citation like "P-G1" with no matching
// R-gate-pg1-* Requirement.ID, or an external doc anchor) are still cited
// verbatim — the citation is honest either way, never fabricated, never
// silently dropped.
//
// Doc-link URL fragments (docURLFragmentPattern, e.g. the
// "#Planning-gates-P-G0--P-G4" in
// "(docs/gates-and-agent-modes.md#Planning-gates-P-G0--P-G4)") are stripped
// BEFORE the anchor scan runs: a fragment names a SECTION of an external
// document, not a per-step gate citation, even though its slug body is built
// from real anchor-shaped substrings — without this strip, pipelineAnchorPattern
// would match "Planning-gates-P-G0" (a truncated doc-heading fragment, not any
// requirement's ID) and "P-G4" (the far endpoint of the heading's OWN range,
// not a gate this specific step is bound by) as if they were independent
// citations. A real anchor that happens to appear inside a step's why text
// OUTSIDE any "#..." fragment (the normal case — "Якорь: R-gate-pg0-source-ready."
// is prose, not a URL) is completely unaffected.
func renderGateCitations(why string, reqBySlug map[string]ontology.Requirement) string {
	stripped := docURLFragmentPattern.ReplaceAllString(why, "")
	matches := pipelineAnchorPattern.FindAllString(stripped, -1)
	if len(matches) == 0 {
		return "—"
	}
	seen := map[string]struct{}{}
	var cites []string
	for _, m := range matches {
		if _, dup := seen[m]; dup {
			continue
		}
		seen[m] = struct{}{}
		if r, ok := reqBySlug[m]; ok {
			cites = append(cites, "`"+m+"` ("+Cell(r.Status)+")")
		} else {
			cites = append(cites, "`"+m+"`")
		}
	}
	return strings.Join(cites, ", ")
}

// renderPipelineMermaid renders a left-to-right Mermaid flowchart, one node
// per Step (in declared order, stage[i] --> stage[i+1]), labeled with the
// step's own authored name — the same verbatim-name discipline
// stageNameCell applies to the table.
func renderPipelineMermaid(p ontology.Process) []string {
	lines := []string{"```mermaid", "flowchart LR"}
	ids := make([]string, len(p.Steps))
	for i, step := range p.Steps {
		id := "S" + MermaidID(p.ID) + "_" + MermaidID(step.Name)
		ids[i] = id
		lines = append(lines, "    "+id+"[\""+mermaidEscape(step.Name)+"\"]")
	}
	for i := 0; i < len(ids)-1; i++ {
		lines = append(lines, "    "+ids[i]+" --> "+ids[i+1])
	}
	lines = append(lines, "```")
	return lines
}

// mermaidEscape strips characters that break a Mermaid node label wrapped in
// double quotes (the label syntax renderPipelineMermaid uses).
func mermaidEscape(s string) string {
	s = strings.ReplaceAll(s, "\"", "'")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
