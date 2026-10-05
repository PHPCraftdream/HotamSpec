package generator

import (
	"sort"
	"strconv"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func DecisionsMDHasContent(g *ontology.Graph) bool {
	for _, r := range g.Requirements {
		if r.MTag != "" {
			return true
		}
	}
	return false
}

func extractOpenQuestion(status string) string {
	stripped := strings.TrimSpace(status[len(ontology.StatusOPENPrefix):])
	if strings.HasPrefix(stripped, "(") && strings.HasSuffix(stripped, ")") {
		return strings.TrimSpace(stripped[1 : len(stripped)-1])
	}
	return status
}

func BuildDecisions(g *ontology.Graph) string {
	var tagged []ontology.Requirement
	for _, r := range g.Requirements {
		if r.MTag != "" {
			tagged = append(tagged, r)
		}
	}
	taggedSorted := make([]ontology.Requirement, len(tagged))
	copy(taggedSorted, tagged)
	sort.SliceStable(taggedSorted, func(i, j int) bool {
		ni, _ := strconv.Atoi(taggedSorted[i].MTag[1:])
		nj, _ := strconv.Atoi(taggedSorted[j].MTag[1:])
		return ni < nj
	})

	lines := []string{localizedBanner(g), ReaderHeaderLine("DECISIONS", g), ""}
	lines = append(lines, serviceText(g, "# DECISIONS.md — Open methodology decisions (Hotam-Spec)"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "Generated mirror of the M-registry. The SINGLE source of truth is the\ngraph's OPEN requirements with non-empty `m_tag` in the active domain's\n`graph.json`. This file retires the hand-maintained M-table\nthat lived in CLAUDE.md — per `R-drift-structurally-impossible` and the\ndev-coin Param.status + HOLES.md precedent: one source of truth,\ngenerated mirror."))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "A requirement carries an M-tag iff it mirrors an open methodology\ndecision the resolver must ratify. Requirements without an M-tag are\ndomain-level open holes that have not been elevated to\nmethodology-altitude decisions."))
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	if g.IsEmpty() {
		lines = append(lines, localizedEmptyNotice(g))
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	lines = append(lines, serviceText(g, "## Open decisions (sorted by M-tag)"))
	lines = append(lines, "")
	if len(taggedSorted) == 0 {
		lines = append(lines, serviceText(g, "_No OPEN requirements carry an M-tag yet._"))
		lines = append(lines, "")
	} else {
		lines = append(lines, serviceText(g, "| M-tag | requirement | owner | question |"))
		lines = append(lines, "|---|---|---|---|")
		for _, r := range taggedSorted {
			question := extractOpenQuestion(r.Status)
			lines = append(lines, "| "+r.MTag+" | `"+r.ID+"` | `"+r.Owner+"` | "+Cell(question)+" |")
		}
		lines = append(lines, "")
	}

	lines = append(lines, serviceText(g, "## Notes"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "Decision-IDs not yet anchored to a graph requirement (no `m_tag` mirror)\nremain prose-only in CLAUDE.md. The convergence direction is to\ncrystallize each such M-row as a Requirement with the corresponding\n`m_tag`."))
	lines = append(lines, "")

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}
