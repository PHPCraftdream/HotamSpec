"""Canon: §Invariants — R-core-periphery-import-ratchet.

Structural intra-package layering ratchet. The framework body splits, purely in
code, into two strata:

  * CORE — the typed node classes plus the graph, proposal, and invariant layers
    built directly on them (requirement/conflict/assumption/signoff/lifecycle/
    operator/stakeholder/axis/entity/process → graph → proposal → invariants,
    plus their path/schema/enforcer-resolution and scope_projection helpers —
    scope_projection is consumed INSIDE a core check, so it is core). Core knows
    nothing about what is layered ON TOP of the graph.

  * PERIPHERY — modules built OVER a fully-formed graph: they read/aggregate it
    but are not part of it, and nothing in core imports them
    (attention, reflection, invariants_table_engine).

The dependency arrow runs one way only: PERIPHERY may import CORE, CORE must
NEVER import PERIPHERY. This layering already holds by construction today; the
ratchet nails it down so a future commit cannot silently pull, e.g., attention.py
into graph.py and quietly raise the cost of the deferred "minimal core + plugins"
extraction (the D1 re-scope, deferred a third time in the same wave that landed
this guard). It is the intra-package sibling of test_backend_neutral_scope.py's
inter-package R-core-imports-stdlib-or-hotam-spec-only scan.

WHY an AST import scan (not a runtime scan): a static scan catches a reversed
dependency the instant it is written, before any test exercises the import path
and with no runtime environment needed.
"""

from __future__ import annotations

import ast
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_HOTAM_SPEC_SRC = _SRC / "hotam_spec"

# Modules layered OVER a fully-formed graph (they read/aggregate it, they are not
# part of it). Core modules must never import any of these. Keep this list in
# sync with the actual periphery; adding a genuinely-core module here (or a new
# periphery module NOT here) is the only way to weaken the ratchet, and doing so
# is a visible, reviewable edit.
_PERIPHERY_MODULES = frozenset(
    {
        "attention",
        "reflection",
        "invariants_table_engine",
    }
)


def _hotam_spec_submodule_imports(tree: ast.Module) -> set[str]:
    """Every `hotam_spec.<sub>` import in a module, returned as the set of
    second-segment names (`<sub>`). Covers both `import hotam_spec.sub` and
    `from hotam_spec.sub import ...`. Relative imports (`from . import x`,
    `from .sub import y`) resolve within hotam_spec too, so they are included
    by their leaf name.
    """
    subs: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for alias in node.names:
                parts = alias.name.split(".")
                if parts[0] == "hotam_spec" and len(parts) >= 2:
                    subs.add(parts[1])
        elif isinstance(node, ast.ImportFrom):
            if node.level and node.level > 0:
                # Relative import within the package. `from .attention import x`
                # has module="attention"; `from . import attention` names the
                # submodule as an alias instead.
                if node.module:
                    subs.add(node.module.split(".")[0])
                else:
                    for alias in node.names:
                        subs.add(alias.name.split(".")[0])
                continue
            if node.module:
                parts = node.module.split(".")
                if parts[0] == "hotam_spec" and len(parts) >= 2:
                    subs.add(parts[1])
    return subs


def _core_module_files() -> list[Path]:
    """Every hotam_spec/*.py module that is NOT itself a periphery module."""
    out: list[Path] = []
    for path in sorted(_HOTAM_SPEC_SRC.glob("*.py")):
        if path.stem in _PERIPHERY_MODULES:
            continue
        out.append(path)
    return out


def _offending_core_imports(files: list[Path]) -> list[str]:
    offenders: list[str] = []
    for path in files:
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
        bad = _hotam_spec_submodule_imports(tree) & _PERIPHERY_MODULES
        if bad:
            offenders.append(f"{path.name} imports periphery module(s): {sorted(bad)}")
    return offenders


def test_core_modules_do_not_import_periphery() -> None:
    """AST-scan every core hotam_spec/*.py module: none may import a periphery
    module (attention / reflection / scope_projection / invariants_table_engine).
    The core→periphery dependency arrow is one-way; core stays graph-only.
    """
    core_files = _core_module_files()
    assert core_files, f"No core .py files found under {_HOTAM_SPEC_SRC}"

    offenders = _offending_core_imports(core_files)

    assert not offenders, (
        "A core hotam_spec module imports a periphery module -- the "
        "core/periphery layering (R-core-periphery-import-ratchet) is reversed. "
        "Periphery may import core, never the other way. Offenders:\n"
        + "\n".join(offenders)
    )


def test_ratchet_catches_a_reversed_import() -> None:
    """Negative control: a synthetic core module that imports `hotam_spec.attention`
    is caught by the same scanner the live test relies on. Guards against the
    positive test passing vacuously (e.g. if the AST walk silently stopped
    matching hotam_spec submodule imports).
    """
    poisoned = ast.parse(
        "from hotam_spec.attention import collect\n"
        "import hotam_spec.reflection\n"
        "from hotam_spec.graph import TensionGraph\n"
    )
    hits = _hotam_spec_submodule_imports(poisoned) & _PERIPHERY_MODULES
    assert hits == {"attention", "reflection"}, (
        "The scanner must flag both `from hotam_spec.attention import ...` and "
        f"`import hotam_spec.reflection`, and only those -- got {sorted(hits)}."
    )

    # And the same relative-import spelling a future in-package edit might use.
    poisoned_rel = ast.parse("from .invariants_table_engine import render\n")
    hits_rel = _hotam_spec_submodule_imports(poisoned_rel) & _PERIPHERY_MODULES
    assert hits_rel == {"invariants_table_engine"}, (
        "The scanner must flag a relative `from .invariants_table_engine import "
        f"...` periphery import -- got {sorted(hits_rel)}."
    )
