"""CLI wrapper for audit_tensions.py (hotam-audit-tensions entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import audit_tensions  # noqa: E402


def main() -> None:
    """Entry point — delegates to audit_tensions.main()."""
    audit_tensions.main()


if __name__ == "__main__":
    main()
