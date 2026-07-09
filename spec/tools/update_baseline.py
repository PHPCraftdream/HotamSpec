"""Canon: §Invariants -- sanctioned baseline updater for enforcement-perimeter files.

The PreToolUse guard (_graph_guard.py) denies direct Edit/Write to
spec/tests/*_baseline.json (R-enforcement-perimeter-baselines-guarded).
This tool is the ONLY sanctioned path to update those baselines:

  - frozen_aspects_baseline.json: recomputes sha256 hashes of frozen-aspect files.
  - atomicity_compound_baseline.json: recomputes COMPOUND sets from audit_atomicity.
  - enforcement_perimeter_baseline.json: recomputes sha256 hashes of enforcement code.

Every update prints a human-readable diff (old hash -> new hash) so the change
is VISIBLE in the tool output and the commit message. The guard does not block
THIS tool because it writes via Python I/O, not via Claude's Edit/Write tools.

Usage (from spec/):
  .venv/Scripts/python.exe tools/update_baseline.py frozen_aspects
  .venv/Scripts/python.exe tools/update_baseline.py atomicity
  .venv/Scripts/python.exe tools/update_baseline.py enforcement_perimeter
  .venv/Scripts/python.exe tools/update_baseline.py --all
  .venv/Scripts/python.exe tools/update_baseline.py --set-frozen-aspects-comment "..."
"""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_TESTS_DIR = _SPEC_ROOT / "tests"
_SRC = _SPEC_ROOT / "src"
_TOOLS = _SPEC_ROOT / "tools"

if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))


def _sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def _update_frozen_aspects() -> bool:
    """Recompute frozen_aspects_baseline.json from current file hashes."""
    baseline_path = _TESTS_DIR / "frozen_aspects_baseline.json"
    data = json.loads(baseline_path.read_text(encoding="utf-8"))
    files = data["files"]
    changed = False
    for rel_path in list(files.keys()):
        full = _SPEC_ROOT / rel_path
        if not full.exists():
            print(f"  WARNING: {rel_path} no longer exists")
            continue
        new_hash = _sha256(full)
        if new_hash != files[rel_path]:
            print(f"  {rel_path}: {files[rel_path][:16]}... -> {new_hash[:16]}...")
            files[rel_path] = new_hash
            changed = True
    if changed:
        baseline_path.write_text(
            json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8"
        )
        print("  WRITTEN frozen_aspects_baseline.json")
    else:
        print("  frozen_aspects_baseline.json: no changes")
    return changed


