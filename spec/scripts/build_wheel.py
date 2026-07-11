"""Atomic, self-verifying release-wheel builder for hotam-spec.

This is the ONE tool that produces a release wheel. It fuses the three steps
that were previously separate, hand-orchestrated actions — populate the tool
scripts into ``src/hotam_spec/_tools/``, run ``uv build --wheel``, and verify
the artifact — into a single subprocess that REFUSES to emit a wheel unless the
tool scripts actually made it into the archive.

WHY atomic-and-verified (R-wheel-build-atomic-verified): the ``uv_build``
backend only packages content under the module root (``src/hotam_spec/``). The
26 ``hotam-*`` CLI entry points import their tool scripts from a shipped
``_tools/`` directory that is populated OUT-OF-BAND, just before the build, by
``populate_tools.py``. Forgetting that manual step publishes a wheel that
imports cleanly but has zero working CLI commands for the consumer. The failure
is invisible in the repo (``_tools/`` is .gitignored) and only surfaces after a
consumer ``pip install``. So the release path must be a single command that
collects the set of ``hotam_spec/_tools/*.py`` member NAMES inside the produced
``.whl`` and compares that against the set of ``spec/tools/*.py`` file names on
disk — on any mismatch (missing, extra, or swapped names — not just a count
that happens to match) it deletes the wheel and exits non-zero, leaving no
broken artifact.

Usage (from spec/):
    python scripts/build_wheel.py                 # build into dist/
    python scripts/build_wheel.py --out-dir <dir> # build into <dir> (tests)

The ``_tools/`` copies are always removed again in a ``finally`` block, so the
canonical source of truth stays ``spec/tools/`` and the working tree is left
clean whether the build succeeded or failed.
"""

from __future__ import annotations

import argparse
import subprocess
import sys
import zipfile
from pathlib import Path

# Import populate/clean from the sibling script rather than re-implementing the
# copy logic (single source of truth for what lands in _tools/).
sys.path.insert(0, str(Path(__file__).resolve().parent))
import populate_tools  # noqa: E402

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_TOOLS_SRC = _SPEC_ROOT / "tools"
_WHEEL_TOOLS_PREFIX = "hotam_spec/_tools/"


def _disk_tool_names() -> set[str]:
    """Basenames of *.py tool scripts on disk (the source of truth in spec/tools/)."""
    return {p.name for p in _TOOLS_SRC.glob("*.py")}


def _wheel_tool_names(wheel_path: Path) -> set[str]:
    """Basenames of hotam_spec/_tools/*.py members inside the built wheel."""
    with zipfile.ZipFile(wheel_path) as zf:
        return {
            name[len(_WHEEL_TOOLS_PREFIX) :]
            for name in zf.namelist()
            if name.startswith(_WHEEL_TOOLS_PREFIX) and name.endswith(".py")
        }


def build_wheel(out_dir: Path) -> Path:
    """Populate, build, and verify a release wheel; return the .whl path.

    Raises SystemExit(1) — after deleting any produced wheel — if the SET of
    ``hotam_spec/_tools/*.py`` member names in the archive does not match the
    set of ``spec/tools/*.py`` file names on disk (not merely a matching count
    — a wheel carrying the right number of wrong-named files would otherwise
    pass). The ``_tools/`` copies are removed in a ``finally`` block regardless
    of outcome (R-wheel-build-atomic-verified).
    """
    out_dir.mkdir(parents=True, exist_ok=True)
    wheels_before = set(out_dir.glob("*.whl"))
    try:
        populate_tools.populate()

        build = subprocess.run(
            ["uv", "build", "--wheel", "--out-dir", str(out_dir), str(_SPEC_ROOT)],
            capture_output=True,
            text=True,
        )
        if build.returncode != 0:
            print(build.stdout)
            print(build.stderr, file=sys.stderr)
            raise SystemExit(1)

        new_wheels = sorted(set(out_dir.glob("*.whl")) - wheels_before)
        if len(new_wheels) != 1:
            print(
                f"build_wheel: expected exactly 1 new wheel, got {len(new_wheels)}: "
                f"{new_wheels}",
                file=sys.stderr,
            )
            for w in new_wheels:
                w.unlink()
            raise SystemExit(1)
        wheel_path = new_wheels[0]

        disk_names = _disk_tool_names()
        wheel_names = _wheel_tool_names(wheel_path)
        if wheel_names != disk_names:
            missing = sorted(disk_names - wheel_names)
            extra = sorted(wheel_names - disk_names)
            print(
                f"build_wheel: REFUSING artifact — wheel's {_WHEEL_TOOLS_PREFIX}*.py "
                f"members do not match spec/tools/*.py on disk (name-set mismatch, "
                f"not just a count). Missing from wheel: {missing}. Unexpected in "
                f"wheel: {extra}. The wheel is missing or mis-shipping tool scripts "
                f"and every hotam-* CLI would break for the consumer "
                f"(R-wheel-build-atomic-verified). Deleting {wheel_path.name}.",
                file=sys.stderr,
            )
            wheel_path.unlink()
            raise SystemExit(1)

        print(
            f"build_wheel: OK — {wheel_path.name} carries {len(wheel_names)} tool "
            f"scripts (name-set matches spec/tools/)."
        )
        return wheel_path
    finally:
        populate_tools.clean()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--out-dir",
        default=str(_SPEC_ROOT / "dist"),
        help="Directory to write the .whl into (default: spec/dist/).",
    )
    args = parser.parse_args()
    build_wheel(Path(args.out_dir).resolve())


if __name__ == "__main__":
    main()
