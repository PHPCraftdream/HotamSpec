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
import functools
import inspect
import re
from collections import deque
from dataclasses import dataclass
from pathlib import Path


#: Intra-process memo for all_violations(g), keyed by id(g). The stored graph
#: reference guards against CPython id reuse: a cache hit is honored only when
#: the live object is the SAME frozen graph. Pure function of a frozen graph, so
#: reusing the result within one process is deterministic; a fresh process (a new
#: env pin) starts empty and nothing is persisted to disk.
_ALL_VIOLATIONS_CACHE = {}  # type: dict[int, tuple[TensionGraph, list[Violation]]]


@functools.lru_cache(maxsize=None)
def _cached_parse_path(path_str: str) -> ast.Module | None:
    """Parse a source file's AST, memoized by absolute path (intra-process).

    Source files do not change within one process (a gen_spec/pytest run), so
    the same ~55 files are re-parsed dozens of times across the check_* layer.
    Returns None on OSError/SyntaxError so callers keep their skip semantics.
    A fresh process starts with an empty cache; nothing is persisted to disk.
    """
    try:
        source = Path(path_str).read_text(encoding="utf-8")
        return ast.parse(source)
    except (OSError, SyntaxError):
        return None


@functools.lru_cache(maxsize=None)
def _cached_parse_source_of(fn: object) -> ast.Module | None:
    """Parse a function's own source via inspect, memoized by function object.

    Same function object => same source within one process. Returns None on
    OSError/TypeError/SyntaxError to preserve the caller's empty-result skip.
    """
    try:
        source = inspect.getsource(fn)  # type: ignore[arg-type]
        return ast.parse(source)
    except (OSError, TypeError, SyntaxError):
        return None

from hotam_spec.assumption import ASSUMPTION_STATES
from hotam_spec.conflict import conflict_identity
from hotam_spec.enforcer_resolution import (
    check_to_tests_map as _enforcer_check_to_tests_map,
    resolve_one_enforcer as _enforcer_resolve_one,
)
from hotam_spec.graph import (
    TensionGraph,
    assumption_ids,
    axis_slugs,
    operator_ids,
    requirement_ids,
    stakeholder_ids,
)
from hotam_spec.lifecycle import (
    CONFLICT_LIFECYCLE,
    REQUIREMENT_STATUS_LIFECYCLE,
    Lifecycle,
)
from hotam_spec.operator import OPERATOR_LIFECYCLE
from hotam_spec.process import GOAL_LIFECYCLE, TARGET_KINDS
from hotam_spec.requirement import (
    ENFORCEABILITY_KINDS,
    ENFORCED,
    ENFORCEMENT_LEVELS,
    OPEN_PREFIX,
    RELATION_KINDS,
    SETTLED,
)

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
# Internal helpers — not in ALL_INVARIANTS; used by multiple check_* functions
# ---------------------------------------------------------------------------


def _requirement_owner_map(g: TensionGraph) -> dict[str, str]:
    """Return {requirement_id: owner} for all requirements in the graph.

    Centralised here so check_* functions that cross-reference requirement
    owners do not each embed a ``for r in g.requirements`` comprehension,
    which the atomicity audit treats as a second entity loop.
    """
    return {r.id: r.owner for r in g.requirements}


def _stakeholder_to_operator_ids(g: TensionGraph) -> dict[str, list[str]]:
    """Return {stakeholder_id: [operator_id, ...]} for all operators in the graph.

    Centralised here so check_* functions that look up operators by stakeholder
    do not embed a ``for op in g.operators`` loop at the call site, which the
    atomicity audit would count as a second entity loop.
    """
    result: dict[str, list[str]] = {}
    for op in g.operators:
        result.setdefault(op.stakeholder, []).append(op.id)
    return result


# ---------------------------------------------------------------------------
# 1. Referential integrity — no dangling ids anywhere (atomized sub-checks)
# ---------------------------------------------------------------------------


