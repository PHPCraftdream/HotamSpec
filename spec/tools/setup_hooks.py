"""Canon: §Operator — generate the committable, portable project sensorium.

WHY (R-sensorium-committed): the hooks lived only in the personal, git-ignored
settings.local.json, so a fresh clone got no pulse and no graph-guard. This
writes a committed .claude/settings.json instead.

This tool writes the PROJECT-level `<repo>/.claude/settings.json` (checked in),
NOT `settings.local.json` (personal, git-ignored). Every command it emits is
PORTABLE: it anchors on `$CLAUDE_PROJECT_DIR` (the repo root Claude Code exports
to every hook) instead of any one machine's absolute path, so a fresh clone on
any machine gets a working sensorium with zero edits.

Claude Code MERGES settings.json (committed) and settings.local.json (personal).
If the personal file still carries the same hooks, they will run TWICE. This tool
does NOT touch settings.local.json (it is the user's private file); instead
`--apply` prints the exact list of now-redundant local hook commands the user may
remove by hand.

Usage (from spec/):
  uv run python tools/setup_hooks.py            # DRY-RUN: print the plan, write nothing
  uv run python tools/setup_hooks.py --apply    # write <repo>/.claude/settings.json (backup if present)
"""

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
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
_PROJECT_SETTINGS = _REPO_ROOT / ".claude" / "settings.json"
_SETTINGS_LOCAL = _REPO_ROOT / ".claude" / "settings.local.json"

# The portable repo-root anchor Claude Code exports to every hook process.
_DIR = "$CLAUDE_PROJECT_DIR"

_MARKER = "# R-sensorium-committed"


def _tool(name: str, tool_args: str = "") -> str:
    """A portable, FAST invocation of spec/tools/<name>.py, anchored on the
    project dir so it works from any clone on any machine.

    Speed vs portability (the honest fallback): `uv run` re-checks the lock /
    sync of the environment on EVERY invocation (~2s of dead weight), and hooks
    are the hottest path in the loop — they fire on every prompt / compact / stop.
    So the emitted command PREFERS the already-built venv interpreter
    (spec/.venv/{bin/python,Scripts/python.exe}) when it exists, and FALLS BACK to
    `uv run --project` only when the venv is absent (a fresh clone before the
    first sync). The fallback is honest: the venv-python is probed with `[ -x ]`
    before use, so a fresh clone is never broken — it just pays the uv cost once,
    until `uv sync` materializes the venv.

    Portability: the probe covers BOTH layouts — POSIX `bin/python` and Windows
    `Scripts/python.exe` — with no hardcoded machine path (only $CLAUDE_PROJECT_DIR).
    Claude Code runs hook commands through a POSIX shell (sh) on every platform,
    so this `[ -x ] && … || …` conditional is valid on Windows too (Git-Bash).
    """
    tool = f'"{_DIR}/spec/tools/{name}.py"'
    args = f" {tool_args}" if tool_args else ""
    venv = f'"{_DIR}/spec/.venv"'
    fast = (
        f'PY={venv}/bin/python; [ -x "$PY" ] || PY={venv}/Scripts/python.exe; '
        f'if [ -x "$PY" ]; then "$PY" {tool}{args}; '
        f'else uv run --project "{_DIR}/spec" python {tool}{args}; fi'
    )
    return fast


def _cmd(body: str) -> dict:
    """Wrap a command body as a Claude-Code command-hook entry, tagged with the
    ownership marker so this tool (and reviewers) can recognize what it emitted."""
    return {"type": "command", "command": f"{body} {_MARKER}"}


def build_settings() -> dict:
    """Return the committed sensorium settings dict (the universal hook set).

    Universal (machine-independent, business-domain-independent) hooks only:
      SessionStart / PostCompact — regenerate docs so a fresh session boots on
                                   current substrate (gen_spec.py).
      UserPromptSubmit          — emit the three-cipher pulse + inject the
                                   CLAUDE.md diff since the last turn + inject
                                   the attention list (attention_hook.py, the
                                   Claude adapter over the attention core,
                                   R-attention-claude-adapter).
      PreToolUse (Edit|Write)   — deny direct hand-edits of domains/*/graph.py
                                   (R-no-hand-edit-graph), the guard a stranger
                                   otherwise lacks entirely.
      Stop                      — persist context measurement.
    """
    gen = _tool("gen_spec") + " >/dev/null 2>&1 || true"
    emit = _tool("emit_cipher") + " 2>/dev/null || true"
    diff = _tool("claude_md_diff_watch") + " 2>/dev/null || true"
    attn = _tool("attention_hook") + " 2>/dev/null || true"
    guard = _tool("_graph_guard")
    ctx = _tool("context_producer") + " 2>/dev/null || true"

    return {
        "hooks": {
            "SessionStart": [{"hooks": [_cmd(gen)]}],
            "PostCompact": [{"hooks": [_cmd(gen)]}],
            "UserPromptSubmit": [{"hooks": [_cmd(emit), _cmd(diff), _cmd(attn)]}],
            "PreToolUse": [
                {"matcher": "Edit|Write", "hooks": [_cmd(guard)]}
            ],
            "Stop": [{"hooks": [_cmd(ctx)]}],
        }
    }


