"""Canonical sys.path bootstrap for spec/tools/*.py scripts.

RULE: every tool script that imports ``hotam_spec`` (or a sibling ``_*.py``
private module) needs ``spec/src`` and ``spec/tools`` on ``sys.path``.
Before this module, ~23 tool scripts each inlined the same 3-line prologue:

    SPEC_ROOT = Path(__file__).resolve().parents[1]
    if str(SPEC_ROOT / "src") not in sys.path:
        sys.path.insert(0, str(SPEC_ROOT / "src"))

Importing this module once achieves the same effect:

    import _bootstrap  # noqa: F401  -- side effect: sys.path configured

Side-effect-only import (no public API). The import itself is the contract:
it MUST run before any ``from hotam_spec... import`` line. Idempotent: safe
to import from multiple tools in the same process (guards with ``in sys.path``).

WHY a private module (``_bootstrap.py``) and not ``__init__.py``: the tools
are run as standalone scripts (``python tools/foo.py``), NOT as package
members (``python -m tools.foo``). A package ``__init__.py`` would require
either (a) installing the package via ``pip install -e .``, changing the
developer workflow, or (b) adding ``spec/`` to ``PYTHONPATH`` externally —
both riskier than a one-line ``import _bootstrap`` that any script can opt
into incrementally without a flag day.
"""

from __future__ import annotations

import sys
from pathlib import Path

#: spec/ root (this file lives at spec/tools/_bootstrap.py; parents[1] = spec/).
_SPEC_ROOT = Path(__file__).resolve().parents[1]

#: spec/src — the hotam_spec package parent.
_SRC = _SPEC_ROOT / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

#: spec/tools — sibling private modules (_ticket_store, _delegation_store, etc.).
_TOOLS = _SPEC_ROOT / "tools"
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))
