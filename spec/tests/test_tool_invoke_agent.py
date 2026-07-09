"""Tests for spec/tools/invoke_agent.py (R-tool-invoke-agent).

Verifies the CLI surface: refuse on unknown agent, refuse on missing CLAUDE.md,
print CLAUDE.md on success, --show-scope prints the SCOPE tuple, and
--show-tools lists shared + private tools.
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

import pytest

SPEC_ROOT = Path(__file__).resolve().parents[1]
TOOLS_DIR = SPEC_ROOT / "tools"
INVOKE_AGENT = TOOLS_DIR / "invoke_agent.py"

# Make the tool importable for monkeypatching.


def _run(*args: str, extra_env: dict | None = None) -> subprocess.CompletedProcess:
    """Run invoke_agent.py with given args; capture stdout+stderr."""
    import os

    env = os.environ.copy()
    if extra_env:
        env.update(extra_env)
    return subprocess.run(
        [sys.executable, str(INVOKE_AGENT), *args],
        capture_output=True,
        text=True,
        cwd=str(SPEC_ROOT),
        env=env,
    )


# ---------------------------------------------------------------------------
# Monkeypatching helpers
# ---------------------------------------------------------------------------


def _make_agent(tmp_path: Path, name: str, *, with_claude_md: bool = True) -> Path:
    """Create a minimal agent directory under tmp_path/agents/<name>/."""
    agent_dir = tmp_path / "agents" / name
    agent_dir.mkdir(parents=True)
    if with_claude_md:
        (agent_dir / "CLAUDE.md").write_text(
            f"# Agent {name}\nSENTINEL-{name}\n", encoding="utf-8"
        )
    return agent_dir


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_refuses_unknown_agent(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """Exit 1 + 'Unknown agent' in stderr when name not found."""
    import invoke_agent

    agents_root = tmp_path / "agents"
    agents_root.mkdir()
    monkeypatch.setattr(invoke_agent, "_AGENTS_ROOT", agents_root)

    rc = invoke_agent.main(["nonexistent-agent"])
    assert rc == 1


def test_refuses_unknown_agent_message(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture
) -> None:
    """stderr contains 'Unknown agent' when agent dir does not exist."""
    import invoke_agent

    agents_root = tmp_path / "agents"
    agents_root.mkdir()
    monkeypatch.setattr(invoke_agent, "_AGENTS_ROOT", agents_root)

    invoke_agent.main(["nonexistent-agent"])
    captured = capsys.readouterr()
    assert "Unknown agent" in captured.err


def test_refuses_missing_claude_md(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Exit 1 when agent dir exists but has no CLAUDE.md."""
    import invoke_agent

    agents_root = tmp_path / "agents"
    _make_agent(tmp_path, "alpha", with_claude_md=False)
    monkeypatch.setattr(invoke_agent, "_AGENTS_ROOT", agents_root)

    rc = invoke_agent.main(["alpha"])
    assert rc == 1


def test_prints_claude_md(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture
) -> None:
    """stdout contains the CLAUDE.md sentinel when agent resolves successfully."""
    import invoke_agent

    agents_root = tmp_path / "agents"
    _make_agent(tmp_path, "beta")
    monkeypatch.setattr(invoke_agent, "_AGENTS_ROOT", agents_root)

    rc = invoke_agent.main(["beta"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "SENTINEL-beta" in out


def test_show_scope_prints_tuple(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture
) -> None:
    """--show-scope includes items from the SCOPE tuple in scope.py."""
    import invoke_agent

    agents_root = tmp_path / "agents"
    agent_dir = _make_agent(tmp_path, "gamma")
    (agent_dir / "scope.py").write_text(
        'SCOPE = ("R-foo-", "R-bar-")\n', encoding="utf-8"
    )
    monkeypatch.setattr(invoke_agent, "_AGENTS_ROOT", agents_root)

    rc = invoke_agent.main(["gamma", "--show-scope"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "R-foo-" in out
    assert "R-bar-" in out


def test_show_tools_lists_shared_and_private(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture
) -> None:
    """--show-tools prints shared tools and the agent's private tools."""
    import invoke_agent

    agents_root = tmp_path / "agents"
    agent_dir = _make_agent(tmp_path, "delta")

    # Create a private tool.
    private_tools = agent_dir / "tools"
    private_tools.mkdir()
    (private_tools / "my_private_tool.py").write_text("# private\n", encoding="utf-8")

    monkeypatch.setattr(invoke_agent, "_AGENTS_ROOT", agents_root)
    # Keep _SPEC_ROOT pointing at real spec so shared tools are found.

    rc = invoke_agent.main(["delta", "--show-tools"])
    assert rc == 0
    out = capsys.readouterr().out

    # At least one shared tool from spec/tools/ must appear.
    shared = invoke_agent._shared_tools(invoke_agent._SPEC_ROOT)
    assert shared, "Expected at least some shared tools in spec/tools/"
    assert shared[0].name in out

    # Private tool must appear.
    assert "my_private_tool.py" in out
