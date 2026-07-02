"""Tests for the check_entity_* invariant family (§Entity aspect, P21.2).

Each check_* is tested with:
  1. A clean graph → returns [].
  2. A broken graph → fires the expected Violation.

Plus an aspect-gating sanity test (empty entity_types/entities → all six return []).
"""

from __future__ import annotations


from hotam_spec.assumption import DEAD, HOLDS, Assumption
from hotam_spec.entity import (
    ENTITY_FIELD_KINDS,
    EntityField,
    EntityInstance,
    EntityType,
)
from hotam_spec.graph import TensionGraph, dead_assumptions
from hotam_spec.invariants import (
    check_entity_field_kind_known,
    check_entity_instance_id_prefix,
    check_entity_instance_refs_resolve,
    check_entity_instance_required_fields,
    check_entity_instance_state_in_lifecycle,
    check_entity_type_lifecycle_wellformed,
    check_transition_guard_assumption_resolves,
    check_typed_anchors_entity,
)
from hotam_spec.lifecycle import (
    INITIAL,
    NORMAL,
    QUIESCENT,
    Lifecycle,
    State,
    Transition,
)
from hotam_spec.stakeholder import Stakeholder

# ---------------------------------------------------------------------------
# Shared fixtures
# ---------------------------------------------------------------------------

CUSTOMER_LC = Lifecycle(
    slug="customer-lifecycle",
    states=(
        State("ACTIVE", kind=INITIAL),
        State("SUSPENDED", kind=NORMAL),
        State("CLOSED", kind=QUIESCENT),
    ),
    transitions=(
        Transition("ACTIVE", "SUSPENDED", event="suspend"),
        Transition("ACTIVE", "CLOSED", event="close"),
        Transition("SUSPENDED", "ACTIVE", event="reopen"),
    ),
    cyclic=False,
)

CUSTOMER_TYPE = EntityType(
    slug="customer",
    description="A paying account.",
    lifecycle=CUSTOMER_LC,
    fields=(
        EntityField(name="email", kind="string", required=True),
        EntityField(name="tier", kind="enum", required=False),
        EntityField(
            name="owner", kind="reference", required=True, ref_target="stakeholder"
        ),
    ),
)

_STAKEHOLDER = Stakeholder(id="s-product", name="Product", domain="customer")


def _graph(**kwargs) -> TensionGraph:
    """Build a minimal TensionGraph for entity invariant tests."""
    return TensionGraph(
        stakeholders=kwargs.get("stakeholders", (_STAKEHOLDER,)),
        entity_types=kwargs.get("entity_types", ()),
        entities=kwargs.get("entities", ()),
    )


def _good_instance() -> EntityInstance:
    return EntityInstance(
        id="ENT-customer-acme",
        entity_type="customer",
        state="ACTIVE",
        field_values=(
            ("email", "acme@example.com"),
            ("owner", "s-product"),
        ),
    )


# ---------------------------------------------------------------------------
# Aspect-gating: empty graph → all checks return []
# ---------------------------------------------------------------------------


def test_aspect_gating_empty_entity_types_and_entities():
    g = _graph()
    assert check_entity_type_lifecycle_wellformed(g) == []
    assert check_entity_instance_state_in_lifecycle(g) == []
    assert check_entity_instance_required_fields(g) == []
    assert check_entity_instance_id_prefix(g) == []
    assert check_entity_instance_refs_resolve(g) == []
    assert check_entity_field_kind_known(g) == []
    assert check_typed_anchors_entity(g) == []


# ---------------------------------------------------------------------------
# check_entity_type_lifecycle_wellformed
# ---------------------------------------------------------------------------


def test_entity_type_lifecycle_wellformed_clean():
    g = _graph(entity_types=(CUSTOMER_TYPE,))
    assert check_entity_type_lifecycle_wellformed(g) == []


def test_entity_type_lifecycle_wellformed_broken_no_initial():
    bad_lc = Lifecycle(
        slug="bad-lc",
        states=(
            State("OPEN", kind=NORMAL),
            State("CLOSED", kind=QUIESCENT),
        ),
        transitions=(Transition("OPEN", "CLOSED", event="close"),),
        cyclic=False,
    )
    bad_type = EntityType(slug="widget", description=".", lifecycle=bad_lc)
    g = _graph(entity_types=(bad_type,))
    violations = check_entity_type_lifecycle_wellformed(g)
    assert len(violations) >= 1
    assert all(v.target == "widget" for v in violations)
    assert any("INITIAL" in v.message for v in violations)


