"""CLI wrapper for claude_md_diff_watch.py (hotam-claude-md-diff-watch entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("claude_md_diff_watch")

if __name__ == "__main__":
    main()
