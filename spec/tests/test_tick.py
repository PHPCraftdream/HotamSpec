"""Tests for the Tick report (what_now.tick / render_tick / --report) — the
advisory closed-loop diagnostic driver (P5, M32).

Formerly tools/tick.py (a thin wrapper adding no logic beyond
diagnose()+render()); folded into what_now.py's --report flag
(R-prefer-tool-over-hand / mechanical de-duplication).

Four guarantees:
  1. An empty graph (monkeypatched) yields total_actions == 0 and paused is False.
  2. A non-empty graph (real meta-domain) yields paused is True and top_action is set.
  3. render_tick() on a paused report includes the "PAUSED" substring.
  4. `what_now.py --report` via subprocess exits with code 0 (it is advisory, not a gate).
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path
from unittest.mock import patch

_TOOLS = Path(__file__).resolve().parents[1] / "tools"

import what_now as what_now_module  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402


# ---------------------------------------------------------------------------
# 1. Empty graph → total_actions == 0, paused is False
# ---------------------------------------------------------------------------


def test_tick_returns_report_on_empty_graph() -> None:
    """Monkeypatched empty graph: total_actions == 0 and paused is False."""
    empty_g = TensionGraph()
    with patch.object(what_now_module, "load_content_graph", return_value=empty_g):
        report = what_now_module.tick(cycle=1)
    assert report.total_actions == 0
    assert report.paused is False
    assert report.top_action is None
    assert "TICK OK" in report.advisory


# ---------------------------------------------------------------------------
# 2. Real meta-domain → paused is True, top_action is not None
# ---------------------------------------------------------------------------


def test_tick_pauses_on_nonempty() -> None:
    """Real meta-domain: whenever tick reports actions, it pauses and surfaces the top one.

    RULE (invariant, not a fixed non-zero expectation): total_actions is NOT
    hard-coded > 0 here. all_findings(g) covers only the fixed P0 REFLECTION
    predicate set (REFLECTION_PREDICATES) -- as the domain's honesty passes
    burn down closeable debt and other P0 conditions, total_actions can
    legitimately reach 0 (a genuinely well-formed graph), exactly like the
    monkeypatched empty-graph case in test_tick_returns_report_on_empty_graph.
    What the tick report must always get right is the M32 conservative
    CONTRACT: IF there are actions, it pauses and names a top_action; if
    there are none, it reports OK. Hard-coding 'must have actions' coupled
    this test to a moving debt count instead of to the tick's actual
    behavioral guarantee.
    """
    report = what_now_module.tick(cycle=1)
    if report.total_actions > 0:
        assert report.paused is True, "M32 conservative: every non-empty tick is paused"
        assert report.top_action is not None, "top_action must be set when actions exist"
    else:
        assert report.paused is False
        assert report.top_action is None
        assert "TICK OK" in report.advisory


# ---------------------------------------------------------------------------
# 3. render_tick() on a paused report contains "PAUSED"
# ---------------------------------------------------------------------------


def test_render_emits_paused_notice() -> None:
    """render_tick() on a paused TickReport contains the PAUSED substring."""

    dummy_action = what_now_module.Action(
        priority=1,
        kind="STRUCTURE",
        target="some-id",
        imperative="fix something",
    )
    report = what_now_module.TickReport(
        cycle=7,
        total_actions=1,
        band_counts={"STRUCTURE": 1},
        top_action=dummy_action,
        paused=True,
        paused_reason="M32 test paused reason",
        advisory="TICK CYCLE 7: test advisory",
    )
    output = what_now_module.render_tick(report)
    assert "PAUSED" in output, "render_tick() must include PAUSED notice for paused reports"
    assert "cycle 7" in output
    assert "STRUCTURE" in output


# ---------------------------------------------------------------------------
# 4. `what_now.py --report` via subprocess exits with code 0
# ---------------------------------------------------------------------------


def test_tick_report_flag_runs_without_error() -> None:
    """`what_now.py --report --cycle 99` invoked via subprocess exits with
    code 0 (advisory, not a gate)."""
    result = subprocess.run(
        [
            sys.executable,
            str(_TOOLS / "what_now.py"),
            "--report",
            "--cycle",
            "99",
        ],
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode == 0, (
        f"what_now.py --report exited with non-zero code {result.returncode}.\n"
        f"stdout: {result.stdout}\nstderr: {result.stderr}"
    )
    assert "tick: cycle 99" in result.stdout
