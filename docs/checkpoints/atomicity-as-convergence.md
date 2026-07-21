# Checkpoint — atomicity-as-convergence

**Topic checkpoint, not session state.** Why atomicity matters; what was
flagged compound; what to fix.

## The thesis (one sentence)

When the substrate generates the operator-prompt, the operator's internal
consistency (convergence) is **only verifiable** if each requirement
asserts exactly one concern — compoundness hides contradiction inside the
conjunction.

## What "atomic" means here

An atomic requirement has:
- ONE rule in its `claim` — no "X and Y", no "X; Y", no "X — also Z".
- ONE enforcer in `enforced_by` (after the bijection is restored).
- ONE check_* function or test corresponding to it.

A compound requirement may LOOK fine when read sentence-by-sentence, but
when two compound R's are compared on an axis, you can't tell if they
agree because each claim mixes multiple axes. Contradictions become
detectable only after decomposition.

## What "atomic" means for check_* methods

Each `check_*` enforces ONE rule. A function that loops over multiple
distinct edge-classes (e.g. `check_no_dangling_ids` checking 8 different
reference types) is compound — the docstring becomes a summary, the code
becomes a list, and the comment↔code correspondence dies.

Atomic check_* is verifiable BY MEANING — read the docstring, read the
function body, see the same rule in both.

## Audit findings (from framework-agent, this session)

### SETTLED requirements: 17 of 42 are compound

Already atomized (commit af051e8):
- ✅ R-active-loop-playbooks → 3 atoms
- ✅ R-statemachine-wellformedness → 4 atoms
- ✅ R-operator-acting-facet → 4 atoms
- ✅ R-goal-as-target-state → 3 atoms

Still compound, waiting (13 more):
- R-content-free-framework (illustrative list — DIRECTOR DECISION)
- R-empty-content-is-legitimate (3 observable behaviors)
- R-boot-from-substrate (2 concerns: WHAT to load + WHEN to cite)
- R-glossary-sync-test (4 conditions)
- R-history-from-rejected-markers (2 source streams)
- R-lifecycle-abstraction (3 distinct validations)
- R-process-aspect-first (6 distinct invariants)
- R-dependency-graph-parallelism (3 claims)
- R-operator-crystal-is-claude-md (3 claims)
- R-enforcement-gradient (2 sub-rules)
- R-critical-core-scope (2 scope claims)
- R-content-free-framework — duplicate, see above
- R-two-altitude-ontology (AMBIGUOUS — DIRECTOR DECISION)

### check_* invariants: 10 of 21 are compound

Most compound:
- `check_no_dangling_ids` — 12 distinct reference-type checks in one body.
  Should split into `check_no_dangling_<kind>` × 12.
- `check_typed_anchors` — 6 distinct prefix checks (R-/A-/C-/OP-/PR-/GOAL-).
  Should split into `check_typed_anchors_<kind>` × 6.
- `check_status_in_lifecycle` — 4 object types validated against
  4 lifecycles. Should split into `check_<type>_status_in_lifecycle` × 4.
- `check_canonical_lifecycles_wellformed` — 5 canonical lifecycles. Could
  stay compound (one helper applied to a list) OR split.
- `check_enforced_names_invariant` — 2 rules (level-valid + has-enforcer).
- `check_decided_has_decided_by` — 3 sub-rules (non-empty + known +
  not-member-owner).
- `check_m_tag_format` — 3 sub-rules (format + unique + OPEN-only).
- `check_conflict_has_axis_context_resolver` — 3 fields checked.
- `check_decided_has_rationale_or_derived` — 2 sufficiency conditions
  (rationale OR derived).

## The plan (post this checkpoint)

### Wave 1 — already done (commit af051e8)
- 4 most compound SETTLED → atomized to 14 atoms (R-active-loop-*, R-statemachine-*, R-operator-* facet split, R-goal-* split).
- 4 orphan check_*s anchored to new SETTLED R's.
- 12 fake-enforcer entries stripped from enforced_by.

### Wave 2 — next session
- Decide 3 ambiguous cases (R-content-free-framework, R-empty-content-is-legitimate, R-two-altitude-ontology).
- Atomize remaining 10 compound SETTLEDs via parallel o46l agents (with
  apply_proposal queue this time to avoid the concurrent-edit hazard).
- Split the 10 compound check_*s — start with check_no_dangling_ids (biggest payoff).

### Wave 3 — after atomization
- Build `check_requirement_is_atomic` (R-requirement-claim-is-atomic) —
  heuristic that flags compound claims (looks for AND/OR/; connectors).
- Build `check_method_is_atomic` (R-check-method-is-atomic) — heuristic
  on `check_*` function bodies (counts distinct rule-clauses).
- Wire as P0 REFLECTION (advisory) initially; promote to P1 STRUCTURE
  later if the false-positive rate stays low.

### Wave 4 — the payoff
- R-operator-prompt-from-substrate built: gen_spec.py emits the
  "Constitutional digest" of atomized SETTLEDs into CLAUDE.md.
- R-constituting-requirements-converge built: invariant that pairs of
  SETTLED atoms on the same axis are detected and either resolved or
  surfaced as a Conflict.

## Three director-irreducible ambiguities

These are RESOLVER decisions, not operator decisions (hard-boundary):

1. **R-content-free-framework**: "ships ZERO business content — no example
   requirements, no example axes, no seed graph". Atomic claim (rule +
   illustrative list) OR compound (rule + 3 enumerated absences)?
   Recommendation: KEEP ATOMIC. The list is illustrative, not normative —
   the rule is "zero business content", the list is "what zero means".
2. **R-empty-content-is-legitimate**: 3 observable behaviors (structurally
   well-formed + calm banner + gen notice). Atomic UX claim OR 3 atoms?
   Recommendation: SPLIT into R-empty-content-wellformed + R-empty-content-
   calm-banner + R-empty-content-gen-notice. They're independently testable.
3. **R-two-altitude-ontology**: ATOMIC philosophical claim or compound of
   "reflexive isomorphism + enforcement promise"? The WHY admits the claim
   is unguarded. Recommendation: keep ATOMIC but DOWNGRADE enforcement to
   PROSE (currently STRUCTURAL — there's no structural enforcer).

## See also

- `docs/checkpoints/phase11-atomization-wave.md` (session state).
- `docs/checkpoints/sensor-substrate-inversion.md` (why atomicity matters
  for the substrate-generates-operator direction).
- `docs/checkpoints/audit-backlog-residue.md` (concrete remaining work).
