# Contributing to HotamSpec

Thank you for your interest in contributing!

## How to contribute

1. **Fork the repository** and create your branch from `master`.
2. **Build the CLI**: `go build -o bin/hotam ./cmd/hotam` (or `go run ./cmd/hotam <command>` without building).
3. **Make your changes** -- follow the patterns below.
4. **Run the full test suite**: `go test ./...` and `go vet ./...` -- all must pass. `go test -race ./...` is recommended for concurrency-sensitive changes.
5. **Regenerate docs**: `go run ./cmd/hotam gen-spec --domain <path>` -- commit the regenerated files.
6. **Open a Pull Request** with a clear description.

## Development workflow

### The closed loop

All changes to a domain graph (`domains/*/graph.json`) go through `hotam apply-proposal`. **`graph.json` is never hand-edited** -- it is a generated/managed artifact, and direct edits will be lost or rejected by invariant checks.

1. Run `hotam what-now --domain <path>` to find the top-priority action.
2. Construct a JSON proposal (`ProposedRequirement` / `ProposedConflictTransition` / `ProposedRejection` / `ProposedEntityType` / ...) -- field reference: `docs/PROPOSAL-REFERENCE.md`.
3. Apply: `hotam apply-proposal proposal.json --domain <path> --today YYYY-MM-DD`.
4. Regenerate docs: `hotam gen-spec --domain <path>`.
5. Verify: `hotam all-violations --domain <path>` -- exit code 1 means at least one structural invariant broke; fix before committing.

### Code style

- **Go**: formatted with `gofmt` (or `go fmt ./...`); keep `go vet ./...` clean.
- **Line endings**: LF (enforced by `.gitattributes`).
- **Encoding**: UTF-8 everywhere.
- **No version bumps** without maintainer approval.

### Adding a requirement

Use `hotam apply-proposal` with a `ProposedRequirement` JSON:

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

Apply it with:

```bash
hotam apply-proposal your-proposal.json --domain domains/hotam-spec-self --today 2026-07-12
```

See `docs/PROPOSAL-REFERENCE.md` for the full set of proposal shapes (conflicts, rejections, entity types, etc.).

### Review freshness policy

Every SETTLED requirement carries `last_reviewed_at` / `review_after` so `hotam due` can tell OVERDUE from NEVER-REVIEWED instead of reporting on empty fields. Default policy: a requirement is due for re-review 6 months after it was last confirmed. When you settle a requirement (via `ProposedRequirement` or a `ProposedConflictTransition` that settles it), set `last_reviewed_at` to the settlement date and `review_after` to that date + 6 months; re-affirming an existing SETTLED requirement without changing its content is a `ProposedReviewMark` proposal (`kind: "ReviewMark"`, fields `requirement_id` / `reviewed_at` / `review_after` / `evidence`), applied the same way as any other proposal. TODO: auto-defaulting `review_after` to `settled_at` + 6 months inside `ProposedRequirement`'s mutate step (so authors don't have to compute it by hand) is deferred -- it changes behavior for every future `Requirement` proposal, not just the backfill, so it needs its own steward-approved change rather than riding along with a one-time backfill.

## Pull request guidelines

- **One concern per PR** -- atomic changes are easier to review.
- **Tests required** -- new functionality must have tests (`go test ./...`); existing tests must stay green.
- **Docs auto-generated** -- run `hotam gen-spec` and commit the output; drift between the graph and generated docs is a defect.
- **No silent conflict closure** -- the AI presents, the human steward decides.
- **Green gate before commit** -- `go test ./...` and `go vet ./...` must pass before you commit; `go test -race ./...` is required for anything touching concurrency.
- **Quick local checks** -- a `Makefile` mirrors the CI gate: `make build`, `make vet`, `make test-race`, `make fmt-check`, and `make check` (runs the whole set). `make gen` regenerates the main domain docs. Run `make check` before pushing to catch CI failures locally.

## Code of conduct

Be respectful, constructive, and honest. We value clarity over cleverness.

## License

By contributing, you agree that your contributions will be dual-licensed under MIT OR Apache-2.0.
