"""Canon: §Invariants -- hash-baseline guard for R-speculative-aspects-frozen.

R-speculative-aspects-frozen (SETTLED, STRUCTURAL prose) declares: "The Entity
aspect, multi-domain federation, and sub-agent recursion machinery shall
receive no inward development while frozen, unfreezing only when a real
business domain demonstrates concrete need." That claim was previously prose
discipline only -- nothing made an accidental edit to a frozen surface
visible. This test makes it mechanical: a sha256 baseline of the frozen file
set, compared on every run.

Frozen file set (the concrete files backing the three named surfaces):
  - Entity aspect:            src/hotam_spec/entity.py
  - sub-agent recursion:      tools/create_agent.py, tools/spawn_agent.py,
                               tools/invoke_agent.py
  - multi-domain federation:  tools/create_domain.py

RULE: if any frozen file's sha256 no longer matches the `frozen_aspects`
section of tests/protected_baselines.json, this test fails RED with a
message naming the changed file(s) -- "unfreezing requires an explicit
steward act: update the baseline." A red test here is not a bug signal; it is
the freeze doing its job. To intentionally unfreeze a file (steward-approved,
e.g. Phase 5's "a real business domain demonstrates concrete need" trigger
fires), use the sanctioned updater (from spec/):

    .venv/Scripts/python.exe tools/update_baseline.py frozen_aspects

then commit the updated baseline alongside the change, with the steward's
rationale in the commit message -- the baseline update itself IS the
recorded act (R-anchor-everything: the diff of this file over time is the
freeze/unfreeze history).

WHY a hash baseline (not e.g. a file-mtime or git-diff check): hashes are
git-history-independent and environment-independent (survive a fresh clone,
a rebase, a CI checkout with squashed history) -- the only thing that can make
this test flip is the FILE CONTENT itself changing, which is exactly the
condition R-speculative-aspects-frozen means by "no inward development".

The actual hash-comparison logic is shared with
test_enforcement_perimeter_pinned.py via _protected_baseline_check.py -- both
sections of protected_baselines.json are structurally identical (a named
file set pinned by sha256), so the check exists exactly once.
"""

from __future__ import annotations

from _protected_baseline_check import (
    assert_baseline_file_exists_and_nonempty,
    assert_section_files_unchanged,
    load_section,
)

_SECTION = "frozen_aspects"
_UPDATE_HINT = ".venv/Scripts/python.exe tools/update_baseline.py frozen_aspects"


def test_baseline_file_exists_and_is_nonempty() -> None:
    assert_baseline_file_exists_and_nonempty(_SECTION)


def test_frozen_aspect_files_unchanged_since_baseline() -> None:
    assert_section_files_unchanged(
        _SECTION, "R-speculative-aspects-frozen", _UPDATE_HINT
    )


def test_baseline_covers_all_three_named_frozen_surfaces() -> None:
    """R-speculative-aspects-frozen names three surfaces: the Entity aspect,
    multi-domain federation, and sub-agent recursion machinery. The baseline
    must contain at least one file anchoring each surface, so a surface is
    never silently dropped from the freeze."""
    baseline = load_section(_SECTION)
    entity_files = [f for f in baseline if "entity" in f]
    federation_files = [f for f in baseline if "create_domain" in f]
    recursion_files = [
        f for f in baseline if any(x in f for x in ("create_agent", "spawn_agent", "invoke_agent"))
    ]
    assert entity_files, "baseline must cover the Entity aspect (e.g. src/hotam_spec/entity.py)"
    assert federation_files, "baseline must cover multi-domain federation (e.g. tools/create_domain.py)"
    assert recursion_files, "baseline must cover sub-agent recursion machinery"