def _iter_commands(settings: dict):
    """Yield every command string in a settings dict's hooks (any event)."""
    for groups in settings.get("hooks", {}).values():
        for group in groups:
            for entry in group.get("hooks", []):
                cmd = entry.get("command")
                if cmd:
                    yield cmd


def redundant_local_commands() -> list[str]:
    """Return the commands in settings.local.json that duplicate what the
    committed settings.json now provides, so the user can prune them by hand.

    A local command is "redundant" if it invokes the SAME spec/tools/<name>.py
    the committed sensorium runs on the SAME event — matched structurally by the
    tool filename, ignoring the machine-specific path prefix and marker. This
    tool never edits settings.local.json (it is the user's private file); it only
    reports.
    """
    if not _SETTINGS_LOCAL.exists():
        return []
    try:
        local = json.loads(_SETTINGS_LOCAL.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return []

    committed_tools = {
        _tool_name(cmd) for cmd in _iter_commands(build_settings())
    }
    committed_tools.discard(None)

    redundant: list[str] = []
    for cmd in _iter_commands(local):
        if _tool_name(cmd) in committed_tools:
            redundant.append(cmd)
    return redundant


def _tool_name(command: str) -> str | None:
    """Extract the spec/tools/<name>.py filename from a hook command, or None."""
    import re

    m = re.search(r"tools[/\\]([A-Za-z0-9_]+)\.py", command)
    return m.group(1) if m else None


def _backup_path(target: Path) -> Path:
    stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    return target.with_name(f"{target.name}.bak-{stamp}")


def _write(settings: dict) -> Path | None:
    """Write the committed settings.json (backing up any existing file first).
    Returns the backup path if one was made, else None."""
    _PROJECT_SETTINGS.parent.mkdir(parents=True, exist_ok=True)
    backup: Path | None = None
    if _PROJECT_SETTINGS.exists():
        backup = _backup_path(_PROJECT_SETTINGS)
        backup.write_text(
            _PROJECT_SETTINGS.read_text(encoding="utf-8"), encoding="utf-8"
        )
    _PROJECT_SETTINGS.write_text(
        json.dumps(settings, indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )
    return backup


def _plan_text(settings: dict) -> str:
    lines = [
        f"Target (committed): {_PROJECT_SETTINGS}",
        "Universal hooks (portable, anchored on $CLAUDE_PROJECT_DIR):",
    ]
    for event, groups in settings["hooks"].items():
        for group in groups:
            matcher = group.get("matcher")
            tag = f" [matcher={matcher}]" if matcher else ""
            for entry in group["hooks"]:
                lines.append(f"  {event}{tag}: {entry['command']}")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description=(
            "Generate the committable project sensorium <repo>/.claude/"
            "settings.json (the universal hook set), portable via "
            "$CLAUDE_PROJECT_DIR. Dry-run by default; --apply writes."
        )
    )
    parser.add_argument(
        "--apply",
        action="store_true",
        help="Write settings.json (a timestamped backup is made if one exists). "
        "Default is dry-run: print the plan, write nothing.",
    )
    args = parser.parse_args(argv)

    settings = build_settings()

    if not args.apply:
        print("=== DRY RUN — nothing written (pass --apply to write) ===")
        print(_plan_text(settings))
        return 0

    backup = _write(settings)
    print(f"WROTE {_PROJECT_SETTINGS}")
    if backup is not None:
        print(f"  backup: {backup}")

    redundant = redundant_local_commands()
    if redundant:
        print(
            "\nNOTE: Claude Code MERGES settings.json + settings.local.json — "
            "these commands in your PERSONAL settings.local.json now DUPLICATE "
            "the committed sensorium and will run TWICE. This tool does NOT edit "
            "your private file; remove these lines by hand if you want a single run:"
        )
        for cmd in redundant:
            print(f"  - {cmd}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
