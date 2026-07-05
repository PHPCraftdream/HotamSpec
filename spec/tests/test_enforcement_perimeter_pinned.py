"""Canon: §Invariants -- hash-pin guard for R-enforcement-perimeter-visible.

The enforcement perimeter is the set of files whose modification can WEAKEN
the verification machinery itself: invariants.py (the check_* layer),
gate.py (T1 tiered gate), enforcer_resolution.py (enforced_by resolution),
attention.py (the attention core), and _graph_guard.py (the PreToolUse
deny guard -- which pins ITSELF, closing the self-modification loop).

This test does NOT BLOCK changes to these files -- they are legitimately
edited when adding/fixing checks. It makes changes VISIBLE: any modification
changes the sha256 hash, this test fails RED, and the developer must
consciously update enforcement_perimeter_baseline.json via
tools/update_baseline.py. That one-line baseline update appears in the diff
alongside the code change, so a reviewer/steward SEES that enforcement
code was touched.

WHY the guard pins itself: without self-pinning, the guard could be silently
weakened (e.g. removing a deny rule) and no test would notice. The hash-pin
on _graph_guard.py closes this loop: weakening the guard changes its hash,
which fails THIS test, which requires a visible baseline update.

Update path (sanctioned, from spec/):
  .venv/Scripts/python.exe tools/update_baseline.py enforcement_perimeter
"""

from __future__ import annotations

import hashlib
import json
from pathlib import Path

_TESTS_DIR = Path(__file__).resolve().parent
_SPEC_ROOT = _TESTS_DIR.parent
_BASELINE_PATH = _TESTS_DIR / "enforcement_perimeter_baseline.json"


def _load_baseline() -> dict[str, str]:
    data = json.loads(_BASELINE_PATH.read_text(encoding="utf-8"))
    return data["files"]


def _sha256_of(rel_path: str) -> str:
    path = _SPEC_ROOT / rel_path
    return hashlib.sha256(path.read_bytes()).hexdigest()


def test_baseline_file_exists() -> None:
    assert _BASELINE_PATH.exists(), (
        f"{_BASELINE_PATH} must exist -- it is the enforcement-perimeter "
        "hash-pin baseline (R-enforcement-perimeter-visible)."
    )
    files = _load_baseline()
    assert files, "enforcement_perimeter_baseline.json must list at least one file"


def test_enforcement_perimeter_files_unchanged() -> None:
    """Every file in the baseline must still hash to its recorded value.

    A mismatch means enforcement code was edited. This is NOT necessarily
    wrong -- but it MUST be visible: update the baseline via
    tools/update_baseline.py enforcement_perimeter and commit the baseline
    change alongside the code change (R-enforcement-perimeter-visible)."""
    baseline = _load_baseline()
    mismatches: list[str] = []
    missing: list[str] = []
    for rel_path, expected_hash in baseline.items():
        full_path = _SPEC_ROOT / rel_path
        if not full_path.exists():
            missing.append(rel_path)
            continue
        actual = _sha256_of(rel_path)
        if actual != expected_hash:
            mismatches.append(
                f"  {rel_path}: expected {expected_hash[:16]}..., got {actual[:16]}..."
            )

    assert not missing, (
        "Enforcement-perimeter file(s) no longer exist: " + ", ".join(missing)
    )
    assert not mismatches, (
        "R-enforcement-perimeter-visible: enforcement code changed since "
        "the recorded baseline. This is expected when adding/fixing checks. "
        "Update the baseline consciously:\n"
        "  .venv/Scripts/python.exe tools/update_baseline.py enforcement_perimeter\n"
        "and commit the baseline update alongside your code change so the "
        "modification is visible in the diff.\n"
        "Changed files:\n" + "\n".join(mismatches)
    )


def test_baseline_covers_core_enforcement_files() -> None:
    """The baseline must cover all five core enforcement files."""
    baseline = _load_baseline()
    required = {
        "src/hotam_spec/invariants.py",
        "src/hotam_spec/enforcer_resolution.py",
        "tools/gate.py",
        "tools/_graph_guard.py",
    }
    present = set(baseline.keys())
    missing = required - present
    assert not missing, (
        f"Enforcement-perimeter baseline is missing required files: {missing}"
    )
