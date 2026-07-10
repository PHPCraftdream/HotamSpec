"""Tests for the §Requirement freshness ontology (Этап O, #117).

Covers:
  - the four freshness fields (last_reviewed_at / review_after / evidence /
    source_refs) are accepted on the dataclass, in a ProposedRequirement, and
    rendered by the apply_proposal writer;
  - the DERIVED per-node history mechanism: an UPDATE of an existing node
    appends a HistoryEntry; a CREATE does not;
  - check_requirement_history_wellformed fires on a structurally broken history
    (empty fields, non-monotonic stamps) and is quiet on a well-formed one —
    a NON-vacuous ("fires") guard, not a phantom.

The writer tests are PURE-FUNCTION tests against apply_proposal's
_apply_requirement_to_source (same approach as test_apply_proposal.py): no
subprocess, no real graph.py mutation.
"""

from __future__ import annotations

import apply_proposal  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.invariants import (  # noqa: E402
    check_requirement_history_wellformed,
)
from hotam_spec.proposal import ProposedRequirement  # noqa: E402
from hotam_spec.requirement import HistoryEntry, Requirement  # noqa: E402


# ---------------------------------------------------------------------------
# Dataclass: the four optional freshness fields + history default
# ---------------------------------------------------------------------------


def _req(rid: str = "R-x", **kw) -> Requirement:
    return Requirement(id=rid, claim="c", owner="o", status="SETTLED", **kw)


def test_freshness_fields_default_empty() -> None:
    r = _req()
    assert r.last_reviewed_at == ""
    assert r.review_after == ""
    assert r.evidence == ()
    assert r.source_refs == ()
    assert r.history == ()


def test_freshness_fields_accept_values() -> None:
    r = _req(
        last_reviewed_at="2026-07-10",
        review_after="2026-12-01",
        evidence=("incident-42", "metric-p99"),
        source_refs=("docs/x.md", "abc123"),
    )
    assert r.last_reviewed_at == "2026-07-10"
    assert r.review_after == "2026-12-01"
    assert r.evidence == ("incident-42", "metric-p99")
    assert r.source_refs == ("docs/x.md", "abc123")


def test_history_entry_shape() -> None:
    h = HistoryEntry(at="2026-07-10", summary="status: DRAFT->SETTLED")
    assert h.at == "2026-07-10"
    assert h.summary == "status: DRAFT->SETTLED"
    assert h.decided_by == ""  # default


def test_existing_construction_without_new_fields_still_works() -> None:
    # The ~228 existing nodes call Requirement(...) without any freshness kwarg;
    # all-optional defaults must not break that (backward compatibility).
    r = Requirement(id="R-legacy", claim="c", owner="o", status="SETTLED")
    assert r.history == ()
    assert r.evidence == ()


# ---------------------------------------------------------------------------
# Writer: CREATE renders freshness fields; UPDATE appends history
# ---------------------------------------------------------------------------

_GRAPH_HEAD = '''from __future__ import annotations

from hotam_spec.requirement import Requirement
from hotam_spec.stakeholder import Stakeholder


def build_graph():
    stakeholders = (
        Stakeholder(id="o", name="O", domain="x"),
    )
    requirements = (
'''

_GRAPH_TAIL = '''    )
    return (stakeholders, requirements)
'''


def _graph_source(req_block: str) -> str:
    return _GRAPH_HEAD + req_block + _GRAPH_TAIL


_EXISTING_REQ = '''        Requirement(
            id="R-foo",
            claim=("old claim"),
            owner="o",
            status="DRAFT",
            created_at="2026-01-01",
        ),
'''


def test_create_renders_freshness_and_no_history() -> None:
    src = _graph_source(_EXISTING_REQ)
    p = ProposedRequirement(
        id="R-new",
        claim="brand new",
        owner="o",
        status="SETTLED",
        why="",
        last_reviewed_at="2026-07-10",
        review_after="2026-12-01",
        evidence=("metric-A",),
        source_refs=("doc.md",),
    )
    out = apply_proposal._apply_requirement_to_source(src, p)
    assert 'id="R-new"' in out
    assert 'last_reviewed_at="2026-07-10"' in out
    assert 'review_after="2026-12-01"' in out
    assert "evidence=(" in out and '"metric-A"' in out
    assert "source_refs=(" in out and '"doc.md"' in out
    # CREATE never seeds a history entry: the R-new block carries no history=.
    new_block = out[out.index('id="R-new"') :]
    new_block = new_block[: new_block.index("),")]
    assert "history=" not in new_block


