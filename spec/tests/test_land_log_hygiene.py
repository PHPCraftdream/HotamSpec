"""Canon: §Proposal — the land-log must not be polluted by test-fixture applies.

R-land-tier-trace's runtime log (spec/.runtime/land-log.jsonl) is an
observability trace of REAL landings. Tests that call apply_proposal.apply(...)
against a sample graph must NEVER leave a row in the real log — otherwise the
trace fills with R-sample-target / R-brand-new-node fixture noise and
gate_status.py's commit-boundary answer is computed over garbage (audit
2026-07-03: 804 fixture rows vs 82 real).

conftest.py's autouse _isolate_runtime_dir fixture guarantees this by
redirecting apply_proposal._RUNTIME_DIR into tmp_path for the whole suite.
This test pins that guarantee: an apply() invoked WITHOUT an explicit
runtime_dir leaves the real land-log byte-for-byte unchanged.
"""

from __future__ import annotations

import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

_SPEC_ROOT = Path(__file__).resolve().parents[1]
if str(_SPEC_ROOT / "tools") not in sys.path:
    sys.path.insert(0, str(_SPEC_ROOT / "tools"))

import apply_proposal  # noqa: E402
from hotam_spec.proposal import ProposedRequirement  # noqa: E402

_REAL_LAND_LOG = _SPEC_ROOT / ".runtime" / "land-log.jsonl"

_SAMPLE_SOURCE = '''\
"""sample graph for land-log hygiene test."""
from hotam_spec.graph import TensionGraph
from hotam_spec.requirement import Requirement
from hotam_spec.stakeholder import Stakeholder


def build_graph() -> TensionGraph:
    return TensionGraph(
        stakeholders=(Stakeholder(id="framework-author", name="A", domain="d"),),
        requirements=(
            Requirement(
                id="R-sample-target",
                claim="an original sample claim",
                owner="framework-author",
                status="SETTLED",
                why="sample",
            ),
        ),
    )
'''


def _fake_run(*a, **k):
    return MagicMock(returncode=0, stdout="", stderr="")


def test_apply_without_runtime_dir_does_not_touch_real_land_log(tmp_path) -> None:
    before = _REAL_LAND_LOG.read_bytes() if _REAL_LAND_LOG.exists() else None

    sample = tmp_path / "graph.py"
    sample.write_text(_SAMPLE_SOURCE, encoding="utf-8")
    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample
    try:
        proposal = ProposedRequirement(
            id="R-sample-target",
            claim="an updated sample claim for hygiene test",
            owner="framework-author",
            status="SETTLED",
            why="sample (updated)",
        )
        with patch.object(
            apply_proposal, "subprocess", MagicMock(run=_fake_run)
        ):
            rc = apply_proposal.apply(proposal, full_suite=True)
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert rc == 0
    after = _REAL_LAND_LOG.read_bytes() if _REAL_LAND_LOG.exists() else None
    assert after == before, (
        "apply() from a test (no explicit runtime_dir) wrote to the REAL "
        "land-log — conftest.py's _isolate_runtime_dir autouse fixture must "
        "redirect apply_proposal._RUNTIME_DIR into tmp_path."
    )
