"""Canon: §Ticket (sibling) -- file-based delegation tool (create / close / show / list).

RULE: delegate.py is the ONLY sanctioned way to create and close delegation
files under delegations/DG-<n>.md. WHY a tool: the steward's verdict
(2026-07-05, verbatim: "давай делегировать все задачи через файлы, и вести их
историю в гите") requires every agent hand-off to be a versioned file whose
git history carries who/when/what. Routing mutations through a single tool
guarantees the header shape and the Result section are always consistent
(R-delegation-is-a-file). Read-only subcommands (show, list) never mutate.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import _delegation_store as ds  # noqa: E402


def _cmd_create(args: argparse.Namespace) -> None:
    ds.ensure_layout()
    did = ds.next_id()
    delegation = ds.new_delegation(
        delegation_id=did,
        date=args.date or ds.now_stamp()[:10],
        from_=args.from_,
        to=args.to,
        task=args.task,
        boundaries=args.boundaries,
        expected_return=args.expected,
    )
    ds.save(delegation)
    try:
        shown = delegation.path.relative_to(ds.REPO_ROOT)
    except ValueError:
        shown = delegation.path
    sys.stdout.write(f"created {did} -> {shown}\n")


def _cmd_close(args: argparse.Namespace) -> None:
    delegation = ds.load(args.id)
    if delegation.status == "done":
        sys.stderr.write(f"{args.id} already closed\n")
        sys.exit(1)
    delegation.header["status"] = "done"
    delegation.header["result"] = args.result
    if args.commit:
        delegation.header["result_commit"] = args.commit
    # Replace the Result section in body
    delegation.body = re.sub(
        r"(## Result\n\n).*",
        rf"\g<1>{args.result}\n",
        delegation.body,
        count=1,
        flags=re.DOTALL,
    )
    ds.save(delegation)
    sys.stdout.write(f"{args.id}: closed\n")


def _cmd_show(args: argparse.Namespace) -> None:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    delegation = ds.load(args.id)
    if args.json:
        sys.stdout.write(
            json.dumps(delegation.header, indent=2, ensure_ascii=False) + "\n"
        )
        return
    h = delegation.header
    sys.stdout.write(
        f"{h['id']}  [{h['status']}]  {h['task']}\n"
        f"  from: {h['from']}  to: {h['to']}  date: {h['date']}\n\n"
    )
    sys.stdout.write(delegation.body.rstrip("\n") + "\n")


def _cmd_list(args: argparse.Namespace) -> None:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    ids = ds.all_ids()
    if not ids:
        sys.stdout.write("(no delegations)\n")
        return
    ids.sort(key=lambda i: int(i.split("-")[1]))
    rows = []
    for did in ids:
        try:
            d = ds.load(did)
        except Exception:
            continue
        if args.status and d.status != args.status:
            continue
        rows.append((d.id, d.status, d.header.get("to", "?"), d.header.get("task", "")))
    if not rows:
        sys.stdout.write("(no delegations match)\n")
        return
    for did, status, to, task in rows:
        sys.stdout.write(f"  {did:<6} [{status:<4}] -> {to:<10} {task}\n")
    sys.stdout.write(f"\n{len(rows)} delegation(s)\n")


def main(argv: list[str] | None = None) -> None:
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    sub = p.add_subparsers(dest="command", required=True)

    cr = sub.add_parser("create")
    cr.add_argument("--to", required=True, help="agent type, e.g. o46l")
    cr.add_argument("--task", required=True, help="one-line task summary")
    cr.add_argument("--boundaries", required=True, help="what can/cannot be done")
    cr.add_argument("--expected", required=True, help="expected return")
    cr.add_argument("--from", dest="from_", default="coordinator")
    cr.add_argument("--date", default="", help="YYYY-MM-DD (default: today)")

    cl = sub.add_parser("close")
    cl.add_argument("id")
    cl.add_argument("--result", required=True)
    cl.add_argument("--commit", default="", help="result commit sha (optional)")

    sh = sub.add_parser("show")
    sh.add_argument("id")
    sh.add_argument("--json", action="store_true")

    ls = sub.add_parser("list")
    ls.add_argument("--status", choices=ds.VALID_STATUSES)

    args = p.parse_args(argv)
    {"create": _cmd_create, "close": _cmd_close, "show": _cmd_show, "list": _cmd_list}[
        args.command
    ](args)


if __name__ == "__main__":
    main()
