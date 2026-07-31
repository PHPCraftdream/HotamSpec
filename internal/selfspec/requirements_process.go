// Origin: one-time bulk extraction by .scratch/selfspec-codegen (task #345, RAC-A) from
// domains/hotam-spec-self/graph.json. That program was a THROWAWAY (run once, never
// committed to git) and is NOT a regeneration path -- it went graph->Go, the OPPOSITE of
// today's sync direction. These literals are HAND-MAINTAINED Go source: edit them directly,
// then mirror Go->graph via `hotam sync-self` (dry-run by default; --confirm-hash to write).
// The "generated" framing describes this file's ORIGIN (task #345's one-time bulk pull from
// the pre-existing JSON graph), not an ongoing regeneration workflow.
//
// Topic: Process and Pipeline (staged behavior, gate passage).
//
// STRUCTURAL fields only: Claim, Why, Owner, Status, Relations, Assumptions,
// Enforcement, EnforcedBy, Enforceability, MTag, Summary, CreatedAt, SettledAt,
// BlockedOn, ImplementedBy, VerifiedBy, SourceRefs, DeclOrder. EVENT fields
// (History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are
// deliberately absent — MergeIntoGraph passes them through from the graph's
// existing node untouched.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-dependency-drives-parallel", ontology.Requirement{
	ID:             "R-dependency-drives-parallel",
	Claim:          "Independent sub-graphs in the dependency network may be delegated to parallel sub-operators.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-dependency-graph-parallelism (parallel concern). Independent components can run concurrently without coordination overhead.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-dependency-graph-parallelism"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestIndependentSubgraphs"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      139,
})

var _ = Requirements.MustRegister("R-dependency-drives-sequential", ontology.Requirement{
	ID:             "R-dependency-drives-sequential",
	Claim:          "Dependency chains in the network shall be processed sequentially.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-dependency-graph-parallelism (sequential concern). Coupled requirements need ordering to avoid stale inputs.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-dependency-graph-parallelism"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestDependencyChains"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      140,
})

var _ = Requirements.MustRegister("R-dependency-graph-parallelism", ontology.Requirement{
	ID:             "R-dependency-graph-parallelism",
	Claim:          "The system shall track the dependency network between requirements/operators/entities (building on Requirement.relations depends_on/supports/refines) so that independent sub-graphs may be delegated to PARALLEL sub-operators while dependency chains are processed SEQUENTIALLY.",
	Owner:          "framework-author",
	Status:         "REJECTED",
	Why:            "REJECTED — REPLACES split into R-dependency-tracked + R-dependency-drives-parallel + R-dependency-drives-sequential (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-dependency-tracked + R-dependency-drives-parallel + R-dependency-drives-sequential (wave 2, decided by framework-author 2026-06-30) — (was: SETTLED (P8): Requirement.relations (depends_on/supports/refines) is the live dependency network; the U‖/A‖/B‖ parallel commits demonstrate the principle operationally — independent sub-graphs ran in parallel, dependency chains ran sequentially. Parallel-vs-sequential is decided by the dependency topology (independent components vs chains), not guessed; this makes delegation sound. Implementation: hotam_spec.requirement.Relation + docs/playbooks/ + tools/what_now.py.))",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      42,
})

var _ = Requirements.MustRegister("R-dependency-tracked", ontology.Requirement{
	ID:             "R-dependency-tracked",
	Claim:          "The system shall track the dependency network between requirements via Requirement.relations (depends_on, refines).",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-dependency-graph-parallelism (tracking concern). Relations are the data that makes dependency-driven delegation possible. (D2, 2026-07-10: claim text updated -- 'supports' was merged into 'refines', so the vocabulary named here shrank from three kinds to two supportive kinds; 'replaces' is the separate anti-relitigation kind, not part of this dependency-network claim.)",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-dependency-graph-parallelism"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_no_dangling_requirement_relations"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-10",
	SourceRefs:     []string{},
	DeclOrder:      138,
})

