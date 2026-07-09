"""Tests for spec/tools/gate.py — the T1 targeted-enforcer LAND-gate selector.

Enforcer/lift for the projected R-tool-gate (R-tool-is-its-own-requirement).

Covers:
  1. Mapping: a Requirement with resolvable enforced_by entries (test file,
     test_file::func, check_* name, bare test_* name) resolves to the
     expected pytest node-id set, ALWAYS_RUN always included.
  2. Fail-closed on an unresolvable enforcer entry (a doc path or bare name
     that is neither test_* nor check_*).
  3. Fail-closed when enforced_by is empty.
  4. Fail-closed when the target is not found in the graph (new/unknown node).
  5. Fail-closed for a Conflict target (no per-instance enforced_by).
  6. Determinism: same graph + same target -> byte-identical node-id tuple.
  7. CLI: prints node-ids on a resolvable target (exit 0), prints the
     FAIL-CLOSED notice + returns 1 on an unresolvable one.
"""

from __future__ import annotations


import gate  # noqa: E402
from hotam_spec.conflict import Conflict  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.requirement import Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402


def _sh() -> tuple[Stakeholder, ...]:
    return (
        Stakeholder(id="ST-a", name="A", domain="d"),
        Stakeholder(id="ST-b", name="B", domain="d"),
    )


# ---------------------------------------------------------------------------
# 1. Mapping — resolvable enforced_by entries
# ---------------------------------------------------------------------------


def test_resolves_test_file_entry() -> None:
    g = TensionGraph(
        stakeholders=_sh(),
        requirements=(
            Requirement(
                id="R-x",
                claim="c",
                owner="ST-a",
                status="SETTLED",
                enforced_by=("test_smoke.py",),
            ),
        ),
    )
    result = gate.select_tier1("R-x", g=g)
    assert result.confident, result.reason
    assert "tests/test_smoke.py" in result.node_ids
    for always in gate.ALWAYS_RUN:
        assert always in result.node_ids


def test_resolves_test_file_func_entry() -> None:
    g = TensionGraph(
        stakeholders=_sh(),
        requirements=(
            Requirement(
                id="R-x",
                claim="c",
                owner="ST-a",
                status="SETTLED",
                enforced_by=("test_smoke.py::test_smoke",),
            ),
        ),
    )
    result = gate.select_tier1("R-x", g=g)
    assert result.confident, result.reason
    assert "tests/test_smoke.py::test_smoke" in result.node_ids


def test_resolves_check_star_entry() -> None:
    # check_axis_in_registry is referenced by tests/test_invariants.py in the
    # real test suite; use the real tests/ dir (default tests_dir).
    g = TensionGraph(
        stakeholders=_sh(),
        requirements=(
            Requirement(
                id="R-x",
                claim="c",
                owner="ST-a",
                status="SETTLED",
                enforced_by=("check_axis_in_registry",),
            ),
        ),
    )
    result = gate.select_tier1("R-x", g=g)
    assert result.confident, result.reason
    assert any("test_invariants.py" in nid for nid in result.node_ids)


def test_resolves_bare_test_func_entry() -> None:
    g = TensionGraph(
        stakeholders=_sh(),
        requirements=(
            Requirement(
                id="R-x",
                claim="c",
                owner="ST-a",
                status="SETTLED",
                enforced_by=("test_smoke",),
            ),
        ),
    )
    result = gate.select_tier1("R-x", g=g)
    assert result.confident, result.reason
    assert "tests/test_smoke.py" in result.node_ids


def test_multiple_enforced_by_entries_union() -> None:
    g = TensionGraph(
        stakeholders=_sh(),
        requirements=(
            Requirement(
                id="R-x",
                claim="c",
                owner="ST-a",
                status="SETTLED",
                enforced_by=("test_smoke.py::test_smoke", "test_smoke.py"),
            ),
        ),
    )
    result = gate.select_tier1("R-x", g=g)
    assert result.confident, result.reason
    # Node-id set is deduped (the file-level and file::func entries both
    # resolve into the same node-id family, unioned not doubled).
    assert result.node_ids.count("tests/test_smoke.py") == 1


