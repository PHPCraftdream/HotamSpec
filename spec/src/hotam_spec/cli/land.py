"""CLI wrapper for land.py (hotam-land entry point with subcommands).

Subcommands: select (gate.py T1 selection), status (gate_status.py
commit-boundary check), verify-closure (closure.py per-action check).
"""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import land  # noqa: E402


def main() -> None:
    """Entry point — delegates to land.main()."""
    raise SystemExit(land.main())


if __name__ == "__main__":
    main()
