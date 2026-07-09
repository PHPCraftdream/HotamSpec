"""CLI wrapper for emit_cipher.py (hotam-emit-cipher entry point)."""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import emit_cipher  # noqa: E402


def main() -> None:
    """Entry point — delegates to emit_cipher.main()."""
    emit_cipher.main()


if __name__ == "__main__":
    main()
