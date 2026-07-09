"""Tests for §Signoff — the frozen provenance record (Wave: Signoff-рекорд).

Covers:
  1. Signoff dataclass: frozen, defaults, equality.
  2. Conflict.signoff / Assumption.signoff: default None, frozen-compatible.
  3. ConflictTransition writer: signoff attached on DECIDED/HELD, NOT on ACKNOWLEDGED.
  4. ConflictTransition writer: variants PRESERVED on HELD->DECIDED (K2(b) fix).
  5. ConflictTransition writer: chosen_variant lands in signoff.
  6. AssumptionTransition writer: signoff attached with decided_by (K2(a) fix).
  7. Signoff renders through _python_repr round-trippably.
  8. Live graph: C-be22cdd1 carries restored variants + signoff after migration.
"""

from __future__ import annotations

import ast

import apply_proposal
from hotam_spec.assumption import Assumption, HOLDS, DEAD
from hotam_spec.conflict import (
    Conflict,
    Variant,
    conflict_identity,
)
from hotam_spec.proposal import (  # noqa: E402
    ProposedAssumptionTransition,
    ProposedConflictTransition,
)
from hotam_spec.signoff import Signoff  # noqa: E402


# ---------------------------------------------------------------------------
# 1. Signoff dataclass
# ---------------------------------------------------------------------------


def test_signoff_frozen_defaults() -> None:
    """Signoff is frozen with sensible defaults for optional fields."""
    s = Signoff(decided_by="domain-user", date="2026-07-06")
    assert s.decided_by == "domain-user"
    assert s.date == "2026-07-06"
    assert s.verbatim == ""
    assert s.instrument == "personal"
    assert s.chosen_variant == ""


def test_signoff_frozen_immutable() -> None:
    """Signoff is a frozen dataclass — mutation raises FrozenInstanceError."""
    s = Signoff(decided_by="x", date="2026-07-06")
    try:
        s.decided_by = "y"  # type: ignore[misc]
    except Exception:
        return
    raise AssertionError("Signoff should be frozen")


def test_signoff_full_payload() -> None:
    """Signoff carries all five fields when fully populated."""
    s = Signoff(
        decided_by="domain-user",
        date="2026-07-02",
        verbatim="все вопросы решай в сторону совершенства",
        instrument="DEL-1",
        chosen_variant="V-unfreeze-entity-projection",
    )
    assert s.chosen_variant == "V-unfreeze-entity-projection"
    assert s.instrument == "DEL-1"


# ---------------------------------------------------------------------------
# 2. Carrier fields default None (existing nodes don't break)
# ---------------------------------------------------------------------------


def test_conflict_signoff_defaults_none() -> None:
    """Conflict.signoff defaults to None — existing nodes are unaffected."""
    c = Conflict(
        id="C-test0001",
        axis="a",
        context="b",
        members=("R-1", "R-2"),
        steward="s",
        lifecycle="ACKNOWLEDGED",
    )
    assert c.signoff is None


def test_assumption_signoff_defaults_none() -> None:
    """Assumption.signoff defaults to None — existing nodes are unaffected."""
    a = Assumption(id="A-1", statement="x", status="HOLDS", owner="s")
    assert a.signoff is None


# ---------------------------------------------------------------------------
# 3-5. ConflictTransition writer
# ---------------------------------------------------------------------------

_AXIS = "a-vs-b"
_CTX = "test-context-for-signoff"
_CID = conflict_identity(_AXIS, _CTX)

_SAMPLE_HELD = f'''\
from hotam_spec.conflict import Conflict, Variant, conflict_identity

conflicts = (
    Conflict(
        id=conflict_identity("{_AXIS}", "{_CTX}"),
        axis="{_AXIS}",
        context="{_CTX}",
        members=("R-1", "R-2"),
        steward="s3",
        lifecycle="HELD(holding)",
        decided_by="s3",
        variants=(
            Variant(id="V-one", behavior="b1", implies="i1", costs="c1"),
            Variant(id="V-two", behavior="b2", implies="i2", costs="c2"),
        ),
    ),
)
'''


def test_conflict_transition_decided_writes_signoff() -> None:
    """A DECIDED transition with decided_by attaches a Signoff payload."""
    p = ProposedConflictTransition(
        conflict_id=_CID,
        new_lifecycle="DECIDED(rationale)",
        decided_by="s3",
        date="2026-07-06",
    )
    out = apply_proposal._apply_conflict_transition(_SAMPLE_HELD, p)
    result = "".join(out)
    assert "signoff=Signoff(" in result
    assert 'decided_by="s3"' in result
    assert 'date="2026-07-06"' in result
    # chosen_variant absent when not supplied
    assert "chosen_variant" not in result


def test_conflict_transition_acknowledged_no_signoff() -> None:
    """An ACKNOWLEDGED transition does NOT attach a signoff (no steward decision)."""
    p = ProposedConflictTransition(
        conflict_id=_CID,
        new_lifecycle="ACKNOWLEDGED",
    )
    out = apply_proposal._apply_conflict_transition(_SAMPLE_HELD, p)
    result = "".join(out)
    assert "signoff=Signoff(" not in result


