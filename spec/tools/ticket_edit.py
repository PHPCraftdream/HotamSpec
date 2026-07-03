"""Canon: §Ticket — edit a ticket's title/body, snapshotting the prior text into History.

RULE: ticket_edit.py changes title and/or the description body, and BEFORE writing
the new text it records a "text changed" History entry snapshotting the OLD
value(s). WHY snapshot-into-History: the steward asked explicitly for "история
изменения текста" — the edit trail must survive the edit. Routing edits through
the tool is the only way to guarantee no text change is lost
(R-ticket-carries-history, R-ticket-mutation-via-tools-only). The description body
is the text ABOVE the ## Comments section; ## Comments and ## History are never
touched by an edit.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _ticket_store as ts  # noqa: E402


def _split_body(body: str) -> tuple[str, str]:
    """Return (description-part, tail) where tail starts at ## Comments."""
    marker = f"\n{ts.COMMENTS_HEADING}"
    idx = body.find(marker)
    if idx == -1:
        return body, ""
    return body[:idx], body[idx + 1 :]


def main(argv: list[str] | None = None) -> None:
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("id")
    p.add_argument("--title")
    p.add_argument("--body", help="new description text (replaces the description block)")
    p.add_argument("--actor", default="operator")
    args = p.parse_args(argv)
    if args.title is None and args.body is None:
        p.error("nothing to edit: pass --title and/or --body")

    ticket = ts.load(args.id)
    desc, tail = _split_body(ticket.body)
    snapshot_bits = []
    if args.title is not None:
        snapshot_bits.append(f"old-title={ticket.header['title']!r}")
        ticket.header["title"] = args.title
    if args.body is not None:
        old_desc = desc.strip()
        # strip a leading "# <title>" heading from the snapshot (kept in header)
        old_lines = old_desc.split("\n")
        if old_lines and old_lines[0].startswith("# "):
            old_desc = "\n".join(old_lines[1:]).strip()
        # preserve the FULL prior text verbatim, newlines escaped so it stays one line
        snapshot_bits.append("old-body=" + old_desc.replace("\n", "\\n"))
        new_title = ticket.header["title"]
        desc = f"# {new_title}\n\n{args.body.rstrip()}\n\n"
    elif args.title is not None:
        # keep body, but refresh the leading "# <title>" heading
        lines = desc.split("\n")
        if lines and lines[0].startswith("# "):
            lines[0] = f"# {args.title}"
            desc = "\n".join(lines)

    ticket.body = desc.rstrip("\n") + "\n\n" + tail.lstrip("\n")
    ticket.header["updated"] = ts.now_stamp()
    ts.append_history(
        ticket, actor=args.actor, action="text changed", detail="; ".join(snapshot_bits)
    )
    ts.save(ticket)
    sys.stdout.write(f"{args.id}: text updated\n")


if __name__ == "__main__":
    main()
