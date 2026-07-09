"""CLI wrapper for context_producer.py (hotam-context-producer entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("context_producer")

if __name__ == "__main__":
    main()
