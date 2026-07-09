"""Canon: §Conscience — Hypothesis property-tests over the critical core.

For each critical-core invariant, build a small valid base graph, mutate it
in a way that should violate that invariant, and assert the violation fires.
This is the formal partner to the latent-connector heuristic — same machinery
(property-tests), narrower scope (the critical core, M7), stronger signal.

Why a property-test and not a single hand-rolled case: the existing
test_invariants.py covers ONE handcrafted broken fixture per invariant.
Property-tests sweep the SPACE of mutations against the SPACE of graphs the
invariant should catch — a real conscience, not a single example.

M7 resolved here: the methodology's critical core is the six invariants in
CRITICAL_CORE_INVARIANTS; the §Conscience sweep is this file.
"""

from __future__ import annotations


from hypothesis import given, settings, HealthCheck, strategies as st  # noqa: E402

from fixtures.seed import DEMO_AXES  # noqa: E402
from hotam_spec.conflict import Conflict, conflict_identity  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.invariants import (  # noqa: E402
    CRITICAL_CORE_INVARIANTS,
    check_decided_has_decided_by,
    check_no_dangling_ids,
    check_open_has_question,
    check_operator_steward_not_self,
    check_steward_not_a_member_owner,
    check_typed_anchors,
    holds,
)
from hotam_spec.operator import ContextBudget, Operator  # noqa: E402
from hotam_spec.requirement import Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

# ---------------------------------------------------------------------------
# Shared small building blocks
# ---------------------------------------------------------------------------

_AXIS = "cost-vs-flexibility"  # present in DEMO_AXES

_STAKEHOLDER_IDS = ["alice", "bob", "carol"]
stakeholder_id = st.sampled_from(_STAKEHOLDER_IDS)


def _stakeholders() -> tuple[Stakeholder, ...]:
    return (
        Stakeholder(id="alice", name="Alice", domain="x"),
        Stakeholder(id="bob", name="Bob", domain="y"),
        Stakeholder(id="carol", name="Carol", domain="z"),
    )


def _req(rid: str, owner: str, status: str = "SETTLED") -> Requirement:
    return Requirement(id=rid, claim=f"claim {rid}", owner=owner, status=status)


def _third(a: str, b: str) -> str:
    """Return the stakeholder id that is neither a nor b."""
    return next(s for s in _STAKEHOLDER_IDS if s not in (a, b))


# ---------------------------------------------------------------------------
# 1. check_steward_not_a_member_owner — steward owns a member → fires
# ---------------------------------------------------------------------------


@given(member_owner=stakeholder_id)
@settings(max_examples=50, suppress_health_check=[HealthCheck.function_scoped_fixture])
def test_steward_not_member_owner_fires_on_violation(member_owner: str) -> None:
    """For any member-owner choice, setting steward=member_owner fires the check."""
    ctx = "shared scenario steward"
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=_stakeholders(),
        requirements=(_req("R-1", member_owner), _req("R-2", "carol")),
        conflicts=(
            Conflict(
                id=conflict_identity(_AXIS, ctx),
                axis=_AXIS,
                context=ctx,
                members=("R-1", "R-2"),
                steward=member_owner,  # violation: steward owns R-1
                lifecycle="ACKNOWLEDGED",
            ),
        ),
    )
    v = check_steward_not_a_member_owner(g)
    assert v, (
        f"check_steward_not_a_member_owner did not fire with steward={member_owner!r} "
        f"who owns R-1"
    )


# ---------------------------------------------------------------------------
# 2. check_operator_steward_not_self — operator stewards its own member → fires
# ---------------------------------------------------------------------------


@given(member_owner=stakeholder_id)
@settings(max_examples=50, suppress_health_check=[HealthCheck.function_scoped_fixture])
def test_operator_steward_not_self_fires_on_violation(member_owner: str) -> None:
    """An operator steward whose stakeholder owns a member fires the check."""
    ctx = "operator self-steward scenario"
    op_id = f"OP-{member_owner}"
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=_stakeholders(),
        operators=(
            Operator(
                id=op_id,
                stakeholder=member_owner,
                lifecycle="ACTIVE",
                context_budget=ContextBudget(limit=0, measure="NODE_COUNT"),
            ),
        ),
        requirements=(_req("R-A", member_owner), _req("R-B", "carol")),
        conflicts=(
            Conflict(
                id=conflict_identity(_AXIS, ctx),
                axis=_AXIS,
                context=ctx,
                members=("R-A", "R-B"),
                steward=op_id,  # violation: operator's stakeholder owns R-A
                lifecycle="ACKNOWLEDGED",
            ),
        ),
    )
    v = check_operator_steward_not_self(g)
    assert v, (
        f"check_operator_steward_not_self did not fire with operator {op_id!r} "
        f"whose stakeholder {member_owner!r} owns R-A"
    )


# ---------------------------------------------------------------------------
# 3. check_decided_has_decided_by — DECIDED with no decided_by → fires
# ---------------------------------------------------------------------------


