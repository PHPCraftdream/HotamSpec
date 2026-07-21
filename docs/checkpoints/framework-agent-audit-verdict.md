# Checkpoint — framework-agent-audit-verdict

**Topic checkpoint, not session state.** Preserves the per-R / per-check
verdicts from the framework-agent atomization audit that ran this session.
The audit returned as a task-notification — without this file, those
detailed verdicts would be lost on auto-compact.

**Honesty mark.** This is reconstructed from the operator's working
context, NOT from a re-run of the audit tool (which does not yet exist —
see "Build trigger" at bottom). Counts and verdicts are accurate to the
operator's recall; if a specific verdict is needed authoritatively,
re-run the (not-yet-built) `tools/audit_atomicity.py`.

## Audit corpus

- 42 SETTLED requirements at the time of audit (before this session's
  af051e8 wave).
- 21 `check_*` invariants in `ALL_INVARIANTS` at the time of audit.
- M-table bijection (CLAUDE.md ↔ `Requirement.m_tag`).
- Orphan analysis (checks with no claiming R; R's with non-enforcer
  entries in `enforced_by`).
- 6 missing-requirement proposals (G1-G6).
- 4 additional gaps surfaced incidentally (G7-G10).
- 10 director-irreducible decisions.

## SETTLED requirements — verdict table (42 entries, pre-af051e8 baseline)

### ATOMIC (24 — kept as-is)

| id | one-line rationale |
|----|--------------------|
| R-agent-never-lost | one claim: dropped agent → deterministic next action |
| R-drift-structurally-impossible | one claim: regen == committed byte-for-byte |
| R-conflict-is-connector-node | one ontological rule: Conflict = NODE not edge |
| R-deterministic-generation | one rule: two runs over unchanged graph = identical bytes |
| R-ai-presents-not-decides | single hard-boundary claim |
| R-resolver-distinct-from-owners | single structural rule: resolver ∉ member-owner |
| R-open-states-question | one rule: OPEN must carry non-empty question |
| R-rejected-preserved-not-deleted | one rule: REJECTED kept, never deleted |
| R-axis-controlled-vocab | one rule: Conflict.axis must be declared Axis slug |
| R-stable-conflict-identity | one rule: id = conflict_identity(axis, context) |
| R-decided-needs-human-signoff | one rule: DECIDED must carry decided_by |
| R-context-budget-rule | one rule: size(domain) <= budget.limit |
| R-operator-not-self-approve | one rule: Operator ≠ resolver of own-stakeholder conflict |
| R-delegation-conclusions-only | one rule: sub-operator returns conclusions, not raw |
| R-context-bounded-delegation | one rule: overloaded operator relieved by sub-operator |
| R-task-vs-action-distinct-altitudes | one discipline: Task and Action stay separable |
| R-crystallize-knowledge-to-code | one rule: working knowledge → substrate continuously |
| R-verify-closure-per-action | one rule: triggering action gone post-apply |
| R-anchor-everything | one rule: every object has stable typed anchor |
| R-speak-by-reference | one rule: every assertion cites ≥1 anchor |
| R-crystallize-before-split | one rule: crystallize first, delegate only if still over |
| R-working-vs-substrate-budget | one rule: budget bounds working store only |
| R-requirement-enforced | one claim: SETTLED w/o enforcer = UNENFORCED |
| R-uncrystallizable-is-missing-type | one rule: uncrystallizable → record as missing-type signal |
| R-stale-substrate | one rule: dead enforcing assumption → surface as stale |
| R-claude-md-live-state-generated | one rule: LIVE-STATE block is generated, never hand-written |

### COMPOUND (17 — decomposition proposed)

#### Already atomized this session (4 — see commit af051e8)

| original | atoms after | enforcers split |
|----------|-------------|------------------|
| R-active-loop-playbooks | R-active-loop-protocol + R-active-loop-apply-tool + R-active-loop-playbook-doc | proposal type ↔ apply tool ↔ playbook doc |
| R-statemachine-wellformedness | R-statemachine-reachable + R-statemachine-deterministic + R-statemachine-terminal-or-cyclic + R-statemachine-guard-on-assumption | 4 lifecycle structural rules |
| R-operator-acting-facet | R-operator-is-frozen-dataclass + R-operator-references-stakeholder + R-operator-has-context-budget + R-operator-may-have-parent | 4 Operator structural properties |
| R-goal-as-target-state | R-goal-is-first-class-type + R-goal-target-kind-known + R-goal-owner-is-operator | 3 Goal structural rules |

