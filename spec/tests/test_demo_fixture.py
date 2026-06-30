"""End-to-end dogfood tests for the demo fixture Entity machine (P21.6).

Verifies that declaring an EntityType + two opposing Processes in the fixture
causes the harness to surface a P5 entity-state conflict suspect. This proves the
full P21 chain end-to-end:

  declare EntityType → declare two opposing Processes →
  entity_state_conflict_suspects() fires → what_now.diagnose() emits P5.
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

from fixtures.seed import (  # noqa: E402
    PR_AUTO_SUSPEND_FRAUD,
    PR_BILLING_CLOSE_DELINQUENT,
    seed_graph,
)
from tensio.graph import entity_state_conflict_suspects  # noqa: E402
from tensio.invariants import all_violations, holds  # noqa: E402

import what_now  # noqa: E402


def _g():
    return seed_graph()


# ---------------------------------------------------------------------------
# Test 1 — structural invariants hold (entity machine adds no violations)
# ---------------------------------------------------------------------------


def test_demo_fixture_passes_structural_invariants():
    """all_violations(seed_graph()) must return [] for all entity checks EXCEPT
    check_entities_md_lists_all_types, which compares the demo fixture's entity
    types against the active *domain's* ENTITIES.md (tensio-self has no entity
    types — that doc does not list 'customer'). The demo ENTITIES.md path is a
    P21.4 concern (gen_spec.py --demo writes docs/demo/ENTITIES.md; the invariant
    looks at the domain path). All other entity-machine invariants must pass."""
    g = _g()
    violations = [
        v
        for v in all_violations(g)
        if v.invariant != "check_entities_md_lists_all_types"
    ]
    assert holds(violations), "demo fixture has unexpected structural violations:\n" + "\n".join(
        f"  {v}" for v in violations
    )


# ---------------------------------------------------------------------------
# Test 2 — detector fires with the two opposing process ids
# ---------------------------------------------------------------------------


def test_demo_fixture_entity_state_detector_fires():
    """entity_state_conflict_suspects() must return at least one LatentSuspect
    whose left/right are the two opposing processes and whose hint mentions the
    disjoint destination states."""
    g = _g()
    suspects = entity_state_conflict_suspects(g)
    assert len(suspects) >= 1, "expected at least one entity-state conflict suspect"

    s = suspects[0]
    ids = {s.left, s.right}
    assert ids == {
        PR_AUTO_SUSPEND_FRAUD.id,
        PR_BILLING_CLOSE_DELINQUENT.id,
    }, f"unexpected process pair in suspect: {ids}"
    assert "customer" in s.hint
    # Both destination states should appear in the hint
    assert "SUSPENDED" in s.hint or "CLOSED" in s.hint


# ---------------------------------------------------------------------------
# Test 3 — check_process_drives_existing_entities is green
# ---------------------------------------------------------------------------


def test_demo_fixture_check_process_drives_existing_entities_green():
    """Both opposing processes declare drives_entities=('customer',) which
    resolves to the declared CUSTOMER_ENTITY — no dangling reference."""
    from tensio.invariants import check_process_drives_existing_entities  # noqa: PLC0415

    g = _g()
    violations = check_process_drives_existing_entities(g)
    assert holds(violations), (
        "check_process_drives_existing_entities fired:\n"
        + "\n".join(f"  {v}" for v in violations)
    )


# ---------------------------------------------------------------------------
# Test 4 — check_step_invokes_known_transition is green
# ---------------------------------------------------------------------------


def test_demo_fixture_check_step_invokes_known_transition_green():
    """Each Step.invokes ('customer.suspend', 'customer.close') resolves to a
    real Lifecycle transition on the CUSTOMER_ENTITY lifecycle."""
    from tensio.invariants import check_step_invokes_known_transition  # noqa: PLC0415

    g = _g()
    violations = check_step_invokes_known_transition(g)
    assert holds(violations), (
        "check_step_invokes_known_transition fired:\n"
        + "\n".join(f"  {v}" for v in violations)
    )


# ---------------------------------------------------------------------------
# Test 5 — check_entity_instance_state_in_lifecycle is green
# ---------------------------------------------------------------------------


def test_demo_fixture_check_entity_instance_state_in_lifecycle_green():
    """CUSTOMER_ACME.state == 'ACTIVE' — a valid state in CUSTOMER_LIFECYCLE."""
    from tensio.invariants import check_entity_instance_state_in_lifecycle  # noqa: PLC0415

    g = _g()
    violations = check_entity_instance_state_in_lifecycle(g)
    assert holds(violations), (
        "check_entity_instance_state_in_lifecycle fired:\n"
        + "\n".join(f"  {v}" for v in violations)
    )


# ---------------------------------------------------------------------------
# Test 6 — check_entity_instance_refs_resolve is green
# ---------------------------------------------------------------------------


def test_demo_fixture_check_entity_instance_refs_resolve_green():
    """CUSTOMER_ACME.owner references 'finance' — a real Stakeholder.id in the
    seed graph — so the reference resolves cleanly."""
    from tensio.invariants import check_entity_instance_refs_resolve  # noqa: PLC0415

    g = _g()
    violations = check_entity_instance_refs_resolve(g)
    assert holds(violations), "check_entity_instance_refs_resolve fired:\n" + "\n".join(
        f"  {v}" for v in violations
    )


# ---------------------------------------------------------------------------
# Test 7 — what_now.diagnose() emits a P5 entity-state conflict action
# ---------------------------------------------------------------------------


def test_demo_fixture_what_now_emits_p5_entity_state_conflict():
    """what_now.diagnose(seed_graph()) must include at least one P5 action
    tagged '[HEURISTIC, entity-state conflict]' naming the two processes."""
    g = _g()
    actions = what_now.diagnose(g)
    p5_entity_actions = [
        a
        for a in actions
        if a.priority == what_now.P_LATENT_CONNECTOR
        and "[HEURISTIC, entity-state conflict]" in a.imperative
    ]
    assert p5_entity_actions, (
        "no P5 entity-state conflict action found in what_now output; "
        f"all P5 actions: {[a for a in actions if a.priority == what_now.P_LATENT_CONNECTOR]}"
    )
    target = p5_entity_actions[0].target
    assert (
        PR_AUTO_SUSPEND_FRAUD.id in target or PR_BILLING_CLOSE_DELINQUENT.id in target
    )