# ---------------------------------------------------------------------------
# check_entity_instance_state_in_lifecycle
# ---------------------------------------------------------------------------


def test_entity_instance_state_in_lifecycle_clean():
    g = _graph(
        entity_types=(CUSTOMER_TYPE,),
        entities=(_good_instance(),),
    )
    assert check_entity_instance_state_in_lifecycle(g) == []


def test_entity_instance_state_in_lifecycle_unknown_state():
    inst = EntityInstance(
        id="ENT-customer-bad",
        entity_type="customer",
        state="BOGUS",
        field_values=(("email", "x@x.com"), ("owner", "s-product")),
    )
    g = _graph(entity_types=(CUSTOMER_TYPE,), entities=(inst,))
    violations = check_entity_instance_state_in_lifecycle(g)
    assert len(violations) == 1
    assert violations[0].target == "ENT-customer-bad"
    assert "BOGUS" in violations[0].message


def test_entity_instance_state_in_lifecycle_unknown_entity_type():
    inst = EntityInstance(
        id="ENT-unknown-x",
        entity_type="unknown",
        state="ACTIVE",
    )
    g = _graph(entity_types=(), entities=(inst,))
    violations = check_entity_instance_state_in_lifecycle(g)
    assert len(violations) == 1
    assert violations[0].target == "ENT-unknown-x"
    assert "not declared" in violations[0].message


# ---------------------------------------------------------------------------
# check_entity_instance_required_fields
# ---------------------------------------------------------------------------


def test_entity_instance_required_fields_clean():
    g = _graph(
        entity_types=(CUSTOMER_TYPE,),
        entities=(_good_instance(),),
    )
    assert check_entity_instance_required_fields(g) == []


def test_entity_instance_required_fields_missing():
    # Missing required 'email' and 'owner'
    inst = EntityInstance(
        id="ENT-customer-bare",
        entity_type="customer",
        state="ACTIVE",
        field_values=(),
    )
    g = _graph(entity_types=(CUSTOMER_TYPE,), entities=(inst,))
    violations = check_entity_instance_required_fields(g)
    assert len(violations) == 2
    assert violations[0].target == "ENT-customer-bare"
    missing_names = {v.message.split("'")[1] for v in violations}
    assert missing_names == {"email", "owner"}


def test_entity_instance_required_fields_optional_missing_ok():
    # 'tier' is optional — missing it should not fire
    inst = EntityInstance(
        id="ENT-customer-notier",
        entity_type="customer",
        state="ACTIVE",
        field_values=(("email", "x@x.com"), ("owner", "s-product")),
    )
    g = _graph(entity_types=(CUSTOMER_TYPE,), entities=(inst,))
    assert check_entity_instance_required_fields(g) == []


# ---------------------------------------------------------------------------
# check_entity_instance_id_prefix
# ---------------------------------------------------------------------------


def test_entity_instance_id_prefix_clean():
    g = _graph(entity_types=(CUSTOMER_TYPE,), entities=(_good_instance(),))
    assert check_entity_instance_id_prefix(g) == []


def test_entity_instance_id_prefix_wrong_prefix():
    inst = EntityInstance(
        id="CUST-acme",
        entity_type="customer",
        state="ACTIVE",
    )
    g = _graph(entity_types=(CUSTOMER_TYPE,), entities=(inst,))
    violations = check_entity_instance_id_prefix(g)
    assert len(violations) == 1
    assert violations[0].target == "CUST-acme"
    assert "ENT-customer-" in violations[0].message


def test_entity_instance_id_prefix_missing_type_segment():
    inst = EntityInstance(
        id="ENT-acme",  # missing slug segment
        entity_type="customer",
        state="ACTIVE",
    )
    g = _graph(entity_types=(CUSTOMER_TYPE,), entities=(inst,))
    violations = check_entity_instance_id_prefix(g)
    assert len(violations) == 1
    assert violations[0].target == "ENT-acme"


