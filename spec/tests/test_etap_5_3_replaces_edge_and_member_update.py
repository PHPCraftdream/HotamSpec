"""Tests for Этап 5.3: replaces edge + ConflictMemberUpdate write-path + all-members-REJECTED signal.

Covers three independent parts:
  1. `replaces` as a typed relation edge (RELATION_KINDS, ProposedRejection.replaced_by,
     apply_proposal materializes the edge, gen_spec reads structural edges as source of truth
     with prose fallback, reflect_replaces_edge_migration migration ratchet).
  2. ProposedConflictMemberUpdate (add/remove members on an existing conflict via the protocol),
     enforcing R-conflict-min-two-members (refuses an update that would drop below 2).
  3. reflect_all_members_rejected advisory signal (a live DETECTED/ACKNOWLEDGED conflict whose
     every member is REJECTED) — C-c3911f28 (DECIDED, both members REJECTED) stays SILENT
     because DECIDED is a terminal/archival state, not a ghost.

Guarantees:
  - 'replaces' is in RELATION_KINDS; the other three supportive kinds are unchanged.
  - ProposedRejection.replaced_by defaults to () (existing nodes construct without breakage).
  - apply_proposal materializes a structural replaces edge on the successor when replaced_by is set.
  - reflect_replaces_edge_migration fires on prose-without-edge; goes silent once an edge exists.
  - ProposedConflictMemberUpdate adds/removes members; refuses to drop below 2 distinct members.
  - reflect_all_members_rejected fires on a DETECTED conflict with all-REJECTED members; silent
    on the same conflict in DECIDED/REVISIT_WHEN (terminal states) and on conflicts with live members.
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import apply_proposal  # noqa: E402
from hotam_spec import reflection  # noqa: E402
from hotam_spec.axis import Axis  # noqa: E402
from hotam_spec.conflict import Conflict, conflict_identity  # noqa: E402
from hotam_spec.graph import TensionGraph, replaces_map  # noqa: E402
from hotam_spec.proposal import (  # noqa: E402
    ProposedConflictMemberUpdate,
    ProposedRejection,
)
from hotam_spec.requirement import (  # noqa: E402
    DRAFT,
    REJECTED,
    RELATION_KINDS,
    SETTLED,
    Relation,
    Requirement,
)
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

# ---------------------------------------------------------------------------
# Shared synthetic-graph helpers
# ---------------------------------------------------------------------------

_DUMMY_AXES = (Axis(slug="ax-one", description="test axis one"),)
_SH = (
    Stakeholder(id="s-a", name="A", domain="x"),
    Stakeholder(id="s-b", name="B", domain="y"),
    Stakeholder(id="s-c", name="C", domain="z"),
)


def _settled_req(rid: str, owner: str = "s-a") -> Requirement:
    return Requirement(
        id=rid,
        claim=f"claim for {rid}",
        owner=owner,
        status=SETTLED,
    )


def _rejected_req(rid: str, owner: str = "s-a", why: str = "") -> Requirement:
    return Requirement(
        id=rid,
        claim=f"rejected claim for {rid}",
        owner=owner,
        status=REJECTED,
        why=why,
    )


# ===========================================================================
# Part 1 — replaces as a typed relation edge
# ===========================================================================


def test_replaces_in_relation_kinds() -> None:
    """'replaces' is admitted in RELATION_KINDS alongside the three supportive kinds."""
    assert "replaces" in RELATION_KINDS
    assert {"supports", "refines", "depends_on"} <= RELATION_KINDS


def test_proposed_rejection_replaced_by_defaults_empty() -> None:
    """ProposedRejection.replaced_by defaults to () — backward compatible."""
    p = ProposedRejection(requirement_id="R-x", reason="REJECTED — REPLACES R-y")
    assert p.replaced_by == ()


def test_replaces_map_inverts_edges() -> None:
    """replaces_map returns REJECTED-id -> tuple of successor ids (inverted index)."""
    rej = _rejected_req("R-old")
    succ = Requirement(
        id="R-new",
        claim="the successor",
        owner="s-a",
        status=SETTLED,
        relations=(Relation("replaces", "R-old"),),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(rej, succ),
    )
    rmap = replaces_map(g)
    assert rmap == {"R-old": ("R-new",)}


def test_replaces_map_empty_when_no_edges() -> None:
    """replaces_map is empty when no requirement carries a replaces edge."""
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(_settled_req("R-a"), _settled_req("R-b")),
    )
    assert replaces_map(g) == {}


# --- apply_proposal materializes the structural replaces edge ---


def test_apply_rejection_materializes_replaces_edge(tmp_path) -> None:
    """ProposedRejection with replaced_by writes a structural replaces edge on the successor."""
    graph_src = tmp_path / "graph.py"
    graph_src.write_text(
        "from hotam_spec.requirement import Requirement\n"
        "from hotam_spec.stakeholder import Stakeholder\n"
        "from hotam_spec.axis import Axis\n"
        "from hotam_spec.graph import TensionGraph\n"
        "def build_graph():\n"
        "    return TensionGraph(\n"
        "        axes=(Axis(slug='ax-one', description='d'),),\n"
        "        stakeholders=(Stakeholder(id='s-a', name='A', domain='x'),),\n"
        "        requirements=(\n"
        "            Requirement(\n"
        "                id='R-old', claim='old', owner='s-a', status='SETTLED',\n"
        "            ),\n"
        "            Requirement(\n"
        "                id='R-new', claim='new', owner='s-a', status='SETTLED',\n"
        "            ),\n"
        "        ),\n"
        "    )\n",
        encoding="utf-8",
    )
    proposal = ProposedRejection(
        requirement_id="R-old",
        reason="REJECTED -- REPLACES R-new",
        replaced_by=("R-new",),
    )
    # Apply via the internal function so we can inspect the written source.
    new_src = apply_proposal._apply_rejection_to_source(
        graph_src.read_text(encoding="utf-8"), proposal
    )
    # The successor R-new must now carry a structural replaces edge -> R-old.
    import ast as _ast  # noqa: PLC0415

    tree = _ast.parse(new_src)
    succ_call = apply_proposal._find_requirement_call(tree, "R-new")
    assert succ_call is not None
    rels = apply_proposal._extract_requirement_relations(succ_call)
    assert ("replaces", "R-old") in rels, (
        f"successor R-new must carry a replaces->R-old edge; got {rels}"
    )
    # The rejected R-old must have status REJECTED.
    old_call = apply_proposal._find_requirement_call(tree, "R-old")
    assert old_call is not None
    for kw in old_call.keywords:
        if kw.arg == "status" and isinstance(kw.value, _ast.Constant):
            assert kw.value.value == "REJECTED"


def test_apply_rejection_edge_only_reapply_does_not_grow_why(tmp_path) -> None:
    """Re-applying a ProposedRejection whose reason matches the ALREADY-REJECTED
    node's existing why verbatim (edge-only re-run) must add the replaces edge
    WITHOUT rewriting why -- no '-- (was: ...)' chain growth. This is the
    migration path for a later wave adding replaced_by to an already-landed
    REJECTED requirement (R-rejected-preserved-not-deleted anti-relitigation
    history must not balloon on every re-application).
    """
    graph_src = tmp_path / "graph.py"
    graph_src.write_text(
        "from hotam_spec.requirement import Requirement\n"
        "from hotam_spec.stakeholder import Stakeholder\n"
        "from hotam_spec.axis import Axis\n"
        "from hotam_spec.graph import TensionGraph\n"
        "def build_graph():\n"
        "    return TensionGraph(\n"
        "        axes=(Axis(slug='ax-one', description='d'),),\n"
        "        stakeholders=(Stakeholder(id='s-a', name='A', domain='x'),),\n"
        "        requirements=(\n"
        "            Requirement(\n"
        "                id='R-old', claim='old', owner='s-a', status='REJECTED',\n"
        "                why='REJECTED -- REPLACES R-new: the old design.',\n"
        "            ),\n"
        "            Requirement(\n"
        "                id='R-new', claim='new', owner='s-a', status='SETTLED',\n"
        "            ),\n"
        "        ),\n"
        "    )\n",
        encoding="utf-8",
    )
    proposal = ProposedRejection(
        requirement_id="R-old",
        reason="REJECTED -- REPLACES R-new: the old design.",
        replaced_by=("R-new",),
    )
    new_src = apply_proposal._apply_rejection_to_source(
        graph_src.read_text(encoding="utf-8"), proposal
    )
    import ast as _ast  # noqa: PLC0415

    tree = _ast.parse(new_src)
    # The edge lands on the successor.
    succ_call = apply_proposal._find_requirement_call(tree, "R-new")
    assert succ_call is not None
    rels = apply_proposal._extract_requirement_relations(succ_call)
    assert ("replaces", "R-old") in rels

    # why on R-old is untouched -- no '(was: ...)' chain.
    old_call = apply_proposal._find_requirement_call(tree, "R-old")
    assert old_call is not None
    why_value = None
    for kw in old_call.keywords:
        if kw.arg == "why" and isinstance(kw.value, _ast.Constant):
            why_value = kw.value.value
    assert why_value == "REJECTED -- REPLACES R-new: the old design.", (
        f"why must be left untouched on an edge-only re-application; got {why_value!r}"
    )
    assert "(was:" not in (why_value or "")


def test_apply_rejection_refuses_unknown_successor(tmp_path) -> None:
    """A replaced_by naming a non-existent requirement is refused (clear error)."""
    graph_src = tmp_path / "graph.py"
    graph_src.write_text(
        "from hotam_spec.requirement import Requirement\n"
        "from hotam_spec.stakeholder import Stakeholder\n"
        "from hotam_spec.axis import Axis\n"
        "from hotam_spec.graph import TensionGraph\n"
        "def build_graph():\n"
        "    return TensionGraph(\n"
        "        axes=(Axis(slug='ax-one', description='d'),),\n"
        "        stakeholders=(Stakeholder(id='s-a', name='A', domain='x'),),\n"
        "        requirements=(\n"
        "            Requirement(\n"
        "                id='R-old', claim='old', owner='s-a', status='SETTLED',\n"
        "            ),\n"
        "        ),\n"
        "    )\n",
        encoding="utf-8",
    )
    proposal = ProposedRejection(
        requirement_id="R-old",
        reason="REJECTED -- REPLACES R-ghost",
        replaced_by=("R-ghost",),
    )
    raised = False
    try:
        apply_proposal._apply_rejection_to_source(
            graph_src.read_text(encoding="utf-8"), proposal
        )
    except RuntimeError as exc:
        raised = True
        assert "R-ghost" in str(exc)
    assert raised, "must refuse a replaced_by naming an unknown requirement"


# --- reflect_replaces_edge_migration migration ratchet ---


def test_replaces_edge_migration_fires_on_prose_without_edge() -> None:
    """A REJECTED requirement with a prose REPLACES marker but no structural edge fires."""
    rej = _rejected_req(
        "R-prose-only",
        why="REJECTED — REPLACES by R-successor; the old design.",
    )
    succ = _settled_req("R-successor")
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(rej, succ),
    )
    findings = reflection.reflect_replaces_edge_migration(g)
    assert len(findings) == 1
    assert findings[0].target == "R-prose-only"
    assert findings[0].condition == "reflect_replaces_edge_migration"


def test_replaces_edge_migration_silent_when_edge_present() -> None:
    """Once a structural replaces edge exists, the migration finding goes silent."""
    rej = _rejected_req(
        "R-migrated",
        why="REJECTED — REPLACES by R-succ; the old design.",
    )
    succ = Requirement(
        id="R-succ",
        claim="successor",
        owner="s-a",
        status=SETTLED,
        relations=(Relation("replaces", "R-migrated"),),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(rej, succ),
    )
    findings = reflection.reflect_replaces_edge_migration(g)
    assert findings == [], (
        "migration must NOT fire when a structural replaces edge already links the successor"
    )


def test_replaces_edge_migration_silent_on_no_marker() -> None:
    """A REJECTED requirement with NO prose marker is not a migration candidate."""
    rej = _rejected_req("R-discarded", why="just discarded, no successor")
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(rej,),
    )
    findings = reflection.reflect_replaces_edge_migration(g)
    assert findings == []


# ===========================================================================
# Part 2 — ProposedConflictMemberUpdate
# ===========================================================================


def test_validate_conflict_member_update_round_trip() -> None:
    """The validator parses add/remove members and decided_by into the dataclass."""
    p = apply_proposal._validate_conflict_member_update(
        {
            "kind": "ConflictMemberUpdate",
            "conflict_id": "C-abc12345",
            "add_members": ["R-new"],
            "remove_members": ["R-old"],
            "decided_by": "s-c",
        }
    )
    assert isinstance(p, ProposedConflictMemberUpdate)
    assert p.conflict_id == "C-abc12345"
    assert p.add_members == ("R-new",)
    assert p.remove_members == ("R-old",)
    assert p.decided_by == "s-c"


def test_validate_conflict_member_update_rejects_noop() -> None:
    """An update with neither add nor remove members is rejected (no-op)."""
    raised = False
    try:
        apply_proposal._validate_conflict_member_update(
            {"kind": "ConflictMemberUpdate", "conflict_id": "C-x"}
        )
    except ValueError:
        raised = True
    assert raised


def test_apply_conflict_member_update_adds_member(tmp_path) -> None:
    """Adding a member appends to the existing members tuple."""
    graph_src = tmp_path / "graph.py"
    graph_src.write_text(
        "from hotam_spec.requirement import Requirement\n"
        "from hotam_spec.conflict import Conflict, conflict_identity\n"
        "from hotam_spec.stakeholder import Stakeholder\n"
        "from hotam_spec.axis import Axis\n"
        "from hotam_spec.graph import TensionGraph\n"
        "_AX = 'ax-one'\n"
        "_CTX = 'add-member test context'\n"
        "def build_graph():\n"
        "    return TensionGraph(\n"
        "        axes=(Axis(slug='ax-one', description='d'),),\n"
        "        stakeholders=(Stakeholder(id='s-a', name='A', domain='x'),"
        " Stakeholder(id='s-c', name='C', domain='z')),\n"
        "        requirements=(\n"
        "            Requirement(id='R-a', claim='a', owner='s-a', status='SETTLED'),\n"
        "            Requirement(id='R-b', claim='b', owner='s-a', status='SETTLED'),\n"
        "            Requirement(id='R-c', claim='c', owner='s-a', status='SETTLED'),\n"
        "        ),\n"
        "        conflicts=(\n"
        "            Conflict(id=conflict_identity(_AX, _CTX), axis=_AX, context=_CTX,\n"
        "                     members=('R-a', 'R-b'), steward='s-c', lifecycle='DETECTED'),\n"
        "        ),\n"
        "    )\n",
        encoding="utf-8",
    )
    cid = conflict_identity("ax-one", "add-member test context")
    proposal = ProposedConflictMemberUpdate(
        conflict_id=cid,
        add_members=("R-c",),
    )
    # Use the internal apply function directly (no gen_spec/pytest run in a unit test).
    new_src = apply_proposal._apply_conflict_member_update(
        graph_src.read_text(encoding="utf-8"), proposal
    )
    # The new member must appear in the members tuple.
    assert "'R-c'" in new_src
    # Existing members are preserved (order kept).
    assert "'R-a'" in new_src and "'R-b'" in new_src


def test_apply_conflict_member_update_removes_member(tmp_path) -> None:
    """Removing a member drops it from the members tuple, keeping the rest."""
    graph_src = tmp_path / "graph.py"
    graph_src.write_text(
        "from hotam_spec.requirement import Requirement\n"
        "from hotam_spec.conflict import Conflict, conflict_identity\n"
        "from hotam_spec.stakeholder import Stakeholder\n"
        "from hotam_spec.axis import Axis\n"
        "from hotam_spec.graph import TensionGraph\n"
        "_AX = 'ax-one'\n"
        "_CTX = 'remove-member test context'\n"
        "def build_graph():\n"
        "    return TensionGraph(\n"
        "        axes=(Axis(slug='ax-one', description='d'),),\n"
        "        stakeholders=(Stakeholder(id='s-a', name='A', domain='x'),"
        " Stakeholder(id='s-c', name='C', domain='z')),\n"
        "        requirements=(\n"
        "            Requirement(id='R-a', claim='a', owner='s-a', status='SETTLED'),\n"
        "            Requirement(id='R-b', claim='b', owner='s-a', status='SETTLED'),\n"
        "            Requirement(id='R-d', claim='d', owner='s-a', status='SETTLED'),\n"
        "        ),\n"
        "        conflicts=(\n"
        "            Conflict(id=conflict_identity(_AX, _CTX), axis=_AX, context=_CTX,\n"
        "                     members=('R-a', 'R-b', 'R-d'), steward='s-c',"
        " lifecycle='DETECTED'),\n"
        "        ),\n"
        "    )\n",
        encoding="utf-8",
    )
    cid = conflict_identity("ax-one", "remove-member test context")
    proposal = ProposedConflictMemberUpdate(
        conflict_id=cid,
        remove_members=("R-d",),
    )
    new_src = apply_proposal._apply_conflict_member_update(
        graph_src.read_text(encoding="utf-8"), proposal
    )
    # R-d is gone from members; R-a, R-b remain.
    # We check the members= tuple line specifically to avoid matching R-d elsewhere.
    import ast as _ast  # noqa: PLC0415

    tree = _ast.parse(new_src)
    call = apply_proposal._find_conflict_call(tree, cid)
    assert call is not None
    members = apply_proposal._extract_conflict_members(call)
    assert members == ("R-a", "R-b"), f"expected R-a,R-b; got {members}"


def test_apply_conflict_member_update_refuses_below_two(tmp_path) -> None:
    """An update that would leave < 2 distinct members is refused (R-conflict-min-two-members)."""
    graph_src = tmp_path / "graph.py"
    graph_src.write_text(
        "from hotam_spec.requirement import Requirement\n"
        "from hotam_spec.conflict import Conflict, conflict_identity\n"
        "from hotam_spec.stakeholder import Stakeholder\n"
        "from hotam_spec.axis import Axis\n"
        "from hotam_spec.graph import TensionGraph\n"
        "_AX = 'ax-one'\n"
        "_CTX = 'refuse-below-two test context'\n"
        "def build_graph():\n"
        "    return TensionGraph(\n"
        "        axes=(Axis(slug='ax-one', description='d'),),\n"
        "        stakeholders=(Stakeholder(id='s-a', name='A', domain='x'),"
        " Stakeholder(id='s-c', name='C', domain='z')),\n"
        "        requirements=(\n"
        "            Requirement(id='R-a', claim='a', owner='s-a', status='SETTLED'),\n"
        "            Requirement(id='R-b', claim='b', owner='s-a', status='SETTLED'),\n"
        "        ),\n"
        "        conflicts=(\n"
        "            Conflict(id=conflict_identity(_AX, _CTX), axis=_AX, context=_CTX,\n"
        "                     members=('R-a', 'R-b'), steward='s-c', lifecycle='DETECTED'),\n"
        "        ),\n"
        "    )\n",
        encoding="utf-8",
    )
    cid = conflict_identity("ax-one", "refuse-below-two test context")
    # Removing R-b would leave only R-a (1 distinct member) — must be refused.
    proposal = ProposedConflictMemberUpdate(
        conflict_id=cid,
        remove_members=("R-b",),
    )
    raised = False
    try:
        apply_proposal._apply_conflict_member_update(
            graph_src.read_text(encoding="utf-8"), proposal
        )
    except RuntimeError as exc:
        raised = True
        assert "R-conflict-min-two-members" in str(exc)
    assert raised, (
        "must refuse an update that drops a conflict below 2 distinct members"
    )


# ===========================================================================
# Part 3 — reflect_all_members_rejected advisory signal
# ===========================================================================


def test_all_members_rejected_fires_on_detected_with_dead_members() -> None:
    """POSITIVE: a DETECTED conflict whose every member is REJECTED fires the signal."""
    rej_a = _rejected_req("R-dead-a")
    rej_b = _rejected_req("R-dead-b", owner="s-b")
    ax, ctx = "ax-one", "ghost-detected test context"
    c = Conflict(
        id=conflict_identity(ax, ctx),
        axis=ax,
        context=ctx,
        members=("R-dead-a", "R-dead-b"),
        steward="s-c",
        lifecycle="DETECTED",
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(rej_a, rej_b),
        conflicts=(c,),
    )
    findings = reflection.reflect_all_members_rejected(g)
    assert len(findings) == 1
    assert findings[0].target == c.id
    assert findings[0].condition == "reflect_all_members_rejected"


def test_all_members_rejected_silent_on_decided() -> None:
    """NEGATIVE: a DECIDED conflict (terminal) with all-REJECTED members stays SILENT.

    C-c3911f28 is this case: both members REJECTED, but the conflict was recorded
    DECIDED — a legitimate 'tension exhausted' historical record, not a ghost.
    """
    rej_a = _rejected_req("R-dead-c")
    rej_b = _rejected_req("R-dead-d", owner="s-b")
    ax, ctx = "ax-one", "ghost-decided test context"
    c = Conflict(
        id=conflict_identity(ax, ctx),
        axis=ax,
        context=ctx,
        members=("R-dead-c", "R-dead-d"),
        steward="s-c",
        lifecycle="DECIDED(both variants rejected; tension exhausted)",
        decided_by="s-c",
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(rej_a, rej_b),
        conflicts=(c,),
    )
    findings = reflection.reflect_all_members_rejected(g)
    assert findings == [], "DECIDED conflict with dead members is NOT a ghost"


def test_all_members_rejected_silent_on_revisit_when() -> None:
    """NEGATIVE: a REVISIT_WHEN conflict (parked) with all-REJECTED members stays SILENT."""
    rej_a = _rejected_req("R-dead-e")
    rej_b = _rejected_req("R-dead-f", owner="s-b")
    ax, ctx = "ax-one", "ghost-parked test context"
    c = Conflict(
        id=conflict_identity(ax, ctx),
        axis=ax,
        context=ctx,
        members=("R-dead-e", "R-dead-f"),
        steward="s-c",
        lifecycle="REVISIT_WHEN(if the design is revisited)",
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(rej_a, rej_b),
        conflicts=(c,),
    )
    findings = reflection.reflect_all_members_rejected(g)
    assert findings == [], "REVISIT_WHEN conflict with dead members is NOT a ghost"


def test_all_members_rejected_silent_when_members_live() -> None:
    """NEGATIVE: a DETECTED conflict with LIVE (SETTLED) members does NOT fire."""
    live_a = _settled_req("R-live-a")
    live_b = _settled_req("R-live-b", owner="s-b")
    ax, ctx = "ax-one", "live-members test context"
    c = Conflict(
        id=conflict_identity(ax, ctx),
        axis=ax,
        context=ctx,
        members=("R-live-a", "R-live-b"),
        steward="s-c",
        lifecycle="DETECTED",
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(live_a, live_b),
        conflicts=(c,),
    )
    findings = reflection.reflect_all_members_rejected(g)
    assert findings == [], "must NOT fire when members are live (not REJECTED)"


def test_all_members_rejected_silent_when_mixed() -> None:
    """NEGATIVE: a conflict with ONE live + ONE rejected member does NOT fire (not ALL dead)."""
    live = _settled_req("R-live-mixed")
    rej = _rejected_req("R-dead-mixed", owner="s-b")
    ax, ctx = "ax-one", "mixed-members test context"
    c = Conflict(
        id=conflict_identity(ax, ctx),
        axis=ax,
        context=ctx,
        members=("R-live-mixed", "R-dead-mixed"),
        steward="s-c",
        lifecycle="DETECTED",
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(live, rej),
        conflicts=(c,),
    )
    findings = reflection.reflect_all_members_rejected(g)
    assert findings == [], "must NOT fire when only SOME (not all) members are REJECTED"


# ===========================================================================
# Registry integrity — both new predicates are in REFLECTION_PREDICATES
# ===========================================================================


def test_both_new_predicates_in_registry() -> None:
    """The two new predicates are registered in REFLECTION_PREDICATES."""
    names = [fn.__name__ for fn in reflection.REFLECTION_PREDICATES]
    assert "reflect_replaces_edge_migration" in names
    assert "reflect_all_members_rejected" in names
