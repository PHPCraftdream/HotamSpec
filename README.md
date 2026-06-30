# Tensio — requirements as a tension graph

> **A methodology whose centerpiece is: an AI agent dropped into the repo in any
> state, at any moment, can deterministically derive the next correct action.**
> "The agent is never lost" — the same way the reference project HotamChain makes
> spec *drift* structurally impossible, Tensio makes *being lost* structurally
> impossible.

Tensio manages the lifecycle of **business requirements** that are many,
constantly changing, and **mutually contradictory**. It is the deliberate
INVERSE of a consistency specification. The companion project HotamChain (a
docs-as-code blockchain spec) proves ONE non-contradictory canon and forbids
drift, closing every conflict forever. Tensio reuses that exact machinery
inverted: not to eliminate contradictions, but to guarantee **we never fail to
see them**, and to keep them visible over time.

## The philosophy

**Requirements are not a truth; they are a tension graph.** A requirement
changes, contradicts its siblings, and rests on assumptions that can die. A
contradiction is never silently fixed — it is a first-class, owned,
history-bearing object that transitions through a lifecycle under a human
steward.

### Conflict is a connector NODE, not an edge

A naive model makes a conflict an edge `conflicts_with` between two requirements.
That edge holds nothing — remove it and the requirements fall back into
isolation. Tensio makes a **Conflict a first-class NODE** (a mediator) through
which two otherwise-unconnectable requirements first come to lie in one
structure:

```
R-87 (latency < 200ms)  ──►  C  ◄──  R-203 (full synchronous compliance check)
                             │
                       axis:    latency-vs-completeness
                       context: approving a payment at checkout
                       shared assumption: A-sync-budget
```

The node `C` carries knowledge belonging to **neither** requirement: the
**tension axis** (the dimension they diverge along, born only from their
meeting), the **shared context** (the scenario where they actually collide), and
the **shared assumption** they interpret differently (often the real root).
Therefore *"to surface a contradiction"* technically means *"to materialize the
missing connector node."* The detector's job is redefined: not "find conflicts"
but **"find requirement pairs that should have a connector node but don't"** —
latent connectors. That is stronger than checking violated invariants, because it
catches the *invisible*, not the already-recorded.

Three consequences fall out, and the node-graph (not an edge-list) makes each
visible:

- **Conflicts cluster.** Many conflict nodes on one axis = one unresolved
  *architectural* choice, not ten local disputes.
- **Conflicts spawn requirements.** Resolving a conflict is often not "pick A or
  B" but the birth of a new requirement R-300 that dissolves the tension — so the
  conflict is the *parent* of a new requirement, with recorded lineage.
- **Conflicts inherit drift.** If a conflict rests on a shared assumption and that
  assumption dies, the whole cluster under it revives at once.

### The three invisibilities it surfaces

1. **Direct contradictions** — two requirements that cannot both hold.
2. **Hidden dependencies** — A silently relies on an assumption B negates;
   a contradiction *through a chain* (needs a graph, not a list).
3. **Context drift** — a requirement meaningful under assumption X, X long false,
   nobody revisited it; a contradiction *with time* (catchable only because
   assumptions carry their own lifecycle).

## The closed loop (why the agent is never lost)

The store is the Python code; the human layer is generated; the harness reads the
whole graph and tells you what to do next:

```
State (graph + generated docs + test status)
  → Diagnosis  (spec/tools/what_now.py)
  → Next-action (typed, prioritized)
  → Action     (edit spec/src/tensio)
  → regenerate (spec/tools/gen_spec.py)
  → State
```

`what_now` emits one priority-ordered list: failing structural invariants (P1),
dead-assumption fallout (P2), conflicts stalled without a steward (P3), open
questions (P4), and heuristic latent-connector suspects for AI review (P5). Run
it from any state and the top line is the next correct action.

## How it is checked (the inverted spec-stack)

Same tools as HotamChain, purpose mirrored from "prove no conflict" to "detect &
hold conflict". **Weight of apparatus ∝ cost of an unnoticed conflict.**

