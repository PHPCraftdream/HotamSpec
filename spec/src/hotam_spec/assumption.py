"""Canon: §Assumption — a claim with its OWN lifecycle (the root of context drift).

An Assumption is a statement the requirements rest on but that the world can
falsify independently ("there is a single customer per account"; "api_version
>= 3"). It is the mechanism behind the THIRD invisibility — context drift: a
requirement was meaningful under assumption X, X is long false, nobody revisited
it. That is only catchable because the assumption carries its own status.

WHY assumptions are first-class (not prose inside a requirement): conflicts and
requirements INHERIT drift. When an Assumption flips to DEAD, every Conflict and
Requirement resting on it must light up at once — one trigger re-opens a whole
semantic cluster (see graph.dead_assumptions + graph.requirements_on_assumption
and what_now's dead-assumption fallout). A shared assumption interpreted two different ways is
also frequently the REAL root of a Conflict (Conflict.shared_assumption).

Lifecycle (the source of truth is the `status` field, params.py-style):
  HOLDS     — currently believed true.               [epistemic — a fact-claim]
  UNCERTAIN — under question; not yet falsified, not safely relied on.
                                                      [epistemic — a fact-claim]
  DEAD      — known false; everything resting on it must be revisited.
                                                      [epistemic — a fact-claim]
  IMPLEMENTS — an ASPIRATION: not a claim about what the world IS, but a
               statement of what we STRIVE to make true ('we are trying to do
               this, we want this'). [VOLITIONAL — not epistemic]

RULE (IMPLEMENTS, R-assumption-implements-state): IMPLEMENTS is a fourth,
VOLITIONAL род of status, categorically distinct from the three epistemic
statuses (HOLDS/UNCERTAIN/DEAD, which answer 'is this true?'). IMPLEMENTS
answers 'do we want this to become true, and are we working toward it?'. Three
consequences follow directly from its non-epistemic nature and are enforced by
the filters/predicates that key off exact status equality:
  (a) the UNCERTAIN-aging predicate does NOT touch it — an aspiration is not an
      unresolved doubt, so it raises no P4 review-debt signal (see
      graph.uncertain_assumptions, what_now's UNCERTAIN-aging band);
  (b) it is NEVER counted as DEAD-fallout — nothing rests on a broken premise,
      because an aspiration is not (yet) a premise (see graph.dead_assumptions,
      reflection.reflect_dead_assumption_on_enforcer);
  (c) legitimate transitions and their signoff (the transition table lives in
      §Proposal — ProposedAssumptionTransition): IMPLEMENTS→HOLDS ('achieved,
      became fact'), IMPLEMENTS→DEAD ('abandoned the striving'),
      UNCERTAIN→IMPLEMENTS ('we understood this is not a fact but a goal') and
      HOLDS→IMPLEMENTS ('we declared it fact too early') — ALL four require a
      human decided_by. Agent entry WITHOUT signoff remains UNCERTAIN-only.

WHY a distinct род rather than reusing HOLDS: re-affirming a doubted premise as
HOLDS is a factual lie when the truth is 'this is not yet true but we are
building toward it'. Collapsing aspiration into HOLDS would silence the
UNCERTAIN-aging review signal under a false FACT-claim; IMPLEMENTS silences the
doubt-signal HONESTLY, by re-typing the claim from fact to goal. The
UNCERTAIN→IMPLEMENTS transition therefore CHANGES the род of the claim and
removes a live P4 doubt signal — by the Wave-12 asymmetry (a transition that
reduces live signal needs a named human) it REQUIRES a decided_by, unlike the
signal-RAISING move to plain UNCERTAIN.
"""

from __future__ import annotations

from dataclasses import dataclass

from hotam_spec.signoff import Signoff

HOLDS = "HOLDS"
UNCERTAIN = "UNCERTAIN"
DEAD = "DEAD"
IMPLEMENTS = "IMPLEMENTS"

#: The admitted assumption lifecycle states (authority for the form invariant).
#: HOLDS/UNCERTAIN/DEAD are epistemic (fact-claims); IMPLEMENTS is volitional
#: (an aspiration) — R-assumption-implements-state.
ASSUMPTION_STATES: frozenset[str] = frozenset({HOLDS, UNCERTAIN, DEAD, IMPLEMENTS})


@dataclass(frozen=True)
class Assumption:
    """Canon: §Assumption — a falsifiable belief underpinning requirements.

    RULE: `status` MUST be one of ASSUMPTION_STATES
    (HOLDS | UNCERTAIN | DEAD | IMPLEMENTS)
    (invariants.check_assumption_status_valid). When status == DEAD, every
    dependent Requirement/Conflict is surfaced for revisit by the harness — it is
    NEVER silently dropped. IMPLEMENTS (the VOLITIONAL род — an aspiration, not a
    fact-claim) is neither surfaced as DEAD-fallout nor aged as an UNCERTAIN
    doubt (R-assumption-implements-state; see the module docstring for the full
    semantics and the transition table).

    Fields:
      id          — stable slug carried by Requirement.assumptions and
                    Conflict.shared_assumption.
      statement   — the claim in plain language.
      status      — HOLDS | UNCERTAIN | DEAD (the source of truth).
      owner       — Stakeholder id accountable for re-checking it.
      machine_check — optional machine-checkable form (e.g. "api_version >= 3");
                    DEFERRED for execution, recorded for the formal layers.

    WHY machine_check is carried but not run: spec-stack layer 5 (Z3 conflict
    detector) and layer 4 (Hypothesis latent-connector hunt) are deferred; the
    field is the seam where they attach without reshaping the ontology.
    """

    id: str
    statement: str
    status: str
    owner: str
    machine_check: str | None = None
    signoff: Signoff | None = None
    # ^ §Signoff — frozen provenance record of the LAST transition that changed
    # this assumption's status. A HOLDS/DEAD/IMPLEMENTS transition requires a
    # human decided_by (R-trust-anchor-mechanism); before this field existed
    # the decided_by lived only in the gitignored proposal JSON and evaporated.
    # None for assumptions never transitioned through the writer, or transitioned
    # before this field existed.
    created_at: str = ""
    # ^ ISO YYYY-MM-DD of node CREATION; "" = unknown (legacy nodes predating
    # the timestamp layer have no honest creation date — do NOT fabricate one).
    # Stamped by apply_proposal.py at first materialization, never at exec-time.
    # Used by reflect_implements_decay to age an IMPLEMENTS aspiration: an
    # assumption with unknown created_at is NOT aged (no false signal).
    decided_at: str = ""
    # ^ ISO YYYY-MM-DD of the LAST transition into HOLDS/DEAD/IMPLEMENTS;
    # "" = unknown. Stamped by apply_proposal.py alongside the Signoff payload
    # when a steward transition lands. Re-typing to IMPLEMENTS re-stamps this,
    # resetting the decay clock.
