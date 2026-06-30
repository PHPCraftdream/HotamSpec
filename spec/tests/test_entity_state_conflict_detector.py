"""Tests for entity_state_conflict_suspects — the entity-state conflict detector (P21.3).

HEURISTIC detector: never auto-materializes a Conflict; surfaces LatentSuspect
entries for AI review. Two Processes driving the same EntityType into mutually-
exclusive terminal/quiescent states is the canonical hidden contradiction (M16).
"""

from __future__ import annotations

from hotam_spec.entity import EntityType
from hotam_spec.graph import TensionGraph, entity_state_conflict_suspects
from hotam_spec.lifecycle import INITIAL, QUIESCENT, Lifecycle, State, Transition
from hotam_spec.process import Process, Step


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _customer_lifecycle() -> Lifecycle:
    return Lifecycle(
        slug="customer-lifecycle",
        states=(
            State("ACTIVE", kind=INITIAL, why="Customer active."),
            State("SUSPENDED", kind=QUIESCENT, why="Temporarily suspended."),
            State("CLOSED", kind=QUIESCENT, why="Account permanently closed."),
        ),
        transitions=(
            Transition("ACTIVE", "SUSPENDED", event="suspend", why="Suspend."),
            Transition("ACTIVE", "CLOSED", event="close", why="Close."),
            Transition("SUSPENDED", "ACTIVE", event="reopen", why="Reopen."),
        ),
        cyclic=False,
    )


def _customer_entity_type() -> EntityType:
    return EntityType(
        slug="customer",
        description="A business customer.",
        lifecycle=_customer_lifecycle(),
    )


def _simple_proc_lifecycle(slug: str) -> Lifecycle:
    return Lifecycle(
        slug=slug,
        states=(
            State("READY", kind=INITIAL, why="Ready."),
            State("DONE", kind=QUIESCENT, why="Done."),
        ),
        transitions=(Transition("READY", "DONE", event="complete", why="Finish."),),
        cyclic=False,
    )


def _proc(pid: str, invokes: str, drives: str = "customer") -> Process:
    step = Step(name="act", requires_role="operator", invokes=invokes)
    return Process(
        id=pid,
        lifecycle=_simple_proc_lifecycle(f"{pid}-lc"),
        steps=(step,),
        roles_required=("operator",),
        drives_entities=(drives,),
    )


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_disjoint_destinations_emits_suspect():
    """Two processes driving customer to SUSPENDED vs CLOSED → one LatentSuspect."""
    et = _customer_entity_type()
    p_suspend = _proc("PR-fraud-suspend", invokes="customer.suspend")
    p_close = _proc("PR-delinquent-close", invokes="customer.close")
    g = TensionGraph(entity_types=(et,), processes=(p_suspend, p_close))
    suspects = entity_state_conflict_suspects(g)
    assert len(suspects) == 1
    s = suspects[0]
    # left/right are sorted
    assert set((s.left, s.right)) == {"PR-fraud-suspend", "PR-delinquent-close"}
    assert "customer" in s.hint
    assert "SUSPENDED" in s.hint or "CLOSED" in s.hint


def test_same_destination_no_suspect():
    """Two processes both driving customer to SUSPENDED → no suspect."""
    et = _customer_entity_type()
    p1 = _proc("PR-fraud-suspend", invokes="customer.suspend")
    p2 = _proc("PR-policy-suspend", invokes="customer.suspend")
    g = TensionGraph(entity_types=(et,), processes=(p1, p2))
    suspects = entity_state_conflict_suspects(g)
    assert suspects == ()


def test_single_process_no_suspect():
    """Only one process — no pair to compare."""
    et = _customer_entity_type()
    p = _proc("PR-fraud-suspend", invokes="customer.suspend")
    g = TensionGraph(entity_types=(et,), processes=(p,))
    suspects = entity_state_conflict_suspects(g)
    assert suspects == ()


def test_no_entity_types_empty_result():
    """Graph with no entity_types — detector returns empty."""
    p = _proc("PR-fraud-suspend", invokes="customer.suspend")
    g = TensionGraph(processes=(p,))
    suspects = entity_state_conflict_suspects(g)
    assert suspects == ()


def test_no_processes_empty_result():
    """Graph with no processes — detector returns empty."""
    et = _customer_entity_type()
    g = TensionGraph(entity_types=(et,))
    suspects = entity_state_conflict_suspects(g)
    assert suspects == ()


def test_process_not_in_drives_entities_ignored():
    """A process whose drives_entities does not include customer is ignored for customer."""
    et = _customer_entity_type()
    p_suspend = _proc("PR-fraud-suspend", invokes="customer.suspend", drives="customer")
    p_other = _proc("PR-other", invokes="customer.close", drives="other-entity")
    g = TensionGraph(entity_types=(et,), processes=(p_suspend, p_other))
    suspects = entity_state_conflict_suspects(g)
    # p_other doesn't drive 'customer', so no pair for customer
    assert suspects == ()
