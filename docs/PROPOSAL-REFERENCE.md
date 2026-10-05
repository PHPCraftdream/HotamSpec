# Proposal JSON reference

The graph is never hand-edited (`R-no-hand-edit-graph`). This reference covers
the JSON-proposal path, `hotam apply-proposal` / `hotam land`, for domains
using graph-authority. A domain declaring
`requirements_authority: "code"` authors Requirements/Rejections in its Go
registry and projects them with `hotam sync-domain`; its Requirement proposals
are locked. See [QUICKSTART-CONSUMER.md](QUICKSTART-CONSUMER.md) and
[AUTHORED-SPEC-CONTRACT.md](AUTHORED-SPEC-CONTRACT.md) for that authoring path.

A proposal is one JSON object with `"kind"` selecting a shape; all other keys
are **snake_case** and match the `json` tag exactly. Decode is strict
(`json.Decoder.DisallowUnknownFields`): an unrecognized or mistyped key,
including an obsolete camelCase spelling, is a hard parse error, never a
silently dropped field.

Source of truth: `internal/proposal/types.go` (the `Proposed*` structs and
their `json` tags), `cmd/hotam/apply_proposal.go` (`parseProposal` /
`unmarshalProposal`) and `internal/proposal/*.go` (`validate()`/`mutate()`).
If this document and code disagree, the code wins — please file an issue.
Each fenced JSON proposal below is checked against the actual decoder by
`cmd/hotam/proposal_reference_test.go`.

Usage:

```bash
hotam apply-proposal proposal.json --domain domains/my-shop --today 2026-07-12
```

## How a proposal is persisted (graph.json, not source code)

Unlike the historical Python prototype (which spliced Python source inside a
hand-authored `graph.py` via `ast` line/column edits), the Go CLI's graph is
plain data in `domains/<name>/graph.json`. `hotam apply-proposal`
(`cmd/hotam/apply_proposal.go` → `internal/proposal.Apply`,
`internal/proposal/apply.go`) does the following, in order:

1. Decode the proposal JSON into the matching `Proposed*` Go struct (strict,
   snake_case, unknown fields rejected — see above).
2. Run that struct's own `validate()` (required fields, enum membership,
   cross-field rules such as "`decided_by` required when `new_lifecycle`
   starts with `DECIDED`").
3. Load `graph.json` into memory (`internal/loader.LoadGraph`).
4. Run the proposal's `mutate(graph, today)`; code-authority
   `sync-domain`/`sync-self` are separate graph projection paths.
5. Recompute `internal/invariants.AllViolations` before and after the
   mutation; if the mutation introduces any NEW violation that did not exist
   before, the whole apply fails closed and NOTHING is written.
6. Only if the violation set did not grow, write the mutated graph back to
   `graph.json` (`internal/loader.WriteGraph`).

`hotam apply-proposal` has no `--dry-run` flag; it applies one proposal unless
`--batch <dir>` is supplied. Code-authority `sync-domain` has its separate
dry-run-by-default / `--confirm-hash` handshake.
The `--batch <dir>` flag on both `hotam apply-proposal` and `hotam land`
accepts a directory of `*.json` proposals, applied atomically in filename order
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

Each entry in `relations` is a JSON **object** of the shape
`{"kind": "<kind>", "target": "<id>"}`, where `target` is the id of another
Requirement already in the graph — e.g.
`{"kind": "refines", "target": "R-parent"}`. (A compact `[kind, target]`
array pair is the historical Python prototype's wire format; the Go decoder
has no custom `UnmarshalJSON` to accept it, so an array here is a hard parse
error: `cannot unmarshal array into Go struct field
ProposedRequirement.relations`.)

| Kind | Meaning | Direction |
|------|---------|-----------|
| `refines` | A supportive, non-adversarial edge -- this requirement elaborates or narrows the target. (Also covers what used to be a separate `supports` kind, merged into `refines`.) | carrier → target |
| `depends_on` | This requirement's guarantee relies on the target holding. | carrier → target |
| `replaces` | Anti-relitigation edge -- this requirement (the carrier) REPLACES the target (normally a REJECTED requirement). Usually written automatically by a `Rejection` proposal's `replaced_by` field, rather than hand-authored. | carrier → target (carrier replaces target) |

