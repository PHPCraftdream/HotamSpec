"""CLI wrapper for setup_context_hook.py (hotam-setup-context-hook entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import setup_context_hook  # noqa: E402


def main() -> None:
    """Entry point — delegates to setup_context_hook.main()."""
    setup_context_hook.main()


if __name__ == "__main__":
    main()
