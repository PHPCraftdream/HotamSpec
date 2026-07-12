package diagnose

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const ImplementsDecayDays = 14

type Finding struct {
	Condition  string
	Target     string
	Imperative string
	Advisory   bool
}

func graphSize(g *ontology.Graph) int {
	return len(g.Requirements) + len(g.Conflicts) + len(g.Assumptions)
}

func ReflectDraftOverhang(g *ontology.Graph) []Finding {
	var settledN, draftN int
	for _, r := range g.Requirements {
		switch r.Status {
		case ontology.StatusSETTLED:
			settledN++
		case ontology.StatusDRAFT:
			draftN++
		}
	}
	if settledN > 0 && float64(draftN) >= float64(settledN)/2 {
		return []Finding{{
			Condition: "reflect_draft_overhang",
			Target:    "burn-down",
			Imperative: fmt.Sprintf(
				"DRAFT-overhang: %d DRAFT vs %d SETTLED — promote DRAFTs toward ENFORCED before crystallizing more (R-crystallize-before-split, C-06e2d84e).",
				draftN, settledN,
			),
		}}
	}
	return nil
}

func ReflectUnenforcedSettled(g *ontology.Graph) []Finding {
	var nUnenforced int
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED && r.IsCloseableDebt() {
			nUnenforced++
		}
	}
	if nUnenforced > 5 {
		return []Finding{{
			Condition: "reflect_unenforced_settled",
			Target:    "enforcement-gradient",
			Imperative: fmt.Sprintf(
				"%d SETTLED requirements are closeable debt (ENFORCEABLE, still PROSE/STRUCTURAL) — claimed but not guaranteed, soft context-debt. See docs/gen/UNENFORCED.md.",
				nUnenforced,
			),
		}}
	}
	return nil
}

func ReflectOverBudgetOperators(g *ontology.Graph) []Finding {
	nodeSize := graphSize(g)
	var out []Finding
	for _, op := range g.Operators {
		limit := op.ContextBudget.Limit
		if limit <= 0 {
			continue
		}
		if op.ContextBudget.Measure == ontology.BudgetMeasureCRYSTAL_CHARS {
			continue
		}
		size := nodeSize
		unit := "nodes (NODE_COUNT measure)"
		if size > limit {
			out = append(out, Finding{
				Condition: "reflect_over_budget_operators",
				Target:    op.ID,
				Imperative: fmt.Sprintf(
					"Operator '%s' holds %d %s > budget %d; crystallize first (R-crystallize-before-split); if still over, delegate a sub-domain (R-context-bounded-delegation).",
					op.ID, size, unit, limit,
				),
			})
		}
	}
	return out
}

func ReflectDeadAssumptionOnEnforcer(g *ontology.Graph) []Finding {
	deadIDs := map[string]struct{}{}
	for _, a := range g.Assumptions {
		if a.Status == ontology.AssumptionDEAD {
			deadIDs[a.ID] = struct{}{}
		}
	}
	var out []Finding
	if len(deadIDs) == 0 {
		return out
	}
	for _, r := range g.Requirements {
		if r.Enforcement != ontology.EnforcementENFORCED {
			continue
		}
		for _, aid := range r.Assumptions {
			if _, ok := deadIDs[aid]; ok {
				out = append(out, Finding{
					Condition: "reflect_dead_assumption_on_enforcer",
					Target:    r.ID,
					Imperative: fmt.Sprintf(
						"R-stale-substrate signal: enforced requirement '%s' rests on DEAD assumption '%s'; its enforcer may be enforcing a now-wrong premise.",
						r.ID, aid,
					),
				})
			}
		}
	}
	return out
}

func ReflectDerivedButUnbuilt(g *ontology.Graph) []Finding {
	idx := ontology.BuildIndex(g)
	draftIDs := map[string]struct{}{}
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusDRAFT {
			draftIDs[r.ID] = struct{}{}
		}
	}
	var out []Finding
	for _, c := range g.Conflicts {
		if !strings.HasPrefix(c.Lifecycle, ontology.ConflictDECIDEDPrefix) {
			continue
		}
		for _, derivedID := range c.Derived {
			derivedReq, found := idx.RequirementByID[derivedID]
			fire := !found
			if found {
				if _, isDraft := draftIDs[derivedReq.ID]; isDraft {
					fire = true
				}
			}
			if fire {
				out = append(out, Finding{
					Condition: "reflect_derived_but_unbuilt",
					Target:    derivedID,
					Imperative: fmt.Sprintf(
						"DECIDED conflict '%s' spawned '%s' but it remains DRAFT/unbuilt — derived-but-unbuilt debt.",
						c.ID, derivedID,
					),
				})
			}
		}
	}
	return out
}

