"""Tests for tools/tick.py — the advisory closed-loop diagnostic driver (P5, M32).

Four guarantees:
  1. An empty graph (monkeypatched) yields total_actions == 0 and paused is False.
  2. A non-empty graph (real meta-domain) yields paused is True and top_action is set.
  3. render() on a paused report includes the "PAUSED" substring.
  4. tick.main() via subprocess exits with code 0 (it is advisory, not a gate).
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path
from unittest.mock import patch

_TOOLS = Path(__file__).resolve().parents[1] / "tools"
_SRC = Path(__file__).resolve().parents[1] / "src"
for _p in (_TOOLS, _SRC):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import tick as tick_module  # noqa: E402
from tensio.graph import TensionGraph  # noqa: E402


# ---------------------------------------------------------------------------
# 1. Empty graph → total_actions == 0, paused is False
# ---------------------------------------------------------------------------


def test_tick_returns_report_on_empty_graph() -> None:
    """Monkeypatched empty graph: total_actions == 0 and paused is False."""
    empty_g = TensionGraph()
    with patch.object(tick_module, "load_content_graph", return_value=empty_g):
        report = tick_module.tick(cycle=1)
    assert report.total_actions == 0
    assert report.paused is False
    assert report.top_action is None
    assert "TICK OK" in report.advisory


# ---------------------------------------------------------------------------
# 2. Real meta-domain → paused is True, top_action is not None
# ---------------------------------------------------------------------------


def test_tick_pauses_on_nonempty() -> None:
    """Real meta-domain has open actions; tick must pause and surface the top one."""
    report = tick_module.tick(cycle=1)
    # The meta-domain always has at least one open action (DETECTED conflict, OPEN reqs)
    assert report.total_actions > 0, "meta-domain should have open actions"
    assert report.paused is True, "M32 conservative: every non-empty tick is paused"
    assert report.top_action is not None, "top_action must be set when actions exist"


# ---------------------------------------------------------------------------
# 3. render() on a paused report contains "PAUSED"
# ---------------------------------------------------------------------------


def test_render_emits_paused_notice() -> None:
    """render() on a paused TickReport contains the PAUSED substring."""

    # Build a minimal paused report directly to avoid coupling to graph state.
    from what_now import Action  # noqa: PLC0415

    dummy_action = Action(
        priority=1,
        kind="STRUCTURE",
        target="some-id",
        imperative="fix something",
    )
    report = tick_module.TickReport(
        cycle=7,
        total_actions=1,
        band_counts={"STRUCTURE": 1},
        top_action=dummy_action,
        paused=True,
        paused_reason="M32 test paused reason",
        advisory="TICK CYCLE 7: test advisory",
    )
    output = tick_module.render(report)
    assert "PAUSED" in output, "render() must include PAUSED notice for paused reports"
    assert "cycle 7" in output
    assert "STRUCTURE" in output


# ---------------------------------------------------------------------------
# 4. tick.main() via subprocess exits with code 0
# ---------------------------------------------------------------------------


def test_tick_main_runs_without_error() -> None:
    """tick.main() invoked via subprocess exits with code 0 (advisory, not a gate)."""
    result = subprocess.run(
        [sys.executable, str(_TOOLS / "tick.py"), "--cycle", "99"],
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode == 0, (
        f"tick.py exited with non-zero code {result.returncode}.\n"
        f"stdout: {result.stdout}\nstderr: {result.stderr}"
    )
    assert "tick: cycle 99" in result.stdout
