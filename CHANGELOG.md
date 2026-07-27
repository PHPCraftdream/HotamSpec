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

### Fixed
- **`-vet=off` added to `internal/gate/compile_cache.go`'s `doCompileTestBinary`** — `go test -c`
  runs `go vet` on the target package by default before compiling, a real repeated cost paid on
  every compile-cache miss across the dozens of `gate.RunVerifiedByTestRecording` call sites
  (`cmd/hotam` subprocess tests, RAC2/RAC3 machinery, `internal/generator`'s SPEC.md rendering).
  Does not weaken vetting of real repository code: `go vet ./...` already runs, separately, over
  the whole repo as its own top-level check every task/CI run already performs; the packages
  compiled here are either the target verified_by test's own (already-repo-vetted) package, or
  hand-written fixture-module stand-ins under a temp module root that were never part of the
  repository at all. Verified post-fix on an uncontended run: `internal/gate` 220-297s → 146s,
  `internal/generator` 279-330s → 167s, `internal/invariants` → 111s. Full `go test ./...` stays
  green (0 FAIL), `all-violations` stays 0 on both self-hosted domains, `go vet ./...` unaffected.
- **RAC3's new subprocess-spawning tests gated behind `-short`** — tasks #369/#370 added 15
  test functions (across `internal/selfspec/claim_derive_test.go`,
  `internal/selfspec/requirement_state_test.go`, `internal/invariants/claim_scenario_current_test.go`)
  that drive `gate.RunVerifiedByTestRecording` (a real `go build` + `go test` subprocess against
  a temp fixture module) with no `testing.Short()` gate — the same class of cost `cmd/hotam`'s 48
  subprocess tests already gate, but these three files were missed at RAC3 landing time. Measured
  before the fix: the `internal/selfspec` subset alone cost ~27.7s, the `internal/invariants`
  subset ~17.5s — a meaningful fraction of why the full suite grew from the user's remembered
  "~1 minute" baseline. Each of the 15 gated functions was individually traced (not assumed) to
  confirm it actually reaches the subprocess call past every scope/discipline early-return; tests
  that don't reach it (e.g. `TestDeriveClaimsFromScenarios_NonFullDisciplineIsNoOp`,
  `TestRequirementState_NoCarrier`) were deliberately left un-gated as honestly cheap already.
  Verified post-fix: `internal/selfspec -short` dropped from ~27.7s to 0.7s; the 4 gated
  `TestCheckClaimMatchesScenario_*` tests in isolation dropped from ~17.5s to 0.066s. Full
  `go test ./...` stays green (0 FAIL), `all-violations` stays 0 on both self-hosted domains.