# ---------------------------------------------------------------------------
# check_entity_instance_refs_resolve
# ---------------------------------------------------------------------------


def test_entity_instance_refs_resolve_clean():
    g = _graph(
        entity_types=(CUSTOMER_TYPE,),
        entities=(_good_instance(),),
    )
    assert check_entity_instance_refs_resolve(g) == []


def test_entity_instance_refs_resolve_bad_stakeholder_ref():
    inst = EntityInstance(
        id="ENT-customer-orphan",
        entity_type="customer",
        state="ACTIVE",
        field_values=(("email", "x@x.com"), ("owner", "s-nonexistent")),
    )
    g = _graph(entity_types=(CUSTOMER_TYPE,), entities=(inst,))
    violations = check_entity_instance_refs_resolve(g)
    assert len(violations) == 1
    assert violations[0].target == "ENT-customer-orphan"
    assert "owner" in violations[0].message
    assert "s-nonexistent" in violations[0].message


def test_entity_instance_refs_resolve_optional_empty_ref_ok():
    # Optional reference field with empty value is fine
    ref_type = EntityType(
        slug="order",
        description="An order.",
        lifecycle=CUSTOMER_LC,
        fields=(
            EntityField(
                name="linked_customer",
                kind="reference",
                required=False,
                ref_target="customer",
            ),
        ),
    )
    inst = EntityInstance(
        id="ENT-order-1",
        entity_type="order",
        state="ACTIVE",
        field_values=(),  # no linked_customer value
    )
    g = _graph(entity_types=(ref_type,), entities=(inst,))
    assert check_entity_instance_refs_resolve(g) == []


def test_entity_instance_refs_resolve_entity_slug_ref():
    """Reference to another entity by slug resolves correctly."""
    order_type = EntityType(
        slug="order",
        description="An order.",
        lifecycle=CUSTOMER_LC,
        fields=(
            EntityField(
                name="customer", kind="reference", required=True, ref_target="customer"
            ),
        ),
    )
    customer_inst = _good_instance()
    order_inst = EntityInstance(
        id="ENT-order-1",
        entity_type="order",
        state="ACTIVE",
        field_values=(("customer", "ENT-customer-acme"),),
    )
    g = _graph(
        entity_types=(CUSTOMER_TYPE, order_type), entities=(customer_inst, order_inst)
    )
    assert check_entity_instance_refs_resolve(g) == []


# ---------------------------------------------------------------------------
# check_entity_field_kind_known
# ---------------------------------------------------------------------------


def test_entity_field_kind_known_clean():
    g = _graph(entity_types=(CUSTOMER_TYPE,))
    assert check_entity_field_kind_known(g) == []


def test_entity_field_kind_known_unknown_kind():
    bad_type = EntityType(
        slug="gadget",
        description=".",
        lifecycle=CUSTOMER_LC,
        fields=(EntityField(name="color", kind="hexcolor"),),
    )
    g = _graph(entity_types=(bad_type,))
    violations = check_entity_field_kind_known(g)
    assert len(violations) == 1
    assert violations[0].target == "gadget"
    assert "hexcolor" in violations[0].message


def test_entity_field_kind_known_all_valid_kinds():
    fields = tuple(EntityField(name=k, kind=k) for k in sorted(ENTITY_FIELD_KINDS))
    et = EntityType(
        slug="allkinds", description=".", lifecycle=CUSTOMER_LC, fields=fields
    )
    g = _graph(entity_types=(et,))
    assert check_entity_field_kind_known(g) == []


# ---------------------------------------------------------------------------
# check_typed_anchors_entity
# ---------------------------------------------------------------------------


def test_typed_anchors_entity_clean():
    g = _graph(entity_types=(CUSTOMER_TYPE,), entities=(_good_instance(),))
    assert check_typed_anchors_entity(g) == []


def test_typed_anchors_entity_wrong_prefix():
    inst = EntityInstance(
        id="CUST-acme",
        entity_type="customer",
        state="ACTIVE",
    )
    g = _graph(entity_types=(CUSTOMER_TYPE,), entities=(inst,))
    violations = check_typed_anchors_entity(g)
    assert len(violations) == 1
    assert violations[0].target == "CUST-acme"
    assert "ENT-" in violations[0].message


