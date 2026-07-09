"""CLI wrapper for create_domain.py (hotam-create-domain entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("create_domain")

if __name__ == "__main__":
    main()
