// Origin: one-time bulk extraction by .scratch/selfspec-codegen (task #345, RAC-A) from
// domains/hotam-spec-self/graph.json. That program was a THROWAWAY (run once, never
// committed to git) and is NOT a regeneration path -- it went graph->Go, the OPPOSITE of
// today's sync direction. These literals are HAND-MAINTAINED Go source: edit them directly,
// then mirror Go->graph via `hotam sync-self` (dry-run by default; --confirm-hash to write).
// The "generated" framing describes this file's ORIGIN (task #345's one-time bulk pull from
// the pre-existing JSON graph), not an ongoing regeneration workflow.
//
// Topic: Active Loop (State-Diagnosis-Action closed loop).
//
// STRUCTURAL fields only: Claim, Why, Owner, Status, Relations, Assumptions,
// Enforcement, EnforcedBy, Enforceability, MTag, Summary, CreatedAt, SettledAt,
// BlockedOn, ImplementedBy, VerifiedBy, SourceRefs, DeclOrder. EVENT fields
// (History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are
// deliberately absent — MergeIntoGraph passes them through from the graph's
// existing node untouched.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-active-loop-apply-tool", ontology.Requirement{
	ID:             "R-active-loop-apply-tool",
	Claim:          "A CLI command (`hotam apply-proposal`, `cmd/hotam`) shall consume an approved Proposed* JSON proposal and mechanically apply the change to the domain's graph.json (`domains/<name>/graph.json`) via `internal/proposal.Apply`, never by a hand-edit.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "Atom of R-active-loop-playbooks (tool concern). `hotam apply-proposal` (and the `hotam land` pipeline that wraps apply + gen-spec + all-violations) lands a resolver-approved JSON proposal into domains/<name>/graph.json and runs the regen+verify pipeline (internal/proposal + internal/generator + internal/invariants). The graph is plain JSON data (not source code spliced by AST), so applying a proposal is a JSON write, never an AST edit (previously implemented as tools/apply_proposal.py + spec/content/graph.py). settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-active-loop-playbooks"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestApply_Requirement_UpdateAppendsHistory"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"cmd/hotam/apply_proposal.go", "cmd/hotam/land.go", "internal/proposal/apply.go"},
	DeclOrder:      20,
})

var _ = Requirements.MustRegister("R-active-loop-playbook-doc", ontology.Requirement{
	ID:             "R-active-loop-playbook-doc",
	Claim:          "At least one band-specific playbook shall exist under docs/playbooks/ describing the agent's role for that band.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "Atom of R-active-loop-playbooks (documentation concern). docs/playbooks/P4-OPEN-ITEM.md is the first band playbook. ENFORCED 2026-07-13: TestActiveLoopPlaybook_AtLeastOneBandPlaybook in internal/selfcheck/playbooks_test.go mechanically verifies this claim.",
	Assumptions:    []string{},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-active-loop-playbooks"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestActiveLoopPlaybook_AtLeastOneBandPlaybook"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      21,
})

