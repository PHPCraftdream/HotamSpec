"""Tests for the REPO-MAP block in CLAUDE.md (R-repo-map-generated).

Verifies that:
1. The REPO-MAP sentinels are present in CLAUDE.md.
2. Every file in the scanned directories appears in the block.
3. Regenerating gen_spec produces byte-identical CLAUDE.md.
4. Role text is extracted from module docstrings.
5. Tool entries carry R-tool-* cross-references.
"""

from __future__ import annotations

import ast
import re
import subprocess
import sys
from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent
CLAUDE_MD = REPO_ROOT / "CLAUDE.md"

_REPO_MAP_BEGIN = "<!-- REPO-MAP:BEGIN -->"
_REPO_MAP_END = "<!-- REPO-MAP:END -->"

SRC_DIR = SPEC_ROOT / "src" / "tensio"
TOOLS_DIR = SPEC_ROOT / "tools"
CONTENT_DIR = SPEC_ROOT / "content"
GEN_DIR = REPO_ROOT / "docs" / "gen"


def _read_claude() -> str:
    return (
        CLAUDE_MD.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")
    )


def _extract_repo_map_block(text: str) -> str:
    """Return the text between REPO-MAP sentinels (exclusive), or empty string."""
    begin = text.find(_REPO_MAP_BEGIN)
    end = text.find(_REPO_MAP_END)
    if begin == -1 or end == -1 or end <= begin:
        return ""
    return text[begin + len(_REPO_MAP_BEGIN) : end]


# ---------------------------------------------------------------------------
# Test 1 — sentinels present
# ---------------------------------------------------------------------------


def test_repo_map_sentinels_present() -> None:
    """Both REPO-MAP sentinels must exist in CLAUDE.md."""
    text = _read_claude()
    assert _REPO_MAP_BEGIN in text, f"Missing sentinel {_REPO_MAP_BEGIN!r} in CLAUDE.md"
    assert _REPO_MAP_END in text, f"Missing sentinel {_REPO_MAP_END!r} in CLAUDE.md"
    begin = text.find(_REPO_MAP_BEGIN)
    end = text.find(_REPO_MAP_END)
    assert begin < end, "REPO-MAP:BEGIN must come before REPO-MAP:END"


# ---------------------------------------------------------------------------
# Test 2 — every scanned file appears in the block
# ---------------------------------------------------------------------------


def test_repo_map_complete() -> None:
    """Every file in scanned directories must appear in the REPO-MAP block."""
    text = _read_claude()
    block = _extract_repo_map_block(text)
    assert block, "REPO-MAP block is empty or sentinels are missing"

    errors: list[str] = []

    # Framework body
    for p in sorted(SRC_DIR.glob("*.py")):
        if p.name.startswith("_"):
            continue
        entry = f"spec/src/tensio/{p.name}"
        if entry not in block:
            errors.append(f"Missing framework file: {entry}")

    # Tools
    for p in sorted(TOOLS_DIR.glob("*.py")):
        if p.name.startswith("_"):
            continue
        entry = f"spec/tools/{p.name}"
        if entry not in block:
            errors.append(f"Missing tool file: {entry}")

    # Domain content
    for p in sorted(CONTENT_DIR.glob("*.py")):
        if p.name.startswith("_"):
            continue
        entry = f"spec/content/{p.name}"
        if entry not in block:
            errors.append(f"Missing content file: {entry}")

    # Generated docs
    for p in sorted(GEN_DIR.glob("*.md")):
        entry = f"docs/gen/{p.name}"
        if entry not in block:
            errors.append(f"Missing generated doc: {entry}")

    assert not errors, "REPO-MAP block is incomplete:\n" + "\n".join(errors)


# ---------------------------------------------------------------------------
# Test 3 — regeneration stability (byte-identical)
# ---------------------------------------------------------------------------


def test_repo_map_regen_stable() -> None:
    """Running gen_spec.py twice must produce byte-identical CLAUDE.md."""
    before = _read_claude()

    result = subprocess.run(
        [sys.executable, str(SPEC_ROOT / "tools" / "gen_spec.py")],
        capture_output=True,
        text=True,
        cwd=str(SPEC_ROOT),
    )
    assert result.returncode == 0, f"gen_spec.py failed:\n{result.stderr}"

    after = _read_claude()
    assert before == after, (
        "CLAUDE.md changed after re-running gen_spec.py — REPO-MAP is not idempotent. "
        "Check _scan_repo_map() for non-determinism."
    )


# ---------------------------------------------------------------------------
# Test 4 — role is extracted from module docstring
# ---------------------------------------------------------------------------


def test_repo_map_role_extracted_from_docstring() -> None:
    """For at least one tool, the role text in REPO-MAP matches the module docstring."""
    block = _extract_repo_map_block(_read_claude())
    assert block, "REPO-MAP block missing"

    canon_re = re.compile(r"^Canon:\s+\S+\s+[—\-]\s+(.+)$")

    # Pick the first tool that has a Canon: line.
    for p in sorted(TOOLS_DIR.glob("*.py")):
        if p.name.startswith("_"):
            continue
        src = p.read_text(encoding="utf-8")
        tree = ast.parse(src)
        doc = ast.get_docstring(tree) or ""
        first = doc.split("\n")[0].strip() if doc else ""
        m = canon_re.match(first)
        if not m:
            continue
        expected_role = m.group(1).strip()
        # The role should appear somewhere in the REPO-MAP block.
        assert expected_role in block, (
            f"Role from {p.name} docstring not found in REPO-MAP block.\n"
            f"Expected: {expected_role!r}"
        )
        return  # One match is enough.

    # Fallback: at least verify a known tool name appears.
    assert "spec/tools/gen_spec.py" in block, (
        "No Canon: docstring found in any tool; and gen_spec.py is missing from block."
    )


# ---------------------------------------------------------------------------
# Test 5 — at least one tool entry has R-tool-* cross-reference
# ---------------------------------------------------------------------------


def test_repo_map_tool_xref_present() -> None:
    """At least one tool entry in REPO-MAP must have a R-tool-* cross-reference."""
    block = _extract_repo_map_block(_read_claude())
    assert block, "REPO-MAP block missing"

    xref_re = re.compile(r"→\s+R-tool-\S+")
    assert xref_re.search(block), (
        "No R-tool-* cross-reference found in the REPO-MAP block. "
        "At least one tool should have a Canon: §... marker producing a cross-reference."
    )
