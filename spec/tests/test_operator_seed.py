"""Tests for the Phase 2 crystal seed blocks: OPERATOR-ROLE, MEDIATION-LOOP,
OPERATOR-RECURSION.

Canon: §Operator — R-crystal-carries-role-seed, R-crystal-carries-mediation-loop,
R-crystal-carries-recursion-seed. Root CLAUDE.md carries a generated "resident
seed": one generative law (Role), a six-step input-processing procedure
(Mediation loop), and a description of sub-operator spawning as the same seed
narrowed (Recursion) — each a sentinel-bounded generated block, bounded in size
so the seed stays genuinely resident.
"""

from __future__ import annotations

from pathlib import Path


import gen_spec  # noqa: E402

ROOT_CLAUDE_MD = gen_spec.CLAUDE_MD

_ROLE_BEGIN = gen_spec._OPERATOR_ROLE_BEGIN
_ROLE_END = gen_spec._OPERATOR_ROLE_END
_LOOP_BEGIN = gen_spec._MEDIATION_LOOP_BEGIN
_LOOP_END = gen_spec._MEDIATION_LOOP_END
_RECURSION_BEGIN = gen_spec._OPERATOR_RECURSION_BEGIN
_RECURSION_END = gen_spec._OPERATOR_RECURSION_END


def _read_normalized(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


def _extract(text: str, begin: str, end: str) -> str | None:
    b = text.find(begin)
    e = text.find(end)
    if b == -1 or e == -1 or e <= b:
        return None
    return text[b + len(begin) : e].strip("\n")


# ---------------------------------------------------------------------------
# 1. Sentinels present
# ---------------------------------------------------------------------------


def test_seed_sentinels_present() -> None:
    """Root CLAUDE.md contains all three seed sentinel pairs."""
    text = _read_normalized(ROOT_CLAUDE_MD)
    for begin, end in (
        (_ROLE_BEGIN, _ROLE_END),
        (_LOOP_BEGIN, _LOOP_END),
        (_RECURSION_BEGIN, _RECURSION_END),
    ):
        assert begin in text, f"{ROOT_CLAUDE_MD} missing {begin} sentinel"
        assert end in text, f"{ROOT_CLAUDE_MD} missing {end} sentinel"


# ---------------------------------------------------------------------------
# 2. OPERATOR-ROLE content
# ---------------------------------------------------------------------------


def test_role_block_states_scope_and_law() -> None:
    """OPERATOR-ROLE names the active domain, the guardian role, the
    generative law, and cites its governing anchors."""
    g = gen_spec.load_content_graph()
    text = _read_normalized(ROOT_CLAUDE_MD)
    block = _extract(text, _ROLE_BEGIN, _ROLE_END)
    assert block is not None, "OPERATOR-ROLE block not found"

    domain = gen_spec._active_domain()
    domain_name = domain.name if domain else "hotam-spec-self"
    assert domain_name in block, f"OPERATOR-ROLE missing active domain {domain_name!r}"
    assert "guardian" in block.lower(), "OPERATOR-ROLE missing 'guardian' role text"
    assert "generative law" in block.lower(), (
        "OPERATOR-ROLE missing 'generative law' phrase"
    )
    for anchor in (
        "R-ai-presents-not-decides",
        "R-anchor-everything",
        "R-conflict-is-connector-node",
    ):
        assert anchor in block, f"OPERATOR-ROLE missing anchor {anchor}"

    # Anti-drift.
    assert block == gen_spec._render_operator_role_block(g), (
        "OPERATOR-ROLE block has drifted from gen_spec output. "
        "Run: uv run python tools/gen_spec.py"
    )


# ---------------------------------------------------------------------------
# 3. MEDIATION-LOOP content
# ---------------------------------------------------------------------------


def test_mediation_loop_names_real_tools() -> None:
    """MEDIATION-LOOP names all six steps and the real tool commands each uses."""
    text = _read_normalized(ROOT_CLAUDE_MD)
    block = _extract(text, _LOOP_BEGIN, _LOOP_END)
    assert block is not None, "MEDIATION-LOOP block not found"

    for verb in ("ORIENT", "LOCATE", "CONFRONT", "TRANSLATE", "PRESENT", "LAND"):
        assert verb in block, f"MEDIATION-LOOP missing step {verb}"

    for tool in (
        "what_now.py",
        "confront.py",
        "apply_proposal.py",
        "gen_spec.py",
        "gate.py",
        "pytest -q",
        "proposal.py",
        "--full",
    ):
        assert tool in block, f"MEDIATION-LOOP missing tool reference {tool!r}"

    assert block == gen_spec._render_mediation_loop_block(), (
        "MEDIATION-LOOP block has drifted from gen_spec output. "
        "Run: uv run python tools/gen_spec.py"
    )


# ---------------------------------------------------------------------------
# 4. OPERATOR-RECURSION content
# ---------------------------------------------------------------------------


def test_recursion_block_names_spawn_path() -> None:
    """OPERATOR-RECURSION names the spawn machinery and the conclusions-only contract."""
    text = _read_normalized(ROOT_CLAUDE_MD)
    block = _extract(text, _RECURSION_BEGIN, _RECURSION_END)
    assert block is not None, "OPERATOR-RECURSION block not found"

    for token in ("create_agent.py", "spawn_agent.py", "--stamp"):
        assert token in block, f"OPERATOR-RECURSION missing {token!r}"

    for anchor in (
        "R-delegation-conclusions-only",
        "R-claude-md-consolidates-when-single-agent",
    ):
        assert anchor in block, f"OPERATOR-RECURSION missing anchor {anchor}"

    assert block == gen_spec._render_operator_recursion_block(), (
        "OPERATOR-RECURSION block has drifted from gen_spec output. "
        "Run: uv run python tools/gen_spec.py"
    )


# ---------------------------------------------------------------------------
# 5. Size bounds — regression guard against seed bloat
# ---------------------------------------------------------------------------


def test_seed_blocks_bounded() -> None:
    """Each seed block stays under 3,000 chars; the sum stays under 7,000 —
    the seed must stay genuinely resident, not become a second catalog."""
    text = _read_normalized(ROOT_CLAUDE_MD)
    role = _extract(text, _ROLE_BEGIN, _ROLE_END)
    loop = _extract(text, _LOOP_BEGIN, _LOOP_END)
    recursion = _extract(text, _RECURSION_BEGIN, _RECURSION_END)
    assert role is not None and loop is not None and recursion is not None

    for name, block in (("OPERATOR-ROLE", role), ("MEDIATION-LOOP", loop), ("OPERATOR-RECURSION", recursion)):
        assert len(block) < 3_000, f"{name} block is {len(block)} chars — expected < 3,000"

    total = len(role) + len(loop) + len(recursion)
    assert total < 7_000, f"seed blocks total {total} chars — expected < 7,000"
