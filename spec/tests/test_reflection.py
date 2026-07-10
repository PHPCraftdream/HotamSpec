"""Tests for the P0 REFLECTION band (hotam_spec.reflection + tools/what_now.py).

Guarantees:
  1. P_REFLECTION < P_STRUCTURE (ranked above all other bands).
  2. DRAFT-overhang fires when DRAFT >= SETTLED/2; disappears when ratio is healthy.
  3. UNENFORCED-SETTLED overhang fires when > 5 SETTLED are PROSE/STRUCTURAL.
  4. Over-budget operator fires per operator whose graph size exceeds budget.
  5. DEAD-assumption-on-ENFORCER fires for each (dead-assumption, enforced-req) pair.
  6. Derived-but-unbuilt fires for DECIDED conflicts whose derived ids remain DRAFT.
  7. The live meta-domain REFLECTION action count is 0–5 (reasonable range); today
     the DRAFT/SETTLED ratio is healthy (DRAFT 4 < SETTLED/2), so burn-down does
     NOT fire. We assert the specific conditions that fire today.
  8. R-reflection-predicates-first-class (§Reflection): the conditions are
     named predicate functions in hotam_spec.reflection composed by the
     what_now harness — never inlined in tool code — and each predicate fires
     in isolation on a synthetic graph violating it.
"""

from __future__ import annotations

from pathlib import Path


from hotam_spec import reflection  # noqa: E402
from hotam_spec.assumption import DEAD, IMPLEMENTS, Assumption  # noqa: E402
from hotam_spec.axis import Axis  # noqa: E402
from hotam_spec.conflict import Conflict, conflict_identity  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.operator import ContextBudget, Operator  # noqa: E402
from hotam_spec.requirement import DRAFT, ENFORCED, PROSE, SETTLED, Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

import what_now  # noqa: E402
from what_now import P_ADVISORY, P_REFLECTION, P_STRUCTURE, diagnose  # noqa: E402

# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

_DUMMY_AXES = (
    Axis(slug="ax-one", description="test axis one"),
    Axis(slug="ax-two", description="test axis two"),
)
_SH = (
    Stakeholder(id="s-a", name="A", domain="x"),
    Stakeholder(id="s-b", name="B", domain="y"),
    Stakeholder(id="s-c", name="C", domain="z"),
)


def _settled_req(rid: str, enforcement: str = ENFORCED) -> Requirement:
    return Requirement(
        id=rid,
        claim=f"claim for {rid}",
        owner="s-a",
        status=SETTLED,
        enforcement=enforcement,
        enforced_by=(f"test_{rid}",) if enforcement == ENFORCED else (),
    )


def _draft_req(rid: str) -> Requirement:
    return Requirement(
        id=rid, claim=f"draft claim for {rid}", owner="s-a", status=DRAFT
    )


# ---------------------------------------------------------------------------
# 1. Band ordering
# ---------------------------------------------------------------------------


def test_reflection_band_above_structure() -> None:
    """P_REFLECTION < P_STRUCTURE — REFLECTION is more urgent than STRUCTURE."""
    assert P_REFLECTION < P_STRUCTURE


# ---------------------------------------------------------------------------
# 2. DRAFT-overhang
# ---------------------------------------------------------------------------


def test_reflection_emits_draft_overhang() -> None:
    """DRAFT-overhang fires when DRAFT (10) >= SETTLED/2 (3/2=1.5)."""
    settled = tuple(_settled_req(f"R-s{i}") for i in range(3))
    drafts = tuple(_draft_req(f"R-d{i}") for i in range(10))
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=settled + drafts,
    )
    actions = diagnose(g)
    reflection_actions = [a for a in actions if a.priority == P_REFLECTION]
    burn_down = [a for a in reflection_actions if a.target == "burn-down"]
    assert burn_down, (
        "REFLECTION must fire burn-down when 10 DRAFT vs 3 SETTLED (ratio >= 0.5)"
    )
    assert burn_down[0].kind == "REFLECTION"