var _ = Requirements.MustRegister("R-active-loop-playbooks", ontology.Requirement{
	ID:             "R-active-loop-playbooks",
	Claim:          "Each what_now priority band shall have a documented agent PLAYBOOK plus a tools/apply_proposal.py that mechanically applies a resolver-approved JSON proposal to spec/content/.",
	Owner:          "ai-agent",
	Status:         "REJECTED",
	Why:            "REJECTED — REPLACES by R-active-loop-protocol + R-active-loop-apply-tool + R-active-loop-playbook-doc per atomicity discipline (R-requirement-claim-is-atomic). The original claim mixed three concerns: data-model, tool, documentation.",
	Assumptions:    []string{"A-stakeholders-care", "A-text-grounded-in-models"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      18,
})

var _ = Requirements.MustRegister("R-active-loop-protocol", ontology.Requirement{
	ID:             "R-active-loop-protocol",
	Claim:          "A set of Proposed* struct kinds (ProposedRequirement, ProposedConflictTransition, ProposedRejection, ProposedConflict, ProposedAssumptionTransition, et al.) shall exist in `internal/proposal` as the typed protocol for resolver-approved operator changes, decoded strictly (snake_case json tags, unknown fields rejected).",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "Atom of R-active-loop-playbooks (data-model concern). `internal/proposal/types.go` defines the Proposed* structs (12 kinds today: Requirement, ConflictTransition, Rejection, Conflict, OperatorBudget, Axis, Stakeholder, Assumption, AssumptionTransition, ConflictMemberUpdate, EntityType, ReviewMark). Each struct carries its own validate() (validate.go) and mutate() (mutate.go) that apply it to the graph; the original three-kind protocol grew as the graph gained first-class types.",
	Assumptions:    []string{"A-stakeholders-care", "A-text-grounded-in-models"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-active-loop-playbooks"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestProposedStructs_JSONTagsRoundTrip"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/proposal/types.go", "internal/proposal/mutate.go", "internal/proposal/validate.go"},
	DeclOrder:      19,
})

var _ = Requirements.MustRegister("R-crystallize-before-split", ontology.Requirement{
	ID:             "R-crystallize-before-split",
	Claim:          "On overload, an operator shall crystallize first, re-measure, and delegate (split) only if still over budget.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "SETTLED (P7): the order discipline is structurally bound. The apply-proposal mechanism (internal/proposal) crystallizes via Proposal types; the constitution (internal/generator/constitution.go, docs/gen/CONSTITUTION.md) names the ORDER explicitly. Splitting is for irreducible size, crystallizing is for un-offloaded knowledge; delegation is the lever of last resort. Splitting before crystallizing fragments knowledge that could have been freed in place. The over-budget REFLECTION finding (ReflectOverBudgetOperators, internal/diagnose/finding.go) renders the ordered imperative: 'crystallize first (R-crystallize-before-split); if still over, delegate a sub-domain (R-context-bounded-delegation).' The closure tool that would verify advancement before any split is considered is Planned (internal/methodology/tools_data.go). settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/proposal/apply.go", "internal/diagnose/finding.go", "internal/generator/constitution.go"},
	DeclOrder:      53,
})

var _ = Requirements.MustRegister("R-crystallize-knowledge-to-code", ontology.Requirement{
	ID:             "R-crystallize-knowledge-to-code",
	Claim:          "An operator shall continuously crystallize working knowledge into requirement-code (the substrate) as the offload instrument, since crystallized knowledge does not count against context.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "SETTLED (P4): the act of crystallization is now structurally supported. Every codified knowledge-piece flows through the proposal → apply → gen-spec → all-violations pipeline (internal/proposal Apply/ApplyBatch + cmd/hotam land.go). The discipline is made audit-able by that pipeline: each applied proposal is written atomically through applyToGraph (internal/proposal/apply.go), regenerated by gen-spec so docs cannot drift, and re-verified by all-violations before land reports success, so the WHAT is crystallized is not merely claimed but structurally verified at the feedback edge. The closure tool (per-action 'did the proposal remove its triggering diagnosis?') is Planned (internal/methodology/tools_data.go) — the land pipeline covers apply+regen+violation-check but does NOT yet run the per-action diagnosis-removal closure. STRUCTURAL (not ENFORCED) because WHAT to crystallize remains a resolver call; the pipeline asserts HOW it is done. Implementation: internal/proposal + cmd/hotam/land.go. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-compaction-loses-working"},
	Relations:      []ontology.Relation{},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/proposal/apply.go", "cmd/hotam/land.go", "internal/methodology/tools_data.go"},
	DeclOrder:      49,
})

var _ = Requirements.MustRegister("R-post-compact-regen-from-substrate", ontology.Requirement{
	ID:             "R-post-compact-regen-from-substrate",
	Claim:          "A PostCompact hook shall run `hotam gen-spec` after every auto-compact so the post-compact prompt reload reads fresh substrate-derived CLAUDE.md, not the stale pre-compact version.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Not yet implemented: the Go CLI (cmd/hotam) is a pure command-line tool with no host-hook / PostCompact integration, so there is no mechanism that automatically runs gen-spec after an auto-compact event. Auto-compact rewrites session context but does not re-read CLAUDE.md unless triggered. Without this hook, the operator post-compact runs on summary + stale CLAUDE.md. The equivalent would invoke `hotam gen-spec` from a host-side hook, but no such hook is wired. (Historical Python mechanism: a PostCompact hook in .claude/settings.local.json ran gen_spec.py.) PROSE (not ENFORCED): no enforcer exists. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"cmd/hotam/gen_spec.go"},
	DeclOrder:      184,
	BlockedOn:      "blocked on the setup_hooks tool (Planned) + a host PostCompact hook",
})

var _ = Requirements.MustRegister("R-verify-closure-per-action", ontology.Requirement{
	ID:             "R-verify-closure-per-action",
	Claim:          "After an applied proposal lands (write + regen), the system shall verify the action that triggered the proposal is no longer present in the post-apply what-now diagnosis.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "P4 -- the feedback edge that makes Drive (P5) safe to automate. Without per-action closure, an apply can technically land (tests green) yet the same diagnosis re-surface -- the tick would spin without advancing. Not yet implemented: a dedicated per-action closure tool. `closure` is Planned (not Implemented) in methodology.Tools. (Historical note: the original mechanism was tools/closure.py (check_closure), which asserted no Action with the original (kind, target) pair remained after a proposal apply; proven by test_closure.py.) The diagnosis half IS present: internal/diagnose (DiagnoseSignals / AllFindings, internal/diagnose/finding.go) emits the prioritized Finding list that `hotam what-now` renders, and `hotam land` (cmd/hotam/land.go) runs apply -> gen-spec -> all-violations (structural re-verification) on every proposal batch. The missing piece is the PER-ACTION comparison: a tool that captures the pre-apply Finding (Condition, Target) pair and asserts the post-apply diagnosis no longer contains it. `hotam what-now` / `hotam all-violations` serve as the manual closure check (the Mediation loop step 6 cites both for closure verification, R-verify-closure-per-action), but no automated diff-and-assert tool exists. The original WHY stands: per-action closure prevents an apply from landing technically green while the same diagnosis re-surfaces. PROSE (not ENFORCED): no enforcer exists. ENFORCED 2026-07-13: TestApply_ThenDiagnose_ClosesTriggeringAction in internal/proposal/closure_test.go mechanically verifies this claim.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestApply_ThenDiagnose_ClosesTriggeringAction"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/diagnose/finding.go", "cmd/hotam/land.go"},
	DeclOrder:      50,
})
