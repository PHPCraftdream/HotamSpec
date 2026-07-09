"""CLI wrapper for create_axis.py (hotam-create-axis entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import create_axis  # noqa: E402


def main() -> None:
    """Entry point — delegates to create_axis.main()."""
    create_axis.main()


if __name__ == "__main__":
    main()
