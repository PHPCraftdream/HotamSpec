"""Tests enforcing R-agent-declares-purpose.

Every spec/agents/<name>/scope.py must define a non-empty module-level
constant PURPOSE describing what the agent stewards (one line).

After P22.C consolidation, domains/hotam-spec-self/agents/ (director +
framework-agent) was deleted — it was a dormant P21 dogfood scaffold, never
actually spawned. No agent directories currently exist anywhere in the repo,
so the "every agent declares purpose" check is vacuously satisfied (skips
cleanly) until a real second agent is scaffolded via create_agent.py.
"""

from __future__ import annotations

import importlib.util
import subprocess
import sys
from pathlib import Path

import pytest

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_TOOLS = _SPEC_ROOT / "tools"

import sys as _sys  # noqa: E402

if str(_TOOLS) not in _sys.path:
    _sys.path.insert(0, str(_TOOLS))
import gen_spec as _gen_spec  # noqa: E402

_AGENTS_ROOT = _gen_spec._AGENTS_ROOT


def _load_scope(agent_dir: Path):
    """Import scope.py from an agent directory and return the module."""
    scope_path = agent_dir / "scope.py"
    spec = importlib.util.spec_from_file_location(
        f"_agent_scope_{agent_dir.name}", scope_path
    )
    assert spec is not None
    mod = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(mod)  # type: ignore[union-attr]
    return mod


def test_every_agent_declares_purpose() -> None:
    """Every agent scope.py (if any exist) defines a non-empty PURPOSE string.

    Post-P22.C: no agents currently exist (domains/hotam-spec-self/agents/ was
    deleted as an unused scaffold) — vacuously satisfied, skip cleanly.
    """
    if not _AGENTS_ROOT.exists():
        pytest.skip("No agents root exists — no agents scaffolded yet (P22.C)")
    scope_files = list(_AGENTS_ROOT.glob("*/scope.py"))
    if not scope_files:
        pytest.skip("No agent scope.py files found — no agents scaffolded yet")

    for scope_path in scope_files:
        agent_name = scope_path.parent.name
        mod = _load_scope(scope_path.parent)
        assert hasattr(mod, "PURPOSE"), (
            f"Agent '{agent_name}': scope.py missing PURPOSE constant "
            "(R-agent-declares-purpose)"
        )
        purpose = getattr(mod, "PURPOSE")
        assert isinstance(purpose, str), (
            f"Agent '{agent_name}': PURPOSE must be a string, got {type(purpose)}"
        )
        assert purpose.strip(), (
            f"Agent '{agent_name}': PURPOSE must be non-empty "
            "(R-agent-declares-purpose)"
        )


def test_create_agent_refuses_missing_purpose() -> None:
    """create_agent.py exits 1 with a clear message when --purpose is omitted."""
    result = subprocess.run(
        [sys.executable, str(_TOOLS / "create_agent.py"), "test-no-purpose"],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 1, (
        f"Expected exit 1 without --purpose, got {result.returncode}"
    )
    assert "R-agent-declares-purpose" in result.stderr, (
        f"Expected R-agent-declares-purpose in stderr, got: {result.stderr!r}"
    )
