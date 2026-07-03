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
from hotam_spec.conflict import conflict_identity  # noqa: E402
from hotam_spec.proposal import (  # noqa: E402
    ProposedConflict,
    ProposedConflictTransition,
    ProposedOperatorBudget,
)


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
from hotam_spec.conflict import Conflict, conflict_identity

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


def test_conflict_transition_repoints_shared_assumption() -> None:
    """A ConflictTransition may re-point the Conflict's shared_assumption edge.

    Signature-wave 2 (V2): when a premise dies and is REPLACED by a narrower
    one, a DECIDED conflict that rested on the dead premise would otherwise
    raise perpetual P2 fallout (conflicts_on_assumption fires for any conflict
    whose shared_assumption is DEAD). The optional shared_assumption field on
    ProposedConflictTransition is the only mechanical path to move that edge
    onto the LIVE replacement premise (R-no-hand-edit-graph).
    """
    proposal = apply_proposal._validate_proposal(
        {
            "kind": "ConflictTransition",
            "conflict_id": _SAMPLE_CID,
            "new_lifecycle": "DECIDED(re-pointed onto the live premise)",
            "decided_by": "domain-user",
            "shared_assumption": "A-text-grounded-in-models",
        }
    )
    assert proposal.shared_assumption == "A-text-grounded-in-models"
    lines = apply_proposal._apply_conflict_transition(_SAMPLE_SOURCE, proposal)
    result = "".join(lines)
    assert 'shared_assumption="A-text-grounded-in-models"' in result
    assert 'shared_assumption="A-prose-suffices"' not in result


def test_conflict_transition_leaves_shared_assumption_untouched_when_empty() -> None:
    """An empty shared_assumption leaves the existing edge in place (common case)."""
    proposal = apply_proposal._validate_proposal(
        {
            "kind": "ConflictTransition",
            "conflict_id": _SAMPLE_CID,
            "new_lifecycle": "ACKNOWLEDGED",
        }
    )
    assert proposal.shared_assumption == ""
    lines = apply_proposal._apply_conflict_transition(_SAMPLE_SOURCE, proposal)
    result = "".join(lines)
    assert 'shared_assumption="A-prose-suffices"' in result


# ---------------------------------------------------------------------------
# 2b. _find_conflict_call — variable-bound axis/context resolution
#     (R-conflict-addressing-resolves-variables)
# ---------------------------------------------------------------------------

# Mirrors the REAL domain graph pattern: axis/context bound to local variables
# inside build_graph() (c3_axis/c3_ctx), not inline string literals.
_VARBOUND_AXIS = "core-vs-aspect"
_VARBOUND_CTX = (
    "extending the framework to surface behavioral contradictions "
    "(dead states, two processes one entity)"
)
_VARBOUND_CID = conflict_identity(_VARBOUND_AXIS, _VARBOUND_CTX)

_VARBOUND_SOURCE = f'''\
from hotam_spec.conflict import Conflict, conflict_identity


def build_graph():
    c3_axis = "{_VARBOUND_AXIS}"
    c3_ctx = (
        "extending the framework to surface behavioral contradictions "
        "(dead states, two processes one entity)"
    )
    conflicts = (
        Conflict(
            id=conflict_identity(c3_axis, c3_ctx),
            axis=c3_axis,
            context=c3_ctx,
            members=("R-content-free-framework", "R-agent-never-lost"),
            steward="domain-user",
            lifecycle="ACKNOWLEDGED",
            shared_assumption="A-prose-suffices",
        ),
    )
    return conflicts
'''


def test_find_conflict_call_resolves_variable_bound_kwargs() -> None:
    """_find_conflict_call resolves axis/context passed as simple local variables.

    This mirrors the real domain graph pattern (c1_axis..c6_axis / c1_ctx..c6_ctx)
    that previously made every existing Conflict node unaddressable
    (R-conflict-addressing-resolves-variables).
    """
    import ast

    tree = ast.parse(_VARBOUND_SOURCE)
    node = apply_proposal._find_conflict_call(tree, _VARBOUND_CID)
    assert node is not None, (
        f"Expected variable-bound conflict {_VARBOUND_CID!r} to be addressable"
    )


def test_find_conflict_call_variable_bound_unknown_id_still_none() -> None:
    """Folding does not over-match: an unknown id still resolves to None."""
    import ast

    tree = ast.parse(_VARBOUND_SOURCE)
    assert apply_proposal._find_conflict_call(tree, "C-deadbeef") is None


