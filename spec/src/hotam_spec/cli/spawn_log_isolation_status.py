"""CLI wrapper for spawn_log_isolation_status.py (hotam-spawn-log-isolation-status entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("spawn_log_isolation_status")

if __name__ == "__main__":
    main()
