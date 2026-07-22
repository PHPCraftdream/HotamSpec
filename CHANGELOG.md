# Changelog

All notable engine-level changes to Hotam-Spec are documented here. Format
loosely follows [Keep a Changelog](https://keepachangelog.com/); this
project has no release/tag cadence yet, so everything lives under
`[Unreleased]`.

Scope: this file covers the **HotamSpec engine repo** only (`internal/`,
`cmd/hotam`). Consumer-domain history (`domains/prat`, `domains/gpsm-sm` in
the separate `PRAT-hotam` repo) is not tracked here.

History predating this file is not backfilled — see `git log` and
`docs/checkpoints/` for earlier waves. Entries below start from the
"review-remediation" wave (external review scored the crystal-interface work
6/10, up from 4–4.5/10) and continue through the follow-up backlog wave.

## [Unreleased]

### Added
- **Typed per-Requirement gate-signoff carrier** (`GateSignoff`,
  `internal/ontology/gate_signoff.go`): `Requirement.GateSignoffs[]` records
  `Stage`/`State`/`DeferredReason`/`Evidence`/`PipelineRun`/`Signoff` per
  Planning-gate stage. Stage order is domain-declared
  (`gate_stage_order` in `manifest.json`, `internal/loader/gate_stage_order.go`),
  not hardcoded. Three new invariants
  (`internal/invariants/gate_signoff_checks.go`): monotonic progression,
  deferred-reason presence, deferred-conflict resolution. New batch proposal
  kind `ProposedGateSignoffBatch` applies N transitions across possibly-
  different Requirements in one `apply-proposal` call.
- **SIGNED gate-signoffs now require human provenance** (task #319,
  R3-signoff-strict — external review): `ProposedGateSignoffBatch.validate()`
  (`internal/proposal/validate.go`) now rejects a `state=SIGNED` entry that
  is missing `decided_by`, `verbatim`, or `evidence` — mirroring the
  DEFERRED branch's existing `deferred_reason` requirement. Before this
  change a SIGNED gate-signoff could land with zero provenance about who
  decided it, what they said, or why. Two new ongoing `all-violations`
  invariants (`internal/invariants/gate_signoff_checks.go`) enforce the same
  rule against already-landed data: `check_gate_signoff_signed_has_provenance`
  (SIGNED requires a populated `Signoff` with `decided_by`/`verbatim` plus
  non-empty `evidence`) and `check_gate_signoff_decided_by_is_known_stakeholder`
  (when present, `decided_by` must resolve to a real `Stakeholder.id`,
  mirroring `Conflict`'s existing `check_decided_by_is_known_stakeholder`).
  Both are ongoing invariants rather than proposal-time-only, matching
  `check_gate_signoff_deferred_reason_present`'s own precedent — verified
  safe against `prat/gpsm-sm`'s 64 already-landed SIGNED gate-signoffs,
  which already carry full provenance. This was blocking task #323
  (life-domain work), which needs provenance guaranteed before real
  personal signoffs land.
- **Crystal freshness invariant** (`check_domain_claude_md_current`,
  `internal/invariants/claude_md_current.go` +
  `cmd/hotam/claude_md_current_wiring.go`): a committed `CLAUDE.md` whose
  generated portion no longer byte-matches a fresh render is now a real
  violation, not silently stale. Wired via the same registry-patch pattern
  `tool_wiring.go` uses (real logic lives in `cmd/hotam` to avoid a genuine
  import cycle: `internal/invariants` must never import `internal/generator`).
- **Typed UPDATE-path for Axis and Assumption**: `ProposedAxis` is now
  CREATE-or-UPDATE (an existing slug patches `Description`, appends a
  `History` entry, mirroring `Requirement`'s coalesce pattern). New proposal
  kind `ProposedAssumptionRewrite` does a clean replace of
  `Assumption.Statement` (distinct from `ProposedAssumptionTransition`,
  which only appends a status-change suffix) — `Reason` required, `History`
  entry appended unconditionally so a rewrite can never silently skip its
  audit trail.
- **`SourceRefs` on `Conflict` and `Assumption`** (`internal/ontology/`),
  mirroring the existing `Requirement.SourceRefs` shape — no resolvability
  invariant added, matching that same precedent.
- **`charter` manifest field** (`internal/loader.DomainPresentation`):
  optional one-line statement of a domain's own "nature of result" (e.g.
  "this is a code-spec-test model, not a deployed system"), rendered
  immediately after `purpose` in the domain's Project-essence block.
- **DOMAIN-MAP gate-progress line**: a sibling domain's Domain-Map entry now
  shows `- **gates** — <frontier-stage>: N/M SIGNED · K DEFERRED`, computed
  in the same pass that already tallies `atoms-count` (a pure read of the
  sibling graph's `GateSignoffs`, never a fresh `AllViolations`/`Diagnose`
  call for that domain).
- **CONFRONT step gains an operational rule** (`mediationLoopText`,
  `internal/generator/claudemd_static.go`): when an input cites a
  real-world event/deadline/party, ask first whether it blocks the MODEL or
  only the deployed reality — resolve by modeling unless the domain's
  charter says otherwise. This is the one core-template edit that
  propagates to every crystal in every domain through regeneration.
- **Business-before-methodology consumer crystal template**
  (`internal/generator/claudemd.go`): consumer-profile domains now render
  purpose/goals/stakeholders/live-state before the generic Hotam-Spec
  methodology seed, instead of methodology-first.
- **Orientation FAQ answerability invariant**: fail-closed on a malformed
  manifest that still declares onboarding intent, reports dropped FAQ
  entries instead of silently discarding them, rejects all-blank keyword
  lists, and checks the linked answer file's actual content
  (`os.ReadFile`, not a bare `os.Stat`).

### Changed
- **CI pipeline split into parallel jobs, ~2.1x wall-clock speedup** (task
  #327): the single `build-and-test` job (`.github/workflows/ci.yml`) that
  ran Build → gofmt → vet → `go test -race -timeout 30m ./...` → gen-spec
  idempotency serially — 9m19s total on the last green baseline run
  (29954625735), of which `go test -race ./...` alone was 8m53s (~95%) —
  is now five jobs: a fast shared `lint-and-build` gate (Build/gofmt/vet,
  ~26s) fanning out via `needs:` to four parallel jobs — `test-race`
  (`-race`, scoped to `internal/gate`/`internal/generator`/
  `internal/invariants`, the only packages with real goroutine/sync usage
  in non-test code, ~100s), `test-cmd-hotam` (`cmd/hotam`'s own tests
  without `-race` — its e2e tests spawn a real compiled subprocess binary
  that `-race` on the parent process doesn't instrument, so `-race` there
  bought near-zero extra coverage for full instrumentation cost, ~169s),
  `test-other` (every remaining small `internal/*` package, no `-race`
  needed, ~40s), and `gen-spec-idempotency` (now depends only on a
  successful build, not on the test jobs finishing, ~35s). First real
  post-split CI run (29960077353): **4m24s total**, down from 9m19s.
  `Makefile` gained `test-race-scoped`/`test-cmd-hotam`/`test-other`
  targets mirroring the new CI jobs, plus `test-fast` (`-short`, no
  `-race`) for quick local iteration; existing `test`/`test-race`/`check`
  targets are unchanged. Stage-3 (`t.Parallel()` expansion) and stage-4
  (killswitch fixture sharing) from the task plan were evaluated against
  real per-test CI timing and not pursued: `test-cmd-hotam`'s actual CI
  cost (169s) no longer dominates the pipeline post-split, the package
  already carries 255 `t.Parallel()` calls, and its few remaining serial
  `t.Setenv`-bound tests are slow because of the real gen-spec/invariant
  work they do against a full 320-node domain graph, not `t.Setenv`
  overhead itself — de-serializing them would require injecting env into
  `resolveDomain` instead of reading `os.Getenv` directly, a production-
  code design change disproportionate to the remaining payoff.
- **`apply-proposal`'s structural false-positive class fixed**: checks that
  compare against an on-disk projection only `gen-spec` regenerates
  (`check_spec_md_current`, `check_domain_claude_md_current`) are now
  excluded from the pre/post-mutation violation diff via a new
  `Invariant.ComparesOnDiskProjection` flag and
  `AllViolationsForProposalGate`/`AllViolationsExcludingDiskProjection`.
  `AllViolations` itself (used by `all-violations`/`status`/`diagnose`) is
  unchanged — staleness is still reported there. Retires the `wlock_tmp`
  hand-edit workaround this class of false positive required (used 8+ times
  across the sessions before this fix).
- **`gen-spec` write ordering**: `activeViolations`/crystal char-count are
  now computed AFTER the first write phase (so a freshly-written SPEC.md
  etc. is on disk before the freshness signal is computed), closing a
  permanently-stale-signal bug.
- Global rename: `steward` → `resolver` across engine code, docs, and
  generated projections (terminology cleanup).
- `abbrev()` (`internal/proposal/history.go`, used by every History-entry
  summarization for Requirement/Axis/Assumption UPDATE/rewrite paths) now
  truncates on a rune boundary instead of a raw byte index — the old byte
  slice could split a multi-byte UTF-8 character mid-encoding, producing a
  literal U+FFFD replacement character in committed graph data on
  re-serialization.

### Fixed
- **Shared `docs/gen/` projections made mode-independent** (continuing task
  #317, CI fix chain): `hotam land`'s own routine regeneration
  (`cmd/hotam/land.go`) always calls plain `genSpec` (never `--spec`), so a
  committed `TRACEABILITY.md`/`COVERAGE.md` rendered in `--spec`-shaped form
  (narration-verdict suffixes) was inherently unstable — the very next
  `hotam land` silently reverted it, which is what broke CI's `gen-spec
  idempotency` step. `BuildTraceability`/`BuildCoverage`
  (`internal/generator/traceability.go`/`coverage.go`) are now pure,
  mode-independent functions of the graph plus a cheap AST scan (their
  `verdicts ...map[string]ScenarioVerdict` variadic parameter is removed
  entirely); `REPO-MAP.md`'s `SPEC.md` listing is now stat-based (reads real
  on-disk content when present) instead of write-set-based, so a plain run
  still acknowledges an existing `SPEC.md` without rewriting it. `SPEC.md`
  remains the sole `--spec`-shaped artifact, whose freshness is separately
  enforced by `check_spec_md_current`. New regression test:
  `TestGenSpec_SharedProjectionsModeIndependent`
  (`cmd/hotam/gen_spec_test.go`).
- **Durable-notes tail preservation**: `gen-spec` regeneration was silently
  dropping an operator's hand-written notes below the durable-notes marker
  in `CLAUDE.md`, despite the template's own promise to preserve them.
  Fixed via `DurableNotesMarkerLine`/`SplitAtDurableNotesMarker`/
  `preserveDurableNotesTail`.
- **Mutual recursion between sibling domains** rendering each other's
  DOMAIN-MAP pulse (each domain's freshness check embedding the other's
  live `AllViolations` output, which itself embeds the first domain's — a
  20+ minute hang) — closed via
  `AllViolationsExcludingDiskProjection`/`DiagnoseSignalsExcludingDiskProjection`
  for sibling-pulse computation.
- **DOMAIN-MAP gate-progress double-count**: a naive per-`GateSignoff`-entry
  tally would double-count a Requirement carrying both a superseded
  `DEFERRED` and a later `SIGNED` entry at the same stage
  (`GateSignoffBatch.mutate` only ever appends) — fixed to count the last
  `State` per Requirement per stage.