var _ = Requirements.MustRegister("R-pipeline-live-state-from-typed-carriers", ontology.Requirement{
	ID:             "R-pipeline-live-state-from-typed-carriers",
	Claim:          "PIPELINE.md's current-status content (which stage a domain's requirements have reached, how many are SIGNED/DEFERRED at each declared gate stage, how many Conflicts are DECIDED/HELD/UNRESOLVED) MUST be generated from typed carriers ON EVERY `hotam gen-spec` run -- Requirement.gate_signoffs and Conflict lifecycle state, read live via internal/graphfacts's GateSignoffTally/GateFrontier/ConflictLifecycleTally -- NEVER carried as a point-in-time snapshot embedded in a Process's or Step's authored `why` prose. Authored `why` text holds DURABLE rationale only (why a stage exists, why this order, why this process matters) -- a live tally or a dated status claim frozen into that prose inevitably goes stale, because nothing regenerates prose the way `hotam gen-spec` regenerates a projection.",
	Owner:          "framework-author",
	Status:         "DRAFT",
	Why:            "Task #331 (R4-process-why), fourth external review: prat/gpsm-sm's Process.Why literally read \"27 из 32 ФТ... ТЕКУЩЕЕ ПОЛОЖЕНИЕ на 2026-07-21\" while the graph had since moved to 32/32 -- a real, confirmed contradiction between durable authored prose and live graph state, because the prose captured a snapshot nothing ever re-derived. The fix has two enforcement halves. (1) PIPELINE.md now renders a generated \"Live state\" section (internal/generator/pipeline.go's renderPipelineLiveState, called from BuildPipeline) placed BEFORE the first `## Process` section (R-domain-overview-projection's own \"where are we now\" before \"how does this work\" ordering) -- a pure function of graph.json state only (no `today`/date parameter, so gen-spec stays byte-reproducible run to run), rendering one line per gate_stage_order stage (SIGNED/DEFERRED tally via GateSignoffTally, honest \"not started\" beyond the GateFrontier) plus a Conflicts DECIDED/HELD/UNRESOLVED line via ConflictLifecycleTally, omitted entirely when the domain has no declared gate_stage_order and no Conflicts (honest no-op, not an empty placeholder). (2) A narrow, ADVISORY-ONLY lint (internal/invariants/process_why_snapshot.go's check_process_why_snapshot_prose, exported as ProcessWhySnapshotWarnings, wired into cmd/hotam's non-blocking ADVISORY section -- mirrors HonoredSkipWarnings' identical never-registered-in-All shape) flags a Process.Why or Step.Why text that either (a) co-occurs a fixed snapshot-marker phrase (\"текущее положение\" / \"по состоянию на\" / \"as of\" / \"current status\") with an ISO date, or (b) co-occurs a \"N из/of M\" tally with any stage token from the domain's OWN declared gate_stage_order. Deliberately scoped to Process.Why/Step.Why only (never Requirement.Why) and deliberately a small fixed pattern set, not a general number/date heuristic, verified against hotam-spec-self's own 253-requirement graph producing zero false positives. The actual gpsm-sm Process.Why migration (rewriting PR-gpsm-ott-delivery's stale text in the sibling PRAT-hotam repo) is intentionally OUT of this requirement's scope -- a separate follow-up task, not this engine-portion change.",
	Assumptions:    nil,
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-domain-overview-projection"}},
	Enforcement:    "PROSE",
	EnforcedBy:     nil,
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-23",
	SettledAt:      "",
	SourceRefs:     []string{"internal/generator/pipeline.go", "internal/invariants/authored_prose_snapshot.go", "internal/graphfacts/facts.go", "cmd/hotam/all_violations.go"},
	DeclOrder:      0,
	ImplementedBy:  []string{"internal/generator/pipeline.go:renderPipelineLiveState", "internal/invariants/authored_prose_snapshot.go:checkAuthoredProseSnapshot"},
	VerifiedBy:     []string{"internal/generator/pipeline_livestate_test.go:TestBuildPipeline_LiveStateRendersDedupedGateTallyAndConflictLifecycle", "internal/invariants/authored_prose_snapshot_test.go:TestCheckAuthoredProseSnapshot_FiresOnGpsmSmShapedSentence"},
})

