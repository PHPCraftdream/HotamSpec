"""CLI wrapper for spawn_log_isolation_status.py (hotam-spawn-log-isolation-status entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import spawn_log_isolation_status  # noqa: E402


def main() -> None:
    """Entry point — delegates to spawn_log_isolation_status.main()."""
    spawn_log_isolation_status.main()


if __name__ == "__main__":
    main()