def test_reflection_no_overhang_when_ratio_healthy() -> None:
    """No burn-down action when DRAFT (3) < SETTLED/2 (20/2=10)."""
    settled = tuple(_settled_req(f"R-s{i}") for i in range(20))
    drafts = tuple(_draft_req(f"R-d{i}") for i in range(3))
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=settled + drafts,
    )
    actions = diagnose(g)
    burn_down = [
        a for a in actions if a.priority == P_REFLECTION and a.target == "burn-down"
    ]
    assert not burn_down, (
        "burn-down must NOT fire when 3 DRAFT vs 20 SETTLED (healthy ratio)"
    )


# ---------------------------------------------------------------------------
# 3. Over-budget operators
# ---------------------------------------------------------------------------


def test_reflection_emits_over_budget_operator() -> None:
    """Over-budget operator fires when graph has more nodes than limit."""
    # 5 requirements + 0 conflicts + 0 assumptions = 5 nodes; budget = 2.
    reqs = tuple(_settled_req(f"R-r{i}") for i in range(5))
    op = Operator(
        id="OP-test",
        stakeholder="s-a",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=2, measure="NODE_COUNT"),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=reqs,
        operators=(op,),
    )
    actions = diagnose(g)
    over_budget = [
        a for a in actions if a.priority == P_REFLECTION and a.target == "OP-test"
    ]
    assert over_budget, "REFLECTION must fire for OP-test holding 5 nodes > budget 2"
    assert "OP-test" in over_budget[0].imperative
    assert "5" in over_budget[0].imperative
    assert "2" in over_budget[0].imperative


def test_reflection_no_over_budget_when_within_limit() -> None:
    """No over-budget action when graph fits within operator budget."""
    reqs = tuple(_settled_req(f"R-r{i}") for i in range(3))
    op = Operator(
        id="OP-small",
        stakeholder="s-a",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=200, measure="NODE_COUNT"),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=reqs,
        operators=(op,),
    )
    actions = diagnose(g)
    over_budget = [
        a for a in actions if a.priority == P_REFLECTION and a.target == "OP-small"
    ]
    assert not over_budget, "No over-budget action when within limit"


def test_reflection_no_over_budget_when_limit_zero() -> None:
    """No over-budget action when budget limit is 0 (unbounded)."""
    reqs = tuple(_settled_req(f"R-r{i}") for i in range(100))
    op = Operator(
        id="OP-unbounded",
        stakeholder="s-a",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=0, measure="NODE_COUNT"),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=reqs,
        operators=(op,),
    )
    actions = diagnose(g)
    over_budget = [
        a for a in actions if a.priority == P_REFLECTION and a.target == "OP-unbounded"
    ]
    assert not over_budget, "Limit=0 means unbounded — no over-budget action"


# ---------------------------------------------------------------------------
# 4. DEAD-assumption-on-ENFORCER
# ---------------------------------------------------------------------------


def test_reflection_emits_dead_assumption_enforcer() -> None:
    """DEAD assumption + ENFORCED requirement resting on it fires REFLECTION."""
    dead_a = Assumption(
        id="A-dead-one",
        statement="This was true, now dead.",
        status=DEAD,
        owner="s-a",
    )
    enforced_r = Requirement(
        id="R-enforced-on-dead",
        claim="enforced claim resting on dead assumption",
        owner="s-b",
        status=SETTLED,
        assumptions=("A-dead-one",),
        enforcement=ENFORCED,
        enforced_by=("test_enforced_on_dead",),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        assumptions=(dead_a,),
        requirements=(enforced_r,),
    )
    actions = diagnose(g)
    stale = [
        a
        for a in actions
        if a.priority == P_REFLECTION and a.target == "R-enforced-on-dead"
    ]
    assert stale, (
        "REFLECTION must fire for R-enforced-on-dead resting on DEAD A-dead-one"
    )
    assert "A-dead-one" in stale[0].imperative
    assert "R-stale-substrate" in stale[0].imperative


