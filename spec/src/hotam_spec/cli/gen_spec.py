"""CLI wrapper for gen_spec.py (hotam-gen-spec entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import gen_spec  # noqa: E402


def main() -> None:
    """Regenerate the human layer (CLAUDE.md, docs/gen/, atoms)."""
    gen_spec.main()


if __name__ == "__main__":
    main()
