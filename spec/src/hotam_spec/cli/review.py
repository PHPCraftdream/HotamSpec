"""CLI wrapper for review.py (hotam-review entry point: tensions|revisit|spawn-isolation).

review.py is a land.py-style dispatcher over the low-traffic review tools
(tools/audit_tensions.py, tools/mark_revisit_evaluated.py,
tools/spawn_log_isolation_status.py — task #106 / L2-#5). The three original
hotam-audit-tensions / hotam-mark-revisit / hotam-spawn-log-status entry
points stay registered as aliases (unchanged wrappers) for backward
compatibility — this wrapper only adds the new unified surface.
"""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import review  # noqa: E402


def main() -> None:
    """Entry point — delegates to review.main()."""
    raise SystemExit(review.main())


if __name__ == "__main__":
    main()
