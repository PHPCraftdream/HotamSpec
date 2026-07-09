"""Canon: §Invariants — R-core-imports-stdlib-or-hotam-spec-only slice of R-backend-scope.

Structural scan proving the ONE mechanically checkable slice of R-backend-scope
("the core stays backend-neutral by construction"): every top-level import in
spec/src/hotam_spec/*.py resolves to either the Python standard library or the
hotam_spec package itself -- no third-party backend/runtime dependency (no
Anthropic SDK, no CI-runner client, no alternate-agent client library) has
crept into the core. The full R-backend-scope claim ("names no target
backends... adapting the skin is the adopting agent's own concern") is a
broader PROSE design stance not fully machine-checkable; this import scan is
the narrow, mechanical slice of it (mirrors R-commit-boundary-checkable's
slice-of-a-broader-claim pattern).

WHY an AST import scan (not a runtime scan): a static scan catches a dependency
the moment it is written, before any test exercises the import path, and needs
no environment with the offending package installed to fail loudly.
"""

from __future__ import annotations

import ast
import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"

_TENSIO_SRC = _SRC / "hotam_spec"

_STDLIB = set(sys.stdlib_module_names)
_ALLOWED_TOP_LEVEL = _STDLIB | {"hotam_spec", "__future__"}


def _imported_top_level_names(tree: ast.Module) -> set[str]:
    names: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for alias in node.names:
                names.add(alias.name.split(".")[0])
        elif isinstance(node, ast.ImportFrom):
            if node.level and node.level > 0:
                continue  # relative import within the package -- always fine
            if node.module:
                names.add(node.module.split(".")[0])
    return names


def test_hotam_spec_core_imports_stdlib_or_self_only() -> None:
    """AST-scan every spec/src/hotam_spec/*.py module: every absolute import's
    top-level module name must be in the Python stdlib or be 'hotam_spec'
    itself. No third-party backend dependency may be imported by the core.
    """
    py_files = sorted(_TENSIO_SRC.glob("*.py"))
    assert py_files, f"No .py files found under {_TENSIO_SRC}"

    offenders: list[str] = []
    for path in py_files:
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
        for name in sorted(_imported_top_level_names(tree)):
            if name not in _ALLOWED_TOP_LEVEL:
                offenders.append(f"{path.name}: imports '{name}'")

    assert not offenders, (
        "spec/src/hotam_spec/*.py must import only the Python stdlib or "
        "hotam_spec itself -- the core stays backend-neutral by construction "
        "(R-backend-scope, R-core-imports-stdlib-or-hotam-spec-only slice). "
        "Offenders:\n" + "\n".join(offenders)
    )