### Requirement source links and coverage qualifications

`source_refs` remains free-form provenance. `source_links` is the typed list
of objects with `source_id` and `anchor`, resolved against the domain
manifest's `specification_sources` (id, path, version, SHA256). Anchors are
Markdown headings or `Lx` / `Lx-Ly` ranges. Missing/drifting bytes or unresolved
anchors are reported by own-field-triggered invariants. Resolving a link
does not prove semantic correspondence or completeness.

`coverage` is an optional object with `status`, `rationale`, and `profile`.
Authored statuses are `unsupported_recommendation`, `unreachable`, and
`unverified`; all declarations need rationale. Unsupported and unreachable
need source links, and unreachable also needs an explicit profile. Runtime
`verified` and `discrepancy` statuses
are observations, not declarations that authors may set to turn failures green.
Observed failures override an authored qualification in evidence reports.

On UPDATE, omitted/null `source_links` and `coverage` preserve existing values.
See [AUTHORED-SPEC-CONTRACT.md](AUTHORED-SPEC-CONTRACT.md) for recorder/context
details and the separate human findings review contract.

Confrontation confidence is not inferred semantic contradiction:
opposite-marker words are advisory `lexical_suspicion`, authored links may
strengthen `linked_suspicion`, and only an explicit unresolved Conflict
carrier produces `formal_conflict` blocking. Acknowledgement must cite the
actual carrier; arbitrary existing Conflict IDs do not waive unrelated pairs.

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
Conflict's resolver must not own any of its members, so you need at least two
distinct stakeholders before you can hold a tension.

**Required:** `id`, `name`, `domain`
**Optional:** `why` (default `""`)

```json
{"kind": "Stakeholder", "id": "carol", "name": "Carol", "domain": "governance", "why": "neutral party for the first conflict"}
```

## Axis

Adds a new controlled-vocabulary tension dimension (e.g. "speed vs rigor"),
or UPDATES an existing one's `description` (when `slug` already resolves to
a node in the graph). Conflicts cluster around axes. There is no dedicated
`hotam create-axis` scaffolding command in this Go CLI yet (unlike the
historical Python prototype) — an Axis proposal is written and applied the
same way as every other kind below, via `hotam apply-proposal`.

**Required:** `slug` (kebab-case; on CREATE must not already exist and
`description` is then also required; on UPDATE must match an existing Axis)
**Optional:** `description` (required on CREATE; on UPDATE, patch semantics —
omitted or empty preserves the existing value, a non-empty value REPLACES
it), `why` (default `""`)

```json
{"kind": "Axis", "slug": "speed-vs-rigor", "description": "ship fast vs verify thoroughly", "why": ""}
```

### UPDATE semantics (description only)

When `slug` already names an existing Axis, the proposal UPDATES its
`description` in place instead of being rejected as a duplicate. Patch
semantics (same idiom as `ProposedRequirement`'s optional fields): an empty
or omitted `description` leaves the existing value untouched; a non-empty
value replaces it outright. A successful UPDATE appends one `HistoryEntry`
to the Axis's `history` (mirroring `ProposedRequirement`'s History-on-
mutation pattern) recording the before/after description.

```json
{"kind": "Axis", "slug": "speed-vs-rigor", "description": "ship fast, verify continuously not up-front", "why": "sharpened after the async-verification decision"}
```

## Requirement

Adds a new business claim, or UPDATES an existing one (when `id` already
resolves to a node in the graph).

