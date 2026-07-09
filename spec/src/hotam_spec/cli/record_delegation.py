"""CLI wrapper for record_delegation.py (hotam-record-delegation entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("record_delegation")

if __name__ == "__main__":
    main()
