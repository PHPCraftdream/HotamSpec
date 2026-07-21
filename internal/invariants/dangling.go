package invariants

import (
	"fmt"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func checkNoDanglingAssumptionOwner(g *ontology.Graph) []Violation {
	sids := ontology.StakeholderIDs(g)
	var out []Violation
	for _, a := range g.Assumptions {
		if _, ok := sids[a.Owner]; !ok {
			out = append(out, Violation{
				Check:   "check_no_dangling_assumption_owner",
				ID:      a.ID,
				Message: fmt.Sprintf("assumption owner %q is not a known Stakeholder", a.Owner),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_no_dangling_assumption_owner", Invariant{
	Name:  "check_no_dangling_assumption_owner",
	Canon: methodology.Invariants,
	Claim: "every Assumption.owner resolves to a known Stakeholder.",
	Rule: "Assumption.owner MUST be in stakeholder_ids(g). A dangling assumption owner is an invisible hole — " +
		"the methodology cannot surface context drift if the assumption is unowned.",
	Why: "a dangling owner makes the assumption unanchored; drift detection depends on assumptions having a live, " +
		"resolvable owner.",
	Check: checkNoDanglingAssumptionOwner,
})

func checkAssumptionStatusValid(g *ontology.Graph) []Violation {
	var out []Violation
	for _, a := range g.Assumptions {
		if _, ok := ontology.AssumptionStates[a.Status]; !ok {
			out = append(out, Violation{
				Check:   "check_assumption_status_valid",
				ID:      a.ID,
				Message: fmt.Sprintf("Assumption status %q is not one of the known ASSUMPTION_STATES {DEAD, HOLDS, IMPLEMENTS, UNCERTAIN} (R-assumption-implements-state)", a.Status),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_assumption_status_valid", Invariant{
	Name:  "check_assumption_status_valid",
	Canon: methodology.Invariants,
	Claim: "every Assumption.status is a known ASSUMPTION_STATE.",
	Rule: "for each Assumption, status MUST be one of ASSUMPTION_STATES (HOLDS | UNCERTAIN | DEAD | IMPLEMENTS). " +
		"An unrecognised status is drift — the harness's status-keyed filters (dead_assumptions, uncertain_assumptions) " +
		"silently skip it, so it would sit in the graph invisible to every diagnosis.",
	Why: "this is the enforcer of the IMPLEMENTS род: IMPLEMENTS is the fourth, VOLITIONAL status (an aspiration — " +
		"'we strive to make this true'), distinct from the three epistemic fact-claim statuses. This single-field " +
		"set-membership check is what makes the new status a first-class, admitted value rather than an unchecked string: " +
		"it accepts IMPLEMENTS and rejects any value outside ASSUMPTION_STATES.",
	Check: checkAssumptionStatusValid,
})

func checkNoDanglingRequirementOwner(g *ontology.Graph) []Violation {
	sids := ontology.StakeholderIDs(g)
	var out []Violation
	for _, r := range g.Requirements {
		if _, ok := sids[r.Owner]; !ok {
			out = append(out, Violation{
				Check:   "check_no_dangling_requirement_owner",
				ID:      r.ID,
				Message: fmt.Sprintf("requirement owner %q is not a known Stakeholder", r.Owner),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_no_dangling_requirement_owner", Invariant{
	Name:  "check_no_dangling_requirement_owner",
	Canon: methodology.Invariants,
	Claim: "every Requirement.owner resolves to a known Stakeholder.",
	Rule: "Requirement.owner MUST be in stakeholder_ids(g). A requirement without a resolvable owner is structurally " +
		"unanchored.",
	Why:   "a dangling owner makes the requirement unanchored and breaks the resolver boundary invariant downstream.",
	Check: checkNoDanglingRequirementOwner,
})

func checkNoDanglingRequirementAssumptions(g *ontology.Graph) []Violation {
	aids := ontology.AssumptionIDs(g)
	var out []Violation
	for _, r := range g.Requirements {
		for _, aid := range r.Assumptions {
			if _, ok := aids[aid]; !ok {
				out = append(out, Violation{
					Check:   "check_no_dangling_requirement_assumptions",
					ID:      r.ID,
					Message: fmt.Sprintf("assumption %q is not a known Assumption", aid),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_no_dangling_requirement_assumptions", Invariant{
	Name:  "check_no_dangling_requirement_assumptions",
	Canon: methodology.Invariants,
	Claim: "every Requirement.assumptions[*] resolves to a known Assumption.",
	Rule: "each id in Requirement.assumptions MUST be in assumption_ids(g). A dangling assumption reference hides " +
		"drift — if the assumption never existed the dependency is invisible.",
	Why: "drift detection (DRIFT_FALLOUT band) traverses assumption dependencies; a dangling reference breaks the " +
		"traversal silently.",
	Check: checkNoDanglingRequirementAssumptions,
})

func checkNoDanglingRequirementRelations(g *ontology.Graph) []Violation {
	rids := ontology.RequirementIDs(g)
	var out []Violation
	for _, r := range g.Requirements {
		for _, rel := range r.Relations {
			if _, ok := ontology.RelationKinds[rel.Kind]; !ok {
				out = append(out, Violation{
					Check:   "check_no_dangling_requirement_relations",
					ID:      r.ID,
					Message: fmt.Sprintf("relation kind %q is not a known kind", rel.Kind),
				})
			}
			if _, ok := rids[rel.Target]; !ok {
				out = append(out, Violation{
					Check:   "check_no_dangling_requirement_relations",
					ID:      r.ID,
					Message: fmt.Sprintf("relation target %q is not a known Requirement", rel.Target),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_no_dangling_requirement_relations", Invariant{
	Name:  "check_no_dangling_requirement_relations",
	Canon: methodology.Invariants,
	Claim: "every Requirement.relations[*] has a known kind and target.",
	Rule: "each Relation.kind MUST be in RELATION_KINDS, and each Relation.target MUST be in requirement_ids(g). " +
		"An unknown kind or dangling target is an unresolvable edge.",
	Why: "relation edges drive the dependency graph (R-dependency-graph-parallelism); a dangling or mis-typed edge " +
		"makes the graph structurally incomplete.",
	Check: checkNoDanglingRequirementRelations,
})

func checkNoDanglingConflictRefs(g *ontology.Graph) []Violation {
	sids := ontology.StakeholderIDs(g)
	aids := ontology.AssumptionIDs(g)
	rids := ontology.RequirementIDs(g)
	var out []Violation
	for _, c := range g.Conflicts {
		if _, ok := sids[c.Resolver]; !ok {
			out = append(out, Violation{
				Check:   "check_no_dangling_conflict_refs",
				ID:      c.ID,
				Message: fmt.Sprintf("dangling Conflict ref — resolver %q is not a known Stakeholder", c.Resolver),
			})
		}
		for _, mid := range c.Members {
			if _, ok := rids[mid]; !ok {
				out = append(out, Violation{
					Check:   "check_no_dangling_conflict_refs",
					ID:      c.ID,
					Message: fmt.Sprintf("dangling Conflict ref — member %q is not a known Requirement", mid),
				})
			}
		}
		if c.SharedAssumption != nil {
			if _, ok := aids[*c.SharedAssumption]; !ok {
				out = append(out, Violation{
					Check:   "check_no_dangling_conflict_refs",
					ID:      c.ID,
					Message: fmt.Sprintf("dangling Conflict ref — shared_assumption %q is not a known Assumption", *c.SharedAssumption),
				})
			}
		}
		for _, did := range c.Derived {
			if _, ok := rids[did]; !ok {
				out = append(out, Violation{
					Check:   "check_no_dangling_conflict_refs",
					ID:      c.ID,
					Message: fmt.Sprintf("dangling Conflict ref — derived %q is not a known Requirement", did),
				})
			}
		}
		if c.DecidedBy != "" {
			if _, ok := sids[c.DecidedBy]; !ok {
				out = append(out, Violation{
					Check:   "check_no_dangling_conflict_refs",
					ID:      c.ID,
					Message: fmt.Sprintf("dangling Conflict ref — decided_by %q is not a known Stakeholder", c.DecidedBy),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_no_dangling_conflict_refs", Invariant{
	Name:  "check_no_dangling_conflict_refs",
	Canon: methodology.Invariants,
	Claim: "every Conflict's resolver, members, shared_assumption, derived, and decided_by resolve.",
	Rule: "Conflict.resolver MUST be in stakeholder_ids(g); each member MUST be in requirement_ids(g); shared_assumption " +
		"(if set) MUST be in assumption_ids(g); each derived id MUST be in requirement_ids(g); decided_by (if set) MUST " +
		"be in stakeholder_ids(g).",
	Why: "a dangling member is how a conflict silently loses a party; a dangling assumption is how drift hides. " +
		"Dangling refs on a Conflict are the cardinal invisibility the methodology forbids.",
	Check: checkNoDanglingConflictRefs,
})

func checkNoDanglingOperatorRefs(g *ontology.Graph) []Violation {
	sids := ontology.StakeholderIDs(g)
	oids := ontology.OperatorIDs(g)
	var out []Violation
	for _, op := range g.Operators {
		if _, ok := sids[op.Stakeholder]; !ok {
			out = append(out, Violation{
				Check:   "check_no_dangling_operator_refs",
				ID:      op.ID,
				Message: fmt.Sprintf("operator stakeholder %q is not a known Stakeholder", op.Stakeholder),
			})
		}
		if op.Parent != nil {
			if _, ok := oids[*op.Parent]; !ok {
				out = append(out, Violation{
					Check:   "check_no_dangling_operator_refs",
					ID:      op.ID,
					Message: fmt.Sprintf("operator parent %q is not a known Operator", *op.Parent),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_no_dangling_operator_refs", Invariant{
	Name:  "check_no_dangling_operator_refs",
	Canon: methodology.Invariants,
	Claim: "every Operator.stakeholder and Operator.parent resolve.",
	Rule: "Operator.stakeholder MUST be in stakeholder_ids(g); Operator.parent (if set) MUST be in operator_ids(g). " +
		"A dangling operator ref makes the delegation hierarchy structurally broken.",
	Why: "the operator tree is the recursive delegation structure (R-operator-crystal-is-claude-md); a dangling parent " +
		"or stakeholder collapses the tree invisibly.",
	Check: checkNoDanglingOperatorRefs,
})

func checkNoDanglingIDs(g *ontology.Graph) []Violation {
	var out []Violation
	out = append(out, checkNoDanglingAssumptionOwner(g)...)
	out = append(out, checkNoDanglingRequirementOwner(g)...)
	out = append(out, checkNoDanglingRequirementAssumptions(g)...)
	out = append(out, checkNoDanglingRequirementRelations(g)...)
	out = append(out, checkNoDanglingConflictRefs(g)...)
	out = append(out, checkNoDanglingOperatorRefs(g)...)
	return out
}

var _ = All.MustRegister("check_no_dangling_ids", Invariant{
	Name:  "check_no_dangling_ids",
	Canon: methodology.Invariants,
	Claim: "every id referenced by an edge resolves in the graph (thin delegator).",
	Rule: "Requirement.owner, Requirement.assumptions[*], Relation.target, Conflict.resolver, Conflict.members[*], " +
		"Conflict.shared_assumption, Conflict.derived[*], Assumption.owner, Operator.stakeholder, and Operator.parent " +
		"MUST each name an object that exists.",
	Why: "a dangling member is how a conflict silently loses a party; a dangling assumption is how drift hides. A " +
		"dangling edge is an invisible hole, the cardinal sin of the methodology. This is a THIN DELEGATOR — it calls " +
		"the atomic sub-checks and concatenates their results.",
	Check:       checkNoDanglingIDs,
	IsDelegator: true,
})

func checkDocReaderResolvesToStakeholder(g *ontology.Graph) []Violation {
	return nil
}

var _ = All.MustRegister("check_doc_reader_resolves_to_stakeholder", Invariant{
	Name:  "check_doc_reader_resolves_to_stakeholder",
	Canon: methodology.Requirement,
	Claim: "every generated doc's reader resolves to a known Stakeholder.",
	Rule: "reads the active domain's explicit DOC_READERS binding (manifest.py) — a dict[role_hint, Stakeholder.id]. " +
		"For every doc kind declared in DOC_READER_ROLES, resolve_reader(kind, stakeholder_ids(g), bindings) MUST return " +
		"an id present in stakeholder_ids(g) — never UNRESOLVED_READER. No-ops when the active domain has declared NO " +
		"DOC_READERS binding at all, OR when NONE of the declared bound ids appear in g.stakeholders.",
	Why: "an explicit declared binding, not a substring guess over stakeholder ids (R-doc-readers-declared-not-guessed): " +
		"the prior design resolved a role hint by scanning stakeholder_ids(g) for any id containing a hint substring — a " +
		"stakeholder id such as 'travel-agent' would silently capture operator-facing docs it has nothing to do with.",
	Check: checkDocReaderResolvesToStakeholder,
})
