"""CLI wrapper for context.py (hotam-context entry point: status|produce|install).

context.py is itself a thin land.py-style dispatcher over the context-cipher
chain (this reader + tools/context_producer.py + tools/setup_context_hook.py,
task #106 / L2-#4) — this wrapper just exposes it as a pip entry point.
"""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import context  # noqa: E402


def main() -> None:
    """Entry point — delegates to context.main()."""
    raise SystemExit(context.main())


if __name__ == "__main__":
    main()
