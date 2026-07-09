"""Tests for tools/setup_hooks.py — the committable sensorium (R-sensorium-committed).

Covers:
  (a) build_settings() emits the universal hook set, portable via $CLAUDE_PROJECT_DIR.
  (b) dry-run (default) writes NOTHING.
  (c) --apply writes a valid settings.json; a second --apply is idempotent
      (byte-identical) and backs up the prior file.
  (d) redundant_local_commands() matches local hooks by tool filename.
"""

from __future__ import annotations

import json


import setup_hooks  # noqa: E402


def test_build_settings_has_universal_events() -> None:
    hooks = setup_hooks.build_settings()["hooks"]
    assert set(hooks) == {
        "SessionStart",
        "PostCompact",
        "UserPromptSubmit",
        "PreToolUse",
        "Stop",
    }


def test_build_settings_commands_are_portable() -> None:
    settings = setup_hooks.build_settings()
    commands = list(setup_hooks._iter_commands(settings))
    assert commands, "expected at least one hook command"
    for cmd in commands:
        # Portable: anchored on the exported project dir, never a hardcoded path.
        assert "$CLAUDE_PROJECT_DIR" in cmd
        assert "D:/dev/HotamSpec" not in cmd
        assert setup_hooks._MARKER in cmd


def test_build_settings_covers_every_universal_tool() -> None:
    settings = setup_hooks.build_settings()
    tools = {setup_hooks._tool_name(c) for c in setup_hooks._iter_commands(settings)}
    assert {
        "gen_spec",
        "emit_cipher",
        "claude_md_diff_watch",
        "_graph_guard",
        "context_producer",
    } <= tools


def test_pretooluse_guards_graph_edits() -> None:
    hooks = setup_hooks.build_settings()["hooks"]
    pre = hooks["PreToolUse"]
    assert pre[0]["matcher"] == "Edit|Write"
    assert setup_hooks._tool_name(pre[0]["hooks"][0]["command"]) == "_graph_guard"


def test_dry_run_writes_nothing(monkeypatch, tmp_path) -> None:
    target = tmp_path / ".claude" / "settings.json"
    monkeypatch.setattr(setup_hooks, "_PROJECT_SETTINGS", target)
    rc = setup_hooks.main([])  # no --apply
    assert rc == 0
    assert not target.exists()


def test_apply_writes_valid_json_and_is_idempotent(monkeypatch, tmp_path) -> None:
    target = tmp_path / ".claude" / "settings.json"
    local = tmp_path / ".claude" / "settings.local.json"
    monkeypatch.setattr(setup_hooks, "_PROJECT_SETTINGS", target)
    monkeypatch.setattr(setup_hooks, "_SETTINGS_LOCAL", local)

    rc = setup_hooks.main(["--apply"])
    assert rc == 0
    assert target.exists()
    first = target.read_text(encoding="utf-8")
    # Valid JSON, and equal to build_settings().
    assert json.loads(first) == setup_hooks.build_settings()

    # Second apply is byte-identical content and makes a backup.
    rc = setup_hooks.main(["--apply"])
    assert rc == 0
    assert target.read_text(encoding="utf-8") == first
    backups = list((tmp_path / ".claude").glob("settings.json.bak-*"))
    assert backups, "expected a timestamped backup on re-apply"


def test_redundant_local_commands_matches_by_tool(monkeypatch, tmp_path) -> None:
    local = tmp_path / ".claude" / "settings.local.json"
    local.parent.mkdir(parents=True, exist_ok=True)
    local.write_text(
        json.dumps(
            {
                "hooks": {
                    "SessionStart": [
                        {
                            "hooks": [
                                {
                                    "type": "command",
                                    "command": "cd /some/where && uv run python tools/gen_spec.py || true",
                                }
                            ]
                        }
                    ],
                    "Stop": [
                        {
                            "hooks": [
                                {
                                    "type": "command",
                                    "command": "python tools/some_unrelated_tool.py",
                                }
                            ]
                        }
                    ],
                }
            }
        ),
        encoding="utf-8",
    )
    monkeypatch.setattr(setup_hooks, "_SETTINGS_LOCAL", local)
    redundant = setup_hooks.redundant_local_commands()
    # gen_spec is in the committed set -> flagged; the unrelated tool is not.
    assert any("gen_spec.py" in c for c in redundant)
    assert not any("some_unrelated_tool" in c for c in redundant)
