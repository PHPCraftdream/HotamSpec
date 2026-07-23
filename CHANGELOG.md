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
- **Typed signoff on Requirement/Assumption History entries**
  (task #335, R4F-req-signoff — fourth external review's final synthesis
  §4.4): task #328's landed `R-shared-projections-mode-independent`/
  `R-orientation-faq-answerable` Requirement UPDATEs recorded a real human
  approval (Marat Karamullin, project owner/resolver, verbatim quote «Да,
  приземлить оба как есть») only as free text in `History.summary` —
  `History.decided_by` ended up `""` despite the real quote, confirmed by
  direct read of `domains/hotam-spec-self/graph.json`. `Conflict`/
  `GateSignoff` already required typed `decided_by`/`verbatim` (task #319);
  `ProposedRequirement`/`ProposedAssumptionRewrite` had no equivalent
  structure. `ontology.HistoryEntry` (`internal/ontology/requirement.go`)
  gains an optional `Signoff *ontology.Signoff` field (`omitempty`, zero
  migration — shared across Requirement/Assumption/Axis/EntityType/Process
  History, every pre-existing entry round-trips byte-identically).
  `ProposedRequirement`/`ProposedAssumptionRewrite` (`internal/proposal/types.go`)
  gain an optional `signoff` field; shape validation
  (`internal/proposal/validate.go`'s `validateHistorySignoffShape`) requires
  non-empty `decided_by`/`verbatim` when set and rejects a non-empty
  `chosen_variant` (Conflict-variant-only, unauditable junk on a History
  signoff). `internal/proposal/mutate.go`'s `resolveHistorySignoff` resolves
  `decided_by` against the domain's declared Stakeholder ids — deliberately
  UNGATED (unlike `ProposedConflict.mutate`'s zero-Stakeholder escape hatch):
  a typed signoff is per-proposal opt-in, so a domain choosing to use it must
  have named its humans. A `ProposedRequirement` UPDATE with a signoff
  appends its `HistoryEntry` UNCONDITIONALLY (even with an otherwise-empty
  field diff, using a `"(no field changes) — signoff recorded"` fallback
  summary), mirroring `ProposedAssumptionRewrite.mutate`'s pre-existing
  unconditional-append discipline (task #306); the `Signoff == nil` path
  stays byte-identical to pre-#335 behavior. CREATE-time signoff is
  explicitly out of scope and rejected with a clear error. Two new ongoing
  `check_*` invariants (`internal/invariants/history_signoff_checks.go`):
  `check_history_signoff_has_provenance` (every `HistoryEntry.Signoff`, when
  non-nil, carries non-empty `decided_by`/`verbatim`) and
  `check_history_signoff_decided_by_is_known_stakeholder` (a non-empty
  `decided_by` resolves to a known Stakeholder; skips when `decided_by` is
  empty, owned by the provenance check instead) — both sweep every
  HistoryEntry-carrying node type (Requirement/Assumption/Axis/EntityType/
  Process), anchored to the existing `R-signoff-preserved-in-substrate`
  requirement's `enforced_by` (mirroring task #319's identical precedent of
  extending `R-gate-signoff-single-carrier` rather than minting a new
  standalone-claim requirement just to satisfy
  `check_bijection_r_to_enforcer`); both start at 0 violations against the
  real `hotam-spec-self` graph (no landed HistoryEntry there carries a
  signoff yet). `cmd/hotam/semantic_gate.go`'s `hasOverride` now accepts a
  typed `ProposedRequirement.Signoff` on par with `--ack-conflict`/
  `--decision-ref` for overriding the semantic-conflict gate — a typed,
  Stakeholder-resolved decision is strictly stronger evidence than free
  text; `appendAckHistory` skips writing a redundant free-text duplicate for
  the decision itself when only a Signoff (no ack flag) overrode the gate,
  since `mutate()` already wrote the real HistoryEntry. `--decision-ref`'s
  help text across `apply-proposal`/`land`/`propose` now points to the typed
  `signoff` field as the preferred mechanism for a real judgment-call
  decision, keeping `--decision-ref` for lighter mechanical acknowledgments
  (task #334's traceability-completion fix is a worked example of the
  latter). A DRAFT `ProposedRequirement`
  (`proposals/task335-r4f-req-signoff/001-R-requirement-update-signoff-typed.json`)
  proposes making typed signoff MANDATORY for a real human decision on a
  Requirement UPDATE/Assumption rewrite — deliberately left unlanded,
  pending resolver review (adopting a new authoring obligation is a judgment
  call, not something the same task that built the capability should also
  mandate). Note: `hotam-spec-self`'s own domain declares only
  `ai-agent`/`domain-user`/`framework-author`/`framework-reviewer`
  Stakeholders — there is no Stakeholder entry for Marat Karamullin in THIS
  domain (unlike `PRAT-hotam`'s `gpsm-sm` domain, which has one) — so the
  first real use of typed signoff in `hotam-spec-self` itself needs either a
  new `ProposedStakeholder` or a resolver decision that `domain-user`/
  `framework-author` is the honest id to use.

### Fixed
- **`R-shared-projections-mode-independent` bound to its own regression test**
  (task #334, R4F-bind-test — fourth external review §4.3 synthesis): task
  #328 landed this requirement into `domains/hotam-spec-self/graph.json` as
  SETTLED, enforcement PROSE, with empty `implemented_by`/`verified_by`, even
  though a real regression test for exactly this property already existed
  and was named in the requirement's own claim evidence —
  `TestGenSpec_SharedProjectionsModeIndependent`
  (`cmd/hotam/gen_spec_test.go`) — making it +1 to the self-hosting domain's
  own "closeable debt" count. This claim is about GENERATOR/engine behavior
  (`cmd/hotam/gen_spec.go`, `internal/generator/traceability.go`+
  `coverage.go`), the same shape as the neighboring `R-empty-content-gen-notice`,
  which binds via `enforced_by` to a bare `Test*` name rather than the
  authored `implemented_by`+`verified_by` path (reserved for path-qualified
  references into a domain's own authored `spec/` tree, per
  `internal/invariants/authored_links.go`'s disjunctive
  `check_enforced_requires_enforcer_or_authored_link` gate — an
  engine-self-hosting requirement is not forced to fabricate one). Set
  `enforcement: ENFORCED` and `enforced_by: [TestGenSpec_SharedProjectionsModeIndependent]`
  only; `implemented_by`/`verified_by` correctly stay empty. Closeable debt
  dropped from 42 to 41 (175/253 → 176/253 SETTLED ENFORCED);
  `all-violations` stays at 0.
- **Authored-prose snapshot lint generalized to manifest goals/charter**
  (task #333, R4F-prose-lint — fourth external review §4.1 synthesis,
  extending task #331/R4-process-why): the "live numbers baked into durable
  authored prose" smell #331 fixed for `Process.Why`/`Step.Why` is a CLASS,
  not a one-field problem — confirmed live in the same sibling domain:
  `prat/gpsm-sm`'s manifest `goals` field, before task #329's rewording,
  baked in an identical "32/32 SIGNED"-shaped snapshot. Renamed
  `internal/invariants/process_why_snapshot.go` →
  `internal/invariants/authored_prose_snapshot.go`
  (`checkProcessWhySnapshotProse` → `checkAuthoredProseSnapshot`,
  `ProcessWhySnapshotWarnings` → `AuthoredProseSnapshotWarnings`,
  `check_process_why_snapshot_prose` → `check_authored_prose_snapshot`;
  `cmd/hotam/all_violations.go`'s `printAdvisorySection` wiring updated to
  the new name, same never-registered-in-`All`, non-blocking ADVISORY-band
  discipline). The check now ALSO scans manifest.json's `goals` (each list
  entry) and `charter` (a single string), resolved via
  `loader.ResolveDomainPresentation` — the same loader the
  DOMAIN-MAP/PROJECT-ESSENCE renderers already use — with the EXACT SAME two
  predicates #331 established (a snapshot-marker phrase co-occurring with an
  ISO date; or an "N из/of M" tally co-occurring with a domain-declared
  `gate_stage_order` token), no broadening of the pattern set. Verified
  read-only against both real sibling-repo consumer manifests: `gpsm-sm`'s
  CURRENT (post-#329) goals/charter text produces zero violations, and a
  reconstructed pre-#329-shaped goals sentence fires as expected; `prat`'s
  goals/charter also produce zero violations. Deliberately NOT extended to
  `Requirement.Claim`/`Conflict.Context`: a scan of `hotam-spec-self`'s own
  297-requirement graph found zero real fires of the two precise predicates
  there, but also found roughly a dozen claims pairing a bare digit with a
  status word in ordinary normative prose — evidence that register is
  noisier than why/goals/charter's narrower "narrate current standing"
  role, left as a future extension pending a dedicated design consult. A
  drafted (not landed) `ProposedRequirement`
  (`proposals/draft-R-authored-prose-no-live-tallies.json`) claims the
  class-wide discipline, pending human review per
  `R-decided-needs-human-signoff`/`R-ai-presents-not-decides`.
- **`gate_signoff_count` assert: explicit cohort denominator + multi-run
  guard** (task #330, R4-cohort — fourth external review): the
  `gate_signoff_count` orientation_faq assert kind
  (`internal/invariants/orientation_faq_assert.go`) computed
  `total = tally.Signed + tally.Deferred` — a Requirement with NO gate
  signoff record at all at a stage (never evaluated) was invisible to that
  sum, so `expect:"all"` could silently pass even though some requirement
  was never assessed. Confirmed live in `gpsm-sm`: 35 requirements total,
  only 32 carry any gate signoff at all (the other 3 legitimately
  out-of-cohort). Two-part fix. (1) New optional manifest-level
  `gate_cohort` declaration (`internal/loader/gate_cohort.go`,
  `ResolveGateCohort`, mirroring `ResolveGateStageOrder`'s exact
  loader-stays-lenient pattern) names WHICH Requirements form the
  denominator via `{"statuses": [...], "exclude": [...]}` (`Statuses`
  defaults to `["SETTLED"]` when declared-but-empty). New
  `graphfacts.CohortCount(g, member)` (`internal/graphfacts/facts.go`) is a
  trivial counted filter. When a domain declares `gate_cohort`,
  `evalOrientationAssert` uses `total = graphfacts.CohortCount(...)` instead
  of `Signed+Deferred` — fail-closed validation of the spec at CHECK time
  (an `exclude` id that matches no real Requirement, or a `statuses` entry
  that is not a recognized status, both fire a named violation, reusing
  `graphfacts.RequirementStatusTally`'s own exact-match + `OPEN`-prefix
  matching rule rather than inventing a new one). Absent `gate_cohort`,
  behavior is byte-identical to before this task. Also wires the
  previously-unused `OrientationFAQAssert.State` field: `""`/`"SIGNED"` (the
  default, byte-identical to before) reads `count=tally.Signed`,
  `"DEFERRED"` reads `count=tally.Deferred`; any other value fails closed.
  (2) Multi-pipeline-run guard: `GateSignoff.PipelineRun` was already
  mandatory/populated but `graphfacts` silently conflated every run when
  tallying. `lastSignoffAtStage`/`GateSignoffTally` now take a `run string`
  parameter (`""` = all runs = 100% backward-compatible default — every
  existing call site, `internal/generator/pipeline.go`'s Live-state
  renderer and `internal/generator/claudemd.go`'s `GateFrontier`-based
  DOMAIN-MAP renderer, keeps passing `""`, unchanged rendered output). New
  `graphfacts.PipelineRunsAtStage(g, order, stage)` returns the distinct
  `pipeline_run` values recorded at a stage. New
  `OrientationFAQAssert.PipelineRun` field
  (`internal/loader/orientation_faq.go`) lets an assert declare which run to
  tally; when a stage has signoffs from more than one distinct
  `pipeline_run` and the assert does not declare `PipelineRun`,
  `evalOrientationAssert` fails closed rather than silently conflating runs.
  A drafted (not landed) `ProposedRequirement`
  (`proposals/draft-R-gate-cohort-explicit-denominator.json`) claims this
  discipline as a graph-level requirement, pending human review per
  `R-decided-needs-human-signoff`/`R-ai-presents-not-decides`. Actually
  declaring `gate_cohort` in `gpsm-sm`'s own manifest.json (activating the
  stronger check there) is a separate follow-up, out of scope here.
- **PIPELINE.md generated "Live state" section + advisory Process-why
  snapshot lint** (task #331, R4-process-why — fourth external review): a
  domain's `Process.Why` (durable authored prose) can carry a stale
  point-in-time status claim that nothing ever re-derives — a real,
  confirmed instance was found in `prat/gpsm-sm`'s `Process.Why`, which
  literally read "27 из 32 ФТ... ТЕКУЩЕЕ ПОЛОЖЕНИЕ на 2026-07-21" while the
  graph had since moved to 32/32. Two-part fix. (1) `BuildPipeline`
  (`internal/generator/pipeline.go`) now takes a `gateOrder []string`
  parameter and renders a generated "Live state" section — one line per
  `gate_stage_order` stage via `graphfacts.GateSignoffTally` (honest "not
  started" beyond the `graphfacts.GateFrontier`), plus a
  `graphfacts.ConflictLifecycleTally` DECIDED/HELD/UNRESOLVED line when the
  graph carries any Conflicts — placed BEFORE the first `## Process` section
  (the "where are we now" before "how does this work" ordering,
  `R-domain-overview-projection`). Deliberately a PURE function of graph
  state only (no `today`/date parameter — dating this section would
  recreate the exact staleness smell being fixed, and would break
  `gen-spec`'s byte-reproducibility). Omitted entirely (not an empty
  placeholder) when the domain declares no `gate_stage_order` and carries no
  Conflicts — `hotam-spec-self`'s own PIPELINE.md gained only a Conflicts
  line (8 total: 8 DECIDED · 0 HELD · 0 UNRESOLVED; it has no declared
  `gate_stage_order`). `cmd/hotam/gen_spec.go` now resolves `gateOrder` via
  `loader.ResolveGateStageOrder` and threads it through. (2) A new narrow,
  ADVISORY-ONLY lint (`internal/invariants/process_why_snapshot.go`,
  `check_process_why_snapshot_prose`, exported as
  `ProcessWhySnapshotWarnings`) flags a `Process.Why`/`Step.Why` (never
  `Requirement.Why`) that either (a) co-occurs a fixed snapshot-marker
  phrase ("текущее положение" / "по состоянию на" / "as of" / "current
  status") with an ISO date, or (b) co-occurs an "N из/of M" tally with any
  stage token from the domain's OWN declared `gate_stage_order`. Never
  registered in the `All` invariant registry — mirrors
  `HonoredSkipWarnings`' identical never-blocking shape, wired into
  `cmd/hotam`'s non-blocking `ADVISORY` section
  (`all_violations.go:printAdvisorySection`) rather than
  `invariants.AllViolations`, so it can never block `hotam all-violations`'s
  exit code or `apply-proposal`'s gate. Verified against
  `hotam-spec-self`'s own 253-requirement graph: 0 false positives. Minor
  hardening: `ProposedProcess.mutate`'s Why-change branch
  (`internal/proposal/mutate.go`) now records an old→new abbreviated diff in
  the `HistoryEntry` (mirroring `ProposedAssumptionRewrite`'s audited-rewrite
  style) instead of a bare `"why updated"` flag. A drafted (not landed)
  `ProposedRequirement`
  (`proposals/draft-R-pipeline-live-state-from-typed-carriers.json`) claims
  this discipline as a graph-level requirement, pending human review per
  `R-decided-needs-human-signoff`/`R-ai-presents-not-decides`. The actual
  `gpsm-sm` `Process.Why` migration (rewriting the stale text in the sibling
  `PRAT-hotam` repo) is a separate follow-up task, out of scope here.
- **Live-graph-fact assertions for Orientation-FAQ entries** (task #321,
  R3-semantic-faq — external review): the Orientation-FAQ invariant
  (`check_orientation_faq_answered`) previously proved only that a declared
  keyword phrase is lexically PRESENT in the crystal, never that the phrase
  is still semantically TRUE relative to the graph's current state — this
  session hit exactly this bug (a manifest FAQ entry claimed "27 of 32
  requirements" and kept passing the keyword check long after the graph
  reached 32/32, fixed by hand in tasks #318/#322 without closing the
  underlying design gap). New package `internal/graphfacts`
  (`internal/graphfacts/facts.go`) adds four pure, LIVE graph-fact readers —
  `GateSignoffTally`/`GateFrontier` (extracted, not reimplemented, from the
  gate-tally logic `internal/generator/claudemd.go`'s DOMAIN-MAP renderer
  already computed inline — proven byte-identical before/after the
  extraction), `ConflictLifecycleTally`, `RequirementStatusTally`. Placed
  outside the pre-existing `internal/query` package deliberately: `query` is
  a PERIPHERY consumer per `internal/selfcheck/imports_test.go`'s
  `R-core-periphery-import-ratchet`, and `internal/invariants` (a consumer
  of these tallies) is CORE — a core package may never import a periphery
  package, so `graphfacts` sits in neither set, importable from both sides
  of that one-way arrow (caught by `TestCorePeriphery_ImportRatchet`
  itself on the first CI push of this change; fixed by relocating the
  package rather than weakening the ratchet). A new
  optional `assert` field on an `OrientationFAQEntry`
  (`internal/loader/orientation_faq.go`) ties an entry to one of these live
  tallies instead of, or ADDITIVELY alongside, the existing keyword/link
  signals: `expect` (`"all"` / `"none"` / `{"op":"gte"|"eq","value":N}`)
  and/or a `phrase` template (`{count}`/`{total}` placeholders,
  live-substituted, then required present in the crystal or linked file —
  closing the exact "27/32 stays lexically present forever" bug class). New
  `internal/invariants/orientation_faq_assert.go` (`evalOrientationAssert`)
  evaluates the assert, failing closed on an unrecognized `kind`, an
  undeclared gate `stage`, a malformed `expect`, or an assert declaring
  neither `expect` nor `phrase`. Fully backward-compatible: `Assert == nil`
  (every entry written before this field existed) behaves byte-identically
  to the pre-existing two-signal check. The self-hosting `hotam-spec-self`
  domain's own `orientation_faq` entries remain keyword/link-only — 0
  violations against `hotam all-violations --domain domains/hotam-spec-self`
  confirms the change is a pure additive capability for this domain;
  migrating consumer-domain manifests (`PRAT-hotam`) to use `assert` is a
  separate follow-up task. A drafted (not landed) `ProposedRequirement`
  UPDATE reflecting the new three-signal claim text lives at
  `proposals/draft-R-orientation-faq-answerable-assert.json`, pending human
  review per `R-decided-needs-human-signoff`/`R-ai-presents-not-decides`.
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
- **CI was silently red for 5+ days, then exposed three real concurrency
  bugs once fixed enough to run to completion** (task #317, R3-ci — external
  review): `.gitignore`'s blanket `vendor/` rule was accidentally excluding
  this project's OWN `internal/recorder/vendor/` package (introduced
  2026-07-17) from git — files existed on disk (so local `go build` always
  passed) but were never tracked, breaking CI's Build step on every push
  since introduction; fixed via a `!internal/recorder/vendor/` exception and
  tracking the previously-untracked files. Once Build passed, CI progressed
  far enough to expose, in turn: (1) two files not `gofmt`-formatted
  (`internal/gate/test_exec_test.go`, `internal/invariants/scenario_
  discipline_test.go`), never caught before because Build always failed
  first; (2) `TestCmdLand_AutoCrystal_RepoRootIsDomainDir` fixed to bootstrap
  via the minimal `initDomain()` scaffold instead of copying the production
  self-hosting graph — the real graph's `internal/...` symbol links depend
  on `internal/gate.engineRoot()`'s CWD-based `go.mod` fallback, which this
  test's own `chdirAndRestore` (needed to exercise `repoRootForDomain`'s
  tier-3 branch) breaks as an unavoidable side effect; (3) a genuine TOCTOU
  race in `internal/gate/compile_cache.go`'s compiled-test-binary cache —
  `invalidateCompileCacheForModule` used to `os.Remove` a stale binary file
  out from under a concurrent goroutine that had already `compileCache.Load`ed
  its path and was mid-`exec.Command` (triggered because `hotam land`'s
  proposal-apply gate hashes the module BEFORE writing graph.json/generated
  docs, then re-verifies AFTER — every verdict-cache entry from the pre-write
  hash mismatches post-write and independently fires invalidation); fixed by
  making invalidation map-only (never delete the file — an orphaned binary
  simply outlives its cache entry until process-exit cleanup) plus giving
  every compiled binary a unique, non-deterministic filename (a process-wide
  atomic counter) so a post-invalidation recompile can never overwrite a
  path a concurrent holder is still executing; (4) `TestRunVerifiedByTest
  Recording_TmpDirCleanedUpAfterReturn` was itself racy — it compared
  whole-`os.TempDir()` `hotam-record-*` directory counts before/after ONE
  call, which any concurrently-running sibling test in the same package
  (several run under `t.Parallel()`) could pollute; fixed by redirecting the
  test's own `TMPDIR`/`TMP`/`TEMP` to a private `t.TempDir()` so its
  before/after glob is isolated from concurrent siblings by construction.
  None of these four were reachable in CI until the ones before them were
  fixed — each was masked by an earlier-failing step for as long as 5 days.
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
