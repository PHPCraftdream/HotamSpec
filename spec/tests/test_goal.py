"""Tests for the §Goal first-class type (P9, M19).

Covers: GOAL-burn-down-zero presence; owner-is-operator invariant;
target-kind-known invariant; lifecycle-valid invariant; typed-anchor enforcement.
"""

from __future__ import annotations

import sys
from pathlib import Path

# Make hotam_spec importable.
_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from hotam_spec.graph import load_content_graph, operator_ids  # noqa: E402
from hotam_spec.invariants import (  # noqa: E402
    check_goal_owner_is_operator,
    check_goal_target_kind_known,
    check_status_in_lifecycle,
    check_typed_anchors,
)
from hotam_spec.process import (
    TARGET_KINDS,
    Goal,
    TargetState,
    TARGET_KIND_GRAPH_PROPERTY,
)  # noqa: E402


def test_goal_burn_down_present() -> None:
    """GOAL-burn-down-zero exists in load_content_graph().goals."""
    g = load_content_graph()
    gids = {go.id for go in g.goals}
    assert "GOAL-burn-down-zero" in gids, (
        "GOAL-burn-down-zero not found in g.goals; check content/graph.py goals tuple"
    )


def test_goal_owner_is_operator() -> None:
    """Every Goal.owner in the real graph resolves to a known Operator.id."""
    g = load_content_graph()
    oids = operator_ids(g)
    for go in g.goals:
        assert go.owner in oids, (
            f"Goal '{go.id}' owner '{go.owner}' is not a known Operator"
        )
    # Also verify via the invariant
    viols = check_goal_owner_is_operator(g)
    assert viols == [], f"check_goal_owner_is_operator fired: {viols}"


def test_goal_target_kind_known() -> None:
    """Every TargetState.kind in the real graph is in TARGET_KINDS."""
    g = load_content_graph()
    for go in g.goals:
        assert go.target_state.kind in TARGET_KINDS, (
            f"Goal '{go.id}' target_state.kind '{go.target_state.kind}' "
            f"not in TARGET_KINDS {TARGET_KINDS}"
        )
    # Also via the invariant
    viols = check_goal_target_kind_known(g)
    assert viols == [], f"check_goal_target_kind_known fired: {viols}"


def test_goal_lifecycle_valid() -> None:
    """Every Goal.lifecycle in the real graph matches GOAL_LIFECYCLE."""
    g = load_content_graph()
    viols = check_status_in_lifecycle(g)
    goal_viols = [v for v in viols if any(go.id == v.target for go in g.goals)]
    assert goal_viols == [], f"Goal lifecycle invariant fired: {goal_viols}"


def test_goal_typed_anchor() -> None:
    """All Goal.id values in the real graph start with 'GOAL-'."""
    g = load_content_graph()
    for go in g.goals:
        assert go.id.startswith("GOAL-"), (
            f"Goal id '{go.id}' does not start with 'GOAL-'"
        )


def test_check_typed_anchors_fires_on_bad_goal_id() -> None:
    """check_typed_anchors fires when a Goal id lacks the GOAL- prefix."""
    from hotam_spec.graph import TensionGraph  # noqa: PLC0415

    bad = Goal(
        id="G-bad",
        owner="OP-director",
        target_state=TargetState(
            kind=TARGET_KIND_GRAPH_PROPERTY,
            predicate="test predicate",
        ),
    )
    g = TensionGraph(goals=(bad,))
    viols = check_typed_anchors(g)
    targets = {v.target for v in viols}
    assert "G-bad" in targets


def test_check_goal_target_kind_fires_on_unknown_kind() -> None:
    """check_goal_target_kind_known fires on an unknown kind."""
    from hotam_spec.graph import TensionGraph  # noqa: PLC0415

    bad = Goal(
        id="GOAL-bad",
        owner="OP-director",
        target_state=TargetState(
            kind="MADE_UP_KIND",
            predicate="some predicate",
        ),
    )
    g = TensionGraph(goals=(bad,))
    viols = check_goal_target_kind_known(g)
    assert len(viols) == 1
    assert viols[0].target == "GOAL-bad"
    assert "MADE_UP_KIND" in viols[0].message


def test_check_goal_owner_fires_on_unknown_operator() -> None:
    """check_goal_owner_is_operator fires when owner is not a known Operator."""
    from hotam_spec.graph import TensionGraph  # noqa: PLC0415

    bad = Goal(
        id="GOAL-orphan",
        owner="OP-nonexistent",
        target_state=TargetState(
            kind=TARGET_KIND_GRAPH_PROPERTY,
            predicate="test",
        ),
    )
    g = TensionGraph(goals=(bad,))
    viols = check_goal_owner_is_operator(g)
    assert len(viols) == 1
    assert viols[0].target == "GOAL-orphan"
    assert "OP-nonexistent" in viols[0].message


def test_goal_aspect_noop_on_empty_goals() -> None:
    """Both goal invariants are no-ops when g.goals is empty."""
    from hotam_spec.graph import TensionGraph  # noqa: PLC0415

    g = TensionGraph()  # empty; no goals
    assert check_goal_target_kind_known(g) == []
    assert check_goal_owner_is_operator(g) == []
