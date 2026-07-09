"""CLI wrapper for update_baseline.py (hotam-update-baseline entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("update_baseline")

if __name__ == "__main__":
    main()
