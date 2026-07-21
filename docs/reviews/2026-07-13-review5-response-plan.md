# Review 5 response plan — sources of truth, domain UX, agent UX (HEAD 9a46847)

**Date:** 2026-07-13
**Source:** fifth independent review; scores Применяемость 8.0 / Простота 7.0 / Развиваемость 8.2 / Поддерживаемость 8.4 / Удобство-для-агентов 8.0 / Общая 8.0. Verdict: "главный резерв роста теперь не в новых типах и проверках, а в уменьшении количества источников истины, generated noise и неочевидных правил выбора домена."

## Verification (done before planning, against live HEAD 9a46847)

| # | Claim | Verdict |
|---|---|---|
| 1 | `toolRequirementData` (internal/generator/tool_reqs.go) is a second hand-maintained source of truth: stale `hotam_req` entry present, real commands (all_violations/req/due/status/inspect/init/init_project/version) missing. | CONFIRMED (35 entries, not 34 — substance exact). |
| 2 | `hotam init <bare-dir>` + `gen-spec --domain <bare-dir>` fails with ProjectRootUnresolved outside a project — init itself PRINTS the command that then fails. | CONFIRMED by live run. Regression-effect of the 66e4af2 fallback (non-domains/<name> layout → CWD search). |
| 3 | After init-project, bare `hotam status` fails (hardcoded defaultDomainRel = domains/hotam-spec-self, cmd/hotam/common.go:21). | CONFIRMED (documented limitation since task #112). |
| 4 | README.md:68 says "thirteen commands", actual count is 14; init-project (the key onboarding command) absent from README's command section. | CONFIRMED. |
| 5 | what-now still yells P0 "37 closeable debt" although 33 are feature-blocked roadmap (per this repo's own c1-roadmap-debt-triage.md). | CONFIRMED by live run. Third independent source naming this (review 5 + @fl review + @fm consultation). |

## Resolver decisions

Scope: ALL of — 5 P1 items + `--profile consumer/full` + reviews/checkpoints archive index + structural boolFlagNames test + a new agent-UX improvement track ("проще развивать/поддерживать/понимать требования, быстрее ловить нестыковки старого и нового, меньше прогонов"). Agent-UX designs resolved via an @fx max-effort consultation (full text relayed in-session); its ranked recommendations are tasks T-i..T-m below.

## Plan

### T-a (P1) — kill toolRequirementData duplication
internal/generator/tool_reqs.go's hand-maintained 35-entry table diverges from methodology.Tools (41 entries): stale `hotam_req`, missing 8 real commands. Make methodology.Tools the single registry; derive claims/enforcers from it (or move those fields into the registry). Add a completeness test: registry ↔ CLI wiring ↔ generated requirements docs.

### T-b (P1) — bare-domain gen-spec resolution
`repoRootForDomain` (cmd/hotam/gen_spec.go) falls back to CWD project-root search for non-domains/<name> layouts, breaking `init <bare-dir>` → `gen-spec --domain <bare-dir>` outside a project. For an explicit --domain, use the domain dir itself as minimal root (no DOMAIN-MAP siblings) instead of erroring.

### T-c (P1) — active domain: marker/config + `hotam use`
Fix hardcoded defaultDomainRel: resolution order explicit --domain → HOTAM_DOMAIN env → active_domain recorded in .hotam-spec-project (set by init-project at scaffold time and by a new tiny `hotam use <domain>` command) → legacy default. Every command prints which domain resolved (honesty over magic — @fx consultation point 5).

### T-d (P1) — README ↔ CLI sync + drift test
Fix "thirteen"→actual, add init-project to the command section; add a test asserting README's command section covers every Implemented tool (pattern precedent: TestToolPurposeDocumentsRealFlags).

### T-e (P1) — burn-down metric split (root-cause fix, three sources demanded it)
Split the one-dimensional "closeable debt: N": ontology gains an optional `blocked_on` field on Requirement (naming the Planned tool/absent package); UNENFORCED.md renders closeable-now vs feature-blocked sub-tables; what-now's enforcement-gradient P0 counts ONLY closeable-now (feature-blocked becomes a lower-priority informational line). Backfill blocked_on for the 33 C1 items via proposals (data already grouped in 2026-07-13-c1-roadmap-debt-triage.md). This is the ontology+generator change previously deferred — now resolver-approved.

### T-f — structural boolFlagNames test
From the @fl review finding: a test that scans all fs.Bool(...) registrations across cmd/hotam/*.go (AST or grep-style, mirroring internal/selfcheck patterns) and asserts each name is present in boolFlagNames — closing the "comment, not test" gap.

### T-g — gen-spec --profile consumer|full
consumer profile skips: Planned tool pages, the 29 thinking docs, empty atom docs. init-project defaults to consumer for external projects; this repo's self-domains stay full. Hits Простота 7.0 directly (external seed 87 files → ~15-20).

### T-h — archive index for reviews/checkpoints
54 historical files / ~587KB under docs/reviews + docs/checkpoints: add a single INDEX.md (chronological, one-line summaries), optionally move superseded ones under archive/ subdirs. Cosmetic, cheap.

## Agent-UX track (@fx consultation, ranked by leverage-per-cost)

### T-i — `hotam propose` (build-first: "if you build only ONE thing, build this")
`hotam propose requirement --claim ... --owner ... --why ... [--axis|--assumes|--out]` writes valid proposal JSON itself (schema knowledge moves from agent memory into the tool), runs schema validation + an AUTOMATIC confront report before writing; optional --land collapses draft→confront→land into one call. Start with the 2-3 most common kinds; complex kinds keep the JSON path. Reuses internal/proposal.Validate + diagnose.Confront. ~250 lines cmd/hotam/propose.go.

### T-j — confront at the land/apply gate (warn, not fail)
land/apply-proposal automatically runs Confront for claim-carrying kinds and prints hits in the report — warn only (R-ai-presents-not-decides: tool presents evidence, resolver already decided; blocking would be theatre). Batch mode summarizes. ~40 lines in land.go/apply_proposal.go.

### T-k — `hotam brief <id>`
One call replacing show+context+related+due: claim/why/status/enforcement + graph neighbors + conflicts w/ axes + assumption liveness + freshness, --json. Aggregator over existing internal/query functions (~150 lines) + cmd (~80). 3-4 round-trips → 1 for the most frequent session operation.

### T-l — structural confront for proposal JSON (`confront --proposal <file>`)
Beyond lexical: shared-assumption clusters, axis co-reference, entity-state mentions — all already computed by inspect, parameterized to "graph node vs external candidate". The honest next step for old-vs-new mismatch detection; embeddings explicitly REJECTED for now (breaks stdlib-only + determinism; review agrees noise reduction comes first). Feeds T-i and T-j. ~150-200 lines refactor in internal/diagnose.

### T-m — `--json` everywhere + crystal command-list trim
Audit which commands lack --json (what-now, gate, all-violations at minimum) and add it. REJECTED (from the consultation): a separate agent-brief boot command — status --json already exists, the crystal loads free (substrate is FREE per §ContextBudget), one more boot surface = one more source of truth. Instead: shrink the crystal's embedded command reference to a one-liner pointing at `hotam -h`/`status --json`, reclaiming ~2-3KB.

## Explicitly rejected this wave
- Embedding-based semantic conflict detection (breaks stdlib-only + determinism; noise reduction first — review itself ranks it last).
- A separate `agent-brief` boot command (one more source of truth, against the review's own thesis).

## Execution
Sequential /crush sub-agents (established pattern), independent orchestrator verification after each (build/vet/gofmt/test -race -count=1 ./... in CLEAN env TMP/TEMP outside repo and home dir, all-violations both domains, gen-spec idempotency incl. explicit --claude-md regen), commit after each task, push + final @fl review at the end on explicit request.
