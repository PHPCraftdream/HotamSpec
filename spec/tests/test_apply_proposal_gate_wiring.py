"""Tests: tools/apply_proposal.py's LAND-verify step is gated by tools/gate.py (Stage B).

Canon: R-land-gate-tier-selector, R-tiered-gate-not-a-commit-gate.

Approach: mock subprocess.run so no real gen_spec/pytest subprocess runs
against the real repo (write happens on an isolated tmp_path copy of
graph.py); assert the pytest_args apply() builds match the T1/T2 tier the
scenario calls for. This is a wiring test — it does NOT re-test gate.py's
own mapping/fail-closed logic (covered by test_tool_gate.py) or apply()'s
AST-splice logic (covered by test_apply_proposal.py); it tests only that
apply() actually CALLS gate.select_tier1(target) and uses its result to
build the pytest invocation, honoring --full / full_suite=True as an
explicit override.
"""

from __future__ import annotations

import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import apply_proposal  # noqa: E402
from hotam_spec.proposal import ProposedRejection, ProposedRequirement  # noqa: E402

_SAMPLE_SOURCE = '''\
from hotam_spec.requirement import Requirement

requirements = (
    Requirement(
        id="R-sample-target",
        claim="a sample claim for gate wiring tests",
        owner="framework-author",
        status="SETTLED",
        why="sample",
    ),
)
'''

_SAMPLE_SOURCE_WITH_BUILD_GRAPH = '''\
from hotam_spec.requirement import Requirement


def build_graph():
    requirements = (
        Requirement(
            id="R-sample-target",
            claim="a sample claim for gate wiring tests",
            owner="framework-author",
            status="SETTLED",
            why="sample",
        ),
    )
    return requirements
'''


def _fake_subprocess_run_factory(pytest_argv_capture: list[list[str]]) -> MagicMock:
    """Build a subprocess.run replacement: gen_spec call succeeds, pytest call
    succeeds AND is captured so the test can assert on its argv."""

    def _fake_run(args, **kwargs):  # noqa: ANN001, ANN003
        result = MagicMock()
        result.returncode = 0
        result.stdout = ""
        result.stderr = ""
        if any("pytest" in str(a) for a in args):
            pytest_argv_capture.append(list(args))
        return result

    return MagicMock(side_effect=_fake_run)


def test_apply_uses_t1_targeted_selection_when_gate_confident(tmp_path: Path) -> None:
    """apply() with full_suite=False (default) calls gate.select_tier1 and, when
    confident, passes ONLY the resolved node-ids to pytest -- not the whole
    tests/ directory."""
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_SOURCE, encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample_file
    pytest_argv_capture: list[list[str]] = []
    try:
        proposal = ProposedRequirement(
            id="R-sample-target",
            claim="a sample claim for gate wiring tests (updated)",
            owner="framework-author",
            status="SETTLED",
            why="sample (updated)",
        )
        with (
            patch.object(
                apply_proposal, "subprocess", MagicMock(
                    run=_fake_subprocess_run_factory(pytest_argv_capture)
                )
            ),
            patch(
                "gate.select_tier1",
                return_value=__import__("gate").GateResult(
                    confident=True,
                    node_ids=("tests/test_smoke.py::test_smoke",),
                    reason="fake confident selection for wiring test",
                ),
            ),
        ):
            result = apply_proposal.apply(proposal, full_suite=False)
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert result == 0
    assert len(pytest_argv_capture) == 1
    argv = pytest_argv_capture[0]
    assert "tests/test_smoke.py::test_smoke" in argv
    # T1 must NOT pass the whole tests/ directory when confident.
    assert not any(str(a).rstrip("/\\").endswith("tests") for a in argv)


def test_apply_falls_back_to_full_suite_when_gate_uncertain(tmp_path: Path) -> None:
    """apply() with full_suite=False falls back to the whole tests/ dir when
    gate.select_tier1 returns confident=False (fail-closed)."""
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_SOURCE, encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample_file
    pytest_argv_capture: list[list[str]] = []
    try:
        proposal = ProposedRequirement(
            id="R-sample-target",
            claim="a sample claim for gate wiring tests (updated)",
            owner="framework-author",
            status="SETTLED",
            why="sample (updated)",
        )
        with (
            patch.object(
                apply_proposal, "subprocess", MagicMock(
                    run=_fake_subprocess_run_factory(pytest_argv_capture)
                )
            ),
            patch(
                "gate.select_tier1",
                return_value=__import__("gate").GateResult(
                    confident=False,
                    node_ids=(),
                    reason="fake fail-closed for wiring test",
                ),
            ),
        ):
            result = apply_proposal.apply(proposal, full_suite=False)
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert result == 0
    assert len(pytest_argv_capture) == 1
    argv = pytest_argv_capture[0]
    assert any(str(a).rstrip("/\\").endswith("tests") for a in argv)


