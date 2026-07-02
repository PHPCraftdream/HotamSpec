"""Canon: §Closure — read spec/.runtime/land-log.jsonl and answer the commit-boundary question.

R-land-tier-trace gives apply_proposal.py's tiered LAND gate (tools/gate.py,
Stage B) a runtime trace: every applied proposal appends one JSONL record
naming its verify tier (T1 targeted-enforcer subset, or T2 full suite) to
spec/.runtime/land-log.jsonl. R-tiered-gate-not-a-commit-gate says the full
suite (T2) remains the mandatory gate at wave/commit boundaries — but that
claim was, until this tool, pure prose: nothing checked it. gate_status.py
makes the checkable slice of that claim mechanical: given the trace, has a
full T2 run landed AT OR AFTER the most recent T1-gated land? If yes, the
commit boundary is honestly covered by a full run; if a T1-gated land is
newer than the last T2, the steward is about to commit on the strength of
targeted subsets alone, which R-tiered-gate-not-a-commit-gate forbids.

This tool does NOT enforce anything by itself — it is advisory, read by a
human or a wave-closing script before `git commit`. It only ANSWERS the
question the trace can answer; it cannot know whether a commit is imminent,
and it does not replace running `uv run pytest -q` yourself.

Policy:
  - empty log (no file, or zero records)              -> boundary satisfied (exit 0)
  - log has entries; latest T2's stamp >= latest T1's stamp
    (or there is no T1 record at all)                 -> boundary satisfied (exit 0)
  - latest T1 stamp > latest T2 stamp (or T2 is absent
    while a T1 exists)                                -> boundary NOT satisfied (exit 1);
    prints the target(s) of every T1 record newer than the last T2 (or all
    T1 records, if no T2 exists at all)

Determinism: given a fixed land-log.jsonl, output is deterministic (records
are read in file order — an append-only log — and compared by ISO 8601
stamp string, which sorts correctly for same-timezone UTC timestamps as
written by apply_proposal.py's _append_land_log).

Run (from spec/):
  uv run python tools/gate_status.py                       # human-readable
  uv run python tools/gate_status.py --json                 # machine-readable
  uv run python tools/gate_status.py --log-path <path>      # override log location (tests)
"""

from __future__ import annotations

import argparse
import json
import sys
from dataclasses import dataclass, field
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_DEFAULT_LOG_PATH = _SPEC_ROOT / ".runtime" / "land-log.jsonl"


@dataclass(frozen=True)
class GateStatusResult:
    """Canon: §Closure — the outcome of a commit-boundary check over the land-log.

    Fields:
      satisfied        — True iff the boundary question holds (empty log, or a
                          full T2 run landed at-or-after the last T1-gated land).
      unverified_targets — target anchors of T1 records newer than the last T2
                          record (or ALL T1 records, if no T2 record exists at
                          all); empty when satisfied.
      reason            — human-readable explanation.
    """

    satisfied: bool
    unverified_targets: tuple[str, ...] = field(default_factory=tuple)
    reason: str = ""


def _read_records(log_path: Path) -> list[dict]:
    """Canon: §Closure — read land-log.jsonl records in file (append) order.

    Missing file -> empty list (an empty log is a legitimate, satisfied state
    — nothing has ever landed T1-only without a following T2). Malformed
    lines are skipped (best-effort read of a best-effort-written log; a
    single corrupt line must not crash the status check).
    """
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


def compute_gate_status(log_path: Path | None = None) -> GateStatusResult:
    """Canon: §Closure — answer 'has a full T2 landed at-or-after the last T1 land?'

    RULE: only records with tier == 'T1' or tier == 'T2' are considered.
    Records are compared by their 'stamp' field (ISO 8601 string; apply_proposal
    always writes UTC-aware isoformat, which sorts correctly lexicographically).
    """
    target = log_path if log_path is not None else _DEFAULT_LOG_PATH
    records = _read_records(target)

    t1_records = [r for r in records if r.get("tier") == "T1"]
    t2_records = [r for r in records if r.get("tier") == "T2"]

    if not records:
        return GateStatusResult(
            satisfied=True,
            reason="land-log is empty (no proposals landed yet) — boundary trivially satisfied.",
        )

    if not t1_records:
        return GateStatusResult(
            satisfied=True,
            reason="no T1-gated lands recorded — every land used the full suite (T2), or the log has none yet.",
        )

    latest_t1_stamp = max(r.get("stamp", "") for r in t1_records)

    if not t2_records:
        unverified = tuple(sorted({r.get("target", "?") for r in t1_records}))
        return GateStatusResult(
            satisfied=False,
            unverified_targets=unverified,
            reason=(
                "T1-gated land(s) exist but NO full T2 run has ever landed — "
                "the commit boundary is not covered by a full suite."
            ),
        )

    latest_t2_stamp = max(r.get("stamp", "") for r in t2_records)

    if latest_t2_stamp >= latest_t1_stamp:
        return GateStatusResult(
            satisfied=True,
            reason=(
                f"latest T2 land (stamp={latest_t2_stamp}) is at-or-after the "
                f"latest T1 land (stamp={latest_t1_stamp}) — boundary satisfied."
            ),
        )

    unverified = tuple(
        sorted(
            {
                r.get("target", "?")
                for r in t1_records
                if r.get("stamp", "") > latest_t2_stamp
            }
        )
    )
    return GateStatusResult(
        satisfied=False,
        unverified_targets=unverified,
        reason=(
            f"T1-gated land(s) newer than the last T2 run (last T2 "
            f"stamp={latest_t2_stamp}, newest T1 stamp={latest_t1_stamp}) — "
            "commit boundary NOT covered by a full suite run."
        ),
    )


def main(argv: list[str] | None = None) -> int:
    """Canon: §Closure — CLI entry point: print the commit-boundary status.

    Exit codes:
      0 — boundary satisfied (log empty, or a full T2 run landed at-or-after
          the last T1-gated land).
      1 — boundary NOT satisfied (a T1-gated land is newer than the last T2,
          or no T2 has ever landed while T1 records exist).
    """
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(
        description=(
            "Answer the commit-boundary question from spec/.runtime/land-log.jsonl: "
            "has a full T2 verification landed at-or-after the last T1-gated land?"
        )
    )
    parser.add_argument(
        "--log-path",
        default=None,
        help="Override the land-log.jsonl path (default: spec/.runtime/land-log.jsonl).",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Print machine-readable JSON instead of human-readable text.",
    )
    args = parser.parse_args(argv)

    log_path = Path(args.log_path) if args.log_path else None
    result = compute_gate_status(log_path)

    if args.json:
        print(
            json.dumps(
                {
                    "satisfied": result.satisfied,
                    "unverified_targets": list(result.unverified_targets),
                    "reason": result.reason,
                },
                ensure_ascii=False,
            )
        )
    else:
        status = "SATISFIED" if result.satisfied else "NOT SATISFIED"
        print(f"commit-boundary: {status}")
        print(f"reason: {result.reason}")
        if result.unverified_targets:
            print("unverified targets (T1-gated, no covering T2 since):")
            for t in result.unverified_targets:
                print(f"  - {t}")

    return 0 if result.satisfied else 1


if __name__ == "__main__":
    sys.exit(main())
