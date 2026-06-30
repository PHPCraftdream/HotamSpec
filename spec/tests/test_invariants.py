"""Tests for tensio.invariants — structural form of the tension graph.

Two duties, mirroring dev-coin's test_invariants:
  1. The demo seed fixture is structurally well-formed (every invariant holds).
  2. Each invariant ACTUALLY FIRES on a deliberately-broken fixture — a guard
     against phantom tests that stay green on broken data. For every check_* we
     build a minimal graph that violates exactly that rule and assert a non-empty
     violation list, naming the offending object.

The fixture lives in `tests/fixtures/seed.py` (outside the framework): Tensio is
a content-free framework, so the example business graph is test data, not
src/tensio/ content.
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TESTS = Path(__file__).resolve().parent
for _p in (_SRC, _TESTS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

from fixtures.seed import DEMO_AXES, seed_graph  # noqa: E402
from tensio.assumption import DEAD, Assumption  # noqa: E402
from tensio.conflict import Conflict, conflict_identity  # noqa: E402
from tensio.graph import TensionGraph  # noqa: E402
from tensio.invariants import (  # noqa: E402
    ALL_INVARIANTS,
    all_violations,
    check_axis_in_registry,
    check_conflict_has_axis_context_steward,
    check_conflict_id_matches_identity,
    check_conflict_min_two_members,
    check_decided_has_rationale_or_derived,
    check_enforced_names_invariant,
    check_m_tag_format,
    check_no_dangling_ids,
    check_open_has_question,
    check_steward_not_a_member_owner,
    check_typed_anchors,
    holds,
)
from tensio.requirement import ENFORCED  # noqa: E402
from tensio.requirement import Relation, Requirement  # noqa: E402
from tensio.stakeholder import Stakeholder  # noqa: E402

# ---------------------------------------------------------------------------
# 1. The demo seed fixture is well-formed
# ---------------------------------------------------------------------------


def test_seed_fixture_is_structurally_wellformed() -> None:
    """Every structural invariant holds on the canonical demo fixture."""
    g = seed_graph()
    violations = all_violations(g)
    assert holds(violations), f"seed fixture has structural violations: {violations}"


def test_all_invariants_in_registry_run_on_seed() -> None:
    """Each registered invariant returns a list (the harness can consume all)."""
    g = seed_graph()
    for check in ALL_INVARIANTS:
        result = check(g)
        assert isinstance(result, list), f"{check.__name__} must return a list"


# ---------------------------------------------------------------------------
# 2. Broken fixtures — each invariant must FIRE (anti-phantom guard)
# ---------------------------------------------------------------------------

_S_OUT = Stakeholder(id="outsider", name="Outsider", domain="x")
_S_A = Stakeholder(id="sa", name="A", domain="x")
_S_B = Stakeholder(id="sb", name="B", domain="x")


def _req(rid: str, owner: str, status: str = "SETTLED", **kw) -> Requirement:
    return Requirement(id=rid, claim=f"claim {rid}", owner=owner, status=status, **kw)


def _wellformed_conflict(**overrides) -> Conflict:
    """A valid conflict (R-1 owner=sa, R-2 owner=sb, steward=outsider) to perturb."""
    axis = overrides.pop("axis", "cost-vs-flexibility")
    context = overrides.pop("context", "some shared scenario")
    base = dict(
        id=conflict_identity(axis, context),
        axis=axis,
        context=context,
        members=("R-1", "R-2"),
        steward="outsider",
        lifecycle="ACKNOWLEDGED",
    )
    base.update(overrides)
    if "id" not in overrides and ("axis" in overrides or "context" in overrides):
        base["id"] = conflict_identity(base["axis"], base["context"])
    return Conflict(**base)  # type: ignore[arg-type]


def _graph_with(conflict: Conflict, reqs=None, assumptions=()) -> TensionGraph:
    """Build a small TensionGraph including the demo axis vocabulary.

    The axis vocabulary lives on the graph (the framework ships none), so test
    graphs must declare the axes they use — otherwise check_axis_in_registry
    would (correctly) fire on "cost-vs-flexibility" etc.
    """
    reqs = reqs if reqs is not None else (_req("R-1", "sa"), _req("R-2", "sb"))
    return TensionGraph(
        axes=DEMO_AXES,
        stakeholders=(_S_OUT, _S_A, _S_B),
        assumptions=assumptions,
        requirements=reqs,
        conflicts=(conflict,),
    )


def test_dangling_member_fires() -> None:
    """check_no_dangling_ids fires when a conflict references a missing member."""
    bad = _wellformed_conflict(members=("R-1", "R-ghost"))
    v = check_no_dangling_ids(_graph_with(bad))
    assert any(x.target == bad.id and "R-ghost" in x.message for x in v)


def test_dangling_owner_fires() -> None:
    """check_no_dangling_ids fires when a requirement owner is unknown."""
    reqs = (_req("R-1", "sa"), _req("R-2", "no_such_owner"))
    g = _graph_with(_wellformed_conflict(), reqs=reqs)
    v = check_no_dangling_ids(g)
    assert any(x.target == "R-2" and "no_such_owner" in x.message for x in v)


def test_dangling_relation_target_fires() -> None:
    """check_no_dangling_ids fires on a relation pointing to a missing requirement."""
    reqs = (
        _req("R-1", "sa", relations=(Relation(kind="supports", target="R-missing"),)),
        _req("R-2", "sb"),
    )
    g = _graph_with(_wellformed_conflict(), reqs=reqs)
    v = check_no_dangling_ids(g)
    assert any(x.target == "R-1" and "R-missing" in x.message for x in v)


def test_missing_axis_fires() -> None:
    """check_conflict_has_axis_context_steward fires on an empty axis."""
    bad = _wellformed_conflict(axis="", id="C-manual")
    v = check_conflict_has_axis_context_steward(_graph_with(bad))
    assert any("axis" in x.message for x in v)


def test_missing_context_fires() -> None:
    """check_conflict_has_axis_context_steward fires on an empty context."""
    bad = _wellformed_conflict(context="", id="C-manual")
    v = check_conflict_has_axis_context_steward(_graph_with(bad))
    assert any("context" in x.message for x in v)


def test_missing_steward_fires() -> None:
    """check_conflict_has_axis_context_steward fires on an empty steward."""
    bad = _wellformed_conflict(steward="")
    v = check_conflict_has_axis_context_steward(_graph_with(bad))
    assert any("steward" in x.message for x in v)


def test_single_member_fires() -> None:
    """check_conflict_min_two_members fires when a conflict has < 2 members."""
    bad = _wellformed_conflict(members=("R-1",))
    v = check_conflict_min_two_members(_graph_with(bad))
    assert any(x.target == bad.id for x in v)


def test_unknown_axis_fires() -> None:
    """check_axis_in_registry fires on an axis not in this domain's vocabulary."""
    bad = _wellformed_conflict(axis="totally-made-up-axis")
    v = check_axis_in_registry(_graph_with(bad))
    assert any("controlled vocabulary" in x.message for x in v)


