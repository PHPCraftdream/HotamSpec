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
- Atom proof state no longer reports a false STALE: `req show` derives an
  atom's fresh text from its method phrase and executed value (as sync does),
  so only a real drift between code and committed Claim is STALE. Atom SPEC
  narratives render that human sentence instead of the qualified method
  symbol; Holds/rule evidence and a failed atom's observed value appear as
  separate `Given` lines.
- Atom SPEC narratives preserve localized claim and evidence text per language,
  render the requested language, and reject missing translations; single-language
  output remains unchanged. The SPEC introduction and Russian/Chinese catalog
  now describe `check_spec_md_current` as the existing freshness check.
- Repo-hygiene scanning now exempts only exact frozen historical files; new
  proposal files are scanned for private machine paths. Gen-spec fixpoint shares
  the crystal-reader check-name set with invariants and reports checks that
  remain unconverged after its bounded iterations.
- Atom narrative order now follows the recorder's actual call order when one
  test proves same-named methods of different types (e.g. w.Role().Practice
  and w.Actually().Practice): each Fact/Holds artifact is matched back to its
  own call site by replaying the recorder's LIFO cleanup write stream, so the
  i-th call of a method name takes the i-th same-name call site instead of
  every same-named atom pinning to the first one.

- Gen-spec/land/sync crystal now converges in ONE run: the crystal-feeding
  violation snapshot is re-rendered to a bounded fixpoint against the run's
  own writes (crystal-reader checks only).

- The binary compile cache is now invalidated on the record-mode path too:
  both RunVerifiedByTest and RunVerifiedByTestRecording hash the module
  (hashPackageInputs) and drop the module's stale compiled test binaries
  when the hash changed mid-process, via the new shared
  syncCompileCacheToHash helper; previously only RunVerifiedByTest's
  verdict-cache mismatch could invalidate, so a source change between two
  RunVerifiedByTestRecording calls in one process could execute a stale
  pre-mutation binary.

### Tests

- New fast test layer: ~100 slow tests (real-domain crystal/gen-spec renders,
  compile-cache binary builds, `go build`/`go test` subprocess tests, e2e)
  now carry `testing.Short()` skip gates with explicit reasons, so `go test
  -short ./...` / `make test-fast` skips them; the full `go test ./...` mode
  is unchanged and still runs everything.
- Fast layer tightened to a measured wall time: 38 more end-to-end tests
  (24 cmd/hotam sync-self/land/apply-proposal/gen-spec flows,
  11 internal/selfspec atom-discovery fixtures that compile and run a nested
  module, 3 internal/gate atom-recording tests) are skipped under `-short`
  with reasons. A single `go test -short -count=1 ./...` went from 192 s to
  77 s wall (warm build cache, weak machine, all packages ok); the cheap
  atom-discovery checks and one apply-proposal end-to-end stay in the fast
  layer, and `go test ./...` still runs every test.

- All-violations cost profile measured and documented: the three most
  expensive invariants are check_scenario_executes_impl (~10s
  scenario-test-binary compiles), check_verified_by_test_passes (~6s
  verified_by `go test` subprocess runs, deliberately 2-worker-capped), and
  check_spec_md_current (~6s full SPEC.md re-render); phase-1 fan-out already
  overlaps them, warm-cache wall ~2s. No semantic changes.

### Commands

- New `hotam upgrade [--domain <path>] [--today YYYY-MM-DD]`: the one-shot
  post-engine-upgrade refresh for a consumer domain — re-vendors the recorder
  and the ontology mirror (skipped, not failed, when the domain has no
  spec/ Go module or never vendored them), refreshes an engine-generated
  `spec/registrydump/main.go` only when it carries the do-not-edit banner (a
  banner-less, hand-modified file is reported and left byte-identical),
  regenerates docs + the project crystal via the same gen-spec pipeline
  `hotam land` uses, re-rendering `docs/gen/SPEC.md` exactly when the domain
  needs it (a committed SPEC.md exists or the manifest declares
  discipline:"full"), then prints the final all-violations result
  (informational — the exit code stays 0 unless a real error occurs).
  Idempotent: a second run on an already-current domain rewrites nothing.

### Self-executing atoms in self-hosting

- Manifest fields `self_executing_atom_packages` and `atom_recorder_import_path`
  opt a domain into the atom pipeline without a separate spec module: the listed
  root-module packages are walked for atom subjects, and the recorder import
  path is taken explicitly from the manifest instead of derived from the spec
  module's `go.mod`.
- The atom pipeline is generalized from `spec/model` to any listed root-module
  package (subject walk, snapshot test discovery, discovery directory filter);
  consumer domains with a spec module are unchanged. `hotam sync-self` now
  merges atoms discovered by `selfspec.DiscoverAtoms` into the hand-maintained
  registry before `SyncGraph` (overrides resolved by `implemented_by[0]`, as in
  `sync-domain`).
- Localization pilot atoms: `internal/localization` ships self-hosted atoms
  (`Catalog.Supported`, `Catalog.Translated`, `MissingTranslation.Message`)
  proven by `hotamspec.Fact`/`Holds` tests, so `domains/hotam-spec-self` runs
  its own dogfood atoms from the root module.

### SPEC generation polish

- SPEC shard paths no longer double the `spec` segment: packages under the
  authored `spec/` tree map to `docs/gen/spec/<rest>.md` (previously
  `docs/gen/spec/spec/...`); packages outside `spec/` keep their nested path,
  colliding package→shard mappings are now a generation error, legacy doubled
  shards are removed as stale, and shard links in CLAUDE.md use the same
  mapping. Documents whose reader stakeholder cannot be resolved no longer
  print a `reader: (unresolved-reader)` line; resolved readers are unchanged.

### Atomic multilingual and conformance specifications

- `hotam init-project` now seeds the first `sync-domain`'s prerequisites with
  zero manual steps: it writes `spec/stakeholders.go` (seed requirement owner,
  `--owner <id>`, default `owner`), adds `atom_defaults` (owner, status
  SETTLED, created_at=settled_at=today) to the scaffolded manifest, and —
  because stakeholders.go exists before the registrydump scaffold runs — the
  first `spec/registrydump/main.go` already prints the
  `{"requirements":[...],"stakeholders":[...]}` envelope, so the first
  `sync-domain` no longer blocks on `check_no_dangling_requirement_owner`.

- Atom order in CLAUDE.md/SPEC projections now follows the authored test
  narrative instead of model source positions: test files and `TestXxx`
  functions in source order, proofs in `Fact`/`Holds` call order; a `Holds`
  relation sits at its own call site, so evidence `Fact` calls nested in its
  arguments follow the relation.

- Localized rendered atom VALUES: an executed value matching a typed string
  constant of the method's return type is substituted per language from that
  constant's `>>>>> lang=<code>` doc blocks (same strictness as phrases); a
  matched string constant without blocks is an error in a multilingual domain,
  and a single `>>>>> lang=*` block marks a verbatim, never-translated value.
  One-language domains are unchanged.
- Added authored `not:` negation phrases for bool atoms (including one per
  `>>>>> lang=<code>` block): bool `true` renders the bare phrase, `false`
  renders the negation, and a `false` verdict without a `not:` phrase is an
  error. `Holds` claims now carry only the predicate phrase; evidence methods
  stay in the artifact and SPEC observations. Existing bool-fact domains that
  can record `false` must add `not:` phrases or their claims error.
  Migration: after each bool-atom phrase that may record `false`, add a
  `// not: <negated phrase>` line (in every `>>>>> lang=<code>` block of
  multilingual methods); without it, claim derivation fails with an error.
- Added opt-in `languages` / `default_language` views with authored per-method
  `>>>>> lang=<code>` phrase blocks, localized claim maps and a single shared
  graph. Plain one-language comments and `Fact` behavior remain unchanged;
  multilingual phrases require exact declared-language coverage and never
  fall back to another locale.
- Added explicit rule/case recording: `WithCase` keeps one authored rule norm
  while independent test inputs and expectations identify stable cases.
  Case descriptors keep those oracle values separate from typed actual
  observations; text, bytes, booleans, scalars and diagnostics retain exact values.
- Added source-clause inventories, `MUST`/`SHOULD`/`MAY`, applicability,
  profiles, scoped precedence and composition provenance. The conformance audit
  reports structural issues relative to declarations; corpus pass, profile
  qualification and cross-language parity do not claim semantic completeness
  or automatic implementation blame.
- Automatic method discovery and rule-case permission remain separate opt-ins:
  `self_executing_atoms` enables the scan, while `conformance.rule_cases`
  enables the new case semantics. Neither activates the other's obligations.
- Localized document bundles use one execution/evidence/review snapshot,
  stage rendered outputs before publication, and remove only obsolete
  generator-owned files. Explicit locale and conformance fields activate their
  own freshness/audit checks rather than inheriting duties from older opt-ins.
- Added canonical self-hosted carriers for
  `check_language_bundle_complete`, `check_language_outputs_current` and
  `check_conformance_audit`, and updated the author contract, quickstart,
  proposal reference and README for the preserved plain path and new fields.
- Atom checks share the publication snapshot, including implementation-subject
  proof and compatible shared-case `verified_by`. A live three-locale consumer
  executes its three cases exactly three times.
- Preserve explicit empty selection lists through evidence JSON round-trips;
  deduplicate shared-case audit findings and count each discrepancy once.
  Report-only requirement summaries retain every observed case.
- Correct localized output resolution for conventional `domains/<name>` roots.
  Source/catalog and authored-collision refusals occur before any bundle write;
  locale add/remove and single/multilingual transitions preserve authored files.
- Proposal previews decode into independent graph collections. Sync previews
  use the current date when unpinned rather than introducing a 1970 history stamp.
- Unexecuted case findings retain declared profile/composition fingerprints
  without fabricated measurements. Changing that profile invalidates its
  corresponding review identity while unrelated profiles remain unchanged.

### Structured evidence, findings, source links and confrontation confidence

- Added `hotam evidence --json --write`: a fresh, deterministic report of
  successful and failing observations, including opted-in atoms before their
  first successful graph sync. Negative output is saved before nonzero exit;
  it does not mutate graph state or publish false passing normative text.
- Recorder `WithInput`, `WithContext`, `Observe`, and `Observed` preserve real
  input/actual/expected/verdict detail. Nested failures fail the real test even
  when its outer summary matches the expectation; methods execute once.
- Added `hotam findings list|show|review`. Human classification and rationale
  live separately from immutable observations and do not transfer to changed
  content-addressed finding IDs. No new persistent test-verdict cache.
- Added typed manifest specification sources, Requirement SourceLinks and
  authored Coverage qualifications. SHA/anchor checks and qualification
  invariants activate through their own explicit fields; they do not infer
  semantic completeness or manufacture versions.
- Coverage distinguishes verified evidence, discrepancies, unsupported
  recommendations, profile-qualified unreachable branches and unverified
  obligations. Observed failures take precedence over authored qualifications.
- Opposite-marker words now produce advisory lexical suspicion. Exact authored
  links strengthen relatedness; only an explicit unresolved Conflict carrier
  can block proposal/sync writes pending a recorded decision. Ack citations
  must cover the actual carrier, and lexical hits do not create ack history.
- Retired incidental wiring/count/source-text tests rather than re-pinning
  them. Strict proposal decoding uses real rejection tests as its carrier;
  the tool-projection wiring claim is honestly STRUCTURAL, not a copy-only
  test advertised as behavioral ENFORCED proof.
- Retired the test-only numeric debt pins instead of bumping their ceilings
  after exposing the old copy-only tool projection proof as STRUCTURAL.
  Debt classification remains tested; unverified obligations remain visible.
  Replaced the plumbing document snapshot with a behavioral partition guard:
  settled business/plumbing obligations are neither misplaced nor lost, and
  DRAFT obligations are not published as settled.
- Live smoke verified a failed nested year comparison plus a successful
  sibling, immutable human review, source drift, coverage qualification
  precedence and advisory unrelated only/any hits. Ktav remains paused;
  its source/backend and existing findings report were not resumed or patched.
- Final verification: `go vet ./...` and `go test -timeout 30m ./...` passed
  (22 packages OK, 2 without tests). Self-hosting sync regenerated 105 docs
  with zero violations. The final built CLI also preserved the complete
  source/method/test/observation trace and refused a valid proposal naming an
  unresolved formal carrier without changing its graph.

### Self-executing atoms (2026-10-01)

- Added per-domain `self_executing_atoms` opt-in and `atom_defaults` lifecycle
  metadata; existing scenario domains retain their contracts.
- Added recorder `Fact`, `Holds`, `Evidence`, and `Expect(false)`. Bound method
  identities survive pointer/generic receivers; anonymous and free functions
  are rejected. Renamed the old key/value pair type to `ValueFact`.
- `sync-domain` recursively discovers model tests, derives claims from method
  doc phrases and executed values, derives method/test links and relation
  evidence dependencies, and applies explicit registry exceptions. Method
  renames require retained REJECTED entries rather than dropping graph history.
- Added structural phrase, single-expression value-method, and single-subject
  Fact invariants. Atom claim freshness compares repeated observations without
  duplicating the claim; contradictory observations remain violations.
