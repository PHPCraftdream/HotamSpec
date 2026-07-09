"""CLI wrapper for tick.py (hotam-tick entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import tick  # noqa: E402


def main() -> None:
    """Entry point — delegates to tick.main()."""
    tick.main()


if __name__ == "__main__":
    main()