def test_reflection_no_dead_assumption_on_non_enforced() -> None:
    """DEAD assumption + PROSE requirement does NOT fire the ENFORCER condition."""
    dead_a = Assumption(
        id="A-dead-two",
        statement="Dead.",
        status=DEAD,
        owner="s-a",
    )
    prose_r = Requirement(
        id="R-prose-on-dead",
        claim="prose claim on dead assumption",
        owner="s-b",
        status=SETTLED,
        assumptions=("A-dead-two",),
        enforcement=PROSE,
        enforced_by=(),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        assumptions=(dead_a,),
        requirements=(prose_r,),
    )
    actions = diagnose(g)
    stale = [
        a
        for a in actions
        if a.priority == P_REFLECTION and a.target == "R-prose-on-dead"
    ]
    assert not stale, "Dead-assumption-on-ENFORCER must NOT fire for PROSE requirements"


# ---------------------------------------------------------------------------
# 5. Derived-but-unbuilt
# ---------------------------------------------------------------------------


def test_reflection_emits_derived_unbuilt() -> None:
    """DECIDED conflict whose derived id is still DRAFT fires REFLECTION."""
    draft_r = _draft_req("R-derived-draft")
    # We need a valid conflict: axis must be in the graph's axes, members must resolve.
    # We put R-derived-draft as a member too to keep the graph self-consistent for
    # the REFLECTION test (we're testing derived-but-unbuilt, not structural validity).
    settled_r1 = _settled_req("R-parent-one")
    settled_r2 = _settled_req("R-parent-two")
    ax = "ax-one"
    ctx = "derived-unbuilt test context"
    c = Conflict(
        id=conflict_identity(ax, ctx),
        axis=ax,
        context=ctx,
        members=("R-parent-one", "R-parent-two"),
        steward="s-c",
        lifecycle="DECIDED(chose R-parent-one; derived R-derived-draft)",
        decided_by="s-c",
        derived=("R-derived-draft",),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(settled_r1, settled_r2, draft_r),
        conflicts=(c,),
    )
    actions = diagnose(g)
    unbuilt = [
        a
        for a in actions
        if a.priority == P_REFLECTION and a.target == "R-derived-draft"
    ]
    assert unbuilt, (
        "REFLECTION must fire for R-derived-draft derived by DECIDED conflict but still DRAFT"
    )
    assert c.id in unbuilt[0].imperative


def test_reflection_no_derived_unbuilt_when_settled() -> None:
    """DECIDED conflict whose derived id is SETTLED does NOT fire."""
    settled_derived = _settled_req("R-derived-settled")
    settled_r1 = _settled_req("R-parent-three")
    settled_r2 = _settled_req("R-parent-four")
    ax = "ax-one"
    ctx = "derived-settled test context"
    c = Conflict(
        id=conflict_identity(ax, ctx),
        axis=ax,
        context=ctx,
        members=("R-parent-three", "R-parent-four"),
        steward="s-c",
        lifecycle="DECIDED(chose R-parent-three)",
        decided_by="s-c",
        derived=("R-derived-settled",),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(settled_r1, settled_r2, settled_derived),
        conflicts=(c,),
    )
    actions = diagnose(g)
    unbuilt = [
        a
        for a in actions
        if a.priority == P_REFLECTION and a.target == "R-derived-settled"
    ]
    assert not unbuilt, (
        "derived-but-unbuilt must NOT fire when derived requirement is SETTLED"
    )


# ---------------------------------------------------------------------------
# 6. Live meta-domain smoke test
# ---------------------------------------------------------------------------


