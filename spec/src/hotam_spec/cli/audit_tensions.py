"""CLI wrapper for audit_tensions.py (hotam-audit-tensions entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("audit_tensions")

if __name__ == "__main__":
    main()
