"""CLI wrapper for gate.py (hotam-gate entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import gate  # noqa: E402


def main() -> None:
    """Entry point — delegates to gate.main()."""
    gate.main()


if __name__ == "__main__":
    main()
