"""Tests for hotam_spec.operator + the Operator layer in the meta-domain graph.

Two duties:
  1. Live meta-domain assertions: OP-director is present, typed-anchors hold,
     stakeholder refs resolve, lifecycle values are valid, budget is within
     the live node count.
  2. Anti-phantom guards: each new invariant FIRES on a deliberately-broken
     fixture and stays green on well-formed data.
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TESTS = Path(__file__).resolve().parent
for _p in (_SRC, _TESTS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

from fixtures.seed import DEMO_AXES  # noqa: E402
from hotam_spec.assumption import HOLDS, Assumption  # noqa: E402
from hotam_spec.conflict import Conflict, conflict_identity  # noqa: E402
from hotam_spec.graph import TensionGraph, load_content_graph, stakeholder_ids  # noqa: E402
from hotam_spec.invariants import (  # noqa: E402
    check_operator_steward_not_self,
    check_operator_within_budget,
    holds,
)
from hotam_spec.operator import (  # noqa: E402
    OPERATOR_LIFECYCLE,
    ContextBudget,
    Operator,
)
from hotam_spec.requirement import Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

_S_AUTHOR = Stakeholder(id="framework-author", name="Author", domain="framework")
_S_REVIEWER = Stakeholder(id="framework-reviewer", name="Reviewer", domain="review")
_S_AGENT = Stakeholder(id="ai-agent", name="Agent", domain="agent")


def _req(rid: str, owner: str, status: str = "SETTLED") -> Requirement:
    return Requirement(id=rid, claim=f"claim {rid}", owner=owner, status=status)


def _simple_graph_with_operator(op: Operator) -> TensionGraph:
    """Minimal well-formed graph holding one Operator."""
    return TensionGraph(
        axes=(),
        stakeholders=(_S_AUTHOR, _S_REVIEWER),
        assumptions=(),
        requirements=(),
        conflicts=(),
        operators=(op,),
    )


# ---------------------------------------------------------------------------
# 1. Live meta-domain assertions
# ---------------------------------------------------------------------------


def test_director_operator_present() -> None:
    """OP-director exists in load_content_graph().operators."""
    g = load_content_graph()
    ids = {op.id for op in g.operators}
    assert "OP-director" in ids, (
        "OP-director must be instantiated in content/graph.py:build_graph()"
    )


def test_operator_typed_anchor() -> None:
    """Every Operator.id in the live graph starts with 'OP-'."""
    g = load_content_graph()
    for op in g.operators:
        assert op.id.startswith("OP-"), (
            f"Operator id '{op.id}' must start with 'OP-' (typed-anchor rule)"
        )


def test_operator_stakeholder_resolves() -> None:
    """Every Operator.stakeholder in the live graph resolves to a known Stakeholder."""
    g = load_content_graph()
    sids = stakeholder_ids(g)
    for op in g.operators:
        assert op.stakeholder in sids, (
            f"Operator '{op.id}' references unknown stakeholder '{op.stakeholder}'"
        )


def test_operator_lifecycle_valid() -> None:
    """Every Operator.lifecycle in the live graph matches OPERATOR_LIFECYCLE."""
    g = load_content_graph()
    for op in g.operators:
        assert OPERATOR_LIFECYCLE.matches(op.lifecycle) is not None, (
            f"Operator '{op.id}' has invalid lifecycle '{op.lifecycle}'; "
            f"must be one of {sorted(OPERATOR_LIFECYCLE.state_names())}"
        )


def test_director_within_budget() -> None:
    """OP-director's budget (200) exceeds the total live node count."""
    g = load_content_graph()
    live_nodes = len(g.requirements) + len(g.conflicts) + len(g.assumptions)
    director = next((op for op in g.operators if op.id == "OP-director"), None)
    assert director is not None, "OP-director not found in live graph"
    assert director.context_budget.limit > live_nodes, (
        f"OP-director budget {director.context_budget.limit} is not above "
        f"live node count {live_nodes}; increase the budget in content/graph.py"
    )


# ---------------------------------------------------------------------------
# 2. check_operator_steward_not_self — anti-phantom (fires + stays green)
# ---------------------------------------------------------------------------