- Atom recording uses package-level native `go test -json` caching and
  replayable stdout artifacts; no new disk verdict cache. Source ASTs are
  indexed once per invocation. SPEC.md becomes a source-ordered package index
  with shards; freshness checks verify the index and all shards.
- Oversized consumer crystals expose package links and counters instead of
  an omitted requirement list. Updated Fact/Holds change workflow and guides.
- Verified Human migration, zero violations, and 1987 → 1988 propagation into
  SPEC and CLAUDE by editing only Init and the expected test value; restored
  1987 afterward. Proved native cache replay and independent package
  invalidation. Preserved the old #23 worktree: its uncommitted changes include
  a separate compiled-binary invalidation idea as well as the rejected disk
  verdict-cache implementation.
- Final verification: `go vet ./...`, full engine
  `go test -timeout 30m ./...`, and Human `go test ./...` passed. Anchored the
  three new enforcers through their own self-hosted requirement and projected
  it with `sync-self`; no orphan-enforcer exemption was introduced. Regression
  coverage also accepts named bound-method values and explicitly instantiated
  `Fact[int]` calls.

### Wave summary — code-authority-completion (tasks #388-#402, #404-#405)

Full plan: `docs/PLAN-code-authority-completion.md`. Individual task entries below carry the full
technical detail (file-by-file, verification-by-verification); this section is the connected
narrative a reader should start with.

**Trigger.** Two independent architectural reviews (one `@oh`-delegated, one separately produced)
converged on the same root cause: task #369 had wired a new mechanical obligation
(`check_claim_matches_scenario`) into an already-spent opt-in trigger (`discipline:"full"`),
breaking two real consumer domains (`PRAT-hotam/domains/prat`, `gpsm-sm`) with a live regression —
20 and 23 violations respectively, zero action on their part — while the "any information system
describable as objects/fields/methods/tests" promise was still badly incomplete on the model side
(interfaces/constructors/consts invisible to the scan, no mock/port classification, no
EntityType-to-Go link, no symmetric public-surface-completeness check, no scenario content-quality
gate).

