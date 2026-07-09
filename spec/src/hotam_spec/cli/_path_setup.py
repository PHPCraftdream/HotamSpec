"""Canon: §Loop — sys.path setup for CLI wrappers.

The tools/ directory lives at ``spec/tools`` INSIDE the framework repo
(reachable via ``repo_paths.tools_root()`` — a framework-internal path, not a
consumer path). CLI wrappers need tools/ on sys.path to import the tool
modules (e.g. ``import gen_spec``), because the tools are standalone scripts
(not package members).

This helper resolves tools_root() once and prepends it to sys.path. It is
idempotent (guards with ``in sys.path``).

In self-hosting mode (editable install from the HotamSpec repo), tools_root()
points at the real spec/tools/ — the dev workflow is unchanged. In a
pip-installed-consumer scenario, tools/ is NOT shipped (only the hotam_spec
package is), so the CLI wrappers are best-effort: they work when the framework
is installed editable from source, and degrade gracefully (clear ImportError)
when tools/ is absent.
"""

from __future__ import annotations

import sys

from hotam_spec.repo_paths import tools_root

_TOOLS_PATH = str(tools_root())
if _TOOLS_PATH not in sys.path:
    sys.path.insert(0, _TOOLS_PATH)


def ensure_tools_on_path() -> None:
    """Idempotent: ensure spec/tools is on sys.path.

    Module-level setup already runs on import of this package, but some
    entry points may be invoked before that if the import order is unusual.
    Calling this explicitly is always safe (it's a no-op if already present).
    """
    if _TOOLS_PATH not in sys.path:
        sys.path.insert(0, _TOOLS_PATH)