def test_find_conflict_call_ambiguous_rebinding_not_resolved() -> None:
    """A name rebound to DIFFERENT strings is ambiguous — dropped, never guessed."""
    import ast

    src = (
        "from hotam_spec.conflict import Conflict\n"
        'axis_var = "axis-one"\n'
        'axis_var = "axis-two"\n'
        "conflicts = (\n"
        "    Conflict(\n"
        '        id="C-whatever",\n'
        "        axis=axis_var,\n"
        '        context="some context",\n'
        '        members=("R-a", "R-b"),\n'
        '        steward="s",\n'
        '        lifecycle="DETECTED",\n'
        "    ),\n"
        ")\n"
    )
    tree = ast.parse(src)
    for candidate_axis in ("axis-one", "axis-two"):
        cid = conflict_identity(candidate_axis, "some context")
        assert apply_proposal._find_conflict_call(tree, cid) is None, (
            f"ambiguous binding must not resolve (matched via {candidate_axis!r})"
        )


def test_real_domain_conflict_c8600b1b8_is_addressable() -> None:
    """The standing P3 target C-8600b1b8 resolves in the REAL domain graph source.

    Before the fix, all six Conflict nodes in domains/hotam-spec-self/graph.py
    were unaddressable (axis/context passed as local variables), so the P3
    CONFLICT_STALLED action was mechanically unlandable. Addressing only — the
    transition itself remains the steward's separate act.
    """
    import ast

    domain_graph = (
        Path(__file__).resolve().parents[2]
        / "domains"
        / "hotam-spec-self"
        / "graph.py"
    )
    tree = ast.parse(domain_graph.read_text(encoding="utf-8"))
    node = apply_proposal._find_conflict_call(tree, "C-8600b1b8")
    assert node is not None, (
        "C-8600b1b8 (core-vs-aspect, ACKNOWLEDGED) must be addressable in the "
        "real domain graph"
    )


def test_all_real_domain_conflicts_are_addressable() -> None:
    """Every Conflict node in the real domain graph is addressable by its C-id."""
    import ast

    domain_graph = (
        Path(__file__).resolve().parents[2]
        / "domains"
        / "hotam-spec-self"
        / "graph.py"
    )
    src = domain_graph.read_text(encoding="utf-8")
    tree = ast.parse(src)

    # Load the graph for the authoritative list of conflict ids.
    import hotam_spec.graph as hs_graph

    g = hs_graph._load_graph_file(domain_graph)
    assert g.conflicts, "the meta-domain declares conflicts"
    for c in g.conflicts:
        node = apply_proposal._find_conflict_call(tree, c.id)
        assert node is not None, f"conflict {c.id} is not addressable"


# ---------------------------------------------------------------------------
# 2c. ProposedConflict — DECIDED-at-creation (constituting-atoms exception)
# ---------------------------------------------------------------------------


def test_conflict_default_initial_lifecycle_is_detected() -> None:
    """A Conflict proposal with no initial_lifecycle defaults to DETECTED."""
    p = apply_proposal._validate_proposal(
        {
            "kind": "Conflict",
            "axis": "core-vs-aspect",
            "context": "some tension context",
            "members": ["R-foo", "R-bar"],
            "steward": "framework-reviewer",
        }
    )
    assert p.initial_lifecycle == "DETECTED"
    assert p.decided_by == ""


def test_conflict_decided_at_creation_requires_decided_by() -> None:
    """initial_lifecycle=DECIDED(...) without decided_by is rejected."""
    with pytest.raises(ValueError, match="decided_by"):
        apply_proposal._validate_proposal(
            {
                "kind": "Conflict",
                "axis": "core-vs-aspect",
                "context": "some tension context",
                "members": ["R-foo", "R-bar"],
                "steward": "framework-reviewer",
                "initial_lifecycle": "DECIDED(resolved)",
            }
        )


def test_conflict_rejects_non_detected_non_decided_initial_lifecycle() -> None:
    """initial_lifecycle other than DETECTED / DECIDED(...) is rejected."""
    with pytest.raises(ValueError, match="initial_lifecycle"):
        apply_proposal._validate_proposal(
            {
                "kind": "Conflict",
                "axis": "core-vs-aspect",
                "context": "some tension context",
                "members": ["R-foo", "R-bar"],
                "steward": "framework-reviewer",
                "initial_lifecycle": "ACKNOWLEDGED",
            }
        )


