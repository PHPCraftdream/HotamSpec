"""CLI wrapper for attention.py (hotam-attention entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("attention")

if __name__ == "__main__":
    main()
