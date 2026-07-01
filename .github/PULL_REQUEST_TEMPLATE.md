## Summary

<!-- 1-3 bullet points describing what this PR does -->

## Changes

<!-- List the key changes -->

## Test plan

<!-- How was this tested? -->
- [ ] `uv run pytest -q` passes (all tests green)
- [ ] `uv run python tools/gen_spec.py` produces no diff
- [ ] `uv run python tools/audit_atomicity.py` shows no new compound claims

## Checklist

- [ ] Changes to `domains/*/graph.py` went through `apply_proposal.py` (not hand-edited)
- [ ] New requirements are atomic (one claim per R, no semicolons)
- [ ] New tools have `Canon: <topic> -- claim` first-line docstring
- [ ] No version bumps without maintainer approval