def test_conflict_source_renders_decided_at_creation() -> None:
    """_render_conflict_source emits the DECIDED lifecycle + decided_by verbatim.

    Signature-wave 2 (V4): a conflict between two SETTLED constituting atoms
    cannot rest at DETECTED (check_constituting_not_in_unresolved_conflict), so
    it is materialized already-DECIDED in one steward act.
    """
    from hotam_spec.proposal import ProposedConflict

    p = ProposedConflict(
        axis="offload-vs-carry",
        context="ctx",
        members=("R-foo", "R-bar"),
        steward="framework-reviewer",
        initial_lifecycle="DECIDED(tree-of-links law)",
        decided_by="domain-user",
    )
    src = apply_proposal._render_conflict_source(p, "        ")
    assert 'lifecycle="DECIDED(tree-of-links law)"' in src
    assert 'decided_by="domain-user"' in src


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


# ---------------------------------------------------------------------------
# 8. ProposedOperatorBudget — validation, locator, apply
# ---------------------------------------------------------------------------

_SAMPLE_OPERATOR_SOURCE = '''\
from hotam_spec.operator import ContextBudget, Operator

operators = (
    Operator(
        id="OP-director",
        stakeholder="framework-author",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=220, measure="NODE_COUNT"),
        parent=None,
        why="test operator",
    ),
)
'''


def test_validate_operator_budget_missing_operator_id() -> None:
    """_validate_proposal raises ValueError when operator_id is missing."""
    raw = {"kind": "OperatorBudget", "new_limit": 150000, "new_measure": "CRYSTAL_CHARS"}
    with pytest.raises(ValueError, match="operator_id"):
        apply_proposal._validate_proposal(raw)


def test_validate_operator_budget_bad_prefix() -> None:
    """_validate_proposal rejects an operator_id without the 'OP-' prefix."""
    raw = {
        "kind": "OperatorBudget",
        "operator_id": "director",
        "new_limit": 150000,
        "new_measure": "CRYSTAL_CHARS",
    }
    with pytest.raises(ValueError, match="OP-"):
        apply_proposal._validate_proposal(raw)


def test_validate_operator_budget_unknown_measure() -> None:
    """_validate_proposal rejects a new_measure not in BUDGET_MEASURES."""
    raw = {
        "kind": "OperatorBudget",
        "operator_id": "OP-director",
        "new_limit": 150000,
        "new_measure": "BOGUS_MEASURE",
    }
    with pytest.raises(ValueError, match="new_measure"):
        apply_proposal._validate_proposal(raw)


def test_validate_operator_budget_ok() -> None:
    """_validate_proposal accepts a well-formed OperatorBudget proposal."""
    raw = {
        "kind": "OperatorBudget",
        "operator_id": "OP-director",
        "new_limit": 150000,
        "new_measure": "CRYSTAL_CHARS",
        "why": "return to R-working-vs-substrate-budget",
    }
    proposal = apply_proposal._validate_proposal(raw)
    assert isinstance(proposal, ProposedOperatorBudget)
    assert proposal.operator_id == "OP-director"
    assert proposal.new_limit == 150000
    assert proposal.new_measure == "CRYSTAL_CHARS"
    assert proposal.target_anchor() == "OP-director"


def test_find_operator_call_locates_by_id() -> None:
    """_find_operator_call locates the Operator(...) AST node by id."""
    import ast

    tree = ast.parse(_SAMPLE_OPERATOR_SOURCE)
    node = apply_proposal._find_operator_call(tree, "OP-director")
    assert node is not None
    assert node.func.id == "Operator"


def test_find_operator_call_returns_none_for_unknown_id() -> None:
    """_find_operator_call returns None when the operator_id is not present."""
    import ast

    tree = ast.parse(_SAMPLE_OPERATOR_SOURCE)
    node = apply_proposal._find_operator_call(tree, "OP-unknown")
    assert node is None


def test_apply_operator_budget_replaces_context_budget() -> None:
    """_apply_operator_budget rewrites the context_budget= kwarg in place."""
    proposal = ProposedOperatorBudget(
        operator_id="OP-director",
        new_limit=150_000,
        new_measure="CRYSTAL_CHARS",
        why="return to R-working-vs-substrate-budget",
    )
    lines = apply_proposal._apply_operator_budget(
        _SAMPLE_OPERATOR_SOURCE, proposal
    )
    new_source = "".join(lines)
    assert 'ContextBudget(limit=150000, measure="CRYSTAL_CHARS")' in new_source
    assert "NODE_COUNT" not in new_source
    # Must still parse as valid Python and preserve the rest of the call.
    import ast

    tree = ast.parse(new_source)
    node = apply_proposal._find_operator_call(tree, "OP-director")
    assert node is not None


