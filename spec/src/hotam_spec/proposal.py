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

from hotam_spec.conflict import Variant, conflict_identity
from hotam_spec.signoff import Signoff


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
    summary: str = ""

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
    decided_by: str = ""  # required when new_lifecycle starts with DECIDED or HELD
    revisit_marker: str = ""
    shared_assumption: str = ""
    # ^ optional re-point of the Conflict's shared_assumption edge. When a
    # premise dies and is REPLACED by a narrower one, a DECIDED conflict that
    # rested on the dead premise would otherwise raise perpetual P2 fallout
    # (conflicts_on_assumption fires for any conflict whose shared_assumption is
    # DEAD). Empty string = leave the existing edge untouched (the common case);
    # a non-empty A-… id re-points the edge so the fallout tracks the LIVE
    # premise the conflict actually rests on (R-no-hand-edit-graph — the only
    # mechanical path to move this edge).
    derived: tuple[str, ...] = field(
        default_factory=tuple
    )  # R-ids spawned by this decision
    variants: tuple[Variant, ...] = field(default_factory=tuple)
    # ^ Variant payloads; required (>=2) when new_lifecycle starts with HELD
    # (§Conflict — Variant, check_held_has_min_two_variants). When transitioning
    # HELD -> DECIDED, supply the SAME variants so the writer preserves them
    # (anti-relitigation: the non-chosen variants' implies/costs must survive).
    # §Signoff fields (optional; the writer builds a Signoff payload from these
    # when new_lifecycle starts with DECIDED or HELD, attached as Conflict.signoff):
    date: str = ""
    # ^ ISO YYYY-MM-DD; the date the steward signed. Human-readable, deterministic
    # as a written string. Coarse by design (a full timestamp layer is a later
    # wave). The writer fills today's date as a default when this is empty.
    verbatim: str = ""
    # ^ the steward's own words carried verbatim (optional).
    instrument: str = "personal"
    # ^ HOW the signoff was captured (the verifiable-signature seam). `personal`
    # (default) | `DEL-<n>`. `git`/`crypto` reserved for the future
    # R-decided-by-verifiable-signature wave.
    chosen_variant: str = ""
    # ^ V-id of the variant the steward picked when resolving HELD -> DECIDED.
    # Written into signoff.chosen_variant; check_signoff_chosen_variant_resolves
    # enforces it is the id of one of the conflict's variants when non-empty.

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
    initial_lifecycle: str = "DETECTED"
    # ^ normally DETECTED (creation is presentation, not decision —
    # R-ai-presents-not-decides). A conflict between two SETTLED *constituting*
    # atoms of the self-host graph, however, CANNOT rest at DETECTED: it would
    # trip check_constituting_not_in_unresolved_conflict (the CONSTITUTION
    # presents both as settled truth while the graph records them as an open,
    # unstewarded contradiction). For that case ONLY, the steward may materialize
    # the node already DECIDED(...) in one act, supplying decided_by. A DETECTED
    # step between constituting atoms has no valid resting state, so splitting
    # creation from decision is impossible here (R-constituting-requirements-
    # converge). Any value starting with DECIDED REQUIRES decided_by.
    decided_by: str = ""  # required when initial_lifecycle starts with DECIDED

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
class ProposedAxis:
    """Canon: §Proposal — propose a new Axis (controlled-vocabulary tension dimension)
    to add to the active domain's graph.

    RULE: kind='Axis'; the apply_proposal tool serializes this into a new
    Axis(...) entry appended to the active domain's `axes` tuple. slug MUST be
    kebab-case and MUST NOT already exist in the graph's axes (a duplicate
    slug is a re-declaration, not a new axis — R-axis-controlled-vocab).
    description MUST be non-empty (an axis with no description names nothing
    to cluster around).

    WHY a gatekeeper precedes this proposal, never a bare CLI shortcut: an
    axis is the ONE structural place two future Conflicts cluster into one
    architectural tension (R-axis-gatekeeper-policy — 'a privatnik is born
    with a door'). Admitting a near-duplicate axis silently FORKS a cluster
    into two, which is exactly the invisibility R-anchor-everything forbids.
    tools/create_axis.py is the confront-gated CLI that constructs this
    proposal; hand-writing the JSON bypasses the similarity check and is
    discouraged (R-prefer-tool-over-hand).
    """

    slug: str
    description: str
    why: str = ""

    def target_anchor(self) -> str:
        """Canon: §Closure — the axis slug is the anchor of this proposal."""
        return f"Axis:{self.slug}"


