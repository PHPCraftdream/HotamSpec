# Playbook — P4 OPEN_ITEM

What the harness says: "OPEN requirement 'R-…' (owner '…') awaits a decision: <question>"

## The agent's role: the SOCRATIC PARTNER (not the decider)

R-ai-presents-not-decides binds. For each P4 action the operator:

1. Reads the OPEN requirement's `claim` + `why` + assumptions (anchors all
   live in `spec/content/graph.py`; the prose in `docs/gen/REQUIREMENTS.md`
   is the human-friendly mirror).
2. Surfaces hidden assumptions: name the assumption(s) the question rests on,
   and whether they are HOLDS/UNCERTAIN/DEAD.
3. Proposes 2-3 distinct resolution variants — short, EARS-shaped, citing the
   axes they would close (R-axis-controlled-vocab).
4. Names the impact: which DRAFT requirements would be unblocked, which
   conflicts would spawn or close, what burn-down delta to expect.
5. Hands the steward a `ProposedRequirement` JSON for the variant the steward
   selects. Steward review is OUT-OF-BAND (here in chat, or a PR).
6. On steward approval: call `tools/apply_proposal.py <approved.json>` to
   mechanically land the change. Verify the pipeline runs green.

## What the operator MUST NOT do

- Decide the OPEN question itself (R-ai-presents-not-decides).
- Edit `spec/content/graph.py` by hand (use `apply_proposal.py`).
- Skip naming the assumption(s) — silent assumption-binding is exactly the
  invisibility Tensio surfaces (the three invisibilities in CLAUDE.md).

## JSON shape for a P4 resolution (ProposedRequirement)

```json
{
  "kind": "Requirement",
  "id": "R-<slug>",
  "claim": "The system shall ...",
  "owner": "<stakeholder-id>",
  "status": "SETTLED",
  "why": "Resolves M<N> by ...",
  "assumptions": ["A-<id>"],
  "enforcement": "PROSE"
}
```

## Checklist before handing to steward

- [ ] The OPEN requirement's id, claim, and question are cited exactly (R-speak-by-reference).
- [ ] At least one assumption is named and its status (HOLDS/UNCERTAIN/DEAD) is stated.
- [ ] At least two distinct resolution variants are presented.
- [ ] The impact (unblocked DRAFTs, spawned/closed conflicts, burn-down delta) is named.
- [ ] The proposed JSON is syntactically valid and targets a real R-id.
- [ ] `decided_by` is omitted from ProposedRequirement (it is only for ProposedConflictTransition).
