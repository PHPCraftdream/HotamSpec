"""Canon: §Conflict — the first-class connector NODE (the centerpiece).

A Conflict is NOT an edge between requirements. It is a mediator node through
which two otherwise-unconnectable requirements first come to lie in one
structure: R-87 -> C-12 <- R-203. The node carries knowledge belonging to
NEITHER member:
  - axis    — the tension dimension they diverge along (controlled vocabulary,
              §Axis). Born only from their meeting.
  - context — the scenario in which they actually collide (outside it the
              members may coexist peacefully).
  - shared_assumption — the assumption they interpret differently; often the
              real root, and the seam through which the conflict INHERITS drift.

WHY a node, not an edge: an edge `conflicts_with` holds nothing — remove it and
the requirements fall back into isolation. The node holds the axis, context and
shared assumption, which exist nowhere else. Three consequences the ontology must
preserve:
  - conflicts CLUSTER by axis — many C-nodes on one axis = one unresolved
    ARCHITECTURAL choice (graph clusters, an edge-list cannot). See TENSIONS.md.
  - conflicts SPAWN requirements — resolving C-12 is often the birth of R-300
    that dissolves the tension, so the Conflict is the PARENT of a new
    requirement, with recorded lineage (`derived`).
  - conflicts INHERIT drift — if shared_assumption dies, the whole cluster under
    it revives at once (one trigger re-opens a semantic cluster).

DETECTION REDEFINED: surfacing a contradiction = MATERIALIZING this missing node.
The detector hunts requirement pairs that SHOULD have a C-node but don't (latent
connectors), which is stronger than checking violated invariants — it catches the
invisible. The heuristic stub lives in graph.latent_connector_suspects; the real
detector (Hypothesis layer 4 / Z3 layer 5) is DEFERRED.

THE HARD BOUNDARY: a Conflict is NEVER closed silently. It transitions through a
lifecycle under a human STEWARD who is, by construction, not the owner of any
member (a conflict lives between stakeholders). The AI presents, justifies, asks;
the decision and its recording stay human.

THE SIGNOFF LOCK: when a Conflict transitions to DECIDED, it MUST carry a
`decided_by` field naming the human Stakeholder who approved the resolution.
This is the structural twin of the steward-distinct boundary applied at the
moment of decision: `decided_by` MUST be non-empty AND must NOT be the owner
of any member (invariants.check_decided_has_decided_by). This prevents an AI
from silently writing DECIDED without a named human decider, making the hard
boundary enforceable at commit time (R-decided-needs-human-signoff,
§Proposal — the closed loop's ACT half).

Lifecycle (source of truth is the `lifecycle` field, params.py-style):
  DETECTED            — surfaced; node materialized; no steward action yet.
  ACKNOWLEDGED        — steward accepts it is real and owns it.
  DECIDED(rationale)  — resolved WITH recorded rationale and/or a derived
                        requirement (invariants.check_decided_has_rationale_or_derived).
                        MUST carry a non-empty decided_by (the human who approved).
  REVISIT_WHEN(cond)  — parked with an explicit revisit condition (anti-relitigation:
                        the historian re-opens it when the condition triggers).
"""

from __future__ import annotations

import hashlib
import re
from dataclasses import dataclass, field

DETECTED = "DETECTED"
ACKNOWLEDGED = "ACKNOWLEDGED"
DECIDED_PREFIX = "DECIDED"  # "DECIDED(<rationale>)"
REVISIT_PREFIX = "REVISIT_WHEN"  # "REVISIT_WHEN(<condition>)"

#: Lifecycle states in which a conflict is still OPEN (no steward resolution yet).
UNRESOLVED_LIFECYCLE: frozenset[str] = frozenset({DETECTED, ACKNOWLEDGED})


def conflict_identity(axis: str, context: str) -> str:
    """Canon: §Conflict — stable identity slug from (axis, context).

    RULE: a Conflict's identity is hash(axis, normalized(context)), NOT its
    member ids. The node therefore SURVIVES renaming/splitting/refinement of its
    member requirements — only its `members` edges update.

    Identity slug = "C-" + first 8 hex of sha256("<axis>\\x00<normctx>"), where
    normctx lowercases and collapses whitespace so cosmetic edits to the context
    prose do not fork the node.

    WHY (axis, context) and not members: the same two requirements can collide on
    different axes/contexts (distinct conflicts); and one conflict can outlive any
    particular pair of member requirements as they are reorganized. Identity must
    track the TENSION, which is (axis, context), not the parties.
    """
    normctx = re.sub(r"\s+", " ", context.strip().lower())
    digest = hashlib.sha256(f"{axis}\x00{normctx}".encode()).hexdigest()
    return "C-" + digest[:8]


