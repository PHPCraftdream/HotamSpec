"""CLI wrapper for setup_hooks.py (hotam-setup-hooks entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import setup_hooks  # noqa: E402


def main() -> None:
    """Entry point — delegates to setup_hooks.main()."""
    setup_hooks.main()


if __name__ == "__main__":
    main()
