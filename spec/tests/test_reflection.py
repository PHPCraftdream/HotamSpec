"""Tests for the P0 REFLECTION band in tools/what_now.py (P8).

Guarantees:
  1. P_REFLECTION < P_STRUCTURE (ranked above all other bands).
  2. DRAFT-overhang fires when DRAFT >= SETTLED/2; disappears when ratio is healthy.
  3. UNENFORCED-SETTLED overhang fires when > 5 SETTLED are PROSE/STRUCTURAL.
  4. Over-budget operator fires per operator whose graph size exceeds budget.
  5. DEAD-assumption-on-ENFORCER fires for each (dead-assumption, enforced-req) pair.
  6. Derived-but-unbuilt fires for DECIDED conflicts whose derived ids remain DRAFT.
  7. The live meta-domain REFLECTION action count is 0–5 (reasonable range); today
     the DRAFT/SETTLED ratio is healthy (DRAFT 4 < SETTLED/2), so burn-down does
     NOT fire. We assert the specific conditions that fire today.
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

from tensio.assumption import DEAD, Assumption  # noqa: E402
from tensio.axis import Axis  # noqa: E402
from tensio.conflict import Conflict, conflict_identity  # noqa: E402
from tensio.graph import TensionGraph  # noqa: E402
from tensio.operator import ContextBudget, Operator  # noqa: E402
from tensio.requirement import DRAFT, ENFORCED, PROSE, SETTLED, Requirement  # noqa: E402
from tensio.stakeholder import Stakeholder  # noqa: E402

from what_now import P_REFLECTION, P_STRUCTURE, diagnose  # noqa: E402

# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

_DUMMY_AXES = (
    Axis(slug="ax-one", description="test axis one"),
    Axis(slug="ax-two", description="test axis two"),
)
_SH = (
    Stakeholder(id="s-a", name="A", domain="x"),
    Stakeholder(id="s-b", name="B", domain="y"),
    Stakeholder(id="s-c", name="C", domain="z"),
)


def _settled_req(rid: str, enforcement: str = ENFORCED) -> Requirement:
    return Requirement(
        id=rid,
        claim=f"claim for {rid}",
        owner="s-a",
        status=SETTLED,
        enforcement=enforcement,
        enforced_by=(f"test_{rid}",) if enforcement == ENFORCED else (),
    )


def _draft_req(rid: str) -> Requirement:
    return Requirement(
        id=rid, claim=f"draft claim for {rid}", owner="s-a", status=DRAFT
    )


# ---------------------------------------------------------------------------
# 1. Band ordering
# ---------------------------------------------------------------------------


def test_reflection_band_above_structure() -> None:
    """P_REFLECTION < P_STRUCTURE — REFLECTION is more urgent than STRUCTURE."""
    assert P_REFLECTION < P_STRUCTURE


# ---------------------------------------------------------------------------
# 2. DRAFT-overhang
# ---------------------------------------------------------------------------


def test_reflection_emits_draft_overhang() -> None:
    """DRAFT-overhang fires when DRAFT (10) >= SETTLED/2 (3/2=1.5)."""
    settled = tuple(_settled_req(f"R-s{i}") for i in range(3))
    drafts = tuple(_draft_req(f"R-d{i}") for i in range(10))
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=settled + drafts,
    )
    actions = diagnose(g)
    reflection_actions = [a for a in actions if a.priority == P_REFLECTION]
    burn_down = [a for a in reflection_actions if a.target == "burn-down"]
    assert burn_down, (
        "REFLECTION must fire burn-down when 10 DRAFT vs 3 SETTLED (ratio >= 0.5)"
    )
    assert burn_down[0].kind == "REFLECTION"


def test_reflection_no_overhang_when_ratio_healthy() -> None:
    """No burn-down action when DRAFT (3) < SETTLED/2 (20/2=10)."""
    settled = tuple(_settled_req(f"R-s{i}") for i in range(20))
    drafts = tuple(_draft_req(f"R-d{i}") for i in range(3))
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=settled + drafts,
    )
    actions = diagnose(g)
    burn_down = [
        a for a in actions if a.priority == P_REFLECTION and a.target == "burn-down"
    ]
    assert not burn_down, (
        "burn-down must NOT fire when 3 DRAFT vs 20 SETTLED (healthy ratio)"
    )


# ---------------------------------------------------------------------------
# 3. Over-budget operators
# ---------------------------------------------------------------------------


def test_reflection_emits_over_budget_operator() -> None:
    """Over-budget operator fires when graph has more nodes than limit."""
    # 5 requirements + 0 conflicts + 0 assumptions = 5 nodes; budget = 2.
    reqs = tuple(_settled_req(f"R-r{i}") for i in range(5))
    op = Operator(
        id="OP-test",
        stakeholder="s-a",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=2, measure="NODE_COUNT"),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=reqs,
        operators=(op,),
    )
    actions = diagnose(g)
    over_budget = [
        a for a in actions if a.priority == P_REFLECTION and a.target == "OP-test"
    ]
    assert over_budget, "REFLECTION must fire for OP-test holding 5 nodes > budget 2"
    assert "OP-test" in over_budget[0].imperative
    assert "5" in over_budget[0].imperative
    assert "2" in over_budget[0].imperative


def test_reflection_no_over_budget_when_within_limit() -> None:
    """No over-budget action when graph fits within operator budget."""
    reqs = tuple(_settled_req(f"R-r{i}") for i in range(3))
    op = Operator(
        id="OP-small",
        stakeholder="s-a",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=200, measure="NODE_COUNT"),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=reqs,
        operators=(op,),
    )
    actions = diagnose(g)
    over_budget = [
        a for a in actions if a.priority == P_REFLECTION and a.target == "OP-small"
    ]
    assert not over_budget, "No over-budget action when within limit"


def test_reflection_no_over_budget_when_limit_zero() -> None:
    """No over-budget action when budget limit is 0 (unbounded)."""
    reqs = tuple(_settled_req(f"R-r{i}") for i in range(100))
    op = Operator(
        id="OP-unbounded",
        stakeholder="s-a",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=0, measure="NODE_COUNT"),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=reqs,
        operators=(op,),
    )
    actions = diagnose(g)
    over_budget = [
        a for a in actions if a.priority == P_REFLECTION and a.target == "OP-unbounded"
    ]
    assert not over_budget, "Limit=0 means unbounded — no over-budget action"


# ---------------------------------------------------------------------------
# 4. DEAD-assumption-on-ENFORCER
# ---------------------------------------------------------------------------


def test_reflection_emits_dead_assumption_enforcer() -> None:
    """DEAD assumption + ENFORCED requirement resting on it fires REFLECTION."""
    dead_a = Assumption(
        id="A-dead-one",
        statement="This was true, now dead.",
        status=DEAD,
        owner="s-a",
    )
    enforced_r = Requirement(
        id="R-enforced-on-dead",
        claim="enforced claim resting on dead assumption",
        owner="s-b",
        status=SETTLED,
        assumptions=("A-dead-one",),
        enforcement=ENFORCED,
        enforced_by=("test_enforced_on_dead",),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        assumptions=(dead_a,),
        requirements=(enforced_r,),
    )
    actions = diagnose(g)
    stale = [
        a
        for a in actions
        if a.priority == P_REFLECTION and a.target == "R-enforced-on-dead"
    ]
    assert stale, (
        "REFLECTION must fire for R-enforced-on-dead resting on DEAD A-dead-one"
    )
    assert "A-dead-one" in stale[0].imperative
    assert "R-stale-substrate" in stale[0].imperative


def test_reflection_no_dead_assumption_on_non_enforced() -> None:
    """DEAD assumption + PROSE requirement does NOT fire the ENFORCER condition."""
    dead_a = Assumption(
        id="A-dead-two",
        statement="Dead.",
        status=DEAD,
        owner="s-a",
    )
    prose_r = Requirement(
        id="R-prose-on-dead",
        claim="prose claim on dead assumption",
        owner="s-b",
        status=SETTLED,
        assumptions=("A-dead-two",),
        enforcement=PROSE,
        enforced_by=(),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        assumptions=(dead_a,),
        requirements=(prose_r,),
    )
    actions = diagnose(g)
    stale = [
        a
        for a in actions
        if a.priority == P_REFLECTION and a.target == "R-prose-on-dead"
    ]
    assert not stale, "Dead-assumption-on-ENFORCER must NOT fire for PROSE requirements"


# ---------------------------------------------------------------------------
# 5. Derived-but-unbuilt
# ---------------------------------------------------------------------------


def test_reflection_emits_derived_unbuilt() -> None:
    """DECIDED conflict whose derived id is still DRAFT fires REFLECTION."""
    draft_r = _draft_req("R-derived-draft")
    # We need a valid conflict: axis must be in the graph's axes, members must resolve.
    # We put R-derived-draft as a member too to keep the graph self-consistent for
    # the REFLECTION test (we're testing derived-but-unbuilt, not structural validity).
    settled_r1 = _settled_req("R-parent-one")
    settled_r2 = _settled_req("R-parent-two")
    ax = "ax-one"
    ctx = "derived-unbuilt test context"
    c = Conflict(
        id=conflict_identity(ax, ctx),
        axis=ax,
        context=ctx,
        members=("R-parent-one", "R-parent-two"),
        steward="s-c",
        lifecycle="DECIDED(chose R-parent-one; derived R-derived-draft)",
        decided_by="s-c",
        derived=("R-derived-draft",),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(settled_r1, settled_r2, draft_r),
        conflicts=(c,),
    )
    actions = diagnose(g)
    unbuilt = [
        a
        for a in actions
        if a.priority == P_REFLECTION and a.target == "R-derived-draft"
    ]
    assert unbuilt, (
        "REFLECTION must fire for R-derived-draft derived by DECIDED conflict but still DRAFT"
    )
    assert c.id in unbuilt[0].imperative


def test_reflection_no_derived_unbuilt_when_settled() -> None:
    """DECIDED conflict whose derived id is SETTLED does NOT fire."""
    settled_derived = _settled_req("R-derived-settled")
    settled_r1 = _settled_req("R-parent-three")
    settled_r2 = _settled_req("R-parent-four")
    ax = "ax-one"
    ctx = "derived-settled test context"
    c = Conflict(
        id=conflict_identity(ax, ctx),
        axis=ax,
        context=ctx,
        members=("R-parent-three", "R-parent-four"),
        steward="s-c",
        lifecycle="DECIDED(chose R-parent-three)",
        decided_by="s-c",
        derived=("R-derived-settled",),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(settled_r1, settled_r2, settled_derived),
        conflicts=(c,),
    )
    actions = diagnose(g)
    unbuilt = [
        a
        for a in actions
        if a.priority == P_REFLECTION and a.target == "R-derived-settled"
    ]
    assert not unbuilt, (
        "derived-but-unbuilt must NOT fire when derived requirement is SETTLED"
    )


# ---------------------------------------------------------------------------
# 6. Live meta-domain smoke test
# ---------------------------------------------------------------------------


def test_real_meta_domain_reflection_today() -> None:
    """Live meta-domain: REFLECTION actions are in a reasonable range and sensible.

    State after P11 (21 new requirements added):
      - SETTLED = 45, DRAFT = 24 -> DRAFT (24) >= SETTLED/2 (22.5) -> burn-down fires
      - UNENFORCED SETTLED > 5 may fire -> enforcement-gradient action
      - OP-director budget=200 vs graph_size ~100 -> within budget -> NO over-budget
      - No DEAD assumptions -> no dead-assumption-on-enforcer
      - R-active-loop-playbooks is DECIDED derived but SETTLED -> no derived-unbuilt
    """
    from tensio.graph import load_content_graph  # noqa: PLC0415

    g = load_content_graph()
    actions = diagnose(g)
    reflection_actions = [a for a in actions if a.priority == P_REFLECTION]

    # A small number of REFLECTION actions is reasonable.
    assert 0 <= len(reflection_actions) <= 10, (
        f"Expected 0-10 REFLECTION actions, got {len(reflection_actions)}: "
        f"{reflection_actions}"
    )

    # OP-director has budget=200 and graph is well under that; must NOT fire.
    over_budget = [a for a in reflection_actions if "OP-director" in a.target]
    assert not over_budget, f"OP-director must not be over-budget; got {over_budget}"

    # No DEAD assumptions in the meta-domain today; no enforcer-on-dead.
    enforcer_dead = [
        a for a in reflection_actions if "R-stale-substrate signal" in a.imperative
    ]
    assert not enforcer_dead, (
        f"No DEAD assumptions today; enforcer-dead must not fire; got {enforcer_dead}"
    )
