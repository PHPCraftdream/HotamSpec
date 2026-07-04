"""Canon: §Attention — the Claude adapter: inject the attention list into context.

RULE (R-attention-claude-adapter): this is a THIN Claude-Code UserPromptSubmit
hook wrapper around the agent-agnostic attention core. It does NOT re-implement
any sensing logic: it loads the active-domain graph, calls
hotam_spec.attention.collect() with the runtime-fs sources injected by
tools/what_now.runtime_fs_sources() (the live superset), and prints the flat
plain-text list to stdout — Claude Code injects a UserPromptSubmit hook's stdout
into the agent's context.

The core (hotam_spec.attention) knows nothing about Claude or hooks
(R-attention-agent-agnostic-core); this file is the ONLY Claude-specific seam,
and it is a tool, not core. tools/setup_hooks.py wires it onto UserPromptSubmit
so a fresh clone gets the attention pulse with zero edits
(R-attention-claude-adapter, R-sensorium-committed).

Fails soft: any error prints nothing and exits 0, so a sensing hiccup never
blocks the agent's turn.

Usage (from spec/):
  uv run python tools/attention_hook.py
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))
_TOOLS = Path(__file__).resolve().parent
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))


_USAGE = (
    "Usage (from spec/):\n"
    "  uv run python tools/attention_hook.py\n"
    "\n"
    "Thin Claude-Code UserPromptSubmit hook. Loads the active-domain graph,\n"
    "calls hotam_spec.attention.collect() and prints the flat attention list\n"
    "to stdout (Claude Code injects it into the agent's turn). No arguments;\n"
    "fails soft (prints nothing, exits 0) on any error.\n"
)


def main() -> int:
    # A STATIC usage string when asked for help. attention_hook has no argparse;
    # without this branch, gen_spec's _capture_tool_help (which invokes main with
    # --help to snapshot help text) would instead run the LIVE collector and bake
    # its calendar/atom-count-dependent output into spec/docs/tools/attention_hook.md
    # — a non-deterministic drift source (re-snapshotted every atom/day).
    if "-h" in sys.argv[1:] or "--help" in sys.argv[1:]:
        sys.stdout.write(_USAGE)
        return 0
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    try:
        from hotam_spec import attention as _attention
        from hotam_spec.graph import load_content_graph
        import what_now as _what_now

        g = load_content_graph()
        if g.is_empty():
            return 0
        signals = _attention.collect(
            g, runtime_sources=_what_now.runtime_fs_sources()
        )
        if not signals:
            return 0
        sys.stdout.write("== attention (auto) ==\n")
        sys.stdout.write(_attention.render_flat(signals))
    except Exception:
        # Fail soft: a sensing hiccup must never block the turn.
        return 0
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