var _ = Requirements.MustRegister("R-process-aspect-first", ontology.Requirement{
	ID:             "R-process-aspect-first",
	Claim:          "hotam_spec.process shall be the FIRST opt-in behavioral aspect — Lifecycle + Steps + roles_required + drives_entities — added after the keystone Lifecycle abstraction lands.",
	Owner:          "framework-author",
	Status:         "REJECTED",
	Why:            "REJECTED — REPLACES split into R-process-types-exist + R-process-opt-in + R-process-lifecycle-wellformed-aspect + R-process-roles-declared-aspect + R-process-goal-owner-is-operator-aspect + R-process-typed-anchors-extended (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-process-types-exist + R-process-opt-in + R-process-lifecycle-wellformed-aspect + R-process-roles-declared-aspect + R-process-goal-owner-is-operator-aspect + R-process-typed-anchors-extended (wave 2, decided by framework-author 2026-06-30) — (was: SETTLED (P9): hotam_spec/process.py ships Process + Step + Goal + TargetState + PROCESS_LIFECYCLE + GOAL_LIFECYCLE. The §Process aspect is opt-in (TensionGraph.processes defaults to empty). PR-closed-loop instantiates ONE worked example at the meta-domain level. Three new invariants enforce the behavioral surface: check_process_lifecycle_wellformed, check_process_roles_declared, and check_goal_owner_is_operator. check_typed_anchors extended for PR- and GOAL- prefixes. M12 resolved: Lifecycle is core; Process is the first opt-in aspect that proves the keystone supports new aspects without parallel machinery.))",
	Assumptions:    []string{"A-text-grounded-in-models", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"test_process.py", "check_process_lifecycle_wellformed", "check_process_roles_declared", "check_typed_anchors"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      27,
})

var _ = Requirements.MustRegister("R-process-drives-existing-entities", ontology.Requirement{
	ID:             "R-process-drives-existing-entities",
	Claim:          "Every entity slug referenced in a Process.drives_entities shall resolve to a declared EntityType slug in g.entity_types.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of §Process/§Entity coupling (referential integrity concern). A Process that references undeclared entity types is structurally broken — the coupling between the behavioral aspect and the entity aspect must be referentially sound.",
	Assumptions:    []string{"A-text-grounded-in-models", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_process_drives_existing_entities"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-06-30",
	SourceRefs:     []string{},
	DeclOrder:      136,
})

var _ = Requirements.MustRegister("R-process-goal-owner-is-operator-aspect", ontology.Requirement{
	ID:             "R-process-goal-owner-is-operator-aspect",
	Claim:          "Every Goal.owner shall reference an existing Operator.id, validated by check_goal_owner_is_operator.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-process-aspect-first (goal-owner concern). A Goal without a valid operator owner is unresolverable.",
	Assumptions:    []string{"A-text-grounded-in-models", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-process-aspect-first"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_goal_owner_is_operator"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-06-30",
	SourceRefs:     []string{},
	DeclOrder:      127,
})

var _ = Requirements.MustRegister("R-process-lifecycle-wellformed-aspect", ontology.Requirement{
	ID:             "R-process-lifecycle-wellformed-aspect",
	Claim:          "Every Process node shall have a well-formed lifecycle validated by check_process_lifecycle_wellformed.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-process-aspect-first (lifecycle wellformedness concern). A Process with an invalid lifecycle is structurally broken.",
	Assumptions:    []string{"A-text-grounded-in-models", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-process-aspect-first"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_process_lifecycle_wellformed"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-06-30",
	SourceRefs:     []string{},
	DeclOrder:      125,
})

var _ = Requirements.MustRegister("R-process-opt-in", ontology.Requirement{
	ID:             "R-process-opt-in",
	Claim:          "The Process aspect shall be opt-in: TensionGraph.processes defaults to an empty tuple.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-process-aspect-first (opt-in concern). Core cost must not be imposed on domains that do not model processes. ENFORCED 2026-07-13: TestProcessIsOptIn in internal/ontology/process_opt_in_test.go mechanically verifies this claim: a zero-value ontology.Graph defaults to an empty Processes slice (the Process aspect is opt-in; the loader injects no default Process).",
	Assumptions:    []string{"A-text-grounded-in-models", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-process-aspect-first"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestProcessIsOptIn"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      124,
})

var _ = Requirements.MustRegister("R-process-roles-declared-aspect", ontology.Requirement{
	ID:             "R-process-roles-declared-aspect",
	Claim:          "Every role referenced in a Process step shall be declared in the Process roles_required tuple.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-process-aspect-first (roles-declared concern). Undeclared roles are dangling references in the behavioral model.",
	Assumptions:    []string{"A-text-grounded-in-models", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-process-aspect-first"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_process_roles_declared"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-06-30",
	SourceRefs:     []string{},
	DeclOrder:      126,
})

var _ = Requirements.MustRegister("R-process-typed-anchors-extended", ontology.Requirement{
	ID:             "R-process-typed-anchors-extended",
	Claim:          "check_typed_anchors shall validate PR- and GOAL- prefixes for Process and Goal nodes.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-process-aspect-first (typed-anchors concern). Without prefix validation, Process and Goal nodes escape the anchoring discipline.",
	Assumptions:    []string{"A-text-grounded-in-models", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-process-aspect-first"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_typed_anchors_process", "check_typed_anchors_goal"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-06-30",
	SourceRefs:     []string{},
	DeclOrder:      128,
})

var _ = Requirements.MustRegister("R-process-types-exist", ontology.Requirement{
	ID:             "R-process-types-exist",
	Claim:          "internal/ontology/process.go shall define Process, Step, Goal, TargetState structs, plus ProcessLifecycle and GoalLifecycle canonical lifecycle variables.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-process-aspect-first (type-existence concern). All six types are defined in internal/ontology/process.go: Process (struct: ID, Lifecycle, Steps, RolesRequired, DrivesEntities, Why, DeclOrder), Step (struct: Name, RequiresRole, Invokes, Why), Goal (struct: ID, Owner, TargetState, Lifecycle, Why, DeclOrder), TargetState (struct: Kind, Predicate, Target), plus the canonical ProcessLifecycle (READY -> RUNNING -> DONE/ABANDONED) and GoalLifecycle (ACTIVE -> MET/ABANDONED) variables. Enforced by TestCanonicalLifecyclesSanity (internal/ontology/graph_smoke_test.go), which verifies every canonical lifecycle -- including ProcessLifecycle and GoalLifecycle -- is structurally well-formed (non-empty states, valid transition endpoints, reachable terminal, single INITIAL). These types are the behavioral surface of the first opt-in aspect. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-text-grounded-in-models", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-process-aspect-first"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestCanonicalLifecyclesSanity"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/ontology/process.go", "internal/ontology/graph_smoke_test.go"},
	DeclOrder:      123,
})

var _ = Requirements.MustRegister("R-step-invokes-known-transition", ontology.Requirement{
	ID:             "R-step-invokes-known-transition",
	Claim:          "Every Step.transition (when non-empty) shall name a transition event declared in the driven EntityType.lifecycle.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of §Process/§Entity coupling (step-transition concern). A Step that invokes an undeclared transition is a structural dead-end — the lifecycle machine cannot process it.",
	Assumptions:    []string{"A-text-grounded-in-models", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_step_invokes_known_transition"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-06-30",
	SourceRefs:     []string{},
	DeclOrder:      137,
})
