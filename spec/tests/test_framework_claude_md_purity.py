"""Locking tests for root CLAUDE.md as the SOLE operator-prompt (P22.C consolidation).

After P22.C, there is exactly ONE CLAUDE.md file in the whole repo (repo root).
It contains everything: LIVE-STATE, DOMAIN-MAP, REPO-MAP, CONSTITUTION,
AGENT-MAP, CONCEPT-MAP, THINKING-INDEX, EMBEDDED-THINKING, EMBEDDED-TOOLS,
RECENTLY-REJECTED. The domains/hotam-spec-self/CLAUDE.md file and the
domains/hotam-spec-self/agents/ scaffold tree (director + framework-agent)
have been deleted — see task #101. Root CLAUDE.md is now generated directly
from CLAUDE.md.template.txt via <!-- mind --> / <!-- business --> placeholder
substitution (R-claude-md-template-driven, task #103).

Canon: §Domain — R-domain-map-generated, R-crystal-is-claude-md,
       R-claude-md-template-driven.
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
    """CLAUDE.md consolidation is CONDITIONAL on domain count, not absolute.

    RULE + WHY (R-claude-md-consolidates-when-single-agent, see
    domains/hotam-spec-self/docs/gen/REQUIREMENTS.md for the exact claim
    text): the requirement itself is conditional -- "one domain, zero active
    sub-agents -> exactly ONE CLAUDE.md" -- not an unconditional repo-wide
    ban on more than one CLAUDE.md file. That condition held when this test
    was first written (single domain, hotam-spec-self, P22.C consolidation)
    but stopped holding the moment a second domain (hotam-dev) was seated:
    RECENTLY-REJECTED already records two prior attempts to make this claim
    unconditional (R-domain-owns-claude-md, R-framework-claude-md-is-domain-
    free) -- both rejected as replaced by the consolidates-when-single-agent
    condition. Re-asserting an absolute one-CLAUDE.md-in-the-repo rule here
    would just be a third attempt at the same rejected claim wearing a test
    instead of a requirement.

    With exactly one domain: the old strict rule holds unchanged -- exactly
    one CLAUDE.md, at repo root.

    With >= 2 domains: legitimate crystals are root CLAUDE.md (the active
    domain, R-crystal-is-claude-md) plus, per domain, domains/<name>/CLAUDE.md
    and domains/<name>/agents/director/CLAUDE.md (the create_domain.py /
    create_agent.py scaffold outputs -- see spec/tools/create_domain.py and
    spec/tools/create_agent.py). Anything else (in particular a CLAUDE.md
    anywhere under spec/) remains forbidden -- spec/ is the content-free
    framework body and must never carry a domain-specific crystal
    (R-content-free-no-business-data).
    """
    found = [
        p
        for p in REPO_ROOT.rglob("CLAUDE.md")
        if ".venv" not in p.parts and "node_modules" not in p.parts
    ]

    domains_root = REPO_ROOT / "domains"
    domain_dirs = sorted(
        d for d in domains_root.iterdir() if d.is_dir() and not d.name.startswith("_")
    ) if domains_root.exists() else []

    # Forbidden everywhere, regardless of domain count: spec/ is content-free.
    spec_root = REPO_ROOT / "spec"
    stray = [p for p in found if spec_root in p.parents]
    assert stray == [], (
        f"CLAUDE.md must never live under spec/ (content-free framework body): {stray}"
    )

    if len(domain_dirs) <= 1:
        assert found == [ROOT_CLAUDE_MD], (
            f"Single-domain state: expected exactly one CLAUDE.md at "
            f"{ROOT_CLAUDE_MD}, found: {found}"
        )
        return

    allowed = {ROOT_CLAUDE_MD}
    for d in domain_dirs:
        allowed.add(d / "CLAUDE.md")
        allowed.add(d / "agents" / "director" / "CLAUDE.md")

    unexpected = [p for p in found if p not in allowed]
    assert unexpected == [], (
        f"Multi-domain state ({[d.name for d in domain_dirs]}): found CLAUDE.md "
        f"files outside the legitimate set (root + per-domain + per-domain "
        f"director): {unexpected}"
    )
    assert ROOT_CLAUDE_MD in found, (
        f"Root CLAUDE.md ({ROOT_CLAUDE_MD}) must always be present as the "
        f"active-domain crystal."
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


def test_domain_map_shows_pulse_per_domain() -> None:
    """R-domain-map-shows-pulse: every domain in DOMAIN-MAP carries an
    'open actions' line, so each domain's open contradictions are visible from
    the root crystal (the two-eyed pulse)."""
    text = _read(ROOT_CLAUDE_MD)
    block = _extract_block(
        text, "<!-- DOMAIN-MAP:BEGIN -->", "<!-- DOMAIN-MAP:END -->"
    )
    assert block is not None, "DOMAIN-MAP block not found in root CLAUDE.md"
    # Count domain headers (### <name>) and 'open actions' lines; every domain
    # header must be followed by an open-actions line.
    # Match real domain headers only: '### <slug>' on its own line. The block
    # opens with a '### Domain Map' title — exclude any header with a space.
    domain_headers = re.findall(r"^### (\S+)\s*$", block, re.MULTILINE)
    open_lines = re.findall(r"\*\*open actions\*\*\s*—", block)
    assert domain_headers, "DOMAIN-MAP lists no domains"
    assert len(open_lines) == len(domain_headers), (
        f"DOMAIN-MAP has {len(domain_headers)} domains but "
        f"{len(open_lines)} 'open actions' lines — every domain must show its "
        "pulse (R-domain-map-shows-pulse)."
    )


def test_emit_cipher_aggregates_other_domain_open_actions() -> None:
    """R-domain-map-shows-pulse: emit_cipher's second eye — when a non-pinned
    domain has open actions, the cipher payload names 'other domains: N open'."""
    import emit_cipher  # noqa: PLC0415

    text = _read(ROOT_CLAUDE_MD)
    n = emit_cipher._other_domains_open(text)
    # n reflects real state; the CONTRACT is that when n>0 the aggregate is a
    # correct sum and >0 domains are non-pinned. We assert the parser is sound:
    # it never counts the pinned domain and returns a non-negative int.
    assert isinstance(n, int) and n >= 0
    pinned = emit_cipher._pinned_domain()
    block = _extract_block(
        text, "<!-- DOMAIN-MAP:BEGIN -->", "<!-- DOMAIN-MAP:END -->"
    )
    assert block is not None
    # Recompute the pinned domain's own open count and confirm it is EXCLUDED.
    pinned_open = 0
    cur = ""
    for line in block.splitlines():
        h = re.match(r"^### (\S+)", line.strip())
        if h:
            cur = h.group(1)
            continue
        m = re.search(r"\*\*open actions\*\*\s*—\s*(\d+)", line)
        if m and cur == pinned:
            pinned_open = int(m.group(1))
    # Sum ALL domains, subtract pinned, must equal emit_cipher's answer.
    all_open = sum(
        int(m) for m in re.findall(r"\*\*open actions\*\*\s*—\s*(\d+)", block)
    )
    assert n == all_open - pinned_open


def test_repo_map_md_references_spec_files() -> None:
    """REPO-MAP.md (relocated from crystal) must reference spec/ files."""
    import gen_spec as _gs  # noqa: PLC0415
    repo_map_md = _gs.REPO_MAP_MD
    assert repo_map_md.exists(), f"REPO-MAP.md not found at {repo_map_md}"
    text = repo_map_md.read_text(encoding="utf-8")
    assert "spec/src/hotam_spec/" in text, "REPO-MAP.md missing framework body section"
    assert "spec/tools/" in text, "REPO-MAP.md missing tools section"


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
