"""Canon: §Context — the producer half of the context cipher, writing spec/.runtime/context.json.

Reads a Claude Code hook JSON payload from stdin (the same shape a
PostToolUse/Stop hook receives: {"transcript_path": ..., "model": {...}, ...})
and, IF the payload carries a usable numeric context-fullness signal, writes
`spec/.runtime/context.json` matching the pinned contract in
tests/test_tool_context.py:

    spec/.runtime/context.json = {
      "ctx_pct": <float 0..100>,   # working-context fullness — REQUIRED
      "model":   "<model id>",     # optional
      "stamp":   "<iso8601>"       # optional
    }

HONESTY NOTE (2026-07-02 investigation): the globally-installed `clock` skill
(cah-stamp, ~/.claude/cah-bin/bin/cah-stamp.js) computes its context-usage
percentage LIVE from the session transcript on every hook invocation and
NEVER persists it to any cache file — ~/.claude/cah-bin/cache/rate-limits.json
holds only {fiveHour, sevenDay, effort, capturedAt} (5h/weekly quota, not
context %), and ~/.claude/cah-bin/cache/last-stamp.json holds only a
{lastStampedAt, lastStampedRequestId} throttle marker. There is therefore NO
existing on-disk cache this producer can honestly "read" for a ctx_pct value.

This producer instead reads the SAME transcript-derived signal a project-local
hook payload would carry on stdin, computed independently here (not by
piggy-backing on a nonexistent cah-stamp cache file). It looks for a
top-level numeric `ctx_pct` (0..100) on the incoming JSON payload itself —
the honest, minimal contract a caller (or a future project-local hook wrapper)
can satisfy without this tool depending on cah-stamp's private cache layout.
If no such field is present, the producer WRITES NOTHING (fail-silent,
matching the reader's honest-UNMEASURED default) rather than guessing.

Run:
  echo '{"ctx_pct": 42.5, "model": "claude-sonnet-5"}' | uv run python tools/context_producer.py
  uv run python tools/context_producer.py --stdin-file payload.json
"""

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

_RUNTIME = Path(__file__).resolve().parents[1] / ".runtime" / "context.json"


def _read_payload(stdin_file: str | None) -> dict:
    """Read and parse the hook JSON payload from stdin or a file. Returns {} on failure."""
    try:
        if stdin_file:
            raw = Path(stdin_file).read_text(encoding="utf-8")
        else:
            raw = sys.stdin.read()
        if not raw.strip():
            return {}
        data = json.loads(raw)
        return data if isinstance(data, dict) else {}
    except (OSError, json.JSONDecodeError):
        return {}


def produce(payload: dict) -> bool:
    """Write spec/.runtime/context.json from payload if it carries a usable ctx_pct.

    Returns True if a stamp was written, False if skipped (no usable signal).
    Fail-silent by design: an absent/invalid ctx_pct writes nothing rather
    than guessing (mirrors context.py's honest-UNMEASURED default).
    """
    ctx_pct = payload.get("ctx_pct")
    if not isinstance(ctx_pct, (int, float)) or not (0 <= ctx_pct <= 100):
        return False

    stamp = {
        "ctx_pct": float(ctx_pct),
        "model": str(payload.get("model", "")),
        "stamp": datetime.now(timezone.utc).isoformat(timespec="seconds"),
    }

    _RUNTIME.parent.mkdir(parents=True, exist_ok=True)
    tmp = _RUNTIME.with_suffix(".json.tmp")
    tmp.write_text(json.dumps(stamp), encoding="utf-8")
    tmp.replace(_RUNTIME)
    return True


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Write spec/.runtime/context.json from a hook JSON payload "
        "(stdin, or --stdin-file for testing/manual runs)."
    )
    parser.add_argument(
        "--stdin-file",
        default=None,
        help="Read the payload from this file instead of stdin (manual/test use).",
    )
    args = parser.parse_args()

    payload = _read_payload(args.stdin_file)
    produce(payload)
    return 0


if __name__ == "__main__":
    sys.exit(main())
