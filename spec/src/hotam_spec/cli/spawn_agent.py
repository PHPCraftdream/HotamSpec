"""CLI wrapper for spawn_agent.py (hotam-spawn-agent entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import spawn_agent  # noqa: E402


def main() -> None:
    """Entry point — delegates to spawn_agent.main()."""
    spawn_agent.main()


if __name__ == "__main__":
    main()
