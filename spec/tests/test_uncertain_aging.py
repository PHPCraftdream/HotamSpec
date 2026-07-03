"""Tests for the UNCERTAIN-aging harness band (R-uncertain-assumptions-surface).

An UNCERTAIN assumption that many Requirements rest on is the graph's largest
silent question. what_now.diagnose must surface it as ONE P4 OPEN_ITEM action
per such assumption once its dependent count reaches
UNCERTAIN_AGING_MIN_DEPENDENTS.
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import what_now  # noqa: E402
from hotam_spec.assumption import HOLDS, UNCERTAIN, Assumption  # noqa: E402
from hotam_spec.graph import TensionGraph, uncertain_assumptions  # noqa: E402
from hotam_spec.requirement import Requirement  # noqa: E402


def _req(rid: str, aid: str) -> Requirement:
    return Requirement(
        id=rid,
        claim=f"claim {rid}",
        owner="framework-author",
        status="SETTLED",
        why="w",
        assumptions=(aid,),
    )


def _graph(assumption_status: str, n_deps: int) -> TensionGraph:
    a = Assumption(
        id="A-doubted",
        statement="A premise under question.",
        status=assumption_status,
        owner="framework-author",
    )
    reqs = tuple(_req(f"R-{i}", "A-doubted") for i in range(n_deps))
    return TensionGraph(assumptions=(a,), requirements=reqs)


def _aging_actions(g: TensionGraph):
    return [
        a
        for a in what_now.diagnose(g)
        if a.target == "A-doubted" and "UNCERTAIN" in a.imperative
    ]


def test_uncertain_assumptions_filter() -> None:
    g = _graph(UNCERTAIN, 3)
    assert {a.id for a in uncertain_assumptions(g)} == {"A-doubted"}
    # A HOLDS assumption is not surfaced by the filter.
    g2 = _graph(HOLDS, 10)
    assert uncertain_assumptions(g2) == ()


def test_below_threshold_is_silent() -> None:
    g = _graph(UNCERTAIN, what_now.UNCERTAIN_AGING_MIN_DEPENDENTS - 1)
    assert _aging_actions(g) == []


def test_at_threshold_surfaces_one_p4_action() -> None:
    g = _graph(UNCERTAIN, what_now.UNCERTAIN_AGING_MIN_DEPENDENTS)
    actions = _aging_actions(g)
    assert len(actions) == 1
    assert actions[0].priority == what_now.P_OPEN_ITEM
    assert actions[0].kind == "OPEN_ITEM"
    assert "5" not in actions[0].target  # target is the assumption id, not a count


def test_holds_assumption_never_ages() -> None:
    g = _graph(HOLDS, 100)
    assert _aging_actions(g) == []


def test_real_graph_surfaces_three_uncertain_assumptions() -> None:
    from hotam_spec.graph import load_content_graph

    g = load_content_graph()
    aging = [
        a
        for a in what_now.diagnose(g)
        if a.target.startswith("A-")
        and "still UNCERTAIN" in a.imperative
        and a.priority == what_now.P_OPEN_ITEM
    ]
    # The three real UNCERTAIN assumptions (deps 58/37/9) all clear K=5.
    assert {a.target for a in aging} == {
        "A-bootstrap-self-applies",
        "A-most-knowledge-crystallizable",
        "A-prose-suffices",
    }
