"""CLI wrapper for apply_proposal.py (hotam-apply-proposal entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import apply_proposal  # noqa: E402


def main() -> None:
    """Apply a steward-approved JSON proposal to the graph."""
    apply_proposal.main()


if __name__ == "__main__":
    main()