def check_no_dangling_assumption_owner(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Assumption.owner resolves to a known Stakeholder.

    RULE: Assumption.owner MUST be in stakeholder_ids(g). A dangling assumption
    owner is an invisible hole — the methodology cannot surface context drift
    if the assumption is unowned.

    WHY: a dangling owner makes the assumption unanchored; drift detection depends
    on assumptions having a live, resolvable owner.
    """
    sids = stakeholder_ids(g)
    out: list[Violation] = []
    for a in g.assumptions:
        if a.owner not in sids:
            out.append(
                Violation(
                    "check_no_dangling_assumption_owner",
                    a.id,
                    f"assumption owner '{a.owner}' is not a known Stakeholder",
                )
            )
    return out


def check_assumption_status_valid(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Assumption.status is a known ASSUMPTION_STATE.

    RULE (R-assumption-implements-state): for each Assumption, `status` MUST be
    one of ASSUMPTION_STATES (HOLDS | UNCERTAIN | DEAD | IMPLEMENTS). An
    unrecognised status is drift — the harness's status-keyed filters
    (dead_assumptions, uncertain_assumptions) silently skip it, so it would sit
    in the graph invisible to every diagnosis.

    WHY this is the enforcer of the IMPLEMENTS род: IMPLEMENTS is the fourth,
    VOLITIONAL status (an aspiration — 'we strive to make this true'), distinct
    from the three epistemic fact-claim statuses. This single-field
    set-membership check is what makes the new status a first-class, admitted
    value rather than an unchecked string: it accepts IMPLEMENTS and rejects any
    value outside ASSUMPTION_STATES.
    """
    out: list[Violation] = []
    for a in g.assumptions:
        if a.status not in ASSUMPTION_STATES:
            out.append(
                Violation(
                    "check_assumption_status_valid",
                    a.id,
                    f"Assumption status '{a.status}' is not one of "
                    f"{sorted(ASSUMPTION_STATES)} (R-assumption-implements-state)",
                )
            )
    return out


def check_no_dangling_requirement_owner(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Requirement.owner resolves to a known Stakeholder.

    RULE: Requirement.owner MUST be in stakeholder_ids(g). A requirement without
    a resolvable owner is structurally unanchored.

    WHY: a dangling owner makes the requirement unanchored and breaks the steward
    boundary invariant downstream.
    """
    sids = stakeholder_ids(g)
    out: list[Violation] = []
    for r in g.requirements:
        if r.owner not in sids:
            out.append(
                Violation(
                    "check_no_dangling_requirement_owner",
                    r.id,
                    f"requirement owner '{r.owner}' is not a known Stakeholder",
                )
            )
    return out


def check_no_dangling_requirement_assumptions(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Requirement.assumptions[*] resolves to a known Assumption.

    RULE: each id in Requirement.assumptions MUST be in assumption_ids(g). A dangling
    assumption reference hides drift — if the assumption never existed the dependency
    is invisible.

    WHY: drift detection (DRIFT_FALLOUT band) traverses assumption dependencies; a
    dangling reference breaks the traversal silently.
    """
    aids = assumption_ids(g)
    out: list[Violation] = []
    for r in g.requirements:
        for aid in r.assumptions:
            if aid not in aids:
                out.append(
                    Violation(
                        "check_no_dangling_requirement_assumptions",
                        r.id,
                        f"assumption '{aid}' is not a known Assumption",
                    )
                )
    return out


def check_no_dangling_requirement_relations(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Requirement.relations[*] has a known kind and target.

    RULE: each Relation.kind MUST be in RELATION_KINDS, and each Relation.target
    MUST be in requirement_ids(g). An unknown kind or dangling target is an
    unresolvable edge.

    WHY: relation edges drive the dependency graph (R-dependency-graph-parallelism);
    a dangling or mis-typed edge makes the graph structurally incomplete.
    """
    rids = requirement_ids(g)
    out: list[Violation] = []
    for r in g.requirements:
        for rel in r.relations:
            if rel.kind not in RELATION_KINDS:
                out.append(
                    Violation(
                        "check_no_dangling_requirement_relations",
                        r.id,
                        f"relation kind '{rel.kind}' is not a known kind",
                    )
                )
            if rel.target not in rids:
                out.append(
                    Violation(
                        "check_no_dangling_requirement_relations",
                        r.id,
                        f"relation target '{rel.target}' is not a known Requirement",
                    )
                )
    return out


def check_no_dangling_conflict_refs(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Conflict's steward, members, shared_assumption, derived, and decided_by resolve.

    RULE: Conflict.steward MUST be in stakeholder_ids(g); each member MUST be in
    requirement_ids(g); shared_assumption (if set) MUST be in assumption_ids(g);
    each derived id MUST be in requirement_ids(g); decided_by (if set) MUST be in
    stakeholder_ids(g).

    WHY: a dangling member is how a conflict silently loses a party; a dangling
    assumption is how drift hides. Dangling refs on a Conflict are the cardinal
    invisibility the methodology forbids.
    """
    sids, aids, rids = stakeholder_ids(g), assumption_ids(g), requirement_ids(g)
    out: list[Violation] = []

    def _dangle(c_id: str, ref_name: str, ref_val: str, kind: str) -> Violation:
        return Violation(
            "check_no_dangling_conflict_refs",
            c_id,
            f"dangling Conflict ref — {ref_name} '{ref_val}' is not a known {kind}",
        )

    for c in g.conflicts:
        if c.steward not in sids:
            out.append(_dangle(c.id, "steward", c.steward, "Stakeholder"))
        for mid in c.members:
            if mid not in rids:
                out.append(_dangle(c.id, "member", mid, "Requirement"))
        if c.shared_assumption is not None and c.shared_assumption not in aids:
            out.append(
                _dangle(c.id, "shared_assumption", c.shared_assumption, "Assumption")
            )
        for did in c.derived:
            if did not in rids:
                out.append(_dangle(c.id, "derived", did, "Requirement"))
        if c.decided_by and c.decided_by not in sids:
            out.append(_dangle(c.id, "decided_by", c.decided_by, "Stakeholder"))
    return out


def check_no_dangling_operator_refs(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Operator.stakeholder and Operator.parent resolve.

    RULE: Operator.stakeholder MUST be in stakeholder_ids(g); Operator.parent
    (if set) MUST be in operator_ids(g). A dangling operator ref makes the
    delegation hierarchy structurally broken.

    WHY: the operator tree is the recursive delegation structure
    (R-operator-crystal-is-claude-md); a dangling parent or stakeholder collapses
    the tree invisibly.
    """
    sids = stakeholder_ids(g)
    oids = operator_ids(g)
    out: list[Violation] = []
    for op in g.operators:
        if op.stakeholder not in sids:
            out.append(
                Violation(
                    "check_no_dangling_operator_refs",
                    op.id,
                    f"operator stakeholder '{op.stakeholder}' is not a known Stakeholder",
                )
            )
        if op.parent is not None and op.parent not in oids:
            out.append(
                Violation(
                    "check_no_dangling_operator_refs",
                    op.id,
                    f"operator parent '{op.parent}' is not a known Operator",
                )
            )
    return out


def check_no_dangling_ids(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every id referenced by an edge resolves in the graph (thin delegator).

    RULE: Requirement.owner, Requirement.assumptions[*], Relation.target,
    Conflict.steward, Conflict.members[*], Conflict.shared_assumption,
    Conflict.derived[*], Assumption.owner, Operator.stakeholder, and
    Operator.parent MUST each name an object that exists.

    WHY first and broadest: a dangling member is how a conflict silently loses a
    party; a dangling assumption is how drift hides. A dangling edge is an
    invisible hole, the cardinal sin of the methodology.

    This is a THIN DELEGATOR — it calls the atomic sub-checks and concatenates
    their results. The atomic sub-checks are registered individually in ALL_INVARIANTS.
    """
    out: list[Violation] = []
    out.extend(check_no_dangling_assumption_owner(g))
    out.extend(check_no_dangling_requirement_owner(g))
    out.extend(check_no_dangling_requirement_assumptions(g))
    out.extend(check_no_dangling_requirement_relations(g))
    out.extend(check_no_dangling_conflict_refs(g))
    out.extend(check_no_dangling_operator_refs(g))
    return out


def check_doc_reader_resolves_to_stakeholder(g: TensionGraph) -> list[Violation]:
    """Canon: §Requirement — every generated doc's reader resolves to a known Stakeholder.

    RULE (aspect-gated): reads the active domain's explicit `DOC_READERS`
    binding (`manifest.py`, via `hotam_spec.graph.active_domain_doc_readers()`)
    — a `dict[role_hint, Stakeholder.id]`. For every doc kind declared in
    `hotam_spec.doc_readers.DOC_READER_ROLES`,
    `doc_readers.resolve_reader(kind, stakeholder_ids(g), bindings)` MUST
    return an id present in `stakeholder_ids(g)` — never `UNRESOLVED_READER`.
    A dangling reader (a generated doc naming a reader nobody is accountable
    for, OR a binding that names an id absent from this graph's Stakeholders)
    is the same invisibility the dangling-ref family already forbids for
    edges — this is that family applied to doc plumbing (R-doc-names-reader).
    No-ops when the active domain has declared NO `DOC_READERS` binding at
    all, OR when NONE of the declared bound ids appear in `g.stakeholders`
    (this graph is not the active domain's own graph — e.g. a synthetic
    test fixture or an unrelated demo domain checked in isolation; nothing
    here is meaningfully "this graph's" reader plumbing to validate) — a
    domain that has not opted into this aspect yet (e.g. a business domain
    whose manifest predates this convention) has not adopted it, mirroring
    the Process/Goal/Entity aspect-gating precedent (no-op when the aspect's
    substrate is empty).

    WHY an explicit declared binding, not a substring guess over stakeholder
    ids (R-doc-readers-declared-not-guessed): the prior design resolved a
    role hint (e.g. "operator") by scanning `stakeholder_ids(g)` for any id
    containing a hint substring like "agent" — a stakeholder id such as
    "travel-agent" in an unrelated business domain would silently capture
    operator-facing docs it has nothing to do with. An explicit
    `DOC_READERS` dict the domain author writes down removes the guess
    entirely; this invariant now validates that DECLARED mapping is
    well-formed (every bound id is a real Stakeholder), not that some
    substring happened to match.

    WHY graph-scoped, not filesystem-scoped: the reader ROLE VOCABULARY is
    framework code (doc_readers.DOC_READER_ROLES); what varies per domain is
    the BINDING from role to Stakeholder id, declared in that domain's own
    manifest.py. Passing the already-loaded `g` (mirrors
    check_axis_in_registry) keeps this check pure and avoids re-importing a
    domain graph from disk, unlike the filesystem-coherence checks
    (check_entities_md_lists_all_types) that must walk domains/ because they
    validate committed generated files against EVERY domain, not just the
    active one.
    """
    from hotam_spec.doc_readers import (  # noqa: PLC0415
        DOC_READER_ROLES,
        UNRESOLVED_READER,
        resolve_reader,
    )
    from hotam_spec.graph import active_domain_doc_readers  # noqa: PLC0415

    if not g.stakeholders:
        return []
    bindings = active_domain_doc_readers()
    if not bindings:
        return []  # aspect not adopted by this domain — legitimate no-op
    sids = stakeholder_ids(g)
    if not any(bound_id in sids for bound_id in bindings.values()):
        return []  # `g` is not the active domain's own graph — legitimate no-op
    out: list[Violation] = []
    for doc_kind in sorted(DOC_READER_ROLES):
        resolved = resolve_reader(doc_kind, sids, bindings)
        if resolved == UNRESOLVED_READER:
            role = DOC_READER_ROLES[doc_kind]
            bound_id = bindings.get(role)
            if bound_id is None:
                detail = (
                    f"doc kind '{doc_kind}' has role '{role}' but the active "
                    "domain's manifest.py DOC_READERS declares no binding for "
                    "it — add one, e.g. DOC_READERS = {...'"
                    f"{role}': '<Stakeholder.id>'...}}"
                )
            else:
                detail = (
                    f"doc kind '{doc_kind}' has role '{role}' bound to "
                    f"'{bound_id}' in manifest.py DOC_READERS, but no "
                    "Stakeholder with that id exists in this graph — fix the "
                    "binding or add the Stakeholder"
                )
            out.append(
                Violation("check_doc_reader_resolves_to_stakeholder", doc_kind, detail)
            )
    return out


# ---------------------------------------------------------------------------
# 2. A conflict is a CONNECTOR — axis, context, steward all present (atomized)
# ---------------------------------------------------------------------------


def check_conflict_has_axis(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — every Conflict carries a non-empty axis.

    RULE: Conflict.axis MUST be a non-empty string. An axis-less conflict is not
    a connector node — it does not name the tension dimension it mediates.

    WHY: the axis is what makes conflicts cluster into architectural choices;
    an axis-less conflict is invisible in any cluster view.
    """
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.axis.strip():
            out.append(
                Violation(
                    "check_conflict_has_axis",
                    c.id,
                    "conflict has no tension axis (along WHAT do they diverge?)",
                )
            )
    return out


def check_conflict_has_context(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — every Conflict carries a non-empty context.

    RULE: Conflict.context MUST be a non-empty string describing the scenario
    where the two requirements collide. A context-less conflict has no scenario
    and cannot be communicated to a steward.

    WHY: without a context the conflict cannot be communicated to a steward or
    a domain user in a way that enables resolution.
    """
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.context.strip():
            out.append(
                Violation(
                    "check_conflict_has_context",
                    c.id,
                    "conflict has no context (in WHICH scenario do they collide?)",
                )
            )
    return out


def check_conflict_has_steward(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — every Conflict carries a non-empty steward.

    RULE: Conflict.steward MUST be a non-empty string. A stewardless conflict
    has no holder — the tension is invisible to the methodology.

    WHY: this is the structural definition of "the contradiction is visible". A
    stewardless conflict is exactly an invisible contradiction — the hard
    boundary (R-ai-presents-not-decides) requires a named outside party.
    """
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.steward.strip():
            out.append(
                Violation(
                    "check_conflict_has_steward",
                    c.id,
                    "conflict has no steward (WHO holds this tension?)",
                )
            )
    return out


def check_conflict_has_axis_context_steward(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — every Conflict carries a non-empty axis, context, steward (thin delegator).

    RULE: axis, context and steward MUST all be non-empty. These three are the
    knowledge that belongs to neither member; a conflict missing any of them is
    not a connector node, it is the empty `conflicts_with` edge we reject.

    WHY: this is the structural definition of "the contradiction is visible". An
    axis-less or stewardless conflict is exactly an invisible contradiction.

    This is a THIN DELEGATOR — calls check_conflict_has_axis,
    check_conflict_has_context, check_conflict_has_steward and concatenates.
    The atomic sub-checks are registered individually in ALL_INVARIANTS.
    """
    out: list[Violation] = []
    out.extend(check_conflict_has_axis(g))
    out.extend(check_conflict_has_context(g))
    out.extend(check_conflict_has_steward(g))
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


#: The self-host operator-prompt is constituted by the domain graph that
#: DEFINES the convergence rule itself. A graph containing this atom IS the
#: self-host graph; any other domain graph is a business domain whose DETECTED
#: conflicts between SETTLED atoms are normal held tensions, not incoherence.
_CONSTITUTING_CONVERGENCE_ATOM = "R-constituting-requirements-converge"


def check_constituting_not_in_unresolved_conflict(g: TensionGraph) -> list[Violation]:
    """Canon: §Requirement — no two SETTLED constituting atoms sit in an unresolved conflict.

    RULE: in the self-host graph (the one composing the operator-prompt — i.e.
    the graph that DEFINES R-constituting-requirements-converge), no unresolved
    Conflict (DETECTED / ACKNOWLEDGED) may hold two SETTLED Requirements as
    members. This is the machine-checkable face of "the set of SETTLED
    requirements composing the operator-prompt shall be pairwise consistent"
    (R-constituting-requirements-converge): a DETECTED conflict between two
    SETTLED atoms means the CONSTITUTION block presents both as settled truth
    while the graph itself records them as an open, unstewarded contradiction.

    WHY scoped to the self-host graph (FRAMEWORK_SCOPED, gated on
    g.self_hosting in all_violations, not per-graph): a business domain's
    DETECTED conflict with SETTLED members is NORMAL life — the tension has
    been found and is awaiting its steward, which is exactly what the
    methodology is for (holding contradictions open as Conflict nodes). Those
    atoms do NOT compose the operator-prompt, so the pairwise-consistency
    demand does not apply to them; firing there would forbid the healthy
    held-tension state (e.g. hotam-dev C-ec1ec532). The rule binds only to the
    atoms that REALLY constitute the operator-prompt: the self-host
    constitution index.

    WHY gated on g.self_hosting (not the presence of the rule's own anchor
    atom): the discriminator must be the same self-host signal every other
    FRAMEWORK_SCOPED invariant uses (R-domain-self-hosting-flag), not a magic
    atom-id string — probing for _CONSTITUTING_CONVERGENCE_ATOM would silently
    go dark the day that atom is renamed/rekeyed, whereas the manifest flag
    travels with the graph. The atom-presence check is kept only as a
    defensive no-op for the (test-only) case of loading this function against a
    graph that carries self_hosting but not its constituting atom.
    """
    settled_ids = {r.id for r in g.requirements if r.status == SETTLED}
    if not g.self_hosting or _CONSTITUTING_CONVERGENCE_ATOM not in settled_ids:
        # Not the self-host constituting graph — the demand does not apply.
        return []
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.is_unresolved():
            continue
        settled_members = sorted(m for m in set(c.members) if m in settled_ids)
        if len(settled_members) >= 2:
            out.append(
                Violation(
                    "check_constituting_not_in_unresolved_conflict",
                    c.id,
                    f"conflict '{c.id}' ({c.lifecycle}) holds >= 2 SETTLED "
                    f"constituting atoms ({', '.join(settled_members)}) as an "
                    f"UNRESOLVED contradiction while the CONSTITUTION presents "
                    f"them as settled truth — steward must resolve it (DECIDED / "
                    f"REVISIT_WHEN) or the members must not both be SETTLED "
                    f"(R-constituting-requirements-converge).",
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
    owner_of = _requirement_owner_map(g)
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
# 5b. Signoff lock — a DECIDED conflict must name its human decider (atomized)
# ---------------------------------------------------------------------------


def check_decided_has_nonempty_decided_by(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — a DECIDED conflict carries a non-empty decided_by field.

    RULE: when Conflict.lifecycle starts with "DECIDED", `decided_by` MUST be
    non-empty. A DECIDED conflict without a named human decider is an
    AI-silently-closeable hole — exactly the invisibility the hard boundary forbids.

    WHY: R-decided-needs-human-signoff makes the closed loop's ACT half structurally
    visible. Without this lock, an AI could write lifecycle="DECIDED(...)" with
    decided_by="" and pass all other invariants.
    """
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.is_decided():
            continue
        if not c.decided_by:
            out.append(
                Violation(
                    "check_decided_has_nonempty_decided_by",
                    c.id,
                    "DECIDED conflict must carry a non-empty decided_by "
                    "(the Stakeholder.id of the human who approved the resolution; "
                    "R-decided-needs-human-signoff)",
                )
            )
    return out


def check_decided_by_is_known_stakeholder(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — a DECIDED conflict's decided_by resolves to a known Stakeholder.

    RULE: when Conflict.lifecycle starts with "DECIDED" and decided_by is non-empty,
    decided_by MUST be in stakeholder_ids(g). An unresolvable decider is a dangling
    reference that cannot be audited.

    WHY: check_no_dangling_conflict_refs also catches this, but naming it explicitly
    in the harness makes the missing signoff traceable to the decision moment.
    """
    sids = stakeholder_ids(g)
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.is_decided():
            continue
        if not c.decided_by:
            continue  # caught by check_decided_has_nonempty_decided_by
        if c.decided_by not in sids:
            out.append(
                Violation(
                    "check_decided_by_is_known_stakeholder",
                    c.id,
                    f"decided_by '{c.decided_by}' is not a known Stakeholder",
                )
            )
    return out


def check_decided_by_not_member_owner(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — a DECIDED conflict's decided_by is not the owner of any member Requirement.

    RULE: when Conflict.lifecycle starts with "DECIDED", decided_by MUST NOT be
    the owner of any of the conflict's member Requirements. The decider must be
    outside the conflict's members (steward-distinct rule applied to the decider).

    WHY: if the decider owned one of the members, the hard boundary would be
    circumvented at the decision step. This is the structural twin of
    check_steward_not_a_member_owner applied at the moment of resolution.
    """
    owner_of = _requirement_owner_map(g)
    sids = stakeholder_ids(g)
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.is_decided():
            continue
        if not c.decided_by or c.decided_by not in sids:
            continue  # caught by prior atomic checks
        member_owners = {owner_of[m] for m in c.members if m in owner_of}
        if c.decided_by in member_owners:
            out.append(
                Violation(
                    "check_decided_by_not_member_owner",
                    c.id,
                    f"decided_by '{c.decided_by}' also owns a member requirement; "
                    f"the decider must be outside the conflict's members "
                    f"(steward-distinct rule applied to the decider)",
                )
            )
    return out


def check_held_has_min_two_variants(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — a HELD conflict carries at least two elaborated Variants.

    RULE: when Conflict.lifecycle starts with "HELD", `variants` MUST contain
    at least two distinct Variant ids. A HELD tension with fewer than two
    variants gives the steward nothing to choose between -- exactly the
    invisible-contradiction-in-a-new-costume the hard boundary forbids.

    WHY: mirrors check_conflict_min_two_members -- a HELD conflict connects
    at least two SIDES of a live tension the same way a Conflict connects at
    least two member requirements.
    """
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.is_held():
            continue
        if len({v.id for v in c.variants}) < 2:
            out.append(
                Violation(
                    "check_held_has_min_two_variants",
                    c.id,
                    "HELD conflict must carry >= 2 distinct Variant ids "
                    "(the steward needs at least two sides to choose between)",
                )
            )
    return out


def check_held_has_nonempty_decided_by(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — a HELD conflict carries a non-empty decided_by field.

    RULE: when Conflict.lifecycle starts with "HELD", `decided_by` MUST be
    non-empty. Entering HELD is a human act (R-decided-needs-human-signoff's
    signoff lock applied at the moment a tension is classified unresolvable
    by its members) -- without this lock an AI could silently write
    lifecycle="HELD(...)" with decided_by="".

    WHY: the structural twin of check_decided_has_nonempty_decided_by applied
    to HELD instead of DECIDED.
    """
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.is_held():
            continue
        if not c.decided_by:
            out.append(
                Violation(
                    "check_held_has_nonempty_decided_by",
                    c.id,
                    "HELD conflict must carry a non-empty decided_by "
                    "(the Stakeholder.id of the human who classified this "
                    "tension unresolvable-by-members; R-decided-needs-human-signoff)",
                )
            )
    return out


def check_held_by_is_known_stakeholder(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — a HELD conflict's decided_by resolves to a known Stakeholder.

    RULE: when Conflict.lifecycle starts with "HELD" and decided_by is
    non-empty, decided_by MUST be in stakeholder_ids(g).

    WHY: mirrors check_decided_by_is_known_stakeholder applied to HELD.
    """
    sids = stakeholder_ids(g)
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.is_held():
            continue
        if not c.decided_by:
            continue  # caught by check_held_has_nonempty_decided_by
        if c.decided_by not in sids:
            out.append(
                Violation(
                    "check_held_by_is_known_stakeholder",
                    c.id,
                    f"decided_by '{c.decided_by}' is not a known Stakeholder",
                )
            )
    return out


def check_held_by_not_member_owner(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — a HELD conflict's decided_by is not the owner of any member Requirement.

    RULE: when Conflict.lifecycle starts with "HELD", decided_by MUST NOT be
    the owner of any of the conflict's member Requirements -- the
    steward-distinct rule applied to the human who holds the tension open.

    WHY: mirrors check_decided_by_not_member_owner applied to HELD; if the
    signoff owned a member, the hard boundary would be circumvented at the
    hold step exactly as it would at the decide step.
    """
    owner_of = _requirement_owner_map(g)
    sids = stakeholder_ids(g)
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.is_held():
            continue
        if not c.decided_by or c.decided_by not in sids:
            continue  # caught by prior atomic checks
        member_owners = {owner_of[m] for m in c.members if m in owner_of}
        if c.decided_by in member_owners:
            out.append(
                Violation(
                    "check_held_by_not_member_owner",
                    c.id,
                    f"decided_by '{c.decided_by}' also owns a member requirement; "
                    f"the human who holds this tension open must be outside the "
                    f"conflict's members (steward-distinct rule applied to HELD)",
                )
            )
    return out


def check_held_has_decided_by(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — a HELD conflict names a human decider outside its members (thin delegator).

    RULE (R-decided-needs-human-signoff applied to HELD): when
    Conflict.lifecycle starts with "HELD", `decided_by` MUST satisfy three
    conditions: (1) non-empty, (2) resolves to a known Stakeholder id, (3)
    NOT the owner of any of the conflict's member Requirements.

    This is a THIN DELEGATOR — calls check_held_has_nonempty_decided_by,
    check_held_by_is_known_stakeholder, check_held_by_not_member_owner and
    concatenates. The atomic sub-checks are registered individually in
    ALL_INVARIANTS.
    """
    out: list[Violation] = []
    out.extend(check_held_has_nonempty_decided_by(g))
    out.extend(check_held_by_is_known_stakeholder(g))
    out.extend(check_held_by_not_member_owner(g))
    return out


def check_typed_anchors_variant(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Variant.id (on every Conflict) starts with 'V-'.

    RULE: for each Conflict, every Variant in its `variants` tuple MUST have
    an id starting with 'V-'. An id with the wrong prefix breaks the
    typed-anchor discipline (R-anchor-everything) for the new Variant payload
    type introduced alongside HELD.

    WHY: Variant is not a graph node (anti-RDF, payload on Conflict), but it
    IS a typed anchor a steward or agent may cite by reference
    (R-speak-by-reference) -- the same discipline that governs R-/C-/A-/OP-
    ids applies to V- ids.
    """
    out: list[Violation] = []
    for c in g.conflicts:
        for v in c.variants:
            if not v.id.startswith("V-"):
                out.append(
                    Violation(
                        "check_typed_anchors_variant",
                        f"{c.id}:{v.id}",
                        f"Variant id '{v.id}' on conflict '{c.id}' must start "
                        f"with 'V-' (typed-anchor rule, R-anchor-everything)",
                    )
                )
    return out


def check_decided_has_decided_by(g: TensionGraph) -> list[Violation]:
    """Canon: §Conflict — a DECIDED conflict names a human decider outside its members (thin delegator).

    RULE (R-decided-needs-human-signoff + §Proposal): when Conflict.lifecycle
    starts with "DECIDED", `decided_by` MUST satisfy three conditions:
      1. Non-empty.
      2. Resolves to a known Stakeholder id.
      3. NOT the owner of any of the conflict's member Requirements.

    This is a THIN DELEGATOR — calls check_decided_has_nonempty_decided_by,
    check_decided_by_is_known_stakeholder, check_decided_by_not_member_owner
    and concatenates. The atomic sub-checks are registered individually in ALL_INVARIANTS.

    WHY: R-decided-needs-human-signoff makes the closed loop's ACT half structurally
    visible (§Proposal — the closed loop's ACT half).
    """
    out: list[Violation] = []
    out.extend(check_decided_has_nonempty_decided_by(g))
    out.extend(check_decided_by_is_known_stakeholder(g))
    out.extend(check_decided_by_not_member_owner(g))
    return out


# ---------------------------------------------------------------------------
# 6. Typed-anchor prefixes — every id carries the right kind prefix (atomized)
# ---------------------------------------------------------------------------


def check_typed_anchors_requirement(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Requirement.id starts with 'R-'.

    RULE: Requirement.id MUST start with 'R-'. An id with the wrong prefix
    breaks the typed-anchor discipline (R-anchor-everything) and makes
    cite-by-reference unreliable (R-speak-by-reference).

    References: R-anchor-everything (DRAFT), R-anchor-taxonomy (OPEN/M28).
    """
    out: list[Violation] = []
    for r in g.requirements:
        if not r.id.startswith("R-"):
            out.append(
                Violation(
                    "check_typed_anchors_requirement",
                    r.id,
                    f"Requirement id '{r.id}' must start with 'R-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    return out


def check_typed_anchors_assumption(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Assumption.id starts with 'A-'.

    RULE: Assumption.id MUST start with 'A-'. An id with the wrong prefix
    breaks the typed-anchor discipline (R-anchor-everything).

    References: R-anchor-everything (DRAFT), R-anchor-taxonomy (OPEN/M28).
    """
    out: list[Violation] = []
    for a in g.assumptions:
        if not a.id.startswith("A-"):
            out.append(
                Violation(
                    "check_typed_anchors_assumption",
                    a.id,
                    f"Assumption id '{a.id}' must start with 'A-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    return out


def check_typed_anchors_conflict(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Conflict.id starts with 'C-'.

    RULE: Conflict.id MUST start with 'C-'. An id with the wrong prefix
    breaks the typed-anchor discipline (R-anchor-everything).

    References: R-anchor-everything (DRAFT), R-anchor-taxonomy (OPEN/M28).
    """
    out: list[Violation] = []
    for c in g.conflicts:
        if not c.id.startswith("C-"):
            out.append(
                Violation(
                    "check_typed_anchors_conflict",
                    c.id,
                    f"Conflict id '{c.id}' must start with 'C-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    return out


def check_typed_anchors_operator(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Operator.id starts with 'OP-'.

    RULE: Operator.id MUST start with 'OP-'. An id with the wrong prefix
    breaks the typed-anchor discipline (R-anchor-everything).

    References: R-anchor-everything (DRAFT), R-anchor-taxonomy (OPEN/M28).
    """
    out: list[Violation] = []
    for op in g.operators:
        if not op.id.startswith("OP-"):
            out.append(
                Violation(
                    "check_typed_anchors_operator",
                    op.id,
                    f"Operator id '{op.id}' must start with 'OP-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    return out


def check_typed_anchors_process(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Process.id starts with 'PR-'.

    RULE: Process.id MUST start with 'PR-'. An id with the wrong prefix
    breaks the typed-anchor discipline (R-anchor-everything).

    References: R-anchor-everything (DRAFT), R-anchor-taxonomy (OPEN/M28).
    """
    out: list[Violation] = []
    for p in g.processes:
        if not p.id.startswith("PR-"):
            out.append(
                Violation(
                    "check_typed_anchors_process",
                    p.id,
                    f"Process id '{p.id}' must start with 'PR-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    return out


def check_typed_anchors_goal(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every Goal.id starts with 'GOAL-'.

    RULE: Goal.id MUST start with 'GOAL-'. An id with the wrong prefix
    breaks the typed-anchor discipline (R-anchor-everything).

    References: R-anchor-everything (DRAFT), R-anchor-taxonomy (OPEN/M28).
    """
    out: list[Violation] = []
    for go in g.goals:
        if not go.id.startswith("GOAL-"):
            out.append(
                Violation(
                    "check_typed_anchors_goal",
                    go.id,
                    f"Goal id '{go.id}' must start with 'GOAL-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    return out


def check_typed_anchors(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants — every id carries the prefix that matches its kind (thin delegator).

    RULE: Requirement.id MUST start with 'R-'; Assumption.id MUST start with
    'A-'; Conflict.id MUST start with 'C-'; Operator.id MUST start with 'OP-'.
    An id with a wrong or missing prefix breaks the typed-anchor discipline
    (R-anchor-everything) and makes cite-by-reference unreliable
    (R-speak-by-reference).

    WHY minimal: this check enforces the CURRENTLY USED prefixes (R-/A-/C-/OP-)
    that are already discipline in the codebase; it does NOT yet encode the full
    M28 taxonomy (GOAL-/GAP-/DLG-/AX-) — those are still OPEN per
    R-anchor-taxonomy.

    This is a THIN DELEGATOR — calls the atomic per-entity-type sub-checks and
    concatenates. The atomic sub-checks are registered individually in ALL_INVARIANTS.

    References: R-anchor-everything (DRAFT), R-anchor-taxonomy (OPEN/M28).
    """
    out: list[Violation] = []
    out.extend(check_typed_anchors_requirement(g))
    out.extend(check_typed_anchors_assumption(g))
    out.extend(check_typed_anchors_conflict(g))
    out.extend(check_typed_anchors_operator(g))
    out.extend(check_typed_anchors_process(g))
    out.extend(check_typed_anchors_goal(g))
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


_TESTS_DIR_FOR_ENFORCER_CHECK = Path(__file__).resolve().parents[2] / "tests"


def check_enforced_by_resolvable(g: TensionGraph) -> list[Violation]:
    """Canon: §Requirement / §Closure — every ENFORCED requirement's enforced_by entry must resolve to a real check_* function or a concrete pytest node-id.

    RULE: every enforced_by entry of a SETTLED/ENFORCED Requirement must resolve to a real check_* function or a concrete pytest node-id, else it fires a Violation naming the entry that does not resolve.

    Concretely, an entry resolves via one of:
      1. a function name present in ALL_INVARIANTS (a real check_* the
         framework runs), OR
      2. a real test path via hotam_spec.enforcer_resolution's rules
         (verbatim "file.py"/"file.py::func", or a check_* name a test file
         references by bare name, or an unambiguous bare test_* function).
    A typo like 'test_gone.py::test_x', a stale/renamed check_* name, or any
    other string matching neither path fires a Violation naming the exact
    entry that failed.

    WHY existence-in-ALL_INVARIANTS is checked FIRST (not folded into the
    grep-only resolver used by gate.py's T1 selector): "does this enforcer
    exist" (this invariant's question) and "is there a test file I can run
    to targeted-verify it" (gate.py's T1 question) are different questions.
    Many real check_* functions in ALL_INVARIANTS are exercised only
    indirectly — via `all_violations(g)` / ALL_INVARIANTS iteration in a
    fixture-driven test, never called by bare name in test source — so
    gate.py's stricter grep-based resolution correctly fails closed to T2
    for THOSE (there genuinely is no narrower targeted test to run), while
    THIS invariant correctly says the enforcer itself is real, not a typo.
    Conflating the two would either make T1 select nothing meaningful, or
    make this invariant noisy with ~46 false positives against real,
    functioning check_* enforcers — that noise was observed when this
    invariant was first landed and is why the two-path resolution exists.

    WHY a NEW invariant (not folded into check_enforced_names_invariant):
    that check only verifies enforced_by is NON-EMPTY — a real typo (a
    renamed test file, a genuinely nonexistent check_*/test_* name) passes
    it silently. This invariant makes that debt visible directly, instead of
    only being discovered when a human runs tools/gate.py by hand.

    WHY reuse hotam_spec.enforcer_resolution (not gate.py's own copy):
    invariants must not import from tools/ (that dependency direction is
    reversed elsewhere in the codebase — tools import src, never the other
    way); the resolution algorithm was extracted into
    src/hotam_spec/enforcer_resolution.py precisely so both gate.py (a tool)
    and this invariant (in src/) can share the grep-based half of the
    algorithm (R-prefer-tool-over-hand).

    WHY filesystem-grep-based and kept fast: this check greps every
    tests/test_*.py file to build the check_* -> test-file map. That grep is
    O(number of test files) and runs once per check() call, in-process — no
    subprocess spawn. It has been measured at well under a second on this
    repo's test suite (~180+ test files) and is safe to run on every
    diagnose()/what_now() pass. If the test suite grows an order of
    magnitude, this may need caching; that is not yet the case, so no cache
    is added preemptively.
    """
    out: list[Violation] = []
    check_to_tests = _enforcer_check_to_tests_map(_TESTS_DIR_FOR_ENFORCER_CHECK)
    known_check_fn_names = {fn.__name__ for fn in ALL_INVARIANTS}
    for r in g.requirements:
        # Mirrors check_bijection_r_to_enforcer's scope: only SETTLED+ENFORCED
        # requirements make a live claim that an enforcer resolves; a REJECTED
        # requirement's enforced_by is historical record, not a live guarantee
        # (R-rejected-preserved-not-deleted keeps the text, but it is no
        # longer an active claim this invariant should hold to account).
        if r.status != "SETTLED" or r.enforcement != ENFORCED:
            continue
        for entry in r.enforced_by:
            stripped = entry.strip()
            if stripped.startswith("check_") and stripped in known_check_fn_names:
                continue
            resolved = _enforcer_resolve_one(
                entry, check_to_tests, _TESTS_DIR_FOR_ENFORCER_CHECK
            )
            if resolved is None:
                out.append(
                    Violation(
                        "check_enforced_by_resolvable",
                        r.id,
                        f"enforced_by entry {entry!r} does not resolve to a "
                        "real check_* function or a concrete pytest node-id "
                        "(no such test file, unknown check_* name, or "
                        "ambiguous/missing bare test_* function) — fix the "
                        "name or the enforced_by tuple",
                    )
                )
    return out


def check_enforceability_kind_known(g: TensionGraph) -> list[Violation]:
    """Canon: §Requirement / §Invariants — every Requirement.enforceability is a known kind.

    RULE: enforceability MUST be in ENFORCEABILITY_KINDS (ENFORCEABLE |
    INHERENTLY_PROSE). An invalid value corrupts the enforcement-gradient
    debt calculation.

    WHY: the enforceability kind is what lets the P0 REFLECTION debt count
    distinguish real closeable debt (ENFORCEABLE, no enforcer yet) from
    permanent discipline (INHERENTLY_PROSE, never checkable by nature). An
    unknown value would silently misclassify a requirement in that count.
    """
    out: list[Violation] = []
    for r in g.requirements:
        if r.enforceability not in ENFORCEABILITY_KINDS:
            out.append(
                Violation(
                    "check_enforceability_kind_known",
                    r.id,
                    f"enforceability '{r.enforceability}' is not in "
                    f"ENFORCEABILITY_KINDS (ENFORCEABLE | INHERENTLY_PROSE)",
                )
            )
    return out


# ---------------------------------------------------------------------------
# 8. M-tag format, uniqueness, and OPEN-only discipline (atomized)
# ---------------------------------------------------------------------------


def check_m_tag_valid_format(g: TensionGraph) -> list[Violation]:
    """Canon: §Requirement — every non-empty m_tag matches ^M[1-9][0-9]*$.

    RULE: a non-empty `m_tag` MUST match `^M[1-9][0-9]*$` — "M" followed by a
    positive integer with no leading zeros (e.g. M3, M17, M26; not M01, m17, M,
    Mfoo). This is the typed-anchor discipline applied to M-tags.

    WHY: invalid format breaks M-registry parsing; the format is the typed-anchor
    convention for methodology decisions (R-drift-structurally-impossible / U5).
    """
    out: list[Violation] = []
    for r in g.requirements:
        if not r.m_tag:
            continue
        if not _M_TAG_RE.match(r.m_tag):
            out.append(
                Violation(
                    "check_m_tag_valid_format",
                    r.id,
                    f"m_tag '{r.m_tag}' does not match ^M[1-9][0-9]*$ "
                    f"(must be 'M' followed by a positive integer, no leading zeros)",
                )
            )
    return out


def check_m_tag_unique(g: TensionGraph) -> list[Violation]:
    """Canon: §Requirement — no two Requirements share the same m_tag.

    RULE: each non-empty `m_tag` MUST appear on at most one Requirement in the
    graph. A duplicate tag breaks the bijection that `docs/gen/DECISIONS.md` relies
    on: one M-decision maps to exactly one Requirement.

    WHY: duplicates break the one-to-one mapping between an M-entry and its
    Requirement (R-drift-structurally-impossible applied to the M-registry).
    """
    out: list[Violation] = []
    seen_tags: dict[str, str] = {}
    for r in g.requirements:
        if not r.m_tag:
            continue
        if r.m_tag in seen_tags:
            out.append(
                Violation(
                    "check_m_tag_unique",
                    r.id,
                    f"m_tag '{r.m_tag}' is already used by '{seen_tags[r.m_tag]}'; "
                    f"each M-tag must be unique across the graph",
                )
            )
        else:
            seen_tags[r.m_tag] = r.id
    return out


def check_m_tag_open_only(g: TensionGraph) -> list[Violation]:
    """Canon: §Requirement — an m_tag appears only on an OPEN requirement.

    RULE: an `m_tag` MUST only appear on a Requirement with status starting with
    OPEN. An M-tag on a SETTLED, DRAFT, or REJECTED requirement would pollute the
    M-registry with decisions that are no longer open.

    WHY: the M-registry tracks live open decisions; a non-OPEN m_tag makes the
    registry structurally incorrect (R-drift-structurally-impossible / U5).
    """
    out: list[Violation] = []
    for r in g.requirements:
        if not r.m_tag:
            continue
        if not r.status.startswith(OPEN_PREFIX):
            out.append(
                Violation(
                    "check_m_tag_open_only",
                    r.id,
                    f"m_tag '{r.m_tag}' appears on a non-OPEN requirement (status={r.status!r}); "
                    f"M-tags are only for OPEN requirements (the live M-decision registry)",
                )
            )
    return out


def check_m_tag_format(g: TensionGraph) -> list[Violation]:
    """Canon: §Requirement — every non-empty m_tag is valid, unique, and OPEN-only (thin delegator).

    RULE (three sub-rules):
      1. FORMAT: a non-empty `m_tag` MUST match `^M[1-9][0-9]*$`.
      2. UNIQUE: no two Requirements may share the same `m_tag`.
      3. OPEN-ONLY: an `m_tag` MUST only appear on an OPEN requirement.

    WHY: the M-tag field is the bridge between the graph and `docs/gen/DECISIONS.md`
    (the generated canonical M-registry). Invalid format breaks parsing; duplicates
    break the one-to-one mapping; non-OPEN tags pollute the registry.

    This is a THIN DELEGATOR — calls check_m_tag_valid_format, check_m_tag_unique,
    check_m_tag_open_only and concatenates. The atomic sub-checks are registered
    individually in ALL_INVARIANTS.
    """
    out: list[Violation] = []
    out.extend(check_m_tag_valid_format(g))
    out.extend(check_m_tag_unique(g))
    out.extend(check_m_tag_open_only(g))
    return out


# ---------------------------------------------------------------------------
# 9. Lifecycle well-formedness helper + Lifecycle-status validators (atomized)
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


def check_requirement_status_in_lifecycle(g: TensionGraph) -> list[Violation]:
    """Canon: §Lifecycle / §Invariants — every Requirement.status matches REQUIREMENT_STATUS_LIFECYCLE.

    RULE: Requirement.status MUST be matched by REQUIREMENT_STATUS_LIFECYCLE
    (exact match for DRAFT/SETTLED/REJECTED; prefix match for OPEN(question)).
    When matches() returns None, fire a Violation.

    WHY structural: status is a hand-rolled string state machine; this invariant
    enforces that stored values belong to the canonical set. References:
    R-lifecycle-abstraction, R-statemachine-wellformedness.
    """
    out: list[Violation] = []
    for r in g.requirements:
        if REQUIREMENT_STATUS_LIFECYCLE.matches(r.status) is None:
            out.append(
                Violation(
                    "check_requirement_status_in_lifecycle",
                    r.id,
                    f"Requirement.status '{r.status}' is not a valid state in "
                    f"lifecycle '{REQUIREMENT_STATUS_LIFECYCLE.slug}' "
                    f"(valid: {sorted(REQUIREMENT_STATUS_LIFECYCLE.state_names())})",
                )
            )
    return out


def check_conflict_lifecycle_in_lifecycle(g: TensionGraph) -> list[Violation]:
    """Canon: §Lifecycle / §Invariants — every Conflict.lifecycle matches CONFLICT_LIFECYCLE.

    RULE: Conflict.lifecycle MUST be matched by CONFLICT_LIFECYCLE (exact match
    for DETECTED/ACKNOWLEDGED; prefix match for DECIDED(rationale) and
    REVISIT_WHEN(condition)). When matches() returns None, fire a Violation.

    WHY structural: conflict lifecycle is a hand-rolled string state machine;
    enforcing canonical values makes the machine structurally visible and checkable.
    References: R-lifecycle-abstraction, R-statemachine-wellformedness.
    """
    out: list[Violation] = []
    for c in g.conflicts:
        if CONFLICT_LIFECYCLE.matches(c.lifecycle) is None:
            out.append(
                Violation(
                    "check_conflict_lifecycle_in_lifecycle",
                    c.id,
                    f"Conflict.lifecycle '{c.lifecycle}' is not a valid state in "
                    f"lifecycle '{CONFLICT_LIFECYCLE.slug}' "
                    f"(valid: {sorted(CONFLICT_LIFECYCLE.state_names())})",
                )
            )
    return out


def check_operator_lifecycle_in_lifecycle(g: TensionGraph) -> list[Violation]:
    """Canon: §Lifecycle / §Invariants — every Operator.lifecycle matches OPERATOR_LIFECYCLE.

    RULE: Operator.lifecycle MUST be matched by OPERATOR_LIFECYCLE (exact match
    for ACTIVE/SATURATED/DELEGATED/RETIRED). When matches() returns None, fire a
    Violation.

    WHY structural: operator lifecycle is a hand-rolled string state machine;
    enforcing canonical values makes the machine structurally visible and checkable.
    References: R-lifecycle-abstraction, R-statemachine-wellformedness.
    """
    out: list[Violation] = []
    for op in g.operators:
        if OPERATOR_LIFECYCLE.matches(op.lifecycle) is None:
            out.append(
                Violation(
                    "check_operator_lifecycle_in_lifecycle",
                    op.id,
                    f"Operator.lifecycle '{op.lifecycle}' is not a valid state in "
                    f"lifecycle '{OPERATOR_LIFECYCLE.slug}' "
                    f"(valid: {sorted(OPERATOR_LIFECYCLE.state_names())})",
                )
            )
    return out


def check_goal_lifecycle_in_lifecycle(g: TensionGraph) -> list[Violation]:
    """Canon: §Lifecycle / §Invariants — every Goal.lifecycle matches GOAL_LIFECYCLE.

    RULE: Goal.lifecycle MUST be matched by GOAL_LIFECYCLE. When matches() returns
    None, fire a Violation.

    WHY structural: goal lifecycle is a hand-rolled string state machine; enforcing
    canonical values makes the machine structurally visible and checkable.
    References: R-lifecycle-abstraction, R-statemachine-wellformedness.
    """
    out: list[Violation] = []
    for go in g.goals:
        if GOAL_LIFECYCLE.matches(go.lifecycle) is None:
            out.append(
                Violation(
                    "check_goal_lifecycle_in_lifecycle",
                    go.id,
                    f"Goal.lifecycle '{go.lifecycle}' is not a valid state in "
                    f"lifecycle '{GOAL_LIFECYCLE.slug}' "
                    f"(valid: {sorted(GOAL_LIFECYCLE.state_names())})",
                )
            )
    return out


def check_status_in_lifecycle(g: TensionGraph) -> list[Violation]:
    """Canon: §Lifecycle / §Invariants — every status/lifecycle value matches a canonical Lifecycle (thin delegator).

    RULE (four sub-rules):
      1. Every Requirement.status MUST be matched by REQUIREMENT_STATUS_LIFECYCLE.
      2. Every Conflict.lifecycle MUST be matched by CONFLICT_LIFECYCLE.
      3. Every Operator.lifecycle MUST be matched by OPERATOR_LIFECYCLE.
      4. Every Goal.lifecycle MUST be matched by GOAL_LIFECYCLE.

    WHY structural: status and lifecycle are hand-rolled string state machines;
    this invariant enforces canonical values. References: R-lifecycle-abstraction,
    R-statemachine-wellformedness.

    This is a THIN DELEGATOR — calls the four atomic per-entity sub-checks and
    concatenates. The atomic sub-checks are registered individually in ALL_INVARIANTS.
    """
    out: list[Violation] = []
    out.extend(check_requirement_status_in_lifecycle(g))
    out.extend(check_conflict_lifecycle_in_lifecycle(g))
    out.extend(check_operator_lifecycle_in_lifecycle(g))
    out.extend(check_goal_lifecycle_in_lifecycle(g))
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
    from hotam_spec.process import GOAL_LIFECYCLE as GL  # noqa: PLC0415
    from hotam_spec.process import PROCESS_LIFECYCLE as PL  # noqa: PLC0415

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
    owner_of = _requirement_owner_map(g)
    # Build lookup: stakeholder id -> list of operator ids (acting facets).
    # Delegated to _stakeholder_to_operator_ids to avoid a second entity loop
    # at this call site (atomicity audit counts for-loops over g.<entity>).
    op_by_stakeholder = _stakeholder_to_operator_ids(g)

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
    """Canon: §ContextBudget / §Invariants — operator budget measure (NODE_COUNT nodes or CRYSTAL_CHARS resident crystal) must not exceed its budget limit, else crystallize first or spawn a sub-operator.

    RULE: for each operator whose budget measure (NODE_COUNT nodes or CRYSTAL_CHARS resident crystal chars) exceeds its budget limit, fire — crystallize first; if still over, spawn a sub-operator:
      - If `measure == NODE_COUNT`, compute:
          size = len(g.requirements) + len(g.conflicts) + len(g.assumptions)
        (full-graph count; DomainScope narrowing is deferred to a later P-phase).
        NOTE: this measure counts the crystallized SUBSTRATE, which
        R-working-vs-substrate-budget declares free — kept only for backward
        compatibility with existing operators/tests that opted into it.
      - If `measure == CRYSTAL_CHARS`, compute:
          size = character-length of the resident crystal (root CLAUDE.md)
        This is the RESIDENT working set the operator actually re-loads by
        reference each boot (R-working-vs-substrate-budget) — the substrate
        (the content graph itself) is NOT counted. If CLAUDE.md is absent,
        size is 0 (nothing resident yet; not a violation).
      - If `size > limit`, fire a Violation with the imperative:
          'crystallize first; if still over, spawn a sub-operator'
          (R-crystallize-before-split, R-context-budget-rule).
      - `limit == 0` means unbounded; the check is skipped for that operator.

    WHY CRYSTAL_CHARS (replacing NODE_COUNT-as-substrate-proxy): NODE_COUNT
    measured the crystallized substrate itself, which R-working-vs-substrate-budget
    declares FREE — this falsely flagged operators as near-OVERLOADED for the very
    act of crystallizing or keeping REJECTED history. CRYSTAL_CHARS measures the
    one thing that costs real working context: the resident crystal (root
    CLAUDE.md) against the host's actual character ceiling. See R-context-budget-rule.

    WHY fire (not warn): 'domain > context' is exactly the kind of measurable,
    structural contradiction Hotam-Spec exists to surface. An over-budget operator
    is a real conflict the graph holds visibly, not a soft warning.
    """
    from hotam_spec.operator import CRYSTAL_CHARS, NODE_COUNT  # noqa: PLC0415

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
        elif op.context_budget.measure == CRYSTAL_CHARS:
            claude_md = _REPO_ROOT_FROM_INVARIANTS / "CLAUDE.md"
            size = len(claude_md.read_text(encoding="utf-8")) if claude_md.exists() else 0
            if size > limit:
                out.append(
                    Violation(
                        "check_operator_within_budget",
                        op.id,
                        f"operator '{op.id}' resident crystal is {size} chars > "
                        f"budget {limit} (CRYSTAL_CHARS measure); crystallize "
                        f"first (R-crystallize-before-split); if still over, "
                        f"spawn a sub-operator (R-context-bounded-delegation)",
                    )
                )
    return out


def check_scoped_node_has_single_presenter(g: TensionGraph) -> list[Violation]:
    """Canon: §Scope / §Invariants — every node in >=2 operators' scope overlap has exactly one presenter.

    RULE (R-overlap-single-presenter): a node contested by two or more
    operators must resolve to exactly one presenter, or the check fires —
    node '<id>' is contested by operators <ids> but no presenter could be
    determined; declare a presenter. Mechanically: compute each Operator's
    ScopeView via hotam_spec.scope_projection.project_scope(g, op.scope) for
    every operator whose `scope` tuple is non-empty. For each unordered pair
    of such operators, compute scope_projection.scope_overlap(view_a, view_b)
    and collect overlap_node_ids(overlap) (the union of overlapping
    Requirement and Conflict ids). For every node id that appears in >= 1
    pairwise overlap, the full set of operators whose scope contains that
    node id is passed to scope_projection.presenter_for_node, which
    deterministically returns the LEXICOGRAPHICALLY FIRST operator id — this
    is the ONE designated presenter. The invariant itself does not need to
    re-derive anything beyond that: presenter_for_node is total and
    deterministic for any non-empty operator-id set, so a violation can only
    mean the contested-operator set was empty, which cannot happen once a
    node has been found in a pairwise overlap. This check therefore currently
    reports NOTHING as a defect — it exists to make single-presentership
    PROVABLE (bijection-style): were presenter_for_node ever to return None
    for a node with >= 2 contesting operators, that would be the fired
    Violation.

    WHY this reads as "always green today": with the CURRENT graph (one
    OP-director, scope=()), no operator has a non-empty scope, so there is
    nothing to overlap — the calm-empty case (R-empty-content-wellformed).
    The invariant is still load-bearing: the moment a SECOND operator is
    spawned with an overlapping `scope` (R-context-bounded-delegation), this
    is the check that guarantees the overlap gets exactly one presenter
    instead of two operators silently disagreeing about who speaks for a
    shared node.

    WHY lexicographic-first (not e.g. "the parent operator" or "graph
    order"): documented on scope_projection.presenter_for_node — id order is
    stable under source reformatting; graph position and parent-hierarchy are
    not guaranteed total orders across arbitrary Operator sets.
    """
    from hotam_spec.scope_projection import (  # noqa: PLC0415
        overlap_node_ids,
        presenter_for_node,
        project_scope,
        scope_overlap,
    )

    scoped_ops = [op for op in g.operators if op.scope]
    if len(scoped_ops) < 2:
        return []

    views = {op.id: project_scope(g, op.scope) for op in scoped_ops}
    # node_id -> set of operator ids whose scope contains it
    contested: dict[str, set[str]] = {}
    for i in range(len(scoped_ops)):
        for j in range(i + 1, len(scoped_ops)):
            a, b = scoped_ops[i], scoped_ops[j]
            overlap = scope_overlap(views[a.id], views[b.id])
            for node_id in overlap_node_ids(overlap):
                contested.setdefault(node_id, set()).update({a.id, b.id})

    out: list[Violation] = []
    for node_id in sorted(contested):
        operator_ids_for_node = tuple(sorted(contested[node_id]))
        presenter = presenter_for_node(node_id, operator_ids_for_node)
        if presenter is None:
            out.append(
                Violation(
                    "check_scoped_node_has_single_presenter",
                    node_id,
                    f"node '{node_id}' is contested by operators "
                    f"{operator_ids_for_node} but no presenter could be "
                    f"determined; declare a presenter (R-overlap-single-presenter)",
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


def check_process_drives_existing_entities(g: TensionGraph) -> list[Violation]:
    """Canon: §Process / §Entity / §Invariants — Process.drives_entities resolves.

    RULE: each entity slug in Process.drives_entities MUST be a declared
    EntityType.slug in g.entity_types. Activates the forward-compat seam
    Process declared from day one (R-process-drives-existing-entity).

    WHY: was deferred while Entity aspect was DEFERRED (M12). Now Entity has
    landed (P21.1), the resolution is real. A Process driving an undeclared
    entity slug is a structural dead-end the harness must surface.
    """
    from hotam_spec.graph import entity_type_slugs  # noqa: PLC0415

    slugs = entity_type_slugs(g)
    out: list[Violation] = []
    for p in g.processes:
        for slug in p.drives_entities:
            if slug not in slugs:
                out.append(
                    Violation(
                        "check_process_drives_existing_entities",
                        p.id,
                        f"drives_entities slug '{slug}' is not a declared "
                        f"EntityType.slug (declared: {sorted(slugs)})",
                    )
                )
    return out


def check_step_invokes_known_transition(g: TensionGraph) -> list[Violation]:
    """Canon: §Process / §Entity / §Invariants — Step.invokes resolves to a real transition.

    RULE: when Step.invokes is non-empty, it MUST have format '<entity-slug>.<event>'
    where entity-slug is a declared EntityType.slug AND event matches a Transition.event
    in that EntityType.lifecycle.

    WHY: Step.invokes was prose-only while Entity was deferred. With Entity landed,
    the verb a Step invokes is a Lifecycle transition — making process steps and
    entity state machines structurally coupled. R-step-invokes-known-transition.

    NOTE (atomicity): one relation checked via three sub-rules (progressive
    validation gates -- format has a dot, entity-slug resolves, event
    resolves -- each a precondition of the next, not three independent
    rules): a Step.invokes value fails this ONE relation for exactly one of
    those three reasons at a time, so the three Violation messages below are
    failure branches of a single check, not evidence of a bundled multi-rule
    function.
    """
    type_by_slug = {et.slug: et for et in g.entity_types}
    out: list[Violation] = []
    for p in g.processes:
        for step in p.steps:
            if not step.invokes:
                continue
            if "." not in step.invokes:
                out.append(
                    Violation(
                        "check_step_invokes_known_transition",
                        p.id,
                        f"step '{step.name}'.invokes='{step.invokes}' must be "
                        f"'<entity-slug>.<event>'",
                    )
                )
                continue
            slug, _, event = step.invokes.partition(".")
            et = type_by_slug.get(slug)
            if et is None:
                out.append(
                    Violation(
                        "check_step_invokes_known_transition",
                        p.id,
                        f"step '{step.name}'.invokes='{step.invokes}' — "
                        f"unknown entity '{slug}'",
                    )
                )
                continue
            events = {t.event for t in et.lifecycle.transitions}
            if event not in events:
                out.append(
                    Violation(
                        "check_step_invokes_known_transition",
                        p.id,
                        f"step '{step.name}'.invokes='{step.invokes}' — "
                        f"event '{event}' is not a transition of '{slug}' "
                        f"(known: {sorted(events)})",
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
    altitude: every demanded role must be named. Supply >= demand is checked
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
# 14. §Entity aspect invariants (aspect-gated: no-op when g.entity_types/entities empty)
# ---------------------------------------------------------------------------


def check_entity_type_lifecycle_wellformed(g: TensionGraph) -> list[Violation]:
    """Canon: §Entity / §Invariants — every EntityType.lifecycle is a well-formed Lifecycle.

    RULE: every EntityType.lifecycle MUST pass check_lifecycle_wellformed (non-empty
    states, exactly one INITIAL, all transition endpoints resolve, terminal reachable
    if non-cyclic). No-ops when g.entity_types is empty (§Entity aspect not loaded).

    WHY: the §Lifecycle keystone is the single source of truth for state-machine
    well-formedness; reusing it here means every EntityType inherits all four
    conditions without parallel machinery (R-statemachine-wellformedness, M12).
    """
    out: list[Violation] = []
    for et in g.entity_types:
        for issue in check_lifecycle_wellformed(et.lifecycle):
            out.append(
                Violation(
                    "check_entity_type_lifecycle_wellformed",
                    et.slug,
                    issue,
                )
            )
    return out


def check_transition_guard_assumption_resolves(g: TensionGraph) -> list[Violation]:
    """Canon: §Lifecycle / §Invariants — every non-empty Transition.guard_assumption resolves.

    RULE: for every EntityType.lifecycle.transitions[*], when guard_assumption is
    non-empty it MUST name an Assumption id present in assumption_ids(g). A
    dangling guard_assumption is the behavioral-drift-seam analogue of a
    dangling Requirement.assumptions[*] reference — the drift machinery
    (R-stale-substrate / dead-assumption fallout) can only surface a guard as
    stale if the id it names actually resolves. No-ops when g.entity_types is
    empty (§Entity aspect not loaded).

    WHY part of the dangling-ref family: this is a shape check — for each
    Transition, guard_assumption must resolve in assumption_ids(g) — the exact
    homogeneous per-entity referential-integrity pattern check_no_dangling_ids'
    atoms already cover for Requirement/Conflict/Operator/Assumption edges;
    Transition.guard_assumption is the one remaining edge of that family that
    had no enforcer.
    """
    aids = assumption_ids(g)
    out: list[Violation] = []
    for et in g.entity_types:
        for t in et.lifecycle.transitions:
            if t.guard_assumption and t.guard_assumption not in aids:
                out.append(
                    Violation(
                        "check_transition_guard_assumption_resolves",
                        et.slug,
                        f"transition '{t.src}->{t.dst}' guard_assumption "
                        f"'{t.guard_assumption}' does not resolve to a known "
                        f"Assumption id",
                    )
                )
    return out


def check_assumption_machine_checks_syntactic(g: TensionGraph) -> list[Violation]:
    """Canon: §Assumption / §Invariants — every non-empty machine_check is a well-formed
    Python EXPRESSION (compilable), not free prose.

    RULE: for each Assumption whose machine_check is non-empty, compile it in
    'eval' mode; a SyntaxError is a Violation on the assumption id. An empty
    machine_check is skipped (the field is optional). This does NOT execute the
    formula and does NOT assert it is TRUE — see the honesty boundary below.

    WHY only a syntax check, deliberately (the honesty boundary): the two
    machine_checks carried in the self-domain graph evaluate against DIFFERENT,
    not-yet-materialized namespaces — 'python.version >= (3, 12)' names a
    `python` object that does not exist as written, and
    'len(graph.requirements) + len(graph.conflicts) < 10_000' expects a `graph`
    binding. There is today no single agreed namespace over which every
    machine_check is executable, so EXECUTING them (§Assumption docstring:
    'machine_check is carried but not run' — spec-stack layers 4/5 deferred)
    would require inventing that namespace, which R-uncrystallizable-automated
    forbids doing speculatively. What CAN be guaranteed structurally, without
    inventing semantics, is that the recorded formula is a well-formed
    expression — a compilable seam the deferred Z3/Hypothesis layers can later
    attach to — rather than prose masquerading as a machine_check. Promoting
    this to real execution is a separate, later act (a new atom) once a domain
    supplies the evaluation namespace.
    """
    out: list[Violation] = []
    for a in g.assumptions:
        mc = (a.machine_check or "").strip()
        if not mc:
            continue
        try:
            compile(mc, "<machine_check>", "eval")
        except SyntaxError as exc:
            out.append(
                Violation(
                    "check_assumption_machine_checks_syntactic",
                    a.id,
                    f"machine_check {mc!r} is not a well-formed Python "
                    f"expression ({exc.msg}) — it must be a compilable formula "
                    f"or empty, never prose",
                )
            )
    return out


def check_entity_instance_state_in_lifecycle(g: TensionGraph) -> list[Violation]:
    """Canon: §Entity / §Invariants — every EntityInstance.state is valid in its EntityType.lifecycle.

    RULE: EntityInstance.state MUST be matched by the lifecycle of the corresponding
    EntityType (via Lifecycle.matches). An instance with an unknown state is
    structurally invalid — the lifecycle machine cannot process it.
    No-ops when g.entities is empty.

    WHY: state integrity at the instance level mirrors check_requirement_status_in_lifecycle
    for requirements — the same keystone discipline applied to domain entities.
    """
    type_by_slug = {et.slug: et for et in g.entity_types}
    out: list[Violation] = []
    for inst in g.entities:
        et = type_by_slug.get(inst.entity_type)
        if et is None:
            out.append(
                Violation(
                    "check_entity_instance_state_in_lifecycle",
                    inst.id,
                    f"entity_type '{inst.entity_type}' is not declared",
                )
            )
            continue
        if et.lifecycle.matches(inst.state) is None:
            out.append(
                Violation(
                    "check_entity_instance_state_in_lifecycle",
                    inst.id,
                    f"state '{inst.state}' is not in lifecycle '{et.lifecycle.slug}' "
                    f"(valid: {sorted(et.lifecycle.state_names())})",
                )
            )
    return out


def check_entity_instance_required_fields(g: TensionGraph) -> list[Violation]:
    """Canon: §Entity / §Invariants — every required EntityField is present in EntityInstance.field_values.

    RULE: for each EntityField with required=True on the EntityType, the
    corresponding field name MUST appear in EntityInstance.field_values. A missing
    required field is a structural gap — the instance is incomplete.
    No-ops when g.entities is empty.

    WHY: required fields are the entity's schema contract; a missing required field
    violates the declared type and makes downstream traversal unreliable.
    """
    type_by_slug = {et.slug: et for et in g.entity_types}
    out: list[Violation] = []
    for inst in g.entities:
        et = type_by_slug.get(inst.entity_type)
        if et is None:
            continue  # already reported by check_entity_instance_state_in_lifecycle
        provided = {n for n, _ in inst.field_values}
        for f in et.fields:
            if f.required and f.name not in provided:
                out.append(
                    Violation(
                        "check_entity_instance_required_fields",
                        inst.id,
                        f"required field '{f.name}' is missing",
                    )
                )
    return out


def check_entity_instance_id_prefix(g: TensionGraph) -> list[Violation]:
    """Canon: §Entity / §Invariants — every EntityInstance.id starts with 'ENT-<slug>-'.

    RULE: EntityInstance.id MUST start with 'ENT-<entity_type>-' (typed-anchor
    discipline, R-anchor-everything). A missing or wrong prefix breaks the
    typed-anchor discipline and makes cite-by-reference unreliable.
    No-ops when g.entities is empty.

    WHY: the prefix encodes both type and entity kind in the id, enabling
    unambiguous cross-reference in the graph (R-anchor-everything).
    """
    out: list[Violation] = []
    for inst in g.entities:
        expected_prefix = f"ENT-{inst.entity_type}-"
        if not inst.id.startswith(expected_prefix):
            out.append(
                Violation(
                    "check_entity_instance_id_prefix",
                    inst.id,
                    f"entity instance id must start with '{expected_prefix}'",
                )
            )
    return out


def check_entity_instance_refs_resolve(g: TensionGraph) -> list[Violation]:
    """Canon: §Entity / §Invariants — every reference EntityField value resolves in the graph.

    RULE: for each EntityField with kind='reference', any non-empty field value in
    EntityInstance.field_values MUST resolve in the graph according to ref_target:
    'stakeholder' resolves in stakeholder_ids(g); 'requirement' in requirement_ids(g);
    'assumption' in assumption_ids(g); any other string is treated as an entity_type
    slug and resolves among EntityInstance ids of that type. Empty values on optional
    references are allowed; missing required references are caught by
    check_entity_instance_required_fields. No-ops when g.entities is empty.

    WHY: a dangling reference field is the entity-level equivalent of a dangling
    Conflict member — the edge exists but resolves to nothing, making the
    dependency invisible.
    """
    sids = stakeholder_ids(g)
    rids = requirement_ids(g)
    aids = assumption_ids(g)
    entities_by_type: dict[str, set[str]] = {}
    for e in g.entities:
        entities_by_type.setdefault(e.entity_type, set()).add(e.id)

    type_by_slug = {et.slug: et for et in g.entity_types}
    out: list[Violation] = []
    for inst in g.entities:
        et = type_by_slug.get(inst.entity_type)
        if et is None:
            continue  # already reported by check_entity_instance_state_in_lifecycle
        fv = dict(inst.field_values)
        for f in et.fields:
            if f.kind != "reference":
                continue
            val = fv.get(f.name, "")
            if not val:
                continue  # required-missing reported by check_entity_instance_required_fields
            target = f.ref_target
            if target == "stakeholder":
                valid = val in sids
            elif target == "requirement":
                valid = val in rids
            elif target == "assumption":
                valid = val in aids
            else:
                valid = val in entities_by_type.get(target, set())
            if not valid:
                out.append(
                    Violation(
                        "check_entity_instance_refs_resolve",
                        inst.id,
                        f"reference field '{f.name}'='{val}' does not resolve "
                        f"in ref_target '{target}'",
                    )
                )
    return out


def check_entity_field_kind_known(g: TensionGraph) -> list[Violation]:
    """Canon: §Entity / §Invariants — every EntityField.kind is in ENTITY_FIELD_KINDS.

    RULE: EntityField.kind MUST be in ENTITY_FIELD_KINDS
    (string | number | enum | reference | state). An unknown kind is a
    misconfiguration that makes the field type undiscoverable.
    No-ops when g.entity_types is empty.

    WHY: the kind discriminant is the seam for future machine-checkable field
    validation; an unknown kind breaks the discriminant and hides the field
    from any kind-specific invariant.
    """
    from hotam_spec.entity import ENTITY_FIELD_KINDS  # noqa: PLC0415

    out: list[Violation] = []
    for et in g.entity_types:
        for f in et.fields:
            if f.kind not in ENTITY_FIELD_KINDS:
                out.append(
                    Violation(
                        "check_entity_field_kind_known",
                        et.slug,
                        f"field '{f.name}' has unknown kind '{f.kind}' "
                        f"(valid: {sorted(ENTITY_FIELD_KINDS)})",
                    )
                )
    return out


def check_typed_anchors_entity(g: TensionGraph) -> list[Violation]:
    """Canon: §Entity / §Invariants — every EntityInstance id starts with 'ENT-'.

    RULE: EntityInstance.id MUST start with 'ENT-' (typed-anchor discipline,
    R-anchor-everything). Note: check_entity_instance_id_prefix verifies the
    STRICTER 'ENT-<slug>-' rule. This check enforces only the prefix family.

    WHY: the 'ENT-' prefix family anchors all entity instances in the typed-anchor
    discipline (R-anchor-everything), enabling unambiguous cross-reference.
    """
    out: list[Violation] = []
    for e in g.entities:
        if not e.id.startswith("ENT-"):
            out.append(
                Violation(
                    "check_typed_anchors_entity",
                    e.id,
                    f"EntityInstance id '{e.id}' must start with 'ENT-' "
                    f"(typed-anchor rule, R-anchor-everything)",
                )
            )
    return out


# ---------------------------------------------------------------------------
# 14b. Entity-docs anti-drift — ENTITIES.md lists every declared EntityType
# ---------------------------------------------------------------------------

_DOMAINS_ROOT_FOR_ENTITY_CHECK = Path(__file__).resolve().parents[3] / "domains"
_REPO_ROOT_FOR_ENTITY_CHECK = Path(__file__).resolve().parents[3]


def check_entities_md_lists_all_types(g: TensionGraph) -> list[Violation]:  # noqa: ARG001
    """Canon: §Entity / §Invariants — every declared EntityType appears as a section in ENTITIES.md.

    RULE: for each domain in domains/<name>/ whose graph.py declares entity_types,
    the corresponding domains/<name>/docs/gen/ENTITIES.md MUST contain a section
    header '## <slug>' for every EntityType slug. A new EntityType without a
    generated map entry would silently disappear from the operator's view —
    R-drift-structurally-impossible applied to entity-derived docs.

    WHY walks domains (not the passed graph): this is a filesystem-coherence check
    on the committed generated docs — mirrors check_domain_manifest_* in style.
    The graph argument is accepted for API consistency but not used; the check
    loads each domain's graph independently. This avoids false positives when
    the invariant is run against an in-memory fixture (e.g. seed_graph()).

    WHY aspect-gated per domain: a domain with no entity_types need not have any
    ## type sections in its ENTITIES.md.
    """
    domains_root = _DOMAINS_ROOT_FOR_ENTITY_CHECK
    if not domains_root.exists():
        return []

    import importlib.util as _ilu  # noqa: PLC0415

    out: list[Violation] = []

    for domain_dir in sorted(domains_root.iterdir()):
        if not domain_dir.is_dir() or domain_dir.name.startswith("_"):
            continue

        domain_graph_py = domain_dir / "graph.py"
        if not domain_graph_py.exists():
            continue
        try:
            _spec = _ilu.spec_from_file_location(
                f"_entity_check_domain_{domain_dir.name}_graph", domain_graph_py
            )
            if _spec is None or _spec.loader is None:
                continue
            _mod = _ilu.module_from_spec(_spec)
            _spec.loader.exec_module(_mod)  # type: ignore[union-attr]
            dg: TensionGraph = _mod.build_graph()
        except Exception:
            continue  # Malformed graph — handled by other domain-manifest checks.

        if not dg.entity_types:
            continue  # §Entity aspect not activated in this domain.

        entities_md = domain_dir / "docs" / "gen" / "ENTITIES.md"
        if not entities_md.exists():
            for et in dg.entity_types:
                out.append(
                    Violation(
                        "check_entities_md_lists_all_types",
                        et.slug,
                        f"EntityType '{et.slug}' in domain '{domain_dir.name}' not listed "
                        f"in ENTITIES.md (file does not exist — run gen_spec.py)",
                    )
                )
            continue

        content = entities_md.read_text(encoding="utf-8")
        for et in dg.entity_types:
            if not re.search(rf"^## {re.escape(et.slug)}\b", content, re.MULTILINE):
                out.append(
                    Violation(
                        "check_entities_md_lists_all_types",
                        et.slug,
                        f"EntityType '{et.slug}' in domain '{domain_dir.name}' has no "
                        f"'## {et.slug}' section in ENTITIES.md "
                        f"(run gen_spec.py to regenerate — R-drift-structurally-impossible)",
                    )
                )
    return out


def check_entity_type_constitution_projection(g: TensionGraph) -> list[Violation]:  # noqa: ARG001
    """Canon: §Entity / §Invariants — every declared EntityType appears as R-entity-<slug> in FRAMEWORK-INVARIANTS.md.

    RULE: for each domain in domains/<name>/ whose graph.py declares entity_types,
    the corresponding domains/<name>/docs/gen/FRAMEWORK-INVARIANTS.md MUST contain
    a row naming 'R-entity-<slug>' for every EntityType slug in that domain's
    graph. A new EntityType without a projected R-entity-<slug> row in
    FRAMEWORK-INVARIANTS.md would silently disappear from the operator's boot
    sequence — the same R-drift-structurally-impossible guarantee
    check_entities_md_lists_all_types gives ENTITIES.md, applied to
    FRAMEWORK-INVARIANTS.md instead. NOTE: entity-derived requirements project
    into FRAMEWORK-INVARIANTS.md, NOT the root CONSTITUTION.md block — they are
    framework-plumbing, relocated out of CONSTITUTION.md by gen_spec.py's
    build_framework_invariants/_render_constitution_block split (see
    test_entity_constitution_section_appears_when_types_present, which asserts
    R-entity-<slug> is ABSENT from the root CONSTITUTION block).

    WHY a sibling check, not a merge into check_entities_md_lists_all_types:
    the two checks cover two DISTINCT generated docs (ENTITIES.md vs
    FRAMEWORK-INVARIANTS.md) driven by two distinct requirements
    (R-entities-md-generated vs R-entity-derived-requirement) —
    R-requirement-claim-is-atomic forbids one check verifying two unrelated
    claims.

    WHY walks domains (not the passed graph): filesystem-coherence check on
    the committed generated docs, same shape as check_entities_md_lists_all_types
    (see that function's WHY for the full rationale on graph-argument unused).
    """
    domains_root = _DOMAINS_ROOT_FOR_ENTITY_CHECK
    if not domains_root.exists():
        return []

    import importlib.util as _ilu  # noqa: PLC0415

    out: list[Violation] = []

    for domain_dir in sorted(domains_root.iterdir()):
        if not domain_dir.is_dir() or domain_dir.name.startswith("_"):
            continue

        domain_graph_py = domain_dir / "graph.py"
        if not domain_graph_py.exists():
            continue
        try:
            _spec = _ilu.spec_from_file_location(
                f"_entity_ctor_check_domain_{domain_dir.name}_graph", domain_graph_py
            )
            if _spec is None or _spec.loader is None:
                continue
            _mod = _ilu.module_from_spec(_spec)
            _spec.loader.exec_module(_mod)  # type: ignore[union-attr]
            dg: TensionGraph = _mod.build_graph()
        except Exception:
            continue  # Malformed graph — handled by other domain-manifest checks.

        if not dg.entity_types:
            continue  # §Entity aspect not activated in this domain.

        framework_invariants_md = domain_dir / "docs" / "gen" / "FRAMEWORK-INVARIANTS.md"
        if not framework_invariants_md.exists():
            for et in dg.entity_types:
                out.append(
                    Violation(
                        "check_entity_type_constitution_projection",
                        et.slug,
                        f"EntityType '{et.slug}' in domain '{domain_dir.name}' not "
                        f"projected as R-entity-{et.slug} in FRAMEWORK-INVARIANTS.md "
                        f"(file does not exist — run gen_spec.py)",
                    )
                )
            continue

        content = framework_invariants_md.read_text(encoding="utf-8")
        for et in dg.entity_types:
            if f"R-entity-{et.slug}" not in content:
                out.append(
                    Violation(
                        "check_entity_type_constitution_projection",
                        et.slug,
                        f"EntityType '{et.slug}' in domain '{domain_dir.name}' has no "
                        f"'R-entity-{et.slug}' row in FRAMEWORK-INVARIANTS.md "
                        f"(run gen_spec.py to regenerate — R-drift-structurally-impossible)",
                    )
                )
    return out


# ---------------------------------------------------------------------------
# 15. Section-anchor coherence — every §-token in framework docstrings is known
# ---------------------------------------------------------------------------

_SECTION_TOKEN_RE = re.compile(r"§[A-Za-z][\w-]*")
_TENSIO_SRC = Path(__file__).resolve().parent


def check_section_anchors_known(g: TensionGraph) -> list[Violation]:
    """Canon: §Invariants / §Glossary — every section-anchor token in framework docstrings is known.

    RULE: every section-anchor token (pattern: section-sign followed by an
    identifier, e.g. the tokens used in 'Canon: §Requirement') found in any
    spec/src/hotam_spec/*.py docstring MUST appear in
    `hotam_spec.glossary.term_slugs()`. An unrecognised section-anchor token is
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
    from hotam_spec.glossary import term_slugs  # noqa: PLC0415

    known = term_slugs()
    out: list[Violation] = []
    for path in sorted(_TENSIO_SRC.glob("*.py")):
        tree = _cached_parse_path(str(path.resolve()))
        if tree is None:
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


def _load_domain_manifest(domain_dir: Path) -> object | None:
    """Load and return the manifest module for a domain directory, or None on failure.

    Helper for domain manifest atomic checks — avoids duplicating importlib logic.
    Returns the loaded module object, or None if the manifest is missing or unloadable.
    Not a check_* function; not in ALL_INVARIANTS.
    """
    import importlib.util  # noqa: PLC0415

    manifest_py = domain_dir / "manifest.py"
    if not manifest_py.exists():
        return None
    spec = importlib.util.spec_from_file_location(
        f"_manifest_{domain_dir.name}", manifest_py
    )
    if spec is None or spec.loader is None:
        return None
    mod = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(mod)  # type: ignore[union-attr]
    except Exception:
        return None
    return mod


def check_domain_manifest_exists_and_importable(  # noqa: ARG001
    g: TensionGraph,
) -> list[Violation]:
    """Canon: §Domain — every domains/<name>/manifest.py exists and can be imported.

    RULE: A domain directory MUST contain a manifest.py that can be loaded
    without error. Missing or unloadable manifests make the domain invisible to
    the framework (R-domain-has-manifest).

    WHY: The manifest is the stable identity anchor for a domain; if it cannot
    be loaded, no field checks are possible and the domain is structurally dark.
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
                    "check_domain_manifest_exists_and_importable",
                    domain_dir.name,
                    f"domains/{domain_dir.name}/manifest.py is missing — "
                    "every domain must declare ID, DESCRIPTION, GOALS, DIRECTOR",
                )
            )
            continue
        load_error: str | None = None
        spec = importlib.util.spec_from_file_location(
            f"_manifest_exi_{domain_dir.name}", manifest_py
        )
        if spec is None or spec.loader is None:
            load_error = "bad spec or loader"
        else:
            mod = importlib.util.module_from_spec(spec)
            try:
                spec.loader.exec_module(mod)  # type: ignore[union-attr]
            except Exception as exc:
                load_error = str(exc)
        if load_error is not None:
            out.append(
                Violation(
                    "check_domain_manifest_exists_and_importable",
                    domain_dir.name,
                    f"manifest.py could not be loaded for domain '{domain_dir.name}': {load_error}",
                )
            )
    return out


def check_domain_manifest_id_matches_dirname(  # noqa: ARG001
    g: TensionGraph,
) -> list[Violation]:
    """Canon: §Domain — every domains/<name>/manifest.py ID field matches the directory name.

    RULE: manifest.py MUST define an ID attribute equal to the directory name.
    A mismatched ID breaks the identity anchor for gen_spec and create_domain tooling.

    WHY: The manifest ID is the stable identity key; a mismatch makes the domain
    undiscoverable or discoverable under the wrong name (R-domain-has-manifest).
    """
    if not _DOMAINS_ROOT.exists():
        return []
    out: list[Violation] = []
    for domain_dir in sorted(_DOMAINS_ROOT.iterdir()):
        if not domain_dir.is_dir():
            continue
        mod = _load_domain_manifest(domain_dir)
        if mod is None:
            continue  # caught by check_domain_manifest_exists_and_importable
        domain_id = getattr(mod, "ID", None)
        if domain_id != domain_dir.name:
            out.append(
                Violation(
                    "check_domain_manifest_id_matches_dirname",
                    domain_dir.name,
                    f"manifest.py ID '{domain_id}' does not match dirname '{domain_dir.name}'",
                )
            )
    return out


def check_domain_manifest_description_nonempty(  # noqa: ARG001
    g: TensionGraph,
) -> list[Violation]:
    """Canon: §Domain — every domains/<name>/manifest.py DESCRIPTION is non-empty.

    RULE: manifest.py MUST define a non-empty DESCRIPTION attribute. A domain
    without a description is undocumented and invisible to human readers.

    WHY: The manifest DESCRIPTION is surfaced in gen_spec output; missing it
    breaks the generated domain map (R-domain-has-manifest).
    """
    if not _DOMAINS_ROOT.exists():
        return []
    out: list[Violation] = []
    for domain_dir in sorted(_DOMAINS_ROOT.iterdir()):
        if not domain_dir.is_dir():
            continue
        mod = _load_domain_manifest(domain_dir)
        if mod is None:
            continue
        if not getattr(mod, "DESCRIPTION", None):
            out.append(
                Violation(
                    "check_domain_manifest_description_nonempty",
                    domain_dir.name,
                    "manifest.py DESCRIPTION is empty",
                )
            )
    return out


def check_domain_manifest_goals_nonempty(  # noqa: ARG001
    g: TensionGraph,
) -> list[Violation]:
    """Canon: §Domain — every domains/<name>/manifest.py GOALS is non-empty.

    RULE: manifest.py MUST define a non-empty GOALS attribute. A domain without
    declared goals has no visible intent and cannot drive the burn-down meter.

    WHY: GOALS drive the Goal objects in the domain graph; missing them makes
    the domain's purpose invisible (R-domain-has-manifest).
    """
    if not _DOMAINS_ROOT.exists():
        return []
    out: list[Violation] = []
    for domain_dir in sorted(_DOMAINS_ROOT.iterdir()):
        if not domain_dir.is_dir():
            continue
        mod = _load_domain_manifest(domain_dir)
        if mod is None:
            continue
        if not getattr(mod, "GOALS", None):
            out.append(
                Violation(
                    "check_domain_manifest_goals_nonempty",
                    domain_dir.name,
                    "manifest.py GOALS is empty or missing",
                )
            )
    return out


def check_domain_manifest_director_nonempty(  # noqa: ARG001
    g: TensionGraph,
) -> list[Violation]:
    """Canon: §Domain — every domains/<name>/manifest.py DIRECTOR is non-empty.

    RULE: manifest.py MUST define a non-empty DIRECTOR attribute naming the
    director agent. A domain without a declared director is headless.

    WHY: The DIRECTOR is the entry point for all domain-level operator delegation;
    missing it means no agent can be discovered (R-domain-declares-director).
    """
    if not _DOMAINS_ROOT.exists():
        return []
    out: list[Violation] = []
    for domain_dir in sorted(_DOMAINS_ROOT.iterdir()):
        if not domain_dir.is_dir():
            continue
        mod = _load_domain_manifest(domain_dir)
        if mod is None:
            continue
        if not getattr(mod, "DIRECTOR", None):
            out.append(
                Violation(
                    "check_domain_manifest_director_nonempty",
                    domain_dir.name,
                    "manifest.py DIRECTOR is empty or missing",
                )
            )
    return out


def check_domain_manifest_valid(g: TensionGraph) -> list[Violation]:  # noqa: ARG001
    """Canon: §Domain — every domains/<name>/manifest.py defines ID (matching dirname), DESCRIPTION, GOALS, DIRECTOR (thin delegator).

    RULE: A domain without a valid manifest is invisible to the framework.
    WHY: The manifest is the stable identity anchor for a domain; missing or
    mismatched fields make the domain undiscoverable by gen_spec and
    create_domain tooling (R-domain-has-manifest).

    This is a THIN DELEGATOR — calls the atomic sub-checks and concatenates.
    The atomic sub-checks are registered individually in ALL_INVARIANTS.
    """
    out: list[Violation] = []
    out.extend(check_domain_manifest_exists_and_importable(g))
    out.extend(check_domain_manifest_id_matches_dirname(g))
    out.extend(check_domain_manifest_description_nonempty(g))
    out.extend(check_domain_manifest_goals_nonempty(g))
    out.extend(check_domain_manifest_director_nonempty(g))
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


def check_agent_has_tools_subdir(g: TensionGraph) -> list[Violation]:  # noqa: ARG001
    """Canon: §Agent — every agent directory must contain a 'tools/' subdirectory.

    RULE: Every agent carries its own tools/ subdir for its private tools
    (R-agent-has-own-tools-dir), separate from the shared spec/tools/
    (R-shared-tools-in-spec-tools).
    WHY: Without tools/ the private/shared tool boundary is invisible;
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
        tools_subdir = agent_dir / "tools"
        if not tools_subdir.exists():
            out.append(
                Violation(
                    "check_agent_has_tools_subdir",
                    agent_dir.name,
                    f"spec/agents/{agent_dir.name}/tools/ is missing — "
                    "every agent must have a tools/ subdir for its private tools",
                )
            )
    return out


# ---------------------------------------------------------------------------
# 16. Meta-invariant — each check_*'s docstring RULE matches its body's Violations
# ---------------------------------------------------------------------------

_STOPWORDS = frozenset(
    {
        "a",
        "an",
        "the",
        "is",
        "must",
        "shall",
        "of",
        "in",
        "to",
        "for",
        "with",
        "that",
        "this",
        "be",
        "not",
        "has",
        "have",
        "or",
        "and",
        "if",
        "its",
        "it",
        "by",
        "from",
        "on",
        "are",
        "each",
        "any",
        "no",
        "all",
    }
)

_CANON_RULE_RE = re.compile(r"Canon:\s*§[A-Za-z][\w-]*\s*[/\w\s-]*—\s*(.+)")
_RULE_LINE_RE = re.compile(r"RULE(?:\s*\([^)]*\))?:\s*(.+)")


def _tokenize(text: str) -> set[str]:
    """Return lowercase alphanumeric/underscore tokens with stopwords removed."""
    tokens = re.findall(r"[a-zA-Z_]\w*", text.lower())
    return {t for t in tokens if t not in _STOPWORDS}


def _extract_rule_from_docstring(doc: str) -> str | None:
    """Return the rule text from a docstring, or None if not found.

    Tries 'Canon: <section-anchor> — <rule>' on the first line first;
    falls back to any line containing RULE: <text>.
    """
    lines = doc.splitlines()
    # Try first line Canon pattern
    if lines:
        m = _CANON_RULE_RE.search(lines[0])
        if m:
            return m.group(1).strip()
    # Fall back: find any RULE: line
    for line in lines:
        m = _RULE_LINE_RE.search(line)
        if m:
            return m.group(1).strip()
    return None


def _extract_violation_messages_from_source(fn: object) -> list[str]:
    """Extract all string literals passed as the message (3rd positional or message=) to Violation(...).

    Uses ast on the function source; returns empty list if source is unavailable or unparseable.
    """
    tree = _cached_parse_source_of(fn)
    if tree is None:
        return []

    messages: list[str] = []
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        # Match Violation(...) constructor calls
        func = node.func
        is_violation = (isinstance(func, ast.Name) and func.id == "Violation") or (
            isinstance(func, ast.Attribute) and func.attr == "Violation"
        )
        if not is_violation:
            continue
        # 3rd positional arg (index 2) is the message
        if len(node.args) >= 3:
            arg = node.args[2]
            if isinstance(arg, ast.Constant) and isinstance(arg.value, str):
                messages.append(arg.value)
            elif isinstance(arg, ast.JoinedStr):
                # f-string — extract literal parts
                parts = [
                    v.value
                    for v in arg.values
                    if isinstance(v, ast.Constant) and isinstance(v.value, str)
                ]
                messages.append(" ".join(parts))
        # keyword message=
        for kw in node.keywords:
            if kw.arg == "message":
                if isinstance(kw.value, ast.Constant) and isinstance(
                    kw.value.value, str
                ):
                    messages.append(kw.value.value)
                elif isinstance(kw.value, ast.JoinedStr):
                    parts = [
                        v.value
                        for v in kw.value.values
                        if isinstance(v, ast.Constant) and isinstance(v.value, str)
                    ]
                    messages.append(" ".join(parts))
    return messages


_JACCARD_THRESHOLD = 0.05


def check_method_matches_docstring(g: TensionGraph) -> list[Violation]:  # noqa: ARG001
    """Canon: §Invariants — each check_* docstring RULE shares non-trivial lexical overlap with its Violation messages.

    RULE: for every function in ALL_INVARIANTS, it MUST have a docstring, that
    docstring MUST contain a RULE line, and the RULE line extracted from its
    docstring MUST share at least 5% Jaccard token overlap with the concatenated
    text of all Violation messages in the function body. A mismatch means the
    docstring describes a different rule from what the code enforces (silent drift).

    WHY: docstring-body drift is the silent failure mode where a check_* says one
    thing and does another. This meta-invariant catches gross mismatches
    automatically — the same 'visible-not-invisible' principle applied to the
    framework's own machinery.

    NOTE (atomicity): one relation checked via three sub-rules (progressive
    validation gates -- docstring exists, RULE line exists, Jaccard overlap
    meets threshold -- each a precondition of the next, not three
    independent rules): a function fails this ONE relation for exactly one
    of those three reasons at a time, so the three Violation messages below
    are failure branches of a single check, not a bundled multi-rule
    function.

    The Jaccard threshold (0.05) is heuristic and chosen to catch obvious mismatches
    without over-flagging terse-but-correct docstrings that use different but
    semantically related vocabulary (R-method-matches-docstring).
    """
    out: list[Violation] = []
    for fn in ALL_INVARIANTS:
        name = fn.__name__
        doc = inspect.getdoc(fn)
        if not doc:
            out.append(
                Violation(
                    "check_method_matches_docstring",
                    name,
                    f"check_* '{name}' has no docstring — add a docstring with a RULE line",
                )
            )
            continue
        rule_text = _extract_rule_from_docstring(doc)
        if rule_text is None:
            out.append(
                Violation(
                    "check_method_matches_docstring",
                    name,
                    f"check_* '{name}' has no RULE line in docstring "
                    f"(expected 'RULE: ...' or first line matching Canon pattern)",
                )
            )
            continue
        messages = _extract_violation_messages_from_source(fn)
        if not messages:
            # No Violation calls found (e.g. thin delegator that calls sub-checks) — skip overlap check
            continue
        rule_tokens = _tokenize(rule_text)
        body_tokens = _tokenize(" ".join(messages))
        if not rule_tokens or not body_tokens:
            continue
        intersection = rule_tokens & body_tokens
        union = rule_tokens | body_tokens
        jaccard = len(intersection) / len(union)
        if jaccard < _JACCARD_THRESHOLD:
            out.append(
                Violation(
                    "check_method_matches_docstring",
                    name,
                    f"check_* '{name}': docstring RULE and body violation messages share "
                    f"{jaccard:.0%} token overlap (threshold 5%) — "
                    f"verify they describe the same rule",
                )
            )
    return out


# ---------------------------------------------------------------------------
# M22 (R-rules-as-data, HYBRID) — classification table: which check_* families
# are TABLE_DRIVEN (homogeneous, one row = one rule, generated by iterating a
# shared per-kind shape) vs BESPOKE (irreducibly hand-written). The table is
# DATA an agent can read/extend without re-deriving the classification from
# scratch; the functions themselves stay hand-written code so
# check_method_matches_docstring (R-method-matches-docstring) keeps working —
# every check_* still carries its OWN inspectable source and literal
# Violation(...) messages, which a purely closure/exec-generated function
# cannot provide without opacity (see WHY note on check_rules_as_data_
# classification_coherent below for the empirical reason this boundary holds).
# ---------------------------------------------------------------------------

TABLE_DRIVEN = "TABLE_DRIVEN"
BESPOKE = "BESPOKE"
RULES_AS_DATA_KINDS = frozenset({TABLE_DRIVEN, BESPOKE})


@dataclass(frozen=True)
class InvariantClassification:
    """Canon: §Invariants — one row: a check_* name, its family kind, and why.

    RULE: `kind` MUST be in RULES_AS_DATA_KINDS. `name` MUST be a function name
    present in ALL_INVARIANTS (enforced by check_rules_as_data_classification_
    coherent). TABLE_DRIVEN marks a homogeneous per-entity structural check
    (dangling-refs, typed-anchors, lifecycle-membership) whose RULE is 'for
    each node of kind K, field F must satisfy relation R against set S' — the
    kind of repetition an agent should read as one row, not N near-duplicate
    functions. BESPOKE marks a check whose logic cannot be reduced to that
    shape (identity derivation, cross-entity bijection, docstring/body
    coherence, filesystem walks, arithmetic branching).

    WHY a table over the functions rather than the functions THEMSELVES being
    generated from a table: check_method_matches_docstring extracts literal
    Violation(...) string-literal messages via inspect.getsource(fn); a
    function assigned from a shared closure/functools.partial has no
    individually inspectable source (verified: inspect.getsource raises OSError
    on such closures), so message-literal auditing would silently stop
    working for every "generated" check. Declaring the CLASSIFICATION as data
    (this table) gets the agent-convenience win — read the family, read one
    representative row, extend by adding a row plus one small function that
    follows the family's established shape — without paying that price.
    """

    name: str
    kind: str
    why: str


RULES_AS_DATA_TABLE: tuple[InvariantClassification, ...] = (
    # --- TABLE_DRIVEN: referential integrity (dangling-refs family) ---
    InvariantClassification(
        "check_no_dangling_assumption_owner",
        TABLE_DRIVEN,
        "shape: for each Assumption, owner must resolve in stakeholder_ids(g)",
    ),
    InvariantClassification(
        "check_no_dangling_requirement_owner",
        TABLE_DRIVEN,
        "shape: for each Requirement, owner must resolve in stakeholder_ids(g)",
    ),
    InvariantClassification(
        "check_no_dangling_requirement_assumptions",
        TABLE_DRIVEN,
        "shape: for each Requirement.assumptions[*], must resolve in assumption_ids(g)",
    ),
    InvariantClassification(
        "check_no_dangling_requirement_relations",
        TABLE_DRIVEN,
        "shape: for each Requirement.relations[*], kind/target must resolve",
    ),
    InvariantClassification(
        "check_no_dangling_conflict_refs",
        TABLE_DRIVEN,
        "shape: for each Conflict, five ref fields must resolve against their registries",
    ),
    InvariantClassification(
        "check_no_dangling_operator_refs",
        TABLE_DRIVEN,
        "shape: for each Operator, stakeholder/parent must resolve",
    ),
    # --- TABLE_DRIVEN: typed-anchor prefix family ---
    InvariantClassification(
        "check_typed_anchors_requirement",
        TABLE_DRIVEN,
        "shape: for each Requirement, id must start with prefix 'R-'",
    ),
    InvariantClassification(
        "check_typed_anchors_assumption",
        TABLE_DRIVEN,
        "shape: for each Assumption, id must start with prefix 'A-'",
    ),
    InvariantClassification(
        "check_typed_anchors_conflict",
        TABLE_DRIVEN,
        "shape: for each Conflict, id must start with prefix 'C-'",
    ),
    InvariantClassification(
        "check_typed_anchors_operator",
        TABLE_DRIVEN,
        "shape: for each Operator, id must start with prefix 'OP-'",
    ),
    InvariantClassification(
        "check_typed_anchors_process",
        TABLE_DRIVEN,
        "shape: for each Process, id must start with prefix 'PR-'",
    ),
    InvariantClassification(
        "check_typed_anchors_goal",
        TABLE_DRIVEN,
        "shape: for each Goal, id must start with prefix 'GOAL-'",
    ),
    InvariantClassification(
        "check_typed_anchors_entity",
        TABLE_DRIVEN,
        "shape: for each EntityInstance, id must start with prefix 'ENT-'",
    ),
    # --- TABLE_DRIVEN: lifecycle-membership family ---
    InvariantClassification(
        "check_requirement_status_in_lifecycle",
        TABLE_DRIVEN,
        "shape: for each Requirement, status must match REQUIREMENT_STATUS_LIFECYCLE",
    ),
    InvariantClassification(
        "check_conflict_lifecycle_in_lifecycle",
        TABLE_DRIVEN,
        "shape: for each Conflict, lifecycle must match CONFLICT_LIFECYCLE",
    ),
    InvariantClassification(
        "check_operator_lifecycle_in_lifecycle",
        TABLE_DRIVEN,
        "shape: for each Operator, lifecycle must match OPERATOR_LIFECYCLE",
    ),
    InvariantClassification(
        "check_goal_lifecycle_in_lifecycle",
        TABLE_DRIVEN,
        "shape: for each Goal, lifecycle must match GOAL_LIFECYCLE",
    ),
    # --- TABLE_DRIVEN: known-set / enum-membership family ---
    InvariantClassification(
        "check_enforceability_kind_known",
        TABLE_DRIVEN,
        "shape: for each Requirement, enforceability must be in ENFORCEABILITY_KINDS",
    ),
    InvariantClassification(
        "check_assumption_status_valid",
        TABLE_DRIVEN,
        "shape: for each Assumption, status must be in ASSUMPTION_STATES",
    ),
    InvariantClassification(
        "check_goal_target_kind_known",
        TABLE_DRIVEN,
        "shape: for each Goal, target_state.kind must be in TARGET_KINDS",
    ),
    InvariantClassification(
        "check_entity_field_kind_known",
        TABLE_DRIVEN,
        "shape: for each EntityField, kind must be in ENTITY_FIELD_KINDS",
    ),
    InvariantClassification(
        "check_goal_owner_is_operator",
        TABLE_DRIVEN,
        "shape: for each Goal, owner must resolve in operator_ids(g)",
    ),
    InvariantClassification(
        "check_entity_instance_state_in_lifecycle",
        TABLE_DRIVEN,
        "shape: for each EntityInstance, state must match its EntityType.lifecycle",
    ),
    InvariantClassification(
        "check_entity_instance_id_prefix",
        TABLE_DRIVEN,
        "shape: for each EntityInstance, id must start with 'ENT-<entity_type>-'",
    ),
    # --- TABLE_DRIVEN: non-empty-field family (Conflict axis/context/steward) ---
    InvariantClassification(
        "check_conflict_has_axis",
        TABLE_DRIVEN,
        "shape: for each Conflict, axis field must be non-empty",
    ),
    InvariantClassification(
        "check_conflict_has_context",
        TABLE_DRIVEN,
        "shape: for each Conflict, context field must be non-empty",
    ),
    InvariantClassification(
        "check_conflict_has_steward",
        TABLE_DRIVEN,
        "shape: for each Conflict, steward field must be non-empty",
    ),
    InvariantClassification(
        "check_conflict_min_two_members",
        TABLE_DRIVEN,
        "shape: for each Conflict, len(set(members)) must satisfy a >= 2 relation",
    ),
    InvariantClassification(
        "check_axis_in_registry",
        TABLE_DRIVEN,
        "shape: for each Conflict, axis must resolve in axis_slugs(g)",
    ),
    InvariantClassification(
        "check_decided_has_nonempty_decided_by",
        TABLE_DRIVEN,
        "shape: for each DECIDED Conflict, decided_by field must be non-empty",
    ),
    InvariantClassification(
        "check_decided_by_is_known_stakeholder",
        TABLE_DRIVEN,
        "shape: for each DECIDED Conflict, decided_by must resolve in stakeholder_ids(g)",
    ),
    InvariantClassification(
        "check_decided_by_not_member_owner",
        TABLE_DRIVEN,
        "shape: for each DECIDED Conflict, decided_by must not be in the computed member-owner set",
    ),
    InvariantClassification(
        "check_held_has_min_two_variants",
        TABLE_DRIVEN,
        "shape: for each HELD Conflict, len(set(variant ids)) must satisfy a >= 2 relation",
    ),
    InvariantClassification(
        "check_held_has_nonempty_decided_by",
        TABLE_DRIVEN,
        "shape: for each HELD Conflict, decided_by field must be non-empty",
    ),
    InvariantClassification(
        "check_held_by_is_known_stakeholder",
        TABLE_DRIVEN,
        "shape: for each HELD Conflict, decided_by must resolve in stakeholder_ids(g)",
    ),
    InvariantClassification(
        "check_held_by_not_member_owner",
        TABLE_DRIVEN,
        "shape: for each HELD Conflict, decided_by must not be in the computed member-owner set",
    ),
    InvariantClassification(
        "check_typed_anchors_variant",
        TABLE_DRIVEN,
        "shape: for each Conflict, each Variant in variants[*] must have id starting with prefix 'V-'",
    ),
    # --- BESPOKE: identity derivation, cross-entity bijection, arithmetic,
    # filesystem walks, meta-coherence — each irreducibly its own shape ---
    InvariantClassification(
        "check_conflict_id_matches_identity",
        BESPOKE,
        "identity is a hash derivation (conflict_identity), not a set-membership test",
    ),
    InvariantClassification(
        "check_steward_not_a_member_owner",
        BESPOKE,
        "cross-references two different fields (steward vs owners-of-members) via a computed set",
    ),
    InvariantClassification(
        "check_constituting_not_in_unresolved_conflict",
        BESPOKE,
        "self-host-scoped pairwise-consistency: intersects each unresolved conflict's members with the SETTLED set, gated on g.self_hosting (R-domain-self-hosting-flag) — not a per-entity single-field lookup",
    ),
    InvariantClassification(
        "check_operator_steward_not_self",
        BESPOKE,
        "M36 reflexive cross-check across Conflict x Operator x Stakeholder, not a single-field lookup",
    ),
    InvariantClassification(
        "check_operator_within_budget",
        BESPOKE,
        "arithmetic branch on measure kind (NODE_COUNT vs CRYSTAL_CHARS) plus filesystem read",
    ),
    InvariantClassification(
        "check_scoped_node_has_single_presenter",
        BESPOKE,
        "pairwise scope-projection overlap computation across all Operator pairs, not a single-field lookup",
    ),
    InvariantClassification(
        "check_open_has_question",
        BESPOKE,
        "parses a mini-grammar (OPEN(question)) out of a free-text field",
    ),
    InvariantClassification(
        "check_decided_has_rationale_or_derived",
        BESPOKE,
        "parses DECIDED(rationale) grammar and considers an OR with the derived tuple",
    ),
    InvariantClassification(
        "check_m_tag_valid_format",
        BESPOKE,
        "regex format validation, not a registry-membership test",
    ),
    InvariantClassification(
        "check_m_tag_unique",
        BESPOKE,
        "cross-requirement uniqueness accumulator, not a per-row independent check",
    ),
    InvariantClassification(
        "check_m_tag_open_only",
        BESPOKE,
        "cross-field status/m_tag correlation, not single-field membership",
    ),
    InvariantClassification(
        "check_canonical_lifecycles_wellformed",
        BESPOKE,
        "structural self-check of the Lifecycle constants themselves (BFS reachability)",
    ),
    InvariantClassification(
        "check_bijection_r_to_enforcer",
        BESPOKE,
        "two-way bijection between Requirements and ALL_INVARIANTS, irreducible to one entity loop",
    ),
    InvariantClassification(
        "check_section_anchors_known",
        BESPOKE,
        "walks the filesystem + AST-parses docstrings; not a graph-field check",
    ),
    InvariantClassification(
        "check_assumption_machine_checks_syntactic",
        BESPOKE,
        "compiles a free-text formula in eval mode (syntax validity of an "
        "expression), not a registry-membership or single-field lookup",
    ),
    InvariantClassification(
        "check_method_matches_docstring",
        BESPOKE,
        "the meta-invariant that makes TABLE_DRIVEN-vs-BESPOKE distinguishable at all (inspects source)",
    ),
    InvariantClassification(
        "check_entities_md_lists_all_types",
        BESPOKE,
        "filesystem walk across all domains' generated docs, not a graph-field check",
    ),
    InvariantClassification(
        "check_entity_type_constitution_projection",
        BESPOKE,
        "filesystem walk across all domains' generated CONSTITUTION.md, not a graph-field check",
    ),
    InvariantClassification(
        "check_entity_instance_required_fields",
        BESPOKE,
        "iterates the EntityType's own field schema per instance, not a fixed field/registry pair",
    ),
    InvariantClassification(
        "check_entity_instance_refs_resolve",
        BESPOKE,
        "ref_target is itself data-driven (stakeholder/requirement/assumption/entity-type), a dispatch not a single set test",
    ),
    InvariantClassification(
        "check_entity_type_lifecycle_wellformed",
        BESPOKE,
        "delegates to check_lifecycle_wellformed's BFS, not a set-membership test",
    ),
    InvariantClassification(
        "check_transition_guard_assumption_resolves",
        TABLE_DRIVEN,
        "shape: for each EntityType.lifecycle.transitions[*], guard_assumption must resolve in assumption_ids(g)",
    ),
    InvariantClassification(
        "check_process_lifecycle_wellformed",
        BESPOKE,
        "delegates to check_lifecycle_wellformed's BFS, not a set-membership test",
    ),
    InvariantClassification(
        "check_process_roles_declared",
        BESPOKE,
        "cross-references Step.requires_role against Process.roles_required, a nested two-level walk",
    ),
    InvariantClassification(
        "check_process_drives_existing_entities",
        BESPOKE,
        "resolves against a dynamically-imported entity_type_slugs(g) helper, not a static registry",
    ),
    InvariantClassification(
        "check_step_invokes_known_transition",
        BESPOKE,
        "parses a '<slug>.<event>' mini-grammar and resolves against a nested lifecycle transition set",
    ),
    InvariantClassification(
        "check_enforced_names_invariant",
        BESPOKE,
        "two-condition branch (level validity, then conditional non-empty enforced_by)",
    ),
    InvariantClassification(
        "check_enforced_by_resolvable",
        BESPOKE,
        "greps tests/*.py to resolve each enforced_by entry via the shared "
        "hotam_spec.enforcer_resolution algorithm, not a static registry",
    ),
    InvariantClassification(
        "check_doc_reader_resolves_to_stakeholder",
        BESPOKE,
        "imports doc_readers plumbing and resolves a role-hint dispatch, not a static registry",
    ),
    InvariantClassification(
        "check_domain_manifest_exists_and_importable",
        BESPOKE,
        "filesystem walk + dynamic module import with exception handling",
    ),
    InvariantClassification(
        "check_domain_manifest_id_matches_dirname",
        BESPOKE,
        "filesystem walk + dynamic module import, field compared against dirname",
    ),
    InvariantClassification(
        "check_domain_manifest_description_nonempty",
        BESPOKE,
        "filesystem walk + dynamic module import",
    ),
    InvariantClassification(
        "check_domain_manifest_goals_nonempty",
        BESPOKE,
        "filesystem walk + dynamic module import",
    ),
    InvariantClassification(
        "check_domain_manifest_director_nonempty",
        BESPOKE,
        "filesystem walk + dynamic module import",
    ),
    InvariantClassification(
        "check_domain_director_exists",
        BESPOKE,
        "filesystem walk resolving a manifest field against a second directory's existence",
    ),
    InvariantClassification(
        "check_agent_has_agents_subdir",
        BESPOKE,
        "filesystem walk over the resolved agents root",
    ),
    InvariantClassification(
        "check_agent_has_docs_subdir",
        BESPOKE,
        "filesystem walk over the resolved agents root",
    ),
    InvariantClassification(
        "check_agent_has_tools_subdir",
        BESPOKE,
        "filesystem walk over the resolved agents root",
    ),
)


def check_rules_as_data_classification_coherent(g: TensionGraph) -> list[Violation]:  # noqa: ARG001
    """Canon: §Invariants — RULES_AS_DATA_TABLE and ALL_INVARIANTS name exactly the same functions.

    RULE (R-rules-as-data, M22 HYBRID): every function in ALL_INVARIANTS MUST
    have exactly one row in RULES_AS_DATA_TABLE (no missing classification, no
    duplicate row), and every row's `name` MUST resolve to a function in
    ALL_INVARIANTS (no stale row surviving a rename/removal). Every row's
    `kind` MUST be in RULES_AS_DATA_KINDS.

    NOTE (atomicity): one bijection checked via four sub-rules (facets of
    the SAME table<->registry bijection -- stale row, bad kind, duplicate
    row, unclassified function -- not four independent rules; the same
    shape as check_bijection_r_to_enforcer's documented two sub-rules, just
    with more facets of one relation): a row/function pair fails this ONE
    bijection for exactly one of those four reasons at a time, so the four
    Violation messages below are failure branches of a single check, not a
    bundled multi-rule function.

    WHY: the classification table is the DATA half of the HYBRID verdict — if
    it silently drifts out of sync with ALL_INVARIANTS (a check added without a
    row, or a row surviving a check's removal), the table stops being a
    trustworthy map an agent can read instead of re-deriving the classification
    by hand, defeating the whole point of declaring it as data.
    """
    out: list[Violation] = []
    inv_names = {fn.__name__ for fn in ALL_INVARIANTS}
    row_names_seen: dict[str, int] = {}
    for row in RULES_AS_DATA_TABLE:
        row_names_seen[row.name] = row_names_seen.get(row.name, 0) + 1
        if row.name not in inv_names:
            out.append(
                Violation(
                    "check_rules_as_data_classification_coherent",
                    row.name,
                    f"RULES_AS_DATA_TABLE names '{row.name}' which is not a "
                    f"function in ALL_INVARIANTS (stale classification row)",
                )
            )
        if row.kind not in RULES_AS_DATA_KINDS:
            out.append(
                Violation(
                    "check_rules_as_data_classification_coherent",
                    row.name,
                    f"RULES_AS_DATA_TABLE row for '{row.name}' has kind "
                    f"'{row.kind}' not in RULES_AS_DATA_KINDS {sorted(RULES_AS_DATA_KINDS)}",
                )
            )
    for name, count in row_names_seen.items():
        if count > 1:
            out.append(
                Violation(
                    "check_rules_as_data_classification_coherent",
                    name,
                    f"RULES_AS_DATA_TABLE names '{name}' {count} times "
                    f"(duplicate classification row)",
                )
            )
    classified = set(row_names_seen)
    for name in sorted(inv_names - classified):
        if name == "check_rules_as_data_classification_coherent":
            continue  # this check classifies its siblings, not itself
        out.append(
            Violation(
                "check_rules_as_data_classification_coherent",
                name,
                f"check_* '{name}' is in ALL_INVARIANTS but has no row in "
                f"RULES_AS_DATA_TABLE (unclassified — add a TABLE_DRIVEN or "
                f"BESPOKE row, R-rules-as-data)",
            )
        )
    return out


# ---------------------------------------------------------------------------
# Registry of all structural invariants (single source for tests + harness)
# ---------------------------------------------------------------------------

ALL_INVARIANTS = (
    # §Assumption — status enum-membership (admits the IMPLEMENTS род)
    check_assumption_status_valid,
    # §Referential integrity (atomized; check_no_dangling_ids is the thin delegator)
    check_no_dangling_assumption_owner,
    check_no_dangling_requirement_owner,
    check_no_dangling_requirement_assumptions,
    check_no_dangling_requirement_relations,
    check_no_dangling_conflict_refs,
    check_no_dangling_operator_refs,
    # §Requirement — generated-doc reader plumbing (R-doc-names-reader)
    check_doc_reader_resolves_to_stakeholder,
    # §Conflict connector node (atomized; check_conflict_has_axis_context_steward is thin delegator)
    check_conflict_has_axis,
    check_conflict_has_context,
    check_conflict_has_steward,
    check_conflict_min_two_members,
    check_axis_in_registry,
    check_conflict_id_matches_identity,
    # §Boundary
    check_steward_not_a_member_owner,
    # §Constituting-set convergence (self-host operator-prompt only)
    check_constituting_not_in_unresolved_conflict,
    # §Visibility of the open
    check_open_has_question,
    # §Anti-relitigation
    check_decided_has_rationale_or_derived,
    # §Signoff lock (atomized; check_decided_has_decided_by is thin delegator)
    check_decided_has_nonempty_decided_by,
    check_decided_by_is_known_stakeholder,
    check_decided_by_not_member_owner,
    # §HELD state + variants (atomized; check_held_has_decided_by is thin delegator)
    check_held_has_min_two_variants,
    check_held_has_nonempty_decided_by,
    check_held_by_is_known_stakeholder,
    check_held_by_not_member_owner,
    # §Typed anchors (atomized; check_typed_anchors is thin delegator)
    check_typed_anchors_requirement,
    check_typed_anchors_assumption,
    check_typed_anchors_conflict,
    check_typed_anchors_operator,
    check_typed_anchors_process,
    check_typed_anchors_goal,
    check_typed_anchors_variant,
    # §Enforcement gradient
    check_enforced_names_invariant,
    check_enforceability_kind_known,
    check_enforced_by_resolvable,
    # §M-tag (atomized; check_m_tag_format is thin delegator)
    check_m_tag_valid_format,
    check_m_tag_unique,
    check_m_tag_open_only,
    # §Lifecycle status (atomized; check_status_in_lifecycle is thin delegator)
    check_requirement_status_in_lifecycle,
    check_conflict_lifecycle_in_lifecycle,
    check_operator_lifecycle_in_lifecycle,
    check_goal_lifecycle_in_lifecycle,
    check_canonical_lifecycles_wellformed,
    # §Operator safety
    check_operator_steward_not_self,
    check_operator_within_budget,
    # §Scope — projection overlap (M18/R-partition-vs-border resolution)
    check_scoped_node_has_single_presenter,
    # §Process aspect invariants (aspect-gated: no-op when g.processes empty)
    check_process_lifecycle_wellformed,
    check_process_roles_declared,
    check_process_drives_existing_entities,
    check_step_invokes_known_transition,
    # §Goal aspect invariants (aspect-gated: no-op when g.goals empty)
    check_goal_target_kind_known,
    check_goal_owner_is_operator,
    # §Entity aspect invariants (aspect-gated: no-op when g.entity_types/entities empty)
    check_entity_type_lifecycle_wellformed,
    check_transition_guard_assumption_resolves,
    # §Assumption — machine_check well-formedness (syntax seam for layers 4/5)
    check_assumption_machine_checks_syntactic,
    check_entity_instance_state_in_lifecycle,
    check_entity_instance_required_fields,
    check_entity_instance_id_prefix,
    check_entity_instance_refs_resolve,
    check_entity_field_kind_known,
    check_typed_anchors_entity,
    check_entities_md_lists_all_types,
    check_entity_type_constitution_projection,
    check_section_anchors_known,
    check_bijection_r_to_enforcer,
    # §Domain + §Agent filesystem invariants (P17 task #64)
    check_domain_manifest_exists_and_importable,
    check_domain_manifest_id_matches_dirname,
    check_domain_manifest_description_nonempty,
    check_domain_manifest_goals_nonempty,
    check_domain_manifest_director_nonempty,
    check_domain_director_exists,
    check_agent_has_agents_subdir,
    check_agent_has_docs_subdir,
    check_agent_has_tools_subdir,
    # §Meta-invariant — docstring/body coherence
    check_method_matches_docstring,
    # §Invariants — M22 rules-as-data classification coherence
    check_rules_as_data_classification_coherent,
)

# ---------------------------------------------------------------------------
# Framework-scoped invariants (R-domain-self-hosting-flag, wave 7 move 1)
# ---------------------------------------------------------------------------
#
# These check_* functions carry FRAMEWORK jurisdiction, not business-domain
# jurisdiction: they inspect ALL_INVARIANTS itself (the bijection, docstring/
# body coherence, rules-as-data classification) or walk the ENTIRE domains/
# filesystem regardless of which domain is active (manifest + agent-scaffold
# checks). Run against an ordinary business domain (e.g. hotam-dev), they fire
# on framework internals that domain has no authority over and did not touch
# — phantom P1s. They are meaningful ONLY when g.self_hosting is True (the
# active domain IS hotam-spec-self, the domain that models the framework).
#
# check_enforced_by_resolvable is deliberately NOT here: every domain is
# responsible for its OWN enforced_by names resolving (that is business
# jurisdiction — each domain's SETTLED/ENFORCED requirements must point at
# real enforcers), so it stays universal.
FRAMEWORK_SCOPED_INVARIANTS = (
    check_bijection_r_to_enforcer,
    check_method_matches_docstring,
    check_rules_as_data_classification_coherent,
    check_domain_manifest_exists_and_importable,
    check_domain_manifest_id_matches_dirname,
    check_domain_manifest_description_nonempty,
    check_domain_manifest_goals_nonempty,
    check_domain_manifest_director_nonempty,
    check_domain_director_exists,
    check_agent_has_agents_subdir,
    check_agent_has_docs_subdir,
    check_agent_has_tools_subdir,
    check_constituting_not_in_unresolved_conflict,
)

# --- M7: the critical core ---
# These invariants guard paths by which contradictions could be INTRODUCED
# without being seen. They get the Hypothesis property-sweep treatment
# (test_conscience.py). Other invariants are still in ALL_INVARIANTS and tested
# normally; this constant marks the boundary.
#
# Canon: §Conscience — the critical core is the methodology's own hard boundary
# made narrow and machine-checkable (M7 resolved). Secondary-ring invariants
# (e.g. check_axis_in_registry, check_conflict_id_matches_identity) are still in
# ALL_INVARIANTS and receive the same Hypothesis machinery but at lower priority.
#
# NOTE: compound thin-delegators (check_no_dangling_ids, check_typed_anchors,
# check_decided_has_decided_by) are replaced here by their atomic sub-checks so
# the conscience boundary uses the smallest possible, independently addressable
# invariants.
CRITICAL_CORE_INVARIANTS = (
    check_steward_not_a_member_owner,
    check_operator_steward_not_self,
    # check_decided_has_decided_by atomized:
    check_decided_has_nonempty_decided_by,
    check_decided_by_is_known_stakeholder,
    check_decided_by_not_member_owner,
    # check_typed_anchors atomized:
    check_typed_anchors_requirement,
    check_typed_anchors_assumption,
    check_typed_anchors_conflict,
    check_typed_anchors_operator,
    check_typed_anchors_process,
    check_typed_anchors_goal,
    # check_no_dangling_ids atomized:
    check_no_dangling_assumption_owner,
    check_no_dangling_requirement_owner,
    check_no_dangling_requirement_assumptions,
    check_no_dangling_requirement_relations,
    check_no_dangling_conflict_refs,
    check_no_dangling_operator_refs,
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

    Canon: §Conscience — CRITICAL_CORE_INVARIANTS is the narrow set of invariants
    whose violation would silently break the hard boundary or anti-drift.
    The §Conscience Hypothesis sweep (test_conscience.py) runs property-tests over
    this boundary; all_violations runs the full set (both rings).

    Canon: §Domain — FRAMEWORK_SCOPED_INVARIANTS (bijection over ALL_INVARIANTS,
    docstring/body coherence, rules-as-data classification, domain+agent
    filesystem walks) run ONLY when g.self_hosting is True (R-domain-self-
    hosting-flag). An ordinary business domain has no jurisdiction over
    framework internals; running these against it produces phantom P1s that
    are not this domain's to fix.
    """
    key = id(g)
    cached = _ALL_VIOLATIONS_CACHE.get(key)
    if cached is not None and cached[0] is g:
        # Same live, frozen graph object within this process → reuse. Return a
        # fresh list so callers that mutate their copy cannot corrupt the cache.
        return list(cached[1])

    out: list[Violation] = []
    framework_scoped = frozenset(fn.__name__ for fn in FRAMEWORK_SCOPED_INVARIANTS)
    for check in ALL_INVARIANTS:
        if check.__name__ in framework_scoped and not g.self_hosting:
            continue
        out.extend(check(g))
    _ALL_VIOLATIONS_CACHE[key] = (g, out)
    return list(out)
