"""CLI wrapper for create_agent.py (hotam-create-agent entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import create_agent  # noqa: E402


def main() -> None:
    """Entry point — delegates to create_agent.main()."""
    create_agent.main()


if __name__ == "__main__":
    main()
