"""Tests for the CONSTITUTION block in the domain CLAUDE.md (P17+).

After the P17 domain-isolation migration, the CONSTITUTION block lives in
`domains/hotam-spec-self/CLAUDE.md`, not the root CLAUDE.md. Root CLAUDE.md is
framework-only (LIVE-STATE + REPO-MAP + DOMAIN-MAP only).

Canon: §Constitution — the CONSTITUTION block in the domain CLAUDE.md lists all
SETTLED requirements grouped by category, generated deterministically from
domains/hotam-spec-self/graph.py by tools/gen_spec.py. Anti-drift: regeneration must
produce byte-identical output.
"""

from __future__ import annotations

import sys
from pathlib import Path

_TOOLS = Path(__file__).resolve().parents[1] / "tools"
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))

import gen_spec  # noqa: E402

REPO_ROOT = Path(__file__).resolve().parents[2]
# P17: constitution block is in the active domain's CLAUDE.md.
_ACTIVE_DOMAIN = gen_spec._active_domain()
DOMAIN_CLAUDE_MD = (
    _ACTIVE_DOMAIN / "CLAUDE.md" if _ACTIVE_DOMAIN is not None else gen_spec.CLAUDE_MD
)

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
# 1. Sentinels present in the domain CLAUDE.md
# ---------------------------------------------------------------------------


def test_constitution_sentinels_present() -> None:
    """Domain CLAUDE.md contains both CONSTITUTION sentinels (P17: not root CLAUDE.md)."""
    text = _read_normalized(DOMAIN_CLAUDE_MD)
    assert _CONST_BEGIN in text, (
        f"{DOMAIN_CLAUDE_MD} missing CONSTITUTION:BEGIN sentinel"
    )
    assert _CONST_END in text, f"{DOMAIN_CLAUDE_MD} missing CONSTITUTION:END sentinel"


def test_root_claude_md_has_exactly_one_constitution_block() -> None:
    """Root CLAUDE.md must contain the CONSTITUTION sentinel pair exactly once.

    Post-R-claude-md-template-driven (supersedes P22.B's DOMAIN-CRYSTAL
    embedding): root CLAUDE.md is generated directly from
    CLAUDE.md.template.txt via render_business_content(), which includes
    CONSTITUTION once. The guarantee that matters is "not duplicated" —
    root no longer nests a second copy of the domain's CLAUDE.md.
    """
    if _ACTIVE_DOMAIN is None:
        return  # Legacy mode: skip.
    root_text = _read_normalized(gen_spec.CLAUDE_MD)
    assert root_text.count(_CONST_BEGIN) == 1, (
        "Root CLAUDE.md must contain exactly one CONSTITUTION:BEGIN sentinel — "
        "run gen_spec.py to fix"
    )


# ---------------------------------------------------------------------------
# 2. Anti-drift: regeneration produces identical block
# ---------------------------------------------------------------------------


def test_constitution_block_generated() -> None:
    """Regenerating gen_spec produces byte-identical CONSTITUTION block in domain CLAUDE.md."""
    g = gen_spec.load_content_graph()
    expected_block = gen_spec._render_constitution_block(g)

    text = _read_normalized(DOMAIN_CLAUDE_MD)
    actual_block = _extract_constitution_block(text)

    assert actual_block is not None, (
        f"CONSTITUTION block not found in {DOMAIN_CLAUDE_MD}"
    )
    assert actual_block == expected_block, (
        "CONSTITUTION block in domain CLAUDE.md has drifted from gen_spec output. "
        "Run: uv run python tools/gen_spec.py"
    )


# ---------------------------------------------------------------------------
# 3. Every SETTLED requirement id appears in the block
# ---------------------------------------------------------------------------


def test_constitution_lists_all_settled() -> None:
    """Every SETTLED requirement id appears in the CONSTITUTION block."""
    g = gen_spec.load_content_graph()
    text = _read_normalized(DOMAIN_CLAUDE_MD)
    block = _extract_constitution_block(text)
    assert block is not None, "CONSTITUTION block not found"

    settled = [r for r in g.requirements if r.status == gen_spec.SETTLED]
    for r in settled:
        assert r.id in block, (
            f"SETTLED requirement {r.id} missing from CONSTITUTION block"
        )
