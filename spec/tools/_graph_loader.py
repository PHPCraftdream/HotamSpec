"""Shared ``--demo`` / active-domain graph loader for spec/tools/*.py CLIs.

RULE (R-shared-tools-in-spec-tools): CLI-specific helpers — not part of the
core hotam_spec API surface — live under spec/tools/, in a private (``_``
prefixed) sibling module, following the same convention as ``_bootstrap.py``,
``_ticket_store.py``, ``_delegation_store.py``.

Before this module, an almost byte-identical ``_load_graph(*, demo: bool)``
was copy-pasted into five tool scripts (attention.py, audit_atomicity.py,
confront.py, what_now.py, audit_tensions.py) — the same "demo fixture opt-in
vs. active-domain content" branch, including the same ``spec/tests`` sys.path
splice needed to import the fixture (which lives under tests/, outside
spec/src, so it is deliberately NOT importable from committed production
code paths). Divergence risk: any future change to how the fixture is loaded
(or to load_content_graph's signature) had to be applied five times.

WHY spec/tools/ and not spec/src/hotam_spec/: the fixture graph
(tests/fixtures/seed.py) is TEST-ONLY content and the sys.path splice that
reaches it is a CLI/script concern, not a redistributable library concern —
hotam_spec (spec/src/) must stay importable standalone without a tests/
directory present (R-core-imports-stdlib-or-hotam-spec-only). A tools/
helper can freely reach into tests/ the way each of the five originals did;
a src/ helper should not.
"""

from __future__ import annotations

import sys
from pathlib import Path

import _bootstrap  # noqa: F401  -- side effect: sys.path configured

from hotam_spec.graph import TensionGraph, load_content_graph  # noqa: E402

#: spec/ root, independent of which tool script imports this module.
_SPEC_ROOT = Path(__file__).resolve().parents[1]


def load_graph(*, demo: bool) -> TensionGraph:
    """Return the graph a CLI should operate on: demo fixture or domain content.

    ``demo=True`` loads the fixture seed graph (tests/fixtures/seed.py),
    splicing spec/tests onto sys.path first (idempotent, guarded). Otherwise
    loads the active domain's content graph via
    ``hotam_spec.graph.load_content_graph()`` (env -> pin -> alphabetical,
    R-active-domain-pin-not-alphabetical).
    """
    if demo:
        tests_dir = str(_SPEC_ROOT / "tests")
        if tests_dir not in sys.path:
            sys.path.insert(0, tests_dir)
        from fixtures.seed import seed_graph  # noqa: PLC0415

        return seed_graph()
    return load_content_graph()


def load_graph_with_label(*, demo: bool) -> tuple[TensionGraph, str]:
    """Return (graph, source_label) — the what_now.py variant of load_graph().

    Same resolution as load_graph(); additionally returns a short label
    ("demo fixture" / "content") describing which source was used, so
    callers can report it alongside the diagnosis.
    """
    label = "demo fixture" if demo else "content"
    return load_graph(demo=demo), label
