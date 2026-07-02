"""Canon: §Scope — an operator's sub-domain as a PROJECTION, not a copy (design B).

M18 (R-partition-vs-border) asked: do operator sub-domains strictly PARTITION
the parent graph, or OVERLAP on explicitly-declared delegation borders? This
module answers: neither extreme — a Scope is a VIEW (a filtered set of ids)
computed from the single shared TensionGraph by prefix match on typed anchors
(the same discipline tools/gen_spec.py already uses for
`_render_scoped_constitution_block`'s per-agent CONSTITUTION digest). No node
is ever copied into a sub-operator's own storage; two operators' Scopes may
name the SAME Requirement/Conflict/Assumption id, and that overlap is not an
error — it is rendered visibly (scope_overlap + the generator's OVERLAP block)
rather than hidden by a hard partition.

WHY prefix-projection over the graph, not a copied sub-graph: a copy forks —
the moment two operators each hold their OWN Requirement objects, editing one
cannot be guaranteed to reach the other, and R-no-hand-edit-graph's single
writer (apply_proposal.py, single domains/*/graph.py) is defeated. A Scope
computed as id-sets over the one graph can never drift from it: re-run the
projection, get the current view, always.

WHY id-sets (not object references): the ontology's other traversal helpers
(hotam_spec.graph: requirements_on_assumption, conflicts_by_axis, ...) all
return tuples of typed dataclasses or id-sets over the ONE TensionGraph; a
Scope follows the same shape so it composes with them (e.g. scope_overlap
below reuses ScopeView.requirement_ids the same way check_no_dangling_ids
reuses graph.requirement_ids()).
"""

from __future__ import annotations

from dataclasses import dataclass, field

from hotam_spec.graph import TensionGraph


@dataclass(frozen=True)
class ScopeView(object):
    """Canon: §Scope — the materialized projection of a prefix-tuple over a graph.

    RULE: given `prefixes` (a tuple of id-prefix strings, e.g.
    ("R-entity-", "R-agent-") — the same shape as an agent's scope.py SCOPE
    tuple), a ScopeView holds exactly the ids of Requirement / Conflict /
    Assumption objects in the graph whose `id` starts with at least one
    prefix, plus the axes referenced by any included Conflict and the
    assumptions referenced by any included Requirement or Conflict
    (shared_assumption). All fields are tuples, SORTED (ascending, plain
    string order) for deterministic byte-stable generation — NOT graph
    insertion order, because two different prefix tuples over the same graph
    must be independently reproducible without carrying graph-iteration state.

    WHY axes/assumptions are DERIVED (not filtered by prefix): axis slugs and
    assumption ids do not carry an 'R-' style typed-anchor prefix under this
    scope's own vocabulary — they are pulled in by REFERENCE from whichever
    Requirements/Conflicts the prefix already selected, mirroring how
    requirements_on_assumption / conflicts_on_assumption (hotam_spec.graph)
    already derive membership from a shared field rather than from an id
    pattern.

    Fields:
      prefixes         — the defining id-prefix tuple (as given; not sorted —
                          this is the SCOPE the caller declared, order is
                          caller intent, not a derived set).
      requirement_ids   — sorted tuple of matching Requirement.id.
      conflict_ids      — sorted tuple of matching Conflict.id.
      assumption_ids    — sorted tuple of Assumption.id referenced by any
                          matched Requirement.assumptions or matched
                          Conflict.shared_assumption.
      axes              — sorted tuple of Axis.slug referenced by any matched
                          Conflict.axis.
    """

    prefixes: tuple[str, ...]
    requirement_ids: tuple[str, ...] = field(default_factory=tuple)
    conflict_ids: tuple[str, ...] = field(default_factory=tuple)
    assumption_ids: tuple[str, ...] = field(default_factory=tuple)
    axes: tuple[str, ...] = field(default_factory=tuple)

    def is_empty(self) -> bool:
        """Canon: §Scope — True iff the projection selected nothing.

        WHY: an empty projection (SCOPE=() or no ids matching) is the
        legitimate "not-yet-delegated" state (mirrors
        R-empty-content-wellformed's calm-empty discipline) — never an error.
        """
        return not (
            self.requirement_ids or self.conflict_ids or self.assumption_ids
        )


def project_scope(g: TensionGraph, prefixes: tuple[str, ...]) -> ScopeView:
    """Canon: §Scope — compute the ScopeView for `prefixes` over graph `g`.

    RULE: NEVER copies a node — walks g.requirements/g.conflicts once each,
    keeps only ids, matching the exact prefix-match discipline
    tools/gen_spec.py::_render_scoped_constitution_block already uses for
    per-agent CONSTITUTION digests (id.startswith(p) for any p in prefixes).
    An empty `prefixes` tuple yields an empty ScopeView (matches
    gen_spec.py's existing "scope=() -> no atoms in scope" behavior).

    WHY reusing gen_spec's exact prefix rule (not a new matching scheme): two
    independently-invented 'is this id in scope' rules would silently
    diverge; the generator's CONSTITUTION-digest projection and this
    graph-level projection must always agree on membership, so both read the
    same `id.startswith(p) for p in prefixes` test.
    """
    if not prefixes:
        return ScopeView(prefixes=prefixes)

    req_ids = sorted(
        r.id for r in g.requirements if any(r.id.startswith(p) for p in prefixes)
    )
    conflict_ids = sorted(
        c.id for c in g.conflicts if any(c.id.startswith(p) for p in prefixes)
    )
    req_by_id = {r.id: r for r in g.requirements}
    conflict_by_id = {c.id: c for c in g.conflicts}

    assumption_set: set[str] = set()
    for rid in req_ids:
        assumption_set.update(req_by_id[rid].assumptions)
    axis_set: set[str] = set()
    for cid in conflict_ids:
        c = conflict_by_id[cid]
        axis_set.add(c.axis)
        if c.shared_assumption:
            assumption_set.add(c.shared_assumption)

    return ScopeView(
        prefixes=prefixes,
        requirement_ids=tuple(req_ids),
        conflict_ids=tuple(conflict_ids),
        assumption_ids=tuple(sorted(assumption_set)),
        axes=tuple(sorted(axis_set)),
    )


