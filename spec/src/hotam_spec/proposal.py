"""Canon: §Proposal — structured operator-→-steward change proposals.

The closed loop's ACT half: the AI operator emits a structured proposal
(ProposedRequirement / ProposedConflictTransition / ProposedRejection), the
steward approves it (out-of-band: review + greenlight), and tools/apply_proposal.py
mechanically writes the change to spec/content/graph.py + runs the regen+verify
pipeline. No free-text AI editing of source.

This honors R-ai-presents-not-decides (the AI never closes a conflict silently)
AND R-active-loop-playbooks (each what_now band has a playbook + a mechanical
apply path).
"""

from __future__ import annotations

from dataclasses import dataclass, field

from hotam_spec.conflict import conflict_identity


@dataclass(frozen=True)
class ProposedRequirement:
    """Canon: §Proposal — propose a new Requirement (typically to close a P4 OPEN_ITEM
    or to spawn a derived requirement from a DECIDED Conflict).

    The proposal IS NOT the requirement; it is a CONTRACT the apply_proposal tool
    serializes into the right Requirement(...) constructor call when the steward
    approves.
    """

    id: str
    claim: str
    owner: str  # Stakeholder.id
    status: str  # DRAFT | SETTLED | OPEN(question)
    why: str
    assumptions: tuple[str, ...] = field(default_factory=tuple)
    relations: tuple[tuple[str, str], ...] = field(
        default_factory=tuple
    )  # (kind, target)
    enforcement: str = "PROSE"
    enforced_by: tuple[str, ...] = field(default_factory=tuple)
    m_tag: str = ""
    enforceability: str = "ENFORCEABLE"

    def target_anchor(self) -> str:
        """Canon: §Closure — the graph object this proposal is meant to change.

        For a ProposedRequirement, the anchor is the R-… id being created/modified.
        Used by closure.check_closure to verify the triggering action was removed.
        """
        return self.id


@dataclass(frozen=True)
class ProposedConflictTransition:
    """Canon: §Proposal — propose a Conflict lifecycle transition + recording.

    For DETECTED→ACKNOWLEDGED, ACKNOWLEDGED→DECIDED(rationale), ACKNOWLEDGED→
    REVISIT_WHEN(condition), and the cyclic re-detect path.

    A DECIDED transition MUST carry decided_by (the steward who approved); the
    apply_proposal tool refuses to write a DECIDED transition with empty decided_by.
    """

    conflict_id: str  # the C-… anchor being moved
    new_lifecycle: str  # the new value (e.g. "DECIDED(... rationale text ...)")
    decided_by: str = ""  # required when new_lifecycle starts with DECIDED
    revisit_marker: str = ""
    derived: tuple[str, ...] = field(
        default_factory=tuple
    )  # R-ids spawned by this decision

    def target_anchor(self) -> str:
        """Canon: §Closure — the graph object this proposal is meant to change.

        For a ProposedConflictTransition, the anchor is the C-… conflict id being moved.
        Used by closure.check_closure to verify the triggering action was removed.
        """
        return self.conflict_id


@dataclass(frozen=True)
class ProposedRejection:
    """Canon: §Proposal — propose REJECTING a Requirement (status → REJECTED).

    Preserves the anti-relitigation discipline: REJECTED is kept in the graph
    (R-rejected-preserved-not-deleted), never deleted.
    """

    requirement_id: str
    reason: str  # the REJECTED — REPLACES … prose

    def target_anchor(self) -> str:
        """Canon: §Closure — the graph object this proposal is meant to change.

        For a ProposedRejection, the anchor is the R-… id being rejected.
        Used by closure.check_closure to verify the triggering action was removed.
        """
        return self.requirement_id


@dataclass(frozen=True)
class ProposedConflict:
    """Canon: §Proposal — propose MATERIALIZING a new Conflict node (kind="Conflict").

    The creation half of the conflict pipeline (§Conflict): the AI operator
    surfaces a tension as a typed proposal, the steward approves, and
    tools/apply_proposal.py writes a Conflict(...) into the domain graph with
    lifecycle DETECTED. Moving it further is a separate
    ProposedConflictTransition — creation and transition stay distinct acts.

    RULE: the node id is NEVER caller-supplied — the writer emits
    id=conflict_identity(axis, context) (R-stable-conflict-identity). axis MUST
    already be a slug in the graph's axes tuple (R-axis-controlled-vocab;
    admitting a NEW axis is a separate act, out of this kind's scope). members
    MUST name >= 2 distinct existing Requirements
    (R-conflict-min-two-members). steward MUST NOT own any member
    (R-steward-distinct-from-owners; re-checked graph-side by
    check_steward_not_a_member_owner after the write).

    `note` is presentation-only context for the steward's review; it is NOT
    written to the graph — the Conflict node itself carries axis, context and
    shared_assumption, which hold the tension's knowledge.
    """

    axis: str
    context: str
    members: tuple[str, ...]
    steward: str
    shared_assumption: str = ""
    note: str = ""

    def target_anchor(self) -> str:
        """Canon: §Closure — the computed C-… id this proposal will materialize.

        Derived via conflict_identity(axis, context), never caller-supplied
        (R-stable-conflict-identity).
        """
        return conflict_identity(self.axis, self.context)


@dataclass(frozen=True)
class ProposedOperatorBudget:
    """Canon: §Proposal / §ContextBudget — propose a new ContextBudget for an existing Operator.

    RULE: kind="OperatorBudget"; the apply_proposal tool locates the
    Operator(...) call whose id matches operator_id and replaces its
    context_budget= kwarg with ContextBudget(limit=new_limit,
    measure=new_measure). Used to move an operator off a stale/mismeasured
    budget (e.g. NODE_COUNT counting the free substrate) onto a measure that
    actually reflects R-working-vs-substrate-budget (e.g. CRYSTAL_CHARS).
    """

    operator_id: str  # the OP-… anchor being re-budgeted
    new_limit: int
    new_measure: str  # one of hotam_spec.operator.BUDGET_MEASURES
    why: str = ""

    def target_anchor(self) -> str:
        """Canon: §Closure — the Operator id this proposal is meant to change."""
        return self.operator_id


@dataclass(frozen=True)
class ProposedEntityType:
    """Canon: §Proposal — propose a new EntityType to add to the active domain's graph.

    RULE: kind="EntityType"; the apply_proposal tool serializes this into the
    right EntityType(...) constructor call when the steward approves. Lifecycle
    is given by serialized states + transitions tuples (the loader rebuilds
    a Lifecycle object).
    """

    slug: str
    description: str
    why: str
    # Lifecycle in serialized form:
    states: tuple[tuple[str, str, str], ...]
    # ^ each: (name, kind, why) — kind ∈ STATE_KINDS
    transitions: tuple[tuple[str, str, str], ...]
    # ^ each: (src, dst, event) — guard/why optional, default ""
    cyclic: bool = False
    fields: tuple[tuple[str, str, bool, str], ...] = field(default_factory=tuple)
    # ^ each: (name, kind, required, ref_target) — kind ∈ ENTITY_FIELD_KINDS

    def target_anchor(self) -> str:
        """Canon: §Closure — the entity slug is the anchor of this proposal."""
        return f"EntityType:{self.slug}"


# A union for type hints (no runtime enforcement; Python keeps it simple):
Proposal = (
    ProposedRequirement
    | ProposedConflictTransition
    | ProposedConflict
    | ProposedRejection
    | ProposedEntityType
    | ProposedOperatorBudget
)
