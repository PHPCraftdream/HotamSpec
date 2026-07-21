package invariants

import (
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/PHPCraftdream/HotamSpec/internal/methodology"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func lifecycleWellformedIssues(lc ontology.Lifecycle) []string {
	var issues []string
	if len(lc.States) == 0 {
		issues = append(issues, fmt.Sprintf("%s: Lifecycle has no states", lc.Slug))
		return issues
	}

	names := lc.StateNames()

	var initials []ontology.State
	for _, s := range lc.States {
		if s.IsInitial() {
			initials = append(initials, s)
		}
	}
	if len(initials) != 1 {
		issues = append(issues, fmt.Sprintf("%s: expected exactly 1 INITIAL state, found %d", lc.Slug, len(initials)))
	}

	for _, t := range lc.Transitions {
		if _, ok := names[t.Src]; !ok {
			issues = append(issues, fmt.Sprintf("%s: transition %q has unknown src %q", lc.Slug, t.Event, t.Src))
		}
		if _, ok := names[t.Dst]; !ok {
			issues = append(issues, fmt.Sprintf("%s: transition %q has unknown dst %q", lc.Slug, t.Event, t.Dst))
		}
	}

	if !lc.Cyclic && len(initials) > 0 {
		start := initials[0].Name
		reachable := map[string]struct{}{start: {}}
		queue := []string{start}
		adjacency := map[string][]string{}
		for _, s := range lc.States {
			adjacency[s.Name] = nil
		}
		for _, t := range lc.Transitions {
			if _, ok := adjacency[t.Src]; ok {
				adjacency[t.Src] = append(adjacency[t.Src], t.Dst)
			}
		}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, nxt := range adjacency[cur] {
				if _, ok := reachable[nxt]; !ok {
					reachable[nxt] = struct{}{}
					queue = append(queue, nxt)
				}
			}
		}
		stateByName := map[string]ontology.State{}
		for _, s := range lc.States {
			stateByName[s.Name] = s
		}
		terminalReachable := false
		for n := range reachable {
			if s, ok := stateByName[n]; ok && s.IsTerminal() {
				terminalReachable = true
				break
			}
		}
		if !terminalReachable {
			issues = append(issues, fmt.Sprintf(
				"%s: no terminal/quiescent state reachable from INITIAL %q (mark cyclic=True if intentional)",
				lc.Slug, start))
		}
	}

	return issues
}

