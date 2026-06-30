"""Tests for .claude/settings.local.json hooks configuration (P22.A)."""

import json
import subprocess
import sys
from pathlib import Path

_SETTINGS = Path(__file__).resolve().parents[2] / ".claude" / "settings.local.json"
_EMIT_CIPHER = Path(__file__).resolve().parents[1] / "tools" / "emit_cipher.py"


def _load_settings() -> dict:
    return json.loads(_SETTINGS.read_text(encoding="utf-8"))


def test_settings_local_json_exists_and_valid():
    assert _SETTINGS.exists(), f"Missing: {_SETTINGS}"
    data = _load_settings()
    assert isinstance(data, dict)
    assert "hooks" in data


def test_session_start_hook_runs_gen_spec():
    data = _load_settings()
    hooks = data["hooks"].get("SessionStart", [])
    commands = [
        h["command"]
        for entry in hooks
        for h in entry.get("hooks", [])
        if h.get("type") == "command"
    ]
    assert any("gen_spec.py" in cmd for cmd in commands), (
        f"No gen_spec.py in SessionStart commands: {commands}"
    )


def test_post_compact_hook_runs_gen_spec():
    data = _load_settings()
    hooks = data["hooks"].get("PostCompact", [])
    commands = [
        h["command"]
        for entry in hooks
        for h in entry.get("hooks", [])
        if h.get("type") == "command"
    ]
    assert any("gen_spec.py" in cmd for cmd in commands), (
        f"No gen_spec.py in PostCompact commands: {commands}"
    )


def test_user_prompt_submit_hook_emits_cipher():
    data = _load_settings()
    hooks = data["hooks"].get("UserPromptSubmit", [])
    commands = [
        h["command"]
        for entry in hooks
        for h in entry.get("hooks", [])
        if h.get("type") == "command"
    ]
    assert any("emit_cipher.py" in cmd for cmd in commands), (
        f"No emit_cipher.py in UserPromptSubmit commands: {commands}"
    )


def test_existing_hooks_preserved():
    data = _load_settings()
    pre = data["hooks"].get("PreToolUse", [])
    assert pre, "PreToolUse hooks missing — R-no-hand-edit-graph guard removed"
    all_prompts = [h.get("prompt", "") for entry in pre for h in entry.get("hooks", [])]
    assert any("domains/" in p and "graph.py" in p for p in all_prompts), (
        "R-no-hand-edit-graph prompt guard not found in PreToolUse"
    )


def test_emit_cipher_returns_valid_json():
    result = subprocess.run(
        [sys.executable, str(_EMIT_CIPHER)],
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode == 0, f"emit_cipher.py failed: {result.stderr}"
    data = json.loads(result.stdout.strip())
    assert data["hookSpecificOutput"]["hookEventName"] == "UserPromptSubmit"
    assert "additionalContext" in data["hookSpecificOutput"]
