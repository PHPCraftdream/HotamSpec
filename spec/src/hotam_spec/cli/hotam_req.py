"""CLI wrapper for hotam_req.py (hotam-req entry point: list|show|search|patch|context).

hotam_req.py is a land.py/review.py-style dispatcher over the requirement
browsing/patching subcommands (task #112 / Etap J). This wrapper only adds
the pip entry point surface.
"""

from __future__ import annotations

from hotam_spec.cli._path_setup import ensure_tools_on_path

ensure_tools_on_path()

import hotam_req  # noqa: E402


def main() -> None:
    """Entry point -- delegates to hotam_req.main()."""
    raise SystemExit(hotam_req.main())


if __name__ == "__main__":
    main()
