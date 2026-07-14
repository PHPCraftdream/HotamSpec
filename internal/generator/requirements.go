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
//     pointers to where the full detail lives (docs/gen/tools/INDEX.md, the
//     root crystal, and — since the consumer profile never writes
//     docs/gen/thinking/*.md, see genSpec's `if !consumer { thinkingDocs :=
//     ... }` gate in cmd/hotam/gen_spec.go — a note that `--profile full`
//     unlocks the full methodology reference on demand).
func BuildRequirements(g *ontology.Graph, consumer bool) string {
	reqs := NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder })
	stakeholders := NarrativeOrder(g.Stakeholders, func(s ontology.Stakeholder) int { return s.DeclOrder })
	assumptions := NarrativeOrder(g.Assumptions, func(a ontology.Assumption) int { return a.DeclOrder })
	operators := NarrativeOrder(g.Operators, func(o ontology.Operator) int { return o.DeclOrder })
	processes := NarrativeOrder(g.Processes, func(p ontology.Process) int { return p.DeclOrder })
	goals := NarrativeOrder(g.Goals, func(gl ontology.Goal) int { return gl.DeclOrder })
	lines := []string{Banner, ReaderHeaderLine("REQUIREMENTS", g), ""}
	lines = append(lines, "# REQUIREMENTS.md — Requirement roster & methodology (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines, "Generated from the executable model: the methodology narrative comes from the framework's own methodology registry (RULE + `Canon:§` + WHY); the roster below comes from `domains/<name>/graph.json`. Source of truth is the code + graph; this text is generated, so it cannot drift from the model.")
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	lines = append(lines, "## Requirement roster")
	lines = append(lines, "")
	if g.IsEmpty() {
		lines = append(lines, EmptyNotice)
		lines = append(lines, "")
	} else if len(reqs) == 0 {
		lines = append(lines, "_No requirements declared in this domain yet._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | status | owner | assumptions | claim |")
		lines = append(lines, "|---|---|---|---|---|")
		for _, r := range reqs {
			assn := "—"
			if len(r.Assumptions) > 0 {
				assn = strings.Join(r.Assumptions, ", ")
			}
			lines = append(lines, "| `"+r.ID+"` | "+Cell(r.Status)+" | `"+r.Owner+"` | "+Cell(assn)+" | "+Cell(r.Claim)+" |")
		}
		lines = append(lines, "")
	}

	if len(stakeholders) > 0 {
		lines = append(lines, "## Stakeholders")
		lines = append(lines, "")
		lines = append(lines, "| id | name | domain |")
		lines = append(lines, "|---|---|---|")
		for _, s := range stakeholders {
			lines = append(lines, "| `"+s.ID+"` | "+Cell(s.Name)+" | "+Cell(s.Domain)+" |")
		}
		lines = append(lines, "")
	}

	if len(assumptions) > 0 {
		lines = append(lines, "## Assumptions")
		lines = append(lines, "")
		lines = append(lines, "| id | status | owner | statement |")
		lines = append(lines, "|---|---|---|---|")
		for _, a := range assumptions {
			lines = append(lines, "| `"+a.ID+"` | "+a.Status+" | `"+a.Owner+"` | "+Cell(a.Statement)+" |")
		}
		lines = append(lines, "")
	}

	if len(operators) > 0 {
		lines = append(lines, "## Operators")
		lines = append(lines, "")
		lines = append(lines, "| id | stakeholder | lifecycle | budget | parent |")
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
		lines = append(lines, "## Processes")
		lines = append(lines, "")
		lines = append(lines, "| id | lifecycle | steps | roles | drives |")
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

	if len(goals) > 0 {
		lines = append(lines, "## Goals")
		lines = append(lines, "")
		lines = append(lines, "| id | owner | lifecycle | target | predicate |")
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
		lines = append(lines, consumerClosingSection()...)
	} else {
		lines = append(lines, BuildToolDerivedSection())

		lines = append(lines, "---")
		lines = append(lines, "")
		lines = append(lines, "## Methodology (generated from module docstrings)")
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
func consumerClosingSection() []string {
	return []string{
		"## About Hotam-Spec",
		"",
		"**Hotam-Spec** is executable memory and discipline for a human + LLM-agent fleet: understand, evolve, protect, and support a shared model over time. Contradictory requirements are one of its properties — held open as tension-graph nodes, never silently discarded.",
		"",
		"This file covers this domain's own requirement roster only. For the framework itself:",
		"",
		"- **Implemented commands** — `docs/gen/tools/INDEX.md`.",
		"- **Operating loop** (how an agent should read and act on this model) — the root crystal: `CLAUDE.md` / `AGENTS.md` / `GEMINI.md`.",
		"- **Full methodology reference** (every §-section's Canon/Narrative/Why) — not generated under this profile; regenerate with `hotam gen-spec --profile full` if ever needed.",
		"",
	}
}
