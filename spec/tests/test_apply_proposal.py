"""Tests for tools/apply_proposal.py — mechanical proposal applier.

Approach: PURE-FUNCTION tests (P3 pragmatic scope).
  - Test the parsing/validation/AST-locate/replace functions in isolation on
    sample strings. These are sufficient to demonstrate the protocol without
    requiring a full subprocess roundtrip against the real graph.py (which
    would be fragile in P3 and is deferred to P4).
  - test_apply_dry_run_does_not_write: invoke apply_proposal as a module
    (not subprocess) on a sample source string to verify --dry-run leaves no
    file mutation.

The full end-to-end subprocess roundtrip (write + regen + pytest) is deferred
to P4+ when the apply path is more robust.
"""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import apply_proposal  # noqa: E402
from tensio.conflict import conflict_identity  # noqa: E402
from tensio.proposal import ProposedConflictTransition  # noqa: E402


# ---------------------------------------------------------------------------
# Sample source containing a minimal Conflict node for testing
# ---------------------------------------------------------------------------

_SAMPLE_AXIS = "core-vs-aspect"
_SAMPLE_CTX = "extending the framework to surface behavioral contradictions (dead states, two processes one entity)"
_SAMPLE_CID = conflict_identity(_SAMPLE_AXIS, _SAMPLE_CTX)  # == "C-8600b1b8"

# NOTE: axis= and context= must be string LITERALS (ast.Constant) for the AST
# locator to extract them via conflict_identity(). Variable references like
# axis=_axis are not supported by _find_conflict_call's current implementation.
_SAMPLE_SOURCE = f'''\
from tensio.conflict import Conflict, conflict_identity

conflicts = (
    Conflict(
        id=conflict_identity("{_SAMPLE_AXIS}", "{_SAMPLE_CTX}"),
        axis="{_SAMPLE_AXIS}",
        context="{_SAMPLE_CTX}",
        members=("R-content-free-framework", "R-agent-never-lost"),
        steward="domain-user",
        lifecycle="DETECTED",
        shared_assumption="A-prose-suffices",
        revisit_marker="",
    ),
)
'''


# ---------------------------------------------------------------------------
# 1. _validate_proposal — already covered in test_proposal.py;
#    add integration-level edge cases here
# ---------------------------------------------------------------------------


def test_validate_missing_conflict_id() -> None:
    """_validate_proposal raises ValueError when conflict_id is missing."""
    raw = {"kind": "ConflictTransition", "new_lifecycle": "ACKNOWLEDGED"}
    with pytest.raises(ValueError, match="conflict_id"):
        apply_proposal._validate_proposal(raw)


def test_validate_missing_new_lifecycle() -> None:
    """_validate_proposal raises ValueError when new_lifecycle is missing."""
    raw = {"kind": "ConflictTransition", "conflict_id": "C-abc"}
    with pytest.raises(ValueError, match="new_lifecycle"):
        apply_proposal._validate_proposal(raw)


# ---------------------------------------------------------------------------
# 2. _find_conflict_call — AST locator on sample source
# ---------------------------------------------------------------------------


def test_find_conflict_call_locates_by_id() -> None:
    """_find_conflict_call finds the Conflict node by conflict_identity match."""
    import ast

    tree = ast.parse(_SAMPLE_SOURCE)
    node = apply_proposal._find_conflict_call(tree, _SAMPLE_CID)
    assert node is not None, (
        f"Expected to find conflict {_SAMPLE_CID!r} in sample source"
    )


def test_find_conflict_call_returns_none_for_unknown_id() -> None:
    """_find_conflict_call returns None when conflict_id doesn't resolve."""
    import ast

    tree = ast.parse(_SAMPLE_SOURCE)
    node = apply_proposal._find_conflict_call(tree, "C-deadbeef")
    assert node is None


# ---------------------------------------------------------------------------
# 3. _python_repr — repr helper
# ---------------------------------------------------------------------------


def test_python_repr_string() -> None:
    """_python_repr produces double-quoted string."""
    assert apply_proposal._python_repr("hello world") == '"hello world"'


def test_python_repr_string_with_quotes() -> None:
    """_python_repr escapes internal double quotes."""
    assert apply_proposal._python_repr('say "hi"') == '"say \\"hi\\""'


def test_python_repr_empty_tuple() -> None:
    """_python_repr produces () for empty tuple."""
    assert apply_proposal._python_repr(()) == "()"