def test_real_meta_domain_reflection_today(active_graph) -> None:
    """Live meta-domain: REFLECTION actions are in a reasonable range and sensible.

    State after P11 (21 new requirements added):
      - SETTLED = 45, DRAFT = 24 -> DRAFT (24) >= SETTLED/2 (22.5) -> burn-down fires
      - UNENFORCED SETTLED > 5 may fire -> enforcement-gradient action
      - OP-director budget=200 vs graph_size ~100 -> within budget -> NO over-budget
      - No DEAD assumptions -> no dead-assumption-on-enforcer
      - R-active-loop-playbooks is DECIDED derived but SETTLED -> no derived-unbuilt
      - ~38 historical REJECTED nodes with prose REPLACES but no structural
        replaces edge -> reflect_replaces_edge_migration fires, but as an
        ADVISORY finding (Finding.advisory=True) it is routed to the P_ADVISORY
        band, not P_REFLECTION (§Attention, A2) — the honest 'not yet migrated'
        signal, kept out of the operator's P0 self-diagnosis band.
    """
    # Task #46, Measure 3: read the session-scoped active graph (frozen, shared
    # read-only) instead of rebuilding it per-test.
    g = active_graph
    actions = diagnose(g)
    reflection_actions = [a for a in actions if a.priority == P_REFLECTION]
    advisory_actions = [a for a in actions if a.priority == P_ADVISORY]

    # The reflection band no longer carries the migration-ratchet findings (they
    # moved to P_ADVISORY); the range covers the remaining non-advisory conditions.
    assert 0 <= len(reflection_actions) <= 20, (
        f"Expected 0-20 REFLECTION actions, got {len(reflection_actions)}: "
        f"{reflection_actions}"
    )

    # OP-director has budget=200 and graph is well under that; must NOT fire.
    over_budget = [a for a in reflection_actions if "OP-director" in a.target]
    assert not over_budget, f"OP-director must not be over-budget; got {over_budget}"

    # No DEAD assumptions in the meta-domain today; no enforcer-on-dead.
    enforcer_dead = [
        a for a in reflection_actions if "R-stale-substrate signal" in a.imperative
    ]
    assert not enforcer_dead, (
        f"No DEAD assumptions today; enforcer-dead must not fire; got {enforcer_dead}"
    )

    # The migration ratchet HONESTLY surfaces historical REJECTED nodes whose
    # anti-relitigation relation is prose-only (no structural replaces edge yet).
    # These are advisory; they must NOT block the graph and must live in the
    # P_ADVISORY band, not P_REFLECTION.
    migration = [
        a for a in advisory_actions
        if a.kind == "ADVISORY"
        and "replaces" in a.imperative.lower()
        and "migrate" in a.imperative.lower()
    ]
    assert migration, (
        "reflect_replaces_edge_migration must fire on the historical REJECTED "
        "nodes (they have prose REPLACES markers but no structural edge), "
        "surfaced in the P_ADVISORY band."
    )
    assert len(migration) <= 60, f"unexpectedly large migration-ratchet count: {len(migration)}"

    # No migration/advisory findings must leak back into P_REFLECTION.
    leaked = [
        a for a in reflection_actions
        if "replaces" in a.imperative.lower() and "migrate" in a.imperative.lower()
    ]
    assert not leaked, f"advisory migration findings must not appear in P_REFLECTION: {leaked}"


# ---------------------------------------------------------------------------
# 7. R-reflection-predicates-first-class — the self-diagnosis conditions are
#    substrate: named predicates in hotam_spec.reflection composed by the
#    harness, never inlined in tool code.
# ---------------------------------------------------------------------------

_PREDICATE_NAMES = [
    "reflect_draft_overhang",
    "reflect_unenforced_settled",
    "reflect_over_budget_operators",
    "reflect_dead_assumption_on_enforcer",
    "reflect_derived_but_unbuilt",
    "reflect_implements_decay",
    "reflect_replaces_edge_migration",
    "reflect_all_members_rejected",
]


