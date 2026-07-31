// Origin: one-time bulk extraction by .scratch/selfspec-codegen (task #345, RAC-A) from
// domains/hotam-spec-self/graph.json. That program was a THROWAWAY (run once, never
// committed to git) and is NOT a regeneration path -- it went graph->Go, the OPPOSITE of
// today's sync direction. These literals are HAND-MAINTAINED Go source: edit them directly,
// then mirror Go->graph via `hotam sync-self` (dry-run by default; --confirm-hash to write).
// The "generated" framing describes this file's ORIGIN (task #345's one-time bulk pull from
// the pre-existing JSON graph), not an ongoing regeneration workflow.
//
// Topic: Goal (first-class target-state type).
//
// STRUCTURAL fields only: Claim, Why, Owner, Status, Relations, Assumptions,
// Enforcement, EnforcedBy, Enforceability, MTag, Summary, CreatedAt, SettledAt,
// BlockedOn, ImplementedBy, VerifiedBy, SourceRefs, DeclOrder. EVENT fields
// (History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are
// deliberately absent — MergeIntoGraph passes them through from the graph's
// existing node untouched.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-goal-as-target-state", ontology.Requirement{
	ID:             "R-goal-as-target-state",
	Claim:          "A Goal shall be a desired target-state predicate; the Gap = (Goal - current state) is the work that drives a Process.",
	Owner:          "domain-user",
	Status:         "REJECTED",
	Why:            "REJECTED — REPLACES by R-goal-is-first-class-type + R-goal-target-kind-known + R-goal-owner-is-operator per atomicity discipline (R-requirement-claim-is-atomic). The original claim was mostly atomic but its enforced_by tuple covered three distinct rules.",
	Assumptions:    []string{"A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      37,
})

var _ = Requirements.MustRegister("R-goal-is-first-class-type", ontology.Requirement{
	ID:             "R-goal-is-first-class-type",
	Claim:          "Goal shall be its own first-class struct type (not a Requirement facet) with typed anchor 'GOAL-'.",
	Owner:          "domain-user",
	Status:         "SETTLED",
	Why:            "Atom of R-goal-as-target-state (type identity concern). Goal is defined as a dedicated struct in internal/ontology/process.go (fields: ID, Owner, TargetState, Lifecycle, Why, DeclOrder), distinct from the Requirement struct -- it is not a facet or field of Requirement. The canonical lifecycle is GoalLifecycle (internal/ontology/process.go: ACTIVE -> MET / ABANDONED). The typed-anchor discipline is enforced by check_typed_anchors_goal (internal/invariants/typed_anchors.go), which verifies every Goal.ID starts with 'GOAL-'. M19 resolved. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-goal-as-target-state"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_typed_anchors_goal"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/ontology/process.go", "internal/invariants/typed_anchors.go"},
	DeclOrder:      38,
})

var _ = Requirements.MustRegister("R-goal-owner-is-operator", ontology.Requirement{
	ID:             "R-goal-owner-is-operator",
	Claim:          "Goal.owner shall reference an existing Operator.id.",
	Owner:          "domain-user",
	Status:         "SETTLED",
	Why:            "Atom of R-goal-as-target-state (ownership concern). check_goal_owner_is_operator and check_no_dangling_ids validate.",
	Assumptions:    []string{"A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-goal-as-target-state"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_goal_owner_is_operator", "check_no_dangling_operator_refs"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-06-30",
	SourceRefs:     []string{},
	DeclOrder:      40,
})

var _ = Requirements.MustRegister("R-goal-target-kind-known", ontology.Requirement{
	ID:             "R-goal-target-kind-known",
	Claim:          "Goal.target_state.kind shall be one of the declared TARGET_KINDS.",
	Owner:          "domain-user",
	Status:         "SETTLED",
	Why:            "Atom of R-goal-as-target-state (target-kind concern). check_goal_target_kind_known validates.",
	Assumptions:    []string{"A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-goal-as-target-state"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_goal_target_kind_known"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-06-30",
	SourceRefs:     []string{},
	DeclOrder:      39,
})

var _ = Requirements.MustRegister("R-goal-type-vs-facet", ontology.Requirement{
	ID:             "R-goal-type-vs-facet",
	Claim:          "Goal shall be its own first-class struct type (not a Requirement facet), with typed anchor 'GOAL-' and its own GoalLifecycle.",
	Owner:          "domain-user",
	Status:         "SETTLED",
	Why:            "M19. DECIDED 2026-06-30 (already recorded in old M-table as DECIDED P9): Goal is a new type, not a Requirement facet. Rationale: the Gap = (Goal - current state) is semantically distinct from a static Requirement claim; a Requirement facet would lose that target-state semantics and the burn-down-to-zero pattern. Goal is a dedicated struct in internal/ontology/process.go (internal/ontology/process.go:Goal), with its own GoalLifecycle (ACTIVE -> MET / ABANDONED). R-goal-is-first-class-type is SETTLED and enforced by check_typed_anchors_goal (internal/invariants/typed_anchors.go), which verifies every Goal.ID starts with 'GOAL-'. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_typed_anchors_goal"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/ontology/process.go", "internal/invariants/typed_anchors.go"},
	DeclOrder:      61,
})
