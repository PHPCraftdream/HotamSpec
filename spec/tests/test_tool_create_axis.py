"""Tests for spec/tools/create_axis.py — Axis scaffolder with a MANDATORY
confront-style similarity gatekeeper (R-axis-gatekeeper-policy).

Uses in-memory TensionGraph fixtures (no tmp graph.py needed for the
gatekeeper logic itself); the apply-side writer (_apply_axis_to_source) is
covered against a tmp graph.py, mirroring test_tool_create_entity_type.py's
pattern.
"""

from __future__ import annotations

import sys
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]


import apply_proposal  # noqa: E402
import create_axis  # noqa: E402
from hotam_spec.axis import Axis  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402


def _g() -> TensionGraph:
    sh = (Stakeholder(id="owner", name="Owner", domain="test"),)
    axes = (
        Axis(
            slug="latency-vs-completeness",
            description=(
                "Fast partial answers vs slow complete ones. Speed serves "
                "urgency; completeness serves correctness."
            ),
        ),
        Axis(
            slug="privacy-vs-analytics",
            description=(
                "Minimizing personal data retained vs maximizing insight "
                "gathered from user behavior."
            ),
        ),
    )
    return TensionGraph(stakeholders=sh, axes=axes)


_MINIMAL_GRAPH = """\
from __future__ import annotations

from hotam_spec.axis import Axis
from hotam_spec.graph import TensionGraph
from hotam_spec.stakeholder import Stakeholder


def build_graph() -> TensionGraph:
    stakeholders = (
        Stakeholder(id="owner", name="Owner", domain="test"),
    )
    axes = (
        Axis(
            slug="latency-vs-completeness",
            description="Fast partial answers vs slow complete ones.",
        ),
    )
    return TensionGraph(
        stakeholders=stakeholders,
        axes=axes,
    )
"""


# ---------------------------------------------------------------------------
# 1. Positive: a genuinely novel axis passes the gatekeeper
# ---------------------------------------------------------------------------


def test_novel_axis_passes_gatekeeper() -> None:
    """A lexically distinct axis is admitted (dry-run: no writer invoked)."""
    rc = create_axis.scaffold(
        slug="cost-vs-flexibility",
        description=(
            "Fixed low-cost infrastructure vs elastic, more expensive, "
            "adaptable capacity."
        ),
        dry_run=True,
        graph=_g(),
    )
    assert rc == 0


# ---------------------------------------------------------------------------
# 2. Negative: near-duplicate is refused with exit != 0
# ---------------------------------------------------------------------------


def test_near_duplicate_refused() -> None:
    """A near-duplicate of an existing axis is refused, naming the nearest match."""
    rc = create_axis.scaffold(
        slug="speed-vs-full-check",
        description=(
            "Fast partial answers vs slow complete ones — speed serves "
            "urgency, completeness serves correctness."
        ),
        dry_run=True,
        graph=_g(),
    )
    assert rc != 0


def test_near_duplicate_refusal_names_nearest(capsys) -> None:
    """The refusal message names the nearest existing axis slug."""
    rc = create_axis.scaffold(
        slug="speed-vs-full-check",
        description=(
            "Fast partial answers vs slow complete ones — speed serves "
            "urgency, completeness serves correctness."
        ),
        dry_run=True,
        graph=_g(),
    )
    captured = capsys.readouterr()
    assert rc != 0
    assert "latency-vs-completeness" in captured.err


def test_exact_slug_duplicate_always_refused_even_with_force() -> None:
    """Re-declaring an existing slug is refused even with --force-new."""
    rc = create_axis.scaffold(
        slug="privacy-vs-analytics",
        description="Something else entirely, unrelated text here.",
        force_new="I really want this",
        dry_run=True,
        graph=_g(),
    )
    assert rc != 0


# ---------------------------------------------------------------------------
# 3. --force-new path: overrides refusal, records justification
# ---------------------------------------------------------------------------


