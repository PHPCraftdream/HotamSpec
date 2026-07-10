"""Canon: §Closure — single CLI entry point over the low-traffic review tools.

Three tools share one theme — "surface something for the steward to look at,
never write graph.py" — but almost no run-time traffic (spec/.runtime/ trace
counts at the 2026-07-10 census: audit_tensions 2 runs, revisit-eval 5,
spawn-log-isolation-status effectively test-only):

  - `tools/audit_tensions.py`             — deterministic, LLM-free shortlist
                                             of SETTLED requirement pairs that
                                             might hide an unmediated tension.
  - `tools/mark_revisit_evaluated.py`     — records that a DECIDED conflict's
                                             revisit_marker was looked at.
  - `tools/spawn_log_isolation_status.py` — scans spawn-log.jsonl for
                                             mutating=true records missing
                                             worktree isolation.

`review.py` does NOT reimplement or move any of that logic — it is a thin
dispatcher (`review.py tensions|revisit|spawn-isolation ...`) that imports the
three modules and forwards argv, exactly the choice `tools/land.py` made for
gate.py/gate_status.py/closure.py (see that module's docstring for the full
rationale). The three modules keep their own filenames and stay independently
importable un-merged because:

  - tests/test_tool_audit_tensions.py, tests/test_tool_mark_revisit_evaluated.py
    and tests/test_tool_spawn_log_isolation_status.py `import <module>` each
    tool directly and reference module-level names (`audit_tensions._SIG_MODAL`,
    `mark_revisit_evaluated.append_evaluation`, `spawn_log_isolation_status.
    compute_isolation_status`) — merging bodies would change what those tests
    import.
  - `domains/hotam-spec-self/graph.py` `enforced_by` entries cite
    `tests/test_tool_audit_tensions.py::...` and
    `tests/test_tool_mark_revisit_evaluated.py::...` node-ids, which resolve
    against the EXISTING test files; those are untouched by this dispatcher.

Run (from spec/):
  python tools/review.py tensions [--limit N]                 (audit_tensions.py)
  python tools/review.py revisit <conflict_id> [--note TEXT]  (mark_revisit_evaluated.py)
  python tools/review.py spawn-isolation [--json]              (spawn_log_isolation_status.py)
"""

from __future__ import annotations

import sys
from pathlib import Path

import _bootstrap  # noqa: E402,F401  -- side effect: configures sys.path for hotam_spec + tools

SPEC_ROOT = Path(__file__).resolve().parents[1]

_SUBCOMMANDS = ("tensions", "revisit", "spawn-isolation")


def _dispatch_tensions(argv: list[str]) -> int:
    """Forward to audit_tensions.py's own argparse CLI, unchanged."""
    import audit_tensions  # noqa: PLC0415  -- lives in tools/, not a package

    return audit_tensions.main(argv)


def _dispatch_revisit(argv: list[str]) -> int:
    """Forward to mark_revisit_evaluated.py's own argparse CLI, unchanged."""
    import mark_revisit_evaluated  # noqa: PLC0415  -- lives in tools/, not a package

    return mark_revisit_evaluated.main(argv)


def _dispatch_spawn_isolation(argv: list[str]) -> int:
    """Forward to spawn_log_isolation_status.py's own argparse CLI, unchanged."""
    import spawn_log_isolation_status  # noqa: PLC0415  -- lives in tools/, not a package

    return spawn_log_isolation_status.main(argv)


_DISPATCH = {
    "tensions": _dispatch_tensions,
    "revisit": _dispatch_revisit,
    "spawn-isolation": _dispatch_spawn_isolation,
}


def main(argv: list[str] | None = None) -> int:
    """Canon: §Closure — dispatch to tensions (audit_tensions.py) / revisit (mark_revisit_evaluated.py) / spawn-isolation (spawn_log_isolation_status.py)."""
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    raw = sys.argv[1:] if argv is None else list(argv)

    if not raw or raw[0] in ("-h", "--help"):
        print("usage: review.py <subcommand> [args]")
        print("subcommands: " + ", ".join(_SUBCOMMANDS))
        print()
        print("  tensions [--limit N]                generative-audit shortlist (audit_tensions.py)")
        print("  revisit <conflict_id> [--note TEXT]  record a revisit-marker evaluation (mark_revisit_evaluated.py)")
        print("  spawn-isolation [--json]             scan spawn-log for mutating/shared records (spawn_log_isolation_status.py)")
        return 0 if raw and raw[0] in ("-h", "--help") else 2

    sub, rest = raw[0], raw[1:]
    handler = _DISPATCH.get(sub)
    if handler is None:
        print(f"error: unknown review subcommand '{sub}'", file=sys.stderr)
        print("available: " + ", ".join(_SUBCOMMANDS), file=sys.stderr)
        return 2
    return handler(rest)


if __name__ == "__main__":
    sys.exit(main())
