"""Tests for Этап 5.2: timestamp fields + IMPLEMENTS-decay reflection predicate.

Guarantees:
  1. created_at/settled_at/decided_at default to "" on all three node types
     (legacy 290 nodes construct without breakage, no fabricated dates).
  2. reflect_implements_decay:
     - POSITIVE: old IMPLEMENTS assumption (created_at > 14 days) fires.
     - NEGATIVE: fresh IMPLEMENTS (created_at within threshold) is silent.
     - NEGATIVE: IMPLEMENTS with unknown created_at ("") is SILENT (no false noise).
     - NEGATIVE: HOLDS/UNCERTAIN/DEAD assumptions never fire.
     - decided_at (last transition) takes precedence over created_at and
       resets the decay clock.
  3. The new predicate is in REFLECTION_PREDICATES (registry integrity).
  4. The writer stamps created_at/settled_at on Requirement creation,
     created_at on Assumption creation, decided_at on transitions.
"""

from __future__ import annotations

from datetime import date as _date
from datetime import timedelta as _timedelta


import apply_proposal  # noqa: E402
from hotam_spec import reflection  # noqa: E402
from hotam_spec.assumption import (  # noqa: E402
    DEAD,
    HOLDS,
    IMPLEMENTS,
    UNCERTAIN,
    Assumption,
)
from hotam_spec.axis import Axis  # noqa: E402
from hotam_spec.conflict import Conflict  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.proposal import ProposedAssumption, ProposedRequirement  # noqa: E402
from hotam_spec.requirement import DRAFT, Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

_DUMMY_AXES = (
    Axis(slug="ax-one", description="test axis one"),
)
_SH = (
    Stakeholder(id="s-a", name="A", domain="x"),
    Stakeholder(id="s-b", name="B", domain="y"),
    Stakeholder(id="s-c", name="C", domain="z"),
)


def _implements_assumption(
    aid: str, created_at: str = "", decided_at: str = ""
) -> Assumption:
    return Assumption(
        id=aid,
        statement=f"striving {aid}",
        status=IMPLEMENTS,
        owner="s-a",
        created_at=created_at,
        decided_at=decided_at,
    )


# ---------------------------------------------------------------------------
# 1. Timestamp fields default to "" (regression-neutrality for legacy nodes)
# ---------------------------------------------------------------------------


def test_requirement_timestamps_default_empty() -> None:
    """created_at/settled_at default to "" — legacy nodes have no fabricated date."""
    r = Requirement(
        id="R-x", claim="x", owner="s-a", status=DRAFT
    )
    assert r.created_at == ""
    assert r.settled_at == ""


def test_conflict_timestamps_default_empty() -> None:
    """created_at/decided_at default to "" on Conflict."""
    c = Conflict(
        id="C-deadbeef",
        axis="ax-one",
        context="ctx",
        members=("R-a", "R-b"),
        steward="s-c",
        lifecycle="DETECTED",
    )
    assert c.created_at == ""
    assert c.decided_at == ""


def test_assumption_timestamps_default_empty() -> None:
    """created_at/decided_at default to "" on Assumption."""
    a = Assumption(
        id="A-x", statement="x", status=HOLDS, owner="s-a"
    )
    assert a.created_at == ""
    assert a.decided_at == ""


# ---------------------------------------------------------------------------
# 2. reflect_implements_decay — POSITIVE
# ---------------------------------------------------------------------------


def test_reflect_implements_decay_fires_for_old() -> None:
    """An IMPLEMENTS aspiration older than the threshold fires the decay signal."""
    old_date = (_date.today() - _timedelta(days=30)).isoformat()
    a = _implements_assumption("A-old-striving", created_at=old_date)
    g = TensionGraph(
        axes=_DUMMY_AXES, stakeholders=_SH, assumptions=(a,)
    )
    found = reflection.reflect_implements_decay(g)
    assert len(found) == 1
    assert found[0].condition == "reflect_implements_decay"
    assert found[0].target == "A-old-striving"
    assert "30 days old" in found[0].imperative
    assert "re-affirm" in found[0].imperative


