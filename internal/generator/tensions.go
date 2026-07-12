package generator

import (
	"sort"
	"strconv"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

type latentSuspect struct {
	Left  string
	Right string
	Hint  string
}

const genericAssumptionThreshold = 8

func BuildTensions(g *ontology.Graph) string {
	conflicts := NarrativeOrder(g.Conflicts, func(c ontology.Conflict) int { return c.DeclOrder })
	axes := NarrativeOrder(g.Axes, func(a ontology.Axis) int { return a.DeclOrder })
	lines := []string{Banner, ReaderHeaderLine("TENSIONS", g), ""}
	lines = append(lines, "# TENSIONS.md — The tension map (Hotam-Spec)")
	lines = append(lines, "")
	lines = append(lines, "Generated from the active domain's `graph.py` (the tension graph). A **Conflict** is a first-class connector NODE — `R-a -> C <- R-b` — carrying the tension axis, the colliding context, and the shared assumption that belong to neither requirement. Conflicts CLUSTER by axis: a cluster of size > 1 is one unresolved architectural choice, not N local disputes.")
	lines = append(lines, "")
	lines = append(lines, "---")
	lines = append(lines, "")

	if g.IsEmpty() {
		lines = append(lines, EmptyNotice)
		lines = append(lines, "")
		return strings.TrimRight(strings.Join(lines, "\n"), " \t\r\n") + "\n"
	}

	clusters := conflictsByAxis(conflicts)
	lines = append(lines, "## Clusters by axis")
	lines = append(lines, "")
	if len(clusters) == 0 {
		lines = append(lines, "_No conflict nodes yet._")
		lines = append(lines, "")
	}
	for _, cl := range clusters {
		cons := cl.conflicts
		kind := "single tension"
		if len(cons) > 1 {
			kind = "ARCHITECTURAL CHOICE (cluster)"
		}
		lines = append(lines, "### Axis `"+cl.axis+"` — "+strconv.Itoa(len(cons))+" conflict(s), "+kind)
		lines = append(lines, "")
		for _, c := range cons {
			lines = append(lines, ConflictBlock(c)...)
		}
	}

	lines = append(lines, "## Hotam-Specn map (Mermaid)")
	lines = append(lines, "")
	if len(conflicts) > 0 {
		lines = append(lines, Mermaid(conflicts)...)
	} else {
		lines = append(lines, "_No conflict nodes to render._")
	}
	lines = append(lines, "")

	lines = append(lines, "## Controlled vocabulary of axes (this domain)")
	lines = append(lines, "")
	if len(axes) == 0 {
		lines = append(lines, "_No axes declared in this domain yet._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| axis slug | description |")
		lines = append(lines, "|---|---|")
		for _, ax := range axes {
			lines = append(lines, "| `"+ax.Slug+"` | "+Cell(ax.Description)+" |")
		}
		lines = append(lines, "")
	}

	suspects := latentConnectorSuspects(g)
	lines = append(lines, "## Latent-connector suspicions (heuristic, for AI review)")
	lines = append(lines, "")
	lines = append(lines, "Requirement pairs that SHOULD perhaps have a connector node but do not. This is a heuristic stub for the deferred detector — a suspicion to judge, never an auto-materialized conflict.")
	lines = append(lines, "")
	if len(suspects) == 0 {
		lines = append(lines, "_None flagged._")
		lines = append(lines, "")
	} else {
		lines = append(lines, "| left | right | hint |")
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
	lines := []string{}
	lines = append(lines, "#### `"+c.ID+"` — "+c.Axis)
	lines = append(lines, "")
	lines = append(lines, "- **context:** "+c.Context)
	members := make([]string, len(c.Members))
	for i, m := range c.Members {
		members[i] = "`" + m + "`"
	}
	lines = append(lines, "- **members:** "+strings.Join(members, ", "))
	lines = append(lines, "- **steward:** `"+c.Steward+"`")
	lines = append(lines, "- **lifecycle:** "+c.Lifecycle)
	if c.SharedAssumption != nil && *c.SharedAssumption != "" {
		lines = append(lines, "- **shared assumption:** `"+*c.SharedAssumption+"`")
	}
	if len(c.Derived) > 0 {
		derived := make([]string, len(c.Derived))
		for i, d := range c.Derived {
			derived[i] = "`" + d + "`"
		}
		lines = append(lines, "- **spawned (lineage):** "+strings.Join(derived, ", "))
	}
	if c.RevisitMarker != "" {
		lines = append(lines, "- **revisit marker:** "+c.RevisitMarker)
	}
	if len(c.Variants) > 0 {
		lines = append(lines, "- **variants** (steward chooses one):")
		for _, v := range c.Variants {
			lines = append(lines, "  - `"+v.ID+"`")
			lines = append(lines, "    - behavior: "+v.Behavior)
			lines = append(lines, "    - implies: "+v.Implies)
			lines = append(lines, "    - costs: "+v.Costs)
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
