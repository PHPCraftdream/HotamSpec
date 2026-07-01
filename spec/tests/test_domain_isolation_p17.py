"""Tests for P17 task #64 — domain isolation, shared docs, filesystem invariants.

Covers:
- check_domain_manifest_valid (empty-domains + with-domain cases)
- check_domain_director_exists (empty-domains + with-domain cases)
- build_shared_thinking_docs generates >= 1 §Topic file
- build_shared_tool_docs generates >= 1 tool file

After P22.C consolidation, domains/hotam-spec-self/agents/director/agents/ (the
former framework-agent scaffold) was deleted — it was a dormant P21 dogfood
demo, never actually spawned. Only a minimal director/scope.py identity marker
remains (to satisfy check_domain_director_exists / R-domain-declares-director);
there is no director CLAUDE.md, no nested agents/, no docs/. Tests that
asserted the deleted scaffold's filesystem shape have been removed — see
test_domain_crystal_embedded... (deleted) and task #101's report.
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

AGENTS_ROOT = gen_spec._AGENTS_ROOT


# ---------------------------------------------------------------------------
# Filesystem invariants — empty domains
# ---------------------------------------------------------------------------


def test_check_domain_manifest_valid_empty_domains():
    """check_domain_manifest_valid returns no violations when domains/ is absent or empty."""
    from hotam_spec.invariants import check_domain_manifest_valid
    from hotam_spec.graph import TensionGraph

    g = TensionGraph()
    viols = check_domain_manifest_valid(g)
    assert viols == [], f"empty/absent domains must produce no violations, got {viols}"


def test_check_domain_director_exists_empty_domains():
    """check_domain_director_exists returns no violations when domains/ is absent or empty."""
    from hotam_spec.invariants import check_domain_director_exists
    from hotam_spec.graph import TensionGraph

    g = TensionGraph()
    viols = check_domain_director_exists(g)
    assert viols == [], f"empty/absent domains must produce no violations, got {viols}"


def test_check_domain_manifest_valid_with_domain(tmp_path):
    """check_domain_manifest_valid catches missing/invalid manifest.py in a tmp domain."""
    from hotam_spec.invariants import check_domain_manifest_valid
    from hotam_spec.graph import TensionGraph
    import hotam_spec.invariants as inv_mod

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
    from hotam_spec.invariants import check_domain_director_exists
    from hotam_spec.graph import TensionGraph
    import hotam_spec.invariants as inv_mod

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
# Filesystem invariants — no agents currently scaffolded (P22.C)
# ---------------------------------------------------------------------------


def test_no_agents_root_check_agent_has_agents_subdir_passes_vacuously():
    """check_agent_has_agents_subdir returns no violations when no agents exist.

    Post-P22.C: _SPEC_AGENTS_ROOT resolves to an absent directory (no
    domains/hotam-spec-self/agents/director/agents/), so the check is
    vacuously satisfied.
    """
    from hotam_spec.invariants import check_agent_has_agents_subdir
    from hotam_spec.graph import TensionGraph

    g = TensionGraph()
    viols = check_agent_has_agents_subdir(g)
    assert viols == [], f"expected no violations with no agents scaffolded: {viols}"


def test_no_agents_root_check_agent_has_docs_subdir_passes_vacuously():
    """check_agent_has_docs_subdir returns no violations when no agents exist."""
    from hotam_spec.invariants import check_agent_has_docs_subdir
    from hotam_spec.graph import TensionGraph

    g = TensionGraph()
    viols = check_agent_has_docs_subdir(g)
    assert viols == [], f"expected no violations with no agents scaffolded: {viols}"


def test_domain_director_exists_check_passes_with_scope_only():
    """check_domain_director_exists is satisfied by a director/scope.py identity marker.

    P22.C: the director has no CLAUDE.md/agents/docs of its own (it operates
    directly through root CLAUDE.md); only scope.py is required to satisfy
    R-domain-declares-director.
    """
    from hotam_spec.invariants import check_domain_director_exists
    from hotam_spec.graph import TensionGraph

    g = TensionGraph()
    viols = check_domain_director_exists(g)
    assert viols == [], f"expected no violations, got: {viols}"


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
# SHARED-DOCS block — no agents exist post-P22.C (no-op)
# ---------------------------------------------------------------------------


def test_no_agent_claude_md_files_exist():
    """No agent CLAUDE.md files exist anywhere — AGENTS_ROOT itself is absent (P22.C)."""
    assert not AGENTS_ROOT.exists(), (
        f"Expected AGENTS_ROOT ({AGENTS_ROOT}) to be absent post-P22.C consolidation."
    )
