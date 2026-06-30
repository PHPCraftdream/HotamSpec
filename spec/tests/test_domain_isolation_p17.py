"""Tests for P17 task #64 — domain isolation, shared docs, filesystem invariants.

Covers:
- check_domain_manifest_valid (empty-domains + with-domain cases)
- check_domain_director_exists (empty-domains + with-domain cases)
- check_agent_has_agents_subdir (real spec/agents/framework-agent/)
- check_agent_has_docs_subdir (real spec/agents/framework-agent/)
- build_shared_thinking_docs generates >= 1 §Topic file
- build_shared_tool_docs generates >= 1 tool file
- SHARED-DOCS block present in spec/agents/framework-agent/CLAUDE.md after regen
"""

from __future__ import annotations

import sys
from pathlib import Path


# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent
SPEC_DOCS_THINKING = SPEC_ROOT / "docs" / "thinking"
SPEC_DOCS_TOOLS = SPEC_ROOT / "docs" / "tools"

# Ensure gen_spec is importable.
_tools_str = str(SPEC_ROOT / "tools")
if _tools_str not in sys.path:
    sys.path.insert(0, _tools_str)
import gen_spec  # noqa: E402

# After P17 migration, agents live inside the active domain; resolve from gen_spec.
AGENTS_ROOT = gen_spec._AGENTS_ROOT
FRAMEWORK_AGENT_DIR = AGENTS_ROOT / "framework-agent"


# ---------------------------------------------------------------------------
# Filesystem invariants — empty domains
# ---------------------------------------------------------------------------


def test_check_domain_manifest_valid_empty_domains():
    """check_domain_manifest_valid returns no violations when domains/ is absent or empty."""
    from tensio.invariants import check_domain_manifest_valid
    from tensio.graph import TensionGraph

    g = TensionGraph()
    viols = check_domain_manifest_valid(g)
    assert viols == [], f"empty/absent domains must produce no violations, got {viols}"


def test_check_domain_director_exists_empty_domains():
    """check_domain_director_exists returns no violations when domains/ is absent or empty."""
    from tensio.invariants import check_domain_director_exists
    from tensio.graph import TensionGraph

    g = TensionGraph()
    viols = check_domain_director_exists(g)
    assert viols == [], f"empty/absent domains must produce no violations, got {viols}"


def test_check_domain_manifest_valid_with_domain(tmp_path):
    """check_domain_manifest_valid catches missing/invalid manifest.py in a tmp domain."""
    from tensio.invariants import check_domain_manifest_valid
    from tensio.graph import TensionGraph
    import tensio.invariants as inv_mod

    # Temporarily override _DOMAINS_ROOT to a tmp dir.
    original = inv_mod._DOMAINS_ROOT
    inv_mod._DOMAINS_ROOT = tmp_path

    try:
        g = TensionGraph()

        # Case 1: domain dir without manifest.py
        bad_domain = tmp_path / "no-manifest-domain"
        bad_domain.mkdir()
        viols = check_domain_manifest_valid(g)
        assert any(v.target == "no-manifest-domain" for v in viols), (
            "missing manifest must produce a violation"
        )

        # Case 2: domain with valid manifest.
        good_domain = tmp_path / "my-domain"
        good_domain.mkdir()
        (good_domain / "manifest.py").write_text(
            'ID = "my-domain"\nDESCRIPTION = "Test"\nGOALS = ("deliver",)\nDIRECTOR = "director"\n',
            encoding="utf-8",
        )
        viols2 = check_domain_manifest_valid(g)
        assert not any(v.target == "my-domain" for v in viols2), (
            "valid manifest must produce no violation"
        )
    finally:
        inv_mod._DOMAINS_ROOT = original


def test_check_domain_director_exists_with_domain(tmp_path):
    """check_domain_director_exists catches missing director agent scope.py."""
    from tensio.invariants import check_domain_director_exists
    from tensio.graph import TensionGraph
    import tensio.invariants as inv_mod

    original = inv_mod._DOMAINS_ROOT
    inv_mod._DOMAINS_ROOT = tmp_path

    try:
        g = TensionGraph()

        domain = tmp_path / "test-domain"
        domain.mkdir()
        (domain / "manifest.py").write_text(
            'ID = "test-domain"\nDESCRIPTION = "Test"\nGOALS = ("x",)\nDIRECTOR = "director"\n',
            encoding="utf-8",
        )

        # Director agent missing.
        viols = check_domain_director_exists(g)
        assert any(v.target == "test-domain" for v in viols), (
            "missing director scope.py must produce a violation"
        )

        # Now create the director agent.
        agent_dir = domain / "agents" / "director"
        agent_dir.mkdir(parents=True)
        (agent_dir / "scope.py").write_text(
            'PURPOSE = "dir"\nSCOPE = ()\n', encoding="utf-8"
        )
        viols2 = check_domain_director_exists(g)
        assert not any(v.target == "test-domain" for v in viols2), (
            "existing director scope.py must produce no violation"
        )
    finally:
        inv_mod._DOMAINS_ROOT = original


