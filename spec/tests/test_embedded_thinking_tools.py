"""Tests: EMBEDDED-THINKING / EMBEDDED-TOOLS blocks embed a Tier-1 distillate in root CLAUDE.md.

Replaces the P22.B DOMAIN-CRYSTAL mechanism (deleted P22.C): instead of embedding
a whole separate domains/<name>/CLAUDE.md file, root CLAUDE.md embeds a compact
RULE+WHY distillate of every spec/docs/thinking/*.md and spec/docs/tools/*.md
topic directly under its own sentinels, plus a Tier-3 pointer to the full text
on disk — one operator, one CLAUDE.md, but the crystal carries the operator's
reasoning lens (Tier 1), not the full substrate re-carried verbatim
(R-crystal-reload-by-reference, R-working-vs-substrate-budget).

Canon: R-crystal-is-claude-md, R-agent-references-shared-docs, R-crystal-is-tiered.
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


def test_embedded_thinking_contains_distilled_topic_content() -> None:
    """EMBEDDED-THINKING block must contain a RULE-bearing distillate of each CORE topic."""
    text = _read(ROOT_CLAUDE_MD)
    block = _extract_block(text, _EMBEDDED_THINKING_BEGIN, _EMBEDDED_THINKING_END)
    assert block is not None, "EMBEDDED-THINKING block not found."
    for slug in ["conflict", "graph", "requirement"]:
        heading = f"#### {slug}"
        assert heading in block, f"EMBEDDED-THINKING missing heading {heading!r}"
        after = block[block.index(heading) :]
        assert "RULE" in after, f"EMBEDDED-THINKING topic {slug!r} has no RULE distillate"


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


def test_embedded_tools_contains_distilled_tool_content() -> None:
    """EMBEDDED-TOOLS block must contain distilled (not full --help) content of known tools."""
    text = _read(ROOT_CLAUDE_MD)
    block = _extract_block(text, _EMBEDDED_TOOLS_BEGIN, _EMBEDDED_TOOLS_END)
    assert block is not None, "EMBEDDED-TOOLS block not found."
    for expected in ["#### gen_spec", "#### what_now", "#### apply_proposal"]:
        assert expected in block, f"EMBEDDED-TOOLS missing heading {expected!r}"
    assert "usage: gen_spec.py" not in block, (
        "EMBEDDED-TOOLS still carries raw --help USAGE transcript — "
        "Tier 3 content leaked into the Tier 1 distillate."
    )


def test_embedded_thinking_block_has_tier3_reference() -> None:
    """Each distilled topic section must point at its full-text file on disk (Tier 3)."""
    text = _read(ROOT_CLAUDE_MD)
    block = _extract_block(text, _EMBEDDED_THINKING_BEGIN, _EMBEDDED_THINKING_END)
    assert block is not None, "EMBEDDED-THINKING block not found."
    for slug in ["conflict", "graph", "requirement"]:
        assert f"spec/docs/thinking/{slug}.md" in block, (
            f"EMBEDDED-THINKING topic {slug!r} missing its Tier-3 full-text pointer"
        )


def test_embedded_thinking_block_is_bounded() -> None:
    """Regression guard: the distilled blocks must stay small, not silently re-balloon."""
    text = _read(ROOT_CLAUDE_MD)
    thinking = _extract_block(text, _EMBEDDED_THINKING_BEGIN, _EMBEDDED_THINKING_END)
    tools = _extract_block(text, _EMBEDDED_TOOLS_BEGIN, _EMBEDDED_TOOLS_END)
    assert thinking is not None and tools is not None
    assert len(thinking) < 20_000, (
        f"EMBEDDED-THINKING block is {len(thinking)} chars — Tier 1 distillation "
        "should keep this well under the pre-P22.D-fix full-text size (~105k)."
    )
    assert len(tools) < 8_000, (
        f"EMBEDDED-TOOLS block is {len(tools)} chars — Tier 1 distillation "
        "should keep this well under the pre-P22.D-fix full-text size (~29k)."
    )


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
