"""Canon: §Operator — auto-injects the diff of CLAUDE.md since the operator's last turn into session context via a UserPromptSubmit hook."""

import argparse
import difflib
import json
import sys
from pathlib import Path

_CLAUDE_MD = Path(__file__).resolve().parents[2] / "CLAUDE.md"
_SNAPSHOT = Path(__file__).resolve().parents[1] / ".runtime" / "claude_md_snapshot.md"

_DIFF_LINE_CAP = 150


def _read_text(path: Path) -> str:
    try:
        return path.read_text(encoding="utf-8")
    except OSError:
        return ""


def _write_snapshot(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def build_payload(claude_md_path: Path, snapshot_path: Path) -> tuple[str, bool]:
    """Return (additionalContext, should_update_snapshot).

    Pure-ish helper split out from main() so tests can drive it directly
    with tmp_path-backed Path objects.
    """
    current = _read_text(claude_md_path)

    if not snapshot_path.exists():
        _write_snapshot(snapshot_path, current)
        return "", False

    previous = _read_text(snapshot_path)

    if previous == current:
        return "", False

    diff_lines = list(
        difflib.unified_diff(
            previous.splitlines(),
            current.splitlines(),
            fromfile="CLAUDE.md (last seen)",
            tofile="CLAUDE.md (now)",
            lineterm="",
        )
    )
    diff_line_count = len(diff_lines)

    if diff_line_count > _DIFF_LINE_CAP:
        body = (
            f"CLAUDE.md changed significantly ({diff_line_count} diff lines) "
            "since your last turn — re-read the file with the Read tool before "
            "citing stale content."
        )
    else:
        body = "\n".join(diff_lines[:_DIFF_LINE_CAP])

    additional = "CLAUDE.md changed since your last turn:\n\n" + body

    _write_snapshot(snapshot_path, current)

    return additional, True


def main() -> None:
    parser = argparse.ArgumentParser(
        description=(
            "Emit a UserPromptSubmit hook payload containing the diff of "
            "CLAUDE.md since the last turn, or nothing if unchanged."
        )
    )
    parser.parse_args()

    additional, _ = build_payload(_CLAUDE_MD, _SNAPSHOT)

    sys.stdout.reconfigure(encoding="utf-8")

    if not additional:
        return

    payload = {
        "hookSpecificOutput": {
            "hookEventName": "UserPromptSubmit",
            "additionalContext": additional,
        }
    }
    sys.stdout.write(json.dumps(payload, ensure_ascii=False) + "\n")


if __name__ == "__main__":
    main()
