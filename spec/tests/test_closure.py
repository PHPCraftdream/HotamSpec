"""Tests for tools/closure.py — per-action verify/closure check (P4).

Canon: §Closure — after apply_proposal writes + regens + pytest-greens, the
closure check asks the load-bearing question: did the proposal actually remove
the triggering action from the post-apply what_now diagnosis?

Six test cases:
  1. target_anchor on ProposedRequirement returns its R-id.
  2. target_anchor on ProposedConflictTransition returns its conflict_id (C-…).
  3. target_anchor on ProposedRejection returns its requirement_id.
  4. check_closure returns advanced=True when the target action is absent.
  5. check_closure returns advanced=False when the target action is still present.
  6. check_closure ignores same-target but different-kind actions (doesn't count them).
"""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

from tensio.proposal import (  # noqa: E402
    ProposedConflictTransition,
    ProposedRejection,
    ProposedRequirement,
)

import closure  # noqa: E402  (lives in tools/)
import what_now  # noqa: E402  (lives in tools/)


# ---------------------------------------------------------------------------
# 1–3. target_anchor on each proposal type
# ---------------------------------------------------------------------------


def test_target_anchor_proposed_requirement() -> None:
    """ProposedRequirement.target_anchor() returns its R-id."""
    pr = ProposedRequirement(
        id="R-foo",
        claim="The system shall do X.",
        owner="framework-author",
        status="SETTLED",
        why="Because X.",
    )
    assert pr.target_anchor() == "R-foo"


def test_target_anchor_proposed_conflict_transition() -> None:
    """ProposedConflictTransition.target_anchor() returns its conflict_id (C-…)."""
    pct = ProposedConflictTransition(
        conflict_id="C-abc123",
        new_lifecycle="DECIDED(rationale)",
        decided_by="framework-reviewer",
    )
    assert pct.target_anchor() == "C-abc123"


def test_target_anchor_proposed_rejection() -> None:
    """ProposedRejection.target_anchor() returns its requirement_id."""
    pr = ProposedRejection(
        requirement_id="R-old",
        reason="REJECTED — REPLACES by R-new.",
    )
    assert pr.target_anchor() == "R-old"


# ---------------------------------------------------------------------------
# 4. check_closure returns advanced=True when target action is gone
# ---------------------------------------------------------------------------


def test_check_closure_advanced_when_target_resolved(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """check_closure returns advanced=True when no matching action remains after apply."""
    # The proposal targets "R-my-open"
    proposal = ProposedRequirement(
        id="R-my-open",
        claim="The system shall answer X.",
        owner="framework-author",
        status="SETTLED",
        why="Resolves M99.",
    )

    # Post-apply diagnosis is EMPTY for this target — action is gone
    def fake_diagnose(g: object) -> list:  # noqa: ANN001
        return [
            what_now.Action(
                priority=4,
                kind="OPEN_ITEM",
                target="R-other-open",  # different target — not ours
                imperative="some other open",
            )
        ]

    monkeypatch.setattr(what_now, "diagnose", fake_diagnose)
    # Also monkeypatch load_content_graph so we don't need real graph.py
    monkeypatch.setattr(closure, "load_content_graph", lambda: object())

    result = closure.check_closure(proposal, triggering_kind="OPEN_ITEM")

    assert result.advanced is True
    assert result.target == "R-my-open"
    assert result.still_open_count == 0
    assert result.triggering_kind == "OPEN_ITEM"
    assert "closure OK" in result.note


# ---------------------------------------------------------------------------
# 5. check_closure returns advanced=False when target action is still present
# ---------------------------------------------------------------------------


def test_check_closure_not_advanced_when_target_still_open(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """check_closure returns advanced=False when the matching action still exists after apply."""
    proposal = ProposedConflictTransition(
        conflict_id="C-8600b1b8",
        new_lifecycle="DECIDED(rationale)",
        decided_by="framework-reviewer",
    )

    # Post-apply diagnosis still contains the stalled conflict
    def fake_diagnose(g: object) -> list:  # noqa: ANN001
        return [
            what_now.Action(
                priority=3,
                kind="CONFLICT_STALLED",
                target="C-8600b1b8",  # SAME target — still there!
                imperative="conflict still stalled",
            )
        ]

    monkeypatch.setattr(what_now, "diagnose", fake_diagnose)
    monkeypatch.setattr(closure, "load_content_graph", lambda: object())

    result = closure.check_closure(proposal, triggering_kind="CONFLICT_STALLED")

    assert result.advanced is False
    assert result.target == "C-8600b1b8"
    assert result.still_open_count == 1
    assert result.triggering_kind == "CONFLICT_STALLED"
    assert "closure FAILED" in result.note


# ---------------------------------------------------------------------------
# 6. check_closure distinguishes kind — same target, different kind = not counted
# ---------------------------------------------------------------------------


def test_check_closure_distinguishes_kind(monkeypatch: pytest.MonkeyPatch) -> None:
    """An action on the same target but a DIFFERENT kind does not count as still-open."""
    proposal = ProposedRequirement(
        id="R-target",
        claim="The system shall do Y.",
        owner="framework-author",
        status="SETTLED",
        why="Resolves M5.",
    )

    # Post-apply diagnosis has same TARGET but DIFFERENT KIND (DRIFT_FALLOUT, not OPEN_ITEM)
    def fake_diagnose(g: object) -> list:  # noqa: ANN001
        return [
            what_now.Action(
                priority=2,
                kind="DRIFT_FALLOUT",  # different kind
                target="R-target",  # same target
                imperative="drifted",
            )
        ]

    monkeypatch.setattr(what_now, "diagnose", fake_diagnose)
    monkeypatch.setattr(closure, "load_content_graph", lambda: object())

    # We are checking closure for OPEN_ITEM — a DRIFT_FALLOUT on same target must NOT block it
    result = closure.check_closure(proposal, triggering_kind="OPEN_ITEM")

    assert result.advanced is True
    assert result.still_open_count == 0
    assert "closure OK" in result.note
