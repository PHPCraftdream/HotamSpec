package generator

import (
	"strconv"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// BuildRequirements renders docs/gen/REQUIREMENTS.md: the domain's own
// requirement roster (and its supporting Stakeholders/Assumptions/Operators/
// Processes/Goals sections) plus a closing section whose shape depends on
// consumer.
//
// consumer selects the output profile (mirrors the naming/threading
// convention RenderClaudeMDFromTemplate/RenderEmbeddedThinkingBlock/
// ComputeCrystalCharCountFixpoint established this wave, task #135, in
// claudemd.go):
//
//   - consumer == false (full profile, the default, backward-compatible
//     case): output is byte-identical to the historical unconditional
//     behavior — the closing section is BuildToolDerivedSection() (~44
//     synthetic requirements describing the HOTAM FRAMEWORK's own CLI
//     surface) followed by the full "## Methodology (generated from module
//     docstrings)" encyclopedia (every §-section's Canon/Narrative/Why).
//   - consumer == true: both of those sections are framework
//     self-documentation, not the consumer's own business domain content —
//     an external domain with one seed requirement otherwise gets a
//     REQUIREMENTS.md dominated by ~27KB of framework internals. They are
//     replaced by a short closing section: a brief contract statement plus
//     pointers to where the full detail lives (domains/<name>/docs/gen/
//     tools/INDEX.md, the root crystal, and — since the consumer profile
//     never writes docs/gen/thinking/*.md, see genSpec's `if !consumer {
//     thinkingDocs := ... }` gate in cmd/hotam/gen_spec.go — a note that
//     `--profile full` unlocks the full methodology reference on demand).
//
// domainName is used only by the consumer-profile closing section (to
// domain-prefix the tools/INDEX.md pointer, matching the repo-root-relative
// convention every other cross-reference inside a generated docs/gen/*.md
// file follows — see domains/hotam-spec-self/docs/gen/AGENT-CONTEXT.md);
// the full-profile path never reads it.
// RequirementsMDHasContent reports whether REQUIREMENTS.md carries real
// domain content — i.e. whether the graph is non-empty (task #364: withheld
// entirely from the docs/gen/ write set when the domain has zero axes/
// stakeholders/requirements/conflicts/assumptions/operators/processes/goals/
// entity_types/entities, mirroring the conditional-write pattern
// DECISIONS.md/ENTITIES.md/TENSIONS.md/PIPELINE.md/MODELS.md already use). A
// genuinely empty domain has nothing to render but the EmptyNotice
// placeholder BuildRequirements already falls back to (g.IsEmpty()) — so a
// fresh, unmodeled domain now produces ZERO files under docs/gen/, not a
// directory full of calm-but-empty placeholders.
func RequirementsMDHasContent(g *ontology.Graph) bool {
	return !g.IsEmpty()
}

func BuildRequirements(g *ontology.Graph, domainName string, consumer bool) string {
	reqs := NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder })
	stakeholders := NarrativeOrder(g.Stakeholders, func(s ontology.Stakeholder) int { return s.DeclOrder })
	assumptions := NarrativeOrder(g.Assumptions, func(a ontology.Assumption) int { return a.DeclOrder })
	operators := NarrativeOrder(g.Operators, func(o ontology.Operator) int { return o.DeclOrder })
	processes := NarrativeOrder(g.Processes, func(p ontology.Process) int { return p.DeclOrder })
	goals := NarrativeOrder(g.Goals, func(gl ontology.Goal) int { return gl.DeclOrder })
	lines := docHeaderLines("REQUIREMENTS", g)
	lines = append(lines, serviceText(g, "# REQUIREMENTS.md — Requirement roster & methodology (Hotam-Spec)"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "Generated from the executable model: the methodology narrative comes from the framework's own methodology registry (RULE + `Canon:§` + WHY); the roster below comes from `domains/<name>/graph.json`. Source of truth is the code + graph; this text is generated, so it cannot drift from the model."))
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	lines = append(lines, serviceText(g, "## Requirement roster"))
	lines = append(lines, "")
	if g.IsEmpty() {
		lines = append(lines, localizedEmptyNotice(g))
		lines = append(lines, "")
	} else if len(reqs) == 0 {
		lines = append(lines, serviceText(g, "_No requirements declared in this domain yet._"))
		lines = append(lines, "")
	} else {
		lines = append(lines, serviceText(g, "| id | status | owner | assumptions | claim |"))
		lines = append(lines, "|---|---|---|---|---|")
		for _, r := range reqs {
			assn := "—"
			if len(r.Assumptions) > 0 {
				assn = strings.Join(r.Assumptions, ", ")
			}
			lines = append(lines, "| `"+r.ID+"` | "+Cell(localizedStatus(g, r.Status))+" | `"+r.Owner+"` | "+Cell(assn)+" | "+Cell(requirementClaim(g, r))+" |")
		}
		lines = append(lines, "")
	}

	if len(stakeholders) > 0 {
		lines = append(lines, serviceText(g, "## Stakeholders"))
		lines = append(lines, "")
		lines = append(lines, serviceText(g, "| id | name | domain |"))
		lines = append(lines, "|---|---|---|")
		for _, s := range stakeholders {
			lines = append(lines, "| `"+s.ID+"` | "+Cell(s.Name)+" | "+Cell(s.Domain)+" |")
		}
		lines = append(lines, "")
	}

	if len(assumptions) > 0 {
		lines = append(lines, serviceText(g, "## Assumptions"))
		lines = append(lines, "")
		lines = append(lines, serviceText(g, "| id | status | owner | statement |"))
		lines = append(lines, "|---|---|---|---|")
		for _, a := range assumptions {
			lines = append(lines, "| `"+a.ID+"` | "+localizedStatus(g, a.Status)+" | `"+a.Owner+"` | "+Cell(a.Statement)+" |")
		}
		lines = append(lines, "")
	}

	if len(operators) > 0 {
		lines = append(lines, serviceText(g, "## Operators"))
		lines = append(lines, "")
		lines = append(lines, serviceText(g, "| id | stakeholder | lifecycle | budget | parent |"))
		lines = append(lines, "|---|---|---|---|---|")
		for _, op := range operators {
			budget := "unbounded"
			if op.ContextBudget.Limit != 0 {
				budget = strconv.Itoa(op.ContextBudget.Limit) + " (" + op.ContextBudget.Measure + ")"
			}
			parent := "—"
			if op.Parent != nil && *op.Parent != "" {
				parent = "`" + *op.Parent + "`"
			}
			lines = append(lines, "| `"+op.ID+"` | `"+op.Stakeholder+"` | "+op.Lifecycle+" | "+budget+" | "+parent+" |")
		}
		lines = append(lines, "")
	}

	if len(processes) > 0 {
		lines = append(lines, serviceText(g, "## Processes"))
		lines = append(lines, "")
		lines = append(lines, serviceText(g, "| id | lifecycle | steps | roles | drives |"))
		lines = append(lines, "|---|---|---|---|---|")
		for _, p := range processes {
			stepNames := "—"
			if len(p.Steps) > 0 {
				names := make([]string, len(p.Steps))
				for i, s := range p.Steps {
					names[i] = s.Name
				}
				stepNames = strings.Join(names, ", ")
			}
			roles := "—"
			if len(p.RolesRequired) > 0 {
				roles = strings.Join(p.RolesRequired, ", ")
			}
			drives := "—"
			if len(p.DrivesEntities) > 0 {
				drives = strings.Join(p.DrivesEntities, ", ")
			}
			lines = append(lines, "| `"+p.ID+"` | "+p.Lifecycle.Slug+" | "+Cell(stepNames)+" | "+Cell(roles)+" | "+Cell(drives)+" |")
		}
		lines = append(lines, "")
	}

	gateSignoffRows := gateSignoffRows(reqs)
	if len(gateSignoffRows) > 0 {
		lines = append(lines, serviceText(g, "## Gate signoffs"))
		lines = append(lines, "")
		lines = append(lines, serviceText(g, "Per-requirement gate-passage facts declared via `gate_signoffs` (see `ontology.GateSignoff`) — the single typed carrier for a domain's staged-gate methodology, when it declares one."))
		lines = append(lines, "")
		lines = append(lines, serviceText(g, "| requirement | stage | state | pipeline_run | deferred_reason |"))
		lines = append(lines, "|---|---|---|---|---|")
		lines = append(lines, gateSignoffRows...)
		lines = append(lines, "")
	}

	if len(goals) > 0 {
		lines = append(lines, serviceText(g, "## Goals"))
		lines = append(lines, "")
		lines = append(lines, serviceText(g, "| id | owner | lifecycle | target | predicate |"))
		lines = append(lines, "|---|---|---|---|---|")
		for _, go_ := range goals {
			target := "—"
			if go_.TargetState.Target != "" {
				target = go_.TargetState.Target
			}
			lines = append(lines, "| `"+go_.ID+"` | `"+go_.Owner+"` | "+go_.Lifecycle+" | "+Cell(target)+" | "+Cell(go_.TargetState.Predicate)+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, "---")
	lines = append(lines, "")
	if consumer {
		lines = append(lines, consumerClosingSection(g)...)
	} else {
		lines = append(lines, BuildToolDerivedSectionLocalized(g.RenderLanguage))
		lines = append(lines, "---")
		lines = append(lines, "")
		lines = append(lines, serviceText(g, "## Methodology (generated from module docstrings)"))
		lines = append(lines, "")
		for i, me := range ModuleOrder {
			doc := ModuleDocstring(me.Mod)
			ordinal := i + 1
			lines = append(lines, "### "+strconv.Itoa(ordinal)+". "+me.Label+" — `hotam_spec."+me.Mod+"`")
			lines = append(lines, "")
			if doc != "" {
				lines = append(lines, doc)
				lines = append(lines, "")
			}
		}
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

// gateSignoffRows renders one table row per (requirement, GateSignoff) pair,
// in requirement narrative order then GateSignoffs declaration order —
// purely additive: a domain that has never populated GateSignoffs on any
// requirement contributes zero rows, so BuildRequirements's "## Gate
// signoffs" section is skipped entirely (see the len(gateSignoffRows) > 0
// guard at its call site), leaving REQUIREMENTS.md byte-identical for every
// domain that predates this field.
func gateSignoffRows(reqs []ontology.Requirement) []string {
	var rows []string
	for _, r := range reqs {
		for _, gs := range r.GateSignoffs {
			reason := "—"
			if gs.DeferredReason != "" {
				reason = Cell(gs.DeferredReason)
			}
			rows = append(rows, "| `"+r.ID+"` | "+Cell(gs.Stage)+" | "+gs.State+" | `"+gs.PipelineRun+"` | "+reason+" |")
		}
	}
	return rows
}

// consumerClosingSection renders the short closing section that REPLACES
// BuildToolDerivedSection() + the methodology encyclopedia under the
// consumer profile: a brief contract statement (closely paraphrasing the
// root crystal's own opening line, claudeMDTemplate in claudemd.go — not new
// marketing copy) plus pointers to where the full detail lives. It is
// deliberately a few lines, not a scaled-down copy of the sections it
// replaces (tool-derived requirements describe the FRAMEWORK's own CLI
// surface, not the consumer's business domain; the methodology encyclopedia
// duplicates docs/gen/thinking/*.md and the crystal's own EMBEDDED-THINKING
// block in full rather than condensed).
//
// The last bullet ("full methodology reference: switch to --profile full")
// is safe to state unconditionally: docs/gen/thinking/*.md is exactly the
// artifact the consumer profile skips (genSpec's `if !consumer { thinkingDocs
// := ... }` gate, cmd/hotam/gen_spec.go) and --profile full is what
// re-enables it, so the pointer never dangles.
//
// The commands pointer is `hotam -h`: the consumer profile no longer writes
// framework/tools/* at all.
func consumerClosingSection(g *ontology.Graph) []string {
	return []string{
		serviceText(g, "## About Hotam-Spec"),
		"",
		serviceText(g, "**Hotam-Spec** is a framework for writing requirements as executable code: each requirement is an atomic object with a method, and the test that runs it is also the generator of its text — one run proves the behavior and emits a minimal sentence that mirrors back into the same code."),
		"",
		serviceText(g, "This file covers this domain's own requirement roster only. For the framework itself:"),
		"",
		serviceText(g, "- **Implemented commands** — `hotam -h`."),
		serviceText(g, "- **Operating loop** (how an agent should read and act on this model) — the root crystal: `CLAUDE.md` / `AGENTS.md` / `GEMINI.md`."),
		serviceText(g, "- **Full methodology reference** (every §-section's Canon/Narrative/Why) — not generated under this profile; regenerate with `hotam gen-spec --profile full` if ever needed."),
		"",
	}
}
