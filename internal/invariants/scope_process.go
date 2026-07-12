package invariants

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PHPCraftdream/HotamSpecGo/internal/methodology"
	"github.com/PHPCraftdream/HotamSpecGo/internal/ontology"
)

type scopeView struct {
	requirementIDs map[string]struct{}
	conflictIDs    map[string]struct{}
}

func projectScope(g *ontology.Graph, prefixes []string) scopeView {
	var v scopeView
	if len(prefixes) == 0 {
		return v
	}
	v.requirementIDs = map[string]struct{}{}
	v.conflictIDs = map[string]struct{}{}
	for _, p := range prefixes {
		for _, r := range g.Requirements {
			if strings.HasPrefix(r.ID, p) {
				v.requirementIDs[r.ID] = struct{}{}
			}
		}
		for _, c := range g.Conflicts {
			if strings.HasPrefix(c.ID, p) {
				v.conflictIDs[c.ID] = struct{}{}
			}
		}
	}
	return v
}

func scopeOverlapNodeIDs(a, b scopeView) []string {
	out := map[string]struct{}{}
	for id := range a.requirementIDs {
		if _, ok := b.requirementIDs[id]; ok {
			out[id] = struct{}{}
		}
	}
	for id := range a.conflictIDs {
		if _, ok := b.conflictIDs[id]; ok {
			out[id] = struct{}{}
		}
	}
	result := make([]string, 0, len(out))
	for id := range out {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func presenterForNode(operatorIDs []string) string {
	if len(operatorIDs) == 0 {
		return ""
	}
	first := operatorIDs[0]
	for _, id := range operatorIDs[1:] {
		if id < first {
			first = id
		}
	}
	return first
}

func checkScopedNodeHasSinglePresenter(g *ontology.Graph) []Violation {
	var scopedOps []ontology.Operator
	for _, op := range g.Operators {
		if len(op.Scope) > 0 {
			scopedOps = append(scopedOps, op)
		}
	}
	if len(scopedOps) < 2 {
		return nil
	}
	views := make([]scopeView, len(scopedOps))
	for i, op := range scopedOps {
		views[i] = projectScope(g, op.Scope)
	}
	contested := map[string]map[string]struct{}{}
	for i := 0; i < len(scopedOps); i++ {
		for j := i + 1; j < len(scopedOps); j++ {
			for _, nodeID := range scopeOverlapNodeIDs(views[i], views[j]) {
				if contested[nodeID] == nil {
					contested[nodeID] = map[string]struct{}{}
				}
				contested[nodeID][scopedOps[i].ID] = struct{}{}
				contested[nodeID][scopedOps[j].ID] = struct{}{}
			}
		}
	}
	var nodeIDs []string
	for k := range contested {
		nodeIDs = append(nodeIDs, k)
	}
	sort.Strings(nodeIDs)
	var out []Violation
	for _, nodeID := range nodeIDs {
		opSet := contested[nodeID]
		opIDs := make([]string, 0, len(opSet))
		for opID := range opSet {
			opIDs = append(opIDs, opID)
		}
		sort.Strings(opIDs)
		if presenterForNode(opIDs) == "" {
			out = append(out, Violation{
				Check:   "check_scoped_node_has_single_presenter",
				ID:      nodeID,
				Message: fmt.Sprintf("node %q is contested by operators %v but no presenter could be determined; declare a presenter (R-overlap-single-presenter)", nodeID, opIDs),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_scoped_node_has_single_presenter", Invariant{
	Name:  "check_scoped_node_has_single_presenter",
	Canon: methodology.Scope,
	Claim: "every node in >=2 operators' scope overlap has exactly one presenter.",
	Rule: "a node contested by two or more operators must resolve to exactly one presenter. Mechanically: compute " +
		"each Operator's ScopeView via prefix-projection over the graph for every operator whose scope tuple is " +
		"non-empty. For each unordered pair of such operators, compute the overlap node ids (the union of overlapping " +
		"Requirement and Conflict ids). For every node id that appears in >= 1 pairwise overlap, the full set of " +
		"operators whose scope contains that node id is passed to presenterForNode, which deterministically returns " +
		"the LEXICOGRAPHICALLY FIRST operator id. presenterForNode is total and deterministic for any non-empty " +
		"operator-id set, so a violation can only mean the contested-operator set was empty, which cannot happen " +
		"once a node has been found in a pairwise overlap. This check therefore currently reports NOTHING as a " +
		"defect -- it exists to make single-presentership PROVABLE.",
	Why: "with the current graph (one OP-director, scope=()), no operator has a non-empty scope, so there is nothing " +
		"to overlap. The invariant is still load-bearing: the moment a second operator is spawned with an overlapping " +
		"scope (R-context-bounded-delegation), this is the check that guarantees the overlap gets exactly one " +
		"presenter instead of two operators silently disagreeing about who speaks for a shared node. " +
		"WHY lexicographic-first: id order is stable under source reformatting; graph position and parent-hierarchy " +
		"are not guaranteed total orders across arbitrary Operator sets.",
	Check: checkScopedNodeHasSinglePresenter,
})

func checkProcessLifecycleWellformed(g *ontology.Graph) []Violation {
	var out []Violation
	for _, p := range g.Processes {
		for _, issue := range lifecycleWellformedIssues(p.Lifecycle) {
			out = append(out, Violation{
				Check:   "check_process_lifecycle_wellformed",
				ID:      p.ID,
				Message: issue,
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_process_lifecycle_wellformed", Invariant{
	Name:  "check_process_lifecycle_wellformed",
	Canon: methodology.Process,
	Claim: "every Process lifecycle is structurally well-formed.",
	Rule: "for each Process in g.processes, run lifecycleWellformedIssues(p.lifecycle); any issues become " +
		"Violations. No-ops when g.processes is empty (Process aspect not loaded).",
	Why: "the Lifecycle keystone is the single source of truth for state-machine well-formedness. Reusing the shared " +
		"lifecycle checker here means the Process aspect inherits all four lifecycle conditions (non-empty, single " +
		"INITIAL, valid transition endpoints, terminal reachable) without parallel machinery. " +
		"References: R-statemachine-wellformedness, M12.",
	Check: checkProcessLifecycleWellformed,
})

func sortedSlugSet(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func checkProcessDrivesExistingEntities(g *ontology.Graph) []Violation {
	slugs := ontology.EntityTypeSlugs(g)
	var out []Violation
	for _, p := range g.Processes {
		for _, slug := range p.DrivesEntities {
			if _, ok := slugs[slug]; !ok {
				out = append(out, Violation{
					Check:   "check_process_drives_existing_entities",
					ID:      p.ID,
					Message: fmt.Sprintf("drives_entities slug %q is not a declared EntityType.slug (declared: %v)", slug, sortedSlugSet(slugs)),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_process_drives_existing_entities", Invariant{
	Name:  "check_process_drives_existing_entities",
	Canon: methodology.Process,
	Claim: "every Process.drives_entities slug resolves to a declared EntityType.",
	Rule: "each entity slug in Process.drives_entities MUST be a declared EntityType.slug in g.entity_types. " +
		"Activates the forward-compat seam Process declared from day one.",
	Why: "a Process driving an undeclared entity slug is a structural dead-end the harness must surface. " +
		"Reference: R-process-drives-existing-entity.",
	Check: checkProcessDrivesExistingEntities,
})

func checkStepInvokesKnownTransition(g *ontology.Graph) []Violation {
	typeBySlug := map[string]ontology.EntityType{}
	for _, et := range g.EntityTypes {
		typeBySlug[et.Slug] = et
	}
	var out []Violation
	for _, p := range g.Processes {
		for _, step := range p.Steps {
			if step.Invokes == "" {
				continue
			}
			if !strings.Contains(step.Invokes, ".") {
				out = append(out, Violation{
					Check:   "check_step_invokes_known_transition",
					ID:      p.ID,
					Message: fmt.Sprintf("step %q.invokes=%q must be '<entity-slug>.<event>'", step.Name, step.Invokes),
				})
				continue
			}
			parts := strings.SplitN(step.Invokes, ".", 2)
			slug, event := parts[0], parts[1]
			et, ok := typeBySlug[slug]
			if !ok {
				out = append(out, Violation{
					Check:   "check_step_invokes_known_transition",
					ID:      p.ID,
					Message: fmt.Sprintf("step %q.invokes=%q -- unknown entity %q", step.Name, step.Invokes, slug),
				})
				continue
			}
			events := map[string]struct{}{}
			for _, t := range et.Lifecycle.Transitions {
				events[t.Event] = struct{}{}
			}
			if _, ok := events[event]; !ok {
				known := make([]string, 0, len(events))
				for e := range events {
					known = append(known, e)
				}
				sort.Strings(known)
				out = append(out, Violation{
					Check:   "check_step_invokes_known_transition",
					ID:      p.ID,
					Message: fmt.Sprintf("step %q.invokes=%q -- event %q is not a transition of %q (known: %v)", step.Name, step.Invokes, event, slug, known),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_step_invokes_known_transition", Invariant{
	Name:  "check_step_invokes_known_transition",
	Canon: methodology.Process,
	Claim: "every non-empty Step.invokes resolves to a real transition of a declared EntityType.",
	Rule: "when Step.invokes is non-empty, it MUST have format '<entity-slug>.<event>' where entity-slug is a " +
		"declared EntityType.slug AND event matches a Transition.event in that EntityType.lifecycle. One relation " +
		"checked via three progressive validation gates: format has a dot, entity-slug resolves, event resolves -- " +
		"each a precondition of the next, not three independent rules.",
	Why: "Step.invokes was prose-only while Entity was deferred. With Entity landed, the verb a Step invokes is a " +
		"Lifecycle transition, making process steps and entity state machines structurally coupled. " +
		"Reference: R-step-invokes-known-transition.",
	Check: checkStepInvokesKnownTransition,
})

func checkProcessRolesDeclared(g *ontology.Graph) []Violation {
	var out []Violation
	for _, p := range g.Processes {
		declared := map[string]struct{}{}
		for _, r := range p.RolesRequired {
			declared[r] = struct{}{}
		}
		for _, s := range p.Steps {
			if _, ok := declared[s.RequiresRole]; !ok {
				out = append(out, Violation{
					Check:   "check_process_roles_declared",
					ID:      p.ID,
					Message: fmt.Sprintf("step %q requires role %q which is not in Process.roles_required %v; declare it explicitly (no implicit roles)", s.Name, s.RequiresRole, sortedSlugSet(declared)),
				})
			}
		}
	}
	return out
}

var _ = All.MustRegister("check_process_roles_declared", Invariant{
	Name:  "check_process_roles_declared",
	Canon: methodology.Process,
	Claim: "every Step.requires_role is declared in its Process.roles_required.",
	Rule: "for each Process p and each Step s in p.steps, s.requires_role MUST be in p.roles_required. A Step that " +
		"demands a role not declared in the Process is a structural dead-end (the 'missing actor' contradiction). " +
		"No-ops when g.processes is empty.",
	Why: "an undeclared role is invisible -- the Process claims to need an actor it has never introduced. Supply " +
		">= demand is checked here; who fulfills each role is a future actor-matching invariant.",
	Check: checkProcessRolesDeclared,
})

func checkGoalTargetKindKnown(g *ontology.Graph) []Violation {
	var out []Violation
	for _, go_ := range g.Goals {
		if _, ok := ontology.TargetKinds[go_.TargetState.Kind]; !ok {
			out = append(out, Violation{
				Check:   "check_goal_target_kind_known",
				ID:      go_.ID,
				Message: fmt.Sprintf("Goal.target_state.kind %q is not in TARGET_KINDS %v; use one of the declared kind constants", go_.TargetState.Kind, sortedSlugSet(ontology.TargetKinds)),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_goal_target_kind_known", Invariant{
	Name:  "check_goal_target_kind_known",
	Canon: methodology.Goal,
	Claim: "every Goal.target_state.kind is in TARGET_KINDS.",
	Rule: "for each Goal in g.goals, target_state.kind MUST be in TARGET_KINDS (GRAPH_PROPERTY | BUSINESS_STATE | " +
		"ENTITY_STATE). An unknown kind is a misconfiguration that breaks the kind discriminant used by future " +
		"machine-checkable predicates. No-ops when g.goals is empty.",
	Why: "the kind field future-proofs Goal for machine-checkable predicates -- the same seam as " +
		"Assumption.machine_check. An unchecked kind lets two Goals with incompatible target types form a Conflict " +
		"that the invariant surface can never detect.",
	Check: checkGoalTargetKindKnown,
})

func checkGoalOwnerIsOperator(g *ontology.Graph) []Violation {
	oids := ontology.OperatorIDs(g)
	var out []Violation
	for _, go_ := range g.Goals {
		if _, ok := oids[go_.Owner]; !ok {
			out = append(out, Violation{
				Check:   "check_goal_owner_is_operator",
				ID:      go_.ID,
				Message: fmt.Sprintf("Goal.owner %q is not a known Operator id; a Goal must be owned by an Operator (the acting facet that pursues it) -- M19", go_.Owner),
			})
		}
	}
	return out
}

var _ = All.MustRegister("check_goal_owner_is_operator", Invariant{
	Name:  "check_goal_owner_is_operator",
	Canon: methodology.Goal,
	Claim: "every Goal.owner resolves to a known Operator.",
	Rule: "for each Goal in g.goals, Goal.owner MUST be in operator_ids(g). A Goal with a dangling owner is a " +
		"structurally invisible target -- no acting facet pursues it. No-ops when g.goals is empty.",
	Why: "a Goal drives a Process; Processes are executed by Operators (the acting facets). A Stakeholder without " +
		"an Operator cannot run steps -- the Goal would be declared but unexecuted. Referential integrity at the " +
		"behavioral altitude. Reference: M19.",
	Check: checkGoalOwnerIsOperator,
})
