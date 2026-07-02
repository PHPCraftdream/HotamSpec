"""Canon: §Agent — reads spec/.runtime/spawn-log.jsonl and flags mutating agents recorded without worktree isolation.

R-parallel-mutating-agents-use-worktree is a POLICY claim: "parallel mutating
agents shall use worktree isolation, not the shared working tree." The full
claim -- that two mutating agents actually ran CONCURRENTLY in the shared
tree -- is not mechanically checkable from spawn-log.jsonl alone: each record
carries only a `stamp` (the moment spawn_agent.py was invoked), never a
duration or an end time, so "overlapping in time" cannot be computed from
this log. Building that would require a second timestamp per record (a
future BUILD-TRIGGER, not fabricated here — R-uncrystallizable-is-missing-type
territory if ever pursued).

What IS honestly checkable today, and what this tool checks: whether ANY
spawn-log record was written with mutating=true and isolation="shared".
Every such record is a POTENTIAL violation of the policy (a mutating agent
that did NOT declare worktree isolation) -- it is a conservative structural
signal, not proof of an actual concurrent collision. An empty or absent log
is vacuously CLEAN (R-empty-content-wellformed: no data is not a violation).

This tool does NOT enforce anything by writing to the graph or blocking a
git operation -- it is a read-only structural check, run directly by
spec/tests/test_tool_spawn_log_isolation_status.py (function-level, not via
a live subprocess) so it can serve as the ENFORCED test for
R-parallel-mutating-agents-use-worktree AS SCOPED (the honest, log-internal
slice of the policy -- see module docstring above for exactly what is and is
not covered).

Run (from spec/):
  uv run python tools/spawn_log_isolation_status.py                  # human-readable
  uv run python tools/spawn_log_isolation_status.py --json           # machine-readable
  uv run python tools/spawn_log_isolation_status.py --log-path <p>   # override log location (tests)
"""

from __future__ import annotations

import argparse
import json
import sys
from dataclasses import dataclass, field
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_DEFAULT_LOG_PATH = _SPEC_ROOT / ".runtime" / "spawn-log.jsonl"


@dataclass(frozen=True)
class IsolationStatusResult:
    """Canon: §Agent — the outcome of a mutating/isolation scan over the spawn-log.

    Fields:
      clean            — True iff no record has mutating=true and isolation="shared".
      flagged_stamps   — stamp values of every offending record (empty when clean).
      reason           — human-readable explanation.
    """

    clean: bool
    flagged_stamps: tuple[str, ...] = field(default_factory=tuple)
    reason: str = ""


def _read_records(log_path: Path) -> list[dict]:
    """Canon: §Agent — read spawn-log.jsonl records in file (append) order.

    Missing file or empty file -> empty list (R-empty-content-wellformed: no
    entries is a legitimate, clean state). Malformed lines are skipped."""
    if not log_path.exists():
        return []
    records: list[dict] = []
    for line in log_path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            records.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    return records


def compute_isolation_status(log_path: Path | None = None) -> IsolationStatusResult:
    """Canon: §Agent — flag every record with mutating=true and isolation='shared'.

    RULE: a record is flagged iff record.get('mutating') is True (bool, not
    truthy string) AND record.get('isolation') == 'shared'. Records missing
    either field are treated as their documented default (mutating=False,
    isolation='shared') per R-spawn-log-carries-isolation, so a pre-Wave-5
    record (written before these fields existed) with mutating absent is
    NEVER flagged -- absence of the mutating field means "not declared
    mutating", not "assume worst case".
    """
    target = log_path if log_path is not None else _DEFAULT_LOG_PATH
    records = _read_records(target)

    if not records:
        return IsolationStatusResult(
            clean=True,
            reason="spawn-log is empty or absent — vacuously clean (R-empty-content-wellformed).",
        )

    flagged = tuple(
        r.get("stamp", "?")
        for r in records
        if r.get("mutating") is True and r.get("isolation", "shared") == "shared"
    )

    if not flagged:
        return IsolationStatusResult(
            clean=True,
            reason=f"{len(records)} spawn-log record(s) scanned — none are mutating=true with isolation=shared.",
        )

    return IsolationStatusResult(
        clean=False,
        flagged_stamps=flagged,
        reason=(
            f"{len(flagged)} of {len(records)} spawn-log record(s) are mutating=true "
            "with isolation=shared — R-parallel-mutating-agents-use-worktree (as "
            "scoped: log-internal signal only, not proof of concurrency) is not "
            "honored by these records."
        ),
    )


def main(argv: list[str] | None = None) -> int:
    """Canon: §Agent — CLI entry point: print the isolation-policy scan result.

    Exit codes:
      0 — clean (log empty, or no mutating=true/isolation=shared records).
      1 — at least one mutating=true/isolation=shared record found.
    """
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(
        description=(
            "Scan spec/.runtime/spawn-log.jsonl for mutating=true records "
            "written with isolation=shared (R-parallel-mutating-agents-use-worktree, "
            "log-internal slice only)."
        )
    )
    parser.add_argument(
        "--log-path",
        default=None,
        help="Override the spawn-log.jsonl path (default: spec/.runtime/spawn-log.jsonl).",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Print machine-readable JSON instead of human-readable text.",
    )
    args = parser.parse_args(argv)

    log_path = Path(args.log_path) if args.log_path else None
    result = compute_isolation_status(log_path)

    if args.json:
        print(
            json.dumps(
                {
                    "clean": result.clean,
                    "flagged_stamps": list(result.flagged_stamps),
                    "reason": result.reason,
                },
                ensure_ascii=False,
            )
        )
    else:
        status = "CLEAN" if result.clean else "FLAGGED"
        print(f"spawn-log isolation policy: {status}")
        print(f"reason: {result.reason}")
        if result.flagged_stamps:
            print("flagged records (mutating=true, isolation=shared) at stamps:")
            for s in result.flagged_stamps:
                print(f"  - {s}")

    return 0 if result.clean else 1


if __name__ == "__main__":
    sys.exit(main())
