"""Canon: §Tick — the closed-loop diagnostic driver (advisory, M32 conservative).

Runs one diagnosis cycle and emits a structured Tick report:
  1. load_content_graph() (fresh substrate)
  2. all_violations + diagnose() -> the band roster
  3. classify each action by band; pick the TOP non-paused action
  4. render a Tick report: snapshot + top action + paused-band notice

M32 (conservative) — the tick is ADVISORY. It does NOT auto-apply any
action; structural fixes, drift fallout, conflict resolutions, and OPEN
items all surface for steward attention. The act-half remains
apply_proposal.py invoked explicitly. Auto-apply is DEFERRED behind P6
conscience (a tick that can't detect a contradiction IT introduces is
dangerous).

Why P5 still lands now: the cadence is the thing. Even an advisory tick
that just prints "what's next" each cycle is the Drive organ — it makes
the loop run AS A LOOP, not as a manually-prompted sequence.
"""

from __future__ import annotations

import argparse
import pathlib
import sys
from dataclasses import dataclass

_TOOLS = pathlib.Path(__file__).resolve().parent
_SRC = _TOOLS.parent / "src"
for p in (_TOOLS, _SRC):
    if str(p) not in sys.path:
        sys.path.insert(0, str(p))

import what_now  # noqa: E402
from tensio.graph import load_content_graph  # noqa: E402


@dataclass(frozen=True)
class TickReport:
    """Canon: §Tick — one diagnostic cycle result (advisory, M32 conservative).

    Fields:
      cycle        — caller-supplied monotonic cycle counter.
      total_actions — count of actions diagnose() returned.
      band_counts  — {band_label: count} summary.
      top_action   — the highest-priority Action, or None if graph is clean.
      paused       — True when top_action requires steward attention (always
                     True under M32 conservative; False only on empty graph).
      paused_reason — human explanation of why the tick is paused.
      advisory     — the one-line human-readable advice for this cycle.

    WHY frozen: a tick report is a point-in-time snapshot; it should not
    be mutated after construction (R-anchor-everything applied to data).
    """

    cycle: int
    total_actions: int
    band_counts: dict[str, int]
    top_action: object | None  # what_now.Action or None
    paused: bool  # True if top action requires steward attention
    paused_reason: str
    advisory: str  # the human-readable advice line


def tick(cycle: int = 1) -> TickReport:
    """Canon: §Tick — one advisory tick: load, diagnose, classify, render advice.

    M32 (conservative): EVERY band pauses — the tick never auto-applies.
    Returns a TickReport; the caller decides whether to print/log/act.

    WHY load fresh each call: the substrate may have changed between ticks
    (e.g. an operator applied a proposal); re-loading ensures each cycle
    reflects the current graph, not a cached snapshot.
    """
    g = load_content_graph()
    actions = what_now.diagnose(g)
    band_counts: dict[str, int] = {}
    for a in actions:
        band_counts[a.kind] = band_counts.get(a.kind, 0) + 1
    if not actions:
        return TickReport(
            cycle=cycle,
            total_actions=0,
            band_counts={},
            top_action=None,
            paused=False,
            paused_reason="",
            advisory=(
                "TICK OK — no actions. The graph is well-formed and every "
                "contradiction is visible, stewarded, and up to date."
            ),
        )
    top = actions[0]
    # M32 conservative: EVERY band pauses. The tick is advisory.
    paused_reason = (
        f"M32 (conservative): tick is advisory. The {top.kind} action requires "
        f"a steward-approved proposal via apply_proposal.py; the tick does not "
        f"auto-apply."
    )
    # If REFLECTION actions are present, surface them explicitly — P0 lands first.
    reflection_n = sum(1 for a in actions if a.kind == "REFLECTION")
    reflection_note = (
        f" ({reflection_n} REFLECTION — operator self-readiness)"
        if reflection_n
        else ""
    )
    advisory = (
        f"TICK CYCLE {cycle}: {len(actions)} action(s){reflection_note};"
        f" top is [P{top.priority}] {top.kind} on `{top.target}`."
        f" Steward should review and submit a"
        f" proposal via apply_proposal.py (see docs/playbooks/)."
    )
    return TickReport(
        cycle=cycle,
        total_actions=len(actions),
        band_counts=band_counts,
        top_action=top,
        paused=True,
        paused_reason=paused_reason,
        advisory=advisory,
    )


def render(report: TickReport) -> str:
    """Canon: §Tick — render a TickReport as a human-readable LF string.

    Format: cycle header, band summary, top action, paused notice, advisory.
    Empty graph: cycle header + advisory only. Always ends with newline.

    WHY a separate render step: keeps tick() pure (returns data); lets
    tests assert on the TickReport fields without parsing text, while
    callers that want human output call render().
    """
    lines = [f"== tick: cycle {report.cycle} =="]
    if report.total_actions == 0:
        lines.append("")
        lines.append(report.advisory)
        return "\n".join(lines) + "\n"
    lines.append("")
    lines.append(f"Bands: {dict(sorted(report.band_counts.items()))}")
    lines.append("")
    if report.top_action is not None:
        a = report.top_action
        lines.append(f"Top action: [P{a.priority}] {a.kind} on `{a.target}`")
        lines.append(f"  imperative: {a.imperative}")
    lines.append("")
    if report.paused:
        lines.append(f"PAUSED — {report.paused_reason}")
    lines.append("")
    lines.append(report.advisory)
    return "\n".join(lines) + "\n"


def main() -> int:
    """Canon: §Tick — CLI entry point: run one advisory tick and print the report.

    Exit code: always 0 — the tick is advisory, not a gate. CI gates remain
    pytest + gen_spec determinism checks. A caller cron can log the output;
    it never fails the build.
    """
    p = argparse.ArgumentParser(description="Tensio tick — advisory diagnostic cycle.")
    p.add_argument(
        "--cycle",
        type=int,
        default=1,
        help="cycle counter (caller-supplied; default 1)",
    )
    args = p.parse_args()
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    report = tick(cycle=args.cycle)
    sys.stdout.write(render(report))
    return 0


if __name__ == "__main__":
    sys.exit(main())