def test_python_repr_single_element_tuple() -> None:
    """_python_repr adds trailing comma for singleton tuple."""
    result = apply_proposal._python_repr(("R-foo",))
    assert result == '("R-foo",)'


def test_python_repr_multi_element_tuple() -> None:
    """_python_repr produces (a, b) for two-element tuple."""
    result = apply_proposal._python_repr(("R-foo", "R-bar"))
    assert result == '("R-foo", "R-bar")'


# ---------------------------------------------------------------------------
# 4. _replace_or_insert_field — string replacement on sample source
# ---------------------------------------------------------------------------


def test_replace_field_lifecycle_on_sample() -> None:
    """_replace_or_insert_field replaces the lifecycle field in sample source."""
    import ast

    tree = ast.parse(_SAMPLE_SOURCE)
    call_node = apply_proposal._find_conflict_call(tree, _SAMPLE_CID)
    assert call_node is not None

    lines = _SAMPLE_SOURCE.splitlines(keepends=True)
    new_lines = apply_proposal._replace_or_insert_field(
        lines, call_node, "lifecycle", "DECIDED(because X)"
    )
    new_src = "".join(new_lines)
    assert "DECIDED(because X)" in new_src
    assert "DETECTED" not in new_src


def test_insert_field_decided_by_on_sample() -> None:
    """_replace_or_insert_field inserts decided_by when absent."""
    import ast

    tree = ast.parse(_SAMPLE_SOURCE)
    call_node = apply_proposal._find_conflict_call(tree, _SAMPLE_CID)
    assert call_node is not None

    lines = _SAMPLE_SOURCE.splitlines(keepends=True)
    new_lines = apply_proposal._replace_or_insert_field(
        lines, call_node, "decided_by", "domain-user"
    )
    new_src = "".join(new_lines)
    assert 'decided_by="domain-user"' in new_src


# ---------------------------------------------------------------------------
# 5. dry-run: apply() with a temp file — no mutation of real graph
# ---------------------------------------------------------------------------


def test_apply_dry_run_does_not_write(tmp_path: Path) -> None:
    """apply(..., dry_run=True) prints diff but does NOT write to the file."""
    # Write sample source to a temp file and point _CONTENT_GRAPH there
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_SOURCE, encoding="utf-8")
    original_text = _SAMPLE_SOURCE

    # Monkey-patch _CONTENT_GRAPH to point at our temp file
    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample_file
    try:
        proposal = ProposedConflictTransition(
            conflict_id=_SAMPLE_CID,
            new_lifecycle="DECIDED(because X resolves core-vs-aspect)",
            decided_by="domain-user",
        )
        result = apply_proposal.apply(proposal, dry_run=True)
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    # File must be unchanged
    assert sample_file.read_text(encoding="utf-8") == original_text
    assert result == 0


# ---------------------------------------------------------------------------
# 6. apply() with unknown conflict_id returns 1 (no write)
# ---------------------------------------------------------------------------


def test_apply_unknown_conflict_id_returns_error(tmp_path: Path) -> None:
    """apply() returns 1 when conflict_id does not resolve in the source."""
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_SOURCE, encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample_file
    try:
        proposal = ProposedConflictTransition(
            conflict_id="C-deadbeef",
            new_lifecycle="DECIDED(rationale)",
            decided_by="framework-reviewer",
        )
        result = apply_proposal.apply(proposal, dry_run=True)
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert result == 1


# ---------------------------------------------------------------------------
# 7. apply() dry-run with --triggering-kind emits closure section in output
# ---------------------------------------------------------------------------


def test_apply_proposal_emits_closure_when_flagged(
    tmp_path: Path, capsys: pytest.CaptureFixture
) -> None:
    """apply(..., dry_run=True, triggering_kind=...) emits the closure section in stdout.

    Dry-run path: no real graph write; we verify the closure section is printed.
    This confirms --triggering-kind is wired through apply() → stdout.
    """
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_SOURCE, encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample_file
    try:
        proposal = ProposedConflictTransition(
            conflict_id=_SAMPLE_CID,
            new_lifecycle="DECIDED(because X resolves core-vs-aspect)",
            decided_by="domain-user",
        )
        result = apply_proposal.apply(
            proposal, dry_run=True, triggering_kind="OPEN_ITEM"
        )
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert result == 0
    captured = capsys.readouterr()
    # The closure section must appear in stdout
    assert "CLOSURE CHECK" in captured.out
    assert "triggering_kind" in captured.out
    assert "OPEN_ITEM" in captured.out
