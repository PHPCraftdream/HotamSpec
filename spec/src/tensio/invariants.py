"""Canon: §Invariants — structural form of the tension graph (the check_* layer).

These are the spec-stack layer-2 invariants: the SHAPE the graph must always
hold, regardless of how many requirements contradict each other. They are the
inversion of dev-coin's consistency invariants — here a green run does NOT mean
"no contradictions"; contradictions are expected and welcome. Green means the
contradictions are WELL-FORMED: every conflict has an axis, a context and a
steward; no edge dangles; every open hole states its question; every decision
justifies itself. A conflict that is invisible (stewardless, axis-less) is the
one thing forbidden.

CONTRACT of each check: `check_*(graph) -> list[Violation]`. An EMPTY list means
the invariant holds. Each Violation names the offending object id and an
imperative message, so the harness (tools/what_now.py) turns failures directly
into prioritized next-actions. A boolean view is `holds(check(graph))`.

WHY return violations, not bool: dev-coin's check_* return bool because there the
goal is a single pass/fail gate. Here the SAME functions feed the "what now"
diagnosis, which needs the offending id and a human imperative — so the richer
return type is load-bearing, and `holds()` recovers the boolean when a test just
wants pass/fail.
"""

from __future__ import annotations

from dataclasses import dataclass

from tensio.conflict import conflict_identity
from tensio.graph import (
    TensionGraph,
    assumption_ids,
    axis_slugs,
    requirement_ids,
    stakeholder_ids,
)
from tensio.requirement import RELATION_KINDS


@dataclass(frozen=True)
class Violation:
    """Canon: §Invariants — one structural defect: which object, what to fix.

    Fields:
      invariant — the check_* name that fired (the rule).
      target    — the offending object id (Requirement/Conflict/Assumption/...).
      message   — imperative fix instruction, surfaced verbatim by the harness.

    WHY a record (not a string): the harness needs `target` to build a typed,
    addressable next-action; the message is the human imperative.
    """

    invariant: str
    target: str
    message: str


def holds(violations: list[Violation]) -> bool:
    """Canon: §Invariants — True iff there are no violations (boolean view).

    WHY: tests and gates that only care pass/fail call holds(check(g)); the
    harness consumes the list itself.
    """
    return len(violations) == 0


# ---------------------------------------------------------------------------
# 1. Referential integrity — no dangling ids anywhere
# ---------------------------------------------------------------------------


