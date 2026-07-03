"""Tests for check_assumption_machine_checks_syntactic (R-machine-check-syntactic).

The invariant guarantees every non-empty Assumption.machine_check is a
well-formed Python EXPRESSION (compilable in eval mode), NOT that it is true or
executable against any namespace — that is the deliberate honesty boundary
(spec-stack layers 4/5 deferred).
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from hotam_spec.assumption import HOLDS, Assumption  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.invariants import (  # noqa: E402
    ALL_INVARIANTS,
    check_assumption_machine_checks_syntactic,
    holds,
)


def _g(mc: str) -> TensionGraph:
    a = Assumption(
        id="A-mc",
        statement="belief",
        status=HOLDS,
        owner="framework-author",
        machine_check=mc,
    )
    return TensionGraph(assumptions=(a,))


def test_registered_in_all_invariants() -> None:
    assert check_assumption_machine_checks_syntactic in ALL_INVARIANTS


def test_compilable_formula_passes() -> None:
    g = _g("len(graph.requirements) + len(graph.conflicts) < 10_000")
    assert holds(check_assumption_machine_checks_syntactic(g))


def test_python_version_formula_passes() -> None:
    g = _g("python.version >= (3, 12)")
    assert holds(check_assumption_machine_checks_syntactic(g))


def test_empty_machine_check_is_skipped() -> None:
    g = _g("")
    assert holds(check_assumption_machine_checks_syntactic(g))
    # None also skipped.
    a = Assumption(id="A-x", statement="s", status=HOLDS, owner="framework-author")
    assert holds(check_assumption_machine_checks_syntactic(TensionGraph(assumptions=(a,))))


def test_prose_formula_fires_violation() -> None:
    g = _g("this is prose not a formula")
    violations = check_assumption_machine_checks_syntactic(g)
    assert len(violations) == 1
    assert violations[0].target == "A-mc"
    assert violations[0].invariant == "check_assumption_machine_checks_syntactic"


def test_real_domain_machine_checks_are_syntactic() -> None:
    from hotam_spec.graph import load_content_graph

    g = load_content_graph()
    assert holds(check_assumption_machine_checks_syntactic(g))
