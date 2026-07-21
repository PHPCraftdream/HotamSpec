<!-- LEGACY (Python-era) command references below (tools/apply_proposal.py,
     graph.py, pytest); not an instruction to run as-is under the Go CLI —
     see README.md and docs/QUICKSTART-CONSUMER.md for current commands
     (`hotam apply-proposal`, `hotam gen-spec`, `go test`). The closed-loop
     CONCEPT this file describes is still current; only the literal tool
     invocations in the diagram below are stale. -->

# Playbooks — band-specific operator procedures

Each file in this directory is a playbook for one `what_now` priority band. A
playbook tells the AI operator exactly what to do when the harness surfaces an
action in that band, so that every action follows the closed-loop discipline
(R-active-loop-playbooks): the AI proposes, the resolver approves, the applier
mechanically writes the change.

## Index

| Band | File | Status |
|------|------|--------|
| P4 OPEN_ITEM | [P4-OPEN-ITEM.md](P4-OPEN-ITEM.md) | LIVE (P3) |
| P3 CONFLICT_STALLED | (deferred to P4) | — |
| P1 STRUCTURE | (deferred to P4) | — |
| P2 DRIFT_FALLOUT | (deferred to P4) | — |
| P5 LATENT_CONNECTOR | (deferred to P6) | — |

## How playbooks connect to the closed loop

```
what-now → band → playbook → ProposedConflictTransition / ProposedRequirement JSON
                                           ↓ resolver approves out-of-band
                              hotam apply-proposal → graph.json → hotam gen-spec → go test
```

The operator NEVER edits the domain's `graph.json` by hand. Every change flows
through a resolver-approved JSON proposal written by the operator and applied
mechanically (R-ai-presents-not-decides, §Proposal).
