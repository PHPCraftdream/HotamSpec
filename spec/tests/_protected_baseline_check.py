"""Canon: §Invariants -- shared sha256 hash-pin check for protected_baselines.json.

Both `enforcement_perimeter` (R-enforcement-perimeter-visible) and
`frozen_aspects` (R-speculative-aspects-frozen) sections of
tests/protected_baselines.json are semantically identical: a named set of
files pinned by sha256, guarding one rule against silent modification. This
module holds the ONE parameterized check both test_*.py files call, so the
verification logic exists exactly once (R-atomicity-ratchet-no-growth spirit:
do not duplicate structurally identical checks).

Not itself collected by pytest (no test_* functions here) -- it is imported
by test_enforcement_perimeter_pinned.py and test_frozen_aspects_snapshot.py.
"""

from __future__ import annotations

import hashlib
import json
from pathlib import Path

_TESTS_DIR = Path(__file__).resolve().parent
_SPEC_ROOT = _TESTS_DIR.parent
_BASELINE_PATH = _TESTS_DIR / "protected_baselines.json"


def _load_all_sections() -> dict:
    return json.loads(_BASELINE_PATH.read_text(encoding="utf-8"))


def load_section(section: str) -> dict[str, str]:
    return _load_all_sections()[section]["files"]


def _sha256_of(rel_path: str) -> str:
    path = _SPEC_ROOT / rel_path
    return hashlib.sha256(path.read_bytes()).hexdigest()


def assert_baseline_file_exists_and_nonempty(section: str) -> None:
    assert _BASELINE_PATH.exists(), (
        f"{_BASELINE_PATH} must exist -- it holds the protected hash-pin "
        f"baselines, including [{section}]."
    )
    files = load_section(section)
    assert files, f"protected_baselines.json [{section}] must list at least one file"


def assert_section_files_unchanged(section: str, rule: str, update_hint: str) -> None:
    """Every file in `section` must still hash to its recorded value.

    A mismatch means the guarded surface was edited. This is NOT necessarily
    wrong -- but it MUST be visible: update the baseline via `update_hint`
    and commit the baseline change alongside the code change."""
    baseline = load_section(section)
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
        f"[{section}] file(s) no longer exist -- this requires an explicit "
        f"steward act (update the baseline): {', '.join(missing)}"
    )
    assert not mismatches, (
        f"{rule}: [{section}] files changed since the recorded baseline. "
        f"Update the baseline consciously:\n  {update_hint}\n"
        "and commit the baseline update alongside your code change so the "
        "modification is visible in the diff.\n"
        "Changed files:\n" + "\n".join(mismatches)
    )
