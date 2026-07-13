# Review 4 response plan — push to 9/10 (HEAD 66e4af2)

**Date:** 2026-07-13
**Source:** fourth independent review; scores Применяемость 7.6 / Простота 7.2 / Развиваемость 8.0 / Поддерживаемость 8.2 / Удобство-для-агентов 8.1 / Общая 7.9. Second independent agent: 7.9–8.1 range.

## Verification (done before planning, against live HEAD 66e4af2)

Every claim re-checked against the repo myself:

| # | Claim | Verdict |
|---|---|---|
| 1 | Boolean flags broken: `reorderFlagsFirst` (cmd/hotam/main.go:158) consumes the next non-dash arg as ANY `--flag`'s value, so `--json "claim"` eats the positional. | CONFIRMED — real bug. |
| 2 | CRYSTAL_CHARS check reads root CLAUDE.md via global `paths.ProjectRoot()` (internal/invariants/lifecycle_checks.go:385), not relative to `--domain` → external domain gets HotamSpec's own crystal size / a false violation. | CONFIRMED — real bug, SAME family as the CI regression just fixed in gen_spec.go. BONUS: uses `len(string(data))` (bytes) while the generator embeds `utf8.RuneCountInString` (runes) since task #102 — measurement mismatch. |
| 3 | 27 Planned tools of 40 in the registry bloat REQUIREMENTS.md; stale paths/terms. | CONFIRMED count (27 Planned / 13 Implemented). Review's size figure (24.7 KB) is wrong — REQUIREMENTS.md is 107 KB; gen file count 84 not 85. Core redundancy point stands. |
| 4 | No `schema_version` on Graph (ontology/graph.go:3); loader `DisallowUnknownFields()` (loader.go:33) → a future field breaks old CLIs with no migration layer. | CONFIRMED (forward-looking, not a live bug). |
| 5 | 41 SETTLED closeable debt (38 PROSE + 3 STRUCTURAL). | CONFIRMED. |
| 6 | README says `@latest` won't resolve without a release tag (README.md:26). | CONFIRMED. |

## Steward decisions taken

- Scope: #1, #2, #3, #4, #5 all IN. Plus self-proposed reliability/maintainability items (below).
- #6 release tag: **declined again** — steward chose "leave without tag" this wave (README already documents the build-from-source / @commit workaround honestly).

## Plan

### T-a (#1, P1) — fix boolean-flag reordering + structural guard
`reorderFlagsFirst` must know which flags are boolean (take no value): `--json`, `--version`, and any other value-less flag. A boolean flag must NOT consume the following token. Implement a known-boolean-flag set (or parse per-subcommand flag definitions) so `hotam confront --json "claim" --domain X` works in any order. Add a test covering the exact failing invocation from the review. Self-proposed maintainability guard: centralize + test so this class can't silently regress.

### T-b (#2, P1) — domain-relative CRYSTAL_CHARS + rune count + reliability sweep
`checkOperatorWithinBudget`'s CRYSTAL_CHARS branch must resolve CLAUDE.md relative to the domain being checked, not global `paths.ProjectRoot()`. The invariant check currently receives only the Graph — thread the domain root (or the resolved CLAUDE.md path) into the check. Switch `len(string(data))` → `utf8.RuneCountInString` to match the generator's post-#102 measurement. **Self-proposed sweep:** audit ALL invariant checks and commands for the same global-CWD file-read pattern that should be domain-relative (this bug + the CI regression are two instances of one family — find any others).

### T-c (#3) — trim seed output for external consumers
Reduce first-contact cognitive load: keep Planned tools out of the consumer-facing REQUIREMENTS.md (or clearly separate Implemented from Planned), fix stale CLAUDE.md references / Python-era terms / dead paths in generated seed docs. Scope carefully — this is about what the seed domain SHIPS, decided with the steward.

### T-d (#4) — graph.json schema_version + tolerant loader
Add a `schema_version` field to Graph, write it during generation, and give the loader a version-aware path: known version → proceed; newer/unknown → a clear migration error instead of an opaque DisallowUnknownFields decode failure. Bounded migration layer, not a full framework.

### T-e (#5) — enforcement debt triage (biggest, last)
The 41 closeable debt (38 PROSE + 3 STRUCTURAL, all ENFORCEABLE). Triage each: write a genuine enforcement test where truly enforceable (flip to ENFORCED), or reclassify to INHERENTLY_PROSE where mechanical enforcement would be theatre. NOT mass evidence-backfill (declined twice before). Same discipline as the earlier A-batch enforcement waves this session.

### C1 roadmap-debt triage (emerged mid-wave via steward Q&A — task #117)

An `@fm` advisory consultation this session concluded that the remaining feature-blocked items in T-e's residual cannot honestly be resolved by "write a test or reclassify" — they describe features that do not exist in the codebase yet (ticket engine, attention core, sub-agent hierarchy, sensorium hooks, land-log, audit tools, perimeter guards, test-tiering, etc.). The full triage is in [`docs/reviews/2026-07-13-c1-roadmap-debt-triage.md`](2026-07-13-c1-roadmap-debt-triage.md): 33 items, grouped into 10 clusters by blocking feature, anchored on the already-steward-approved `R-speculative-aspects-frozen` principle. The document also identifies (but does not implement) the root-cause fix: splitting the burn-down metric to visually separate "closeable-now" from "feature-blocked" — a future-wave ontology+generator change.

## Execution
Sequential /crush sub-agents (established pattern), independent orchestrator verification after each (build/vet/gofmt/test -race -count=1 ./.../all-violations both domains, in a CLEAN env — TMP/TEMP outside the repo AND outside this machine's contaminated home dir, since C:\Users\Computer has stray domains/ + CLAUDE.md + .claude that false-positive project-root resolution). Commit after each task; push + final @fl review at the end on explicit request.
