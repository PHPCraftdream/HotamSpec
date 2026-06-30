"""Tests for build_constitution — the operator's boot sequence generator.

Canon: §Constitution — the generated reconstitution from the substrate's
SETTLED laws. A fresh agent reading docs/gen/CONSTITUTION.md reconstitutes
as operator #1 (OP-director) without needing a session checkpoint.
"""

from __future__ import annotations

import sys
from pathlib import Path

_TOOLS = Path(__file__).resolve().parents[1] / "tools"
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))

import gen_spec  # noqa: E402


def _content_graph():
    """Load the live content graph (spec/content/graph.py)."""
    return gen_spec.load_content_graph()


# ---------------------------------------------------------------------------
# Hard boundary claims appear verbatim
# ---------------------------------------------------------------------------


def test_constitution_includes_hard_boundary() -> None:
    """R-ai-presents-not-decides claim appears verbatim in the constitution."""
    g = _content_graph()
    text = gen_spec.build_constitution(g)
    # The verbatim claim from the requirement.
    req_by_id = {r.id: r for r in g.requirements}
    r = req_by_id["R-ai-presents-not-decides"]
    # The claim is rendered in §3 (hard boundary section).
    assert r.claim in text, (
        "CONSTITUTION.md must include R-ai-presents-not-decides verbatim claim"
    )


# ---------------------------------------------------------------------------
# Super-rule claims appear verbatim
# ---------------------------------------------------------------------------


def test_constitution_includes_super_rules() -> None:
    """R-crystallize-knowledge-to-code + R-anchor-everything appear verbatim."""
    g = _content_graph()
    text = gen_spec.build_constitution(g)
    req_by_id = {r.id: r for r in g.requirements}

    for rid in ("R-crystallize-knowledge-to-code", "R-anchor-everything"):
        r = req_by_id[rid]
        assert r.claim in text, f"CONSTITUTION.md must include {rid} verbatim claim"


# ---------------------------------------------------------------------------
# Critical-core invariant names appear in §5
# ---------------------------------------------------------------------------


def test_constitution_includes_critical_core_invariants() -> None:
    """The six CRITICAL_CORE_INVARIANTS function names appear in §5."""
    g = _content_graph()
    text = gen_spec.build_constitution(g)

    expected = gen_spec._CRITICAL_CORE_NAMES
    assert len(expected) == 6, "Expected exactly 6 critical-core invariants"
    for name in expected:
        assert name in text, (
            f"CONSTITUTION.md §5 must list critical-core invariant `{name}`"
        )


# ---------------------------------------------------------------------------
# All constitution-set requirement anchors appear in the §7 table
# ---------------------------------------------------------------------------


def test_constitution_lists_all_constitution_requirements() -> None:
    """For every id in CONSTITUTION_SET, its anchor R-... appears in the output."""
    g = _content_graph()
    text = gen_spec.build_constitution(g)

    for rid in gen_spec.CONSTITUTION_SET:
        assert rid in text, f"CONSTITUTION.md §7 must include anchor `{rid}`"


# ---------------------------------------------------------------------------
# Boot sequence references apply_proposal.py and tick.py
# ---------------------------------------------------------------------------


def test_constitution_cites_apply_proposal_and_tick() -> None:
    """The boot-sequence section names tools/apply_proposal.py and tools/tick.py."""
    g = _content_graph()
    text = gen_spec.build_constitution(g)

    assert "tools/apply_proposal.py" in text, (
        "CONSTITUTION.md §6 boot-sequence must reference tools/apply_proposal.py"
    )
    assert "tools/tick.py" in text, (
        "CONSTITUTION.md §6 boot-sequence must reference tools/tick.py"
    )
