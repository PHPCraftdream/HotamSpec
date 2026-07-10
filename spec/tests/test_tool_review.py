"""Tests for spec/tools/review.py — the thin CLI dispatcher over
audit_tensions.py / mark_revisit_evaluated.py / spawn_log_isolation_status.py
(task #106 / L2-#5, R-shared-tools-in-spec-tools).

review.py does not reimplement any of the three tools' logic — it only
forwards a subcommand's argv to the existing tool, exactly like
tools/land.py does for gate.py/gate_status.py/closure.py. These tests cover
the dispatch surface itself (unknown subcommand, --help, exit codes
forwarded verbatim) plus one live call per subcommand to confirm the wiring
reaches the real implementation.
"""

from __future__ import annotations

import json

import review  # noqa: E402


def test_review_no_args_prints_usage_and_fails() -> None:
    """No subcommand -> usage printed, non-zero exit (distinct from --help)."""
    assert review.main([]) == 2


def test_review_help_prints_usage_and_succeeds() -> None:
    """--help -> usage printed, exit 0."""
    assert review.main(["--help"]) == 0
    assert review.main(["-h"]) == 0


def test_review_unknown_subcommand_fails_closed() -> None:
    """An unrecognized subcommand is rejected, not silently ignored."""
    assert review.main(["frobnicate"]) == 2


def test_tensions_dispatches_to_audit_tensions(capsys) -> None:
    """`review.py tensions --no-stamp` reaches audit_tensions.audit (no write)."""
    exit_code = review.main(["tensions", "--no-stamp", "--limit", "1"])
    assert exit_code == 0
    out = capsys.readouterr().out
    assert "audit_tensions" in out or "generative shortlist" in out


def test_revisit_dispatches_to_mark_revisit_evaluated(tmp_path, monkeypatch) -> None:
    """`review.py revisit <id> --dry-run` reaches mark_revisit_evaluated.main."""
    import mark_revisit_evaluated as mre  # noqa: PLC0415

    monkeypatch.setattr(mre, "REVISIT_EVAL_FILE", tmp_path / "revisit-eval.jsonl")

    exit_code = review.main(["revisit", "C-does-not-exist-xyz", "--dry-run"])
    assert exit_code == 1  # unknown conflict id -> mark_revisit_evaluated's own fail path


def test_spawn_isolation_dispatches_to_spawn_log_isolation_status(tmp_path, capsys) -> None:
    """`review.py spawn-isolation --json --log-path <empty>` reaches
    spawn_log_isolation_status.compute_isolation_status."""
    log_path = tmp_path / "spawn-log.jsonl"
    exit_code = review.main(["spawn-isolation", "--json", "--log-path", str(log_path)])
    assert exit_code == 0  # absent log -> vacuously clean, per that tool's own policy
    out = capsys.readouterr().out
    data = json.loads(out)
    assert data["clean"] is True
