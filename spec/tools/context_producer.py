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

HONEST, LOCAL-ONLY SOURCE (R-work-within-launch-dir): the ONLY source of
ctx_pct is the incoming hook JSON payload on stdin (a top-level numeric
`ctx_pct`). The framework will NOT touch the host — it does not read, patch,
or depend on any global ~/.claude cache or the host statusline. If the local
stdin payload does not carry a usable 0..100 `ctx_pct`, the producer WRITES
NOTHING (fail-silent, matching the reader's honest-UNMEASURED default) rather
than guessing.

NOTE: Claude Code's project-local hook events do not, today, deliver
context-window usage on stdin — so in practice this producer stays silent and
the cipher stays honestly UNMEASURED. It writes only if a caller (test, or a
future host that honestly supplies ctx_pct on stdin) provides it.

Run:
  echo '{"ctx_pct": 42.5, "model": "claude-sonnet-5"}' | .venv/Scripts/python.exe tools/context_producer.py
  .venv/Scripts/python.exe tools/context_producer.py --stdin-file payload.json
"""

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
if str(_SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(_SPEC_ROOT / "src"))

from hotam_spec.runtime_paths import runtime_dir as _runtime_dir  # noqa: E402

_RUNTIME = _runtime_dir() / "context.json"


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
    """Write spec/.runtime/context.json from the stdin payload if a usable ctx_pct exists.

    Returns True if a stamp was written, False if skipped (no usable signal in
    the payload). Fail-silent by design: an absent/invalid ctx_pct writes
    nothing rather than guessing (mirrors context.py's honest-UNMEASURED
    default). The ONLY source is the local stdin payload — the framework never
    reads the host (R-work-within-launch-dir).
    """
    ctx_pct = payload.get("ctx_pct")
    model = payload.get("model", "")
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


def main(argv: list[str] | None = None) -> int:
    """CLI entry point. ``argv`` defaults to ``sys.argv[1:]`` (argparse's own
    default) but accepts an explicit list so `tools/context.py produce ...`
    can forward argv without touching sys.argv (same convention as
    tools/gate.py / tools/gate_status.py, forwarded by tools/land.py)."""
    parser = argparse.ArgumentParser(
        description="Write spec/.runtime/context.json from a hook JSON payload "
        "(stdin, or --stdin-file for testing/manual runs)."
    )
    parser.add_argument(
        "--stdin-file",
        default=None,
        help="Read the payload from this file instead of stdin (manual/test use).",
    )
    args = parser.parse_args(argv)

    payload = _read_payload(args.stdin_file)
    produce(payload)
    return 0


if __name__ == "__main__":
    sys.exit(main())