# ---------------------------------------------------------------------------
# 2-5. Fail-closed cases
# ---------------------------------------------------------------------------


def test_fails_closed_on_unresolvable_entry() -> None:
    g = TensionGraph(
        stakeholders=_sh(),
        requirements=(
            Requirement(
                id="R-x",
                claim="c",
                owner="ST-a",
                status="SETTLED",
                enforced_by=("docs/methodology/discipline.md",),
            ),
        ),
    )
    result = gate.select_tier1("R-x", g=g)
    assert not result.confident
    assert result.node_ids == ()
    assert "unresolved" in result.reason.lower() or "could not be resolved" in result.reason


def test_fails_closed_on_partially_unresolvable_entries() -> None:
    """One good entry + one bad entry -> the WHOLE selection fails closed."""
    g = TensionGraph(
        stakeholders=_sh(),
        requirements=(
            Requirement(
                id="R-x",
                claim="c",
                owner="ST-a",
                status="SETTLED",
                enforced_by=("test_smoke.py", "CRITICAL_CORE_INVARIANTS"),
            ),
        ),
    )
    result = gate.select_tier1("R-x", g=g)
    assert not result.confident


def test_fails_closed_on_empty_enforced_by() -> None:
    g = TensionGraph(
        stakeholders=_sh(),
        requirements=(
            Requirement(id="R-x", claim="c", owner="ST-a", status="SETTLED"),
        ),
    )
    result = gate.select_tier1("R-x", g=g)
    assert not result.confident
    assert "empty" in result.reason.lower()


def test_fails_closed_on_unknown_target() -> None:
    g = TensionGraph(stakeholders=_sh(), requirements=())
    result = gate.select_tier1("R-does-not-exist", g=g)
    assert not result.confident
    assert "not found" in result.reason.lower()


def test_fails_closed_on_conflict_target() -> None:
    g = TensionGraph(
        stakeholders=_sh(),
        requirements=(
            Requirement(id="R-a", claim="a", owner="ST-a", status="SETTLED"),
            Requirement(id="R-b", claim="b", owner="ST-b", status="SETTLED"),
        ),
        conflicts=(
            Conflict(
                id="C-x",
                axis="ax",
                context="ctx",
                members=("R-a", "R-b"),
                steward="ST-a",
                lifecycle="DETECTED",
            ),
        ),
    )
    result = gate.select_tier1("C-x", g=g)
    assert not result.confident
    assert "conflict" in result.reason.lower()


# ---------------------------------------------------------------------------
# 6. Determinism
# ---------------------------------------------------------------------------


def test_selection_is_deterministic() -> None:
    g = TensionGraph(
        stakeholders=_sh(),
        requirements=(
            Requirement(
                id="R-x",
                claim="c",
                owner="ST-a",
                status="SETTLED",
                enforced_by=("test_smoke.py::test_smoke", "check_axis_in_registry"),
            ),
        ),
    )
    r1 = gate.select_tier1("R-x", g=g)
    r2 = gate.select_tier1("R-x", g=g)
    assert r1.confident and r2.confident
    assert r1.node_ids == r2.node_ids


# ---------------------------------------------------------------------------
# 7. CLI
# ---------------------------------------------------------------------------


def test_cli_prints_node_ids_on_real_settled_requirement(capsys) -> None:
    rc = gate.main(["R-smoke-test"])
    out = capsys.readouterr().out
    assert rc == 0
    assert "tests/test_smoke.py" in out


def test_cli_fails_closed_on_conflict(capsys) -> None:
    rc = gate.main(["C-8600b1b8"])
    captured = capsys.readouterr()
    assert rc == 1
    assert "FAIL-CLOSED" in captured.err