**Required:** `id`, `claim`, `owner` (a Stakeholder id), `status` (`DRAFT` |
`SETTLED` | `OPEN(<question>)` — see [Enum reference](#enum-reference) above;
`REJECTED` is set only via the `Rejection` kind below, never directly)
**Optional:** `why` (default `""`), `assumptions` (list of Assumption ids, default `[]`),
`relations` (list of `{"kind": "<kind>", "target": "<id>"}` objects — kind is
`refines` | `depends_on` | `replaces`, see [Enum reference](#enum-reference);
default `[]`), `enforcement`
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
sentinel `"<clear>"` clears an existing value once the blocking feature ships),
`implemented_by` (list of path-qualified `file:symbol` refs into the domain's
authored spec code where this claim is EMBODIED, e.g.
`"spec/model/risk.go:NewRisk"`; default `[]`; on an UPDATE, the single-element
`["<clear>"]` sentinel empties an existing list), `verified_by` (list of
path-qualified `file:test` refs where this claim is PROVEN, e.g.
`"spec/model/risk_test.go:TestNewRisk_RejectsMissingOwner"`; default `[]`;
same `["<clear>"]` sentinel on UPDATE — the authored-era counterpart of
`enforced_by`, which keeps naming engine-side `check_*`/`Test*` enforcers),
`source_links` (typed source-id/anchor objects; omitted/null preserves an
existing list on UPDATE; see the source links section above), `coverage`
(authored status/rationale/profile qualification; omitted/null preserves
an existing declaration on UPDATE; verified/discrepancy are computed),

`signoff` (an `{"decided_by": "...", "date": "...", "verbatim": "...",
"instrument": "..."}` object recording a typed human-decision provenance for
THIS UPDATE — default omitted/`null`; UPDATE-only, rejected on CREATE;
`decided_by` MUST resolve to a declared Stakeholder id and `verbatim` is
required; `date`/`instrument` default to today/`"personal"` when omitted;
`chosen_variant` MUST stay empty — it is a Conflict-variant-only concept.
When set, the resulting `HistoryEntry` gets `decided_by`/`signoff`
populated instead of the free-text-only entry an UPDATE otherwise leaves.
Prefer this over `--decision-ref` for a real judgment-call decision;
`--decision-ref` remains best for lighter mechanical acknowledgments — see
the semantic-conflict gate docs)

Additional optional Requirement fields are `claim_texts` (language-code to
localized claim), `atom_kind` (`"rule"` explicitly selects rule mode;
absent/empty preserves fact mode), `cases`, `clause_links`, `strength`
(`MUST`/`SHOULD`/`MAY`), `applicability` (`operations`, `profiles`,
`features`) and `precedence` (`target`, `scope`, optional `applicability`).
If `claim_texts` is supplied it covers every declared language, and its
default-language text is exactly the primary `claim`; it is not a fallback
map. Cases/inventories do not become mandatory because the graph is
multilingual.

```json
{
  "kind": "Requirement",
  "id": "R-ship-fast",
  "claim": "Ship within one week.",
  "owner": "alice",
  "status": "SETTLED",
  "why": "customers expect weekly releases",
  "relations": [
    {"kind": "refines", "target": "R-release-cadence"},
    {"kind": "depends_on", "target": "R-ci-pipeline-green"}
  ],
  "enforcement": "PROSE",
  "last_reviewed_at": "2026-07-10",
  "review_after": "2026-12-01",
  "evidence": ["p99 latency held under 200ms for 3 releases"],
  "source_refs": ["docs/roadmap.md", "review-2026-07"]
}
```

### Multilingual and conformance Requirement fields

These fields use exact snake_case wire names and the existing Requirement
patch semantics: an omitted optional field preserves its value.
`languages`, `default_language` and `conformance` are manifest fields, not
Requirement proposal fields. `conformance.rule_cases: true` permits `WithCase`
rule mode; it does not by itself scan model methods. Automatic `sync-domain`
discovery of rule atoms requires both `requirements_authority: "code"` and
`self_executing_atoms: true`. Source clauses, profiles and compositions are
declared in `manifest.json` and referenced from Requirements.

In the manifest's `conformance` object, the exact keys are `rule_cases`,
`clauses`, `profiles` and `compositions`. A clause uses `id`, `source_links`
(`source_id` plus `anchor`), `sides`, `strength` and `applicability`; a profile
uses `id`, `operations`, `features`, `integer_min`, `integer_max`,
`float_domain`, `rounding`, `preserves_order` and `capabilities`; a composition
uses `id` and `components` (`id`, `role`, `version`, `sha256`, `measured`).
Applicability keys are `operations`, `profiles` and `features`.

```json
{
  "kind": "Requirement",
  "id": "R-parser-accepts",
  "claim": "The parser accepts a valid input.",
  "claim_texts": {
    "en": "The parser accepts a valid input.",
    "ru": "Парсер принимает корректный ввод."
  },
  "atom_kind": "rule",
  "owner": "alice",
  "status": "DRAFT",
  "cases": [
    {
      "id": "valid-input",
      "atom_ids": ["R-parser-accepts"],
      "input": {"kind": "text", "text": "valid"},
      "expected": {"kind": "text", "text": "accepted"},
      "operation": "parse",
      "target": "parser",
      "producer": "in-package-parser"
    }
  ]
}
```

`CaseDefinition.Input` and `.Expected` are typed independent case/oracle
values, not the SUT's actual output. A Go case table's `WithInput` and
independent `want`/`Expect` may project into a new graph case; an explicitly
authored registry/proposal descriptor must agree with recorded declarations
and is never overwritten. Its file-qualified `test` link comes from a real
execution, not a guessed filename; it is a case/report reference, not
`Requirement.verified_by` or proof by itself. `atom_ids`, `profile`,
`operation`, `target`, `producer`, `fixtures`, `conditions`, `sides` and `selection`
carry case scope and provenance; condition/selection evidence is supplied only
when observed. One case may link multiple atoms and an atom may have multiple
cases.

These persistent `CaseDefinition` values are distinct from runtime-only
`CaseContext.Expected`: the recorder excludes that field from the serialized
`case` object and stores an optional shared oracle at root `case_expected`.
Without it, root `case_expected` is supplied by `Fact`'s independent `want` or
`Holds`' `Expect`; each `Observation.RawExpected` remains property-specific.

`fixtures` entries require `id`, `category`, `path`, `role`, `sha256` and
`raw_bytes`; `sha256` is 64 hexadecimal characters validated against exact
bytes. `raw_bytes` is an explicit boolean serialized as both `false` and
`true`; `true` marks raw-byte input. Roles are `input`, `expected`, `canonical`,
`error` or `extra`; `category` is generic domain-defined data. Relative paths
resolve from the domain specification root; explicit absolute paths are allowed.
`input` and `expected` use `ObservedValue`'s typed fields, not replacement
text for raw bytes. For
`kind: "bool"`, the JSON `bool` field preserves explicit `false` separately
from an absent value. A passing corpus or case set is not a claim of semantic
completeness.

`clause_links` identify authored `ConformanceConfig.Clauses` by `clause_id`
and a side when the clause declares sides. Each source clause declares its
stable `id`, typed `source_links`, `sides`, optional `strength` and applicability.
`profiles` declare `id`, operations/features, integer bounds, float domain,
rounding, order-preservation and string-valued capabilities. `compositions`
declare `id` and components (`id`, `role`, `version`, `sha256`, `measured`).
`applicability` combines its non-empty classes: operations/profiles match any
listed value, while all listed features are required. A precedence entry is
`{"target":"R-prior-rule","scope":"parse-selection","applicability":{"operations":["parse"],"profiles":["reference"],"features":["raw-bytes"]}}`;
the target must resolve, and cycles within a strict scope are invalid. These
declarations establish structural traceability/selection, not semantic proof
or automatic component blame. See
[AUTHORED-SPEC-CONTRACT.md §13](AUTHORED-SPEC-CONTRACT.md#13-atomic-multilingual-and-conformance-specifications)
for the audit and evidence boundary.


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
`review_after`, `created_at`, and the atomic fields `claim_texts`, `atom_kind`,
`cases`, `clause_links`, `strength`, `applicability`, and `precedence` keep
their existing values when omitted/defaulted. The behavior described here is
the current patch contract.

#### Atomic fields on UPDATE

Omitted or nil fields preserve their current values. Clear/reset behavior is
explicit: `atom_kind` and `strength` accept the `"<clear>"` sentinel;
`claim_texts` clears with the sole map entry `{"<clear>": ""}`; explicit empty
`cases`, `clause_links` or `precedence` arrays clear those lists; and
`applicability: {}` explicitly sets universal applicability. An omitted or
`null` applicability preserves the current pointer. Clear sentinels are
rejected on CREATE.

When supplied, non-empty `claim_texts` must contain one non-empty phrase for
every manifest language and the default-language entry must exactly equal
`claim`. Case IDs must be unique and typed `Input`/`Expected` descriptors must
validate; a supplied selection's selected value must be among its matched
values. These fields are strict schema data, not ignored metadata.

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
`members` (list of >= 2 distinct Requirement ids), `resolver` (a Stakeholder id
that owns none of the members)
**Optional:** `shared_assumption` (an Assumption id, default `""`), `note`
(presentation-only, never written to the graph, default `""`),
`initial_lifecycle` (default `"DETECTED"`; only a DECIDED constituting-atoms
edge case may start elsewhere — see `internal/proposal/mutate.go`),
`decided_by` (required only if `initial_lifecycle` starts with `DECIDED`),
`source_refs` (list of pointers to where the conflict's context/evidence
originated — doc paths, URLs, review ids, commit hashes; default `[]`. The
same free-form provenance shape `Requirement.source_refs` already
establishes — no `check_*` invariant validates these entries resolve to
anything real, mirroring the Requirement precedent rather than inventing a
stricter rule for Conflict alone.)

```json
{
  "kind": "Conflict",
  "axis": "speed-vs-rigor",
  "context": "first release cadence",
  "members": ["R-ship-fast", "R-verify-all"],
  "resolver": "carol",
  "shared_assumption": "",
  "note": "surfaced while scaffolding the demo domain",
  "source_refs": ["docs/roadmap.md"]
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
`date` (ISO date, defaults to today), `verbatim` (the resolver's own words,
default `""`), `instrument` (`"personal"` default, or `"DEL-<n>"` for a filed
delegation), `chosen_variant` (a `V-id` from `variants`, when resolving
`HELD` -> `DECIDED`), `source_refs` (list of provenance refs; default `[]` =
leave untouched; a non-empty value REPLACES the Conflict's existing
`source_refs` outright — same "empty preserves, non-empty replaces" idiom as
`derived`/`variants` above)

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
**Optional:** `why` (default `""`), `created_at` (ISO date, defaults to today),
`source_refs` (list of pointers to where the assumption originated — doc
paths, URLs, review ids, commit hashes; default `[]`. The same free-form
provenance shape `Requirement.source_refs` already establishes — no `check_*`
invariant validates these entries resolve to anything real.)

```json
{"kind": "Assumption", "id": "A-weekly-cadence-tolerated", "statement": "Customers tolerate a weekly release cadence.", "status": "HOLDS", "owner": "alice", "why": "stated in the last customer survey", "source_refs": ["docs/customer-survey-2026-07.md"]}
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

## AssumptionRewrite

A CLEAN REWRITE of an existing Assumption's `statement` — distinct from
`AssumptionTransition` above, which changes `status` and APPENDS a
`" — [STATUS] reason"` suffix onto `statement` as a side effect of a status
decision. Use `AssumptionRewrite` when the assumption's WORDING needs
correcting (a typo, an ambiguity, a scope clarification) with no status
change involved at all. A successful rewrite ALWAYS appends one
`HistoryEntry` to the Assumption's `history` recording the before/after
statement plus the reason — a rewrite with no History trail would be silent,
unaudited drift of what the assumption even claims.

**Required:** `assumption_id`, `new_statement` (non-empty — replaces
`statement` outright), `reason` (non-empty — a rewrite with no recorded
reason is drift, not a decision, mirroring `AssumptionTransition`'s own
`reason` requirement)
**Optional:** `signoff` (an `{"decided_by": "...", "date": "...",
"verbatim": "...", "instrument": "..."}` object recording a typed
human-decision provenance SUPPLEMENTING `reason` — default omitted/`null`;
never replaces the required `reason` field; `decided_by` MUST resolve to a
declared Stakeholder id and `verbatim` is required; `date`/`instrument`
default to today/`"personal"` when omitted; `chosen_variant` MUST stay
empty — it is a Conflict-variant-only concept. When set, the rewrite's
already-unconditional `HistoryEntry` gets `decided_by`/`signoff` populated
too)

```json
{
  "kind": "AssumptionRewrite",
  "assumption_id": "A-weekly-cadence-tolerated",
  "new_statement": "Most customers tolerate a weekly release cadence; a vocal minority want daily.",
  "reason": "the original wording overstated the survey's actual finding"
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
(the resolver re-affirmed a claim is still true) stays distinguishable from a
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
warranted, is a resolver call, not decided here.

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
`reference`), `model_symbol` (default `""`; an optional `"file:Symbol"`-shaped
reference, same shape as a Requirement's `implemented_by` entries, naming the
Go type in the domain's authored `spec/model/` tree that this EntityType
corresponds to — empty means no Go type yet, or no 1:1 Go counterpart;
one-directional only, this never generates the named Go symbol)

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
`why`, `model_symbol` — any of these non-empty on a proposal whose `slug`
already exists is rejected (`... UPDATE currently supports ONLY appending new
'fields' ...`). This is a scope limit, not a bug: editing an already-landed
EntityType's lifecycle/description/why/model_symbol is not yet supported by
any proposal kind.

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
`domains/hotam-spec-self/graph.json` for the one worked example). Supports
both CREATE (below) and a narrow UPDATE mode for an already-landed Process
(see "UPDATE mode" subsection further down) — this mirrors EntityType's
CREATE/UPDATE split (ffa4977).

The Process's `lifecycle` is NOT author-supplied on either CREATE or UPDATE:
every landed Process is stamped with the single shared
`ontology.ProcessLifecycle` (`READY → RUNNING → BLOCKED → DONE → ABANDONED`)
— there is no field to override it, and UPDATE never touches an
already-landed Process's `lifecycle`.

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
    {"name": "approve", "requires_role": "resolver", "invokes": "", "why": "resolver signs off before ship"}
  ],
  "roles_required": ["operator", "resolver"],
  "drives_entities": ["release"],
  "why": "models the release-review behavioral flow as a first-class Process"
}
```

### UPDATE mode (append steps/drives_entities, replace why, on an existing Process)

When `id` already names a Process in the graph, the proposal is an UPDATE
instead of a duplicate-rejected CREATE. UPDATE mode is deliberately narrow
(mirrors EntityType's UPDATE-mode scope limit, ffa4977): it can APPEND new
entries to `steps` and `drives_entities`, and REPLACE `why` — but it can
never redefine, remove, or reorder an existing step or `drives_entities`
entry, and it never touches `lifecycle`.

**Required (UPDATE shape):** `id` (must match an existing Process); at least
ONE of `steps` (non-empty list of NEW steps to APPEND to the end of the
existing list — validated with the exact same per-step rules as CREATE:
non-empty `name`/`requires_role`/`why`; a `name` that already exists on the
target Process is rejected, not silently redefined or reordered),
`drives_entities` (list of NEW EntityType slugs to APPEND — each MUST
resolve to a declared EntityType, and a slug already present on the target
Process is rejected, not silently deduplicated), or `why` (non-empty —
REPLACES, not appends, the existing Process's `why`; this is the one field
UPDATE treats as a correction rather than an addition, matching
`ProposedRequirement`'s `why` semantics rather than EntityType's
UPDATE-mode ban on touching `why` at all)
**Conditionally required:** `roles_required` — when `steps` is non-empty, it
must equal EXACTLY the set of `requires_role` values used by the NEW steps
in THIS proposal (same "no implicit, no undemanded" rule as CREATE; roles
already declared by pre-existing steps do not need to be restated — they are
carried over automatically). When `steps` is empty, `roles_required` MUST
also be empty (there is nothing in this proposal for it to declare).

A successful UPDATE appends one `HistoryEntry` to the Process (mirroring
`ProposedEntityType`'s UPDATE-mode History-on-mutation pattern) recording
which step names and/or drives_entities slugs were added, and/or that `why`
was updated.

```json
{
  "kind": "Process",
  "id": "PR-release-review",
  "steps": [
    {"name": "notify", "requires_role": "operator", "invokes": "", "why": "tell stakeholders the release shipped"}
  ],
  "roles_required": ["operator"],
  "drives_entities": ["feature-flag"]
}
```

## Goal

Adds a new §Goal node (the first-class target-state type, M19: a moving
target that yields a Gap driving a Process — `internal/ontology/process.go`).
Resolved (task #392/W0.5) onto the ordinary Proposed* JSON path, symmetric to
Conflict/Assumption/EntityType above: Goal is a typed graph node with its own
semantics, not a structural domain-model object, so it does NOT move to
Go-code authorship the way Requirement/Rejection do under
`requirements_authority: code` / self-hosting — those locks never touch Goal.

CREATE-only: there is no UPDATE mode (unlike EntityType/Process). A `id` that
already names a Goal in the graph is rejected as a duplicate re-declaration,
not merged.

`lifecycle` is NOT author-supplied: every landed Goal is stamped with the
single shared `ontology.GoalLifecycle`'s INITIAL state (`ACTIVE`) — there is
no field to override it.

**Required:** `id` (must start with `GOAL-`), `owner` (must resolve to a
declared Operator id — the acting facet that pursues the goal, M19),
`target_state` (object `{"kind", "predicate", "target"}` — `kind` must be one
of `GRAPH_PROPERTY` | `BUSINESS_STATE` | `ENTITY_STATE`; `target` is required
and non-empty; `predicate` is a free-text description of the condition)
**Optional:** `why` (default `""`)

```json
{
  "kind": "Goal",
  "id": "GOAL-burn-down-zero",
  "owner": "OP-director",
  "target_state": {
    "kind": "GRAPH_PROPERTY",
    "predicate": "count(r for r in g.requirements if r.status==SETTLED and r.enforcement!=ENFORCED) == 0",
    "target": "enforcement-gradient"
  },
  "why": "burn-down meter as an ACTIVE goal owned by the director: every SETTLED requirement should reach ENFORCED"
}
```

## EntityInstance

Adds a concrete instance of an already-declared EntityType (§Entity's
EntityType + EntityField + EntityInstance triad — `internal/ontology/entity.go`).
Resolved (task #392/W0.5) onto the ordinary Proposed* JSON path, symmetric to
`EntityType` above: an EntityInstance is a graph node with its own
referential-integrity semantics (`entity_type`/`state`/`field_values`
resolution against the rest of the graph), not a structural domain-model
object, so it stays off the Go-code-authority path.

CREATE-only: there is no UPDATE mode. A `id` that already names an
EntityInstance in the graph is rejected as a duplicate re-declaration, not
merged.

`field_values` uses the same array-of-2-element-arrays wire shape
`ontology.EntityInstance.FieldValues` already uses on `graph.json` (a list of
`["name", "value"]` pairs), not a JSON object.

**Required:** `id` (must start with `ENT-`; note the STRICTER
`check_entity_instance_id_prefix` invariant additionally requires the exact
form `ENT-<entity_type>-...` once landed), `entity_type` (must resolve to a
declared EntityType slug in the target domain's graph), `state` (must be a
valid state name in that EntityType's lifecycle)
**Optional:** `field_values` (list of `["name", "value"]` pairs, default
`[]`; a `reference`-kind field's value must resolve per its `ref_target`, and
every `required` field must be present — both checked by the
`check_entity_instance_*` invariant family after landing, not by this
proposal's own validation)

```json
{
  "kind": "EntityInstance",
  "id": "ENT-feature-flag-1",
  "entity_type": "feature-flag",
  "state": "INIT",
  "field_values": [
    ["owner", "sa"]
  ]
}
```
