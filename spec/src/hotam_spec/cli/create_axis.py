"""CLI wrapper for create_axis.py (hotam-create-axis entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("create_axis")

if __name__ == "__main__":
    main()
