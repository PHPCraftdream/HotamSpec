# Checkpoint — 2026-06-30 [tensio-substrate-built]

## Session summary

We built **Hotam-Spec** from scratch in `D:\dev\HotamSpec` — an executable methodology
for the lifecycle of contradictory business requirements, modeled as a tension
graph; the inverse of the dev-coin (`D:\dev\dev-coin`) blockchain spec from
which it borrows the docs-as-code machinery. Core ontology is frozen dataclasses
(`Requirement`, `Conflict`, `Assumption`, `Axis`, `Stakeholder`) under
`spec/src/hotam_spec/`; the centerpieces are the `what_now` harness ("agent is never
lost") and the anti-drift `gen_spec` meta-test ("drift is structurally
impossible"). The hard-won refactor split the framework (`src/hotam_spec/`,
content-free) from user content (`spec/content/graph.py`); the worked example
was demoted to a test fixture (`spec/tests/fixtures/seed.py`). Two commits
shipped: the framework + the meta-domain (Hotam-Spec modeling itself in
`spec/content/graph.py`).

Mid-session the user added a series of architectural insights that we **crystallized
as DRAFT requirements** in the meta-domain rather than building: (1) typed
stateful aspects (Lifecycle / Entity / Process / Task); (2) the full-system
ontology — operators with capabilities/states, goals as target-states, processes
as transformers (4 mutation kinds: state/req/refine/entity); (3) the
**context-bounded recursive delegation** scaling law (`size(domain) ≤ budget` →
spawn sub-operator with conclusions-only interface — Hotam-Spec's answer to M8);
(4) the **crystallization super-rule** (knowledge → code; offload, don't carry —
substrate is enforced/regenerable/addressable and doesn't count against
context); (5) the **anchoring super-rule** (everything addressable, speak by
reference) — its pair; (6) **dependency graph for parallelism**
(`R-dependency-graph-parallelism` + `C-d4f3eadf`); (7) **CLAUDE.md as the
director-operator's crystal** with a Director's Map indexing the whole graph,
sub-operators carrying their own CLAUDE.md.

Around 34% context the user named the dynamic explicitly: I AM operator #1 of
this system and must apply it to myself. From that point I crystallized my
working knowledge into the substrate via delegated sub-agents (textbook
crystallize-before-split), kept conclusions, returned anchored summaries.
Current graph state: **54 requirements** (13 SETTLED · 25 DRAFT · 13 OPEN · 3
REJECTED), 9 axes, 6 conflicts (5 DECIDED, 1 live DETECTED — `C-8600b1b8` on
`core-vs-aspect`, resolver `domain-user`), 9 assumptions, 4 stakeholders;
M-decisions M1–M31 in CLAUDE.md (currently DUPLICATED with the graph's OPEN
reqs — flagged as U5, Hotam-Spec's own anti-drift violation).

Final move before checkpoint: I produced a **ratification sheet** resolving the
blocking open decisions via dev-coin precedent — M26 (enforcement = FIELD AND
generated report, like `Param.status`+`HOLES.md`), M17 (NODE_COUNT now, token
deferred behind seam), M20 (Operator = new TYPE referencing Stakeholder, keeps
acting/accountability separable), build-order via dependency topology
(**Lifecycle keystone first**, then M26 field, then Operator trio, then
Process). Two batches recommended: **Batch A** mechanical (U1–U4 doc drift, A1
content-validity test, A2 smoke, A3 content-free instance-scan, A4 typed-anchor
check, U5 cross-anchor interim); **Batch B** sequential framework-touching
(Lifecycle → M26 field → U5 generated DECISIONS.md → Operator trio → Process
aspect). Waiting on user ratification of the sheet before executing.

Files studied this session (verified, not guessed): all of
`D:\dev\HotamSpec\spec\src\hotam_spec\*.py`, `spec/tools/{what_now,gen_spec}.py`,
`spec/tests/*.py`, `spec/tests/fixtures/seed.py`, `spec/content/graph.py`,
`CLAUDE.md`, `README.md`, `docs/methodology/README.md`,
`docs/development/ROADMAP.md`; and from dev-coin: `spec/src/hotam/params.py`,
`spec/tools/gen_spec.py` (the source-of-truth + generated-mirror precedent),
`CLAUDE.md`. Three max-effort design dossiers were delegated to oxx
(lifecycle/process/entity/task; full-system + context-bounded delegation;
crystallization + anchoring) — their outputs were crystallized into
`spec/content/graph.py` and not preserved separately. No /loop or /babysit
timers active.

Live state verified at checkpoint: `uv run pytest -q` → **81 passed**;
`all_violations(load_content_graph()) == []`; `gen_spec` deterministic
(md5-identical across runs); `what_now` shows 232 actions (P3=1, P4=13, P5=~218
heuristic flood — empirically validates M6 / `R-uncrystallizable-automated`
about the shared-assumption heuristic over-flagging).

## Active goal

None — no `/goal` Stop hook in force.

## TaskList

### pending
- (none — all decision tasks completed at checkpoint time)

### recently completed
- #26 Resolve the blocking open decisions via the methodology's own machinery (oxx)
- #25 Present anchored backlog + resolver decision to user
- #24 Audit substrate → anchored UPDATE/ANCHOR/DEFER backlog (oxx)
- #23 Crystallize dependency-graph + director-crystal trio; build CLAUDE.md Director's Map (delegated)
- #22 Director verification: pytest green + what_now sane after crystallization
- #21 Crystallize the 3 design dossiers into meta-domain + CLAUDE.md (delegated)
- #20 Synthesize super-rule design for user
- #19 Delegate crystallization super-rule design (oxx continuation)
- #18 Synthesize ontology design + scope decision for user (folded into next dossier)
- #17 Delegate full-system ontology + context-bounded delegation design (folded into M25+)

Older tasks (#4–#16) were the framework-build + content-free refactor work; all completed.

## Decisions

- **Storage = Python code, NOT RDF/SHACL.** Frozen dataclasses + tuple-of-id edges + plain-function traversal + `check_*` invariants + generator. Rejected RDF as the heavy parallel substrate dev-coin already disproves the need for. (Recorded as `R-rdf-store` REJECTED.)
- **Framework is CONTENT-FREE.** Demo seed lives in `spec/tests/fixtures/seed.py`, real domain in `spec/content/graph.py`; `src/hotam_spec/` ships zero business data. (Recorded as `R-content-free-framework` SETTLED; the leaked seed in src as `R-seed-in-src` REJECTED.)
- **Hotam-Spec models itself in `spec/content/graph.py`** (the meta-domain). The methodology eats its own dog food as the strongest stress test — proving `R-two-altitude-ontology` (operator : methodology :: actor : business).
- **M26 (enforcement representation) — BOTH field AND generated report**, per dev-coin's `Param.status`+`HOLES.md` precedent. Pre-decided before resolver ratification, awaiting greenlight.
- **First heavy increment = Lifecycle keystone**, not Operator trio. The dependency-graph principle (`R-dependency-graph-parallelism`) puts Lifecycle upstream of Operator and aspects; trio has two now-resolved blockers (M17 NODE_COUNT, M20 new type). Awaiting resolver ratification.

## Open questions

- **Ratify the resolution sheet?** Seven items in the table (mechanical batch, U5 canonicalization, M26 enforcement = field+report, M17 NODE_COUNT, M20 new type, build-order = Lifecycle-first, commit timing). Single greenlight unblocks Batch A immediately.
- **Run Batch A now?** Six mechanical items (delegate-and-go), each enforces a SETTLED principle of the framework against itself.
- **Commit cadence?** Recommendation: batch-by-batch; operator proposes, resolver runs `git commit`. Working tree currently dirty (5 files modified — the two crystallization waves not yet committed).

## Repo state

```
 M CLAUDE.md
 M docs/gen/OPEN.md
 M docs/gen/REQUIREMENTS.md
 M docs/gen/TENSIONS.md
 M spec/content/graph.py
```

```
3b11447 feat(content): meta-domain — Hotam-Spec modeling its own design
6465c93 feat: Hotam-Spec — content-free requirements-as-tension-graph methodology framework
```
