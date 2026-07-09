"""Tests for the proposals/ pending-vs-applied split (R-presented-pending-decision-type).

Verifies: pending_proposal_files() sees flat-layout files AND files under
pending/, but NOT files under applied/; apply()'s successful land moves the
proposal_file into applied/ (best-effort, never breaks the return code on a
filesystem hiccup); what_now.pending_proposal_actions() renders one P6 line
per pending file, age computed from a caller-supplied `now` for determinism.
"""

from __future__ import annotations

from pathlib import Path
from unittest.mock import MagicMock, patch


import apply_proposal  # noqa: E402
import what_now  # noqa: E402
from hotam_spec.proposal import ProposedRequirement  # noqa: E402

_SAMPLE_SOURCE = '''\
from hotam_spec.requirement import Requirement

requirements = (
    Requirement(
        id="R-sample-target",
        claim="a sample claim for pending-proposal-archive tests",
        owner="framework-author",
        status="SETTLED",
        why="sample",
    ),
)
'''


def _fake_subprocess_run() -> MagicMock:
    def _run(args, **kwargs):  # noqa: ANN001, ANN003
        result = MagicMock()
        result.returncode = 0
        result.stdout = ""
        result.stderr = ""
        return result

    return MagicMock(side_effect=_run)


# --- pending_proposal_files() ------------------------------------------------


def test_pending_sees_flat_layout_files(tmp_path: Path) -> None:
    proposals_dir = tmp_path / "proposals"
    proposals_dir.mkdir()
    (proposals_dir / "a.json").write_text("{}", encoding="utf-8")
    (proposals_dir / "b.json").write_text("{}", encoding="utf-8")
    result = apply_proposal.pending_proposal_files(proposals_dir)
    assert {p.name for p in result} == {"a.json", "b.json"}


def test_pending_sees_pending_subfolder(tmp_path: Path) -> None:
    proposals_dir = tmp_path / "proposals"
    (proposals_dir / "pending").mkdir(parents=True)
    (proposals_dir / "pending" / "c.json").write_text("{}", encoding="utf-8")
    result = apply_proposal.pending_proposal_files(proposals_dir)
    assert {p.name for p in result} == {"c.json"}


def test_pending_excludes_applied_subfolder(tmp_path: Path) -> None:
    proposals_dir = tmp_path / "proposals"
    (proposals_dir / "applied").mkdir(parents=True)
    (proposals_dir / "applied" / "landed.json").write_text("{}", encoding="utf-8")
    (proposals_dir / "still-open.json").write_text("{}", encoding="utf-8")
    result = apply_proposal.pending_proposal_files(proposals_dir)
    assert {p.name for p in result} == {"still-open.json"}


def test_pending_missing_dir_returns_empty(tmp_path: Path) -> None:
    assert apply_proposal.pending_proposal_files(tmp_path / "nope") == []


def test_pending_sorted_oldest_first(tmp_path: Path) -> None:
    import os
    import time

    proposals_dir = tmp_path / "proposals"
    proposals_dir.mkdir()
    older = proposals_dir / "older.json"
    newer = proposals_dir / "newer.json"
    older.write_text("{}", encoding="utf-8")
    time.sleep(0.05)
    newer.write_text("{}", encoding="utf-8")
    result = apply_proposal.pending_proposal_files(proposals_dir)
    assert [p.name for p in result] == ["older.json", "newer.json"]


# --- _archive_proposal_file() / apply()'s wiring -----------------------------


def test_archive_moves_file_into_applied(tmp_path: Path) -> None:
    proposals_dir = tmp_path / "proposals"
    proposals_dir.mkdir()
    src = proposals_dir / "landme.json"
    src.write_text("{}", encoding="utf-8")
    dest = apply_proposal._archive_proposal_file(
        src, applied_dir=proposals_dir / "applied"
    )
    assert dest is not None
    assert dest.name == "landme.json"
    assert dest.parent.name == "applied"
    assert dest.exists()
    assert not src.exists()


def test_archive_missing_file_is_noop(tmp_path: Path) -> None:
    missing = tmp_path / "gone.json"
    result = apply_proposal._archive_proposal_file(
        missing, applied_dir=tmp_path / "applied"
    )
    assert result is None


def test_archive_none_path_is_noop(tmp_path: Path) -> None:
    assert apply_proposal._archive_proposal_file(None) is None


def test_archive_collision_gets_numeric_suffix(tmp_path: Path) -> None:
    applied_dir = tmp_path / "applied"
    applied_dir.mkdir()
    (applied_dir / "dup.json").write_text("existing", encoding="utf-8")
    src = tmp_path / "dup.json"
    src.write_text("new", encoding="utf-8")
    dest = apply_proposal._archive_proposal_file(src, applied_dir=applied_dir)
    assert dest is not None
    assert dest.name == "dup-2.json"


