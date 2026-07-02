"""Canon: §Context — installs/removes the project-local hook that feeds tools/context_producer.py.

A user-invocable installer: it does NOT touch the user's GLOBAL Claude config
(anything under the home ~/.claude) UNLESS the user explicitly runs it with
`--patch-global --apply` (see PATCH-GLOBAL below). By default it only
merge-adds PostToolUse and Stop hook entries into THIS project's
.claude/settings.local.json, pointing them at `spec/tools/context_producer.py`
(which writes spec/.runtime/context.json — the contract tools/context.py
reads, pinned by tests/test_tool_context.py).

Merge discipline: every hook entry this installer adds carries the marker
string `"# cah-context-hook:v1"` appended to its command so `--off` can find
and remove EXACTLY the entries this tool added, without touching any
pre-existing or foreign hook (SessionStart/UserPromptSubmit/PostCompact/
PreToolUse entries already present in this repo's settings.local.json are
read, preserved verbatim, and re-written unchanged).

Idempotent: running `install` twice does not duplicate entries (matched by
the same marker string already being present in an existing command).

USER ACTION REQUIRED (context cipher stays UNMEASURED until this runs):
  uv run python tools/setup_context_hook.py                       # install project-local hook (default)
  uv run python tools/setup_context_hook.py --status               # report installed? + context.json freshness
  uv run python tools/setup_context_hook.py --off                  # remove exactly the entries this tool added
  uv run python tools/setup_context_hook.py --patch-global          # DRY-RUN: show the diff to the global statusline script
  uv run python tools/setup_context_hook.py --patch-global --apply  # APPLY the patch (backs up first) — then restart the statusline
  uv run python tools/setup_context_hook.py --revert-global         # restore the global statusline script from its backup

PATCH-GLOBAL (§Context bridge, R-context-hook-piggybacks-cah-stamp design):
the project-local hook above can only READ a context % that something
already computed. The global statusline script (found via the `statusLine`
command in ~/.claude/settings.json — the `cah-status.js` bin from the
`clock` skill) computes context-window usage LIVE on every render but never
persists it anywhere on disk. `--patch-global` inserts one small, idempotent
block into that script: right after its existing `persistSessionState(...)`
call (which already writes `~/.claude/cah-bin/cache/rate-limits.json`), it
adds a `persistContextCache(pct, model)` call that writes a SIBLING cache
file `~/.claude/cah-bin/cache/context-cache.json` = {"ctx_pct", "model",
"stamp"}. `context_producer.py` then reads that cache file (see its
CACHE CONTRACT docstring) to write `spec/.runtime/context.json`.

This tool NEVER writes to ~/.claude directly except inside `--patch-global
--apply` (explicit steward action) and NEVER without a timestamped backup
(`cah-status.js.bak-<iso8601>` next to the original) so `--revert-global` can
always restore the pre-patch file. If the target script does not match the
expected shape (missing anchors, already patched by something else, unknown
version), the tool refuses with a clear message and writes nothing —
corrupting the user's live statusline is treated as unacceptable.
"""

from __future__ import annotations

import argparse
import json
import re
import time
from datetime import datetime, timezone
from pathlib import Path

_REPO_ROOT = Path(__file__).resolve().parents[2]  # .../HotamSpec
_SETTINGS_LOCAL = _REPO_ROOT / ".claude" / "settings.local.json"
_RUNTIME_CONTEXT = Path(__file__).resolve().parents[1] / ".runtime" / "context.json"
_PRODUCER = Path(__file__).resolve().parent / "context_producer.py"

_MARKER = "# cah-context-hook:v1"
_HOOK_EVENTS = ("PostToolUse", "Stop")

# ---------------------------------------------------------------------------
# --patch-global: surgical patch to the user's global cah-status.js
# ---------------------------------------------------------------------------

_HOME = Path.home()
_GLOBAL_SETTINGS = _HOME / ".claude" / "settings.json"
_CAH_STATUS_DEFAULT = _HOME / ".claude" / "cah-bin" / "bin" / "cah-status.js"
_GLOBAL_CACHE_DEFAULT = _HOME / ".claude" / "cah-bin" / "cache" / "context-cache.json"

_GLOBAL_MARKER = "// cah-context-cache:v1"

# The exact call this patch anchors after — the existing, un-owned line in
# cah-status.js's buildLine() that already persists rate-limit state to a
# sibling cache file. Patching right after it keeps the new write in the
# same place, same error-handling style, same fail-silent discipline.
_ANCHOR_CALL = "persistSessionState(fiveHour, sevenDay, effort);"

