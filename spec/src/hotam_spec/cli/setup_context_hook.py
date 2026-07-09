"""CLI wrapper for setup_context_hook.py (hotam-setup-context-hook entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("setup_context_hook")

if __name__ == "__main__":
    main()
