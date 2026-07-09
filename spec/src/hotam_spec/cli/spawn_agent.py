"""CLI wrapper for spawn_agent.py (hotam-spawn-agent entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("spawn_agent")

if __name__ == "__main__":
    main()
