# Checkpoint — 2026-06-30 [phased-plan-p0-p9]

## Session summary

We crystallized the entire forward plan for Hotam-Spec into a phased, executable
development schedule (P0–P9 + three parallel ‖ mechanical items A‖/B‖/U‖). The
plan merges three previously-decided threads — the "becoming-the-operator"
organ-path, the ratified Batch B framework increments, and the remaining audit
ANCHOR items — into one sequence governed by a single binding rule: every phase
must NET DRAFT down, and each promotion must NAME the invariant/test that flips
DRAFT→ENFORCED. The starting ratio is 13 SETTLED : 25 DRAFT with only 1
lifetime promotion (`R-smoke-test` via Batch A); the projected trajectory ends
at ~38 SETTLED : 0 DRAFT, with the SETTLED-overtakes-DRAFT crossover at P2.

The critical reconciliation: **P0 is the M26 enforcement meter, NOT the
Lifecycle keystone.** Reasoning — until `Requirement.enforcement` +
`UNENFORCED.md` exist, "promoted DRAFT→ENFORCED" is an unverifiable claim
(there is no field to set, no report to diff). The meter must precede every
phase that claims to promote. Lifecycle (the type-dependency root) follows at
P1. The chain then runs P0(meter) → P1(lifecycle) → P2(Self+M36) → P3(action) →
P4(verify) → P5(drive tick) → P6(conscience) → P7(constitution) → P8(reflection
band) → P9(process aspect). Three independent mechanical items run in parallel:
A‖ glossary-sync, B‖ history-from-markers, U‖ generated DECISIONS.md (replaces
the U5 cross-anchor interim with a bijection).

Earlier this session: I formally became "operator #1" of the system at user
direction (around 34% context); applied the methodology to myself — crystallized
three design dossiers (lifecycle/process/entity/task; full-system + context-
bounded delegation; crystallization+anchoring super-rules) and the
dependency-graph + director-crystal trio into the meta-domain via delegated
sub-agents. Executed Batch A (drift fixes U1–U4 + U5 interim cross-anchor +
A1–A4 anchor tests + check_typed_anchors invariant), reaching 92 passed. Two
commits landed: `efac8fa` (crystallization+Batch A) and `5603635` (prior
checkpoint). Then ran a director's audit and resolved the open M-decisions via
dev-coin precedent (the `Param.status`+`HOLES.md` pattern decides M26 as
field-AND-report). Finally produced this consolidated plan.

