"""CLI wrapper for gate_status.py (hotam-gate-status entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import gate_status  # noqa: E402


def main() -> None:
    """Entry point — delegates to gate_status.main()."""
    gate_status.main()


if __name__ == "__main__":
    main()
