"""Tests for the enforceability kind (ENFORCEABLE | INHERENTLY_PROSE).

Covers:
  1. check_enforceability_kind_known fires on an invalid enforceability value.
  2. check_enforceability_kind_known is clean on both valid values.
  3. The debt-counting predicate (Requirement.is_closeable_debt) excludes
     INHERENTLY_PROSE requirements from the enforcement-gradient debt count.
  4. docs/gen/UNENFORCED.md separates closeable debt from inherent discipline.
"""

from __future__ import annotations


from fixtures.seed import DEMO_AXES  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.invariants import check_enforceability_kind_known  # noqa: E402
from hotam_spec.requirement import (  # noqa: E402
    ENFORCEABLE,
    INHERENTLY_PROSE,
    PROSE,
    Requirement,
)
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

_S_A = Stakeholder(id="sa", name="A", domain="x")
_S_B = Stakeholder(id="sb", name="B", domain="x")


def _req(rid: str, owner: str, **kw) -> Requirement:
    return Requirement(
        id=rid, claim=f"claim {rid}", owner=owner, status="SETTLED", **kw
    )


def test_enforceability_kind_known_fires_on_invalid_value() -> None:
    """check_enforceability_kind_known fires when enforceability is not a known kind."""
    reqs = (_req("R-1", "sa", enforceability="bogus"),)
    g = TensionGraph(
        axes=DEMO_AXES, stakeholders=(_S_A, _S_B), requirements=reqs, conflicts=()
    )
    v = check_enforceability_kind_known(g)
    assert any(x.target == "R-1" and "ENFORCEABILITY_KINDS" in x.message for x in v), (
        "check_enforceability_kind_known must fire on an unknown enforceability value"
    )


def test_enforceability_kind_known_clean_on_valid_values() -> None:
    """check_enforceability_kind_known is clean for both ENFORCEABLE and INHERENTLY_PROSE."""
    reqs = (
        _req("R-1", "sa", enforceability=ENFORCEABLE),
        _req("R-2", "sb", enforceability=INHERENTLY_PROSE),
    )
    g = TensionGraph(
        axes=DEMO_AXES, stakeholders=(_S_A, _S_B), requirements=reqs, conflicts=()
    )
    assert check_enforceability_kind_known(g) == []


def test_debt_count_excludes_inherently_prose() -> None:
    """The closeable-debt predicate counts only ENFORCEABLE, non-ENFORCED requirements."""
    reqs = (
        _req("R-1", "sa", enforcement=PROSE, enforceability=ENFORCEABLE),
        _req("R-2", "sb", enforcement=PROSE, enforceability=INHERENTLY_PROSE),
    )
    debt = [r for r in reqs if r.is_closeable_debt()]
    assert [r.id for r in debt] == ["R-1"], (
        "is_closeable_debt() must exclude INHERENTLY_PROSE requirements from the "
        "enforcement-gradient debt count"
    )


def test_unenforced_md_separates_categories() -> None:
    """docs/gen/UNENFORCED.md contains both the closeable-debt and inherent-discipline
    section headers after gen_spec regeneration."""
    import gen_spec

    g = gen_spec.load_content_graph()
    rendered = gen_spec.build_unenforced(g)
    assert "## Closeable debt (ENFORCEABLE, no enforcer yet)" in rendered
    assert (
        "## Inherent discipline (INHERENTLY_PROSE — not debt, permanent by design)"
        in rendered
    )
