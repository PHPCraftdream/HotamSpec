"""CLI wrapper for audit_atomicity.py (hotam-audit-atomicity entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import audit_atomicity  # noqa: E402


def main() -> None:
    """Entry point — delegates to audit_atomicity.main()."""
    audit_atomicity.main()


if __name__ == "__main__":
    main()
