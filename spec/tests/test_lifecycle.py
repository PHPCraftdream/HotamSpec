"""Tests for hotam_spec.lifecycle — the generic state-machine value-type (P1).

Two duties:
  1. The framework-supplied canonical Lifecycles (REQUIREMENT_STATUS_LIFECYCLE,
     CONFLICT_LIFECYCLE) are structurally well-formed.
  2. Lifecycle.matches() handles exact, prefix, and unknown values correctly.
  3. check_status_in_lifecycle fires on bogus stored values and stays green on
     the real content graph.
  4. check_lifecycle_wellformed fires on structurally broken hand-built Lifecycles
     (anti-phantom guard).
  5. check_canonical_lifecycles_wellformed passes on the real content graph.
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from hotam_spec.conflict import Conflict, conflict_identity  # noqa: E402
from hotam_spec.graph import TensionGraph, load_content_graph  # noqa: E402
from hotam_spec.invariants import (  # noqa: E402
    check_canonical_lifecycles_wellformed,
    check_lifecycle_wellformed,
    check_status_in_lifecycle,
)
from hotam_spec.lifecycle import (  # noqa: E402
    CONFLICT_LIFECYCLE,
    INITIAL,
    NORMAL,
    REQUIREMENT_STATUS_LIFECYCLE,
    TERMINAL,
    Lifecycle,
    State,
    Transition,
)
from hotam_spec.requirement import Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_S_A = Stakeholder(id="sa", name="A", domain="x")
_S_B = Stakeholder(id="sb", name="B", domain="x")
_S_OUT = Stakeholder(id="outsider", name="Outsider", domain="x")


def _req(rid: str, owner: str, status: str = "SETTLED") -> Requirement:
    return Requirement(id=rid, claim=f"claim {rid}", owner=owner, status=status)


def _conflict(lifecycle: str = "DETECTED") -> Conflict:
    axis = "cost-vs-flexibility"
    ctx = "some scenario"
    return Conflict(
        id=conflict_identity(axis, ctx),
        axis=axis,
        context=ctx,
        members=("R-1", "R-2"),
        steward="outsider",
        lifecycle=lifecycle,
    )


def _small_graph(
    req_status: str = "SETTLED", conflict_lifecycle: str = "DETECTED"
) -> TensionGraph:
    from hotam_spec.axis import Axis  # noqa: PLC0415

    axes = (Axis(slug="cost-vs-flexibility", description="cost vs flexibility"),)
    reqs = (_req("R-1", "sa", status=req_status), _req("R-2", "sb"))
    c = _conflict(lifecycle=conflict_lifecycle)
    return TensionGraph(
        axes=axes,
        stakeholders=(_S_A, _S_B, _S_OUT),
        requirements=reqs,
        conflicts=(c,),
    )


# ---------------------------------------------------------------------------
# 1. Canonical Lifecycles are well-formed
# ---------------------------------------------------------------------------


def test_requirement_status_lifecycle_wellformed() -> None:
    """REQUIREMENT_STATUS_LIFECYCLE has no structural issues."""
    issues = check_lifecycle_wellformed(REQUIREMENT_STATUS_LIFECYCLE)
    assert issues == [], f"REQUIREMENT_STATUS_LIFECYCLE issues: {issues}"


def test_conflict_lifecycle_wellformed() -> None:
    """CONFLICT_LIFECYCLE has no structural issues."""
    issues = check_lifecycle_wellformed(CONFLICT_LIFECYCLE)
    assert issues == [], f"CONFLICT_LIFECYCLE issues: {issues}"


# ---------------------------------------------------------------------------
# 2. Lifecycle.matches() — exact, prefix, and unknown
# ---------------------------------------------------------------------------


def test_lifecycle_matches_exact() -> None:
    """matches() returns the correct State for an exact name like 'SETTLED'."""
    s = REQUIREMENT_STATUS_LIFECYCLE.matches("SETTLED")
    assert s is not None
    assert s.name == "SETTLED"


def test_lifecycle_matches_prefix_open() -> None:
    """matches() returns OPEN State for 'OPEN(which scope?)'."""
    s = REQUIREMENT_STATUS_LIFECYCLE.matches("OPEN(which scope?)")
    assert s is not None
    assert s.name == "OPEN"


def test_lifecycle_matches_prefix_decided() -> None:
    """matches() returns DECIDED State for 'DECIDED(picked X)'."""
    s = CONFLICT_LIFECYCLE.matches("DECIDED(picked X)")
    assert s is not None
    assert s.name == "DECIDED"


def test_lifecycle_matches_prefix_revisit_when() -> None:
    """matches() returns REVISIT_WHEN State for 'REVISIT_WHEN(condition fires)'."""
    s = CONFLICT_LIFECYCLE.matches("REVISIT_WHEN(condition fires)")
    assert s is not None
    assert s.name == "REVISIT_WHEN"


def test_lifecycle_matches_unknown() -> None:
    """matches() returns None for a value not in any state."""
    s = REQUIREMENT_STATUS_LIFECYCLE.matches("BOGUS")
    assert s is None


def test_lifecycle_matches_prefix_no_false_match() -> None:
    """matches() does NOT match 'DECIDED2' against the 'DECIDED' prefix state."""
    s = CONFLICT_LIFECYCLE.matches("DECIDED2")
    assert s is None, (
        "prefix match must require '(' after the name, not just startswith"
    )


# ---------------------------------------------------------------------------
# 3. check_status_in_lifecycle fires on bogus values
# ---------------------------------------------------------------------------


def test_check_status_in_lifecycle_fires_on_bogus_status() -> None:
    """check_status_in_lifecycle fires when Requirement.status is not canonical."""
    g = _small_graph(req_status="BOGUS")
    v = check_status_in_lifecycle(g)
    assert any(x.target == "R-1" and "BOGUS" in x.message for x in v), (
        f"expected violation for status='BOGUS', got: {v}"
    )


def test_check_status_in_lifecycle_fires_on_bogus_conflict_lifecycle() -> None:
    """check_status_in_lifecycle fires when Conflict.lifecycle is not canonical."""
    g = _small_graph(conflict_lifecycle="WHATEVER")
    v = check_status_in_lifecycle(g)
    assert any("WHATEVER" in x.message for x in v), (
        f"expected violation for lifecycle='WHATEVER', got: {v}"
    )


def test_check_status_in_lifecycle_passes_on_valid_values() -> None:
    """check_status_in_lifecycle stays green on canonical status/lifecycle values."""
    for status in ("DRAFT", "SETTLED", "REJECTED", "OPEN(a question?)"):
        g = _small_graph(req_status=status)
        v = [x for x in check_status_in_lifecycle(g) if x.target == "R-1"]
        assert not v, f"unexpected violation for status={status!r}: {v}"

    for lc_val in (
        "DETECTED",
        "ACKNOWLEDGED",
        "DECIDED(rationale)",
        "REVISIT_WHEN(cond)",
    ):
        g = _small_graph(conflict_lifecycle=lc_val)
        v = [
            x
            for x in check_status_in_lifecycle(g)
            if "lifecycle" in x.message.lower()
            and x.invariant == "check_status_in_lifecycle"
        ]
        assert not v, f"unexpected lifecycle violation for {lc_val!r}: {v}"


# ---------------------------------------------------------------------------
# 4. check_lifecycle_wellformed fires on structurally broken Lifecycles
# ---------------------------------------------------------------------------


def test_check_lifecycle_wellformed_fires_on_dangling_transition() -> None:
    """check_lifecycle_wellformed fires when a transition references an unknown state."""
    lc = Lifecycle(
        slug="bad-dangling",
        states=(
            State("START", kind=INITIAL),
            State("END", kind=TERMINAL),
        ),
        transitions=(
            Transition("START", "GHOST", event="go"),  # GHOST is unknown
        ),
    )
    issues = check_lifecycle_wellformed(lc)
    assert any("GHOST" in i for i in issues), (
        f"expected dangling-dst issue, got: {issues}"
    )


def test_check_lifecycle_wellformed_fires_on_no_initial() -> None:
    """check_lifecycle_wellformed fires when no INITIAL state is declared."""
    lc = Lifecycle(
        slug="bad-no-initial",
        states=(
            State("A", kind=NORMAL),
            State("B", kind=TERMINAL),
        ),
        transitions=(Transition("A", "B", event="go"),),
    )
    issues = check_lifecycle_wellformed(lc)
    assert any("INITIAL" in i for i in issues), (
        f"expected no-initial issue, got: {issues}"
    )


def test_check_lifecycle_wellformed_fires_on_no_reachable_terminal() -> None:
    """check_lifecycle_wellformed fires when no terminal is reachable (non-cyclic)."""
    lc = Lifecycle(
        slug="bad-no-terminal",
        states=(
            State("START", kind=INITIAL),
            State("MID", kind=NORMAL),
        ),
        transitions=(Transition("START", "MID", event="go"),),
        cyclic=False,
    )
    issues = check_lifecycle_wellformed(lc)
    assert any("terminal" in i for i in issues), (
        f"expected terminal-reachability issue, got: {issues}"
    )


def test_check_lifecycle_wellformed_cyclic_no_terminal_ok() -> None:
    """check_lifecycle_wellformed does NOT require a terminal for cyclic=True Lifecycles."""
    lc = Lifecycle(
        slug="cyclic-ok",
        states=(
            State("A", kind=INITIAL),
            State("B", kind=NORMAL),
        ),
        transitions=(
            Transition("A", "B", event="go"),
            Transition("B", "A", event="back"),
        ),
        cyclic=True,
    )
    issues = check_lifecycle_wellformed(lc)
    assert not issues, (
        f"cyclic lifecycle with no terminal must be well-formed, got: {issues}"
    )


# ---------------------------------------------------------------------------
# 5. check_canonical_lifecycles_wellformed passes on the real content graph
# ---------------------------------------------------------------------------


def test_canonical_lifecycles_pass_on_real_graph() -> None:
    """check_canonical_lifecycles_wellformed returns [] on the real content graph."""
    g = load_content_graph()
    v = check_canonical_lifecycles_wellformed(g)
    assert v == [], f"canonical lifecycle well-formedness failed: {v}"
