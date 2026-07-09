"""CLI wrapper for claude_md_diff_watch.py (hotam-claude-md-diff-watch entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import claude_md_diff_watch  # noqa: E402


def main() -> None:
    """Entry point — delegates to claude_md_diff_watch.main()."""
    claude_md_diff_watch.main()


if __name__ == "__main__":
    main()
