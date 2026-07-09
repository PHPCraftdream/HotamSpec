"""Tests for spec/tools/spawn_log_isolation_status.py
(R-parallel-mutating-agents-use-worktree, log-internal slice).

Verifies compute_isolation_status()'s reading of
spec/.runtime/spawn-log.jsonl and the CLI's exit code / --json surface,
across: absent log, empty log, clean records, flagged records, and
pre-Wave-5 records missing the isolation/mutating fields entirely (must
default to non-flagged). Hermetic tmp_path logs throughout — never touches
the real spec/.runtime/spawn-log.jsonl.
"""

from __future__ import annotations

import json
from pathlib import Path


import spawn_log_isolation_status as sl_status  # noqa: E402


def _write_log(path: Path, records: list[dict]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="\n") as fh:
        for r in records:
            fh.write(json.dumps(r) + "\n")


def _rec(stamp: str, *, isolation: str = "shared", mutating: bool = False) -> dict:
    return {
        "stamp": stamp,
        "agent": "domains/x/agents/y",
        "task_first_line": "do a thing",
        "prompt_chars": 42,
        "isolation": isolation,
        "mutating": mutating,
    }


# ---------------------------------------------------------------------------
# compute_isolation_status
# ---------------------------------------------------------------------------


def test_absent_log_is_clean(tmp_path: Path) -> None:
    """No log file at all -> clean (vacuously, nothing spawned yet)."""
    log_path = tmp_path / "spawn-log.jsonl"
    result = sl_status.compute_isolation_status(log_path)
    assert result.clean is True
    assert result.flagged_stamps == ()


def test_empty_file_is_clean(tmp_path: Path) -> None:
    """A log file that exists but is empty -> clean."""
    log_path = tmp_path / "spawn-log.jsonl"
    log_path.write_text("", encoding="utf-8")
    result = sl_status.compute_isolation_status(log_path)
    assert result.clean is True


def test_worktree_mutating_is_clean(tmp_path: Path) -> None:
    """mutating=true with isolation=worktree honors the policy -> clean."""
    log_path = tmp_path / "spawn-log.jsonl"
    _write_log(
        log_path,
        [_rec("2026-07-02T00:00:00Z", isolation="worktree", mutating=True)],
    )
    result = sl_status.compute_isolation_status(log_path)
    assert result.clean is True


def test_non_mutating_shared_is_clean(tmp_path: Path) -> None:
    """mutating=false with isolation=shared is fine (default, no risk)."""
    log_path = tmp_path / "spawn-log.jsonl"
    _write_log(
        log_path,
        [_rec("2026-07-02T00:00:00Z", isolation="shared", mutating=False)],
    )
    result = sl_status.compute_isolation_status(log_path)
    assert result.clean is True


def test_mutating_shared_is_flagged(tmp_path: Path) -> None:
    """mutating=true with isolation=shared is exactly the flagged condition."""
    log_path = tmp_path / "spawn-log.jsonl"
    _write_log(
        log_path,
        [_rec("2026-07-02T01:00:00Z", isolation="shared", mutating=True)],
    )
    result = sl_status.compute_isolation_status(log_path)
    assert result.clean is False
    assert result.flagged_stamps == ("2026-07-02T01:00:00Z",)


def test_mixed_records_flags_only_offenders(tmp_path: Path) -> None:
    log_path = tmp_path / "spawn-log.jsonl"
    _write_log(
        log_path,
        [
            _rec("2026-07-02T00:00:00Z", isolation="worktree", mutating=True),
            _rec("2026-07-02T00:01:00Z", isolation="shared", mutating=False),
            _rec("2026-07-02T00:02:00Z", isolation="shared", mutating=True),
        ],
    )
    result = sl_status.compute_isolation_status(log_path)
    assert result.clean is False
    assert result.flagged_stamps == ("2026-07-02T00:02:00Z",)


def test_pre_wave5_record_missing_fields_defaults_to_not_flagged(
    tmp_path: Path,
) -> None:
    """A record written before R-spawn-log-carries-isolation existed (no
    isolation/mutating keys at all) must NOT be flagged -- absence means
    'not declared mutating', not 'assume worst case'."""
    log_path = tmp_path / "spawn-log.jsonl"
    legacy_record = {
        "stamp": "2026-06-29T12:00:00Z",
        "agent": "domains/x/agents/y",
        "task_first_line": "old task",
        "prompt_chars": 10,
    }
    _write_log(log_path, [legacy_record])
    result = sl_status.compute_isolation_status(log_path)
    assert result.clean is True


# ---------------------------------------------------------------------------
# CLI (main)
# ---------------------------------------------------------------------------


def test_main_exit_0_when_clean(tmp_path: Path, capsys) -> None:
    log_path = tmp_path / "spawn-log.jsonl"
    rc = sl_status.main(["--log-path", str(log_path)])
    assert rc == 0
    out = capsys.readouterr().out
    assert "CLEAN" in out


def test_main_exit_1_when_flagged(tmp_path: Path, capsys) -> None:
    log_path = tmp_path / "spawn-log.jsonl"
    _write_log(
        log_path,
        [_rec("2026-07-02T02:00:00Z", isolation="shared", mutating=True)],
    )
    rc = sl_status.main(["--log-path", str(log_path)])
    assert rc == 1
    out = capsys.readouterr().out
    assert "FLAGGED" in out


def test_main_json_output(tmp_path: Path, capsys) -> None:
    log_path = tmp_path / "spawn-log.jsonl"
    _write_log(
        log_path,
        [_rec("2026-07-02T03:00:00Z", isolation="shared", mutating=True)],
    )
    rc = sl_status.main(["--log-path", str(log_path), "--json"])
    assert rc == 1
    payload = json.loads(capsys.readouterr().out)
    assert payload["clean"] is False
    assert payload["flagged_stamps"] == ["2026-07-02T03:00:00Z"]