| Level | Mechanism | Guarantees | Status |
|---|---|---|---|
| Frozen dataclass form | ruff | objects well-formed | CORE |
| Structural graph invariants | `tensio.invariants.check_*` | every conflict has axis+context+steward; no dangling refs; OPEN states its question; a decision justifies itself; steward ≠ member owner | CORE |
| Visibility of the open | `OPEN(question)` → generated `OPEN.md` | open holes & unresolved conflicts cannot hide | CORE |
| Latent-conflict property detector | Hypothesis (hunts missing connectors) | catches the invisible | DEFERRED |
| Formal conflict detector | Z3 (model where two requirements jointly break) | machine proof a pair collides | DEFERRED |
| Behavioral/temporal | Quint/Apalache | workflow contradiction | DEFERRED |
| Stateful PBT of evolution | Hypothesis stateful | dead assumption revives dependents | DEFERRED |
| Mutation testing of detectors | cosmic-ray | detectors are not phantom | DEFERRED |
| Human layer + anti-drift | `gen_spec.py` + meta-test | generated text cannot drift | CORE |

`cd spec && uv run pytest -q` → **81 passed**.

## Framework vs content (content-free by design)

Tensio is a **blank kit**, not a project with example business data. The package
`spec/src/tensio/` ships ZERO business content: no example requirements, no
example axes. A real domain populates a single file:

```
spec/content/graph.py   →   def build_graph() -> TensionGraph: ...
```

The tools (`what_now`, `gen_spec`) discover that file automatically; if it is
absent (the legitimate ship state) the harness prints a calm "no content yet"
banner and the generator emits the same notice into the documents. The worked
example lives **outside** the framework under `spec/tests/fixtures/seed.py` and
is loaded only via the explicit `--demo` flag.

```bash
uv run python tools/what_now.py            # diagnose YOUR domain (spec/content/)
uv run python tools/what_now.py --demo     # explore the fixture demo graph
uv run python tools/gen_spec.py            # regen docs/gen/ from YOUR domain
uv run python tools/gen_spec.py --demo     # write docs/demo/ from the fixture
```

## Repository map

| Layer | Where | What |
|---|---|---|
| **Framework (content-free)** | [`spec/src/tensio/`](spec/src/tensio/) | the ontology + traversal + content loader + structural invariants |
| **Your domain's graph** | [`spec/content/graph.py`](spec/content/README.md) | `build_graph() -> TensionGraph` — empty by default |
| Worked example (test fixture) | [`spec/tests/fixtures/seed.py`](spec/tests/fixtures/seed.py) | the demo graph used by tests and `--demo` |
| The harness | [`spec/tools/what_now.py`](spec/tools/what_now.py) | derives the next action from any state |
| Generator | [`spec/tools/gen_spec.py`](spec/tools/gen_spec.py) | deterministic human layer |
| Generated human layer | [`docs/gen/`](docs/gen/) | `REQUIREMENTS.md`, `TENSIONS.md`, `OPEN.md` — **do not edit by hand** |
| Methodology | [`docs/methodology/README.md`](docs/methodology/README.md) | philosophy + the loop |
| Roadmap | [`docs/development/ROADMAP.md`](docs/development/ROADMAP.md) | deferred layers + trust-anchoring ritual |
| Operating contract | [`CLAUDE.md`](CLAUDE.md) | how to work and never get lost |

## Quick start

```bash
cd spec
uv run pytest -q                      # 81 passed
uv run python tools/what_now.py       # "no content yet" until you populate
uv run python tools/what_now.py --demo  # see the harness on the worked example
```

Requires [uv](https://github.com/astral-sh/uv) (Python 3.12+). Dual-licensed
**MIT OR Apache-2.0**.

## Status

CORE built: the ontology, structural invariants, the content loader, the
generator + anti-drift meta-test, and the `what_now` harness — all green
against an empty content slot AND against the demo fixture's live working
surface (an open requirement, a stalled conflict, a dead assumption with
dependents, a latent suspect). Formal layers (Hypothesis, Z3, Quint, cosmic-ray)
are DEFERRED for the critical core. Methodology decisions M1–M9 are flagged
OPEN in [`CLAUDE.md`](CLAUDE.md) awaiting confirmation.
