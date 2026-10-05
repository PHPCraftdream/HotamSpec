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

// HistoryMDHasContent reports whether HISTORY.md carries real domain content
// — i.e. whether the graph is non-empty (task #364: withheld entirely from
// the docs/gen/ write set when the domain has zero axes/stakeholders/
// requirements/conflicts/assumptions/operators/processes/goals/entity_types/
// entities, mirroring the conditional-write pattern DECISIONS.md/
// ENTITIES.md/TENSIONS.md/PIPELINE.md/MODELS.md already use). A genuinely
// empty domain has nothing to render but the EmptyNotice placeholder
// BuildHistory already falls back to (g.IsEmpty()) — so a fresh, unmodeled
// domain now produces ZERO files under docs/gen/, not a directory full of
// calm-but-empty placeholders.
func HistoryMDHasContent(g *ontology.Graph) bool {
	return !g.IsEmpty()
}

func BuildHistory(g *ontology.Graph) string {
	reqs := NarrativeOrder(g.Requirements, func(r ontology.Requirement) int { return r.DeclOrder })
	conflicts := NarrativeOrder(g.Conflicts, func(c ontology.Conflict) int { return c.DeclOrder })
	lines := docHeaderLines("HISTORY", g)
	lines = append(lines, serviceText(g, "# HISTORY.md — Methodology decision history (Hotam-Spec)"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "Generated from the anti-relitigation markers in the model: REJECTED\nrequirements (what was tried and discarded — REPLACES marker) and DECIDED /\nREVISIT_WHEN conflict lifecycles (what was resolved, why, and the condition\nunder which to re-open). Source of truth is the active domain's `graph.json`;\nthis text is generated so it cannot drift."))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "A fresh agent reads this to recover the methodology's history without\nre-litigating settled questions — the historian role of the AI made into\nsubstrate (R-history-from-rejected-markers)."))
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	if g.IsEmpty() {
		lines = append(lines, localizedEmptyNotice(g))
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	var rejected []ontology.Requirement
	for _, r := range reqs {
		if r.Status == ontology.StatusREJECTED {
			rejected = append(rejected, r)
		}
	}

	lines = append(lines, serviceText(g, "## REJECTED requirements (what we tried and discarded)"))
	lines = append(lines, "")
	if len(rejected) == 0 {
		lines = append(lines, serviceText(g, "_None._"))
		lines = append(lines, "")
	} else {
		for _, r := range rejected {
			lines = append(lines, "### `"+r.ID+"` — "+Cell(requirementClaim(g, r)))
			lines = append(lines, "")
			lines = append(lines, "- "+serviceText(g, "**owner:**")+" `"+r.Owner+"`")
			lines = append(lines, "- "+serviceText(g, "**why:**")+" "+r.Why)
			lines = append(lines, "")
		}
	}

	var decided []ontology.Conflict
	for _, c := range conflicts {
		if c.IsDecided() {
			decided = append(decided, c)
		}
	}

	lines = append(lines, serviceText(g, "## DECIDED conflicts (resolutions on record)"))
	lines = append(lines, "")
	if len(decided) == 0 {
		lines = append(lines, serviceText(g, "_None._"))
		lines = append(lines, "")
	} else {
		for _, c := range decided {
			rationale := extractDecidedRationale(c.Lifecycle)
			lines = append(lines, "### `"+c.ID+"` — "+serviceText(g, "axis")+" `"+c.Axis+"`")
			lines = append(lines, "")
			lines = append(lines, "- "+serviceText(g, "**context:**")+" "+c.Context)
			lines = append(lines, "- "+serviceText(g, "**members:**")+" "+backtickedList(c.Members))
			lines = append(lines, "- "+serviceText(g, "**resolver:**")+" `"+c.Resolver+"`")
			lines = append(lines, "- "+serviceText(g, "**rationale:**")+" "+rationale)
			if c.SharedAssumption != nil && *c.SharedAssumption != "" {
				lines = append(lines, "- "+serviceText(g, "**shared assumption:**")+" `"+*c.SharedAssumption+"`")
			}
			if len(c.Derived) > 0 {
				lines = append(lines, "- "+serviceText(g, "**spawned (derived):**")+" "+backtickedList(c.Derived))
			}
			if c.RevisitMarker != "" {
				lines = append(lines, "- "+serviceText(g, "**revisit when:**")+" "+c.RevisitMarker)
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

	lines = append(lines, serviceText(g, "## Parked decisions (REVISIT_WHEN)"))
	lines = append(lines, "")
	if len(parked) == 0 {
		lines = append(lines, serviceText(g, "_None._"))
		lines = append(lines, "")
	} else {
		for _, c := range parked {
			condition := extractRevisitRationale(c.Lifecycle)
			lines = append(lines, "### `"+c.ID+"` — "+serviceText(g, "axis")+" `"+c.Axis+"`")
			lines = append(lines, "")
			lines = append(lines, "- "+serviceText(g, "**context:**")+" "+c.Context)
			lines = append(lines, "- "+serviceText(g, "**members:**")+" "+backtickedList(c.Members))
			lines = append(lines, "- "+serviceText(g, "**resolver:**")+" `"+c.Resolver+"`")
			lines = append(lines, "- "+serviceText(g, "**condition:**")+" "+condition)
			if c.SharedAssumption != nil && *c.SharedAssumption != "" {
				lines = append(lines, "- "+serviceText(g, "**shared assumption:**")+" `"+*c.SharedAssumption+"`")
			}
			if len(c.Derived) > 0 {
				lines = append(lines, "- "+serviceText(g, "**spawned (derived):**")+" "+backtickedList(c.Derived))
			}
			lines = append(lines, "")
		}
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}