def test_reflect_implements_decay_fires_just_over_threshold() -> None:
    """15 days old (> 14 threshold) fires."""
    just_over = (_date.today() - _timedelta(days=15)).isoformat()
    a = _implements_assumption("A-fifteen", created_at=just_over)
    g = TensionGraph(
        axes=_DUMMY_AXES, stakeholders=_SH, assumptions=(a,)
    )
    found = reflection.reflect_implements_decay(g)
    assert len(found) == 1


# ---------------------------------------------------------------------------
# 2b. reflect_implements_decay — NEGATIVE (silent cases)
# ---------------------------------------------------------------------------


def test_reflect_implements_decay_silent_for_fresh() -> None:
    """A fresh IMPLEMENTS (within threshold) does NOT fire."""
    fresh = (_date.today() - _timedelta(days=5)).isoformat()
    a = _implements_assumption("A-fresh", created_at=fresh)
    g = TensionGraph(
        axes=_DUMMY_AXES, stakeholders=_SH, assumptions=(a,)
    )
    found = reflection.reflect_implements_decay(g)
    assert found == [], f"fresh IMPLEMENTS must not fire; got {found}"


def test_reflect_implements_decay_silent_for_unknown_date() -> None:
    """An IMPLEMENTS with unknown created_at (legacy node) does NOT fire.

    This is the critical regression- neutrality guarantee: the ~290 pre-timestamp
    nodes must NOT produce false noise. No date = no age = no signal.
    """
    a = _implements_assumption("A-legacy-no-date")  # created_at="", decided_at=""
    g = TensionGraph(
        axes=_DUMMY_AXES, stakeholders=_SH, assumptions=(a,)
    )
    found = reflection.reflect_implements_decay(g)
    assert found == [], (
        "IMPLEMENTS with unknown date must NOT fire (no false noise on legacy nodes)"
    )


def test_reflect_implements_decay_silent_for_non_implements() -> None:
    """HOLDS/UNCERTAIN/DEAD assumptions never fire the IMPLEMENTS-decay predicate."""
    for status in (HOLDS, UNCERTAIN, DEAD):
        a = Assumption(
            id=f"A-{status.lower()}",
            statement=f"{status} assumption",
            status=status,
            owner="s-a",
            created_at="2020-01-01",  # very old, but wrong status
        )
        g = TensionGraph(
            axes=_DUMMY_AXES, stakeholders=_SH, assumptions=(a,)
        )
        found = reflection.reflect_implements_decay(g)
        assert found == [], (
            f"{status} assumption must not fire implements-decay; got {found}"
        )


def test_reflect_implements_decay_decided_at_resets_clock() -> None:
    """decided_at (last transition) takes precedence and resets the decay clock.

    An assumption with an OLD created_at but a RECENT decided_at (re-typed to
    IMPLEMENTS recently) must NOT fire — the clock was reset by the transition.
    """
    old_created = (_date.today() - _timedelta(days=100)).isoformat()
    recent_decided = (_date.today() - _timedelta(days=3)).isoformat()
    a = _implements_assumption(
        "A-reset", created_at=old_created, decided_at=recent_decided
    )
    g = TensionGraph(
        axes=_DUMMY_AXES, stakeholders=_SH, assumptions=(a,)
    )
    found = reflection.reflect_implements_decay(g)
    assert found == [], (
        "decided_at resets the decay clock; recent transition must not fire"
    )


def test_reflect_implements_decay_decided_at_old_fires() -> None:
    """An OLD decided_at (transition into IMPLEMENTS long ago) fires."""
    old_decided = (_date.today() - _timedelta(days=60)).isoformat()
    a = _implements_assumption(
        "A-old-transition", created_at="", decided_at=old_decided
    )
    g = TensionGraph(
        axes=_DUMMY_AXES, stakeholders=_SH, assumptions=(a,)
    )
    found = reflection.reflect_implements_decay(g)
    assert len(found) == 1
    assert "60 days old" in found[0].imperative


# ---------------------------------------------------------------------------
# 3. Registry integrity
# ---------------------------------------------------------------------------


def test_reflect_implements_decay_in_registry() -> None:
    """The new predicate is registered in REFLECTION_PREDICATES."""
    names = [fn.__name__ for fn in reflection.REFLECTION_PREDICATES]
    assert "reflect_implements_decay" in names


