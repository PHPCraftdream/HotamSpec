"""Tests for spec/tools/gate_status.py (R-tool-gate-status, R-commit-boundary-checkable).

Verifies compute_gate_status()'s reading of spec/.runtime/land-log.jsonl and
the CLI's exit code / --json surface, across: empty log, T1-then-T2 (ok),
T2-then-T1 (fails), and mixed multi-target scenarios. Hermetic tmp_path logs
throughout — never touches the real spec/.runtime/land-log.jsonl.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

TOOLS_DIR = Path(__file__).resolve().parents[1] / "tools"
if str(TOOLS_DIR) not in sys.path:
    sys.path.insert(0, str(TOOLS_DIR))

import gate_status  # noqa: E402


def _write_log(path: Path, records: list[dict]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="\n") as fh:
        for r in records:
            fh.write(json.dumps(r) + "\n")


def _rec(stamp: str, tier: str, target: str = "R-foo") -> dict:
    return {
        "stamp": stamp,
        "kind": "ProposedRequirement",
        "target": target,
        "tier": tier,
        "node_ids": ["tests/test_smoke.py::test_smoke"] if tier == "T1" else "full",
        "pytest_ok": True,
        "closure_exit": None,
    }


# ---------------------------------------------------------------------------
# compute_gate_status
# ---------------------------------------------------------------------------


def test_empty_log_is_satisfied(tmp_path: Path) -> None:
    """No log file at all -> satisfied (trivially, nothing has landed)."""
    log_path = tmp_path / "land-log.jsonl"
    result = gate_status.compute_gate_status(log_path)
    assert result.satisfied is True
    assert result.unverified_targets == ()


def test_empty_file_is_satisfied(tmp_path: Path) -> None:
    """A log file that exists but is empty -> satisfied."""
    log_path = tmp_path / "land-log.jsonl"
    log_path.write_text("", encoding="utf-8")
    result = gate_status.compute_gate_status(log_path)
    assert result.satisfied is True


def test_t1_then_t2_is_satisfied(tmp_path: Path) -> None:
    """A T1 land followed by a later T2 land -> boundary satisfied."""
    log_path = tmp_path / "land-log.jsonl"
    _write_log(
        log_path,
        [
            _rec("2026-07-01T10:00:00+00:00", "T1", target="R-a"),
            _rec("2026-07-01T11:00:00+00:00", "T2", target="R-a"),
        ],
    )
    result = gate_status.compute_gate_status(log_path)
    assert result.satisfied is True
    assert result.unverified_targets == ()


def test_t2_then_t1_is_not_satisfied(tmp_path: Path) -> None:
    """A T2 land followed by a LATER T1 land -> boundary NOT satisfied."""
    log_path = tmp_path / "land-log.jsonl"
    _write_log(
        log_path,
        [
            _rec("2026-07-01T10:00:00+00:00", "T2", target="R-a"),
            _rec("2026-07-01T11:00:00+00:00", "T1", target="R-b"),
        ],
    )
    result = gate_status.compute_gate_status(log_path)
    assert result.satisfied is False
    assert result.unverified_targets == ("R-b",)


def test_only_t1_records_never_t2_is_not_satisfied(tmp_path: Path) -> None:
    """Only T1 records exist, no T2 ever landed -> NOT satisfied; all T1 targets listed."""
    log_path = tmp_path / "land-log.jsonl"
    _write_log(
        log_path,
        [
            _rec("2026-07-01T10:00:00+00:00", "T1", target="R-a"),
            _rec("2026-07-01T10:05:00+00:00", "T1", target="R-b"),
        ],
    )
    result = gate_status.compute_gate_status(log_path)
    assert result.satisfied is False
    assert result.unverified_targets == ("R-a", "R-b")


def test_only_t2_records_is_satisfied(tmp_path: Path) -> None:
    """Only T2 records (no T1 at all) -> satisfied."""
    log_path = tmp_path / "land-log.jsonl"
    _write_log(
        log_path,
        [
            _rec("2026-07-01T10:00:00+00:00", "T2", target="R-a"),
        ],
    )
    result = gate_status.compute_gate_status(log_path)
    assert result.satisfied is True


def test_mixed_only_t1_after_last_t2_are_unverified(tmp_path: Path) -> None:
    """Mixed history: T1s before the last T2 are covered; T1s after are not."""
    log_path = tmp_path / "land-log.jsonl"
    _write_log(
        log_path,
        [
            _rec("2026-07-01T09:00:00+00:00", "T1", target="R-old"),
            _rec("2026-07-01T10:00:00+00:00", "T2", target="R-old"),
            _rec("2026-07-01T11:00:00+00:00", "T1", target="R-new1"),
            _rec("2026-07-01T12:00:00+00:00", "T1", target="R-new2"),
        ],
    )
    result = gate_status.compute_gate_status(log_path)
    assert result.satisfied is False
    assert result.unverified_targets == ("R-new1", "R-new2")


def test_malformed_lines_are_skipped(tmp_path: Path) -> None:
    """A corrupt line in the log does not crash the check; valid lines still count."""
    log_path = tmp_path / "land-log.jsonl"
    log_path.write_text(
        "not valid json\n"
        + json.dumps(_rec("2026-07-01T10:00:00+00:00", "T2", target="R-a"))
        + "\n",
        encoding="utf-8",
    )
    result = gate_status.compute_gate_status(log_path)
    assert result.satisfied is True


# ---------------------------------------------------------------------------
# CLI (main)
# ---------------------------------------------------------------------------


def test_cli_exit_0_on_satisfied(tmp_path: Path, capsys) -> None:  # noqa: ANN001
    log_path = tmp_path / "land-log.jsonl"
    _write_log(log_path, [])
    rc = gate_status.main(["--log-path", str(log_path)])
    assert rc == 0
    out = capsys.readouterr().out
    assert "SATISFIED" in out


def test_cli_exit_1_on_not_satisfied(tmp_path: Path, capsys) -> None:  # noqa: ANN001
    log_path = tmp_path / "land-log.jsonl"
    _write_log(
        log_path,
        [_rec("2026-07-01T10:00:00+00:00", "T1", target="R-unverified")],
    )
    rc = gate_status.main(["--log-path", str(log_path)])
    assert rc == 1
    out = capsys.readouterr().out
    assert "NOT SATISFIED" in out
    assert "R-unverified" in out


def test_cli_json_output(tmp_path: Path, capsys) -> None:  # noqa: ANN001
    log_path = tmp_path / "land-log.jsonl"
    _write_log(
        log_path,
        [_rec("2026-07-01T10:00:00+00:00", "T1", target="R-unverified")],
    )
    rc = gate_status.main(["--log-path", str(log_path), "--json"])
    assert rc == 1
    out = capsys.readouterr().out
    payload = json.loads(out)
    assert payload["satisfied"] is False
    assert payload["unverified_targets"] == ["R-unverified"]
    assert isinstance(payload["reason"], str) and payload["reason"]
