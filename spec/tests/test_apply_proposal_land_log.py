"""Tests for apply_proposal.py's land-log writer (R-land-tier-trace).

Verifies: record shape (stamp/kind/target/tier/node_ids/pytest_ok/closure_exit),
dry-run silence (no log write at all), and best-effort behavior when the
.runtime directory cannot be written (must warn, never raise / never affect
apply()'s return code). Mirrors test_tool_spawn_agent.py's hermetic tmp_path
style for spawn-log.jsonl.
"""

from __future__ import annotations

import json
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

import apply_proposal
from hotam_spec.proposal import ProposedRequirement

_SAMPLE_SOURCE = '''\
from hotam_spec.requirement import Requirement

requirements = (
    Requirement(
        id="R-sample-target",
        claim="a sample claim for land-log tests",
        owner="framework-author",
        status="SETTLED",
        why="sample",
    ),
)
'''


def _fake_subprocess_run_factory(pytest_argv_capture: list[list[str]]) -> MagicMock:
    def _fake_run(args, **kwargs):  # noqa: ANN001, ANN003
        result = MagicMock()
        result.returncode = 0
        result.stdout = ""
        result.stderr = ""
        if any("pytest" in str(a) for a in args):
            pytest_argv_capture.append(list(args))
        return result

    return MagicMock(side_effect=_fake_run)


def _run_apply_with_gate(
    tmp_path: Path,
    runtime_dir: Path,
    *,
    gate_confident: bool = True,
    gate_node_ids: tuple[str, ...] = ("tests/test_smoke.py::test_smoke",),
    triggering_kind: str | None = None,
    closure_advanced: bool = True,
) -> int:
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_SOURCE, encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    real_runtime = apply_proposal._RUNTIME_DIR
    apply_proposal._CONTENT_GRAPH = sample_file
    apply_proposal._RUNTIME_DIR = runtime_dir
    pytest_argv_capture: list[list[str]] = []
    try:
        proposal = ProposedRequirement(
            id="R-sample-target",
            claim="a sample claim for land-log tests (updated)",
            owner="framework-author",
            status="SETTLED",
            why="sample (updated)",
        )
        patches = [
            patch.object(
                apply_proposal,
                "subprocess",
                MagicMock(run=_fake_subprocess_run_factory(pytest_argv_capture)),
            ),
            patch(
                "gate.select_tier1",
                return_value=__import__("gate").GateResult(
                    confident=gate_confident,
                    node_ids=gate_node_ids if gate_confident else (),
                    reason="fake for land-log test",
                ),
            ),
        ]
        if triggering_kind is not None:
            fake_closure_result = MagicMock()
            fake_closure_result.advanced = closure_advanced
            fake_closure_result.target = "R-sample-target"
            fake_closure_result.triggering_kind = triggering_kind
            fake_closure_result.still_open_count = 0
            fake_closure_result.note = "fake"
            patches.append(
                patch(
                    "closure.check_closure",
                    return_value=fake_closure_result,
                )
            )
        with patches[0], patches[1]:
            if len(patches) == 3:
                with patches[2]:
                    result = apply_proposal.apply(
                        proposal,
                        triggering_kind=triggering_kind,
                        full_suite=False,
                    )
            else:
                result = apply_proposal.apply(
                    proposal, triggering_kind=triggering_kind, full_suite=False
                )
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph
        apply_proposal._RUNTIME_DIR = real_runtime
    return result


def _read_log(runtime_dir: Path) -> list[dict]:
    log_path = runtime_dir / apply_proposal._LAND_LOG_NAME
    if not log_path.exists():
        return []
    return [
        json.loads(line)
        for line in log_path.read_text(encoding="utf-8").splitlines()
        if line.strip()
    ]


def test_land_log_record_shape_t1(tmp_path: Path) -> None:
    """A T1-gated land appends one record with the expected fields/shape."""
    runtime_dir = tmp_path / ".runtime"
    rc = _run_apply_with_gate(tmp_path, runtime_dir, gate_confident=True)
    assert rc == 0

    entries = _read_log(runtime_dir)
    assert len(entries) == 1
    entry = entries[0]
    assert entry["kind"] == "ProposedRequirement"
    assert entry["target"] == "R-sample-target"
    assert entry["tier"] == "T1"
    assert entry["node_ids"] == ["tests/test_smoke.py::test_smoke"]
    assert entry["pytest_ok"] is True
    assert entry["closure_exit"] is None
    assert isinstance(entry["stamp"], str) and entry["stamp"]


