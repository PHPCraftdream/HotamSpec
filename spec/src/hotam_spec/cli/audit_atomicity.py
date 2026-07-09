"""CLI wrapper for audit_atomicity.py (hotam-audit-atomicity entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("audit_atomicity")

if __name__ == "__main__":
    main()
