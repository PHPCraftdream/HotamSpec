"""Tests for R-agent-code-imports-framework (R-agent-imports-framework split).

Structural scan proving the mechanically checkable slice of the split claim:
agents/tools import hotam_spec.* as shared infrastructure, but hotam_spec.*
(and the shared spec/tools/*.py) never import FROM an agent's private
tools/ directory -- the dependency arrow points one way only, so no agent
can make itself load-bearing for the framework body it borrows.

Mirrors tests/test_backend_neutral_scope.py's AST-import-scan pattern
(R-core-imports-stdlib-or-hotam-spec-only): a static scan catches a reversed
dependency the moment it is written, no runtime environment needed.
"""

from __future__ import annotations

import ast
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_SRC = _SPEC_ROOT / "src"

_HOTAM_SPEC_SRC = _SRC / "hotam_spec"
_SHARED_TOOLS = _SPEC_ROOT / "tools"
_DOMAINS_ROOT = _SPEC_ROOT.parent / "domains"


def _agent_tools_dirs() -> list[Path]:
    """Every domain's agents/<name>/tools/ directory that currently exists."""
    out: list[Path] = []
    if not _DOMAINS_ROOT.exists():
        return out
    for domain_dir in sorted(d for d in _DOMAINS_ROOT.iterdir() if d.is_dir()):
        agents_dir = domain_dir / "agents"
        if not agents_dir.exists():
            continue
        for agent_dir in sorted(d for d in agents_dir.iterdir() if d.is_dir()):
            tools_dir = agent_dir / "tools"
            if tools_dir.exists():
                out.append(tools_dir)
    return out


def _imported_top_level_names(tree: ast.Module) -> set[str]:
    names: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for alias in node.names:
                names.add(alias.name.split(".")[0])
        elif isinstance(node, ast.ImportFrom):
            if node.level and node.level > 0:
                continue
            if node.module:
                names.add(node.module.split(".")[0])
    return names


def test_framework_body_never_imports_from_an_agent_tools_dir() -> None:
    """hotam_spec.* source files must never import a module that lives under
    any agent's private tools/ directory -- the framework body is owned by
    no single agent, so it cannot depend back on one agent's private code.
    """
    agent_tool_module_stems = set()
    for tools_dir in _agent_tools_dirs():
        for py_file in tools_dir.glob("*.py"):
            agent_tool_module_stems.add(py_file.stem)

    if not agent_tool_module_stems:
        # No agent has private tools yet (only the director stub exists,
        # with no tools/ dir) -- the invariant is vacuously satisfied.
        return

    offenders: list[str] = []
    for path in sorted(_HOTAM_SPEC_SRC.glob("*.py")):
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
        imported = _imported_top_level_names(tree)
        bad = imported & agent_tool_module_stems
        if bad:
            offenders.append(f"{path.name} imports agent-private module(s): {sorted(bad)}")

    assert not offenders, (
        "hotam_spec.* must not import from any agent's private tools/ dir "
        f"(dependency direction reversed): {offenders}"
    )


def test_shared_tools_never_import_from_an_agent_tools_dir() -> None:
    """spec/tools/*.py (shared tools) must never import a module that lives
    under any agent's private tools/ directory -- private stays private.
    """
    agent_tool_module_stems = set()
    for tools_dir in _agent_tools_dirs():
        for py_file in tools_dir.glob("*.py"):
            agent_tool_module_stems.add(py_file.stem)

    if not agent_tool_module_stems:
        return

    offenders: list[str] = []
    for path in sorted(_SHARED_TOOLS.glob("*.py")):
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
        imported = _imported_top_level_names(tree)
        bad = imported & agent_tool_module_stems
        if bad:
            offenders.append(f"{path.name} imports agent-private module(s): {sorted(bad)}")

    assert not offenders, (
        "spec/tools/*.py must not import from any agent's private tools/ dir: "
        f"{offenders}"
    )
