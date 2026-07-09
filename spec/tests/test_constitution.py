"""Tests for the CONSTITUTION block in root CLAUDE.md (P22.C consolidation).

After P22.C, there is exactly ONE CLAUDE.md file (repo root). The CONSTITUTION
block — all SETTLED requirements grouped by category — renders directly into
root CLAUDE.md's own sentinels (no more domain-file indirection).

Canon: §Constitution — the CONSTITUTION block lists all SETTLED requirements
grouped by category, generated deterministically from
domains/hotam-spec-self/graph.py by tools/gen_spec.py. Anti-drift: regeneration
must produce byte-identical output.
"""

from __future__ import annotations

from pathlib import Path


import gen_spec  # noqa: E402

REPO_ROOT = Path(__file__).resolve().parents[2]
ROOT_CLAUDE_MD = gen_spec.CLAUDE_MD

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
# 1. Sentinels present in root CLAUDE.md
# ---------------------------------------------------------------------------


def test_constitution_sentinels_present() -> None:
    """Root CLAUDE.md contains both CONSTITUTION sentinels."""
    text = _read_normalized(ROOT_CLAUDE_MD)
    assert _CONST_BEGIN in text, f"{ROOT_CLAUDE_MD} missing CONSTITUTION:BEGIN sentinel"
    assert _CONST_END in text, f"{ROOT_CLAUDE_MD} missing CONSTITUTION:END sentinel"


def test_root_claude_md_has_exactly_one_constitution_block() -> None:
    """Root CLAUDE.md must contain the CONSTITUTION sentinel pair exactly once.

    Post-R-claude-md-template-driven: root CLAUDE.md is generated directly
    from CLAUDE.md.template.txt via render_business_content(), which
    includes CONSTITUTION once. The guarantee that matters is "not
    duplicated" — there is exactly one CLAUDE.md file in the whole repo
    (P22.C consolidation, tasks #101/#102).
    """
    root_text = _read_normalized(gen_spec.CLAUDE_MD)
    assert root_text.count(_CONST_BEGIN) == 1, (
        "Root CLAUDE.md must contain exactly one CONSTITUTION:BEGIN sentinel — "
        "run gen_spec.py to fix"
    )


# ---------------------------------------------------------------------------
# 2. Anti-drift: regeneration produces identical block
# ---------------------------------------------------------------------------


def test_constitution_block_generated() -> None:
    """Regenerating gen_spec produces byte-identical CONSTITUTION block in root CLAUDE.md."""
    g = gen_spec.load_content_graph()
    expected_block = gen_spec._render_constitution_block(g)

    text = _read_normalized(ROOT_CLAUDE_MD)
    actual_block = _extract_constitution_block(text)

    assert actual_block is not None, f"CONSTITUTION block not found in {ROOT_CLAUDE_MD}"
    assert actual_block == expected_block, (
        "CONSTITUTION block in root CLAUDE.md has drifted from gen_spec output. "
        "Run: uv run python tools/gen_spec.py"
    )


# ---------------------------------------------------------------------------
# 3. Every SETTLED requirement id appears in the block
# ---------------------------------------------------------------------------


