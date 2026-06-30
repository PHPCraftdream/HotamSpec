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

import re
from collections import deque
from dataclasses import dataclass

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
    return out


def check_canonical_lifecycles_wellformed(g: TensionGraph) -> list[Violation]:
    """Canon: §Lifecycle / §Invariants — the framework's own lifecycle constants are well-formed.

    RULE: REQUIREMENT_STATUS_LIFECYCLE, CONFLICT_LIFECYCLE, and OPERATOR_LIFECYCLE
    MUST each pass check_lifecycle_wellformed (no structural issues). This check
    runs on every invocation of the full invariant suite — the framework checks
    its own shipped state machines, not only user content.

    WHY self-application: strong self-application is the methodology's bootstrap
    test. If the framework's own lifecycles are malformed, all downstream status
    validation is meaningless. References: R-statemachine-wellformedness,
    R-lifecycle-abstraction.
    """
    out: list[Violation] = []
    for lc in (REQUIREMENT_STATUS_LIFECYCLE, CONFLICT_LIFECYCLE, OPERATOR_LIFECYCLE):
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
    check_typed_anchors,
    check_enforced_names_invariant,
    check_m_tag_format,
    check_status_in_lifecycle,
    check_canonical_lifecycles_wellformed,
    check_operator_steward_not_self,
    check_operator_within_budget,
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
