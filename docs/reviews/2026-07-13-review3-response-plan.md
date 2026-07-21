# Review 3 response plan (8.6/10, HEAD 93e0be8)

**Date:** 2026-07-13
**Source:** third independent review; score 8.6/10 (up from 8.4). Verdict: to reach 9/10, fix the generator's self-referential idempotency and close/reframe the remaining 42 enforcement-debt items.

## Verified findings

Both P0/P1 measurement claims were independently reproduced before planning:

- `CLAUDE.md:99` embeds `resident crystal 15323 chars` while the actual file is 16174 bytes — the generator measures the PRE-EXISTING crystal, then writes a new (differently-sized) one containing that stale measurement. Two consecutive `gen-spec --claude-md` runs therefore never converge in one pass (LIVE-STATE line changes each time), which is exactly why CI's regen-idempotency diff goes non-zero.
- `domains/hotam-spec-self/docs/gen/AGENT-CONTEXT.md:11` shows `resident crystal 0 chars` — generated in a `gen-spec` run without `--claude-md`, so the two artifacts disagree about the same fact depending on invocation mode.

## Plan

### 1. P0 — self-referential crystal measurement (review items 1+2, one root cause) → task

Make the LIVE-STATE `resident crystal N chars` measurement a **fixpoint computed at generation time**, not a read of the stale pre-existing file:

- Render the crystal, measure the RENDERED output, re-render with that measurement embedded, repeat until stable (converges in ≤2-3 iterations since only the digits change; add an iteration cap + error if it oscillates).
- Both the root crystal AND AGENT-CONTEXT.md must consume the SAME measurement from the same render pass — no more `--claude-md`-mode-dependent disagreement (`0` vs real).
- Alternative considered and rejected: dropping the size line from tracked docs entirely — it's load-bearing content (`R-context-budget-rule` is ENFORCED on CRYSTAL_CHARS; the budget line is the operator's context-budget instrument, not decoration).
- CI's idempotency check must go green as a result: a fresh regen (pinned `--today`) over a clean tree yields zero diff. Add a Go test pinning double-run convergence including the size line.

### 2. P1 — 42 closeable debt: implement short-form rendering (review item 3) → task

The review names one concrete, already-resolver-briefed item: `R-crystal-carries-short-form` (generator still uses `runeTruncate`, mechanically truncating mid-word, which the SETTLED claim prohibits). This was item #1 in the C2 decision brief (`2026-07-13-category-c2-decision-brief.md`) with recommendation (a): implement summary-first/first-sentence-fallback short-form rendering. The review independently pushes the same direction — take it as a task now, coupled with brief item #5 (`R-operator-crystal-embeds-thinking-distilled`) ONLY if budget allows after measuring; primary scope is replacing `runeTruncate` call sites in crystal rendering with a short-form helper + test + flipping the requirement to ENFORCED.

The other ~40 debt items stay as-is this wave: 33 are C1 (feature-blocked: tickets/hooks/agents/delegation — honest roadmap debt), the rest are C2 resolver-decision items already presented in the decision brief. No mass reclassification without resolver picks.

### 3. P2 — evidence on 4/236 SETTLED (review item 4) → declined this wave, policy already stated

Mass re-attestation of ~232 historical requirements would be exactly the administrative-theatre anti-pattern the evidence mandate exists to prevent (a stamp without a genuine re-review). The stated policy (documented in `docs/PROPOSAL-REFERENCE.md`'s ReviewMark section this session): evidence accumulates naturally as each requirement reaches its own `review_after` date (all 236 now have one; horizon ≈ 2027-01). No task.

### 4. P2 — stale docs (review item 5) → task

- `docs/QUICKSTART-CONSUMER.md:159` describes an already-fixed Windows-filename problem — remove/correct.
- `gen-spec`'s documented usage (registry Purpose string, README, tool doc) doesn't mention `--today`/`--claude-md` — sync all doc surfaces with the real flag set. Sweep other commands' Purpose strings for the same drift while there.

### 5. P2 — release tag (review item 6) → task, gated on explicit user push approval

`go install ...@latest` can't resolve without a git tag. Tag `v0.1.0` on a green HEAD after items 1-2 land. Pushing a tag is an outward-facing action — requires the resolver's explicit go-ahead at execution time (per standing rules, never push without explicit request).

## Execution

Sequential `sh` sub-agents (same pattern as the two prior review-response waves), independent orchestrator verification after each task (build/vet/gofmt/full tests/all-violations both domains), commit+push only on explicit user request at the end.
