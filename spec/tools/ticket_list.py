"""Canon: §Ticket — list tickets, optionally filtered by status or assignee (read-only).

RULE: ticket_list.py is a pure reader that enumerates tickets across the status
folders, with optional --status / --assignee filters, printing one line per
ticket (id, status, assignee, title). WHY a dedicated lister: the steward's queue
now lives on disk; a single command that answers "what is where, and whose is it"
is the on-disk analogue of the chat backlog it replaces (R-open-tickets-visible).
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _ticket_store as ts  # noqa: E402


def main(argv: list[str] | None = None) -> None:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--status", choices=ts.STATUSES)
    p.add_argument("--assignee")
    args = p.parse_args(argv)

    statuses = [args.status] if args.status else list(ts.STATUSES)
    rows: list[tuple[str, str, str, str]] = []
    for s in statuses:
        d = ts.TICKETS_DIR / s
        if not d.exists():
            continue
        for path in sorted(d.glob("T-*.md")):
            try:
                ticket = ts.load(path.stem)
            except Exception:
                continue
            if args.assignee and ticket.header.get("assignee") != args.assignee:
                continue
            rows.append(
                (ticket.id, s, ticket.header.get("assignee", "?"), ticket.header.get("title", ""))
            )
    rows.sort(key=lambda r: int(r[0].split("-")[1]))
    if not rows:
        sys.stdout.write("(no tickets match)\n")
        return
    for tid, status, assignee, title in rows:
        sys.stdout.write(f"  {tid:<6} [{status:<11}] {assignee:<10} {title}\n")
    sys.stdout.write(f"\n{len(rows)} ticket(s)\n")


if __name__ == "__main__":
    main()
