"""Tests for check_process_drives_existing_entities and check_step_invokes_known_transition.

Covers: P21.3 — activating the forward-compat seam between Process and Entity.
"""

from __future__ import annotations


from hotam_spec.entity import EntityType
from hotam_spec.graph import TensionGraph
from hotam_spec.invariants import (
    check_process_drives_existing_entities,
    check_step_invokes_known_transition,
)
from hotam_spec.lifecycle import INITIAL, QUIESCENT, Lifecycle, State, Transition
from hotam_spec.process import Process, Step


# ---------------------------------------------------------------------------
# Shared fixtures
# ---------------------------------------------------------------------------


def _customer_lifecycle() -> Lifecycle:
    return Lifecycle(
        slug="customer-lifecycle",
        states=(
            State("ACTIVE", kind=INITIAL, why="Customer is active."),
            State("SUSPENDED", kind=QUIESCENT, why="Customer suspended."),
            State("CLOSED", kind=QUIESCENT, why="Customer account closed."),
        ),
        transitions=(
            Transition("ACTIVE", "SUSPENDED", event="suspend", why="Suspend customer."),
            Transition(
                "ACTIVE", "CLOSED", event="close", why="Close customer account."
            ),
            Transition("SUSPENDED", "ACTIVE", event="reopen", why="Reopen."),
        ),
        cyclic=False,
    )


def _customer_entity_type() -> EntityType:
    return EntityType(
        slug="customer",
        description="A business customer.",
        lifecycle=_customer_lifecycle(),
        why="Core domain entity.",
    )


def _make_process(
    proc_id: str, drives: tuple[str, ...], steps: tuple[Step, ...]
) -> Process:
    from hotam_spec.lifecycle import (
        INITIAL as I,
        QUIESCENT as Q,
        Lifecycle,
        State,
        Transition,
    )

    lc = Lifecycle(
        slug=f"{proc_id}-lc",
        states=(
            State("READY", kind=I, why="Ready."),
            State("DONE", kind=Q, why="Done."),
        ),
        transitions=(Transition("READY", "DONE", event="complete", why="Finish."),),
        cyclic=False,
    )
    roles = tuple({s.requires_role for s in steps}) if steps else ("operator",)
    return Process(
        id=proc_id,
        lifecycle=lc,
        steps=steps,
        roles_required=roles,
        drives_entities=drives,
    )


def _make_step(name: str, invokes: str = "") -> Step:
    return Step(name=name, requires_role="operator", invokes=invokes)


# ---------------------------------------------------------------------------
# check_process_drives_existing_entities
# ---------------------------------------------------------------------------


def test_drives_existing_entities_no_violation():
    et = _customer_entity_type()
    p = _make_process(
        "PR-1", drives=("customer",), steps=(_make_step("go", "customer.suspend"),)
    )
    g = TensionGraph(entity_types=(et,), processes=(p,))
    assert check_process_drives_existing_entities(g) == []


def test_drives_existing_entities_fires_on_undeclared_slug():
    p = _make_process("PR-1", drives=("nonexistent",), steps=(_make_step("go"),))
    g = TensionGraph(processes=(p,))
    violations = check_process_drives_existing_entities(g)
    assert len(violations) == 1
    assert violations[0].invariant == "check_process_drives_existing_entities"
    assert violations[0].target == "PR-1"
    assert "nonexistent" in violations[0].message


def test_drives_existing_entities_empty_graph_noop():
    g = TensionGraph()
    assert check_process_drives_existing_entities(g) == []


# ---------------------------------------------------------------------------
# check_step_invokes_known_transition
# ---------------------------------------------------------------------------


def test_step_invokes_valid_event_no_violation():
    et = _customer_entity_type()
    step = _make_step("suspend-step", invokes="customer.suspend")
    p = _make_process("PR-2", drives=("customer",), steps=(step,))
    g = TensionGraph(entity_types=(et,), processes=(p,))
    assert check_step_invokes_known_transition(g) == []


def test_step_invokes_bogus_event_fires():
    et = _customer_entity_type()
    step = _make_step("bad-step", invokes="customer.bogus_event")
    p = _make_process("PR-2", drives=("customer",), steps=(step,))
    g = TensionGraph(entity_types=(et,), processes=(p,))
    violations = check_step_invokes_known_transition(g)
    assert len(violations) == 1
    assert violations[0].invariant == "check_step_invokes_known_transition"
    assert "bogus_event" in violations[0].message
    assert "known:" in violations[0].message


def test_step_invokes_unknown_entity_fires():
    et = _customer_entity_type()
    step = _make_step("bad-step", invokes="unknown.suspend")
    p = _make_process("PR-2", drives=("customer",), steps=(step,))
    g = TensionGraph(entity_types=(et,), processes=(p,))
    violations = check_step_invokes_known_transition(g)
    assert len(violations) == 1
    assert "unknown entity 'unknown'" in violations[0].message


def test_step_invokes_missing_dot_fires():
    et = _customer_entity_type()
    step = _make_step("bad-step", invokes="customersuspend")
    p = _make_process("PR-2", drives=("customer",), steps=(step,))
    g = TensionGraph(entity_types=(et,), processes=(p,))
    violations = check_step_invokes_known_transition(g)
    assert len(violations) == 1
    assert "<entity-slug>.<event>" in violations[0].message


def test_step_invokes_empty_no_violation():
    """A step with empty invokes is valid (no binding — prose-only)."""
    et = _customer_entity_type()
    step = _make_step("look-step", invokes="")
    p = _make_process("PR-2", drives=("customer",), steps=(step,))
    g = TensionGraph(entity_types=(et,), processes=(p,))
    assert check_step_invokes_known_transition(g) == []


def test_step_invokes_empty_graph_noop():
    g = TensionGraph()
    assert check_step_invokes_known_transition(g) == []
