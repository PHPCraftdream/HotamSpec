"""Tests: RECENTLY-REJECTED block surfaces anti-relitigation evidence in root CLAUDE.md (P22.B).

Canon: R-recently-rejected-surfaced.
"""

from __future__ import annotations

from pathlib import Path

import pytest

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent

_tools = str(SPEC_ROOT / "tools")

import gen_spec as _gs  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.requirement import Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

ROOT_CLAUDE_MD = REPO_ROOT / "CLAUDE.md"
_ACTIVE_DOMAIN = _gs._active_domain()

_RECENTLY_REJECTED_BEGIN = "<!-- RECENTLY-REJECTED:BEGIN -->"
_RECENTLY_REJECTED_END = "<!-- RECENTLY-REJECTED:END -->"

# A REJECTED requirement with REPLACES known to exist in domains/hotam-spec-self/graph.py.
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


def test_recently_rejected_matches_fresh_gen_spec(gen_spec_snapshot) -> None:
    """The RECENTLY-REJECTED block is present + non-empty in a FRESH gen_spec run.

    Task #46, Measure 4: gen_spec byte-idempotency is proven once in
    test_gen_spec_idempotency.py. This test asserts block content against the
    session-scoped freshly-generated snapshot (Measure 1) rather than spawning a
    subprocess to regenerate.
    """
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P22.B not applicable")
    block = _extract_block(
        gen_spec_snapshot["claude_md_text"],
        _RECENTLY_REJECTED_BEGIN,
        _RECENTLY_REJECTED_END,
    )
    assert block, "Fresh CLAUDE.md is missing the RECENTLY-REJECTED block"
    assert _KNOWN_REJECTED_ID in block, (
        f"Fresh RECENTLY-REJECTED block lost known rejection {_KNOWN_REJECTED_ID}"
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


# ===========================================================================
# Test 5: Phase 2 compression bound
# ===========================================================================


def test_recently_rejected_bounded() -> None:
    """RECENTLY-REJECTED block stays under 8,000 chars after Phase 2 compression."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P22.B not applicable")
    text = _read(ROOT_CLAUDE_MD)
    block = _extract_block(text, _RECENTLY_REJECTED_BEGIN, _RECENTLY_REJECTED_END)
    assert block is not None
    assert len(block) < 8_000, (
        f"RECENTLY-REJECTED block is {len(block)} chars — expected < 8,000 "
        "after Phase 2 compression (dropped duplicate italic why-tail)."
    )


# ===========================================================================
# Test 6: cap enforced — at most N entries shown, with a pointer to full history
# ===========================================================================


def test_recently_rejected_capped_with_pointer() -> None:
    """RECENTLY-REJECTED shows at most _RECENTLY_REJECTED_CAP entries and, when the
    roster exceeds the cap, a pointer line to docs/gen/HISTORY.md for the rest —
    the resident (paid) crystal must not grow monotonically as rejections accumulate.
    """
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P22.B not applicable")
    text = _read(ROOT_CLAUDE_MD)
    block = _extract_block(text, _RECENTLY_REJECTED_BEGIN, _RECENTLY_REJECTED_END)
    assert block is not None
    entry_count = block.count("\n- **R-")
    assert entry_count <= _gs._RECENTLY_REJECTED_CAP, (
        f"RECENTLY-REJECTED lists {entry_count} entries — expected at most "
        f"{_gs._RECENTLY_REJECTED_CAP} (cap not applied)."
    )
    if entry_count == _gs._RECENTLY_REJECTED_CAP:
        assert "docs/gen/HISTORY.md" in block, (
            "RECENTLY-REJECTED is at the cap but has no pointer to the full "
            "history (docs/gen/HISTORY.md) for the remaining rejections."
        )


def test_recently_rejected_block_matches_cap_constant() -> None:
    """The rendered block for a synthetic over-cap graph truncates deterministically
    to _RECENTLY_REJECTED_CAP entries, alphabetical by id, with a pointer line.
    """
    stakeholder = Stakeholder(id="s1", name="S1", domain="d")
    n = _gs._RECENTLY_REJECTED_CAP + 3
    reqs = tuple(
        Requirement(
            id=f"R-rej-{i:02d}",
            claim=f"Claim {i}.",
            owner="s1",
            status="REJECTED",
            why=f"REJECTED — REPLACES by R-something-{i}. Some rationale text.",
        )
        for i in range(n)
    )
    g = TensionGraph(axes=(), stakeholders=(stakeholder,), requirements=reqs)
    block = _gs._render_recently_rejected_block(g)
    entry_count = block.count("\n- **R-")
    assert entry_count == _gs._RECENTLY_REJECTED_CAP, (
        f"Expected exactly {_gs._RECENTLY_REJECTED_CAP} entries for an "
        f"over-cap synthetic graph of {n}, got {entry_count}."
    )
    assert "docs/gen/HISTORY.md" in block
    assert f"showing {_gs._RECENTLY_REJECTED_CAP} of {n}" in block


# ===========================================================================
# Test 8: double-dash REJECTED entries are not silently dropped
# ===========================================================================


def test_recently_rejected_double_dash_not_dropped() -> None:
    """REJECTED requirements using double-dash ('--') in their why must appear
    in the RECENTLY-REJECTED block, not only those using em-dash.

    This is the regression test for the bug where 6 of 34 REJECTED entries
    were silently dropped because gen_spec matched only the em-dash variant.
    """
    stakeholder = Stakeholder(id="s1", name="S1", domain="d")
    reqs = (
        Requirement(
            id="R-emdash-ok",
            claim="Em-dash entry.",
            owner="s1",
            status="REJECTED",
            why="REJECTED — REPLACES by R-successor-a. Rationale A.",
        ),
        Requirement(
            id="R-doubledash-ok",
            claim="Double-dash entry.",
            owner="s1",
            status="REJECTED",
            why="REJECTED -- REPLACES by R-successor-b. Rationale B.",
        ),
    )
    g = TensionGraph(axes=(), stakeholders=(stakeholder,), requirements=reqs)
    block = _gs._render_recently_rejected_block(g)
    assert "R-emdash-ok" in block, "Em-dash REJECTED entry missing from block"
    assert "R-doubledash-ok" in block, (
        "Double-dash REJECTED entry missing from block — "
        "regression: gen_spec must match both em-dash and double-dash variants"
    )
