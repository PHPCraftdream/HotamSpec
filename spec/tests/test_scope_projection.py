"""Canon: §Scope — tests for hotam_spec.scope_projection (design B: view, not copy).

Covers: project_scope (id-set derivation by prefix), scope_overlap (visible
intersection of two projections), presenter_for_node
(R-overlap-single-presenter), and check_scoped_node_has_single_presenter
(invariants.py).
"""

from __future__ import annotations

from hotam_spec.axis import Axis
from hotam_spec.conflict import Conflict
from hotam_spec.graph import TensionGraph
from hotam_spec.invariants import check_scoped_node_has_single_presenter
from hotam_spec.operator import ContextBudget, Operator
from hotam_spec.requirement import Requirement
from hotam_spec.scope_projection import (
    ScopeOverlap,
    ScopeView,
    overlap_node_ids,
    presenter_for_node,
    project_scope,
    scope_overlap,
)
from hotam_spec.stakeholder import Stakeholder


def _mini_graph() -> TensionGraph:
    stakeholders = (
        Stakeholder(id="s-a", name="A", domain="d"),
        Stakeholder(id="s-b", name="B", domain="d"),
    )
    reqs = (
        Requirement(
            id="R-entity-alpha",
            claim="alpha claim",
            owner="s-a",
            status="SETTLED",
            why="w",
            assumptions=("A-shared",),
        ),
        Requirement(
            id="R-entity-beta",
            claim="beta claim",
            owner="s-a",
            status="SETTLED",
            why="w",
        ),
        Requirement(
            id="R-agent-gamma",
            claim="gamma claim",
            owner="s-b",
            status="SETTLED",
            why="w",
            assumptions=("A-shared",),
        ),
        Requirement(
            id="R-other-delta",
            claim="delta claim",
            owner="s-b",
            status="SETTLED",
            why="w",
        ),
    )
    conflicts = (
        Conflict(
            id="C-entity-tension",
            axis="ax-1",
            context="entity vs agent",
            steward="s-b",
            members=("R-entity-alpha", "R-agent-gamma"),
            lifecycle="DETECTED",
            shared_assumption="A-shared",
        ),
    )
    axes = (Axis(slug="ax-1", description="d"),)
    return TensionGraph(
        axes=axes, stakeholders=stakeholders, requirements=reqs, conflicts=conflicts
    )


def test_project_scope_selects_by_prefix():
    g = _mini_graph()
    # Conflict ids use the 'C-' typed anchor, not 'R-'; a scope must name both
    # prefixes to pull in the conflict node (matches gen_spec's own
    # id.startswith(p)-for-any-p discipline — no implicit cross-prefix pull).
    view = project_scope(g, ("R-entity-", "C-entity-"))
    assert view.requirement_ids == ("R-entity-alpha", "R-entity-beta")
    assert view.conflict_ids == ("C-entity-tension",)
    assert view.assumption_ids == ("A-shared",)
    assert view.axes == ("ax-1",)


def test_project_scope_requirement_only_prefix_excludes_conflict():
    """A scope naming only an 'R-' prefix does NOT pull in a 'C-' conflict
    node, even if that conflict's members are in scope — conflict inclusion
    is its OWN id-prefix match, not derived from member membership."""
    g = _mini_graph()
    view = project_scope(g, ("R-entity-",))
    assert view.requirement_ids == ("R-entity-alpha", "R-entity-beta")
    assert view.conflict_ids == ()
    # axes/assumptions are still derived from the matched REQUIREMENTS' own
    # .assumptions field, independent of whether the conflict itself matched.
    assert view.assumption_ids == ("A-shared",)


def test_project_scope_empty_prefixes_is_empty():
    g = _mini_graph()
    view = project_scope(g, ())
    assert view.is_empty()
    assert view.requirement_ids == ()
    assert view.conflict_ids == ()


def test_project_scope_never_matches_unrelated_prefix():
    g = _mini_graph()
    view = project_scope(g, ("R-nonexistent-",))
    assert view.is_empty()


def test_scope_overlap_finds_shared_conflict_and_assumption():
    g = _mini_graph()
    entity_view = project_scope(g, ("R-entity-", "C-entity-"))
    agent_view = project_scope(g, ("R-agent-", "C-entity-"))
    overlap = scope_overlap(entity_view, agent_view)
    assert overlap.conflict_ids == ("C-entity-tension",)
    assert overlap.assumption_ids == ("A-shared",)
    assert overlap.axes == ("ax-1",)
    # requirement id-sets themselves are disjoint (alpha vs gamma)
    assert overlap.requirement_ids == ()
    assert not overlap.is_empty()