@dataclass(frozen=True)
class ProposedStakeholder:
    """Canon: §Proposal / §Stakeholder — propose a new Stakeholder (accountable party)
    to add to the active domain's graph.

    RULE: kind='Stakeholder'; the apply_proposal tool serializes this into a new
    Stakeholder(...) entry appended to the active domain's `stakeholders` tuple.
    id MUST be unique (not already present in the graph's stakeholders) — a
    duplicate id is a re-declaration, not a new party. id, name and domain MUST
    all be non-empty.

    WHY this kind exists (the stranger's first door): the very first Conflict a
    newcomer models requires a steward who is NOT the owner of any member
    Requirement (check_steward_not_a_member_owner) — i.e. at least two distinct
    Stakeholders must exist before any tension can be held. Yet every Requirement
    and every Axis already had a Proposed* door while Stakeholder did not, leaving
    a newcomer locked between R-no-hand-edit-graph (the graph is writable only
    through apply_proposal) and the absence of a door. This kind is that missing
    door — the mechanical path by which a fresh accountability node is
    materialized without hand-editing the graph (R-no-hand-edit-graph).
    """

    id: str
    name: str
    domain: str
    why: str = ""

    def target_anchor(self) -> str:
        """Canon: §Closure — the stakeholder id is the anchor of this proposal."""
        return self.id


@dataclass(frozen=True)
class ProposedAssumption:
    """Canon: §Proposal / §Assumption — propose a new Assumption (falsifiable belief)
    to add to the active domain's graph.

    RULE: kind='Assumption'; the apply_proposal tool serializes this into a new
    Assumption(...) entry appended to the active domain's `assumptions` tuple.
    id MUST be unique (not already present in the graph's assumptions) and
    SHOULD start with 'A-' (R-anchor-everything). status MUST be one of
    hotam_spec.assumption.ASSUMPTION_STATES (HOLDS | UNCERTAIN | DEAD).
    owner MUST be a Stakeholder id.

    WHY this kind exists: latent-connector clustering (§Conflict —
    latent_connector_clusters) flags requirements that share an
    over-broad assumption as suspiciously linked. Splitting an over-broad
    assumption into narrower, more specific ones — each genuinely shared
    only by requirements that are actually about the same claim — is the
    mechanical remedy; this proposal kind is how a narrower Assumption node
    gets materialized without hand-editing the graph (R-no-hand-edit-graph).
    """

    id: str
    statement: str
    status: str
    owner: str  # Stakeholder id
    why: str = ""

    def target_anchor(self) -> str:
        """Canon: §Closure — the assumption id is the anchor of this proposal."""
        return self.id


