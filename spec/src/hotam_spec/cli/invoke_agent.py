"""CLI wrapper for invoke_agent.py (hotam-invoke-agent entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import invoke_agent  # noqa: E402


def main() -> None:
    """Entry point — delegates to invoke_agent.main()."""
    invoke_agent.main()


if __name__ == "__main__":
    main()
