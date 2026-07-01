# Hotam-Spec

[![License: MIT OR Apache-2.0](https://img.shields.io/badge/license-MIT%20OR%20Apache--2.0-blue.svg)](LICENSE)
[![Python 3.12+](https://img.shields.io/badge/python-3.12+-green.svg)](https://www.python.org)
[![Tests](https://img.shields.io/badge/tests-502%20passing-brightgreen.svg)](#testing)

> Executable methodology for contradictory business requirements, modeled as a tension graph.

## What is Hotam-Spec?

Hotam-Spec is a framework for managing the lifecycle of **contradictory business requirements**. Instead of pretending conflicts don't exist, it models them as first-class **tension nodes** in a graph -- visible, stewarded, and tracked through resolution.

### Key ideas

- **Conflict is a connector node**, not an edge -- it carries axis, context, steward, and lifecycle
- **Requirements contradict -- and that's expected** -- the methodology surfaces them, never hides them
- **The AI presents, never decides** -- every resolution stays with a human steward
- **Docs-as-code with structural anti-drift** -- generated docs must match the model byte-for-byte
- **Substrate generates the operator** -- the spec writes the AI's own prompt, not the reverse

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

## Project structure

```
spec/                          # Framework (shared, content-free)
├── src/hotam_spec/            # Ontology: Requirement, Conflict, Lifecycle, Entity, ...
├── tools/                     # Shared meta-tools (gen_spec, what_now, apply_proposal, ...)
├── tests/                     # 502+ tests
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
