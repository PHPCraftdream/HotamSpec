"""Canon: §Attention — the Claude adapter: inject the attention list into context.

RULE (R-attention-claude-adapter): this is a THIN Claude-Code UserPromptSubmit
hook wrapper around the agent-agnostic attention core. It does NOT re-implement
any sensing logic: it loads the active-domain graph, calls
hotam_spec.attention.collect() with the runtime-fs sources injected by
tools/what_now.runtime_fs_sources() (the live superset), and prints the flat
plain-text list to stdout — Claude Code injects a UserPromptSubmit hook's stdout
into the agent's context.

Delta mode (default): the hook snapshots the set of shown signals (by a stable
key: source + priority + target + message) in spec/.runtime/attention-last-shown.json.
On the NEXT invocation, if the signal set is IDENTICAL to the snapshot, only a
one-line counter is printed instead of the full list. A NEW or DISAPPEARED signal
produces the full list and updates the snapshot. This prevents attention-blindness
from seeing the same wall of text every prompt.

The core (hotam_spec.attention) knows nothing about Claude or hooks
(R-attention-agent-agnostic-core); this file is the ONLY Claude-specific seam,
and it is a tool, not core. tools/setup_hooks.py wires it onto UserPromptSubmit
so a fresh clone gets the attention pulse with zero edits
(R-attention-claude-adapter, R-sensorium-committed).

Fails soft: any error prints nothing and exits 0, so a sensing hiccup never
blocks the agent's turn.

Usage (from spec/):
  python tools/attention_hook.py
"""

from __future__ import annotations

import json
import sys
from collections import Counter
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))
_TOOLS = Path(__file__).resolve().parent
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))

from hotam_spec.runtime_paths import runtime_dir as _runtime_dir

_RUNTIME = _runtime_dir()
_SNAPSHOT_FILE = _RUNTIME / "attention-last-shown.json"

_USAGE = (
    "Usage (from spec/):\n"
    "  python tools/attention_hook.py\n"
    "\n"
    "Thin Claude-Code UserPromptSubmit hook. Loads the active-domain graph,\n"
    "calls hotam_spec.attention.collect() and prints the flat attention list\n"
    "to stdout (Claude Code injects it into the agent's turn). Shows a delta\n"
    "summary when the signal set is unchanged since the last invocation.\n"
    "No arguments; fails soft (prints nothing, exits 0) on any error.\n"
)


def _signal_key(s: object) -> str:
    """Stable string key for an AttentionSignal, used for delta comparison.

    Deliberately excludes `message`: messages for pending-proposal signals
    contain a wall-clock-dependent age ("N day(s)") that changes daily,
    causing false "changed" delta triggers every calendar day. The triple
    (source, priority, target) identifies a signal stably; a message
    *change* on an existing (source, priority, target) is cosmetic, not a
    new signal that the operator must re-read.
    """
    return f"{s.source}|{s.priority}|{s.target}"  # type: ignore[union-attr]


def _load_snapshot() -> set[str] | None:
    """Load the previous signal-key set, or None if no snapshot exists."""
    try:
        if not _SNAPSHOT_FILE.exists():
            return None
        data = json.loads(_SNAPSHOT_FILE.read_text(encoding="utf-8"))
        if isinstance(data, list):
            return set(data)
        return None
    except Exception:  # noqa: BLE001
        return None


def _save_snapshot(keys: set[str]) -> None:
    """Save the current signal-key set."""
    try:
        _RUNTIME.mkdir(parents=True, exist_ok=True)
        _SNAPSHOT_FILE.write_text(
            json.dumps(sorted(keys), ensure_ascii=False), encoding="utf-8"
        )
    except Exception:  # noqa: BLE001
        pass


def _band_summary(signals: list[object]) -> str:
    """Build a compact band-count summary like '3 pending, 2 revisit, 1 ticket'."""
    from hotam_spec.attention import BAND_LABEL  # noqa: PLC0415

    counts: Counter[str] = Counter()
    for s in signals:
        band = BAND_LABEL.get(s.priority, f"P{s.priority}")  # type: ignore[union-attr]
        counts[band] += 1
    parts = [f"{n} {label}" for label, n in sorted(counts.items(), key=lambda x: x[0])]
    return " / ".join(parts) if parts else "clear"


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
            # Clear the snapshot when there are no signals.
            _save_snapshot(set())
            return 0

        current_keys = {_signal_key(s) for s in signals}
        previous_keys = _load_snapshot()

        if previous_keys is not None and current_keys == previous_keys:
            # Delta: unchanged — print compact one-liner.
            summary = _band_summary(signals)
            sys.stdout.write(
                f"== attention: {len(signals)} signal(s) unchanged ({summary})"
                f" -- python tools/attention.py for full list ==\n"
            )
        else:
            # Changed or first run — print full list.
            sys.stdout.write("== attention (auto) ==\n")
            sys.stdout.write(_attention.render_flat(signals))

        _save_snapshot(current_keys)
    except Exception:
        # Fail soft: a sensing hiccup must never block the turn.
        return 0
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
