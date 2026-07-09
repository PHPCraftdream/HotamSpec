"""CLI wrapper for record_delegation.py (hotam-record-delegation entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import record_delegation  # noqa: E402


def main() -> None:
    """Entry point — delegates to record_delegation.main()."""
    record_delegation.main()


if __name__ == "__main__":
    main()
