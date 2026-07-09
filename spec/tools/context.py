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
Stop) do not receive context-window usage on stdin today; only the host
statusline pipeline sees it, and the host is sovereign — the framework will
NOT touch it (R-work-within-launch-dir). The requirement stays DRAFT until the
host honestly delivers ctx_pct on the local stdin payload.
"""

from __future__ import annotations
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
