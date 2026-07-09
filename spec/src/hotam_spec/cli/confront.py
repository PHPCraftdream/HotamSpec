"""CLI wrapper for confront.py (hotam-confront entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import confront  # noqa: E402


def main() -> None:
    """Entry point — delegates to confront.main()."""
    confront.main()


if __name__ == "__main__":
    main()