def test_update_appends_history_entry() -> None:
    src = _graph_source(_EXISTING_REQ)
    p = ProposedRequirement(
        id="R-foo", claim="new claim", owner="o", status="SETTLED", why=""
    )
    out = apply_proposal._apply_requirement_to_source(src, p)
    # History was appended and the HistoryEntry symbol imported.
    assert "history=(" in out
    assert "HistoryEntry(" in out
    assert "HistoryEntry" in out.split("def build_graph")[0]  # import line
    # The summary records the real field transitions (claim + status changed).
    assert "claim:" in out
    assert "status:" in out


def test_update_history_is_appended_not_clobbered() -> None:
    # A node that ALREADY has one history entry gains a SECOND on the next
    # UPDATE, preserving the first (append-only).
    existing = '''        Requirement(
            id="R-foo",
            claim=("c1"),
            owner="o",
            status="DRAFT",
            created_at="2026-01-01",
            history=(HistoryEntry(at="2026-06-01", summary="first change"),),
        ),
'''
    head = _GRAPH_HEAD.replace(
        "from hotam_spec.requirement import Requirement",
        "from hotam_spec.requirement import Requirement, HistoryEntry",
    )
    src = head + existing + _GRAPH_TAIL
    p = ProposedRequirement(
        id="R-foo", claim="c2", owner="o", status="SETTLED", why=""
    )
    out = apply_proposal._apply_requirement_to_source(src, p)
    assert out.count("HistoryEntry(") == 2  # first preserved + new appended
    assert "first change" in out


def test_noop_field_diff_summary_is_empty() -> None:
    # The diff summarizer alone returns "" when nothing tracked changed
    # (default enforcement/enforceability normalized) — no phantom entry.
    old = {
        "claim": "c",
        "owner": "o",
        "status": "SETTLED",
        "why": None,
        "assumptions": None,
        "enforcement": None,
        "enforced_by": None,
        "enforceability": None,
        "m_tag": None,
        "summary": None,
        "settled_at": "2026-01-01",
        "last_reviewed_at": None,
        "review_after": None,
        "evidence": None,
        "source_refs": None,
    }
    new = {
        "claim": "c",
        "owner": "o",
        "status": "SETTLED",
        "why": "",
        "assumptions": (),
        "enforcement": "PROSE",
        "enforced_by": (),
        "enforceability": "ENFORCEABLE",
        "m_tag": "",
        "summary": "",
        "settled_at": "2026-01-01",
        "last_reviewed_at": "",
        "review_after": "",
        "evidence": (),
        "source_refs": (),
    }
    assert apply_proposal._summarize_field_diff(old, new) == ""


# ---------------------------------------------------------------------------
# Invariant: check_requirement_history_wellformed (structure only)
# ---------------------------------------------------------------------------


def _g(hist) -> TensionGraph:
    return TensionGraph(
        requirements=(
            Requirement(id="R-h", claim="c", owner="o", status="SETTLED", history=hist),
        )
    )


def test_history_wellformed_quiet_on_valid() -> None:
    g = _g(
        (
            HistoryEntry(at="2026-01-01", summary="a"),
            HistoryEntry(at="2026-02-01", summary="b"),
        )
    )
    assert check_requirement_history_wellformed(g) == []


def test_history_wellformed_quiet_on_empty_history() -> None:
    assert check_requirement_history_wellformed(_g(())) == []


def test_fires_requirement_history_wellformed_non_monotonic() -> None:
    # Deliberately break monotonicity: the second stamp precedes the first.
    g = _g(
        (
            HistoryEntry(at="2026-02-01", summary="a"),
            HistoryEntry(at="2026-01-01", summary="b"),
        )
    )
    violations = check_requirement_history_wellformed(g)
    assert violations  # NON-vacuous: this MUST fire
    assert any("monotonic" in v.message for v in violations)


def test_fires_requirement_history_wellformed_empty_summary() -> None:
    g = _g((HistoryEntry(at="2026-01-01", summary=""),))
    violations = check_requirement_history_wellformed(g)
    assert violations
    assert any("summary" in v.message for v in violations)


def test_fires_requirement_history_wellformed_empty_at() -> None:
    g = _g((HistoryEntry(at="", summary="x"),))
    violations = check_requirement_history_wellformed(g)
    assert violations
    assert any("`at`" in v.message for v in violations)
