# Category C2 enforcement-debt decision brief

**Date:** 2026-07-13
**Author:** AI agent (task #87), re-verifying an earlier fm consultation against current graph/code state
**Audience:** human resolver
**Rule in force:** R-ai-presents-not-decides / R-decided-needs-human-signoff — this document presents options with anchors; it does not decide anything. No `enforcement`/`enforceability`/`status` field on any of the 9 requirements below has been changed by this task, and no proposal has been written or landed.

## How to read this

Each section: **current verified state** (re-fetched today via `hotam req show <id> --domain domains/hotam-spec-self --json`, cross-checked against code/filesystem) → **options** (corrected where re-verification changed the picture) → **scope/risk read** per option that implies work → **recommendation** (opinion, not a decision — the resolver picks).

All 9 items are currently `status: SETTLED`, `enforceability: ENFORCEABLE`, `enforcement: STRUCTURAL` or `PROSE` (i.e., still-open closeable debt — none were touched by this session's A/B batches). Verified via direct `req show --json` calls on 2026-07-13.

---

## 1. R-crystal-carries-short-form

**Claim:** crystal generator shall render every object with a meaningful short form (explicit summary, else first whole sentence), not mechanical truncation.

**Current verified state:** `enforcement: STRUCTURAL`. Confirmed still true: `internal/generator/claudemd.go` uses `runeTruncate` — mechanical rune-count truncation — for imperative text. The `Requirement` struct already carries a `Summary` field (`internal/ontology/requirement.go`) that a short-form renderer would consult first, but nothing wires it in yet. Unchanged from fm's original framing.

**Options:**
- (a) Implement short-form rendering (summary-priority, first-sentence fallback) + a test that asserts no mid-word truncation → ENFORCED.
- (b) Amend claim to legalize mechanical truncation as an honest downgrade.

**Scope/risk:**
- (a) — **Medium.** Touches one function area (`internal/generator/claudemd.go`), needs a short-form helper (summary-first, first-sentence fallback) and a focused test. Contained blast radius (crystal rendering only), but requires care that no existing crystal-size/budget invariant (R-context-budget-rule, ENFORCED, CRYSTAL_CHARS-gated) regresses — the LIVE-STATE budget line depends on the exact rendered length. Also intersects item 5 below (same rendering path).
- (b) — **Small.** Reword-only.

**Recommendation:** (a) is worth doing — the resolver's own 2026-07-05 verdict ("all that gets truncated must not be truncated but have a short version") is explicit and the `Summary` field already exists as the natural hook, so the work is mostly wiring, not design. Note the coupling to item 5 (both touch the same crystal-rendering surface) — if the resolver picks (a) here, doing items 1 and 5 together in one pass avoids touching `claudemd.go` twice.

---

## 2. R-empty-content-gen-notice

**Claim:** when the active domain has no content yet (**missing** graph.json), `gen-spec` shall emit a "no content yet" notice, not fail.

**Current verified state — narrowed by this session's own landed work, confirming fm's caveat was correct.** `R-empty-content-wellformed` and `R-empty-content-calm-banner` both landed ENFORCED this session (batch A5, `TestEmptyContentWellFormed` / `TestEmptyContentCalmBanner`). I read both full `why` fields directly: **both explicitly say the EMPTY-but-present-graph case is now covered, and both explicitly flag the MISSING-file case as the remaining honest gap** — verbatim from `R-empty-content-calm-banner`'s why: *"the 'missing graph.json' case... is NOT yet handled with a calm banner: loader.LoadGraph... returns a read error and what-now propagates it rather than rendering a welcoming notice."* This item (#2) is specifically about `gen-spec` (not `what-now`) on a missing file — confirmed still true: `cmd/hotam/gen_spec.go` → `loadDomainGraph` → `loader.LoadGraph` hard-errors via `os.ReadFile` on an absent `graph.json`, and `gen-spec` propagates that error.

**So: item 2 is NOT redundant with the landed A5 pair — it covers exactly the gap those two items explicitly left open, just for a third code path (`gen-spec` instead of `what-now`/invariants).** Original option (c) "REJECT with REPLACES to that pair" is now demonstrably wrong and should be dropped — confront-worthy overlap check confirms no duplicate coverage exists. The narrower, corrected framing is (b): item 2's true remaining scope is the missing-graph.json path for `gen-spec` specifically (and, by the same pattern, `what-now` still has the identical gap per its own landed why-text, though that's not this item's anchor).

**Options (corrected):**
- (a) Implement the missing-graph "no content yet" notice path for `gen-spec` + a test → ENFORCED.
- (b) ~~REJECT with REPLACES~~ — **ruled out this session**: verified no overlap exists; dropping this option.
- (c) Leave as debt, but re-word the claim/why to make explicit it's now the *narrower* missing-file-only gap (both empty-but-present cases are handled elsewhere).

**Scope/risk:**
- (a) — **Small.** One error-path branch in `gen_spec.go`/`loader.go`: detect `os.IsNotExist` on the graph.json read, render a placeholder into `docs/gen/*.md` instead of propagating the error, plus one test. Directly analogous to the just-landed A5 pattern (`TestEmptyContentWellFormed`, `TestEmptyContentCalmBanner`), so there's a fresh in-repo template to copy.

**Recommendation:** (a). This is the cheapest, most template-ready item on the whole list — the sibling pair landed this session shows exactly the shape of enforcer + test to write, and the remaining gap is real and narrow (confirmed by the two landed items' own text, not assumed). Worth flagging to the resolver that the identical gap also exists in `what-now` (per `R-empty-content-calm-banner`'s own why-text) even though that's not tracked under this anchor — the resolver may want to fold that in as a twin fix rather than leaving it to surface as its own future finding.

---

## 3. R-domain-owns-tools-and-agents

**Claim:** every `domains/<name>/` shall contain `tools/` and `agents/` subdirs, even if empty.

**Current verified state:** `enforcement: STRUCTURAL`, unchanged. Confirmed via filesystem: neither `domains/hotam-spec-self/` nor `domains/hotam-dev/` has `tools/` or `agents/` subdirectories today. Confirmed via code: the three `check_agent_has_*_subdir` functions in `internal/invariants/domain_structure.go` are honest no-ops (they operate on the in-memory graph, not the filesystem, so they cannot see directory absence). `create_domain` scaffolding tool remains `Planned` in `internal/methodology/tools_data.go`, not implemented. All matches fm's original framing exactly — no drift.

**Options:**
- (a) Scaffold `.gitkeep`'d `tools/`+`agents/` dirs in both existing domains + a new filesystem-aware check.
- (b) Amend claim to "materialize at spawn time" (i.e., dirs appear only when a sub-tool/sub-agent is actually created, not eagerly).
- (c) REJECT with REPLACES to `R-claude-md-consolidates-when-single-agent`.

**Scope/risk:**
- (a) — **Small for the scaffolding itself** (two empty dirs + two `.gitkeep` files), but the "new filesystem-aware check" part is a **category change**: every other invariant check in `internal/invariants` operates on the in-memory `ontology.Graph`, never touches disk. Adding one that does is a small architectural precedent-setter (worth the resolver's awareness, not necessarily a blocker).
- (b) — **Small.** Reword only; consistent with the actually-observed pattern (`R-claude-md-consolidates-when-single-agent`, cited in CLAUDE.md's own Recursion section: *"Sub-agent crystals materialize only at real spawn time"*) — this is the same lazy-materialization philosophy already SETTLED elsewhere in the graph.
- (c) — **Small** (a REPLACES edge + status change), but weakens the requirement's standalone claim about `tools/`+`agents/` specifically (that RULE targets domain-level dirs; `R-claude-md-consolidates-when-single-agent` targets CLAUDE.md consolidation) — the two are adjacent, not identical, so REJECT risks losing a distinct, still-true claim.

**Recommendation:** (b). The project already has a SETTLED lazy-materialization precedent for the closely related sub-agent-crystal case; extending the same logic to `tools/`+`agents/` subdirs is consistent and cheap, and avoids inventing the first filesystem-touching invariant check for a low-value structural nicety. (c) is available but risks conflating two distinct claims — flagging it as the weaker option.

---

## 4. R-project-name-hotam-spec

**Claim:** project name shall be `Hotam-Spec` (display) and `hotam-spec` kebab-case for filesystem/repository/**Go module path suffix**.

**Current verified state — this item has visibly drifted since fm's original pass and needs re-framing.** The claim text itself already changed this session (history shows a 2026-07-13 edit from a Python-era wording to the current Go-module wording), and its own `why` field now states: *"Renames completed in three sequential passes (#89 package, #90 domain, #91 prose)"* — referencing tasks that already landed (verified in git log: `465b778` rename #1 Python package, `2ba9c96` rename #2 domain, `d5f3206` rename #3 prose). **But the module path itself was separately and deliberately changed** in commit `4325ac8` "chore: rename Go module to match the real git remote (P1-6)" — confirmed today: `go.mod` reads `module github.com/PHPCraftdream/HotamSpec`. So the current mismatch is real and intentional-on-both-sides: the claim demands kebab-case (`hotam-spec`), the actual module path uses `HotamSpec` (PascalCase suffix) matching the real GitHub remote name `PHPCraftdream/HotamSpec`. This was a conscious choice (commit message says "to match the real git remote"), not an oversight — worth the resolver knowing this wasn't accidental drift.

**Options:**
- (a) Rename the Go module path suffix to kebab-case (`github.com/PHPCraftdream/hotam-spec` or similar) to match the claim.
- (b) Amend claim to legalize the current module suffix (match the real repo name) while keeping kebab-case for domain-level artifacts (folder names, CLI binary name, etc., which already are kebab/lowercase).

**Scope/risk:**
- (a) — **Large, genuinely repo-wide, real risk of missing a spot.** Verified count today: **119 `.go` files** reference `PHPCraftdream/HotamSpec` in their import paths, **178 total occurrences** (some files have multiple imports of internal packages). This spans every `internal/*` package plus all of `cmd/hotam/*.go` — essentially the whole compiled surface. A module rename requires: updating `go.mod`, updating every import statement (mechanical but total-coverage-dependent — a single missed import breaks the build, not silently), likely `go build ./...` + full test suite to catch every miss, and separately confirming the real GitHub remote name matches (or renaming the remote too, which is outside this repo's control surface and resolver-only). This also reverses a **conscious decision made in the same session-family** (`4325ac8` explicitly renamed *to* match the remote) — doing (a) would be an explicit reversal of recent intentional work, which the resolver should weigh, not just a technical risk.
- (b) — **Small.** Reword-only; the claim already documents the current module path as a parenthetical historical note, so this is tightening language to match already-accepted reality.

**Recommendation:** (b), with an explicit flag that (a) is the highest-risk option in this entire batch of 9 — a 119-file/178-occurrence mechanical rename that also reverses a deliberate recent commit (`4325ac8`). I am not recommending (a); if the resolver wants the module path to be kebab-case badly enough to justify that reversal and the migration risk, that's a call only the resolver should make, with eyes open to the scope above.

---

## 5. R-operator-crystal-embeds-thinking-distilled

**Claim:** CLAUDE.md shall embed one RULE sentence per thinking topic via `short_form`, plus a path link — not a multi-line RULE+WHY distillate.

**Current verified state:** `enforcement: STRUCTURAL`. Confirmed via code comment/architecture: the actual renderer emits only a slug-index (topic list + path links — see CLAUDE.md's own "Methodology — how to think" section, which is exactly a flat list of `§`-anchors with no per-topic RULE sentence). This was a deliberate context-budget cut from an earlier wave per the requirement's own `why` (REPLACES `R-operator-crystal-embeds-thinking`, the older multi-paragraph approach that cost ~19k chars across 22 topics vs. ~2.4k chars for the slug-index). Matches fm's original framing — no drift, but note this item literally depends on the same `short_form` mechanism named in item 1 (`R-crystal-carries-short-form`), which does not exist yet either.

**Options:**
- (a) Add a `ShortForm` field to the relevant struct + implement full one-RULE-per-topic distillation.
- (b) Amend claim to match the shipped compact-index behavior (slug-index only, no per-topic RULE sentence).
- (c) Defer, bundled with item 1's short-form work if the resolver picks (a) there.

**Scope/risk:**
- (a) — **Medium-to-large.** This is the more ambitious sibling of item 1: item 1 needs a short-form *string* per object; this item needs that string *embedded into the live CLAUDE.md render* for every one of the ~30 `§`-topics, which touches the generator's CLAUDE.md-assembly logic and the char-budget accounting (the whole point of the REPLACES history here is that the fuller form blew the budget once already — 19k vs 2.4k chars). Real risk of budget regression if not done carefully against `R-context-budget-rule` (ENFORCED, CRYSTAL_CHARS-gated).
- (b) — **Small.** Reword-only, and arguably the more honest state given the REPLACES history already explains *why* the compact form was chosen deliberately.
- (c) — **N/A cost by itself** — just sequencing advice.

**Recommendation:** (c) if the resolver picks (a) on item 1; otherwise (b). This item's claim is aspirational against a budget constraint the project already hit once and deliberately backed off from (that's what the REPLACES chain records) — re-attempting the fuller distillation should not happen in isolation from item 1's short-form primitive, and should be watched closely against the char budget given the documented history of blowing it.

---

## 6. R-work-within-launch-dir

**Claim:** operator shall confine all file mutations to its launch working directory, never touching the host harness/global config, absent explicit user request.

**Current verified state:** `enforcement: PROSE` (already downgraded from ENFORCED — history shows it briefly went to `ENFORCED` on 2026-07-10 via `test_launch_dir_write_scope.py` then reverted to `PROSE` on 2026-07-12 when that AST-scanner test wasn't carried forward into the Go port). The claim is explicit that this is genuinely two different things bundled: (1) a structural half — an AST scanner over committed Go source for home-directory write sinks, which historically existed and could exist again — and (2) a live-agent-runtime-conduct half (an agent shelling out at runtime) that is "discipline-of-prose" per the claim's own text, citing `R-agent-conduct-is-rules-not-tests`.

**This is worth flagging precisely because it's a partial mismatch with fm's original framing:** fm characterized this as wholesale "structurally unobservable," but the current claim text says the opposite for half of it — a committed-code AST scanner is fully buildable (it existed once, in Python, and its rules are stated precisely in the why-field: scan for `os.UserHomeDir`/home-path literals co-located with filesystem-write sinks). Only the *live shell-command* half is truly unobservable to a static test suite.

**Options (corrected):**
- (a) Reclassify INHERENTLY_PROSE — but this would be **inaccurate for the whole claim**, since half of it (the static AST-scanner half) is mechanically checkable and was mechanically checked as recently as 2026-07-10. Reclassifying the *whole* requirement INHERENTLY_PROSE would be a step backward from where it stood 3 days ago.
- (a') **Corrected option**: split the claim (same atomicity discipline used across this session's category-B batch, e.g. R-requirement-claim-is-atomic) into two: a structural half (re-port the AST scanner to Go, over `internal/`+`cmd/hotam/`) that stays ENFORCEABLE, and a runtime-conduct half that becomes INHERENTLY_PROSE.
- (b) Couple to a future Go sensorium PreToolUse guard (defers into C1 feature-blocked territory).
- (c) Partial/narrow enforcement (flagged by fm as possibly dishonest if it implies more coverage than it has).

**Scope/risk:**
- (a') — **Small for the split** (a proposal + REPLACES edges, same mechanical pattern as the 7 already-landed category-B atomicity splits), **small-to-medium for re-porting the AST scanner** (a Go source-scanner over `internal/`+`cmd/hotam/` for `os.UserHomeDir`/`~`-literal usage co-located with write-sink calls — self-contained, one new test file, no risk to other subsystems).
- (b) — Explicitly C1 (feature-blocked) territory; not really a C2 action today.

**Recommendation:** (a') — re-splitting this into a structural half (re-enforce the AST scanner, it's cheap and it already worked once) and a PROSE half (the live-conduct discipline) is more honest than a blanket INHERENTLY_PROSE reclassification, and reuses the exact atomicity-split pattern this session already validated 7 times in category B. Flagging that plain "reclassify whole thing INHERENTLY_PROSE" (fm's original (a)) would actually be a regression from the requirement's state 3 days ago, not a neutral move — the resolver should know that before picking it.

---

## 7. R-working-vs-substrate-budget

**Claim:** context budget shall bound only the WORKING store of active uncrystallized knowledge, leaving crystallized substrate unbounded.

**Current verified state:** `enforcement: PROSE`. Confirmed: `ReflectOverBudgetOperators` (`internal/diagnose/finding.go`) measures only live graph nodes (`len(Requirements)+len(Conflicts)+len(Assumptions)`), never the crystallized substrate — matches the claim. This is a REFLECTION-tier Finding (advisory), not a gate — confirmed it doesn't block a write, only surfaces via `hotam what-now` / the LIVE-STATE budget line.

**Checked fm's suggested cross-reference — `R-context-budget-rule`:** fetched directly, confirmed **`enforcement: ENFORCED`**, `enforced_by: [check_operator_within_budget]`. Its own `why` field explicitly states the relationship: *"History: NODE_COUNT originally measured the crystallized SUBSTRATE... which R-working-vs-substrate-budget declares free — this falsely flagged operators... OP-director moved to CRYSTAL_CHARS... the RESIDENT crystal... is now the thing actually metered, per R-working-vs-substrate-budget."* So `R-context-budget-rule` is the ENFORCED sibling that already carries the mechanical teeth this item's *principle* motivates — confirmed still accurate, no drift from fm's framing.

**Options:**
- (a) Reclassify INHERENTLY_PROSE.
- (b) REJECT with REPLACES to `R-context-budget-rule` — confirmed accurate: that item is ENFORCED and its own why-text explicitly credits this item as the design rationale it operationalizes.
- (c) Leave as debt.

**Scope/risk:**
- (a) — **Small**, same one-line reclassification pattern as the 7 landed category-B items (e.g. `R-crystal-is-claude-md`, `R-crystal-reload-by-reference` — both explicitly RECLASSIFIED to INHERENTLY_PROSE this session for the same reason: architectural/design-principle claims where "any test would be vacuous or just re-assert the design," which is this item's exact fm-given rationale).
- (b) — **Small** (a REPLACES edge + status→REJECTED), but this would retire the *principle* statement entirely in favor of only the *mechanism* statement — loses the "why the mechanism is shaped this way" documentation that `R-context-budget-rule`'s own why-text currently leans on by cross-reference. If REJECTed, that explanatory content would need folding into `R-context-budget-rule`'s why (or it becomes orphaned rationale).

**Recommendation:** (a). This is architecturally identical to the already-landed `R-crystal-is-claude-md`/`R-crystal-reload-by-reference` INHERENTLY_PROSE reclassifications from this session's category B — same "design principle, any test is vacuous" shape. Preferred over (b)/REJECT because `R-context-budget-rule`'s own why-text currently *depends on* this item's explanation for its own justification; REJECTing the source of that explanation without migrating the content first would leave a dangling citation.

---

## 8. R-speculative-aspects-frozen

**Claim:** Entity aspect, multi-domain federation, sub-agent recursion machinery shall receive no inward development while frozen, unfreezing only on real business-domain need.

**Current verified state:** `enforcement: PROSE` (downgraded from ENFORCED on 2026-07-12 — the original `sha256` hash-baseline guard, `test_frozen_aspects_snapshot.py`, was never carried forward into the Go port). Confirmed via code: `internal/ontology/entity.go` exists but is inert (0 entity_types/entities), `internal/methodology/tools_data.go` still lists `create-agent`/`spawn-agent`/`invoke-agent`/`create-domain` as `Planned`. Matches fm's framing exactly.

**Checked fm's suggested cross-reference — `R-enforcement-perimeter-visible`:** fetched directly, confirmed **still `PROSE`, not enforced** — its own why-text says explicitly *"there is no sha256 hash-pin test covering the enforcement-perimeter source files... PROSE (not ENFORCED): no enforcer exists."* So option (a) below ("hash-ratchet pin... overlaps R-enforcement-perimeter-visible's mechanism") is accurate: that sibling mechanism is *also* still unimplemented, meaning doing (a) here would mean either building the hash-pin machinery twice (once generically for the enforcement perimeter, once specifically for the frozen aspects) or building one shared mechanism that serves both items at once.

**Options:**
- (a) Hash-ratchet pin on the frozen files — mechanical but noisy; confirmed overlaps `R-enforcement-perimeter-visible`'s still-unimplemented mechanism.
- (b) Reclassify INHERENTLY_PROSE.
- (c) Retire when a real business domain unfreezes the aspects (i.e., leave as-is, doesn't require action today).

**Scope/risk:**
- (a) — **Medium**, and specifically **not clearly worth doing standalone**: building a one-off hash-pin just for the 2-3 frozen files (`entity.go`, `tools_data.go`'s frozen entries, scope_process.go's no-op checks) when the *general* enforcement-perimeter hash-pin (`R-enforcement-perimeter-visible`) doesn't exist yet either means either duplicating work later or scoping this narrowly now and redoing it. If the resolver wants both, doing `R-enforcement-perimeter-visible`'s general mechanism first and covering the frozen files as one instance of it is the efficient order — but that's a two-item bundle, bigger than "just this C2 item."
- (b) — **Small**, one-line reclassification — but this item is *not* purely a design-principle claim like items 6/7's INHERENTLY_PROSE candidates; it's a concrete "don't touch these files" operational rule that genuinely *could* be hash-pinned (unlike, say, `R-working-vs-substrate-budget`'s architectural principle). Reclassifying INHERENTLY_PROSE here would be less honest than items 6/7 — flagging this distinction explicitly.
- (c) — **No cost**, status quo.

**Recommendation:** (c), leave as debt for now, with a note that if the resolver independently wants to invest in `R-enforcement-perimeter-visible`'s general hash-pin mechanism (item 8's sibling, not in this batch of 9), the frozen-aspects guard should ride along as one instance of that rather than being built twice. Not recommending (b) — unlike items 6/7, this claim is concretely mechanizable, so INHERENTLY_PROSE would understate what's actually possible here.

---

## 9. R-presented-pending-decision-type

**Claim:** presented-awaiting-decision state shall live in a dedicated `pending/`/`applied/` runtime folder split, surfaced by the harness — not a new graph node type.

**Current verified state:** `enforcement: PROSE`. Confirmed via code: `cmd/hotam/what_now.go` has no pending-proposal signal producer (the `P6`/`PENDING_PROPOSAL` label exists in `priorityLabel` as dead code only), no `pending/`/`applied/` folder split, no `.runtime/` proposal tracking.

**Concrete evidence, freshly counted today — this is the item where fm's original framing has most visibly strengthened.** Listed the actual current `proposals/` tree:

```
proposals/wave1-draft-resolution
proposals/wave1-replaces-edges
proposals/wave2-enforced-by
proposals/wave3-content-refresh/  (batch01..batch08b, 9 subfolders)
proposals/wave3-gate-selector-reword
proposals/wave3-hotam-dev-enforced-by
proposals/wave4-freshness-regimen
proposals/wave5-deportify/  (hotam-dev, hotam-spec-self)
proposals/wave6-enforcement-debt/  (category-a, category-b, p0-freshness,
    p0-settled-at-recovery, p1-inspect-confront-noise, p1-transactional-land,
    p2-json-null-arrays, p2-status-command — 8 subfolders)
```

That's **9 top-level waves**, several with multiple sub-batches, and **wave6 alone landed 8 sub-batches in this single session** (categories A/B plus 6 P0-P2 fixes) — a genuinely rich, real, in-repo precedent for "wave-numbered folders under `proposals/` as the de facto pending/applied-adjacent practice," exactly as the task brief anticipated. Every file in every wave folder here already represents a *landed* (applied) proposal — there is currently no *pending* (awaiting-decision) folder in active use, so the wave-folder practice as observed today models the "applied" half of the split well, but doesn't yet demonstrate a live "pending" counterpart in this repo (this task's own C2 items are themselves an example of "awaiting decision" material, just not folder-tracked as JSON proposals).

**Options:**
- (a) Implement a real pending/applied split + what-now surfacing.
- (b) Amend claim to legalize the wave-folder practice — now has substantially more real precedent (9 waves, several multi-batch) than when fm first looked.
- (c) Leave as debt until proposal volume justifies it.

**Scope/risk:**
- (a) — **Medium.** Needs: a `.runtime/proposals/pending/` writer (something writes a proposal there before resolver approval — today proposals are written straight into `proposals/waveN-.../` after approval, with no separate pre-approval staging step in the current workflow), a mover on `apply-proposal`/`land` (pending→applied), and a `what-now` signal producer to fill the already-stubbed `P6`/`PENDING_PROPOSAL` label. The mover/signal parts are small; the bigger open question is workflow — the current practice (per the mediation loop in CLAUDE.md itself: TRANSLATE drafts a `.json` file, PRESENT shows the resolver, LAND applies after approval) doesn't have an obvious "pending" state distinct from "not yet written" — proposals seem to go from idea straight to resolver review to applied, without an intermediate on-disk pending artifact today. Implementing (a) faithfully would mean also changing *when* a proposal file gets written (before vs. after approval), which is a workflow change, not just a folder-mover.
- (b) — **Small.** Reword-only, and the precedent argument is now much stronger than fm's original pass had available (9 waves vs. presumably fewer at the time of the original consultation).

**Recommendation:** (b). The observed practice — wave-numbered folders under `proposals/`, now with 9 top-level waves and multiple multi-batch waves landed in this session alone — already serves the practical need this claim was reaching for, and legalizing it is cheap. (a)'s "real" pending/applied split would require a workflow change (an on-disk pre-approval staging artifact that doesn't exist in current practice) on top of the mechanical folder-mover, which is more invasive than it first appears — flagging that the true cost of (a) is a workflow redesign question for the resolver, not just an engineering task.

---

## Summary table

| # | Item | Recommendation | Note |
|---|------|-----------------|------|
| 1 | R-crystal-carries-short-form | (a) implement short-form rendering | medium scope; couples with #5 |
| 2 | R-empty-content-gen-notice | (a) implement missing-graph notice for gen-spec | small scope; template exists from landed A5 pair; REJECT option ruled out — no overlap |
| 3 | R-domain-owns-tools-and-agents | (b) amend claim to "materialize at spawn time" | matches existing SETTLED lazy-materialization precedent |
| 4 | R-project-name-hotam-spec | (b) amend claim to legalize current module path | (a) flagged high-risk: 119 files/178 import occurrences, reverses a deliberate recent commit |
| 5 | R-operator-crystal-embeds-thinking-distilled | (c) defer, bundle with #1 if picked; else (b) amend | budget-history risk if attempted alone |
| 6 | R-work-within-launch-dir | (a') split into structural half (re-enforce) + PROSE half | corrects fm's blanket-INHERENTLY_PROSE framing — half is genuinely mechanizable |
| 7 | R-working-vs-substrate-budget | (a) reclassify INHERENTLY_PROSE | same shape as 2 already-landed category-B items |
| 8 | R-speculative-aspects-frozen | (c) leave as debt | (a) not worth doing standalone — overlaps unbuilt R-enforcement-perimeter-visible mechanism |
| 9 | R-presented-pending-decision-type | (b) amend claim to legalize wave-folder practice | precedent much stronger now: 9 waves, wave6 alone has 8 sub-batches |

No graph fields were modified, no proposal was written or landed, and no Go source file was touched in the course of producing this report.
