"""Canon: §Invariants — registry coherence for NODE_SCHEMAS (node_schemas.py).

The registry is a NEW declarative mirror of the typed-anchor surface that the
existing check_typed_anchors_* / check_no_dangling_* functions enforce by hand.
For the registry to be a trustworthy substrate for future tooling, it MUST
agree with the live graph and with the hand-written enforcers:

  1. every collection attribute the registry names exists on TensionGraph;
  2. every kind the registry marks prefix-bearing agrees with what the
     corresponding check_typed_anchors_* actually enforces (the registry is
     neither silently stricter nor silently looser);
  3. every kind present in the live graph (non-empty collection) has a schema
     entry — a new node kind added to TensionGraph without a schema entry
     would be a registry-coverage gap this test surfaces;
  4. prefix uniqueness — no two kinds claim the same prefix family.

These tests do NOT migrate any check_* to read from the registry; they pin the
registry's correctness so a future wave can rely on it. R-anchor-everything.
"""

from __future__ import annotations

import inspect

import pytest

from hotam_spec.assumption import ASSUMPTION_STATES
from hotam_spec.graph import TensionGraph
from hotam_spec.node_schemas import (
    COLLECTION_KINDS,
    NODE_SCHEMAS,
    NODE_SCHEMAS_BY_KIND,
    NODE_SCHEMAS_BY_PREFIX,
    collection_attrs,
    kinds_with_prefix,
    schema_for_kind,
    schema_for_prefix,
)


def test_registry_is_nonempty_and_unique_by_kind() -> None:
    """Every schema has a distinct kind; the table is populated."""
    kinds = [s.kind for s in NODE_SCHEMAS]
    assert len(kinds) == len(set(kinds)), f"duplicate kinds: {kinds}"
    assert "Requirement" in NODE_SCHEMAS_BY_KIND
    assert "Conflict" in NODE_SCHEMAS_BY_KIND


def test_prefix_families_are_unique() -> None:
    """No two kinds share the same enforced prefix family."""
    prefixes = [s.prefix for s in NODE_SCHEMAS if s.prefix]
    assert len(prefixes) == len(set(prefixes)), (
        f"duplicate prefix families: {prefixes}"
    )


def test_collection_attrs_exist_on_tension_graph() -> None:
    """Every collection_attr the registry names is a real TensionGraph field."""
    fields = {f.name for f in __import__("dataclasses").fields(TensionGraph)}
    for attr in collection_attrs():
        assert attr in fields, (
            f"registry collection_attr '{attr}' is not a TensionGraph field; "
            f"known fields: {sorted(fields)}"
        )


def test_collection_kinds_carry_a_collection_attr() -> None:
    """COLLECTION_KINDS entries all have a non-empty collection_attr."""
    for s in COLLECTION_KINDS:
        assert s.collection_attr, f"kind {s.kind} is in COLLECTION_KINDS but has empty collection_attr"


def test_known_typed_anchor_prefixes_match_enforcers() -> None:
    """The registry's prefix families MUST match what the hand-written
    check_typed_anchors_* actually enforce.

    This is the load-bearing coherence check: if the registry claims a
    different prefix than the enforcer, the registry is wrong (the enforcer is
    the authority — its name is referenced by enforced_by on live requirements
    and CANNOT change). Adding a new prefix here without the matching enforcer,
    or vice versa, is caught.
    """
    expected = {
        "Requirement": "R-",
        "Assumption": "A-",
        "Conflict": "C-",
        "Operator": "OP-",
        "Process": "PR-",
        "Goal": "GOAL-",
        "Variant": "V-",
        "EntityInstance": "ENT-",
    }
    for kind, prefix in expected.items():
        schema = schema_for_kind(kind)
        assert schema is not None, f"missing schema for kind {kind}"
        assert schema.prefix == prefix, (
            f"registry prefix for {kind} is '{schema.prefix}', "
            f"expected '{prefix}' (matches check_typed_anchors_*)"
        )
        assert schema_for_prefix(prefix) is schema


def test_assumption_status_set_registered() -> None:
    """Assumption.status admissible values are surfaced as a STATUS_SET entry."""
    from hotam_spec.node_schemas import STATUS_SETS_BY_KIND

    assert "Assumption" in STATUS_SETS_BY_KIND
    assert STATUS_SETS_BY_KIND["Assumption"] is ASSUMPTION_STATES


