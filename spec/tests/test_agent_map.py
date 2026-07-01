"""Tests for the AGENT-MAP sentinel block in root CLAUDE.md (P22.C consolidation).

After P22.C, there is exactly ONE CLAUDE.md (repo root) and the
domains/hotam-spec-self/agents/ scaffold (director + framework-agent) has been
deleted — framework-agent was a dormant P21 dogfood demo, never actually
spawned as a concurrent operator. AGENT-MAP now lives in root CLAUDE.md and
renders the "no active sub-agents" placeholder since no agent directories
exist. create_agent.py / spawn_agent.py remain fully functional for when a
real second agent is scaffolded.

Verifies that:
1. Both AGENT-MAP sentinels are present in root CLAUDE.md.
2. The block renders the "no active sub-agents" placeholder (no agents exist).
3. Regenerating gen_spec twice produces byte-identical output.
4. An empty agents root yields the '_(no sub-operators yet)_' placeholder
   (direct unit test of _scan_agent_map).
"""

from __future__ import annotations

import importlib.util
import subprocess
import sys
import tempfile
from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]  # .../spec
REPO_ROOT = SPEC_ROOT.parent
ROOT_CLAUDE_MD = REPO_ROOT / "CLAUDE.md"

_tools_am = str(SPEC_ROOT / "tools")
if _tools_am not in sys.path:
    sys.path.insert(0, _tools_am)
import gen_spec as _gen_spec_am  # noqa: E402

# After P22.C, no agents exist anywhere in the repo; _AGENTS_ROOT resolves to
# an absent directory (SPEC_ROOT / "agents" fallback).
AGENTS_ROOT = _gen_spec_am._AGENTS_ROOT

_AGENT_MAP_BEGIN = "<!-- AGENT-MAP:BEGIN -->"
_AGENT_MAP_END = "<!-- AGENT-MAP:END -->"


def _agent_map_block() -> str:
    """Extract the AGENT-MAP block content from root CLAUDE.md."""
    text = ROOT_CLAUDE_MD.read_text(encoding="utf-8")
    begin = text.find(_AGENT_MAP_BEGIN)
    end = text.find(_AGENT_MAP_END)
    assert begin != -1 and end != -1 and end > begin, "AGENT-MAP sentinels missing"
    return text[begin + len(_AGENT_MAP_BEGIN) : end]


# ---------------------------------------------------------------------------
# Test 1: sentinels present in root CLAUDE.md
# ---------------------------------------------------------------------------


def test_agent_map_sentinels_present() -> None:
    """Both AGENT-MAP sentinels shall exist in root CLAUDE.md."""
    text = ROOT_CLAUDE_MD.read_text(encoding="utf-8")
    assert _AGENT_MAP_BEGIN in text, f"Missing {_AGENT_MAP_BEGIN!r} in {ROOT_CLAUDE_MD}"
    assert _AGENT_MAP_END in text, f"Missing {_AGENT_MAP_END!r} in {ROOT_CLAUDE_MD}"


def test_root_claude_md_has_exactly_one_agent_map_block() -> None:
    """Root CLAUDE.md must contain the AGENT-MAP sentinel pair exactly once.

    Post-R-claude-md-template-driven: root CLAUDE.md is generated directly
    from CLAUDE.md.template.txt via render_business_content(), which
    includes AGENT-MAP once. The guarantee that matters is "not
    duplicated" — there is exactly one CLAUDE.md file in the whole repo
    (P22.C consolidation, tasks #101/#102), so no nested second copy can
    reintroduce a duplicate block.
    """
    root_text = ROOT_CLAUDE_MD.read_text(encoding="utf-8")
    assert root_text.count(_AGENT_MAP_BEGIN) == 1, (
        "Root CLAUDE.md must contain exactly one AGENT-MAP:BEGIN sentinel — "
        "run gen_spec.py to fix"
    )


# ---------------------------------------------------------------------------
# Test 2: no active sub-agents -> placeholder rendered
# ---------------------------------------------------------------------------


def test_agent_map_shows_no_active_sub_agents() -> None:
    """With no agent scaffolds present, AGENT-MAP must show the calm placeholder."""
    block = _agent_map_block()
    assert "no sub-operators yet" in block, (
        f"Expected '_(no sub-operators yet)_' placeholder, got:\n{block}"
    )
    # Must not reference the deleted scaffold.
    assert "framework-agent" not in block, (
        "AGENT-MAP block still references deleted framework-agent scaffold"
    )


def test_agents_root_does_not_exist() -> None:
    """domains/hotam-spec-self/agents/ was deleted (P22.C) — _AGENTS_ROOT is absent."""
    assert not AGENTS_ROOT.exists(), (
        f"Expected _AGENTS_ROOT ({AGENTS_ROOT}) to be absent after P22.C consolidation "
        "— domains/hotam-spec-self/agents/ should have been deleted."
    )


# ---------------------------------------------------------------------------
# Test 3: regen is byte-identical (idempotency)
# ---------------------------------------------------------------------------


def test_agent_map_regen_stable() -> None:
    """Running gen_spec.py twice shall produce byte-identical CLAUDE.md."""
    gen_spec = SPEC_ROOT / "tools" / "gen_spec.py"

    def run_gen() -> None:
        result = subprocess.run(
            [sys.executable, str(gen_spec)],
            capture_output=True,
            text=True,
            cwd=str(SPEC_ROOT),
        )
        assert result.returncode == 0, f"gen_spec.py failed:\n{result.stderr}"

    run_gen()
    text1 = ROOT_CLAUDE_MD.read_bytes()
    run_gen()
    text2 = ROOT_CLAUDE_MD.read_bytes()
    assert text1 == text2, (
        "gen_spec.py is not idempotent: CLAUDE.md differs between two runs"
    )


# ---------------------------------------------------------------------------
# Test 4: empty agents root yields placeholder (direct unit test)
# ---------------------------------------------------------------------------


def test_agent_map_empty_when_no_agents() -> None:
    """When agents_root is an empty directory, _scan_agent_map emits the placeholder."""
    gen_spec_path = SPEC_ROOT / "tools" / "gen_spec.py"
    spec = importlib.util.spec_from_file_location("gen_spec_isolated", gen_spec_path)
    assert spec is not None and spec.loader is not None
    mod = importlib.util.module_from_spec(spec)
    sys.modules["gen_spec_isolated"] = mod
    try:
        spec.loader.exec_module(mod)  # type: ignore[union-attr]
    finally:
        sys.modules.pop("gen_spec_isolated", None)

    sys.path.insert(0, str(SPEC_ROOT / "src"))
    from hotam_spec.graph import load_content_graph  # noqa: PLC0415

    g = load_content_graph()

    with tempfile.TemporaryDirectory() as tmp:
        empty_root = Path(tmp)
        block = mod._scan_agent_map(g, agents_root=empty_root)
        assert "_(no sub-operators yet)_" in block, (
            f"Expected placeholder in block, got:\n{block}"
        )