def _partition_check() -> None:
    """Shared body: every SETTLED id appears in exactly ONE of the two
    generated locations — the root CONSTITUTION index (business + discipline)
    or docs/gen/FRAMEWORK-INVARIANTS.md (framework-plumbing) — and the union
    of both equals the full SETTLED set (nothing lost by the relocation).

    Two test names call this: test_constitution_lists_all_settled (the
    pre-Phase-3 name, kept so the pre-existing R-constitution-is-index
    enforced_by reference stays resolvable — R-bijection-r-to-enforcer) and
    test_constitution_partitions_all_settled (the Phase-3 name naming the
    partition explicitly, referenced by R-constitution-separates-plumbing).
    """
    g = gen_spec.load_content_graph()
    text = _read_normalized(ROOT_CLAUDE_MD)
    block = _extract_constitution_block(text)
    assert block is not None, "CONSTITUTION block not found"

    domain = gen_spec._active_domain()
    domain_name = domain.name if domain else "hotam-spec-self"
    invariants_path = (
        REPO_ROOT / "domains" / domain_name / "docs" / "gen" / "FRAMEWORK-INVARIANTS.md"
    )
    invariants_text = _read_normalized(invariants_path)

    settled = [r for r in g.requirements if r.status == gen_spec.SETTLED]
    in_constitution: list[str] = []
    in_invariants: list[str] = []
    in_neither: list[str] = []
    in_both: list[str] = []
    for r in settled:
        c = r.id in block
        i = r.id in invariants_text
        if c and i:
            in_both.append(r.id)
        elif c:
            in_constitution.append(r.id)
        elif i:
            in_invariants.append(r.id)
        else:
            in_neither.append(r.id)

    assert not in_neither, f"SETTLED ids missing from BOTH locations: {in_neither}"
    assert not in_both, f"SETTLED ids duplicated in BOTH locations: {in_both}"
    assert len(in_constitution) + len(in_invariants) == len(settled)


def test_constitution_lists_all_settled() -> None:
    """Pre-Phase-3 name, kept resolvable for R-constitution-is-index.enforced_by.

    See _partition_check docstring: the assertion now checks the two-location
    partition rather than single-block membership, since Phase 3 relocated
    framework-plumbing atoms out of the root CONSTITUTION block.
    """
    _partition_check()


def test_constitution_partitions_all_settled() -> None:
    """Phase 3 (task #9) name for _partition_check, referenced by
    R-constitution-separates-plumbing.enforced_by."""
    _partition_check()


def test_constitution_pointer_to_framework_invariants() -> None:
    """The CONSTITUTION index carries an in-block pointer to FRAMEWORK-INVARIANTS.md."""
    text = _read_normalized(ROOT_CLAUDE_MD)
    block = _extract_constitution_block(text)
    assert block is not None, "CONSTITUTION block not found"
    assert "FRAMEWORK-INVARIANTS.md" in block, (
        "CONSTITUTION index must point to the relocated framework-plumbing "
        "index at docs/gen/FRAMEWORK-INVARIANTS.md"
    )


def test_framework_invariants_md_up_to_date() -> None:
    """docs/gen/FRAMEWORK-INVARIANTS.md matches regeneration from the graph."""
    g = gen_spec.load_content_graph()
    domain = gen_spec._active_domain()
    domain_name = domain.name if domain else "hotam-spec-self"
    invariants_path = (
        REPO_ROOT / "domains" / domain_name / "docs" / "gen" / "FRAMEWORK-INVARIANTS.md"
    )
    assert invariants_path.exists(), (
        f"{invariants_path} does not exist — run `uv run python tools/gen_spec.py`."
    )
    expected = gen_spec.build_framework_invariants(g)
    actual = _read_normalized(invariants_path)
    assert expected == actual, (
        "FRAMEWORK-INVARIANTS.md is stale: run `uv run python tools/gen_spec.py`."
    )


# ---------------------------------------------------------------------------
# 4. Phase 2: CONSTITUTION is an index, not a catalog (R-constitution-is-index)
# ---------------------------------------------------------------------------


def test_constitution_is_index() -> None:
    """CONSTITUTION block is a compact index: roster pointer present, bounded,
    id+flag format with no claims inline."""
    text = _read_normalized(ROOT_CLAUDE_MD)
    block = _extract_constitution_block(text)
    assert block is not None, "CONSTITUTION block not found"

    assert "docs/gen/REQUIREMENTS.md" in block, (
        "CONSTITUTION index must point to the full roster in "
        "docs/gen/REQUIREMENTS.md"
    )
    assert len(block) < 6_000, (
        f"CONSTITUTION index block is {len(block)} chars — expected < 6,000 "
        "chars for the id+flag-only format. Run: uv run python tools/gen_spec.py"
    )
    assert "[ENFORCED·" not in block, (
        "CONSTITUTION block still contains full '[ENFORCED·...]' enforcer "
        "chains — expected the compact index format ([E]/[S]/[P] flags only)."
    )
