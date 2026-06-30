# hotam_spec (spec)

Executable model for the **Hotam-Spec** methodology: managing the lifecycle of
contradictory business requirements as a **tension graph**. The store IS this
Python code — frozen dataclasses in `src/hotam_spec/`, structural invariants as
`check_*` functions, the human layer generated into `../docs/gen/`.

This is the spec package. For the philosophy and the closed loop see the
repository [`README.md`](../README.md), [`CLAUDE.md`](../CLAUDE.md) and
[`docs/methodology/README.md`](../docs/methodology/README.md).

## Commands

```bash
uv run ruff check --fix && uv run ruff format   # lint / format
uv run python tools/gen_spec.py                 # regen docs/gen/{REQUIREMENTS,TENSIONS,OPEN}.md
uv run python tools/what_now.py                 # the harness: prioritized next actions
uv run pytest -q                                # tests (meta-test: regen == committed)
```

## Layout

- `src/hotam_spec/` — the ontology (Requirement, Conflict, Assumption, Axis,
  Stakeholder), the seed graph + traversal (`graph.py`), and the structural
  invariants (`invariants.py`).
- `tools/gen_spec.py` — deterministic generator of the human layer.
- `tools/what_now.py` — the harness ("agent is never lost"): derives the next
  correct action from any graph state.
- `tests/` — invariants (with broken fixtures), the anti-drift meta-test, and
  the harness tests.

Requires [uv](https://github.com/astral-sh/uv) (Python 3.12+). Dual-licensed
MIT OR Apache-2.0.