#### Still compound — proposed decomposition (13 remaining)

| original | proposed atoms | rationale |
|----------|----------------|-----------|
| R-content-free-framework | R-content-free-no-business-data + (R-content-free-no-examples) | rule + illustrative list; DIRECTOR DECISION (D1) — recommend keep ATOMIC |
| R-empty-content-is-legitimate | R-empty-content-wellformed + R-empty-content-calm-banner + R-empty-content-gen-notice | 3 observable behaviors; DIRECTOR DECISION (D2) — recommend SPLIT |
| R-boot-from-substrate | R-boot-reload-three-facts + R-boot-cite-in-first-sentence | WHAT to load + WHEN to cite |
| R-glossary-sync-test | R-glossary-generated + R-glossary-sync-fails-dead + R-glossary-sync-fails-unused + R-glossary-drift-stable | 4 independently testable conditions |
| R-history-from-rejected-markers | R-history-generated-from-rejected + R-history-generated-from-decided | 2 source streams |
| R-lifecycle-abstraction | R-lifecycle-type-exists + R-lifecycle-validates-requirement + R-lifecycle-validates-conflict | type + 2 distinct validations |
| R-process-aspect-first | R-process-types-exist + R-process-opt-in + R-process-lifecycle-wellformed + R-process-roles-declared + R-process-goal-owner-is-operator + R-process-typed-anchors-extended | 6 distinct invariants bundled |
| R-dependency-graph-parallelism | R-dependency-tracked + R-dependency-drives-parallel + R-dependency-drives-sequential | 3 claims about the dependency graph |
| R-operator-crystal-is-claude-md | R-crystal-is-claude-md + R-crystal-reload-by-reference + R-crystal-tree-hierarchy | 3 independent claims |
| R-enforcement-gradient | R-enforcement-levels-declared + R-enforced-names-enforcer | 2 sub-rules conjoined with "and" |
| R-critical-core-scope | R-critical-core-methodology + R-critical-core-per-domain | 2 distinct scope claims |
| R-decided-conflict-justifies-itself (NEW from this session) | (already ATOMIC by construction) | added af051e8 |
| R-m-tag-format-valid (NEW from this session) | (compound check it claims) | added af051e8 — claim is atomic but the check it enforces is compound (3 sub-rules) |

### AMBIGUOUS (1 — director judgment)

| id | issue | director recommendation |
|----|-------|-------------------------|
| R-two-altitude-ontology | claim asserts reflexive isomorphism; WHY admits "PROSE-enforced only" — implicit compound of assertion + enforcement promise | D3: keep ATOMIC, DOWNGRADE enforcement STRUCTURAL → PROSE (currently lying about structural enforcement) |

## check_* invariants — verdict table (21 entries)

### ATOMIC (11 — kept as-is)

`check_conflict_min_two_members`, `check_axis_in_registry`, `check_conflict_id_matches_identity`, `check_resolver_not_a_member_owner`, `check_open_has_question`, `check_operator_resolver_not_self`, `check_operator_within_budget`, `check_process_lifecycle_wellformed`, `check_process_roles_declared`, `check_goal_target_kind_known`, `check_goal_owner_is_operator`, `check_section_anchors_known`.

(Note: 12 names listed — `check_section_anchors_known` was added later in P5/P6 and the audit ran against earlier state; treat as atomic-by-construction.)

### COMPOUND (10 — split proposed)

