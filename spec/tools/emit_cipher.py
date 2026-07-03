"""Canon: §Operator — emits the three-cipher pulse (top action / debt / context) extracted from the active domain's LIVE-STATE block."""

import argparse
import json
import re
import sys
from pathlib import Path

_CLAUDE_MD = Path(__file__).resolve().parents[2] / "CLAUDE.md"

_BEGIN = "<!-- LIVE-STATE:BEGIN -->"
_END = "<!-- LIVE-STATE:END -->"

_DOMAIN_MAP_BEGIN = "<!-- DOMAIN-MAP:BEGIN -->"
_DOMAIN_MAP_END = "<!-- DOMAIN-MAP:END -->"

_PIN_FILE = Path(__file__).resolve().parents[2] / "domains" / ".active-domain"


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


def _pinned_domain() -> str:
    """Return the pinned self-host domain name (whose LIVE-STATE the cipher reflects)."""
    try:
        return _PIN_FILE.read_text(encoding="utf-8").strip()
    except OSError:
        return ""


def _other_domains_open(text: str) -> int:
    """Sum open-action counts across every domain in DOMAIN-MAP EXCEPT the pinned one.

    The three-cipher pulse (top/debt/context) already reflects the pinned
    self-host domain via LIVE-STATE. The DOMAIN-MAP block carries a per-domain
    'open actions — N (...)' line (R-domain-map-shows-pulse). This aggregate is
    the SECOND eye: how many open actions live in OTHER domains, invisible to
    the self-host cipher (e.g. hotam-dev's DETECTED conflict). Returns 0 when
    the block or the lines are absent.
    """
    try:
        start = text.index(_DOMAIN_MAP_BEGIN)
        end = text.index(_DOMAIN_MAP_END, start)
    except ValueError:
        return 0
    dm = text[start:end]
    pinned = _pinned_domain()
    total = 0
    current_domain = ""
    for line in dm.splitlines():
        h = re.match(r"^### (\S+)", line.strip())
        if h:
            current_domain = h.group(1)
            continue
        m = re.search(r"\*\*open actions\*\*\s*—\s*(\d+)", line)
        if m and current_domain and current_domain != pinned:
            total += int(m.group(1))
    return total


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

    other_open = _other_domains_open(text)

    if top or debt or context or other_open:
        parts = [p for p in [top, debt, context] if p]
        if other_open > 0:
            parts.append(f"other domains: {other_open} open")
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