def test_reflect_implements_decay_via_all_findings() -> None:
    """all_findings() includes the decay predicate's output."""
    old_date = (_date.today() - _timedelta(days=30)).isoformat()
    a = _implements_assumption("A-all-findings", created_at=old_date)
    g = TensionGraph(
        axes=_DUMMY_AXES, stakeholders=_SH, assumptions=(a,)
    )
    findings = reflection.all_findings(g)
    decay = [f for f in findings if f.condition == "reflect_implements_decay"]
    assert len(decay) == 1
    assert decay[0].target == "A-all-findings"


# ---------------------------------------------------------------------------
# 4. Writer stamps created_at/settled_at/decided_at (apply_proposal.py)
# ---------------------------------------------------------------------------

_SAMPLE_GRAPH_SOURCE = '''\
from __future__ import annotations

from hotam_spec.assumption import Assumption, HOLDS
from hotam_spec.requirement import Requirement


def build_graph():
    requirements = (
        Requirement(
            id="R-existing",
            claim="existing",
            owner="framework-author",
            status="SETTLED",
        ),
    )

    assumptions = (
        Assumption(
            id="A-existing",
            statement="Something already believed.",
            status=HOLDS,
            owner="framework-author",
        ),
    )
'''


def test_writer_stamps_created_at_on_new_requirement() -> None:
    """_render_requirement_source emits created_at= with today's ISO date."""
    proposal = ProposedRequirement(
        id="R-new-stamped",
        claim="a new requirement",
        owner="framework-author",
        status="DRAFT",
        why="testing",
    )
    src = apply_proposal._render_requirement_source(proposal, "        ")
    today = _date.today().isoformat()
    assert f'created_at="{today}"' in src, (
        f"writer must stamp created_at with today's date; source:\n{src}"
    )


def test_writer_stamps_settled_at_on_settled_requirement() -> None:
    """A SETTLED requirement gets BOTH created_at and settled_at stamped."""
    proposal = ProposedRequirement(
        id="R-settled-new",
        claim="a settled requirement",
        owner="framework-author",
        status="SETTLED",
        why="testing",
    )
    src = apply_proposal._render_requirement_source(proposal, "        ")
    today = _date.today().isoformat()
    assert f'created_at="{today}"' in src
    assert f'settled_at="{today}"' in src


def test_writer_does_not_stamp_settled_at_on_draft() -> None:
    """A DRAFT requirement gets created_at but NOT settled_at."""
    proposal = ProposedRequirement(
        id="R-draft-new",
        claim="a draft",
        owner="framework-author",
        status="DRAFT",
        why="testing",
    )
    src = apply_proposal._render_requirement_source(proposal, "        ")
    assert "settled_at=" not in src, (
        f"DRAFT must not get settled_at; source:\n{src}"
    )


def test_writer_respects_explicit_created_at() -> None:
    """If the proposal supplies created_at, the writer uses it verbatim."""
    proposal = ProposedRequirement(
        id="R-explicit",
        claim="explicit date",
        owner="framework-author",
        status="DRAFT",
        why="testing",
        created_at="2025-01-15",
    )
    src = apply_proposal._render_requirement_source(proposal, "        ")
    assert 'created_at="2025-01-15"' in src
    today = _date.today().isoformat()
    assert f'created_at="{today}"' not in src


def test_writer_stamps_created_at_on_new_assumption() -> None:
    """_render_assumption_source emits created_at= with today's ISO date."""
    proposal = ProposedAssumption(
        id="A-new-stamped",
        statement="a new assumption",
        status="HOLDS",
        owner="framework-author",
    )
    src = apply_proposal._render_assumption_source(proposal, "        ")
    today = _date.today().isoformat()
    assert f'created_at="{today}"' in src, (
        f"writer must stamp created_at; source:\n{src}"
    )


def test_writer_stamps_settled_at_on_requirement_update_to_settled() -> None:
    """Updating a requirement's status to SETTLED stamps settled_at."""
    proposal = ProposedRequirement(
        id="R-existing",
        claim="existing",
        owner="framework-author",
        status="SETTLED",
        why="testing",
    )
    result = apply_proposal._apply_requirement_to_source(
        _SAMPLE_GRAPH_SOURCE, proposal
    )
    today = _date.today().isoformat()
    assert f'settled_at="{today}"' in result, (
        f"UPDATE to SETTLED must stamp settled_at; result:\n{result}"
    )
    import ast
    ast.parse(result)
