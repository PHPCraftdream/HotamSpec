"""Tests for the §Process opt-in behavioral aspect (P9, M12).

Covers: PR-closed-loop presence; process lifecycle well-formedness;
role-declaration invariant; typed-anchor enforcement; negative role test.
"""

from __future__ import annotations

import sys
from pathlib import Path

# Make hotam_spec importable.
_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from hotam_spec.graph import load_content_graph  # noqa: E402
from hotam_spec.invariants import (  # noqa: E402
    check_process_lifecycle_wellformed,
    check_process_roles_declared,
    check_typed_anchors,
)
from hotam_spec.process import PROCESS_LIFECYCLE, Process, Step  # noqa: E402


def test_pr_closed_loop_present() -> None:
    """PR-closed-loop exists in load_content_graph().processes."""
    g = load_content_graph()
    pids = {p.id for p in g.processes}
    assert "PR-closed-loop" in pids, (
        "PR-closed-loop not found in g.processes; "
        "check content/graph.py processes tuple"
    )


def test_process_lifecycle_wellformed_on_real_graph() -> None:
    """check_process_lifecycle_wellformed passes on the real meta-domain graph."""
    g = load_content_graph()
    viols = check_process_lifecycle_wellformed(g)
    assert viols == [], f"Process lifecycle invariant fired: {viols}"


def test_process_roles_declared_passes() -> None:
    """check_process_roles_declared passes on the real meta-domain graph."""
    g = load_content_graph()
    viols = check_process_roles_declared(g)
    assert viols == [], f"Process roles invariant fired: {viols}"


def test_check_process_roles_undeclared_fires() -> None:
    """Manufacturing a Step with undeclared role fires check_process_roles_declared."""
    from hotam_spec.graph import TensionGraph  # noqa: PLC0415

    bad_process = Process(
        id="PR-bad",
        lifecycle=PROCESS_LIFECYCLE,
        steps=(Step(name="do-thing", requires_role="phantom-role", why="test"),),
        roles_required=("operator",),  # phantom-role NOT declared
        why="deliberate violation for test",
    )
    g = TensionGraph(processes=(bad_process,))
    viols = check_process_roles_declared(g)
    assert len(viols) == 1
    assert viols[0].target == "PR-bad"
    assert "phantom-role" in viols[0].message


def test_process_typed_anchor() -> None:
    """All Process.id values in the real graph start with 'PR-'."""
    g = load_content_graph()
    for p in g.processes:
        assert p.id.startswith("PR-"), f"Process id '{p.id}' does not start with 'PR-'"


def test_check_typed_anchors_fires_on_bad_process_id() -> None:
    """check_typed_anchors fires when a Process id lacks the PR- prefix."""
    from hotam_spec.graph import TensionGraph  # noqa: PLC0415

    bad = Process(
        id="PROCESS-bad",
        lifecycle=PROCESS_LIFECYCLE,
        why="deliberate bad anchor",
    )
    g = TensionGraph(processes=(bad,))
    viols = check_typed_anchors(g)
    targets = {v.target for v in viols}
    assert "PROCESS-bad" in targets


def test_process_aspect_noop_on_empty_processes() -> None:
    """Both process invariants are no-ops when g.processes is empty."""
    from hotam_spec.graph import TensionGraph  # noqa: PLC0415

    g = TensionGraph()  # empty; no processes
    assert check_process_lifecycle_wellformed(g) == []
    assert check_process_roles_declared(g) == []