func checkRequirementStatusInLifecycle(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		if _, ok := ontology.RequirementStatusLifecycle.Matches(r.Status); !ok {
			out = append(out, Violation{
				Check: "check_requirement_status_in_lifecycle",
				ID:    r.ID,
				Message: fmt.Sprintf(
					"Requirement.status %q is not a valid state in lifecycle %q",
					r.Status, ontology.RequirementStatusLifecycle.Slug),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_requirement_status_in_lifecycle", Invariant{
	Name:  "check_requirement_status_in_lifecycle",
	Canon: methodology.Lifecycle,
	Claim: "every Requirement.status matches REQUIREMENT_STATUS_LIFECYCLE.",
	Rule: "Requirement.status MUST be matched by REQUIREMENT_STATUS_LIFECYCLE (exact match for DRAFT/SETTLED/REJECTED; " +
		"prefix match for OPEN(question)). When matches() returns no hit, fire a Violation.",
	Why: "status is a hand-rolled string state machine; this invariant enforces that stored values belong to the canonical " +
		"set. References: R-lifecycle-abstraction, R-statemachine-wellformedness.",
	Check: checkRequirementStatusInLifecycle,
})

func checkRequirementHistoryWellformed(g *ontology.Graph) []Violation {
	var out []Violation
	for _, r := range g.Requirements {
		prevAt := ""
		hasPrev := false
		for idx, entry := range r.History {
			if entry.At == "" {
				out = append(out, Violation{
					Check: "check_requirement_history_wellformed",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"history entry [%d] has an empty `at` stamp -- every HistoryEntry MUST be dated (sub-rule 1).",
						idx),
				})
			}
			if entry.Summary == "" {
				out = append(out, Violation{
					Check: "check_requirement_history_wellformed",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"history entry [%d] has an empty `summary` -- every HistoryEntry MUST record what changed (sub-rule 2).",
						idx),
				})
			}
			if hasPrev && entry.At != "" && entry.At < prevAt {
				out = append(out, Violation{
					Check: "check_requirement_history_wellformed",
					ID:    r.ID,
					Message: fmt.Sprintf(
						"history entry [%d] stamp `at` %q is earlier than the preceding entry stamp %q -- "+
							"the append-only history trail MUST be monotonically non-decreasing (sub-rule 3).",
						idx, entry.At, prevAt),
				})
			}
			if entry.At != "" {
				prevAt = entry.At
				hasPrev = true
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_requirement_history_wellformed", Invariant{
	Name:  "check_requirement_history_wellformed",
	Canon: methodology.Requirement,
	Claim: "every Requirement history tuple is structurally well-formed.",
	Rule: "SYNTACTIC ONLY, three sub-rules over each Requirement's history HistoryEntry tuple: 1. every HistoryEntry MUST " +
		"have a non-empty `at` stamp. 2. every HistoryEntry MUST have a non-empty `summary`. 3. the history entries MUST be " +
		"monotonically non-decreasing in `at` (each stamp >= the preceding stamp), because the history trail is append-only " +
		"and never travels backwards in time. Stamps compare as plain strings (ISO-8601 sorts lexicographically == " +
		"chronologically).",
	Why: "only structure, never content: this history check deliberately does NOT judge whether a summary is 'accurate' or " +
		"a review is 'current'. The history trail is derived from the field diff; the only thing this check_* can honestly " +
		"guarantee is that each derived history record is shaped correctly (dated, non-empty, ordered), not that its prose " +
		"is meaningful.",
	Check: checkRequirementHistoryWellformed,
})

func checkConflictLifecycleInLifecycle(g *ontology.Graph) []Violation {
	var out []Violation
	for _, c := range g.Conflicts {
		if _, ok := ontology.ConflictLifecycle.Matches(c.Lifecycle); !ok {
			out = append(out, Violation{
				Check: "check_conflict_lifecycle_in_lifecycle",
				ID:    c.ID,
				Message: fmt.Sprintf(
					"Conflict.lifecycle %q is not a valid state in lifecycle %q",
					c.Lifecycle, ontology.ConflictLifecycle.Slug),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_conflict_lifecycle_in_lifecycle", Invariant{
	Name:  "check_conflict_lifecycle_in_lifecycle",
	Canon: methodology.Lifecycle,
	Claim: "every Conflict.lifecycle matches CONFLICT_LIFECYCLE.",
	Rule: "Conflict.lifecycle MUST be matched by CONFLICT_LIFECYCLE (exact match for DETECTED/ACKNOWLEDGED; prefix match " +
		"for DECIDED(rationale) and REVISIT_WHEN(condition)). When matches() returns no hit, fire a Violation.",
	Why: "conflict lifecycle is a hand-rolled string state machine; enforcing canonical values makes the machine " +
		"structurally visible and checkable. References: R-lifecycle-abstraction, R-statemachine-wellformedness.",
	Check: checkConflictLifecycleInLifecycle,
})

func checkOperatorLifecycleInLifecycle(g *ontology.Graph) []Violation {
	var out []Violation
	for _, op := range g.Operators {
		if _, ok := ontology.OperatorLifecycle.Matches(op.Lifecycle); !ok {
			out = append(out, Violation{
				Check: "check_operator_lifecycle_in_lifecycle",
				ID:    op.ID,
				Message: fmt.Sprintf(
					"Operator.lifecycle %q is not a valid state in lifecycle %q",
					op.Lifecycle, ontology.OperatorLifecycle.Slug),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_operator_lifecycle_in_lifecycle", Invariant{
	Name:  "check_operator_lifecycle_in_lifecycle",
	Canon: methodology.Lifecycle,
	Claim: "every Operator.lifecycle matches OPERATOR_LIFECYCLE.",
	Rule: "Operator.lifecycle MUST be matched by OPERATOR_LIFECYCLE (exact match for ACTIVE/SATURATED/DELEGATED/RETIRED). " +
		"When matches() returns no hit, fire a Violation.",
	Why: "operator lifecycle is a hand-rolled string state machine; enforcing canonical values makes the machine " +
		"structurally visible and checkable. References: R-lifecycle-abstraction, R-statemachine-wellformedness.",
	Check: checkOperatorLifecycleInLifecycle,
})

func checkGoalLifecycleInLifecycle(g *ontology.Graph) []Violation {
	var out []Violation
	for _, go_ := range g.Goals {
		if _, ok := ontology.GoalLifecycle.Matches(go_.Lifecycle); !ok {
			out = append(out, Violation{
				Check: "check_goal_lifecycle_in_lifecycle",
				ID:    go_.ID,
				Message: fmt.Sprintf(
					"Goal.lifecycle %q is not a valid state in lifecycle %q",
					go_.Lifecycle, ontology.GoalLifecycle.Slug),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_goal_lifecycle_in_lifecycle", Invariant{
	Name:  "check_goal_lifecycle_in_lifecycle",
	Canon: methodology.Lifecycle,
	Claim: "every Goal.lifecycle matches GOAL_LIFECYCLE.",
	Rule:  "Goal.lifecycle MUST be matched by GOAL_LIFECYCLE. When matches() returns no hit, fire a Violation.",
	Why: "goal lifecycle is a hand-rolled string state machine; enforcing canonical values makes the machine structurally " +
		"visible and checkable. References: R-lifecycle-abstraction, R-statemachine-wellformedness.",
	Check: checkGoalLifecycleInLifecycle,
})

func checkStatusInLifecycle(g *ontology.Graph) []Violation {
	var out []Violation
	out = append(out, checkRequirementStatusInLifecycle(g)...)
	out = append(out, checkConflictLifecycleInLifecycle(g)...)
	out = append(out, checkOperatorLifecycleInLifecycle(g)...)
	out = append(out, checkGoalLifecycleInLifecycle(g)...)
	return out
}

var _ = All.MustRegister("check_status_in_lifecycle", Invariant{
	Name:  "check_status_in_lifecycle",
	Canon: methodology.Lifecycle,
	Claim: "every status/lifecycle value matches a canonical Lifecycle (thin delegator).",
	Rule: "four sub-rules: 1. every Requirement.status MUST be matched by REQUIREMENT_STATUS_LIFECYCLE. 2. every " +
		"Conflict.lifecycle MUST be matched by CONFLICT_LIFECYCLE. 3. every Operator.lifecycle MUST be matched by " +
		"OPERATOR_LIFECYCLE. 4. every Goal.lifecycle MUST be matched by GOAL_LIFECYCLE. This is a THIN DELEGATOR -- calls " +
		"the four atomic per-entity sub-checks and concatenates. The atomic sub-checks are registered individually.",
	Why: "status and lifecycle are hand-rolled string state machines; this invariant enforces canonical values. " +
		"References: R-lifecycle-abstraction, R-statemachine-wellformedness.",
	Check:       checkStatusInLifecycle,
	IsDelegator: true,
})

func checkCanonicalLifecyclesWellformed(g *ontology.Graph) []Violation {
	canonical := []ontology.Lifecycle{
		ontology.RequirementStatusLifecycle,
		ontology.ConflictLifecycle,
		ontology.OperatorLifecycle,
		ontology.ProcessLifecycle,
		ontology.GoalLifecycle,
	}
	var out []Violation
	for _, lc := range canonical {
		for _, issue := range lifecycleWellformedIssues(lc) {
			out = append(out, Violation{
				Check:   "check_canonical_lifecycles_wellformed",
				ID:      lc.Slug,
				Message: issue,
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_canonical_lifecycles_wellformed", Invariant{
	Name:  "check_canonical_lifecycles_wellformed",
	Canon: methodology.Lifecycle,
	Claim: "the framework's own lifecycle constants are well-formed.",
	Rule: "REQUIREMENT_STATUS_LIFECYCLE, CONFLICT_LIFECYCLE, OPERATOR_LIFECYCLE, PROCESS_LIFECYCLE, and GOAL_LIFECYCLE " +
		"MUST each pass check_lifecycle_wellformed (no structural issues). This check runs on every invocation of the full " +
		"invariant suite -- the framework checks its own shipped state machines, not only user content.",
	Why: "self-application is the methodology's bootstrap test. If the framework's own lifecycles are malformed, all " +
		"downstream status validation is meaningless. References: R-statemachine-wellformedness, R-lifecycle-abstraction, " +
		"R-process-aspect-first.",
	Check: checkCanonicalLifecyclesWellformed,
})

func stakeholderToOperatorIDs(g *ontology.Graph) map[string][]string {
	result := map[string][]string{}
	for _, op := range g.Operators {
		result[op.Stakeholder] = append(result[op.Stakeholder], op.ID)
	}
	return result
}

func checkOperatorResolverNotSelf(g *ontology.Graph) []Violation {
	ownerOf := requirementOwnerMap(g)
	opByStakeholder := stakeholderToOperatorIDs(g)
	var out []Violation
	for _, c := range g.Conflicts {
		memberOwners := map[string]struct{}{}
		for _, m := range c.Members {
			if owner, ok := ownerOf[m]; ok {
				memberOwners[owner] = struct{}{}
			}
		}
		for sid := range memberOwners {
			for _, opID := range opByStakeholder[sid] {
				if c.Resolver == opID {
					out = append(out, Violation{
						Check: "check_operator_resolver_not_self",
						ID:    c.ID,
						Message: fmt.Sprintf(
							"Operator %q (acting facet of stakeholder %q) cannot resolver conflict %q because "+
								"its underlying Stakeholder owns a member requirement; M36 -- operator must not "+
								"self-approve (R-operator-not-self-approve)",
							opID, sid, c.ID),
					})
				}
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_operator_resolver_not_self", Invariant{
	Name:  "check_operator_resolver_not_self",
	Canon: methodology.Operator,
	Claim: "an Operator may not resolver a Conflict that contains its own Stakeholder's requirement.",
	Rule: "(M36): for each Conflict, collect the set of Stakeholder ids that own the conflict's member Requirements " +
		"('member-owners'). For each Operator whose stakeholder field is in that set, if any such Operator id equals the " +
		"Conflict's resolver, fire a Violation. An Operator is the acting facet of a Stakeholder; the resolver-distinct " +
		"boundary applies THROUGH that facet -- an Operator cannot resolver a Conflict in which its own underlying " +
		"Stakeholder owns one of the member Requirements.",
	Why: "(R-ai-presents-not-decides + R-operator-not-self-approve): the hard boundary that prevents an interested party " +
		"from judging its own side extends to the acting facet. If an Operator could resolver a conflict its Stakeholder has " +
		"a stake in, the boundary would be defeated at the operator level while formally satisfied at the Stakeholder level " +
		"-- structural invisibility. This is the reflexive twin of check_resolver_not_a_member_owner.",
	Check: checkOperatorResolverNotSelf,
})

// crystalPathForDomain resolves the resident-crystal path (root CLAUDE.md)
// for a domain using the established <repoRoot>/domains/<name> convention
// (see cmd/hotam/gen_spec.go's repoRootForDomain): when the domain
// directory's parent is literally "domains", the crystal root is the
// grandparent. It returns ok=false when the domain does not follow that
// layout OR DomainDir is unset -- the check then treats size as 0
// (already-documented graceful-absence behavior).
//
// Unlike repoRootForDomain, it deliberately does NOT fall back to a
// CWD-based project-root search: for an external --domain such a search
// resolves THIS framework's own root (or nothing at all), measuring an
// unrelated crystal and producing false violations/non-violations. The
// domain's own layout is the only input (task #108).
func crystalPathForDomain(domainDir string) (string, bool) {
	if domainDir == "" {
		return "", false
	}
	if filepath.Base(filepath.Dir(domainDir)) != "domains" {
		return "", false
	}
	return filepath.Join(filepath.Dir(filepath.Dir(domainDir)), "CLAUDE.md"), true
}

func checkOperatorWithinBudget(g *ontology.Graph) []Violation {
	var out []Violation
	for _, op := range g.Operators {
		limit := op.ContextBudget.Limit
		if limit <= 0 {
			continue
		}
		switch op.ContextBudget.Measure {
		case ontology.BudgetMeasureNODE_COUNT:
			size := len(g.Requirements) + len(g.Conflicts) + len(g.Assumptions)
			if size > limit {
				out = append(out, Violation{
					Check: "check_operator_within_budget",
					ID:    op.ID,
					Message: fmt.Sprintf(
						"operator %q holds %d nodes > budget %d (NODE_COUNT measure); crystallize first "+
							"(R-crystallize-before-split); if still over, spawn a sub-operator "+
							"(R-context-bounded-delegation)",
						op.ID, size, limit),
				})
			}
		case ontology.BudgetMeasureCRYSTAL_CHARS:
			size := 0
			if crystalPath, ok := crystalPathForDomain(g.DomainDir); ok {
				if data, err := os.ReadFile(crystalPath); err == nil {
					size = utf8.RuneCountInString(string(data))
				}
			}
			if size > limit {
				out = append(out, Violation{
					Check: "check_operator_within_budget",
					ID:    op.ID,
					Message: fmt.Sprintf(
						"operator %q resident crystal is %d chars > budget %d (CRYSTAL_CHARS measure); "+
							"crystallize first (R-crystallize-before-split); if still over, spawn a "+
							"sub-operator (R-context-bounded-delegation)",
						op.ID, size, limit),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_operator_within_budget", Invariant{
	Name:  "check_operator_within_budget",
	Canon: methodology.ContextBudget,
	Claim: "operator budget measure (NODE_COUNT nodes or CRYSTAL_CHARS resident crystal) must not exceed its budget limit.",
	Rule: "for each operator whose budget measure (NODE_COUNT nodes or CRYSTAL_CHARS resident crystal chars) exceeds its " +
		"budget limit, fire -- crystallize first; if still over, spawn a sub-operator. If measure == NODE_COUNT, size = " +
		"len(requirements) + len(conflicts) + len(assumptions) (full-graph count; DomainScope narrowing is deferred). If " +
		"measure == CRYSTAL_CHARS, size = character-length of the resident crystal (root CLAUDE.md) -- the resident working " +
		"set the operator re-loads by reference each boot (0 if CLAUDE.md is absent). If size > limit, fire. limit == 0 " +
		"means unbounded; the check is skipped for that operator.",
	Why: "WHY CRYSTAL_CHARS (replacing NODE_COUNT-as-substrate-proxy): NODE_COUNT measured the crystallized substrate " +
		"itself, which R-working-vs-substrate-budget declares FREE -- this falsely flagged operators as near-OVERLOADED " +
		"for the very act of crystallizing. CRYSTAL_CHARS measures the one thing that costs real working context: the " +
		"resident crystal (root CLAUDE.md) against the host's actual character ceiling. WHY fire (not warn): 'domain > " +
		"context' is exactly the kind of measurable, structural contradiction Hotam-Spec exists to surface. An over-budget " +
		"operator is a real conflict the graph holds visibly, not a soft warning.",
	Check: checkOperatorWithinBudget,
})