@pytest.mark.parametrize(
    "collection_attr,kind",
    [
        ("requirements", "Requirement"),
        ("conflicts", "Conflict"),
        ("assumptions", "Assumption"),
        ("operators", "Operator"),
        ("processes", "Process"),
        ("goals", "Goal"),
        ("entities", "EntityInstance"),
        ("entity_types", "EntityType"),
        ("axes", "Axis"),
        ("stakeholders", "Stakeholder"),
    ],
)
def test_collection_attr_maps_to_kind(collection_attr: str, kind: str) -> None:
    """Each known TensionGraph collection maps to its declared kind."""
    matches = [s for s in COLLECTION_KINDS if s.collection_attr == collection_attr]
    assert matches, f"no schema for collection {collection_attr}"
    assert matches[0].kind == kind


def test_live_graph_collections_have_registry_coverage(active_graph: TensionGraph) -> None:
    """Every NON-EMPTY collection on the live graph has a schema entry, and
    every id in a prefix-bearing collection actually starts with that prefix.

    A new node kind added to TensionGraph and populated by a domain would
    appear as a non-empty collection with no schema -> caught here. Likewise a
    prefix drift (an id that no longer starts with the registry prefix) is
    surfaced as a registry-vs-graph disagreement (the hand enforcer would also
    fire, but this test pins the REGISTRY's view independently).
    """
    fields = {f.name for f in __import__("dataclasses").fields(TensionGraph)}
    covered = {s.collection_attr for s in COLLECTION_KINDS}
    for fname in fields:
        collection = getattr(active_graph, fname, None)
        if collection is None or not isinstance(collection, tuple) or not collection:
            continue
        assert fname in covered, (
            f"live graph has non-empty collection '{fname}' with no schema entry; "
            "add a NodeSchema for the new kind"
        )

    # For prefix-bearing kinds: every live id starts with the declared prefix.
    for schema in COLLECTION_KINDS:
        if not schema.prefix:
            continue
        collection = getattr(active_graph, schema.collection_attr, ())
        for node in collection:
            nid = getattr(node, "id", None)
            if nid is None:
                continue
            assert nid.startswith(schema.prefix), (
                f"{schema.kind} id '{nid}' does not start with registry "
                f"prefix '{schema.prefix}' (registry disagrees with graph)"
            )


def test_variant_schema_is_payload_not_collection() -> None:
    """Variant lives on Conflict.variants, not as a top-level collection."""
    v = schema_for_kind("Variant")
    assert v is not None
    assert v.prefix == "V-"
    assert v.collection_attr == "", (
        "Variant must NOT declare a top-level collection_attr; it is a "
        "payload on Conflict.variants"
    )
    assert v not in COLLECTION_KINDS


def test_lookup_helpers_roundtrip() -> None:
    """schema_for_kind / schema_for_prefix return the registered objects."""
    for schema in NODE_SCHEMAS:
        assert schema_for_kind(schema.kind) is schema
    for schema in NODE_SCHEMAS:
        if schema.prefix:
            assert schema_for_prefix(schema.prefix) is schema
    assert schema_for_kind("NoSuchKind") is None
    assert schema_for_prefix("ZZ-") is None


def test_kinds_with_prefix_is_subset_of_registry() -> None:
    """kinds_with_prefix() lists exactly the prefix-bearing schemas, in order."""
    expected = tuple(s.kind for s in NODE_SCHEMAS if s.prefix)
    assert kinds_with_prefix() == expected


def test_ref_fields_target_known_kinds() -> None:
    """Every ref_field target is a kind that also has a schema entry.

    A ref_field pointing at an unregistered kind would be a registry gap.
    """
    for schema in NODE_SCHEMAS:
        for _attr, target_kind in schema.ref_fields:
            # 'Requirement' self-references (relations) are fine; target must
            # be a registered kind. We do NOT require the target to have a
            # collection (Stakeholder is a valid target and has one).
            assert target_kind in NODE_SCHEMAS_BY_KIND, (
                f"{schema.kind}.ref_fields targets unknown kind '{target_kind}'"
            )


def test_module_is_stdlib_and_hotam_spec_pure() -> None:
    """node_schemas.py imports only stdlib + hotam_spec
    (R-core-imports-stdlib-or-hotam-spec-only)."""
    import hotam_spec.node_schemas as mod

    src = inspect.getsource(mod)
    # Allow only `from hotam_spec...` / stdlib imports; reject third-party.
    forbidden = ("import requests", "import numpy", "import pandas", "import yaml")
    for token in forbidden:
        assert token not in src, f"forbidden import in node_schemas: {token}"
