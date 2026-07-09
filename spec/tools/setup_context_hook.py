"""Canon: §Context — installs/removes the project-local hook that feeds tools/context_producer.py.

A user-invocable installer STRICTLY within the launch directory
(R-work-within-launch-dir): it NEVER touches the user's GLOBAL Claude config
(anything under the home ~/.claude) and NEVER patches or reads the host
statusline. It only merge-adds PostToolUse and Stop hook entries into THIS
project's .claude/settings.local.json, pointing them at
`spec/tools/context_producer.py` (which writes spec/.runtime/context.json —
the contract tools/context.py reads, pinned by tests/test_tool_context.py)
when — and only when — the local stdin payload honestly carries ctx_pct.

Merge discipline: every hook entry this installer adds carries the marker
string `"# cah-context-hook:v1"` appended to its command so `--off` can find
and remove EXACTLY the entries this tool added, without touching any
pre-existing or foreign hook. Only PostToolUse is installed here; the Stop
context_producer hook lives in the committed settings.json
(R-sensorium-committed) and is NOT duplicated into settings.local.json.

Idempotent: running `install` twice does not duplicate entries (matched by
the same marker string already being present in an existing command).

NOTE on measurement: Claude Code's project-local hook events do not deliver
context-window usage on stdin today. Installing this hook is harmless and
in-bounds, but it will only ever write a real number if the host honestly
supplies ctx_pct on the local stdin payload; until then the cipher stays
honestly UNMEASURED. The framework will NOT reach into the host to close that
gap (R-work-within-launch-dir).

Usage (commands run from spec/):
  .venv/Scripts/python.exe tools/setup_context_hook.py            # install project-local hook (default)
  .venv/Scripts/python.exe tools/setup_context_hook.py --status   # report installed? + context.json freshness
  .venv/Scripts/python.exe tools/setup_context_hook.py --off      # remove exactly the entries this tool added
"""

from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path

# Make hotam_spec importable so this standalone tool can resolve the consumer
# project root via the shared R1-R6 chain (R-project-root-not-hardcoded).
_SPEC_ROOT = Path(__file__).resolve().parents[1]
if str(_SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(_SPEC_ROOT / "src"))

from hotam_spec.project_paths import project_root_or_raise  # noqa: E402

# Consumer project root: .claude/ is CONSUMER data, resolved via project_root().
# In self-hosting mode R3 (CWD markers) yields the same path as parents[2].
_REPO_ROOT = project_root_or_raise()
_SETTINGS_LOCAL = _REPO_ROOT / ".claude" / "settings.local.json"
_RUNTIME_CONTEXT = Path(__file__).resolve().parents[1] / ".runtime" / "context.json"
_PRODUCER = Path(__file__).resolve().parent / "context_producer.py"

_MARKER = "# cah-context-hook:v1"
_HOOK_EVENTS = ("PostToolUse",)  # Stop context_producer is in committed settings.json (R-sensorium-committed)


def _hook_command() -> str:
    """The command string this installer wires into PostToolUse/Stop hooks."""
    producer_posix = _PRODUCER.as_posix()
    spec_dir = f"{_REPO_ROOT.as_posix()}/spec"
    return (
        f'PY="{spec_dir}/.venv/bin/python"; '
        f'[ -x "$PY" ] || PY="{spec_dir}/.venv/Scripts/python.exe"; '
        f'[ -x "$PY" ] && "$PY" "{producer_posix}" 2>/dev/null || true; {_MARKER}'
    )


def _load_settings() -> dict:
    """Read the existing settings.local.json, or {} if absent/unparseable."""
    if not _SETTINGS_LOCAL.exists():
        return {}
    try:
        return json.loads(_SETTINGS_LOCAL.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}


def _write_settings(data: dict) -> None:
    _SETTINGS_LOCAL.parent.mkdir(parents=True, exist_ok=True)
    _SETTINGS_LOCAL.write_text(
        json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
    )


def _has_marker(hook_group: dict) -> bool:
    for entry in hook_group.get("hooks", []):
        if _MARKER in entry.get("command", ""):
            return True
    return False


def install(data: dict) -> dict:
    """Merge-add PostToolUse + Stop hook entries carrying _MARKER. Idempotent."""
    hooks = data.setdefault("hooks", {})
    for event in _HOOK_EVENTS:
        groups = hooks.setdefault(event, [])
        already = any(_has_marker(g) for g in groups)
        if already:
            continue
        groups.append(
            {
                "hooks": [
                    {
                        "type": "command",
                        "command": _hook_command(),
                    }
                ]
            }
        )
    return data


def uninstall(data: dict) -> dict:
    """Remove exactly the hook groups this installer added (matched by _MARKER)."""
    hooks = data.get("hooks", {})
    for event in _HOOK_EVENTS:
        if event not in hooks:
            continue
        hooks[event] = [g for g in hooks[event] if not _has_marker(g)]
        if not hooks[event]:
            del hooks[event]
    if not hooks:
        data.pop("hooks", None)
    return data


def is_installed(data: dict) -> bool:
    hooks = data.get("hooks", {})
    return any(
        _has_marker(g) for event in _HOOK_EVENTS for g in hooks.get(event, [])
    )


def status_report() -> str:
    data = _load_settings()
    installed = is_installed(data)
    lines = [f"installed: {'yes' if installed else 'no'}"]
    if _RUNTIME_CONTEXT.exists():
        age_s = time.time() - _RUNTIME_CONTEXT.stat().st_mtime
        lines.append(f"context.json: present, age {age_s:.0f}s")
    else:
        lines.append("context.json: absent")
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(
        description=(
            "Install/remove the project-local PostToolUse+Stop hook that feeds "
            "tools/context_producer.py -> spec/.runtime/context.json (default action). "
            "Strictly within the launch directory: this tool never touches ~/.claude "
            "or the host statusline (R-work-within-launch-dir). The context cipher "
            "stays honestly UNMEASURED until the local stdin payload carries ctx_pct."
        )
    )
    group = parser.add_mutually_exclusive_group()
    group.add_argument(
        "--status", action="store_true", help="Report install state + context.json freshness."
    )
    group.add_argument(
        "--off", action="store_true", help="Remove exactly the hook entries this tool added."
    )
    args = parser.parse_args()

    if args.status:
        print(status_report())
        return 0

    data = _load_settings()
    if args.off:
        data = uninstall(data)
        _write_settings(data)
        print("removed context hook entries (if present).")
        return 0

    data = install(data)
    _write_settings(data)
    print(f"installed context hook entries into {_SETTINGS_LOCAL}")
    print(
        "NOTE: the context cipher stays honestly UNMEASURED unless the local "
        "stdin payload carries ctx_pct. The framework will not touch the host "
        "to close that gap (R-work-within-launch-dir)."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