| check_* | sub-checks | maps to R-… (after split) |
|---------|-----------|---------------------------|
| check_no_dangling_ids | 12 sub-checks (one per reference type: assumption-owner, requirement-owner, requirement-assumptions, relation-kind, relation-target, conflict-resolver, conflict-members, conflict-shared-assumption, conflict-derived, conflict-decided-by, operator-stakeholder, operator-parent) | each → corresponding new atomic R; all roll up under R-agent-never-lost + R-operator-acting-facet's atomized successors |
| check_conflict_has_axis_context_resolver | 3 sub-checks (axis + context + resolver) | R-conflict-structurally-visible (added af051e8) |
| check_decided_has_rationale_or_derived | 2 sufficiency conditions (rationale OR derived) | R-decided-conflict-justifies-itself (added af051e8) |
| check_decided_has_decided_by | 3 sub-checks (non-empty + known-stakeholder + not-member-owner) | R-decided-needs-human-signoff (compound enforcer for atomic R) |
| check_typed_anchors | 6 sub-checks (one per prefix: R-/A-/C-/OP-/PR-/GOAL-) | R-anchor-everything |
| check_enforced_names_invariant | 2 sub-checks (level-valid + enforced-has-enforcer) | R-enforcement-gradient (which is itself compound — will split with it) |
| check_m_tag_format | 3 sub-checks (regex-format + uniqueness + OPEN-only) | R-m-tag-format-valid (added af051e8) |
| check_status_in_lifecycle | 4 sub-checks (requirement-status + conflict-lifecycle + operator-lifecycle + goal-lifecycle) | R-lifecycle-abstraction (compound, will split with it) + R-goal-as-target-state's atom |
| check_canonical_lifecycles_wellformed | 5 sub-checks (one per canonical lifecycle: requirement-status, conflict, operator, process, goal) | R-statemachine-* atoms (added af051e8) |
| `(orphan-from-earlier)` check_lifecycle_wellformed | helper, not a check_* in ALL_INVARIANTS directly; called by check_canonical_lifecycles_wellformed | n/a — helper function |

## Bijection analysis (highlights, not full table)

### Clean one-to-one (8 pairs at audit time)

R-resolver-distinct-from-owners ↔ check_resolver_not_a_member_owner; R-open-states-question ↔ check_open_has_question; R-axis-controlled-vocab ↔ check_axis_in_registry; R-stable-conflict-identity ↔ check_conflict_id_matches_identity; R-decided-needs-human-signoff ↔ check_decided_has_decided_by; R-operator-not-self-approve ↔ check_operator_resolver_not_self; R-context-budget-rule ↔ check_operator_within_budget; R-verify-closure-per-action ↔ check_closure + test_closure.

### Many enforcers per R (healthy redundancy — kept) (~7 cases)

R-drift-structurally-impossible has 4 docs × 1 test each; R-empty-content-is-legitimate has what_now + gen_spec tests; R-deterministic-generation has 1 dedicated test; R-content-free-framework has test_content_free; R-glossary-sync-test has 2 tests; R-history-from-rejected-markers has 2 tests; etc.

### One enforcer covering many R's (compound check) (6 cases)

`check_no_dangling_ids` covers R-operator-acting-facet (+ implicit structural integrity many); `check_typed_anchors` covers R-anchor-everything + R-operator-acting-facet + R-process-aspect-first + R-goal-as-target-state; `check_status_in_lifecycle` covers R-lifecycle-abstraction + R-goal-as-target-state; `check_enforced_names_invariant` covers R-enforcement-gradient + R-requirement-enforced; `check_section_anchors_known` covers R-anchor-everything + R-speak-by-reference; `test_conscience.py` covers R-critical-core-scope + R-uncrystallizable-is-missing-type + R-stale-substrate.

### Orphan checks (cited by no SETTLED R's enforced_by) — now anchored (af051e8)

| check_* | new R created (af051e8) |
|---------|-------------------------|
| check_conflict_has_axis_context_resolver | R-conflict-structurally-visible |
| check_conflict_min_two_members | R-conflict-min-two-members |
| check_decided_has_rationale_or_derived | R-decided-conflict-justifies-itself |
| check_m_tag_format | R-m-tag-format-valid |

### Orphan-by-fake-enforcer requirements (12) — now cleaned (af051e8)

R-boot-from-substrate (had: CLAUDE.md section + gen doc); R-delegation-conclusions-only (had: source + tool + docs/playbooks/); R-task-vs-action-distinct-altitudes (had: type name + gen doc + docs); R-context-bounded-delegation (had: band constant + docs); R-dependency-graph-parallelism (had: type + docs + tool); R-operator-crystal-is-claude-md (had: 2 gen docs); R-crystallize-knowledge-to-code (had: 2 tools + docs); R-crystallize-before-split (had: 2 tools + gen doc); R-working-vs-substrate-budget (had: 2 tools + 1 test → kept test); R-active-loop-playbooks (had: 2 tests + playbook doc → kept tests); R-critical-core-scope (had: test + code constant → kept test); R-claude-md-live-state-generated (had: test + generator → kept test).

After cleanup (af051e8): 6 requirements honestly carry `enforced_by=()` with STRUCTURAL enforcement (no machine enforcer exists yet; architecture + convention enforce).

## Missing requirements (G1-G10) — proposed but not all added

