"""Locking tests for root CLAUDE.md as the SOLE operator-prompt (P22.C consolidation).

After P22.C, there is exactly ONE CLAUDE.md file in the whole repo (repo root).
It contains everything: LIVE-STATE, DOMAIN-MAP, REPO-MAP, CONSTITUTION,
AGENT-MAP, CONCEPT-MAP, THINKING-INDEX, EMBEDDED-THINKING, EMBEDDED-TOOLS,
RECENTLY-REJECTED. The domains/hotam-spec-self/CLAUDE.md file and the
domains/hotam-spec-self/agents/ scaffold tree (director + framework-agent)
have been deleted — see task #101.

Canon: §Domain — R-domain-map-generated, R-crystal-is-claude-md.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent

_tools = str(SPEC_ROOT / "tools")
if _tools not in sys.path:
    sys.path.insert(0, _tools)

ROOT_CLAUDE_MD = REPO_ROOT / "CLAUDE.md"


def _read(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


def _extract_block(text: str, begin: str, end: str) -> str | None:
    bp = text.find(begin)
    ep = text.find(end)
    if bp == -1 or ep == -1 or ep <= bp:
        return None
    return text[bp + len(begin) : ep]


def test_exactly_one_claude_md_in_repo() -> None:
    """Exactly one CLAUDE.md file shall exist in the whole repo (repo root)."""
    found = [
        p
        for p in REPO_ROOT.rglob("CLAUDE.md")
        if ".venv" not in p.parts and "node_modules" not in p.parts
    ]
    assert found == [ROOT_CLAUDE_MD], (
        f"Expected exactly one CLAUDE.md at {ROOT_CLAUDE_MD}, found: {found}"
    )


def test_domain_claude_md_and_agent_scaffold_tree_deleted() -> None:
    """domains/hotam-spec-self/CLAUDE.md and the nested agent scaffold are gone (P22.C).

    A minimal domains/hotam-spec-self/agents/director/scope.py identity marker
    remains (required by check_domain_director_exists / R-domain-declares-director)
    — but it carries no CLAUDE.md, no docs/, no nested agents/ (the deleted
    framework-agent scaffold lived at agents/director/agents/framework-agent/).
    """
    domain_dir = REPO_ROOT / "domains" / "hotam-spec-self"
    assert not (domain_dir / "CLAUDE.md").exists(), (
        "domains/hotam-spec-self/CLAUDE.md should have been deleted in the "
        "P22.C consolidation to a single root CLAUDE.md."
    )
    director_dir = domain_dir / "agents" / "director"
    assert not (director_dir / "CLAUDE.md").exists(), (
        "domains/hotam-spec-self/agents/director/CLAUDE.md should have been deleted."
    )
    assert not (director_dir / "agents").exists(), (
        "domains/hotam-spec-self/agents/director/agents/ (the former framework-agent "
        "scaffold) should have been deleted in the P22.C consolidation — it was a "
        "dormant P21 dogfood demo, never actually spawned."
    )


def test_framework_claude_md_has_live_state() -> None:
    """Root CLAUDE.md must contain a LIVE-STATE:BEGIN sentinel."""
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- LIVE-STATE:BEGIN -->" in text, (
        "Root CLAUDE.md is missing LIVE-STATE:BEGIN sentinel"
    )


def test_framework_claude_md_has_domain_map() -> None:
    """Root CLAUDE.md must contain a populated DOMAIN-MAP block referencing hotam-spec-self."""
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
    assert "spec/src/hotam_spec/" in block, "REPO-MAP missing framework body section"
    assert "spec/tools/" in block, "REPO-MAP missing tools section"


def test_framework_claude_md_has_recently_rejected_sentinels() -> None:
    """Root CLAUDE.md must contain RECENTLY-REJECTED:BEGIN and RECENTLY-REJECTED:END sentinels."""
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- RECENTLY-REJECTED:BEGIN -->" in text, (
        "Root CLAUDE.md missing RECENTLY-REJECTED:BEGIN sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )
    assert "<!-- RECENTLY-REJECTED:END -->" in text, (
        "Root CLAUDE.md missing RECENTLY-REJECTED:END sentinel. "
        "Run: uv run python tools/gen_spec.py"
    )


def test_root_claude_md_constitution_has_atoms() -> None:
    """CONSTITUTION block in root CLAUDE.md must be non-empty (contains R-id entries)."""
    text = _read(ROOT_CLAUDE_MD)
    block = _extract_block(
        text, "<!-- CONSTITUTION:BEGIN -->", "<!-- CONSTITUTION:END -->"
    )
    assert block is not None, "CONSTITUTION block not found in root CLAUDE.md"
    assert re.search(r"\bR-[a-z][a-zA-Z0-9-]+", block), (
        "CONSTITUTION block in root CLAUDE.md appears empty (no R-id entries found)"
    )


def test_root_claude_md_agent_map_has_no_stale_scaffold_reference() -> None:
    """AGENT-MAP in root CLAUDE.md must NOT reference the deleted framework-agent scaffold."""
    text = _read(ROOT_CLAUDE_MD)
    block = _extract_block(text, "<!-- AGENT-MAP:BEGIN -->", "<!-- AGENT-MAP:END -->")
    assert block is not None, "AGENT-MAP block not found in root CLAUDE.md"
    assert "framework-agent" not in block, (
        "root CLAUDE.md AGENT-MAP still references deleted framework-agent scaffold"
    )
    assert "no sub-operators yet" in block, (
        "root CLAUDE.md AGENT-MAP should show the calm placeholder — no agents exist"
    )
