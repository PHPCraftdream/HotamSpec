package invariants

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/methodology"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func checkTypedAnchorsVariant(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		for _, v := range c.Variants {
			if !strings.HasPrefix(v.ID, "V-") {
				out = append(out, Violation{
					Check:   "check_typed_anchors_variant",
					ID:      fmt.Sprintf("%s:%s", c.ID, v.ID),
					Message: fmt.Sprintf("Variant id %q on conflict %q must start with 'V-' (typed-anchor rule, R-anchor-everything)", v.ID, c.ID),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_typed_anchors_variant", Invariant{
	Name:  "check_typed_anchors_variant",
	Canon: methodology.Invariants,
	Claim: "every Variant.id (on every Conflict) starts with 'V-'.",
	Rule: "for each Conflict, every Variant in its `variants` tuple MUST have an id starting " +
		"with 'V-'. An id with the wrong prefix breaks the typed-anchor discipline " +
		"(R-anchor-everything) for the new Variant payload type introduced alongside HELD.",
	Why: "Variant is not a graph node (anti-RDF, payload on Conflict), but it IS a typed anchor " +
		"a steward or agent may cite by reference (R-speak-by-reference) -- the same discipline " +
		"that governs R-/C-/A-/OP- ids applies to V- ids.",
	Check: checkTypedAnchorsVariant,
})

func checkSignoffChosenVariantResolves(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		if c.Signoff == nil {
			continue
		}
		cv := c.Signoff.ChosenVariant
		if cv == "" {
			continue
		}
		variantIDs := map[string]struct{}{}
		for _, v := range c.Variants {
			variantIDs[v.ID] = struct{}{}
		}
		if _, ok := variantIDs[cv]; !ok {
			sortedIDs := make([]string, 0, len(variantIDs))
			for id := range variantIDs {
				sortedIDs = append(sortedIDs, id)
			}
			sort.Strings(sortedIDs)
			none := "none"
			listing := strings.Join(sortedIDs, ", ")
			if listing == "" {
				listing = none
			}
			out = append(out, Violation{
				Check:   "check_signoff_chosen_variant_resolves",
				ID:      c.ID,
				Message: fmt.Sprintf("signoff.chosen_variant %q is not the id of any Variant on conflict %q (variants: %s)", cv, c.ID, listing),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_signoff_chosen_variant_resolves", Invariant{
	Name:  "check_signoff_chosen_variant_resolves",
	Canon: methodology.Signoff,
	Claim: "a signoff.chosen_variant (when non-empty) resolves to a Variant id on the conflict carrying the signoff.",
	Rule: "for each Conflict with a non-None signoff whose chosen_variant is non-empty, " +
		"chosen_variant MUST be the id of one of the conflict's variants. A chosen_variant " +
		"pointing at a variant that is NOT on the conflict (or at nothing) breaks the " +
		"anti-relitigation guarantee: the non-chosen variants' implies/costs survive the " +
		"decision precisely so the chosen one can be cited — an unresolvable chosen_variant " +
		"severs that citation.",
	Why: "this check is on the Conflict (not the Signoff alone): the chosen variant is " +
		"meaningful ONLY relative to the variants the steward was choosing BETWEEN, which live " +
		"on the Conflict. A Signoff detached from its conflict has no variants to resolve " +
		"against, so the check must walk conflict.signoff against conflict.variants together.",
	Check: checkSignoffChosenVariantResolves,
})

func checkDecidedConflictCarriesSignoff(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		if c.Signoff == nil {
			continue
		}
		if !(c.IsDecided() || c.IsHeld()) {
			continue
		}
		if c.Signoff.DecidedBy != c.DecidedBy {
			out = append(out, Violation{
				Check:   "check_decided_conflict_carries_signoff",
				ID:      c.ID,
				Message: fmt.Sprintf("signoff.decided_by %q disagrees with conflict %q decided_by %q — the provenance record and the conflict field must name the same human decider", c.Signoff.DecidedBy, c.ID, c.DecidedBy),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_decided_conflict_carries_signoff", Invariant{
	Name:  "check_decided_conflict_carries_signoff",
	Canon: methodology.Signoff,
	Claim: "a DECIDED/HELD conflict's signoff is consistent with its decided_by field (SOFT: pre-existing decisions without signoff are legitimate).",
	Rule: "this invariant does NOT require every DECIDED/HELD conflict to carry a signoff — " +
		"decisions taken before the §Signoff mechanism landed are legitimate and are NOT forced " +
		"to migrate. Instead it enforces CONSISTENCY: when a signoff IS present on a DECIDED/HELD " +
		"conflict, signoff.decided_by MUST equal the conflict's decided_by field. A mismatch " +
		"would mean the provenance record and the conflict's own decided_by disagree about WHO " +
		"decided — exactly the kind of silent drift R-trust-anchor-mechanism exists to prevent.",
	Why: "there are 8 pre-existing DECIDED conflicts in the live graph decided before this field " +
		"existed; demanding a signoff on each would manufacture false P1s that are not this " +
		"wave's to fix. The consistency check (when signoff IS present) is the honest boundary: " +
		"new decisions get a signoff via the writer, and any future edit that " +
		"inconsistentifies an existing signoff is caught.",
	Check: checkDecidedConflictCarriesSignoff,
})

func checkTypedAnchorsRequirement(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		if !strings.HasPrefix(r.ID, "R-") {
			out = append(out, Violation{
				Check:   "check_typed_anchors_requirement",
				ID:      r.ID,
				Message: fmt.Sprintf("Requirement id %q must start with 'R-' (typed-anchor rule, R-anchor-everything)", r.ID),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_typed_anchors_requirement", Invariant{
	Name:  "check_typed_anchors_requirement",
	Canon: methodology.Invariants,
	Claim: "every Requirement.id starts with 'R-'.",
	Rule: "Requirement.id MUST start with 'R-'. An id with the wrong prefix breaks the " +
		"typed-anchor discipline (R-anchor-everything) and makes cite-by-reference unreliable " +
		"(R-speak-by-reference).",
	Check: checkTypedAnchorsRequirement,
})

func checkTypedAnchorsAssumption(g *ontology.Graph) []Violation {
	var out []Violation
	for _, a := range g.Assumptions {
		if !strings.HasPrefix(a.ID, "A-") {
			out = append(out, Violation{
				Check:   "check_typed_anchors_assumption",
				ID:      a.ID,
				Message: fmt.Sprintf("Assumption id %q must start with 'A-' (typed-anchor rule, R-anchor-everything)", a.ID),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_typed_anchors_assumption", Invariant{
	Name:  "check_typed_anchors_assumption",
	Canon: methodology.Invariants,
	Claim: "every Assumption.id starts with 'A-'.",
	Rule: "Assumption.id MUST start with 'A-'. An id with the wrong prefix breaks the " +
		"typed-anchor discipline (R-anchor-everything).",
	Check: checkTypedAnchorsAssumption,
})

func checkTypedAnchorsConflict(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		if !strings.HasPrefix(c.ID, "C-") {
			out = append(out, Violation{
				Check:   "check_typed_anchors_conflict",
				ID:      c.ID,
				Message: fmt.Sprintf("Conflict id %q must start with 'C-' (typed-anchor rule, R-anchor-everything)", c.ID),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_typed_anchors_conflict", Invariant{
	Name:  "check_typed_anchors_conflict",
	Canon: methodology.Invariants,
	Claim: "every Conflict.id starts with 'C-'.",
	Rule: "Conflict.id MUST start with 'C-'. An id with the wrong prefix breaks the " +
		"typed-anchor discipline (R-anchor-everything).",
	Check: checkTypedAnchorsConflict,
})

func checkTypedAnchorsOperator(g *ontology.Graph) []Violation {
	var out []Violation
	for _, op := range g.Operators {
		if !strings.HasPrefix(op.ID, "OP-") {
			out = append(out, Violation{
				Check:   "check_typed_anchors_operator",
				ID:      op.ID,
				Message: fmt.Sprintf("Operator id %q must start with 'OP-' (typed-anchor rule, R-anchor-everything)", op.ID),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_typed_anchors_operator", Invariant{
	Name:  "check_typed_anchors_operator",
	Canon: methodology.Invariants,
	Claim: "every Operator.id starts with 'OP-'.",
	Rule: "Operator.id MUST start with 'OP-'. An id with the wrong prefix breaks the " +
		"typed-anchor discipline (R-anchor-everything).",
	Check: checkTypedAnchorsOperator,
})

func checkTypedAnchorsProcess(g *ontology.Graph) []Violation {
	var out []Violation
	for _, p := range g.Processes {
		if !strings.HasPrefix(p.ID, "PR-") {
			out = append(out, Violation{
				Check:   "check_typed_anchors_process",
				ID:      p.ID,
				Message: fmt.Sprintf("Process id %q must start with 'PR-' (typed-anchor rule, R-anchor-everything)", p.ID),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_typed_anchors_process", Invariant{
	Name:  "check_typed_anchors_process",
	Canon: methodology.Invariants,
	Claim: "every Process.id starts with 'PR-'.",
	Rule: "Process.id MUST start with 'PR-'. An id with the wrong prefix breaks the " +
		"typed-anchor discipline (R-anchor-everything).",
	Check: checkTypedAnchorsProcess,
})

func checkTypedAnchorsGoal(g *ontology.Graph) []Violation {
	var out []Violation
	for _, go_ := range g.Goals {
		if !strings.HasPrefix(go_.ID, "GOAL-") {
			out = append(out, Violation{
				Check:   "check_typed_anchors_goal",
				ID:      go_.ID,
				Message: fmt.Sprintf("Goal id %q must start with 'GOAL-' (typed-anchor rule, R-anchor-everything)", go_.ID),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_typed_anchors_goal", Invariant{
	Name:  "check_typed_anchors_goal",
	Canon: methodology.Invariants,
	Claim: "every Goal.id starts with 'GOAL-'.",
	Rule: "Goal.id MUST start with 'GOAL-'. An id with the wrong prefix breaks the " +
		"typed-anchor discipline (R-anchor-everything).",
	Check: checkTypedAnchorsGoal,
})

func checkTypedAnchors(g *ontology.Graph) []Violation {
	var out []Violation
	out = append(out, checkTypedAnchorsRequirement(g)...)
	out = append(out, checkTypedAnchorsAssumption(g)...)
	out = append(out, checkTypedAnchorsConflict(g)...)
	out = append(out, checkTypedAnchorsOperator(g)...)
	out = append(out, checkTypedAnchorsProcess(g)...)
	out = append(out, checkTypedAnchorsGoal(g)...)
	return out
}

var _ = All.MustRegister("check_typed_anchors", Invariant{
	Name:  "check_typed_anchors",
	Canon: methodology.Invariants,
	Claim: "every id carries the prefix that matches its kind (thin delegator).",
	Rule: "Requirement.id MUST start with 'R-'; Assumption.id MUST start with 'A-'; " +
		"Conflict.id MUST start with 'C-'; Operator.id MUST start with 'OP-'. An id with a " +
		"wrong or missing prefix breaks the typed-anchor discipline (R-anchor-everything) and " +
		"makes cite-by-reference unreliable (R-speak-by-reference). This is a THIN DELEGATOR — " +
		"calls the atomic per-entity-type sub-checks and concatenates.",
	Why: "this check enforces the CURRENTLY USED prefixes (R-/A-/C-/OP-) that are already " +
		"discipline in the codebase; it does NOT yet encode the full M28 taxonomy (GOAL-/GAP-/" +
		"DLG-/AX-) — those are still OPEN per R-anchor-taxonomy.",
	Check:       checkTypedAnchors,
	IsDelegator: true,
})
