# Checkpoint — audit-backlog-residue

**Topic checkpoint, not session state.** Concrete, addressable, actionable
items from the framework-agent atomization audit that did NOT land this
session. Use this as a worklist for the next session.

## Director-irreducible decisions (do these FIRST — they unblock the rest)

### D1. R-content-free-framework: ATOMIC or split?

CURRENT: "ships ZERO business content — no example requirements, no
example axes, no seed graph."

OPTION A (ATOMIC): leave as-is. The list is illustrative, not normative.
OPTION B (SPLIT): R-content-free-no-business-data + R-content-free-no-
examples + R-content-free-no-seed-graph (3 atoms).

DIRECTOR'S RECOMMENDATION: A. The rule is one rule ("zero business
content"); the dash-list is what zero MEANS, not a second constraint.
But the user/resolver decides.

### D2. R-empty-content-is-legitimate: ATOMIC UX claim or 3 atoms?

CURRENT: "A freshly-cloned framework with no spec/content/graph.py shall
be structurally well-formed; what_now renders a calm 'no content yet'
banner and gen_spec emits the same notice."

THREE OBSERVABLE BEHAVIORS:
- Structural well-formedness.
- what_now calm banner.
- gen_spec notice.

DIRECTOR'S RECOMMENDATION: SPLIT into R-empty-content-wellformed +
R-empty-content-calm-banner + R-empty-content-gen-notice. Each
independently testable; the bijection R↔check would be cleaner.

### D3. R-two-altitude-ontology: ATOMIC or compound?

CURRENT (SETTLED/STRUCTURAL): "The methodology shall use ONE ontology
at two altitudes: operator is to the methodology as actor is to the
business..."

AUDIT FLAGGED AMBIGUOUS: WHY admits "currently PROSE-enforced only" —
so the claim asserts something the framework can't structurally enforce.

DIRECTOR'S RECOMMENDATION: keep ATOMIC but DOWNGRADE enforcement from
STRUCTURAL → PROSE. The current STRUCTURAL is a lie; PROSE honestly
says "discipline, not check".

## Remaining compound SETTLED requirements (10 — atomization wave 2)

Listed in order of likely complexity (simplest first):

1. **R-content-free-framework** — gated by D1 above.
2. **R-empty-content-is-legitimate** — gated by D2 above.
3. **R-glossary-sync-test** → 4 atoms (generated + fails-dead + fails-unused + drift-stable).
4. **R-history-from-rejected-markers** → 2 atoms (generated-from-rejected + generated-from-decided).
5. **R-boot-from-substrate** → 2 atoms (reload-three-facts + cite-in-first-sentence).
6. **R-enforcement-gradient** → 2 atoms (levels-declared + enforced-names-enforcer).
7. **R-critical-core-scope** → 2 atoms (methodology-critical + per-domain-critical).
8. **R-lifecycle-abstraction** → 3 atoms (type-exists + validates-requirement + validates-conflict).
9. **R-operator-crystal-is-claude-md** → 3 atoms (crystal-is-claude-md + reload-by-reference + tree-hierarchy).
10. **R-dependency-graph-parallelism** → 3 atoms (tracked + drives-parallel + drives-sequential).
11. **R-process-aspect-first** → 6 atoms (types-exist + opt-in + lifecycle-wellformed + roles-declared + goal-owner + typed-anchors-extended).

Expected SETTLED growth: ~+24, REJECTED growth: ~+10 (REJECTED stubs preserving lineage).

## Remaining compound check_* invariants (10 — wave 2)

In order of payoff (most fan-out first):

1. **check_no_dangling_ids** → 12 atomic sub-checks (biggest payoff):
   - check_no_dangling_assumption_owner
   - check_no_dangling_requirement_owner
   - check_no_dangling_requirement_assumptions
   - check_no_dangling_relation_kind
   - check_no_dangling_relation_target
   - check_no_dangling_conflict_resolver
   - check_no_dangling_conflict_members
   - check_no_dangling_conflict_shared_assumption
   - check_no_dangling_conflict_derived
   - check_no_dangling_conflict_decided_by
   - check_no_dangling_operator_stakeholder
   - check_no_dangling_operator_parent

