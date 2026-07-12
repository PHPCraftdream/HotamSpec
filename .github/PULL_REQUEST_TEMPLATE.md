## Summary

<!-- 1-3 bullet points describing what this PR does -->

## Changes

<!-- List the key changes -->

## Test plan

<!-- How was this tested? -->
- [ ] `go test ./...` passes (all tests green)
- [ ] `go vet ./...` is clean
- [ ] `go run ./cmd/hotam gen-spec --domain <path> --claude-md CLAUDE.md` produces no diff (docs regenerate identically)

## Checklist

- [ ] Changes to `domains/*/graph.json` went through `hotam apply-proposal` or `hotam land` (not hand-edited — R-no-hand-edit-graph)
- [ ] New requirements are atomic (one claim per R, no semicolons)
- [ ] New Go invariants/tools carry a `Canon`/`Claim`/`Why` in their registry entry
- [ ] No version bumps without maintainer approval
