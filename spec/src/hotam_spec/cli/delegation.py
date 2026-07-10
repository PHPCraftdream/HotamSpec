"""CLI wrapper for delegation tools (hotam-delegation entry point with subcommands).

Subcommands: delegate (create/close/show/list).
Each delegates to the corresponding tool's main().
"""

from __future__ import annotations

import sys

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()


_SUBCOMMANDS = {
    "delegate": "delegate",
}


def main() -> None:
    """Dispatch to the delegation subcommand based on sys.argv[1]."""
    if len(sys.argv) < 2 or sys.argv[1] in ("-h", "--help"):
        print("usage: hotam-delegation <subcommand> [args]")
        print("subcommands: " + ", ".join(sorted(_SUBCOMMANDS)))
        raise SystemExit(0 if (len(sys.argv) >= 2 and sys.argv[1] in ("-h", "--help")) else 2)
    sub = sys.argv[1]
    module_name = _SUBCOMMANDS.get(sub)
    if module_name is None:
        print(f"error: unknown delegation subcommand '{sub}'", file=sys.stderr)
        print("available: " + ", ".join(sorted(_SUBCOMMANDS)), file=sys.stderr)
        raise SystemExit(2)
    sys.argv = [sys.argv[0]] + sys.argv[2:]
    import importlib  # noqa: PLC0415

    mod = importlib.import_module(module_name)
    mod.main()


if __name__ == "__main__":
    main()