@dataclass(frozen=True)
class ScopeOverlap(object):
    """Canon: §Scope — the shared slice between two ScopeViews (R-scope-overlap-generated).

    RULE: every field is the SORTED intersection of the two ScopeViews' same-
    named field. An overlap with all-empty fields ("no shared ids") is
    legitimate and rendered as such, never suppressed — R-empty-content-
    wellformed's calm-empty discipline applies here too (an empty OVERLAP
    block is the correct output when exactly one operator's scope is active,
    which is the CURRENT meta-domain state: one OP-director, SCOPE=()).

    Fields mirror ScopeView: requirement_ids, conflict_ids, assumption_ids,
    axes — each the intersection, sorted.
    """

    requirement_ids: tuple[str, ...] = field(default_factory=tuple)
    conflict_ids: tuple[str, ...] = field(default_factory=tuple)
    assumption_ids: tuple[str, ...] = field(default_factory=tuple)
    axes: tuple[str, ...] = field(default_factory=tuple)

    def is_empty(self) -> bool:
        """Canon: §Scope — True iff the two scopes share nothing."""
        return not (
            self.requirement_ids
            or self.conflict_ids
            or self.assumption_ids
            or self.axes
        )


def scope_overlap(a: ScopeView, b: ScopeView) -> ScopeOverlap:
    """Canon: §Scope — the visible intersection of two operators' projections.

    RULE: pure set-intersection per field, sorted for determinism
    (R-scope-overlap-generated: an overlap is PRINTED, never hidden — the
    generator renders this into every affected agent's crystal, including the
    legitimate empty case when only one operator's scope is active).

    WHY a pure two-arg function (not graph-wide all-pairs by default): callers
    (tools/gen_spec.py) drive the all-pairs walk over g.operators themselves,
    the same shape gen_spec already uses to iterate agent scope.py files one
    at a time; keeping this function two-arg keeps it trivially testable on a
    synthetic pair of ScopeViews without needing a full graph+agents fixture.
    """
    return ScopeOverlap(
        requirement_ids=tuple(
            sorted(set(a.requirement_ids) & set(b.requirement_ids))
        ),
        conflict_ids=tuple(sorted(set(a.conflict_ids) & set(b.conflict_ids))),
        assumption_ids=tuple(
            sorted(set(a.assumption_ids) & set(b.assumption_ids))
        ),
        axes=tuple(sorted(set(a.axes) & set(b.axes))),
    )


def overlap_node_ids(overlap: ScopeOverlap) -> tuple[str, ...]:
    """Canon: §Scope — every node id present in an overlap (requirements + conflicts).

    RULE: sorted union of overlap.requirement_ids and overlap.conflict_ids.
    Assumptions and axes are NOT node ids under R-overlap-single-presenter
    (that invariant is about who PRESENTS a contested Requirement/Conflict
    node, not a shared axis or assumption reference) — kept out of this
    helper's output on purpose so callers computing "which node needs a
    presenter" don't have to filter axis/assumption strings back out.

    WHY a helper (not inlined at each call site): both
    invariants.check_scoped_node_has_single_presenter and any future overlap
    renderer need exactly this node-id union; one function keeps the
    definition of "a contested node" singular.
    """
    return tuple(sorted(set(overlap.requirement_ids) | set(overlap.conflict_ids)))


def presenter_for_node(node_id: str, operator_ids: tuple[str, ...]) -> str | None:
    """Canon: §Scope — the single deterministic presenter for a contested node.

    RULE (R-overlap-single-presenter): given the set of Operator ids whose
    scopes both contain `node_id`, the presenter is the LEXICOGRAPHICALLY
    FIRST operator id (plain ascending string sort of `operator_ids`, e.g.
    "OP-director" < "OP-entity-agent"). Returns None if `operator_ids` is
    empty (no contest — nothing to present).

    WHY lexicographic-first-id over graph-declaration-order: graph order
    depends on WHERE in domains/*/graph.py the Operator(...) literal happens
    to be written — an accidental reordering during an unrelated edit would
    silently reassign presentership. The operator's `id` string is a stable,
    typed anchor (R-anchor-everything) that does not move when the graph
    source is reformatted, so sorting by id is the more robust deterministic
    tie-break and needs no auxiliary "graph position" bookkeeping.
    """
    if not operator_ids:
        return None
    return min(operator_ids)
