"""CLI wrapper for context_producer.py (hotam-context-producer entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import context_producer  # noqa: E402


def main() -> None:
    """Entry point — delegates to context_producer.main()."""
    context_producer.main()


if __name__ == "__main__":
    main()
