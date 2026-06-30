"""Canon: §Graph — the tension graph store and its traversal helpers.

The store IS the Python code (like dev-coin's params.py): a frozen TensionGraph
holding tuples of Axes, Stakeholders, Assumptions, Requirements and Conflicts;
edges are tuple-of-id fields on those objects; traversal is the plain functions
below. No database, no RDF — the graph instance the invariants, the generator
and the harness all read is the one assembled by the loader.

CONTENT-FREE FRAMEWORK: this module ships ZERO business data. Tensio is a blank
kit. Real domains populate `spec/content/graph.py` exposing `build_graph() ->
TensionGraph`; an empty `spec/content/` is the legitimate ship state ("no
content yet"). The example demo lives outside the framework in
`spec/tests/fixtures/seed.py` and is loaded only via the explicit `--demo` flag
of the tools or by the tests.

WHY traversal lives here as functions (not methods on a graph class doing logic):
keeps the ontology dataclasses pure data and the queries in one auditable place,
mirroring dev-coin where chain logic is module functions over frozen dataclasses.
"""

from __future__ import annotations

import importlib.util
import sys
from dataclasses import dataclass, field
from pathlib import Path

from tensio.assumption import DEAD, Assumption
from tensio.axis import Axis
from tensio.conflict import Conflict
from tensio.operator import Operator
from tensio.process import Goal, Process
from tensio.requirement import Requirement
from tensio.stakeholder import Stakeholder


@dataclass(frozen=True)
class TensionGraph:
    """Canon: §Graph — the whole requirement world as one frozen object.

    RULE: this is the single in-memory source of truth consumed by invariants,
    the generator and the harness. All collections are tuples (ordered,
    hashable, frozen); lookups go through the index helpers below.

    Fields:
      axes         — tuple of Axis (the controlled vocabulary for Conflict.axis).
      stakeholders — tuple of Stakeholder.
      assumptions  — tuple of Assumption.
      requirements — tuple of Requirement.
      conflicts    — tuple of Conflict.
      operators    — tuple of Operator (§Operator — the acting facets; M20).
      processes    — tuple of Process (§Process — opt-in behavioral aspect, M12).
      goals        — tuple of Goal (§Goal — first-class target-state type, M19).

    WHY frozen + tuples: determinism. The generator emits docs in graph order and
    the meta-test demands byte-for-byte stability; a mutable/unordered store would
    make regeneration non-deterministic and reintroduce drift.

    WHY axes live on the graph (not as a module constant): a domain owns its
    controlled vocabulary; the framework ships with none. Two domains may admit
    different axes, and the invariant `check_axis_in_registry` reads from this
    field so the per-domain vocabulary is the authority.

    WHY processes and goals default to empty tuple: they are opt-in aspects (M12).
    A domain that does not model processes pays nothing — is_empty() still works
    correctly with the extended set.
    """

    axes: tuple[Axis, ...] = field(default_factory=tuple)
    stakeholders: tuple[Stakeholder, ...] = field(default_factory=tuple)
    assumptions: tuple[Assumption, ...] = field(default_factory=tuple)
    requirements: tuple[Requirement, ...] = field(default_factory=tuple)
    conflicts: tuple[Conflict, ...] = field(default_factory=tuple)
    operators: tuple[Operator, ...] = field(default_factory=tuple)
    processes: tuple[Process, ...] = field(default_factory=tuple)
    goals: tuple[Goal, ...] = field(default_factory=tuple)

    def is_empty(self) -> bool:
        """Canon: §Graph — True iff no domain content has been loaded.

        RULE: empty iff every collection is empty. An empty graph is the
        legitimate ship state of the framework (no content under spec/content/).
        Includes the §Process and §Goal aspect collections.

        WHY: the harness and generator use this to render a calm "no content
        yet" message instead of an awkwardly empty roster.
        """
        return not (
            self.axes
            or self.stakeholders
            or self.assumptions
            or self.requirements
            or self.conflicts
            or self.operators
            or self.processes
            or self.goals
        )


# ---------------------------------------------------------------------------
# Content loader — discovers spec/content/graph.py:build_graph(), else empty
# ---------------------------------------------------------------------------

#: Path to the user-content slot. Empty in a freshly cloned framework.
# graph.py lives at spec/src/tensio/graph.py; parents[2] is `spec/`.
CONTENT_DIR = Path(__file__).resolve().parents[2] / "content"
CONTENT_GRAPH_FILE = CONTENT_DIR / "graph.py"
CONTENT_BUILDER_NAME = "build_graph"


