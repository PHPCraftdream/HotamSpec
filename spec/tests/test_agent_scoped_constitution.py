"""Tests for per-agent scoped CONSTITUTION generation (R-agent-scoped-constitution).

Canon: §Agent — gen_spec._regenerate_agent_constitutions rewrites each
spec/agents/<name>/CLAUDE.md CONSTITUTION block filtered by that agent's SCOPE.
"""

from __future__ import annotations

from pathlib import Path

import pytest

# Make gen_spec importable from spec/tools/.
_SPEC_ROOT = Path(__file__).resolve().parents[1]
_TOOLS_DIR = _SPEC_ROOT / "tools"

import gen_spec  # noqa: E402
from hotam_spec.graph import load_content_graph  # noqa: E402

_CONST_BEGIN = gen_spec._CONST_BEGIN
_CONST_END = gen_spec._CONST_END

# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


def _make_agent(
    agents_root: Path,
    name: str,
    scope_tuple: str,
    claude_md_extra: str = "",
) -> Path:
    """Scaffold a minimal fake agent directory."""
    agent_dir = agents_root / name
    agent_dir.mkdir(parents=True)

    scope_py = f'"""Canon: §Agent — declares the scope of agent \'{name}\'."""\n\nSCOPE = {scope_tuple}\n'
    (agent_dir / "scope.py").write_text(scope_py, encoding="utf-8")

    claude_md = (
        f"# {name}\n\n{claude_md_extra}## Scope\n\n{_CONST_BEGIN}\n{_CONST_END}\n"
    )
    (agent_dir / "CLAUDE.md").write_text(claude_md, encoding="utf-8")
    return agent_dir


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_agent_with_scope_R_check_gets_only_R_check_entries(tmp_path: Path) -> None:
    """An agent with SCOPE=('R-check-',) has only R-check-* atoms in its CONSTITUTION."""
    g = load_content_graph()
    _make_agent(tmp_path, "check-agent", '("R-check-",)')

    gen_spec._regenerate_agent_constitutions(g, agents_root=tmp_path)

    claude_md = (tmp_path / "check-agent" / "CLAUDE.md").read_text(encoding="utf-8")
    # Extract the CONSTITUTION block content between sentinels.
    begin_pos = claude_md.find(_CONST_BEGIN)
    end_pos = claude_md.find(_CONST_END)
    assert begin_pos != -1 and end_pos != -1, "Sentinels must be present after regen"
    block = claude_md[begin_pos + len(_CONST_BEGIN) : end_pos]

    # All R-* mentions in the block must start with R-check-.
    import re

    r_ids = re.findall(r"\bR-[a-z][a-zA-Z0-9-]+", block)
    non_check = [rid for rid in r_ids if not rid.startswith("R-check-")]
    assert non_check == [], f"Agent scoped to R-check- should not contain: {non_check}"


def test_agent_with_empty_scope_gets_empty_constitution(tmp_path: Path) -> None:
    """An agent with SCOPE=() gets a present-but-empty CONSTITUTION block."""
    g = load_content_graph()
    _make_agent(tmp_path, "empty-agent", "()")

    gen_spec._regenerate_agent_constitutions(g, agents_root=tmp_path)

    claude_md = (tmp_path / "empty-agent" / "CLAUDE.md").read_text(encoding="utf-8")
    begin_pos = claude_md.find(_CONST_BEGIN)
    end_pos = claude_md.find(_CONST_END)
    assert begin_pos != -1 and end_pos != -1
    block = claude_md[begin_pos + len(_CONST_BEGIN) : end_pos]
    assert "(no atoms in scope)" in block, (
        f"Empty scope block should contain placeholder. Got:\n{block}"
    )


def test_agent_with_R_tool_scope_gets_tool_derived_entries(tmp_path: Path) -> None:
    """An agent with SCOPE=('R-tool-',) gets tool-derived entries in its CONSTITUTION."""
    g = load_content_graph()
    _make_agent(tmp_path, "tool-agent", '("R-tool-",)')

    # Only run if there are tool requirements to project.
    tool_reqs = gen_spec._scan_tool_requirements()
    if not tool_reqs:
        pytest.skip("No tool-derived requirements found — nothing to assert against.")

    gen_spec._regenerate_agent_constitutions(g, agents_root=tmp_path)

    claude_md = (tmp_path / "tool-agent" / "CLAUDE.md").read_text(encoding="utf-8")
    begin_pos = claude_md.find(_CONST_BEGIN)
    end_pos = claude_md.find(_CONST_END)
    assert begin_pos != -1 and end_pos != -1
    block = claude_md[begin_pos + len(_CONST_BEGIN) : end_pos]

    # At least one R-tool- entry must appear.
    assert "R-tool-" in block, (
        f"Agent scoped to R-tool- should contain R-tool- entries. Got:\n{block}"
    )
    # Must NOT contain bare graph-requirement atoms (R-check-, R-agent-, etc.)
    import re

    r_ids = re.findall(r"\bR-[a-z][a-zA-Z0-9-]+", block)
    non_tool = [rid for rid in r_ids if not rid.startswith("R-tool-")]
    assert non_tool == [], (
        f"Agent scoped to R-tool- should not contain non-tool ids: {non_tool}"
    )


def test_regen_byte_identical(tmp_path: Path) -> None:
    """Running _regenerate_agent_constitutions twice produces identical CLAUDE.md (anti-drift)."""
    g = load_content_graph()
    _make_agent(tmp_path, "stable-agent", '("R-agent-",)')

    gen_spec._regenerate_agent_constitutions(g, agents_root=tmp_path)
    first = (tmp_path / "stable-agent" / "CLAUDE.md").read_bytes()

    gen_spec._regenerate_agent_constitutions(g, agents_root=tmp_path)
    second = (tmp_path / "stable-agent" / "CLAUDE.md").read_bytes()

    assert first == second, (
        "Two consecutive regenerations must produce byte-identical output (anti-drift)."
    )


def test_missing_sentinels_error(tmp_path: Path) -> None:
    """An agent CLAUDE.md without sentinels raises a clear RuntimeError."""
    g = load_content_graph()
    agent_dir = tmp_path / "broken-agent"
    agent_dir.mkdir()

    scope_py = '"""Canon: §Agent — scope."""\n\nSCOPE = ("R-agent-",)\n'
    (agent_dir / "scope.py").write_text(scope_py, encoding="utf-8")

    # CLAUDE.md intentionally lacks CONSTITUTION sentinels.
    (agent_dir / "CLAUDE.md").write_text(
        "# broken-agent\n\nNo sentinels here.\n", encoding="utf-8"
    )

    with pytest.raises(RuntimeError, match="CONSTITUTION sentinels"):
        gen_spec._regenerate_agent_constitutions(g, agents_root=tmp_path)


def test_no_agents_dir_is_noop(tmp_path: Path) -> None:
    """When agents_root does not exist, _regenerate_agent_constitutions is a no-op."""
    g = load_content_graph()
    missing_root = tmp_path / "nonexistent_agents"
    # Should not raise; simply returns without action.
    gen_spec._regenerate_agent_constitutions(g, agents_root=missing_root)
