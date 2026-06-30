"""Glossary sync: terminology drift is structurally impossible.

Three assertions:
  1. Every term in TERMS is referenced in at least one spec/src/hotam_spec/*.py
     docstring (no dead vocab).
  2. Every §-prefixed token in framework docstrings is in term_slugs()
     (no invented terms).
  3. GLOSSARY.md matches regeneration byte-for-byte (anti-drift).

Canon: §Glossary — these tests make R-glossary-sync-test ENFORCED.
"""

from __future__ import annotations

import ast
import re
import sys
from pathlib import Path

# ---------------------------------------------------------------------------
# Path setup — mirrors test_docs_gen.py
# ---------------------------------------------------------------------------

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_TOOLS = _SPEC_ROOT / "tools"
_TESTS = Path(__file__).resolve().parent
_SRC = _SPEC_ROOT / "src" / "hotam_spec"

for _p in (_TOOLS, _TESTS, _SPEC_ROOT / "src"):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import gen_spec  # noqa: E402

from hotam_spec.glossary import TERMS, term_slugs  # noqa: E402


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _read_normalized(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


def _all_framework_docstrings() -> str:
    """Collect every module + class + function docstring in spec/src/hotam_spec/*.py."""
    parts: list[str] = []
    for path in sorted(_SRC.glob("*.py")):
        tree = ast.parse(_read_normalized(path))
        # module docstring
        doc = ast.get_docstring(tree)
        if doc:
            parts.append(doc)
        for node in tree.body:
            if isinstance(node, (ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)):
                doc = ast.get_docstring(node)
                if doc:
                    parts.append(doc)
                # methods inside classes
                if isinstance(node, ast.ClassDef):
                    for child in node.body:
                        if isinstance(child, (ast.FunctionDef, ast.AsyncFunctionDef)):
                            doc = ast.get_docstring(child)
                            if doc:
                                parts.append(doc)
    return "\n".join(parts)


# ---------------------------------------------------------------------------
# Test 1: every TERMS entry is referenced in at least one docstring
# ---------------------------------------------------------------------------


def test_every_term_used_in_docstrings() -> None:
    """Every Term.slug in TERMS appears in at least one hotam_spec/*.py docstring.

    Canon: §Glossary — dead vocabulary (added to TERMS but never referenced
    in docstrings) is caught here. Fix: either add the slug to a relevant
    docstring or remove it from TERMS.
    """
    corpus = _all_framework_docstrings()
    missing = [t.slug for t in TERMS if t.slug not in corpus]
    assert not missing, (
        f"Terms in TERMS but never referenced in any hotam_spec docstring "
        f"(dead vocab — add to a docstring or remove from TERMS): {missing}"
    )


# ---------------------------------------------------------------------------
# Test 2: every §-token in framework docstrings is in term_slugs()
# ---------------------------------------------------------------------------


def test_section_tokens_in_docstrings_are_known() -> None:
    """Every §-prefixed token in hotam_spec docstrings is in term_slugs().

    Canon: §Glossary — invented §-tokens (written in a docstring but missing
    from TERMS) are caught here. Fix: add the term to glossary.TERMS or
    correct the typo in the docstring.
    """
    corpus = _all_framework_docstrings()
    known = term_slugs()
    # Match §Word or §Word-with-dashes — the §-section naming convention.
    found = set(re.findall(r"§[A-Za-z][\w-]*", corpus))
    unknown = found - known
    assert not unknown, (
        f"§-tokens in hotam_spec docstrings that are NOT in term_slugs() "
        f"(invented terms — add to TERMS or fix the docstring typo): {unknown}"
    )


# ---------------------------------------------------------------------------
# Test 3: GLOSSARY.md matches regeneration byte-for-byte
# ---------------------------------------------------------------------------


def test_glossary_md_up_to_date() -> None:
    """docs/gen/GLOSSARY.md matches regeneration byte-for-byte.

    Canon: §Glossary — the generated GLOSSARY.md cannot be hand-edited;
    only glossary.py can change it. Fix: run `uv run python tools/gen_spec.py`.
    """
    g = gen_spec.load_content_graph()
    expected = gen_spec.build_glossary(g)
    committed = _read_normalized(gen_spec.GLOSSARY_MD)
    assert expected == committed, (
        "GLOSSARY.md is stale: run `uv run python tools/gen_spec.py`."
    )
