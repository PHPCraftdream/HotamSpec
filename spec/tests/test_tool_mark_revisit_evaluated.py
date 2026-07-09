"""Tests for tools/mark_revisit_evaluated.py and the what_now revisit-marker band.

Guarantees (Wave 11, R-revisit-markers-evaluated):
  1. append_evaluation writes a {stamp, conflict, settled_count} record to an
     append-only file; a second call appends, never overwrites.
  2. main() refuses an unknown conflict id (exit 1).
  3. main() refuses a conflict that carries no revisit_marker (exit 1).
  4. The what_now revisit band fires on a DECIDED conflict with a marker that
     was never evaluated, and goes silent once evaluated within the delta.
  5. The band re-fires after the graph grows past the staleness delta.
  6. The band is CLI-only: it NEVER enters diagnose(g).
"""

from __future__ import annotations

import json


from hotam_spec.axis import Axis  # noqa: E402
from hotam_spec.conflict import Conflict, conflict_identity  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.requirement import SETTLED, Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

import mark_revisit_evaluated as mre  # noqa: E402
import what_now  # noqa: E402


# ---------------------------------------------------------------------------
# 1. append_evaluation shape + append-only
# ---------------------------------------------------------------------------


def test_append_evaluation_writes_record(tmp_path) -> None:
    path = tmp_path / "revisit-eval.jsonl"
    rec = mre.append_evaluation("C-abc", 42, path=path)
    assert rec["conflict"] == "C-abc"
    assert rec["settled_count"] == 42
    assert "stamp" in rec
    lines = path.read_text(encoding="utf-8").splitlines()
    assert len(lines) == 1
    mre.append_evaluation("C-abc", 50, path=path)
    lines = path.read_text(encoding="utf-8").splitlines()
    assert len(lines) == 2, "second evaluation must append, not overwrite"
    assert json.loads(lines[-1])["settled_count"] == 50


# ---------------------------------------------------------------------------
# 2/3. main() refusals
# ---------------------------------------------------------------------------


def _graph_with_marked_conflict() -> TensionGraph:
    sh = (
        Stakeholder(id="a", name="A", domain="x"),
        Stakeholder(id="b", name="B", domain="y"),
        Stakeholder(id="c", name="C", domain="z"),
    )
    reqs = (
        Requirement(id="R-1", claim="c1", owner="a", status=SETTLED),
        Requirement(id="R-2", claim="c2", owner="b", status=SETTLED),
    )
    axis, ctx = "cost-vs-flex", "shared scenario"
    con = Conflict(
        id=conflict_identity(axis, ctx),
        axis=axis,
        context=ctx,
        members=("R-1", "R-2"),
        steward="c",
        lifecycle="DECIDED(picked R-1)",
        decided_by="c",
        revisit_marker="REVISIT if the picked side stops paying off.",
    )
    return TensionGraph(
        axes=(Axis(slug=axis, description="cost vs flex"),),
        stakeholders=sh,
        requirements=reqs,
        conflicts=(con,),
    )


def test_main_refuses_unknown_conflict(monkeypatch) -> None:
    monkeypatch.setattr(mre, "load_content_graph", _graph_with_marked_conflict)
    assert mre.main(["C-nope"]) == 1


def test_main_refuses_conflict_without_marker(monkeypatch) -> None:
    sh = (
        Stakeholder(id="a", name="A", domain="x"),
        Stakeholder(id="b", name="B", domain="y"),
        Stakeholder(id="c", name="C", domain="z"),
    )
    reqs = (
        Requirement(id="R-1", claim="c1", owner="a", status=SETTLED),
        Requirement(id="R-2", claim="c2", owner="b", status=SETTLED),
    )
    axis, ctx = "cost-vs-flex", "scenario"
    cid = conflict_identity(axis, ctx)
    con = Conflict(
        id=cid,
        axis=axis,
        context=ctx,
        members=("R-1", "R-2"),
        steward="c",
        lifecycle="DECIDED(picked R-1)",
        decided_by="c",
    )
    g = TensionGraph(
        axes=(Axis(slug=axis, description="cost vs flex"),),
        stakeholders=sh,
        requirements=reqs,
        conflicts=(con,),
    )
    monkeypatch.setattr(mre, "load_content_graph", lambda: g)
    assert mre.main([cid]) == 1


# ---------------------------------------------------------------------------
# 4/5. what_now revisit band lifecycle
# ---------------------------------------------------------------------------


def test_revisit_band_fires_when_never_evaluated(monkeypatch, tmp_path) -> None:
    monkeypatch.setattr(what_now, "REVISIT_EVAL_FILE", tmp_path / "absent.jsonl")
    g = _graph_with_marked_conflict()
    acts = what_now.revisit_marker_actions(g)
    assert len(acts) == 1
    assert acts[0].target.startswith("C-")
    assert "never evaluated" in acts[0].imperative


def test_revisit_band_silent_after_evaluation(monkeypatch, tmp_path) -> None:
    g = _graph_with_marked_conflict()
    cid = g.conflicts[0].id
    evalfile = tmp_path / "revisit-eval.jsonl"
    now = sum(1 for r in g.requirements if r.status == SETTLED)
    mre.append_evaluation(cid, now, path=evalfile)
    monkeypatch.setattr(what_now, "REVISIT_EVAL_FILE", evalfile)
    assert what_now.revisit_marker_actions(g) == []


def test_revisit_band_refires_after_growth(monkeypatch, tmp_path) -> None:
    g = _graph_with_marked_conflict()
    cid = g.conflicts[0].id
    evalfile = tmp_path / "revisit-eval.jsonl"
    # Evaluated long ago at settled_count=0; graph now has 2 SETTLED... too small.
    # Grow the graph well past the delta.
    extra = tuple(
        Requirement(id=f"R-x{i}", claim=f"cx{i}", owner="a", status=SETTLED)
        for i in range(what_now.GENERATIVE_AUDIT_STALE_DELTA + 1)
    )
    g2 = TensionGraph(
        axes=g.axes,
        stakeholders=g.stakeholders,
        requirements=g.requirements + extra,
        conflicts=g.conflicts,
    )
    mre.append_evaluation(cid, 0, path=evalfile)
    monkeypatch.setattr(what_now, "REVISIT_EVAL_FILE", evalfile)
    acts = what_now.revisit_marker_actions(g2)
    assert len(acts) == 1
    assert "last evaluated" in acts[0].imperative


# ---------------------------------------------------------------------------
# 6. Never enters diagnose
# ---------------------------------------------------------------------------


def test_revisit_band_never_enters_diagnose(monkeypatch, tmp_path) -> None:
    monkeypatch.setattr(what_now, "REVISIT_EVAL_FILE", tmp_path / "absent.jsonl")
    g = _graph_with_marked_conflict()
    acts = what_now.diagnose(g)
    assert not [a for a in acts if "revisit marker" in a.imperative]