def test_empty_axes_vocabulary_fires_on_any_axis() -> None:
    """A graph with NO axes declared fires check_axis_in_registry on every conflict.

    This is the structural enforcement that a content-free framework cannot
    silently accept conflicts: a domain must declare its axis vocabulary.
    """
    bad = _wellformed_conflict()
    g = TensionGraph(
        axes=(),  # no vocabulary declared at all
        stakeholders=(_S_OUT, _S_A, _S_B),
        requirements=(_req("R-1", "sa"), _req("R-2", "sb")),
        conflicts=(bad,),
    )
    v = check_axis_in_registry(g)
    assert any("controlled vocabulary" in x.message for x in v), (
        "an empty axes vocabulary must be flagged for every conflict that uses an axis"
    )


def test_id_mismatch_fires() -> None:
    """check_conflict_id_matches_identity fires when id != hash(axis, context)."""
    bad = _wellformed_conflict(id="C-deadbeef")  # forced wrong id
    v = check_conflict_id_matches_identity(_graph_with(bad))
    assert any(x.target == "C-deadbeef" for x in v)


def test_steward_is_member_owner_fires() -> None:
    """check_steward_not_a_member_owner fires when steward owns a member."""
    bad = _wellformed_conflict(steward="sa")
    v = check_steward_not_a_member_owner(_graph_with(bad))
    assert any(x.target == bad.id and "sa" in x.message for x in v)


def test_open_without_question_fires() -> None:
    """check_open_has_question fires on a bare 'OPEN' with no question."""
    reqs = (_req("R-1", "sa", status="OPEN"), _req("R-2", "sb"))
    g = _graph_with(_wellformed_conflict(), reqs=reqs)
    v = check_open_has_question(g)
    assert any(x.target == "R-1" for x in v)


def test_open_empty_parens_fires() -> None:
    """check_open_has_question fires on 'OPEN()' (empty question)."""
    reqs = (_req("R-1", "sa", status="OPEN()"), _req("R-2", "sb"))
    g = _graph_with(_wellformed_conflict(), reqs=reqs)
    v = check_open_has_question(g)
    assert any(x.target == "R-1" for x in v)