def test_apply_rejection_always_uses_full_suite(tmp_path: Path) -> None:
    """A ProposedRejection MUST always verify with T2 (full suite), even when
    gate.select_tier1 would be confident for the target -- a rejected atom's
    own enforced_by does not bound its removal blast radius."""
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_SOURCE, encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample_file
    pytest_argv_capture: list[list[str]] = []
    try:
        proposal = ProposedRejection(
            requirement_id="R-sample-target",
            reason="REJECTED — superseded for wiring test",
        )
        with (
            patch.object(
                apply_proposal, "subprocess", MagicMock(
                    run=_fake_subprocess_run_factory(pytest_argv_capture)
                )
            ),
            patch(
                "gate.select_tier1",
                return_value=__import__("gate").GateResult(
                    confident=True,
                    node_ids=("tests/test_smoke.py::test_smoke",),
                    reason="would-be-confident selection; must not be consulted",
                ),
            ) as mock_select,
        ):
            result = apply_proposal.apply(proposal, full_suite=False)
            mock_select.assert_not_called()
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert result == 0
    assert len(pytest_argv_capture) == 1
    argv = pytest_argv_capture[0]
    assert any(str(a).rstrip("/\\").endswith("tests") for a in argv), (
        "ProposedRejection must always select the full tests/ directory (T2)"
    )


def test_apply_new_requirement_creation_always_uses_full_suite(tmp_path: Path) -> None:
    """A ProposedRequirement that ADDS a brand-new node (not an update of an
    existing one) MUST always verify with T2 -- a new node's blast radius
    cannot be trusted from an absent or steward-supplied enforced_by tuple."""
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_SOURCE_WITH_BUILD_GRAPH, encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample_file
    pytest_argv_capture: list[list[str]] = []
    try:
        proposal = ProposedRequirement(
            id="R-brand-new-node",
            claim="a brand new requirement for wiring test",
            owner="framework-author",
            status="DRAFT",
            why="new node creation must fail closed to T2",
        )
        with (
            patch.object(
                apply_proposal, "subprocess", MagicMock(
                    run=_fake_subprocess_run_factory(pytest_argv_capture)
                )
            ),
            patch(
                "gate.select_tier1",
                return_value=__import__("gate").GateResult(
                    confident=True,
                    node_ids=("tests/test_smoke.py::test_smoke",),
                    reason="would-be-confident selection; must not be consulted",
                ),
            ) as mock_select,
        ):
            result = apply_proposal.apply(proposal, full_suite=False)
            mock_select.assert_not_called()
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert result == 0
    assert len(pytest_argv_capture) == 1
    argv = pytest_argv_capture[0]
    assert any(str(a).rstrip("/\\").endswith("tests") for a in argv), (
        "new Requirement creation must always select the full tests/ directory (T2)"
    )


def test_apply_existing_requirement_update_still_uses_gate(tmp_path: Path) -> None:
    """A ProposedRequirement that UPDATES an already-present node still goes
    through gate.select_tier1 as before (target_preexisting=True does not
    change the T1-eligible path)."""
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_SOURCE, encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample_file
    pytest_argv_capture: list[list[str]] = []
    try:
        proposal = ProposedRequirement(
            id="R-sample-target",
            claim="a sample claim for gate wiring tests (updated again)",
            owner="framework-author",
            status="SETTLED",
            why="sample (updated again)",
        )
        with (
            patch.object(
                apply_proposal, "subprocess", MagicMock(
                    run=_fake_subprocess_run_factory(pytest_argv_capture)
                )
            ),
            patch(
                "gate.select_tier1",
                return_value=__import__("gate").GateResult(
                    confident=True,
                    node_ids=("tests/test_smoke.py::test_smoke",),
                    reason="fake confident selection for wiring test",
                ),
            ) as mock_select,
        ):
            result = apply_proposal.apply(proposal, full_suite=False)
            mock_select.assert_called_once()
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert result == 0
    assert len(pytest_argv_capture) == 1
    argv = pytest_argv_capture[0]
    assert "tests/test_smoke.py::test_smoke" in argv
    assert not any(str(a).rstrip("/\\").endswith("tests") for a in argv)


def test_apply_full_flag_skips_gate_entirely(tmp_path: Path) -> None:
    """apply(full_suite=True) never consults gate.select_tier1 -- always T2."""
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_SOURCE, encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample_file
    pytest_argv_capture: list[list[str]] = []
    try:
        proposal = ProposedRequirement(
            id="R-sample-target",
            claim="a sample claim for gate wiring tests (updated)",
            owner="framework-author",
            status="SETTLED",
            why="sample (updated)",
        )
        with (
            patch.object(
                apply_proposal, "subprocess", MagicMock(
                    run=_fake_subprocess_run_factory(pytest_argv_capture)
                )
            ),
            patch("gate.select_tier1") as mock_select,
        ):
            result = apply_proposal.apply(proposal, full_suite=True)
            mock_select.assert_not_called()
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert result == 0
    assert len(pytest_argv_capture) == 1
    argv = pytest_argv_capture[0]
    assert any(str(a).rstrip("/\\").endswith("tests") for a in argv)