def test_scope_overlap_disjoint_scopes_is_empty():
    g = _mini_graph()
    entity_view = project_scope(g, ("R-entity-",))
    other_view = project_scope(g, ("R-other-",))
    overlap = scope_overlap(entity_view, other_view)
    assert overlap.is_empty()
    assert overlap == ScopeOverlap()


def test_overlap_node_ids_is_union_of_requirement_and_conflict_overlap():
    overlap = ScopeOverlap(
        requirement_ids=("R-b", "R-a"),
        conflict_ids=("C-x",),
    )
    assert overlap_node_ids(overlap) == ("C-x", "R-a", "R-b")


def test_presenter_for_node_picks_lexicographically_first():
    assert presenter_for_node("R-x", ("OP-zeta", "OP-alpha")) == "OP-alpha"


def test_presenter_for_node_empty_operator_set_is_none():
    assert presenter_for_node("R-x", ()) is None


def test_scope_view_matches_gen_spec_prefix_rule_directly():
    """project_scope must agree with the raw `id.startswith(p)` rule
    tools/gen_spec.py::_render_scoped_constitution_block already applies."""
    g = _mini_graph()
    prefixes = ("R-entity-", "R-agent-")
    view = project_scope(g, prefixes)
    expected = tuple(
        sorted(r.id for r in g.requirements if any(r.id.startswith(p) for p in prefixes))
    )
    assert view.requirement_ids == expected


# ---------------------------------------------------------------------------
# check_scoped_node_has_single_presenter (invariants.py)
# ---------------------------------------------------------------------------


def _graph_with_two_operators() -> TensionGraph:
    g = _mini_graph()
    operators = (
        Operator(
            id="OP-zeta",
            stakeholder="s-a",
            context_budget=ContextBudget(limit=0),
        ),
        Operator(
            id="OP-alpha",
            stakeholder="s-b",
            context_budget=ContextBudget(limit=0),
        ),
    )
    return TensionGraph(
        axes=g.axes,
        stakeholders=g.stakeholders,
        requirements=g.requirements,
        conflicts=g.conflicts,
        operators=operators,
    )


def test_single_operator_graph_has_no_violations():
    g = _mini_graph()
    operators = (
        Operator(id="OP-solo", stakeholder="s-a", context_budget=ContextBudget(limit=0)),
    )
    g2 = TensionGraph(
        axes=g.axes,
        stakeholders=g.stakeholders,
        requirements=g.requirements,
        conflicts=g.conflicts,
        operators=operators,
    )
    # No SCOPE metadata is attached to Operator (deferred), so with the
    # generic check there is nothing to contest with a single operator.
    assert check_scoped_node_has_single_presenter(g2) == []


def test_two_operators_no_declared_scope_no_violation():
    """Operators with scope=() (the default / current meta-domain state) have
    nothing to project, so there is nothing to overlap — calm-empty case."""
    g = _graph_with_two_operators()
    assert check_scoped_node_has_single_presenter(g) == []


def test_two_operators_overlapping_scope_resolves_to_one_presenter():
    """Two operators whose declared scope prefixes overlap on a real
    Conflict/assumption/axis must still pass — presenter_for_node is total,
    so an empty Violation list PROVES single-presentership held, not that
    nothing was checked (see docstring on check_scoped_node_has_single_
    presenter: it only ever fires if presenter_for_node returned None, which
    cannot happen for a non-empty operator-id set)."""
    g = _mini_graph()
    operators = (
        Operator(
            id="OP-zeta",
            stakeholder="s-a",
            context_budget=ContextBudget(limit=0),
            scope=("R-entity-", "C-entity-"),
        ),
        Operator(
            id="OP-alpha",
            stakeholder="s-b",
            context_budget=ContextBudget(limit=0),
            scope=("R-agent-", "C-entity-"),
        ),
    )
    g2 = TensionGraph(
        axes=g.axes,
        stakeholders=g.stakeholders,
        requirements=g.requirements,
        conflicts=g.conflicts,
        operators=operators,
    )
    # Sanity: these two scopes DO overlap on C-entity-tension.
    view_zeta = project_scope(g2, operators[0].scope)
    view_alpha = project_scope(g2, operators[1].scope)
    overlap = scope_overlap(view_zeta, view_alpha)
    assert overlap.conflict_ids == ("C-entity-tension",)
    assert presenter_for_node("C-entity-tension", ("OP-zeta", "OP-alpha")) == "OP-alpha"
    # The invariant itself must hold (single presenter is always resolvable).
    assert check_scoped_node_has_single_presenter(g2) == []
