#!/usr/bin/env python
"""Build-time snapshot generator for the enforcer-name -> pytest node-id map.

§3.6 of the portability requirement: when the framework is pip-installed,
``spec/tests/`` is NOT shipped to the consumer, so the live-scan resolver
(enforcer_resolution.py) has no tests directory to walk. This script
generates a static snapshot of the CURRENT enforcer→node-id mapping by
running the existing resolver against ``spec/tests/`` and serializing the
result to ``hotam_spec/_data/enforcer_map.json``.

The snapshot is a FALLBACK layer (priority: self-hosting live-scan first,
then this package-data snapshot). It is regenerated at build time (or
manually by a framework maintainer after adding/renaming tests).

Run (from spec/):
    .venv/Scripts/python.exe tools/gen_enforcer_map.py

Output: src/hotam_spec/_data/enforcer_map.json (two dicts: func_index,
check_map — same shape as the .runtime/enforcer-index.json entries).
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(SPEC_ROOT / "tools"))
sys.path.insert(0, str(SPEC_ROOT / "src"))

import _bootstrap  # noqa: E402,F401  -- sys.path for hotam_spec

from hotam_spec.enforcer_resolution import _build_scan_uncached  # noqa: E402
from hotam_spec.repo_paths import tests_root  # noqa: E402


def main() -> None:
    """Generate _data/enforcer_map.json from the live spec/tests/ scan."""
    tests_dir = tests_root()
    if not tests_dir.exists():
        print(f"ERROR: tests dir not found at {tests_dir}", file=sys.stderr)
        raise SystemExit(1)

    scan = _build_scan_uncached(tests_dir)
    snapshot = {
        "func_index": dict(sorted(scan.func_index.items())),
        "check_map": {k: sorted(v) for k, v in sorted(scan.check_map.items())},
    }

    out_path = SPEC_ROOT / "src" / "hotam_spec" / "_data" / "enforcer_map.json"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(
        json.dumps(snapshot, indent=2, sort_keys=True, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )
    print(f"written: {out_path} ({len(scan.func_index)} funcs, {len(scan.check_map)} checks)")


if __name__ == "__main__":
    main()
