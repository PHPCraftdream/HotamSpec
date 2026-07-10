"""Canon: §Loop — sys.path setup for CLI wrappers.

The tools/ directory lives at ``spec/tools`` INSIDE the framework repo
(reachable via ``repo_paths.tools_root()`` — a framework-internal path, not a
consumer path). CLI wrappers need tools/ on sys.path to import the tool
modules (e.g. ``import gen_spec``), because the tools are standalone scripts
(not package members).

This helper resolves the tools directory and prepends it to sys.path. It is
idempotent (guards with ``in sys.path``).

Resolution order (first existing directory wins):

1. ``repo_paths.tools_root()`` — the real ``spec/tools/`` in the framework
   checkout. Present in self-hosting mode (editable install) and in any
   working-tree context where the full repo is available.

2. ``hotam_spec/_tools/`` — a flat copy of the tool scripts shipped inside
   the installed package (populated at build time by
   ``scripts/populate_tools.py``). Present in a non-editable wheel install
   where ``spec/tools/`` does not exist. This directory is NOT a Python
   package (no ``__init__.py``); it is added to ``sys.path`` so that the
   tools' bare-module imports (``import gen_spec``, ``from _graph_loader
   import ...``) resolve the same way they do in self-hosting mode.
"""

from __future__ import annotations

import sys
from pathlib import Path

from hotam_spec.repo_paths import tools_root


def _resolve_tools_path() -> str:
    """Return the best available tools directory path.

    Prefers the live ``spec/tools/`` checkout (self-hosting / editable install).
    Falls back to the ``hotam_spec/_tools/`` directory shipped inside a wheel.
    """
    live = tools_root()
    if live.is_dir():
        return str(live)
    # Wheel-installed fallback: _tools/ sits next to this cli/ package,
    # i.e. at hotam_spec/_tools/.
    bundled = Path(__file__).resolve().parent.parent / "_tools"
    if bundled.is_dir():
        return str(bundled)
    # Neither found — return the live path anyway so downstream gets a clear
    # error pointing at the expected location rather than a silent no-op.
    return str(live)


_TOOLS_PATH = _resolve_tools_path()
if _TOOLS_PATH not in sys.path:
    sys.path.insert(0, _TOOLS_PATH)


def ensure_tools_on_path() -> None:
    """Idempotent: ensure the tools directory is on sys.path.

    Module-level setup already runs on import of this package, but some
    entry points may be invoked before that if the import order is unusual.
    Calling this explicitly is always safe (it's a no-op if already present).
    """
    if _TOOLS_PATH not in sys.path:
        sys.path.insert(0, _TOOLS_PATH)
