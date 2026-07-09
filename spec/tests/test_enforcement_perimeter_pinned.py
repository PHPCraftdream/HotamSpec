"""Canon: §Invariants -- hash-pin guard for R-enforcement-perimeter-visible.

The enforcement perimeter is the set of files whose modification can WEAKEN
the verification machinery itself: invariants.py (the check_* layer),
gate.py (T1 tiered gate), enforcer_resolution.py (enforced_by resolution),
attention.py (the attention core), and _graph_guard.py (the PreToolUse
deny guard -- which pins ITSELF, closing the self-modification loop).

This test does NOT BLOCK changes to these files -- they are legitimately
edited when adding/fixing checks. It makes changes VISIBLE: any modification
changes the sha256 hash, this test fails RED, and the developer must
consciously update the [enforcement_perimeter] section of
tests/protected_baselines.json via tools/update_baseline.py. That one-line
baseline update appears in the diff alongside the code change, so a
reviewer/steward SEES that enforcement code was touched.

WHY the guard pins itself: without self-pinning, the guard could be silently
weakened (e.g. removing a deny rule) and no test would notice. The hash-pin
on _graph_guard.py closes this loop: weakening the guard changes its hash,
which fails THIS test, which requires a visible baseline update.

The actual hash-comparison logic is shared with test_frozen_aspects_snapshot.py
via _protected_baseline_check.py -- both sections of protected_baselines.json
are structurally identical (a named file set pinned by sha256), so the check
exists exactly once.

Update path (sanctioned, from spec/):
  .venv/Scripts/python.exe tools/update_baseline.py enforcement_perimeter
"""

from __future__ import annotations

from _protected_baseline_check import (
    assert_baseline_file_exists_and_nonempty,
    assert_section_files_unchanged,
    load_section,
)

_SECTION = "enforcement_perimeter"
_UPDATE_HINT = ".venv/Scripts/python.exe tools/update_baseline.py enforcement_perimeter"


def test_baseline_file_exists() -> None:
    assert_baseline_file_exists_and_nonempty(_SECTION)


def test_enforcement_perimeter_files_unchanged() -> None:
    assert_section_files_unchanged(
        _SECTION, "R-enforcement-perimeter-visible", _UPDATE_HINT
    )


def test_baseline_covers_core_enforcement_files() -> None:
    """The baseline must cover all four core enforcement files."""
    baseline = load_section(_SECTION)
    required = {
        "src/hotam_spec/invariants.py",
        "src/hotam_spec/enforcer_resolution.py",
        "tools/gate.py",
        "tools/_graph_guard.py",
    }
    present = set(baseline.keys())
    missing = required - present
    assert not missing, (
        f"protected_baselines.json [{_SECTION}] is missing required files: {missing}"
    )
