# Delegation node — design candidate (PRESENTED, not landed)

> Reader: framework-author (steward). Status: DESIGN CANDIDATE. This document
> is a PRESENT-step artifact for `R-domain-delegation-as-node` and
> `R-domain-delegation-persists` (both DRAFT). It proposes a `Delegation`
> substrate node type. It does NOT mutate the graph; a matching pending
> proposal lives at `spec/.runtime/proposals/pending/delegation-node-candidate.json`
> carrying `decided_by: "STEWARD_SIGNATURE_REQUIRED"` and must not be applied
> until the steward signs (R-ai-presents-not-decides, R-trust-anchor-mechanism).

## 1. What the two DRAFT requirements ask for

- `R-domain-delegation-as-node` — "A domain-delegation shall be recorded as a
  `Delegation` substrate node with fields parent_op, child_op, scope, border,
  returns_contract, crystal_path."
- `R-domain-delegation-persists` — "A domain-delegation shall persist as a
  directory + a substrate node (`Delegation`)."

Both are held DRAFT behind a BUILD-TRIGGER: the `Delegation` type does not yet
exist. This document is the design that would let a steward decide whether to
build it — it does not itself trigger.

## 2. Proposed type

```python
@dataclass(frozen=True)
class Delegation:
    """Canon: §Delegation — a durable, stewardable parent->child hand-off.

    Distinct from a task-spawn (spec/.runtime/spawn-log.jsonl, ephemeral,
    R-task-spawn-log-runtime): a Delegation is COMMITTED graph substrate,
    carrying the border contract under which a sub-operator acts on a slice of
    the one shared TensionGraph.
    """
    id: str               # typed anchor, prefix "DG-" (see §5)
    parent_op: str        # Operator.id granting the delegation
    child_op: str         # Operator.id receiving it
    scope: tuple[str, ...]  # id-prefix tuple (same shape as scope_projection prefixes)
    border: str           # the shared-object boundary: what crosses, what does not
    returns_contract: str # what the sub-operator returns (CONCLUSIONS-only per R-delegation-conclusions-only)
    crystal_path: str     # path to the child operator's CLAUDE.md crystal
```

Stored as a new `TensionGraph.delegations: tuple[Delegation, ...]` field
(default empty tuple — opt-in, an ordinary domain pays nothing, mirroring
`processes`/`goals`/`entity_types`).

## 3. Invariants (the `check_*` layer this type would add)

1. `check_no_dangling_delegation_ops` — every `parent_op` and `child_op` MUST
   resolve to an `Operator.id` in the graph (mirrors `check_no_dangling_ids`
   for operator refs). A dangling endpoint is a caught Violation, never a
   silent pass.
2. `check_delegation_scope_prefixes_valid` — every entry of `scope` MUST be a
   non-empty typed-anchor prefix (e.g. `"R-"`, `"R-entity-"`), the exact shape
   `scope_projection.project_scope` consumes; an empty or malformed prefix is a
   Violation.