def test_apply_operator_budget_dry_run_does_not_write(tmp_path: Path) -> None:
    """apply(..., dry_run=True) on an OperatorBudget proposal does not write."""
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_OPERATOR_SOURCE, encoding="utf-8")
    original_text = _SAMPLE_OPERATOR_SOURCE

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample_file
    try:
        proposal = ProposedOperatorBudget(
            operator_id="OP-director",
            new_limit=150_000,
            new_measure="CRYSTAL_CHARS",
        )
        result = apply_proposal.apply(proposal, dry_run=True)
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert sample_file.read_text(encoding="utf-8") == original_text
    assert result == 0


def test_apply_operator_budget_unknown_operator_id_returns_error(
    tmp_path: Path,
) -> None:
    """apply() returns 1 when operator_id does not resolve in the source."""
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_SAMPLE_OPERATOR_SOURCE, encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample_file
    try:
        proposal = ProposedOperatorBudget(
            operator_id="OP-unknown",
            new_limit=150_000,
            new_measure="CRYSTAL_CHARS",
        )
        result = apply_proposal.apply(proposal, dry_run=True)
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert result == 1


# ---------------------------------------------------------------------------
# 9. ProposedConflict — validation, writer (R-proposed-conflict-kind-exists)
# ---------------------------------------------------------------------------

_CONFLICT_GRAPH_SOURCE = '''\
from __future__ import annotations

from hotam_spec.axis import Axis
from hotam_spec.conflict import Conflict, conflict_identity
from hotam_spec.graph import TensionGraph
from hotam_spec.requirement import Requirement
from hotam_spec.stakeholder import Stakeholder


def build_graph() -> TensionGraph:
    stakeholders = (
        Stakeholder(id="owner-a", name="Owner A", domain="a"),
        Stakeholder(id="owner-b", name="Owner B", domain="b"),
        Stakeholder(id="neutral-steward", name="Neutral", domain="c"),
    )
    axes = (
        Axis(slug="speed-vs-safety", description="fast delivery vs careful checks"),
    )
    requirements = (
        Requirement(id="R-fast", claim="ship fast", owner="owner-a", status="SETTLED"),
        Requirement(id="R-safe", claim="ship safe", owner="owner-b", status="SETTLED"),
    )
    conflicts = (
        Conflict(
            id=conflict_identity("speed-vs-safety", "existing tension"),
            axis="speed-vs-safety",
            context="existing tension",
            members=("R-fast", "R-safe"),
            steward="neutral-steward",
            lifecycle="DETECTED",
        ),
    )
    return TensionGraph(
        axes=axes,
        stakeholders=stakeholders,
        requirements=requirements,
        conflicts=conflicts,
    )
'''


def test_validate_conflict_ok() -> None:
    """_validate_proposal accepts a well-formed Conflict creation proposal."""
    raw = {
        "kind": "Conflict",
        "axis": "speed-vs-safety",
        "context": "release week crunch",
        "members": ["R-fast", "R-safe"],
        "steward": "neutral-steward",
        "shared_assumption": "A-deadline",
        "note": "steward context only",
    }
    proposal = apply_proposal._validate_proposal(raw)
    assert isinstance(proposal, ProposedConflict)
    assert proposal.members == ("R-fast", "R-safe")
    assert proposal.target_anchor() == conflict_identity(
        "speed-vs-safety", "release week crunch"
    )


def test_validate_conflict_rejects_caller_supplied_id() -> None:
    """A Conflict proposal with a caller-supplied id is refused (R-stable-conflict-identity)."""
    raw = {
        "kind": "Conflict",
        "id": "C-12345678",
        "axis": "speed-vs-safety",
        "context": "release week crunch",
        "members": ["R-fast", "R-safe"],
        "steward": "neutral-steward",
    }
    with pytest.raises(ValueError, match="conflict_identity"):
        apply_proposal._validate_proposal(raw)


def test_validate_conflict_rejects_caller_supplied_lifecycle() -> None:
    """A Conflict proposal with a lifecycle is refused (new conflicts start DETECTED)."""
    raw = {
        "kind": "Conflict",
        "axis": "speed-vs-safety",
        "context": "release week crunch",
        "members": ["R-fast", "R-safe"],
        "steward": "neutral-steward",
        "lifecycle": "DECIDED(shortcut)",
    }
    with pytest.raises(ValueError, match="DETECTED"):
        apply_proposal._validate_proposal(raw)


