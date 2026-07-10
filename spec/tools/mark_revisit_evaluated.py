"""Canon: §Conflict — record that a DECIDED conflict's revisit_marker was evaluated.

WHY this tool exists: a DECIDED Conflict carries a `revisit_marker` — the
historian's anti-relitigation trigger ("REVISIT if the DRAFT backlog grows
faster than it is built"). The marker names the CONDITION under which the
decision should be re-opened, but nothing tracked whether anyone ever LOOKED at
it again. An unread trigger is important-yet-invisible: the decision silently
ossifies while its stated revisit condition may already have come true.

This tool is the write-half of that visibility loop (the read-half is
tools/what_now.py's CLI-only revisit-evaluation band). Given a conflict id, it
appends a run stamp to spec/.runtime/revisit-eval.jsonl recording {stamp,
conflict, settled_count}. The harness reads the LAST evaluation per conflict and
re-surfaces a marker as "evaluate revisit marker" when it has NEVER been
evaluated, or when the graph has grown by more than the harness's staleness
delta of SETTLED atoms since the last evaluation — the same "you have not looked
lately" signal, per decision.

The tool is present-only over the graph: it NEVER edits graph.py. Evaluating a
marker is an OBSERVATION (the steward looked and judged the condition still
un-met); acting on it — re-opening the conflict — is a separate ProposedConflict
the steward drafts (R-ai-presents-not-decides).

Usage (from spec/):
  python tools/mark_revisit_evaluated.py C-06e2d84e
  python tools/mark_revisit_evaluated.py C-06e2d84e --note "backlog still shrinking"

Exit codes:
  0 — evaluation recorded (or --dry-run print).
  1 — the conflict id is unknown, or carries no revisit_marker to evaluate.
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

from hotam_spec.graph import load_content_graph  # noqa: E402
from hotam_spec.requirement import SETTLED  # noqa: E402
from hotam_spec.runtime_paths import runtime_dir as _runtime_dir  # noqa: E402

REVISIT_EVAL_FILE = _runtime_dir() / "revisit-eval.jsonl"


def append_evaluation(
    conflict_id: str, settled_count: int, *, note: str = "", path: Path = REVISIT_EVAL_FILE
) -> dict:
    """Append one {stamp, conflict, settled_count, note} evaluation record.

    Returns the written record. The file is append-only (mirrors
    tension-audit.jsonl): every evaluation is history, never overwritten.
    """
    record = {
        "stamp": datetime.now(timezone.utc).isoformat(),
        "conflict": conflict_id,
        "settled_count": settled_count,
    }
    if note:
        record["note"] = note
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8", newline="\n") as fh:
        fh.write(json.dumps(record) + "\n")
    return record


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("conflict_id", help="the C-… id whose revisit_marker was evaluated")
    parser.add_argument("--note", default="", help="optional steward note on the evaluation")
    parser.add_argument("--dry-run", action="store_true", help="print, do not append")
    args = parser.parse_args(argv)

    g = load_content_graph()
    conflict = next((c for c in g.conflicts if c.id == args.conflict_id), None)
    if conflict is None:
        print(f"ERROR: unknown conflict id '{args.conflict_id}'.", file=sys.stderr)
        return 1
    if not conflict.revisit_marker:
        print(
            f"ERROR: conflict '{args.conflict_id}' carries no revisit_marker to evaluate.",
            file=sys.stderr,
        )
        return 1

    settled_count = sum(1 for r in g.requirements if r.status == SETTLED)
    if args.dry_run:
        print(
            f"[dry-run] would record evaluation of {args.conflict_id} "
            f"at settled_count={settled_count}"
        )
        return 0
    record = append_evaluation(args.conflict_id, settled_count, note=args.note)
    print(
        f"recorded revisit-marker evaluation of {args.conflict_id} "
        f"(settled_count={record['settled_count']}) -> {REVISIT_EVAL_FILE}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
