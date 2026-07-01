"""Locking tests for framework/domain CLAUDE.md content (post R-claude-md-template-driven).

Historical note (P17 domain isolation, superseded): root CLAUDE.md used to be
FRAMEWORK-ONLY (LIVE-STATE + REPO-MAP + DOMAIN-MAP) with CONSTITUTION/AGENT-MAP/
SHARED-DOCS nested one level down inside a DOMAIN-CRYSTAL block that embedded a
full second copy of domains/<active>/CLAUDE.md. R-claude-md-template-driven
replaced that split: root CLAUDE.md is now the single unified crystal, generated
directly from CLAUDE.md.template.txt, and legitimately contains CONSTITUTION/
AGENT-MAP/CONCEPT-MAP once each, directly (no nested DOMAIN-CRYSTAL).

- domains/hotam-spec-self/CLAUDE.md remains the domain's OWN operator-prompt
  (used directly by per-domain tooling and sub-agents): it still must contain
  ALL 5 blocks: LIVE-STATE + CONSTITUTION + REPO-MAP + AGENT-MAP + SHARED-DOCS.

Canon: §Domain — R-domain-owns-claude-md, R-domain-map-generated,
       R-claude-md-template-driven.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import pytest

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent

_tools = str(SPEC_ROOT / "tools")
if _tools not in sys.path:
    sys.path.insert(0, _tools)

import gen_spec as _gs  # noqa: E402

ROOT_CLAUDE_MD = REPO_ROOT / "CLAUDE.md"
_ACTIVE_DOMAIN = _gs._active_domain()
DOMAIN_CLAUDE_MD = _ACTIVE_DOMAIN / "CLAUDE.md" if _ACTIVE_DOMAIN is not None else None


def _read(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


def _extract_block(text: str, begin: str, end: str) -> str | None:
    bp = text.find(begin)
    ep = text.find(end)
    if bp == -1 or ep == -1 or ep <= bp:
        return None
    return text[bp + len(begin) : ep]


# ===========================================================================
# Root CLAUDE.md — framework-only assertions
# ===========================================================================


def _strip_domain_crystal(text: str) -> str:
    """Remove the DOMAIN-CRYSTAL block (and its content) from text."""
    begin = "<!-- DOMAIN-CRYSTAL:BEGIN -->"
    end = "<!-- DOMAIN-CRYSTAL:END -->"
    bp = text.find(begin)
    ep = text.find(end)
    if bp == -1 or ep == -1 or ep <= bp:
        return text
    return text[:bp] + text[ep + len(end) :]


def _strip_recently_rejected(text: str) -> str:
    """Remove the RECENTLY-REJECTED block (and its content) from text."""
    begin = "<!-- RECENTLY-REJECTED:BEGIN -->"
    end = "<!-- RECENTLY-REJECTED:END -->"
    bp = text.find(begin)
    ep = text.find(end)
    if bp == -1 or ep == -1 or ep <= bp:
        return text
    return text[:bp] + text[ep + len(end) :]


def test_framework_claude_md_constitution_exactly_once() -> None:
    """CONSTITUTION:BEGIN must appear exactly once in root CLAUDE.md (no nested duplicate).

    Post-R-claude-md-template-driven: CONSTITUTION is rendered directly into
    root's BUSINESS bucket by render_business_content() -- there is no longer
    a second copy nested inside a DOMAIN-CRYSTAL-embedded file.
    """
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert text.count("<!-- CONSTITUTION:BEGIN -->") == 1, (
        "Root CLAUDE.md must contain exactly one CONSTITUTION:BEGIN sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )


def test_framework_claude_md_agent_map_exactly_once() -> None:
    """AGENT-MAP:BEGIN must appear exactly once in root CLAUDE.md (no nested duplicate)."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert text.count("<!-- AGENT-MAP:BEGIN -->") == 1, (
        "Root CLAUDE.md must contain exactly one AGENT-MAP:BEGIN sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )


