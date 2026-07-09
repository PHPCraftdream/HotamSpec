"""Tests for the CONCEPT-MAP sentinel block in the domain CLAUDE.md (P19b).

Verifies that:
1. Both CONCEPT-MAP sentinels are present in the domain CLAUDE.md.
2. Every §-section term in glossary.TERMS appears as a heading in the block.
3. Regenerating gen_spec twice produces byte-identical output (stability).
"""

from __future__ import annotations

from pathlib import Path


# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------

SPEC_ROOT = Path(__file__).resolve().parents[1]  # .../spec

_tools = str(SPEC_ROOT / "tools")

import gen_spec as _gen_spec  # noqa: E402
from hotam_spec.glossary import TERMS  # noqa: E402

CLAUDE_MD = _gen_spec.CLAUDE_MD

_CONCEPT_MAP_BEGIN = "<!-- CONCEPT-MAP:BEGIN -->"
_CONCEPT_MAP_END = "<!-- CONCEPT-MAP:END -->"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _read_claude_md() -> str:
    return CLAUDE_MD.read_text(encoding="utf-8").replace("\r\n", "\n")


def _concept_map_block(text: str) -> str:
    """Extract the text between CONCEPT-MAP sentinels."""
    begin_pos = text.find(_CONCEPT_MAP_BEGIN)
    end_pos = text.find(_CONCEPT_MAP_END)
    assert begin_pos != -1 and end_pos != -1, "CONCEPT-MAP sentinels not found"
    return text[begin_pos + len(_CONCEPT_MAP_BEGIN) : end_pos]


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_concept_map_sentinels_present() -> None:
    """Both CONCEPT-MAP sentinels must appear in the domain CLAUDE.md."""
    text = _read_claude_md()
    assert _CONCEPT_MAP_BEGIN in text, f"Missing '{_CONCEPT_MAP_BEGIN}' in {CLAUDE_MD}"
    assert _CONCEPT_MAP_END in text, f"Missing '{_CONCEPT_MAP_END}' in {CLAUDE_MD}"
    begin_pos = text.find(_CONCEPT_MAP_BEGIN)
    end_pos = text.find(_CONCEPT_MAP_END)
    assert begin_pos < end_pos, "CONCEPT-MAP:END must come after CONCEPT-MAP:BEGIN"


def test_concept_map_lists_all_glossary_sections() -> None:
    """Every §-section slug from glossary.TERMS must appear in the CONCEPT-MAP block."""
    text = _read_claude_md()
    block = _concept_map_block(text)

    section_slugs = [t.slug for t in TERMS if t.kind == "SECTION"]
    assert section_slugs, "No SECTION terms found in glossary.TERMS — check glossary.py"

    # New table format: | **§slug** | ... | ... | ... |
    missing = [slug for slug in section_slugs if f"**{slug}**" not in block]
    assert not missing, (
        f"The following §-section slugs are absent from the CONCEPT-MAP block: {missing}"
    )


def test_concept_map_matches_fresh_gen_spec(gen_spec_snapshot) -> None:
    """Every §-section slug appears in the CONCEPT-MAP block of a FRESH gen_spec run.

    Task #46, Measure 4: byte-idempotency is proven once in
    test_gen_spec_idempotency.py. This test asserts the block content against the
    session-scoped freshly-generated snapshot (Measure 1) rather than spawning a
    subprocess to regenerate.
    """
    text = gen_spec_snapshot["claude_md_text"]
    block = _concept_map_block(text)
    section_slugs = [t.slug for t in TERMS if t.kind == "SECTION"]
    missing = [slug for slug in section_slugs if f"**{slug}**" not in block]
    assert not missing, (
        f"Fresh CONCEPT-MAP block is missing §-section slugs: {missing}"
    )