def test_open_with_question_does_not_fire() -> None:
    """check_open_has_question stays green on a proper OPEN(question)."""
    reqs = (_req("R-1", "sa", status="OPEN(which scope?)"), _req("R-2", "sb"))
    g = _graph_with(_wellformed_conflict(), reqs=reqs)
    assert holds(check_open_has_question(g))


def test_decided_without_justification_fires() -> None:
    """check_decided_has_rationale_or_derived fires on bare DECIDED, no rationale."""
    bad = _wellformed_conflict(lifecycle="DECIDED()", derived=())
    v = check_decided_has_rationale_or_derived(_graph_with(bad))
    assert any(x.target == bad.id for x in v)


def test_decided_with_derived_does_not_fire() -> None:
    """A DECIDED conflict justified by a derived requirement stays green."""
    reqs = (
        _req("R-1", "sa"),
        _req("R-2", "sb"),
        _req("R-3", "outsider", status="DRAFT"),
    )
    bad = _wellformed_conflict(lifecycle="DECIDED()", derived=("R-3",))
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=(_S_OUT, _S_A, _S_B),
        requirements=reqs,
        conflicts=(bad,),
    )
    assert holds(check_decided_has_rationale_or_derived(g))


def test_dead_assumption_owner_dangling_fires() -> None:
    """check_no_dangling_ids fires on an assumption with an unknown owner."""
    bad_assum = Assumption(id="A-x", statement="x", status=DEAD, owner="ghost_owner")
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=(_S_OUT, _S_A, _S_B),
        assumptions=(bad_assum,),
        requirements=(_req("R-1", "sa"), _req("R-2", "sb")),
        conflicts=(_wellformed_conflict(),),
    )
    v = check_no_dangling_ids(g)
    assert any(x.target == "A-x" and "ghost_owner" in x.message for x in v)


# ---------------------------------------------------------------------------
# 3. Empty graph is well-formed (legitimate ship state)
# ---------------------------------------------------------------------------


def test_empty_graph_is_wellformed() -> None:
    """A freshly-shipped framework (no content loaded) has no structural defects.

    "No content yet" is a legitimate state. The structural invariants must NOT
    fire on an empty graph — otherwise the framework would ship red.
    """
    g = TensionGraph()
    assert g.is_empty()
    assert holds(all_violations(g)), "empty framework graph must be well-formed"


# ---------------------------------------------------------------------------
# 4. Typed-anchor checks (check_typed_anchors) — anti-phantom + real content
# ---------------------------------------------------------------------------


def test_typed_anchors_fires_on_bad_requirement_id() -> None:
    """check_typed_anchors fires on a Requirement whose id lacks the 'R-' prefix."""
    bad_req = _req("foo", "sa")  # id "foo" has no R- prefix
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=(_S_OUT, _S_A, _S_B),
        requirements=(bad_req, _req("R-2", "sb")),
        conflicts=(),
    )
    v = check_typed_anchors(g)
    assert any(x.target == "foo" and "R-" in x.message for x in v), (
        "check_typed_anchors must fire on a Requirement with id 'foo'"
    )


def test_typed_anchors_fires_on_bad_assumption_id() -> None:
    """check_typed_anchors fires on an Assumption whose id lacks the 'A-' prefix."""
    from tensio.assumption import Assumption, HOLDS  # noqa: PLC0415

    bad_assum = Assumption(id="assum-x", statement="x", status=HOLDS, owner="sa")
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=(_S_OUT, _S_A, _S_B),
        assumptions=(bad_assum,),
        requirements=(_req("R1", "sa"), _req("R2", "sb")),
        conflicts=(),
    )
    v = check_typed_anchors(g)
    assert any(x.target == "assum-x" and "A-" in x.message for x in v), (
        "check_typed_anchors must fire on an Assumption with id 'assum-x'"
    )


def test_typed_anchors_fires_on_bad_conflict_id() -> None:
    """check_typed_anchors fires on a Conflict whose id lacks the 'C-' prefix."""
    bad = _wellformed_conflict(id="CONFLICT-hand-written")
    g = _graph_with(bad)
    v = check_typed_anchors(g)
    assert any(x.target == "CONFLICT-hand-written" and "C-" in x.message for x in v), (
        "check_typed_anchors must fire on a Conflict with id 'CONFLICT-hand-written'"
    )


def test_typed_anchors_passes_on_seed_fixture() -> None:
    """check_typed_anchors stays green on the well-formed seed fixture."""
    g = seed_graph()
    assert holds(check_typed_anchors(g)), (
        "seed fixture must pass check_typed_anchors — all ids use correct prefixes"
    )


# ---------------------------------------------------------------------------
# 5. Enforcement gradient (check_enforced_names_invariant)
# ---------------------------------------------------------------------------


