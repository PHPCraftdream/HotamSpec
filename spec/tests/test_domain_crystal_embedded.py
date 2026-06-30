"""Tests: DOMAIN-CRYSTAL block embeds the active domain's CLAUDE.md in root CLAUDE.md (P22.B).

Canon: R-root-claude-md-contains-domain-crystal.
"""

from __future__ import annotations

import subprocess
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
_ACTIVE_DOMAIN = _gs._active_domain()
DOMAIN_CLAUDE_MD = _ACTIVE_DOMAIN / "CLAUDE.md" if _ACTIVE_DOMAIN is not None else None

_DOMAIN_CRYSTAL_BEGIN = "<!-- DOMAIN-CRYSTAL:BEGIN -->"
_DOMAIN_CRYSTAL_END = "<!-- DOMAIN-CRYSTAL:END -->"

# A stable substring known to live in domains/tensio-self/CLAUDE.md header prose.
# This is hand-written prose, not generated — it is stable.
_KNOWN_DOMAIN_SUBSTRING = "operator crystal for the `tensio-self` domain director"


def _read(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


def _extract_block(text: str, begin: str, end: str) -> str | None:
    bp = text.find(begin)
    ep = text.find(end)
    if bp == -1 or ep == -1 or ep <= bp:
        return None
    return text[bp + len(begin) : ep]


# ===========================================================================
# Test 1: sentinels present
# ===========================================================================


def test_domain_crystal_sentinels_present() -> None:
    """Root CLAUDE.md must contain DOMAIN-CRYSTAL:BEGIN and DOMAIN-CRYSTAL:END sentinels."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P22.B not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert _DOMAIN_CRYSTAL_BEGIN in text, (
        "Root CLAUDE.md missing DOMAIN-CRYSTAL:BEGIN sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )
    assert _DOMAIN_CRYSTAL_END in text, (
        "Root CLAUDE.md missing DOMAIN-CRYSTAL:END sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )


# ===========================================================================
# Test 2: domain CLAUDE.md content embedded
# ===========================================================================


def test_domain_crystal_contains_domains_claude_md_content() -> None:
    """DOMAIN-CRYSTAL block in root CLAUDE.md must contain content from the domain's CLAUDE.md.

    Source: domains/<active>/CLAUDE.md (NOT the director's agents/director/CLAUDE.md).
    """
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P22.B not applicable")
    text = _read(ROOT_CLAUDE_MD)
    block = _extract_block(text, _DOMAIN_CRYSTAL_BEGIN, _DOMAIN_CRYSTAL_END)
    assert block is not None, (
        "DOMAIN-CRYSTAL block not found in root CLAUDE.md. "
        "Run: uv run python tools/gen_spec.py"
    )
    assert _KNOWN_DOMAIN_SUBSTRING in block, (
        f"DOMAIN-CRYSTAL block does not contain expected domain CLAUDE.md content. "
        f"Expected substring: {_KNOWN_DOMAIN_SUBSTRING!r}. "
        "Ensure gen_spec.py embeds domains/<active>/CLAUDE.md (not the director's). "
        "Run: uv run python tools/gen_spec.py"
    )


# ===========================================================================
# Test 3: regen byte-identical
# ===========================================================================


def test_domain_crystal_regen_byte_identical() -> None:
    """Running gen_spec.py again must not change root CLAUDE.md (idempotency)."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P22.B not applicable")
    before = _read(ROOT_CLAUDE_MD)
    result = subprocess.run(
        [sys.executable, str(SPEC_ROOT / "tools" / "gen_spec.py")],
        capture_output=True,
        text=True,
        cwd=str(SPEC_ROOT),
    )
    assert result.returncode == 0, (
        f"gen_spec.py failed:\n{result.stderr}"
    )
    after = _read(ROOT_CLAUDE_MD)
    assert before == after, (
        "Root CLAUDE.md changed on re-regen — DOMAIN-CRYSTAL block is not idempotent. "
        "Check _render_domain_crystal_block() for non-determinism."
    )