3. `check_delegation_parent_not_child` — `parent_op != child_op` (a delegation
   is a hand-off between two distinct acting facets, mirroring
   `check_operator_steward_not_self`'s distinctness discipline).
4. `check_delegation_crystal_path_declared` — `crystal_path` MUST be non-empty
   (the child boots from substrate, R-boot-from-substrate's replacement
   R-boot-reload-three-facts; an unanchored child would invent state).
5. `check_typed_anchors_delegation` — `id` MUST start with the `DG-` prefix
   (R-anchor-everything, R-typed-anchors discipline).

## 4. Relationship to the two existing runtime traces

The `Delegation` node is the COMMITTED apex of a three-layer record already
half-built in the repo; it does not replace either lower layer:

| layer | file | committed? | records | anchor |
|-------|------|-----------|---------|--------|
| campaign signature | `domains/<name>/delegations.jsonl` | yes (committed ledger) | steward's verbatim trust-anchor grant `{id, steward, verbatim, date, scope}` | `DEL-<n>` |
| **domain-delegation** | **graph substrate (`Delegation` node)** | **yes (graph.py)** | **parent/child operator hand-off + border contract** | **`DG-<n>` (proposed)** |
| task-spawn | `spec/.runtime/spawn-log.jsonl` | no (`.runtime/`, gitignored) | per-invocation ephemeral spawn trace | (log line, no anchor) |

- Downward link: a `Delegation.id` (`DG-n`) MAY be cited by the runtime
  spawn-log entries it authorizes, the same way a `Conflict.lifecycle` marker
  cites a `DEL-n` campaign record (R-trust-anchor-delegation-explicit-only). A
  spawn without an authorizing `Delegation` is task-ephemera; a spawn that
  claims a `DG-n` must resolve to a real node (a future
  `test_delegation_marker_honesty`-style check).
- Upward link: a `Delegation` that acts under a steward-signed campaign SHOULD
  reference the `DEL-n` grant in `delegations.jsonl` that authorized it — the
  signature layer stays the human trust anchor; the `Delegation` node is the
  operational hand-off it permits.

## 5. Anchor choice: `DG-`, not `DEL-`

`DEL-` is ALREADY the anchor of the committed campaign-signature ledger
(`domains/hotam-spec-self/delegations.jsonl`: `DEL-1`, auto-incremented by
`tools/record_delegation.py`). Reusing `DEL-` would collapse two distinct
kinds — a human trust-anchor SIGNATURE and an operator-to-operator operational
HAND-OFF — into one anchor namespace, defeating the "prefix names the kind"
discipline (R-anchor-everything). Between the two free candidates:

- **`DG-` (chosen)** — "delegation-graph-node"; short, two letters like the
  established `R-`/`C-`/`A-`/`OP-` anchors, and visually distinct from `DEL-`
  at a glance so the two layers never blur in a marker string.
- `DLG-` (rejected) — also free and unambiguous, but three letters and one
  keystroke from `DEL-`; the near-collision is exactly the readability hazard
  `DG-` avoids.

Recommendation: **`DG-`**.

## 6. Why a node (not just the two logs)

The campaign ledger records WHO signed WHAT verbatim; the spawn-log records
THAT a spawn happened. Neither records the operational BORDER — which slice of
the graph the child may touch, what crosses the boundary, what it must return.
That border is important-yet-invisible today: it lives only in a task prompt's
prose. The generative law (CLAUDE.md) says important-yet-invisible → typed
anchored node under a named steward. `Delegation` is that node; its `border`
and `returns_contract` fields are the currently-invisible contract made
first-class and checkable.

## 7. Future: cross-domain overlap (honest boundary — NOT designed here)

`scope_projection.scope_overlap(a, b)` computes the shared slice between two
`ScopeView`s. Confirmed by reading `spec/src/hotam_spec/scope_projection.py`:
both views are produced by `project_scope(g, prefixes)` over **one** shared
`TensionGraph g`; `scope_overlap` then intersects id-sets that were both drawn
from that single graph. There is no code path that projects across two distinct
domain graphs, and none is proposed here.

Consequently a `Delegation` whose `parent_op` and `child_op` belong to
**different domains** is OUT OF SCOPE: overlap/presenter resolution
(`overlap_node_ids`, `presenter_for_node`) is only meaningful within one graph,
and multi-domain federation is frozen (R-speculative-aspects-frozen). Until
federation is unfrozen by an explicit steward decision, a `Delegation` MUST
stay intra-domain (both operators resolvable in the same `graph.py`); a
cross-domain hand-off is a separate, unbuilt mechanism, not an incremental
tweak to this node. Documenting the wall here keeps the limit visible rather
than inviting a caller to assume cross-domain overlap already works.

---

**Resolved 2026-07-05:** file-based delegations chosen (R-delegation-is-a-file). The steward decided that delegations are versioned files under `delegations/DG-<n>.md`, created and closed via `tools/delegate.py`, with git history as the audit trail. The graph-node design described above was not built; `R-domain-delegation-as-node` and `R-domain-delegation-persists` are now REJECTED.
