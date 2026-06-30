"""Tests for spec/tools/create_agent.py — sub-agent directory scaffolder.

Uses tmp_path to isolate all file creation from the real spec/agents/ directory.
"""

from __future__ import annotations

import sys
from pathlib import Path


_TOOLS = Path(__file__).resolve().parents[1] / "tools"
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))

import create_agent  # noqa: E402


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _scaffold(
    tmp_path: Path, name: str, scope: str = "", purpose: str = "Test agent."
) -> int:
    """Invoke scaffold() with tmp_path as agents_root."""
    scope_prefixes = [p.strip() for p in scope.split(",") if p.strip()]
    return create_agent.scaffold(
        name=name,
        scope_prefixes=scope_prefixes,
        purpose=purpose,
        agents_root=tmp_path,
    )


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_creates_required_files(tmp_path: Path) -> None:
    """All four expected paths exist after a successful scaffold."""
    rc = _scaffold(tmp_path, "my-agent", purpose="Does something useful.")
    assert rc == 0

    agent_dir = tmp_path / "my-agent"
    assert (agent_dir / "CLAUDE.md").is_file()
    assert (agent_dir / "scope.py").is_file()
    assert (agent_dir / "tools" / "__init__.py").is_file()
    assert (agent_dir / "README.md").is_file()

    # Content fragments
    claude_text = (agent_dir / "CLAUDE.md").read_text(encoding="utf-8")
    assert "my-agent" in claude_text
    assert "Does something useful." in claude_text

    readme_text = (agent_dir / "README.md").read_text(encoding="utf-8")
    assert "my-agent" in readme_text
    assert "Does something useful." in readme_text

    scope_text = (agent_dir / "scope.py").read_text(encoding="utf-8")
    assert "SCOPE" in scope_text


def test_refuses_existing(tmp_path: Path) -> None:
    """Returns exit code 1 if the agent directory already exists."""
    (tmp_path / "existing-agent").mkdir()
    rc = _scaffold(tmp_path, "existing-agent", purpose="Some purpose.")
    assert rc == 1


def test_refuses_invalid_name(tmp_path: Path) -> None:
    """Returns exit code 1 for names that fail validation."""
    invalid_names = ["Foo", "with space", "with/slash", "123start", ""]
    for bad in invalid_names:
        rc = _scaffold(tmp_path, bad, purpose="Some purpose.")
        assert rc == 1, f"Expected rc=1 for name={bad!r}, got {rc}"


def test_refuses_missing_purpose(tmp_path: Path) -> None:
    """Returns exit code 1 when --purpose is not supplied via main()."""
    import subprocess
    import sys

    result = subprocess.run(
        [sys.executable, str(_TOOLS / "create_agent.py"), "no-purpose-agent"],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 1
    assert "R-agent-declares-purpose" in result.stderr


def test_purpose_written_to_scope_py(tmp_path: Path) -> None:
    """PURPOSE constant is written into scope.py."""
    rc = _scaffold(
        tmp_path, "purposeful-agent", purpose="Stewards something important."
    )
    assert rc == 0

    scope_text = (tmp_path / "purposeful-agent" / "scope.py").read_text(
        encoding="utf-8"
    )
    assert 'PURPOSE = "Stewards something important."' in scope_text


def test_scope_py_contains_passed_prefixes(tmp_path: Path) -> None:
    """scope.py contains every prefix passed via --scope as a tuple entry."""
    rc = _scaffold(tmp_path, "scoped-agent", scope="R-check-,R-bijection-")
    assert rc == 0

    scope_text = (tmp_path / "scoped-agent" / "scope.py").read_text(encoding="utf-8")
    assert '"R-check-"' in scope_text
    assert '"R-bijection-"' in scope_text


def test_claude_md_has_constitution_sentinels(tmp_path: Path) -> None:
    """CLAUDE.md placeholder contains both CONSTITUTION sentinels."""
    rc = _scaffold(tmp_path, "sentinel-agent")
    assert rc == 0

    claude_text = (tmp_path / "sentinel-agent" / "CLAUDE.md").read_text(
        encoding="utf-8"
    )
    assert "<!-- CONSTITUTION:BEGIN -->" in claude_text
    assert "<!-- CONSTITUTION:END -->" in claude_text


def test_creates_agents_subdir(tmp_path: Path) -> None:
    """scaffold() creates an agents/ subdirectory for recursive nesting (R-agent-is-recursive-director)."""
    rc = _scaffold(tmp_path, "recursive-agent", purpose="Has sub-agents.")
    assert rc == 0

    agent_dir = tmp_path / "recursive-agent"
    agents_subdir = agent_dir / "agents"
    assert agents_subdir.is_dir(), "agents/ subdir must exist for recursive nesting"
    assert (agents_subdir / "__init__.py").is_file(), "agents/__init__.py must exist"


def test_creates_docs_dir(tmp_path: Path) -> None:
    """scaffold() creates a docs/ subdirectory with .gitkeep (R-agent-has-docs-dir)."""
    rc = _scaffold(tmp_path, "docs-agent", purpose="Has a docs dir.")
    assert rc == 0

    agent_dir = tmp_path / "docs-agent"
    docs_dir = agent_dir / "docs"
    assert docs_dir.is_dir(), "docs/ subdir must exist (R-agent-has-docs-dir)"
    assert (docs_dir / ".gitkeep").is_file(), (
        "docs/.gitkeep must exist so git tracks the dir"
    )


def test_parent_flag_creates_nested_agent(tmp_path: Path) -> None:
    """--parent flag places the new agent under the given parent directory."""
    # Simulate a parent agent directory already having an agents/ subdir
    parent_agents = tmp_path / "parent-agent" / "agents"
    parent_agents.mkdir(parents=True)

    rc = create_agent.scaffold(
        name="child-agent",
        scope_prefixes=[],
        purpose="Nested child agent.",
        agents_root=parent_agents,
    )
    assert rc == 0

    child_dir = parent_agents / "child-agent"
    assert (child_dir / "CLAUDE.md").is_file()
    assert (child_dir / "scope.py").is_file()
    assert (child_dir / "agents").is_dir(), "child also gets recursive agents/ subdir"