# ---------------------------------------------------------------------------
# Filesystem invariants — real framework-agent
# ---------------------------------------------------------------------------


def test_agent_has_agents_subdir():
    """spec/agents/framework-agent/agents/ must exist (R-agent-is-recursive-director)."""
    agents_subdir = FRAMEWORK_AGENT_DIR / "agents"
    assert agents_subdir.exists(), (
        "spec/agents/framework-agent/agents/ must exist; got absent. "
        "Create it or run the create_agent scaffold."
    )
    assert agents_subdir.is_dir(), "agents/ must be a directory"


def test_agent_has_docs_subdir():
    """spec/agents/framework-agent/docs/ must exist (R-agent-has-docs-dir)."""
    docs_subdir = FRAMEWORK_AGENT_DIR / "docs"
    assert docs_subdir.exists(), (
        "spec/agents/framework-agent/docs/ must exist; got absent. "
        "Create it or run the create_agent scaffold."
    )
    assert docs_subdir.is_dir(), "docs/ must be a directory"


def test_check_agent_has_agents_subdir_passes_on_real_agents():
    """check_agent_has_agents_subdir returns no violations for the real agents root."""
    from tensio.invariants import check_agent_has_agents_subdir
    from tensio.graph import TensionGraph

    g = TensionGraph()
    viols = check_agent_has_agents_subdir(g)
    assert viols == [], f"real agents must have agents/ subdir, violations: {viols}"


def test_check_agent_has_docs_subdir_passes_on_real_agents():
    """check_agent_has_docs_subdir returns no violations for the real agents root."""
    from tensio.invariants import check_agent_has_docs_subdir
    from tensio.graph import TensionGraph

    g = TensionGraph()
    viols = check_agent_has_docs_subdir(g)
    assert viols == [], f"real agents must have docs/ subdir, violations: {viols}"


# ---------------------------------------------------------------------------
# Shared thinking docs
# ---------------------------------------------------------------------------


def test_shared_thinking_docs_generated():
    """spec/docs/thinking/ must contain at least one §Topic file after regen."""
    assert SPEC_DOCS_THINKING.exists(), (
        "spec/docs/thinking/ does not exist — run `uv run python tools/gen_spec.py`"
    )
    md_files = list(SPEC_DOCS_THINKING.glob("*.md"))
    assert len(md_files) >= 1, (
        f"spec/docs/thinking/ must have at least one .md file; found {md_files}"
    )


def test_shared_thinking_docs_content():
    """build_shared_thinking_docs produces at least one topic from the real src."""
    docs = gen_spec.build_shared_thinking_docs()
    assert len(docs) >= 1, "Must find at least one §Topic in framework docstrings"
    # Every generated doc must reference Canon:
    for slug, content in docs.items():
        assert "Canon" in content, f"thinking doc {slug}.md must mention Canon:"


# ---------------------------------------------------------------------------
# Shared tool docs
# ---------------------------------------------------------------------------


def test_shared_tool_docs_generated():
    """spec/docs/tools/ must contain at least one tool doc file after regen."""
    assert SPEC_DOCS_TOOLS.exists(), (
        "spec/docs/tools/ does not exist — run `uv run python tools/gen_spec.py`"
    )
    md_files = list(SPEC_DOCS_TOOLS.glob("*.md"))
    assert len(md_files) >= 1, (
        f"spec/docs/tools/ must have at least one .md file; found {md_files}"
    )


def test_shared_tool_docs_content():
    """build_shared_tool_docs produces at least one entry from the real tools dir."""
    docs = gen_spec.build_shared_tool_docs()
    assert len(docs) >= 1, "Must find at least one Canon:-marked tool"
    for basename, content in docs.items():
        assert "# Tool —" in content, f"tool doc {basename}.md must have H1 header"
        assert "## Synopsis" in content, (
            f"tool doc {basename}.md must have Synopsis section"
        )


# ---------------------------------------------------------------------------
# SHARED-DOCS block in agent CLAUDE.md
# ---------------------------------------------------------------------------


def test_agent_shared_docs_block_present():
    """spec/agents/framework-agent/CLAUDE.md must have SHARED-DOCS sentinel block."""
    claude_md = FRAMEWORK_AGENT_DIR / "CLAUDE.md"
    assert claude_md.exists(), "framework-agent/CLAUDE.md must exist"
    text = claude_md.read_text(encoding="utf-8")
    assert "<!-- SHARED-DOCS:BEGIN -->" in text, (
        "framework-agent/CLAUDE.md must have SHARED-DOCS:BEGIN sentinel after regen. "
        "Run `uv run python tools/gen_spec.py`."
    )
    assert "<!-- SHARED-DOCS:END -->" in text, (
        "framework-agent/CLAUDE.md must have SHARED-DOCS:END sentinel after regen."
    )
