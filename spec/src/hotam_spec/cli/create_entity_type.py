"""CLI wrapper for create_entity_type.py (hotam-create-entity-type entry point)."""

from __future__ import annotations

from hotam_spec.cli._dispatch import make_main

main = make_main("create_entity_type")

if __name__ == "__main__":
    main()