# The function this patch injects, textually identical in shape to the
# existing persistSessionState() so a human diff-reviewing the patched file
# recognizes the pattern immediately. Written once, right before buildLine().
_INJECTED_FUNCTION_TEMPLATE = '''{marker}
// Added by HotamSpec spec/tools/setup_context_hook.py --patch-global --apply.
// Persists the SAME context-window pct this bin already renders into the
// status line, so a project-local hook (context_producer.py) can read a
// real number instead of leaving R-measure-context-size DRAFT/UNMEASURED.
// Fail-silent by the same discipline as persistSessionState: a cache-write
// error must never break the status bar.
const CONTEXT_CACHE =
  process.env.CAH_CONTEXT_CACHE ||
  join(homedir(), '.claude', 'cah-bin', 'cache', 'context-cache.json');

function persistContextCache(pct, model) {{
  if (pct === null || pct === undefined) return;
  try {{
    mkdirSync(dirname(CONTEXT_CACHE), {{ recursive: true }});
    const tmp = CONTEXT_CACHE + '.tmp';
    writeFileSync(
      tmp,
      JSON.stringify({{ ctx_pct: pct, model: model || '', stamp: new Date().toISOString() }}) + '\\n',
    );
    renameSync(tmp, CONTEXT_CACHE);
  }} catch {{
    // Fail-silent: the statusLine bin must never break the bar over a cache miss.
  }}
}}

'''

_INJECTED_CALL_TEMPLATE = (
    "  {marker_inline}\n"
    "  persistContextCache(\n"
    "    limit && usedTokens != null ? (usedTokens / limit) * 100 : null,\n"
    "    displayName,\n"
    "  );\n"
)


def _read_text(path: Path) -> str | None:
    try:
        return path.read_text(encoding="utf-8")
    except OSError:
        return None


def find_global_status_script() -> Path | None:
    """Resolve the global statusline script path from ~/.claude/settings.json.

    Returns None if settings.json is absent/unparseable or has no
    `statusLine.command` naming a .js file that exists on disk.
    """
    text = _read_text(_GLOBAL_SETTINGS)
    if text is None:
        return None
    try:
        data = json.loads(text)
    except json.JSONDecodeError:
        return None
    command = (data.get("statusLine") or {}).get("command", "")
    m = re.search(r'"([^"]+\.js)"', command)
    if not m:
        return None
    candidate = Path(m.group(1))
    return candidate if candidate.exists() else None


class GlobalPatchError(Exception):
    """Refusal to patch — script shape unexpected, never a corrupting write."""


def build_patched_source(original: str) -> str:
    """Return the patched cah-status.js source, or raise GlobalPatchError.

    Anchors: `_ANCHOR_CALL` must appear exactly once (the call site inside
    buildLine()), and the injected function is placed immediately before the
    `function buildLine(` definition. Both anchors are required; if either is
    missing or duplicated, refuse rather than guess a splice point.
    """
    if _GLOBAL_MARKER in original:
        raise GlobalPatchError("already patched (marker present) — idempotent, nothing to do")

    anchor_count = original.count(_ANCHOR_CALL)
    if anchor_count != 1:
        raise GlobalPatchError(
            f"expected exactly one occurrence of the anchor call "
            f"{_ANCHOR_CALL!r}, found {anchor_count} — refusing to guess a splice point "
            "(the script may have been updated/rewritten; re-check by hand)."
        )

    buildline_marker = "function buildLine(data) {"
    if original.count(buildline_marker) != 1:
        raise GlobalPatchError(
            f"expected exactly one occurrence of {buildline_marker!r} — "
            "refusing to guess where to inject persistContextCache()."
        )

    injected_fn = _INJECTED_FUNCTION_TEMPLATE.format(marker=_GLOBAL_MARKER)
    patched = original.replace(buildline_marker, injected_fn + buildline_marker, 1)

    injected_call = _INJECTED_CALL_TEMPLATE.format(marker_inline=_GLOBAL_MARKER)
    patched = patched.replace(
        _ANCHOR_CALL, _ANCHOR_CALL + "\n" + injected_call, 1
    )
    return patched


def _backup_path(target: Path) -> Path:
    stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    return target.with_name(f"{target.name}.bak-{stamp}")


