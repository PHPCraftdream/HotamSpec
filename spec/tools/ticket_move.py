"""Canon: §Ticket — move a ticket to a new status (relocates the file + records the transition in History).

RULE: a status change is a FILE MOVE between tickets/<status>/ folders plus a
History line ("status: X→Y") and an `updated` bump. ticket_move.py is the only
writer of that transition. WHY move-not-flag: encoding status by folder makes the
kanban visible at the filesystem/git level, and forcing the move through the tool
guarantees the History trail the steward asked for is never skipped
(R-ticket-carries-history, R-ticket-mutation-via-tools-only).
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _ticket_store as ts  # noqa: E402


def main(argv: list[str] | None = None) -> None:
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("id")
    p.add_argument("status", choices=ts.STATUSES)
    p.add_argument("--actor", default="operator")
    args = p.parse_args(argv)

    ts.ensure_layout()
    ticket = ts.load(args.id)
    old = ticket.status
    if old == args.status:
        sys.stdout.write(f"{args.id} already {old}; no move\n")
        return
    old_path = ticket.path
    ticket.header["status"] = args.status
    ticket.header["updated"] = ts.now_stamp()
    ts.append_history(
        ticket, actor=args.actor, action="status", detail=f"{old}→{args.status}"
    )
    ticket.path = ts.status_dir(args.status) / f"{args.id}.md"
    ts.save(ticket)
    old_path.unlink()
    sys.stdout.write(f"{args.id}: {old} -> {args.status}\n")


if __name__ == "__main__":
    main()
