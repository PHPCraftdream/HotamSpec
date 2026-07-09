"""CLI wrapper for delegate.py (hotam-delegate entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("delegate")

if __name__ == "__main__":
    main()