def test_enforced_without_enforcer_fires() -> None:
    """check_enforced_names_invariant fires when enforcement=ENFORCED but enforced_by is empty."""
    reqs = (
        _req("R-1", "sa", enforcement=ENFORCED, enforced_by=()),
        _req("R-2", "sb"),
    )
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=(_S_OUT, _S_A, _S_B),
        requirements=reqs,
        conflicts=(),
    )
    v = check_enforced_names_invariant(g)
    assert any(x.target == "R-1" and "enforced_by is empty" in x.message for x in v), (
        "must fire on ENFORCED requirement with empty enforced_by"
    )


def test_invalid_enforcement_level_fires() -> None:
    """check_enforced_names_invariant fires on an unknown enforcement level."""
    reqs = (
        _req("R-1", "sa", enforcement="bogus"),
        _req("R-2", "sb"),
    )
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=(_S_OUT, _S_A, _S_B),
        requirements=reqs,
        conflicts=(),
    )
    v = check_enforced_names_invariant(g)
    assert any(x.target == "R-1" and "ENFORCEMENT_LEVELS" in x.message for x in v), (
        "must fire on unknown enforcement level"
    )


def test_settled_with_enforcer_passes() -> None:
    """check_enforced_names_invariant stays green when enforcement=ENFORCED with an enforcer named."""
    reqs = (
        _req("R-1", "sa", enforcement=ENFORCED, enforced_by=("check_foo",)),
        _req("R-2", "sb"),
    )
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=(_S_OUT, _S_A, _S_B),
        requirements=reqs,
        conflicts=(),
    )
    assert holds(check_enforced_names_invariant(g)), (
        "ENFORCED requirement with non-empty enforced_by must pass"
    )


# ---------------------------------------------------------------------------
# 6. M-tag format invariant (check_m_tag_format)
# ---------------------------------------------------------------------------


def test_m_tag_bad_format_fires() -> None:
    """check_m_tag_format fires on a badly-formatted m_tag (e.g. 'm17', 'M01', 'Mfoo')."""
    bad_cases = ["m17", "M01", "M", "Mfoo", "m1", "M00", "17"]
    for bad_tag in bad_cases:
        reqs = (
            _req("R-1", "sa", status="OPEN(question?)", m_tag=bad_tag),
            _req("R-2", "sb"),
        )
        g = TensionGraph(
            axes=DEMO_AXES,
            stakeholders=(_S_OUT, _S_A, _S_B),
            requirements=reqs,
            conflicts=(),
        )
        v = check_m_tag_format(g)
        assert any(x.target == "R-1" and "M[1-9]" in x.message for x in v), (
            f"check_m_tag_format must fire on m_tag={bad_tag!r}"
        )


def test_m_tag_duplicate_fires() -> None:
    """check_m_tag_format fires when two requirements share the same m_tag."""
    reqs = (
        _req("R-1", "sa", status="OPEN(q1?)", m_tag="M5"),
        _req("R-2", "sb", status="OPEN(q2?)", m_tag="M5"),
    )
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=(_S_OUT, _S_A, _S_B),
        requirements=reqs,
        conflicts=(),
    )
    v = check_m_tag_format(g)
    assert any("M5" in x.message and "unique" in x.message for x in v), (
        "check_m_tag_format must fire on duplicate m_tag M5"
    )


def test_m_tag_on_non_open_fires() -> None:
    """check_m_tag_format fires when m_tag appears on a non-OPEN requirement."""
    for bad_status in ("SETTLED", "DRAFT", "REJECTED"):
        reqs = (
            _req("R-1", "sa", status=bad_status, m_tag="M7"),
            _req("R-2", "sb"),
        )
        g = TensionGraph(
            axes=DEMO_AXES,
            stakeholders=(_S_OUT, _S_A, _S_B),
            requirements=reqs,
            conflicts=(),
        )
        v = check_m_tag_format(g)
        assert any(x.target == "R-1" and "non-OPEN" in x.message for x in v), (
            f"check_m_tag_format must fire on m_tag on status={bad_status!r}"
        )


def test_m_tag_valid_open_does_not_fire() -> None:
    """check_m_tag_format stays green on a properly-formatted m_tag on an OPEN req."""
    reqs = (
        _req("R-1", "sa", status="OPEN(which metric?)", m_tag="M17"),
        _req("R-2", "sb"),
    )
    g = TensionGraph(
        axes=DEMO_AXES,
        stakeholders=(_S_OUT, _S_A, _S_B),
        requirements=reqs,
        conflicts=(),
    )
    assert holds(check_m_tag_format(g)), (
        "check_m_tag_format must not fire on a valid m_tag='M17' on OPEN req"
    )
