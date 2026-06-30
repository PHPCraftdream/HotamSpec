"""Tests for the AGENT-MAP sentinel block in CLAUDE.md.

Verifies that:
1. Both AGENT-MAP sentinels are present in CLAUDE.md.
2. Every spec/agents/<name>/ directory has a corresponding heading in the block.
3. The framework-agent section has non-empty purpose, scope, atoms, tools, crystal.
4. Regenerating gen_spec twice produces byte-identical CLAUDE.md.
5. An empty agents root yields the '_(no sub-operators yet)_' placeholder.
"""

from __future__ import annotations

import importlib.util
import subprocess
import sys
import tempfile
from pathlib import Path

import pytest

# ---------------------------------------------------------------------------
# Paths
# ---------------------------------------------------------------------------

import sys as _sys_am

SPEC_ROOT = Path(__file__).resolve().parents[1]  # .../spec
REPO_ROOT = SPEC_ROOT.parent
CLAUDE_MD = REPO_ROOT / "CLAUDE.md"

_tools_am = str(SPEC_ROOT / "tools")
if _tools_am not in _sys_am.path:
    _sys_am.path.insert(0, _tools_am)
import gen_spec as _gen_spec_am  # noqa: E402

# After P17 migration, agents_root is inside the active domain.
AGENTS_ROOT = _gen_spec_am._AGENTS_ROOT

_AGENT_MAP_BEGIN = "<!-- AGENT-MAP:BEGIN -->"
_AGENT_MAP_END = "<!-- AGENT-MAP:END -->"


def _agent_map_block() -> str:
    """Extract the AGENT-MAP block content from CLAUDE.md."""
    text = CLAUDE_MD.read_text(encoding="utf-8")
    begin = text.find(_AGENT_MAP_BEGIN)
    end = text.find(_AGENT_MAP_END)
    assert begin != -1 and end != -1 and end > begin, "AGENT-MAP sentinels missing"
    return text[begin + len(_AGENT_MAP_BEGIN) : end]


# ---------------------------------------------------------------------------
# Test 1: sentinels present
# ---------------------------------------------------------------------------


def test_agent_map_sentinels_present() -> None:
    """Both AGENT-MAP sentinels shall exist in CLAUDE.md."""
    text = CLAUDE_MD.read_text(encoding="utf-8")
    assert _AGENT_MAP_BEGIN in text, f"Missing {_AGENT_MAP_BEGIN!r} in CLAUDE.md"
    assert _AGENT_MAP_END in text, f"Missing {_AGENT_MAP_END!r} in CLAUDE.md"


# ---------------------------------------------------------------------------
# Test 2: every agent directory has a heading in the block
# ---------------------------------------------------------------------------


def test_agent_map_complete() -> None:
    """Every spec/agents/<name>/ with a scope.py shall appear in the AGENT-MAP block."""
    if not AGENTS_ROOT.exists():
        pytest.skip("No agents root found")

    agent_names = [
        d.name
        for d in sorted(AGENTS_ROOT.iterdir())
        if d.is_dir() and (d / "scope.py").exists()
    ]
    if not agent_names:
        pytest.skip("No agent directories with scope.py found")

    block = _agent_map_block()
    for name in agent_names:
        assert f"#### {name}" in block, (
            f"Agent '{name}' not found as '#### {name}' in AGENT-MAP block"
        )


# ---------------------------------------------------------------------------
# Test 3: framework-agent section has all required fields non-empty
# ---------------------------------------------------------------------------


def test_agent_map_framework_agent_present() -> None:
    """The framework-agent section shall appear with non-empty purpose, scope, atoms, tools, crystal."""
    block = _agent_map_block()
    assert "#### framework-agent" in block, (
        "framework-agent heading not found in AGENT-MAP block"
    )

    # Find the framework-agent sub-block (from its heading to the next #### or end).
    start = block.index("#### framework-agent")
    next_heading = block.find("#### ", start + 1)
    sub = block[start : next_heading if next_heading != -1 else len(block)]

    assert (
        "**purpose**" in sub and len(sub.split("**purpose** —", 1)[-1].strip()) > 0
    ), "framework-agent purpose is empty"
    assert "**scope**" in sub, "framework-agent scope line missing"
    # Scope must contain at least one prefix (backtick-quoted).
    assert "`R-" in sub, "framework-agent scope prefixes are empty"
    assert "**atoms**" in sub, "framework-agent atoms line missing"
    # Atoms count must be a digit.
    atoms_line = [ln for ln in sub.splitlines() if "**atoms**" in ln]
    assert atoms_line, "framework-agent atoms line not found"
    assert any(c.isdigit() for c in atoms_line[0]), (
        "framework-agent atoms count contains no digit"
    )
    assert "**tools**" in sub, "framework-agent tools line missing"
    assert "**crystal**" in sub, "framework-agent crystal line missing"
    # Crystal path is relative to repo root; after P17 migration it lives in the domain.
    fa_claude = AGENTS_ROOT / "framework-agent" / "CLAUDE.md"
    try:
        fa_crystal = str(fa_claude.relative_to(REPO_ROOT)).replace("\\", "/")
    except ValueError:
        fa_crystal = "spec/agents/framework-agent/CLAUDE.md"
    assert fa_crystal in sub, (
        f"framework-agent crystal path incorrect; expected '{fa_crystal}' in block"
    )


# ---------------------------------------------------------------------------
# Test 4: regen is byte-identical (idempotency)
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
    text1 = CLAUDE_MD.read_bytes()
    run_gen()
    text2 = CLAUDE_MD.read_bytes()
    assert text1 == text2, (
        "gen_spec.py is not idempotent: CLAUDE.md differs between two runs"
    )


# ---------------------------------------------------------------------------
# Test 5: empty agents root yields placeholder
# ---------------------------------------------------------------------------


def test_agent_map_empty_when_no_agents() -> None:
    """When agents_root is an empty directory, _scan_agent_map emits the placeholder."""
    # Import _scan_agent_map from gen_spec via importlib.
    # Register the module in sys.modules first so that dataclass string annotations
    # resolve correctly (avoids AttributeError on cls.__module__ lookup).
    gen_spec_path = SPEC_ROOT / "tools" / "gen_spec.py"
    spec = importlib.util.spec_from_file_location("gen_spec_isolated", gen_spec_path)
    assert spec is not None and spec.loader is not None
    mod = importlib.util.module_from_spec(spec)
    sys.modules["gen_spec_isolated"] = mod
    try:
        spec.loader.exec_module(mod)  # type: ignore[union-attr]
    finally:
        sys.modules.pop("gen_spec_isolated", None)

    # Load the real graph.
    sys.path.insert(0, str(SPEC_ROOT / "src"))
    from tensio.graph import load_content_graph  # noqa: PLC0415

    g = load_content_graph()

    with tempfile.TemporaryDirectory() as tmp:
        empty_root = Path(tmp)
        block = mod._scan_agent_map(g, agents_root=empty_root)
        assert "_(no sub-operators yet)_" in block, (
            f"Expected placeholder in block, got:\n{block}"
        )