@dataclass(frozen=True)
class Conflict:
    """Canon: §Conflict — a materialized connector node between >=2 requirements.

    RULE (enforced by invariants):
      - axis MUST be non-empty and in the §Axis REGISTRY (check_axis_in_registry,
        check_conflict_has_axis_context_steward);
      - context MUST be non-empty (check_conflict_has_axis_context_steward);
      - steward MUST be a Stakeholder id (check_no_dangling_ids) and MUST NOT be
        the owner of any member (check_steward_not_a_member_owner);
      - members MUST contain >= 2 Requirement ids, all resolving in the graph
        (check_conflict_min_two_members, check_no_dangling_ids);
      - if lifecycle is DECIDED(...), it MUST carry a non-empty rationale OR a
        non-empty `derived` (check_decided_has_rationale_or_derived);
      - if lifecycle starts with DECIDED, `decided_by` MUST be non-empty AND
        resolve to a known Stakeholder AND NOT be the owner of any member
        (check_decided_has_decided_by — the signoff lock, R-decided-needs-human-signoff).

    Fields:
      id                — pass conflict_identity(axis, context); validated by
                          check_conflict_id_matches_identity (the node's identity
                          is its tension, not its members).
      axis              — §Axis slug (the tension dimension).
      context           — the colliding scenario (where they actually clash).
      members           — tuple of >= 2 Requirement ids.
      steward           — Stakeholder id holding the tension (not a member owner).
      lifecycle         — DETECTED | ACKNOWLEDGED | DECIDED(r) | REVISIT_WHEN(c).
      shared_assumption — optional Assumption id (the root / drift seam).
      derived           — tuple of Requirement ids this conflict SPAWNED (lineage).
      revisit_marker    — anti-relitigation note (the historian's trigger / the
                          "RESOLVED — REPLACES ..." / "REJECTED ..." record).
      decided_by        — Stakeholder.id of the human who approved DECIDED; empty
                          for non-DECIDED conflicts. When lifecycle starts with
                          DECIDED, MUST be non-empty and not a member-owner
                          (the steward-distinct rule applied to the decider).

    WHY identity is validated, not just stored: passing a hand-written id would
    let the node drift from its (axis, context), breaking clustering and survival
    across member churn. The invariant forces id == conflict_identity(axis, ctx).
    """

    id: str
    axis: str
    context: str
    members: tuple[str, ...]
    steward: str
    lifecycle: str
    shared_assumption: str | None = None
    derived: tuple[str, ...] = field(default_factory=tuple)
    revisit_marker: str = ""
    decided_by: str = ""  # Stakeholder.id of the human who approved DECIDED; required when lifecycle starts with DECIDED

    def is_unresolved(self) -> bool:
        """Canon: §Conflict — True iff no steward resolution has landed yet.

        RULE: unresolved iff lifecycle is DETECTED or ACKNOWLEDGED. The harness
        prioritizes these — a conflict stuck unresolved with no steward movement
        is a primary "next action".

        WHY: DECIDED and REVISIT_WHEN are stewarded outcomes (visible, owned);
        DETECTED/ACKNOWLEDGED are the dangerous middle where a contradiction is
        seen but not yet held — exactly what must never silently fade.
        """
        return self.lifecycle in UNRESOLVED_LIFECYCLE

    def is_decided(self) -> bool:
        """Canon: §Conflict — True iff lifecycle records a steward DECISION.

        RULE: decided iff lifecycle starts with "DECIDED". A decided conflict
        MUST justify itself (rationale in the marker or a derived requirement).

        WHY a prefix test: the rationale travels inline as "DECIDED(<why>)", the
        anti-relitigation record of the resolution; membership is by prefix so
        invariant, harness and generator agree.
        """
        return self.lifecycle.startswith(DECIDED_PREFIX)
