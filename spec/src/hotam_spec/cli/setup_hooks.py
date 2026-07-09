"""CLI wrapper for setup_hooks.py (hotam-setup-hooks entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("setup_hooks")

if __name__ == "__main__":
    main()
