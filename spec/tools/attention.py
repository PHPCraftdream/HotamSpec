"""Canon: §Attention — the agent-agnostic CLI over the attention core.

RULE (R-attention-agent-agnostic-core): this is the UNIVERSAL entry point any
agent, on any platform, runs to get the "pay attention here" list. It prints
plain text (one line per signal) and knows nothing of Claude — the Claude hook
(tools/attention_hook.py) is a thin wrapper that shells out to this same core.

It loads the active domain graph, runs hotam_spec.attention.collect() with the
runtime-fs sources injected by tools/what_now.runtime_fs_sources() (the live
SUPERSET: graph diagnosis + pending proposals + open tickets + stale audit +
unread revisit markers), and renders the flat text via
hotam_spec.attention.render_flat.

Usage (from spec/):
  uv run python tools/attention.py            # active-domain live attention list
  uv run python tools/attention.py --demo     # the fixture demo graph
  uv run python tools/attention.py --graph-only   # deterministic subset only
  uv run python tools/attention.py --json     # machine-readable signals

WHY separate from what_now.py: what_now renders the operator's banded action
report (a rich human console). attention.py is the minimal, machine-consumable
sensorium surface an ARBITRARY agent calls; what_now is one consumer of the same
core (R-prefer-tool-over-hand).
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))
_TOOLS = Path(__file__).resolve().parent
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))

from hotam_spec import attention as _attention  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from _graph_loader import load_graph as _load_graph  # noqa: E402


def collect_signals(g: TensionGraph, *, graph_only: bool) -> list:
    """Return the attention signals: the deterministic subset if graph_only, else
    the live superset (graph + injected runtime-fs bands)."""
    if graph_only:
        return _attention.collect(g)
    import what_now as _what_now  # noqa: PLC0415

    return _attention.collect(g, runtime_sources=_what_now.runtime_fs_sources())


def main(argv: list[str] | None = None) -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--demo", action="store_true", help="use the fixture graph.")
    parser.add_argument(
        "--graph-only",
        action="store_true",
        help="deterministic subset only (no runtime-fs bands).",
    )
    parser.add_argument(
        "--json", action="store_true", help="emit machine-readable JSON signals."
    )
    args = parser.parse_args(argv)

    g = _load_graph(demo=args.demo)
    if g.is_empty() and not args.demo:
        sys.stdout.write("attention: no domain content — the framework is blank.\n")
        return 0
    signals = collect_signals(g, graph_only=args.graph_only)
    if args.json:
        sys.stdout.write(
            json.dumps(
                [
                    {
                        "source": s.source,
                        "priority": s.priority,
                        "target": s.target,
                        "message": s.message,
                    }
                    for s in signals
                ],
                ensure_ascii=False,
                indent=2,
            )
            + "\n"
        )
    else:
        sys.stdout.write(_attention.render_flat(signals))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
