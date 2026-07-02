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

RULE: if any frozen file's sha256 no longer matches
tests/frozen_aspects_baseline.json, this test fails RED with a message naming
the changed file(s) -- "unfreezing requires an explicit steward act: update
the baseline." A red test here is not a bug signal; it is the freeze doing its
job. To intentionally unfreeze a file (steward-approved, e.g. Phase 5's "a
real business domain demonstrates concrete need" trigger fires):

    uv run python -c "
    import hashlib, json
    from pathlib import Path
    files = json.loads(Path('tests/frozen_aspects_baseline.json').read_text())['files']
    for f in files:
        files[f] = hashlib.sha256(Path(f).read_bytes()).hexdigest()
    Path('tests/frozen_aspects_baseline.json').write_text(
        json.dumps({'_comment': '...', 'files': files}, indent=2, sort_keys=True) + '\\n'
    )
    "

then commit the updated baseline alongside the change, with the steward's
rationale in the commit message -- the baseline update itself IS the
recorded act (R-anchor-everything: the diff of this file over time is the
freeze/unfreeze history).

WHY a hash baseline (not e.g. a file-mtime or git-diff check): hashes are
git-history-independent and environment-independent (survive a fresh clone,
a rebase, a CI checkout with squashed history) -- the only thing that can make
this test flip is the FILE CONTENT itself changing, which is exactly the
condition R-speculative-aspects-frozen means by "no inward development".
"""

from __future__ import annotations

import hashlib
import json
from pathlib import Path

_TESTS_DIR = Path(__file__).resolve().parent
_REPO_ROOT = _TESTS_DIR.parents[1]  # .../HotamSpec (mirrors invariants.py convention)
_SPEC_ROOT = _TESTS_DIR.parent  # .../HotamSpec/spec
_BASELINE_PATH = _TESTS_DIR / "frozen_aspects_baseline.json"


def _load_baseline() -> dict[str, str]:
    data = json.loads(_BASELINE_PATH.read_text(encoding="utf-8"))
    return data["files"]


def _sha256_of(rel_path: str) -> str:
    path = _SPEC_ROOT / rel_path
    return hashlib.sha256(path.read_bytes()).hexdigest()


def test_baseline_file_exists_and_is_nonempty() -> None:
    assert _BASELINE_PATH.exists(), (
        f"{_BASELINE_PATH} must exist -- it is the frozen-aspects baseline "
        "guarding R-speculative-aspects-frozen."
    )
    files = _load_baseline()
    assert files, "frozen_aspects_baseline.json must list at least one frozen file"


def test_frozen_aspect_files_unchanged_since_baseline() -> None:
    """Every file in the baseline must still hash to its recorded value.

    A mismatch means a frozen surface (Entity aspect / multi-domain
    federation / sub-agent recursion machinery) was edited without the
    explicit steward act of regenerating this baseline
    (R-speculative-aspects-frozen)."""
    baseline = _load_baseline()
    mismatches: list[str] = []
    missing: list[str] = []
    for rel_path, expected_hash in baseline.items():
        full_path = _SPEC_ROOT / rel_path
        if not full_path.exists():
            missing.append(rel_path)
            continue
        actual_hash = _sha256_of(rel_path)
        if actual_hash != expected_hash:
            mismatches.append(
                f"{rel_path}: expected {expected_hash}, got {actual_hash}"
            )

    assert not missing, (
        "frozen aspect file(s) no longer exist -- unfreezing/removal requires "
        f"an explicit steward act (update the baseline): {missing}"
    )
    assert not mismatches, (
        "R-speculative-aspects-frozen: the following frozen file(s) changed "
        "since the recorded baseline -- unfreezing requires an explicit "
        "steward act: regenerate tests/frozen_aspects_baseline.json (see "
        "this test module's docstring for the exact command) and record the "
        "rationale in the commit message.\n" + "\n".join(mismatches)
    )


def test_baseline_covers_all_three_named_frozen_surfaces() -> None:
    """R-speculative-aspects-frozen names three surfaces: the Entity aspect,
    multi-domain federation, and sub-agent recursion machinery. The baseline
    must contain at least one file anchoring each surface, so a surface is
    never silently dropped from the freeze."""
    baseline = _load_baseline()
    entity_files = [f for f in baseline if "entity" in f]
    federation_files = [f for f in baseline if "create_domain" in f]
    recursion_files = [
        f for f in baseline if any(x in f for x in ("create_agent", "spawn_agent", "invoke_agent"))
    ]
    assert entity_files, "baseline must cover the Entity aspect (e.g. src/hotam_spec/entity.py)"
    assert federation_files, "baseline must cover multi-domain federation (e.g. tools/create_domain.py)"
    assert recursion_files, "baseline must cover sub-agent recursion machinery"