def load_content_graph() -> TensionGraph:
    """Canon: §Graph — load the user's graph from spec/content/, else empty.

    RULE: import `spec/content/graph.py` and call its `build_graph()` if the file
    exists. If absent (a fresh framework with no domain populated yet), return an
    empty TensionGraph. Never raise just because nothing is populated yet —
    emptiness is a legitimate state.

    WHY a file-discovery loader (not a CLI arg / env var): the framework's
    "agent is never lost" property requires a deterministic location every tool
    agrees on. One slot, one convention; populating a domain is dropping a file.
    """
    if not CONTENT_GRAPH_FILE.exists():
        return TensionGraph()
    spec = importlib.util.spec_from_file_location(
        "tensio_user_content_graph", CONTENT_GRAPH_FILE
    )
    if spec is None or spec.loader is None:  # pragma: no cover — defensive
        return TensionGraph()
    module = importlib.util.module_from_spec(spec)
    # Make sure user content can `from tensio.* import …` cleanly.
    src_dir = str(Path(__file__).resolve().parents[1])
    if src_dir not in sys.path:
        sys.path.insert(0, src_dir)
    spec.loader.exec_module(module)
    builder = getattr(module, CONTENT_BUILDER_NAME, None)
    if builder is None:
        raise RuntimeError(
            f"{CONTENT_GRAPH_FILE} does not expose `{CONTENT_BUILDER_NAME}()`; "
            f"see CLAUDE.md §How to populate."
        )
    g = builder()
    if not isinstance(g, TensionGraph):
        raise TypeError(
            f"{CONTENT_GRAPH_FILE}:{CONTENT_BUILDER_NAME}() must return a "
            f"TensionGraph, got {type(g).__name__}"
        )
    return g


# ---------------------------------------------------------------------------
# Indexing / lookup
# ---------------------------------------------------------------------------


def axis_slugs(g: TensionGraph) -> frozenset[str]:
    """Canon: §Axis — the set of admitted axis slugs on this graph.

    RULE: this set is the authority used by invariants.check_axis_in_registry —
    a Conflict.axis must be one of these. Per-domain vocabulary, no global state.

    WHY a function over `g.axes`: keeps the membership test in one place so the
    invariant, the generator and any consumer agree on what "a known axis" means.
    """
    return frozenset(a.slug for a in g.axes)


def stakeholder_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Graph — set of all Stakeholder ids (for dangling-ref checks)."""
    return frozenset(s.id for s in g.stakeholders)


def assumption_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Graph — set of all Assumption ids (for dangling-ref checks)."""
    return frozenset(a.id for a in g.assumptions)


def requirement_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Graph — set of all Requirement ids (for dangling-ref checks)."""
    return frozenset(r.id for r in g.requirements)


def operator_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Operator — set of all Operator ids (for dangling-ref checks).

    RULE: used by check_no_dangling_ids to verify Operator.parent references
    and by check_operator_steward_not_self to identify operator acting facets.
    """
    return frozenset(op.id for op in g.operators)


def process_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Process — set of all Process ids in the graph.

    RULE: used by check_typed_anchors to verify PR- prefix discipline and by
    tests to confirm a named process is present. Empty when the §Process aspect
    is not loaded (opt-in, M12).
    """
    return frozenset(p.id for p in g.processes)


def goal_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Goal — set of all Goal ids in the graph.

    RULE: used by check_typed_anchors to verify GOAL- prefix discipline and by
    check_goal_owner_is_operator to resolve Goal.owner references. Empty when
    the §Goal aspect is not loaded (opt-in, M12/M19).
    """
    return frozenset(go.id for go in g.goals)


def requirement_by_id(g: TensionGraph, rid: str) -> Requirement | None:
    """Canon: §Graph — Requirement with id `rid`, or None.

    WHY None-returning (not raising): callers in the harness tolerate dangling
    ids on purpose — a dangling member is itself a diagnosable issue surfaced as
    a next-action, not a crash.
    """
    for r in g.requirements:
        if r.id == rid:
            return r
    return None


def assumption_by_id(g: TensionGraph, aid: str) -> Assumption | None:
    """Canon: §Graph — Assumption with id `aid`, or None (see requirement_by_id)."""
    for a in g.assumptions:
        if a.id == aid:
            return a
    return None


# ---------------------------------------------------------------------------
# Drift traversal — assumptions and their dependents
# ---------------------------------------------------------------------------


def requirements_on_assumption(g: TensionGraph, aid: str) -> tuple[Requirement, ...]:
    """Canon: §Graph — Requirements that rest on assumption `aid`.

    RULE: a requirement depends on `aid` iff aid in requirement.assumptions.
    Used to walk invisibility #2 (hidden dependency) and #3 (context drift).

    WHY: when `aid` dies, these are exactly the requirements whose ground moved.
    """
    return tuple(r for r in g.requirements if aid in r.assumptions)


