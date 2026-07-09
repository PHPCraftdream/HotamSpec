"""CLI wrapper for delegate.py (hotam-delegate entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import delegate  # noqa: E402


def main() -> None:
    """Entry point — delegates to delegate.main()."""
    delegate.main()


if __name__ == "__main__":
    main()