def test_apply_success_archives_proposal_file(tmp_path: Path) -> None:
    sample_graph = tmp_path / "graph.py"
    sample_graph.write_text(_SAMPLE_SOURCE, encoding="utf-8")
    proposals_dir = tmp_path / "proposals"
    proposals_dir.mkdir()
    proposal_file = proposals_dir / "land-me.json"
    proposal_file.write_text("{}", encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    real_applied_dir = apply_proposal._PROPOSALS_APPLIED_DIR
    apply_proposal._CONTENT_GRAPH = sample_graph
    apply_proposal._PROPOSALS_APPLIED_DIR = proposals_dir / "applied"
    try:
        proposal = ProposedRequirement(
            id="R-sample-target",
            claim="a sample claim for pending-proposal-archive tests (updated)",
            owner="framework-author",
            status="SETTLED",
            why="sample (updated)",
        )
        with (
            patch.object(
                apply_proposal, "subprocess", MagicMock(run=_fake_subprocess_run())
            ),
            patch(
                "gate.select_tier1",
                return_value=__import__("gate").GateResult(
                    confident=True,
                    node_ids=("tests/test_smoke.py::test_smoke",),
                    reason="fake",
                ),
            ),
            patch.object(apply_proposal, "_append_land_log"),
        ):
            rc = apply_proposal.apply(
                proposal, full_suite=False, proposal_file=proposal_file
            )
        assert rc == 0
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph
        apply_proposal._PROPOSALS_APPLIED_DIR = real_applied_dir

    assert not proposal_file.exists()
    landed = proposals_dir / "applied" / "land-me.json"
    assert landed.exists()


def test_apply_failure_does_not_archive(tmp_path: Path) -> None:
    """A red pytest run must NOT archive the proposal — it never landed."""
    sample_graph = tmp_path / "graph.py"
    sample_graph.write_text(_SAMPLE_SOURCE, encoding="utf-8")
    proposals_dir = tmp_path / "proposals"
    proposals_dir.mkdir()
    proposal_file = proposals_dir / "fail-me.json"
    proposal_file.write_text("{}", encoding="utf-8")

    def _fake_run_fail(args, **kwargs):  # noqa: ANN001, ANN003
        result = MagicMock()
        if any("pytest" in str(a) for a in args):
            result.returncode = 1
            result.stdout = ""
            result.stderr = "boom"
        else:
            result.returncode = 0
            result.stdout = ""
            result.stderr = ""
        return result

    real_graph = apply_proposal._CONTENT_GRAPH
    real_applied_dir = apply_proposal._PROPOSALS_APPLIED_DIR
    apply_proposal._CONTENT_GRAPH = sample_graph
    apply_proposal._PROPOSALS_APPLIED_DIR = proposals_dir / "applied"
    try:
        proposal = ProposedRequirement(
            id="R-sample-target",
            claim="a sample claim for pending-proposal-archive tests (updated)",
            owner="framework-author",
            status="SETTLED",
            why="sample (updated)",
        )
        with (
            patch.object(
                apply_proposal,
                "subprocess",
                MagicMock(run=MagicMock(side_effect=_fake_run_fail)),
            ),
            patch(
                "gate.select_tier1",
                return_value=__import__("gate").GateResult(
                    confident=True,
                    node_ids=("tests/test_smoke.py::test_smoke",),
                    reason="fake",
                ),
            ),
            patch.object(apply_proposal, "_append_land_log"),
        ):
            rc = apply_proposal.apply(
                proposal, full_suite=False, proposal_file=proposal_file
            )
        assert rc == 1
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph
        apply_proposal._PROPOSALS_APPLIED_DIR = real_applied_dir

    assert proposal_file.exists()
    assert not (proposals_dir / "applied" / "fail-me.json").exists()


# --- what_now.pending_proposal_actions() -------------------------------------


def test_pending_proposal_actions_one_per_file(tmp_path: Path) -> None:
    proposals_dir = tmp_path / "proposals"
    proposals_dir.mkdir()
    (proposals_dir / "one.json").write_text("{}", encoding="utf-8")
    (proposals_dir / "two.json").write_text("{}", encoding="utf-8")

    real_dir = apply_proposal._PROPOSALS_DIR
    apply_proposal._PROPOSALS_DIR = proposals_dir
    try:
        actions = what_now.pending_proposal_actions(now=None)
    finally:
        apply_proposal._PROPOSALS_DIR = real_dir

    assert len(actions) == 2
    assert all(a.priority == what_now.P_PENDING_PROPOSAL for a in actions)
    assert all(a.kind == "PENDING_PROPOSAL" for a in actions)
    assert {a.target for a in actions} == {"one.json", "two.json"}


def test_pending_proposal_actions_age_in_days(tmp_path: Path) -> None:
    proposals_dir = tmp_path / "proposals"
    proposals_dir.mkdir()
    f = proposals_dir / "old.json"
    f.write_text("{}", encoding="utf-8")
    fixed_now = f.stat().st_mtime + (3 * 86400) + 10  # ~3 days later

    real_dir = apply_proposal._PROPOSALS_DIR
    apply_proposal._PROPOSALS_DIR = proposals_dir
    try:
        actions = what_now.pending_proposal_actions(now=fixed_now)
    finally:
        apply_proposal._PROPOSALS_DIR = real_dir

    assert len(actions) == 1
    assert "3 day(s)" in actions[0].imperative


def test_pending_proposal_actions_empty_when_no_dir(tmp_path: Path) -> None:
    real_dir = apply_proposal._PROPOSALS_DIR
    apply_proposal._PROPOSALS_DIR = tmp_path / "does-not-exist"
    try:
        actions = what_now.pending_proposal_actions(now=1000.0)
    finally:
        apply_proposal._PROPOSALS_DIR = real_dir
    assert actions == []


def test_diagnose_never_includes_pending_band() -> None:
    """diagnose(g) is pure-over-the-graph; it must never emit P6 (filesystem-
    sourced, non-deterministic) so gen_spec.py's docs stay byte-stable."""
    from hotam_spec.graph import load_content_graph

    g = load_content_graph()
    actions = what_now.diagnose(g)
    assert all(a.priority != what_now.P_PENDING_PROPOSAL for a in actions)
