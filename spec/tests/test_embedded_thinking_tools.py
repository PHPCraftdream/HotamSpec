"""Tests: EMBEDDED-THINKING / EMBEDDED-TOOLS blocks embed full content in root CLAUDE.md.

Replaces the P22.B DOMAIN-CRYSTAL mechanism (deleted P22.C): instead of embedding
a whole separate domains/<name>/CLAUDE.md file, root CLAUDE.md now embeds the
full content of every spec/docs/thinking/*.md and spec/docs/tools/*.md file
directly under its own sentinels — one operator, one CLAUDE.md.

Canon: R-crystal-is-claude-md, R-agent-references-shared-docs.
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent

_tools = str(SPEC_ROOT / "tools")
if _tools not in sys.path:
    sys.path.insert(0, _tools)

ROOT_CLAUDE_MD = REPO_ROOT / "CLAUDE.md"

_EMBEDDED_THINKING_BEGIN = "<!-- EMBEDDED-THINKING:BEGIN -->"
_EMBEDDED_THINKING_END = "<!-- EMBEDDED-THINKING:END -->"
_EMBEDDED_TOOLS_BEGIN = "<!-- EMBEDDED-TOOLS:BEGIN -->"
_EMBEDDED_TOOLS_END = "<!-- EMBEDDED-TOOLS:END -->"


def _read(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


def _extract_block(text: str, begin: str, end: str) -> str | None:
    bp = text.find(begin)
    ep = text.find(end)
    if bp == -1 or ep == -1 or ep <= bp:
        return None
    return text[bp + len(begin) : ep]


def test_embedded_thinking_sentinels_present() -> None:
    """Root CLAUDE.md must contain EMBEDDED-THINKING:BEGIN/END sentinels."""
    text = _read(ROOT_CLAUDE_MD)
    assert _EMBEDDED_THINKING_BEGIN in text, (
        "Root CLAUDE.md missing EMBEDDED-THINKING:BEGIN sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )
    assert _EMBEDDED_THINKING_END in text, (
        "Root CLAUDE.md missing EMBEDDED-THINKING:END sentinel."
    )


def test_embedded_thinking_contains_full_topic_content() -> None:
    """EMBEDDED-THINKING block must contain full content of known §Topic docs."""
    text = _read(ROOT_CLAUDE_MD)
    block = _extract_block(text, _EMBEDDED_THINKING_BEGIN, _EMBEDDED_THINKING_END)
    assert block is not None, "EMBEDDED-THINKING block not found."
    for expected in ["§Conflict", "§Graph", "§Requirement"]:
        assert expected in block, f"EMBEDDED-THINKING missing heading for {expected}"


def test_embedded_tools_sentinels_present() -> None:
    """Root CLAUDE.md must contain EMBEDDED-TOOLS:BEGIN/END sentinels."""
    text = _read(ROOT_CLAUDE_MD)
    assert _EMBEDDED_TOOLS_BEGIN in text, (
        "Root CLAUDE.md missing EMBEDDED-TOOLS:BEGIN sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )
    assert _EMBEDDED_TOOLS_END in text, (
        "Root CLAUDE.md missing EMBEDDED-TOOLS:END sentinel."
    )


def test_embedded_tools_contains_full_tool_content() -> None:
    """EMBEDDED-TOOLS block must contain full content of known tool docs."""
    text = _read(ROOT_CLAUDE_MD)
    block = _extract_block(text, _EMBEDDED_TOOLS_BEGIN, _EMBEDDED_TOOLS_END)
    assert block is not None, "EMBEDDED-TOOLS block not found."
    for expected in ["#### gen_spec", "#### what_now", "#### apply_proposal"]:
        assert expected in block, f"EMBEDDED-TOOLS missing heading {expected!r}"


def test_embedded_blocks_regen_byte_identical() -> None:
    """Running gen_spec.py again must not change root CLAUDE.md (idempotency)."""
    before = _read(ROOT_CLAUDE_MD)
    result = subprocess.run(
        [sys.executable, str(SPEC_ROOT / "tools" / "gen_spec.py")],
        capture_output=True,
        text=True,
        cwd=str(SPEC_ROOT),
    )
    assert result.returncode == 0, f"gen_spec.py failed:\n{result.stderr}"
    after = _read(ROOT_CLAUDE_MD)
    assert before == after, (
        "Root CLAUDE.md changed on re-regen — EMBEDDED-THINKING/EMBEDDED-TOOLS "
        "blocks are not idempotent."
    )