- **W0 — stop the bleeding (#388-#392).** Fixed the regression by giving `check_claim_matches_scenario`
  its own opt-in trigger (`claim_authority:"scenario"`) instead of weakening the check; deduped
  scenario-title derivation per-`verified_by`-entry; restored a `Why` field silently dropped from a
  vendored mirror; unified vendor/generated-file exclusion across three producers; added
  `ProposedGoal`/`ProposedEntityInstance` JSON-proposal kinds. Established the wave's central,
  repeatedly-cited law: **an already-spent opt-in trigger is closed to new obligations — a new duty
  needs its OWN, separately-declared, separately-ratcheted trigger**
  (`R-opt-in-trigger-owns-its-own-obligations`).
- **W1 — complete the model half (#393-#397).** AST scan extended to interface methods,
  constructors, typed consts, struct tags, doc comments (previously invisible); added
  `object`/`value`/`port`/`mock`/`policy` semantic classification (mocks are legitimate executable
  surrogates, never structurally exempted); linked `EntityType.ModelSymbol` to real Go symbols and
  REJECTED (not fulfilled) a self-hosted requirement that had been demanding the wrong direction
  (Go-generation-from-EntityType); added a symmetric "every public symbol linked+scenario'd or
  explicitly marked infrastructure/ignored" check (own trigger, `public_surface_authority:"linked"`);
  added a scenario content-quality gate — non-empty title, exact reqID match, real
  Given-before-When-before-Then ordering (own trigger, `scenario_authority:"quality"`).
- **W2 — make proof visible and cheap (#398-#400).** Surfaced `RequirementState()` in `hotam req
  show`/`hotam brief` (deliberately not in `list`/`search`, which would multiply real-test-execution
  cost across a whole roster, and not in `TRACEABILITY.md`/`COVERAGE.md`, which task #317 made
  execution-independent for `hotam land`'s idempotency); built a five-section evidence packet into
  `hotam brief` (signatures, port/mock contract, real Given/When/Then, related entity/process),
  promoting a citation-matcher from `internal/invariants` up into `internal/gate` along the way
  (a layering fix, `internal/query` must not depend on the enforcement layer); added an
  engine-identity content-hash fingerprint (deliberately not the raw git commit SHA, which would
  invalidate on every unrelated commit) stamped into `docs/gen/ENGINE-VERSION.md`, with an
  unconditional freshness check.
- **W3 — prove it on a real vertical slice (#404-#405, replacing the original umbrella task #401).**
  Founded the `life` domain (a separate external repository, `life`) with 7 objects, one real
  Lifecycle, one port+mock, 7 requirements with real scenarios — the first real domain to opt into
  `discipline:"full"` plus both of this wave's new triggers — then ran a genuinely fresh cold-start
  AI evaluation (15 pre-written questions, pre-written answer key, zero prior context). Result: 14/15
  (93%) correct, median 1 tool call per question (bar: ≤5), ~15,000 tokens for all 15 questions
  combined (bar: ≤15k per question; the plan's own cited baseline was ~54k for one question). Full
  report: `docs/reviews/2026-07-31-life-methodology-acceptance.md`.
- **W4 — reconcile docs with reality (#402).** Fixed 5 documentation-vs-reality gaps, including a
  real correction found via ground-truth measurement: the plan's own "289 of 302 requirements lack a
  carrier" estimate was dramatically wrong (didn't account for `enforced_by` alone satisfying the
  gate) — the real, measured number is 43.

**Verification discipline.** Every task was independently zero-trust-verified before committing:
the actual diff read, build/vet/gofmt/targeted-tests/all-violations-on-both-self-domains/full-suite
all re-run personally, and in three cases a delegate's own claimed "refutation test" (deliberately
breaking a check to confirm it fires, then reverting) was personally reproduced rather than trusted.
One cross-domain staleness gap a delegate missed (task #396: `hotam-dev`'s `CLAUDE.md` referencing
`hotam-spec-self`'s pulse figures) was caught by the resolver's own `all-violations` run and fixed
before committing; every subsequent task's brief was updated to warn about this exact class of gap.

**A separate, related deliverable**: mid-wave, a read-only test-suite-speed investigation
(`docs/reviews/2026-07-30-test-suite-speed-analysis.md`, commit `9c930c5`) found the session's own
`go test ./...` runs were the dominant time cost, largely from concurrent-agent contention on this
shared workspace; its top recommendation (one full-suite run per commit boundary, never concurrent
with another agent) informed how later tasks in this wave were verified.

### Changed
- Split the five largest engine files by responsibility — move-only, no behavior or generated-output
  change (review P2-4): `internal/localization/localization.go` → per-locale catalog files
  (`catalog_ru.go`/`catalog_zh.go`); `internal/gate/test_exec.go` → execution / record-mode /
  input-hashing / concurrency-control; `internal/gate/spec_build.go` → row collection / rendering /
  shards+index; `internal/gate/atom_source.go` → source index / doc-phrase parsing / value constants /
  claim derivation; `cmd/hotam/gen_spec.go` → command+pipeline / loading / cleanup / project-framework
  files. Added `TestCatalogCoversAllTextLookupTemplates`: a static go/ast completeness check — following
  templates through wrapper functions whose template parameter reaches `localization.Text`/`Lookup`
  (resolved transitively, e.g. generator.serviceText, gate.specText/atomText) — that every
  literal-or-constant template across internal/ and cmd/ exists in both the ru and zh catalogs.
- **Documentation reconciliation: corrected five claims where the docs described a different reality
  than the code (task #402, W4.1).** An agent reading the docs got two incompatible instructions about
  who generates what and what counts as truth; this task closes those gaps with no graph-content change
  and no new code behavior — all 27 Go-file edits are comment-only.
  - **`internal/selfspec/requirements_*.go` (27 files):** replaced the misleading `// Code generated …
    DO NOT EDIT BY HAND` banner (which pointed at a throwaway codegen program that was never committed
    to git and went graph→Go — the opposite of today's sync direction) with an honest banner stating the
    files are HAND-MAINTAINED Go source, kept in sync Go→graph via `hotam sync-self`. Comment-only.
  - **`docs/AUTHORED-SPEC-CONTRACT.md`:** (a) §8.2 — the claim "no manual text in `docs/gen/` at all"
    was false: measured authored prose is ~12% of `docs/gen/*.md` on the leanest domain (hotam-dev) and
    ~49% on hotam-spec-self (the authored `Claim` alone is ~54% of `REQUIREMENTS.md`); the files are
    regenerated but their normative body is projected human-authored `Claim`/`Why`, not pure derived
    structure. (b) §0 — disclosed the declared asymmetry that `hotam-spec-self` does NOT opt into
    `discipline:"full"` (so `check_settled_requires_scenario` is a no-op for it): a measured temporary
    flip yields 43 violations, NOT ~289 as previously estimated — the old estimate ignored that
    `enforced_by` alone satisfies the gate (180 of 259 SETTLED requirements pass that way; 36 more via
    `INHERENTLY_PROSE`). (c) new §11 documents the code-authority path (`requirements_authority:"code"`,
    `vendor-ontology`/`scaffold-registrydump`/`sync-domain`), previously absent from the contract.
  - **`README.md`:** `hotam init`/`init-project` described a "seed Stakeholder + seed Requirement" that
    task #364 retired (both now write a genuinely empty graph); `init-project` also now defaults to
    `--discipline full` (BORN FULLY OBLIGATED) — corrected both descriptions.
  - **`docs/QUICKSTART-CONSUMER.md`:** same seed→empty-graph correction throughout, plus a new concise
    "Code-authority" section (the quickstart previously documented only the JSON-proposal path).
  - **`docs/PLAN-code-authority-completion.md`:** `check_method_matches_docstring` was listed as a
    "пустышка … implement or delete"; corrected to match the check's own (already-correct) `Why`: it is
    an architecturally-justified honest no-op (`Claim`/`Rule`/`Why`/`Check` are fields of one struct
    registered together, so docstring/body drift cannot arise), not unfinished work.

### Added
- **Engine identity fingerprint stamped into every domain's generated docs via a new `docs/gen/ENGINE-VERSION.md`,
  with a mechanical freshness check (`check_engine_docs_fingerprint_current`) that fires when the stamped fingerprint
  disagrees with the current engine's own (task #400, W2.3).**
  Consumer-generated documentation can silently lag behind engine evolution: the engine changes in its own commits,
  the consumer's generated docs sit in a separate repository, and nothing told the consumer's resolver "the engine
  that produced these docs is not the engine you have now." This exact class of problem caused the W0.1 regression
  (task #369's engine upgrade retroactively broke domains in another repository with zero warning). This task closes
  that gap.
  - **`gate.EngineDocsFingerprint(moduleRoot string) (string, error)` (new, `internal/gate/engine_fingerprint.go`):**
    a deterministic sha256 content-hash over the three generator-relevant packages (`internal/generator`,
    `internal/ontology`, `internal/loader`) — the set whose changes affect a domain's generated-doc shape/content.
    Deliberately NOT a raw git commit SHA (which would invalidate on every unrelated commit, far too noisy) NOR a
    manually-bumped version number (which defaults to "dev"/"unknown" for local builds). A new scope-limited
    `hashDirContent` helper walks one directory at a time (same algorithm philosophy `hashPackageInputs` uses, but
    scoped to a single package dir — `hashPackageInputs` itself ignores its `pkgDir` parameter and hashes the whole
    module, load-bearing for `runCache`/`coverageRunCache` cache invalidation and untouched by this task). The
    three per-dir hashes are sorted lexicographically before a final sha256 combination, so the result is
    deterministic regardless of which package is hashed first.
  - **`generator.BuildEngineVersionMD(moduleRoot string) (string, error)` (new, `internal/generator/engine_version.go`):**
    renders `docs/gen/ENGINE-VERSION.md` — the `generatedHeaderComment` banner, a short 16-char hex fingerprint, and
    a human-readable note directing the reader to regenerate via `hotam gen-spec` when the fingerprint disagrees.
    Written unconditionally by genSpec (not content-gated — engine metadata, not a graph-derived projection), but
    silently skipped when the fingerprint can't be computed (a temp/test domain without engine source, or a consumer
    repo whose engine is a compiled binary), mirroring the invariant check's own honest-no-op degradation.
  - **`check_engine_docs_fingerprint_current` (new, `internal/invariants/engine_version_current.go`):** the 122nd
    registered invariant. Reads the domain's `docs/gen/ENGINE-VERSION.md` (if present), extracts the stamped
    fingerprint, computes the current engine's own via `gate.EngineDocsFingerprint`, and fires one violation with an
    actionable message when they differ. Honest no-op when the file is absent, when the stamped value is unparseable,
    or when the current engine's fingerprint can't be computed — the same unconditional-but-honest-no-op-when-absent
    class as `check_spec_md_current`/`check_domain_claude_md_current`. No opt-in trigger needed (unlike
    `check_claim_authority_ratchet`/`check_public_surface_authority_ratchet`): this is not a new obligation riding an
    already-spent trigger.
  - **`R-engine-docs-fingerprint-current` (new anchor, `internal/selfspec/requirements_deterministic.go`):** ENFORCED
    by `check_engine_docs_fingerprint_current`, projected onto the graph via `hotam sync-self`.
  - **Scope boundary.** Does not touch `internal/generator/traceability.go`, `coverage.go`, or any existing
    `docs/gen/*.md` banner — Decision 2's dedicated new file is the entire stamp surface. The fingerprint is a pure
    function of the checked-out source tree (byte-idempotent across two consecutive gen-spec runs from the same
    unchanged engine binary), so stamping it does not violate task #317's byte-idempotency guarantee.

- **`hotam brief` now assembles a five-section evidence packet for a Requirement anchor -- object/method
  signatures, port/mock contracts, the actual Given/When/Then scenario narrative, related entity/process
  links, and per-section provenance -- assembling into one single-call answer what previously required
  reading the graph, the Go source, the generated docs, and the tests separately (task #399, W2.2).**
  An AI agent orienting on one Requirement today had to cold-start across 3-4 separate round-trips
  (~54k tokens, 8.6 tool calls, 60s per question in a saved evaluation run). The four content sections
  are all Requirement-only -- nil/omitempty for Conflict/Assumption anchors, exactly like `Freshness`
  already is -- and each is independently honest-absent: a stale/malformed citation, a missing scenario,
  or no EntityType match simply omits that section rather than erroring.
  - **Promoted `gate.MatchCitedSymbol` + `gate.SymbolKind`.** The citation-resolution machinery
    (`matchCitedSymbol`/`symbolKind`/`splitQualifiedCitationSymbol`/`normalizeRelPath`) was previously
    UNEXPORTED inside `internal/invariants/model_complete.go` -- the enforcement/violation layer.
    `internal/query` (a read-only presentation layer) must NOT import `internal/invariants` (a real
    layering violation with a latent import-cycle risk), so the matcher was promoted to `internal/gate`
    (a true leaf both layers already depend on) as the EXPORTED `MatchCitedSymbol` + `SymbolKind` enum
    (`internal/gate/citation_match.go`). The three-category priority order (receiver methods → interface
    methods → bare-only top-level funcs) is preserved byte-for-byte. `internal/invariants` was refactored
    to call `gate.MatchCitedSymbol`/`gate.SymbolKind` directly -- **behavior-preserving**, proven by a
    zero-line `git diff --stat` on both `model_complete_test.go` and `model_complete_symmetric_test.go`.
  - **New `BriefCard` fields (`internal/query/brief.go`).** `Signatures []EvidenceSignature`,
    `PortContracts []EvidencePortContract`, `Scenario *EvidenceScenario`, `EntityProcess
    *EvidenceEntityProcess` -- computed by `buildEvidence` in the new `internal/query/evidence.go`.
    Each section carries its own `Source` provenance string (e.g. `"gate.ScanAuthoredModels +
    gate.MatchCitedSymbol of spec/model/risk.go"`, `"live go test execution of TestValidate_Scenario
    (verdict: pass)"`, `"graph.json EntityType.model_symbol reverse-match against implemented_by files"`).
  - **Section 1 (signatures):** for every `implemented_by` entry that resolves via `gate.MatchCitedSymbol`,
    the owning `ModelObject`'s Name/Kind/ModelKind/Doc plus the resolved symbol's own signature+doc.
    Section 2 (port/mock contract): when the resolved object is a port (interface), its full
    `InterfaceMethods`; when a mock, its own `Methods` (a known simplification that does NOT
    cross-reference back to the specific port it implements -- noted rather than building a reverse index).
    Section 3 (scenario): the first `verified_by` entry that AST-carries a scenario
    (`gate.ResolveSpecTest`'s `HasScenario`) and produces at least one `Verdict=="pass"` artifact via
    `gate.RunVerifiedByTestRecording` -- its Title plus full ordered Steps. Deliberately does NOT apply
    task #397's four-rule quality gate (presentational, not a compliance check). Section 4 (entity/process):
    reverse-match implemented_by files against `EntityType.ModelSymbol` fields, surfacing matched
    EntityTypes and any `Process` whose `DrivesEntities` includes their Slug.
  - **Scope boundary.** `hotam brief` operates on ONE anchor at a time -- the `gate.ScanAuthoredModels`
    + one `gate.RunVerifiedByTestRecording` call per brief invocation is the SAME acceptable, already-
    established cost class task #398's `State` field pays. NOT wired into `hotam req list`/`search`
    (same reasoning). Does not touch `internal/generator/traceability.go`, `coverage.go`, or any
    `docs/gen/*.md` rendering path (task #317's byte-idempotency guarantee for `hotam land`).
    No new opt-in trigger/invariant/ratchet -- a pure read-path enhancement over already-computed data.
  - New tests: `internal/gate/citation_match_test.go` (10 tests: qualified/bare receiver method,
    interface method, top-level func, qualified-cannot-match-func, unexported excluded, wrong file,
    type-only excluded, priority order, path normalization), `internal/query/evidence_test.go` (10 tests:
    receiver-method+func signatures, port contract, mock contract, real passing scenario narrative via
    vendored recorder, honest-absent no-scenario, entity/process reverse-match, nothing-matches,
    Conflict/Assumption never populate evidence, stale citation omitted), `cmd/hotam/brief_test.go`
    (`TestCmdBrief_JSONIncludesEvidenceKeys` against real CLI stdout via `captureStdout`).

- **`hotam req show` and `hotam brief` now surface a Requirement's COMPUTED proof lifecycle state
  (NO_CARRIER / UNVERIFIED / FAILING / STALE / PROVEN) as a `proof state:` line in human-readable
  output and a `state` key in `--json` output (task #398, W2.1).** `selfspec.RequirementState`
  (task #370, `internal/selfspec/requirement_state.go`) already computed this state by re-running
  a Requirement's `verified_by` test(s) via `gate.RunVerifiedByTestRecording` -- the SAME real
  `go test` subprocess execution `check_claim_matches_scenario`/`DeriveClaimsFromScenarios` already
  use -- but the production read path (`hotam req show`/`hotam brief`) never displayed it. An agent
  or human looking at a Requirement card could see Status (SETTLED/DRAFT), Enforcement
  (ENFORCED/PROSE/STRUCTURAL), and the carrier entries themselves, but had no way to tell whether
  the evidence those carriers point at actually passes RIGHT NOW: no implementation at all
  (NO_CARRIER), a declared carrier that does not resolve (UNVERIFIED), a failing test (FAILING),
  stale evidence whose narrated scenario no longer matches the committed Claim (STALE), or current,
  passing evidence (PROVEN) are now all distinguishable at a glance.
  - **Wiring.** `RequirementCard` (`internal/query/show.go`) gains a `State string` field
    (`json:"state"`), populated inside `ShowRequirement` by calling
    `selfspec.RequirementState(r, gate.SpecRootForGraph(g), g.SelfHosting)` -- mirroring exactly how
    `internal/selfspec/claim_derive.go`'s own callers already obtain these two values from a
    `*ontology.Graph`. `FormatRequirementCard` (`internal/query/format.go`) renders it as a
    `proof state:` line positioned alongside enforcement (after `enforcement:`, before
    `enforced_by:`). `Brief` (`internal/query/brief.go`) needs no change: its `briefRequirement`
    composes a `RequirementCard` via `Context` → `ShowRequirement`, so the `State` field and its
    render line flow through automatically. Conflict/Assumption anchors are entirely unaffected --
    a Conflict/Assumption has no `verified_by`-driven proof lifecycle, and their formatters
    (`FormatConflictCard`/`FormatAssumptionCard`) do not reference `State` at all.
  - **Scope boundary.** Deliberately `hotam req show`/`hotam brief` ONLY, for a SINGLE Requirement
    at a time -- NOT `hotam req list`/`search`/`context`/`related`. `RequirementState` spawns a
    real `go test` subprocess per `verified_by` entry; computing it for every listed Requirement in
    a roster (hotam-spec-self alone has 258+) would be the exact class of severe, silent performance
    regression this session's own `docs/reviews/2026-07-30-test-suite-speed-analysis.md` flagged.
    `hotam req show`/`hotam brief` compute FRESH state on every invocation (a live query, not part
    of `hotam gen-spec`'s generated-doc output) -- they have no byte-idempotency contract to violate,
    exactly like `hotam status`/`hotam what-now` already do. `docs/gen/TRACEABILITY.md`/`COVERAGE.md`
    (task #317's execution-independent documents) are untouched.
  - New tests: `internal/query/state_test.go` (`ShowRequirement` populates State across two distinct
    real states -- NO_CARRIER for empty VerifiedBy, UNVERIFIED for an unresolvable entry, driven
    through the REAL `RequirementState` call against real fixture graphs; `FormatRequirementCard`
    renders the proof state line in the right position; `RequirementCard` JSON includes the `state`
    key; Brief on Conflict/Assumption anchors renders no spurious state content, Brief on a
    Requirement anchor does); `cmd/hotam/req_test.go` (`captureStdout` helper + JSON shape test
    asserting the `state` key round-trips through the actual CLI `--json` output).
  - Verified: `go build`/`go vet`/`gofmt` clean (excl. pre-existing untracked `.scratch` draft),
    targeted tests (`internal/query`, `internal/selfspec`, `cmd/hotam`) all pass,
    `all-violations` 0 on both `domains/hotam-spec-self` and `domains/hotam-dev`, full
    `go test ./...` green.

- **New check `check_scenario_quality` — a POST-HOC QUALITY gate over a requirement's already-recorded scenario
  artifact, sitting on top of `check_settled_requires_scenario`'s cheap AST-only "has a scenario at all" signal,
  gated behind its own brand-new opt-in trigger (task #397, W1.5).** `hotamspec.NewScenario` presence was
  detected purely by AST (does the test body call it anywhere) — a test that does `s :=
  hotamspec.NewScenario(t, "R-x", ""); s.Then("", true)` formally satisfied that bar with an empty title, an
  empty `Then` description, and no guarantee of sensible Given/When/Then ordering, directly undermining "a
  claim cannot exist without a GREEN and MEANINGFUL proof beside it."
  - **Four quality rules**, evaluated against every PASS-verdict artifact of every scenario-carrying
    `verified_by` entry (via `gate.RunVerifiedByTestRecording`, the same call shape
    `check_claim_matches_scenario`/`DeriveClaimsFromScenarios` already use — never a bespoke AST parser reading
    `Given`/`When`/`Then` call literals, which would be fragile against dynamic titles, helper indirection, and
    loops): (1) non-empty title; (2) the artifact's own `req_id` exactly matches the citing requirement's ID;
    (3) at least one `Then` step; (4) for a BEHAVIORAL scenario (carries a `When` step), a strict
    Given-before-When-before-Then order (`Value` steps ignored) — a DECLARATIVE scenario (zero `When` steps,
    the explicit "no action step" signal) is exempt from rule 4 entirely. A requirement is COMPLIANT iff AT
    LEAST ONE checked artifact across ALL its scenario-carrying `verified_by` entries satisfies all four rules
    (OR-across-entries, mirroring `anyVerifiedByEntryHasScenario`'s own semantics). An AST prefilter
    (`gate.ResolveSpecTest`'s `HasScenario`) skips any `verified_by` entry with no scenario constructor call at
    all — zero execution cost for a requirement `check_settled_requires_scenario` already covers.
  - **Own, independent opt-in trigger.** New manifest.json key `scenario_authority: "quality"`
    (`loader.ScenarioAuthorityQuality`, `internal/loader/loader.go`/`manifest.go`), wired into `LoadGraph` and
    `ontology.Graph.ScenarioAuthorityQuality` (`json:"-"`). Deliberately NOT co-gated with `discipline:"full"`
    (mirrors `public_surface_authority:"linked"`'s own precedent, not `claim_authority:"scenario"`'s, which
    uniquely requires `discipline:"full"` as a co-requirement) — `check_settled_requires_scenario`/
    `check_model_complete` (`discipline:"full"`) and `check_claim_matches_scenario`/
    `check_public_surface_linked_or_marked` (their own separate triggers) already have real consumer-domain
    obligation sets tied to their CURRENT "has a scenario" bar; tightening any of them instead of giving this
    new demand its own trigger would reproduce the live regression task #369 caused and task #388 fixed.
    Ratcheted one-way, mirroring `check_public_surface_authority_ratchet`'s exact shape: `graph.lock` gained
    `ScenarioAuthorityQualityObserved` (`internal/loader/lock.go`, `loader.WriteLock`/
    `loader.ReadScenarioAuthorityPin`), and new invariant `check_scenario_authority_ratchet`
    (`internal/invariants/scenario_authority_ratchet.go`) fires if a domain ever observed with
    `scenario_authority:"quality"` later reports a manifest without it.
  - **Performance note (corrects an initial assumption in the task brief):** `gate.RunVerifiedByTestRecording`
    carries NO verdict memoization of its own — its own doc comment states plainly it has "no in-memory
    memoization or singleflight collapsing at all... every call spawns its OWN real `go test` subprocess." The
    layer that DOES help a repeated call against an unchanged package is the LOWER, compile-artifact cache
    (`internal/gate/compile_cache.go`'s `compileCache`, keyed by `(moduleRoot, pkgPattern, coverPkgPattern)`):
    it skips a repeat `go test -c` (~42% of a cold engine pass per `docs/reviews/2026-07-30-test-suite-speed-
    analysis.md` §1.7) but the run itself always re-executes — a real, fresh subprocess run each time, not a
    cache hit. The recorder's own byte-for-byte determinism guarantee (proved by
    `TestRunVerifiedByTestRecording_Deterministic_TwoRunsByteIdentical`) is what makes two independently-run
    calls agree on content regardless.
  - New self-hosting anchor requirement `R-scenario-authority-owns-its-own-obligations`
    (`internal/selfspec/requirements_authoredspec.go`, `refines R-opt-in-trigger-owns-its-own-obligations`).
    `EnforcedBy` names both new checks (`check_scenario_quality`, `check_scenario_authority_ratchet`). Landed
    via `hotam sync-self` (confront-gate opposite-marker hits against 16 unrelated requirements were reviewed
    and confirmed lexical false positives — shared ONLY/ANY/MUST/MUST-NOT vocabulary, e.g. sharing only the
    word "match" with `R-m-tag-format-valid` — recorded via `--decision-ref`).
  - `internal/invariants/registry_complete_test.go`'s registered-invariant count moves 119 → 121 (two new
    checks); `internal/selfspec/merge_test.go`'s `wantRequirementCount` and
    `internal/loader/loader_domain_test.go`'s requirement-count assertion both move 306 → 307.
  - **This wave opts NO real domain into `scenario_authority:"quality"`** — `domains/hotam-spec-self`,
    `domains/hotam-dev`, and everything under `PRAT-hotam` stay honest no-ops for this check; `all-violations`
    confirmed 0 on both `domains/hotam-spec-self` and `domains/hotam-dev` after landing (`domains/hotam-dev`'s
    own generated `CLAUDE.md` needed one `hotam gen-spec --claude-md` regen after `hotam-spec-self`'s SETTLED
    count bumped, mirroring task #396's own cross-domain staleness catch).
  - New tests: `internal/loader/scenario_authority_test.go` (`resolveScenarioAuthorityQuality`
    true/false/absent/malformed cases, `LoadGraph` end-to-end wiring),
    `internal/invariants/scenario_authority_ratchet_test.go` (mirrors
    `public_surface_authority_ratchet_test.go`'s full regression/no-op/independence suite),
    `internal/invariants/scenario_quality_test.go` (all four quality rules individually, against REAL recorded
    fixtures built from the vendored `hotamspec` canon and run via `gate.RunVerifiedByTestRecording` — never
    hand-constructed fake JSON — plus the OR-across-entries compliance case, the honest-no-op cases, and a
    fully-compliant green case).
  - Verified: `go build`/`go vet`/`gofmt` clean (excl. pre-existing untracked `.scratch` draft), targeted tests
    (`internal/loader`, `internal/invariants`, `internal/ontology`, `internal/gate`, `internal/selfspec`,
    `cmd/hotam`) all pass, full `go test ./...` green.

- **New check `check_public_surface_linked_or_marked` — the SYMMETRIC INVERSE of `check_model_complete`: every
  exported authored symbol (receiver method, interface method, or top-level function/constructor) must be
  EITHER cited + scenario-complete OR explicitly marked infrastructure/ignored, gated behind its own brand-new
  opt-in trigger (task #396, W1.4).** `check_model_complete` only ever asked "for every method ALREADY cited as
  `implemented_by` by a SETTLED requirement, is it scenario-complete?" — a public method/interface-method/
  constructor no requirement has ever cited was completely invisible to that check, so a domain's model could
  silently accumulate uncovered public surface with zero gate noticing. This task was blocked on task #393
  specifically because "symmetry only makes sense once the scan sees the WHOLE public surface, including
  interface methods and constructors" — before #393, `gate.ScanAuthoredModels` could not see those categories
  at all (`ModelObject.InterfaceMethods`, `ModelFile.Funcs`).
  - **Compliance rule.** For every exported symbol in the domain's scanned model inventory
    (`ModelObject.Methods`, `ModelObject.InterfaceMethods` with `Name != ""` — `Embedded` entries skipped, they
    are not this object's own declared method — and `ModelFile.Funcs`), the symbol is compliant iff (a) it is
    cited by at least one SETTLED requirement's `implemented_by` AND that citation's aggregated
    `anyVerifiedByEntryHasScenario` is true (the IDENTICAL bar `check_model_complete` already applies), OR (b)
    its own doc comment carries the literal substring `"INFRASTRUCTURE:"` or `"IGNORED:"` followed by
    non-empty trimmed trailing text (`infrastructureOrIgnoredReason`, new file `internal/invariants/
    model_complete_symmetric.go`). A bare marker with nothing after it does NOT count as marked — a reason is
    required. The marker is deliberately an ordinary-prose reserved-word convention, NOT a `//directive:`-line
    convention: `gate.docText` joins a `*ast.CommentGroup` via `(*ast.CommentGroup).Text()`, which
    AUTOMATICALLY STRIPS Go-directive-shaped lines (e.g. `//go:generate`) from its output, so a bare directive
    marker would be silently deleted before `Doc` ever captured it.
  - **Own, independent opt-in trigger — the single most important design constraint.** New manifest.json key
    `public_surface_authority: "linked"` (`loader.PublicSurfaceAuthorityLinked`,
    `internal/loader/loader.go`/`manifest.go`), wired into `LoadGraph` and `ontology.Graph.PublicSurfaceAuthorityLinked`
    (`json:"-"`, mirrors `ClaimAuthorityScenario`'s shape exactly). This check does NOT activate on
    `discipline:"full"` — `check_model_complete` and its three siblings plus `check_discipline_ratchet` are ALL
    already bundled under that trigger, and two real consumer domains (`PRAT-hotam/domains/prat`,
    `domains/gpsm-sm`) had ALREADY flipped it long before this check existed, consenting only to the
    obligation set live at that time. Tacking this new obligation onto `discipline:"full"` would have
    reproduced EXACTLY the live regression task #369 caused and task #388 fixed (`R-opt-in-trigger-owns-its-
    own-obligations`): both domains would jump from 0 to many violations overnight, since neither has ever
    cited an interface method/constructor nor added any infrastructure/ignored marker anywhere. Unlike
    `claim_authority:"scenario"` (which requires `discipline:"full"` IN ADDITION), `public_surface_authority:
    "linked"` is deliberately NOT co-gated with `discipline:"full"` at all — it is sufficient and necessary on
    its own. Ratcheted one-way, mirroring `check_claim_authority_ratchet`'s exact shape: `graph.lock` gained
    `PublicSurfaceAuthorityLinkedObserved` (`internal/loader/lock.go`, `loader.WriteLock`/
    `loader.ReadPublicSurfaceAuthorityPin`), and new invariant `check_public_surface_authority_ratchet`
    (`internal/invariants/public_surface_authority_ratchet.go`) fires if a domain ever observed with
    `public_surface_authority:"linked"` later reports a manifest without it.
  - **Citation-collection refactor, not a re-implementation.** `check_model_complete`'s former inline citation-
    collection loop is now the shared `collectCitedSymbols` helper (`internal/invariants/model_complete.go`),
    generalized from `matchCitedExportedMethod` (receiver methods only) to `matchCitedSymbol` (receiver
    methods, interface methods, AND top-level funcs, in that priority order per candidate object/file — a
    qualified `"Type.Symbol"` citation never accidentally resolves to an unrelated func of the same bare
    name). `check_model_complete` filters `collectCitedSymbols`' output back down to receiver methods only, so
    its own behavior is UNCHANGED — `internal/invariants/model_complete_test.go` has a ZERO-line diff, proving
    the refactor did not alter that check's output.
  - New self-hosting anchor requirement `R-public-surface-authority-owns-its-own-obligations`
    (`internal/selfspec/requirements_authoredspec.go`, `refines R-opt-in-trigger-owns-its-own-obligations` —
    the concrete INSTANCE of that general law for this specific new check, not a restatement of it, mirroring
    `R-scenario-spec-obligations-mechanically-enforced`'s own relationship to its four gates). `EnforcedBy`
    names both new checks (`check_public_surface_linked_or_marked`, `check_public_surface_authority_ratchet`).
    Landed via `hotam sync-self`.
  - `internal/invariants/registry_complete_test.go`'s registered-invariant count moves 117 → 119 (two new
    checks); `internal/selfspec/merge_test.go`'s `wantRequirementCount` and
    `internal/loader/loader_domain_test.go`'s requirement-count assertion both move 305 → 306.
  - **This wave opts NO real domain into `public_surface_authority:"linked"`** — `domains/hotam-spec-self`,
    `domains/hotam-dev`, and everything under `PRAT-hotam` stay honest no-ops for this check; `all-violations`
    confirmed 0 on both `domains/hotam-spec-self` and `domains/hotam-dev` after landing.
  - New tests: `internal/loader/public_surface_authority_test.go` (`resolvePublicSurfaceAuthorityLinked`
    true/false/absent/malformed cases, `LoadGraph` end-to-end wiring),
    `internal/invariants/public_surface_authority_ratchet_test.go` (mirrors
    `claim_authority_ratchet_test.go`'s full regression/no-op/independence suite),
    `internal/invariants/model_complete_symmetric_test.go` (`infrastructureOrIgnoredReason` marker parsing,
    `check_public_surface_linked_or_marked` across all combinations of cited/uncited × complete/incomplete ×
    marked/unmarked, across all three symbol categories, plus the honest-no-op case).
  - Verified: `go build`/`go vet`/`gofmt` clean (excl. pre-existing untracked `.scratch` draft), targeted tests
    (`internal/loader`, `internal/invariants`, `internal/ontology`, `cmd/hotam`) all pass, full `go test ./...`
    green.

- **`ontology.EntityType` gains `ModelSymbol`, a one-directional link from a graph EntityType node to the Go
  type in a domain's authored `spec/model/` that realizes it; `R-generations-inherit-doc-test-code`'s
  EntityType-generation half is REJECTED and replaced by two atomic successors (task #395, W1.3).** Two
  previously-unconnected object catalogs — `ontology.EntityType` (graph node, rendered to ENTITIES.md) and a
  domain's authored Go type (rendered to MODELS.md) — had no formal link, so an AI reader could not tell
  whether a large-entity-type/dozens-of-Go-objects mismatch was an error or a normal abstraction-level split.
  Separately, the live SETTLED requirement `R-generations-inherit-doc-test-code` claimed the methodology
  MUST generate a Go struct, lifecycle methods, and transition tests FROM every EntityType — its own `Why`
  text already admitted this was "honest REMAINING debt" and pointed in the WRONG direction relative to this
  whole wave's goal (Go code is the source of truth; graph nodes are projections of it, never generation
  targets — `docs/AUTHORED-SPEC-CONTRACT.md` §9 already rejected `gen-code`).
  - **Deliverable A — the `model_symbol` link.** `EntityType.ModelSymbol string` (`internal/ontology/entity.go`,
    `json:"model_symbol,omitempty"`) is an OPTIONAL `"file:Symbol"`-shaped reference, the same shape/parser as
    `Requirement.ImplementedBy` (`gate.ParseFileColonSymbol`) — chosen over a `Relation`/`realized_by` edge
    because `Relation`/`RelationKinds` (`refines`/`depends_on`/`replaces`) is reserved for requirement-to-
    requirement links, semantically wrong for "this graph node is embodied by a Go symbol." Empty means "no
    Go type yet, or no 1:1 Go counterpart" — a calm, expected default; ONE-DIRECTIONAL by design (EntityType
    names its own Go symbol, no MODELS.md back-reference is added — that would be scope creep). Threaded
    through `internal/proposal/types.go`'s `ProposedEntityType` (new `ModelSymbol` field) and
    `internal/proposal/mutate.go`'s `mutate()` on the CREATE path; the UPDATE path (fields-only append) now
    also rejects a non-empty `model_symbol`, mirroring its existing states/transitions/description/why shape
    guard (`errEntityTypeUpdateShape`). New invariant `check_entity_type_model_symbol_resolves`
    (`internal/invariants/entity_checks.go`) mirrors `checkImplementedBySymbolResolvable` exactly (same
    `gate.ParseFileColonSymbol`/`gate.SpecRootForGraph`/`gate.ResolveSpecSymbol` resolution), iterating
    `g.EntityTypes` generically per `R-entity-checks-by-iteration` — a malformed `file:symbol` shape or an
    unresolvable symbol is a `Violation`; empty `ModelSymbol` is a silent no-op (the overwhelming majority
    case today — zero live EntityTypes set this field yet). `internal/generator/entities.go`'s `BuildEntities`
    renders a `**Model:** \`file:Symbol\`` line near the type's own heading when `ModelSymbol` is set, and
    renders nothing when it is not.
  - **Deliverable B — reject-and-replace.** `R-generations-inherit-doc-test-code` is now `Status: "REJECTED"`
    in place (`internal/selfspec/requirements_authoredspec.go`), mirroring `R-active-loop-playbooks`'s exact
    REJECTED shape field-by-field, replaced by two atomic successors per `R-requirement-claim-is-atomic`:
    `R-requirement-generation-mechanized-by-registry` (the surviving, already-true half — every SETTLED
    requirement in a self-hosting domain is authored as a named Go declaration in `internal/selfspec`,
    mechanically verified byte-identical via `hotam sync-self`; `Enforcement: "ENFORCED"`, citing
    `check_self_requirements_match_registry`/`TestMergeIntoGraph_ByteIdenticalRoundTrip`/
    `TestMergeIntoGraph_AllRequirementsRegistered`) and `R-entity-type-realized-by-go-symbol-never-generated`
    (the REVERSED-direction claim — an EntityType may declare `model_symbol` naming the Go type that
    realizes it, the Go type is the PRIMARY source, and the methodology shall NEVER generate a Go struct/
    lifecycle-methods/transition-tests FROM an EntityType; `Enforcement: "ENFORCED"` on the strength of the
    checkable reference-resolution half (`check_entity_type_model_symbol_resolves`), the same honesty-
    boundary precedent `R-authored-spec-projections-are-derived`'s and `R-structural-floor-vs-mirror-audit`'s
    own "shall NEVER..." prohibition clauses already set — ENFORCED via a real carrier for the checkable
    half, with no enforcer directly proving the negative-existence half itself). Landed via `hotam sync-self`
    (two runs: the substantive change, then a `SourceRefs` typo fix caught by re-reading the rendered diff);
    hit the same confront-gate lexical false-positive task #388 documented (shared modal words "never"/
    "always" against `R-domain-overview-projection` and `R-shared-projections-mode-independent`, both
    unrelated in substance, sharing only the token "projection"), resolved the same way via `--decision-ref`.
  - `internal/invariants/registry_complete_test.go`'s registered-invariant count moves 116 → 117 (the new
    `check_entity_type_model_symbol_resolves`); `internal/selfspec/merge_test.go`'s `wantRequirementCount`
    moves 303 → 305 (the rejected parent's SETTLED→REJECTED status flip does not change node count; the two
    new SETTLED successors do, +2).
  - New tests: `internal/invariants/entity_checks_test.go` (empty no-op, malformed shape, resolving symbol
    clean, non-resolving symbol violation, break→fix mutation round-trip),
    `internal/proposal/proposal_test.go` (`TestApply_EntityType_ModelSymbolRoundTrip`,
    `TestApply_EntityType_UpdateWithNonEmptyModelSymbolFails`), `internal/generator/entities_test.go`
    (`TestBuildEntities_RendersModelSymbolWhenSet`, `TestBuildEntities_OmitsModelLineWhenUnset`). Regenerated
    `docs/gen/` for hotam-spec-self (102 docs via `hotam sync-self`'s own regen step) and hotam-dev (its
    `CLAUDE.md` picks up hotam-spec-self's changed requirement/debt counts via the cross-domain pulse).
  - Verified: `go build`/`go vet`/`gofmt` clean (excl. pre-existing untracked `.scratch` draft), targeted
    tests (`internal/ontology`, `internal/invariants`, `internal/proposal`, `internal/generator`,
    `internal/selfspec`, `cmd/hotam`) all pass, `all-violations` 0 on both `domains/hotam-spec-self` and
    `domains/hotam-dev`.

- **`ModelObject` gains `ModelKind` — a semantic `object`/`value`/`port`/`mock`/`policy` classification, and
  mocks (unexported `_test.go` structs satisfying a port's method set) become visible in MODELS.md for the
  first time (task #394, W1.2).** The plan's own words: "Классификация object / value / port / mock / policy
  без изменения их исполняемой природы. Моки — не временная подмена, а правильные исполняемые суррогаты
  внешнего мира" — this is a read-only classification/visibility feature layered on top of task #393's
  (W1.1) `Kind` field; nothing about how any code actually executes changes, no mock is moved out of its
  `_test.go` file.
  - `ModelObject.ModelKind` is computed by `classifyModelKind` (`internal/gate/model_scan.go`) in priority
    order: (1) `"policy"` — the object's own file lives under an EXACT `spec/policy/` path-SEGMENT sequence
    (a segment match, not `strings.Contains` — a hypothetical `spec/model/policyholder.go` must NOT
    false-positive on the substring "policy"; mirrors `docs/AUTHORED-SPEC-CONTRACT.md` §1's own established
    `spec/policy/` convention); (2) `"port"` — `Kind == "interface"`; (3) `"value"` — `Kind == "type"` (a
    named primitive/alias) AND zero `Methods` anywhere in the file (a value-shaped type that DOES carry a
    method, e.g. a `String()`-like receiver, deliberately falls through to `"object"` instead — a narrow,
    honest heuristic documented as a known limitation, not special-cased further); (4) `"object"` — the
    default fallback (every ordinary struct/typed-with-methods declaration, and every object classified
    before this task's `ModelKind` field existed).
  - `"mock"` (rule 5) is NEVER derived from the four file-local rules above — it requires a separate
    cross-file pass, `scanDomainMockFiles`, wired into `ScanAuthoredModels`'s non-self-hosting branch AFTER
    `scanDomainModelFiles` returns its normal (unchanged) file list. It (a) collects every already-scanned
    `ModelKind == "port"` object whose `InterfaceMethods` has NO `Embedded` entry (an interface that embeds
    another interface, e.g. `io.Closer`, has an incomplete method-name set from AST alone — full
    `go/types` resolution is out of scope for this AST-only scanner, so embedding interfaces are excluded
    from mock-matching entirely, a deliberate, documented limitation) and whose required method-name set is
    non-empty (an empty interface is never eligible either — it would trivially "match" everything); (b)
    short-circuits to `(nil, nil)` WITHOUT walking the filesystem at all when zero eligible ports exist (the
    common case for most domains — "no ports declared yet" is a calm, expected state, not a scan failure);
    (c) otherwise walks the SAME `specDir` root's `_test.go` files (reusing `SpecRootForGraph`/
    `IsGeneratedOrVendoredFile`, task #391's existing exclusion, defense in depth); (d) parses each via a
    new `extractModelFileForMockScan` — the exact same extraction algorithm as `extractModelFile`
    (`extractModelFileImpl` is the one shared body both thin wrappers call; `extractModelFile`'s own public
    signature/behavior is completely unchanged), except top-level type declarations are collected regardless
    of exportedness. This one relaxation exists because a real mock is CONVENTIONALLY UNEXPORTED (confirmed
    against `PRAT-hotam/domains/gpsm-sm`'s real `mockOcrRecognizer`/`recordingAdapter` — each an unexported
    struct declared in a `_test.go` file, same package as but a different file from the port interface it
    implements): `extractModelFile`'s ordinary exported-only gate would otherwise make every real-world mock
    invisible, defeating this task's entire purpose. The general (non-test) scan's own behavior for
    top-level types is entirely untouched. (e) Each candidate struct's own method-name set (from its
    `Methods`, name-only — never full `go/types` signature checking of params/results, the same accepted
    approximation this whole AST-only scanner already uses everywhere else) is checked against every
    eligible port; a FULL SUPERSET match marks it `ModelKind = "mock"` and keeps it, a partial or empty
    overlap drops it. (f) A matched test file's retained `ModelFile` carries ONLY its matched mock
    `Objects` — `Funcs`/`Consts`/`Errors` are always left zero-value even if the source test file declares
    them, and a test file with zero matched mocks is dropped from the result entirely, so ordinary test
    helpers/table-driven test funcs never leak into MODELS.md as noise. (g) Mock files are merged into the
    normal file list and the combined slice is re-sorted by `RelPath`, so output stays fully deterministic.
  - `internal/generator/models.go`'s `BuildModels` renders a NON-default `ModelKind` (`"port"`/`"mock"`/
    `"value"`/`"policy"`) as a `, modelkind` suffix on the existing `### `Name` (kind)` heading — e.g.
    `### `OcrRecognizer` (interface, port)`, `### `mockOcrRecognizer` (struct, mock)` — via a new
    `modelKindHeadingSuffix` helper; `ModelKind == "object"` (or empty/unset, defense only) renders EXACTLY
    as before this task, preserving byte-identical output for the common case.
  - New unit tests: `internal/gate/model_scan_test.go` gained 10 new tests (all four file-local
    classification rules including the policy false-positive-avoidance case and the value-shaped-type-with-
    a-method fall-through, plus the mock pass's true-positive full-superset match, true-negative partial
    overlap, zero-ports short-circuit, and embedding-interface exclusion — the true-positive fixture is
    modeled directly on the real `PRAT-hotam/domains/gpsm-sm` `OcrRecognizer`/`mockOcrRecognizer` shape,
    replicated inline, never read/copied from that reference repo). `internal/generator/
    models_new_categories_test.go` gained an end-to-end rendering test proving a port and its mock both
    surface with the new heading suffixes while the test file's OTHER content (a plain test function) does
    not leak into MODELS.md.
  - `ModelLayerCounts`/`ScanModelLayerCounts` (COVERAGE.md's narrower layer-progression ratchet) and the
    self-hosting scan (`scanSelfHostingModelFiles`) are both UNCHANGED by design — the mock pass is scoped
    to ordinary (non-self-hosting) domains only, no real-world evidence of test-file mocks in the engine's
    own self-hosting file slice.
  - Regenerating `docs/gen/` for `hotam-spec-self` (self-hosting: rules 1-4 classification DOES run inside
    the shared `extractModelFile`, since self-hosting also calls it) changed exactly two lines: `internal/
    proposal.ConflictChecker`/`ProvenanceChecker` (both `type X func(...)` aliases with zero receiver
    methods) now render as `(type, value)` instead of `(type)` — a correct, honest application of rule 3,
    not a regression. `hotam-dev`'s own `docs/gen/` had zero content changes. `all-violations` stayed 0 on
    both domains before and after regeneration.
- **`gate.ScanAuthoredModels`'s AST scan now extracts interface methods, top-level functions/constructors,
  typed const enum groups, struct field doc comments, and raw struct tags (task #393, W1.1).** Before this
  task, the scan (`internal/gate/model_scan.go`) inventoried only exported top-level types, struct fields
  (name+type only), receiver methods, and `Err*` sentinel vars — an `interface` type rendered in MODELS.md
  as a bare name with NO visible method set, so a port/mock contract with the outside world (e.g.
  `domains/gpsm-sm`'s `OcrRecognizer` seam) looked empty even though it names a real contract. Priority #1
  of this task closes exactly that gap.
  - `ModelObject` gained `InterfaceMethods []ModelInterfaceMethod` (populated only for `Kind == "interface"`
    — the interface's own declared method set, rendered as readable `Name(params) (results)` signatures, with
    embedded interfaces recorded via `Embedded` instead of `Name`/`Signature`) and `Consts []ModelConst` (an
    exported typed const enum group whose own type matches this object's `Name`, e.g. `type Status string;
    const StatusActive Status = "active"`, kept in declaration order — enum member order is itself authored
    intent, not alphabetized).
  - New `ModelFunc` (`Name`/`Signature`/`Doc`) captures every exported top-level (non-receiver) `func` —
    constructors (`NewWidget`) and other free functions — on `ModelFile.Funcs`, sorted by name; a function
    with a receiver is unaffected (still attaches to its `ModelObject.Methods` exactly as before).
  - `ModelFile` gained `Consts []ModelConst` for exported top-level const declarations whose type does NOT
    match any object declared in the same file (untyped/primitive const groups, e.g. `const MaxRetries = 3`)
    — a typed enum group attaches to its own type's `ModelObject.Consts` instead, so it renders next to its
    type rather than being duplicated at file level.
  - `ModelField` gained `Tag` (the field's raw struct tag text verbatim, e.g. `json:"name,omitempty"
    validate:"required"` — unparsed; this scan does not interpret any specific tag vocabulary, only preserves
    the literal text) and `Doc` (the field's own doc comment, independent of the enclosing type's `Doc`).
  - `internal/generator/models.go`'s `BuildModels` (MODELS.md renderer) gained matching sections: an
    "Interface methods:" list per interface object, a "Functions:" list per file, a "Values:"
    table per object carrying a typed const group, and a file-level "Constants:" table for untyped consts;
    the existing Fields table grew `tag`/`doc` columns. The summary line grew a
    "N top-level function(s)" count. `ModelLayerCounts`/`ScanModelLayerCounts` (COVERAGE.md's narrower
    models→fields→methods→tests layer ratchet) are UNCHANGED — this task only extends MODELS.md's fuller
    inventory, not COVERAGE.md's layer-progression counts.
  - Task #391's (W0.4) vendor/generated-file exclusion (`IsGeneratedOrVendoredFile`) needed NO code change:
    every new category is extracted inside `extractModelFile`, which `parseModelFiles` only ever calls AFTER
    the exclusion check — the same single choke point automatically covers interfaces/functions/consts too.
    Verified experimentally: `cmd/hotam/model_scan_vendor_exclusion_test.go` gained
    `TestScanAuthoredModels_ExcludesVendoredInterfacesConstructorsAndConsts`, extending the shared fixture
    with a real `hotam vendor-recorder` copy (whose own source genuinely declares an interface `T`, a typed
    const enum group `StepKind`, and a top-level constructor `NewScenario`) and proving none of the three
    leak into the scan or into rendered MODELS.md.
  - New unit tests: `internal/gate/model_scan_test.go` (interface methods incl. embedding/sorting,
    constructors vs. unexported top-level funcs, typed vs. untyped const groups in declaration order, struct
    tag/field-doc preservation) and `internal/generator/models_new_categories_test.go` (end-to-end MODELS.md
    rendering of all five new categories against a fixture modeled on the real `OcrRecognizer` port).
  - Regenerating `docs/gen/` for `hotam-spec-self` (self-hosting: scans the engine's own linked files) grew
    MODELS.md from 339 to 785 lines purely from previously-invisible top-level functions, struct tags, and
    const groups now surfacing on the engine's own real code — an expected, not accidental, growth.
- **`ProposedGoal` and `ProposedEntityInstance` proposal kinds resolve the "no sanctioned write path"
  gap for the `Goal` and `EntityInstance` graph node types (task #392, W0.5).** Neither type had a
  Go-authored path (`internal/ontology/canon/` vendors only `Requirement`+`Registry`) nor a JSON-proposal
  path (`internal/proposal/types.go` had 15 kinds, none covering `Goal`/`EntityInstance`) — the only way
  to create either was a direct hand-edit of `graph.json`, forbidden by `R-no-hand-edit-graph`. This left
  the 4 `check_entity_instance_*` invariants (`internal/invariants/entity_checks.go`) with nothing to
  validate, and the one live `GOAL-burn-down-zero` node in `domains/hotam-spec-self/graph.json` with no
  reproducible creation path. Resolved onto the ordinary JSON-proposal path (not Go-code authority): both
  are typed graph nodes with their own referential-integrity semantics, not structural domain-model
  objects — the same rationale that already keeps `Conflict`/`Assumption`/`EntityType` off the
  Go-authored path.
  - `internal/proposal/types.go` gained `ProposedGoal` (`id`/`owner`/`target_state`/`why`) and
    `ProposedEntityInstance` (`id`/`entity_type`/`state`/`field_values`), plus `KindGoal` (`"Goal"`) and
    `KindEntityInstance` (`"EntityInstance"`). Both are CREATE-only (no UPDATE mode), mirroring
    `ProposedConflict`/`ProposedAssumption`/`ProposedStakeholder`'s shape rather than
    `ProposedEntityType`/`ProposedProcess`'s CREATE+UPDATE split.
  - `internal/proposal/validate.go` / `mutate.go`: `ProposedGoal.mutate` rejects a duplicate `id`, requires
    `owner` to resolve to a declared Operator id (mirrors `check_goal_owner_is_operator`), and stamps
    `Lifecycle` to `ontology.GoalLifecycle`'s INITIAL state (`ACTIVE`) — never author-supplied, mirroring
    `ProposedProcess`'s `Lifecycle` handling. `ProposedEntityInstance.mutate` rejects a duplicate `id`,
    requires `entity_type` to resolve to a declared `EntityType` slug, and requires `state` to be valid in
    that `EntityType`'s lifecycle (mirrors `check_entity_instance_state_in_lifecycle`).
  - `cmd/hotam/apply_proposal.go`'s `parseProposal` kind-dispatch switch gained both kinds, so
    `hotam apply-proposal`/`hotam land` (single-file and `--batch`) accept them.
  - `docs/PROPOSAL-REFERENCE.md` gained `## Goal` and `## EntityInstance` sections (required/optional
    field lists + worked JSON examples), and `cmd/hotam/proposal_reference_test.go`'s
    `proposalKindsSample` now includes both, keeping the doc mechanically in sync with the structs
    (`TestProposalReferenceExamples_AllParse`, `TestProposalReferenceRequiredOptionalFields_InSync`).
  - New unit tests in `internal/proposal/proposal_test.go` (CREATE success, duplicate-id rejection,
    missing-required-field rejection, unresolvable-owner/entity_type rejection, invalid-state rejection,
    and a `TestApply_EntityInstance_ThenInvariantsSatisfied` proof that a landed `EntityInstance` passes
    the real `internal/invariants.AllViolations` sweep) plus `internal/proposal/types_json_test.go`
    snake-case field round-trip tests for both kinds.
  - The `self_hosting`/`requirements_authority` Requirement/Rejection lock in `applyToGraph`
    (`internal/proposal/apply.go`) is untouched — it does not gate `Goal`/`EntityInstance` at all, on
    `hotam-spec-self` or any other domain.
- **New manifest.json opt-in key `claim_authority: "scenario"` gives `check_claim_matches_scenario`
  its own opt-in trigger, fixing a live regression against two real consumer domains (task #388,
  W0.1).** `check_claim_matches_scenario` (task #369) activated for ANY domain with
  `discipline: "full"`, but `PRAT-hotam/domains/prat` and `PRAT-hotam/domains/gpsm-sm` had both
  flipped `discipline: "full"` long before that check existed, consenting only to the four gates
  live at that time (`check_settled_requires_scenario`, `check_scenario_executes_impl`,
  `check_spec_md_current`, `check_model_complete`) — never to a fifth. Result: `all-violations`
  against `domains/prat` reported **20 violations before this fix (19 × `check_claim_matches_scenario`
  + 1 pre-existing, unrelated `check_domain_claude_md_current`)**; `domains/gpsm-sm` reported
  **23 (22 × `check_claim_matches_scenario` + 1 pre-existing `check_domain_claude_md_current`)**.
  This directly violated `R-scenario-spec-obligations-mechanically-enforced`'s own documented law
  ("each gate is an honest no-op before its own opt-in trigger fires... the discipline:\"full\"
  flip lands in the same commit that completes a domain's migration") — a new mechanical
  obligation was silently wired into an already-spent trigger.
  - **Fix**: `internal/loader/manifest.go`'s `DomainManifest` gained a `ClaimAuthority string
    json:"claim_authority,omitempty"` field (mirroring `RequirementsAuthority`'s exact shape);
    `internal/loader/loader.go` gained `ClaimAuthorityScenario` (the recognized literal
    `"scenario"`) and `resolveClaimAuthorityScenario` (mirroring
    `resolveRequirementsAuthorityCode`); `internal/ontology/graph.go`'s `Graph` gained
    `ClaimAuthorityScenario bool json:"-"`, populated by `loader.LoadGraph`.
    `checkClaimMatchesScenario` (`internal/invariants/claim_scenario_current.go`) now requires
    BOTH `g.Discipline == loader.DisciplineFull` AND `g.ClaimAuthorityScenario` — neither
    condition alone activates the check; a domain missing either is an honest no-op, exactly as
    it was for every domain before task #369 shipped.
  - **Ratchet**: `claim_authority: "scenario"` is a ONE-WAY door, symmetric with
    `discipline: "full"`'s own F2 ratchet. `internal/loader/lock.go`'s `graph.lock` gained
    `ClaimAuthorityScenarioObserved bool json:"claim_authority_scenario_observed,omitempty"`
    (additive, ratcheted by `WriteLock` exactly like `DisciplineFullObserved`), plus
    `ReadClaimAuthorityPin`. A new invariant, `check_claim_authority_ratchet`
    (`internal/invariants/claim_authority_ratchet.go`), fires if a domain that was ever observed
    with `claim_authority: "scenario"` later reports a manifest without it — mirroring
    `check_discipline_ratchet` exactly.
  - **Self-hosting anchor**: a new SETTLED requirement, `R-opt-in-trigger-owns-its-own-obligations`
    (`internal/selfspec/requirements_authoredspec.go`, `refines R-scenario-spec-obligations-
    mechanically-enforced`), names `check_claim_authority_ratchet` as its `enforced_by`, landed via
    `hotam sync-self` (confront-gate false positives on shared modal words — must/must-not/never/
    always against unrelated requirements' shared vocabulary — acknowledged via `--decision-ref`,
    the sanctioned override for exactly this class of lexical-only collision).
  - **Verification**: `all-violations` against `PRAT-hotam/domains/prat` and `domains/gpsm-sm`
    dropped from 20/23 to **1 violation each** (the pre-existing, unrelated
    `check_domain_claude_md_current` staleness, present in the baseline before this fix and
    untouched by it) — **zero `check_claim_matches_scenario` violations remain, with no hand-edit
    to either domain's `graph.json`/`graph.lock`/tests/code**. `all-violations` against
    `domains/hotam-spec-self` and `domains/hotam-dev` (this repo's own self-hosting domains): 0.
- **`internal/invariants/scenario_coverage.go`'s `check_scenario_executes_impl` (the
  coverage-proof gate) now caches `RunVerifiedByTestRecording` results for the process's whole
  lifetime, not just within one `checkScenarioExecutesImpl` call (task #387).** Round-2
  profiling after the #383/#384 caching wave above found this check newly dominant: 78% of all
  subprocess-record cost (272.9 of 349.9s measured) across a single `cmd/hotam` test binary's
  run, because `coverageRunCache` was allocated fresh (`cache := &coverageRunCache{}`) inside
  every `checkScenarioExecutesImpl` call — every one of `cmd/hotam`'s 8-12 tests that call
  `AllViolations()`/`whatNow()`/`buildStatusReport()` against the SAME real `hotam-spec-self`
  graph (336 nodes, unchanged across those calls) re-ran every `verified_by` test's coverage
  recording from a cold cache, even though `gate.runCache` (`internal/gate/test_exec.go`) had
  already proven this exact class of waste safe to eliminate for the plain pass/fail sibling
  check (`check_verified_by_test_passes`), via content-hash invalidation instead of
  scope-limited caching.
  - **Fix**: `coverageRunCache` is now a package-level, process-lifetime `sync.Map` (mirroring
    `gate.runCache`'s shape exactly), keyed by `(testFile, testName, coverPkgFile)`. A lookup now
    also carries a `gate.HashPackageInputs(moduleRoot, pkgDir)` content hash (the SAME hash
    `gate.runCache` already trusts, over the SAME module root `RunVerifiedByTestRecording` itself
    resolves the test's package to, via a new minimal exported wrapper,
    `gate.HashPackageInputs`, around the existing package-private `hashPackageInputs`): a hash
    match reuses the cached `RecordingResult` (pass/fail + scenario artifacts + coverage
    profile — all deterministic for the same binary+test); a hash mismatch (real content changed
    since the entry was cached) replaces the entry and re-runs the subprocess, exactly as
    `gate.runCache` already does for `RunVerifiedByTest`. Single-flight `sync.Once`-per-entry
    de-duplication of concurrent callers for the same key is preserved (previously scoped to one
    `checkScenarioExecutesImpl` call, now scoped to the whole process) via a
    `LoadOrStore`+`CompareAndSwap` retry loop, race-free without holding a lock across the
    (subprocess-spawning) run itself. `ResetCoverageRunCacheForTest` added (mirrors
    `gate.ResetRunCacheForTest`'s naming/shape) for tests that need a known-empty cache.
  - **3 new tests** in `internal/invariants/scenario_coverage_test.go`: cache reuse across two
    separate `checkScenarioExecutesImpl` calls over an unchanged package (asserts exactly one
    cache entry survives both calls, with byte-identical `CoverProfile` on the second call);
    cache invalidation on a real content edit between two calls (asserts the content hash
    actually changes and the stale entry is replaced, not silently reused — the exact silent
    forgery this whole check exists to prevent); and race-free concurrent access to
    `runOrReuseCoverage` under `-race` with 32 goroutines contending on the same key across two
    different hashes (exercises both the single-flight-hit and invalidate-and-retry paths at
    once).
  - **Verified by the resolver**: `go build`/`vet`/`gofmt` clean; `go test -race` clean on both
    `internal/invariants` (33.0s) and `internal/gate` (46.4s); full `go test ./...` green (see
    below); `all-violations` 0 on both `domains/hotam-spec-self` and `domains/hotam-dev`. Before/
    after timing, `TestBuildStatusReport_MatchesOnRealDomain` (2 in-process `AllViolations`-class
    calls over the real self-hosting graph): **15.33s → 11.30s** (−26%). A sharper synthetic
    probe (4 back-to-back `invariants.AllViolations(g)` calls, same real graph, same process,
    reverted/re-applied in place to get an apples-to-apples before/after): before — call 1
    (cold) 8.5s, calls 2-4 (warm but re-recording every time) ≈1.41-1.42s each; after — call 1
    (cold) 9.1s, calls 2-4 (warm, cache hit) ≈0.66-0.71s each, a **~52% reduction** in warm
    per-call cost. The win scales with call count per process — `cmd/hotam`'s own 8-12-test suite
    that motivated this fix pays the ~9s cold cost once instead of once per test.
- **In-process caches for `hashPackageInputs` and test-file AST parsing (tasks #383/#384),
  closing the top finding from tasks #380/#381/#382's broader post-fix profiling.** That
  profiling found `hashPackageInputs` (`internal/gate/test_exec.go`) still dominant even after
  task #379's build-output exclusion — the real remaining cost was `os.ReadFile`-ing the FULL
  CONTENT of every regular file in a module on every single `RunVerifiedByTest`/
  `RunVerifiedByTestRecording` call, with zero reuse between calls even within the same process
  — plus a smaller, separately-surfaced cost in uncached `go/parser.ParseFile` calls inside
  `internal/gate.collectTestFuncNames`/`scanTestFile` (~6-8% cum). Both were written and reviewed
  under an unusual constraint: the implementing agents were required to make zero `go build`/
  `vet`/`test`/`gofmt` calls (the resolver's machine was reserved for the user's own benchmark
  session), so both diffs were designed, implemented, and self-reviewed purely by static reading
  before any execution — then fully verified by the resolver afterward (this entry).
  - **`internal/gate/test_exec.go`'s `hashPackageInputs`** now consults a process-lifetime,
    `moduleRoot`-keyed cache (`perFileHashCache`, mutex-guarded map matching this file's existing
    `inFlightCalls` style) before recomputing: the directory walk still runs in full every call
    (the walk itself was never the expensive part), but for each file, its current `(mtime, size)`
    is compared against what was recorded the last time this cache actually read that file's
    bytes — a match reuses the previously-read bytes (no `os.ReadFile`, no re-hash), a miss (or a
    file added/removed since the last call) falls through to a real read. The combined digest for
    an unchanged tree is byte-for-byte identical to a full cold computation (verified by dedicated
    tests), and a fully-unchanged tree short-circuits to just one `os.Stat` per file. Honest
    residual gap, explicitly documented rather than hidden: a mutation that preserves the exact
    same byte size **and** lands within the same filesystem mtime tick as the cached read is not
    caught — a genuinely new, narrow trade made for this perf win, not a pre-existing limitation
    (the old implementation always re-read raw bytes, so it never had this gap). 7 new tests cover
    warm-cache reuse, size-change detection, same-size-forced-mtime-change detection, added-file
    detection, removed-file detection, and race-free concurrent access (`-race` clean, personally
    verified 3× by the resolver).
  - **`internal/gate/gate.go`'s `collectTestFuncNames`/`scanTestFile`** now share a single
    `sync.Map`-based cache (`testFileParseCache`, keyed by absolute path) of the EXTRACTED
    result (`Test*` names + `check_*` literal associations) rather than the raw `*ast.File` —
    deliberate, since `go/ast` nodes have no internal synchronization and would be unsafe to
    share across goroutines, while the extracted shape is small and immutable. A cache entry is
    valid only while a fresh `os.Stat`'s `(mtime, size)` matches what was recorded at parse time.
    Every real call site of both functions was audited (via grep across the whole repo) and none
    write a `_test.go` file and then re-scan it in the same process — a materially lower-risk
    shape than `hashPackageInputs`' target (which `hotam land` genuinely does mutate-then-rehash
    mid-process against `graph.json`), so the cache needed no extra defense beyond the same
    mtime+size check applied unconditionally as defense-in-depth. Bonus finding surfaced while
    auditing call sites: `internal/invariants/*_test.go` files were already being parsed TWICE
    per `buildScan` call (once via `scanTestDir`, once via `walkTestFuncs`) even before this
    change — the shared cache now collapses that redundancy within a single call, not just across
    process-lifetime calls. 4 new tests cover unchanged-file reuse, mutation invalidation,
    same-(mtime,size)-different-path non-collision, and race-free concurrent access.
  - **Verified by the resolver** (build/vet/gofmt clean; `-race` clean on all new tests plus the
    whole `internal/gate` package; `internal/invariants`/`internal/generator` green;
    `all-violations` 0 on both `hotam-spec-self` and `hotam-dev`; full `go test ./...` green,
    0 FAIL, 19/19 packages). **Real, dramatic wall-clock effect**, well beyond task #379's earlier
    modest ~7%: a clean full-suite run dropped from the 11-15 minute range measured earlier this
    same session to **5m0.9s**, with `cmd/hotam` alone (the session's dominant cost all day) down
    from the previously-measured 650-900s range to **293.1s** — consistent with tasks #380/#381's
    profiling, which found `hashPackageInputs` and uncached AST parsing together accounting for a
    large share of that package's own CPU time.
- **`internal/gate/fixture_cache.go` — `EnsureContentAddressedFixture`, a stable content-hash
  path for test fixture modules (pilot).** Continuation of the `-vet=off`/`-short` speed wave
  above: task #374 (same investigation) proved the `t.TempDir()`-per-test-run fixture pattern
  (used throughout `internal/selfspec`, `internal/invariants`, `internal/generator` via
  `gate.RunVerifiedByTestRecording`) structurally defeats Go's build cache — the compiler bakes
  the fixture's absolute source path into its action ID, so byte-identical fixture content
  written to a fresh temp directory is a guaranteed cache MISS across separate `go test`
  processes, independent of disk speed or antivirus exclusions. `EnsureContentAddressedFixture`
  publishes a fixture's file set under `<os.TempDir()>/hotamspec-fixture-cache/<sha256-of-tree>/`
  instead: two separate `go test` processes building the identical fixture converge on the same
  path and share one Go build-cache entry. Design resolves four correctness concerns explicitly
  (see the file's own doc comment): concurrent publishers race via an atomic `os.Rename` from a
  private tmp dir (no lock needed — the rename itself is the arbiter, verified empirically on
  Windows that renaming onto an existing directory fails cleanly rather than corrupting either
  side); a completeness marker written last, inside the tmp dir, before the rename, means a
  reader can never observe a partially-published directory; unbounded growth is bounded by a
  documented best-effort age-based eviction (7-day cutoff, 1-in-20-calls opportunistic sweep);
  the function's return contract (an absolute path to exactly the requested files) is identical
  to `t.TempDir()`'s, so no caller assertion changes, only where the bytes live on disk. Verified:
  a 12-goroutine concurrent-publish race test passes under `-race`; a direct `-x`-trace rebuild of
  an already-published fixture from a second, independent `go test -c` process shows zero
  `compile.exe` invocations (pure `packagefile ...=<hash>` cache references) — a genuine
  cross-process build-cache hit, the mirror image of task #374's own cache-miss demonstration.
  Applied as a PILOT to one call site only (`internal/selfspec/claim_derive_test.go`'s
  `writeClaimDeriveFixtureModule`) — `internal/invariants` and `internal/generator`'s equivalent
  fixture helpers are untouched, left for a follow-up wave if this pilot proves durable.

### Fixed
- **`gate.ScanAuthoredModels` (`internal/gate/model_scan.go`), the single scan `MODELS.md`,
  `COVERAGE.md`, and `check_model_complete` all funnel through, only excluded ONE of three known
  vendored/generated file kinds from a domain's authored-model inventory — the vendored scenario
  recorder (`spec/hotamspec/`, `hotam vendor-recorder`) — leaving the vendored ontology mirror
  (`spec/hotamontology/`, `hotam vendor-ontology`, task #365) and the generated registrydump
  scaffold (`spec/registrydump/`, `hotam scaffold-registrydump`, task #367) completely unguarded
  (task #391, W0.4).** As soon as a consumer domain adopted `requirements_authority: "code"`
  (`R-domain-founded-in-wave-order` step 6), the vendored ontology mirror's own exported types —
  `Relation`, `Requirement`, `Registry` (`internal/ontology/canon/requirement.go`, `registry.go`)
  — and anything the registrydump scaffold declares would be swept into the same `spec/` walk as
  the domain's real `spec/model/*.go` files and counted AS IF they were authored domain object
  model surface in `MODELS.md`, `COVERAGE.md`'s layer counts, and `check_model_complete`'s
  completeness gate — exactly the "engine/framework boilerplate leaked into the business folder"
  defect class task #367 (the `life` domain, task #364: "empty domain = zero files") already
  caught once this same session, now confirmed live for the ontology-vendor and registrydump-
  scaffold paths by direct code reading.
  - **Fix**: retired the single-purpose `vendoredRecorderBannerFirstLine string` /
    `IsVendoredRecorderFile(path string) bool` pair and replaced them with a UNIFIED registry:
    `knownGeneratedBannerFirstLines []string` (one entry per known banner producer, each derived
    from that producer's own exported `Banner` constant — never a hand-copied literal) and
    `IsGeneratedOrVendoredFile(path string) bool`, which reports true iff a candidate file's first
    line exactly matches ANY registered banner first line. `parseModelFiles` (both
    `scanDomainModelFiles`'s ordinary-domain path and `scanSelfHostingModelFiles`'s self-hosting
    path) now calls this single choke point instead of the old recorder-only check. Adding a
    fourth vendored/generated kind later is a one-line append to the registry, not a new parallel
    `isVendoredXFile` function.
  - **New leaf package for the third banner**: the registrydump scaffold's do-not-edit banner
    previously lived as a private `const registrydumpBanner` inside `cmd/hotam/scaffold_registrydump.go`
    (`package main`) — unreachable from `internal/gate` without an import cycle (`cmd/hotam` already
    imports `internal/gate`, e.g. `cmd/hotam/main.go`, `sync_domain.go`). Extracted the literal into
    a new leaf package, `internal/registrydump/vendor` (`Banner` constant only — no canonical source
    to embed, since `spec/registrydump/main.go` is a per-domain TEMPLATED program, not a
    byte-for-byte vendored copy), mirroring the existing `internal/recorder/vendor` /
    `internal/ontology/vendor` leaf-package shape exactly. `cmd/hotam/scaffold_registrydump.go`'s
    `registrydumpBanner` is now a thin re-export (`const registrydumpBanner = registrydumpvendor.Banner`)
    so its own writer (`registrydumpSource`) is unchanged; `internal/gate/model_scan.go` imports the
    same package directly, so both the writer and the unified detector read the exact one banner
    literal — no risk of the two silently diverging.
  - **`internal/generator/models.go`'s `isVendoredRecorderFile` wrapper** (kept for its existing
    test, `models_vendor_exclusion_test.go`, which reaches it by its lowercase in-package name) now
    forwards to `gate.IsGeneratedOrVendoredFile`, so `BuildModels`/`ScanModelLayerCounts` inherit the
    same three-banner exclusion with zero call-site changes.
  - **`internal/invariants/model_complete.go`'s `checkModelComplete`** needed NO code change — it
    already calls `gate.ScanAuthoredModels` directly (the shared scan), so the fix closes the gap
    for `check_model_complete` automatically, at the one shared source.
  - **Non-goal proven, not assumed**: the fix must exclude by BANNER CONTENT only, never by
    file/directory name or path convention — a genuinely domain-authored file whose name merely
    echoes a vendored file's own name (e.g. an authored `spec/model/registry.go` declaring the
    domain's OWN `Registry` type) must still be scanned normally. The new regression fixture
    (below) deliberately names its real domain model file `spec/model/registry.go` to prove this
    directly, rather than leaving it as an unverified claim in a doc comment.
  - **Regression coverage**: a new fixture, `cmd/hotam/model_scan_vendor_exclusion_test.go`, builds
    a domain that has gone through BOTH real `vendorOntology` (task #365's own writer) AND real
    `scaffoldRegistrydump` (task #367's own writer) — not hand-approximated banners — plus a real
    domain-authored `spec/model/registry.go`. `TestScanAuthoredModels_ExcludesVendorOntologyAndRegistrydumpScaffold`
    proves `gate.ScanAuthoredModels` returns exactly one file (the real model) with exactly one
    object (`Widget`), and that rendered `MODELS.md` contains `Widget` but none of `Relation`,
    `hotamontology`, `registrydump`, `spec.Requirements`. `TestIsGeneratedOrVendoredFile_RecognizesAllThreeKnownBanners`
    unit-tests the detector directly against all three real vendored/generated paths plus the
    same-named real domain file, table-driven. The pre-existing recorder-only regression test
    (`internal/generator/models_vendor_exclusion_test.go`,
    `TestScanDomainModelFiles_ExcludesVendoredRecorder` / `TestIsVendoredRecorderFile_DetectsRealVendoredCopy`)
    is unchanged and still green — the unification is a strict superset, not a behavior change for
    the recorder path.
  - **Verified the regression would have failed before the fix**: before this task, `parseModelFiles`
    only ever called the recorder-only check, so the new fixture's vendored `spec/hotamontology/`
    and `spec/registrydump/` files would have been parsed as ordinary Go source and their exported
    types (`Relation`, `Requirement`, `Registry`, the registrydump `main` package) counted as domain
    model objects — confirmed by reading `parseModelFiles`' pre-fix loop body directly (it called
    `IsVendoredRecorderFile(p)` and nothing else).
  - **Full verification**: `go build ./...`, `go vet ./...`, `gofmt -l .` clean; full
    `go test ./... -timeout 45m -count=1` green, 0 FAIL; `go run ./cmd/hotam all-violations --domain
    domains/hotam-spec-self` and `--domain domains/hotam-dev` both `0 violations — graph clean`.
- **`internal/ontology/canon/requirement.go`'s vendored `hotamontology.Requirement` mirror was
  missing the `Why` field entirely, silently losing a domain's hand-authored requirement rationale
  the first time it synced under `requirements_authority: "code"` (task #390, W0.3).** The vendored
  mirror (copied byte-for-byte, via `hotam vendor-ontology`, into every consumer domain's own
  `spec/hotamontology/` package — the type a domain's `spec/requirements.go` actually compiles
  against, per `PLAN-authored-spec-discipline.md`'s module-boundary design) carried every field of
  the full `internal/ontology.Requirement` (`internal/ontology/requirement.go`) EXCEPT `Why`, even
  though `internal/selfspec/merge.go`'s `MergeIntoGraph`/`SyncGraph` — used by both `hotam sync-self`
  and `hotam sync-domain` — already treated `Why` as a STRUCTURAL field, wholesale-replaced from the
  registry on every sync (its own doc comment named `Why` among the structural fields all along).
  Consequence: a domain that had a hand-written `Why` on a requirement BEFORE adopting
  `requirements_authority: "code"` could never express that `Why` in `spec/requirements.go` — the
  vendored struct simply had no such field, a compile error (`unknown field Why in struct literal
  of type hotamontology.Requirement`) if attempted — so the very first `hotam sync-domain` silently
  overwrote the graph's `Why` with the Go zero value `""`, with no error, no warning, and no way for
  the domain author to prevent it.
  - **Fix**: added `Why string \`json:"why"\`` to `internal/ontology/canon/requirement.go`'s
    `Requirement` struct, in the same field position as the source (`internal/ontology/requirement.go`
    ): immediately after `Status`, before `Relations`. The package doc comment's field-classification
    prose (previously listing `Why` among the fields that "stay engine-side") was corrected to name
    `Why` as a structural, registry-replaced field alongside `Claim`/`Owner`/`Status`.
  - **Vendoring mechanism required no change**: `internal/ontology/vendor/vendor.go` banner-stamps
    and embeds `requirement.go` in full via `//go:embed` (`internal/ontology/canon/embed.go`), so the
    new field is picked up automatically by every `hotam vendor-ontology` run — confirmed by grepping
    the whole repo for any other site constructing a `canon.Requirement`/`hotamontology.Requirement{}`
    literal outside test fixtures; none exists, and `cmd/hotam/sync_domain.go`'s
    `domainRegistryFromSubprocess` already unmarshaled registrydump's JSON directly into
    `[]ontology.Requirement` (the full type), so the fix closes the data-loss path completely at its
    one true source.
  - **Regression coverage**: `cmd/hotam/sync_domain_test.go`'s `TestCmdSyncDomain_FullRoundTrip`
    (the existing sync-domain e2e fixture, extended rather than duplicated) now asserts the
    registry's authored `Why` lands unchanged in `graph.json` after an ADDED sync. A new dedicated
    test, `TestCmdSyncDomain_WhyRoundTripSurvivesChangedSync`, reproduces the exact reported
    scenario end-to-end through the real `go run ./registrydump` subprocess bridge: sync-domain
    creates a requirement with an authored `Why`, the fixture's `spec/requirements.go` is then
    rewritten with a DIFFERENT `Why` (mirroring a domain author editing their own rationale), and a
    second `hotam sync-domain` pass (the CHANGED path) is proven to carry the NEW `Why` through —
    never resetting it to `""`, never leaving the stale first value. Two new unit tests in
    `internal/ontology/canon/requirement_test.go`
    (`TestRequirement_WhyFieldRoundTripsToOntologyRequirement`,
    `TestRequirement_WhyJSONTagMatchesOntology`) isolate the same proof at the JSON-marshal level,
    without the e2e subprocess/gate machinery.
  - **Verified the bug before the fix, not just the fix**: temporarily reverting the `Why` field
    addition reproduced the failure exactly as diagnosed — `go run ./registrydump` fails with
    `unknown field Why in struct literal of type hotamontology.Requirement`, causing both new/
    extended tests to fail with that literal compiler error — then restoring the fix made both pass.
  - **Full verification**: `go build ./...`, `go vet ./...`, `gofmt -l .` clean; full
    `go test ./... -timeout 45m -count=1` green, 20/20 packages `ok`, 0 FAIL; `go run ./cmd/hotam
    all-violations --domain domains/hotam-spec-self` and `--domain domains/hotam-dev` both
    `0 violations — graph clean`.
- **`deriveClaimFromVerifiedBy` (`internal/selfspec/claim_derive.go`) and its duplicated mirror
  `freshDerivedClaim` (`internal/invariants/claim_scenario_current.go`) no longer repeat a
  scenario title N times when N subscenarios inside ONE `verified_by` test share the same
  `hotamspec.NewScenario(...)` title (task #389, W0.2).** A real fixture shaped like
  `PRAT-hotam/domains/prat`'s `R-gate-pg0-source-ready` — one test that calls `t.Run` in a loop
  over several sub-cases, each constructing its own `Scenario` with the IDENTICAL title, plus a
  second `verified_by` entry with a distinct title — proved a fresh derivation currently
  concatenates the repeated title once per subscenario before joining the trailing distinct
  title on. **Before this fix** (reproduced on a controlled fixture matching the brief's
  captured example verbatim): a fresh derivation for an `R-gate-pg0-source-ready`-shaped
  requirement would read
  > "SourceReady() only returns nil when columns, scope, and the project profile are ALL present
  > SourceReady() only returns nil when columns, scope, and the project profile are ALL present
  > SourceReady() only returns nil when columns, scope, and the project profile are ALL present
  > SourceReady() only returns nil when columns, scope, and the project profile are ALL present
  > Approve refuses to set ApprovedByPM when the artifact is not yet SourceReady, even if a
  > caller tries to force approval"

  (the four-subscenario sentence repeated once per sub-case, run against the live fixture in
  `internal/selfspec` before the fix landed). **After this fix**, the same fixture derives:
  > "SourceReady() only returns nil when columns, scope, and the project profile are ALL present
  > Approve refuses to set ApprovedByPM when the artifact is not yet SourceReady, even if a
  > caller tries to force approval"

  — the repeated sentence contributes once, the distinct second-entry sentence follows
  unchanged. **Fix**: both `deriveClaimFromVerifiedBy` and `freshDerivedClaim` now dedupe titles
  with a `map[string]bool` scoped to ONE `verified_by` entry's own `result.Artifacts` loop
  (first-occurrence order preserved via a parallel `entryTitles` slice), reset for every new
  entry. **Deliberately scoped to a single entry, not global across a requirement's whole
  `verified_by` list**: within one entry, a repeated title is a mechanical artifact of
  loop-driven/`t.Run`-driven sub-cases narrating the identical sentence-worthy behavior — pure
  noise. Across two DIFFERENT `verified_by` entries, a repeated title is a distinct signal
  (either two tests intentionally proving the same behavior from two angles, worth keeping
  visible as two clauses, or a duplicate-coverage smell in the requirement's own `verified_by`
  list worth surfacing) — collapsing it globally would silently hide that signal and make
  Claim's clause count an unreliable proxy for how many `verified_by` entries actually
  contributed text. See `deriveClaimFromVerifiedBy`'s own doc comment
  (`internal/selfspec/claim_derive.go`) for the full rationale, mirrored byte-for-byte in
  `freshDerivedClaim`'s doc comment so `hotam sync-domain`'s write path and
  `check_claim_matches_scenario`'s read path can never silently disagree on one requirement's
  derived Claim again. New regression test
  `TestDeriveClaimsFromScenarios_RepeatedSubscenarioTitleDedupedWithinEntry`
  (`internal/selfspec/claim_derive_test.go`) proves the intra-entry dedup on a controlled
  fixture; the pre-existing `TestDeriveClaimsFromScenarios_
  MultiVerifiedByEntryConcatenatesInOrder` (proving two DIFFERENT verified_by entries still
  concatenate in full, unaffected by this fix) stays green unmodified. `go build ./...`,
  `go vet ./...`, `gofmt -l .` clean; full `go test ./... -timeout 45m -count=1` green (19
  packages ok, 0 FAIL); `all-violations` 0 on both `domains/hotam-spec-self` and
  `domains/hotam-dev`. Not applied to any `PRAT-hotam` domain — out of scope for this task, whose
  own `claim_authority:"scenario"` opt-in (task #388) is not enabled there.
- **`hashPackageInputs` (`internal/gate/test_exec.go`) no longer walks/hashes stray compiled
  build outputs at the module root (task #379, continuing task #378's profiling)** — task #378
  CPU-profiled three real heavy tests (`TestCmdSyncSelf_FullRoundTrip`,
  `TestGenSpec_CrystalFixpointConvergesAcrossRuns`, `TestBuildStatusReport_MatchesOnRealDomain`)
  and found `gate.RunVerifiedByTest`'s `hashPackageInputs` — the whole-module content hash that
  keys the verdict cache, computed UNCONDITIONALLY before every cache lookup, once per
  `verified_by` entry (13 in `hotam-spec-self`, run pairwise via `runVerifiedByTestJobs`'s
  2-worker pool) — at 20.6-28.8% of cumulative CPU across all three profiles, with
  `path/filepath.WalkDir` (its underlying full-module traversal) at 16.6-27.3%. Reading the walk
  (moduleRoot-recursive, skipping only dot-prefixed directories and `vendor/`, no file-suffix
  filter since task #368-era NEW-4 widened it to catch `//go:embed`/`testdata/` inputs) found
  this repo's own module root littered with ~50MB of gitignored, untracked, ad-hoc `go build -o`/
  `go test -c` debugging leftovers (`hotam.exe`, `hotam.test.exe`, `invariants.test.exe`, etc.) —
  files that are pure BUILD OUTPUTS, structurally incapable of being a `go test` INPUT (no
  `.go` source `//go:embed`s or opens a sibling `.exe`), yet walked and SHA-256'd on every single
  call regardless. **Fix**: `hashPackageInputs`'s walk now also skips regular files whose
  extension is `.exe`/`.dll`/`.so`/`.dylib`/`.test` (`isBuildOutputExtension`, case-insensitive) —
  a strict, narrowly-scoped subset of NEW-4's "hash everything a test could read" rule, excluded
  only because these specific extensions can never be read as an input, not because hashing them
  is merely inconvenient; every other non-`.go` file (testdata, embedded fixtures) is still
  hashed exactly as before, so NEW-4's stale-green guarantee is fully preserved. Whole-module
  hashing itself (vs. a precise transitive-import-graph scope) was NOT changed — re-read the
  file's own NEW-2 doc comment and confirmed the coarse-but-correct design is still the right
  tradeoff for this engine's module sizes; a blind "compute the walk once per process" in-memory
  cache was considered and REJECTED, because the existing `TestRunVerifiedByTest_MUTATION_*`
  tests genuinely mutate source files mid-process between two `RunVerifiedByTest` calls in the
  SAME process and rely on the second call observing the new content — a walk-level cache with no
  invalidation signal would silently break exactly the tests that prove cache correctness.
  Added `TestHashPackageInputs_BuildOutputExtensionExcluded` (`internal/gate/test_exec_test.go`)
  proving a stray `.exe`'s creation AND later content mutation both leave the module hash
  unchanged, alongside the pre-existing NEW-2/NEW-4 mutation tests (unmodified, still green) that
  prove everything else still invalidates. Verified before/after on the same three tests task
  #378 profiled (plain wall-clock, `go test -run <name> -count=1`): `TestCmdSyncSelf_
  FullRoundTrip` 83.92s → 79.98s, `TestBuildStatusReport_MatchesOnRealDomain` 22.50s → 18.55s,
  `TestGenSpec_CrystalFixpointConvergesAcrossRuns` 33.68s → 29.19s (combined wall 117.71s →
  109.24s, ~7.2%). A fresh cpuprofile of `TestBuildStatusReport_MatchesOnRealDomain` post-fix
  confirms the direction: `hashPackageInputs` cum% 28.76% → 24.00%, `filepath.walkDir` unaffected
  (23.74%→23.58%, expected — directory count didn't change, only ~8 file reads were removed out
  of ~2400 walked files; the win comes from the removed files' large byte size dominating
  `sha256.block` time, not their small share of total file/syscall count, which is also why the
  wall-clock win is real but modest rather than dramatic). Full `go test ./... -timeout 45m
  -count=1` stays green (0 FAIL, 19 packages ok), `all-violations` stays 0 on both
  `domains/hotam-spec-self` and `domains/hotam-dev`, `go build ./...`/`go vet ./...`/`gofmt -l .`
  clean. `internal/gate/compile_cache.go` and `internal/gate/fixture_cache.go` (both flagged as
  changed earlier this session) were re-read in full during this task's investigation — no bug
  found in either; `compile_cache.go`'s compiled binaries and `fixture_cache.go`'s published
  fixtures both live under `os.TempDir()`, never under `moduleRoot`, so neither contributes to
  `hashPackageInputs`'s walk cost and neither needed a fix.
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
  GOCACHE=`внешний каталог сборки Go`), both hypotheses this task set out to test came back
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
- **REJECTED history in multilingual domains**: `claim_texts` is no longer
  required for REJECTED requirements when a second language is enabled — a
  single historical formulation (Claim and/or a claim_texts language subset)
  is accepted, projections (HISTORY/CLAUDE per language) show the historical
  wording as is without a missing-translation failure, and switching
  `default_language` no longer invalidates REJECTED history. For all other
  requirements, sync-domain/sync-self now derive an empty `Claim` from
  `claim_texts[default_language]` before validation (both set and diverging
  remains an error).