@dataclass(frozen=True)
class ProposedAssumptionTransition:
    """Canon: §Proposal / §Assumption — propose CHANGING an existing Assumption's status
    (the assumption kill-path: HOLDS → UNCERTAIN → DEAD and back).

    RULE: kind='AssumptionTransition'. `assumption_id` MUST already exist in the
    active domain's graph. `new_status` MUST be one of ASSUMPTION_STATES
    (HOLDS | UNCERTAIN | DEAD | IMPLEMENTS). `reason` MUST be non-empty — a status change with
    no recorded reason is drift, not a decision. The apply_proposal tool UPDATES
    the existing Assumption(...) call's status= field in place and APPENDS the
    reason to its statement (NEVER deletes the node — the assumption survives its
    own death, mirroring R-rejected-preserved-not-deleted).

    SIGNOFF asymmetry (`decided_by`): a transition that REDUCES live signal needs
    a named human, a transition that RAISES it does not.
      - new_status == DEAD  → decided_by REQUIRED (a Stakeholder id). Killing a
        premise is a factual claim about the world with cluster-wide fallout —
        the same altitude as closing a Conflict, so it carries the same signoff
        lock (R-decided-needs-human-signoff, R-trust-anchor-mechanism). The AI
        NEVER kills an assumption silently (R-ai-presents-not-decides).
      - new_status == HOLDS → decided_by REQUIRED. Re-affirming a previously
        doubted premise SILENCES the review/fallout signal it was raising;
        re-trusting is as consequential as killing, so it too needs a human.
      - new_status == UNCERTAIN → decided_by OPTIONAL. Moving to UNCERTAIN only
        RAISES a question (it ADDS a P4 review signal, removes none); surfacing
        doubt is exactly the operator's PRESENT step, which R-ai-presents-not-
        decides permits the agent to do alone. (Decided honestly per
        thinking/assumption.md: UNCERTAIN is 'under question, not yet
        falsified' — a question opened, not a decision closed.)
      - new_status == IMPLEMENTS → decided_by REQUIRED (R-assumption-implements-
        state). IMPLEMENTS is the VOLITIONAL род (an aspiration, not a
        fact-claim). Whatever the source, re-typing a claim to IMPLEMENTS
        REMOVES live signal and CHANGES the род of the claim: from UNCERTAIN it
        silences the P4 doubt signal ('we understood this is not a fact but a
        goal'); from HOLDS it retracts a fact-declaration made too early; and it
        commits the graph to a stated striving. By the Wave-12 asymmetry (a
        transition that reduces live signal / changes the род needs a named
        human) it carries the signoff lock. The agent MAY still open plain
        UNCERTAIN alone, but declaring an aspiration is a steward act.

    TRANSITION TABLE (source → target : who signs):
      *          → UNCERTAIN   : agent (no signoff) — RAISES a doubt signal.
      *          → HOLDS       : human (decided_by)  — re-affirms a fact.
      *          → DEAD        : human (decided_by)  — kills a premise.
      UNCERTAIN  → IMPLEMENTS  : human (decided_by)  — 'not a fact, a goal';
                                 changes род, drops the P4 doubt signal.
      HOLDS      → IMPLEMENTS  : human (decided_by)  — 'declared fact too early'.
      IMPLEMENTS → HOLDS       : human (decided_by)  — 'achieved, became fact'.
      IMPLEMENTS → DEAD        : human (decided_by)  — 'abandoned the striving'.
    (The validator keys the lock on `new_status` alone: DEAD/HOLDS/IMPLEMENTS
    all require decided_by; only UNCERTAIN is agent-enterable.)

    WHY a transition kind at all: drift of assumptions is the DECLARED root of
    the methodology (§Assumption — 'the root of context drift'), yet with only
    ProposedAssumption (add-only) and the hand-edit lock, no assumption could
    EVER change status — the kill-path was mechanically absent, so a DEAD
    assumption's cluster-wide fallout (graph.dead_assumptions +
    graph.requirements_on_assumption, what_now P2 DRIFT_FALLOUT) could never
    actually fire. This kind is that
    missing edge.
    """

    assumption_id: str  # the A-… anchor being transitioned
    new_status: str  # HOLDS | UNCERTAIN | DEAD | IMPLEMENTS
    reason: str  # non-empty; appended to the Assumption's statement
    decided_by: str = ""  # REQUIRED when new_status in (DEAD, HOLDS, IMPLEMENTS)
    # §Signoff fields (optional; the writer builds a Signoff payload from these
    # when decided_by is non-empty, attached as Assumption.signoff — the LAST
    # transition's provenance. decided_by no longer evaporates into gitignored
    # JSON; R-trust-anchor-mechanism becomes auditable from the substrate):
    date: str = ""
    # ^ ISO YYYY-MM-DD; the date the steward signed. The writer fills today's
    # date as a default when this is empty.
    verbatim: str = ""
    # ^ the steward's own words carried verbatim (optional).
    instrument: str = "personal"
    # ^ HOW the signoff was captured (the verifiable-signature seam).

    def target_anchor(self) -> str:
        """Canon: §Closure — the Assumption id this transition is meant to change."""
        return self.assumption_id


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
    | ProposedAxis
    | ProposedAssumption
    | ProposedStakeholder
    | ProposedAssumptionTransition
)
