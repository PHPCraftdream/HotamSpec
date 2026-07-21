package diagnose

import (
	"fmt"
	"regexp"
	"sort"
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
	// P0 counts ONLY closeable-now debt: a real test could be written for it
	// TODAY if someone did the work. Feature-blocked debt (the described
	// feature does not exist yet) is NOT actionable debt — it is surfaced
	// separately as a lower-priority Advisory by ReflectFeatureBlockedDebt so
	// the P0 signal reflects genuine urgency, not frozen roadmap.
	var nCloseableNow int
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED && r.IsCloseableDebtNow() {
			nCloseableNow++
		}
	}
	if nCloseableNow > 5 {
		return []Finding{{
			Condition: "reflect_unenforced_settled",
			Target:    "enforcement-gradient",
			Imperative: fmt.Sprintf(
				"%d SETTLED requirements are closeable now (ENFORCEABLE, no feature blocker, still PROSE/STRUCTURAL) — claimed but not guaranteed, soft context-debt. See docs/gen/UNENFORCED.md.",
				nCloseableNow,
			),
		}}
	}
	return nil
}

// ReflectFeatureBlockedDebt surfaces feature-blocked closeable debt as a
// lower-priority Advisory (P7): these ENFORCEABLE requirements stay PROSE
// because the feature they describe does not exist yet, so no honest test is
// possible until the blocking feature is built (itself frozen by
// R-speculative-aspects-frozen). This is informational roadmap visibility, not
// a P0 actionability trigger — it fires whenever the count is nonzero.
func ReflectFeatureBlockedDebt(g *ontology.Graph) []Finding {
	var nFeatureBlocked int
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED && r.IsFeatureBlockedDebt() {
			nFeatureBlocked++
		}
	}
	if nFeatureBlocked == 0 {
		return nil
	}
	return []Finding{{
		Condition: "reflect_feature_blocked_debt",
		Target:    "feature-blocked-roadmap",
		Advisory:  true,
		Imperative: fmt.Sprintf(
			"%d SETTLED requirements are feature-blocked debt (ENFORCEABLE, but the described feature does not exist yet — correctly PROSE, frozen by R-speculative-aspects-frozen). Honest roadmap, not neglected. See docs/reviews/2026-07-13-c1-roadmap-debt-triage.md.",
			nFeatureBlocked,
		),
	}}
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
					"Conflict '%s' is live (%s) but ALL its members are REJECTED (%s). The tension's parties are gone; DECIDE it (mark exhausted) or REVISIT_WHEN (park) so the graph stops holding a ghost connector. Advisory; never a gate (the resolver closes a conflict, never the harness — R-decided-needs-human-signoff).",
					c.ID, c.Lifecycle, pyListRepr(c.Members),
				),
				Advisory: true,
			})
		}
	}
	return out
}

// ReflectOrphanEntityType surfaces an advisory (never blocking) signal for
// each EntityType that no Process in the domain drives (i.e. its slug never
// appears in any Process.DrivesEntities). It fires ONLY when the domain has
// declared at least one Process node — a domain with zero Process nodes has
// not opted into the behavioral aspect at all (§Process, M12), so "which
// EntityType is orphaned from a Process" is not a meaningful question there
// and the signal stays silent (no Process aspect => no drives_entities
// wiring to be orphaned from, by construction). This mirrors the existing
// no-ops-when-aspect-absent convention already used across the Process/Goal
// invariants (e.g. check_process_drives_existing_entities,
// check_goal_target_kind_known): silence on an absent aspect is honest, not
// a gap.
//
// This is deliberately a diagnose-layer Finding, NOT an invariants.Violation:
// invariants.AllViolations gates apply-proposal (internal/proposal/apply.go
// refuses to land a proposal that introduces a NEW violation) and hotam
// all-violations' exit code. An EntityType landed before the Process step
// that wires it in (a normal, sequential multi-proposal land) must not be
// treated as a hard error — it is a "not wired in yet" signal for the
// resolver's attention, not a structural break. Advisory: true routes it to
// PAdvisory in DiagnoseSignals (what-now's lowest-priority band), never to
// PStructure.
func ReflectOrphanEntityType(g *ontology.Graph) []Finding {
	if len(g.Processes) == 0 {
		return nil
	}
	driven := map[string]struct{}{}
	for _, p := range g.Processes {
		for _, slug := range p.DrivesEntities {
			driven[slug] = struct{}{}
		}
	}
	var orphanSlugs []string
	for _, et := range g.EntityTypes {
		if _, ok := driven[et.Slug]; !ok {
			orphanSlugs = append(orphanSlugs, et.Slug)
		}
	}
	if len(orphanSlugs) == 0 {
		return nil
	}
	sort.Strings(orphanSlugs)
	out := make([]Finding, 0, len(orphanSlugs))
	for _, slug := range orphanSlugs {
		out = append(out, Finding{
			Condition: "reflect_orphan_entity_type",
			Target:    slug,
			Advisory:  true,
			Imperative: fmt.Sprintf(
				"orphan detail: EntityType '%s' is not driven by any Process.drives_entities in this domain — wire it into a Process (or confirm it is deliberately free-standing). Advisory; never a gate (skeleton-first: artifacts should follow the Process wave that drives them).",
				slug,
			),
		})
	}
	return out
}

func AllFindings(g *ontology.Graph) []Finding {
	var out []Finding
	out = append(out, ReflectDraftOverhang(g)...)
	out = append(out, ReflectUnenforcedSettled(g)...)
	out = append(out, ReflectFeatureBlockedDebt(g)...)
	out = append(out, ReflectOverBudgetOperators(g)...)
	out = append(out, ReflectDeadAssumptionOnEnforcer(g)...)
	out = append(out, ReflectDerivedButUnbuilt(g)...)
	out = append(out, ReflectImplementsDecay(g)...)
	out = append(out, ReflectReplacesEdgeMigration(g)...)
	out = append(out, ReflectAllMembersRejected(g)...)
	out = append(out, ReflectOrphanEntityType(g)...)
	return out
}
