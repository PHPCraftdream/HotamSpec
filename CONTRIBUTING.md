# Contributing to Hotam-Spec

Thank you for your interest in contributing!

## How to contribute

1. **Fork the repository** and create your branch from `master`.
2. **Install dependencies**: `cd spec && python -m venv .venv && .venv/Scripts/activate && pip install -e .`
   (or, if you use [uv](https://docs.astral.sh/uv/): `cd spec && uv sync`)
3. **Make your changes** -- follow the patterns below.
4. **Run the full test suite**: `python -m pytest -q` -- all tests must pass.
5. **Regenerate docs**: `python tools/gen_spec.py` -- commit the regenerated files.
6. **Open a Pull Request** with a clear description.

## Development workflow

### The closed loop

All changes to the domain graph (`domains/*/graph.py`) go through `tools/apply_proposal.py`:

1. Construct a JSON proposal (ProposedRequirement / ProposedConflictTransition / ProposedRejection / ProposedEntityType).
2. Apply: `python tools/apply_proposal.py proposal.json` (or `uv run python tools/apply_proposal.py proposal.json` if you use uv)
3. The tool validates, writes, regenerates docs, and runs tests automatically.

Direct hand-editing of `graph.py` is discouraged.

### Code style

- **Python**: formatted with `ruff format`, linted with `ruff check`
- **Line endings**: LF (enforced by `.gitattributes`)
- **Encoding**: UTF-8 everywhere
- **No version bumps** without maintainer approval

### Adding a requirement

Use `apply_proposal.py` with a `ProposedRequirement` JSON:

```json
{
  "kind": "Requirement",
  "id": "R-your-requirement",
  "claim": "One atomic claim (no semicolons, no 'and + verb').",
  "owner": "your-stakeholder-id",
  "status": "DRAFT",
  "why": "Why this requirement matters.",
  "assumptions": ["A-relevant-assumption"],
  "enforcement": "STRUCTURAL",
  "enforced_by": []
}
```

### Adding a tool

Create `spec/tools/your_tool.py` with a first-line docstring:
```
Canon: <topic> -- one-line claim describing what this tool guarantees.
```

The tool auto-projects as `R-tool-your-tool` in the constitution on the next `gen_spec.py` run.

### Adding an entity type

```bash
python tools/create_entity_type.py your-entity \
  --description "What this entity represents" \
  --states "ACTIVE:initial,CLOSED:quiescent" \
  --transitions "close:ACTIVE->CLOSED" \
  --fields "name:string:required"
```

## Pull request guidelines

- **One concern per PR** -- atomic changes are easier to review.
- **Tests required** -- new functionality must have tests; existing tests must stay green.
- **Docs auto-generated** -- run `gen_spec.py` and commit the output; the anti-drift test catches staleness.
- **No silent conflict closure** -- the AI presents, the human steward decides (R-ai-presents-not-decides).

## Code of conduct

Be respectful, constructive, and honest. We value clarity over cleverness.

## License

By contributing, you agree that your contributions will be dual-licensed under MIT OR Apache-2.0.
