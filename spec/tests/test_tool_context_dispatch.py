"""Tests for spec/tools/context.py's CLI dispatcher (status/produce/install).

context.py's reader half (read_context/render_line) is covered by
test_tool_context.py; this file covers only the land.py-style dispatch
surface added in task #106 (L2-#4): the CLI does not reimplement
context_producer.py / setup_context_hook.py, it forwards argv to them.
"""

from __future__ import annotations

import json

import pytest

import context  # noqa: E402


def test_no_args_defaults_to_status(capsys) -> None:
    """No subcommand -> defaults to `status` (prints the cipher line)."""
    exit_code = context.main([])
    assert exit_code == 0
    out = capsys.readouterr().out
    assert out.startswith("context:")


def test_context_help_prints_usage_and_succeeds() -> None:
    """--help -> usage printed, exit 0."""
    assert context.main(["--help"]) == 0
    assert context.main(["-h"]) == 0


def test_status_subcommand_prints_cipher_line(capsys) -> None:
    exit_code = context.main(["status"])
    assert exit_code == 0
    out = capsys.readouterr().out
    assert out.startswith("context:")


def test_produce_dispatches_to_context_producer(tmp_path, monkeypatch) -> None:
    """`context.py produce --stdin-file P` reaches context_producer.produce."""
    import context_producer as producer  # noqa: PLC0415

    runtime_file = tmp_path / "context.json"
    monkeypatch.setattr(producer, "_RUNTIME", runtime_file)

    payload_file = tmp_path / "payload.json"
    payload_file.write_text(json.dumps({"ctx_pct": 55.0}), encoding="utf-8")

    exit_code = context.main(["produce", "--stdin-file", str(payload_file)])
    assert exit_code == 0
    assert runtime_file.exists()
    data = json.loads(runtime_file.read_text(encoding="utf-8"))
    assert data["ctx_pct"] == 55.0


def test_install_dispatches_to_setup_context_hook(tmp_path, monkeypatch) -> None:
    """`context.py install --status` reaches setup_context_hook.status_report."""
    import setup_context_hook as sch  # noqa: PLC0415

    monkeypatch.setattr(sch, "_SETTINGS_LOCAL", tmp_path / ".claude" / "settings.local.json")
    monkeypatch.setattr(sch, "_RUNTIME_CONTEXT", tmp_path / "context.json")

    exit_code = context.main(["install", "--status"])
    assert exit_code == 0


def test_unknown_subcommand_falls_through_to_status_argparse() -> None:
    """An unrecognized first token falls through to the status default
    (there is no separate fail-closed branch for typos — any token that is
    not `status`/`produce`/`install` is handed to status's own argparse,
    which rejects the unexpected positional the same way any bad flag
    would). SystemExit(2) is argparse's own behavior, not a dispatcher path."""
    with pytest.raises(SystemExit) as exc_info:
        context.main(["frobnicate"])
    assert exc_info.value.code == 2
