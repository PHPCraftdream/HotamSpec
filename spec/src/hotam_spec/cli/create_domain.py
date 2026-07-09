"""CLI wrapper for create_domain.py (hotam-create-domain entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import create_domain  # noqa: E402


def main() -> None:
    """Scaffold a new business domain."""
    create_domain.main()


if __name__ == "__main__":
    main()