def test_what_now_sources_reflection_predicates_from_module() -> None:
    """diagnose() composes hotam_spec.reflection — the conditions are not tool code.

    Enforcer of R-reflection-predicates-first-class (together with
    test_diagnose_p0_and_advisory_partition_reflection_findings): the harness
    imports the module's single entry point by reference, the registry names
    exactly the eight canonical predicates, and no reflection imperative text
    remains inlined in tools/what_now.py source.
    """
    assert what_now.all_findings is reflection.all_findings, (
        "what_now must compose hotam_spec.reflection.all_findings by reference"
    )
    names = [fn.__name__ for fn in reflection.REFLECTION_PREDICATES]
    assert names == _PREDICATE_NAMES, (
        f"REFLECTION_PREDICATES registry drifted: {names}"
    )
    src = Path(what_now.__file__).read_text(encoding="utf-8")
    for inlined_fragment in (
        "DRAFT-overhang:",
        "R-stale-substrate signal",
        "derived-but-unbuilt debt",
        "soft context-debt",
    ):
        assert inlined_fragment not in src, (
            f"reflection imperative {inlined_fragment!r} is inlined in "
            "tools/what_now.py — it must live in hotam_spec.reflection "
            "(R-reflection-predicates-first-class)"
        )


def _all_conditions_violating_graph() -> TensionGraph:
    """One synthetic graph that violates ALL eight reflection conditions at once."""
    from datetime import date as _date
    from datetime import timedelta as _timedelta

    from hotam_spec.requirement import REJECTED  # noqa: PLC0415

    dead_a = Assumption(
        id="A-dead-all", statement="was true, now dead", status=DEAD, owner="s-a"
    )
    # An IMPLEMENTS aspiration well past the decay threshold (30 days old).
    old_stamp = (_date.today() - _timedelta(days=30)).isoformat()
    decaying = Assumption(
        id="A-implements-old",
        statement="a striving forgotten",
        status=IMPLEMENTS,
        owner="s-a",
        created_at=old_stamp,
    )
    closeable = tuple(_settled_req(f"R-u{i}", enforcement=PROSE) for i in range(6))
    stale = Requirement(
        id="R-stale-all",
        claim="enforced claim on a dead premise",
        owner="s-b",
        status=SETTLED,
        assumptions=("A-dead-all",),
        enforcement=ENFORCED,
        enforced_by=("test_stale_all",),
    )
    # A REJECTED requirement with a prose REPLACES marker but NO structural
    # replaces edge — fires reflect_replaces_edge_migration.
    prose_rejected = Requirement(
        id="R-prose-rejected-all",
        claim="old design, rejected",
        owner="s-b",
        status=REJECTED,
        why="REJECTED -- REPLACES by R-successor-all; the old way.",
    )
    successor = _settled_req("R-successor-all")
    parents = (_settled_req("R-pa"), _settled_req("R-pb"))
    drafts = tuple(_draft_req(f"R-d{i}") for i in range(4)) + (
        _draft_req("R-unbuilt-all"),
    )
    ax, ctx = "ax-one", "all-conditions context"
    c = Conflict(
        id=conflict_identity(ax, ctx),
        axis=ax,
        context=ctx,
        members=("R-pa", "R-pb"),
        steward="s-c",
        lifecycle="DECIDED(chose R-pa; derived R-unbuilt-all)",
        decided_by="s-c",
        derived=("R-unbuilt-all",),
    )
    # A DETECTED conflict whose every member is REJECTED — fires
    # reflect_all_members_rejected (the ghost-connector signal).
    ghost_ax, ghost_ctx = "ax-two", "ghost-all-conditions context"
    ghost = Conflict(
        id=conflict_identity(ghost_ax, ghost_ctx),
        axis=ghost_ax,
        context=ghost_ctx,
        members=("R-prose-rejected-all",),
        steward="s-a",
        lifecycle="DETECTED",
    )
    # Add a second REJECTED member so the ghost has >= 2 members.
    ghost_member_two = Requirement(
        id="R-ghost-second",
        claim="another dead party",
        owner="s-c",
        status=REJECTED,
    )
    ghost = Conflict(
        id=conflict_identity(ghost_ax, ghost_ctx),
        axis=ghost_ax,
        context=ghost_ctx,
        members=("R-prose-rejected-all", "R-ghost-second"),
        steward="s-a",
        lifecycle="DETECTED",
    )
    op = Operator(
        id="OP-over-all",
        stakeholder="s-a",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=2, measure="NODE_COUNT"),
    )
    return TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=closeable
        + (stale, prose_rejected, successor, ghost_member_two)
        + parents
        + drafts,
        conflicts=(c, ghost),
        assumptions=(dead_a, decaying),
        operators=(op,),
    )


