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

M7 resolved here (operator proposes, the goal hook ratifies via continuation):
The methodology's critical core is the six invariants in CRITICAL_CORE_INVARIANTS.
These six guard every path by which a contradiction could be INTRODUCED without
being seen — the hard boundary, the anti-drift discipline, the decision-moment
lock, the typed-anchor discipline, referential integrity, and visible openness.
All other invariants are structurally sound but occupy a SECONDARY ring: same
machinery, lower priority signal. The §Conscience Hypothesis sweep (test_conscience.py)
covers the critical core with property-tests; secondary invariants pass the same
suite but are not the primary conscience boundary.
"""

from __future__ import annotations

import ast
import re
from collections import deque
from dataclasses import dataclass
from pathlib import Path

from tensio.conflict import conflict_identity
from tensio.graph import (
    TensionGraph,
    assumption_ids,
    axis_slugs,
    operator_ids,
    requirement_ids,
    stakeholder_ids,
)
from tensio.lifecycle import (
    CONFLICT_LIFECYCLE,
    REQUIREMENT_STATUS_LIFECYCLE,
    Lifecycle,
)
from tensio.operator import OPERATOR_LIFECYCLE
from tensio.process import GOAL_LIFECYCLE, TARGET_KINDS
from tensio.requirement import ENFORCED, ENFORCEMENT_LEVELS, OPEN_PREFIX, RELATION_KINDS

_M_TAG_RE = re.compile(r"^M[1-9][0-9]*$")


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
    Conflict.derived[*], Assumption.owner, Operator.stakeholder, and
    Operator.parent MUST each name an object that exists.

    WHY first and broadest: a dangling member is how a conflict silently loses a
    party; a dangling assumption is how drift hides. A dangling edge is an
    invisible hole, the cardinal sin of the methodology.
    """
    sids, aids, rids = stakeholder_ids(g), assumption_ids(g), requirement_ids(g)
    oids = operator_ids(g)
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
        if c.decided_by and c.decided_by not in sids:
            fire(c.id, f"decided_by '{c.decided_by}' is not a known Stakeholder")
    for op in g.operators:
        if op.stakeholder not in sids:
            fire(
                op.id,
                f"operator stakeholder '{op.stakeholder}' is not a known Stakeholder",
            )
        if op.parent is not None and op.parent not in oids:
            fire(op.id, f"operator parent '{op.parent}' is not a known Operator")
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
# 5b. Signoff lock — a DECIDED conflict must name its human decider
# ---------------------------------------------------------------------------


