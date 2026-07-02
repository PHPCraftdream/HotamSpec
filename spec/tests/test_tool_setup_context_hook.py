"""Tests for spec/tools/setup_context_hook.py + spec/tools/context_producer.py.

Hermetic: never touches the real project .claude/settings.local.json or the
real spec/.runtime/context.json. All settings-file operations run against a
tmp_path copy via monkeypatched module-level path constants.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

_TOOLS = Path(__file__).resolve().parents[1] / "tools"
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))

import setup_context_hook as sch  # noqa: E402
import context_producer as producer  # noqa: E402


# ---------------------------------------------------------------------------
# setup_context_hook: install / merge-preserves-foreign-hooks / off / idempotent
# ---------------------------------------------------------------------------


def _settings_path(tmp_path: Path) -> Path:
    return tmp_path / ".claude" / "settings.local.json"


def test_install_on_empty_settings_adds_posttooluse_and_stop(tmp_path, monkeypatch):
    settings_path = _settings_path(tmp_path)
    monkeypatch.setattr(sch, "_SETTINGS_LOCAL", settings_path)

    data = sch.install({})
    sch._write_settings(data)

    assert settings_path.exists()
    written = json.loads(settings_path.read_text(encoding="utf-8"))
    assert "PostToolUse" in written["hooks"]
    assert "Stop" in written["hooks"]
    assert sch._MARKER in written["hooks"]["PostToolUse"][0]["hooks"][0]["command"]
    assert sch._MARKER in written["hooks"]["Stop"][0]["hooks"][0]["command"]


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
    assert "Stop" in written["hooks"]


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
    stop_marker_groups = [g for g in written["hooks"]["Stop"] if sch._has_marker(g)]
    assert len(post_marker_groups) == 1
    assert len(stop_marker_groups) == 1


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
# context_producer: hermetic (fixture payload, never the real global cache)
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


def test_producer_falls_back_to_global_cache_when_payload_lacks_ctx_pct(
    tmp_path, monkeypatch
):
    runtime_file = tmp_path / "context.json"
    cache_file = tmp_path / "context-cache.json"
    cache_file.write_text(
        json.dumps({"ctx_pct": 55.5, "model": "claude-sonnet-5", "stamp": "x"}),
        encoding="utf-8",
    )
    monkeypatch.setattr(producer, "_RUNTIME", runtime_file)
    monkeypatch.setenv("CAH_CONTEXT_CACHE", str(cache_file))

    wrote = producer.produce({})

    assert wrote is True
    data = json.loads(runtime_file.read_text(encoding="utf-8"))
    assert data["ctx_pct"] == 55.5
    assert data["model"] == "claude-sonnet-5"


def test_producer_skips_when_global_cache_absent_and_no_payload(tmp_path, monkeypatch):
    runtime_file = tmp_path / "context.json"
    monkeypatch.setattr(producer, "_RUNTIME", runtime_file)
    monkeypatch.setenv("CAH_CONTEXT_CACHE", str(tmp_path / "missing.json"))

    wrote = producer.produce({})

    assert wrote is False
    assert not runtime_file.exists()


def test_producer_payload_ctx_pct_takes_priority_over_global_cache(tmp_path, monkeypatch):
    runtime_file = tmp_path / "context.json"
    cache_file = tmp_path / "context-cache.json"
    cache_file.write_text(json.dumps({"ctx_pct": 99.0}), encoding="utf-8")
    monkeypatch.setattr(producer, "_RUNTIME", runtime_file)
    monkeypatch.setenv("CAH_CONTEXT_CACHE", str(cache_file))

    wrote = producer.produce({"ctx_pct": 5.0, "model": "m"})

    assert wrote is True
    data = json.loads(runtime_file.read_text(encoding="utf-8"))
    assert data["ctx_pct"] == 5.0


# ---------------------------------------------------------------------------
# --patch-global: hermetic, always against a tmp fixture script — NEVER the
# real ~/.claude/cah-bin/bin/cah-status.js.
# ---------------------------------------------------------------------------

_FAKE_CAH_STATUS_JS = """\
import { readFileSync, mkdirSync, writeFileSync, renameSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { homedir } from 'node:os';

const RATE_LIMITS_CACHE =
  process.env.CAH_RATE_LIMITS_CACHE ||
  join(homedir(), '.claude', 'cah-bin', 'cache', 'rate-limits.json');

function persistSessionState(fiveHour, sevenDay, effort) {
  if (!fiveHour && !sevenDay && !effort) return;
  try {
    mkdirSync(dirname(RATE_LIMITS_CACHE), { recursive: true });
    writeFileSync(RATE_LIMITS_CACHE, JSON.stringify({ fiveHour, sevenDay, effort }));
  } catch {
    // Fail-silent
  }
}

function buildLine(data) {
  let displayName = null;
  let usedTokens = null;
  let limit = null;
  let fiveHour = null;
  let sevenDay = null;
  let effort = null;
  persistSessionState(fiveHour, sevenDay, effort);
  return 'fake-line';
}

function main() {
  console.log(buildLine({}));
}

main();
"""


def _write_fake_script(tmp_path):
    script = tmp_path / "cah-status.js"
    script.write_text(_FAKE_CAH_STATUS_JS, encoding="utf-8")
    return script


def test_patch_global_dry_run_does_not_modify_file(tmp_path):
    script = _write_fake_script(tmp_path)
    original = script.read_text(encoding="utf-8")

    report = sch.patch_global(apply=False, target=script)

    assert "DRY-RUN" in report
    assert script.read_text(encoding="utf-8") == original
    assert not list(tmp_path.glob("*.bak-*"))


def test_patch_global_apply_injects_function_and_call_and_backs_up(tmp_path):
    script = _write_fake_script(tmp_path)
    original = script.read_text(encoding="utf-8")

    report = sch.patch_global(apply=True, target=script)

    assert "APPLIED" in report
    patched = script.read_text(encoding="utf-8")
    assert sch._GLOBAL_MARKER in patched
    assert "function persistContextCache(" in patched
    assert "persistContextCache(" in patched.split("persistSessionState(fiveHour, sevenDay, effort);")[1]

    backups = list(tmp_path.glob("cah-status.js.bak-*"))
    assert len(backups) == 1
    assert backups[0].read_text(encoding="utf-8") == original


def test_patch_global_apply_twice_is_idempotent_refusal(tmp_path):
    script = _write_fake_script(tmp_path)
    sch.patch_global(apply=True, target=script)
    once_patched = script.read_text(encoding="utf-8")

    try:
        sch.patch_global(apply=True, target=script)
        raised = False
    except sch.GlobalPatchError as exc:
        raised = True
        assert "already patched" in str(exc)

    assert raised is True
    # file untouched by the second (refused) attempt
    assert script.read_text(encoding="utf-8") == once_patched


def test_patch_global_refuses_on_missing_anchor(tmp_path):
    script = tmp_path / "cah-status.js"
    script.write_text("// nothing resembling cah-status.js here\n", encoding="utf-8")
    original = script.read_text(encoding="utf-8")

    try:
        sch.patch_global(apply=True, target=script)
        raised = False
    except sch.GlobalPatchError:
        raised = True

    assert raised is True
    assert script.read_text(encoding="utf-8") == original  # never a corrupting write


def test_patch_global_refuses_on_duplicate_anchor(tmp_path):
    script = tmp_path / "cah-status.js"
    doubled = _FAKE_CAH_STATUS_JS + "\npersistSessionState(fiveHour, sevenDay, effort);\n"
    script.write_text(doubled, encoding="utf-8")
    original = script.read_text(encoding="utf-8")

    try:
        sch.patch_global(apply=True, target=script)
        raised = False
    except sch.GlobalPatchError as exc:
        raised = True
        assert "found 2" in str(exc)

    assert raised is True
    assert script.read_text(encoding="utf-8") == original


def test_revert_global_restores_from_backup(tmp_path):
    script = _write_fake_script(tmp_path)
    original = script.read_text(encoding="utf-8")
    sch.patch_global(apply=True, target=script)
    assert script.read_text(encoding="utf-8") != original

    report = sch.revert_global(target=script)

    assert "REVERTED" in report
    assert script.read_text(encoding="utf-8") == original


def test_revert_global_refuses_without_backup(tmp_path):
    script = _write_fake_script(tmp_path)

    try:
        sch.revert_global(target=script)
        raised = False
    except sch.GlobalPatchError as exc:
        raised = True
        assert "no backup found" in str(exc)

    assert raised is True


def test_find_global_status_script_resolves_from_settings_json(tmp_path, monkeypatch):
    settings = tmp_path / "settings.json"
    script = tmp_path / "cah-status.js"
    script.write_text(_FAKE_CAH_STATUS_JS, encoding="utf-8")
    settings.write_text(
        json.dumps({"statusLine": {"command": f'node "{script.as_posix()}"'}}),
        encoding="utf-8",
    )
    monkeypatch.setattr(sch, "_GLOBAL_SETTINGS", settings)

    found = sch.find_global_status_script()

    assert found == script


def test_find_global_status_script_returns_none_when_settings_absent(tmp_path, monkeypatch):
    monkeypatch.setattr(sch, "_GLOBAL_SETTINGS", tmp_path / "missing-settings.json")

    assert sch.find_global_status_script() is None
