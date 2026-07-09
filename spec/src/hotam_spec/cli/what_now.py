"""CLI wrapper for what_now.py (hotam-what-now entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import what_now  # noqa: E402


def main() -> None:
    """Diagnose the graph and print the prioritized next-action list."""
    what_now.main()


if __name__ == "__main__":
    main()