def test_typed_anchors_entity_empty_entities():
    g = _graph(entity_types=(CUSTOMER_TYPE,), entities=())
    assert check_typed_anchors_entity(g) == []


# ---------------------------------------------------------------------------
# check_transition_guard_assumption_resolves
# ---------------------------------------------------------------------------

_A_FRAUD_WINDOW = Assumption(
    id="A-fraud-window-30d",
    statement="A fraud signal is actionable for 30 days from detection.",
    status=HOLDS,
    owner="s-product",
)


def test_transition_guard_assumption_resolves_clean():
    """A guard_assumption naming a real Assumption id is not a violation."""
    lc = Lifecycle(
        slug="customer-lifecycle-guarded",
        states=(
            State("ACTIVE", kind=INITIAL),
            State("SUSPENDED", kind=QUIESCENT),
        ),
        transitions=(
            Transition(
                "ACTIVE",
                "SUSPENDED",
                event="suspend",
                guard_assumption="A-fraud-window-30d",
            ),
        ),
        cyclic=False,
    )
    et = EntityType(slug="customer", description=".", lifecycle=lc)
    g = _graph(entity_types=(et,), stakeholders=(_STAKEHOLDER,))
    g = TensionGraph(
        stakeholders=g.stakeholders,
        assumptions=(_A_FRAUD_WINDOW,),
        entity_types=g.entity_types,
        entities=g.entities,
    )
    assert check_transition_guard_assumption_resolves(g) == []


def test_transition_guard_assumption_resolves_dangling_fires():
    """A guard_assumption naming a nonexistent Assumption id fires exactly one
    Violation targeting the EntityType slug."""
    lc = Lifecycle(
        slug="customer-lifecycle-dangling",
        states=(
            State("ACTIVE", kind=INITIAL),
            State("SUSPENDED", kind=QUIESCENT),
        ),
        transitions=(
            Transition(
                "ACTIVE",
                "SUSPENDED",
                event="suspend",
                guard_assumption="A-does-not-exist",
            ),
        ),
        cyclic=False,
    )
    et = EntityType(slug="customer", description=".", lifecycle=lc)
    g = _graph(entity_types=(et,))
    violations = check_transition_guard_assumption_resolves(g)
    assert len(violations) == 1
    assert violations[0].target == "customer"
    assert "A-does-not-exist" in violations[0].message


def test_transition_guard_assumption_empty_is_not_a_violation():
    """guard_assumption defaults to None/empty — no reference to check, no fire."""
    g = _graph(entity_types=(CUSTOMER_TYPE,))
    assert check_transition_guard_assumption_resolves(g) == []


def test_transition_guard_assumption_dead_assumption_still_visible_in_dependents():
    """R-stale-substrate drift-fallout: a DEAD Assumption referenced ONLY via a
    Transition.guard_assumption must still be discoverable through
    graph.dead_assumptions() — the guard is a real edge, not an invisible one,
    so the harness's dead-assumption fallout machinery can surface it even
    though Requirement/Conflict-scoped helpers (requirements_on_assumption,
    conflicts_on_assumption) do not know about Transition edges."""
    dead_a = Assumption(
        id="A-fraud-window-30d",
        statement="A fraud signal is actionable for 30 days from detection.",
        status=DEAD,
        owner="s-product",
    )
    lc = Lifecycle(
        slug="customer-lifecycle-dead-guard",
        states=(
            State("ACTIVE", kind=INITIAL),
            State("SUSPENDED", kind=QUIESCENT),
        ),
        transitions=(
            Transition(
                "ACTIVE",
                "SUSPENDED",
                event="suspend",
                guard_assumption="A-fraud-window-30d",
            ),
        ),
        cyclic=False,
    )
    et = EntityType(slug="customer", description=".", lifecycle=lc)
    g = TensionGraph(
        stakeholders=(_STAKEHOLDER,),
        assumptions=(dead_a,),
        entity_types=(et,),
    )
    # The guard's referential integrity still holds (id resolves) ...
    assert check_transition_guard_assumption_resolves(g) == []
    # ... but the assumption it rests on is DEAD and visible to the harness's
    # dead-assumption fallout scan, so the guard's staleness is not hidden.
    assert dead_a.id in {a.id for a in dead_assumptions(g)}
