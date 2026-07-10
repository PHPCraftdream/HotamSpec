# Proposal JSON reference

Every change to a Hotam-Spec graph goes through `hotam-apply-proposal
<file.json>` (never a hand-edit — see `R-no-hand-edit-graph`). A proposal file
is a single JSON object with a `"kind"` field selecting one of the shapes
below. This is the field-level reference; for the guided end-to-end walk see
[QUICKSTART-CONSUMER.md](QUICKSTART-CONSUMER.md).

Source of truth: `spec/src/hotam_spec/proposal.py` (dataclasses) and
`spec/tools/apply_proposal.py` (`_validate_*` functions). If this document and
the code disagree, the code wins — please file an issue.

Usage:

```bash
hotam-apply-proposal proposal.json
hotam-apply-proposal --dry-run proposal.json
hotam-apply-proposal --batch proposals_array.json   # array of proposal objects
```

## The consumer graph.py AST contract

`apply_proposal.py` never re-parses your whole graph into an AST and rewrites
it wholesale — it locates a small number of exact source shapes with `ast`
and splices new source text in via targeted line/column edits, so the rest of
your file's formatting is left untouched. That speed and formatting-fidelity
trade-off means the writer requires your `graph.py` to hold a handful of
shapes EXACTLY — reformatting them (even functionally-equivalently, e.g. with
`ruff format` / `black` collapsing a multi-line tuple onto one line) can make
the writer unable to find where to append, in which case `apply_proposal.py`
fails fast with a `RuntimeError` naming exactly what it looked for and what it
found instead, rather than silently doing nothing.

**Required shape** (this is exactly what `tools/create_domain.py` scaffolds
for every new domain — see `domains/<name>/graph.py` after `hotam-create-domain`
for a live example):

```python
def build_graph() -> TensionGraph:
    stakeholders = (
        Stakeholder(id="...", name="...", domain="..."),
    )
    axes = (
        Axis(slug="...", description="..."),
    )
    requirements = (
        Requirement(id="R-...", claim="...", owner="...", status="...", why="..."),
    )
    conflicts = (
        Conflict(axis="...", context="...", members=(...), steward="..."),
    )
    assumptions = (
        Assumption(id="A-...", statement="...", status=HOLDS, owner="...", created_at="..."),
    )
    return TensionGraph(
        axes=axes,
        stakeholders=stakeholders,
        requirements=requirements,
        conflicts=conflicts,
        assumptions=assumptions,
    )
```

Load-bearing rules the locator depends on (breaking any one of these is what
triggers the `RuntimeError` below):

1. **A `def build_graph():` function must exist.** This is the ONLY place the
   writer looks for the tuples below — a graph assembled some other way (a
   module-level constant, a class method, a factory with a different name) is
   invisible to `apply_proposal.py`.
2. **Each roster is a top-level, bare-name assignment inside `build_graph()`**
   — `requirements = (...)`, `conflicts = (...)`, `axes = (...)`,
   `stakeholders = (...)`, `assumptions = (...)`. "Top-level" means a direct
   statement in the function body, not nested inside an `if`/`for`/`with`; "bare
   name" means the assignment target is a single identifier (`requirements`),
   not an attribute (`self.requirements`), not a tuple-unpack
   (`requirements, x = ...`), and not inlined directly as a `TensionGraph(...)`
   kwarg (`TensionGraph(requirements=(...))` with no separate variable). The
   NAME matters too — renaming `requirements` to `reqs` breaks every
   `Requirement`-kind proposal.
3. **The right-hand side must be a literal parenthesized tuple** — `(...)`,
   not `list(...)`, not a generator expression, not a function call that
   returns a tuple. An empty roster is still `()`  (or `(\n)`, both parse the
   same), never omitted. Note: a non-tuple RHS (e.g. `requirements = list(...)`)
   may not cause an immediate error at write time -- the locator accepts any
   assignment RHS without checking its type. The breakage typically surfaces
   later, during the regen/verify step, as corrupted source output.
4. **`return TensionGraph(...)` must appear as a direct call expression** in
   `build_graph()`'s `return` statement — not built up across several
   statements (`g = TensionGraph(...); return g` is NOT recognized; this
   matters specifically for `EntityType` proposals, which locate the
   `TensionGraph(...)` call itself to insert or extend `entity_types=`).
