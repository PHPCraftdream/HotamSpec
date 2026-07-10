"""Canon: §Context — the operator's working-context measurement (reader + CLI dispatcher).

The honesty organ: the operator's context % must be MEASURED, not guessed.
This reader looks for a runtime stamp at spec/.runtime/context.json (written
by a harness hook — DEFERRED, the hook lives in the user's global settings
and is a steward decision, not part of the framework body). If the file is
absent, returns UNMEASURED — honestly, rather than faking a number.

PRODUCER CONTRACT (what a future hook must write, verified by
tests/test_tool_context.py):

    spec/.runtime/context.json = {
      "ctx_pct": <float 0..100>,   # working-context fullness — REQUIRED
      "model":   "<model id>",     # optional
      "stamp":   "<iso8601>"       # optional — when the measurement was taken
    }

R-measure-context-size (DRAFT): the reader + schema + LIVE-STATE bridge exist
(gen_spec renders render_line()); the PRODUCING hook is still deferred —
project-local hook events (SessionStart / UserPromptSubmit / PostToolUse /
Stop) do not receive context-window usage on stdin today; only the host
statusline pipeline sees it, and the host is sovereign — the framework will
NOT touch it (R-work-within-launch-dir). The requirement stays DRAFT until the
host honestly delivers ctx_pct on the local stdin payload.

CLI DISPATCHER (land.py precedent, task #106 / L2-#4): three tools orbit the
same artifact (spec/.runtime/context.json) and the same question — "is the
operator's context measured?": this reader (`context.py`), the stdin-payload
writer (`tools/context_producer.py`), and the hook installer
(`tools/setup_context_hook.py`). This module does NOT reimplement or move
their logic — `python tools/context.py status|produce|install [--status|--off]`
is a thin dispatcher that imports the sibling modules and forwards argv,
exactly like `tools/land.py` does for gate.py/gate_status.py/closure.py.

The three modules keep their own filenames and stay independently importable
un-merged, for three concrete reasons:
  - `.claude/settings.json`'s committed Stop hook (R-sensorium-committed)
    invokes `spec/tools/context_producer.py` directly by path; changing that
    filename means editing the committed sensorium, out of scope here.
  - `enforced_by=("tools/context.py",)` on R-measure-context-size (DRAFT,
    domains/hotam-spec-self/graph.py) names this reader module by path.
  - tests/test_tool_context.py and tests/test_tool_setup_context_hook.py
    `monkeypatch.setattr` module-level constants on each module by name
    (`context._RUNTIME`, `producer._RUNTIME`, `sch._SETTINGS_LOCAL`) — merging
    bodies would silently change what each patch target hits.

Run (from spec/):
  python tools/context.py status                    # render_line() (default if no subcommand)
  python tools/context.py produce [--stdin-file P]   # context_producer.py forwarded
  python tools/context.py install [--status] [--off] # setup_context_hook.py forwarded
"""

from __future__ import annotations
import argparse
import json
import sys
from dataclasses import dataclass
from pathlib import Path

# Runtime is CONSUMER data — resolved via the R1-R6 chain (§3.2 variant 4-C).
# Make hotam_spec importable for the runtime_paths accessor.
_SPEC_ROOT = Path(__file__).resolve().parents[1]
if str(_SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(_SPEC_ROOT / "src"))

from hotam_spec.runtime_paths import runtime_dir as _runtime_dir

_RUNTIME = _runtime_dir() / "context.json"


@dataclass(frozen=True)
class ContextState:
    """Canon: §Context — the measured or absent context-fullness stamp."""

    measured: bool
    pct: float | None = None  # 0..100 working-context fullness
    model: str = ""
    stamp: str = ""  # iso8601 of the measurement (staleness visibility)
    note: str = ""


def read_context() -> ContextState:
    """Read the runtime context stamp, or UNMEASURED if absent."""
    if not _RUNTIME.exists():
        return ContextState(
            measured=False,
            note="UNMEASURED — no runtime stamp (the producing hook is a "
            "deferred steward decision; see R-measure-context-size).",
        )
    try:
        data = json.loads(_RUNTIME.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError):
        return ContextState(measured=False, note="UNMEASURED — stamp unreadable.")
    return ContextState(
        measured=True,
        pct=float(data.get("ctx_pct")) if data.get("ctx_pct") is not None else None,
        model=str(data.get("model", "")),
        stamp=str(data.get("stamp", "")),
        note="",
    )


_UNMEASURED_ACTION = (
    "context: UNMEASURED — measuring working-context requires host cooperation "
    "the framework will not touch (R-work-within-launch-dir); it measures only "
    "if the local stdin payload honestly carries ctx_pct "
    "— R-unmeasured-cipher-names-host-boundary"
)


def render_line() -> str:
    """One-line context cipher for the LIVE-STATE block / tick.

    R-unmeasured-cipher-names-host-boundary: while UNMEASURED, this line
    honestly explains WHY there is no number — measuring working-context would
    require cooperation from the sovereign host (statusline / ~/.claude), which
    the framework will not touch (R-work-within-launch-dir). There is no
    command to call: the cipher measures only if the local stdin payload
    honestly carries ctx_pct. Once measured, the explanation disappears — it is
    only useful while the gap it describes still exists.
    """
    s = read_context()
    if not s.measured or s.pct is None:
        return _UNMEASURED_ACTION
    suffix = f"{s.model} @ {s.stamp}" if s.stamp else s.model
    return f"context: {s.pct:.0f}% ({suffix})"


_SUBCOMMANDS = ("status", "produce", "install")


def _dispatch_status(argv: list[str]) -> int:
    """Print the current context cipher line (this module's own reader)."""
    parser = argparse.ArgumentParser(prog="context.py status")
    parser.parse_args(argv)  # no options today — parses only for --help/error consistency
    print(render_line())
    return 0


def _dispatch_produce(argv: list[str]) -> int:
    """Forward to context_producer.py's own argparse CLI, unchanged."""
    import context_producer  # noqa: PLC0415  -- lives in tools/, not a package

    return context_producer.main(argv)


def _dispatch_install(argv: list[str]) -> int:
    """Forward to setup_context_hook.py's own argparse CLI, unchanged."""
    import setup_context_hook  # noqa: PLC0415  -- lives in tools/, not a package

    return setup_context_hook.main(argv)


_DISPATCH = {
    "status": _dispatch_status,
    "produce": _dispatch_produce,
    "install": _dispatch_install,
}


def main(argv: list[str] | None = None) -> int:
    """Canon: §Context — dispatch to status (this reader) / produce (context_producer.py) / install (setup_context_hook.py).

    No subcommand defaults to `status` (the common case: "what does the
    cipher read right now?").
    """
    raw = sys.argv[1:] if argv is None else list(argv)

    if raw and raw[0] in ("-h", "--help"):
        print("usage: context.py [status|produce|install] [args]")
        print("subcommands: " + ", ".join(_SUBCOMMANDS))
        print()
        print("  status                    print the context cipher line (default)")
        print("  produce [--stdin-file P]  write context.json from a hook payload (context_producer.py)")
        print("  install [--status|--off]  install/inspect/remove the local hook (setup_context_hook.py)")
        return 0

    if not raw or raw[0] not in _DISPATCH:
        # No recognized subcommand -> default to status (backward-friendly:
        # `python tools/context.py` alone just prints the cipher line).
        return _dispatch_status(raw)

    sub, rest = raw[0], raw[1:]
    return _DISPATCH[sub](rest)


if __name__ == "__main__":
    sys.exit(main())
