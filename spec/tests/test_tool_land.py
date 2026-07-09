"""Tests for spec/tools/land.py — the thin CLI dispatcher over gate.py /
gate_status.py / closure.py (C3 consolidation, R-shared-tools-in-spec-tools).

land.py does not reimplement any selection/status/closure logic — it only
forwards a subcommand's argv to the existing tool. These tests cover the
dispatch surface itself (unknown subcommand, --help, exit codes forwarded
verbatim) plus one live call per subcommand to confirm the wiring reaches
the real implementation.
"""

from __future__ import annotations

import land  # noqa: E402


def test_no_args_prints_usage_and_fails() -> None:
    """No subcommand -> usage printed, non-zero exit (distinct from --help)."""
    assert land.main([]) == 2


def test_help_prints_usage_and_succeeds() -> None:
    """--help -> usage printed, exit 0."""
    assert land.main(["--help"]) == 0
    assert land.main(["-h"]) == 0


def test_unknown_subcommand_fails_closed() -> None:
    """An unrecognized subcommand is rejected, not silently ignored."""
    assert land.main(["frobnicate"]) == 2


def test_select_dispatches_to_gate() -> None:
    """`land.py select <target>` reaches gate.select_tier1 for a known target."""
    exit_code = land.main(["select", "R-smoke-test"])
    assert exit_code in (0, 1)  # confident (0) or fail-closed (1) — both are real gate.py outcomes


def test_select_unknown_target_fails_closed() -> None:
    """A target absent from the graph -> gate.py's own fail-closed exit 1."""
    assert land.main(["select", "R-does-not-exist-xyz"]) == 1


def test_status_dispatches_to_gate_status(tmp_path) -> None:
    """`land.py status --log-path <empty>` reaches gate_status.compute_gate_status."""
    log_path = tmp_path / "land-log.jsonl"
    exit_code = land.main(["status", "--log-path", str(log_path)])
    assert exit_code == 1  # no log at all -> NOT satisfied, per gate_status's own fail-closed policy


def test_verify_closure_dispatches_to_closure() -> None:
    """`land.py verify-closure <target> <kind>` reaches closure.check_closure."""
    exit_code = land.main(["verify-closure", "R-smoke-test", "P4"])
    assert exit_code in (0, 2)  # advanced (0) or not-advanced (2) — both are real closure.py outcomes
