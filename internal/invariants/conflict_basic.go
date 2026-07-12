package invariants

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const constitutingConvergenceAtom = "R-constituting-requirements-converge"

func checkConflictHasAxis(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		if strings.TrimSpace(c.Axis) == "" {
			out = append(out, Violation{
				Check:   "check_conflict_has_axis",
				ID:      c.ID,
				Message: "conflict has no tension axis (along WHAT do they diverge?)",
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_conflict_has_axis", Invariant{
	Name:  "check_conflict_has_axis",
	Canon: methodology.Conflict,
	Claim: "every Conflict carries a non-empty axis.",
	Rule: "Conflict.axis MUST be a non-empty string. An axis-less conflict is not a connector node — it does not name " +
		"the tension dimension it mediates.",
	Why:   "the axis is what makes conflicts cluster into architectural choices; an axis-less conflict is invisible in any cluster view.",
	Check: checkConflictHasAxis,
})

func checkConflictHasContext(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		if strings.TrimSpace(c.Context) == "" {
			out = append(out, Violation{
				Check:   "check_conflict_has_context",
				ID:      c.ID,
				Message: "conflict has no context (in WHICH scenario do they collide?)",
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_conflict_has_context", Invariant{
	Name:  "check_conflict_has_context",
	Canon: methodology.Conflict,
	Claim: "every Conflict carries a non-empty context.",
	Rule: "Conflict.context MUST be a non-empty string describing the scenario where the two requirements collide. " +
		"A context-less conflict has no scenario and cannot be communicated to a steward.",
	Why:   "without a context the conflict cannot be communicated to a steward or a domain user in a way that enables resolution.",
	Check: checkConflictHasContext,
})

func checkConflictHasSteward(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		if strings.TrimSpace(c.Steward) == "" {
			out = append(out, Violation{
				Check:   "check_conflict_has_steward",
				ID:      c.ID,
				Message: "conflict has no steward (WHO holds this tension?)",
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_conflict_has_steward", Invariant{
	Name:  "check_conflict_has_steward",
	Canon: methodology.Conflict,
	Claim: "every Conflict carries a non-empty steward.",
	Rule: "Conflict.steward MUST be a non-empty string. A stewardless conflict has no holder — the tension is invisible " +
		"to the methodology.",
	Why: "this is the structural definition of 'the contradiction is visible'. A stewardless conflict is exactly an " +
		"invisible contradiction — the hard boundary (R-ai-presents-not-decides) requires a named outside party.",
	Check: checkConflictHasSteward,
})

func checkConflictHasAxisContextSteward(g *ontology.Graph) []Violation {
	var out []Violation
	out = append(out, checkConflictHasAxis(g)...)
	out = append(out, checkConflictHasContext(g)...)
	out = append(out, checkConflictHasSteward(g)...)
	return out
}

var _ = All.MustRegister("check_conflict_has_axis_context_steward", Invariant{
	Name:  "check_conflict_has_axis_context_steward",
	Canon: methodology.Conflict,
	Claim: "every Conflict carries a non-empty axis, context, steward (thin delegator).",
	Rule:  "axis, context and steward MUST all be non-empty. These three are the knowledge that belongs to neither member.",
	Why: "this is the structural definition of 'the contradiction is visible'. An axis-less or stewardless conflict is " +
		"exactly an invisible contradiction. This is a THIN DELEGATOR — calls the three atomic sub-checks and concatenates.",
	Check:       checkConflictHasAxisContextSteward,
	IsDelegator: true,
})

func checkConflictMinTwoMembers(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		seen := map[string]struct{}{}
		for _, m := range c.Members {
			seen[m] = struct{}{}
		}
		if len(seen) < 2 {
			out = append(out, Violation{
				Check:   "check_conflict_min_two_members",
				ID:      c.ID,
				Message: "conflict needs >= 2 distinct member requirements",
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_conflict_min_two_members", Invariant{
	Name:  "check_conflict_min_two_members",
	Canon: methodology.Conflict,
	Claim: "every Conflict mediates >= 2 distinct requirements.",
	Rule:  "members MUST contain at least two DISTINCT Requirement ids. A conflict with fewer is not a tension between parties.",
	Why: "a connector node connects; with one (or zero) members there is nothing to hold between, and clustering/lineage " +
		"become meaningless.",
	Check: checkConflictMinTwoMembers,
})

func checkConstitutingNotInUnresolvedConflict(g *ontology.Graph) []Violation {
	settledIDs := map[string]struct{}{}
	for _, r := range g.Requirements {
		if r.Status == ontology.StatusSETTLED {
			settledIDs[r.ID] = struct{}{}
		}
	}
	if !g.SelfHosting {
		return nil
	}
	if _, ok := settledIDs[constitutingConvergenceAtom]; !ok {
		return nil
	}
	var out []Violation
	for _, c := range g.Conflicts {
		if !c.IsUnresolved() {
			continue
		}
		memberSet := map[string]struct{}{}
		for _, m := range c.Members {
			memberSet[m] = struct{}{}
		}
		var settledMembers []string
		for m := range memberSet {
			if _, ok := settledIDs[m]; ok {
				settledMembers = append(settledMembers, m)
			}
		}
		sort.Strings(settledMembers)
		if len(settledMembers) >= 2 {
			out = append(out, Violation{
				Check: "check_constituting_not_in_unresolved_conflict",
				ID:    c.ID,
				Message: fmt.Sprintf(
					"conflict %q (%s) holds >= 2 SETTLED constituting atoms (%s) as an UNRESOLVED contradiction while the "+
						"CONSTITUTION presents them as settled truth — steward must resolve it (DECIDED / REVISIT_WHEN) "+
						"or the members must not both be SETTLED (R-constituting-requirements-converge).",
					c.ID, c.Lifecycle, strings.Join(settledMembers, ", ")),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_constituting_not_in_unresolved_conflict", Invariant{
	Name:  "check_constituting_not_in_unresolved_conflict",
	Canon: methodology.Requirement,
	Claim: "no two SETTLED constituting atoms sit in an unresolved conflict.",
	Rule: "in the self-host graph, no unresolved Conflict (DETECTED / ACKNOWLEDGED) may hold two SETTLED Requirements as " +
		"members. This is the machine-checkable face of 'the set of SETTLED requirements composing the operator-prompt " +
		"shall be pairwise consistent' (R-constituting-requirements-converge).",
	Why: "scoped to the self-host graph (FRAMEWORK_SCOPED, gated on g.self_hosting): a business domain's DETECTED " +
		"conflict with SETTLED members is NORMAL life — the tension has been found and is awaiting its steward, which is " +
		"exactly what the methodology is for.",
	Check: checkConstitutingNotInUnresolvedConflict,
})

func checkAxisInRegistry(g *ontology.Graph) []Violation {
	slugs := ontology.AxisSlugs(g)
	var out []Violation
	for _, c := range g.Conflicts {
		if c.Axis == "" {
			continue
		}
		if _, ok := slugs[c.Axis]; !ok {
			out = append(out, Violation{
				Check:   "check_axis_in_registry",
				ID:      c.ID,
				Message: fmt.Sprintf("axis %q is not in the controlled vocabulary (add it to the graph's `axes` tuple or pick an existing slug)", c.Axis),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_axis_in_registry", Invariant{
	Name:  "check_axis_in_registry",
	Canon: methodology.Axis,
	Claim: "every Conflict.axis is a slug in the graph's vocabulary.",
	Rule: "Conflict.axis MUST be in axis_slugs(g) — i.e. the slug of some Axis in TensionGraph.axes. An unknown or " +
		"ad-hoc axis is rejected so conflicts CLUSTER.",
	Why: "clustering by axis is how a node-graph reveals an architectural choice; free-text axes would fragment the " +
		"cluster and hide it. Since the framework is content-free, the per-domain vocabulary lives on the graph itself.",
	Check: checkAxisInRegistry,
})

func checkConflictIDMatchesIdentity(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		if c.Axis == "" || c.Context == "" {
			continue
		}
		expected := ontology.ConflictIdentity(c.Axis, c.Context)
		if c.ID != expected {
			out = append(out, Violation{
				Check:   "check_conflict_id_matches_identity",
				ID:      c.ID,
				Message: fmt.Sprintf("conflict id should be %q (= conflict_identity(axis, context))", expected),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_conflict_id_matches_identity", Invariant{
	Name:  "check_conflict_id_matches_identity",
	Canon: methodology.Conflict,
	Claim: "id == conflict_identity(axis, context).",
	Rule: "a Conflict's id MUST be the deterministic hash of (axis, context). A hand-written id is rejected so the " +
		"node's identity tracks its TENSION, not its members, and survives member renaming/splitting.",
	Why:   "identity-from-tension is what makes the same conflict survive churn and keeps clustering stable; a free id would let the node drift from its meaning.",
	Check: checkConflictIDMatchesIdentity,
})

func requirementOwnerMap(g *ontology.Graph) map[string]string {
	out := make(map[string]string, len(g.Requirements))
	for _, r := range g.Requirements {
		out[r.ID] = r.Owner
	}
	return out
}

func checkStewardNotAMemberOwner(g *ontology.Graph) []Violation {
	ownerOf := requirementOwnerMap(g)
	var out []Violation
	for _, c := range g.Conflicts {
		memberOwners := map[string]struct{}{}
		for _, m := range c.Members {
			if owner, ok := ownerOf[m]; ok {
				memberOwners[owner] = struct{}{}
			}
		}
		if _, ok := memberOwners[c.Steward]; ok {
			out = append(out, Violation{
				Check:   "check_steward_not_a_member_owner",
				ID:      c.ID,
				Message: fmt.Sprintf("steward %q also owns a member requirement; a conflict must be stewarded from outside its members", c.Steward),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_steward_not_a_member_owner", Invariant{
	Name:  "check_steward_not_a_member_owner",
	Canon: methodology.Conflict,
	Claim: "steward is not the owner of any member.",
	Rule: "Conflict.steward MUST NOT equal the owner of any member Requirement. A conflict lives BETWEEN stakeholders; " +
		"if the steward owned a side, the tension would be judged by an interested party and quietly resolved in their favor.",
	Why: "this is the hard boundary made structural: it is the same principle as the AI never closing a conflict silently — " +
		"the holder of the tension must be a party who does not own either claim, or invisibility returns.",
	Check: checkStewardNotAMemberOwner,
})

func checkOpenHasQuestion(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		if !r.IsOpen() {
			continue
		}
		inside := strings.TrimSpace(strings.TrimPrefix(r.Status, "OPEN"))
		var question string
		if strings.HasPrefix(inside, "(") && strings.HasSuffix(inside, ")") {
			question = strings.TrimSpace(inside[1 : len(inside)-1])
		}
		if question == "" {
			out = append(out, Violation{
				Check:   "check_open_has_question",
				ID:      r.ID,
				Message: "OPEN requirement must state a non-empty question: status = 'OPEN(<question>)'",
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_open_has_question", Invariant{
	Name:  "check_open_has_question",
	Canon: methodology.Requirement,
	Claim: "an OPEN requirement carries a non-empty question.",
	Rule: "if status starts with \"OPEN\", it MUST be of the form \"OPEN(<question>)\" with a non-empty question. An OPEN " +
		"with no question is a hole nobody can act on — invisible openness.",
	Why: "the harness and OPEN.md surface open holes by their question; an empty question gives the steward nothing to " +
		"decide, defeating the point of marking it open at all.",
	Check: checkOpenHasQuestion,
})
