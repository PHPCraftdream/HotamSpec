"""CLI wrapper for mark_revisit_evaluated.py (hotam-mark-revisit entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

# tool module filename (mark_revisit_evaluated.py) != this wrapper's short name
main = make_main("mark_revisit_evaluated")

if __name__ == "__main__":
    main()
