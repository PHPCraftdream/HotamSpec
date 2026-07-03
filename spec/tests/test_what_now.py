"""Tests for tools/what_now.py — the harness ("agent is never lost").

Guarantees:
  1. Against the demo fixture the harness produces a NON-EMPTY, sensible list
     covering the deliberately-planted surface (open item, stalled conflict,
     dead-assumption fallout, latent suspect).
  2. The diagnosis is DETERMINISTIC (same graph -> same list).
  3. A well-formed, fully-stewarded, up-to-date graph yields an EMPTY list and
     the "you are not lost / nothing to do" render — proving the harness is not a
     phantom that always shouts.
  4. STRUCTURE actions outrank everything (a malformed graph surfaces P1 first).
  5. An empty content slot (the framework's ship state) yields the calm "no
     content yet" banner, not actions.
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
_TESTS = Path(__file__).resolve().parent
for _p in (_SRC, _TOOLS, _TESTS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

from fixtures.seed import DEMO_AXES, seed_graph  # noqa: E402
from hotam_spec.conflict import Conflict, conflict_identity  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.requirement import Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

import what_now  # noqa: E402
from what_now import (  # noqa: E402
    P_CONFLICT_STALLED,
    P_DRIFT_FALLOUT,
    P_LATENT_CONNECTOR,
    P_OPEN_ITEM,
    P_REFLECTION,
    P_STRUCTURE,
    diagnose,
    render,
)


def test_seed_fixture_yields_nonempty_prioritized_list() -> None:
    """The seed fixture produces a non-empty, prioritized next-action list."""
    actions = diagnose(seed_graph())
    assert actions, "harness must surface work on the seed fixture"
    priorities = [a.priority for a in actions]
    assert priorities == sorted(priorities), "actions must be priority-ordered"


def test_seed_surface_is_covered() -> None:
    """All four planted signal kinds appear (drift, stalled, open, latent)."""
    actions = diagnose(seed_graph())
    kinds = {a.kind for a in actions}
    assert P_DRIFT_FALLOUT in {a.priority for a in actions}, "dead-assumption fallout"
    assert "CONFLICT_STALLED" in kinds, "a DETECTED conflict must be surfaced"
    assert "OPEN_ITEM" in kinds, "the OPEN requirement must be surfaced"
    assert "LATENT_CONNECTOR" in kinds, "a latent-connector suspect must be flagged"
    # The seed fixture is structurally well-formed -> NO structure actions.
    assert P_STRUCTURE not in {a.priority for a in actions}, (
        "seed fixture is well-formed; there must be no STRUCTURE actions"
    )


def test_seed_surface_targets_expected_objects() -> None:
    """The planted issues point at the expected object ids."""
    actions = diagnose(seed_graph())
    by_target = {a.target for a in actions}
    # OPEN requirement R-205.
    assert "R-205" in by_target
    # DETECTED conflict on automation-vs-control.
    detected_id = conflict_identity(
        "automation-vs-control", "acting inside a multi-user organization"
    )
    assert detected_id in by_target
    # Dead-assumption fallout names R-90 and R-150 (rest on A-single-customer).
    drift_targets = {a.target for a in actions if a.priority == P_DRIFT_FALLOUT}
    assert {"R-90", "R-150"} <= drift_targets


def test_diagnosis_is_deterministic() -> None:
    """Diagnosing the same graph twice yields identical action lists."""
    a1 = diagnose(seed_graph())
    a2 = diagnose(seed_graph())
    assert a1 == a2


def test_render_is_deterministic_and_nonempty() -> None:
    """render() is stable and mentions the loop on a non-empty list."""
    actions = diagnose(seed_graph())
    r1 = render(actions, source_label="demo")
    r2 = render(actions, source_label="demo")
    assert r1 == r2
    assert "what_now" in r1
    assert "Loop:" in r1


def test_resolved_graph_says_not_lost() -> None:
    """A well-formed, fully-resolved graph yields no actions and a calm message."""
    sh = (
        Stakeholder(id="a", name="A", domain="x"),
        Stakeholder(id="b", name="B", domain="y"),
        Stakeholder(id="c", name="C", domain="z"),
    )
    reqs = (
        Requirement(id="R-1", claim="c1", owner="a", status="SETTLED"),
        Requirement(id="R-2", claim="c2", owner="b", status="SETTLED"),
    )
    axis, ctx = "cost-vs-flexibility", "shared scenario"
    con = Conflict(
        id=conflict_identity(axis, ctx),
        axis=axis,
        context=ctx,
        members=("R-1", "R-2"),
        steward="c",
        lifecycle="DECIDED(picked R-1; documented)",
        decided_by="c",
    )
    g = TensionGraph(
        axes=DEMO_AXES, stakeholders=sh, requirements=reqs, conflicts=(con,)
    )
    actions = diagnose(g)
    assert actions == [], f"resolved graph must yield no actions, got {actions}"
    out = render(actions)
    assert "No open actions" in out


def test_structure_outranks_everything() -> None:
    """A malformed graph surfaces a P1 STRUCTURE action ahead of softer signals."""
    sh = (
        Stakeholder(id="a", name="A", domain="x"),
        Stakeholder(id="b", name="B", domain="y"),
        Stakeholder(id="c", name="C", domain="z"),
    )
    reqs = (
        Requirement(id="R-1", claim="c1", owner="a", status="OPEN(what?)"),
        Requirement(id="R-2", claim="c2", owner="b", status="SETTLED"),
    )
    axis, ctx = "cost-vs-flexibility", "shared scenario"
    con = Conflict(
        id=conflict_identity(axis, ctx),
        axis=axis,
        context=ctx,
        members=("R-1", "R-ghost"),  # dangling — R-ghost not in requirements
        steward="c",
        lifecycle="ACKNOWLEDGED",
    )
    g = TensionGraph(
        axes=DEMO_AXES, stakeholders=sh, requirements=reqs, conflicts=(con,)
    )
    actions = diagnose(g)
    assert actions[0].priority == P_STRUCTURE
    assert any(a.priority == P_OPEN_ITEM for a in actions)


def test_priority_band_constants_are_ordered() -> None:
    """Priority bands are strictly increasing (P0=REFLECTION most urgent)."""
    bands = [
        P_REFLECTION,
        P_STRUCTURE,
        P_DRIFT_FALLOUT,
        P_CONFLICT_STALLED,
        P_OPEN_ITEM,
        P_LATENT_CONNECTOR,
    ]
    assert bands == sorted(bands)
    assert len(set(bands)) == len(bands)


def test_main_demo_prints_without_error(capsys) -> None:
    """`what_now --demo` runs end-to-end and prints the report header."""
    what_now.main(["--demo"])
    captured = capsys.readouterr()
    assert "what_now" in captured.out
    assert "demo" in captured.out


def test_main_empty_content_prints_calm_banner(capsys, monkeypatch) -> None:
    """No content slot -> the empty-banner runs, not an action list.

    We force `load_content_graph` to return an empty graph so the test is robust
    even if the developer later drops a real graph.py into spec/content/.
    """
    monkeypatch.setattr(what_now, "load_content_graph", lambda: TensionGraph())
    what_now.main([])  # default = content
    captured = capsys.readouterr()
    assert "no content yet" in captured.out
    assert "--demo" in captured.out  # banner points at the demo flag


# ---------------------------------------------------------------------------
# Generative-audit staleness band (Wave 11, R-tension-audit-staleness-visible)
# ---------------------------------------------------------------------------

from hotam_spec.axis import Axis  # noqa: E402


def _stale_test_graph(n_settled: int) -> TensionGraph:
    sh = (Stakeholder(id="a", name="A", domain="x"),)
    reqs = tuple(
        Requirement(id=f"R-{i}", claim=f"c{i}", owner="a", status="SETTLED")
        for i in range(n_settled)
    )
    return TensionGraph(
        axes=(Axis(slug="a-vs-b", description="a vs b"),),
        stakeholders=sh,
        requirements=reqs,
    )


def _write_stamp(path: Path, settled_count: int) -> None:
    import json as _json

    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        _json.dumps(
            {"stamp": "2026-01-01T00:00:00+00:00", "settled_count": settled_count, "candidates": 0}
        )
        + "\n",
        encoding="utf-8",
    )


def test_staleness_never_run_fires(monkeypatch, tmp_path) -> None:
    """No stamp file -> 'never run' staleness action."""
    monkeypatch.setattr(what_now, "TENSION_AUDIT_STAMP", tmp_path / "absent.jsonl")
    acts = what_now.generative_audit_staleness_actions(_stale_test_graph(3))
    assert [a.target for a in acts] == ["generative-audit"]
    assert "NEVER run" in acts[0].imperative


def test_staleness_fresh_is_silent(monkeypatch, tmp_path) -> None:
    """Recent sweep within DELTA -> no action."""
    stamp = tmp_path / "tension-audit.jsonl"
    _write_stamp(stamp, settled_count=3)
    monkeypatch.setattr(what_now, "TENSION_AUDIT_STAMP", stamp)
    assert what_now.generative_audit_staleness_actions(_stale_test_graph(3)) == []


def test_staleness_after_growth_fires(monkeypatch, tmp_path) -> None:
    """Grew by more than DELTA SETTLED since the last sweep -> action."""
    stamp = tmp_path / "tension-audit.jsonl"
    _write_stamp(stamp, settled_count=3)
    monkeypatch.setattr(what_now, "TENSION_AUDIT_STAMP", stamp)
    n = 3 + what_now.GENERATIVE_AUDIT_STALE_DELTA + 1
    acts = what_now.generative_audit_staleness_actions(_stale_test_graph(n))
    assert [a.target for a in acts] == ["generative-audit"]
    assert "stale" in acts[0].imperative


def test_staleness_never_enters_diagnose(monkeypatch, tmp_path) -> None:
    """diagnose(g) must NEVER carry a generative-audit action.

    Determinism guard (R-tension-audit-staleness-visible / R-deterministic-generation):
    the staleness signal is CLI-only; even with the stamp absent, diagnose() —
    which gen_spec renders into the byte-stable LIVE-STATE — stays clean.
    """
    monkeypatch.setattr(what_now, "TENSION_AUDIT_STAMP", tmp_path / "absent.jsonl")
    acts = what_now.diagnose(_stale_test_graph(50))
    assert not [a for a in acts if a.target == "generative-audit"]
