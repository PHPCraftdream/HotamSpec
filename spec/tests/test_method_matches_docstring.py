"""Tests for check_method_matches_docstring — the meta-invariant that verifies
each check_* function's docstring RULE line has non-trivial lexical overlap
with its body's Violation messages.

The threshold is 0.05 Jaccard similarity (5%), chosen to catch obvious
mismatches where the RULE says one thing and the body does something completely
different, while tolerating terse-but-correct docstrings that use related
but not identical vocabulary.

Known exceptions at current threshold: 0 (the invariant fires on zero
existing check_* functions at the 5% threshold).
"""

from __future__ import annotations

from tensio.graph import TensionGraph
from tensio.invariants import (
    _JACCARD_THRESHOLD,
    _extract_rule_from_docstring,
    _tokenize,
    check_method_matches_docstring,
)


def test_check_method_matches_docstring_passes_for_current_invariants() -> None:
    """The meta-invariant fires 0 violations for all current check_* functions.

    Exception budget: no more than 2 documented exceptions are acceptable given
    the heuristic nature of Jaccard similarity on terse docstrings. At threshold
    5% (0.05), the current count is 0.
    """
    g = TensionGraph()
    violations = check_method_matches_docstring(g)
    EXCEPTION_BUDGET = 2
    assert len(violations) <= EXCEPTION_BUDGET, (
        f"check_method_matches_docstring fired {len(violations)} violations "
        f"(budget: {EXCEPTION_BUDGET}):\n"
        + "\n".join(f"  {v.target}: {v.message}" for v in violations)
    )


def test_check_with_obvious_mismatch_fires() -> None:
    """The heuristic fires when a RULE and violation messages share no tokens.

    This tests the Jaccard logic directly, since inspect.getsource is
    unreliable for functions defined inline in test code.
    """
    # Completely unrelated vocabularies
    rule_text = "foo widget sprocket nozzle valve pump lever gear cog piston cylinders carburetor"
    violation_msg = (
        "database transaction rollback commit isolation deadlock mutex semaphore"
    )

    rule_tokens = _tokenize(rule_text)
    body_tokens = _tokenize(violation_msg)
    union = rule_tokens | body_tokens
    intersection = rule_tokens & body_tokens
    jaccard = len(intersection) / len(union) if union else 0.0

    assert jaccard < _JACCARD_THRESHOLD, (
        f"Expected obvious mismatch to have low Jaccard similarity "
        f"(got {jaccard:.2f}, threshold {_JACCARD_THRESHOLD})"
    )


def test_no_rule_line_fires_violation() -> None:
    """A docstring with no RULE or Canon line yields None from _extract_rule_from_docstring."""
    doc = "This function does something important."
    result = _extract_rule_from_docstring(doc)
    assert result is None, "should return None when no RULE or Canon line present"


def test_canon_pattern_on_first_line_is_extracted() -> None:
    """The Canon: section-anchor pattern on the first line is parsed correctly."""
    doc = "Canon: §Invariants — every Requirement.id must start with R-.\n\nMore detail here."
    rule = _extract_rule_from_docstring(doc)
    assert rule is not None
    assert "every" in rule.lower()
    assert "R-" in rule


def test_rule_keyword_in_body_is_extracted() -> None:
    """A RULE: line in a docstring body is found as a fallback."""
    doc = "Summary line with no Canon.\n\nRULE: each member must have a unique id."
    rule = _extract_rule_from_docstring(doc)
    assert rule is not None
    assert "unique" in rule


def test_rule_keyword_variant_with_parens() -> None:
    """RULE (aspect-gated): and RULE (M36): patterns are also matched."""
    doc = "Summary line.\n\nRULE (M36): the decider must be outside."
    rule = _extract_rule_from_docstring(doc)
    assert rule is not None
    assert "decider" in rule

    doc2 = "Summary line.\n\nRULE (aspect-gated): skip when empty list."
    rule2 = _extract_rule_from_docstring(doc2)
    assert rule2 is not None
    assert "skip" in rule2


def test_high_overlap_not_flagged() -> None:
    """Two texts with high token overlap should yield Jaccard above threshold."""
    rule_text = "every requirement owner must resolve to a known stakeholder"
    violation_msg = "requirement owner is not a known stakeholder"

    rule_tokens = _tokenize(rule_text)
    body_tokens = _tokenize(violation_msg)
    union = rule_tokens | body_tokens
    intersection = rule_tokens & body_tokens
    jaccard = len(intersection) / len(union) if union else 0.0

    assert jaccard >= _JACCARD_THRESHOLD, (
        f"Expected related texts to have high Jaccard (got {jaccard:.2f})"
    )
