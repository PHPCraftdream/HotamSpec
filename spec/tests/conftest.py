"""Canon: §Proposal — suite-wide guard against runtime-log contamination.

Several tests call apply_proposal.apply(...) directly (test_apply_proposal_gate_wiring.py,
test_pending_proposal_archive.py, ...) to exercise the gate/verify/archive
wiring against a throw-away sample graph. Those calls reach _append_land_log(),
which — absent an explicit runtime_dir — appends to the LIVE
spec/.runtime/land-log.jsonl. The result: ~800 fixture rows (R-sample-target,
R-brand-new-node) drowning the ~80 real landings, so gate_status.py and any
human reading the trace see mostly noise (audit 2026-07-03).

This autouse fixture redirects apply_proposal._RUNTIME_DIR (and the derived
proposals dirs) into each test's tmp_path for the WHOLE suite, so no test can
write to the real runtime log by omission. A test that specifically wants to
assert real-log behavior can still pass an explicit runtime_dir to the API.
"""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_TOOLS = str(_SPEC_ROOT / "tools")
if _TOOLS not in sys.path:
    sys.path.insert(0, _TOOLS)


@pytest.fixture(autouse=True)
def _isolate_runtime_dir(tmp_path, monkeypatch):
    """Redirect apply_proposal's runtime dir into tmp_path for every test.

    Best-effort: if apply_proposal is not importable in some minimal env, the
    fixture is a no-op rather than a collection error.
    """
    try:
        import apply_proposal  # noqa: PLC0415
    except Exception:
        yield
        return

    runtime = tmp_path / ".runtime"
    proposals = runtime / "proposals"
    applied = proposals / "applied"
    monkeypatch.setattr(apply_proposal, "_RUNTIME_DIR", runtime, raising=False)
    monkeypatch.setattr(apply_proposal, "_PROPOSALS_DIR", proposals, raising=False)
    monkeypatch.setattr(
        apply_proposal, "_PROPOSALS_APPLIED_DIR", applied, raising=False
    )
    yield
