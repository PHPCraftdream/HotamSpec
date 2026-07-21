package generator

import (
	"strconv"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

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

	lines := []string{Banner, ReaderHeaderLine("OPEN", g), ""}
	lines = append(lines, "# OPEN.md — Open registry (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines,
		"Generated mirror of what is still open: OPEN(question) requirements and "+
			"conflicts not yet resolved by a resolver (DETECTED / ACKNOWLEDGED). This is "+
			"the visibility-of-the-open layer; run `hotam what-now` for the "+
			"prioritized next actions that close these.")
	lines = append(lines, "")

	if g.IsEmpty() {
		lines = append(lines, EmptyNotice)
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	lines = append(lines,
		"Open requirements: **"+strconv.Itoa(len(openReqs))+"**. "+
			"Unresolved conflicts: **"+strconv.Itoa(len(unresolved))+"**.")
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	lines = append(lines, "## OPEN requirements")
	lines = append(lines, "")
	if len(openReqs) == 0 {
		lines = append(lines, "_None._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | owner | question |")
		lines = append(lines, "|---|---|---|")
		for _, r := range openReqs {
			question := openQuestion(r.Status)
			lines = append(lines, "| `"+r.ID+"` | `"+r.Owner+"` | "+Cell(question)+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, "## Unresolved conflicts (no resolver resolution yet)")
	lines = append(lines, "")
	if len(unresolved) == 0 {
		lines = append(lines, "_None._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| id | axis | lifecycle | resolver | members |")
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
