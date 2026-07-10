# Hotam-Spec

[![License: MIT OR Apache-2.0](https://img.shields.io/badge/license-MIT%20OR%20Apache--2.0-blue.svg)](LICENSE)
[![Python 3.12+](https://img.shields.io/badge/python-3.12+-green.svg)](https://www.python.org)

> Executable methodology for contradictory business requirements, modeled as a tension graph.

## What is Hotam-Spec?

Hotam-Spec is a framework for managing the lifecycle of **contradictory business requirements**. Instead of pretending conflicts don't exist, it models them as first-class **tension nodes** in a graph -- visible, stewarded, and tracked through resolution.

### Glossary (six terms to get started)

- **Stakeholder** -- an accountable party (a person, team, or role) who can own requirements or steward a conflict.
- **Requirement** -- a business claim with a lifecycle (draft -> settled -> rejected).
- **Axis** -- a named dimension along which requirements can pull in opposite directions (e.g. "speed vs rigor").
- **Conflict** -- two or more requirements that contradict each other on an axis; tracked explicitly instead of silently overridden.
- **Steward != owner** -- the person who decides how a conflict resolves must NOT be the owner of any requirement on either side of it. This keeps conflict resolution from being one side simply overruling the other.
- **Assumption** -- a belief a requirement or conflict rests on, which can later turn out to be wrong ("drift").

That's everything a human team needs to start. Everything past this point --
operators, crystals, context budgets, spawn logs -- is internal machinery for
running an **AI agent as an operator** of the graph; skip it if you're just
using Hotam-Spec as a CLI discipline for your team.

### Key ideas

- **Conflict is a connector node**, not an edge -- it carries axis, context, steward, and lifecycle
- **Requirements contradict -- and that's expected** -- the methodology surfaces them, never hides them
- **A human always decides** -- every conflict resolution stays with a named steward; an AI operator, if you use one, may only present options
- **Docs-as-code with structural anti-drift** -- generated docs must match the model byte-for-byte

### The tension graph

```
Requirement A ──┐
                ├── Conflict node (axis, context, steward)
Requirement B ──┘
```

Two requirements that contradict on an axis (e.g. "latency vs completeness") are connected by a Conflict node. The conflict has a lifecycle (DETECTED -> ACKNOWLEDGED -> DECIDED), a steward who is NOT the owner of either side, and a rationale recorded for anti-relitigation.

## Quick start

```bash
# Clone
git clone https://github.com/PHPCraftdream/HotamSpec.git
cd HotamSpec

# Install dependencies
cd spec && uv sync

# Run tests
uv run pytest -q

# See what the harness recommends
uv run python tools/what_now.py

# Generate docs from the model
uv run python tools/gen_spec.py

# Audit atomicity
uv run python tools/audit_atomicity.py
```

## Adopt: your domain in 15 minutes

Hotam-Spec ships modeling *itself* (the `hotam-spec-self` domain). To model **your
own** contradictory business requirements, you seat a fresh domain beside it — no
framework code changes. Every graph write goes through `apply_proposal.py`; the
graph is never hand-edited (`R-no-hand-edit-graph`, enforced by a committed
PreToolUse guard).

This section covers the self-hosting path (working inside a clone of this
repo, running tools from `spec/`). **If you're installing Hotam-Spec into your
own separate repo** (via `pip`, using the `hotam-*` console scripts), use
[docs/QUICKSTART-CONSUMER.md](docs/QUICKSTART-CONSUMER.md) instead — same
ideas, different install path.

### Required for any team (no AI agent needed)

Everything below works from a plain shell — no AI operator involved. This is
the whole discipline: scaffold a domain, then add stakeholders/requirements/
conflicts through the CLI or `apply_proposal.py`.

```bash
cd spec && uv sync                                   # 1. install

# 2. scaffold YOUR domain and activate it (pins domains/.active-domain and
#    regenerates the ROOT CLAUDE.md as your domain's operator crystal --
#    the CLAUDE.md file itself only matters if you add an AI operator later).
uv run python tools/create_domain.py my-shop \
    --description "My shop's contradictory requirements" \
    --goals "hold the first tension;decide honestly" \
    --director-purpose "steward my-shop" \
    --activate
```

Now populate the tension graph — each step is a `Proposed*` JSON fed to
`apply_proposal.py`:

```bash
# 3. at least TWO stakeholders (a conflict's steward may not own any member).
echo '{"kind":"Stakeholder","id":"alice","name":"Alice","domain":"product"}'   > sh1.json
echo '{"kind":"Stakeholder","id":"bob","name":"Bob","domain":"engineering"}'   > sh2.json
echo '{"kind":"Stakeholder","id":"carol","name":"Carol","domain":"governance"}'> sh3.json
uv run python tools/apply_proposal.py sh1.json
uv run python tools/apply_proposal.py sh2.json
uv run python tools/apply_proposal.py sh3.json   # a NEUTRAL steward for the conflict

# 4. an axis — the shared dimension your first tension lives on.
uv run python tools/create_axis.py speed-vs-rigor --description "ship fast vs verify thoroughly"

# 5. a few atomic requirements (enforcement is PROSE | STRUCTURAL | ENFORCED).
echo '{"kind":"Requirement","id":"R-ship-fast","claim":"Ship within one week.","owner":"alice","status":"SETTLED","enforcement":"PROSE"}'      > r1.json
echo '{"kind":"Requirement","id":"R-verify-all","claim":"Verify every change before release.","owner":"bob","status":"SETTLED","enforcement":"PROSE"}' > r2.json
uv run python tools/apply_proposal.py r1.json
uv run python tools/apply_proposal.py r2.json

# 6. your FIRST conflict — the tension between them, stewarded by the neutral party.
echo '{"kind":"Conflict","axis":"speed-vs-rigor","context":"first release cadence","members":["R-ship-fast","R-verify-all"],"steward":"carol"}' > c1.json
uv run python tools/apply_proposal.py c1.json

# 7. regenerate the crystal and read your pulse.
uv run python tools/gen_spec.py
uv run python tools/what_now.py     # your DETECTED conflict now awaits carol's ACKNOWLEDGE
```

`what_now.py` is your live pulse — the next correct action, derived from the
graph. For the full JSON reference of every `Proposed*` kind, see
[docs/PROPOSAL-REFERENCE.md](docs/PROPOSAL-REFERENCE.md).

### Optional: only if you run an AI agent as an operator

The following steps are **not needed** for a human team using the CLI
directly. They matter only if you want an AI agent (e.g. Claude Code) to act
as an **operator** of the graph — presenting proposals for the steward to
approve, never deciding on its own (`R-ai-presents-not-decides`).

```bash
# install the committed sensorium (SessionStart/PostCompact regen,
# UserPromptSubmit cipher, PreToolUse graph-guard, Stop context+boot-cite).
# Portable via $CLAUDE_PROJECT_DIR — writes <repo>/.claude/settings.json.
uv run python tools/setup_hooks.py            # dry-run: prints the plan
uv run python tools/setup_hooks.py --apply    # writes the committed settings.json
```

With this installed, the repository-root `CLAUDE.md` (regenerated from
whichever domain is pinned in `domains/.active-domain`) becomes the AI
operator's **crystal** -- the substrate that generates the agent's own prompt,
rather than the agent improvising one. The per-domain
`domains/my-shop/CLAUDE.md` is only a pointer to it. None of this -- crystals,
context budgets, spawn logs -- is required to use Hotam-Spec as a plain CLI
discipline.

## Project structure

```
spec/                          # Framework (shared, content-free)
├── src/hotam_spec/            # Ontology: Requirement, Conflict, Lifecycle, Entity, ...
├── tools/                     # Shared meta-tools (gen_spec, what_now, apply_proposal, ...)
├── tests/                     # pytest suite (run `pytest -q` under spec/ for the current count)
└── docs/                      # Generated thinking + tool docs (DRY source)

domains/                       # Per-business domain content
└── hotam-spec-self/           # The framework modeling itself (meta-domain)
    ├── graph.py               # The tension graph (build_graph() -> TensionGraph)
    ├── manifest.py            # Domain identity (ID, description, goals, director)
    ├── agents/                # Sub-operators with scoped CLAUDE.md crystals
    └── docs/gen/              # Generated docs for this domain
```

## Framework concepts

| Concept | What it is |
|---------|-----------|
| **Requirement** | A business claim with lifecycle (DRAFT -> SETTLED -> REJECTED) |
| **Conflict** | A first-class connector node between contradicting requirements |
| **Assumption** | A claim with its own lifecycle -- the root of context drift |
| **EntityType** | A domain-declared business concept with lifecycle and fields |
| **Process** | Ordered steps driving entity state transitions |
| **Operator** | An acting facet with context budget; delegates via agent tree |
| **Goal** | A target-state predicate the operator pursues |

## Tools

| Tool | Purpose |
|------|---------|
| `what_now.py` | Derives the prioritized next correct action from any graph state |
| `gen_spec.py` | Regenerates all docs from the executable model |
| `apply_proposal.py` | Mechanically applies steward-approved changes to the graph |
| `audit_atomicity.py` | Surfaces compound claims and compound check_* functions |
| `create_domain.py` | Scaffolds a new business domain |
| `create_agent.py` | Scaffolds a new sub-operator agent |
| `create_entity_type.py` | Declares a new EntityType with lifecycle and fields |

## Testing

```bash
cd spec && uv run pytest -q
```

The test suite includes:
- Structural invariants (50+ check_* functions)
- Anti-drift meta-tests (regenerated docs == committed bytes)
- Property tests via Hypothesis (critical-core conscience boundary)
- Entity/Process coupling tests
- Tool integration tests

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Dual-licensed under [MIT](LICENSE-MIT) OR [Apache-2.0](LICENSE-APACHE), at your option.