The deepest gap identified by the operator self-examination: **the system
DESCRIBES the operator (25 DRAFT requirements + Director's map) but does not
yet CONSTITUTE one.** The loop in CLAUDE.md is a circle with one dotted arc:
`State→Diagnosis→Next-action` is code (`what_now`); `→Action→` is my hand.
Verified absent in repo (Glob/Grep): types `Operator`/`ContextBudget`/`Lifecycle`,
`tools/apply_proposal.py`, any Reflection band, generated DECISIONS/UNENFORCED/
GLOSSARY/HISTORY docs. That absence IS the gap; the plan closes it.

The greenlight ledger is pre-computed: 4 decisions resolved-on-paper need
one-line confirmation (M26, M20, M17, M19); 5 are new and need a steward
decision (M36 operator-not-self-approve, M32 tick autonomy, M33 constitution
location, M34 reflection band, M35 burn-down metric); 2 live OPEN need scope
calls (M7 critical-core for conscience, M18 partition vs border). A single
ratification of this ledger pre-clears the full P0–P9 chain.

Working tree: clean. 4 commits in history. Awaiting user ratification of the
ledger before executing P0 (which is pure mechanical: requirement.py field +
one invariant + one generator section + one test + status flips for
R-enforcement-gradient and R-requirement-enforced, netting DRAFT 25→23).

Files inspected this session (via the delegated max-effort audits and plan):
`spec/content/graph.py` (the 54-req substrate), `CLAUDE.md` (M1–M31 table +
Director's map), `spec/src/hotam_spec/{requirement,invariants,graph}.py`,
`spec/tools/{what_now,gen_spec}.py`, `spec/tests/test_*.py`,
`docs/development/ROADMAP.md`, `docs/methodology/README.md`, prior checkpoint.
No /loop or /babysit timers active.

## Active goal

None — no `/goal` Stop hook in force.

## TaskList

### in_progress
- (none — all in-flight tasks completed during this turn)

### pending
- (none)

### recently completed
- #31 Consolidate the phased executable development plan (oxx)
- #30 Synthesize the becoming-path + decision for user
- #29 Operator self-examination: organs of "becoming the system" (oxx)
- #28 Director verification of Batch A
- #27 Execute Batch A: mechanical drift fixes + anchor tests (sh)
- #26 Resolve the blocking open decisions via the methodology's own machinery (oxx)
- #25 Present anchored backlog + steward decision to user
- #24 Audit substrate → anchored UPDATE/ANCHOR/DEFER backlog (oxx)
- #23 Crystallize dependency-graph + director-crystal trio; build Director's Map (delegated)
- #22 Director verification post-crystallization

Older tasks (#4–#21) covered the framework build, content-free refactor, and
prior crystallization waves — all completed and committed in `6465c93`,
`3b11447`, and `efac8fa`.

## Decisions

- **P0 = M26 enforcement meter, NOT Lifecycle.** Rationale: the burn-down
  governor demands verifiable promotion; without `Requirement.enforcement` +
  `UNENFORCED.md`, "DRAFT→ENFORCED" is unmeasurable. Lifecycle follows at P1.
- **M26 representation = field AND report**, per dev-coin's
  `Param.status`+`HOLES.md` precedent. Not either/or. Pre-decided in the
  ratification sheet, ready for one-line steward confirm.
- **A‖/B‖/U‖ run as parallel sub-operators** (independent sub-graph per
  `R-dependency-graph-parallelism`); only the P-chain is strictly sequential.
  P6 (conscience) also parallelizes with P3-P5 once P2 lands.
- **M36 (operator-not-self-approve) is mandatory at P2**, not deferred.
  Without it, a Self-node could self-approve its own status/budget changes,
  re-importing the invisibility the hard boundary forbids. The reflexive twin
  of `check_steward_not_a_member_owner`.
- **Each phase promotes ≥2 DRAFTs to ENFORCED** and never spawns more DRAFT
  than it retires. Only P9 (Process aspect) may spawn one new DRAFT; offset
  in-phase. This is the honesty governor that keeps crystallization from
  degenerating into hoarding (`C-06e2d84e` revisit_marker mechanized).

## Open questions

- **Ratify the greenlight ledger?** One pass pre-clears the entire P0–P9
  chain. The five NEW M-decisions (M32–M36) are the only substantive items;
  the four resolved-on-paper (M17/M19/M20/M26) need one-line confirm.
- **Launch P0 immediately?** Pure mechanical, ~5 file touches, drops DRAFT
  25→23, makes burn-down CI-measurable from this commit forward.
- **Run A‖/B‖/U‖ in parallel** with the P-chain? They are independent and
  delegate cleanly; would accelerate burn-down by 3 DRAFTs without serializing.

## Repo state

```

```

```
5603635 docs: session checkpoint 2026-06-30 (tensio-substrate-built)
efac8fa feat(content): crystallize operator/super-rule designs + Batch A anchor
3b11447 feat(content): meta-domain — Hotam-Spec modeling its own design
6465c93 feat: Hotam-Spec — content-free requirements-as-tension-graph methodology framework
```

## The plan — phase spine

Reference table for resume (full per-phase detail in conversation; key dependencies preserved):

| # | Goal | Promotes (DRAFT→ENFORCED) | Settles M# | Gate | ‖/→ |
|---|---|---|---|---|---|
| P0 | Burn-down meter | enforcement-gradient, requirement-enforced | M26 | MECH | → root |
| P1 | Lifecycle keystone | lifecycle-abstraction, statemachine-wellformedness | M11 | MECH | → |
| P2 | Self node + steward-safety | operator-acting-facet, context-budget-rule | M20, M17, **M36** | GREEN | → |
| P3 | Action half | active-loop-playbooks, decided-needs-human-signoff | — | MECH | → |
| P4 | Per-action verify | smoke-test (flip), crystallize-knowledge-to-code | — | MECH | → |
| P5 | Drive/tick | anchor-everything (broaden), speak-by-reference | **M32** | GREEN | → |
| P6 | Conscience (formal) | uncrystallizable-is-missing-type, stale-substrate | M7 | GREEN | → (‖ P3-5) |
| P7 | Constitution doc | operator-crystal-is-claude-md, crystallize-before-split | **M33** | GREEN | → |
| P8 | Reflection band | working-vs-substrate-budget, delegation-conclusions-only, context-bounded-delegation, dependency-graph-parallelism | **M34, M35**, M18 | GREEN | → |
| P9 | Process aspect (last) | process-aspect-first, goal-as-target-state, task-vs-action-distinct-altitudes (+1 new DRAFT, offset) | M12, M19, M23 | GREEN | → |
| A‖ | Glossary sync | glossary-sync-test | — | MECH | ‖ |
| B‖ | History from markers | history-from-rejected-markers | — | MECH | ‖ |
| U‖ | DECISIONS.md (gen, bijection) | (closes U5 anti-drift; adds m_tag field) | — | MECH | ‖ |

Burn-down: **13:25 → ~38:0**, crossover at P2.
