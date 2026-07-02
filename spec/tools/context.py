"""Canon: §Context — the operator's working-context measurement (reader).

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
Stop) do not receive context-window usage on stdin today; only the global
statusline pipeline sees it, and the user's global ~/.claude config is outside
the framework body. The requirement stays DRAFT until a hook can honestly
write this stamp.
"""

from __future__ import annotations
import json
from dataclasses import dataclass
from pathlib import Path

_RUNTIME = Path(__file__).resolve().parents[1] / ".runtime" / "context.json"


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


def render_line() -> str:
    """One-line context cipher for the LIVE-STATE block / tick."""
    s = read_context()
    if not s.measured or s.pct is None:
        return "context: UNMEASURED (R-measure-context-size; hook deferred)"
    suffix = f"{s.model} @ {s.stamp}" if s.stamp else s.model
    return f"context: {s.pct:.0f}% ({suffix})"
