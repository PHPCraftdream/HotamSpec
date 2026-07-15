# Proposal JSON reference

Every change to a Hotam-Spec graph goes through
`hotam apply-proposal <file.json> --domain <path> --today YYYY-MM-DD` (never a
hand-edit — see `R-no-hand-edit-graph`). A proposal file is a single JSON
object with a `"kind"` field selecting one of the shapes below, and every
other field name is **snake_case** and matches its Go struct's `json` tag
exactly — the decoder is strict (`json.Decoder.DisallowUnknownFields`), so an
unrecognized or mistyped key (including any leftover camelCase from an older
convention) is a hard parse error, never a silently-dropped field. This is
the field-level reference; for the guided end-to-end walk see
[QUICKSTART-CONSUMER.md](QUICKSTART-CONSUMER.md).

Source of truth: `internal/proposal/types.go` (the `Proposed*` structs and
their `json` tags) and `cmd/hotam/apply_proposal.go` (`parseProposal` /
`unmarshalProposal`, the kind-dispatch + strict-decode logic) and
`internal/proposal/*.go` (the `validate()`/`mutate()` methods that apply each
kind to the graph). If this document and the code disagree, the code wins —
please file an issue. Every JSON example below is checked against the actual
decoder by `cmd/hotam/proposal_reference_test.go`, which extracts every
` ```json ` fenced block from this file and round-trips it through
`parseProposal`.

Usage:

```bash
hotam apply-proposal proposal.json --domain domains/my-shop --today 2026-07-12
```

## How a proposal is persisted (graph.json, not source code)

Unlike the historical Python prototype (which spliced Python source inside a
hand-authored `graph.py` via `ast` line/column edits), the Go CLI's graph is
plain data: `domains/<name>/docs/gen/graph.json`. `hotam apply-proposal`
(`cmd/hotam/apply_proposal.go` → `internal/proposal.Apply`,
`internal/proposal/apply.go`) does the following, in order:

1. Decode the proposal JSON into the matching `Proposed*` Go struct (strict,
   snake_case, unknown fields rejected — see above).
2. Run that struct's own `validate()` (required fields, enum membership,
   cross-field rules such as "`decided_by` required when `new_lifecycle`
   starts with `DECIDED`").
3. Load `graph.json` into memory (`internal/loader.LoadGraph`).
4. Run the struct's `mutate(graph, today)` — this is the ONLY code path that
   ever changes graph state; there is no hand-edit path
   (`R-no-hand-edit-graph`).
5. Recompute `internal/invariants.AllViolations` before and after the
   mutation; if the mutation introduces any NEW violation that did not exist
   before, the whole apply fails closed and NOTHING is written.
6. Only if the violation set did not grow, write the mutated graph back to
   `graph.json` (`internal/loader.WriteGraph`).

There is no `--dry-run` flag today. There IS a `--batch <dir>` flag on both
`hotam apply-proposal` and `hotam land`: point it at a directory of `*.json`
proposal files and every one is applied atomically, in filename order
(all-or-nothing — if any proposal in the batch fails steps 1-5 above, the
whole batch is rejected and `graph.json` is left completely untouched, not
partially written). Without `--batch`, each `hotam apply-proposal` call
applies exactly one proposal file containing exactly one JSON object; either
way, `hotam apply-proposal` alone does not regenerate docs -- run `hotam
gen-spec --domain <path>` afterward, or use `hotam land` (single proposal or
`--batch <dir>`) to apply + regenerate + re-verify in one step.

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
Conflicts cluster around axes. There is no dedicated `hotam create-axis`
scaffolding command in this Go CLI yet (unlike the historical Python
prototype) — an Axis proposal is written and applied the same way as every
other kind below, via `hotam apply-proposal`.

**Required:** `slug` (kebab-case, must not already exist), `description`
**Optional:** `why` (default `""`)

```json
{"kind": "Axis", "slug": "speed-vs-rigor", "description": "ship fast vs verify thoroughly", "why": ""}
```

## Requirement

Adds a new business claim, or UPDATES an existing one (when `id` already
resolves to a node in the graph).

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
`YYYY-MM-DD`; on a NEW node, defaults to today when omitted — see the UPDATE
subsection below for how this field behaves on an existing node), `settled_at`
(ISO date, filled with today only when `status` is `SETTLED` and this is
empty), `last_reviewed_at` (ISO date the claim was last re-confronted and
held, default `""`), `review_after` (ISO date after which re-confrontation is
due, default `""`), `evidence` (list of free-form evidence strings backing the
claim, default `[]`), `source_refs` (list of pointers to where the claim
originated — doc paths, URLs, review ids, commit hashes — default `[]`),
`blocked_on` (names a Planned tool or absent package that blocks enforcement of
this claim — marks it feature-blocked debt; default `""`; on an UPDATE, the
sentinel `"<clear>"` clears an existing value once the blocking feature ships)

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

### UPDATE semantics: a real patch, not a full replace

When `id` already names an existing Requirement, `ProposedRequirement.mutate`
(`internal/proposal/mutate.go`) UPDATES it in place rather than adding a second node. `claim`/`owner`/`status` are
**required on every UPDATE too** (there's no partial identity — you always
restate what the node currently is/should be for those three), but every
OTHER field is **patched**: if you omit an optional field (or send it at its
bare dataclass default — `""`, `[]`, `"PROSE"`, `"ENFORCEABLE"`), the
**existing value on the node is left untouched**, not overwritten with the
default. Only a field whose value you actually set to something non-default
is written. This means a minimal UPDATE proposal —

```json
{"kind": "Requirement", "id": "R-ship-fast", "claim": "Ship within one week.", "owner": "alice", "status": "SETTLED", "why": "", "summary": "clarified after the retro"}
```

— changes ONLY `summary`; `assumptions`, `enforcement`, `enforced_by`,
`relations`, `enforceability`, `evidence`, `source_refs`, `last_reviewed_at`,
`review_after`, and `created_at` all keep whatever they already held on the
node. (Prior to the Этап X / #126 fix, the UPDATE path did NOT patch — any
field missing from the proposal JSON was silently reset to its bare default,
which made a minimal UPDATE proposal a quiet data-loss trap. This is fixed;
the behavior described here is current.)

One known limitation of this patch convention: because "field omitted" and
"field explicitly set to its own default" are indistinguishable in a plain
JSON/dataclass proposal, there is no way to EXPLICITLY reset an optional
field back to its bare default (e.g. clear `enforced_by` to `[]`) via a single
UPDATE proposal — that requires setting it to a distinguishable non-default
value first, or a direct hand-edit. In practice this is rarely a real
constraint (fields are extended far more often than they are cleared), but
it's the honest edge case of the coalescing rule above.

**`created_at` on UPDATE.** `created_at` is the node's birth date, not a
repeatable transition — it is normally set once, at creation, and left alone.
The writer CAN write `created_at` on an UPDATE (this was previously impossible
— the field was entirely absent from the UPDATE path), which exists for the
BACKFILL case: a legacy node created before the timestamp layer existed (or
before this proposal system covered it) can have its true creation date filled
in later via an UPDATE that supplies `created_at` explicitly. Per the usual
patch rule, omitting `created_at` on an UPDATE preserves whatever the node
already has (or leaves it absent, if it was never set) — it is never
overwritten with today's date on an UPDATE (unlike on a brand-new node, where
omitting it means "stamp today", since there is no existing value to
preserve). A `created_at` change is **not** narrated in the derived `history`
trail below — see that subsection for why.

**Per-node change history (`history`) — derived, never supplied.** Every time
`hotam apply-proposal` UPDATES an already-existing Requirement (not at first
creation), it diffs the changed fields (after the patch-coalescing above — a
field the UPDATE left untouched never appears as a phantom "change") and
appends one `HistoryEntry` (`at` · `summary` · optional `decided_by`) to the
node's `history` tuple — the change trail lives IN the committed graph, next
to the claim (not only in git blame or gitignored runtime JSON). `history` is
a DERIVED field: it is **not** a proposal key, and supplying `"history"` in a
Requirement proposal is rejected. Its structure (dated, non-empty entries,
monotonic stamps) is enforced by `check_requirement_history_wellformed`; its
CONTENT is never machine-judged (that would repeat the `R-boot-cite-measured`
form-metric theatre).

`created_at` is deliberately EXCLUDED from this diff, unlike every other
tracked field (including `settled_at`, which DOES narrate — it stamps a
repeatable status *transition* worth recording each time it recurs).
`created_at` is a one-time birth fact, not content; writing or backfilling it
is a bookkeeping correction, not a substantive edit to the requirement, so it
would misrepresent the change trail to narrate it there.

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
edge case may start elsewhere — see `internal/proposal/mutate.go`),
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

## ReviewMark

Stamps an EXISTING Requirement's freshness metadata (`last_reviewed_at`,
`review_after`, `evidence`) without touching its content fields
(`claim`/`why`/`status`/`enforcement`/... are all left untouched — see
`ProposedReviewMark` in `internal/proposal/types.go`). It exists as its own
narrow kind rather than going through a `Requirement` UPDATE so a review act
(the steward re-affirmed a claim is still true) stays distinguishable from a
content edit.

**Required:** `requirement_id`, `evidence` (list of strings; at least one
non-whitespace entry — R-review-mark-carries-evidence). Evidence must be a
SUBSTANTIVE, independently re-verifiable attestation (e.g. a test name plus
the command that reproduces it, a doc path, a review id) — a bare "reviewed
today" string with no verifiable referent is the administrative-backfill
anti-pattern this field exists to prevent.
**Optional:** `reviewed_at` (ISO date, defaults to today), `review_after`
(ISO date after which re-confirmation is due; left untouched if omitted)

```json
{
  "kind": "ReviewMark",
  "requirement_id": "R-ship-fast",
  "reviewed_at": "2026-07-13",
  "review_after": "2027-01-13",
  "evidence": ["re-ran `go test -run TestShipFast ./...` on 2026-07-13, still green"]
}
```

Note on the existing corpus: this mandatory-evidence rule is forward-looking
only (it gates the next ReviewMark applied; it does not retroactively touch
SETTLED requirements that already carry empty `evidence`). Whether the
corpus's existing empty-evidence requirements should be left to accumulate
real evidence naturally as each one comes up for its own `review_after` date
(the patient reading), or whether some other forward-looking policy is
warranted, is a steward call, not decided here.

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
transitions) and optional typed fields. The most structurally involved kind.
Unlike every other kind above, `states`/`transitions`/`fields` are lists of
JSON **objects** (not compact array-triples — that was the Python
prototype's wire format; the Go decoder has no custom `UnmarshalJSON` to
accept it, so an array-of-arrays value here is a parse error).

**Required:** `slug` (kebab-case), `description`, `states` (non-empty list of
`{"name", "kind", "why"}` objects; `kind` is one of `initial` | `normal` |
`terminal` | `quiescent`, and exactly one state must have `"kind": "initial"`),
`transitions` (list of `{"src", "dst", "event"}` objects — `src`/`dst` must
each name a declared state)
**Optional:** `why` (default `""`), `cyclic` (bool, default `false`), `fields`
(list of `{"name", "kind", "required", "ref_target"}` objects, default `[]`;
`kind` is one of `string` | `number` | `enum` | `reference` | `state`;
`ref_target` names the target EntityType/Stakeholder when `kind` is
`reference`)

```json
{
  "kind": "EntityType",
  "slug": "release",
  "description": "A shippable unit of work moving from draft to live.",
  "why": "the domain needs to track releases through their own lifecycle",
  "states": [
    {"name": "draft", "kind": "initial", "why": "work not yet ready to ship"},
    {"name": "shipped", "kind": "terminal", "why": "released to customers"}
  ],
  "transitions": [
    {"src": "draft", "dst": "shipped", "event": "ship"}
  ],
  "cyclic": false,
  "fields": [
    {"name": "owner", "kind": "reference", "required": true, "ref_target": "Stakeholder"}
  ]
}
```

### UPDATE mode (append new fields to an existing EntityType)

When `slug` already names an EntityType in the graph, the proposal is an
UPDATE instead of a duplicate-rejected CREATE. UPDATE mode is deliberately
narrow (first-iteration scope): it can ONLY append brand-new fields to the
existing EntityType's `fields` list — it cannot redefine an existing field,
and it cannot change `states`/`transitions`/`description`/`why`.

**Required (UPDATE shape):** `slug` (must match an existing EntityType),
`fields` (non-empty list of `{"name", "kind", "required", "ref_target"}`
objects to APPEND — a `name` that already exists on the target EntityType is
rejected, not silently redefined)
**Must be empty/omitted on UPDATE:** `states`, `transitions`, `description`,
`why` — any of these non-empty on a proposal whose `slug` already exists is
rejected (`... UPDATE currently supports ONLY appending new 'fields' ...`).
This is a scope limit, not a bug: editing an already-landed EntityType's
lifecycle/description/why is not yet supported by any proposal kind.

A successful UPDATE appends one `HistoryEntry` to the EntityType (mirroring
`ProposedRequirement`'s History-on-mutation pattern) recording the field
names that were added.

```json
{
  "kind": "EntityType",
  "slug": "release",
  "fields": [
    {"name": "linked_feature_flag", "kind": "reference", "required": false, "ref_target": "feature-flag"}
  ]
}
```

## Process

Adds a new §Process node (the opt-in behavioral aspect: a Lifecycle +
ordered Steps + `roles_required` + `drives_entities` — see
`internal/ontology/process.go`, and `PR-closed-loop` in
`domains/hotam-spec-self/graph.json` for the one worked example). **CREATE
only** — there is currently no UPDATE path for an already-landed Process; a
proposal whose `id` already exists is rejected as a duplicate, not merged.

The Process's `lifecycle` is NOT author-supplied: every landed Process is
stamped with the single shared `ontology.ProcessLifecycle`
(`READY → RUNNING → BLOCKED → DONE → ABANDONED`) — there is no field to
override it.

**Required:** `id` (must start with `PR-`), `steps` (non-empty list of
`{"name", "requires_role", "invokes", "why"}` objects — each step's `name`,
`requires_role`, and `why` must be non-empty; `invokes`, when non-empty, must
be `"<entity-slug>.<event>"` naming a real transition of a declared
EntityType), `roles_required` (list of role names — must equal EXACTLY the
set of `requires_role` values used across `steps`: every step's role must be
declared here, and every declared role must be used by at least one step; no
implicit and no undemanded roles)
**Optional:** `drives_entities` (list of EntityType slugs; each MUST resolve
to a declared EntityType in the target domain's graph — an unresolvable slug
is rejected with a clear error naming it), `why` (default `""`)

```json
{
  "kind": "Process",
  "id": "PR-release-review",
  "steps": [
    {"name": "propose", "requires_role": "operator", "invokes": "", "why": "draft the release for review"},
    {"name": "approve", "requires_role": "steward", "invokes": "", "why": "steward signs off before ship"}
  ],
  "roles_required": ["operator", "steward"],
  "drives_entities": ["release"],
  "why": "models the release-review behavioral flow as a first-class Process"
}
```
