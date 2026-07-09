"""CLI wrapper for gate.py (hotam-gate entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("gate")

if __name__ == "__main__":
    main()