def test_diagnose_p0_and_advisory_partition_reflection_findings() -> None:
    """diagnose()'s P0 + P_ADVISORY bands ARE reflection.all_findings, split by advisory.

    Enforcer of R-reflection-predicates-first-class + §Attention A2: on a graph
    violating all eight conditions, hotam_spec.reflection.all_findings(g) is
    partitioned EXACTLY in two by Finding.advisory —

      * non-advisory findings (advisory=False) == diagnose()'s P_REFLECTION (P0) band
      * advisory findings (advisory=True) == diagnose()'s P_ADVISORY (P7) band

    — no finding is lost, duplicated, or misclassified between the two bands,
    and every predicate in the registry still contributed at least one
    finding. This supersedes the pre-A2 1:1 P0-equals-all_findings bijection:
    the invariant is now a bijection PER BAND instead of a single band.
    """
    g = _all_conditions_violating_graph()
    findings = reflection.all_findings(g)
    assert sorted({f.condition for f in findings}) == sorted(_PREDICATE_NAMES), (
        "the synthetic graph must fire every predicate in the registry"
    )

    non_advisory_findings = [f for f in findings if not f.advisory]
    advisory_findings = [f for f in findings if f.advisory]
    assert advisory_findings, (
        "the synthetic graph must fire at least one advisory finding "
        "(reflect_replaces_edge_migration / reflect_all_members_rejected)"
    )
    assert non_advisory_findings, (
        "the synthetic graph must fire at least one non-advisory finding"
    )

    actions = diagnose(g)
    p0_pairs = [(a.target, a.imperative) for a in actions if a.priority == P_REFLECTION]
    p_advisory_pairs = [(a.target, a.imperative) for a in actions if a.priority == P_ADVISORY]

    non_advisory_pairs = [(f.target, f.imperative) for f in non_advisory_findings]
    advisory_pairs = [(f.target, f.imperative) for f in advisory_findings]

    assert sorted(p0_pairs) == sorted(non_advisory_pairs), (
        "diagnose()'s P0 band must be exactly the non-advisory subset of "
        "hotam_spec.reflection.all_findings"
    )
    assert sorted(p_advisory_pairs) == sorted(advisory_pairs), (
        "diagnose()'s P_ADVISORY band must be exactly the advisory subset of "
        "hotam_spec.reflection.all_findings"
    )


def test_reflect_draft_overhang_fires_direct() -> None:
    """reflect_draft_overhang fires on 10 DRAFT vs 3 SETTLED; quiet when healthy."""
    settled = tuple(_settled_req(f"R-s{i}") for i in range(3))
    drafts = tuple(_draft_req(f"R-d{i}") for i in range(10))
    g = TensionGraph(
        axes=_DUMMY_AXES, stakeholders=_SH, requirements=settled + drafts
    )
    found = reflection.reflect_draft_overhang(g)
    assert len(found) == 1
    assert found[0].condition == "reflect_draft_overhang"
    assert found[0].target == "burn-down"
    assert "DRAFT-overhang: 10 DRAFT vs 3 SETTLED" in found[0].imperative
    healthy = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=tuple(_settled_req(f"R-s{i}") for i in range(20)),
    )
    assert reflection.reflect_draft_overhang(healthy) == []


