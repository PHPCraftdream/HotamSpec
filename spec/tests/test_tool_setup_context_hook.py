"""Tests for spec/tools/setup_context_hook.py + spec/tools/context_producer.py.

Hermetic: never touches the real project .claude/settings.local.json or the
real spec/.runtime/context.json, and NEVER reads the real ~/.claude. All
settings-file operations run against a tmp_path copy via monkeypatched
module-level path constants. The producer's ONLY source is the local stdin
payload (R-work-within-launch-dir) — there is no host cache to read.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path


import setup_context_hook as sch  # noqa: E402
import context_producer as producer  # noqa: E402


# ---------------------------------------------------------------------------
# setup_context_hook: install / merge-preserves-foreign-hooks / off / idempotent
# ---------------------------------------------------------------------------


def _settings_path(tmp_path: Path) -> Path:
    return tmp_path / ".claude" / "settings.local.json"


def test_install_on_empty_settings_adds_posttooluse(tmp_path, monkeypatch):
    settings_path = _settings_path(tmp_path)
    monkeypatch.setattr(sch, "_SETTINGS_LOCAL", settings_path)

    data = sch.install({})
    sch._write_settings(data)

    assert settings_path.exists()
    written = json.loads(settings_path.read_text(encoding="utf-8"))
    assert "PostToolUse" in written["hooks"]
    # Stop context_producer now lives in committed settings.json, not local
    assert "Stop" not in written["hooks"]
    assert sch._MARKER in written["hooks"]["PostToolUse"][0]["hooks"][0]["command"]


def test_install_preserves_foreign_hooks_and_permissions(tmp_path, monkeypatch):
    settings_path = _settings_path(tmp_path)
    monkeypatch.setattr(sch, "_SETTINGS_LOCAL", settings_path)

    foreign = {
        "hooks": {
            "SessionStart": [
                {"hooks": [{"type": "command", "command": "echo session-start"}]}
            ],
            "PostToolUse": [
                {
                    "matcher": "Edit|Write",
                    "hooks": [{"type": "command", "command": "echo foreign-posttooluse"}],
                }
            ],
        },
        "permissions": {"allow": ["Bash(git status)"]},
    }

    data = sch.install(foreign)
    sch._write_settings(data)

    written = json.loads(settings_path.read_text(encoding="utf-8"))
    # foreign entries untouched
    assert written["hooks"]["SessionStart"][0]["hooks"][0]["command"] == "echo session-start"
    assert any(
        g.get("matcher") == "Edit|Write"
        and g["hooks"][0]["command"] == "echo foreign-posttooluse"
        for g in written["hooks"]["PostToolUse"]
    )
    assert written["permissions"] == {"allow": ["Bash(git status)"]}
    # our entries added alongside
    assert any(sch._MARKER in g["hooks"][0]["command"] for g in written["hooks"]["PostToolUse"])
    # Stop context_producer now lives in committed settings.json, not local
    assert "Stop" not in written["hooks"]


def test_install_twice_is_idempotent(tmp_path, monkeypatch):
    settings_path = _settings_path(tmp_path)
    monkeypatch.setattr(sch, "_SETTINGS_LOCAL", settings_path)

    data = sch.install({})
    data = sch.install(data)  # second install, same in-memory dict
    sch._write_settings(data)

    written = json.loads(settings_path.read_text(encoding="utf-8"))
    post_marker_groups = [
        g for g in written["hooks"]["PostToolUse"] if sch._has_marker(g)
    ]
    assert len(post_marker_groups) == 1
    # Stop not present in local (lives in committed settings.json)
    assert "Stop" not in written["hooks"]


def test_off_removes_only_our_entries(tmp_path, monkeypatch):
    settings_path = _settings_path(tmp_path)
    monkeypatch.setattr(sch, "_SETTINGS_LOCAL", settings_path)

    foreign = {
        "hooks": {
            "PostToolUse": [
                {
                    "matcher": "Edit|Write",
                    "hooks": [{"type": "command", "command": "echo foreign"}],
                }
            ]
        }
    }
    data = sch.install(foreign)
    sch._write_settings(data)

    loaded = sch._load_settings()
    data = sch.uninstall(loaded)
    sch._write_settings(data)

    written = json.loads(settings_path.read_text(encoding="utf-8"))
    # foreign PostToolUse group survives
    assert any(
        g.get("matcher") == "Edit|Write" for g in written["hooks"]["PostToolUse"]
    )
    # our marker group is gone
    assert not any(sch._has_marker(g) for g in written["hooks"]["PostToolUse"])
    # Stop key removed entirely (only contained our entry)
    assert "Stop" not in written.get("hooks", {})


def test_hook_command_never_names_the_host(tmp_path, monkeypatch):
    """The installed command touches only the launch dir — no ~/.claude, no uv."""
    monkeypatch.setattr(sch, "_SETTINGS_LOCAL", _settings_path(tmp_path))
    cmd = sch._hook_command()
    assert ".claude" not in cmd.replace(".claude/settings.local.json", "")  # no home ~/.claude
    assert "patch-global" not in cmd
    assert "context-cache" not in cmd


def test_status_reports_not_installed_when_absent(tmp_path, monkeypatch):
    settings_path = _settings_path(tmp_path)
    monkeypatch.setattr(sch, "_SETTINGS_LOCAL", settings_path)
    monkeypatch.setattr(sch, "_RUNTIME_CONTEXT", tmp_path / "context.json")

    report = sch.status_report()
    assert "installed: no" in report
    assert "context.json: absent" in report


def test_status_reports_installed_and_context_age(tmp_path, monkeypatch):
    settings_path = _settings_path(tmp_path)
    monkeypatch.setattr(sch, "_SETTINGS_LOCAL", settings_path)
    ctx_file = tmp_path / "context.json"
    ctx_file.write_text(json.dumps({"ctx_pct": 10}), encoding="utf-8")
    monkeypatch.setattr(sch, "_RUNTIME_CONTEXT", ctx_file)

    data = sch.install({})
    sch._write_settings(data)

    report = sch.status_report()
    assert "installed: yes" in report
    assert "context.json: present" in report


# ---------------------------------------------------------------------------
# context_producer: hermetic — local stdin payload is the ONLY source
# ---------------------------------------------------------------------------


def test_producer_writes_stamp_from_valid_payload(tmp_path, monkeypatch):
    runtime_file = tmp_path / "context.json"
    monkeypatch.setattr(producer, "_RUNTIME", runtime_file)

    wrote = producer.produce({"ctx_pct": 37.5, "model": "claude-sonnet-5"})

    assert wrote is True
    data = json.loads(runtime_file.read_text(encoding="utf-8"))
    assert data["ctx_pct"] == 37.5
    assert data["model"] == "claude-sonnet-5"
    assert "stamp" in data and data["stamp"]


def test_producer_skips_write_when_ctx_pct_missing(tmp_path, monkeypatch):
    runtime_file = tmp_path / "context.json"
    monkeypatch.setattr(producer, "_RUNTIME", runtime_file)

    wrote = producer.produce({"model": "claude-sonnet-5"})

    assert wrote is False
    assert not runtime_file.exists()


def test_producer_skips_write_when_ctx_pct_out_of_range(tmp_path, monkeypatch):
    runtime_file = tmp_path / "context.json"
    monkeypatch.setattr(producer, "_RUNTIME", runtime_file)

    wrote = producer.produce({"ctx_pct": 150})

    assert wrote is False
    assert not runtime_file.exists()


def test_producer_main_reads_stdin_file(tmp_path, monkeypatch):
    runtime_file = tmp_path / "context.json"
    monkeypatch.setattr(producer, "_RUNTIME", runtime_file)

    payload_file = tmp_path / "payload.json"
    payload_file.write_text(json.dumps({"ctx_pct": 12.0}), encoding="utf-8")

    monkeypatch.setattr(
        sys, "argv", ["context_producer.py", "--stdin-file", str(payload_file)]
    )
    rc = producer.main()

    assert rc == 0
    assert runtime_file.exists()
    data = json.loads(runtime_file.read_text(encoding="utf-8"))
    assert data["ctx_pct"] == 12.0


def test_producer_never_reads_host_cache(tmp_path, monkeypatch):
    """Empty payload writes nothing — there is no host cache fallback."""
    runtime_file = tmp_path / "context.json"
    monkeypatch.setattr(producer, "_RUNTIME", runtime_file)

    wrote = producer.produce({})

    assert wrote is False
    assert not runtime_file.exists()
    # The producer module carries no host-cache machinery.
    assert not hasattr(producer, "_read_global_cache")
    assert not hasattr(producer, "_GLOBAL_CACHE")
