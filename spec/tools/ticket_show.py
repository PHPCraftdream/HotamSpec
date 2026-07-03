"""Canon: §Ticket — print one ticket's header, body, comments and full History (read-only).

RULE: ticket_show.py is a pure reader — it never mutates. It resolves a ticket by
id and prints the machine header plus the human body verbatim, so an operator can
inspect state without opening the file by hand. WHY a reader tool exists beside the
mutators: reading is safe and frequent; keeping it a separate no-write tool means
the mutating tools stay the sole writers (R-ticket-mutation-via-tools-only).
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _ticket_store as ts  # noqa: E402


def main(argv: list[str] | None = None) -> None:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("id")
    p.add_argument("--json", action="store_true", help="print header as JSON only")
    args = p.parse_args(argv)

    ticket = ts.load(args.id)
    if args.json:
        sys.stdout.write(json.dumps(ticket.header, indent=2, ensure_ascii=False) + "\n")
        return
    h = ticket.header
    sys.stdout.write(
        f"{h['id']}  [{h['status']}]  {h['title']}\n"
        f"  assignee: {h['assignee']}   updated: {h['updated']}\n"
        f"  links: {', '.join(h.get('links') or []) or '-'}\n\n"
    )
    sys.stdout.write(ticket.body.rstrip("\n") + "\n")


if __name__ == "__main__":
    main()
