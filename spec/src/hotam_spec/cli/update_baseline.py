"""CLI wrapper for update_baseline.py (hotam-update-baseline entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import update_baseline  # noqa: E402


def main() -> None:
    """Entry point — delegates to update_baseline.main()."""
    update_baseline.main()


if __name__ == "__main__":
    main()
