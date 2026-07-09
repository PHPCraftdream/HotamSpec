"""CLI wrapper for attention.py (hotam-attention entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import attention  # noqa: E402


def main() -> None:
    """Entry point — delegates to attention.main()."""
    attention.main()


if __name__ == "__main__":
    main()
