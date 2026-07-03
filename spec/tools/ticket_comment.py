"""Canon: §Ticket — append a stamped comment to a ticket (and a History "commented" entry).

RULE: ticket_comment.py appends a timestamped line under ## Comments AND records a
"commented" History entry, then bumps `updated`. WHY through a tool: comments and
their History mirror must both be written atomically so the audit trail stays
complete (R-ticket-mutation-via-tools-only, R-ticket-carries-history); a
hand-typed comment would leave no History footprint.
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
    p.add_argument("text")
    p.add_argument("--actor", default="operator")
    args = p.parse_args(argv)

    ticket = ts.load(args.id)
    ts.append_comment(ticket, actor=args.actor, text=args.text)
    ticket.header["updated"] = ts.now_stamp()
    ts.append_history(ticket, actor=args.actor, action="commented")
    ts.save(ticket)
    sys.stdout.write(f"{args.id}: comment added\n")


if __name__ == "__main__":
    main()
