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


# A union for type hints (no runtime enforcement; Python keeps it simple):
Proposal = ProposedRequirement | ProposedConflictTransition | ProposedRejection