def check_decided_has_decided_by(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — a DECIDED conflict names a human decider outside its members.

    RULE (R-decided-needs-human-signoff + §Proposal): when Conflict.lifecycle
    starts with "DECIDED", `decided_by` MUST satisfy three conditions:
      1. Non-empty (a DECIDED conflict without a named human decider is an
         AI-silently-closeable hole — exactly the invisibility the hard
         boundary forbids).
      2. Resolves to a known Stakeholder id (check_no_dangling_ids also
         catches this; this check names it explicitly for the harness).
      3. NOT the owner of any of the conflict's member Requirements (the
         steward-distinct boundary applied to the decider, not just the
         steward — the act of deciding must be distinct from owning a side).

    This is the structural twin of check_steward_not_a_member_owner applied
    at the moment of resolution: if the decider owned one of the members,
    the hard boundary would be circumvented at the decision step.

    WHY: R-decided-needs-human-signoff makes the closed loop's ACT half
    structurally visible. Without this lock, an AI could write
    lifecycle="DECIDED(...)" with decided_by="" and pass all other
    invariants. This invariant is the machine-checkable enforcement of
    "the human steward approves" (§Proposal — the closed loop's ACT half).
    """
    owner_of = {r.id: r.owner for r in g.requirements}
    sids = stakeholder_ids(g)
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.is_decided():
            continue
        if not c.decided_by:
            out.append(
                Violation(
                    "check_decided_has_decided_by",
                    c.id,
                    "DECIDED conflict must carry a non-empty decided_by "
                    "(the Stakeholder.id of the human who approved the resolution; "
                    "R-decided-needs-human-signoff)",
                )
            )
            continue
        if c.decided_by not in sids:
            out.append(
                Violation(
                    "check_decided_has_decided_by",
                    c.id,
                    f"decided_by '{c.decided_by}' is not a known Stakeholder",
                )
            )
            continue
        member_owners = {owner_of[m] for m in c.members if m in owner_of}
        if c.decided_by in member_owners:
            out.append(
                Violation(
                    "check_decided_has_decided_by",
                    c.id,
                    f"decided_by '{c.decided_by}' also owns a member requirement; "
                    f"the decider must be outside the conflict's members "
                    f"(steward-distinct rule applied to the decider)",
                )
            )
    return out


# ---------------------------------------------------------------------------
# 6. Typed-anchor prefixes — every id carries the right kind prefix
# ---------------------------------------------------------------------------


def check_typed_anchors(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every id carries the prefix that matches its kind.

    RULE: Requirement.id MUST start with 'R-'; Assumption.id MUST start with
    'A-'; Conflict.id MUST start with 'C-'; Operator.id MUST start with 'OP-'.
    An id with a wrong or missing prefix breaks the typed-anchor discipline
    (R-anchor-everything) and makes cite-by-reference unreliable
    (R-speak-by-reference).

    WHY minimal: this check enforces the CURRENTLY USED prefixes (R-/A-/C-/OP-)
    that are already discipline in the codebase; it does NOT yet encode the full
    M28 taxonomy (GOAL-/GAP-/DLG-/AX-) — those are still OPEN per
    R-anchor-taxonomy.

    References: R-anchor-everything (DRAFT), R-anchor-taxonomy (OPEN/M28).
    """
    out: list[Violation] = []
    for r in g.requirements:
        if not r.id.startswith("R-"):
            out.append(
                Violation(
                    "check_typed_anchors",
                    r.id,
                    f"Requirement id '{r.id}' must start with 'R-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    for a in g.assumptions:
        if not a.id.startswith("A-"):
            out.append(
                Violation(
                    "check_typed_anchors",
                    a.id,
                    f"Assumption id '{a.id}' must start with 'A-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    for c in g.conflicts:
        if not c.id.startswith("C-"):
            out.append(
                Violation(
                    "check_typed_anchors",
                    c.id,
                    f"Conflict id '{c.id}' must start with 'C-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    for op in g.operators:
        if not op.id.startswith("OP-"):
            out.append(
                Violation(
                    "check_typed_anchors",
                    op.id,
                    f"Operator id '{op.id}' must start with 'OP-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    for p in g.processes:
        if not p.id.startswith("PR-"):
            out.append(
                Violation(
                    "check_typed_anchors",
                    p.id,
                    f"Process id '{p.id}' must start with 'PR-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    for go in g.goals:
        if not go.id.startswith("GOAL-"):
            out.append(
                Violation(
                    "check_typed_anchors",
                    go.id,
                    f"Goal id '{go.id}' must start with 'GOAL-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    return out


# ---------------------------------------------------------------------------
# 7. Enforcement gradient — ENFORCED requirements must name their enforcer
# ---------------------------------------------------------------------------


def check_enforced_names_invariant(g: TensionGraph) -> list[Violation]:
    """Canon: §Requirement — every ENFORCED requirement names its enforcer(s).

    RULE (R-requirement-enforced): two conditions are checked for every
    Requirement:
      1. `enforcement` MUST be in ENFORCEMENT_LEVELS (PROSE | STRUCTURAL |
         ENFORCED); any other value is a misconfiguration.
      2. When `enforcement == ENFORCED`, `enforced_by` MUST be a non-empty
         tuple; an ENFORCED requirement with no named enforcer is an
         unverifiable claim — the guarantee cannot be audited.

    WHY: naming the enforcer is what makes "ENFORCED" mean something beyond
    PROSE; without the anchor the audit trail is broken and the burn-down
    meter cannot distinguish real reflexes from aspirational labels.
    An invalid enforcement level is rejected early so the UNENFORCED.md
    report is never built on corrupt data.
    """
    out: list[Violation] = []
    for r in g.requirements:
        if r.enforcement not in ENFORCEMENT_LEVELS:
            out.append(
                Violation(
                    "check_enforced_names_invariant",
                    r.id,
                    f"enforcement '{r.enforcement}' is not in ENFORCEMENT_LEVELS "
                    f"(PROSE | STRUCTURAL | ENFORCED)",
                )
            )
        elif r.enforcement == ENFORCED and not r.enforced_by:
            out.append(
                Violation(
                    "check_enforced_names_invariant",
                    r.id,
                    "enforcement is ENFORCED but enforced_by is empty; "
                    "name the check_* invariant or test that fires on violation",
                )
            )
    return out


# ---------------------------------------------------------------------------
# 8. M-tag format, uniqueness, and OPEN-only discipline
# ---------------------------------------------------------------------------


def check_m_tag_format(g: TensionGraph) -> list[Violation]:
    """Canon: §Requirement — every non-empty m_tag is valid, unique, and OPEN-only.

    RULE (three sub-rules):
      1. FORMAT: a non-empty `m_tag` MUST match `^M[1-9][0-9]*$` — "M" followed
         by a positive integer with no leading zeros (e.g. M3, M17, M26; not M01,
         m17, M, Mfoo). This is the typed-anchor discipline applied to M-tags.
      2. UNIQUE: no two Requirements in the graph may share the same `m_tag`. A
         duplicate tag breaks the bijection that `docs/gen/DECISIONS.md` relies on:
         one M-decision maps to exactly one Requirement.
      3. OPEN-ONLY: an `m_tag` SHOULD appear only on an OPEN requirement. An M-tag
         on a SETTLED, DRAFT, or REJECTED requirement fires a violation — the
         M-registry tracks live open decisions, not resolved or proposed ones.

    WHY: the M-tag field is the bridge between the graph and `docs/gen/DECISIONS.md`
    (the generated canonical M-registry). Invalid format breaks parsing; duplicates
    break the one-to-one mapping; non-OPEN tags would pollute the registry with
    decisions that are no longer open (R-drift-structurally-impossible applied to the
    M-registry itself — see U5).
    """
    out: list[Violation] = []
    seen_tags: dict[str, str] = {}  # tag -> first requirement id

    for r in g.requirements:
        if not r.m_tag:
            continue
        # Rule 1: format
        if not _M_TAG_RE.match(r.m_tag):
            out.append(
                Violation(
                    "check_m_tag_format",
                    r.id,
                    f"m_tag '{r.m_tag}' does not match ^M[1-9][0-9]*$ "
                    f"(must be 'M' followed by a positive integer, no leading zeros)",
                )
            )
        # Rule 2: uniqueness
        if r.m_tag in seen_tags:
            out.append(
                Violation(
                    "check_m_tag_format",
                    r.id,
                    f"m_tag '{r.m_tag}' is already used by '{seen_tags[r.m_tag]}'; "
                    f"each M-tag must be unique across the graph",
                )
            )
        else:
            seen_tags[r.m_tag] = r.id
        # Rule 3: OPEN-only
        if not r.status.startswith(OPEN_PREFIX):
            out.append(
                Violation(
                    "check_m_tag_format",
                    r.id,
                    f"m_tag '{r.m_tag}' appears on a non-OPEN requirement (status={r.status!r}); "
                    f"M-tags are only for OPEN requirements (the live M-decision registry)",
                )
            )

    return out


# ---------------------------------------------------------------------------
# 9. Lifecycle well-formedness helper + Lifecycle-status validators
# ---------------------------------------------------------------------------


def check_lifecycle_wellformed(lc: Lifecycle) -> list[str]:
    """Canon: §Lifecycle — return structural issues in a Lifecycle itself.

    RULE: a well-formed Lifecycle satisfies all four conditions below.
    Returns a list of human-readable issue strings (empty = well-formed).
    This is a plain helper (not a graph-level check_*); it is called by
    check_canonical_lifecycles_wellformed and by tests directly.

    Four conditions checked:
      1. states is non-empty.
      2. Exactly one INITIAL state.
      3. Every transition endpoint (src and dst) resolves to a declared state.
      4. If cyclic=False, at least one terminal/quiescent state is reachable
         from the INITIAL state via BFS on the transition graph.

    WHY BFS: deterministic traversal, no hidden ordering; only reachable
    states matter for the terminal-reachability check.
    """
    issues: list[str] = []
    if not lc.states:
        issues.append(f"{lc.slug}: Lifecycle has no states")
        return issues  # no further checks meaningful

    names = lc.state_names()

    # Condition 2: exactly one INITIAL
    initials = [s for s in lc.states if s.is_initial()]
    if len(initials) != 1:
        issues.append(
            f"{lc.slug}: expected exactly 1 INITIAL state, found {len(initials)}"
        )

    # Condition 3: every transition endpoint resolves
    for t in lc.transitions:
        if t.src not in names:
            issues.append(
                f"{lc.slug}: transition '{t.event}' has unknown src '{t.src}'"
            )
        if t.dst not in names:
            issues.append(
                f"{lc.slug}: transition '{t.event}' has unknown dst '{t.dst}'"
            )

    # Condition 4: if not cyclic, at least one terminal/quiescent is reachable
    if not lc.cyclic and initials:
        start = initials[0].name
        # BFS over transition graph
        reachable: set[str] = {start}
        queue: deque[str] = deque([start])
        adjacency: dict[str, list[str]] = {s.name: [] for s in lc.states}
        for t in lc.transitions:
            if t.src in adjacency:
                adjacency[t.src].append(t.dst)
        while queue:
            cur = queue.popleft()
            for nxt in adjacency.get(cur, []):
                if nxt not in reachable:
                    reachable.add(nxt)
                    queue.append(nxt)
        state_by_name = {s.name: s for s in lc.states}
        terminal_reachable = any(
            state_by_name[n].is_terminal() for n in reachable if n in state_by_name
        )
        if not terminal_reachable:
            issues.append(
                f"{lc.slug}: no terminal/quiescent state reachable from INITIAL "
                f"'{start}' (mark cyclic=True if intentional)"
            )

    return issues


def check_status_in_lifecycle(g: TensionGraph) -> list[Violation]:
    """Canon: §Lifecycle / §Invariants — every status/lifecycle value matches a canonical Lifecycle.

    RULE (three sub-rules):
      1. Every Requirement.status MUST be matched by REQUIREMENT_STATUS_LIFECYCLE
         (exact match for DRAFT/SETTLED/REJECTED; prefix match for OPEN(question)).
      2. Every Conflict.lifecycle MUST be matched by CONFLICT_LIFECYCLE
         (exact match for DETECTED/ACKNOWLEDGED; prefix match for
         DECIDED(rationale) and REVISIT_WHEN(condition)).
      3. Every Operator.lifecycle MUST be matched by OPERATOR_LIFECYCLE
         (exact match for ACTIVE/SATURATED/DELEGATED/RETIRED).

    When matches() returns None, the value is not a recognized state of the
    canonical lifecycle; a Violation is fired naming the offending value and
    lifecycle slug.

    WHY structural: status and lifecycle are hand-rolled string state machines;
    this invariant enforces that stored values belong to the canonical set,
    making the state machines structurally visible and checkable rather than
    only convention-held. References: R-lifecycle-abstraction,
    R-statemachine-wellformedness.
    """
    out: list[Violation] = []
    for r in g.requirements:
        if REQUIREMENT_STATUS_LIFECYCLE.matches(r.status) is None:
            out.append(
                Violation(
                    "check_status_in_lifecycle",
                    r.id,
                    f"Requirement.status '{r.status}' is not a valid state in "
                    f"lifecycle '{REQUIREMENT_STATUS_LIFECYCLE.slug}' "
                    f"(valid: {sorted(REQUIREMENT_STATUS_LIFECYCLE.state_names())})",
                )
            )
    for c in g.conflicts:
        if CONFLICT_LIFECYCLE.matches(c.lifecycle) is None:
            out.append(
                Violation(
                    "check_status_in_lifecycle",
                    c.id,
                    f"Conflict.lifecycle '{c.lifecycle}' is not a valid state in "
                    f"lifecycle '{CONFLICT_LIFECYCLE.slug}' "
                    f"(valid: {sorted(CONFLICT_LIFECYCLE.state_names())})",
                )
            )
    for op in g.operators:
        if OPERATOR_LIFECYCLE.matches(op.lifecycle) is None:
            out.append(
                Violation(
                    "check_status_in_lifecycle",
                    op.id,
                    f"Operator.lifecycle '{op.lifecycle}' is not a valid state in "
                    f"lifecycle '{OPERATOR_LIFECYCLE.slug}' "
                    f"(valid: {sorted(OPERATOR_LIFECYCLE.state_names())})",
                )
            )
    for go in g.goals:
        if GOAL_LIFECYCLE.matches(go.lifecycle) is None:
            out.append(
                Violation(
                    "check_status_in_lifecycle",
                    go.id,
                    f"Goal.lifecycle '{go.lifecycle}' is not a valid state in "
                    f"lifecycle '{GOAL_LIFECYCLE.slug}' "
                    f"(valid: {sorted(GOAL_LIFECYCLE.state_names())})",
                )
            )
    return out


def check_canonical_lifecycles_wellformed(g: TensionGraph) -> list[Violation]:
    """Canon: §Lifecycle / §Invariants — the framework's own lifecycle constants are well-formed.

    RULE: REQUIREMENT_STATUS_LIFECYCLE, CONFLICT_LIFECYCLE, OPERATOR_LIFECYCLE,
    PROCESS_LIFECYCLE, and GOAL_LIFECYCLE MUST each pass check_lifecycle_wellformed
    (no structural issues). This check runs on every invocation of the full
    invariant suite — the framework checks its own shipped state machines, not
    only user content.

    WHY self-application: strong self-application is the methodology's bootstrap
    test. If the framework's own lifecycles are malformed, all downstream status
    validation is meaningless. References: R-statemachine-wellformedness,
    R-lifecycle-abstraction, R-process-aspect-first.
    """
    from tensio.process import GOAL_LIFECYCLE as GL  # noqa: PLC0415
    from tensio.process import PROCESS_LIFECYCLE as PL  # noqa: PLC0415

    out: list[Violation] = []
    for lc in (
        REQUIREMENT_STATUS_LIFECYCLE,
        CONFLICT_LIFECYCLE,
        OPERATOR_LIFECYCLE,
        PL,
        GL,
    ):
        for issue in check_lifecycle_wellformed(lc):
            out.append(
                Violation(
                    "check_canonical_lifecycles_wellformed",
                    lc.slug,
                    issue,
                )
            )
    return out


# ---------------------------------------------------------------------------
# 10. Operator steward-safety — M36: operator cannot self-approve (§Operator)
# ---------------------------------------------------------------------------


def check_operator_steward_not_self(g: TensionGraph) -> list[Violation]:
    """Canon: §Operator / §Invariants — an Operator may not steward a Conflict that contains its own Stakeholder's requirement.

    RULE (M36): For each Conflict in the graph, collect the set of Stakeholder
    ids that own the conflict's member Requirements ('member-owners'). For each
    Operator whose `stakeholder` field is in that set, if any such Operator id
    equals the Conflict's `steward`, fire a Violation.

    Plain-English: an Operator is the acting facet of a Stakeholder
    (§Stakeholder); the steward-distinct boundary (check_steward_not_a_member_owner)
    applies THROUGH that facet — an Operator cannot steward a Conflict in which
    its own underlying Stakeholder owns one of the member Requirements.

    WHY (R-ai-presents-not-decides + R-operator-not-self-approve): the hard
    boundary that prevents an interested party from judging its own side extends
    to the acting facet. If an Operator could steward a conflict its Stakeholder
    has a stake in, the boundary would be defeated at the operator level while
    formally satisfied at the Stakeholder level — structural invisibility.

    This is the reflexive twin of check_steward_not_a_member_owner.
    """
    owner_of = {r.id: r.owner for r in g.requirements}
    # Map from stakeholder id -> operator ids that are that stakeholder's acting facet
    op_by_stakeholder: dict[str, list[str]] = {}
    for op in g.operators:
        op_by_stakeholder.setdefault(op.stakeholder, []).append(op.id)

    out: list[Violation] = []
    for c in g.conflicts:
        member_owners = {owner_of[m] for m in c.members if m in owner_of}
        for sid in member_owners:
            for op_id in op_by_stakeholder.get(sid, []):
                if c.steward == op_id:
                    out.append(
                        Violation(
                            "check_operator_steward_not_self",
                            c.id,
                            f"Operator '{op_id}' (acting facet of stakeholder "
                            f"'{sid}') cannot steward conflict '{c.id}' because "
                            f"its underlying Stakeholder owns a member requirement; "
                            f"M36 — operator must not self-approve "
                            f"(R-operator-not-self-approve)",
                        )
                    )
    return out


# ---------------------------------------------------------------------------
# 11. Operator budget — check_operator_within_budget (§ContextBudget, M17)
# ---------------------------------------------------------------------------


def check_operator_within_budget(g: TensionGraph) -> list[Violation]:
    """Canon: §ContextBudget / §Invariants — operator domain must not exceed its budget.

    RULE: for each Operator with `context_budget.limit > 0`:
      - If `measure == NODE_COUNT`, compute:
          size = len(g.requirements) + len(g.conflicts) + len(g.assumptions)
        (full-graph count; DomainScope narrowing is deferred to a later P-phase).
      - If `size > limit`, fire a Violation with the imperative:
          'crystallize first; if still over, spawn a sub-operator'
          (R-crystallize-before-split, R-context-budget-rule).
      - `limit == 0` means unbounded; the check is skipped for that operator.

    WHY NODE_COUNT (M17): deterministic and computable without token-estimation
    infrastructure; the TOKEN_ESTIMATE and COMPLEXITY measures are deferred
    behind a seam for future phases. See R-budget-measure and R-context-budget-rule.

    WHY fire (not warn): 'domain > context' is exactly the kind of measurable,
    structural contradiction Tensio exists to surface. An over-budget operator
    is a real conflict the graph holds visibly, not a soft warning.
    """
    from tensio.operator import NODE_COUNT  # noqa: PLC0415

    out: list[Violation] = []
    for op in g.operators:
        limit = op.context_budget.limit
        if limit <= 0:
            continue  # unbounded; aspect off for this operator
        if op.context_budget.measure == NODE_COUNT:
            size = len(g.requirements) + len(g.conflicts) + len(g.assumptions)
            if size > limit:
                out.append(
                    Violation(
                        "check_operator_within_budget",
                        op.id,
                        f"operator '{op.id}' holds {size} nodes > budget {limit} "
                        f"(NODE_COUNT measure); crystallize first "
                        f"(R-crystallize-before-split); if still over, spawn a "
                        f"sub-operator (R-context-bounded-delegation)",
                    )
                )
    return out


# ---------------------------------------------------------------------------
# 12. §Process aspect invariants (aspect-gated: no-op when g.processes empty)
# ---------------------------------------------------------------------------


def check_process_lifecycle_wellformed(g: TensionGraph) -> list[Violation]:
    """Canon: §Process / §Invariants — every Process lifecycle is structurally well-formed.

    RULE (aspect-gated): for each Process in g.processes, run
    check_lifecycle_wellformed(p.lifecycle); any issues become Violations.
    No-ops when g.processes is empty (§Process aspect not loaded).

    WHY structural: the §Lifecycle keystone is the single source of truth for
    state-machine well-formedness. Reusing check_lifecycle_wellformed here
    means the Process aspect inherits all four lifecycle conditions (non-empty,
    single INITIAL, valid transition endpoints, terminal reachable) without
    parallel machinery (R-statemachine-wellformedness, M12).
    """
    out: list[Violation] = []
    for p in g.processes:
        for issue in check_lifecycle_wellformed(p.lifecycle):
            out.append(
                Violation(
                    "check_process_lifecycle_wellformed",
                    p.id,
                    issue,
                )
            )
    return out


def check_process_roles_declared(g: TensionGraph) -> list[Violation]:
    """Canon: §Process / §Invariants — every Step.requires_role is in Process.roles_required.

    RULE (aspect-gated): for each Process p and each Step s in p.steps,
    s.requires_role MUST be in p.roles_required. A Step that demands a role
    not declared in the Process is a structural dead-end (the 'missing actor'
    contradiction). No-ops when g.processes is empty.

    WHY 'no implicit role': an undeclared role is invisible — the Process
    claims to need an actor it has never introduced. This is the structural
    twin of check_conflict_has_axis_context_steward applied to the behavioral
    altitude: every demanded role must be named. Supply ≥ demand is checked
    here; who fulfills each role is a future actor-matching invariant.
    """
    out: list[Violation] = []
    for p in g.processes:
        declared = frozenset(p.roles_required)
        for s in p.steps:
            if s.requires_role not in declared:
                out.append(
                    Violation(
                        "check_process_roles_declared",
                        p.id,
                        f"step '{s.name}' requires role '{s.requires_role}' "
                        f"which is not in Process.roles_required "
                        f"{sorted(declared)}; "
                        f"declare it explicitly (no implicit roles)",
                    )
                )
    return out


# ---------------------------------------------------------------------------
# 13. §Goal aspect invariants (aspect-gated: no-op when g.goals empty)
# ---------------------------------------------------------------------------


def check_goal_target_kind_known(g: TensionGraph) -> list[Violation]:
    """Canon: §Goal / §Invariants — every Goal.target_state.kind is in TARGET_KINDS.

    RULE (aspect-gated): for each Goal in g.goals, target_state.kind MUST be
    in TARGET_KINDS (GRAPH_PROPERTY | BUSINESS_STATE | ENTITY_STATE). An
    unknown kind is a misconfiguration that breaks the kind discriminant used
    by future machine-checkable predicates. No-ops when g.goals is empty.

    WHY a discriminant (not free-text): the kind field future-proofs Goal for
    machine-checkable predicates — the same seam as Assumption.machine_check.
    An unchecked kind lets two Goals with incompatible target types form a
    Conflict that the invariant surface can never detect.
    """
    out: list[Violation] = []
    for go in g.goals:
        if go.target_state.kind not in TARGET_KINDS:
            out.append(
                Violation(
                    "check_goal_target_kind_known",
                    go.id,
                    f"Goal.target_state.kind '{go.target_state.kind}' is not "
                    f"in TARGET_KINDS {sorted(TARGET_KINDS)}; "
                    f"use one of the declared kind constants",
                )
            )
    return out


def check_goal_owner_is_operator(g: TensionGraph) -> list[Violation]:
    """Canon: §Goal / §Operator / §Invariants — every Goal.owner resolves to a known Operator.

    RULE (aspect-gated): for each Goal in g.goals, Goal.owner MUST be in
    operator_ids(g). A Goal with a dangling owner is a structurally invisible
    target — no acting facet pursues it. No-ops when g.goals is empty.

    WHY Operator (not Stakeholder): a Goal drives a Process; Processes are
    executed by Operators (the acting facets). A Stakeholder without an
    Operator cannot run steps — the Goal would be declared but unexecuted.
    Referential integrity at the behavioral altitude (M19).
    """
    oids = operator_ids(g)
    out: list[Violation] = []
    for go in g.goals:
        if go.owner not in oids:
            out.append(
                Violation(
                    "check_goal_owner_is_operator",
                    go.id,
                    f"Goal.owner '{go.owner}' is not a known Operator id; "
                    f"a Goal must be owned by an Operator (the acting facet "
                    f"that pursues it) — M19",
                )
            )
    return out


# ---------------------------------------------------------------------------
# 14. Section-anchor coherence — every §-token in framework docstrings is known
# ---------------------------------------------------------------------------

_SECTION_TOKEN_RE = re.compile(r"§[A-Za-z][\w-]*")
_TENSIO_SRC = Path(__file__).resolve().parent


def check_section_anchors_known(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants / §Glossary — every section-anchor token in framework docstrings is known.

    RULE: every section-anchor token (pattern: section-sign followed by an
    identifier, e.g. the tokens used in 'Canon: §Requirement') found in any
    spec/src/tensio/*.py docstring MUST appear in
    `tensio.glossary.term_slugs()`. An unrecognised section-anchor token is
    an invented term (R-speak-by-reference violation) that makes
    R-anchor-everything structurally unsafe — the anchor does not resolve.

    WHY a structural invariant (not only a test): test_glossary_sync.py
    catches this at test-time, but the invariant surface makes it a P1
    STRUCTURE violation in the what_now harness — the same drift that
    breaks the glossary also raises the priority band, ensuring the steward
    sees it as the top-ranked action.

    Implementation mirrors test_glossary_sync.py::test_section_tokens_in_docstrings_are_known
    but returns Violation records keyed on the bad token so the harness can
    target them individually.

    References: R-anchor-everything, R-speak-by-reference,
    test_glossary_sync.py::test_section_tokens_in_docstrings_are_known.
    """
    from tensio.glossary import term_slugs  # noqa: PLC0415

    known = term_slugs()
    out: list[Violation] = []
    for path in sorted(_TENSIO_SRC.glob("*.py")):
        try:
            source = path.read_text(encoding="utf-8")
            tree = ast.parse(source)
        except (OSError, SyntaxError):
            continue
        # Collect all docstrings in the module
        docstrings: list[str] = []
        mod_doc = ast.get_docstring(tree)
        if mod_doc:
            docstrings.append(mod_doc)
        for node in ast.walk(tree):
            if isinstance(node, (ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)):
                doc = ast.get_docstring(node)
                if doc:
                    docstrings.append(doc)
        for doc in docstrings:
            for token in _SECTION_TOKEN_RE.findall(doc):
                if token not in known:
                    out.append(
                        Violation(
                            "check_section_anchors_known",
                            token,
                            f"§-token '{token}' appears in {path.name} docstring(s) "
                            f"but is not in glossary.term_slugs() "
                            f"(R-speak-by-reference: cite only admitted anchors)",
                        )
                    )
    # Deduplicate: same token may appear in multiple docstrings; one violation per token.
    seen: set[str] = set()
    deduped: list[Violation] = []
    for v in out:
        if v.target not in seen:
            seen.add(v.target)
            deduped.append(v)
    return deduped


# ---------------------------------------------------------------------------
# 15. R↔check_* bijection — enforcer names resolve; no orphan checks
# ---------------------------------------------------------------------------


def check_bijection_r_to_enforcer(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every enforcer name resolves; no orphan check_* exists.

    RULE (R-bijection-r-to-enforcer): two sub-rules enforce the bijection
    between SETTLED/ENFORCED requirements and the check_* functions in
    ALL_INVARIANTS:

      1. RESOLVABILITY: for every SETTLED requirement with enforcement==ENFORCED,
         each name in `enforced_by` that starts with 'check_' MUST resolve to a
         function in ALL_INVARIANTS. Names starting with 'test_' or other
         prefixes are exempt (they reference test files, not invariant functions).
         An unresolvable check_* name is an unverifiable claim — the named
         enforcer does not exist.

      2. ORPHAN DETECTION: every function in ALL_INVARIANTS MUST be named by at
         least one SETTLED/ENFORCED requirement's `enforced_by`. A check_*
         function that no SETTLED/ENFORCED requirement points to is an orphan —
         it runs but is not anchored to a claim, making the bijection
         incomplete.

    SHARED enforcers (one check_* named by multiple SETTLED/ENFORCED
    requirements) are acceptable — they are not violations today.

    WHY structural: the bijection between claim and check is what makes
    ENFORCED mean something beyond a label. An unresolvable enforcer name
    hides compoundness; an orphan check hides unclaimed guarantees.
    """
    inv_names = {fn.__name__ for fn in ALL_INVARIANTS}
    out: list[Violation] = []

    # Build map: check_name → [R-ids that name it]
    check_to_rids: dict[str, list[str]] = {name: [] for name in inv_names}
    has_enforced = False

    for r in g.requirements:
        if r.status != "SETTLED" or r.enforcement != ENFORCED:
            continue
        has_enforced = True
        for name in r.enforced_by:
            if name.startswith("check_"):
                if name not in inv_names:
                    out.append(
                        Violation(
                            "check_bijection_r_to_enforcer",
                            r.id,
                            f"enforced_by names '{name}' which is not a function "
                            f"in ALL_INVARIANTS (unresolvable check_* enforcer)",
                        )
                    )
                else:
                    check_to_rids[name].append(r.id)

    # Orphan detection: only meaningful when there are SETTLED/ENFORCED requirements.
    # An empty graph or a graph with no ENFORCED requirements has no bijection to check.
    if has_enforced:
        for name, rids in sorted(check_to_rids.items()):
            if not rids:
                out.append(
                    Violation(
                        "check_bijection_r_to_enforcer",
                        name,
                        f"check_* function '{name}' exists in ALL_INVARIANTS but "
                        f"is not named by any SETTLED/ENFORCED requirement's "
                        f"enforced_by (orphan enforcer — anchor it to a "
                        f"Requirement)",
                    )
                )

    return out


# ---------------------------------------------------------------------------
# Domain and agent filesystem invariants (Concern 6 / P17 task #64)
# These iterate the filesystem, not the graph; g is accepted for signature
# compatibility with the check_* protocol but is not used.
# ---------------------------------------------------------------------------

_REPO_ROOT_FROM_INVARIANTS = Path(__file__).resolve().parents[3]  # .../HotamSpec
_DOMAINS_ROOT = _REPO_ROOT_FROM_INVARIANTS / "domains"


def _resolve_spec_agents_root() -> Path:
    """Return the active spec/agents root.

    After P17 migration, agents live inside domains/<first>/agents/director/agents/.
    Legacy fallback: spec/agents/ for pre-migration layouts.
    """
    if _DOMAINS_ROOT.exists():
        domain_dirs = sorted(
            d
            for d in _DOMAINS_ROOT.iterdir()
            if d.is_dir() and not d.name.startswith("_")
        )
        for domain_dir in domain_dirs:
            director_agents = domain_dir / "agents" / "director" / "agents"
            if director_agents.exists():
                return director_agents
    return Path(__file__).resolve().parents[3] / "spec" / "agents"


_SPEC_AGENTS_ROOT = _resolve_spec_agents_root()


def check_domain_manifest_valid(g: TensionGraph) -> list[Violation]:  # noqa: ARG001
    """Canon: §Domain — every domains/<name>/manifest.py defines ID (matching dirname), DESCRIPTION, GOALS, DIRECTOR.

    RULE: A domain without a valid manifest is invisible to the framework.
    WHY: The manifest is the stable identity anchor for a domain; missing or
    mismatched fields make the domain undiscoverable by gen_spec and
    create_domain tooling (R-domain-has-manifest).
    """
    if not _DOMAINS_ROOT.exists():
        return []
    import importlib.util  # noqa: PLC0415

    out: list[Violation] = []
    for domain_dir in sorted(_DOMAINS_ROOT.iterdir()):
        if not domain_dir.is_dir():
            continue
        manifest_py = domain_dir / "manifest.py"
        if not manifest_py.exists():
            out.append(
                Violation(
                    "check_domain_manifest_valid",
                    domain_dir.name,
                    f"domains/{domain_dir.name}/manifest.py is missing — "
                    "every domain must declare ID, DESCRIPTION, GOALS, DIRECTOR",
                )
            )
            continue
        spec = importlib.util.spec_from_file_location(
            f"_manifest_{domain_dir.name}", manifest_py
        )
        if spec is None or spec.loader is None:
            out.append(
                Violation(
                    "check_domain_manifest_valid",
                    domain_dir.name,
                    f"Cannot load manifest.py for domain '{domain_dir.name}'",
                )
            )
            continue
        mod = importlib.util.module_from_spec(spec)
        try:
            spec.loader.exec_module(mod)  # type: ignore[union-attr]
        except Exception as exc:
            out.append(
                Violation(
                    "check_domain_manifest_valid",
                    domain_dir.name,
                    f"manifest.py import error: {exc}",
                )
            )
            continue
        domain_id = getattr(mod, "ID", None)
        description = getattr(mod, "DESCRIPTION", None)
        goals = getattr(mod, "GOALS", None)
        director = getattr(mod, "DIRECTOR", None)
        if domain_id != domain_dir.name:
            out.append(
                Violation(
                    "check_domain_manifest_valid",
                    domain_dir.name,
                    f"manifest.py ID '{domain_id}' does not match dirname '{domain_dir.name}'",
                )
            )
        if not description:
            out.append(
                Violation(
                    "check_domain_manifest_valid",
                    domain_dir.name,
                    "manifest.py DESCRIPTION is empty",
                )
            )
        if not goals:
            out.append(
                Violation(
                    "check_domain_manifest_valid",
                    domain_dir.name,
                    "manifest.py GOALS is empty or missing",
                )
            )
        if not director:
            out.append(
                Violation(
                    "check_domain_manifest_valid",
                    domain_dir.name,
                    "manifest.py DIRECTOR is empty or missing",
                )
            )
    return out


def check_domain_director_exists(g: TensionGraph) -> list[Violation]:  # noqa: ARG001
    """Canon: §Domain — every domains/<name>/agents/<DIRECTOR>/scope.py must exist.

    RULE: A domain whose declared director agent is missing is headless.
    WHY: The director is the entry point for all domain-level operator delegation
    (R-domain-declares-director). Missing scope.py means the agent is not
    discoverable by gen_spec or invoke_agent.
    """
    if not _DOMAINS_ROOT.exists():
        return []
    import importlib.util  # noqa: PLC0415

    out: list[Violation] = []
    for domain_dir in sorted(_DOMAINS_ROOT.iterdir()):
        if not domain_dir.is_dir():
            continue
        manifest_py = domain_dir / "manifest.py"
        if not manifest_py.exists():
            continue  # already caught by check_domain_manifest_valid
        spec = importlib.util.spec_from_file_location(
            f"_manifest_dir_{domain_dir.name}", manifest_py
        )
        if spec is None or spec.loader is None:
            continue
        mod = importlib.util.module_from_spec(spec)
        try:
            spec.loader.exec_module(mod)  # type: ignore[union-attr]
        except Exception:
            continue
        director = getattr(mod, "DIRECTOR", None)
        if not director:
            continue
        scope_py = domain_dir / "agents" / director / "scope.py"
        if not scope_py.exists():
            out.append(
                Violation(
                    "check_domain_director_exists",
                    domain_dir.name,
                    f"domains/{domain_dir.name}/agents/{director}/scope.py is missing — "
                    f"director agent '{director}' declared in manifest but not scaffolded",
                )
            )
    return out


def check_agent_has_agents_subdir(g: TensionGraph) -> list[Violation]:  # noqa: ARG001
    """Canon: §Agent — every agent directory must contain an 'agents/' subdirectory.

    RULE: Every agent is itself a potential director that can spawn sub-agents;
    the agents/ subdir is the recursion slot (R-agent-is-recursive-director).
    WHY: Without the subdir the recursive delegation pattern collapses —
    create_agent.py always scaffolds it; its absence indicates manual corruption.
    """
    if not _SPEC_AGENTS_ROOT.exists():
        return []
    out: list[Violation] = []
    for agent_dir in sorted(_SPEC_AGENTS_ROOT.iterdir()):
        if not agent_dir.is_dir():
            continue
        if not (agent_dir / "scope.py").exists():
            continue  # not a valid agent dir
        agents_subdir = agent_dir / "agents"
        if not agents_subdir.exists():
            out.append(
                Violation(
                    "check_agent_has_agents_subdir",
                    agent_dir.name,
                    f"spec/agents/{agent_dir.name}/agents/ is missing — "
                    "every agent must have an agents/ subdir for recursive delegation",
                )
            )
    return out


def check_agent_has_docs_subdir(g: TensionGraph) -> list[Violation]:  # noqa: ARG001
    """Canon: §Agent — every agent directory must contain a 'docs/' subdirectory.

    RULE: Every agent carries its own docs/ for generated CLAUDE.md and thinking
    fragments scoped to its domain (R-agent-has-docs-dir).
    WHY: Without docs/ the agent cannot receive generated shared-docs links;
    create_agent.py always scaffolds it; its absence indicates manual corruption.
    """
    if not _SPEC_AGENTS_ROOT.exists():
        return []
    out: list[Violation] = []
    for agent_dir in sorted(_SPEC_AGENTS_ROOT.iterdir()):
        if not agent_dir.is_dir():
            continue
        if not (agent_dir / "scope.py").exists():
            continue
        docs_subdir = agent_dir / "docs"
        if not docs_subdir.exists():
            out.append(
                Violation(
                    "check_agent_has_docs_subdir",
                    agent_dir.name,
                    f"spec/agents/{agent_dir.name}/docs/ is missing — "
                    "every agent must have a docs/ subdir for generated docs",
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
    check_decided_has_decided_by,
    check_typed_anchors,
    check_enforced_names_invariant,
    check_m_tag_format,
    check_status_in_lifecycle,
    check_canonical_lifecycles_wellformed,
    check_operator_steward_not_self,
    check_operator_within_budget,
    # §Process aspect invariants (aspect-gated: no-op when g.processes empty)
    check_process_lifecycle_wellformed,
    check_process_roles_declared,
    # §Goal aspect invariants (aspect-gated: no-op when g.goals empty)
    check_goal_target_kind_known,
    check_goal_owner_is_operator,
    check_section_anchors_known,
    check_bijection_r_to_enforcer,
    # §Domain + §Agent filesystem invariants (P17 task #64)
    check_domain_manifest_valid,
    check_domain_director_exists,
    check_agent_has_agents_subdir,
    check_agent_has_docs_subdir,
)

# --- M7: the critical core ---
# These six invariants guard paths by which contradictions could be INTRODUCED
# without being seen. They get the Hypothesis property-sweep treatment
# (test_conscience.py). Other invariants are still in ALL_INVARIANTS and tested
# normally; this constant marks the boundary.
#
# Canon: §Conscience — the critical core is the methodology's own hard boundary
# made narrow and machine-checkable (M7 resolved). Secondary-ring invariants
# (e.g. check_axis_in_registry, check_conflict_id_matches_identity) are still in
# ALL_INVARIANTS and receive the same Hypothesis machinery but at lower priority.
CRITICAL_CORE_INVARIANTS = (
    check_steward_not_a_member_owner,
    check_operator_steward_not_self,
    check_decided_has_decided_by,
    check_typed_anchors,
    check_no_dangling_ids,
    check_open_has_question,
)


def all_violations(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — run every structural invariant, concatenate violations.

    RULE: the graph is structurally well-formed iff this returns []. The harness
    (tools/what_now.py) and the Drive organ (§Tick) call this first; a structural
    failure outranks every softer signal because a malformed graph makes all
    downstream diagnosis unreliable.

    WHY one entry point: keeps tests, the gate and the harness reading the exact
    same set of invariants in the exact same order (determinism). The §Tick driver
    (P5) calls diagnose() which calls this; §Tick is advisory (M32 conservative).

    Canon: §Conscience — CRITICAL_CORE_INVARIANTS is the narrow set of six
    invariants whose violation would silently break the hard boundary or anti-drift.
    The §Conscience Hypothesis sweep (test_conscience.py) runs property-tests over
    this boundary; all_violations runs the full set (both rings).
    """
    out: list[Violation] = []
    for check in ALL_INVARIANTS:
        out.extend(check(g))
    return out
