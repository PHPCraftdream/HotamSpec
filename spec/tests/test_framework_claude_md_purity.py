"""Locking tests for framework/domain CLAUDE.md separation (P17 domain isolation).

After P17 migration:
- Root CLAUDE.md is FRAMEWORK-ONLY: LIVE-STATE + REPO-MAP + DOMAIN-MAP.
  It must NOT contain CONSTITUTION, AGENT-MAP, or SHARED-DOCS sentinels.
- domains/tensio-self/CLAUDE.md is the domain operator-prompt:
  it must contain ALL 5 blocks: LIVE-STATE + CONSTITUTION + REPO-MAP + AGENT-MAP + SHARED-DOCS.

Canon: §Domain — R-framework-claude-md-is-domain-free, R-domain-owns-claude-md,
       R-domain-map-generated.
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


def test_framework_claude_md_has_no_constitution_sentinels() -> None:
    """Root CLAUDE.md must NOT contain CONSTITUTION:BEGIN (belongs in domain CLAUDE.md)."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P17 not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- CONSTITUTION:BEGIN -->" not in text, (
        "Root CLAUDE.md has CONSTITUTION:BEGIN sentinel — must be removed. "
        "Run: uv run python tools/gen_spec.py"
    )


def test_framework_claude_md_has_no_agent_map_sentinels() -> None:
    """Root CLAUDE.md must NOT contain AGENT-MAP:BEGIN (belongs in domain CLAUDE.md)."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P17 not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- AGENT-MAP:BEGIN -->" not in text, (
        "Root CLAUDE.md has AGENT-MAP:BEGIN sentinel — must be removed. "
        "Run: uv run python tools/gen_spec.py"
    )


def test_framework_claude_md_has_no_shared_docs_sentinels() -> None:
    """Root CLAUDE.md must NOT contain SHARED-DOCS:BEGIN (belongs in domain CLAUDE.md)."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P17 not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- SHARED-DOCS:BEGIN -->" not in text, (
        "Root CLAUDE.md has SHARED-DOCS:BEGIN sentinel — must be removed. "
        "Run: uv run python tools/gen_spec.py"
    )


def test_framework_claude_md_has_live_state() -> None:
    """Root CLAUDE.md must contain a LIVE-STATE:BEGIN sentinel."""
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- LIVE-STATE:BEGIN -->" in text, (
        "Root CLAUDE.md is missing LIVE-STATE:BEGIN sentinel"
    )


def test_framework_claude_md_has_domain_map() -> None:
    """Root CLAUDE.md must contain a populated DOMAIN-MAP block referencing tensio-self."""
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P17 not applicable")
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- DOMAIN-MAP:BEGIN -->" in text, (
        "Root CLAUDE.md missing DOMAIN-MAP:BEGIN"
    )
    block = _extract_block(text, "<!-- DOMAIN-MAP:BEGIN -->", "<!-- DOMAIN-MAP:END -->")
    assert block is not None, "DOMAIN-MAP block not found in root CLAUDE.md"
    assert "tensio-self" in block, "DOMAIN-MAP block does not reference 'tensio-self'"


def test_framework_claude_md_has_repo_map_scoped_to_spec() -> None:
    """Root CLAUDE.md REPO-MAP must reference spec/ files; domain content listed under domains/."""
    text = _read(ROOT_CLAUDE_MD)
    assert "<!-- REPO-MAP:BEGIN -->" in text, "Root CLAUDE.md missing REPO-MAP:BEGIN"
    block = _extract_block(text, "<!-- REPO-MAP:BEGIN -->", "<!-- REPO-MAP:END -->")
    assert block is not None, "REPO-MAP block not found in root CLAUDE.md"
    # Framework body files must be present.
    assert "spec/src/tensio/" in block, "REPO-MAP missing framework body section"
    assert "spec/tools/" in block, "REPO-MAP missing tools section"


def test_framework_claude_md_no_domain_atoms() -> None:
    """Root CLAUDE.md body must not list R-id atoms (only DOMAIN-MAP may reference atom-counts).

    'atoms-count' references like '108 SETTLED' in DOMAIN-MAP are fine — what is
    forbidden is R-<id> atom lines in the body (CONSTITUTION-style entries).
    """
    if _ACTIVE_DOMAIN is None:
        pytest.skip("No active domain — P17 not applicable")
    text = _read(ROOT_CLAUDE_MD)

    # Strip known blocks that may legitimately reference R-ids.
    # The LIVE-STATE, REPO-MAP, and DOMAIN-MAP are allowed to reference R-ids
    # only in a cross-reference capacity (e.g. DOMAIN-MAP atoms-count line).
    # The CONSTITUTION-style bullet pattern is "- **R-<id>** — *<claim>*".
    constitution_bullet_pattern = re.compile(
        r"^\s*-\s+\*\*R-[a-z][a-zA-Z0-9-]+\*\*\s+[—\-]", re.MULTILINE
    )
    matches = constitution_bullet_pattern.findall(text)
    assert not matches, (
        f"Root CLAUDE.md contains {len(matches)} CONSTITUTION-style R-id bullet(s) "
        "— domain atoms must live in the domain CLAUDE.md, not root. "
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
    """domains/tensio-self/CLAUDE.md must contain all 5 sentinel block pairs."""
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
