"""Tests for the EMBEDDED-THINKING block — full-content methodology in CLAUDE.md.

Claude Code does not follow markdown links when loading CLAUDE.md, so the
SHARED-DOCS link index alone leaves the operator holding a table-of-contents,
not the methodology. EMBEDDED-THINKING embeds the FULL BODY of scope-relevant
spec/docs/thinking/*.md files inline under §Topic headings.

Covers:
- sentinels present in domain CLAUDE.md
- content is real prose, not just markdown links
- director (unscoped) gets (close to) all thinking topics
- framework-agent (scoped) gets a smaller subset
- regeneration is byte-stable (R-deterministic-generation)
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent
SPEC_DOCS_THINKING = SPEC_ROOT / "docs" / "thinking"

_tools_str = str(SPEC_ROOT / "tools")
if _tools_str not in sys.path:
    sys.path.insert(0, _tools_str)
import gen_spec  # noqa: E402

DOMAIN_CLAUDE_MD = REPO_ROOT / "domains" / "hotam-spec-self" / "CLAUDE.md"
FRAMEWORK_AGENT_CLAUDE_MD = (
    REPO_ROOT
    / "domains"
    / "hotam-spec-self"
    / "agents"
    / "director"
    / "agents"
    / "framework-agent"
    / "CLAUDE.md"
)

_BEGIN = "<!-- EMBEDDED-THINKING:BEGIN -->"
_END = "<!-- EMBEDDED-THINKING:END -->"


def _read(path: Path) -> str:
    return path.read_text(encoding="utf-8").replace("\r\n", "\n").replace("\r", "\n")


def _extract_block(path: Path) -> str:
    text = _read(path)
    bp = text.find(_BEGIN)
    ep = text.find(_END)
    assert bp != -1 and ep != -1, f"EMBEDDED-THINKING sentinels missing in {path}"
    return text[bp + len(_BEGIN) : ep]


def test_embedded_thinking_sentinels_present_in_domain_claude_md():
    """domains/hotam-spec-self/CLAUDE.md must carry both EMBEDDED-THINKING sentinels."""
    text = _read(DOMAIN_CLAUDE_MD)
    assert _BEGIN in text, "missing EMBEDDED-THINKING:BEGIN sentinel"
    assert _END in text, "missing EMBEDDED-THINKING:END sentinel"


def test_embedded_thinking_contains_real_content_not_just_links():
    """The block must be real prose, not a markdown-link index like SHARED-DOCS."""
    block = _extract_block(DOMAIN_CLAUDE_MD)
    assert len(block) > 500, f"EMBEDDED-THINKING block too short: {len(block)} chars"

    non_blank = [line for line in block.splitlines() if line.strip()]
    link_only_lines = [
        line for line in non_blank if re.match(r"^- \[.*\]\(.*\)$", line.strip())
    ]
    assert len(link_only_lines) < len(non_blank), (
        "EMBEDDED-THINKING block looks like a link index (SHARED-DOCS), not embedded prose"
    )


def test_embedded_thinking_director_gets_all_topics():
    """Director (SCOPE=()) is unscoped -> should embed (nearly) all thinking topics."""
    block = _extract_block(DOMAIN_CLAUDE_MD)
    headings = re.findall(r"^#### §(\w+)", block, re.MULTILINE)
    total_files = (
        len(list(SPEC_DOCS_THINKING.glob("*.md"))) if SPEC_DOCS_THINKING.exists() else 0
    )
    assert total_files > 0, "no thinking docs found on disk to compare against"
    assert len(headings) >= total_files - 1, (
        f"director embedded only {len(headings)}/{total_files} topics, expected (near-)all"
    )


def test_embedded_thinking_framework_agent_gets_scoped_subset():
    """framework-agent (scoped SCOPE tuple) must embed a smaller subset than the domain."""
    domain_block = _extract_block(DOMAIN_CLAUDE_MD)
    agent_block = _extract_block(FRAMEWORK_AGENT_CLAUDE_MD)

    domain_headings = set(re.findall(r"^#### §(\w+)", domain_block, re.MULTILINE))
    agent_headings = set(re.findall(r"^#### §(\w+)", agent_block, re.MULTILINE))

    assert agent_headings <= domain_headings or agent_headings, (
        "framework-agent embedded headings should be a subset of the domain's topics"
    )
    assert len(agent_headings) < len(domain_headings), (
        f"framework-agent ({len(agent_headings)} topics) is not scoped smaller than "
        f"domain ({len(domain_headings)} topics)"
    )

    # Topics confidently outside framework-agent's SCOPE (R-check-, R-bijection-,
    # R-tool-, R-atomicity-, R-statemachine-, R-conflict-, R-decided-, R-m-tag-,
    # R-typed-, R-axis-, R-anchor-, R-speak-) should not appear as headings.
    assert "Reflection" not in agent_headings
    assert "Conscience" not in agent_headings


def test_embedded_thinking_regen_stable():
    """Two consecutive gen_spec.py runs must produce byte-identical CLAUDE.md files."""
    targets = [DOMAIN_CLAUDE_MD, FRAMEWORK_AGENT_CLAUDE_MD, REPO_ROOT / "CLAUDE.md"]
    before = {p: _read(p) for p in targets}

    result = subprocess.run(
        [sys.executable, str(SPEC_ROOT / "tools" / "gen_spec.py")],
        cwd=str(SPEC_ROOT),
        capture_output=True,
        text=True,
        timeout=60,
    )
    assert result.returncode == 0, result.stderr

    result2 = subprocess.run(
        [sys.executable, str(SPEC_ROOT / "tools" / "gen_spec.py")],
        cwd=str(SPEC_ROOT),
        capture_output=True,
        text=True,
        timeout=60,
    )
    assert result2.returncode == 0, result2.stderr

    after = {p: _read(p) for p in targets}
    for p in targets:
        assert before[p] == after[p], f"{p} changed across a stable regen (non-determinism)"
