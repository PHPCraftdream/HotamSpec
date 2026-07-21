<!-- LEGACY (Python-era) — content below describes the pre-port Python implementation;
     tools and paths (spec/tools/*.py, spec/src/hotam_spec/cli/*.py, spec/pyproject.toml,
     graph.py) are historical references to the retired Python codebase. The delegation
     RETIREMENT DECISION itself remains in effect; only the file paths and tool names
     below are artifacts of the pre-Go era. -->

# Resolver campaign-delegation history (jsonl-era, retired)

Hand-authored (not generated). This file preserves the full, unabridged
content of `domains/hotam-spec-self/delegations.jsonl` before that file was
deleted (resolver triage decision, item B1: "снести старую jsonl-систему
делегаций полностью").

## Background — the two delegation mechanisms

Two distinct concepts historically shared the word "delegation" in this
repo, and only one of them survives:

1. **Campaign/case trust-anchor signature** (`DEL-<n>`, this file's subject)
   — the resolver's own explicit act of handing the DECIDED/HELD
   personal-signature duty to an agent, per-case or for a declared campaign,
   recorded in `domains/hotam-spec-self/delegations.jsonl` via
   `spec/tools/record_delegation.py`. This mechanism is now RETIRED: the
   tool, its CLI wrapper, its tests, and the ledger file itself have all
   been removed. `R-trust-anchor-delegation-explicit-only` (still SETTLED)
   now describes the file-based mechanism below instead.

2. **Task hand-off to a sub-agent** (`DG-<n>`, current, unrelated) — every
   task delegation to an agent, recorded as a versioned file under
   `delegations/DG-<n>.md`, created and closed only via
   `spec/tools/delegate.py` (`R-delegation-is-a-file`, SETTLED,
   STRUCTURAL). Git history on the committed file IS the audit trail. This
   mechanism is unaffected by this retirement and remains the live path.

The `DEL-` and `DG-` prefixes were deliberately chosen to be visually
distinct precisely so the two layers would never blur (see the discussion
in `domains/hotam-spec-self/docs/delegation-node-design.md` section 5,
written when a third design — a graph-node `Delegation` type — was
considered and ultimately rejected in favor of the file-based `DG-`
mechanism).

## Why retire the jsonl ledger rather than keep both

The resolver's verdict (triage item B1, verbatim): "снести старую
jsonl-систему делегаций полностью" (tear down the old jsonl delegation
system completely). Two delegation-recording systems doing adjacent but
distinct jobs (one full ledger for personal-signature campaign grants, one
lighter-weight file-per-hand-off log for task spawns) turned out to be more
machinery than the single concern warranted once the file-based mechanism
existed and was in active use — the campaign-signature use case (in
practice: one seed record, DEL-1, granted once and closed once) did not
justify a parallel, separately-tested, separately-enforced ledger format.

The underlying PRINCIPLE `R-trust-anchor-delegation-explicit-only` encodes
— delegation of the resolver's personal-signature duty is valid ONLY when
granted EXPLICITLY, never implied or standing by default — is untouched by
this retirement. What changed is the RECORDING MECHANISM: it is no longer a
dedicated jsonl ledger with its own writer tool, but the same versioned-file
mechanism (`delegations/DG-<n>.md` + `spec/tools/delegate.py`) already used
for every other resolver hand-off, per `R-delegation-is-a-file`.

## Verbatim content of `domains/hotam-spec-self/delegations.jsonl` (deleted)

The file held exactly one record, DEL-1, from creation to closure. Preserved
here byte-for-byte (as parsed JSON, one record per line in the original):

```json
{"id": "DEL-1", "resolver": "domain-user", "verbatim": "реши все задачи. Все вопросы решай в сторону совершенства", "date": "2026-07-02", "scope": "campaign for current work, until resolver revokes", "status": "closed", "closed_date": "2026-07-05"}
```

Field-by-field:

- **id**: `DEL-1` — the seed and only record ever written to this ledger.
- **resolver**: `domain-user` — the Stakeholder id of the human who granted
  the delegation.
- **verbatim**: `реши все задачи. Все вопросы решай в сторону совершенства`
  ("resolve all tasks. Decide every question in the direction of
  perfection") — the resolver's exact wording, carried verbatim per
  `R-speak-by-reference` (an unlabeled or unscoped delegation cannot be
  resolved back to a specific human act).
- **date**: `2026-07-02` — the day the campaign delegation was granted.
- **scope**: `campaign for current work, until resolver revokes` — an
  open-ended, standing campaign grant (not a single case).
- **status**: `closed` — the campaign was later revoked.
- **closed_date**: `2026-07-05` — the day the resolver closed it, after
  which every conflict resolution again required an explicit personal
  signature.

## What this record authorized

DEL-1 anchors the verbatim delegation that authorized the `core-vs-aspect`
Conflict's (`C-be22cdd1`) HELD -> DECIDED transition choosing variant
`V-unfreeze-entity-projection` on 2026-07-02, while the campaign was still
open. That Conflict's `lifecycle` field in `domains/hotam-spec-self/graph.py`
still carries the historical marker text
`"... per explicit campaign delegation 2026-07-02 (...)"`, and its `signoff`
field (`Signoff(decided_by="domain-user", date="2026-07-02",
instrument="DEL-1", chosen_variant="V-unfreeze-entity-projection")`,
landed by `R-signoff-preserved-in-substrate`) carries `instrument="DEL-1"`
as the permanent, in-graph provenance pointer to this historical record.
Both are left exactly as they were written — they are immutable history of
a real decision, not live pointers to a file that must keep existing;
this document is now the durable, git-preserved trace of what `DEL-1` was.

## What was removed (B1 execution)

- `spec/tools/record_delegation.py` (the writer/closer tool)
- `spec/src/hotam_spec/cli/record_delegation.py` (CLI wrapper,
  `hotam-record-delegation` entry point)
- `spec/tests/test_tool_record_delegation.py` (writer tests)
- `spec/tests/test_delegation_marker_honesty.py` (cross-file check that
  every `"per explicit campaign delegation <date>"` Conflict-lifecycle
  marker resolved to a dated `delegations.jsonl` record — retired together
  with the ledger it validated against; the historical marker text and the
  `Signoff.instrument="DEL-1"` field remain in `graph.py` as inert,
  human-readable history, no longer mechanically cross-checked)
- `domains/hotam-spec-self/delegations.jsonl` (the ledger itself, content
  preserved verbatim above)
- the `record` subcommand of `hotam-delegation` (`spec/src/hotam_spec/cli/delegation.py`)
  and the `hotam-record-delegation` entry in `spec/pyproject.toml`
  `[project.scripts]`

What was NOT removed: `spec/tools/delegate.py`, `spec/tools/_delegation_store.py`,
`delegations/DG-*.md`, and `R-delegation-is-a-file` — the file-based
task-delegation mechanism this retirement leaves untouched.