2. **check_typed_anchors** → 6 atomic sub-checks (one per prefix).
3. **check_status_in_lifecycle** → 4 sub-checks (one per object type).
4. **check_canonical_lifecycles_wellformed** → 5 sub-checks (one per
   canonical lifecycle) — OR keep compound as one-per-list helper.
5. **check_enforced_names_invariant** → 2 sub-checks
   (level-valid + has-enforcer).
6. **check_decided_has_decided_by** → 3 sub-checks (non-empty + known +
   not-member-owner).
7. **check_m_tag_format** → 3 sub-checks (format + unique + OPEN-only).
8. **check_conflict_has_axis_context_resolver** → 3 sub-checks (axis +
   context + resolver).
9. **check_decided_has_rationale_or_derived** → 2 sufficiency conditions
   (rationale OR derived).
10. (others as audit identifies)

## Build-trigger-fired DRAFTs ready to activate

After atomization completes, these DRAFTs become buildable (their
triggers fire):

- **R-requirement-claim-is-atomic** — heuristic check_*.
- **R-check-method-is-atomic** — heuristic check_*.
- **R-bijection-r-to-enforcer** — meta-check that asserts cardinality 1.
- **R-operator-prompt-from-substrate** — gen_spec generates the
  Constitutional digest into CLAUDE.md.
- **R-constituting-requirements-converge** — pair-wise consistency check
  on SETTLED atoms.
- **R-live-state-constitutional-digest** — extends LIVE-STATE block with
  the digest + entity-graph + delegation-graph sections.

## DRAFTs blocked by external triggers (not in atomization chain)

These have BUILD-TRIGGERs that aren't about atomization:

- **R-agent-is-a-directory** etc. (C-group) — triggered by "first real
  second operator instantiated".
- **R-domain-delegation-as-node** — same trigger.
- **R-tools-registry-generated** — triggered by first agent existing.
- **R-claude-md-budget-phi-cap** — triggered by CLAUDE.md crossing
  ~50% of φ-cap (today ~1%).
- **R-claude-md-tree-of-crystals** — triggered by phi-cap firing.
- **R-tree-of-crystals-cognitive-trigger** — same.
- **R-subagent-gets-its-claude-md** — same.
- **R-operator-backend-protocol** + **R-backend-scope** (OPEN, M37) —
  triggered by a 2nd concrete backend becoming real.
- **R-setup-claude-generates-settings** + **R-context-hook-piggybacks-
  cah-stamp** (J1, J2) — recoverable from killed P10b; lower priority
  after sensor-substrate inversion.

## Quick-win mechanical fixes (low cost, immediate value)

- **Strip more fake enforcers**: the wave 1 cleanup did 12; the audit
  may have flagged more. Pass through all `enforced_by` tuples once
  more.
- **Promote PROSE → STRUCTURAL where evidence is structural**: several
  PROSE R's actually have file-layout or type-existence enforcement.
  Honest upgrade.
- **Promote STRUCTURAL → ENFORCED for D1/D2/D3 split products**: after
  the director decisions, the split atoms can claim their individual
  check_*s (those exist as orphans today).

## Estimated effort

- Director decisions D1/D2/D3: 5 minutes of director thinking.
- Atomization wave 2 (10 SETTLEDs): one parallel o46l fan-out with
  apply_proposal queue. ~15 minutes wall.
- check_* splitting wave (10 checks → ~40 atomic): one parallel o46l
  fan-out. ~20 minutes wall. Care: each split must keep tests green
  (the existing tests test the COMPOUND behavior; need to also write
  tests for each new atomic check_*).
- After both waves: build R-operator-prompt-from-substrate. ~15 minutes
  delegated to sh-high (heavier work — gen_spec extension + meta-test).

Total: ~1 hour next session if focused.

## See also

- `docs/checkpoints/phase11-atomization-wave.md` (session state).
- `docs/checkpoints/atomicity-as-convergence.md` (why atomicity matters).
- `docs/checkpoints/agent-vs-hand.md` (the agent pattern that would
  make multi-agent atomization safer via apply_proposal queue).
