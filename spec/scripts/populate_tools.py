"""Populate spec/src/hotam_spec/_tools/ from spec/tools/ before building a wheel.

The uv_build backend only packages content under the module root (src/hotam_spec/).
The CLI entry points need the tool scripts at runtime, so they must be inside the
wheel.  This script copies every *.py file from spec/tools/ into
src/hotam_spec/_tools/ (a non-package directory — no __init__.py), preserving the
flat namespace the tools expect (they import each other as bare modules via
sys.path, not as package members).

Usage (from spec/):
    python scripts/populate_tools.py        # copy
    python scripts/populate_tools.py --clean  # remove copies

The _tools/ directory is .gitignored (except for the .gitignore itself), so the
copies never appear in version control — the canonical source of truth remains
spec/tools/.
"""

from __future__ import annotations

import argparse
import shutil
import sys
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_TOOLS_SRC = _SPEC_ROOT / "tools"
_TOOLS_DST = _SPEC_ROOT / "src" / "hotam_spec" / "_tools"


def populate() -> None:
    """Copy all *.py from tools/ to _tools/."""
    _TOOLS_DST.mkdir(parents=True, exist_ok=True)
    count = 0
    for py in sorted(_TOOLS_SRC.glob("*.py")):
        dst = _TOOLS_DST / py.name
        shutil.copy2(py, dst)
        count += 1
    print(f"populate_tools: copied {count} files to {_TOOLS_DST}")


def clean() -> None:
    """Remove all *.py from _tools/ (keep .gitignore)."""
    count = 0
    for py in _TOOLS_DST.glob("*.py"):
        py.unlink()
        count += 1
    print(f"populate_tools: removed {count} files from {_TOOLS_DST}")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--clean",
        action="store_true",
        help="Remove copies instead of creating them.",
    )
    args = parser.parse_args()
    if args.clean:
        clean()
    else:
        populate()


if __name__ == "__main__":
    main()
