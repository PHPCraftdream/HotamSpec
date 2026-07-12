package invariants

import (
	"fmt"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/methodology"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

func checkDecidedHasRationaleOrDerived(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		if !c.IsDecided() {
			continue
		}
		inside := strings.TrimSpace(strings.TrimPrefix(c.Lifecycle, ontology.ConflictDECIDEDPrefix))
		var rationale string
		if strings.HasPrefix(inside, "(") && strings.HasSuffix(inside, ")") && len(inside) >= 2 {
			rationale = strings.TrimSpace(inside[1 : len(inside)-1])
		}
		if rationale == "" && len(c.Derived) == 0 {
			out = append(out, Violation{
				Check:   "check_decided_has_rationale_or_derived",
				ID:      c.ID,
				Message: "DECIDED conflict must record a rationale 'DECIDED(<why>)' or reference a derived requirement",
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_decided_has_rationale_or_derived", Invariant{
	Name:  "check_decided_has_rationale_or_derived",
	Canon: methodology.Conflict,
	Claim: "a DECIDED conflict records rationale or a derived req.",
	Rule: "if lifecycle starts with \"DECIDED\", it MUST carry a non-empty rationale inside " +
		"\"DECIDED(<rationale>)\" OR a non-empty `derived` tuple. A decision with neither is a " +
		"silent close — forbidden.",
	Why: "the historian role depends on every decision carrying its rationale and (often) the " +
		"requirement it spawned; without that the resolution is invisible and gets relitigated. " +
		"This is the anti-relitigation marker made structural.",
	Check: checkDecidedHasRationaleOrDerived,
})

func checkDecidedHasNonemptyDecidedBy(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		if !c.IsDecided() {
			continue
		}
		if c.DecidedBy == "" {
			out = append(out, Violation{
				Check:   "check_decided_has_nonempty_decided_by",
				ID:      c.ID,
				Message: "DECIDED conflict must carry a non-empty decided_by (the Stakeholder.id of the human who approved the resolution; R-decided-needs-human-signoff)",
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_decided_has_nonempty_decided_by", Invariant{
	Name:  "check_decided_has_nonempty_decided_by",
	Canon: methodology.Conflict,
	Claim: "a DECIDED conflict carries a non-empty decided_by field.",
	Rule: "when Conflict.lifecycle starts with \"DECIDED\", `decided_by` MUST be non-empty. A " +
		"DECIDED conflict without a named human decider is an AI-silently-closeable hole — " +
		"exactly the invisibility the hard boundary forbids.",
	Why: "R-decided-needs-human-signoff makes the closed loop's ACT half structurally visible. " +
		"Without this lock, an AI could write lifecycle=\"DECIDED(...)\" with decided_by=\"\" and " +
		"pass all other invariants.",
	Check: checkDecidedHasNonemptyDecidedBy,
})

func checkDecidedByIsKnownStakeholder(g *ontology.Graph) []Violation {
	sids := ontology.StakeholderIDs(g)
	var out []Violation
	for _, c := range g.Conflicts {
		if !c.IsDecided() {
			continue
		}
		if c.DecidedBy == "" {
			continue
		}
		if _, ok := sids[c.DecidedBy]; !ok {
			out = append(out, Violation{
				Check:   "check_decided_by_is_known_stakeholder",
				ID:      c.ID,
				Message: fmt.Sprintf("decided_by %q is not a known Stakeholder", c.DecidedBy),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_decided_by_is_known_stakeholder", Invariant{
	Name:  "check_decided_by_is_known_stakeholder",
	Canon: methodology.Conflict,
	Claim: "a DECIDED conflict's decided_by resolves to a known Stakeholder.",
	Rule: "when Conflict.lifecycle starts with \"DECIDED\" and decided_by is non-empty, " +
		"decided_by MUST be in stakeholder_ids(g). An unresolvable decider is a dangling " +
		"reference that cannot be audited.",
	Why: "check_no_dangling_conflict_refs also catches this, but naming it explicitly in the " +
		"harness makes the missing signoff traceable to the decision moment.",
	Check: checkDecidedByIsKnownStakeholder,
})

func checkDecidedByNotMemberOwner(g *ontology.Graph) []Violation {
	ownerOf := requirementOwnerMap(g)
	sids := ontology.StakeholderIDs(g)
	var out []Violation
	for _, c := range g.Conflicts {
		if !c.IsDecided() {
			continue
		}
		if c.DecidedBy == "" {
			continue
		}
		if _, ok := sids[c.DecidedBy]; !ok {
			continue
		}
		memberOwners := map[string]struct{}{}
		for _, m := range c.Members {
			if owner, ok := ownerOf[m]; ok {
				memberOwners[owner] = struct{}{}
			}
		}
		if _, ok := memberOwners[c.DecidedBy]; ok {
			out = append(out, Violation{
				Check:   "check_decided_by_not_member_owner",
				ID:      c.ID,
				Message: fmt.Sprintf("decided_by %q also owns a member requirement; the decider must be outside the conflict's members (steward-distinct rule applied to the decider)", c.DecidedBy),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_decided_by_not_member_owner", Invariant{
	Name:  "check_decided_by_not_member_owner",
	Canon: methodology.Conflict,
	Claim: "a DECIDED conflict's decided_by is not the owner of any member Requirement.",
	Rule: "when Conflict.lifecycle starts with \"DECIDED\", decided_by MUST NOT be the owner of " +
		"any of the conflict's member Requirements. The decider must be outside the conflict's " +
		"members (steward-distinct rule applied to the decider).",
	Why: "if the decider owned one of the members, the hard boundary would be circumvented at " +
		"the decision step. This is the structural twin of check_steward_not_a_member_owner " +
		"applied at the moment of resolution.",
	Check: checkDecidedByNotMemberOwner,
})

func checkHeldHasMinTwoVariants(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		if !c.IsHeld() {
			continue
		}
		seen := map[string]struct{}{}
		for _, v := range c.Variants {
			seen[v.ID] = struct{}{}
		}
		if len(seen) < 2 {
			out = append(out, Violation{
				Check:   "check_held_has_min_two_variants",
				ID:      c.ID,
				Message: "HELD conflict must carry >= 2 distinct Variant ids (the steward needs at least two sides to choose between)",
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_held_has_min_two_variants", Invariant{
	Name:  "check_held_has_min_two_variants",
	Canon: methodology.Conflict,
	Claim: "a HELD conflict carries at least two elaborated Variants.",
	Rule: "when Conflict.lifecycle starts with \"HELD\", `variants` MUST contain at least two " +
		"distinct Variant ids. A HELD tension with fewer than two variants gives the steward " +
		"nothing to choose between -- exactly the invisible-contradiction-in-a-new-costume the " +
		"hard boundary forbids.",
	Why: "mirrors check_conflict_min_two_members -- a HELD conflict connects at least two SIDES " +
		"of a live tension the same way a Conflict connects at least two member requirements.",
	Check: checkHeldHasMinTwoVariants,
})

func checkHeldHasNonemptyDecidedBy(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		if !c.IsHeld() {
			continue
		}
		if c.DecidedBy == "" {
			out = append(out, Violation{
				Check:   "check_held_has_nonempty_decided_by",
				ID:      c.ID,
				Message: "HELD conflict must carry a non-empty decided_by (the Stakeholder.id of the human who classified this tension unresolvable-by-members; R-decided-needs-human-signoff)",
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_held_has_nonempty_decided_by", Invariant{
	Name:  "check_held_has_nonempty_decided_by",
	Canon: methodology.Conflict,
	Claim: "a HELD conflict carries a non-empty decided_by field.",
	Rule: "when Conflict.lifecycle starts with \"HELD\", `decided_by` MUST be non-empty. Entering " +
		"HELD is a human act (R-decided-needs-human-signoff's signoff lock applied at the moment " +
		"a tension is classified unresolvable by its members) -- without this lock an AI could " +
		"silently write lifecycle=\"HELD(...)\" with decided_by=\"\".",
	Why: "the structural twin of check_decided_has_nonempty_decided_by applied to HELD instead " +
		"of DECIDED.",
	Check: checkHeldHasNonemptyDecidedBy,
})

func checkHeldByIsKnownStakeholder(g *ontology.Graph) []Violation {
	sids := ontology.StakeholderIDs(g)
	var out []Violation
	for _, c := range g.Conflicts {
		if !c.IsHeld() {
			continue
		}
		if c.DecidedBy == "" {
			continue
		}
		if _, ok := sids[c.DecidedBy]; !ok {
			out = append(out, Violation{
				Check:   "check_held_by_is_known_stakeholder",
				ID:      c.ID,
				Message: fmt.Sprintf("decided_by %q is not a known Stakeholder", c.DecidedBy),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_held_by_is_known_stakeholder", Invariant{
	Name:  "check_held_by_is_known_stakeholder",
	Canon: methodology.Conflict,
	Claim: "a HELD conflict's decided_by resolves to a known Stakeholder.",
	Rule: "when Conflict.lifecycle starts with \"HELD\" and decided_by is non-empty, decided_by " +
		"MUST be in stakeholder_ids(g).",
	Why:   "mirrors check_decided_by_is_known_stakeholder applied to HELD.",
	Check: checkHeldByIsKnownStakeholder,
})

func checkHeldByNotMemberOwner(g *ontology.Graph) []Violation {
	ownerOf := requirementOwnerMap(g)
	sids := ontology.StakeholderIDs(g)
	var out []Violation
	for _, c := range g.Conflicts {
		if !c.IsHeld() {
			continue
		}
		if c.DecidedBy == "" {
			continue
		}
		if _, ok := sids[c.DecidedBy]; !ok {
			continue
		}
		memberOwners := map[string]struct{}{}
		for _, m := range c.Members {
			if owner, ok := ownerOf[m]; ok {
				memberOwners[owner] = struct{}{}
			}
		}
		if _, ok := memberOwners[c.DecidedBy]; ok {
			out = append(out, Violation{
				Check:   "check_held_by_not_member_owner",
				ID:      c.ID,
				Message: fmt.Sprintf("decided_by %q also owns a member requirement; the human who holds this tension open must be outside the conflict's members (steward-distinct rule applied to HELD)", c.DecidedBy),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_held_by_not_member_owner", Invariant{
	Name:  "check_held_by_not_member_owner",
	Canon: methodology.Conflict,
	Claim: "a HELD conflict's decided_by is not the owner of any member Requirement.",
	Rule: "when Conflict.lifecycle starts with \"HELD\", decided_by MUST NOT be the owner of any " +
		"of the conflict's member Requirements -- the steward-distinct rule applied to the human " +
		"who holds the tension open.",
	Why: "mirrors check_decided_by_not_member_owner applied to HELD; if the signoff owned a " +
		"member, the hard boundary would be circumvented at the hold step exactly as it would " +
		"at the decide step.",
	Check: checkHeldByNotMemberOwner,
})

func checkHeldHasDecidedBy(g *ontology.Graph) []Violation {
	var out []Violation
	out = append(out, checkHeldHasNonemptyDecidedBy(g)...)
	out = append(out, checkHeldByIsKnownStakeholder(g)...)
	out = append(out, checkHeldByNotMemberOwner(g)...)
	return out
}

var _ = All.MustRegister("check_held_has_decided_by", Invariant{
	Name:  "check_held_has_decided_by",
	Canon: methodology.Conflict,
	Claim: "a HELD conflict names a human decider outside its members (thin delegator).",
	Rule: "(R-decided-needs-human-signoff applied to HELD): when Conflict.lifecycle starts with " +
		"\"HELD\", `decided_by` MUST satisfy three conditions: (1) non-empty, (2) resolves to a " +
		"known Stakeholder id, (3) NOT the owner of any of the conflict's member Requirements. " +
		"This is a THIN DELEGATOR — calls check_held_has_nonempty_decided_by, " +
		"check_held_by_is_known_stakeholder, check_held_by_not_member_owner and concatenates.",
	Check:       checkHeldHasDecidedBy,
	IsDelegator: true,
})

func checkDecidedHasDecidedBy(g *ontology.Graph) []Violation {
	var out []Violation
	out = append(out, checkDecidedHasNonemptyDecidedBy(g)...)
	out = append(out, checkDecidedByIsKnownStakeholder(g)...)
	out = append(out, checkDecidedByNotMemberOwner(g)...)
	return out
}

var _ = All.MustRegister("check_decided_has_decided_by", Invariant{
	Name:  "check_decided_has_decided_by",
	Canon: methodology.Conflict,
	Claim: "a DECIDED conflict names a human decider outside its members (thin delegator).",
	Rule: "(R-decided-needs-human-signoff + §Proposal): when Conflict.lifecycle starts with " +
		"\"DECIDED\", `decided_by` MUST satisfy three conditions: 1. Non-empty. 2. Resolves to " +
		"a known Stakeholder id. 3. NOT the owner of any of the conflict's member Requirements. " +
		"This is a THIN DELEGATOR — calls check_decided_has_nonempty_decided_by, " +
		"check_decided_by_is_known_stakeholder, check_decided_by_not_member_owner and " +
		"concatenates.",
	Why: "R-decided-needs-human-signoff makes the closed loop's ACT half structurally visible " +
		"(§Proposal — the closed loop's ACT half).",
	Check:       checkDecidedHasDecidedBy,
	IsDelegator: true,
})