def check_no_dangling_ids(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every id referenced by an edge resolves in the graph.

    RULE: Requirement.owner, Requirement.assumptions[*], Relation.target,
    Conflict.steward, Conflict.members[*], Conflict.shared_assumption,
    Conflict.derived[*] and Assumption.owner MUST each name an object that exists.

    WHY first and broadest: a dangling member is how a conflict silently loses a
    party; a dangling assumption is how drift hides. A dangling edge is an
    invisible hole, the cardinal sin of the methodology.
    """
    sids, aids, rids = stakeholder_ids(g), assumption_ids(g), requirement_ids(g)
    out: list[Violation] = []

    def fire(target: str, msg: str) -> None:
        out.append(Violation("check_no_dangling_ids", target, msg))

    for a in g.assumptions:
        if a.owner not in sids:
            fire(a.id, f"assumption owner '{a.owner}' is not a known Stakeholder")
    for r in g.requirements:
        if r.owner not in sids:
            fire(r.id, f"requirement owner '{r.owner}' is not a known Stakeholder")
        for aid in r.assumptions:
            if aid not in aids:
                fire(r.id, f"assumption '{aid}' is not a known Assumption")
        for rel in r.relations:
            if rel.kind not in RELATION_KINDS:
                fire(r.id, f"relation kind '{rel.kind}' is not a known kind")
            if rel.target not in rids:
                fire(r.id, f"relation target '{rel.target}' is not a known Requirement")
    for c in g.conflicts:
        if c.steward not in sids:
            fire(c.id, f"steward '{c.steward}' is not a known Stakeholder")
        for mid in c.members:
            if mid not in rids:
                fire(c.id, f"member '{mid}' is not a known Requirement")
        if c.shared_assumption is not None and c.shared_assumption not in aids:
            fire(c.id, f"shared_assumption '{c.shared_assumption}' is not known")
        for did in c.derived:
            if did not in rids:
                fire(c.id, f"derived '{did}' is not a known Requirement")
    return out


# ---------------------------------------------------------------------------
# 2. A conflict is a CONNECTOR — axis, context, steward all present
# ---------------------------------------------------------------------------


def check_conflict_has_axis_context_steward(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — every Conflict carries a non-empty axis, context, steward.

    RULE: axis, context and steward MUST all be non-empty. These three are the
    knowledge that belongs to neither member; a conflict missing any of them is
    not a connector node, it is the empty `conflicts_with` edge we reject.

    WHY: this is the structural definition of "the contradiction is visible". An
    axis-less or stewardless conflict is exactly an invisible contradiction.
    """
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.axis.strip():
            out.append(
                Violation(
                    "check_conflict_has_axis_context_steward",
                    c.id,
                    "conflict has no tension axis (along WHAT do they diverge?)",
                )
            )
        if not c.context.strip():
            out.append(
                Violation(
                    "check_conflict_has_axis_context_steward",
                    c.id,
                    "conflict has no context (in WHICH scenario do they collide?)",
                )
            )
        if not c.steward.strip():
            out.append(
                Violation(
                    "check_conflict_has_axis_context_steward",
                    c.id,
                    "conflict has no steward (WHO holds this tension?)",
                )
            )
    return out


def check_conflict_min_two_members(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — every Conflict mediates >= 2 distinct requirements.

    RULE: members MUST contain at least two DISTINCT Requirement ids. A conflict
    with fewer is not a tension between parties.

    WHY: a connector node connects; with one (or zero) members there is nothing to
    hold between, and clustering/lineage become meaningless.
    """
    out: list[Violation] = []
    for c in g.conflicts:
        if len(set(c.members)) < 2:
            out.append(
                Violation(
                    "check_conflict_min_two_members",
                    c.id,
                    "conflict needs >= 2 distinct member requirements",
                )
            )
    return out


def check_axis_in_registry(g: TensionGraph) -> list[Violation]:
    """Canon: §Axis — every Conflict.axis is a slug in the graph's vocabulary.

    RULE: Conflict.axis MUST be in `axis_slugs(g)` — i.e. the slug of some Axis
    in TensionGraph.axes. An unknown or ad-hoc axis is rejected so conflicts
    CLUSTER (one tension dimension = one slug, not two synonyms splitting a
    cluster).

    WHY: clustering by axis is how a node-graph reveals an architectural choice;
    free-text axes would fragment the cluster and hide it. Since the framework
    is content-free, the per-domain vocabulary lives on the graph itself; new
    dimension = a new Axis row in the domain's `axes` (AI duplicate-gatekeeper
    deferred).
    """
    slugs = axis_slugs(g)
    out: list[Violation] = []
    for c in g.conflicts:
        if c.axis and c.axis not in slugs:
            out.append(
                Violation(
                    "check_axis_in_registry",
                    c.id,
                    f"axis '{c.axis}' is not in the controlled vocabulary "
                    f"(add it to the graph's `axes` tuple or pick an existing slug)",
                )
            )
    return out


def check_conflict_id_matches_identity(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — id == conflict_identity(axis, context).

    RULE: a Conflict's id MUST be the deterministic hash of (axis, context). A
    hand-written id is rejected so the node's identity tracks its TENSION, not its
    members, and survives member renaming/splitting.

    WHY: identity-from-tension is what makes the same conflict survive churn and
    keeps clustering stable; a free id would let the node drift from its meaning.
    """
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.axis or not c.context:
            continue  # axis/context emptiness is reported by its own invariant
        expected = conflict_identity(c.axis, c.context)
        if c.id != expected:
            out.append(
                Violation(
                    "check_conflict_id_matches_identity",
                    c.id,
                    f"conflict id should be '{expected}' "
                    f"(= conflict_identity(axis, context))",
                )
            )
    return out


# ---------------------------------------------------------------------------
# 3. The boundary — a steward stands OUTSIDE the requirements in tension
# ---------------------------------------------------------------------------


def check_steward_not_a_member_owner(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict / §Stakeholder — steward is not the owner of any member.

    RULE: Conflict.steward MUST NOT equal the owner of any member Requirement. A
    conflict lives BETWEEN stakeholders; if the steward owned a side, the tension
    would be judged by an interested party and quietly resolved in their favor.

    WHY this is the hard boundary made structural: it is the same principle as the
    AI never closing a conflict silently — the holder of the tension must be a
    party who does not own either claim, or invisibility returns.
    """
    owner_of = {r.id: r.owner for r in g.requirements}
    out: list[Violation] = []
    for c in g.conflicts:
        member_owners = {owner_of[m] for m in c.members if m in owner_of}
        if c.steward in member_owners:
            out.append(
                Violation(
                    "check_steward_not_a_member_owner",
                    c.id,
                    f"steward '{c.steward}' also owns a member requirement; "
                    f"a conflict must be stewarded from outside its members",
                )
            )
    return out


# ---------------------------------------------------------------------------
# 4. Visibility of the open — an OPEN hole must state its question
# ---------------------------------------------------------------------------


def check_open_has_question(g: TensionGraph) -> list[Violation]:
    """Canon: §Requirement — an OPEN requirement carries a non-empty question.

    RULE: if status starts with "OPEN", it MUST be of the form "OPEN(<question>)"
    with a non-empty question. An OPEN with no question is a hole nobody can act
    on — invisible openness.

    WHY: the harness and OPEN.md surface open holes by their question; an empty
    question gives the steward nothing to decide, defeating the point of marking
    it open at all.
    """
    out: list[Violation] = []
    for r in g.requirements:
        if not r.is_open():
            continue
        inside = r.status[len("OPEN") :].strip()
        question = (
            inside[1:-1].strip()
            if inside.startswith("(") and inside.endswith(")")
            else ""
        )
        if not question:
            out.append(
                Violation(
                    "check_open_has_question",
                    r.id,
                    "OPEN requirement must state a non-empty question: "
                    "status = 'OPEN(<question>)'",
                )
            )
    return out


# ---------------------------------------------------------------------------
# 5. A decision must justify itself (anti-relitigation)
# ---------------------------------------------------------------------------


def check_decided_has_rationale_or_derived(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — a DECIDED conflict records rationale or a derived req.

    RULE: if lifecycle starts with "DECIDED", it MUST carry a non-empty rationale
    inside "DECIDED(<rationale>)" OR a non-empty `derived` tuple. A decision with
    neither is a silent close — forbidden.

    WHY: the historian role depends on every decision carrying its rationale and
    (often) the requirement it spawned; without that the resolution is invisible
    and gets relitigated. This is the anti-relitigation marker made structural.
    """
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.is_decided():
            continue
        inside = c.lifecycle[len("DECIDED") :].strip()
        rationale = (
            inside[1:-1].strip()
            if inside.startswith("(") and inside.endswith(")")
            else ""
        )
        if not rationale and not c.derived:
            out.append(
                Violation(
                    "check_decided_has_rationale_or_derived",
                    c.id,
                    "DECIDED conflict must record a rationale 'DECIDED(<why>)' "
                    "or reference a derived requirement",
                )
            )
    return out


# ---------------------------------------------------------------------------
# Registry of all structural invariants (single source for tests + harness)
# ---------------------------------------------------------------------------

ALL_INVARIANTS = (
    check_no_dangling_ids,
    check_conflict_has_axis_context_steward,
    check_conflict_min_two_members,
    check_axis_in_registry,
    check_conflict_id_matches_identity,
    check_steward_not_a_member_owner,
    check_open_has_question,
    check_decided_has_rationale_or_derived,
)


def all_violations(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — run every structural invariant, concatenate violations.

    RULE: the graph is structurally well-formed iff this returns []. The harness
    calls this first; a structural failure outranks every softer signal because a
    malformed graph makes all downstream diagnosis unreliable.

    WHY one entry point: keeps tests, the gate and the harness reading the exact
    same set of invariants in the exact same order (determinism).
    """
    out: list[Violation] = []
    for check in ALL_INVARIANTS:
        out.extend(check(g))
    return out