G1-G6 were named by audit; many landed as DRAFTs in commit 3465f83 (today's atom-recording wave). Mapping:

| audit G# | recorded as | status this session |
|---------|-------------|---------------------|
| G1 atomicity discipline | R-requirement-claim-is-atomic (B1), R-check-method-is-atomic (B2), R-bijection-r-to-enforcer (B3) | DRAFT in 3465f83 |
| G2 operator-from-substrate | R-operator-prompt-from-substrate (A1) | DRAFT in 3465f83 |
| G3 domain-vs-task agent | R-domain-agent-vs-task-agent (covered by D1-D3 atoms in 3465f83) | DRAFT in 3465f83 |
| G4 cognitive-load trigger | R-tree-of-crystals-cognitive-trigger (G1 in 3465f83 numbering — note collision with audit G1; different scope) | DRAFT in 3465f83 |
| G5 constituting consistency | R-constituting-requirements-converge (A2) | DRAFT in 3465f83 |
| G6 director self-state digest | R-live-state-constitutional-digest (covered by I1 R-docs-generated-from-requirements + future expansion) | I1 SETTLED/ENFORCED; full digest as future R |
| G7 conflict structurally visible | R-conflict-structurally-visible | SETTLED/ENFORCED in af051e8 |
| G8 conflict min two members | R-conflict-min-two-members | SETTLED/ENFORCED in af051e8 |
| G9 decided justifies itself | R-decided-conflict-justifies-itself | SETTLED/ENFORCED in af051e8 |
| G10 m_tag discipline | R-m-tag-format-valid | SETTLED/ENFORCED in af051e8 |

## 10 director-irreducible decisions (audit highlight)

Summarized; the 3 atomization-specific ones are in `audit-backlog-residue.md`:

1. R-content-free-framework: ATOMIC vs split (D1 in backlog).
2. R-two-altitude-ontology: ATOMIC vs compound + enforcement-honesty (D3).
3. R-empty-content-is-legitimate: 1 atom vs 3 (D2).
4. REJECTED→REPLACES vs in-place edit for compound R's — chose REJECTED→REPLACES this session (preserves lineage, honors R-rejected-preserved-not-deleted).
5. R-enforcement-gradient + R-requirement-enforced share an enforcer — after splitting the check, each sub-check_* maps to one or both R's?
6. R-atomicity-discipline's future check_*: P1 STRUCTURE (blocking) vs P0 REFLECTION (advisory)?
7. LIVE-STATE block growth: inline vs split into multiple sentinel blocks?
8. Delegation node type: core vs aspect vs stay-PROSE until 2nd delegation?
9. check_m_tag_format orphan resolution: granularity of M-tag discipline R?
10. STRUCTURAL with non-check enforced_by: valid (= "architecture enforces it") or always require empty enforced_by for STRUCTURAL?

## Recommended ordering (from audit)

1. ATOMIZE SETTLED (17 compound → atoms) — partial: 4 done af051e8; 13 remaining.
2. ATOMIZE check_* (10 compound) — pending.
3. RESTORE R↔check bijection — partial: orphans anchored, fake-enforcers cleaned; bijection 1:1 still needs check_* atomization first.
4. ADD MISSING R's (G1-G6) — done as DRAFT in 3465f83.
5. EXPAND LIVE-STATE RENDERING (R-live-state-constitutional-digest) — gated by 1-4.

## Build trigger for `tools/audit_atomicity.py`

This whole audit was a HAND (sh-subagent) — violates R-prefer-tool-over-hand
recorded the same session. The right artifact is `spec/tools/audit_atomicity.py`:
deterministic, parameterized, re-runnable, stamp-able into the LIVE-STATE
block.

Build trigger: **next session, before any further atomization wave**. The
audit tool produces this verdict table as a generated doc
(`docs/gen/AUDIT.md` perhaps), making this checkpoint obsolete (or the
checkpoint becomes a snapshot of the tool's first run).

Until the tool exists, this file is the substrate's memory of the audit.

## See also

- `docs/checkpoints/phase11-atomization-wave.md` (session state).
- `docs/checkpoints/atomicity-as-convergence.md` (discipline + summary findings).
- `docs/checkpoints/audit-backlog-residue.md` (concrete worklist with priorities).
- `docs/checkpoints/agent-vs-hand.md` (why the audit-tool DRAFT matters).
- `docs/checkpoints/sensor-substrate-inversion.md` (architectural ground).
