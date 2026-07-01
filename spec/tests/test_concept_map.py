"""Tests for the CONCEPT-MAP sentinel block in the domain CLAUDE.md (P19b).

Verifies that:
1. Both CONCEPT-MAP sentinels are present in the domain CLAUDE.md.
2. Every §-section term in glossary.TERMS appears as a heading in the block.
3. Regenerating gen_spec twice produces byte-identical output (stability).
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path


# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------

SPEC_ROOT = Path(__file__).resolve().parents[1]  # .../spec
REPO_ROOT = SPEC_ROOT.parent

_tools = str(SPEC_ROOT / "tools")
if _tools not in sys.path:
    sys.path.insert(0, _tools)
if str(SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(SPEC_ROOT / "src"))

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

    missing = [slug for slug in section_slugs if f"**{slug}**" not in block]
    assert not missing, (
        f"The following §-section slugs are absent from the CONCEPT-MAP block: {missing}"
    )


def test_concept_map_regen_stable() -> None:
    """Running gen_spec.py twice produces identical CLAUDE.md output (idempotency)."""
    result1 = subprocess.run(
        [sys.executable, str(SPEC_ROOT / "tools" / "gen_spec.py")],
        capture_output=True,
        text=True,
        cwd=str(SPEC_ROOT),
    )
    assert result1.returncode == 0, (
        f"gen_spec.py failed on first run:\n{result1.stderr}"
    )

    text_after_first = _read_claude_md()

    result2 = subprocess.run(
        [sys.executable, str(SPEC_ROOT / "tools" / "gen_spec.py")],
        capture_output=True,
        text=True,
        cwd=str(SPEC_ROOT),
    )
    assert result2.returncode == 0, (
        f"gen_spec.py failed on second run:\n{result2.stderr}"
    )

    text_after_second = _read_claude_md()

    assert text_after_first == text_after_second, (
        "gen_spec.py is not idempotent: two consecutive runs produced different CLAUDE.md output. "
        "This means _scan_concept_map() or an upstream block is non-deterministic."
    )