func ReflectImplementsDecay(g *ontology.Graph) []Finding {
	now := time.Now()
	todayY, todayM, todayD := now.Date()
	today := time.Date(todayY, todayM, todayD, 0, 0, 0, 0, time.UTC)
	var out []Finding
	for _, a := range g.Assumptions {
		if a.Status != ontology.AssumptionIMPLEMENTS {
			continue
		}
		stampStr := a.DecidedAt
		if stampStr == "" {
			stampStr = a.CreatedAt
		}
		if stampStr == "" {
			continue
		}
		stamp, err := time.Parse("2006-01-02", stampStr)
		if err != nil {
			continue
		}
		stampY, stampM, stampD := stamp.Date()
		stampDate := time.Date(stampY, stampM, stampD, 0, 0, 0, 0, time.UTC)
		ageDays := int(today.Sub(stampDate).Hours() / 24)
		if ageDays > ImplementsDecayDays {
			out = append(out, Finding{
				Condition: "reflect_implements_decay",
				Target:    a.ID,
				Imperative: fmt.Sprintf(
					"IMPLEMENTS aspiration '%s' is %d days old (last stamped %s) without re-affirmation — re-affirm (transition to HOLDS if achieved) or downgrade (to DEAD if abandoned). An aspiration that ages silently is the invisible corner IMPLEMENTS created (Ontology K2(c)).",
					a.ID, ageDays, stampStr,
				),
			})
		}
	}
	return out
}

var replacesProseRE = regexp.MustCompile(`REJECTED\s*(?:—|–|--|-)\s*REPLACES`)

func ReflectReplacesEdgeMigration(g *ontology.Graph) []Finding {
	rmap := ontology.ReplacesMap(g)
	var out []Finding
	for _, r := range g.Requirements {
		if r.Status != ontology.StatusREJECTED {
			continue
		}
		if _, ok := rmap[r.ID]; ok {
			continue
		}
		if !replacesProseRE.MatchString(r.Why) {
			continue
		}
		out = append(out, Finding{
			Condition: "reflect_replaces_edge_migration",
			Target:    r.ID,
			Imperative: fmt.Sprintf(
				"REJECTED requirement '%s' claims a REPLACES successor in prose but has NO structural `replaces` edge — migrate it via a ProposedRejection (with replaced_by) so the anti-relitigation relation becomes machine-traversable (R-rejected-preserved-not-deleted). Advisory; never a gate.",
				r.ID,
			),
			Advisory: true,
		})
	}
	return out
}

func ReflectAllMembersRejected(g *ontology.Graph) []Finding {
	idx := ontology.BuildIndex(g)
	var out []Finding
	for _, c := range g.Conflicts {
		if strings.HasPrefix(c.Lifecycle, ontology.ConflictDECIDEDPrefix) {
			continue
		}
		if strings.HasPrefix(c.Lifecycle, ontology.ConflictREVISITPrefix) {
			continue
		}
		if len(c.Members) < 1 {
			continue
		}
		allRejected := true
		for _, mid := range c.Members {
			r, ok := idx.RequirementByID[mid]
			if !ok || r.Status != ontology.StatusREJECTED {
				allRejected = false
				break
			}
		}
		if allRejected {
			out = append(out, Finding{
				Condition: "reflect_all_members_rejected",
				Target:    c.ID,
				Imperative: fmt.Sprintf(
					"Conflict '%s' is live (%s) but ALL its members are REJECTED (%s). The tension's parties are gone; DECIDE it (mark exhausted) or REVISIT_WHEN (park) so the graph stops holding a ghost connector. Advisory; never a gate (the steward closes a conflict, never the harness — R-decided-needs-human-signoff).",
					c.ID, c.Lifecycle, pyListRepr(c.Members),
				),
				Advisory: true,
			})
		}
	}
	return out
}

func AllFindings(g *ontology.Graph) []Finding {
	var out []Finding
	out = append(out, ReflectDraftOverhang(g)...)
	out = append(out, ReflectUnenforcedSettled(g)...)
	out = append(out, ReflectOverBudgetOperators(g)...)
	out = append(out, ReflectDeadAssumptionOnEnforcer(g)...)
	out = append(out, ReflectDerivedButUnbuilt(g)...)
	out = append(out, ReflectImplementsDecay(g)...)
	out = append(out, ReflectReplacesEdgeMigration(g)...)
	out = append(out, ReflectAllMembersRejected(g)...)
	return out
}
