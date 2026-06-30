"""Tests for hotam_spec.proposal — the structured operator-→-steward proposal types.

Two duties:
  1. Type-shape tests: frozen dataclasses with the right fields and defaults.
  2. Validator helper test: the apply_proposal._validate_proposal function rejects
     a DECIDED transition with empty decided_by (the type-level contract).
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import pytest  # noqa: E402

from hotam_spec.proposal import (  # noqa: E402
    ProposedConflictTransition,
    ProposedRejection,
    ProposedRequirement,
)


# ---------------------------------------------------------------------------
# 1. Type-shape: frozen, fields present, defaults work
# ---------------------------------------------------------------------------


def test_proposed_requirement_is_frozen() -> None:
    """ProposedRequirement is a frozen dataclass."""
    pr = ProposedRequirement(
        id="R-foo",
        claim="The system shall do X.",
        owner="framework-author",
        status="SETTLED",
        why="Because X.",
    )
    with pytest.raises((AttributeError, TypeError)):
        pr.id = "R-bar"  # type: ignore[misc]


def test_proposed_requirement_defaults() -> None:
    """ProposedRequirement defaults work (assumptions/relations/enforced_by empty)."""
    pr = ProposedRequirement(
        id="R-foo",
        claim="claim",
        owner="framework-author",
        status="DRAFT",
        why="why",
    )
    assert pr.assumptions == ()
    assert pr.relations == ()
    assert pr.enforcement == "PROSE"
    assert pr.enforced_by == ()
    assert pr.m_tag == ""


def test_proposed_conflict_transition_is_frozen() -> None:
    """ProposedConflictTransition is a frozen dataclass."""
    pct = ProposedConflictTransition(
        conflict_id="C-abc",
        new_lifecycle="ACKNOWLEDGED",
    )
    with pytest.raises((AttributeError, TypeError)):
        pct.conflict_id = "C-xyz"  # type: ignore[misc]


def test_proposed_conflict_transition_defaults() -> None:
    """ProposedConflictTransition defaults: decided_by empty, derived empty tuple."""
    pct = ProposedConflictTransition(
        conflict_id="C-abc",
        new_lifecycle="ACKNOWLEDGED",
    )
    assert pct.decided_by == ""
    assert pct.revisit_marker == ""
    assert pct.derived == ()


def test_proposed_conflict_transition_decided_fields() -> None:
    """ProposedConflictTransition holds decided_by and derived correctly."""
    pct = ProposedConflictTransition(
        conflict_id="C-abc",
        new_lifecycle="DECIDED(because X)",
        decided_by="framework-reviewer",
        derived=("R-new-req",),
    )
    assert pct.decided_by == "framework-reviewer"
    assert pct.derived == ("R-new-req",)


def test_proposed_rejection_is_frozen() -> None:
    """ProposedRejection is a frozen dataclass."""
    pr = ProposedRejection(
        requirement_id="R-foo", reason="REJECTED — REPLACES by R-bar."
    )
    with pytest.raises((AttributeError, TypeError)):
        pr.requirement_id = "R-baz"  # type: ignore[misc]


def test_proposed_rejection_fields() -> None:
    """ProposedRejection holds requirement_id and reason."""
    pr = ProposedRejection(
        requirement_id="R-old",
        reason="REJECTED — REPLACES by R-new because the old approach leaked scope.",
    )
    assert pr.requirement_id == "R-old"
    assert "REPLACES" in pr.reason


# ---------------------------------------------------------------------------
# 2. Validator helper: _validate_proposal from apply_proposal
# ---------------------------------------------------------------------------


def test_proposed_conflict_decided_allows_empty_decided_by_at_type_level() -> None:
    """ProposedConflictTransition can be CONSTRUCTED with empty decided_by (type-level only).

    The runtime validation (refusing to WRITE decided="" for DECIDED) lives in
    apply_proposal._validate_proposal, not in the dataclass constructor.
    """
    # This should NOT raise — the type allows empty decided_by
    pct = ProposedConflictTransition(
        conflict_id="C-abc",
        new_lifecycle="DECIDED(some rationale)",
        decided_by="",  # empty — type allows it; applier rejects it
    )
    assert pct.decided_by == ""


def test_validate_proposal_rejects_decided_without_decided_by() -> None:
    """apply_proposal._validate_proposal raises ValueError for DECIDED with empty decided_by."""
    import apply_proposal  # noqa: PLC0415

    raw = {
        "kind": "ConflictTransition",
        "conflict_id": "C-abc",
        "new_lifecycle": "DECIDED(some rationale)",
        "decided_by": "",  # missing!
    }
    with pytest.raises(ValueError, match="decided_by is empty"):
        apply_proposal._validate_proposal(raw)


def test_validate_proposal_rejects_wrong_kind() -> None:
    """apply_proposal._validate_proposal raises ValueError for unknown proposal kind."""
    import apply_proposal  # noqa: PLC0415

    raw = {
        "kind": "UnknownKind",
        "conflict_id": "C-abc",
        "new_lifecycle": "ACKNOWLEDGED",
    }
    with pytest.raises(ValueError, match="Unsupported proposal kind"):
        apply_proposal._validate_proposal(raw)


def test_validate_proposal_accepts_valid_decided() -> None:
    """apply_proposal._validate_proposal accepts a well-formed DECIDED transition."""
    import apply_proposal  # noqa: PLC0415

    raw = {
        "kind": "ConflictTransition",
        "conflict_id": "C-8600b1b8",
        "new_lifecycle": "DECIDED(because X resolves the tension)",
        "decided_by": "framework-reviewer",
        "revisit_marker": "REVISIT if Y changes",
        "derived": ["R-new-thing"],
    }
    result = apply_proposal._validate_proposal(raw)
    assert result.conflict_id == "C-8600b1b8"
    assert result.decided_by == "framework-reviewer"
    assert result.derived == ("R-new-thing",)


def test_validate_proposal_accepts_acknowledged() -> None:
    """apply_proposal._validate_proposal accepts ACKNOWLEDGED (no decided_by needed)."""
    import apply_proposal  # noqa: PLC0415

    raw = {
        "kind": "ConflictTransition",
        "conflict_id": "C-abc",
        "new_lifecycle": "ACKNOWLEDGED",
    }
    result = apply_proposal._validate_proposal(raw)
    assert result.new_lifecycle == "ACKNOWLEDGED"
    assert result.decided_by == ""
