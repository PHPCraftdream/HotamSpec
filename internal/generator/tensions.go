package generator

import (
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/localization"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

type latentSuspect struct {
	Left  string
	Right string
	Hint  string
}

const genericAssumptionThreshold = 8

// TensionsMDHasContent reports whether TENSIONS.md carries real domain content
// — i.e. whether the graph has at least one Conflict node OR at least one Axis
// (the two structural elements TENSIONS.md exists to display). When false,
// genSpec withholds TENSIONS.md from the docs/gen/ write set entirely (the
// same conditional-write pattern DECISIONS.md/ENTITIES.md already use),
// because a young domain with no conflicts and no axes has nothing to show
// but the "no conflict nodes yet" template — pure noise for a young domain.
// The latent-connector suspicions heuristic (which CAN produce output from
// requirements alone, without conflicts/axes) is deliberately NOT the gate
// here: it is a secondary advisory section, and a domain with zero conflicts
// and zero axes has no tension architecture to document regardless.
func TensionsMDHasContent(g *ontology.Graph) bool {
	return len(g.Conflicts) > 0 || len(g.Axes) > 0
}

func BuildTensions(g *ontology.Graph) string {
	conflicts := NarrativeOrder(g.Conflicts, func(c ontology.Conflict) int { return c.DeclOrder })
	axes := NarrativeOrder(g.Axes, func(a ontology.Axis) int { return a.DeclOrder })
	lines := []string{localizedBanner(g), ReaderHeaderLine("TENSIONS", g), ""}
	lines = append(lines, serviceText(g, "# TENSIONS.md — The tension map (Hotam-Spec)"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "Generated from the active domain's `graph.json` (the requirement store). Conflicts are secondary bookkeeping around the executable-requirement core: a **Conflict** is a first-class connector NODE — `R-a -> C <- R-b` — carrying the tension axis, the colliding context, and the shared assumption that belong to neither requirement. Conflicts CLUSTER by axis: a cluster of size > 1 is one unresolved architectural choice, not N local disputes."))
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	if g.IsEmpty() {
		lines = append(lines, localizedEmptyNotice(g))
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	clusters := conflictsByAxis(conflicts)
	lines = append(lines, serviceText(g, "## Clusters by axis"))
	lines = append(lines, "")
	if len(clusters) == 0 {
		lines = append(lines, serviceText(g, "_No conflict nodes yet._"))
		lines = append(lines, "")
	}
	for _, cl := range clusters {
		cons := cl.conflicts
		kind := serviceText(g, "single tension")
		if len(cons) > 1 {
			kind = serviceText(g, "ARCHITECTURAL CHOICE (cluster)")
		}
		lines = append(lines, serviceText(g, "### Axis `%s` — %d conflict(s), %s", cl.axis, len(cons), kind))
		lines = append(lines, "")
		for _, c := range cons {
			lines = append(lines, conflictBlockForLanguage(g.RenderLanguage, c)...)
		}
	}

	lines = append(lines, serviceText(g, "## Hotam-Specn map (Mermaid)"))
	lines = append(lines, "")
	if len(conflicts) > 0 {
		lines = append(lines, Mermaid(conflicts)...)
	} else {
		lines = append(lines, serviceText(g, "_No conflict nodes to render._"))
	}
	lines = append(lines, "")

	lines = append(lines, serviceText(g, "## Controlled vocabulary of axes (this domain)"))
	lines = append(lines, "")
	if len(axes) == 0 {
		lines = append(lines, serviceText(g, "_No axes declared in this domain yet._"))
		lines = append(lines, "")
	} else {
		lines = append(lines, serviceText(g, "| axis slug | description |"))
		lines = append(lines, "|---|---|")
		for _, ax := range axes {
			lines = append(lines, "| `"+ax.Slug+"` | "+Cell(ax.Description)+" |")
		}
		lines = append(lines, "")
	}

	suspects := latentConnectorSuspects(g)
	lines = append(lines, serviceText(g, "## Latent-connector suspicions (heuristic, for AI review)"))
	lines = append(lines, "")
	lines = append(lines, serviceText(g, "Requirement pairs that SHOULD perhaps have a connector node but do not. This is a heuristic stub for the deferred detector — a suspicion to judge, never an auto-materialized conflict."))
	lines = append(lines, "")
	if len(suspects) == 0 {
		lines = append(lines, serviceText(g, "_None flagged._"))
		lines = append(lines, "")
	} else {
		lines = append(lines, serviceText(g, "| left | right | hint |"))
		lines = append(lines, "|---|---|---|")
		for _, s := range suspects {
			lines = append(lines, "| `"+s.Left+"` | `"+s.Right+"` | "+Cell(s.Hint)+" |")
		}
		lines = append(lines, "")
	}

	return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
}

type axisCluster struct {
	axis      string
	conflicts []ontology.Conflict
}

func conflictsByAxis(conflicts []ontology.Conflict) []axisCluster {
	var order []string
	groups := map[string][]ontology.Conflict{}
	for _, c := range conflicts {
		if _, ok := groups[c.Axis]; !ok {
			order = append(order, c.Axis)
		}
		groups[c.Axis] = append(groups[c.Axis], c)
	}
	out := make([]axisCluster, 0, len(order))
	for _, axis := range order {
		out = append(out, axisCluster{axis: axis, conflicts: groups[axis]})
	}
	return out
}

func ConflictBlock(c ontology.Conflict) []string {
	return conflictBlockForLanguage("", c)
}

func conflictBlockForLanguage(language string, c ontology.Conflict) []string {
	lines := []string{
		localization.Text(language, "#### `%s` — %s", c.ID, c.Axis),
		"",
		localization.Text(language, "- **context:** %s", c.Context),
	}
	members := make([]string, len(c.Members))
	for i, member := range c.Members {
		members[i] = "`" + member + "`"
	}
	lines = append(lines, localization.Text(language, "- **members:** %s", strings.Join(members, ", ")))
	lines = append(lines, localization.Text(language, "- **resolver:** `%s`", c.Resolver))
	lines = append(lines, localization.Text(language, "- **lifecycle:** %s", c.Lifecycle))
	if c.SharedAssumption != nil && *c.SharedAssumption != "" {
		lines = append(lines, localization.Text(language, "- **shared assumption:** `%s`", *c.SharedAssumption))
	}
	if len(c.Derived) > 0 {
		derived := make([]string, len(c.Derived))
		for i, id := range c.Derived {
			derived[i] = "`" + id + "`"
		}
		lines = append(lines, localization.Text(language, "- **spawned (lineage):** %s", strings.Join(derived, ", ")))
	}
	if c.RevisitMarker != "" {
		lines = append(lines, localization.Text(language, "- **revisit marker:** %s", c.RevisitMarker))
	}
	if len(c.Variants) > 0 {
		lines = append(lines, localization.Text(language, "- **variants** (resolver chooses one):"))
		for _, variant := range c.Variants {
			lines = append(lines, "  - `"+variant.ID+"`")
			lines = append(lines, localization.Text(language, "    - behavior: %s", variant.Behavior))
			lines = append(lines, localization.Text(language, "    - implies: %s", variant.Implies))
			lines = append(lines, localization.Text(language, "    - costs: %s", variant.Costs))
		}
	}
	lines = append(lines, "")
	return lines
}

func Mermaid(conflicts []ontology.Conflict) []string {
	lines := []string{"```mermaid", "graph TD"}
	referenced := []string{}
	seen := map[string]struct{}{}
	for _, c := range conflicts {
		ids := append(append([]string{}, c.Members...), c.Derived...)
		for _, rid := range ids {
			if _, ok := seen[rid]; !ok {
				seen[rid] = struct{}{}
				referenced = append(referenced, rid)
			}
		}
	}
	for _, rid := range referenced {
		lines = append(lines, "    "+MermaidID(rid)+"[\""+rid+"\"]")
	}
	for _, c := range conflicts {
		cid := MermaidID(c.ID)
		lines = append(lines, "    "+cid+"{\""+c.ID+"\\n"+c.Axis+"\"}")
		for _, m := range c.Members {
			lines = append(lines, "    "+MermaidID(m)+" --> "+cid)
		}
		for _, d := range c.Derived {
			lines = append(lines, "    "+cid+" -.spawns.-> "+MermaidID(d))
		}
	}
	lines = append(lines, "```")
	return lines
}

func latentConnectorSuspects(g *ontology.Graph) []latentSuspect {
	already := map[string]struct{}{}
	for _, c := range g.Conflicts {
		ms := c.Members
		for i := 0; i < len(ms); i++ {
			for j := i + 1; j < len(ms); j++ {
				a, b := ms[i], ms[j]
				key := a + "\x00" + b
				if a > b {
					key = b + "\x00" + a
				}
				already[key] = struct{}{}
			}
		}
	}
	refCounts := map[string]int{}
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusREJECTED {
			continue
		}
		for _, aID := range r.Assumptions {
			refCounts[aID]++
		}
	}
	var reqs []ontology.Requirement
	for _, r := range g.Requirements {
		if r.Status != ontology.StatusREJECTED {
			reqs = append(reqs, r)
		}
	}
	type record struct {
		minCount  int
		signature []string
		left      string
		right     string
	}
	var records []record
	for i := 0; i < len(reqs); i++ {
		for j := i + 1; j < len(reqs); j++ {
			a, b := reqs[i], reqs[j]
			aSet := map[string]struct{}{}
			for _, x := range a.Assumptions {
				aSet[x] = struct{}{}
			}
			var shared []string
			seenShared := map[string]struct{}{}
			for _, x := range b.Assumptions {
				if _, ok := aSet[x]; ok {
					if _, dup := seenShared[x]; !dup {
						seenShared[x] = struct{}{}
						shared = append(shared, x)
					}
				}
			}
			if len(shared) == 0 {
				continue
			}
			var specific []string
			for _, aID := range shared {
				if refCounts[aID] < genericAssumptionThreshold {
					specific = append(specific, aID)
				}
			}
			if len(specific) == 0 {
				continue
			}
			key := a.ID + "\x00" + b.ID
			if a.ID > b.ID {
				key = b.ID + "\x00" + a.ID
			}
			if _, ok := already[key]; ok {
				continue
			}
			left, right := a.ID, b.ID
			if left > right {
				left, right = right, left
			}
			minCount := refCounts[specific[0]]
			for _, aID := range specific {
				if refCounts[aID] < minCount {
					minCount = refCounts[aID]
				}
			}
			sig := append([]string{}, specific...)
			sort.Strings(sig)
			records = append(records, record{minCount: minCount, signature: sig, left: left, right: right})
		}
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].minCount != records[j].minCount {
			return records[i].minCount < records[j].minCount
		}
		if records[i].left != records[j].left {
			return records[i].left < records[j].left
		}
		return records[i].right < records[j].right
	})
	out := make([]latentSuspect, 0, len(records))
	for _, rec := range records {
		out = append(out, latentSuspect{Left: rec.left, Right: rec.right, Hint: "shares assumption(s): " + strings.Join(rec.signature, ", ")})
	}
	return out
}
