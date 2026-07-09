"""CLI wrapper for ticket tools (hotam-ticket entry point with subcommands).

Subcommands: create, list, show, move, edit, comment. Each delegates to the
corresponding tools/ticket_<sub>.py main().
"""

from __future__ import annotations

import sys

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()


_SUBCOMMANDS = {
    "create": "ticket_create",
    "list": "ticket_list",
    "show": "ticket_show",
    "move": "ticket_move",
    "edit": "ticket_edit",
    "comment": "ticket_comment",
}


def main() -> None:
    """Dispatch to the ticket subcommand based on sys.argv[1]."""
    if len(sys.argv) < 2 or sys.argv[1] in ("-h", "--help"):
        print("usage: hotam-ticket <subcommand> [args]")
        print("subcommands: " + ", ".join(sorted(_SUBCOMMANDS)))
        raise SystemExit(0 if (len(sys.argv) >= 2 and sys.argv[1] in ("-h", "--help")) else 2)
    sub = sys.argv[1]
    module_name = _SUBCOMMANDS.get(sub)
    if module_name is None:
        print(f"error: unknown ticket subcommand '{sub}'", file=sys.stderr)
        print("available: " + ", ".join(sorted(_SUBCOMMANDS)), file=sys.stderr)
        raise SystemExit(2)
    # Strip the subcommand from argv so the tool's own argparse sees only its args.
    sys.argv = [sys.argv[0]] + sys.argv[2:]
    import importlib  # noqa: PLC0415

    mod = importlib.import_module(module_name)
    mod.main()


if __name__ == "__main__":
    main()