def test_validate_conflict_rejects_single_member() -> None:
    """Fewer than two distinct members is refused (R-conflict-min-two-members)."""
    raw = {
        "kind": "Conflict",
        "axis": "speed-vs-safety",
        "context": "release week crunch",
        "members": ["R-fast", "R-fast"],
        "steward": "neutral-steward",
    }
    with pytest.raises(ValueError, match="DISTINCT"):
        apply_proposal._validate_proposal(raw)


def test_apply_conflict_creates_detected_node() -> None:
    """_apply_conflict_to_source inserts a Conflict with computed id + DETECTED lifecycle."""
    import ast

    proposal = ProposedConflict(
        axis="speed-vs-safety",
        context="release week crunch",
        members=("R-fast", "R-safe"),
        steward="neutral-steward",
        shared_assumption="",
    )
    new_source = apply_proposal._apply_conflict_to_source(
        _CONFLICT_GRAPH_SOURCE, proposal
    )
    # The id is emitted as a conflict_identity(...) CALL, never a hand-written literal.
    assert 'id=conflict_identity("speed-vs-safety", "release week crunch"),' in new_source
    assert 'lifecycle="DETECTED",' in new_source
    # The new node is addressable by its computed id.
    new_id = conflict_identity("speed-vs-safety", "release week crunch")
    tree = ast.parse(new_source)
    assert apply_proposal._find_conflict_call(tree, new_id) is not None
    # The existing node is untouched and still addressable.
    old_id = conflict_identity("speed-vs-safety", "existing tension")
    assert apply_proposal._find_conflict_call(tree, old_id) is not None


def test_apply_conflict_refuses_duplicate_tension() -> None:
    """Creating a Conflict whose (axis, context) already exists is refused."""
    proposal = ProposedConflict(
        axis="speed-vs-safety",
        context="EXISTING   tension",  # normalization: case/whitespace-insensitive
        members=("R-fast", "R-safe"),
        steward="neutral-steward",
    )
    with pytest.raises(RuntimeError, match="already exists"):
        apply_proposal._apply_conflict_to_source(_CONFLICT_GRAPH_SOURCE, proposal)


def test_apply_conflict_refuses_unknown_axis() -> None:
    """An axis not declared in the graph's axes is refused (R-axis-controlled-vocab)."""
    proposal = ProposedConflict(
        axis="brand-new-axis",
        context="release week crunch",
        members=("R-fast", "R-safe"),
        steward="neutral-steward",
    )
    with pytest.raises(RuntimeError, match="R-axis-controlled-vocab"):
        apply_proposal._apply_conflict_to_source(_CONFLICT_GRAPH_SOURCE, proposal)


def test_apply_conflict_refuses_member_owner_steward() -> None:
    """A steward who owns a member is refused (R-steward-distinct-from-owners)."""
    proposal = ProposedConflict(
        axis="speed-vs-safety",
        context="release week crunch",
        members=("R-fast", "R-safe"),
        steward="owner-a",  # owns R-fast
    )
    with pytest.raises(RuntimeError, match="R-steward-distinct-from-owners"):
        apply_proposal._apply_conflict_to_source(_CONFLICT_GRAPH_SOURCE, proposal)


def test_apply_conflict_refuses_missing_member() -> None:
    """A member that is not an existing Requirement id is refused."""
    proposal = ProposedConflict(
        axis="speed-vs-safety",
        context="release week crunch",
        members=("R-fast", "R-ghost"),
        steward="neutral-steward",
    )
    with pytest.raises(RuntimeError, match="R-ghost"):
        apply_proposal._apply_conflict_to_source(_CONFLICT_GRAPH_SOURCE, proposal)


def test_apply_conflict_written_graph_loads_and_passes_shape(tmp_path: Path) -> None:
    """The post-write source still builds a TensionGraph whose new node is well-formed."""
    proposal = ProposedConflict(
        axis="speed-vs-safety",
        context="release week crunch",
        members=("R-fast", "R-safe"),
        steward="neutral-steward",
    )
    new_source = apply_proposal._apply_conflict_to_source(
        _CONFLICT_GRAPH_SOURCE, proposal
    )
    graph_file = tmp_path / "graph.py"
    graph_file.write_text(new_source, encoding="utf-8")

    import hotam_spec.graph as hs_graph
    from hotam_spec.invariants import (
        check_conflict_id_matches_identity,
        check_steward_not_a_member_owner,
    )

    g = hs_graph._load_graph_file(graph_file)
    new_id = conflict_identity("speed-vs-safety", "release week crunch")
    added = [c for c in g.conflicts if c.id == new_id]
    assert len(added) == 1
    assert added[0].lifecycle == "DETECTED"
    assert added[0].members == ("R-fast", "R-safe")
    assert check_conflict_id_matches_identity(g) == []
    assert check_steward_not_a_member_owner(g) == []