def test_land_log_record_shape_t2(tmp_path: Path) -> None:
    """A T2 (fail-closed) land records tier='T2' and node_ids='full'."""
    runtime_dir = tmp_path / ".runtime"
    rc = _run_apply_with_gate(tmp_path, runtime_dir, gate_confident=False)
    assert rc == 0

    entries = _read_log(runtime_dir)
    assert len(entries) == 1
    entry = entries[0]
    assert entry["tier"] == "T2"
    assert entry["node_ids"] == "full"


def test_land_log_records_closure_exit(tmp_path: Path) -> None:
    """When --triggering-kind is supplied and closure advances, closure_exit == 0."""
    runtime_dir = tmp_path / ".runtime"
    rc = _run_apply_with_gate(
        tmp_path,
        runtime_dir,
        gate_confident=True,
        triggering_kind="OPEN_ITEM",
        closure_advanced=True,
    )
    assert rc == 0

    entries = _read_log(runtime_dir)
    assert len(entries) == 1
    assert entries[0]["closure_exit"] == 0


def test_land_log_records_closure_exit_2_on_not_advanced(tmp_path: Path) -> None:
    """When closure does NOT advance, apply() returns 2 and closure_exit == 2."""
    runtime_dir = tmp_path / ".runtime"
    rc = _run_apply_with_gate(
        tmp_path,
        runtime_dir,
        gate_confident=True,
        triggering_kind="OPEN_ITEM",
        closure_advanced=False,
    )
    assert rc == 2

    entries = _read_log(runtime_dir)
    assert len(entries) == 1
    assert entries[0]["closure_exit"] == 2
    assert entries[0]["pytest_ok"] is True


def test_dry_run_writes_no_log(tmp_path: Path) -> None:
    """--dry-run (apply(dry_run=True)) never writes to land-log.jsonl."""
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_SOURCE, encoding="utf-8")
    runtime_dir = tmp_path / ".runtime"

    real_graph = apply_proposal._CONTENT_GRAPH
    real_runtime = apply_proposal._RUNTIME_DIR
    apply_proposal._CONTENT_GRAPH = sample_file
    apply_proposal._RUNTIME_DIR = runtime_dir
    try:
        proposal = ProposedRequirement(
            id="R-sample-target",
            claim="a sample claim for land-log tests (dry-run)",
            owner="framework-author",
            status="SETTLED",
            why="sample (dry-run)",
        )
        rc = apply_proposal.apply(proposal, dry_run=True)
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph
        apply_proposal._RUNTIME_DIR = real_runtime

    assert rc == 0
    assert not (runtime_dir / apply_proposal._LAND_LOG_NAME).exists()
    assert not runtime_dir.exists() or not any(runtime_dir.iterdir())


def test_land_log_write_failure_is_best_effort(
    tmp_path: Path, capsys: pytest.CaptureFixture
) -> None:
    """If the log directory cannot be created/written, apply() still succeeds
    (best-effort logging) and a WARNING is printed to stderr."""
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_SOURCE, encoding="utf-8")

    # Point runtime dir at a path that cannot be a directory (a file occupies it).
    blocked_path = tmp_path / "blocked-runtime"
    blocked_path.write_text("occupied", encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    real_runtime = apply_proposal._RUNTIME_DIR
    apply_proposal._CONTENT_GRAPH = sample_file
    apply_proposal._RUNTIME_DIR = blocked_path
    pytest_argv_capture: list[list[str]] = []
    try:
        proposal = ProposedRequirement(
            id="R-sample-target",
            claim="a sample claim for land-log tests (unwritable dir)",
            owner="framework-author",
            status="SETTLED",
            why="sample (unwritable)",
        )
        with (
            patch.object(
                apply_proposal,
                "subprocess",
                MagicMock(run=_fake_subprocess_run_factory(pytest_argv_capture)),
            ),
            patch(
                "gate.select_tier1",
                return_value=__import__("gate").GateResult(
                    confident=True,
                    node_ids=("tests/test_smoke.py::test_smoke",),
                    reason="fake for unwritable-dir test",
                ),
            ),
        ):
            rc = apply_proposal.apply(proposal, full_suite=False)
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph
        apply_proposal._RUNTIME_DIR = real_runtime

    assert rc == 0, "a broken log directory must never fail an otherwise-green apply"
    captured = capsys.readouterr()
    assert "WARNING" in captured.err
    assert "land-log" in captured.err