def test_force_new_overrides_refusal() -> None:
    """--force-new overrides a near-duplicate refusal."""
    rc = create_axis.scaffold(
        slug="speed-vs-full-check",
        description=(
            "Fast partial answers vs slow complete ones — speed serves "
            "urgency, completeness serves correctness."
        ),
        force_new="This is deliberately a distinct axis from latency-vs-completeness "
        "because it applies to a different subsystem.",
        dry_run=True,
        graph=_g(),
    )
    assert rc == 0


def test_force_new_justification_recorded(capsys) -> None:
    """The --force-new justification is folded into the printed proposal's why."""
    rc = create_axis.scaffold(
        slug="speed-vs-full-check",
        description=(
            "Fast partial answers vs slow complete ones — speed serves "
            "urgency, completeness serves correctness."
        ),
        force_new="deliberately distinct, applies to a different subsystem",
        dry_run=True,
        graph=_g(),
    )
    captured = capsys.readouterr()
    assert rc == 0
    assert "deliberately distinct, applies to a different subsystem" in captured.out


# ---------------------------------------------------------------------------
# 4. Validation errors
# ---------------------------------------------------------------------------


def test_refuses_invalid_slug() -> None:
    for bad in ["Cost-Vs-Flex", "with space", "with/slash"]:
        rc = create_axis.scaffold(
            slug=bad,
            description="A test axis.",
            dry_run=True,
            graph=_g(),
        )
        assert rc == 1, f"Expected rc=1 for slug={bad!r}, got {rc}"


def test_refuses_empty_description() -> None:
    rc = create_axis.scaffold(
        slug="some-new-axis",
        description="   ",
        dry_run=True,
        graph=_g(),
    )
    assert rc == 1


# ---------------------------------------------------------------------------
# 5. apply-side writer: _apply_axis_to_source against a tmp graph.py
# ---------------------------------------------------------------------------


def _apply_axis(tmp_graph: Path, proposal_dict: dict) -> int:
    proposal = apply_proposal._validate_axis(proposal_dict)
    source_text = tmp_graph.read_text(encoding="utf-8")
    try:
        new_source = apply_proposal._apply_axis_to_source(source_text, proposal)
    except RuntimeError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    tmp_graph.write_text(new_source, encoding="utf-8")
    return 0


def test_writer_inserts_new_axis(tmp_path: Path) -> None:
    graph_py = tmp_path / "graph.py"
    graph_py.write_text(_MINIMAL_GRAPH, encoding="utf-8")

    rc = _apply_axis(
        graph_py,
        {
            "kind": "Axis",
            "slug": "cost-vs-flexibility",
            "description": "Fixed low-cost infra vs elastic adaptable capacity.",
            "why": "test",
        },
    )
    assert rc == 0
    result_text = graph_py.read_text(encoding="utf-8")
    assert 'slug="cost-vs-flexibility"' in result_text
    assert "Axis(" in result_text


def test_writer_refuses_duplicate_slug(tmp_path: Path) -> None:
    graph_py = tmp_path / "graph.py"
    graph_py.write_text(_MINIMAL_GRAPH, encoding="utf-8")

    rc = _apply_axis(
        graph_py,
        {
            "kind": "Axis",
            "slug": "latency-vs-completeness",
            "description": "Duplicate slug attempt.",
            "why": "test",
        },
    )
    assert rc == 1


def test_validate_axis_rejects_missing_slug() -> None:
    import pytest

    with pytest.raises(ValueError):
        apply_proposal._validate_axis({"description": "no slug"})


def test_validate_axis_rejects_bad_slug() -> None:
    import pytest

    with pytest.raises(ValueError):
        apply_proposal._validate_axis({"slug": "Bad Slug", "description": "x"})


def test_validate_axis_rejects_empty_description() -> None:
    import pytest

    with pytest.raises(ValueError):
        apply_proposal._validate_axis({"slug": "ok-slug", "description": "   "})
