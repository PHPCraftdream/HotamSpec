"""CLI wrapper for apply_proposal.py (hotam-apply-proposal entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("apply_proposal")

if __name__ == "__main__":
    main()