@given(owner_a=stakeholder_id, owner_b=stakeholder_id)
@settings(max_examples=50, suppress_health_check=[HealthCheck.function_scoped_fixture])
def test_decided_without_decided_by_fires(owner_a: str, owner_b: str) -> None:
    """A DECIDED conflict with empty decided_by fires the check."""
    ctx = "decided no signoff"
    steward = _third(owner_a, owner_b) if owner_a != owner_b else "carol"
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=_stakeholders(),
        requirements=(_req("R-10", owner_a), _req("R-11", owner_b)),
        conflicts=(
            Conflict(
                id=conflict_identity(_AXIS, ctx),
                axis=_AXIS,
                context=ctx,
                members=("R-10", "R-11"),
                steward=steward,
                lifecycle="DECIDED(some rationale)",
                decided_by="",  # violation: no human signoff
            ),
        ),
    )
    v = check_decided_has_decided_by(g)
    assert v, (
        "check_decided_has_decided_by did not fire on DECIDED conflict with "
        "empty decided_by"
    )


# ---------------------------------------------------------------------------
# 4. check_typed_anchors — Requirement id without R- prefix → fires
# ---------------------------------------------------------------------------


@given(bad_prefix=st.sampled_from(["foo", "req-1", "123", "A-bad", "C-bad", "OP-bad"]))
@settings(max_examples=50, suppress_health_check=[HealthCheck.function_scoped_fixture])
def test_typed_anchor_violation_fires(bad_prefix: str) -> None:
    """A Requirement whose id does not start with 'R-' fires the check."""
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=_stakeholders(),
        requirements=(
            Requirement(
                id=bad_prefix,
                claim=f"claim {bad_prefix}",
                owner="alice",
                status="SETTLED",
            ),
        ),
    )
    v = check_typed_anchors(g)
    assert v, (
        f"check_typed_anchors did not fire on Requirement with id={bad_prefix!r} "
        f"(missing 'R-' prefix)"
    )


# ---------------------------------------------------------------------------
# 5. check_no_dangling_ids — requirement with unknown owner → fires
# ---------------------------------------------------------------------------


@given(ghost=st.sampled_from(["ghost", "nobody", "unknown-stakeholder"]))
@settings(max_examples=50, suppress_health_check=[HealthCheck.function_scoped_fixture])
def test_no_dangling_ids_fires(ghost: str) -> None:
    """A requirement whose owner is not a known Stakeholder fires the check."""
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=_stakeholders(),  # alice, bob, carol only
        requirements=(
            _req("R-ghost", ghost),  # violation: ghost not in stakeholders
        ),
    )
    v = check_no_dangling_ids(g)
    assert v, (
        f"check_no_dangling_ids did not fire on requirement with unknown owner {ghost!r}"
    )


# ---------------------------------------------------------------------------
# 6. check_open_has_question — OPEN status with no question → fires
# ---------------------------------------------------------------------------


@given(bad_status=st.sampled_from(["OPEN", "OPEN()", "OPEN(   )", "OPEN( )"]))
@settings(max_examples=50, suppress_health_check=[HealthCheck.function_scoped_fixture])
def test_open_without_question_fires(bad_status: str) -> None:
    """An OPEN requirement with an empty or missing question fires the check."""
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=_stakeholders(),
        requirements=(
            Requirement(
                id="R-open-bad",
                claim="some open claim",
                owner="alice",
                status=bad_status,
            ),
        ),
    )
    v = check_open_has_question(g)
    assert v, (
        f"check_open_has_question did not fire on Requirement with "
        f"status={bad_status!r} (no non-empty question)"
    )


# ---------------------------------------------------------------------------
# Negative sweep: a valid generated graph passes ALL critical-core checks
# ---------------------------------------------------------------------------


@given(
    rid_a=st.integers(min_value=1, max_value=999),
    rid_b=st.integers(min_value=1000, max_value=1999),
    owner_a=stakeholder_id,
    owner_b=stakeholder_id,
)
@settings(max_examples=30, suppress_health_check=[HealthCheck.function_scoped_fixture])
def test_valid_graph_passes_critical_core(
    rid_a: int, rid_b: int, owner_a: str, owner_b: str
) -> None:
    """A well-formed generated graph passes EVERY critical-core invariant."""
    if owner_a == owner_b:
        # Need two distinct owners so steward can be the third
        return
    steward = _third(owner_a, owner_b)
    ctx = f"scenario {rid_a}-{rid_b}"
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=_stakeholders(),
        requirements=(
            _req(f"R-{rid_a}", owner_a),
            _req(f"R-{rid_b}", owner_b),
        ),
        conflicts=(
            Conflict(
                id=conflict_identity(_AXIS, ctx),
                axis=_AXIS,
                context=ctx,
                members=(f"R-{rid_a}", f"R-{rid_b}"),
                steward=steward,
                lifecycle="ACKNOWLEDGED",
            ),
        ),
    )
    for check in CRITICAL_CORE_INVARIANTS:
        v = check(g)
        assert holds(v), f"{check.__name__} fired on a valid graph: {v}"


# ---------------------------------------------------------------------------
# Meta-domain sweep: the live meta-domain passes every critical-core check
# ---------------------------------------------------------------------------


def test_real_meta_domain_passes_critical_core() -> None:
    """The live meta-domain passes every critical-core invariant (today's truth).

    Canon: §Conscience — if this test ever fires, the framework's own substrate
    has a critical-core violation. Surface it before any further phase advances.
    This is also the §Conscience structural path for R-stale-substrate: if any
    DEAD assumption causes a critical-core invariant to fire on the meta-domain,
    it appears here first.
    """
    from hotam_spec.graph import load_content_graph  # noqa: PLC0415

    g = load_content_graph()
    for check in CRITICAL_CORE_INVARIANTS:
        v = check(g)
        assert holds(v), (
            f"critical-core invariant {check.__name__!r} fires on meta-domain: {v}"
        )
