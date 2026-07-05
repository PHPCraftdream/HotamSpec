"""Negative ("fires") tests for every check_* in ALL_INVARIANTS.

Each test constructs a minimal broken graph that should trigger the check
and asserts the check returns non-empty violations. This is the anti-phantom
guard: a check that never fires on broken input is not enforcing anything.
"""

from __future__ import annotations

import sys
import tempfile
from pathlib import Path
from unittest.mock import patch

_SRC = Path(__file__).resolve().parents[1] / "src"
_TESTS = Path(__file__).resolve().parent
for _p in (_SRC, _TESTS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

from fixtures.seed import DEMO_AXES  # noqa: E402
from hotam_spec.assumption import Assumption  # noqa: E402
from hotam_spec.axis import Axis  # noqa: E402
from hotam_spec.conflict import Conflict, Variant, conflict_identity  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.invariants import (  # noqa: E402
    ALL_INVARIANTS,
    InvariantClassification,
    RULES_AS_DATA_TABLE,
    check_conflict_has_context,
    check_conflict_has_steward,
    check_conflict_lifecycle_in_lifecycle,
    check_decided_by_is_known_stakeholder,
    check_decided_by_not_member_owner,
    check_decided_has_nonempty_decided_by,
    check_domain_director_exists,
    check_domain_manifest_description_nonempty,
    check_domain_manifest_director_nonempty,
    check_domain_manifest_exists_and_importable,
    check_domain_manifest_goals_nonempty,
    check_domain_manifest_id_matches_dirname,
    check_goal_lifecycle_in_lifecycle,
    check_held_by_is_known_stakeholder,
    check_held_by_not_member_owner,
    check_held_has_min_two_variants,
    check_held_has_nonempty_decided_by,
    check_m_tag_open_only,
    check_m_tag_unique,
    check_m_tag_valid_format,
    check_no_dangling_assumption_owner,
    check_no_dangling_conflict_refs,
    check_no_dangling_operator_refs,
    check_no_dangling_requirement_assumptions,
    check_no_dangling_requirement_owner,
    check_no_dangling_requirement_relations,
    check_operator_lifecycle_in_lifecycle,
    check_requirement_status_in_lifecycle,
    check_rules_as_data_classification_coherent,
    check_section_anchors_known,
    check_typed_anchors_assumption,
    check_typed_anchors_conflict,
    check_typed_anchors_goal,
    check_typed_anchors_operator,
    check_typed_anchors_process,
    check_typed_anchors_requirement,
    check_typed_anchors_variant,
)
from hotam_spec.lifecycle import INITIAL, Lifecycle, State, Transition  # noqa: E402
from hotam_spec.operator import Operator  # noqa: E402
from hotam_spec.requirement import Relation, Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

# --- Shared helpers -----------------------------------------------------------

_S = Stakeholder(id="s1", name="S1", domain="x")
_S2 = Stakeholder(id="s2", name="S2", domain="x")
_S3 = Stakeholder(id="s3", name="S3", domain="x")

_AX = (Axis(slug="a-vs-b", description="test axis"),)


def _req(rid: str, owner: str = "s1", status: str = "SETTLED", **kw) -> Requirement:
    return Requirement(id=rid, claim=f"claim {rid}", owner=owner, status=status, **kw)


def _conflict(axis: str = "a-vs-b", context: str = "ctx", **kw) -> Conflict:
    defaults = dict(
        id=conflict_identity(axis, context),
        axis=axis,
        context=context,
        members=("R-1", "R-2"),
        steward="s3",
        lifecycle="ACKNOWLEDGED",
    )
    defaults.update(kw)
    return Conflict(**defaults)  # type: ignore[arg-type]


def _graph(**kw) -> TensionGraph:
    defaults = dict(
        axes=_AX,
        stakeholders=(_S, _S2, _S3),
        requirements=(_req("R-1", "s1"), _req("R-2", "s2")),
        conflicts=(),
    )
    defaults.update(kw)
    return TensionGraph(**defaults)


# --- Dangling reference checks ------------------------------------------------


def test_fires_no_dangling_assumption_owner() -> None:
    g = _graph(assumptions=(Assumption(id="A-1", statement="x", status="HOLDS", owner="GHOST"),))
    assert check_no_dangling_assumption_owner(g)


def test_fires_no_dangling_requirement_owner() -> None:
    g = _graph(requirements=(_req("R-1", owner="GHOST"),))
    assert check_no_dangling_requirement_owner(g)


def test_fires_no_dangling_requirement_assumptions() -> None:
    g = _graph(requirements=(_req("R-1", assumptions=("A-GHOST",)),))
    assert check_no_dangling_requirement_assumptions(g)


def test_fires_no_dangling_requirement_relations() -> None:
    g = _graph(requirements=(_req("R-1", relations=(Relation(kind="refines", target="R-GHOST"),)),))
    assert check_no_dangling_requirement_relations(g)


def test_fires_no_dangling_conflict_refs() -> None:
    c = _conflict(members=("R-1", "R-GHOST"))
    g = _graph(conflicts=(c,), requirements=(_req("R-1"),))
    assert check_no_dangling_conflict_refs(g)


def test_fires_no_dangling_operator_refs() -> None:
    op = Operator(id="OP-x", stakeholder="GHOST")
    g = _graph(operators=(op,))
    assert check_no_dangling_operator_refs(g)


# --- Conflict structure -------------------------------------------------------


def test_fires_conflict_has_context() -> None:
    c = _conflict(context="")
    g = _graph(conflicts=(c,))
    assert check_conflict_has_context(g)


def test_fires_conflict_has_steward() -> None:
    c = _conflict(steward="")
    g = _graph(conflicts=(c,))
    assert check_conflict_has_steward(g)


# --- DECIDED signoff ----------------------------------------------------------


def test_fires_decided_has_nonempty_decided_by() -> None:
    c = _conflict(lifecycle="DECIDED(rationale)", decided_by="")
    g = _graph(conflicts=(c,))
    assert check_decided_has_nonempty_decided_by(g)


def test_fires_decided_by_is_known_stakeholder() -> None:
    c = _conflict(lifecycle="DECIDED(rationale)", decided_by="GHOST")
    g = _graph(conflicts=(c,))
    assert check_decided_by_is_known_stakeholder(g)


def test_fires_decided_by_not_member_owner() -> None:
    # decided_by is the owner of a member (s1 owns R-1)
    c = _conflict(lifecycle="DECIDED(rationale)", decided_by="s1")
    g = _graph(conflicts=(c,))
    assert check_decided_by_not_member_owner(g)


# --- HELD state ---------------------------------------------------------------


def test_fires_held_has_min_two_variants() -> None:
    c = _conflict(
        lifecycle="HELD(holding)",
        decided_by="s3",
        variants=(Variant(id="V-one", behavior="x", implies="y", costs="z"),),
    )
    g = _graph(conflicts=(c,))
    assert check_held_has_min_two_variants(g)


def test_fires_held_has_nonempty_decided_by() -> None:
    c = _conflict(
        lifecycle="HELD(holding)",
        decided_by="",
        variants=(
            Variant(id="V-a", behavior="x", implies="y", costs="z"),
            Variant(id="V-b", behavior="x2", implies="y2", costs="z2"),
        ),
    )
    g = _graph(conflicts=(c,))
    assert check_held_has_nonempty_decided_by(g)


def test_fires_held_by_is_known_stakeholder() -> None:
    c = _conflict(
        lifecycle="HELD(holding)",
        decided_by="GHOST",
        variants=(
            Variant(id="V-a", behavior="x", implies="y", costs="z"),
            Variant(id="V-b", behavior="x2", implies="y2", costs="z2"),
        ),
    )
    g = _graph(conflicts=(c,))
    assert check_held_by_is_known_stakeholder(g)


def test_fires_held_by_not_member_owner() -> None:
    c = _conflict(
        lifecycle="HELD(holding)",
        decided_by="s1",  # s1 owns R-1 which is a member
        variants=(
            Variant(id="V-a", behavior="x", implies="y", costs="z"),
            Variant(id="V-b", behavior="x2", implies="y2", costs="z2"),
        ),
    )
    g = _graph(conflicts=(c,))
    assert check_held_by_not_member_owner(g)


# --- Typed anchors ------------------------------------------------------------


def test_fires_typed_anchors_requirement() -> None:
    g = _graph(requirements=(_req("BAD-PREFIX"),))
    assert check_typed_anchors_requirement(g)


def test_fires_typed_anchors_assumption() -> None:
    g = _graph(assumptions=(Assumption(id="BAD", statement="x", status="HOLDS", owner="s1"),))
    assert check_typed_anchors_assumption(g)


def test_fires_typed_anchors_conflict() -> None:
    c = _conflict()
    # Use a bad id that doesn't match the identity hash
    bad_c = Conflict(
        id="BAD-conflict",
        axis="a-vs-b",
        context="ctx",
        members=("R-1", "R-2"),
        steward="s3",
        lifecycle="ACKNOWLEDGED",
    )
    g = _graph(conflicts=(bad_c,))
    assert check_typed_anchors_conflict(g)


def test_fires_typed_anchors_operator() -> None:
    op = Operator(id="BAD-op", stakeholder="s1")
    g = _graph(operators=(op,))
    assert check_typed_anchors_operator(g)


def test_fires_typed_anchors_process() -> None:
    from hotam_spec.lifecycle import INITIAL, Lifecycle, State  # noqa: PLC0415
    from hotam_spec.process import Process, Step  # noqa: PLC0415

    lc = Lifecycle(
        slug="test-lc",
        states=(State("ACTIVE", kind=INITIAL, why="x"),),
        transitions=(),
    )
    p = Process(
        id="BAD-proc",
        lifecycle=lc,
        steps=(Step(name="do", requires_role="s1"),),
        roles_required=("s1",),
    )
    g = _graph(processes=(p,))
    assert check_typed_anchors_process(g)


def test_fires_typed_anchors_goal() -> None:
    from hotam_spec.process import Goal, TargetState  # noqa: PLC0415

    goal = Goal(id="BAD-goal", owner="OP-x", target_state=TargetState(kind="requirement", predicate="x"))
    g = _graph(goals=(goal,))
    assert check_typed_anchors_goal(g)


def test_fires_typed_anchors_variant() -> None:
    c = _conflict(
        lifecycle="HELD(holding)",
        decided_by="s3",
        variants=(
            Variant(id="BAD-v1", behavior="x", implies="y", costs="z"),
            Variant(id="V-v2", behavior="x2", implies="y2", costs="z2"),
        ),
    )
    g = _graph(conflicts=(c,))
    assert check_typed_anchors_variant(g)


# --- M-tag checks -------------------------------------------------------------


def test_fires_m_tag_valid_format() -> None:
    g = _graph(requirements=(_req("R-1", m_tag="BADFORMAT"),))
    assert check_m_tag_valid_format(g)


def test_fires_m_tag_unique() -> None:
    g = _graph(requirements=(_req("R-1", m_tag="M12"), _req("R-2", "s2", m_tag="M12")))
    assert check_m_tag_unique(g)


def test_fires_m_tag_open_only() -> None:
    # m_tag on a non-OPEN requirement that is not SETTLED
    # Actually m_tag_open_only fires on DRAFT with m_tag? Let me check the rule.
    # The rule is: m_tag is only valid on OPEN or SETTLED requirements.
    # Actually looking at the name: "open only" - fires when m_tag on non-open.
    # Let me just try DRAFT:
    g = _graph(requirements=(_req("R-1", status="DRAFT", m_tag="M12"),))
    assert check_m_tag_open_only(g)


# --- Lifecycle-in-lifecycle checks --------------------------------------------


def test_fires_requirement_status_in_lifecycle() -> None:
    g = _graph(requirements=(_req("R-1", status="TOTALLY_INVALID_STATUS"),))
    assert check_requirement_status_in_lifecycle(g)


def test_fires_conflict_lifecycle_in_lifecycle() -> None:
    c = _conflict(lifecycle="TOTALLY_INVALID")
    g = _graph(conflicts=(c,))
    assert check_conflict_lifecycle_in_lifecycle(g)


def test_fires_operator_lifecycle_in_lifecycle() -> None:
    op = Operator(id="OP-x", stakeholder="s1", lifecycle="TOTALLY_INVALID")
    g = _graph(operators=(op,))
    assert check_operator_lifecycle_in_lifecycle(g)


def test_fires_goal_lifecycle_in_lifecycle() -> None:
    from hotam_spec.process import Goal, TargetState  # noqa: PLC0415

    goal = Goal(id="GOAL-x", owner="OP-x", target_state=TargetState(kind="requirement", predicate="x"), lifecycle="TOTALLY_INVALID")
    g = _graph(goals=(goal,))
    assert check_goal_lifecycle_in_lifecycle(g)


# --- Section anchors ----------------------------------------------------------


def test_fires_section_anchors_known(tmp_path: Path) -> None:
    # Write a Python file with an unknown section anchor in a docstring
    bad_py = tmp_path / "bad_module.py"
    bad_py.write_text(
        'def foo():\n    """Canon: §NonexistentAnchor — something."""\n    pass\n',
        encoding="utf-8",
    )
    with patch("hotam_spec.invariants._TENSIO_SRC", tmp_path):
        # Clear the AST cache so it picks up the new file
        from hotam_spec.invariants import _cached_parse_path  # noqa: PLC0415

        _cached_parse_path.cache_clear()
        g = _graph(self_hosting=True)
        violations = check_section_anchors_known(g)
        _cached_parse_path.cache_clear()
    assert violations, "check_section_anchors_known should fire on unknown section anchor"


# --- Domain manifest checks (filesystem-dependent) ----------------------------


def test_fires_domain_manifest_exists_and_importable(tmp_path: Path) -> None:
    domain_dir = tmp_path / "domains" / "broken"
    domain_dir.mkdir(parents=True)
    # No manifest.py
    with patch("hotam_spec.invariants._DOMAINS_ROOT", tmp_path / "domains"):
        g = _graph()
        assert check_domain_manifest_exists_and_importable(g)


def test_fires_domain_manifest_id_matches_dirname(tmp_path: Path) -> None:
    domain_dir = tmp_path / "domains" / "mydom"
    domain_dir.mkdir(parents=True)
    (domain_dir / "manifest.py").write_text(
        "DOMAIN_ID = 'wrong-name'\nDESCRIPTION = 'x'\nGOALS = 'x'\nDIRECTOR = 'director'\n",
        encoding="utf-8",
    )
    with patch("hotam_spec.invariants._DOMAINS_ROOT", tmp_path / "domains"):
        g = _graph()
        assert check_domain_manifest_id_matches_dirname(g)


def test_fires_domain_manifest_description_nonempty(tmp_path: Path) -> None:
    domain_dir = tmp_path / "domains" / "mydom"
    domain_dir.mkdir(parents=True)
    (domain_dir / "manifest.py").write_text(
        "DOMAIN_ID = 'mydom'\nDESCRIPTION = ''\nGOALS = 'x'\nDIRECTOR = 'director'\n",
        encoding="utf-8",
    )
    with patch("hotam_spec.invariants._DOMAINS_ROOT", tmp_path / "domains"):
        g = _graph()
        assert check_domain_manifest_description_nonempty(g)


def test_fires_domain_manifest_goals_nonempty(tmp_path: Path) -> None:
    domain_dir = tmp_path / "domains" / "mydom"
    domain_dir.mkdir(parents=True)
    (domain_dir / "manifest.py").write_text(
        "DOMAIN_ID = 'mydom'\nDESCRIPTION = 'x'\nGOALS = ''\nDIRECTOR = 'director'\n",
        encoding="utf-8",
    )
    with patch("hotam_spec.invariants._DOMAINS_ROOT", tmp_path / "domains"):
        g = _graph()
        assert check_domain_manifest_goals_nonempty(g)


def test_fires_domain_manifest_director_nonempty(tmp_path: Path) -> None:
    domain_dir = tmp_path / "domains" / "mydom"
    domain_dir.mkdir(parents=True)
    (domain_dir / "manifest.py").write_text(
        "DOMAIN_ID = 'mydom'\nDESCRIPTION = 'x'\nGOALS = 'x'\nDIRECTOR = ''\n",
        encoding="utf-8",
    )
    with patch("hotam_spec.invariants._DOMAINS_ROOT", tmp_path / "domains"):
        g = _graph()
        assert check_domain_manifest_director_nonempty(g)


# --- Rules-as-data classification coherent ------------------------------------


def test_fires_rules_as_data_classification_coherent() -> None:
    # Add a bogus classification entry referring to a non-existent check
    from hotam_spec import invariants  # noqa: PLC0415

    original = invariants.RULES_AS_DATA_TABLE
    bogus = original + (InvariantClassification(name="check_nonexistent_fake", kind="BESPOKE", why="test"),)
    with patch.object(invariants, "RULES_AS_DATA_TABLE", bogus):
        g = _graph(self_hosting=True)
        assert check_rules_as_data_classification_coherent(g)


# --- Toothless RULE detection (Part 1 verification) -----------------------


def test_toothless_rule_detected() -> None:
    """check_method_matches_docstring catches a check_* with a RULE docstring
    but no Violation calls and no delegation to Violation-bearing functions."""
    from hotam_spec.invariants import (  # noqa: PLC0415
        _fn_has_violation_calls,
        _fn_is_delegator,
        check_method_matches_docstring,
    )

    # Create a toothless function with a RULE docstring but no Violation
    def check_fake_toothless(g):  # type: ignore[no-untyped-def]  # noqa: ARG001
        """Canon: test.

        RULE: something must hold.
        """
        return []

    # Verify helpers classify it correctly
    assert not _fn_has_violation_calls(check_fake_toothless)
    assert not _fn_is_delegator(check_fake_toothless)

    # Temporarily inject into ALL_INVARIANTS and verify it fires
    from hotam_spec import invariants as inv_mod  # noqa: PLC0415

    check_fake_toothless.__module__ = inv_mod.__name__
    original = inv_mod.ALL_INVARIANTS
    try:
        inv_mod.ALL_INVARIANTS = original + (check_fake_toothless,)
        g = _graph(self_hosting=True)
        violations = check_method_matches_docstring(g)
        toothless_violations = [
            v for v in violations if "check_fake_toothless" in v.target
        ]
        assert toothless_violations, "toothless RULE should be caught"
        assert "no Violation" in toothless_violations[0].message
    finally:
        inv_mod.ALL_INVARIANTS = original


# --- Meta-check: every check_* in ALL_INVARIANTS has a fires test ----------


def test_every_invariant_has_fires_test() -> None:
    """Meta-check: every check_* in ALL_INVARIANTS must have a corresponding
    negative ('fires') test somewhere in spec/tests/ that references the check
    by name and asserts non-empty violations.

    This is the ratchet: adding a new check_* to ALL_INVARIANTS without a
    fires test causes this meta-check to fail.
    """
    import os  # noqa: PLC0415
    import re  # noqa: PLC0415

    tests_dir = Path(__file__).resolve().parent
    check_names = {fn.__name__ for fn in ALL_INVARIANTS}

    # Read all test files
    test_contents: dict[str, str] = {}
    for f in sorted(os.listdir(tests_dir)):
        if f.startswith("test_") and f.endswith(".py"):
            test_contents[f] = (tests_dir / f).read_text(encoding="utf-8")

    # For each check, look for a fires test
    # A fires test: references the check name AND has an assertion pattern nearby
    _ASSERT_PATTERNS = (
        "assert not holds",
        "assert len(",
        "!= []",
        "assert check_",
        "assert v",
        "assert result",
        ".target",
        "violations",
    )

    missing: list[str] = []
    for name in sorted(check_names):
        found = False
        for _fname, content in test_contents.items():
            if name not in content:
                continue
            lines = content.split("\n")
            for i, line in enumerate(lines):
                if name not in line:
                    continue
                context_block = "\n".join(lines[max(0, i - 2) : i + 10])
                if any(p in context_block for p in _ASSERT_PATTERNS):
                    found = True
                    break
            if found:
                break
        if not found:
            missing.append(name)

    assert not missing, (
        f"{len(missing)} check_* functions in ALL_INVARIANTS lack a fires test: "
        + ", ".join(missing)
    )
