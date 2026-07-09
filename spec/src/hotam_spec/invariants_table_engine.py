"""Canon: §Invariants — declarative table engine for TABLE_DRIVEN check_*.

A *table-driven* check is one whose logic fits the form:

    for each item X in collection g.<collection_attr>:
        if <condition>(X.<field>) is NOT satisfied:
            emit Violation(check_name, X.id, <message>)

The body is mechanically identical across a family of checks; only the
(collection, field, condition, message-template, target-extractor) varies.
This module lets such a family be expressed as a data table, with one
generic runner producing the same `Violation` list the hand-written loop
would have produced.

SCOPE (this wave): only the ``id_prefix`` condition is exercised — it backs
the ``check_typed_anchors_*`` family (Requirement/Assumption/Conflict/
Operator/Process/Goal/EntityInstance ids must start with a typed prefix).
The condition-kind enum and the ``FieldCondition`` shape are designed to
admit the other lens-1 condition families (``non_empty``, ``matches_regex``,
``in_set``, ``references_existing_id``) in later waves WITHOUT changing the
runner contract or the pilot checks already migrated to this engine.

WHY a parallel module, not an in-place rewrite: invariants.py is hash-pinned
under R-enforcement-perimeter-visible. Keeping the engine here lets the
migrated check_* bodies become one-line delegations (a visible, reviewable
diff) while the engine itself lives outside the pin in a pure stdlib-only
module. stdlib/hotam_spec-only — same purity contract as invariants.py
(R-core-imports-stdlib-or-hotam-spec-only).
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Callable, Protocol

from hotam_spec.graph import TensionGraph
from hotam_spec.invariants import Violation


# ---------------------------------------------------------------------------
# Condition kinds — extensible vocabulary (lens-1 family)
# ---------------------------------------------------------------------------

#: The set of condition kinds the engine understands. ``id_prefix`` is the
#: only kind EXERCISED in this wave; the others are reserved names so later
#: waves can add conditions without touching the runner contract or the
#: already-migrated pilot checks.
_CONDITION_NON_EMPTY = "non_empty"
_CONDITION_ID_PREFIX = "id_prefix"
_CONDITION_MATCHES_REGEX = "matches_regex"
_CONDITION_IN_SET = "in_set"
_CONDITION_REFERENCES_EXISTING_ID = "references_existing_id"


class _ItemProtocol(Protocol):
    """Structural shape the runner needs from each collection item.

    The runner reads ``.id`` (the Violation target) and one named field. Any
    dataclass on TensionGraph collections satisfies this — it is purely a
    typing aid, not enforced at runtime.
    """

    id: str


@dataclass(frozen=True)
class FieldCondition:
    """Canon: §Invariants — one predicate over a single field of every item in a collection.

    ``kind`` selects the evaluation; ``args`` carries the kind-specific
    parameters. Kept as a closed dataclass (not a Callable) so the table is
    declarative and serialisable, matching the lens-1 description.
    """

    kind: str
    args: tuple[object, ...]

    @classmethod
    def id_prefix(cls, prefix: str) -> "FieldCondition":
        return cls(kind=_CONDITION_ID_PREFIX, args=(prefix,))

    def is_satisfied(self, value: str) -> bool:
        """Return True iff ``value`` satisfies this condition.

        The runner inverts this: a violation is emitted when the condition
        is NOT satisfied, mirroring the ``if not <cond>: Violation(...)``
        shape of the hand-written checks.
        """
        if self.kind == _CONDITION_ID_PREFIX:
            prefix = self.args[0]
            assert isinstance(prefix, str)
            return value.startswith(prefix)
        if self.kind == _CONDITION_NON_EMPTY:
            return bool(value.strip())
        raise ValueError(
            f"FieldCondition kind '{self.kind}' is reserved but not yet "
            f"implemented in this wave of the table engine. Only "
            f"'{_CONDITION_ID_PREFIX}' (and the non-exercised "
            f"'{_CONDITION_NON_EMPTY}') are wired."
        )


@dataclass(frozen=True)
class TableCheck:
    """Canon: §Invariants — declarative description of one table-driven check_*.

    Fields:
      check_name      — the invariant name stamped into every Violation
                        (MUST equal the enclosing check_* function name, so
                        the existing enforcer-resolution + fires-tests keep
                        resolving unchanged).
      collection_attr — name of the ``g.<attr>`` list/tuple walked.
      field           — name of the per-item attribute the condition tests.
      condition       — the FieldCondition the field MUST satisfy.
      message_tpl     — format string with ``{id}`` and ``{prefix}`` slots
                        (kind-specific extras may be added later). Produces
                        the exact message the hand-written check emitted.
      target_field    — name of the per-item attribute used as the Violation
                        target (defaults to ``"id"``; separated so nested
                        collections can compose a composite target later).
    """

    check_name: str
    collection_attr: str
    field: str
    condition: FieldCondition
    message_tpl: str
    target_field: str = "id"


def run_table_check(g: TensionGraph, spec: TableCheck) -> list[Violation]:
    """Canon: §Invariants — execute one TableCheck against the graph, returning Violations.

    Walks ``getattr(g, spec.collection_attr)``; for each item whose
    ``spec.field`` does NOT satisfy ``spec.condition``, appends a Violation
    with ``spec.check_name`` and the rendered message. Order is the
    collection's natural iteration order — identical to a hand-written
    ``for x in g.<attr>:`` loop, which is what the migrated checks used.
    """
    out: list[Violation] = []
    collection = getattr(g, spec.collection_attr)
    for item in collection:
        field_value = getattr(item, spec.field)
        if not spec.condition.is_satisfied(field_value):
            target = getattr(item, spec.target_field)
            message = spec.message_tpl.format(
                id=target, prefix=spec.condition.args[0]
            )
            out.append(Violation(spec.check_name, target, message))
    return out
