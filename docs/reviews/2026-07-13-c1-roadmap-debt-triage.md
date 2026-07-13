# C1 roadmap-debt triage — feature-blocked PROSE is honest, not neglected

**Date:** 2026-07-13
**Author:** AI agent (task #117), documenting an already-steward-approved classification surfaced via `@fm` advisory consultation this session
**Audience:** future external reviewer · `hotam what-now`'s P0 signal reader · the operator's own ORIENT step
**Rule in force:** R-speculative-aspects-frozen (SETTLED, steward-approved 2026-07-02) — the principle this document applies, not introduces. No `enforcement`/`enforceability`/`status` field on any requirement was changed by this task, no proposal was written, and no Go source or graph.json was touched.

## How to read this

This document is NOT presenting options for a steward decision (unlike its sibling `2026-07-13-category-c2-decision-brief.md`, which laid out choices the steward then picked). It documents an **already-settled classification** and explains why its current state is correct and will persist until a real business-domain need unfreezes the underlying features.

The problem it solves: `docs/gen/UNENFORCED.md`'s burn-down line is one-dimensional — a single `closeable debt: N` number that does not visually separate *closeable-now* items (genuinely enforceable today, just no test written yet) from *feature-blocked* items (the feature the requirement describes does not exist in the codebase at all). This conflation is why an external reviewer, `what-now`'s own P0 `enforcement-gradient` signal, or the burn-down line itself keeps re-flagging the same set as if it were neglected debt. It is not — it is honestly-documented future-feature roadmap, correctly labeled `enforcement: PROSE`, `enforceability: ENFORCEABLE`, blocked on features whose implementation is itself frozen by an already-approved steward decision.

---

## The anchor principle — R-speculative-aspects-frozen

The entire framing rests on one SETTLED requirement the steward already approved (2026-07-02, frozen after audit). Its current claim text, fetched verbatim via `hotam req show R-speculative-aspects-frozen --domain domains/hotam-spec-self --json`:

> "The Entity aspect, multi-domain federation, and sub-agent recursion machinery shall receive no inward development while frozen, unfreezing only when a real business domain demonstrates concrete need."

Its `why` field is explicit about the mechanism: the frozen surface is carried as `Planned` (not-implemented) methodology.Tools entries — `create_agent`, `spawn_agent`, `invoke_agent`, `create_domain` — plus the Entity aspect substrate that exists but is inert (`internal/ontology/entity.go` with 0 entity_types, scope_process checks that no-op when `g.EntityTypes` is empty). The freeze is carried by review discipline + Planned tool status, not by a structural guard (the requirement is PROSE, not ENFORCED — the original sha256 baseline test was not carried forward into the Go port). Unfreeze trigger: Phase 5, a real business domain.

**This document does not introduce a new policy.** It applies this already-approved principle honestly to explain why the 33 items below are correctly PROSE and will remain so until real business need justifies building the underlying feature. Forcing these items into "write a test or reclassify INHERENTLY_PROSE" (as an earlier, now-abandoned task framing proposed) would be dishonest on both counts:

- **You cannot write a real enforcement test for a feature that does not exist.** A test asserting `ticket_create` writes a file under `tickets/open/T-1.md` would pass only if `ticket_create` existed as a Go command — it is `Planned` in `tools_data.go`, not implemented. Writing a mock that simulates the tool's behavior and then asserting the mock's behavior is test theatre: it verifies nothing about the real system.
- **Reclassifying them `INHERENTLY_PROSE` would be factually wrong.** `INHERENTLY_PROSE` is reserved for claims no conceivable test could ever verify (architectural design principles, live-conduct discipline — the 7 items already moved there this session). These 33 items ARE mechanically enforceable in principle, once the feature is built — that is why they carry `enforceability: ENFORCEABLE`, not `INHERENTLY_PROSE`.

---

## Current state — derived fresh, not from a stale count

The burn-down line at the time of writing reads: **`closeable debt 37`** (SETTLED-ENFORCED 165 / SETTLED 237). This number dropped from 41 at the start of this review-4 wave: tasks #103, #114, #115, #116 each moved one item to ENFORCED or INHERENTLY_PROSE, and task #113 moved one more to INHERENTLY_PROSE (the 3 claim-amendment decisions in #113 honestly remained debt — their `enforcement`/`enforceability` did not change, only their claim text was corrected to match reality).

The 37 items in `docs/gen/UNENFORCED.md`'s "Closeable debt (ENFORCEABLE, no enforcer yet)" section split into two categories:

| Category | Count | What it is |
|---|---|---|
| **C1 — feature-blocked roadmap** | **33** | The requirement describes a feature that does not exist in the codebase. Correctly `ENFORCEABLE` + `PROSE`. No honest test can be written until the feature is built. |
| **C2 residual — steward-decided, claim-corrected** | **4** | Resolved this wave by task #113 (steward-approved claim amendments). The steward chose to correct the claim text to match reality rather than build an enforcer or reclassify. They remain honest debt by design — the steward is aware and decided to leave them. |

The 4 C2-residual items (NOT part of the C1 set documented below — included here only to account for the full 37):

| id | Why it stays debt |
|---|---|
| `R-domain-owns-tools-and-agents` | Claim amended to lazy-materialization semantics ("dirs appear only at real spawn time"). Steward picked option (b) from the C2 brief. Still ENFORCEABLE + PROSE. |
| `R-project-name-hotam-spec` | Claim amended to legalize the current Go module path (`github.com/PHPCraftdream/HotamSpec`, PascalCase per commit `4325ac8`). Module rename (119 files/178 imports) explicitly rejected as high-risk. |
| `R-speculative-aspects-frozen` | Steward picked option (c) "leave as debt." The freeze guard (sha256 hash-pin) overlaps the unbuilt `R-enforcement-perimeter-visible` general mechanism — not worth building twice. |
| `R-presented-pending-decision-type` | Claim amended to legalize the observed wave-folder `proposals/` practice (9+ waves of real precedent). No separate pending/applied runtime split needed. |

---

## The 33 C1 items — grouped by blocking feature

Each cluster names the specific feature that does not exist yet, cross-referenced to `internal/methodology/tools_data.go` (where the feature is registered as `Planned`) and/or the filesystem (where the package/directory is absent).

### Cluster A — Ticket engine (3 items)

**Blocking feature:** the `ticket_*` tool family — `ticket_create`, `ticket_edit`, `ticket_move`, `ticket_comment`, `ticket_list`, `ticket_show` — all `Planned` in `tools_data.go` (§Ticket). No Go code implements any of them; `rg "ticket_create|ticket_edit|ticket_move"` in `internal/`+`cmd/` finds only data/metadata files, no implementation. A `tickets/done/` directory exists on disk (used for manual tracking) but no `tickets/<status>/T-<n>.md` engine, no frontmatter parser, no History appender.

| id | Claim (short) | Blocking feature |
|---|---|---|
| `R-ticket-engine-on-disk` | Work items tracked as durable on-disk tickets under `tickets/<status>/T-<n>.md`, JSON-frontmatter + Markdown body, created/moved by `ticket_*` tools. | Entire `ticket_*` tool family (6 tools, all Planned). |
| `R-ticket-carries-history` | Every ticket carries an append-only `## History` section, each mutation records one entry, text changes snapshot prior text. | `ticket_*` tool family — the History appender is part of the engine that doesn't exist. |
| `R-open-tickets-visible` | `what-now` surfaces a CLI-only band summarizing open on-disk tickets by status, read from filesystem. | `ticket_*` tool family + `what-now` integration (the `open_tickets` signal producer does not exist in `internal/diagnose`). |

### Cluster B — Attention-sensing core (4 items)

**Blocking feature:** an `internal/attention` package — it does not exist on disk (`ls internal/attention` → not found). The `attention` and `attention_hook` tools are `Planned` in `tools_data.go` (§Attention). The CONCEPT-MAP in CLAUDE.md itself confirms: `§Attention — _(not yet implemented)_`. No `collect()` function, no `AttentionSignal` type, no registry.

| id | Claim (short) | Blocking feature |
|---|---|---|
| `R-attention-registry` | Agent-agnostic attention-source registry, `collect()` runs every source, returns typed `AttentionSignal` records. | `internal/attention` package (does not exist); `attention` tool (Planned). |
| `R-attention-agent-agnostic-core` | The attention core names no agent-platform token (Claude/Anthropic/hook/model), so a platform adapter is one consumer, never the owner. | `internal/attention` package (does not exist). |
| `R-attention-superset-of-diagnose` | `collect(g)` is a superset of `diagnose(g)`, equal when no runtime-fs sources injected. | `internal/attention` package — the superset relationship is only testable once both sides exist. |
| `R-attention-claude-adapter` | Committed sensorium generator wires the Claude attention adapter onto UserPromptSubmit, delegating to the core. | `attention_hook` tool (Planned) + `setup_hooks` tool (Planned) + `internal/attention`. |

### Cluster C — Sub-agent CLAUDE.md hierarchy (9 items)

**Blocking feature:** the sub-agent lifecycle — `create_agent`, `spawn_agent`, `invoke_agent` tools, all `Planned` in `tools_data.go` (§Agent). No agent directory exists under any `domains/*/agents/` except the seeded `director/`. `gen-spec` does not yet render per-agent CLAUDE.md crystals (the `R-claude-md-consolidates-when-single-agent` ENFORCED requirement explicitly states: exactly one CLAUDE.md while zero active sub-agents). No `.runtime/spawn-log.jsonl` infrastructure.

| id | Claim (short) | Blocking feature |
|---|---|---|
| `R-subagent-gets-its-claude-md` | A delegated sub-operator receives its OWN crystal, a CLAUDE.md generated from its sub-domain. | `create_agent` tool (Planned) + per-agent crystal rendering in gen-spec. |
| `R-crystal-tree-hierarchy` | The delegation hierarchy is a tree of CLAUDE.md crystals, one per operator, each budget-bounded. | `create_agent`/`spawn_agent`/`invoke_agent` (all Planned) — no second operator exists to form a tree. |
| `R-agent-scoped-constitution` | `gen-spec` regenerates an agent's CLAUDE.md CONSTITUTION block filtered by the agent's SCOPE tuple. | `create_agent` tool + scoped-rendering path in `internal/generator` (not implemented). |
| `R-agent-declares-purpose` | Every agent declares a non-empty PURPOSE describing what it stewards (machine-readable, alongside SCOPE). | `create_agent` tool — no non-director agent exists to carry a PURPOSE. |
| `R-agent-references-shared-docs` | Each agent CLAUDE.md contains a SHARED-DOCS block listing paths to shared thinking/tool docs. | `create_agent` tool + agent-crystal rendering path. |
| `R-sub-agent-crystal-triad` | Every sub-agent's CLAUDE.md contains three parts: scope-filtered thinking, parent reference, scope-filtered constitution. | `create_agent` tool — the triad structure is only renderable once a real sub-agent exists. |
| `R-task-spawn-log-runtime` | `spawn_agent` appends a spawn-log entry to `.runtime/spawn-log.jsonl` on every invocation. | `spawn_agent` tool (Planned) + `.runtime/` directory (does not exist). |
| `R-spawn-log-carries-isolation` | Every spawn-log entry carries isolation (worktree\|shared) and mutating (bool) fields. | `spawn_agent` tool + `.runtime/spawn-log.jsonl` infrastructure. |
| `R-delegation-is-a-file` | Every task delegation recorded as a versioned file under `delegations/` (DG-<n>.md), created/closed via a dedicated delegate tool. | A `delegate` tool (not registered at all — not even Planned in tools_data.go). A `delegations/` dir with hand-written DG-1..DG-4 exists, but no tool manages it. |

### Cluster D — Sensorium hooks (3 items)

**Blocking feature:** the host-hook sensorium — `setup_hooks` tool (`Planned`, §Operator) generates a committed project `settings.json` with portable hook commands. No hook infrastructure is committed; the `SessionStart`/`PostCompact`/`UserPromptSubmit`/`PreToolUse`/`Stop` hooks do not exist as generated, version-controlled artifacts.

| id | Claim (short) | Blocking feature |
|---|---|---|
| `R-sensorium-committed` | Universal sensorium hooks live in a committed project `settings.json` generated by a setup tool, never only in git-ignored `settings.local.json`. | `setup_hooks` tool (Planned). |
| `R-operator-prompt-loaded-at-session-start` | A `SessionStart` hook runs `hotam gen-spec` before the operator's first turn. | `setup_hooks` tool + host `SessionStart` hook (Claude Code / equivalent harness). |
| `R-post-compact-regen-from-substrate` | A `PostCompact` hook runs `hotam gen-spec` after every auto-compact. | `setup_hooks` tool + host `PostCompact` hook. |

### Cluster E — Land-log tier tracing (4 items)

**Blocking feature:** a `.runtime/land-log.jsonl` writer and reader — `gate_status` tool (`Planned`, §Closure). No `.runtime/` directory exists; `land` does not currently write any tier/verify trace. The `P6`/`PENDING_PROPOSAL` label in `what_now.go` is dead code.

| id | Claim (short) | Blocking feature |
|---|---|---|
| `R-land-tier-trace` | Every applied proposal reaching LAND verify appends its tier (T1/T2), selected test node-ids, and verify outcome to a runtime land-log. | `.runtime/land-log.jsonl` writer (does not exist) + `gate_status` tool (Planned). |
| `R-commit-boundary-checkable` | A `gate_status` command answers whether a full T2 has landed at-or-after the most recent T1-gated land. | `gate_status` tool (Planned) + the land-log it reads (does not exist). |
| `R-land-tier-trace-skips-dry-run` | A dry-run proposal never writes a land-log record. | Land-log writer (does not exist). |
| `R-land-tier-trace-best-effort` | A broken/unwritable land-log location never fails an otherwise-green apply. | Land-log writer (does not exist). |

### Cluster F — Generative audit (4 items)

**Blocking feature:** `audit_atomicity` (Planned, §Invariants), `audit_tensions` (Planned, §Loop), `mark_revisit_evaluated` (Planned, §Conflict) tools. No Go code implements these; no audit baseline exists.

| id | Claim (short) | Blocking feature |
|---|---|---|
| `R-audit-atomicity-tool` | Atomicity audit (compound requirements + compound check_* + R↔enforcer bijection + orphan analysis) by a deterministic tool, not hand invocation. | `audit_atomicity` tool (Planned). |
| `R-atomicity-ratchet-no-growth` | The set of COMPOUND-flagged claims/invariants never grows beyond the frozen baseline. | `audit_atomicity` tool — the ratchet needs the audit baseline the tool produces. |
| `R-tension-audit-staleness-visible` | `what-now` surfaces a CLI-only action on the 'generative-audit' meter when the tension audit has never run or the graph has grown past the stale-delta. | `audit_tensions` tool (Planned) + `what-now` staleness signal. |
| `R-revisit-markers-evaluated` | `what-now` surfaces a CLI-only action for each DECIDED conflict whose revisit_marker was never evaluated or is stale. | `mark_revisit_evaluated` tool (Planned) + `what-now` signal. |

### Cluster G — Enforcement-perimeter guards (2 items)

**Blocking feature:** a dedicated baseline-update tool + host-hook guards (PreToolUse Edit/Write denial). Neither tool is registered (not even Planned in tools_data.go); no content-hash pin exists; no ratchet-test baseline file is guarded.

| id | Claim (short) | Blocking feature |
|---|---|---|
| `R-enforcement-perimeter-baselines-guarded` | A host-hook guard denies direct Edit/Write to enforcement-perimeter baseline files (ratchet-test baselines, active-domain pin), with sanctioned updates via a dedicated tool. | Baseline-update tool (unregistered) + host PreToolUse hooks (not committed). |
| `R-enforcement-perimeter-visible` | A content-hash pin covers the enforcement-perimeter code files, failing RED on any change until the baseline is consciously updated. | Content-hash pin mechanism + baseline-update tool (both unregistered). The C2 brief item #8 noted this overlaps the `R-speculative-aspects-frozen` freeze guard — building one shared mechanism is the efficient order. |

### Cluster H — Test-suite tiering (2 items)

**Blocking feature:** a test-suite partitioning mechanism (build tags or equivalent) that separates 'framework' tests (exercising `internal/` mechanics) from 'domain' tests (asserting concrete self-domain content). No such partition exists; `go test ./...` runs everything flat.

| id | Claim (short) | Blocking feature |
|---|---|---|
| `R-framework-suite-tiered` | The test suite partitions every test into exactly one of two tiers — 'framework' or 'domain' — so the framework tier is separately selectable. | Test-tiering infrastructure (build tags / package layout). |
| `R-framework-suite-domain-independent` | The framework test subset passes green under any active domain, or none. | Test-tiering infrastructure — independence is only assertable once the partition exists. |

### Cluster I — Axis gatekeeping (1 item)

**Blocking feature:** `create_axis` tool (`Planned`, §Axis). No Go code implements axis creation with a confront-style similarity check.

| id | Claim (short) | Blocking feature |
|---|---|---|
| `R-axis-gatekeeper-policy` | Axis-duplicate gatekeeping is a mandatory part of the axis-creation path (confront-style similarity check at creation time), refusing near-duplicates unless `--force-new`. | `create_axis` tool (Planned) — the gatekeeper check is part of the creation path. |

### Cluster J — Performance guard (1 item)

**Blocking feature:** per-machine test-run timing infrastructure (off-git baseline storage, self-calibration). No timing harness exists.

| id | Claim (short) | Blocking feature |
|---|---|---|
| `R-run-speed-guarded` | Test-run duration does not silently degrade: a self-calibrating guard (baseline = mean of first 5 local runs × 1.2, per-machine, off-git) fails when a run exceeds baseline. | Timing harness + per-machine baseline store (neither exists). |

---

## Summary

| Cluster | Items | Blocking feature (all Planned/absent) |
|---|---|---|
| A — Ticket engine | 3 | `ticket_*` tool family (6 tools, Planned) |
| B — Attention core | 4 | `internal/attention` package (absent) + `attention`/`attention_hook` (Planned) |
| C — Sub-agent hierarchy | 9 | `create_agent`/`spawn_agent`/`invoke_agent` (Planned) + per-agent crystal rendering |
| D — Sensorium hooks | 3 | `setup_hooks` tool (Planned) + committed host hooks |
| E — Land-log tracing | 4 | `.runtime/land-log.jsonl` (absent) + `gate_status` tool (Planned) |
| F — Generative audit | 4 | `audit_atomicity`/`audit_tensions`/`mark_revisit_evaluated` (Planned) |
| G — Perimeter guards | 2 | Baseline-update tool (unregistered) + content-hash pin |
| H — Test-suite tiering | 2 | Test-partitioning infrastructure (absent) |
| I — Axis gatekeeping | 1 | `create_axis` tool (Planned) |
| J — Performance guard | 1 | Timing harness + per-machine baseline (absent) |
| **Total C1** | **33** | |

**Why this state is correct and will persist:** every one of these 33 requirements describes behavior of a feature that is registered as `Planned` (not implemented) in `internal/methodology/tools_data.go` or depends on a package/directory that does not exist on disk. They carry `enforceability: ENFORCEABLE` (correct — they ARE mechanically enforceable once the feature is built) and `enforcement: PROSE` (correct — no honest test exists today). Reclassifying to `INHERENTLY_PROSE` would be factually wrong; writing mock-enforcement tests would be theatre. The freeze on building these features is itself a steward-approved decision (`R-speculative-aspects-frozen`), not an oversight.

---

## Out of scope (future candidate — NOT this wave)

The root cause of the re-flagging signal is that `docs/gen/UNENFORCED.md`'s burn-down metric is one-dimensional: `closeable debt: N` does not separate *closeable-now* (genuinely enforceable today) from *feature-blocked* (the feature doesn't exist yet). Splitting the metric to show `closeable-now: N | feature-blocked: M` would make the honest state self-documenting and stop the re-flagging cycle.

