"""Tests for the CONSTITUTION block in root CLAUDE.md (P22.C consolidation).

After P22.C, there is exactly ONE CLAUDE.md file (repo root). The CONSTITUTION
block — all SETTLED requirements grouped by category — renders directly into
root CLAUDE.md's own sentinels (no more domain-file indirection).

Canon: §Constitution — the CONSTITUTION block lists all SETTLED requirements
grouped by category, generated deterministically from
domains/hotam-spec-self/graph.py by tools/gen_spec.py. Anti-drift: regeneration
must produce byte-identical output.
"""

from __future__ import annotations

import sys
from pathlib import Path

_TOOLS = Path(__file__).resolve().parents[1] / "tools"
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))

import gen_spec  # noqa: E402

REPO_ROOT = Path(__file__).resolve().parents[2]
ROOT_CLAUDE_MD = gen_spec.CLAUDE_MD

_CONST_BEGIN = gen_spec._CONST_BEGIN
_CONST_END = gen_spec._CONST_END


def _read_normalized(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


def _extract_constitution_block(text: str) -> str | None:
    begin = text.find(_CONST_BEGIN)
    end = text.find(_CONST_END)
    if begin == -1 or end == -1 or end <= begin:
        return None
    return text[begin + len(_CONST_BEGIN) : end].strip("\n")


# ---------------------------------------------------------------------------
# 1. Sentinels present in root CLAUDE.md
# ---------------------------------------------------------------------------


def test_constitution_sentinels_present() -> None:
    """Root CLAUDE.md contains both CONSTITUTION sentinels."""
    text = _read_normalized(ROOT_CLAUDE_MD)
    assert _CONST_BEGIN in text, f"{ROOT_CLAUDE_MD} missing CONSTITUTION:BEGIN sentinel"
    assert _CONST_END in text, f"{ROOT_CLAUDE_MD} missing CONSTITUTION:END sentinel"


# ---------------------------------------------------------------------------
# 2. Anti-drift: regeneration produces identical block
# ---------------------------------------------------------------------------


def test_constitution_block_generated() -> None:
    """Regenerating gen_spec produces byte-identical CONSTITUTION block in root CLAUDE.md."""
    g = gen_spec.load_content_graph()
    expected_block = gen_spec._render_constitution_block(g)

    text = _read_normalized(ROOT_CLAUDE_MD)
    actual_block = _extract_constitution_block(text)

    assert actual_block is not None, f"CONSTITUTION block not found in {ROOT_CLAUDE_MD}"
    assert actual_block == expected_block, (
        "CONSTITUTION block in root CLAUDE.md has drifted from gen_spec output. "
        "Run: uv run python tools/gen_spec.py"
    )


# ---------------------------------------------------------------------------
# 3. Every SETTLED requirement id appears in the block
# ---------------------------------------------------------------------------


def test_constitution_lists_all_settled() -> None:
    """Every SETTLED requirement id appears in the CONSTITUTION block."""
    g = gen_spec.load_content_graph()
    text = _read_normalized(ROOT_CLAUDE_MD)
    block = _extract_constitution_block(text)
    assert block is not None, "CONSTITUTION block not found"

    settled = [r for r in g.requirements if r.status == gen_spec.SETTLED]
    for r in settled:
        assert r.id in block, (
            f"SETTLED requirement {r.id} missing from CONSTITUTION block"
        )
