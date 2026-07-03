"""Canon: §Invariants -- ratchet guard for R-requirement-claim-is-atomic /
R-check-method-is-atomic against tools/audit_atomicity.py.

Both requirements are honest closeable debt (STRUCTURAL, no full enforcer --
"decomposed into separate requirements" / "split into separate check_*
functions" cannot be forced by a check_*, only detected and flagged). This
test makes the DIRECTION of that debt mechanically checkable even though the
debt's full resolution stays human-judgment: it freezes the CURRENT set of
COMPOUND-flagged requirement and check_* ids in
tests/atomicity_compound_baseline.json (Wave 2 burn-down, 2026-07-02) and
fails RED the moment audit_atomicity.py reports a COMPOUND id that is NOT
already in that frozen set -- i.e. a genuinely NEW compound claim or
compound check_* entering the graph/framework after the baseline was taken.

Existing debt is NOT silently re-litigated: ids already in the baseline stay
frozen (removing an id because it was split/atomized is always allowed and
welcome -- it naturally shrinks the set; the test does not require
shrinkage, only forbids growth beyond the frozen baseline).

WHY a ratchet (not a strict "zero COMPOUND" gate): burning down 34
pre-existing compound atoms in one wave is out of scope and would force
either a mass mechanical split (losing nuance) or leaving the debt
completely invisible (the prior state). A ratchet is the honest middle:
new debt cannot sneak in unnoticed, and old debt is visibly, explicitly
carried rather than silently ignored.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_SRC = _SPEC_ROOT / "src"
_TOOLS = _SPEC_ROOT / "tools"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))

_BASELINE_PATH = _SPEC_ROOT / "tests" / "atomicity_compound_baseline.json"


def _load_baseline() -> tuple[set[str], set[str]]:
    data = json.loads(_BASELINE_PATH.read_text(encoding="utf-8"))
    return set(data["requirements"]), set(data["invariants"])


def _current_compound_ids() -> tuple[set[str], set[str]]:
    """Re-derive the CURRENT COMPOUND sets straight from audit_atomicity's
    own classification functions (not by parsing the generated AUDIT.md, so
    this stays correct even if AUDIT.md is momentarily stale/regenerating).
    """
    import audit_atomicity as aa
    from hotam_spec.graph import load_content_graph
    from hotam_spec.invariants import ALL_INVARIANTS

    g = load_content_graph()

    compound_reqs = {
        r.id
        for r in g.requirements
        if (r.status == "SETTLED" or r.status.startswith("OPEN"))
        and aa._audit_claim(r.claim)[0] == "COMPOUND"
    }
    compound_checks = {
        func.__name__
        for func in ALL_INVARIANTS
        if aa._audit_invariant(func)[0] == "COMPOUND"
    }
    return compound_reqs, compound_checks


def test_no_new_compound_requirements_beyond_baseline() -> None:
    """Every COMPOUND requirement id today must already be in the frozen
    baseline -- a new one appearing means a fresh compound claim was
    introduced without being split (R-requirement-claim-is-atomic debt
    growing instead of just being carried).
    """
    baseline_reqs, _ = _load_baseline()
    current_reqs, _ = _current_compound_ids()

    new_offenders = current_reqs - baseline_reqs
    assert not new_offenders, (
        "New COMPOUND requirement claim(s) introduced beyond the frozen "
        f"atomicity baseline: {sorted(new_offenders)}. Either split the "
        "claim into atomic requirements (R-requirement-claim-is-atomic), or "
        "if this is genuinely pre-existing debt being re-surfaced, add it to "
        f"{_BASELINE_PATH.name} with a steward-reviewed rationale."
    )


def test_no_new_compound_invariants_beyond_baseline() -> None:
    """Every COMPOUND check_* name today must already be in the frozen
    baseline -- a new one appearing means a fresh multi-rule enforcer was
    introduced without being split (R-check-method-is-atomic debt growing).
    """
    _, baseline_checks = _load_baseline()
    _, current_checks = _current_compound_ids()

    new_offenders = current_checks - baseline_checks
    assert not new_offenders, (
        "New COMPOUND check_* invariant(s) introduced beyond the frozen "
        f"atomicity baseline: {sorted(new_offenders)}. Either split the "
        "check_* into single-rule invariants (R-check-method-is-atomic), or "
        "if this is genuinely pre-existing debt being re-surfaced, add it to "
        f"{_BASELINE_PATH.name} with a steward-reviewed rationale."
    )