def _set_frozen_aspects_comment(new_comment: str) -> bool:
    """Replace the `_comment` field of frozen_aspects_baseline.json verbatim.

    Sanctioned write path (same guard-bypass rationale as _update_frozen_aspects:
    this writes via Python I/O, not Claude's Edit/Write tools). Used to keep the
    guarded JSON's inline comment SHORT — a fixed pointer to the full narrative
    history, which lives in a hand-authored doc outside the enforcement
    perimeter (docs/development/FROZEN-ASPECTS-HISTORY.md) rather than growing
    unboundedly inline in a machine-checked baseline file.
    """
    baseline_path = _TESTS_DIR / "frozen_aspects_baseline.json"
    data = json.loads(baseline_path.read_text(encoding="utf-8"))
    old_comment = data.get("_comment", "")
    if old_comment == new_comment:
        print("  frozen_aspects_baseline.json: _comment unchanged")
        return False
    data["_comment"] = new_comment
    baseline_path.write_text(
        json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    print(f"  _comment: {len(old_comment)} chars -> {len(new_comment)} chars")
    print("  WRITTEN frozen_aspects_baseline.json (_comment only)")
    return True


def _update_atomicity() -> bool:
    """Recompute atomicity_compound_baseline.json from audit_atomicity."""
    import audit_atomicity as aa
    from hotam_spec.graph import load_content_graph
    from hotam_spec.invariants import ALL_INVARIANTS

    g = load_content_graph()
    compound_reqs = sorted(
        r.id
        for r in g.requirements
        if (r.status == "SETTLED" or r.status.startswith("OPEN"))
        and aa._audit_claim(r.claim)[0] == "COMPOUND"
    )
    compound_checks = sorted(
        func.__name__
        for func in ALL_INVARIANTS
        if aa._audit_invariant(func)[0] == "COMPOUND"
    )

    baseline_path = _TESTS_DIR / "atomicity_compound_baseline.json"
    data = json.loads(baseline_path.read_text(encoding="utf-8"))
    old_reqs = sorted(data.get("requirements", []))
    old_checks = sorted(data.get("invariants", []))

    changed = old_reqs != compound_reqs or old_checks != compound_checks
    if changed:
        if set(compound_reqs) - set(old_reqs):
            print(f"  NEW compound reqs: {set(compound_reqs) - set(old_reqs)}")
        if set(compound_checks) - set(old_checks):
            print(f"  NEW compound checks: {set(compound_checks) - set(old_checks)}")
        data["requirements"] = compound_reqs
        data["invariants"] = compound_checks
        baseline_path.write_text(
            json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8"
        )
        print("  WRITTEN atomicity_compound_baseline.json")
    else:
        print("  atomicity_compound_baseline.json: no changes")
    return changed


def _update_enforcement_perimeter() -> bool:
    """Recompute enforcement_perimeter_baseline.json from current file hashes."""
    baseline_path = _TESTS_DIR / "enforcement_perimeter_baseline.json"
    data = json.loads(baseline_path.read_text(encoding="utf-8"))
    files = data["files"]
    changed = False
    for rel_path in list(files.keys()):
        full = _SPEC_ROOT / rel_path
        if not full.exists():
            print(f"  WARNING: {rel_path} no longer exists")
            continue
        new_hash = _sha256(full)
        if new_hash != files[rel_path]:
            print(f"  {rel_path}: {files[rel_path][:16]}... -> {new_hash[:16]}...")
            files[rel_path] = new_hash
            changed = True
    if changed:
        baseline_path.write_text(
            json.dumps(data, indent=2, sort_keys=True) + "\n", encoding="utf-8"
        )
        print("  WRITTEN enforcement_perimeter_baseline.json")
    else:
        print("  enforcement_perimeter_baseline.json: no changes")
    return changed


_UPDATERS = {
    "frozen_aspects": _update_frozen_aspects,
    "atomicity": _update_atomicity,
    "enforcement_perimeter": _update_enforcement_perimeter,
}


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Sanctioned baseline updater (R-enforcement-perimeter-baselines-guarded)."
    )
    parser.add_argument(
        "baseline",
        nargs="?",
        choices=list(_UPDATERS.keys()),
        help="Which baseline to update.",
    )
    parser.add_argument("--all", action="store_true", help="Update all baselines.")
    parser.add_argument(
        "--set-frozen-aspects-comment",
        metavar="TEXT",
        help=(
            "replace frozen_aspects_baseline.json's _comment field verbatim "
            "with TEXT (sanctioned write path; does not touch file hashes)."
        ),
    )
    args = parser.parse_args(argv)

    if args.set_frozen_aspects_comment is not None:
        print("[frozen_aspects._comment]")
        changed = _set_frozen_aspects_comment(args.set_frozen_aspects_comment)
        print(
            "\nComment updated." if changed else "\nComment unchanged."
        )
        return 0

    if not args.baseline and not args.all:
        parser.error("specify a baseline name or --all")

    targets = list(_UPDATERS.keys()) if args.all else [args.baseline]
    any_changed = False
    for name in targets:
        print(f"[{name}]")
        any_changed |= _UPDATERS[name]()

    if any_changed:
        print("\nBaseline(s) updated. Include the updated file(s) in your commit.")
    else:
        print("\nAll baselines up to date.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
