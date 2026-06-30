"""Content-graph validity tests — enforces R-drift-structurally-impossible
for the CONTENT graph itself.

Today test_invariants.py validates ONLY the demo fixture (seed.py) and
test_docs_gen.py guards the generated docs against drift. Neither asserts
that spec/content/graph.py is structurally well-formed at test time.
This file closes that gap: every structural invariant must hold on the
real meta-domain graph, and the generators must be deterministic over it.

References: R-drift-structurally-impossible (SETTLED), R-deterministic-generation
(SETTLED), R-smoke-test (DRAFT).
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

from tensio.graph import load_content_graph  # noqa: E402
from tensio.invariants import all_violations, holds  # noqa: E402
import gen_spec  # noqa: E402
import what_now  # noqa: E402


def test_content_graph_is_structurally_wellformed() -> None:
    """Every structural invariant holds on the real spec/content/ meta-domain graph."""
    g = load_content_graph()
    violations = all_violations(g)
    assert holds(violations), (
        "spec/content/graph.py has structural violations:\n"
        + "\n".join(f"  {v.invariant} @ {v.target}: {v.message}" for v in violations)
    )


def test_diagnosis_is_deterministic_on_content_graph() -> None:
    """diagnose(g) is deterministic: two calls on the same graph yield the same list."""
    g = load_content_graph()
    a1 = what_now.diagnose(g)
    a2 = what_now.diagnose(g)
    assert a1 == a2, "what_now.diagnose must be deterministic over the content graph"


def test_generators_are_deterministic_on_content_graph() -> None:
    """build_requirements/build_tensions/build_open are byte-stable over content graph."""
    g = load_content_graph()
    assert gen_spec.build_requirements(g) == gen_spec.build_requirements(g)
    assert gen_spec.build_tensions(g) == gen_spec.build_tensions(g)
    assert gen_spec.build_open(g) == gen_spec.build_open(g)
