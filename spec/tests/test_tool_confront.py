"""Tests for spec/tools/confront.py — the CONFRONT step's tool.

Enforcer/lift for the projected R-tool-confront (R-tool-is-its-own-requirement).

Covers:
  1. Re-derivation: a claim echoing a SETTLED claim ranks that requirement first.
  2. Relitigation: a claim echoing a REJECTED requirement surfaces it with the
     replacement ids parsed from its 'REPLACES …' why-marker.
  3. Determinism: same graph + same input → byte-identical report.
  4. Novel input: calm 'no overlap' outcome, never a false alarm.
  5. CLI: --demo end-to-end run prints the report header (argv path).
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import confront  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.requirement import Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402


def _g() -> TensionGraph:
    sh = (Stakeholder(id="ST-o", name="O", domain="d"),)
    reqs = (
        Requirement(
            id="R-ship-fast",
            claim="Orders shall be dispatched to the customer within one business day.",
            owner="ST-o",
            status="SETTLED",
        ),
        Requirement(
            id="R-audit-trail",
            claim="Every dispatch event shall append an immutable audit record.",
            owner="ST-o",
            status="SETTLED",
        ),
        Requirement(
            id="R-store-in-rdf",
            claim="The graph shall be persisted as an RDF triple store.",
            owner="ST-o",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES by R-ship-fast + R-audit-trail per storage "
                "simplification; the RDF triple store duplicated existing checks."
            ),
        ),
    )
    return TensionGraph(stakeholders=sh, requirements=reqs)


# ---------------------------------------------------------------------------
# 1. Re-derivation of a SETTLED claim
# ---------------------------------------------------------------------------


def test_rederivation_of_settled_claim_ranks_first() -> None:
    """An input echoing a SETTLED claim surfaces that claim as the top match."""
    matches = confront.confront(
        _g(), "orders must be dispatched to customers within a business day"
    )
    assert matches, "expected at least one match"
    top = matches[0]
    assert top.rid == "R-ship-fast"
    assert top.status == "SETTLED"
    assert top.score > 0.3


# ---------------------------------------------------------------------------
# 2. Relitigation of a REJECTED requirement + replacement pointers
# ---------------------------------------------------------------------------


def test_relitigation_of_rejected_names_replacements() -> None:
    """An input echoing a REJECTED claim surfaces it with parsed REPLACES ids."""
    matches = confront.confront(
        _g(), "persist the graph as an RDF triple store"
    )
    rejected = [m for m in matches if m.status == "REJECTED"]
    assert rejected, f"expected the REJECTED node to surface, got {matches}"
    hit = rejected[0]
    assert hit.rid == "R-store-in-rdf"
    assert hit.replaced_by == ("R-ship-fast", "R-audit-trail")


def test_replacements_parser_handles_missing_marker() -> None:
    """A REJECTED why without 'REPLACES' yields an empty replacement tuple."""
    assert confront._replacements_from_why("just rejected, no pointer") == ()


# ---------------------------------------------------------------------------
# 3. Determinism
# ---------------------------------------------------------------------------


def test_report_is_deterministic() -> None:
    """Same graph + same input → byte-identical rendered report."""
    g = _g()
    text = "dispatch orders and record an audit trail"
    r1 = confront.render(confront.confront(g, text), text=text)
    r2 = confront.render(confront.confront(g, text), text=text)
    assert r1 == r2
    assert r1.startswith("== confront:")


# ---------------------------------------------------------------------------
# 4. Novel input — calm outcome
# ---------------------------------------------------------------------------


def test_novel_input_reports_no_overlap() -> None:
    """Input sharing nothing with the graph yields the calm 'no overlap' line."""
    text = "zebra habitats require seasonal migration corridors"
    matches = confront.confront(_g(), text)
    out = confront.render(matches, text=text)
    assert matches == []
    assert "No substantial overlap found" in out


# ---------------------------------------------------------------------------
# 5. CLI end-to-end (--demo)
# ---------------------------------------------------------------------------


def test_main_demo_prints_report(capsys) -> None:
    """`confront --demo <text>` runs end-to-end and prints the report header."""
    rc = confront.main(["--demo", "orders", "ship", "within", "hours"])
    captured = capsys.readouterr()
    assert rc == 0
    assert "== confront:" in captured.out
    assert "possibly re-derives" in captured.out


def test_main_without_text_errors(capsys, monkeypatch) -> None:
    """No argv text and empty stdin → clear error, exit 1."""
    import io

    monkeypatch.setattr(sys, "stdin", io.StringIO(""))
    rc = confront.main(["--demo"])
    captured = capsys.readouterr()
    assert rc == 1
    assert "no claim text" in captured.err
