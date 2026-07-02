"""Canon: §Operator — the acting facet of a Stakeholder (M20: NEW TYPE).

RULE: an Operator is the acting facet of a Stakeholder (§Stakeholder). Where a
Stakeholder answers 'who is accountable', an Operator answers 'who can act,
within what context, over which slice of the graph'. The two facets MUST stay
separate — single-altitude-vs-multi-altitude.

Canon: §Reflection — the P0 operator self-diagnosis band observes Operator nodes
directly: it checks context_budget.limit vs graph size (over-budget), and flags
DRAFT-overhang, UNENFORCED-SETTLED debt, DEAD-assumption-on-ENFORCER, and
derived-but-unbuilt debt. An operator that cannot see its own state is worse than
a malformed graph (ranked P0, above §Invariants P1 STRUCTURE).

Canon: §Context — the working-context fullness is MEASURED, not guessed; the
three-cipher pulse cites it as the first cipher (tools/context.py reader;
R-measure-context-size DRAFT).

Canon: §Operator — every Operator:
  - carries a typed anchor starting with 'OP-' (R-anchor-everything);
  - references a Stakeholder.id (the accountability facet, §Stakeholder);
  - has a lifecycle value matched by OPERATOR_LIFECYCLE (§Lifecycle);
  - carries a ContextBudget (§ContextBudget) bounding its WORKING store;
  - optionally has a parent Operator (the delegation hierarchy).

Canon: §ContextBudget — bounds the working store only; the crystallized
substrate (graph + generated docs) is FREE. See R-working-vs-substrate-budget.

References:
  R-operator-acting-facet — the acting facet is where context and capability live.
  R-context-budget-rule   — size(domain) <= budget.limit is a structural check.
  R-crystallize-before-split — crystallize first, split only if still over budget.
  R-operator-crystal-is-claude-md — each operator's crystal is its CLAUDE.md.

WHY a new type (M20 = new type, not a Stakeholder facet): separating them
(Operator is a NEW TYPE referencing Stakeholder) preserves the steward-distinct
boundary at the methodology altitude. A Stakeholder is an accountability node;
the acting/context/domain facet lives on Operator. Conflating them would merge
two things that the single-altitude-vs-multi-altitude axis explicitly separates.
"""

from __future__ import annotations

from dataclasses import dataclass, field

from hotam_spec.lifecycle import (
    INITIAL,
    NORMAL,
    QUIESCENT,
    Lifecycle,
    State,
    Transition,
)

# ---------------------------------------------------------------------------
# ContextBudget measure constants (M17: NODE_COUNT is the default now)
# ---------------------------------------------------------------------------

NODE_COUNT = "NODE_COUNT"
TOKEN_ESTIMATE = "TOKEN_ESTIMATE"  # deferred seam, not measured yet
COMPLEXITY = "COMPLEXITY"  # deferred seam, not measured yet
CRYSTAL_CHARS = "CRYSTAL_CHARS"
"""Canon: §ContextBudget — measure = len(root CLAUDE.md) in characters.

RULE: size(operator.domain) under CRYSTAL_CHARS is the character-length of
the resident crystal (root CLAUDE.md), NOT the crystallized substrate (the
content graph). The substrate is FREE (R-working-vs-substrate-budget); only
what the operator must hold RESIDENT in its working context — the crystal it
re-reads at boot — competes against the host's real ceiling.

WHY this measure exists (replacing the NODE_COUNT-as-substrate-proxy
mistake): NODE_COUNT counted requirements+conflicts+assumptions — the
crystallized substrate itself — and so punished the very act of
crystallizing and of keeping REJECTED history, exactly what
R-working-vs-substrate-budget declares free. CRYSTAL_CHARS measures the one
thing that actually costs working context: the size of the file the
operator re-loads by reference each boot (R-context-budget-rule).
"""
BUDGET_MEASURES: frozenset[str] = frozenset(
    {NODE_COUNT, TOKEN_ESTIMATE, COMPLEXITY, CRYSTAL_CHARS}
)


# ---------------------------------------------------------------------------
# ContextBudget value-type (§ContextBudget)
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class ContextBudget:
    """Canon: §ContextBudget — the working-store ceiling for an Operator.

    RULE: bounds the WORKING store only; the crystallized substrate is FREE.
    `measure` selects how `size(operator.domain)` is computed.
    NODE_COUNT (M17 default) = |requirements| + |conflicts| + |assumptions|
    that resolve in the operator's DomainScope (a future P-phase will narrow
    by scope; for now, full-graph count).

    limit=0 means unbounded (the aspect is opt-in; 0 = off for this operator).
    A positive limit activates check_operator_within_budget.

    WHY bounding the substrate would be wrong: bounding the substrate would
    punish the very act — crystallizing — that the budget rewards. Only
    un-offloaded working knowledge competes for context, so only it is metered.
    See R-working-vs-substrate-budget.
    """

    limit: int  # 0 = unbounded (aspect off; opt-in)
    measure: str = NODE_COUNT


