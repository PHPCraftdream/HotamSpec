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

**Required:** `id`, `claim`, `owner` (a Stakeholder id), `status`, `why`
**Optional:** `assumptions` (list of Assumption ids, default `[]`),
`relations` (list of `[kind, target]` pairs, default `[]`), `enforcement`
(`PROSE` | `STRUCTURAL` | `ENFORCED`, default `"PROSE"`), `enforced_by` (list
of strings, default `[]`), `m_tag` (default `""`), `enforceability`
(default `"ENFORCEABLE"`), `summary` (default `""`), `created_at` (ISO
`YYYY-MM-DD`, defaults to today when omitted), `settled_at` (ISO date, filled
with today only when `status` is `SETTLED` and this is empty)

```json
{
  "kind": "Requirement",
  "id": "R-ship-fast",
  "claim": "Ship within one week.",
  "owner": "alice",
  "status": "SETTLED",
  "why": "customers expect weekly releases",
  "enforcement": "PROSE"
}
```

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
`HOLDS` | `UNCERTAIN` | `DEAD` | `IMPLEMENTS`), `owner` (a Stakeholder id)
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
`IMPLEMENTS`), `reason` (non-empty)
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

**Required:** `slug` (kebab-case), `description`, `why`, `states` (non-empty
list of `[name, kind]` or `[name, kind, why]` triples; `kind` is one of the
framework's `STATE_KINDS` and exactly one state must have `kind == "initial"`),
`transitions` (list of `[src, dst, event]` triples, optionally with
guard/why — see `proposal.py` for the full serialized shape)
**Optional:** `cyclic` (bool, default `false`), `fields` (list of
`[name, kind, required, ref_target]` quadruples, default `[]`; `kind` is one
of the framework's `ENTITY_FIELD_KINDS`)

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
