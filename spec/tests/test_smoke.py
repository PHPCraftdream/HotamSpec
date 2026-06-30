"""Smoke test — one fast end-to-end health signal for the framework.

Imports every tensio module, loads the content graph, runs all invariants,
runs the harness, and runs the generators twice to confirm determinism. A single
green signal means: the framework is alive, the content graph is structurally
sound, the harness and generators work.

R-smoke-test (DRAFT) declared: "spec/tests/test_smoke.py shall provide one fast
end-to-end signal that the framework is healthy." This file is that signal —
DRAFT becomes real-not-proposed.
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

# Import every tensio module explicitly (framework health).
import tensio.assumption  # noqa: E402, F401
import tensio.axis  # noqa: E402, F401
import tensio.conflict  # noqa: E402, F401
import tensio.graph  # noqa: E402, F401
import tensio.invariants  # noqa: E402, F401
import tensio.requirement  # noqa: E402, F401
import tensio.stakeholder  # noqa: E402, F401

from tensio.graph import load_content_graph  # noqa: E402
from tensio.invariants import all_violations, holds  # noqa: E402
import gen_spec  # noqa: E402
import what_now  # noqa: E402


def test_smoke() -> None:
    """One-shot framework health check (R-smoke-test).

    - load_content_graph() without error;
    - all_violations(g) == [];
    - what_now.diagnose(g) runs and is deterministic;
    - build_requirements/build_tensions/build_open each run and are byte-stable.
    """
    g = load_content_graph()

    violations = all_violations(g)
    assert holds(violations), (
        "smoke: content graph has structural violations:\n"
        + "\n".join(f"  {v}" for v in violations)
    )

    d1 = what_now.diagnose(g)
    d2 = what_now.diagnose(g)
    assert d1 == d2, "smoke: what_now.diagnose is not deterministic"

    r1 = gen_spec.build_requirements(g)
    r2 = gen_spec.build_requirements(g)
    assert r1 == r2, "smoke: build_requirements is not deterministic"

    t1 = gen_spec.build_tensions(g)
    t2 = gen_spec.build_tensions(g)
    assert t1 == t2, "smoke: build_tensions is not deterministic"

    o1 = gen_spec.build_open(g)
    o2 = gen_spec.build_open(g)
    assert o1 == o2, "smoke: build_open is not deterministic"
