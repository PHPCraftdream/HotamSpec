"""CLI wrapper for what_now.py (hotam-what-now entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("what_now")

if __name__ == "__main__":
    main()
