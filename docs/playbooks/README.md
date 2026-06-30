# Playbooks — band-specific operator procedures

Each file in this directory is a playbook for one `what_now` priority band. A
playbook tells the AI operator exactly what to do when the harness surfaces an
action in that band, so that every action follows the closed-loop discipline
(R-active-loop-playbooks): the AI proposes, the steward approves, the applier
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
what_now → band → playbook → ProposedConflictTransition / ProposedRequirement JSON
                                           ↓ steward approves out-of-band
                              tools/apply_proposal.py → graph.py → gen_spec → pytest
```

The operator NEVER edits `spec/content/graph.py` by hand. Every change flows
through a steward-approved JSON proposal written by the operator and applied
mechanically (R-ai-presents-not-decides, §Proposal).
