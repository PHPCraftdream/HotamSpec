"""CLI wrapper for create_agent.py (hotam-create-agent entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("create_agent")

if __name__ == "__main__":
    main()
