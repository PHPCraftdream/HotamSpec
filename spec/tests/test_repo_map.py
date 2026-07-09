"""Tests for REPO-MAP.md (relocated from root CLAUDE.md REPO-MAP block).

Verifies that:
1. REPO-MAP.md exists in the active domain's docs/gen/.
2. Every file in the scanned directories appears in the doc.
3. Role text is extracted from module docstrings.
4. Tool entries carry R-tool-* cross-references.

Canon: §Generator — R-repo-map-generated (REJECTED, relocated to docs/gen/REPO-MAP.md).
"""

from __future__ import annotations

import ast
import re
from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]

_tools = str(SPEC_ROOT / "tools")

import gen_spec  # noqa: E402

SRC_DIR = SPEC_ROOT / "src" / "hotam_spec"
TOOLS_DIR = SPEC_ROOT / "tools"
CONTENT_DIR = gen_spec.CONTENT_DIR

# The REPO-MAP.md is now a generated doc in the active domain's docs/gen/.
REPO_MAP_MD = gen_spec.REPO_MAP_MD


def _read_repo_map() -> str:
    return REPO_MAP_MD.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


# ---------------------------------------------------------------------------
# Test 1 — REPO-MAP.md exists
# ---------------------------------------------------------------------------


def test_repo_map_md_exists() -> None:
    """REPO-MAP.md must exist in the active domain's docs/gen/."""
    assert REPO_MAP_MD.exists(), (
        f"REPO-MAP.md not found at {REPO_MAP_MD}. "
        "Run: uv run python tools/gen_spec.py"
    )


# ---------------------------------------------------------------------------
# Test 2 — every scanned file appears in the doc
# ---------------------------------------------------------------------------


def test_repo_map_complete() -> None:
    """Every file in scanned directories must appear in REPO-MAP.md."""
    text = _read_repo_map()
    errors: list[str] = []

    # Framework body
    for p in sorted(SRC_DIR.glob("*.py")):
        if p.name.startswith("_"):
            continue
        entry = f"spec/src/hotam_spec/{p.name}"
        if entry not in text:
            errors.append(f"Missing framework file: {entry}")

    # Tools
    for p in sorted(TOOLS_DIR.glob("*.py")):
        if p.name.startswith("_"):
            continue
        entry = f"spec/tools/{p.name}"
        if entry not in text:
            errors.append(f"Missing tool file: {entry}")

    assert not errors, "REPO-MAP.md is incomplete:\n" + "\n".join(errors)


# ---------------------------------------------------------------------------
# Test 3 — role is extracted from module docstring
# ---------------------------------------------------------------------------


def test_repo_map_role_extracted_from_docstring() -> None:
    """For at least one tool, the role text in REPO-MAP.md matches the module docstring."""
    text = _read_repo_map()
    canon_re = re.compile(r"^Canon:\s+\S+\s+[—\-]\s+(.+)$")

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
        assert expected_role in text, (
            f"Role from {p.name} docstring not found in REPO-MAP.md.\n"
            f"Expected: {expected_role!r}"
        )
        return

    assert "spec/tools/gen_spec.py" in text, (
        "No Canon: docstring found in any tool; and gen_spec.py is missing."
    )


# ---------------------------------------------------------------------------
# Test 4 — at least one tool entry has R-tool-* cross-reference
# ---------------------------------------------------------------------------


def test_repo_map_tool_xref_present() -> None:
    """At least one tool entry in REPO-MAP.md must have a R-tool-* cross-reference."""
    text = _read_repo_map()
    xref_re = re.compile(r"→\s+R-tool-\S+")
    assert xref_re.search(text), (
        "No R-tool-* cross-reference found in REPO-MAP.md. "
        "At least one tool should have a Canon: §... marker producing a cross-reference."
    )
