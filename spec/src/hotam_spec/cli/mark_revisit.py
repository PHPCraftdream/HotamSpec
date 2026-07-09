"""CLI wrapper for mark_revisit_evaluated.py (hotam-mark-revisit entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import mark_revisit_evaluated as mark_revisit  # noqa: E402  -- module name != filename


def main() -> None:
    """Record that a DECIDED conflict's revisit_marker was evaluated."""
    mark_revisit.main()


if __name__ == "__main__":
    main()