# ---------------------------------------------------------------------------
# Operator lifecycle (uses the §Lifecycle keystone — NO parallel machinery)
# ---------------------------------------------------------------------------

OPERATOR_LIFECYCLE = Lifecycle(
    slug="operator-lifecycle",
    states=(
        State(
            "ACTIVE",
            kind=INITIAL,
            why=(
                "Operator is acting on its sub-domain within budget. "
                "This is the normal running state."
            ),
        ),
        State(
            "SATURATED",
            kind=NORMAL,
            why=(
                "Working context near or over budget; operator must crystallize-"
                "before-split (R-crystallize-before-split)."
            ),
        ),
        State(
            "DELEGATED",
            kind=NORMAL,
            why=(
                "A sub-operator has been spawned; this operator delegated a "
                "sub-domain to the sub-operator (R-context-bounded-delegation)."
            ),
        ),
        State(
            "RETIRED",
            kind=QUIESCENT,
            why=(
                "Acting facet retired; the underlying Stakeholder (§Stakeholder) "
                "remains. Quiescent: the Stakeholder's accountability persists."
            ),
        ),
    ),
    transitions=(
        Transition(
            "ACTIVE",
            "SATURATED",
            event="approach-limit",
            why="Working context nears the budget ceiling; crystallize before more work.",
        ),
        Transition(
            "SATURATED",
            "ACTIVE",
            event="crystallize",
            why=(
                "Vertical relief — offloaded working knowledge into the substrate; "
                "budget re-measured and now under limit (R-crystallize-before-split)."
            ),
        ),
        Transition(
            "SATURATED",
            "DELEGATED",
            event="spawn-sub-operator",
            why=(
                "Horizontal relief of last resort — still over budget after "
                "crystallizing; sub-operator spawned for a bounded sub-domain "
                "(R-context-bounded-delegation)."
            ),
        ),
        Transition(
            "ACTIVE",
            "RETIRED",
            event="retire",
            why="Acting facet retires; underlying Stakeholder remains in the graph.",
        ),
        Transition(
            "DELEGATED",
            "ACTIVE",
            event="merge-back",
            why=(
                "Sub-operator's work is merged back; parent resumes acting on "
                "the full (now re-budgeted) domain."
            ),
        ),
    ),
    cyclic=True,
)


# ---------------------------------------------------------------------------
# Operator dataclass (§Operator)
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class Operator:
    """Canon: §Operator — the acting facet (M20 = new type referencing Stakeholder).

    RULE (enforced by check_*):
      - `id` MUST start with 'OP-' (typed anchor; R-anchor-everything);
        enforced by check_typed_anchors.
      - `stakeholder` MUST resolve to a Stakeholder.id (§Stakeholder —
        the role/accountability facet stays separate —
        single-altitude-vs-multi-altitude axis); enforced by
        check_no_dangling_ids.
      - `lifecycle` value MUST match OPERATOR_LIFECYCLE (validated by
        check_status_in_lifecycle's operator branch).
      - `context_budget.limit > 0` activates the budget rule; 0 = unbounded
        (the aspect is opt-in per-operator); enforced by
        check_operator_within_budget.
      - M36: an operator cannot steward a conflict in which its underlying
        Stakeholder owns a member — enforced by check_operator_steward_not_self
        (R-operator-not-self-approve).

    WHY: Stakeholder (§Stakeholder) answers 'who is accountable'; Operator
    answers 'who can act, within what context, over which slice'. Separating
    them (a new type, not a Stakeholder facet) preserves the steward-distinct
    boundary at the methodology altitude.

    Fields:
      id              — typed anchor 'OP-…' (R-anchor-everything).
      stakeholder     — Stakeholder.id (the accountability facet, §Stakeholder).
      lifecycle       — operator-lifecycle value (ACTIVE/SATURATED/DELEGATED/RETIRED).
      context_budget  — the working-store ceiling (§ContextBudget).
      parent          — parent Operator id or None (root operator has no parent).
      why             — anti-relitigation prose.

    Domain scope (full DomainScope) is DEFERRED to a later P-phase; for now an
    Operator implicitly owns the entire content graph (its budget is measured
    over the whole graph). That is enough to make the operator appear AS A NODE
    in the graph it operates — the constituting move (P2) is the existence, not
    yet the sub-scope mechanics (those land with R-context-bounded-delegation,
    P5+).
    """

    id: str
    stakeholder: str
    lifecycle: str = "ACTIVE"
    context_budget: ContextBudget = field(
        default_factory=lambda: ContextBudget(limit=0)
    )
    parent: str | None = None
    why: str = ""