### Investigated (task #368, no code change)
- **Test suite wall-clock time** — measured, not guessed, on an idle machine (16 cores,
  GOCACHE=`D:\system_artefact\go-build`), both hypotheses this task set out to test came back
  negative, so no refactor was made:
  - **`cmd/hotam -short` vs full**: `-short` (skips the 48 real-subprocess tests) = 349.1s,
    full = 449.6s — only ~22% savings, below the ~30% threshold this task set as the bar for
    a structural in-process-conversion refactor (`cmd/hotam/vendor_ontology.go`'s
    `cmdX(args)`/`x(domainDir)` split pattern). Below that bar, refactoring 48 subprocess
    tests carries real regression risk (weakening what they assert) for a modest win — not
    done.
  - **`-p 4` on `cmd/hotam`**: 460.7s — statistically the same as the `-p 16` default
    (`-p` controls cross-PACKAGE build/test parallelism; `cmd/hotam` is a single package, so
    `-p` has no effect on it at all, as expected once this was checked directly).
  - **`-parallel 4` on `cmd/hotam`**: 810.6s — **80% SLOWER** than the `-p`/`-parallel`-default
    baseline (449.6-460.7s). This directly REFUTES the standing hypothesis (carried over from
    task #366's own `TestCmdSyncSelf_RollbackOnPostWriteFailure` CPU-contention symptom) that
    reducing intra-package test parallelism would relieve subprocess-vs-subprocess CPU
    oversubscription. It does the opposite: `cmd/hotam`'s subprocess tests are I/O/exec-wait
    heavy, and cutting parallelism just serializes that waiting. **Do not pass `-parallel <N>`
    below the default when running this suite.**
  - **Root cause of the earlier apparent contention symptoms** (task #366's flake, and this
    resolver's own two false-timeout signals earlier this session) was simpler and already
    identified independently: **running more than one `go test ./...` invocation
    concurrently** in the same session/machine, not oversubscription *within* a single
    `go test` run. The fix is process discipline (one full-suite run at a time — already
    adopted this session, see task #369's commit message for the fuller incident writeup),
    not a test-suite code or flag change.
  - Windows Defender exclusion (GOCACHE + repo + temp dir) remains a plausible, unverified
    lever — requires administrator privileges this task could not exercise; left as a
    standing recommendation for the resolver/user to apply manually, not evaluated here.

### Added
- **Claim derived from a Requirement's `verified_by` scenario test(s) (task #369, RAC3-A)**:
  a Requirement's `Claim` — the normative claim text — is no longer only a hand-authored
  string; for a `discipline:"full"` domain, it is now DERIVED from the actually-executed
  `verified_by` test(s)' recorded `hotamspec.NewScenario(t, id, description)` description(s),
  using the exact same real-`go test`-execution machinery `hotam gen-spec --spec` already uses
  to render `docs/gen/SPEC.md` (`gate.RunVerifiedByTestRecording`).
  - **`internal/selfspec/claim_derive.go`** (new): `DeriveClaimsFromScenarios(reg, specRoot,
    selfHosting, discipline)` mutates a `*registry.Registry[ontology.Requirement]` in place
    (via `registry.Registry.Update`): for every registered Requirement in scope
    (`RequirementInClaimDerivationScope` — `Enforceability != INHERENTLY_PROSE` AND at least
    one `verified_by` entry), `Claim` is replaced with the CONCATENATION of every `verified_by`
    test's recorded scenario title, in `verified_by`'s own declared order, joined by a single
    space (`ClaimDerivationSeparator`) — resolver-settled design (confirmed against the real
    `PRAT-hotam/domains/prat/spec/model/brd_package_test.go` pilot, whose
    `R-brd-integrity-zero-blockers` requirement carries two `verified_by` tests, each
    constructing its own `hotamspec.NewScenario`). A single `verified_by` entry is the
    trivial n=1 case of the same rule, not a separate code path. Gated a pure domain-level
    honest no-op when `discipline != loader.DisciplineFull` — every domain in this wave
    (`hotam-spec-self`, `hotam-dev`, neither `discipline:full`) sees zero behavior change.
  - **Integration point: `hotam sync-domain` only** (`cmd/hotam/sync_domain.go`) — not
    `gen-spec --spec`. `Claim` is a graph-resident structural field the sync-domain/sync-self
    machinery already replaces wholesale from a domain's Go-authored registry
    (`internal/selfspec.MergeIntoGraph`/`SyncGraph`); `gen-spec --spec`'s `BuildSpecFromRows`
    reads `Claim` verbatim from the graph (PLAN-scenario-generated-spec.md §2 D2: "claim
    остаётся коротким авторским intent") and must keep doing so unmodified. `sync-domain` now
    calls `selfspec.DeriveClaimsFromScenarios` on the freshly subprocess-dumped registry
    BEFORE `SyncGraph` ever sees it, so a derived `Claim` flows through
    `StructuralFieldDiffs`/`SyncGraph`/the confront gate exactly like any other authored
    structural field, with zero special-casing downstream.
  - **`internal/invariants/claim_scenario_current.go`** (new): `check_claim_matches_scenario`,
    the drift-detection sibling of `check_settled_requires_scenario` — same
    `discipline:"full"`-gated honest no-op, same `INHERENTLY_PROSE` exemption — proving a
    Requirement's committed `Claim` still matches a fresh re-derivation from its `verified_by`
    test(s)' CURRENTLY recorded scenario description(s) right now. Modeled on
    `check_spec_md_current`'s form (a full re-render/re-execution comparison, not a cheap hash
    compare, since Claim derivation is not a pure function of source text alone) and marked
    `ComparesOnDiskProjection: true` for the identical reason `check_spec_md_current` carries
    it — excluded from `AllViolationsForProposalGate`'s pre/post-mutation diff gate, since
    `sync-domain` itself already derives and writes the current `Claim` through this exact
    machinery immediately before that gate runs. Registered invariant count 114 → 115
    (`internal/invariants/registry_complete_test.go` updated).
  - **Tests**: `internal/selfspec/claim_derive_test.go` (single verified_by entry, multi-entry
    concatenation in declared order — including a reversed-order variant proving declared
    order, not some other ordering, drives the join — `discipline:full` gate, `INHERENTLY_PROSE`
    exemption, no-`verified_by` no-op, a plain non-narrating entry contributing nothing without
    aborting derivation, idempotent re-run reporting zero changes); `internal/invariants/
    claim_scenario_current_test.go` (no-op without `discipline:full`, green when `Claim` already
    matches, red when it diverges, `INHERENTLY_PROSE` exemption, no-`verified_by` skip, a
    MUTATION probe proving genuine drift detection — clean, then red after editing the
    underlying test's scenario title without re-deriving, then clean again after re-deriving —
    and a cross-check proving the check's own re-derivation logic never disagrees with
    `internal/selfspec.DeriveClaimsFromScenarios`); `cmd/hotam/
    sync_domain_claim_derive_test.go` (`TestCmdSyncDomain_ClaimDerivedFromScenarioEndToEnd`, a
    full real subprocess-driven end-to-end proof that `hotam sync-domain` itself — not merely
    the derivation function in isolation — overrides a deliberately stale registry-literal
    `Claim` with the real derived scenario text before landing, with `0` violations afterward).
- **`Requirement.State()` — a computed, execution-based proof lifecycle (task #370, RAC3-B)**:
  the second half of the "is a Requirement's Claim real?" wave #369/RAC3-A started. A
  Requirement's proof status — is `verified_by` even declared, has it ever been run, did the
  last run pass, and does the committed `Claim` still match a fresh re-derivation — is now a
  single computed value instead of something every consumer would otherwise have to re-derive
  ad hoc.
  - **`internal/ontology/lifecycle.go`**: new `RequirementProofLifecycle` (`Slug:
    "requirement-proof"`), a canonical `Lifecycle` value in the same family as
    `RequirementStatusLifecycle`/`ConflictLifecycle` — 5 states (`NO_CARRIER` → `UNVERIFIED` →
    `FAILING`/`STALE`/`PROVEN`, `Cyclic: true`, since a passing/fresh requirement can regress the
    moment its scenario text changes without a re-sync or its test starts failing again).
    Registered into `checkCanonicalLifecyclesWellformed`'s canonical list
    (`internal/invariants/lifecycle_checks.go`) alongside the other four, so the framework's own
    self-application check validates it too. Deliberately a SEPARATE Lifecycle value from
    `RequirementStatusLifecycle` — that one governs the editorial DRAFT/SETTLED/OPEN/REJECTED
    workflow (resolver-decided); this one governs a mechanically computed verdict over the same
    requirement's `verified_by` carrier (never resolver-decided, never stored on the node,
    re-derived fresh on every read).
  - **`internal/selfspec/requirement_state.go`** (new): `RequirementState(r, specRoot,
    selfHosting)` computes the state by actually re-running every `verified_by` entry — the
    same `gate.RunVerifiedByTestRecording` + drift-comparison machinery
    `check_claim_matches_scenario`/`DeriveClaimsFromScenarios` already use. Lives in
    `internal/selfspec` (not as an `ontology.Requirement` method) for the same import-cycle
    reason `claim_derive.go`'s functions do: `internal/gate` already imports `internal/ontology`,
    so `ontology` cannot import `gate` back. NOT gated by `Enforceability` (unlike
    `RequirementInClaimDerivationScope`) — an `INHERENTLY_PROSE` requirement with a real
    `verified_by` entry still has a real, checkable proof state; `State()` answers "is the
    evidence that exists actually current and passing", not "is this requirement in
    Claim-derivation scope".
  - **Audit finding — no generator duplicates this predicate (by design, not an oversight)**:
    every `internal/generator` doc (`TRACEABILITY.md`, `COVERAGE.md`, `UNENFORCED.md`,
    `REQUIREMENTS.md`, `AGENT-CONTEXT.md`'s constitution index) was audited against this new
    function. None duplicate it: task #317 already, deliberately, removed a real-execution
    "verdict overlay" from `TRACEABILITY.md`/`COVERAGE.md` specifically to keep them pure,
    mode-independent functions of the graph plus a cheap AST scan (proven by
    `internal/generator/scenario_traceability_test.go`'s `TestBuild{Traceability,Coverage}_ModeIndependent_...`
    tests) — `hotam land`'s routine, non-`--spec` regeneration must never flip their content.
    Migrating them to `RequirementState()` (which spawns real `go test` subprocesses) would
    reintroduce exactly the instability #317 fixed, not perform a safe refactor. `docs/gen/` was
    regenerated for both self-hosted domains (`hotam-spec-self`, `hotam-dev`) and diffed
    byte-for-byte against the pre-task tree to confirm zero incidental output drift from this
    change.
  - **Fixed a staleness bug found in passing while auditing #369's `spec.go`/`spec_build.go`**:
    `gate.ScenarioVerdict`/`ScenarioVerdictsFromRows` (and their `generator.ScenarioVerdict`
    re-export) were leftover dead code from BEFORE task #317's fix — their doc comments still
    claimed `BuildTraceability`/`BuildCoverage` render an optional "verdict" sub-column from
    them, but neither function has accepted a verdicts parameter since #317 removed that overlay
    (confirmed via `grep`: the type is now referenced only by its own definition, the `spec.go`
    re-export, and a test that calls it solely to prove it has zero effect on generator output).
    Corrected both doc comments (`internal/gate/spec_build.go`, `internal/generator/spec.go`) to
    describe the current, accurate wiring and point at `RequirementState` as the current home
    for a real per-requirement proof verdict; left the exported symbols themselves in place
    (still used by `internal/generator/scenario_traceability_test.go`'s mode-independence
    proof), out of scope for this task to remove.
  - **Tests**: `internal/selfspec/requirement_state_test.go` — one test per
    `RequirementProofLifecycle` state (`NO_CARRIER`, `UNVERIFIED` via both an unresolvable
    module path and a malformed `file:symbol` entry, `FAILING` via both a genuine test failure
    and a compile failure, `PROVEN` with and without a narrated scenario, `STALE` on Claim
    drift, a multi-`verified_by`-entry case proving ALL entries must pass), plus a single
    end-to-end MUTATION test walking one fixture requirement through every transition
    (`NO_CARRIER` → `UNVERIFIED` → `FAILING` → `STALE` → `PROVEN`) by mutating its
    `verified_by`/`Claim`/on-disk test body between reads, mirroring
    `claim_scenario_current_test.go`'s own mutation-probe shape for the drift half alone.
- **Generalized the self-hosting Requirement/Rejection lock to any consumer domain
  (`requirements_authority: "code"`, task #367, RAC2 Phase C)**: the final step of
  "Go-code-only authority" for consumer-domain requirements (#365 shipped vendoring
  infrastructure, #366 shipped `hotam sync-domain`). `internal/proposal/apply.go`'s
  `applyToGraph` self-hosting lock (task #350/RAC-B3, previously gated only on
  `g.SelfHosting` — hotam-spec-self's own 27-thematic-file registry) now ALSO refuses a
  hand-authored `ProposedRequirement`/`ProposedRejection` when a domain's `manifest.json`
  declares `"requirements_authority": "code"`, via a new `internal/ontology.Graph.RequirementsAuthorityCode`
  field (populated by `internal/loader.resolveRequirementsAuthorityCode`, mirroring
  `resolveSelfHosting`'s exact tolerant-default pattern). Unlike the self-hosting lock's
  per-ID `selfspec.SourceFileFor` file lookup (needed because hotam-spec-self's registry is
  split across 27 `requirements_<topic>.go` files), the new
  `errRequirementsAuthorityCodeRequirementLocked`/`errRequirementsAuthorityCodeRejectionLocked`
  errors name a single fixed path, `spec/requirements.go`, unconditionally — a consumer
  domain adopting this path keeps its whole registry in ONE file. Both locks now run
  independently in `applyToGraph`; a domain could in principle set both flags. New permanent
  regression test `internal/proposal/requirements_authority_lock_test.go` mirrors
  `self_hosting_lock_test.go`'s coverage shape (CREATE/UPDATE refused, Rejection's `replaces`
  reminder, other proposal kinds unaffected, unset-flag negative control) and proves the lock
  refuses a real `apply-proposal` invocation with the graph left untouched on disk.
  - **`internal/loader/manifest.go`**: added `DomainManifest.RequirementsAuthority` (the typed
    projection of the new manifest field, mirroring `Discipline`'s `"full"`-is-the-only-
    recognized-value pattern).
  - **Root CLAUDE.md founding-canvas step 6 updated** (`internal/generator/claudemd_static.go`'s
    `mediationLoopFoundingCanvasStep6MiddlePath`): the "middle path" paragraph no longer
    disclaims a consumer `sync-self`-equivalent as "a distinct, NOT-YET-IMPLEMENTED future
    feature" — RAC2 (#365/#366/#367) built it. The paragraph now names `hotam vendor-ontology`
    + `hotam sync-domain` + the `requirements_authority: "code"` manifest opt-in directly.
    `TestRenderMediationLoopBlock_FoundingCanvasStep6MiddlePath` (generator package) updated to
    assert the real commands are named and the stale disclaimer is gone.
  - **Fixed a gap left by #365/#366**: `hotam vendor-ontology`, `hotam scaffold-registrydump`,
    and `hotam sync-domain` were real, working `cmd/hotam` subcommands but had never been
    registered in `internal/methodology/tools_data.go` (the tool registry every generated doc,
    `README.md`'s command count, and the root crystal's Tool-reference line project from) or
    wired into `cmd/hotam/tool_wiring.go`'s `Run` dispatch table. Registered all three
    (Implemented count 18 → 21, total registry 45 → 48) and wired their `Run` functions;
    regenerated the golden byte-identity fixtures that embed the tool listing
    (`internal/generator/testdata/fixture/{REQUIREMENTS,REPO-MAP,tools-INDEX,FRAMEWORK-INVARIANTS}.md`)
    and updated `README.md`'s CLI command list + count sentence to match.
  - **Scope note**: this task's brief also called for piloting the migration on a real
    business-empty domain (`domains/life`, a separate repo). That pilot was started, then
    reverted mid-task by the resolver: a business-empty domain (0 Requirements) should not
    carry vendored `spec/` scaffolding before it has a real Requirement to justify it — see
    task #364's "empty domain generates 0 `docs/gen/` files" precedent, which the pilot's
    vendored `spec/hotamontology` broke (the generator mistook it for an authored `spec/model`
    and started writing `MODELS.md`). The lock generalization and CLAUDE.md text update above
    are engine-only and unaffected; the pilot itself did not land — `domains/life` was left
    with its working tree unchanged from before this task (nothing there was committed).
- **`hotam sync-domain` + `hotam scaffold-registrydump` — Go-code-only authority for
  CONSUMER-domain Requirements (task #366, RAC2 Phase B)**: the second step of "Go-code-only
  authority" for consumer-domain requirements (task #365 shipped the vendoring infrastructure
  only). `hotam sync-domain` generalizes `hotam sync-self` from the engine's own self-hosting
  registry (`internal/selfspec.Requirements`) to ANY domain that authors its Requirement
  literals in its own `spec/requirements.go`, importing the vendored `spec/hotamontology`
  mirror — same dry-run-by-default / `--confirm-hash` handshake, same gate order (7 confront ->
  8 pre/post-violation-diff -> 9 append-only), same transactional snapshot/rollback machinery
  (`land.go`'s `snapshotGraphFiles`/`rollbackLand`) as `sync-self`.
  - **`internal/selfspec/merge.go` + `sync.go`**: `MergeIntoGraph`/`SyncGraph` now take an
    explicit `*registry.Registry[ontology.Requirement]` parameter instead of reading the
    package-global `selfspec.Requirements` directly — a small, mechanical, behavior-preserving
    refactor (`hotam sync-self` passes `selfspec.Requirements` explicitly as a thin wrapper;
    proven byte-identical before/after via the full `sync-self` test suite).
  - **`cmd/hotam/sync_domain.go`** (`hotam sync-domain --domain <path>`): the one structural
    difference from `sync-self` — instead of a package-global Go registry embedded at the
    engine's own build time, sync-domain subprocess-execs `go run ./registrydump` INSIDE the
    domain's own `spec/` Go module (the same module-boundary-crossing principle
    `internal/gate/test_exec.go` already uses to run a domain's `verified_by` tests), captures
    its stdout as a JSON `[]ontology.Requirement` array, and builds an in-process registry from
    it before handing it to the same `MergeIntoGraph`/`SyncGraph` sync-self itself calls. Every
    subprocess failure mode (missing `spec/`, missing `registrydump/`, non-zero exit, invalid
    JSON, duplicate ID) surfaces as a specific, actionable error — never a silently empty
    registry that would look like "nothing to sync". Does NOT port sync-self's stale-binary
    guard (`selfspec.SourceFiles`/`SourceFileFor`): that guard exists only because `go:embed`
    freezes the engine's OWN source into the compiled `hotam` binary at build time; sync-domain
    always re-executes `go run` fresh, so the "stale embedded binary" failure class does not
    exist for this command.
  - **`cmd/hotam/scaffold_registrydump.go`** (`hotam scaffold-registrydump --domain <path>`):
    writes `<domainDir>/spec/registrydump/main.go`, a minimal generated program importing the
    domain's own spec/ module root package (expected to declare
    `var Requirements = hotamontology.New[hotamontology.Requirement]()`) plus the vendored
    `spec/hotamontology`, and prints `json.Marshal(Requirements.All())` to stdout — the
    domain-side half of the module-boundary bridge. Modeled on `hotam vendor-ontology`: requires
    `spec/go.mod` AND an already-vendored `spec/hotamontology` to exist first; idempotent.

- **`hotam vendor-ontology` + `check_ontology_vendor_current` — minimal Requirement/Registry
  vendoring infrastructure for consumer-domain Go-code-authored requirements (task #365, RAC2
  Phase A, "Go-code-only authority" for consumer-domain requirements)**: the first step toward
  letting a consumer domain author its Requirement identity (id/claim/owner/status/relations) in
  Go code, the same way `hotam-spec-self` already does via `internal/selfspec/requirements_*.go`
  + `hotam sync-self`. Modeled 1-in-1 on the existing `hotam vendor-recorder` /
  `check_recorder_current` mechanism.
  - **`internal/ontology/canon`** (new package `hotamontology`): the canonical, minimal,
    JSON-tag-identical mirror of `ontology.Requirement`'s STRUCTURAL fields only (`ID, Claim,
    Owner, Status, Relations, Assumptions, Enforcement, EnforcedBy, Enforceability, MTag, Summary,
    CreatedAt, SettledAt, BlockedOn, ImplementedBy, VerifiedBy, SourceRefs, DeclOrder`) — the
    EVENT fields (`History`, `GateSignoffs`, `LastReviewedAt`, `ReviewAfter`, `Evidence`) are
    deliberately excluded, mirroring `internal/selfspec/merge.go`'s own structural/event split.
    Plus a generic `Registry[T]` mirror (`New`/`MustRegister`/`All`/`Get`), copied unchanged from
    `internal/registry.Registry[T]`. Pure data/mechanism, no behavior — designed to be vendored
    (copied by file, never imported via go.mod) into a domain's separate `spec/` Go module.
  - **`internal/ontology/vendor`**: banner-stamping/stripping logic for the two canonical files
    (`requirement.go`, `registry.go`), mirroring `internal/recorder/vendor`'s identical shape.
  - **`cmd/hotam/vendor_ontology.go`** (`hotam vendor-ontology --domain <path>`): writes both
    banner-stamped vendored files to `<domainDir>/spec/hotamontology/`, modeled directly on
    `hotam vendor-recorder`. Requires `<domainDir>/spec/go.mod` to already exist; idempotent.
  - **`internal/invariants/ontology_vendor_check.go`** (`check_ontology_vendor_current`):
    sha256-compares each vendored file (if present) against the engine's own canon, post-banner —
    an honest no-op for a domain that has never vendored the mirror, checked independently per
    file. Modeled directly on `check_recorder_current`.
  - **`internal/selfspec/requirements_authoredspec.go`**: added `R-vendored-ontology-matches-
    engine-canon`, the self-hosting anchor for `check_ontology_vendor_current` (mirrors
    `R-vendored-recorder-matches-engine-canon`'s identical role for `check_recorder_current`),
    landed via `hotam sync-self`.
  - Scope note: infrastructure only — does NOT implement `hotam sync-domain` (projecting a
    domain's own Requirement registry onto its `graph.json`) and does NOT touch
    `internal/proposal/apply.go`'s self-hosting lock; those are separate follow-up tasks
    (#366, #367).

- **Empty domains produce zero `docs/gen/` files; `hotam init` no longer auto-seeds
  (task #364)**: a business-empty domain graph (0 axes/stakeholders/requirements/
  conflicts/assumptions/operators/processes/goals/entity_types/entities) now
  produces NO generated files at all — completing the conditional-write pattern
  task #361 started for `TENSIONS.md`/`PIPELINE.md`/`MODELS.md` (and the
  longer-standing `DECISIONS.md`/`ENTITIES.md` pattern) across the remaining
  `docs/gen/*.md` set.
  - **`cmd/hotam/init_cmd.go`** (`initDomain`): removed the auto-seeded
    Stakeholder (`"owner"`) + SETTLED Requirement (`"R-domain-exists"`) a fresh
    `hotam init`/`hotam init-project` used to write. A freshly-scaffolded domain
    is now genuinely empty (`ontology.Graph{}`) — safe by construction, since an
    empty graph already passes every structural invariant
    (`R-empty-content-wellformed`).
  - **`internal/generator`**: eleven new `*MDHasContent(g) bool` predicates —
    `RequirementsMDHasContent`, `OpenMDHasContent`, `UnenforcedMDHasContent`,
    `FrameworkInvariantsMDHasContent`, `HistoryMDHasContent`,
    `ConstitutionMDHasContent`, `TraceabilityMDHasContent`, `CoverageMDHasContent`,
    `RepoMapMDHasContent`, `AgentContextMDHasContent`, `LiveStateMDHasContent` —
    plus `GraphJSONHasContent` for the `docs/gen/graph.json` archival copy. Each
    reduces to `!g.IsEmpty()`, mirroring the existing `EmptyNotice`/`g.IsEmpty()`
    fallback every one of these files' own `Build*` function already used.
  - **`cmd/hotam/gen_spec.go`**: all twelve are now write-set-conditional (same
    pattern as `DECISIONS.md`/`ENTITIES.md`/`TENSIONS.md`/`PIPELINE.md`/
    `MODELS.md`) — `REQUIREMENTS.md`/`OPEN.md`/`UNENFORCED.md`/
    `FRAMEWORK-INVARIANTS.md`/`HISTORY.md`/`CONSTITUTION.md`/`TRACEABILITY.md`/
    `COVERAGE.md`/`REPO-MAP.md`/`AGENT-CONTEXT.md`/`live-state.md`/`graph.json`.
    `repoMapDocs`/`mdDocs` construction moved off fixed-position slice indexing
    (no longer valid once every entry's PRESENCE, not just its content, varies)
    onto named `*MD` string variables gated by their own `*Written` bool. The
    root/local crystal (`CLAUDE.md`/`AGENTS.md`/`GEMINI.md`) and the
    project-shared `framework/` output (`GLOSSARY.md`, `tools/*.md`/`INDEX.md`)
    are UNAFFECTED — both are written unconditionally, independent of domain
    content, so a `hotam init-project` on an empty domain still gets a working
    crystal.
  - **`internal/selfspec/requirements_empty.go`**: `R-empty-content-gen-notice`'s
    `Claim`/`Why` revised (via `hotam sync-self`, not a hand-edit) from "emit a
    'no content yet' notice into `docs/gen/*.md`" to "write ZERO files under
    `docs/gen/`" — the requirement's own text now matches the new behavior
    instead of describing the retired one.
  - `internal/generator/docs_gen_ownership_test.go`, `cmd/hotam/gen_spec_test.go`
    (`TestGenSpec_MissingGraphRendersCalmNotice` updated for the new zero-file
    behavior; `TestGenSpec_SharedProjectionsModeIndependent` restored — it was
    referenced by `R-shared-projections-mode-independent`'s `enforced_by` but had
    gone missing from the working tree), and every `cmd/hotam` test fixture that
    relied on `initDomain`'s retired auto-seed (`semantic_gate_test.go`,
    `land_test.go`, `provenance_gate_test.go`, `propose_test.go`,
    `gen_spec_profile_test.go`, `init_project_test.go`,
    `init_project_e2e_test.go`) updated for the new empty-by-default scaffold —
    new shared test helpers `seedOwnerStakeholder`/`seedPlaceholderRequirement`/
    `seedMinimalRequirement` in `cmd/hotam/common_test.go` reconstruct the old
    seed's shape where a test's actual subject needs non-empty domain content.
  - Unrelated, mechanical fixes needed to get `go vet ./cmd/hotam/...` clean
    (both pre-existing gaps in the working tree, not introduced by this task):
    `chdirAndRestore` test helper (used by `land_test.go`, only ever defined in
    `internal/paths`) added locally to `cmd/hotam/common_test.go`.
  - **Follow-up regression fix**: the first landing of this task left 8 real
    `go test ./...` failures, caught by independent verification (not by the
    task's own report). `cmd/hotam/killswitch_e2e_test.go`'s
    `killswitchFixtureDomain` (shared by all 6 `TestKillswitch_*` e2e tests)
    called bare `hotam init` and then landed a Requirement with
    `"owner": "owner"`, relying on the auto-seeded Stakeholder `initDomain`
    used to write — now removed. Fixed by seeding a real `Stakeholder "owner"`
    via an explicit `apply-proposal` step inside the fixture (mirrors
    `TestExternal_FullLifecycle`'s pattern), not by hand-editing `graph.json`.
    Separately, `cmd/hotam/claudemd_links_test.go`'s
    `TestConsumerProfile_NoFrameworkSourceReferences`/
    `TestConsumerProfile_DocsGenNoFrameworkSourceReferences` hard-assumed
    `REPO-MAP.md`/`CONSTITUTION.md` always exist on disk; both now treat
    `os.IsNotExist` on those paths as the vacuous-true case (no file, nothing
    to check) instead of failing. Full `go test ./... -timeout 30m`: 19/19
    packages `ok`, 0 FAIL.
- **Conditional docs/gen/ files for young domains (task #361)**: `TENSIONS.md`,
  `PIPELINE.md`, and `MODELS.md` are now withheld from `docs/gen/` when the domain
  has no content for them — exactly the conditional-write pattern `DECISIONS.md`/
  `ENTITIES.md` already use. A freshly-scaffolded domain (no conflicts/axes, no
  Process nodes, no authored `spec/model/*.go`) gets an honest `_(not written: …)_`
  line in `REPO-MAP.md` instead of a pure-template file with zero facts.
  - `generator.TensionsMDHasContent(g)` (conflict OR axis nodes),
    `generator.PipelineMDHasContent(g)` (process nodes),
    `generator.ModelsMDHasContent(g)` (the same `gate.ScanAuthoredModels` scan
    `BuildModels` uses) gate the three files.
  - **Fix A** (`internal/generator/constitution.go`): sections 3 ("hard boundary"),
    4 ("super-rules"), and 7 ("methodology's laws") no longer render empty for a
    domain that has content but lacks the constitution-set requirement IDs. Each
    section now shows an honest `_No … SETTLED in this domain's graph yet._`
    placeholder, and section 7's table suppresses empty category rows entirely.
  - **`cmd/hotam/gen_spec.go`**: the three files are removed from the unconditional
    `repoMapDocs`/`mdDocs` slices and appended conditionally (same pattern as
    DECISIONS/ENTITIES). `BuildRepoMap` receives three new bool params for the
    `_(not written: …)_ ` listing lines.
  - **`internal/generator/agentcontext.go`**: `renderAgentContextDocsGenIndex`
    and the "Details on demand" section omit the `TENSIONS.md` reference when the
    domain has no conflict/axis nodes.
  - **`internal/generator/claudemd.go`**: the mediation-loop text's
    `spec/docs/gen/PIPELINE.md` and `spec/docs/gen/TENSIONS.md` sentinels are
    reduced to bare file names (no path prefix) when the files aren't written,
    so the crystal never claims a `docs/gen/` path that doesn't exist on disk.
  - `docs_gen_ownership_test.go` (`genTopLevelOwned` + `ownedGenRelPaths`) and
    `gen_spec_profile_test.go` (file-count pins) updated for the new conditional set.
- **Framework-self-documentation split: project-shared vs per-domain (tasks #355→#357)**:
  engine content that is byte-identical across all domains (`tools/*.md` +
  `tools/INDEX.md` — the `hotam` CLI command reference, 46 files; and `GLOSSARY.md`
  — the methodology controlled vocabulary) is promoted to a SINGLE project-root
  `framework/` directory (sibling of `domains/`, NOT inside any one domain). Every
  domain's `gen-spec` writes the same bytes to the same paths (idempotent
  overwrite), so a multi-domain project carries exactly ONE copy. `GLOSSARY.md`
  dropped its per-domain `reader:` header line to achieve true byte-identity
  regardless of which domain regenerated it (mirroring `tools/*.md`, which never
  carried one).
  - `FRAMEWORK-INVARIANTS.md`, by contrast, is PER-DOMAIN (which framework-plumbing
    SETTLED atoms THIS domain carries — it grows with the domain's own content,
    like REQUIREMENTS.md). Task #355 had misclassified it as project-shared and
    moved it to the per-domain `framework/`; #357 returns it to `docs/gen/` where
    it belongs (and where the entity-projection invariant's own rule text already
    referenced `domains/<name>/docs/gen/FRAMEWORK-INVARIANTS.md`).
  - **`cmd/hotam/gen_spec.go`**: `projectFrameworkDir := filepath.Join(repoRoot,
    "framework")` routes `GLOSSARY.md` + `tools/*.md` + `INDEX.md` to the project
    root; `FRAMEWORK-INVARIANTS.md` writes back through `mdDocs` into `docs/gen/`.
    `cleanupStaleProjectFrameworkFiles` (closed list: GLOSSARY.md + tools/*.md)
    and `cleanupStaleDomainFrameworkDir` (removes the retired per-domain
    `framework/` dir + its empty `tools/` subdir) replace the old single
    `cleanupStaleFrameworkFiles`.
  - **`internal/generator/repomap.go`**: "Framework reference (project-shared)"
    section now lists project-root `framework/` files with repo-root-relative
    paths; `FRAMEWORK-INVARIANTS.md` is back in the "Generated docs" listing.
  - **Cross-reference path pointers** updated: `claudemd.go` (EMBEDDED-TOOLS:
    `spec/framework/` → bare `framework/`), `requirements.go` (consumer closing:
    `framework/tools/INDEX.md`), `agentcontext.go` (file index: FRAMEWORK-
    INVARIANTS.md → docs/gen/, tools + GLOSSARY → project framework/),
    `claudemd_constitutionindex.go` (invariantsPath → docs/gen/).
  - New/updated tests: `TestProjectOwnsFramework_NoForeignOrOrphanFiles`
    (project-level ownership), profile-count tests, link-existence tests, and
    byte-identity fixtures updated for the new paths. Full `go test ./...` green.
- **`hotam init-project --discipline full|""` flag (task #353)**: init-project's
  BORN FULLY OBLIGATED scaffold (unconditional `discipline: "full"`, a vendored
  `spec/hotamspec/hotamspec.go` recorder, `spec/go.mod`, and a rendered
  `docs/gen/SPEC.md` — see task #273/W6.2) is now an explicit `--discipline`
  flag instead of hardcoded behavior. Default (`full`, or the flag omitted)
  reproduces the exact prior behavior byte-for-byte — the born-obligated e2e
  proof (`TestExternal_InitProjectBornObligated`) is unchanged and still green.
  `--discipline ""` is a new opt-out: no `discipline` key in `manifest.json`,
  no `spec/` tree scaffolded at all, no `SPEC.md` rendered — matching a bare
  `hotam init` domain exactly. Motivated by dogfooding a brand-new external
  business domain (`life`, outside this repo) that wanted zero framework code
  sitting in it before any real content existed: `discipline: full`'s value is
  the scenario-narration upgrade (tests double as `SPEC.md` source via
  `hotamspec.NewScenario`), not a precondition for writing real
  `implemented_by`/`verified_by`-linked requirements, so an explicit opt-out
  needed to exist rather than requiring a manual manifest/`spec/` cleanup after
  every scaffold. `--discipline full` can always be turned on later via
  `hotam vendor-recorder` if the narration upgrade is wanted after the fact.
  New tests: `TestCmdInitProject_DisciplineDefaultFull`,
  `TestCmdInitProject_DisciplineOffOptOut`,
  `TestCmdInitProject_DisciplineFlagRejectsBogusValue`. Full `go test ./...`
  (19 packages) green.
- **Tunable, decoupled `verified_by` exec timeout — flake-class fix (task #352, FLAKY)**:
  a systemic test-infrastructure flake, surfaced and confirmed three times this
  session (#350, #340, #341/#342 verifications), not a one-off test bug.
  `check_verified_by_test_passes` (internal/invariants/authored_links.go) and
  `check_scenario_executes_impl` (internal/invariants/scenario_coverage.go)
  shell `go test` as a SUBPROCESS (via internal/gate `RunVerifiedByTest` /
  `RunVerifiedByTestRecording`) to actually execute each `verified_by` test.
  The per-test execution timeout was a hardcoded **60s** sized for an unloaded
  box; under the heavy parallelism of a full `go test ./...` (many `t.Parallel()`
  tests, several of which each spawn `hotam`/`go test` subprocesses that in turn
  call `RunVerifiedByTest`), CPU + Go build-cache contention pushed individual
  subprocess `go test` invocations past 60s — *silently* in the sense that the
  resulting `Err` (DeadlineExceeded) is correctly NOT memoized in `runCache`
  (only `result.Err==nil` outcomes are cached), which made `AllViolations`
  return **different violation counts across two calls inside the same process**
  (the direct cause of `TestBuildStatusReport_MatchesOnRealDomain`'s
  ViolationCount mismatch: `buildStatusReport` → `AllViolations` and the test's
  own `AllViolations` diverged when one call timed out and the other didn't),
  and of whole-package flakes (`TestRunVerifiedByTest_MUTATION_*`,
  `TestRunVerifiedByTestRecording_*`) under load. All affected tests passed
  reliably in isolation and failed only under the full parallel suite.
  - **Configurable timeout** (`internal/gate/test_exec.go`): new
    `HOTAM_VERIFIED_BY_EXEC_TIMEOUT` env var (a Go `time.ParseDuration` string,
    e.g. `"120s"`, `"3m"`) overrides the per-test execution budget, read fresh
    on every call so a loaded CI runner or a heavy local `go test ./...` can
    extend it without recompiling. Unset/unparseable/non-positive →
    `defaultTestExecTimeout` (fail-safe: a misconfigured value can never
    disable the timeout guard).
  - **Raised default 60s → 180s** (`defaultTestExecTimeout`), now equal to
    `compileTimeout` with the same load-headroom rationale: under parallel
    contention a compiled test binary's wall-clock execution can approach
    cold-compile time, so the execution bound needs the same generosity the
    compile bound already grants.
  - **Decoupled execution budget from slot-wait budget** (the structural fix,
    mirroring what `doCompileTestBinary` already did for the COMPILE step):
    `runGoTest`/`runGoTestRecording` now run `cmd.Run` under their OWN
    freshly-minted `execCtx` (`testExecTimeout`), NOT the caller's ctx, so time
    spent QUEUEUING for a `globalExecSlots` slot under load cannot eat the
    test's execution budget. Previously slot-wait + execution shared one 60s
    ctx, so a 50s slot-wait left only 10s to run — the exact structural defect
    the compile step was already fixed for; the execution step now gets the
    same treatment on both sides of the slot-wait/run split. The caller's ctx
    still bounds the pre-execution waits (compile singleflight + slot
    acquisition), independently sized by `testExecTimeout`.
  - **Honesty preserved, not weakened**: a timeout still surfaces as a
    blocking `Err` → `check_verified_by_test_passes` violation ("could not be
    executed") / an infra-note in `check_scenario_executes_impl`, never a quiet
    pass or skip — a genuine infinite-loop test still fails loud within the
    bound; the fix just ensures mere parallel contention no longer trips it.
  - **Regression tests** (`internal/gate/test_exec_test.go`):
    `TestTestExecTimeout_DefaultAndEnvOverride` (pure unit test: default/valid/
    garbage/non-positive cases), and
    `TestRunVerifiedByTest[_Recording]_ExecTimeoutEnv_SpuriousTimeoutIsHonestErr`
    (force `HOTAM_VERIFIED_BY_EXEC_TIMEOUT=300ms` against a 3s-sleeping
    otherwise-passing test; assert the run surfaces `Err` containing "timed
    out", is never `Skipped`, never `Passed` — and that compile succeeds under
    its own 180s ctx, proving the budgets are decoupled).

- **Generate-don't-lint inventory + `Requirement.Evidence` retirement (task #342, R5-generate-dont-lint)**:
  read-only inventory-consult over the authoring text surfaces the
  requirements-as-code wave (#343-347) and task #341 left untouched: FAQ
  `orientation_faq` content, `Process.why`/`Step.why`, `Requirement.evidence`,
  domain `README.md`, and a meta-rule about task framing. Report presented to
  the resolver; three of five surfaces recommended LEAVE (FAQ content is
  already computed not stored, `Process.why`/`Step.why` is already ratcheted
  by `checkAuthoredProseSnapshot`, domain READMEs don't exist and would
  duplicate the generated DOMAIN-MAP), one recommended a drafted
  self-referential Requirement kept unlanded per resolver decision
  (`internal/selfspec/drafts/R-task-names-object-method-test.go`, `//go:build
  ignore`, not landed), and one — `Requirement.Evidence` — got a resolver
  decision: **retire, don't generate**. `internal/ontology/requirement.go`'s
  `Evidence` field doc comment now marks it RETIRED/superseded (the
  mechanical proof it informally carried already lives in `EnforcedBy`/
  `VerifiedBy` + `check_verified_by_test_passes`) — no new entries should be
  hand-written; a genuinely needed rationale pointer belongs in `Why`
  instead. The field itself (and its JSON tag, without `omitempty`, and its
  struct-declaration position) is UNCHANGED — most committed requirements
  already carry an explicit `"evidence": []`, and either change would have
  broken the byte-identical round-trip proof (`selfspec/merge_test.go`) for
  ~90% of this domain's requirements; caught by running that test after an
  initial draft edit, before committing.

- **`DomainManifest` typed object + byte-identical manifest round-trip (task #341, R5-manifest-object — Phase 1)**:
  applies the same "object is primary, JSON is a serialization/projection"
  discipline the requirements-as-code wave (#343-#347) established for
  Requirements (the `internal/selfspec` registry ↔ `graph.json` via
  `MergeIntoGraph` / `hotam sync-self`) to `manifest.json` — which until now had
  NO single typed object: every field was read piecemeal by an independent
  `Resolve*` function that re-opened and re-parsed the file for one key each
  (`resolveSelfHosting`, `ResolveDiscipline`, `ResolveGenProfile`,
  `ResolveRequireProvenance`, `ResolveParent`, `ResolveDomainPresentation`,
  `ResolveGateStageOrder`, `ResolveGateCohort`, `ResolveOrientationFAQ`). Phase 1
  is deliberately the conservative pilot (exactly as RAC-A/B phased the
  requirements migration): READ + REPRODUCE only, no authority flip, no mutation
  layer, no hand-edit guard.
  - **Typed struct** (`internal/loader/manifest.go`): `DomainManifest` models
    all 12 real manifest.json fields — `self_hosting` (bool), `purpose`,
    `goals`, `director`, `parent` (`*string`, reproduces explicit JSON null),
    `orientation_faq` ([]`OrientationFAQEntry`), `charter`, `discipline`,
    `gen_profile`, `require_provenance`, `gate_stage_order`,
    `gate_cohort` (`*GateCohortSpec`) — reusing the existing loader entry types
    directly (one source of truth). Struct field-declaration order is
    load-bearing for byte identity: it matches the on-disk field order of both
    real committed manifests (self_hosting, purpose, goals, director, parent,
    then orientation_faq), with the optional fields neither real manifest
    carries today declared after that prefix with `omitempty`.
  - **`LoadManifest`/`WriteManifest`**: lenient decode (no
    `DisallowUnknownFields`, matching every existing `Resolve*` tolerant
    contract) and a canonical write through the SAME encoder `graph.json` uses
    (`SetEscapeHTML(false)` + 2-space indent), atomic write-to-temp + rename.
    `ManifestPath(graphPath)` factors out the shared manifest.json path
    resolution every `Resolve*` function performs inline.
  - **Byte-identical round-trip proof**
    (`internal/loader/manifest_test.go:TestLoadWriteManifest_ByteIdenticalRoundTrip`):
    loads each real committed manifest (`domains/hotam-spec-self`,
    `domains/hotam-dev`), re-serializes through `WriteManifest`, and asserts
    byte-identical output — the same proof
    `internal/selfspec/merge_test.go:TestMergeIntoGraph_ByteIdenticalRoundTrip`
    provides for the Requirements registry. Plus idempotence, non-vacuous
    field-capture, and nil-guard controls.
  - **One-time canonicalization of the two real manifests**: the hand-authored
    `manifest.json` files used inline arrays for short `keywords` lists
    (`"keywords": ["a", "b"]`), which Go's `encoding/json` cannot reproduce
    (it always multi-lines arrays — `graph.json` never hits this because it is
    always machine-written in canonical form). Both real manifests were
    canonicalized once through `WriteManifest` (the `keywords` arrays expanded
    to multi-line; all field values byte-for-byte preserved — `hotam
    all-violations` still 0 on both domains). Going forward, manifest.json
    edits through the typed path stay canonical; the round-trip is now stable.
  - **`Keywords` omitempty** (`internal/loader/orientation_faq.go`): the one
    shared-type tag change byte identity required — `OrientationFAQEntry.Keywords`
    gained `omitempty` so a nil Keywords slice re-marshals as an omitted key
    (not `"keywords": null`). Behaviorally inert for all pre-existing callers
    (`omitempty` affects only marshaling, never unmarshaling, and this type was
    never marshaled before `WriteManifest`).
  - **Design decisions documented in code** (the de-facto design consult, per
    the brief): (1) ALL manifest fields are AUTHOR input — none is computed or
    derived from graph state, so the whole `DomainManifest` is one homogeneous
    authored surface (the structural/event split RAC draws for Requirements does
    not apply here); (2) the `Resolve*` functions remain the runtime authority —
    `DomainManifest` is a parallel proven-byte-identical typed surface, not a
    replacement yet (mirrors how the `selfspec` registry was a mirror before
    RAC-B flipped authority); (3) `parent` absent-vs-null is not distinguished
    in Phase 1 (`*string` conflates both to nil; `ResolveParent` stays the
    authority for that distinction until a later phase needs it on
    `DomainManifest`).
  - **Drafted Phase 2/3 plan**
    (`internal/loader/drafts/manifest-mutation-phases.go`, `//go:build ignore`):
    a durable architecture note — not implemented, not landed — for the two
    follow-on phases this task explicitly scoped out: Phase 2 (typed mutation
    layer, `ProposedManifestChange` with `Validate`/`Mutate`/`Apply`, mirroring
    `internal/proposal`'s shape for graph nodes) and Phase 3 (hand-edit guard,
    the `R-no-hand-edit-graph` analogue — a manifest content-pin or check that
    refuses a direct hand-edit bypassing the typed path). Per the #340 draft
    precedent, landing either is a separate resolver-approved step.

- **debt ratchet (task #340, R5-debt-ratchet)**: a one-way ratchet that fails
  CI the moment "claimed but not mechanically guaranteed" debt GROWS against a
  frozen per-domain pin, turning silent debt accumulation into a visible,
  conscious commit-time decision. New test
  `internal/selfcheck/debt_ratchet_test.go` (alongside the existing
  `race_ratchet_test.go` ratchet home), two distinct signals:
  - **Check (a) — closeable-debt ceiling** (`TestDebtRatchet_CloseableDebtNotGrowing`):
    for every domain with a pin, the live count of requirements where
    `Requirement.IsCloseableDebt()` is true (`Enforcement != ENFORCED AND
    Enforceability == ENFORCEABLE` — the actionable "a real test could be
    written for it" set) MUST NOT exceed the pin. The count is status-agnostic
    (DRAFT claims count, because a brand-new unenforced claim is growing debt
    the moment it lands); pinned at 73 for hotam-spec-self, 6 for hotam-dev
    (captured 2026-07-24).
  - **Check (b) — PROSE-enforceable ceiling** (`TestDebtRatchet_ProseEnforceableNotGrowing`):
    the narrower, softer signal — the count of `Enforcement == PROSE AND
    Enforceability == ENFORCEABLE` requirements (no engine link at all) MUST
    NOT exceed the pin, so adding a new PROSE claim requires converting an old
    one to ENFORCED in the same wave; pinned at 70 / 1.
  - **Design decisions (documented in the test file's header comment):** (1)
    the pin is a per-domain numeric constant in two Go maps in the test itself
    (`closeableDebtPins` / `proseEnforceablePins`), mirroring
    `registry_complete_test.go`'s `const expected = N` ceiling pattern — a
    single source of truth, not a second hand-maintained baseline file; (2)
    per-domain (not one global ceiling) — each domain has its own maturity,
    and only the two domains living in this repo are pinned; (3)
    `INHERENTLY_PROSE` is excluded from both counts by construction
    (`IsCloseableDebt` ANDs `ENFORCEABLE`; check (b) ANDs it explicitly) —
    legitimate never-ENFORCED prose is not debt; (4) a domain with no pin
    entry is SKIPPED (new-domain grace — a fresh domain legitimately starts
    with a wall of DRAFT PROSE, and the ratchet must punish only GROWTH
    relative to an already-recorded pin). On improvement (`got < pin`) the
    test also fails, asking the author to LOWER the pin so a future regression
    cannot hide inside the slack. Three non-vacuity controls prove the ratchet
    actually fires (synthetic-graph fire-branch reachability, the
    INHERENTLY_PROSE exclusion, and a dead-pin-key guard against a typo'd or
    renamed domain). Mutation-tested against the live graph: pin 72 vs actual
    73 fails RED with an actionable message.
  - **Drafted (NOT landed) methodology requirement**
    `.scratch/drafted_R-debt-ratchet-no-growth.go`: the self-referential
    `R-debt-ratchet-no-growth` requirement formalizing this ratchet as a
    SETTLED graph node, authored in the Go-literal form RAC-B now mandates for
    hotam-spec-self Requirements (a `//go:build ignore` file so it never
    enters the registry or touches graph.json), with a landing checklist for a
    resolver-approved follow-on wave (paste into
    `internal/selfspec/requirements_enforcement.go` → `hotam sync-self` →
    `gen-spec`). `refines R-enforceability-kind-declared` (the enforceability
    split this ratchet is the convergence mechanism for).

- **RAC-C: requirements-as-code propagated into the operator-facing crystal text (task #347 — closes the requirements-as-code wave, tasks #343-347)**:
  the fourth and final phase of the RAC wave (#343 R5-head-cure → #344 RAC-0 →
  #345 RAC-A → #346/#348-351 RAC-B) propagates the RAC-B authority flip
  (`hotam sync-self`, RAC-B2/B3/B4) from mechanism into the text every
  operator actually reads: the generated CLAUDE.md's MEDIATION-LOOP block,
  the founding-canvas guidance for new domains, an explicit open Conflict
  about the resolver-trust shift the flip introduces, and an honest
  partial-debt correction on the requirement that predicted this whole
  migration.
  - **TRANSLATE-step branching** (`internal/generator/claudemd_static.go`,
    `claudemd.go`): `RenderMediationLoopBlock` now takes `g *ontology.Graph`
    (was: no args) and branches its TRANSLATE-step text on `g.SelfHosting`.
    On the self-hosting domain, a Requirement/Rejection outcome is described
    as a Go `ontology.Requirement` literal in
    `internal/selfspec/requirements_<topic>.go` + `hotam sync-self` (whose
    default dry-run render is named as the PRESENT-step artifact the
    resolver reviews before `--confirm-hash`) — matching what RAC-B3 already
    enforces (the old JSON-file description would otherwise describe a path
    `apply-proposal`/`land` now actively refuse on this domain). Every other
    outcome kind (Conflict, Assumption, ConflictTransition, OperatorBudget,
    EntityType, GateSignoffBatch, ReviewMark) stays on the plain
    `Proposed*`-JSON path on EVERY domain, self-hosting included. A consumer
    domain's crystal keeps the single unbranched pre-RAC-C text for
    Requirement too and never mentions `internal/selfspec`/`sync-self`
    (`sync-self`'s own gate 0 refuses to run against a non-self-hosting
    domain, so naming it to a consumer operator would point at a command
    that cannot succeed for them). `nil` is treated as non-self-hosting (the
    same default `ontology.Graph{}`'s zero value already establishes).
    New test: `internal/generator/claudemd_selfhosting_translate_test.go`
    (`TestRenderMediationLoopBlock_SelfHostingBranchesTranslateStep`) renders
    both branches directly and asserts the self-hosting branch names
    `internal/selfspec`/`sync-self`/`requirements_` while the consumer/nil
    branch's TRANSLATE-step text does not, and that both keep every one of
    the six mediation-loop step headings.
  - **Founding-canvas step 6 middle path** (same files): the 8-step
    `R-domain-founded-in-wave-order` canvas's step 6 ("Requirements, linked
    to that code and those tests") now also describes a MIDDLE path for a
    brand-new domain that wants requirement-authorship-in-code from day one
    without adopting the self-hosting domain's full authority-flip
    machinery: author Requirement drafts as `[]ontology.Requirement`
    literals in that domain's own `spec/requirements.go`, then generate the
    `ProposedRequirement` JSON proposals FROM those literals for landing
    through the ordinary `apply-proposal`/`land` path — JSON is always a
    projection of the code, never the reverse. Explicitly disclaimed as NOT
    getting its own `sync-self`-equivalent CLI (that append-only/
    byte-identity-enforcing machinery stays exclusive to `hotam-spec-self`;
    a consumer analog is a distinct, not-yet-implemented future feature, and
    the text says so rather than promising tooling that doesn't exist). This
    paragraph is unconditional (rendered on every domain, self-hosting
    included) since it is general methodology guidance about founding new
    domains, not gated by `g.SelfHosting`. Named as the intended path for a
    future life-domain (task #323's personal-operating-system pilot) and
    similar new domains, without implementing that domain itself. Same test
    file, `TestRenderMediationLoopBlock_FoundingCanvasStep6MiddlePath`.
  - **New open Conflict — `reviewability-vs-code-authority`**
    (`domains/hotam-spec-self/graph.json`, landed via `hotam land --batch`
    against `domains/hotam-spec-self/proposals/task347-rac-c/`): a new Axis
    (`reviewability-vs-code-authority` — "what form the resolver's signoff
    attaches to: the raw artifact under review vs a rendered projection
    trusted by construction") plus Conflict `C-d20cf537` on that axis,
    members `R-ai-presents-not-decides` + `R-requirement-update-signoff-typed`
    (only one SETTLED — `check_constituting_not_in_unresolved_conflict`
    forbids an unresolved self-hosting Conflict from holding two SETTLED
    constituting atoms, so the second member is deliberately the DRAFT
    `R-requirement-update-signoff-typed`), resolver `framework-reviewer`,
    lifecycle left `DETECTED` (per `R-conflict-is-connector-node`/
    `R-ai-presents-not-decides`: presented, not silently resolved). Context:
    before RAC-B, a resolver reviewed the exact `ProposedRequirement`/
    `ProposedRejection` JSON that would be written; after RAC-B3, the
    resolver instead reviews `hotam sync-self`'s rendered `SyncReport` diff
    and trusts it BY CONSTRUCTION (proven byte-identity/append-only, not
    re-derived by eye). Trivial for a programmer-resolver (an ordinary Go
    code review); the open question is whether this pattern would still work
    if ever extended to a consumer domain with a non-programmer resolver —
    recorded now, unresolved, before any consumer domain proposes adopting
    it.
  - **`R-generations-inherit-doc-test-code` honest partial-debt correction**
    (`internal/selfspec/requirements_authoredspec.go`, landed via
    `hotam sync-self --confirm-hash`): the requirement's `Why` now records
    that its Requirement-half claim ("EVERY SETTLED requirement MUST yield a
    named Go declaration") is now backed by a real mechanism — the RAC-A
    registry + RAC-B `sync-self` + RAC-B4's live
    `check_self_requirements_match_registry` gate — while its EntityType-half
    claim ("EVERY EntityType MUST yield a Go struct + lifecycle methods +
    transition tests") remains completely unmet: no EntityType-to-Go
    generator exists anywhere in this codebase. `Enforcement` stays `PROSE`;
    this is a documentation-honesty fix, not a status promotion — the RAC
    wave solved half the claim and the `Why` field says so instead of
    implying more progress than exists.
- **`check_self_requirements_match_registry` promoted to a real gate + dogfood landing (task #351, RAC-B4 — closes RAC-B/task #346, the authority-flip phase of the requirements-as-code migration)**:
  the RAC-A (task #345) shadow-only advisory check (`internal/invariants/
  selfspec_shadow.go`) — comparing `internal/selfspec.Requirements` (the Go
  registry) against `domains/hotam-spec-self/graph.json`'s `Requirement`
  nodes — is now registered in `All` (`All.MustRegister`), so it runs inside
  `hotam all-violations`/`invariants.AllViolations` like any other framework
  invariant and blocks the exit code on drift. Self-hosting-gated the same
  way `check_bijection_r_to_enforcer` is: an internal `!g.SelfHosting`
  early-return AND an entry in `frameworkScopedInvariantNames`
  (`internal/invariants/all_violations.go`) so the fan-out never even calls
  `Check` for a non-self-hosting graph. The now-redundant advisory wiring
  (`SelfRequirementsMatchRegistryWarnings` in `cmd/hotam/all_violations.go`'s
  `printAdvisorySection`) was removed to avoid double-reporting the same
  drift in both the ordinary violations list and the ADVISORY section; the
  exported wrapper function itself stays (still used directly by this
  file's own tests). `TestCheckSelfRequirementsMatchRegistry_
  NeverRegisteredInAllRegistry` (the RAC-A shadow-band contract) was
  inverted into `TestCheckSelfRequirementsMatchRegistry_
  RegisteredInAllRegistry`, plus a new
  `TestCheckSelfRequirementsMatchRegistry_SelfHostingScopedInFrameworkNames`
  belt-and-braces control. `TestRegistryComplete_CountMatchesTarget`'s
  registered-invariant count ratchet updated 112 → 113.
  - **Orphan-enforcer coupling**: `check_bijection_r_to_enforcer`
    (`internal/invariants/self_reference.go`) requires every registered
    `check_*` to be named in at least one SETTLED/ENFORCED requirement's
    `enforced_by`. `R-no-hand-edit-graph` (`internal/selfspec/
    requirements_deterministic.go`) now names
    `check_self_requirements_match_registry` alongside its existing two
    enforcers (`TestNoHandEditGraph_RealDomainLocksPinCurrentGraph`,
    `check_graph_lock_pins_graph_json`), and its `Claim`/`Why` were updated
    to reflect the new reality: for the self-hosting domain, a
    Requirement/Rejection node's structural fields are now authored in Go
    (`internal/selfspec`) and projected onto `graph.json` by
    `hotam sync-self` (RAC-B2) — never by hand-editing `graph.json` — with
    `check_self_requirements_match_registry` as the mechanism that makes a
    bypass (a hand-edit RAC-B3's `apply-proposal`/`land` lock didn't catch,
    or a registry edit nobody synced yet) mechanically detectable.
  - **Dogfood landing**: the registry edit above was landed onto
    `domains/hotam-spec-self/graph.json` THROUGH `hotam sync-self`
    `--confirm-hash` itself — the first real, disk-writing `hotam sync-self`
    run since RAC-B2 introduced the command, and the first time this
    engine's own self-hosting domain was mutated by that path rather than
    `apply-proposal`/`land`. The confront gate (gate 7) flagged three
    lexical opposite-marker false positives against unrelated requirements
    (`R-authored-spec-layer-progression`, `R-context-budget-rule`,
    `R-rules-as-data`, all sharing only/any-style tokens with the new Claim
    text, no real semantic tension) — landed with `--decision-ref` recording
    the acknowledgment as a `HistoryEntry`, per the confront gate's
    designed override contract (RAC-B2). Verified clean afterward:
    `hotam all-violations --domain domains/hotam-spec-self` reports 0
    violations (the new gate is silent because the registry and graph are
    now in sync by construction).
  - **`hotam sync-self` write-path fix found by the dogfood run itself**:
    `runSyncSelfWrite` (`cmd/hotam/sync_self.go`) previously called
    `genSpec` with `includeSpec=false` (mirroring `hotam land`'s three
    identical call sites) — but a sync that changes a `Requirement`'s
    `Claim`/`Why` can invalidate a committed `docs/gen/SPEC.md` (which
    embeds `Claim` text verbatim for every SETTLED requirement with a
    resolvable `verified_by`), and `check_spec_md_current`
    (`ComparesOnDiskProjection`) is NOT filtered out of the ordinary
    `allViolations` call the write path uses for its final post-write
    check (only the proposal-gate view filters those) — so a Claim-changing
    sync-self run against a `verified_by`-linked requirement rolled itself
    back on its own SPEC.md staleness. Fixed by flipping
    `runSyncSelfWrite`'s `genSpec` call to `includeSpec=true` (paying the
    real `go test`-per-`verified_by` cost once, only on the rarer,
    explicitly `--confirm-hash`-gated write path), plus a new local
    snapshot/restore pair (`snapshotSpecMD`/`rollbackSyncSelf`) so a
    post-genSpec rollback also restores `SPEC.md` to its pre-sync bytes —
    `rollbackLand`'s own shared `genSpec(..., false)` call deliberately
    never touches an existing `SPEC.md` (correct for `hotam land`, which
    never opts into `--spec`), so without this, a rollback after the new
    `includeSpec=true` call would leave a post-sync `SPEC.md` on disk next
    to a restored pre-sync `graph.json`. `hotam land`'s own three call
    sites are unchanged (same pre-existing `includeSpec=false` gap remains
    there, out of RAC-B4's scope).
- **Self-hosting Requirement/Rejection lock (task #350, RAC-B3)**:
  `internal/proposal.applyToGraph` — the single choke point single-file
  `apply-proposal`, `--batch`, `hotam land`, and `propose --land` all funnel
  through — now refuses a `ProposedRequirement` (CREATE or UPDATE) or
  `ProposedRejection` BEFORE any mutation when the target graph's
  `manifest.json` declares `self_hosting: true`. `hotam sync-self` (RAC-B2)
  is now the SOLE sanctioned path for Requirement/Rejection changes in the
  self-hosting domain; every other proposal kind (Conflict\*, Assumption\*,
  EntityType, Process, Axis, Stakeholder, OperatorBudget, ReviewMark,
  GateSignoffBatch) and every non-self-hosting domain are unaffected.
  - The refusal message names the concrete `internal/selfspec/
    requirements_<topic>.go` file the ID lives in (via the existing
    `selfspec.SourceFileFor`) when the ID is already registered, or the
    `internal/selfspec` package + `requirements_<topic>.go` filing
    convention (without inventing a nonexistent filename) when the ID is
    brand-new. Both name `hotam sync-self` as the way forward.
  - The Rejection-specific message additionally reminds the operator that
    landing a Rejection also appends a `replaces` Relation onto the
    successor node(s) named in `replaced_by` — a structural edit to a
    DIFFERENT node than the one being rejected — which a manual migration
    into the Go registry workflow must add by hand.
  - `internal/proposal` now imports `internal/selfspec`; verified against
    `TestCorePeriphery_ImportRatchet` (`internal/selfcheck/imports_test.go`)
    — `internal/selfspec` imports only `internal/ontology` +
    `internal/registry` (both core-ward), so no cycle and no
    core→periphery violation.
  - New tests: `internal/proposal/self_hosting_lock_test.go` (known-ID /
    unknown-ID / Rejection message shape, other-kinds-unaffected,
    non-self-hosting-unaffected — all at the `applyToGraph` level, with a
    real `manifest.json` on disk since `loader.LoadGraph` always resolves
    `Graph.SelfHosting` from the manifest, never from an in-memory struct
    field or graph.json's own `self_hosting` key) and
    `cmd/hotam/self_hosting_lock_test.go` (CLI-level: `apply-proposal` and
    `land` against the real `hotam-spec-self` fixture, plus a non-self-
    hosting negative control).
  - Existing `cmd/hotam` tests that landed a Requirement/Rejection against a
    copy of the real self-hosting `hotam-spec-self` fixture (`copySelfDomain`)
    were migrated to two new fixture helpers in `main_test.go`:
    `copyNonSelfHostingDomain`/`copyNonSelfHostingDomainUnderRoot`, which
    copy the real graph+manifest, force `self_hosting: false`, AND clear
    every requirement's `implemented_by`/`verified_by` (illegal outside
    `self_hosting: true` per `internal/gate.SpecRoot`), downgrading the 3
    requirements that relied solely on the authored-link enforcement path to
    `PROSE`/`INHERENTLY_PROSE` so `all-violations` stays clean post-land. A
    bare manifest-flag flip alone is NOT sufficient for any test that runs
    `hotam land`'s post-apply `gen-spec` + `all-violations` pipeline — see
    `makeNonSelfHosting`'s doc comment in `cmd/hotam/main_test.go`.
- **`hotam sync-self` command (task #349, RAC-B2)**: the CLI that finally
  exercises `internal/selfspec.SyncGraph` (RAC-B1) as a real authority-flip
  tool — mirrors `internal/selfspec.Requirements` (the Go registry) onto
  `domains/hotam-spec-self/graph.json`, the engine's OWN self-hosting
  domain, and ONLY that domain (refuses any `--domain` whose
  `manifest.json` does not set `self_hosting: true`). `graph.json` remains
  the sole runtime-trusted format; this command is the first thing that can
  actually write to it FROM the registry (previously `MergeIntoGraph`/
  `SyncGraph` were library-only, unreachable from the CLI).
  - `internal/selfspec.verifyAppendOnly` renamed to the exported
    `VerifyAppendOnly` (`verify.go`, `verify_test.go`) — required so
    `cmd/hotam` (outside the package) can call it as the last write-gate.
  - **Default mode is dry-run** (no `--confirm-hash`): loads the domain
    graph, runs `SyncGraph` against an in-memory copy (a second, independent
    `loader.LoadGraph` of the same file — the simple stand-in for a
    dedicated deep-clone helper, since the graph is otherwise mutated
    in-place), renders a human-readable report per `SyncReportEntry`
    (ID/Kind/FieldDiffs, values abbreviated to ~150 runes for display only),
    runs every write-gate as a PREVIEW (see below), and prints
    `diff-hash: <hex>` — a sha256 over canonical JSON of
    `{base_graph_sha256, diff: SyncReport}` with FULL untruncated
    `FieldDiff` values and entries sorted by ID. Writes nothing.
  - **`--confirm-hash <hex> --today YYYY-MM-DD [--reason "..."]`** re-runs
    `SyncGraph` against the CURRENT on-disk state (which may have moved
    since the dry-run), refuses with "diff changed since PRESENT" on any
    hash mismatch, then runs the full gate sequence for real. Gate order
    before any write, strictly: **stale-binary → confront → pre/post-
    violation-diff → append-only**.
    1. **Stale-binary guard**: compares every file embedded in
       `selfspec.SourceFiles` against the SAME files read fresh off disk
       under the resolved engine repo root (`repoRootForDomain(domainDir)` +
       `internal/selfspec/`) — a byte mismatch refuses with "stale binary;
       run via `go run ./cmd/hotam sync-self` or rebuild". A `go run`
       invocation always compiles fresh, so it trivially passes.
    2. **Confront gate**: for every `ADDED` entry, and every `CHANGED` entry
       whose `FieldDiffs` include `Field=="Claim"`, runs
       `diagnose.Confront` + `diagnose.IsBlockingHit` against the working
       graph — EXCLUDING, for a `CHANGED` entry, any hit whose ID equals the
       entry's own ID (a requirement's own claim changing must never
       self-block). Blockers require `--ack-conflict <C-id>` (validated
       against a real Conflict node) or `--decision-ref "..."` to proceed,
       mirroring `semanticConflictGate`'s override contract; an ack writes a
       History audit entry on every flagged requirement (post-write,
       pre-regen — mirrors `appendAckHistory`'s placement).
    3. **Pre/post violation diff**: `invariants.AllViolationsForProposalGate`
       on the untouched `before` graph vs. the post-`SyncGraph` `after`
       graph; any violation present only in `after` refuses.
    4. **Append-only guard**: `selfspec.VerifyAppendOnly(before, after)` —
       last gate, since it is the most expensive-to-explain failure.
  - **Write path** (only after every gate passes): `loader.WriteGraph` +
    `loader.WriteLock` (note: `"sync-self: N changed, M added; diff-hash
    <hex-prefix>[; reason: ...]"`) → `gen-spec` doc regeneration →
    `all-violations` re-verification. Reuses `land.go`'s EXACT transactional
    snapshot/rollback machinery (`snapshotGraphFiles`/`rollbackLand`) — a
    snapshot is taken before the write, and any failure after it (lock
    write, ack-history append, doc regen, or a post-regen violation) rolls
    the domain back to its pre-sync state, same as `hotam land`.
  - Registered in `internal/methodology/tools_data.go` as `sync_self`
    (Implemented) and wired via `cmd/hotam/tool_wiring.go` — bumps the
    engine's Implemented-tool count from 17 to 18 (README.md, the
    `methodology` package's own count tests, and every generated doc/
    fixture that projects the tool registry — `docs/gen/tools/INDEX.md`,
    `REQUIREMENTS.md`, `REPO-MAP.md`, `FRAMEWORK-INVARIANTS.md`, the root
    crystal's "Tool reference" line, and `internal/generator`'s golden test
    fixtures — regenerated to match).
  - e2e coverage in `cmd/hotam/sync_self_test.go`: dry-run never writes;
    hash-mismatch refuses without writing; full ADDED and CHANGED
    round-trips (dry-run → hash → `--confirm-hash` → graph actually
    written, docs regenerated, `all-violations` clean); rollback on a
    post-write failure (genSpec blocked, same injection technique as
    `TestCmdLand_GenSpecFailure_RollsBackGraphJSON`); the self-hosting-only
    domain guard; the stale-binary guard (a corrupted fixture-copied source
    file); and a real-subprocess smoke test. The confront-blocker-refusal
    and append-only-refusal scenarios are proven at the `runSyncGates`
    function level instead of end-to-end through the CLI (registry entries
    are fixed at compile time, so a real contradicting claim pair can't be
    constructed by a test without depending on incidental, driftable
    registry content) — `TestRunSyncGates_ConfrontBlocker_RequiresAck`,
    `TestRunSyncGates_ConfrontBlocker_ExcludesSelfID`,
    `TestRunSyncGates_NewViolation_Refuses`,
    `TestRunSyncGates_AppendOnlyGate_CalledLast`.
- **`internal/selfspec`: Phase B sync primitives (task #348, RAC-B1)**:
  in-memory building blocks for the authority-flip `hotam sync-self` command
  (task #349, RAC-B2, not yet built) — `graph.json` remains the sole
  runtime-trusted format after this step; nothing here writes to disk or
  changes what any existing command reads.
  - `StructuralFieldDiffs(reg, graph ontology.Requirement) []FieldDiff`
    (`diff.go`): the single source of truth for the FULL list of structural
    fields (Claim, Owner, Status, Why, Assumptions, Relations, Enforcement,
    EnforcedBy, MTag, Enforceability, Summary, CreatedAt, SettledAt,
    SourceRefs, DeclOrder, BlockedOn, ImplementedBy, VerifiedBy) that differ
    between a registry entry and a graph node — a strictly richer
    replacement for `internal/invariants`' unexported
    `firstStructuralFieldDiff` (which stops at the first difference and was
    missing `CreatedAt`/`DeclOrder` even though `MergeIntoGraph` replaces
    both). `FieldDiff{Field string; Old, New any}` carries values WHOLE,
    never truncated — truncation for display/hashing is a caller's job.
    Meant to be reused by name at three future call sites (a dry-run render,
    a live invariant, and a history-summary generator).
  - `SyncGraph(g *ontology.Graph, today string) (*SyncReport, error)`
    (`sync.go`): unlike `MergeIntoGraph` (which hard-errors on a registered
    ID absent from the graph), `SyncGraph` CREATES a new node for it — with
    structural fields from the registry, empty event fields, and one seed
    `HistoryEntry{At: today, Summary: "created via sync-self"}`. For an
    existing node whose structural fields disagree with the registry, it
    performs the same field replacement `MergeIntoGraph` does AND appends a
    `HistoryEntry` summarizing the change (`"field X: <old> -> <new>; ..."`)
    — closing the gap that `MergeIntoGraph` deliberately never writes
    History (it backs a shadow/mirror check, not a landing path).
    `SyncReport{Entries []SyncReportEntry{ID string; Kind SyncKind; FieldDiffs
    []FieldDiff}}` lists only nodes actually created (`SyncKindAdded`) or
    changed (`SyncKindChanged`) — a no-op node is untouched and absent from
    the report. A graph node with no matching registry ID is left completely
    alone (deletion is out of scope; a future boot invariant, task
    #351/RAC-B4, catches that drift). Mutates `g` in place; never touches
    disk.
  - `verifyAppendOnly(old, new *ontology.Graph) error` (`verify.go`): a pure,
    `SyncGraph`-independent guard proving a graph transition respects the
    append-only journal invariant — for every Requirement in `old`, its ID
    must still exist in `new`, and its `History` AND `GateSignoffs` must each
    be an exact PREFIX of the corresponding `new` list (stricter than a
    length-only or superset check: catches both truncation and in-place
    mutation of an already-recorded entry).
  - `SourceFiles embed.FS` (`embed.go`, `//go:embed requirements_*.go
    selfspec.go`): a build-time-frozen copy of this package's own
    registration source files, laying the groundwork for a future
    stale-binary guard (task #349+). `SourceFileFor(id string) (string,
    bool)` looks up which `requirements_<topic>.go` file registers a given
    ID (exact `MustRegister("<id>"` literal match); `false` is the
    legitimate answer for a brand-new ID not yet placed in any thematic
    file.
  - Full unit test coverage in `diff_test.go`, `sync_test.go`,
    `verify_test.go`, `embed_test.go` — including an integration test
    (`TestSyncGraph_SatisfiesAppendOnly`) proving `SyncGraph`'s own output
    actually satisfies `verifyAppendOnly`. `MergeIntoGraph` itself is
    UNCHANGED — Phase A's byte-identical round-trip test
    (`TestMergeIntoGraph_ByteIdenticalRoundTrip`) still passes.
- **`internal/selfspec`: Phase A full-coverage requirements-as-code registry
  (task #345, RAC-A)**: scales Phase 0's (task #344, RAC-0) proven
  byte-identity mechanism from an 18-requirement hand-picked pilot subset to
  ALL 301 `domains/hotam-spec-self/graph.json` Requirement nodes (253
  SETTLED + 42 REJECTED + 6 DRAFT). NO authority flip — `graph.json` remains
  the sole runtime-trusted, universally-read format (that is task #346,
  RAC-B, deliberately out of scope here).
  - Replaced the single `requirements_pilot.go` file with 27 thematic
    `internal/selfspec/requirements_<topic>.go` files (`operator`, `agent`,
    `domain`, `entity`, `process`, `crystal`, `lifecycle`, `conflict`,
    `gate`, `goal`, `docs`, `framework`, `enforcement`, `anchor`, `boot`,
    `authoredspec`, `attention`, `activeloop`, `ticket`, `tension`,
    `critical`, `budget`, `trust`, `scope`, `reflection`, `empty`,
    `deterministic`) — mirroring `internal/invariants`' many-files-by-topic
    layout instead of one ~10k-line file. Each requirement ID is classified
    into exactly one bucket by a deterministic, reviewable prefix-match rule
    table (`.scratch/selfspec-codegen/main.go`'s `topicRules`, not committed
    — throwaway codegen, same "generate once, review the diff, discard the
    generator" precedent as Phase 0); every one of the real graph's 301 IDs
    matched a rule on the first design pass except 8 (`R-axes-as-module-
    constant`, `R-director-agent-required-per-domain`, `R-proposed-conflict-
    kind-exists`, `R-proposed-stakeholder-kind-exists`, `R-sensorium-
    committed`, `R-spawn-log-carries-isolation`, `R-speculative-aspects-
    frozen`, `R-verify-closure-per-action`), each placed by hand into its
    nearest thematic neighbor.
  - `internal/selfspec/merge_test.go`: `TestMergeIntoGraph_
    ByteIdenticalRoundTrip` passed on the FIRST full-scale run — no fix
    cycle needed, confirming Phase 0's `decl_order` per-node preservation,
    nil-vs-`[]string{}` slice handling, and `%q` Cyrillic/typographic-dash
    escaping generalize cleanly to the full 301-node scale. Added
    `TestMergeIntoGraph_AllRequirementsRegistered`, a stronger claim than
    Phase 0's non-vacuity check: pins the registry at exactly 301 entries
    and asserts set-equality between the registry's IDs and the graph's IDs
    (not just "whatever is registered round-trips cleanly").
  - Manual spot-check: 15 requirement IDs chosen by a seeded random
    permutation (not cherry-picked) were compared field-by-field against
    the registry and found byte-for-byte identical on every structural
    field — a secondary check layered on top of the exhaustive automated
    round-trip, which remains the primary correctness guarantee for a
    ~9500-line generated diff no one reads line-by-line.
  - `internal/selfcheck/contentfree_test.go`'s existing `isContentIntakeSite`
    exemption for `internal/selfspec/*` (added in Phase 0) covers all 27 new
    files unchanged — it is a path-substring check, not a per-file allowlist,
    so no widening was needed for the ~16x file-count increase.
  - Value of this phase: authority-by-construction now covers the WHOLE
    `hotam-spec-self` constitution, not a curated slice — every one of the
    301 requirements' structural fields is a reviewed Go diff waiting to
    happen, not a hand-edited JSON blob, with the byte-identical round-trip
    test as the standing proof the mirror has not drifted.

- **`check_self_requirements_match_registry`: SHADOW-mode drift detector
  between `internal/selfspec.Requirements` and `hotam-spec-self`'s graph
  (task #345, RAC-A)**: `internal/invariants/selfspec_shadow.go` compares
  the Phase A registry against `g.Requirements` and reports three drift
  classes — a registered ID missing from the graph, a graph ID missing from
  the registry, or a structural-field mismatch (Claim/Owner/Status/Why/
  Assumptions/Relations/Enforcement/EnforcedBy/MTag/Enforceability/Summary/
  CreatedAt/SettledAt/SourceRefs/DeclOrder/BlockedOn/ImplementedBy/
  VerifiedBy — exactly the fields `selfspec.MergeIntoGraph` replaces,
  deliberately excluding event fields) between an ID present in both.
  - SHADOW, never a gate: mirrors `HonoredSkipWarnings`/
    `AuthoredProseSnapshotWarnings`'s established "advisory band" pattern —
    a plain exported function (`SelfRequirementsMatchRegistryWarnings`),
    deliberately NOT registered via `All.MustRegister`, wired into
    `cmd/hotam/all_violations.go`'s non-blocking `printAdvisorySection`
    alongside the other two. Never appears in `invariants.AllViolations`,
    never blocks `hotam all-violations`'s exit code or `internal/proposal/
    apply.go`'s proposal gate — the registry is a mirror, not yet the
    authority, so its own staleness is a visible signal, not a CI failure.
  - `g.SelfHosting`-gated (an honest no-op otherwise): the registry mirrors
    exactly ONE graph — this repo's own `domains/hotam-spec-self/
    graph.json` — so running the comparison against any other domain
    (a consumer's `prat`/`gpsm-sm`, or even this repo's own non-self-hosting
    `hotam-dev`) would report all 301 registered IDs as spuriously
    "missing", pure noise from comparing unrelated domains' requirement
    sets. Verified clean (zero advisory output) against the real
    `hotam-spec-self` graph, `hotam-dev`, and the sibling `PRAT-hotam`
    repo's `prat`/`gpsm-sm` domains (read-only `all-violations` runs).
  - 8 tests in `internal/invariants/selfspec_shadow_test.go`: a clean-pass
    proof against the real committed graph, a non-SelfHosting/nil-graph
    no-op proof, three mutation-based non-vacuity controls (one per drift
    class), an exported-wrapper-matches-internal-check proof, and the
    never-registered-in-`All` contract test — all pass under `-race`.

- **`internal/selfspec`: Phase 0 pilot of the requirements-as-code migration
  (task #344, RAC-0)**: a Go registry proving the byte-identity mechanism
  that later phases will scale to all 301 `hotam-spec-self` Requirements. NO
  authority flip — `domains/hotam-spec-self/graph.json` remains the sole
  runtime-trusted, universally-read format; `selfspec.Requirements`
  (`internal/registry.Registry[ontology.Requirement]`, the same generic
  registry `internal/invariants`' `All` and `internal/methodology`'s
  `Sections`/`Tools` already use) is a proven-byte-identical MIRROR of a
  hand-picked 18-requirement subset, extending the `internal/methodology/
  sections_data.go`+`tools_data.go` pattern (canonical prose as reviewed Go
  registry values) to a third content family.
  - `internal/selfspec/requirements_pilot.go`: 18 `Requirements.MustRegister`
    literals — the 6 requirements landed this session
    (`R-shared-projections-mode-independent`, `R-gate-cohort-explicit-
    denominator`, `R-authored-prose-no-live-tallies`, `R-pipeline-live-
    state-from-typed-carriers`, `R-requirement-update-signoff-typed`,
    `R-claude-md-current`), well-known stable SETTLED anchors
    (`R-gate-signoff-single-carrier`, `R-orientation-faq-answerable`,
    `R-signoff-preserved-in-substrate`, `R-generations-inherit-doc-test-code`,
    `R-no-hand-edit-graph`, `R-anchor-everything`, `R-ai-presents-not-decides`,
    `R-decided-needs-human-signoff`, `R-glossary-drift-stable`,
    `R-conflict-is-connector-node`, `R-resolver-distinct-from-owners`), and
    one REJECTED requirement (`R-agent-imports-framework`) proving the
    pattern covers REJECTED status ahead of Phase A's ~42 REJECTED nodes.
    Generated once by a throwaway codegen program
    (`.scratch/selfspec-codegen/`, not committed — reads the real graph.json
    and emits the `%q`-escaped Go literals) and reviewed by hand; carries
    ONLY STRUCTURAL fields (`Claim`/`Why`/`Owner`/`Status`/`Relations`/
    `Assumptions`/`Enforcement`/`EnforcedBy`/`Enforceability`/`MTag`/
    `Summary`/`CreatedAt`/`SettledAt`/`BlockedOn`/`ImplementedBy`/
    `VerifiedBy`/`SourceRefs`/`DeclOrder`) — `ImplementedBy`/`VerifiedBy`
    stay `[]string`, deliberately never typed further (Proof cannot be typed
    in Go at all, and execution-checking a named test actually exists and
    passes is a strictly stronger guarantee than any compile-time type over
    a `file:symbol` string).
  - `internal/selfspec/merge.go`: `MergeIntoGraph(g *ontology.Graph) error`
    replaces each registered requirement's structural fields in place from
    the registry, passing the EVENT fields (`History`, `GateSignoffs`,
    `LastReviewedAt`, `ReviewAfter`, `Evidence`) through from the graph's
    existing node untouched. Phase 0 scope, deliberately narrow: a
    registered ID absent from the graph is an error (no node creation yet);
    a graph node not in the registry is left completely untouched.
  - `internal/selfspec/merge_test.go`: `TestMergeIntoGraph_
    ByteIdenticalRoundTrip` loads the real committed graph.json, runs the
    merge, re-serializes via `loader.WriteGraph` (reusing the loader's own
    canonical-marshal path — `SetEscapeHTML(false)`, sort-by-ID, 2-space
    indent — rather than reimplementing serialization), and asserts the
    output is byte-identical to the committed file; passed on the first run
    (the codegen's nil-vs-`[]string{}` slice handling, `decl_order`
    per-node preservation — the graph has 307 requirements with a non-zero
    `decl_order`, not uniformly 0 as initially assumed — and `%q` escaping
    of Cyrillic/typographic-dash content in `R-authored-prose-no-live-
    tallies`'s claim/why all round-tripped correctly without a fix cycle).
    `TestMergeIntoGraph_Idempotent` proves a second merge pass is a no-op.
  - Extended `internal/selfcheck/contentfree_test.go`'s content-intake
    exemption (`isContentIntakeSite`, previously `internal/proposal/*` +
    `cmd/hotam/init_cmd.go` only) to include `internal/selfspec/*`: its
    `ontology.Requirement{...}` literals are codegen'd FROM the real graph
    (not illustrative examples) and its real `Owner`/business-content string
    literals (e.g. `"framework-reviewer"`) are exactly what
    `R-content-free-no-examples`/`R-content-free-no-business-data` exist to
    forbid everywhere else — `check_content_free_no_examples`/
    `check_content_free_no_business_data` correctly fired against the
    unmodified checks on first run, confirming the checks have real teeth
    before being deliberately, narrowly widened for this one new legitimate
    boundary.
  - No import-ratchet or race-ratchet change needed: `internal/selfspec`
    imports only `internal/ontology`+`internal/registry` (core-ward, not a
    periphery consumer), so `internal/selfcheck`'s `TestCorePeriphery_
    ImportRatchet` needed no new allowed-set entry (same as `internal/
    graphfacts`'s precedent, task #331/#334 wave); it spawns no goroutines,
    so `TestRaceRatchet_GoroutinePackagesCoveredByCI` needed no CI package
    list change either.
  - Value of this phase: authority-by-construction (a Requirement's
    structural shape becomes a reviewed Go diff, not a hand-edited JSON
    blob) and a reviewed-diff workflow for the highest-friction fields —
    explicitly NOT type safety. No invariant reads this registry yet (shadow
    mode starts in Phase A, the ~301-requirement scale-up); no CLI command
    writes through it (Phase B, the authority flip) — both future tasks.

- **Land 4 R4-wave requirements with explicit human signoff** (task #339,
  R4F-land-batch): four Requirements drafted during earlier engine-work
  tasks — whose capability was already built and verified in those tasks —
  are now landed into `hotam-spec-self`'s own graph, following
  `R-decided-needs-human-signoff`/`R-ai-presents-not-decides` (present,
  never decide; the resolver decided in an interactive conversation, one
  question per draft, before this task executed the landings).
  - `R-gate-cohort-explicit-denominator` (task #330, R4-cohort): a
    `gate_signoff_count` orientation_faq assert that also declares a
    manifest-level `gate_cohort` answers the stronger "have ALL cohort
    requirements passed this stage" question — a cohort member never
    assessed at the target stage counts AGAINST `expect:"all"` rather than
    staying invisible to it; and a `gate_signoff_count` assert fails closed
    when its target stage carries signoffs from more than one
    `pipeline_run` and the assert itself doesn't disambiguate via a
    declared `pipeline_run`.
  - `R-authored-prose-no-live-tallies` (task #333, R4F-prose-lint,
    generalizing task #331): authored prose fields narrating a domain's
    durable rationale (`Process.Why`/`Step.Why`, manifest `goals`/
    `charter`) must not carry a point-in-time status snapshot or a live
    tally — that belongs in a generated projection instead, enforced by
    `check_authored_prose_snapshot` (renamed/generalized from
    `check_process_why_snapshot_prose`).
  - `R-pipeline-live-state-from-typed-carriers` (task #331, R4-process-why):
    PIPELINE.md's current-status content must be generated from typed
    carriers (`Requirement.gate_signoffs`, Conflict lifecycle state) on
    every `hotam gen-spec` run, never carried as a snapshot embedded in
    authored `why` prose.
  - `R-requirement-update-signoff-typed` (task #335, R4F-req-signoff): a
    Requirement UPDATE or Assumption rewrite recording a real human
    decision must carry a typed signoff (`ontology.Signoff`) on the
    resulting HistoryEntry, whose `decided_by` resolves to a declared
    Stakeholder; `--decision-ref` remains for lighter mechanical
    acknowledgments only.

  All four landed via `hotam apply-proposal --decision-ref` (consistent
  with task #328's precedent — `hotam-spec-self`'s own domain declares no
  Stakeholder for the human resolver, so `--decision-ref` sidesteps that
  gap rather than inventing one). Two of the four drafts had gone stale
  since being written: `draft-R-pipeline-live-state-from-typed-carriers.json`
  still cited the pre-rename `process_why_snapshot.go`/
  `checkProcessWhySnapshotProse` (task #333, landed earlier in this same
  batch, renamed it to `authored_prose_snapshot.go`/
  `checkAuthoredProseSnapshot`), and
  `001-R-requirement-update-signoff-typed.json`'s `verified_by` cited only
  an ongoing-invariant test that never executes `resolveHistorySignoff`/
  `validateHistorySignoffShape` — both fixed as mechanical
  reference-correction, no claim/content change, mirroring task #328's own
  precedent for this kind of drift.

- **`-race` CI coverage ratchet, plus a fixed real gap it found** (task
  #336, R4F-race-ratchet — fourth external review's final synthesis §4.5):
  task #327's `test-race` job comment/CHANGELOG text claimed its
  hand-picked package list (`internal/gate`, `internal/generator`,
  `internal/invariants`) was "the only packages with real goroutine/sync
  usage in non-test code". That was already false: `cmd/hotam/common.go`'s
  `writeFilesParallel` used a real `go func` + `sync.WaitGroup` fan-out to
  write generated files concurrently, and `cmd/hotam` is deliberately
  excluded from `-race` wholesale (its e2e tests spawn a compiled
  subprocess `-race` on the parent process can't instrument) — so this one
  genuinely concurrent function silently rode along uncovered. Read
  confirms it was safe today: each goroutine writes to a distinct index of
  a pre-sized `errs` slice and a distinct file path, synchronized by
  `wg.Wait()` before the merge — no shared mutable state, no data race —
  but nothing mechanically connected "what has goroutines" to "what CI
  race-tests", so the gap could silently reappear and grow. Fixed two ways:
  (1) extracted `writeFileMkdir`/`writeFilesParallel` out of `cmd/hotam`
  into a new `internal/fsio` package (pure stdlib, no `cmd/hotam`-internal
  dependencies, zero behavior change — `cmd/hotam`'s own functions now just
  delegate) and added `internal/fsio` to the `test-race` CI job and
  `Makefile`'s `test-race-scoped`, keeping the rest of `cmd/hotam`'s
  e2e-heavy tests out of `-race` as task #327 intended, at far lower cost
  than adding all of `cmd/hotam` (measured at ~9-20 min under `-race` in
  task #327); (2) added `TestRaceRatchet_GoroutinePackagesCoveredByCI`
  (`internal/selfcheck/race_ratchet_test.go`), mirroring
  `TestCorePeriphery_ImportRatchet`'s shape: AST-scans every non-test `.go`
  file under `internal/` and `cmd/hotam/` for real `go` statements
  (`go/ast.GoStmt`, not a grep for the word "goroutine"), collects the
  packages that contain one, and cross-checks that set against the package
  list parsed directly out of `.github/workflows/ci.yml`'s `test-race`
  job's `go test -race` line — one source of truth (the YAML), no
  hand-maintained list duplicated in Go. A new package with a goroutine and
  no `-race` coverage now fails `go test ./internal/selfcheck/...`. Three
  non-vacuity controls prove the ratchet actually fires: a synthetic
  uncovered-package case, a synthetic AST comment-vs-real-statement case,
  and a synthetic YAML-parsing case.

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
