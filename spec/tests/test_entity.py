"""Tests for spec/src/tensio/entity.py — P21.1 Entity layer.

Covers EntityField, EntityType, EntityInstance, ENTITY_FIELD_KINDS, and
TensionGraph integration (entity_types, entities fields + helpers).
"""

import dataclasses

import pytest

from tensio.entity import (
    ENTITY_FIELD_KINDS,
    EntityField,
    EntityInstance,
    EntityType,
)
from tensio.graph import TensionGraph, entity_ids, entity_type_slugs
from tensio.lifecycle import REQUIREMENT_STATUS_LIFECYCLE


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _minimal_lifecycle():
    """Return a valid Lifecycle reusing the framework constant."""
    return REQUIREMENT_STATUS_LIFECYCLE


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_entity_type_is_frozen():
    """EntityType is a frozen dataclass — mutation must raise FrozenInstanceError."""
    et = EntityType(
        slug="customer",
        description="A paying customer.",
        lifecycle=_minimal_lifecycle(),
    )
    assert dataclasses.is_dataclass(et)
    with pytest.raises((dataclasses.FrozenInstanceError, AttributeError)):
        et.slug = "changed"  # type: ignore[misc]


def test_entity_type_reuses_lifecycle_keystone():
    """EntityType carries a Lifecycle field; it has no parallel state-machine attrs."""
    et = EntityType(
        slug="order",
        description="A customer order.",
        lifecycle=_minimal_lifecycle(),
    )
    # Lifecycle keystone is on the field
    from tensio.lifecycle import Lifecycle

    assert isinstance(et.lifecycle, Lifecycle)
    # No parallel state-machine attributes directly on EntityType
    assert not hasattr(et, "states")
    assert not hasattr(et, "transitions")
    # States are accessible through the lifecycle
    assert len(et.lifecycle.states) > 0


def test_entity_field_kinds_controlled():
    """ENTITY_FIELD_KINDS contains exactly the 5 expected kinds."""
    expected = {"string", "number", "enum", "reference", "state"}
    assert ENTITY_FIELD_KINDS == expected
    # EntityField with an out-of-vocabulary kind is constructable (check_* fires later)
    f = EntityField(name="x", kind="weird-kind")
    assert f.kind == "weird-kind"


def test_entity_instance_field_value_lookup():
    """EntityInstance.field_value returns value by name or None for missing keys."""
    inst = EntityInstance(
        id="ENT-customer-alice",
        entity_type="customer",
        state="DRAFT",
        field_values=(("email", "a@b"), ("tier", "gold")),
    )
    assert inst.field_value("email") == "a@b"
    assert inst.field_value("tier") == "gold"
    assert inst.field_value("missing") is None


def test_tension_graph_empty_with_entity_types_only():
    """TensionGraph with only entity_types populated is NOT empty."""
    et = EntityType(
        slug="foo",
        description="",
        lifecycle=_minimal_lifecycle(),
    )
    g = TensionGraph(entity_types=(et,))
    assert not g.is_empty()


def test_tension_graph_helpers_correct():
    """entity_type_slugs and entity_ids return correct frozensets."""
    et1 = EntityType(slug="customer", description="", lifecycle=_minimal_lifecycle())
    et2 = EntityType(slug="order", description="", lifecycle=_minimal_lifecycle())
    inst = EntityInstance(
        id="ENT-customer-alice", entity_type="customer", state="DRAFT"
    )
    g = TensionGraph(entity_types=(et1, et2), entities=(inst,))

    assert entity_type_slugs(g) == frozenset({"customer", "order"})
    assert entity_ids(g) == frozenset({"ENT-customer-alice"})

    # Empty-collection edge case
    g_empty = TensionGraph()
    assert entity_type_slugs(g_empty) == frozenset()
    assert entity_ids(g_empty) == frozenset()


def test_tension_graph_is_empty_with_no_entities():
    """Completely empty TensionGraph (including entity collections) is empty."""
    g = TensionGraph()
    assert g.is_empty()