def test_conflict_transition_variants_preserved_on_decided() -> None:
    """K2(b) fix: HELD->DECIDED preserves variants (they are NOT erased)."""
    p = ProposedConflictTransition(
        conflict_id=_CID,
        new_lifecycle="DECIDED(picked V-one)",
        decided_by="s3",
        chosen_variant="V-one",
        date="2026-07-06",
    )
    out = apply_proposal._apply_conflict_transition(_SAMPLE_HELD, p)
    result = "".join(out)
    # BOTH variants survive (anti-relitigation cargo preserved)
    assert "V-one" in result
    assert "V-two" in result
    # The chosen variant is recorded in the signoff
    assert 'chosen_variant="V-one"' in result


def test_conflict_transition_signoff_carries_verbatim_and_instrument() -> None:
    """Signoff carries verbatim and instrument when supplied."""
    p = ProposedConflictTransition(
        conflict_id=_CID,
        new_lifecycle="DECIDED(rationale)",
        decided_by="s3",
        date="2026-07-06",
        verbatim="pick V-one",
        instrument="DEL-2",
    )
    out = apply_proposal._apply_conflict_transition(_SAMPLE_HELD, p)
    result = "".join(out)
    assert 'verbatim="pick V-one"' in result
    assert 'instrument="DEL-2"' in result


# ---------------------------------------------------------------------------
# 6. AssumptionTransition writer
# ---------------------------------------------------------------------------

_SAMPLE_ASSUMPTION = '''\
from __future__ import annotations

from hotam_spec.assumption import Assumption, HOLDS

assumptions = (
    Assumption(
        id="A-target",
        statement="A belief.",
        status=HOLDS,
        owner="framework-author",
    ),
)
'''


def test_assumption_transition_writes_signoff() -> None:
    """K2(a) fix: AssumptionTransition with decided_by writes Assumption.signoff."""
    p = ProposedAssumptionTransition(
        assumption_id="A-target",
        new_status="DEAD",
        reason="falsified",
        decided_by="domain-user",
        date="2026-07-06",
    )
    out = apply_proposal._apply_assumption_transition(_SAMPLE_ASSUMPTION, p)
    assert "signoff=Signoff(" in out
    assert 'decided_by="domain-user"' in out
    assert 'date="2026-07-06"' in out
    assert "from hotam_spec.signoff import Signoff" in out


def test_assumption_transition_uncertain_no_signoff() -> None:
    """An UNCERTAIN transition (agent-enterable, no signoff) writes no signoff."""
    p = ProposedAssumptionTransition(
        assumption_id="A-target",
        new_status="UNCERTAIN",
        reason="questioning",
    )
    out = apply_proposal._apply_assumption_transition(_SAMPLE_ASSUMPTION, p)
    assert "signoff=Signoff(" not in out


# ---------------------------------------------------------------------------
# 7. _python_repr round-trip
# ---------------------------------------------------------------------------


def test_python_repr_renders_signoff() -> None:
    """_python_repr renders a Signoff as a constructor call (source-insertable)."""
    s = Signoff(
        decided_by="x",
        date="2026-07-06",
        chosen_variant="V-a",
    )
    rendered = apply_proposal._python_repr(s)
    assert rendered.startswith("Signoff(")
    assert 'decided_by="x"' in rendered
    assert 'chosen_variant="V-a"' in rendered
    # Defaults (verbatim="", instrument="personal") are omitted for cleanliness
    assert "verbatim" not in rendered
    assert "instrument" not in rendered


# ---------------------------------------------------------------------------
# 8. Live graph: C-be22cdd1 migration
# ---------------------------------------------------------------------------


def test_live_c_be22cdd1_carries_restored_variants_and_signoff() -> None:
    """C-be22cdd1 carries its restored Variants + Signoff after migration.

    This is the demonstration migration: the conflict's variants were erased
    by the old writer bug (K2(b)); the wave restored them from git history and
    attached a signoff with chosen_variant pointing at the picked variant.
    """
    from hotam_spec.graph import load_content_graph  # noqa: PLC0415

    g = load_content_graph()
    c = next((x for x in g.conflicts if x.id == "C-be22cdd1"), None)
    assert c is not None, "C-be22cdd1 must exist in the live graph"
    # Variants restored (both the chosen and the non-chosen anti-relitigation cargo)
    variant_ids = {v.id for v in c.variants}
    assert "V-unfreeze-entity-projection" in variant_ids
    assert "V-keep-freeze-defer-enforce" in variant_ids
    # Signoff attached with the chosen variant
    assert c.signoff is not None
    assert c.signoff.chosen_variant == "V-unfreeze-entity-projection"
    assert c.signoff.decided_by == "domain-user"
    assert c.signoff.date == "2026-07-02"
    assert c.signoff.instrument == "DEL-1"
