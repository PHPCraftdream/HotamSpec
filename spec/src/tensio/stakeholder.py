"""Canon: §Stakeholder — who owns requirements and stewards conflicts.

A Stakeholder is the human (or role) accountable for a piece of the requirement
graph. Two distinct accountabilities exist and MUST stay separate:
  - OWNER of a Requirement — defends that requirement's claim.
  - STEWARD of a Conflict — holds the TENSION between requirements; by
    construction the steward is NOT the owner of any member requirement, because
    a conflict lives BETWEEN stakeholders, not inside one (see §Conflict and
    invariants.check_steward_not_a_member_owner).

WHY a first-class node (not a free-text "owner: Finance"): accountability is the
external anchor of the whole internal loop. Every OPEN question, every undecided
conflict, every dead-assumption fallout resolves to a named stakeholder the
harness can point at. Without a stable id there is nobody to escalate to and the
loop floats free of reality (see ROADMAP trust-anchoring ritual).
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Stakeholder:
    """Canon: §Stakeholder — a named, accountable party in the tension graph.

    RULE: every Requirement.owner and every Conflict.steward MUST be the id of a
    Stakeholder present in the graph (invariants.check_no_dangling_ids). The
    `id` is stable identity; renaming `name` never changes edges that point here.

    Fields:
      id     — stable slug (e.g. "finance", "platform"); the value edges carry.
      name   — human-readable display name.
      domain — the area of accountability (e.g. "money", "latency/SLA").

    WHY domain matters: the trust-anchoring ritual (ROADMAP) signs the tension
    map PER DOMAIN; the steward of a conflict and the domain it touches are how
    the internal loop is bound to a living human signature.
    """

    id: str
    name: str
    domain: str
