# Checkpoint — agent-vs-hand

**Topic checkpoint, not session state.** What was conflated, what the right
distinction is, what's recorded in the substrate.

## The conflation (honest)

All session I called sh-subagent invocations "delegations" or "framework-
agents". They are not. They are HANDS. The user corrected me three times
across the session.

A **hand** is:
- An sh / Agent-tool invocation.
- Ephemeral — returns conclusions, doesn't persist.
- No own state, no own crystal, no own tools.
- The operator's extended capability for a single named action.

An **agent** is:
- A DIRECTORY at `spec/agents/<name>/`.
- Persistent across sessions (it's in git).
- Has its own `CLAUDE.md` (its crystal — its operator-prompt).
- Has its own `tools/` subdirectory (its private tool set).
- Uses framework code (`import tensio.*`) as shared infrastructure.
- Has authority over a SETTLED sub-domain.

Today I spawned 30+ hands. Zero agents.

## What's recorded in the substrate (commit 3465f83)

Five DRAFT requirements capture the architecture (build-triggers
explicit):

- **R-agent-is-a-directory** (DRAFT) — domain-agent = `spec/agents/<name>/`.
- **R-agent-has-own-crystal** (DRAFT) — its own `CLAUDE.md`.
- **R-agent-has-own-tools-dir** (DRAFT) — its private `tools/`.
- **R-agent-imports-framework** (DRAFT) — uses `tensio.*` as shared
  infrastructure, owns nothing in the framework body.

Plus the distinction itself:

- **R-task-spawn-is-ephemeral** (DRAFT) — hand: returns conclusions, no
  persistence between invocations.
- **R-domain-delegation-persists** (DRAFT) — agent: persists as
  directory + substrate node.
- **R-task-spawn-log-runtime** (DRAFT) — hand-invocations append to
  `spec/.runtime/spawn-log.jsonl` (runtime ephemera, not committed
  substrate).

And the substrate-side recording:

- **R-domain-delegation-as-node** (DRAFT) — `Delegation` substrate node
  type with fields `parent_op`, `child_op`, `scope`, `border`,
  `returns_contract`, `crystal_path`.

## The trigger I noticed but didn't have in substrate before

The user added a SECOND trigger for tree-of-crystals delegation (the first
is the φ-cap by size):

- **R-tree-of-crystals-cognitive-trigger** (DRAFT) — delegate when a
  sub-domain's detail granularity exceeds the director's altitude, even
  if size is under the cap.

This is the trigger I was already past today. The framework body (AST
locators, edge-cases of each `check_*`, multi-clause docstrings) is
detail-deep, and I was holding it all in working context. A framework-
agent would have its own crystal scoped to `spec/src/tensio/` + its own
tools, and I would see only its surfaced conclusions.

## The tool-over-hand discipline (separate but related)

The user named: "не делай руками, сделай инструмент и вызови его". This
is bigger than agent-vs-hand. It says: even for hand-actions, prefer
creating a reusable tool over performing the action one-off.

Recorded:
- **R-prefer-tool-over-hand** (SETTLED/STRUCTURAL, commit 3465f83) —
  operator shall prefer creating a tool; one-off acts only at bootstrap
  or genuinely unique events.

Today I violated this often. Example: the framework-agent atomization
audit should have been `spec/tools/audit_atomicity.py` (deterministic,
re-runnable) rather than an sh-invocation. Next session: build that tool.

## Tools registry + sharing

To support agents-with-own-tools, the tool registry must be addressable:

- **R-tools-registry-generated** (DRAFT) — list of tools generated into
  CLAUDE.md from scanning `spec/tools/*.py` (shared) and
  `spec/agents/<name>/tools/*.py` (private).
- **R-shared-tools-in-spec-tools** (SETTLED/STRUCTURAL) — common tools at
  `spec/tools/`.
- **R-private-tools-in-agent-folder** (DRAFT) — agent-specific tools at
  `spec/agents/<name>/tools/`.

## What "first real agent" would look like

A `framework-agent` directory:
```
spec/agents/framework-agent/
├── CLAUDE.md                  # its operator-prompt; scoped to spec/src/tensio/
├── tools/
│   ├── audit_atomicity.py     # private — scans claims/methods for compoundness
│   └── audit_bijection.py     # private — checks R ↔ check_* mapping
└── README.md                  # human-readable rationale
```

When invoked (via a meta-tool like `tools/invoke_agent.py framework-agent
<task>`), it:
1. Loads `spec/agents/framework-agent/CLAUDE.md` as its prompt (NOT the
   root CLAUDE.md).
2. Sees only its private tools + the shared `spec/tools/`.
3. Operates with a bounded `DomainScope` (cannot edit `spec/content/`,
   cannot decide methodology questions).
4. Returns `Proposed*` JSON to the director, conclusions-only.

None of this is built. All in DRAFT. BUILD-TRIGGER: "first real second
operator is instantiated."

## The meta-tools we need to enable agents

To avoid hand-building agent directories:

- `tools/create_agent.py` (NOT YET) — meta-tool that scaffolds a new
  agent directory with CLAUDE.md template + tools/ skeleton.
- `tools/create_tool.py` (NOT YET) — meta-tool that creates a new tool
  with docstring + test + registry entry.
- `tools/invoke_agent.py` (NOT YET) — meta-tool that calls an agent by
  loading its CLAUDE.md as prompt and dispatching the task.

Bootstrap exception applies: someone writes the first meta-tool by
hand. After that, all tool creation goes through `create_tool.py`.

## The concurrent-edit hazard

Three parallel o46l agents wrote to `spec/content/graph.py` this session.
Final state is consistent (256 tests pass), but one agent reported losing
edits to another's write. The right pattern is `apply_proposal` queue:

- Each agent returns a `Proposed*` JSON instead of editing files directly.
- A serializer applies them one at a time.
- Concurrency hazard goes away.

NOT BUILT — apply_proposal exists for ProposedConflictTransition only;
batch-add for ProposedRequirement (which is what we'd need) is missing.
This is implicit in R-active-loop-protocol but the apply-tool covers
only one type today.

## See also

- `docs/checkpoints/phase11-atomization-wave.md` (session state).
- `docs/checkpoints/sensor-substrate-inversion.md` (the architectural
  ground for why agents-as-directories matter).
- `docs/checkpoints/audit-backlog-residue.md` (concrete work that
  would benefit from a real framework-agent).
