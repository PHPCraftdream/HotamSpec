```json
{
  "id": "DG-3",
  "date": "2026-07-06",
  "from": "coordinator",
  "to": "crush",
  "task": "Etap 5.1: Signoff record -- signature survives into graph, Variants survive DECIDED, chosen_variant field",
  "boundaries": "spec/src+tools+tests, graph via apply_proposal only; no git writes; host untouched; update_baseline after invariants.py",
  "expected_return": "Signoff dataclass + writer fixes + invariants + fires tests, T2 green, determinism, report",
  "status": "done",
  "result": "Signoff dataclass + Conflict/Assumption.signoff fields; writer preserves decided_by/date/instrument/chosen_variant instead of losing it in gitignored JSON; Variants no longer erased on HELD->DECIDED; C-be22cdd1 migrated (variants restored from git history + signoff attached); 2 new invariants + fires tests; R-signoff-preserved-in-substrate [E]. T2 1067 passed. Session hit a network rate-limit near the end but work was already complete (verified by coordinator).",
  "result_commit": ""
}
```

## Task

Etap 5.1: Signoff record -- signature survives into graph, Variants survive DECIDED, chosen_variant field

## Result

Signoff dataclass + Conflict/Assumption.signoff fields; writer preserves decided_by/date/instrument/chosen_variant instead of losing it in gitignored JSON; Variants no longer erased on HELD->DECIDED; C-be22cdd1 migrated (variants restored from git history + signoff attached); 2 new invariants + fires tests; R-signoff-preserved-in-substrate [E]. T2 1067 passed. Session hit a network rate-limit near the end but work was already complete (verified by coordinator).