def patch_global(apply: bool, target: Path | None = None) -> str:
    """Dry-run (default) or apply (apply=True) the global cah-status.js patch.

    Returns a human-readable report. Raises GlobalPatchError on any shape
    mismatch — callers must not catch-and-continue past that; the whole
    point is refusing a corrupting write.
    """
    script = target or find_global_status_script()
    if script is None:
        raise GlobalPatchError(
            "could not resolve the global statusline script — checked "
            f"{_GLOBAL_SETTINGS} for a `statusLine.command` naming an "
            "existing .js file. Nothing was touched."
        )
    original = _read_text(script)
    if original is None:
        raise GlobalPatchError(f"could not read {script} — nothing was touched.")

    patched = build_patched_source(original)  # raises GlobalPatchError on mismatch

    if not apply:
        return (
            f"DRY-RUN — would patch {script}\n"
            f"  + inject persistContextCache() before buildLine()\n"
            f"  + call it with (ctx_pct, model) right after {_ANCHOR_CALL}\n"
            f"  + writes: ~/.claude/cah-bin/cache/context-cache.json (or $CAH_CONTEXT_CACHE)\n"
            "Re-run with --apply to write (a timestamped backup is made first)."
        )

    backup = _backup_path(script)
    backup.write_text(original, encoding="utf-8")
    script.write_text(patched, encoding="utf-8")
    return (
        f"APPLIED — patched {script}\n"
        f"  backup: {backup}\n"
        "Restart the statusline (new Claude Code session, or however your "
        "terminal reloads it) for the change to take effect.\n"
        "Revert any time with: uv run python tools/setup_context_hook.py --revert-global"
    )


def revert_global(target: Path | None = None) -> str:
    """Restore the global statusline script from its most recent backup."""
    script = target or find_global_status_script()
    if script is None:
        raise GlobalPatchError(
            "could not resolve the global statusline script — nothing to revert."
        )
    candidates = sorted(script.parent.glob(f"{script.name}.bak-*"))
    if not candidates:
        raise GlobalPatchError(f"no backup found next to {script} (pattern {script.name}.bak-*).")
    latest = candidates[-1]
    original = _read_text(latest)
    if original is None:
        raise GlobalPatchError(f"could not read backup {latest} — nothing was touched.")
    script.write_text(original, encoding="utf-8")
    return f"REVERTED {script} from {latest}"


def _hook_command() -> str:
    """The command string this installer wires into PostToolUse/Stop hooks."""
    producer_posix = _PRODUCER.as_posix()
    return f'uv run --project "{_REPO_ROOT.as_posix()}/spec" python "{producer_posix}" 2>/dev/null || true {_MARKER}'


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
            "The context cipher stays UNMEASURED until BOTH this project-local hook "
            "AND the global statusline patch (--patch-global --apply, a SEPARATE "
            "explicit step touching ~/.claude) are in place — this tool never "
            "touches ~/.claude on its own."
        )
    )
    group = parser.add_mutually_exclusive_group()
    group.add_argument(
        "--status", action="store_true", help="Report install state + context.json freshness."
    )
    group.add_argument(
        "--off", action="store_true", help="Remove exactly the hook entries this tool added."
    )
    group.add_argument(
        "--patch-global",
        action="store_true",
        help="Show (dry-run) or apply (--apply) the surgical patch to the user's "
        "global cah-status.js statusline script, so it also caches ctx_pct/model/"
        "stamp for context_producer.py to read. Off by default; requires --apply "
        "to actually write.",
    )
    group.add_argument(
        "--revert-global",
        action="store_true",
        help="Restore the global statusline script from its most recent "
        "--patch-global backup.",
    )
    parser.add_argument(
        "--apply",
        action="store_true",
        help="With --patch-global: actually write the patch (default is dry-run). "
        "Ignored otherwise.",
    )
    args = parser.parse_args()

    if args.status:
        print(status_report())
        return 0

    if args.patch_global:
        try:
            print(patch_global(apply=args.apply))
        except GlobalPatchError as exc:
            print(f"refused: {exc}")
            return 1
        return 0

    if args.revert_global:
        try:
            print(revert_global())
        except GlobalPatchError as exc:
            print(f"refused: {exc}")
            return 1
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
        "NOTE: the context cipher stays UNMEASURED until you ALSO run:\n"
        "  uv run python tools/setup_context_hook.py --patch-global --apply\n"
        "then restart the statusline (new session)."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
