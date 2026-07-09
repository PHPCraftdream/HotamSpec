"""CLI wrapper for attention_hook.py (hotam-attention-hook entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("attention_hook")

if __name__ == "__main__":
    main()