def test_apply_conflict_dry_run_does_not_write(tmp_path: Path) -> None:
    """apply(..., dry_run=True) on a Conflict proposal does not write the file."""
    sample_file = tmp_path / "graph.py"
    sample_file.write_text(_CONFLICT_GRAPH_SOURCE, encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = sample_file
    try:
        proposal = ProposedConflict(
            axis="speed-vs-safety",
            context="release week crunch",
            members=("R-fast", "R-safe"),
            steward="neutral-steward",
        )
        result = apply_proposal.apply(proposal, dry_run=True)
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert sample_file.read_text(encoding="utf-8") == _CONFLICT_GRAPH_SOURCE
    assert result == 0


# ---------------------------------------------------------------------------
# Post-write verify: the UPDATE path must not silently no-op a reattachment
# (signature-wave-2 lesson — a batch reported clean while 3 targets' assumptions
#  were never moved; only the DEAD-fallout net later caught the loss).
# ---------------------------------------------------------------------------

_REQ_UPDATE_SOURCE = '''"""sample"""
from __future__ import annotations

from hotam_spec.requirement import Requirement, PROSE


def build_graph():
    requirements = (
        Requirement(
            id="R-x",
            claim=("c"),
            owner="o",
            status="SETTLED",
            assumptions=("A-old",),
            enforcement=PROSE,
        ),
    )
    return requirements
'''


def _proposed_req(**over):
    from hotam_spec.proposal import ProposedRequirement

    base = dict(
        id="R-x",
        claim="c",
        owner="o",
        status="SETTLED",
        why="",
        assumptions=("A-new",),
        relations=(),
        enforcement="PROSE",
        enforced_by=(),
        m_tag="",
        enforceability="ENFORCEABLE",
    )
    base.update(over)
    return ProposedRequirement(**base)


def test_update_reattaches_assumptions_and_verify_passes() -> None:
    """An honest reattachment UPDATE lands and the post-check accepts it."""
    p = _proposed_req(assumptions=("A-new",))
    out = apply_proposal._apply_requirement_to_source(_REQ_UPDATE_SOURCE, p)
    assert "A-new" in out and "A-old" not in out
    # Post-check runs inside _apply_requirement_to_source and did not raise.


def test_verify_requirement_update_reflected_catches_silent_noop() -> None:
    """If the emitted source does NOT reflect the intended assumptions, the
    post-check raises (exit != 0) rather than let a silent no-op land."""
    # Emitted source still carries the OLD assumption though the proposal wanted
    # a different one — simulate a writer that silently failed to move it.
    stale_source = _REQ_UPDATE_SOURCE  # still has assumptions=("A-old",)
    p = _proposed_req(assumptions=("A-new",))
    with pytest.raises(RuntimeError, match="Post-write verify FAILED"):
        apply_proposal._verify_requirement_update_reflected(stale_source, p)


def test_verify_requirement_update_reflected_catches_status_mismatch() -> None:
    p = _proposed_req(assumptions=("A-old",), status="REJECTED")
    # Source has status="SETTLED"; proposal wants REJECTED — mismatch must raise.
    with pytest.raises(RuntimeError, match="status="):
        apply_proposal._verify_requirement_update_reflected(_REQ_UPDATE_SOURCE, p)


def test_verify_requirement_update_reflected_missing_target_raises() -> None:
    p = _proposed_req(id="R-does-not-exist")
    with pytest.raises(RuntimeError, match="vanished|did not land"):
        apply_proposal._verify_requirement_update_reflected(_REQ_UPDATE_SOURCE, p)


def test_read_requirement_kwarg_resolves_bare_name_and_tuple() -> None:
    import ast as _ast

    tree = _ast.parse(_REQ_UPDATE_SOURCE)
    call = apply_proposal._find_requirement_call(tree, "R-x")
    assert call is not None
    assert apply_proposal._read_requirement_kwarg(call, "enforcement") == "PROSE"
    assert apply_proposal._read_requirement_kwarg(call, "assumptions") == ("A-old",)
    assert apply_proposal._read_requirement_kwarg(call, "status") == "SETTLED"
    assert apply_proposal._read_requirement_kwarg(call, "nonexistent") is None