def conflicts_on_assumption(g: TensionGraph, aid: str) -> tuple[Conflict, ...]:
    """Canon: §Graph — Conflicts whose shared_assumption is `aid`.

    RULE: these are the cluster that INHERITS drift from `aid`. When `aid` dies,
    the whole cluster under it must revive at once — one trigger, one cluster.
    """
    return tuple(c for c in g.conflicts if c.shared_assumption == aid)


def dead_assumptions(g: TensionGraph) -> tuple[Assumption, ...]:
    """Canon: §Graph — assumptions currently flipped to DEAD.

    RULE: every DEAD assumption with live dependents is fallout the harness MUST
    surface; a DEAD assumption is never silently dropped.
    """
    return tuple(a for a in g.assumptions if a.status == DEAD)


# ---------------------------------------------------------------------------
# Conflict traversal — clustering and latent connectors
# ---------------------------------------------------------------------------


def conflicts_by_axis(g: TensionGraph) -> dict[str, tuple[Conflict, ...]]:
    """Canon: §Conflict — conflicts grouped by axis (the CLUSTERS).

    RULE: a cluster = all conflicts sharing one axis. A cluster of size > 1 is an
    unresolved ARCHITECTURAL choice, not N local disputes — rendered as such in
    TENSIONS.md. Keys are emitted in first-seen graph order for determinism.

    WHY a node-graph reveals this and an edge-list cannot: clustering needs the
    axis to be a shared, normalized property of a node; edges between pairs carry
    no common axis to group on.
    """
    out: dict[str, list[Conflict]] = {}
    for c in g.conflicts:
        out.setdefault(c.axis, []).append(c)
    return {axis: tuple(cs) for axis, cs in out.items()}


def members_pair_set(g: TensionGraph) -> frozenset[frozenset[str]]:
    """Canon: §Conflict — the set of unordered member pairs already mediated.

    RULE: for every conflict, every pair of its members is "already connected".
    The latent-connector heuristic skips pairs already in this set (they have a
    C-node), hunting only pairs that SHOULD have one but don't.

    WHY pairs (not whole member tuples): a latent suspect is a 2-requirement
    tension; a conflict with 3 members already mediates all 3 of its pairs.
    """
    pairs: set[frozenset[str]] = set()
    for c in g.conflicts:
        ms = list(c.members)
        for i in range(len(ms)):
            for j in range(i + 1, len(ms)):
                pairs.add(frozenset({ms[i], ms[j]}))
    return frozenset(pairs)


@dataclass(frozen=True)
class LatentSuspect:
    """Canon: §Conflict — a requirement pair that SHOULD have a C-node but doesn't.

    RULE (HEURISTIC, not a proof): two SETTLED/OPEN requirements that share at
    least one assumption and are NOT already mediated by any conflict are flagged
    "for AI review". This is a stub for the deferred detector (spec-stack layer
    4/5); it is explicitly a suspicion, never an auto-materialized conflict.

    Fields:
      left, right — the two Requirement ids (sorted for stable identity).
      hint        — why they were flagged (the shared signal).

    WHY only a heuristic now: the real detector ("find the missing connector")
    needs Hypothesis-generated tuples / Z3 joint-violation models; both DEFERRED.
    The hard boundary holds — a suspect is presented to a human, never resolved.
    """

    left: str
    right: str
    hint: str


def latent_connector_suspects(g: TensionGraph) -> tuple[LatentSuspect, ...]:
    """Canon: §Conflict — HEURISTIC hunt for missing connector nodes.

    RULE: flag any pair of non-REJECTED requirements that (a) share >= 1
    assumption id and (b) are not already mediated by a conflict
    (members_pair_set). Output is sorted by (left, right) for determinism.

    WHY share-an-assumption is the cheap signal: two requirements interpreting one
    assumption differently is the commonest hidden root of a real conflict
    (Conflict.shared_assumption). It is a SUSPICION the AI/steward must judge —
    deliberately stronger than "violated invariant" because it points at the
    not-yet-recorded. Real semantic detection is deferred (see CLAUDE.md / ROADMAP).
    """
    from tensio.requirement import REJECTED  # noqa: PLC0415

    already = members_pair_set(g)
    reqs = [r for r in g.requirements if r.status != REJECTED]
    suspects: list[LatentSuspect] = []
    for i in range(len(reqs)):
        for j in range(i + 1, len(reqs)):
            a, b = reqs[i], reqs[j]
            shared = set(a.assumptions) & set(b.assumptions)
            if not shared:
                continue
            if frozenset({a.id, b.id}) in already:
                continue
            left, right = sorted((a.id, b.id))
            hint = "shares assumption(s): " + ", ".join(sorted(shared))
            suspects.append(LatentSuspect(left=left, right=right, hint=hint))
    suspects.sort(key=lambda s: (s.left, s.right))
    return tuple(suspects)
