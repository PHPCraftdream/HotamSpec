"""Tests for check_bijection_r_to_enforcer — R-to-check_* bijection invariant.

Three duties:
  1. The content graph (meta-domain) passes with zero violations.
  2. A fake/unresolvable check_* enforcer name fires a violation.
  3. An orphan check_* (in ALL_INVARIANTS but unnamed by any R) fires a violation.
"""

from __future__ import annotations


from hotam_spec.axis import Axis  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.invariants import (  # noqa: E402
    check_bijection_r_to_enforcer,
    holds,
)
from hotam_spec.requirement import ENFORCED, Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402


def _minimal_graph(*reqs: Requirement) -> TensionGraph:
    """Build a minimal well-formed graph with the given requirements."""
    return TensionGraph(
        axes=(Axis(slug="x-vs-y", description="test axis"),),
        stakeholders=(Stakeholder(id="s1", name="S1", domain="test"),),
        requirements=reqs,
    )


# ---------------------------------------------------------------------------
# 1. Content graph passes cleanly
# ---------------------------------------------------------------------------


def test_content_graph_bijection_clean() -> None:
    """The meta-domain graph has no bijection violations."""
    from hotam_spec.graph import load_content_graph  # noqa: PLC0415

    g = load_content_graph()
    assert holds(check_bijection_r_to_enforcer(g))


# ---------------------------------------------------------------------------
# 2. Unresolvable check_* enforcer fires
# ---------------------------------------------------------------------------


def test_unresolvable_check_fires() -> None:
    """A SETTLED/ENFORCED R naming a check_* not in ALL_INVARIANTS fires."""
    r = Requirement(
        id="R-fake",
        claim="fake",
        owner="s1",
        status="SETTLED",
        enforcement=ENFORCED,
        enforced_by=("check_nonexistent_invariant",),
    )
    g = _minimal_graph(r)
    vs = check_bijection_r_to_enforcer(g)
    assert len(vs) >= 1
    assert any("unresolvable" in v.message.lower() for v in vs)
    assert any(v.target == "R-fake" for v in vs)


# ---------------------------------------------------------------------------
# 3. Orphan check_* fires
# ---------------------------------------------------------------------------


def test_orphan_check_fires() -> None:
    """A check_* in ALL_INVARIANTS not named by any SETTLED/ENFORCED R fires."""
    # Build a graph with one SETTLED/ENFORCED R that names only ONE check.
    # All other checks in ALL_INVARIANTS become orphans.
    r = Requirement(
        id="R-one",
        claim="one",
        owner="s1",
        status="SETTLED",
        enforcement=ENFORCED,
        enforced_by=("check_no_dangling_ids",),
    )
    g = _minimal_graph(r)
    vs = check_bijection_r_to_enforcer(g)
    # There should be orphan violations for all checks except check_no_dangling_ids
    assert len(vs) >= 1
    assert all(v.invariant == "check_bijection_r_to_enforcer" for v in vs)
    assert any("orphan" in v.message.lower() for v in vs)
    # check_no_dangling_ids should NOT be an orphan
    orphan_targets = {v.target for v in vs if "orphan" in v.message.lower()}
    assert "check_no_dangling_ids" not in orphan_targets


# ---------------------------------------------------------------------------
# 4. test_* names are exempt (not flagged as unresolvable)
# ---------------------------------------------------------------------------


def test_test_names_exempt() -> None:
    """enforced_by entries starting with 'test_' are not checked against ALL_INVARIANTS."""
    # Pick a real check_* to satisfy at least one invariant to avoid orphan noise
    # Use a requirement that names only a test_* — should not fire unresolvable.
    r = Requirement(
        id="R-test-only",
        claim="test",
        owner="s1",
        status="SETTLED",
        enforcement=ENFORCED,
        enforced_by=("test_something.py",),
    )
    g = _minimal_graph(r)
    vs = check_bijection_r_to_enforcer(g)
    # Should have orphan violations (for ALL_INVARIANTS checks) but NOT
    # an unresolvable violation for test_something.py
    unresolvable = [v for v in vs if "unresolvable" in v.message.lower()]
    assert len(unresolvable) == 0