def test_framework_claude_md_has_no_domain_crystal_sentinels() -> None:
    """Root CLAUDE.md must NOT contain DOMAIN-CRYSTAL sentinels (retired by template model)."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- DOMAIN-CRYSTAL:BEGIN -->" not in text, (
        "Root CLAUDE.md still has a DOMAIN-CRYSTAL:BEGIN sentinel — this nested-embedding "
        "mechanism was retired by R-claude-md-template-driven."
    )
    assert "<!-- DOMAIN-CRYSTAL:END -->" not in text, (
        "Root CLAUDE.md still has a DOMAIN-CRYSTAL:END sentinel — this nested-embedding "
        "mechanism was retired by R-claude-md-template-driven."
    )


def test_framework_claude_md_has_recently_rejected_sentinels() -> None:
    """Root CLAUDE.md must contain RECENTLY-REJECTED:BEGIN and RECENTLY-REJECTED:END sentinels."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P22.B not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- RECENTLY-REJECTED:BEGIN -->" in text, (
        "Root CLAUDE.md missing RECENTLY-REJECTED:BEGIN sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )
    assert "<!-- RECENTLY-REJECTED:END -->" in text, (
        "Root CLAUDE.md missing RECENTLY-REJECTED:END sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )


def test_framework_claude_md_has_live_state() -> None:
    """Root CLAUDE.md must contain a LIVE-STATE:BEGIN sentinel."""
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- LIVE-STATE:BEGIN -->" in text, (
        "Root CLAUDE.md is missing LIVE-STATE:BEGIN sentinel"
    )


def test_framework_claude_md_has_domain_map() -> None:
    """Root CLAUDE.md must contain a populated DOMAIN-MAP block referencing hotam-spec-self."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P17 not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- DOMAIN-MAP:BEGIN -->" in text, (
        "Root CLAUDE.md missing DOMAIN-MAP:BEGIN"
    )
    block = _extract_block(text, "<!-- DOMAIN-MAP:BEGIN -->", "<!-- DOMAIN-MAP:END -->")
    assert block is not None, "DOMAIN-MAP block not found in root CLAUDE.md"
    assert "hotam-spec-self" in block, (
        "DOMAIN-MAP block does not reference 'hotam-spec-self'"
    )


def test_framework_claude_md_has_repo_map_scoped_to_spec() -> None:
    """Root CLAUDE.md REPO-MAP must reference spec/ files; domain content listed under domains/."""
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- REPO-MAP:BEGIN -->" in text, "Root CLAUDE.md missing REPO-MAP:BEGIN"
    block = _extract_block(text, "<!-- REPO-MAP:BEGIN -->", "<!-- REPO-MAP:END -->")
    assert block is not None, "REPO-MAP block not found in root CLAUDE.md"
    # Framework body files must be present.
    assert "spec/src/hotam_spec/" in block, "REPO-MAP missing framework body section"
    assert "spec/tools/" in block, "REPO-MAP missing tools section"


def test_framework_claude_md_domain_atoms_not_duplicated() -> None:
    """Root CLAUDE.md's CONSTITUTION-style R-id bullets must appear only within its
    single CONSTITUTION block (plus legitimate mentions in RECENTLY-REJECTED).

    Post-R-claude-md-template-driven: domain atoms legitimately live directly in
    root CLAUDE.md's CONSTITUTION block (there is no longer a separate
    domain-only vs. framework-only split at the root level). What this test
    guards against is DUPLICATION -- e.g. a stray leftover second CONSTITUTION
    rendering outside the one CONSTITUTION:BEGIN/END pair.
    """
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — not applicable")
    text = _read(ROOT_CLAUDE_MD)

    # Strip the one legitimate CONSTITUTION block and the RECENTLY-REJECTED block.
    const_begin, const_end = "<!-- CONSTITUTION:BEGIN -->", "<!-- CONSTITUTION:END -->"
    bp, ep = text.find(const_begin), text.find(const_end)
    if bp != -1 and ep != -1 and ep > bp:
        text = text[:bp] + text[ep + len(const_end) :]
    text = _strip_recently_rejected(text)

    # The CONSTITUTION-style bullet pattern is "- **R-<id>** — *<claim>*".
    constitution_bullet_pattern = re.compile(
        r"^\s*-\s+\*\*R-[a-z][a-zA-Z0-9-]+\*\*\s+[—\-]\s+\*", re.MULTILINE
    )
    matches = constitution_bullet_pattern.findall(text)
    assert not matches, (
        f"Root CLAUDE.md contains {len(matches)} CONSTITUTION-style R-id bullet(s) "
        "outside its single CONSTITUTION/RECENTLY-REJECTED blocks — looks duplicated. "
        f"First match: {matches[0]!r}"
    )


# ===========================================================================
# Domain CLAUDE.md — all 5 blocks populated
# ===========================================================================


def _require_domain_claude_md() -> Path:
    if DOMAIN_CLAUDE_MD is None:
        pytest.skip("No active domain — P17 not applicable")
    return DOMAIN_CLAUDE_MD


def test_domain_claude_md_has_all_5_blocks() -> None:
    """domains/hotam-spec-self/CLAUDE.md must contain all 5 sentinel block pairs."""
    path = _require_domain_claude_md()
    text = _read(path)
    for sentinel in [
        "<!-- LIVE-STATE:BEGIN -->",
        "<!-- LIVE-STATE:END -->",
        "<!-- CONSTITUTION:BEGIN -->",
        "<!-- CONSTITUTION:END -->",
        "<!-- REPO-MAP:BEGIN -->",
        "<!-- REPO-MAP:END -->",
        "<!-- AGENT-MAP:BEGIN -->",
        "<!-- AGENT-MAP:END -->",
        "<!-- SHARED-DOCS:BEGIN -->",
        "<!-- SHARED-DOCS:END -->",
    ]:
        assert sentinel in text, (
            f"Domain CLAUDE.md missing sentinel: {sentinel}. "
            "Run: uv run python tools/gen_spec.py"
        )


def test_domain_claude_md_constitution_has_atoms() -> None:
    """CONSTITUTION block in domain CLAUDE.md must be non-empty (contains R-id entries)."""
    path = _require_domain_claude_md()
    text = _read(path)
    block = _extract_block(
        text, "<!-- CONSTITUTION:BEGIN -->", "<!-- CONSTITUTION:END -->"
    )
    assert block is not None, "CONSTITUTION block not found in domain CLAUDE.md"
    # Must contain at least one R-id atom entry.
    assert re.search(r"\bR-[a-z][a-zA-Z0-9-]+", block), (
        "CONSTITUTION block in domain CLAUDE.md appears empty (no R-id entries found)"
    )


def test_domain_claude_md_agent_map_lists_director() -> None:
    """AGENT-MAP in domain CLAUDE.md must list the framework-agent (sub-agent of director)."""
    path = _require_domain_claude_md()
    text = _read(path)
    block = _extract_block(text, "<!-- AGENT-MAP:BEGIN -->", "<!-- AGENT-MAP:END -->")
    assert block is not None, "AGENT-MAP block not found in domain CLAUDE.md"
    assert "framework-agent" in block, (
        "domain CLAUDE.md AGENT-MAP does not list framework-agent"
    )
