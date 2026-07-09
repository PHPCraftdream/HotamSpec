"""Tests for build_history — the historian role made into substrate.

Verifies that HISTORY.md generation captures every REJECTED requirement and
every DECIDED conflict from the content graph, with rationale text preserved.
"""

from __future__ import annotations


import gen_spec  # noqa: E402
from hotam_spec.graph import load_content_graph  # noqa: E402


def test_history_contains_every_rejected_requirement() -> None:
    """Every REJECTED requirement's id appears in build_history output."""
    g = load_content_graph()
    text = gen_spec.build_history(g)
    rejected = [r for r in g.requirements if r.status == "REJECTED"]
    assert rejected, "Expected at least one REJECTED requirement in content graph"
    for r in rejected:
        assert r.id in text, f"HISTORY.md missing REJECTED requirement {r.id}"


def test_history_contains_every_decided_conflict() -> None:
    """Every DECIDED conflict's id appears in build_history output."""
    g = load_content_graph()
    text = gen_spec.build_history(g)
    decided = [c for c in g.conflicts if c.is_decided()]
    assert decided, "Expected at least one DECIDED conflict in content graph"
    for c in decided:
        assert c.id in text, f"HISTORY.md missing DECIDED conflict {c.id}"


def test_history_contains_rationale_for_decided() -> None:
    """A known DECIDED conflict's rationale text appears in build_history output.

    The autonomy-vs-boundary conflict (C1) carries the rationale:
    "structured proposal protocol" — verify the substring is present.
    """
    g = load_content_graph()
    text = gen_spec.build_history(g)
    decided = [c for c in g.conflicts if c.is_decided()]
    # Pick the first DECIDED conflict whose lifecycle contains a known substring.
    known_substring = "structured proposal protocol"
    matching = [c for c in decided if known_substring in c.lifecycle]
    assert matching, (
        f"No DECIDED conflict found with substring {known_substring!r} in lifecycle"
    )
    assert known_substring in text, (
        f"Rationale substring {known_substring!r} not found in HISTORY.md output"
    )