5. **Requirement's `enforcement=` values (`PROSE`/`STRUCTURAL`/`ENFORCED`) are
   written as bare name references, not string literals** (mirroring how
   `hotam-spec-self`'s own graph.py renders them) — so your `graph.py` MUST
   `from hotam_spec.requirement import ENFORCED, PROSE, STRUCTURAL` even
   before you have hand-authored a single `Requirement` yourself, because a
   `ProposedRequirement` with no explicit `enforcement` defaults to `"PROSE"`
   and the FIRST mechanically-added requirement will already need the name in
   scope (a `NameError` at graph-load time otherwise — caught by
   `spec/tests/test_apply_proposal_batch_stress.py`, which stress-tests a
   30+-item mixed batch against a freshly `create_domain.py`-scaffolded
   graph). `tools/create_domain.py`'s scaffold template pre-declares this
   import for exactly this reason; keep it if you hand-edit the template.

**What happens when the contract breaks**: every locator that depends on
these shapes fails CLOSED — it raises `RuntimeError` (surfaced as
`apply_proposal.py` exit code 1, nothing written) rather than guessing or
silently no-op'ing. The message names which tuple/function it was looking
for, whether `build_graph()` exists at all, and — when `build_graph()` exists
but the target roster doesn't — which OTHER top-level tuple names it DID find
(so a rename is easy to spot). Example, after a hypothetical reformat that
renamed `requirements` to `reqs`:

```
ERROR: Cannot find `requirements = (...)` tuple in build_graph(). `build_graph()`
DOES contain these top-level tuple assignment(s): ['axes', 'assumptions', 'conflicts',
'reqs', 'stakeholders'] — was `requirements` renamed to one of these, or inlined as a
TensionGraph(...) kwarg instead of a separate variable? See docs/PROPOSAL-REFERENCE.md,
'The consumer graph.py AST contract', for the exact shape required (a bare top-level
`requirements = (...)` assignment, one Requirement/Conflict/Axis/Stakeholder/Assumption
call per element, never reformatted into an inline TensionGraph(...) kwarg or renamed).
```

**Practical guidance**: run your formatter/linter on everything EXCEPT the
five roster tuples inside `build_graph()` and the `return TensionGraph(...)`
call, or configure it to leave `graph.py` alone entirely and only run it on
your own domain code elsewhere. `tools/gen_spec.py`'s own regeneration never
touches `graph.py` (it is hand/proposal-authored, never generated), so there
is no round-trip formatting concern beyond your own tool choices.

## Enum reference

These value sets are reused across several proposal kinds below.

### Requirement `status`

| Value | Meaning |
|-------|---------|
| `DRAFT` | Proposed, not yet accepted into the canon. |
| `SETTLED` | Accepted and currently held. |
| `OPEN(<question>)` | Accepted-with-a-hole; the literal string `OPEN(` followed by a non-empty question and `)`. Surfaced by the harness until resolved. |

`REJECTED` is **not** a `status` value you set directly on a `ProposedRequirement`
— use the separate `Rejection` kind below, which moves an existing requirement
to `REJECTED` and preserves it for history (`R-rejected-preserved-not-deleted`).

### Requirement / EntityType `enforcement`

| Value | Meaning | When to choose it |
|-------|---------|--------------------|
| `PROSE` | Recorded only; no structural or automated check enforces it. The promise is held by human discipline alone. | Default for a fresh claim, or a claim that is inherently a human judgment call. |
| `STRUCTURAL` | Visible and addressable (surfaced by the harness, listed in docs) but no `check_*` invariant or test fires automatically on violation. | The claim is real and trackable, but writing an automated check is not yet worth the cost — an honest middle step, not a reflex. |
| `ENFORCED` | A `check_*` invariant or test fires automatically on violation; `enforced_by` MUST name the enforcer(s). | The claim has a real, running enforcer today. Never set this without also filling `enforced_by`. |

The intended direction of progress is `PROSE` → `STRUCTURAL` → `ENFORCED`.

### Requirement `enforceability` (default `"ENFORCEABLE"`)

| Value | Meaning |
|-------|---------|
| `ENFORCEABLE` | A `check_*` or test COULD exist for this claim (even if `enforcement` is still `PROSE`/`STRUCTURAL` today — that gap is real, trackable debt). |
| `INHERENTLY_PROSE` | The claim is a disposition or social/judgment discipline that cannot be mechanically checked even in principle (e.g. "be respectful in code review"). Staying `PROSE` forever is honest labeling, not debt. |

### Requirement `relations` — relation kinds

Each entry in `relations` is a `[kind, target]` pair, where `target` is the id
of another Requirement already in the graph.

| Kind | Meaning | Direction |
|------|---------|-----------|
| `refines` | A supportive, non-adversarial edge -- this requirement elaborates or narrows the target. (Also covers what used to be a separate `supports` kind, merged into `refines`.) | carrier → target |
| `depends_on` | This requirement's guarantee relies on the target holding. | carrier → target |
| `replaces` | Anti-relitigation edge -- this requirement (the carrier) REPLACES the target (normally a REJECTED requirement). Usually written automatically by a `Rejection` proposal's `replaced_by` field, rather than hand-authored. | carrier → target (carrier replaces target) |

### Assumption `status` / `AssumptionTransition new_status`

| Value | Meaning |
|-------|---------|
| `HOLDS` | The belief is currently trusted as true. |
| `UNCERTAIN` | Under question, not yet falsified -- a doubt has been raised but nothing is decided. |
| `DEAD` | Falsified / abandoned; kills the premise and any requirements resting on it are flagged as drifted. |
| `IMPLEMENTS` | A volitional status: an aspiration being worked toward, not a fact-claim. |

---

## Stakeholder

Adds a new accountable party. Usually the *first* thing you create — a
Conflict's steward must not own any of its members, so you need at least two
distinct stakeholders before you can hold a tension.

**Required:** `id`, `name`, `domain`
**Optional:** `why` (default `""`)

```json
{"kind": "Stakeholder", "id": "carol", "name": "Carol", "domain": "governance", "why": "neutral party for the first conflict"}
```

## Axis

Adds a new controlled-vocabulary tension dimension (e.g. "speed vs rigor").
Conflicts cluster around axes; prefer `hotam-create-axis` over hand-writing
this JSON, since the CLI runs a similarity check against existing axes first
(`R-axis-gatekeeper-policy`).

**Required:** `slug` (kebab-case, must not already exist), `description`
**Optional:** `why` (default `""`)

```json
{"kind": "Axis", "slug": "speed-vs-rigor", "description": "ship fast vs verify thoroughly", "why": ""}
```

## Requirement

Adds or updates a business claim.

**Required:** `id`, `claim`, `owner` (a Stakeholder id), `status` (`DRAFT` |
`SETTLED` | `OPEN(<question>)` — see [Enum reference](#enum-reference) above;
`REJECTED` is set only via the `Rejection` kind below, never directly)
**Optional:** `why` (default `""`), `assumptions` (list of Assumption ids, default `[]`),
`relations` (list of `[kind, target]` pairs — kind is `refines` | `depends_on` |
`replaces`, see [Enum reference](#enum-reference); default `[]`), `enforcement`
(`PROSE` | `STRUCTURAL` | `ENFORCED`, default `"PROSE"` — see
[Enum reference](#enum-reference) for what each means), `enforced_by` (list
of strings, default `[]`), `m_tag` (default `""`), `enforceability`
(`ENFORCEABLE` | `INHERENTLY_PROSE`, default `"ENFORCEABLE"` — see
[Enum reference](#enum-reference)), `summary` (default `""`), `created_at` (ISO
`YYYY-MM-DD`, defaults to today when omitted), `settled_at` (ISO date, filled
with today only when `status` is `SETTLED` and this is empty), `last_reviewed_at`
(ISO date the claim was last re-confronted and held, default `""`),
`review_after` (ISO date after which re-confrontation is due, default `""`),
`evidence` (list of free-form evidence strings backing the claim, default `[]`),
`source_refs` (list of pointers to where the claim originated — doc paths, URLs,
review ids, commit hashes — default `[]`)

```json
{
  "kind": "Requirement",
  "id": "R-ship-fast",
  "claim": "Ship within one week.",
  "owner": "alice",
  "status": "SETTLED",
  "why": "customers expect weekly releases",
  "enforcement": "PROSE",
  "last_reviewed_at": "2026-07-10",
  "review_after": "2026-12-01",
  "evidence": ["p99 latency held under 200ms for 3 releases"],
  "source_refs": ["docs/roadmap.md", "review-2026-07"]
}
```

**Per-node change history (`history`) — derived, never supplied.** Every time
`apply_proposal.py` UPDATES an already-existing Requirement (not at first
creation), it diffs the changed fields and appends one `HistoryEntry`
(`at` · `summary` · optional `decided_by`) to the node's `history` tuple — the
change trail lives IN the committed graph, next to the claim (not only in git
blame or gitignored runtime JSON). `history` is a DERIVED field: it is **not** a
proposal key, and supplying `"history"` in a Requirement proposal is rejected.
Its structure (dated, non-empty entries, monotonic stamps) is enforced by
`check_requirement_history_wellformed`; its CONTENT is never machine-judged
(that would repeat the `R-boot-cite-measured` form-metric theatre).

## Conflict (creation)

Materializes a new Conflict node between >= 2 existing Requirements, always
starting at lifecycle `DETECTED` (creation is presentation, not decision —
`R-ai-presents-not-decides`). The node id is **never** caller-supplied — the
writer computes it as `conflict_identity(axis, context)`
(`R-stable-conflict-identity`).

**Required:** `axis` (must already exist in the graph's axes), `context`,
`members` (list of >= 2 distinct Requirement ids), `steward` (a Stakeholder id
that owns none of the members)
**Optional:** `shared_assumption` (an Assumption id, default `""`), `note`
(presentation-only, never written to the graph, default `""`),
`initial_lifecycle` (default `"DETECTED"`; only a DECIDED constituting-atoms
edge case may start elsewhere — see the docstring in `proposal.py`),
`decided_by` (required only if `initial_lifecycle` starts with `DECIDED`)

```json
{
  "kind": "Conflict",
  "axis": "speed-vs-rigor",
  "context": "first release cadence",
  "members": ["R-ship-fast", "R-verify-all"],
  "steward": "carol",
  "shared_assumption": "",
  "note": "surfaced while scaffolding the demo domain"
}
```

## ConflictTransition

Moves an existing Conflict's lifecycle (`DETECTED` -> `ACKNOWLEDGED` ->
`DECIDED(...)`, or into `HELD(...)` / `REVISIT_WHEN(...)`). A `DECIDED` or
`HELD` transition requires a named human decider
(`R-decided-needs-human-signoff`).

**Required:** `conflict_id`, `new_lifecycle` (a string; if it starts with
`DECIDED` or `HELD`, `decided_by` becomes required)
**Optional:** `decided_by` (default `""`), `revisit_marker` (default `""`),
`shared_assumption` (re-points the shared-assumption edge; default `""` =
leave untouched), `derived` (list of R-ids spawned by this decision, default
`[]`), `variants` (list of `{id, behavior, implies, costs}` objects; required
with >= 2 entries when `new_lifecycle` starts with `HELD`, and must be
repeated unchanged on a later `HELD` -> `DECIDED` move to preserve them),
`date` (ISO date, defaults to today), `verbatim` (the steward's own words,
default `""`), `instrument` (`"personal"` default, or `"DEL-<n>"` for a filed
delegation), `chosen_variant` (a `V-id` from `variants`, when resolving
`HELD` -> `DECIDED`)

```json
{
  "kind": "ConflictTransition",
  "conflict_id": "C-8600b1b8",
  "new_lifecycle": "DECIDED(ship weekly; verification gate runs async after release)",
  "decided_by": "carol",
  "revisit_marker": "REVISIT if a shipped defect reaches a customer",
  "derived": []
}
```

## Rejection

Marks an existing Requirement `REJECTED` (never deleted —
`R-rejected-preserved-not-deleted`).

**Required:** `requirement_id`, `reason` (the "REJECTED — REPLACES ..." prose)
**Optional:** `replaced_by` (a string or list of Requirement ids that
supersede this one; default `[]` = no successor edge)

```json
{"kind": "Rejection", "requirement_id": "R-old-approach", "reason": "REJECTED — REPLACES R-new-approach; superseded by the async-verification decision", "replaced_by": ["R-new-approach"]}
```

## Assumption

Adds a new falsifiable belief that Requirements or Conflicts can rest on.

**Required:** `id` (must start with `A-`), `statement`, `status` (one of
`HOLDS` | `UNCERTAIN` | `DEAD` | `IMPLEMENTS` — see [Enum reference](#enum-reference)
above for what each means), `owner` (a Stakeholder id)
**Optional:** `why` (default `""`), `created_at` (ISO date, defaults to today)

```json
{"kind": "Assumption", "id": "A-weekly-cadence-tolerated", "statement": "Customers tolerate a weekly release cadence.", "status": "HOLDS", "owner": "alice", "why": "stated in the last customer survey"}
```

## AssumptionTransition

Changes an existing Assumption's status (the kill/re-affirm path). Signoff is
asymmetric: moving to `UNCERTAIN` only *raises* a doubt signal and needs no
signoff; moving to `HOLDS`, `DEAD`, or `IMPLEMENTS` all *reduce* live signal
or re-type the claim, and require `decided_by`.

**Required:** `assumption_id`, `new_status` (`HOLDS` | `UNCERTAIN` | `DEAD` |
`IMPLEMENTS` — see [Enum reference](#enum-reference) above), `reason` (non-empty)
**Optional:** `decided_by` (required when `new_status` is `HOLDS`, `DEAD`, or
`IMPLEMENTS`; optional for `UNCERTAIN`), `date` (ISO date, defaults to
today), `verbatim` (default `""`), `instrument` (default `"personal"`)

```json
{
  "kind": "AssumptionTransition",
  "assumption_id": "A-weekly-cadence-tolerated",
  "new_status": "DEAD",
  "reason": "the latest survey shows customers now expect daily releases",
  "decided_by": "alice"
}
```

## ConflictMemberUpdate

Adds or removes members on an existing Conflict without touching its
lifecycle. The resulting member count must stay >= 2
(`R-conflict-min-two-members`).

**Required:** `conflict_id`, and at least one of `add_members` /
`remove_members` non-empty (both empty is a no-op and rejected)
**Optional:** `add_members` (list of Requirement ids, default `[]`),
`remove_members` (list of Requirement ids, default `[]`), `decided_by`
(optional provenance, default `""` = no signoff recorded)

```json
{"kind": "ConflictMemberUpdate", "conflict_id": "C-8600b1b8", "add_members": ["R-canary-release"], "remove_members": [], "decided_by": "carol"}
```

## OperatorBudget

Replaces an existing Operator's context budget (limit + measure). Used to
move an operator off a stale or mismeasured budget.

**Required:** `operator_id` (must start with `OP-`), `new_limit` (int >= 0),
`new_measure` (one of `NODE_COUNT` | `CRYSTAL_CHARS`)
**Optional:** `why` (default `""`)

```json
{"kind": "OperatorBudget", "operator_id": "OP-director", "new_limit": 150000, "new_measure": "CRYSTAL_CHARS", "why": "NODE_COUNT was counting the free substrate, not working context"}
```

## EntityType

Adds a domain-declared business concept with its own lifecycle (states +
transitions) and optional typed fields. The most structurally involved kind —
prefer `hotam-create-entity-type` for interactive scaffolding when possible.

**Required:** `slug` (kebab-case), `description`, `states` (non-empty
list of `[name, kind]` or `[name, kind, why]` triples; `kind` is one of the
framework's `STATE_KINDS` and exactly one state must have `kind == "initial"`),
`transitions` (list of `[src, dst, event]` triples, optionally with
guard/why — see `proposal.py` for the full serialized shape)
**Optional:** `why` (default `""`), `cyclic` (bool, default `false`), `fields`
(list of `[name, kind, required, ref_target]` quadruples, default `[]`;
`kind` is one of the framework's `ENTITY_FIELD_KINDS`)

```json
{
  "kind": "EntityType",
  "slug": "release",
  "description": "A shippable unit of work moving from draft to live.",
  "why": "the domain needs to track releases through their own lifecycle",
  "states": [
    ["draft", "initial", "work not yet ready to ship"],
    ["shipped", "terminal", "released to customers"]
  ],
  "transitions": [
    ["draft", "shipped", "ship"]
  ],
  "cyclic": false,
  "fields": [
    ["owner", "ref", true, "Stakeholder"]
  ]
}
```
