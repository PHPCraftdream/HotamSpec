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

HONESTY NOTE, UPDATED (2026-07-02): the globally-installed `clock` skill
(cah-stamp, ~/.claude/cah-bin/bin/cah-stamp.js and cah-status.js) computes its
context-usage percentage LIVE from the statusLine envelope on every render
but historically never persisted it — ~/.claude/cah-bin/cache/rate-limits.json
holds only {fiveHour, sevenDay, effort, capturedAt} (5h/weekly quota, not
context %). `spec/tools/setup_context_hook.py --patch-global --apply` is the
STEWARD-RUN tool that closes this gap: it patches the user's global
cah-status.js to also write ~/.claude/cah-bin/cache/context-cache.json =
{"ctx_pct", "model", "stamp"} (CACHE CONTRACT below). That patch is opt-in
and outside this framework's write authority — this producer only READS the
cache path if it exists; it never assumes the patch has been applied.

CACHE CONTRACT (what --patch-global writes, if applied):
    ~/.claude/cah-bin/cache/context-cache.json = {
      "ctx_pct": <float 0..100>,
      "model":   "<display name>",
      "stamp":   "<iso8601>"
    }
Overridable via env var CAH_CONTEXT_CACHE (mirrors cah-status.js's own
CAH_CONTEXT_CACHE env override, so tests can redirect both sides to the same
tmp fixture).

This producer looks for a usable `ctx_pct` in TWO places, in order:
  1. the incoming hook JSON payload itself (top-level numeric `ctx_pct`) —
     the honest, minimal contract a caller can satisfy directly on stdin;
  2. if absent, the global context-cache.json (see CACHE CONTRACT above),
     read ONLY if it exists and parses — never assumed present.
If neither yields a usable 0..100 float, the producer WRITES NOTHING
(fail-silent, matching the reader's honest-UNMEASURED default) rather than
guessing.

Run:
  echo '{"ctx_pct": 42.5, "model": "claude-sonnet-5"}' | uv run python tools/context_producer.py
  uv run python tools/context_producer.py --stdin-file payload.json
  # or, with no payload at all, after the user has run
  # `setup_context_hook.py --patch-global --apply`:
  echo '{}' | uv run python tools/context_producer.py   # falls back to the global cache
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path

_RUNTIME = Path(__file__).resolve().parents[1] / ".runtime" / "context.json"
_GLOBAL_CACHE = Path.home() / ".claude" / "cah-bin" / "cache" / "context-cache.json"


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


def _read_global_cache() -> dict:
    """Read the global context-cache.json (see CACHE CONTRACT), or {} if absent/unreadable.

    Path is overridable via CAH_CONTEXT_CACHE (matches cah-status.js's own
    env override), so tests can point both sides at the same tmp fixture
    without touching the real ~/.claude.
    """
    path = Path(os.environ.get("CAH_CONTEXT_CACHE", "") or _GLOBAL_CACHE)
    try:
        raw = path.read_text(encoding="utf-8")
        data = json.loads(raw)
        return data if isinstance(data, dict) else {}
    except (OSError, json.JSONDecodeError):
        return {}


def produce(payload: dict) -> bool:
    """Write spec/.runtime/context.json from payload (or the global cache) if a usable ctx_pct exists.

    Returns True if a stamp was written, False if skipped (no usable signal
    in either source). Fail-silent by design: an absent/invalid ctx_pct
    writes nothing rather than guessing (mirrors context.py's honest-
    UNMEASURED default).
    """
    ctx_pct = payload.get("ctx_pct")
    model = payload.get("model", "")
    if not isinstance(ctx_pct, (int, float)) or not (0 <= ctx_pct <= 100):
        cache = _read_global_cache()
        ctx_pct = cache.get("ctx_pct")
        model = cache.get("model", model)
        if not isinstance(ctx_pct, (int, float)) or not (0 <= ctx_pct <= 100):
            return False

    stamp = {
        "ctx_pct": float(ctx_pct),
        "model": str(model or ""),
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
