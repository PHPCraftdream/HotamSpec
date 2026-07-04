"""Tests for the tool-as-requirement projection (R-tool-is-its-own-requirement).

Verifies that every spec/tools/*.py file whose module docstring opens with
'Canon: §<topic> — <claim>' is correctly projected into the human layer and
that the generated docs stay stable under regeneration.
"""

from __future__ import annotations

import sys
from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent
TOOLS_DIR = SPEC_ROOT / "tools"
CLAUDE_MD = REPO_ROOT / "CLAUDE.md"

# Make gen_spec importable.
if str(TOOLS_DIR) not in sys.path:
    sys.path.insert(0, str(TOOLS_DIR))
if str(SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(SPEC_ROOT / "src"))

import gen_spec as _gen_spec_mod  # noqa: E402
from gen_spec import _scan_tool_requirements  # noqa: E402

# Use the gen dir resolved by gen_spec (may be in active domain after P17 migration).
GEN_DIR = _gen_spec_mod.GEN_DIR


def test_scan_returns_all_canon_tools() -> None:
    """Every spec/tools/*.py with Canon: §... first line appears in scan results."""
    import ast
    import re

    CANON_RE = re.compile(r"^Canon:\s+§(.+?)\s+[—\-]\s+(.+)$")
    expected_basenames: set[str] = set()
    for path in sorted(TOOLS_DIR.glob("*.py")):
        if path.name.startswith("_"):
            continue
        try:
            src = path.read_text(encoding="utf-8")
            tree = ast.parse(src)
            doc = ast.get_docstring(tree) or ""
        except Exception:
            continue
        first_line = doc.split("\n")[0].strip() if doc else ""
        if CANON_RE.match(first_line):
            expected_basenames.add(path.stem)

    scanned_basenames = {tr.basename for tr in _scan_tool_requirements()}
    assert expected_basenames == scanned_basenames, (
        f"scan mismatch — expected {sorted(expected_basenames)}, got {sorted(scanned_basenames)}"
    )


def test_tools_with_canon_appear_in_claude_md() -> None:
    """Every R-tool-<basename> projected from a Canon: marker appears in CLAUDE.md."""
    tool_reqs = _scan_tool_requirements()
    assert tool_reqs, (
        "no tool requirements found — at least some tools should have Canon: markers"
    )
    claude_text = CLAUDE_MD.read_text(encoding="utf-8")
    missing = [tr.id for tr in tool_reqs if tr.id not in claude_text]
    assert not missing, (
        f"These R-tool-* ids are missing from CLAUDE.md: {missing}. "
        "Run `uv run python tools/gen_spec.py` to regenerate."
    )


def test_tool_derived_section_present_in_requirements_md() -> None:
    """The '## Tool-derived requirements' section header exists in docs/gen/REQUIREMENTS.md."""
    req_md = GEN_DIR / "REQUIREMENTS.md"
    assert req_md.exists(), f"REQUIREMENTS.md not found at {req_md}"
    text = req_md.read_text(encoding="utf-8")
    assert "## Tool-derived requirements" in text, (
        "'## Tool-derived requirements' section is missing from REQUIREMENTS.md. "
        "Run `uv run python tools/gen_spec.py` to regenerate."
    )


def test_tool_derived_ids_appear_in_requirements_md() -> None:
    """Every R-tool-<basename> id projected from Canon: markers appears in REQUIREMENTS.md."""
    tool_reqs = _scan_tool_requirements()
    req_md = GEN_DIR / "REQUIREMENTS.md"
    assert req_md.exists()
    text = req_md.read_text(encoding="utf-8")
    missing = [tr.id for tr in tool_reqs if tr.id not in text]
    assert not missing, (
        f"These R-tool-* ids are missing from REQUIREMENTS.md: {missing}. "
        "Run `uv run python tools/gen_spec.py` to regenerate."
    )


def test_tool_derived_requirements_md_is_fresh(gen_spec_snapshot) -> None:
    """The committed REQUIREMENTS.md equals a FRESH gen_spec regeneration.

    Task #46, Measure 4: gen_spec byte-idempotency is proven once in
    test_gen_spec_idempotency.py. This test no longer spawns a subprocess to
    re-prove stability; instead it confirms the committed REQUIREMENTS.md matches
    the session-scoped freshly-generated snapshot (Measure 1) — i.e. the on-disk
    doc is not stale relative to the current substrate.
    """
    fresh = gen_spec_snapshot["docs_gen"].get("REQUIREMENTS.md")
    assert fresh is not None, "Fresh gen_spec produced no REQUIREMENTS.md"
    committed = (
        (GEN_DIR / "REQUIREMENTS.md").read_text(encoding="utf-8").replace("\r\n", "\n")
    )
    assert committed == fresh, (
        "Committed REQUIREMENTS.md differs from a fresh gen_spec run — output is "
        "stale or non-deterministic. Run: uv run python tools/gen_spec.py"
    )
