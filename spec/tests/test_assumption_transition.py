"""Tests for the assumption kill-path (§Proposal / §Assumption).

Covers:
  - ProposedAssumptionTransition validation (signoff asymmetry).
  - the HOLDS → UNCERTAIN → DEAD cycle applied to a synthetic graph source.
  - the closure signal: a transition to DEAD, once loaded, makes
    dependents_of_dead_assumptions non-empty and surfaces a P2 DRIFT_FALLOUT
    action in what_now.diagnose (R-assumption-transition-kind-exists).
"""

from __future__ import annotations

import ast
import sys
from pathlib import Path

import pytest

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import apply_proposal  # noqa: E402
from hotam_spec.proposal import ProposedAssumptionTransition  # noqa: E402

# A minimal, self-contained domain-graph source with one Assumption that one
# Requirement rests on — enough to exercise the writer and the drift traversal.
_SAMPLE_SOURCE = '''\
from __future__ import annotations

from hotam_spec.assumption import Assumption, HOLDS


def build_graph():
    assumptions = (
        Assumption(
            id="A-target",
            statement="A belief the requirement rests on.",
            status=HOLDS,
            owner="framework-author",
        ),
    )

    requirements = (
    )
'''


# ---------------------------------------------------------------------------
# Validation — the signoff asymmetry
# ---------------------------------------------------------------------------


def test_validate_uncertain_needs_no_signoff() -> None:
    raw = {
        "kind": "AssumptionTransition",
        "assumption_id": "A-target",
        "new_status": "UNCERTAIN",
        "reason": "a new data point cast doubt",
    }
    p = apply_proposal._validate_proposal(raw)
    assert isinstance(p, ProposedAssumptionTransition)
    assert p.decided_by == ""
    assert p.target_anchor() == "A-target"


@pytest.mark.parametrize("status", ["DEAD", "HOLDS", "IMPLEMENTS"])
def test_validate_dead_and_holds_require_signoff(status: str) -> None:
    raw = {
        "kind": "AssumptionTransition",
        "assumption_id": "A-target",
        "new_status": status,
        "reason": "because",
    }
    with pytest.raises(ValueError, match="decided_by"):
        apply_proposal._validate_assumption_transition(raw)
    # With decided_by, it validates.
    raw["decided_by"] = "framework-author"
    p = apply_proposal._validate_assumption_transition(raw)
    assert p.decided_by == "framework-author"


@pytest.mark.parametrize(
    "raw,fragment",
    [
        ({"new_status": "DEAD", "reason": "x"}, "assumption_id"),
        (
            {"assumption_id": "A-x", "new_status": "BOGUS", "reason": "x"},
            "new_status",
        ),
        (
            {"assumption_id": "A-x", "new_status": "UNCERTAIN", "reason": ""},
            "reason",
        ),
    ],
)
def test_validate_rejects_bad_input(raw, fragment) -> None:
    raw = {"kind": "AssumptionTransition", **raw}
    with pytest.raises(ValueError, match=fragment):
        apply_proposal._validate_assumption_transition(raw)


# ---------------------------------------------------------------------------
# IMPLEMENTS — the VOLITIONAL род (R-assumption-implements-state)
# ---------------------------------------------------------------------------


def test_implements_requires_signoff() -> None:
    raw = {
        "kind": "AssumptionTransition",
        "assumption_id": "A-target",
        "new_status": "IMPLEMENTS",
        "reason": "we understood this is not a fact but a goal",
    }
    with pytest.raises(ValueError, match="decided_by"):
        apply_proposal._validate_assumption_transition(raw)
    raw["decided_by"] = "domain-user"
    p = apply_proposal._validate_assumption_transition(raw)
    assert p.new_status == "IMPLEMENTS"
    assert p.decided_by == "domain-user"


def test_implements_is_valid_assumption_state() -> None:
    from hotam_spec.assumption import ASSUMPTION_STATES, IMPLEMENTS

    assert IMPLEMENTS == "IMPLEMENTS"
    assert IMPLEMENTS in ASSUMPTION_STATES


@pytest.mark.parametrize(
    "src_status,dst_status",
    [
        ("UNCERTAIN", "IMPLEMENTS"),  # 'not a fact, a goal'
        ("HOLDS", "IMPLEMENTS"),  # 'declared fact too early'
        ("IMPLEMENTS", "HOLDS"),  # 'achieved, became fact'
        ("IMPLEMENTS", "DEAD"),  # 'abandoned the striving'
    ],
)
def test_implements_transition_directions_apply(src_status, dst_status) -> None:
    """All four IMPLEMENTS-touching transitions write cleanly with signoff."""
    source = f'''\
from __future__ import annotations

from hotam_spec.assumption import Assumption, {src_status}


def build_graph():
    assumptions = (
        Assumption(
            id="A-target",
            statement="A belief.",
            status={src_status},
            owner="framework-author",
        ),
    )

    requirements = (
    )
'''
    p = ProposedAssumptionTransition(
        assumption_id="A-target",
        new_status=dst_status,
        reason=f"{src_status} to {dst_status}",
        decided_by="domain-user",
    )
    out = apply_proposal._apply_assumption_transition(source, p)
    assert f"status={dst_status}" in out
    ast.parse(out)


def test_implements_status_valid_invariant() -> None:
    """check_assumption_status_valid accepts IMPLEMENTS, rejects a bogus status."""
    from hotam_spec.assumption import Assumption
    from hotam_spec.graph import TensionGraph
    from hotam_spec.invariants import check_assumption_status_valid

    good = TensionGraph(
        assumptions=(
            Assumption(
                id="A-aspire",
                statement="we strive for this",
                status="IMPLEMENTS",
                owner="framework-author",
            ),
        )
    )
    assert check_assumption_status_valid(good) == []

    bad = TensionGraph(
        assumptions=(
            Assumption(
                id="A-bogus",
                statement="x",
                status="WISHFUL",
                owner="framework-author",
            ),
        )
    )
    viols = check_assumption_status_valid(bad)
    assert len(viols) == 1 and viols[0].target == "A-bogus"


