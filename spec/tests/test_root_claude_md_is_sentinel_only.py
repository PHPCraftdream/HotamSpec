"""Locking tests: root CLAUDE.md is a thin sentinel-only shell.

After P19a:
- Root CLAUDE.md is under 6000 chars (sentinel blocks + minimal header).
- Root CLAUDE.md has a THINKING-INDEX sentinel block.
- Root CLAUDE.md has no ## or ### headings OUTSIDE sentinel blocks
  (the framework-identity header uses a single # heading only).

Canon: §Domain — R-root-claude-md-is-sentinel-only.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import pytest

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent

_tools = str(SPEC_ROOT / "tools")
if _tools not in sys.path:
    sys.path.insert(0, _tools)

import gen_spec as _gs  # noqa: E402

ROOT_CLAUDE_MD = REPO_ROOT / "CLAUDE.md"

_SENTINEL_PAIRS = [
    ("<!-- LIVE-STATE:BEGIN -->", "<!-- LIVE-STATE:END -->"),
    ("<!-- REPO-MAP:BEGIN -->", "<!-- REPO-MAP:END -->"),
    ("<!-- DOMAIN-MAP:BEGIN -->", "<!-- DOMAIN-MAP:END -->"),
    ("<!-- CONSTITUTION:BEGIN -->", "<!-- CONSTITUTION:END -->"),
    ("<!-- AGENT-MAP:BEGIN -->", "<!-- AGENT-MAP:END -->"),
    ("<!-- CONCEPT-MAP:BEGIN -->", "<!-- CONCEPT-MAP:END -->"),
    ("<!-- THINKING-INDEX:BEGIN -->", "<!-- THINKING-INDEX:END -->"),
    ("<!-- EMBEDDED-THINKING:BEGIN -->", "<!-- EMBEDDED-THINKING:END -->"),
    ("<!-- EMBEDDED-TOOLS:BEGIN -->", "<!-- EMBEDDED-TOOLS:END -->"),
    ("<!-- RECENTLY-REJECTED:BEGIN -->", "<!-- RECENTLY-REJECTED:END -->"),
]

_ACTIVE_DOMAIN = _gs._active_domain()

# Character cap: header + sentinel blocks + padding.
# P22.C consolidation: root CLAUDE.md now embeds the FULL methodology (thinking
# docs) and FULL tool docs content directly (EMBEDDED-THINKING/EMBEDDED-TOOLS)
# — there is only ONE CLAUDE.md file in the whole repo. Cap raised accordingly,
# but still far under the phi-cap (~2.47M chars) enforced elsewhere.
_CHAR_CAP = 1_000_000


def _read(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


def _strip_sentinels(text: str) -> str:
    """Remove all content within sentinel block pairs (including sentinels)."""
    stripped = text
    for begin, end in _SENTINEL_PAIRS:
        while begin in stripped and end in stripped:
            bp = stripped.find(begin)
            ep = stripped.find(end) + len(end)
            stripped = stripped[:bp] + stripped[ep:]
    return stripped


def test_root_claude_md_has_thinking_index_sentinels() -> None:
    """Root CLAUDE.md must contain THINKING-INDEX:BEGIN and THINKING-INDEX:END sentinels."""
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- THINKING-INDEX:BEGIN -->" in text, (
        "Root CLAUDE.md missing THINKING-INDEX:BEGIN sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )
    assert "<!-- THINKING-INDEX:END -->" in text, (
        "Root CLAUDE.md missing THINKING-INDEX:END sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )


def test_root_claude_md_thinking_index_lists_thinking_files() -> None:
    """THINKING-INDEX block must list at least the core thinking files."""
    text = _read(ROOT_CLAUDE_MD)
    bp = text.find("<!-- THINKING-INDEX:BEGIN -->")
    ep = text.find("<!-- THINKING-INDEX:END -->")
    if bp == -1 or ep == -1:
        pytest.skip("THINKING-INDEX sentinels absent — covered by other test")
    block = text[bp:ep]
    # Must reference at least conflict.md, graph.md, requirement.md.
    for expected in ["conflict.md", "graph.md", "requirement.md"]:
        assert expected in block, (
            f"THINKING-INDEX block missing expected link to {expected}"
        )


def test_root_claude_md_under_size_cap() -> None:
    """Root CLAUDE.md total size must be under the cap (thin shell — no large prose)."""
    text = _read(ROOT_CLAUDE_MD)
    char_count = len(text)
    assert char_count < _CHAR_CAP, (
        f"Root CLAUDE.md is {char_count} chars, exceeding cap of {_CHAR_CAP}. "
        "Move prose to domains/hotam-spec-self/CLAUDE.md or spec/docs/thinking/. "
        "Run: uv run python tools/gen_spec.py"
    )


def test_root_claude_md_no_large_prose_outside_sentinels() -> None:
    """Outside sentinel blocks, root CLAUDE.md must have no ## or ### headings.

    The only allowed heading is the single # framework-identity line at the top.
    """
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P19a not applicable")
    text = _read(ROOT_CLAUDE_MD)
    outside = _strip_sentinels(text)
    # Find any ## or ### headings.
    bad_headings = re.findall(r"^#{2,}\s+\S", outside, re.MULTILINE)
    assert not bad_headings, (
        f"Root CLAUDE.md has {len(bad_headings)} heading(s) outside sentinel blocks: "
        f"{bad_headings[:3]}. "
        "Move prose to the domain CLAUDE.md or spec/docs/thinking/."
    )
