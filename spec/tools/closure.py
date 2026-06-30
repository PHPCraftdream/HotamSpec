"""Canon: §Closure — per-action verify: did the proposal remove its diagnosis?

The feedback edge of the closed loop. After apply_proposal writes + regens +
pytest-greens, the closure check asks the load-bearing question: is the
action that motivated this proposal STILL in the diagnosis? If yes, the
write technically landed but did NOT advance — the tick (P5) must NOT count
this as progress. If no, the action is closed and progress is real.

The check is structural and deterministic: it loads the fresh graph, runs
diagnose(), and asserts no Action has `target == proposal.target_anchor()`
that matches the original triggering action's (priority, kind, target).
"""

from __future__ import annotations

import sys
from dataclasses import dataclass
from pathlib import Path

# Make tensio importable (lives in spec/src)
_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from tensio.graph import load_content_graph  # noqa: E402
from tensio.proposal import Proposal  # noqa: E402


@dataclass(frozen=True)
class ClosureResult:
    """Canon: §Closure — result of a per-action closure check.

    Fields:
      advanced         — True = the original action is gone from diagnosis.
      target           — the proposal.target_anchor() that was checked.
      still_open_count — count of post-apply actions still touching this target.
      triggering_kind  — the original action's kind (e.g. "OPEN_ITEM").
      note             — human-readable summary of the closure check result.
    """

    advanced: bool
    target: str
    still_open_count: int
    triggering_kind: str
    note: str


def check_closure(
    proposal: Proposal,
    triggering_kind: str,
) -> ClosureResult:
    """Canon: §Closure — load the live (post-apply) graph, diagnose, and assert closure.

    `triggering_kind` is the band/kind of the original action this proposal
    was meant to close (e.g. "OPEN_ITEM" for a P4 action, "CONFLICT_STALLED"
    for a P3, "STRUCTURE" for a P1 violation, "DRIFT_FALLOUT" for P2).
    The closure check passes iff after the write, no action with that
    (kind, target) pair remains in diagnose() output.

    Exit semantics (when called from apply_proposal):
      0 — advanced (closure confirmed, progress is real)
      1 — pytest failed (apply_proposal returns 1 before reaching closure)
      2 — not advanced (write landed but same action still in diagnosis)

    Returns a ClosureResult with `advanced=True` if closure confirmed, else False.
    """
    # Lazy-import what_now (lives in tools/, not a package)
    tools_dir = Path(__file__).resolve().parent
    if str(tools_dir) not in sys.path:
        sys.path.insert(0, str(tools_dir))
    import what_now  # noqa: PLC0415

    g = load_content_graph()
    actions = what_now.diagnose(g)
    target = proposal.target_anchor()
    still = [a for a in actions if a.target == target and a.kind == triggering_kind]
    if still:
        return ClosureResult(
            advanced=False,
            target=target,
            still_open_count=len(still),
            triggering_kind=triggering_kind,
            note=(
                f"closure FAILED: {len(still)} action(s) with kind={triggering_kind!r} "
                f"and target={target!r} remain in diagnose() after apply."
            ),
        )
    return ClosureResult(
        advanced=True,
        target=target,
        still_open_count=0,
        triggering_kind=triggering_kind,
        note=f"closure OK: no {triggering_kind} action remains on target {target!r}.",
    )