def test_implements_neither_ages_nor_falls_out() -> None:
    """An IMPLEMENTS assumption raises no UNCERTAIN-aging and no DEAD-fallout."""
    from hotam_spec.assumption import Assumption
    from hotam_spec.graph import (
        TensionGraph,
        dead_assumptions,
        uncertain_assumptions,
    )
    from hotam_spec.requirement import Requirement

    a = Assumption(
        id="A-aspire",
        statement="we strive",
        status="IMPLEMENTS",
        owner="framework-author",
    )
    reqs = tuple(
        Requirement(
            id=f"R-{i}",
            claim="c",
            owner="framework-author",
            status="SETTLED",
            why="w",
            assumptions=("A-aspire",),
        )
        for i in range(50)
    )
    g = TensionGraph(assumptions=(a,), requirements=reqs)
    assert uncertain_assumptions(g) == ()  # not a doubt
    assert dead_assumptions(g) == ()  # not a broken premise


# ---------------------------------------------------------------------------
# Writer — HOLDS → UNCERTAIN → DEAD cycle over synthetic source
# ---------------------------------------------------------------------------


def test_transition_missing_assumption_is_refused() -> None:
    p = ProposedAssumptionTransition(
        assumption_id="A-does-not-exist",
        new_status="UNCERTAIN",
        reason="x",
    )
    with pytest.raises(RuntimeError, match="not found"):
        apply_proposal._apply_assumption_transition(_SAMPLE_SOURCE, p)


def test_holds_uncertain_dead_cycle() -> None:
    # HOLDS → UNCERTAIN
    p1 = ProposedAssumptionTransition(
        assumption_id="A-target",
        new_status="UNCERTAIN",
        reason="doubt raised",
    )
    src = apply_proposal._apply_assumption_transition(_SAMPLE_SOURCE, p1)
    assert "status=UNCERTAIN" in src
    assert "doubt raised" in src
    ast.parse(src)  # still valid python
    # import ensured
    assert "UNCERTAIN" in src.split("build_graph")[0]

    # UNCERTAIN → DEAD
    p2 = ProposedAssumptionTransition(
        assumption_id="A-target",
        new_status="DEAD",
        reason="falsified in prod",
        decided_by="framework-author",
    )
    src2 = apply_proposal._apply_assumption_transition(src, p2)
    assert "status=DEAD" in src2
    assert "falsified in prod" in src2
    ast.parse(src2)

    # DEAD → HOLDS (re-affirm)
    p3 = ProposedAssumptionTransition(
        assumption_id="A-target",
        new_status="HOLDS",
        reason="re-verified true",
        decided_by="framework-author",
    )
    src3 = apply_proposal._apply_assumption_transition(src2, p3)
    assert "status=HOLDS" in src3
    ast.parse(src3)
    # The node is never deleted; the full falsification trail survives.
    assert "doubt raised" in src3
    assert "falsified in prod" in src3
    assert "re-verified true" in src3


# ---------------------------------------------------------------------------
# CLOSURE — a DEAD transition surfaces P2 DRIFT_FALLOUT in the harness
# ---------------------------------------------------------------------------


def _load_graph_from_source(src: str):
    """Exec a synthetic graph-source string and return its TensionGraph."""
    from hotam_spec.graph import TensionGraph

    ns: dict = {}
    exec(compile(src, "<synthetic>", "exec"), ns)  # noqa: S102
    return ns["build_graph"]()


def test_dead_transition_surfaces_drift_fallout() -> None:
    from hotam_spec.graph import (  # noqa
        dead_assumptions,
        requirements_on_assumption,
    )

    # A synthetic graph with a Requirement resting on A-target.
    source_with_dep = '''\
from __future__ import annotations

from hotam_spec.assumption import Assumption, HOLDS
from hotam_spec.requirement import Requirement
from hotam_spec.graph import TensionGraph


def build_graph():
    assumptions = (
        Assumption(
            id="A-target",
            statement="A belief the requirement rests on.",
            status=HOLDS,
            owner="framework-author",
        ),
    )
    requirements = (
        Requirement(
            id="R-rests-on-it",
            claim="A claim that rests on A-target.",
            owner="framework-author",
            status="SETTLED",
            why="w",
            assumptions=("A-target",),
        ),
    )
    return TensionGraph(assumptions=assumptions, requirements=requirements)
'''
    # Before: no dead assumptions, no fallout.
    g0 = _load_graph_from_source(source_with_dep)
    assert dead_assumptions(g0) == ()

    # Kill A-target.
    p = ProposedAssumptionTransition(
        assumption_id="A-target",
        new_status="DEAD",
        reason="world changed",
        decided_by="framework-author",
    )
    killed_src = apply_proposal._apply_assumption_transition(source_with_dep, p)
    g1 = _load_graph_from_source(killed_src)

    # After: the assumption is DEAD and its dependent requirement is fallout.
    assert {a.id for a in dead_assumptions(g1)} == {"A-target"}
    dep_ids = {r.id for r in requirements_on_assumption(g1, "A-target")}
    assert "R-rests-on-it" in dep_ids

    # And what_now.diagnose renders it as a P2 DRIFT_FALLOUT action.
    import what_now

    actions = what_now.diagnose(g1)
    drift = [a for a in actions if a.kind == "DRIFT_FALLOUT" and a.target == "R-rests-on-it"]
    assert drift, "a DEAD assumption's dependent must surface as P2 DRIFT_FALLOUT"
