"""Canon: §Ticket — create a new on-disk ticket (auto-id, initial status, first History entry).

RULE: ticket_create.py is the ONLY sanctioned way to bring a ticket into
existence. It allocates the next T-<n> id, writes the file into
tickets/backlog/, and stamps the first History line ("created"). WHY a tool and
not a hand-written file: id allocation, the frontmatter shape, and the mandatory
first History entry must be identical for every ticket (R-ticket-mutation-via-tools-only) —
a hand-made ticket would drift from the machine header contract the reader tools rely on.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _ticket_store as ts  # noqa: E402


def main(argv: list[str] | None = None) -> None:
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--title", required=True)
    p.add_argument("--assignee", required=True)
    p.add_argument("--link", action="append", default=[], help="anchor (repeatable)")
    p.add_argument("--description", default="")
    p.add_argument("--actor", default="operator")
    args = p.parse_args(argv)

    ts.ensure_layout()
    tid = ts.next_id()
    ticket = ts.new_ticket(
        ticket_id=tid,
        title=args.title,
        assignee=args.assignee,
        links=args.link,
        description=args.description,
    )
    ts.append_history(
        ticket, actor=args.actor, action="created", detail=f"assignee={args.assignee}"
    )
    ts.save(ticket)
    try:
        shown = ticket.path.relative_to(ts.REPO_ROOT)
    except ValueError:
        shown = ticket.path
    sys.stdout.write(f"created {tid} -> {shown}\n")


if __name__ == "__main__":
    main()
