package generator

import (
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func extractDecidedRationale(lifecycle string) string {
	if !strings.HasPrefix(lifecycle, ontology.ConflictDECIDEDPrefix) {
		return ""
	}
	inner := strings.TrimSpace(lifecycle[len(ontology.ConflictDECIDEDPrefix):])
	if strings.HasPrefix(inner, "(") && strings.HasSuffix(inner, ")") {
		return strings.TrimSpace(inner[1 : len(inner)-1])
	}
	return inner
}

func extractRevisitRationale(lifecycle string) string {
	if !strings.HasPrefix(lifecycle, ontology.ConflictREVISITPrefix) {
		return ""
	}
	inner := strings.TrimSpace(lifecycle[len(ontology.ConflictREVISITPrefix):])
	if strings.HasPrefix(inner, "(") && strings.HasSuffix(inner, ")") {
		return strings.TrimSpace(inner[1 : len(inner)-1])
	}
	return inner
}

func backtickedList(items []string) string {
	out := make([]string, len(items))
	for i, m := range items {
		out[i] = "`" + m + "`"
	}
	return strings.Join(out, ", ")
}

func BuildHistory(g *ontology.Graph) string {
	reqs := NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder })
	conflicts := NarrativeOrder(g.Conflicts, func(c ontology.Conflict) int { return c.DeclOrder })
	lines := []string{Banner, ReaderHeaderLine("HISTORY", g), ""}
	lines = append(lines, "# HISTORY.md — Methodology decision history (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines, "Generated from the anti-relitigation markers in the model: REJECTED\nrequirements (what was tried and discarded — REPLACES marker) and DECIDED /\nREVISIT_WHEN conflict lifecycles (what was resolved, why, and the condition\nunder which to re-open). Source of truth is the active domain's `graph.json`;\nthis text is generated so it cannot drift.")
	lines = append(lines, "")
	lines = append(lines, "A fresh agent reads this to recover the methodology's history without\nre-litigating settled questions — the historian role of the AI made into\nsubstrate (R-history-from-rejected-markers).")
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	if g.IsEmpty() {
		lines = append(lines, EmptyNotice)
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	var rejected []ontology.Requirement
	for _, r := range reqs {
		if r.Status == ontology.StatusREJECTED {
			rejected = append(rejected, r)
		}
	}

	lines = append(lines, "## REJECTED requirements (what we tried and discarded)")
	lines = append(lines, "")
	if len(rejected) == 0 {
		lines = append(lines, "_None._")
		lines = append(lines, "")
	} else {
		for _, r := range rejected {
			lines = append(lines, "### `"+r.ID+"` — "+Cell(r.Claim))
			lines = append(lines, "")
			lines = append(lines, "- **owner:** `"+r.Owner+"`")
			lines = append(lines, "- **why:** "+r.Why)
			lines = append(lines, "")
		}
	}

	var decided []ontology.Conflict
	for _, c := range conflicts {
		if c.IsDecided() {
			decided = append(decided, c)
		}
	}

	lines = append(lines, "## DECIDED conflicts (resolutions on record)")
	lines = append(lines, "")
	if len(decided) == 0 {
		lines = append(lines, "_None._")
		lines = append(lines, "")
	} else {
		for _, c := range decided {
			rationale := extractDecidedRationale(c.Lifecycle)
			lines = append(lines, "### `"+c.ID+"` — axis `"+c.Axis+"`")
			lines = append(lines, "")
			lines = append(lines, "- **context:** "+c.Context)
			lines = append(lines, "- **members:** "+backtickedList(c.Members))
			lines = append(lines, "- **resolver:** `"+c.Resolver+"`")
			lines = append(lines, "- **rationale:** "+rationale)
			if c.SharedAssumption != nil && *c.SharedAssumption != "" {
				lines = append(lines, "- **shared assumption:** `"+*c.SharedAssumption+"`")
			}
			if len(c.Derived) > 0 {
				lines = append(lines, "- **spawned (derived):** "+backtickedList(c.Derived))
			}
			if c.RevisitMarker != "" {
				lines = append(lines, "- **revisit when:** "+c.RevisitMarker)
			}
			lines = append(lines, "")
		}
	}

	var parked []ontology.Conflict
	for _, c := range conflicts {
		if strings.HasPrefix(c.Lifecycle, ontology.ConflictREVISITPrefix) {
			parked = append(parked, c)
		}
	}

	lines = append(lines, "## Parked decisions (REVISIT_WHEN)")
	lines = append(lines, "")
	if len(parked) == 0 {
		lines = append(lines, "_None._")
		lines = append(lines, "")
	} else {
		for _, c := range parked {
			condition := extractRevisitRationale(c.Lifecycle)
			lines = append(lines, "### `"+c.ID+"` — axis `"+c.Axis+"`")
			lines = append(lines, "")
			lines = append(lines, "- **context:** "+c.Context)
			lines = append(lines, "- **members:** "+backtickedList(c.Members))
			lines = append(lines, "- **resolver:** `"+c.Resolver+"`")
			lines = append(lines, "- **condition:** "+condition)
			if c.SharedAssumption != nil && *c.SharedAssumption != "" {
				lines = append(lines, "- **shared assumption:** `"+*c.SharedAssumption+"`")
			}
			if len(c.Derived) > 0 {
				lines = append(lines, "- **spawned (derived):** "+backtickedList(c.Derived))
			}
			lines = append(lines, "")
		}
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}
