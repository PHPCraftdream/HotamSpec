"""Smoke test — one fast end-to-end health signal for the framework.

Imports every hotam_spec module, loads the content graph, runs all invariants,
runs the harness, and runs the generators twice to confirm determinism. A single
green signal means: the framework is alive, the content graph is structurally
sound, the harness and generators work.

R-smoke-test (DRAFT) declared: "spec/tests/test_smoke.py shall provide one fast
end-to-end signal that the framework is healthy." This file is that signal —
DRAFT becomes real-not-proposed.
"""

from __future__ import annotations

# Import every hotam_spec module explicitly (framework health).
import hotam_spec.assumption  # noqa: F401
import hotam_spec.axis  # noqa: F401
import hotam_spec.conflict  # noqa: F401
import hotam_spec.graph  # noqa: F401
import hotam_spec.invariants  # noqa: F401
import hotam_spec.requirement  # noqa: F401
import hotam_spec.stakeholder  # noqa: F401

from hotam_spec.graph import load_content_graph
from hotam_spec.invariants import all_violations, holds
import gen_spec
import what_now


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