def test_reflect_unenforced_settled_fires_direct() -> None:
    """reflect_unenforced_settled fires above the >5 closeable-debt threshold."""
    debt6 = tuple(_settled_req(f"R-u{i}", enforcement=PROSE) for i in range(6))
    g = TensionGraph(axes=_DUMMY_AXES, stakeholders=_SH, requirements=debt6)
    found = reflection.reflect_unenforced_settled(g)
    assert len(found) == 1
    assert found[0].condition == "reflect_unenforced_settled"
    assert found[0].target == "enforcement-gradient"
    assert "6 SETTLED requirements are closeable debt" in found[0].imperative
    debt5 = tuple(_settled_req(f"R-u{i}", enforcement=PROSE) for i in range(5))
    at_threshold = TensionGraph(
        axes=_DUMMY_AXES, stakeholders=_SH, requirements=debt5
    )
    assert reflection.reflect_unenforced_settled(at_threshold) == []


def test_reflect_over_budget_operators_fires_direct() -> None:
    """reflect_over_budget_operators fires per operator over its positive limit."""
    reqs = tuple(_settled_req(f"R-r{i}") for i in range(5))
    over = Operator(
        id="OP-o",
        stakeholder="s-a",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=2, measure="NODE_COUNT"),
    )
    unbounded = Operator(
        id="OP-free",
        stakeholder="s-b",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=0, measure="NODE_COUNT"),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=reqs,
        operators=(over, unbounded),
    )
    found = reflection.reflect_over_budget_operators(g)
    assert [f.target for f in found] == ["OP-o"], (
        "only the positive-limit, over-budget operator fires (limit=0 is unbounded)"
    )
    assert found[0].condition == "reflect_over_budget_operators"
    assert "holds 5 nodes (NODE_COUNT measure) > budget 2" in found[0].imperative


_REPO_ROOT_FOR_REFLECTION = Path(__file__).resolve().parents[2]
_ROOT_CLAUDE_MD_SIZE_FOR_REFLECTION = len(
    (_REPO_ROOT_FOR_REFLECTION / "CLAUDE.md").read_text(encoding="utf-8")
)


def test_reflect_over_budget_operators_crystal_chars_fires_direct() -> None:
    """reflect_over_budget_operators dispatches to CRYSTAL_CHARS measure,
    firing when root CLAUDE.md exceeds the limit (mirrors
    check_operator_within_budget's CRYSTAL_CHARS branch, R-context-budget-rule)."""
    op = Operator(
        id="OP-crystal-tight",
        stakeholder="s-a",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=1, measure="CRYSTAL_CHARS"),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=(),
        operators=(op,),
    )
    found = reflection.reflect_over_budget_operators(g)
    assert found, "CRYSTAL_CHARS measure must fire when crystal exceeds limit"
    assert found[0].target == "OP-crystal-tight"
    assert found[0].condition == "reflect_over_budget_operators"
    assert "CRYSTAL_CHARS measure" in found[0].imperative


def test_reflect_over_budget_operators_crystal_chars_green_when_under() -> None:
    """CRYSTAL_CHARS measure stays green when the limit comfortably covers
    the real, currently-committed root CLAUDE.md character count — even
    though the graph itself has far more than `limit` nodes, proving the
    predicate is NOT blind to the operator's declared measure."""
    reqs = tuple(_settled_req(f"R-r{i}") for i in range(50))
    op = Operator(
        id="OP-crystal-ok",
        stakeholder="s-a",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(
            limit=_ROOT_CLAUDE_MD_SIZE_FOR_REFLECTION + 1_000_000,
            measure="CRYSTAL_CHARS",
        ),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=reqs,
        operators=(op,),
    )
    found = reflection.reflect_over_budget_operators(g)
    assert not found, (
        "CRYSTAL_CHARS operator must not fire from node count alone "
        f"(50 nodes >> limit would fire under NODE_COUNT); got {found}"
    )


