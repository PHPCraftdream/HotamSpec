package generator

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

// OpenMDHasContent reports whether OPEN.md carries real domain content —
// i.e. whether the graph is non-empty (task #364: withheld entirely from the
// docs/gen/ write set when the domain has zero axes/stakeholders/
// requirements/conflicts/assumptions/operators/processes/goals/entity_types/
// entities, mirroring the conditional-write pattern DECISIONS.md/
// ENTITIES.md/TENSIONS.md/PIPELINE.md/MODELS.md already use). A genuinely
// empty domain has nothing to render but the EmptyNotice placeholder
// BuildOpen already falls back to (g.IsEmpty()) — so a fresh, unmodeled
// domain now produces ZERO files under docs/gen/, not a directory full of
// calm-but-empty placeholders.
func OpenMDHasContent(g *ontology.Graph) bool {
	return !g.IsEmpty()
}

func BuildOpen(g *ontology.Graph) string {
	reqs := NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder })
	conflicts := NarrativeOrder(g.Conflicts, func(c ontology.Conflict) int { return c.DeclOrder })
	var openReqs []ontology.Requirement
	for _, r := range reqs {
		if r.IsOpen() {
			openReqs = append(openReqs, r)
		}
	}
	var unresolved []ontology.Conflict
	for _, c := range conflicts {
		if c.IsUnresolved() {
			unresolved = append(unresolved, c)
		}
	}

	lines := docHeaderLines("OPEN", g)
	lines = append(lines, serviceText(g, "# OPEN.md — Open registry (Hotam-Spec)"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "Generated mirror of what is still open: OPEN(question) requirements and conflicts not yet resolved by a resolver (DETECTED / ACKNOWLEDGED). This is the visibility-of-the-open layer; run `hotam what-now` for the prioritized next actions that close these."))
	lines = append(lines, "")

	if g.IsEmpty() {
		lines = append(lines, localizedEmptyNotice(g))
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	lines = append(lines,
		serviceText(g, "Open requirements: **%d**. Unresolved conflicts: **%d**.", len(openReqs), len(unresolved)))
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	lines = append(lines, serviceText(g, "## OPEN requirements"))
	lines = append(lines, "")
	if len(openReqs) == 0 {
		lines = append(lines, serviceText(g, "_None._"))
		lines = append(lines, "")
	} else {
		lines = append(lines, serviceText(g, "| id | owner | question |"))
		lines = append(lines, "|---|---|---|")
		for _, r := range openReqs {
			question := openQuestion(r.Status)
			lines = append(lines, "| `"+r.ID+"` | `"+r.Owner+"` | "+Cell(question)+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, serviceText(g, "## Unresolved conflicts (no resolver resolution yet)"))
	lines = append(lines, "")
	if len(unresolved) == 0 {
		lines = append(lines, serviceText(g, "_None._"))
		lines = append(lines, "")
	} else {
		lines = append(lines, serviceText(g, "| id | axis | lifecycle | resolver | members |"))
		lines = append(lines, "|---|---|---|---|---|")
		for _, c := range unresolved {
			mem := strings.Join(c.Members, ", ")
			lines = append(lines, "| `"+c.ID+"` | `"+c.Axis+"` | "+c.Lifecycle+" | `"+c.Resolver+"` | "+Cell(mem)+" |")
		}
		lines = append(lines, "")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

func openQuestion(status string) string {
	q := status[len("OPEN"):]
	q = strings.TrimSpace(q)
	q = strings.Trim(q, "()")
	q = strings.TrimSpace(q)
	return q
}
