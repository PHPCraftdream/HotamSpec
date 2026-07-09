"""CLI wrapper for gen_spec.py (hotam-gen-spec entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("gen_spec")

if __name__ == "__main__":
    main()
