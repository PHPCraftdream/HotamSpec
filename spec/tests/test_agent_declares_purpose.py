"""Tests enforcing R-agent-declares-purpose.

Every spec/agents/<name>/scope.py must define a non-empty module-level
constant PURPOSE describing what the agent stewards (one line).
"""

from __future__ import annotations

import importlib.util
import subprocess
import sys
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_TOOLS = _SPEC_ROOT / "tools"

# After P17 migration, framework-agent lives under domains/hotam-spec-self/agents/director/agents/.
# Resolve from gen_spec so the path is always authoritative.
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
    """Every spec/agents/*/scope.py defines a non-empty PURPOSE string."""
    scope_files = list(_AGENTS_ROOT.glob("*/scope.py"))
    assert scope_files, (
        "No agent scope.py files found — at least framework-agent must exist."
    )

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


def test_framework_agent_has_purpose() -> None:
    """framework-agent/scope.py has a PURPOSE that starts with 'Stewards'."""
    fa_dir = _AGENTS_ROOT / "framework-agent"
    assert fa_dir.is_dir(), "spec/agents/framework-agent/ must exist."

    mod = _load_scope(fa_dir)
    assert hasattr(mod, "PURPOSE"), "framework-agent/scope.py missing PURPOSE."
    assert "Stewards" in mod.PURPOSE, (
        f"Expected 'Stewards' in framework-agent PURPOSE, got: {mod.PURPOSE!r}"
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