def test_reflect_over_budget_operators_names_crystallize_then_delegate() -> None:
    """R-context-bounded-delegation, ENFORCED: on a synthetic over-budget
    Operator (both NODE_COUNT and CRYSTAL_CHARS measures), the finding's
    imperative must name the two-step relief sequence in order --
    'crystallize' (R-crystallize-before-split) BEFORE 'delegate'
    (R-context-bounded-delegation) -- proving the delegate path is not a
    silent/implicit fallback but a named, ordered instruction the harness
    actually renders."""
    reqs = tuple(_settled_req(f"R-r{i}") for i in range(5))
    over_node_count = Operator(
        id="OP-over-node-count",
        stakeholder="s-a",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=1, measure="NODE_COUNT"),
    )
    over_crystal_chars = Operator(
        id="OP-over-crystal-chars",
        stakeholder="s-b",
        lifecycle="ACTIVE",
        context_budget=ContextBudget(limit=1, measure="CRYSTAL_CHARS"),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=reqs,
        operators=(over_node_count, over_crystal_chars),
    )
    found = reflection.reflect_over_budget_operators(g)
    assert len(found) == 2, f"expected both over-budget operators to fire, got {found}"
    for finding in found:
        assert finding.condition == "reflect_over_budget_operators"
        imperative = finding.imperative
        assert "crystallize first" in imperative
        assert "R-crystallize-before-split" in imperative
        assert "delegate" in imperative
        assert "R-context-bounded-delegation" in imperative
        # order: crystallize is instructed BEFORE delegate is even considered
        assert imperative.index("crystallize first") < imperative.index(
            "delegate"
        ), "imperative must name crystallize-first BEFORE delegate (R-crystallize-before-split, R-context-bounded-delegation)"


def test_reflect_dead_assumption_on_enforcer_fires_direct() -> None:
    """reflect_dead_assumption_on_enforcer fires per ENFORCED-req×DEAD-assumption pair."""
    dead_a = Assumption(id="A-x", statement="dead", status=DEAD, owner="s-a")
    enforced_r = Requirement(
        id="R-e",
        claim="enforced on dead",
        owner="s-b",
        status=SETTLED,
        assumptions=("A-x",),
        enforcement=ENFORCED,
        enforced_by=("test_e",),
    )
    prose_r = Requirement(
        id="R-p",
        claim="prose on dead",
        owner="s-b",
        status=SETTLED,
        assumptions=("A-x",),
        enforcement=PROSE,
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        assumptions=(dead_a,),
        requirements=(enforced_r, prose_r),
    )
    found = reflection.reflect_dead_assumption_on_enforcer(g)
    assert [f.target for f in found] == ["R-e"], (
        "only the ENFORCED requirement fires; PROSE is P2 DRIFT_FALLOUT territory"
    )
    assert found[0].condition == "reflect_dead_assumption_on_enforcer"
    assert "'R-e' rests on DEAD assumption 'A-x'" in found[0].imperative


def test_reflect_derived_but_unbuilt_fires_direct() -> None:
    """reflect_derived_but_unbuilt fires for DRAFT and for ABSENT derived ids."""
    parents = (_settled_req("R-m1"), _settled_req("R-m2"))
    draft_derived = _draft_req("R-still-draft")
    ax, ctx = "ax-one", "direct derived-unbuilt context"
    c = Conflict(
        id=conflict_identity(ax, ctx),
        axis=ax,
        context=ctx,
        members=("R-m1", "R-m2"),
        steward="s-c",
        lifecycle="DECIDED(chose R-m1)",
        decided_by="s-c",
        derived=("R-still-draft", "R-never-created"),
    )
    g = TensionGraph(
        axes=_DUMMY_AXES,
        stakeholders=_SH,
        requirements=parents + (draft_derived,),
        conflicts=(c,),
    )
    found = reflection.reflect_derived_but_unbuilt(g)
    assert [f.target for f in found] == ["R-still-draft", "R-never-created"], (
        "both the DRAFT derived id and the absent derived id must fire"
    )
    for f in found:
        assert f.condition == "reflect_derived_but_unbuilt"
        assert "derived-but-unbuilt debt" in f.imperative
