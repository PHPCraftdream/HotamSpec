"""Canon: §Operator — emits the three-cipher pulse (top action / debt / context) extracted from the active domain's LIVE-STATE block."""

import argparse
import json
import re
import sys
from pathlib import Path

_CLAUDE_MD = Path(__file__).resolve().parents[2] / "CLAUDE.md"

_BEGIN = "<!-- LIVE-STATE:BEGIN -->"
_END = "<!-- LIVE-STATE:END -->"


def _extract_live_state(text: str) -> str:
    """Return the text between LIVE-STATE markers, or empty string."""
    try:
        start = text.index(_BEGIN) + len(_BEGIN)
        end = text.index(_END, start)
        return text[start:end]
    except ValueError:
        return ""


def _extract_bullet(block: str, key: str) -> str:
    """Return the value of a bullet line like `- **key:** value`."""
    pattern = rf"\*\*{re.escape(key)}:\*\*\s*(.+)"
    m = re.search(pattern, block)
    return m.group(1).strip() if m else ""


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Emit the three-cipher pulse as a hook JSON payload."
    )
    parser.parse_args()

    try:
        text = _CLAUDE_MD.read_text(encoding="utf-8")
    except OSError:
        text = ""

    block = _extract_live_state(text)

    top = _extract_bullet(block, "top action")
    debt = _extract_bullet(block, "debt")
    context = _extract_bullet(block, "context")

    if top or debt or context:
        parts = [p for p in [top, debt, context] if p]
        additional = "Three-cipher pulse — cite in first sentence: " + " · ".join(parts)
    else:
        additional = ""

    payload = {
        "hookSpecificOutput": {
            "hookEventName": "UserPromptSubmit",
            "additionalContext": additional,
        }
    }
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stdout.write(json.dumps(payload, ensure_ascii=False) + "\n")


if __name__ == "__main__":
    main()
