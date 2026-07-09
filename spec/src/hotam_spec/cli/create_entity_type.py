"""CLI wrapper for create_entity_type.py (hotam-create-entity-type entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import create_entity_type  # noqa: E402


def main() -> None:
    """Entry point — delegates to create_entity_type.main()."""
    create_entity_type.main()


if __name__ == "__main__":
    main()
