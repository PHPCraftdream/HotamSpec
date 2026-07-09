"""Canon: §Closure — single CLI entry point over gate.py/gate_status.py/closure.py.

Three tools orbit one artifact (`spec/.runtime/land-log.jsonl`, resolved via
`hotam_spec.runtime_paths.runtime_dir()`) and one question — "is this write
safe to land, and is the commit boundary honestly covered?":

  - `tools/gate.py`        — T1 selector: which targeted pytest node-ids cover
                              a Requirement/Conflict's `enforced_by`, or
                              fail-closed to the full suite.
  - `tools/gate_status.py` — commit-boundary reader: has a full T2 run landed
                              at-or-after the last T1-gated land?
  - `tools/closure.py`     — per-action verify: did the proposal that motivated
                              a write actually remove its diagnosis?

`land.py` does NOT reimplement or move any of that logic — it is a thin
dispatcher (`land.py select|status|verify-closure ...`) that imports the three
modules and forwards argv. This is a deliberate choice over a physical merge:

  - `tools/gate.py` is one of the 5 files under the enforcement-perimeter hash
    pin (spec/tests/protected_baselines.json) and is imported BY NAME
    (`import gate as _gate`) from `tools/apply_proposal.py`, plus directly by
    `tests/test_tool_gate.py` and `src/hotam_spec/cli/gate.py`. Merging its
    body into a new module would change its import identity everywhere it is
    referenced and force a baseline rehash for zero behavioral gain.
  - `tools/closure.py` is imported the same way (`import closure`) from
    `apply_proposal.py`'s closure-check step.
  - Consolidating the CALLING SURFACE (one command instead of three) captures
    the actual ergonomic win from R-shared-tools-in-spec-tools without
    touching the sensitive files' content, their module identity, or their
    existing direct callers.

Run (from spec/):
  uv run python tools/land.py select R-smoke-test [--explain]
  uv run python tools/land.py status [--json] [--log-path PATH]
  uv run python tools/land.py verify-closure <target_anchor> <triggering_kind>

`select` and `status` forward exit codes and stdout/stderr formatting exactly
as `gate.py`/`gate_status.py` would standalone (this module parses only the
subcommand name; everything after it is handed to the target's own argparse
unchanged). `verify-closure` is a convenience wrapper: it loads the current
graph's proposal-shaped target via closure.check_closure and prints the
ClosureResult (closure.py itself exposes no CLI — apply_proposal.py is its
only in-process caller — so this is the first command-line surface for it).
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

import _bootstrap  # noqa: E402,F401  -- side effect: configures sys.path for hotam_spec + tools

SPEC_ROOT = Path(__file__).resolve().parents[1]

_SUBCOMMANDS = ("select", "status", "verify-closure")


def _dispatch_select(argv: list[str]) -> int:
    """Forward to gate.py's own argparse CLI, unchanged."""
    import gate  # noqa: PLC0415  -- lives in tools/, not a package

    return gate.main(argv)


def _dispatch_status(argv: list[str]) -> int:
    """Forward to gate_status.py's own argparse CLI, unchanged."""
    import gate_status  # noqa: PLC0415  -- lives in tools/, not a package

    return gate_status.main(argv)


def _dispatch_verify_closure(argv: list[str]) -> int:
    """Run closure.check_closure for a target anchor + triggering kind.

    closure.py has no CLI of its own (only check_closure(proposal,
    triggering_kind), called in-process by apply_proposal.py with a live
    Proposal object). This wraps it with a minimal argparse surface driven
    by the already-applied graph, for standalone/manual use.
    """
    import closure  # noqa: PLC0415  -- lives in tools/, not a package
    from hotam_spec.proposal import ProposedRequirement  # noqa: PLC0415

    parser = argparse.ArgumentParser(
        prog="land.py verify-closure",
        description=(
            "Check whether a target anchor's triggering action is gone from "
            "diagnose() after a write (the P4 feedback edge, R-verify-closure-per-action)."
        ),
    )
    parser.add_argument("target_anchor", help="Requirement/Conflict id the write targeted.")
    parser.add_argument(
        "triggering_kind",
        help="The original action's kind (e.g. OPEN_ITEM, CONFLICT_STALLED, STRUCTURE, DRIFT_FALLOUT).",
    )
    args = parser.parse_args(argv)

    # check_closure only reads proposal.target_anchor(); a minimal stand-in
    # proposal carrying just that id is sufficient for a standalone check.
    stub_proposal = ProposedRequirement(
        id=args.target_anchor,
        claim="(standalone land.py verify-closure invocation — not a real write)",
        owner="",
        status="DRAFT",
        why="(standalone check — closure.check_closure only reads target_anchor())",
    )
    result = closure.check_closure(stub_proposal, args.triggering_kind)
    print(f"advanced        : {result.advanced}")
    print(f"target          : {result.target}")
    print(f"triggering_kind : {result.triggering_kind}")
    print(f"still_open      : {result.still_open_count}")
    print(f"note            : {result.note}")
    return 0 if result.advanced else 2


_DISPATCH = {
    "select": _dispatch_select,
    "status": _dispatch_status,
    "verify-closure": _dispatch_verify_closure,
}


def main(argv: list[str] | None = None) -> int:
    """Canon: §Closure — dispatch to select (gate.py) / status (gate_status.py) / verify-closure (closure.py)."""
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    raw = sys.argv[1:] if argv is None else list(argv)

    if not raw or raw[0] in ("-h", "--help"):
        print("usage: land.py <subcommand> [args]")
        print("subcommands: " + ", ".join(_SUBCOMMANDS))
        print()
        print("  select <target> [--explain]              T1 targeted-enforcer selection (gate.py)")
        print("  status [--json] [--log-path PATH]         commit-boundary check (gate_status.py)")
        print("  verify-closure <target> <triggering_kind> per-action closure check (closure.py)")
        return 0 if raw and raw[0] in ("-h", "--help") else 2

    sub, rest = raw[0], raw[1:]
    handler = _DISPATCH.get(sub)
    if handler is None:
        print(f"error: unknown land subcommand '{sub}'", file=sys.stderr)
        print("available: " + ", ".join(_SUBCOMMANDS), file=sys.stderr)
        return 2
    return handler(rest)


if __name__ == "__main__":
    sys.exit(main())
