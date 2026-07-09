"""CLI wrapper for gate_status.py (hotam-gate-status entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("gate_status")

if __name__ == "__main__":
    main()