This is a real, separate, larger change touching:
- **Ontology:** a new optional `blocked_on` field on `Requirement` (naming the Planned tool or absent package).
- **Generator:** `docs/gen/UNENFORCED.md` rendering two sub-tables under "Closeable debt" instead of one flat table.
- **Diagnose:** `what-now`'s `enforcement-gradient` reflection counting only *closeable-now* items, not feature-blocked ones.

It requires its own steward-approved proposal (a new `Requirement` field is an ontology change, not a docs-only edit) and is a candidate for a genuinely future wave, not this one. Flagging it here so the next reviewer who re-encounters the burn-down number knows the fix is identified and deliberately deferred, not missed.

### Update (task #123, 2026-07-13) — IMPLEMENTED

The metric split described above as future work has now been implemented in task #123 (this same review-response wave, steward-approved). The `Requirement` struct gained an optional `blocked_on` field (`internal/ontology/requirement.go`, serialized as `json:"blocked_on,omitempty"`, schema_version bumped 1→2 with a lossless additive v1→v2 migration). The 33 C1 items below were backfilled with a one-line `blocked_on` naming their blocking feature via `proposals/wave14-burndown-split-backfill/`. Consequently `docs/gen/UNENFORCED.md` now renders two tables — "Closeable debt — closeable now" and "Closeable debt — feature-blocked (honest roadmap, not neglected)" — and its burn-down line reads `closeable-now 4; feature-blocked 33` (the 4 are the C2-residual items that honestly remain closeable-now debt by steward choice). `hotam what-now`'s `enforcement-gradient` P0 reflection now counts ONLY closeable-now items (4 ≤ 5, so the P0 signal is silent), while the 33 feature-blocked items surface as a separate lower-priority Advisory (`reflect_feature_blocked_debt`, P7) pointing back at this document — so the re-flagging cycle this section predicted is now structurally discharged.

---

*No graph.json, no generated docs, and no Go source file was touched in the course of producing this document. The C1 item list was derived freshly from `docs/gen/UNENFORCED.md`'s current "Closeable debt" section (37 items) minus the 4 C2-residual items (steward-decided claim-amendments from task #113 that honestly remain debt) = 33. All blocking-feature claims cross-checked against `internal/methodology/tools_data.go` (Planned/Implemented status) and filesystem state (`internal/attention` absent, `.runtime/` absent, no `ticket_*`/`gate_status`/`create_agent` Go implementations).*
