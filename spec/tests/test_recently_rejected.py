"""Tests: RECENTLY-REJECTED block surfaces anti-relitigation evidence in root CLAUDE.md (P22.B).

Canon: R-recently-rejected-surfaced.
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
from tensio.graph import TensionGraph
from tensio.requirement import Requirement
from tensio.stakeholder import Stakeholder

ROOT_CLAUDE_MD = REPO_ROOT / "CLAUDE.md"
_ACTIVE_DOMAIN = _gs._active_domain()

_RECENTLY_REJECTED_BEGIN = "<!-- RECENTLY-REJECTED:BEGIN -->"
_RECENTLY_REJECTED_END = "<!-- RECENTLY-REJECTED:END -->"

# A REJECTED requirement with REPLACES known to exist in domains/tensio-self/graph.py.
_KNOWN_REJECTED_ID = "R-axes-as-module-constant"


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


def test_recently_rejected_sentinels_present() -> None:
    """Root CLAUDE.md must contain RECENTLY-REJECTED:BEGIN and RECENTLY-REJECTED:END sentinels."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P22.B not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert _RECENTLY_REJECTED_BEGIN in text, (
        "Root CLAUDE.md missing RECENTLY-REJECTED:BEGIN sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )
    assert _RECENTLY_REJECTED_END in text, (
        "Root CLAUDE.md missing RECENTLY-REJECTED:END sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )


# ===========================================================================
# Test 2: known rejection listed
# ===========================================================================


def test_recently_rejected_lists_known_rejections() -> None:
    """RECENTLY-REJECTED block must list R-axes-as-module-constant (known REJECTED+REPLACES)."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P22.B not applicable")
    text = _read(ROOT_CLAUDE_MD)
    block = _extract_block(text, _RECENTLY_REJECTED_BEGIN, _RECENTLY_REJECTED_END)
    assert block is not None, (
        "RECENTLY-REJECTED block not found in root CLAUDE.md. "
        "Run: uv run python tools/gen_spec.py"
    )
    assert _KNOWN_REJECTED_ID in block, (
        f"RECENTLY-REJECTED block does not list {_KNOWN_REJECTED_ID}. "
        "Ensure the domain graph has that requirement with 'REJECTED — REPLACES' in why. "
        "Run: uv run python tools/gen_spec.py"
    )


# ===========================================================================
# Test 3: regen byte-identical
# ===========================================================================


def test_recently_rejected_regen_byte_identical() -> None:
    """Running gen_spec.py again must not change the RECENTLY-REJECTED block (idempotency)."""
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
        "Root CLAUDE.md changed on re-regen — RECENTLY-REJECTED block is not idempotent. "
        "Check _render_recently_rejected_block() for non-determinism."
    )


# ===========================================================================
# Test 4: empty graph renders calm placeholder
# ===========================================================================


def test_recently_rejected_empty_when_none() -> None:
    """A graph with no REJECTED requirements renders the calm placeholder, not an empty list."""
    # Build a minimal graph with no REJECTED requirements.
    g = TensionGraph(
        axes=(),
        stakeholders=(Stakeholder(id="s1", name="S1", domain="d"),),
        requirements=(
            Requirement(
                id="R-live-one",
                claim="Some claim.",
                owner="s1",
                status="DRAFT",
                why="A draft requirement with no rejection markers.",
            ),
        ),
    )
    block = _gs._render_recently_rejected_block(g)
    assert "no anti-relitigation entries" in block, (
        "Expected calm placeholder '_(no anti-relitigation entries — nothing recently rejected.)_' "
        f"but got:\n{block}"
    )
    assert "**R-" not in block, (
        "Calm placeholder should not contain any R-id bullet entries."
    )
