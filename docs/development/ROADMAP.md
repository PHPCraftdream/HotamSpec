# ROADMAP — phased rollout

Tensio ships the CORE now and DEFERS the heavy formal layers. The governing rule
(from [`../../CLAUDE.md`](../../CLAUDE.md)) is the calibration principle: **the
weight of the apparatus is proportional to the cost of an unnoticed conflict.**
Heavy formal machinery is justified only for the critical core (money, access,
SLA, workflow); everywhere else the graph + structural invariants + an AI/heuristic
detector is the right weight.

## Phase 0 — CORE (built, green)

The minimal core from which the "agent is never lost" property is demonstrable:

- **Layer 1 — form.** Frozen-dataclass ontology (`requirement`, `conflict`,
  `assumption`, `axis`, `stakeholder`), ruff-clean.
- **Layer 2 — structural graph invariants.** `tensio.invariants.check_*`
  returning `[Violation]`: referential integrity, conflict has axis/context/
  steward, ≥2 members, axis ∈ registry, id == `hash(axis, context)`, steward ≠
  member owner, OPEN states its question, DECIDED justifies itself. Each invariant
  has a deliberately-broken fixture proving it fires (anti-phantom).
- **Layer 3 — visibility of the open.** `OPEN(question)` requirements +
  unresolved conflicts → generated `docs/gen/OPEN.md`.
- **Layer 9 — human layer + anti-drift.** `tools/gen_spec.py` → `REQUIREMENTS.md`,
  `TENSIONS.md` (tension map: nodes, clusters by axis, Mermaid), `OPEN.md`; the
  meta-test makes regeneration == committed, byte-for-byte.
- **The harness.** `tools/what_now.py` — the Diagnosis step of the closed loop,
  emitting a typed, prioritized next-action list from any graph state.
- **The operating contract.** `CLAUDE.md` — the loop, the three-layer rules, the
  AI roles + hard boundary, the OPEN methodology decisions.

## Phase 1 — DEFERRED formal layers (critical core only)

Switched on per critical-core domain, not globally. Dependencies (`z3-solver`,
`cosmic-ray`, and `hypothesis` for stateful use) are already pinned in
`spec/pyproject.toml` so turning a layer on does not move the dependency surface.

- **Layer 4 — property-detector of latent conflicts (Hypothesis).** Generate
  requirement tuples and hunt for *missing connector nodes*: pairs that should be
  mediated by a Conflict but aren't. Promotes the current heuristic stub
  (`graph.latent_connector_suspects`, the shared-assumption signal) into
  generated, adversarial search over the claim predicates. Output remains "for AI
  review" — never an auto-materialized conflict (the hard boundary).
- **Layer 5 — formal conflict detector (Z3).** The inversion of dev-coin's "prove
  the economics hold for all values." Here Z3 is a *conflict detector*: for a pair
  of machine-readable claims (e.g. `latency_ms < 200` vs
  `latency_ms >= aml_check_ms` with `aml_check_ms >= 800`), produce a **model
  where both are jointly violated** — a concrete witness scenario the steward
  cannot dismiss. Applied only to the critical-core claims that carry machine
  predicates (`Requirement.claim` / `Assumption.machine_check`).
- **Layer 6 — behavioral/temporal conflicts (Quint/Apalache).** For *process*
  requirements (workflows, state machines): two requirements that are
  individually satisfiable but jointly produce a deadlock or an unreachable
  required state. Symbolic, no enumeration.
- **Layer 7 — stateful PBT of graph evolution (Hypothesis stateful).** Model the
  graph as a state machine and assert the evolution invariants across histories,
  the key one being: **flipping an assumption to DEAD revives every dependent
  Requirement and Conflict** (one trigger, one cluster). The independent oracle is
  a hand-rolled dependency map vs `graph.dependents`-style traversal.
- **Layer 8 — mutation testing of the detectors (cosmic-ray).** Guards against
  *phantom detectors* — detectors that stay green on broken data. Generic mutation
  operators are not enough; Tensio needs **semantic mutation operators** that
  perturb the meaning of a requirement/assumption and assert the detector
  notices:
  - **weaken assumption** — flip a `machine_check` bound to a strictly weaker one
    (`>= 800` → `>= 0`); a conflict that depended on it should stop being detected.
  - **tighten bound** — strengthen a claim's numeric bound (`< 200` → `< 50`); a
    latent collision should newly appear.
  - **swap actor** — replace a `Requirement.owner` / `Conflict.steward` with
    another stakeholder; `check_steward_not_a_member_owner` should fire or clear.
  - **invert cardinality** — change a "single X" assumption to "many X"
    (`A-single-customer`), which is exactly the drift that should revive a cluster.
  A surviving mutant = a detector that is not actually testing what it claims.

## Phase 2 — Trust-anchoring ritual (described, not built)

The whole `State → Diagnosis → Next-action → Action → State` loop is an
**internal contour**: it is self-consistent but, by itself, floats free of the
living organization. The answer is an **external anchor**:

- **The ritual.** Periodically (per release / per quarter), the human stakeholder
  accountable for a **domain** (`Stakeholder.domain`) reviews the tension map for
  their domain and applies a **cryptographic signature** over the generated
  `TENSIONS.md` slice for that domain (e.g. a detached signature over the
  canonical bytes of the domain's conflict nodes + their lifecycles).
- **What it buys.** A signature is a timestamped, attributable assertion "as of
  this state, I, the accountable human, have seen these tensions and their
  current resolutions." It converts the internal loop's "everything is visible"
  into an external "a named human has actually looked." Drift in *attention*
  (nobody re-signs while the graph changes underneath) becomes detectable: the
  harness can add a band — **stale/unsigned domain** — when a domain's tension map
  has changed since its last signature, or a signature has aged past its cadence.
- **Why it is deferred.** It needs a key-management and verification mechanism and
  a per-domain canonicalization of the tension map; both are out of scope for the
  CORE. The seam is ready: `Conflict.steward` and `Stakeholder.domain` already
  give the binding between a tension and the human who must sign for it.

This ritual is decision **M5** in the CLAUDE.md OPEN list — the signature
mechanism and cadence await user confirmation.

## Open methodology decisions

All seven defaulted framework decisions (package name, conflict identity, axis
vocabulary admission, mandatory-distinct steward, trust anchor, latent-connector
heuristic, critical-core scope) are catalogued with their `OPEN(question)` in the
"OPEN methodology decisions" table of [`../../CLAUDE.md`](../../CLAUDE.md). They
are implemented with sensible defaults and remain open until the user confirms or
overrides — the framework practicing its own discipline of visible openness.