def test_check_operator_steward_not_self_fires() -> None:
    """check_operator_steward_not_self fires when Operator stewarding its own side.

    Manufactured graph: Operator OP-x has stakeholder 'framework-author';
    R-1 is owned by 'framework-author'; a Conflict contains R-1 with
    steward='OP-x' — M36 violation.
    """
    op_x = Operator(
        id="OP-x",
        stakeholder="framework-author",
        lifecycle="ACTIVE",
    )
    r1 = _req("R-1", "framework-author")
    r2 = _req("R-2", "framework-reviewer")
    c_axis = "agent-autonomy-vs-human-control"
    c_ctx = "conflict steward is operator self-test"
    c = Conflict(
        id=conflict_identity(c_axis, c_ctx),
        axis=c_axis,
        context=c_ctx,
        members=("R-1", "R-2"),
        steward="OP-x",  # the operator acts as steward — M36 violation
        lifecycle="DETECTED",
    )
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=(_S_AUTHOR, _S_REVIEWER),
        assumptions=(),
        requirements=(r1, r2),
        conflicts=(c,),
        operators=(op_x,),
    )
    v = check_operator_steward_not_self(g)
    assert v, "check_operator_steward_not_self must fire on M36 violation"
    assert any("OP-x" in x.message and "self-approve" in x.message for x in v), (
        "violation message must mention OP-x and self-approve"
    )


def test_check_operator_steward_not_self_green_on_independent_steward() -> None:
    """check_operator_steward_not_self stays green when operator does not steward own conflict.

    OP-x's stakeholder is 'framework-author'; conflict is stewarded by
    the Stakeholder 'framework-reviewer' (not any operator) — no violation.
    """
    op_x = Operator(
        id="OP-x",
        stakeholder="framework-author",
        lifecycle="ACTIVE",
    )
    r1 = _req("R-1", "framework-author")
    r2 = _req("R-2", "framework-reviewer")
    c_axis = "agent-autonomy-vs-human-control"
    c_ctx = "well-formed operator steward test"
    c = Conflict(
        id=conflict_identity(c_axis, c_ctx),
        axis=c_axis,
        context=c_ctx,
        members=("R-1", "R-2"),
        steward="framework-reviewer",  # independent stakeholder steward — ok
        lifecycle="DETECTED",
    )
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=(_S_AUTHOR, _S_REVIEWER),
        assumptions=(),
        requirements=(r1, r2),
        conflicts=(c,),
        operators=(op_x,),
    )
    v = check_operator_steward_not_self(g)
    assert holds(v), (
        f"check_operator_steward_not_self must not fire when steward is independent; got {v}"
    )


# ---------------------------------------------------------------------------
# 3. check_operator_within_budget — anti-phantom (fires + stays green)
# ---------------------------------------------------------------------------


def test_check_operator_within_budget_fires() -> None:
    """check_operator_within_budget fires when node count exceeds limit.

    Graph has 3 requirements + 0 conflicts + 1 assumption = 4 nodes.
    Operator budget limit = 2 → must fire.
    """
    op = Operator(
        id="OP-tight",
        stakeholder="framework-author",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=2, measure="NODE_COUNT"),
    )
    r1 = _req("R-1", "framework-author")
    r2 = _req("R-2", "framework-reviewer")
    r3 = _req("R-3", "framework-author")
    a1 = Assumption(
        id="A-test", statement="test", status=HOLDS, owner="framework-author"
    )
    g = TensionGraph(
        axes=(),
        stakeholders=(_S_AUTHOR, _S_REVIEWER),
        assumptions=(a1,),
        requirements=(r1, r2, r3),
        conflicts=(),
        operators=(op,),
    )
    # node count = 3 reqs + 0 conflicts + 1 assumption = 4 > limit 2
    v = check_operator_within_budget(g)
    assert v, "check_operator_within_budget must fire when node count exceeds limit"
    assert any("OP-tight" in x.target for x in v), "violation must target OP-tight"
    assert any("crystallize" in x.message for x in v), (
        "violation message must mention crystallize"
    )


def test_check_operator_within_budget_green_when_unbounded() -> None:
    """check_operator_within_budget stays green when limit=0 (unbounded)."""
    op = Operator(
        id="OP-free",
        stakeholder="framework-author",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=0),  # unbounded
    )
    g = TensionGraph(
        axes=(),
        stakeholders=(_S_AUTHOR, _S_REVIEWER),
        assumptions=(),
        requirements=tuple(_req(f"R-{i}", "framework-author") for i in range(50)),
        conflicts=(),
        operators=(op,),
    )
    v = check_operator_within_budget(g)
    assert holds(v), "unbounded operator (limit=0) must not fire budget check"


def test_check_operator_within_budget_green_when_under() -> None:
    """check_operator_within_budget stays green when node count <= limit."""
    op = Operator(
        id="OP-ok",
        stakeholder="framework-author",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=100, measure="NODE_COUNT"),
    )
    r1 = _req("R-1", "framework-author")
    g = TensionGraph(
        axes=(),
        stakeholders=(_S_AUTHOR, _S_REVIEWER),
        assumptions=(),
        requirements=(r1,),
        conflicts=(),
        operators=(op,),
    )
    # 1 req + 0 conflicts + 0 assumptions = 1 <= 100
    v = check_operator_within_budget(g)
    assert holds(v), "operator within budget must not fire"
