"""Canon: §Invariants — registry of every typed-anchor node kind in the framework.

RULE (R-anchor-everything): each kind of node the graph can carry has a
NodeSchema — its id-prefix family, the TensionGraph collection attribute that
holds its instances, the fields that reference OTHER nodes (ref_fields), and
(where applicable) the Lifecycle that governs the node's status field.

This module is the single declarative source describing node kinds. It is a
REGISTRY, not an enforcer: the existing check_typed_anchors_* / check_no_dangling_*
functions remain the enforcers (their names are referenced by enforced_by in
domains/*/graph.py, so they CANNOT be renamed). The registry exists so that:

  - new tooling (skeletons, generators, validators) can consult one place
    instead of re-deriving the prefix/collection mapping;
  - a future wave can drive more check_* from the registry without touching
    the 88 hand-written functions in one shot;
  - tests can assert the registry matches the live graph (every kind present
    in the graph has a schema; prefixes agree with what the enforcers expect).

CONTENT-FREE: no business data. stdlib + hotam_spec only
(R-core-imports-stdlib-or-hotam-spec-only).

WHY a frozenset/frozen-dataclass registry (not dynamic discovery): the kinds,
prefixes and ref-fields are structural facts about the framework ontology that
change rarely; a declarative table is auditable and diff-stable, whereas
walking dataclass fields at runtime would couple the registry to incidental
field additions (e.g. a new comment field is NOT a new ref-field).
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable

from hotam_spec.assumption import ASSUMPTION_STATES
from hotam_spec.lifecycle import (
    CONFLICT_LIFECYCLE,
    REQUIREMENT_STATUS_LIFECYCLE,
    Lifecycle,
)
from hotam_spec.operator import OPERATOR_LIFECYCLE
from hotam_spec.process import GOAL_LIFECYCLE, PROCESS_LIFECYCLE


@dataclass(frozen=True)
class NodeSchema:
    """Canon: §Invariants — one node kind's structural fingerprint.

    Fields:
      kind            — the Proposal/kind name and the human label
                        (e.g. 'Requirement', 'Conflict', 'Axis').
      prefix          — the typed-anchor id prefix family enforced by
                        check_typed_anchors_* (e.g. 'R-', 'C-', 'OP-').
                        Empty string means the kind has NO enforced prefix
                        family (Axis/Stakeholder use bare slugs/ids; the
                        framework has not pinned a prefix for them).
      collection_attr — the TensionGraph attribute holding instances
                        (e.g. 'requirements', 'conflicts'); empty string for
                        kinds that are payloads on another node (Variant lives
                        on Conflict.variants, not as a top-level collection).
      ref_fields      — tuple of (attr_name, target_kind) pairs for fields
                        that reference OTHER nodes — the dangling-id surface.
                        Used by check_no_dangling_*; documented here so a
                        future table-driven dangling check can consult it.
      lifecycle       — the Lifecycle governing the node's status/lifecycle
                        field, or None when the kind has no lifecycle field.

    WHY frozen: the schema describes an immutable structural fact; building it
    at import time and sharing the instance keeps the registry a pure table.
    """

    kind: str
    prefix: str = ""
    collection_attr: str = ""
    ref_fields: tuple[tuple[str, str], ...] = ()
    lifecycle: Lifecycle | None = None


# ---------------------------------------------------------------------------
# The registry
# ---------------------------------------------------------------------------
#
# Order is the canonical declaration order (Requirement first, as the central
# node; then Conflict the connector; then Assumption; then the acting/process/
# goal aspects; then Entity; then the vocabulary/accountability kinds; finally
# Variant as a payload, not a top-level node).

REQUIREMENT_SCHEMA = NodeSchema(
    kind="Requirement",
    prefix="R-",
    collection_attr="requirements",
    ref_fields=(
        ("owner", "Stakeholder"),
        # assumptions[*] -> Assumption, relations[*].target -> Requirement,
        # relations[*].kind -> RELATION_KINDS. Captured as the dangling surface
        # documented by check_no_dangling_requirement_*.
        ("assumptions", "Assumption"),
        ("relations", "Requirement"),
    ),
    lifecycle=REQUIREMENT_STATUS_LIFECYCLE,
)

CONFLICT_SCHEMA = NodeSchema(
    kind="Conflict",
    prefix="C-",
    collection_attr="conflicts",
    ref_fields=(
        ("steward", "Stakeholder"),
        ("members", "Requirement"),
        ("shared_assumption", "Assumption"),
        ("derived", "Requirement"),
        ("decided_by", "Stakeholder"),
    ),
    lifecycle=CONFLICT_LIFECYCLE,
)

ASSUMPTION_SCHEMA = NodeSchema(
    kind="Assumption",
    prefix="A-",
    collection_attr="assumptions",
    ref_fields=(
        ("owner", "Stakeholder"),
    ),
    # Assumption.status is the free-form-status surface; the admissible set is
    # ASSUMPTION_STATES (a frozenset, not a Lifecycle). Recorded on the schema
    # as a non-Lifecycle status_set via a separate attribute below to keep
    # lifecycle semantically a Lifecycle-or-None.
    lifecycle=None,
)

OPERATOR_SCHEMA = NodeSchema(
    kind="Operator",
    prefix="OP-",
    collection_attr="operators",
    ref_fields=(
        ("stakeholder", "Stakeholder"),
        ("parent", "Operator"),
    ),
    lifecycle=OPERATOR_LIFECYCLE,
)

PROCESS_SCHEMA = NodeSchema(
    kind="Process",
    prefix="PR-",
    collection_attr="processes",
    ref_fields=(),
    lifecycle=PROCESS_LIFECYCLE,
)

GOAL_SCHEMA = NodeSchema(
    kind="Goal",
    prefix="GOAL-",
    collection_attr="goals",
    # Goal.owner -> Stakeholder, Goal.kind -> TARGET_KINDS. Owner is the
    # dangling-relevant ref.
    ref_fields=(
        ("owner", "Stakeholder"),
    ),
    lifecycle=GOAL_LIFECYCLE,
)

ENTITY_INSTANCE_SCHEMA = NodeSchema(
    kind="EntityInstance",
    prefix="ENT-",
    collection_attr="entities",
    # EntityInstance.entity_type -> EntityType slug (same-graph reference).
    ref_fields=(
        ("entity_type", "EntityType"),
    ),
    lifecycle=None,
)

ENTITY_TYPE_SCHEMA = NodeSchema(
    kind="EntityType",
    # EntityType has NO enforced prefix family: it is addressed by slug, and
    # EntityInstance carries the 'ENT-<slug>-' family. check_typed_anchors_*
    # does NOT cover EntityType ids.
    prefix="",
    collection_attr="entity_types",
    ref_fields=(),
    lifecycle=None,
)

AXIS_SCHEMA = NodeSchema(
    kind="Axis",
    # Axis is addressed by slug with no enforced prefix family; the M28
    # taxonomy (GOAL-/GAP-/DLG-/AX-) is still OPEN per R-anchor-taxonomy.
    prefix="",
    collection_attr="axes",
    ref_fields=(),
    lifecycle=None,
)

STAKEHOLDER_SCHEMA = NodeSchema(
    kind="Stakeholder",
    # Stakeholder ids are bare slugs (e.g. 'finance', 'platform'); no prefix
    # family is enforced. This is intentional — stakeholders predate the
    # typed-anchor discipline and remain slug-addressed.
    prefix="",
    collection_attr="stakeholders",
    ref_fields=(),
    lifecycle=None,
)

VARIANT_SCHEMA = NodeSchema(
    kind="Variant",
    prefix="V-",
    # Variant is a PAYLOAD on Conflict.variants, not a top-level collection.
    # collection_attr is empty on purpose; check_typed_anchors_variant walks
    # g.conflicts -> c.variants.
    collection_attr="",
    ref_fields=(),
    lifecycle=None,
)


#: The ordered registry. Every typed-anchor node kind the framework knows.
NODE_SCHEMAS: tuple[NodeSchema, ...] = (
    REQUIREMENT_SCHEMA,
    CONFLICT_SCHEMA,
    ASSUMPTION_SCHEMA,
    OPERATOR_SCHEMA,
    PROCESS_SCHEMA,
    GOAL_SCHEMA,
    ENTITY_INSTANCE_SCHEMA,
    ENTITY_TYPE_SCHEMA,
    AXIS_SCHEMA,
    STAKEHOLDER_SCHEMA,
    VARIANT_SCHEMA,
)

#: kind -> schema (lookup convenience).
NODE_SCHEMAS_BY_KIND: dict[str, NodeSchema] = {s.kind: s for s in NODE_SCHEMAS}

#: prefix -> schema, for the kinds that HAVE an enforced prefix family.
#: A prefix is unique to one kind (no two kinds share 'R-'); collisions would
#: be a framework ontology bug. Empty-prefix kinds are NOT in this map.
NODE_SCHEMAS_BY_PREFIX: dict[str, NodeSchema] = {
    s.prefix: s for s in NODE_SCHEMAS if s.prefix
}

#: The kinds that the graph exposes as top-level collections (excludes
#: Variant, which is a payload on Conflict).
COLLECTION_KINDS: tuple[NodeSchema, ...] = tuple(
    s for s in NODE_SCHEMAS if s.collection_attr
)

#: Status-set surface for kinds whose status field is a frozenset, not a
#: Lifecycle (currently only Assumption). Kept separate so `lifecycle`
#: stays semantically Lifecycle-or-None.
STATUS_SETS_BY_KIND: dict[str, frozenset[str]] = {
    "Assumption": ASSUMPTION_STATES,
}


def schema_for_kind(kind: str) -> NodeSchema | None:
    """Canon: §Invariants — return the NodeSchema for ``kind``, or None if unknown.

    Pure lookup over the frozen registry; no allocation, no side effects.
    """
    return NODE_SCHEMAS_BY_KIND.get(kind)


def schema_for_prefix(prefix: str) -> NodeSchema | None:
    """Canon: §Invariants — return the NodeSchema whose enforced prefix family is ``prefix``.

    Returns None for empty prefixes (Axis/Stakeholder/EntityType) and for
    unknown prefixes. Pure lookup.
    """
    return NODE_SCHEMAS_BY_PREFIX.get(prefix)


def kinds_with_prefix() -> tuple[str, ...]:
    """Canon: §Invariants — return the kinds that have an enforced id-prefix family, in order.

    These are exactly the kinds covered by some check_typed_anchors_*.
    Pure derivation over the registry.
    """
    return tuple(s.kind for s in NODE_SCHEMAS if s.prefix)


def collection_attrs() -> tuple[str, ...]:
    """Canon: §Invariants — return the TensionGraph collection attribute names, in order.

    Pure derivation; useful for validators that walk the graph uniformly.
    """
    return tuple(s.collection_attr for s in COLLECTION_KINDS)


__all__ = [
    "NodeSchema",
    "NODE_SCHEMAS",
    "NODE_SCHEMAS_BY_KIND",
    "NODE_SCHEMAS_BY_PREFIX",
    "COLLECTION_KINDS",
    "STATUS_SETS_BY_KIND",
    "REQUIREMENT_SCHEMA",
    "CONFLICT_SCHEMA",
    "ASSUMPTION_SCHEMA",
    "OPERATOR_SCHEMA",
    "PROCESS_SCHEMA",
    "GOAL_SCHEMA",
    "ENTITY_INSTANCE_SCHEMA",
    "ENTITY_TYPE_SCHEMA",
    "AXIS_SCHEMA",
    "STAKEHOLDER_SCHEMA",
    "VARIANT_SCHEMA",
    "schema_for_kind",
    "schema_for_prefix",
    "kinds_with_prefix",
    "collection_attrs",
]
