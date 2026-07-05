"""Deterministic PreToolUse guard: deny direct Edit/Write on protected files.

Reads the PreToolUse JSON payload from stdin and denies edits to:
  1. domains/*/graph.py — R-no-hand-edit-graph (use tools/apply_proposal.py).
  2. spec/tests/*_baseline.json — R-enforcement-perimeter-baselines-guarded
     (use tools/update_baseline.py for sanctioned updates).
  3. spec/.runtime/active-domain, domains/.active-domain — pin files that
     select which domain is verified (use tools/create_domain.py --activate
     or write the pin through a tool).

Silent (no output) for everything else, which Claude Code treats as allow.
"""

import json
import sys


# Baseline filenames that are protected from direct edits.
# These are the ratchet baselines whose silent modification would weaken
# the enforcement perimeter (R-enforcement-perimeter-baselines-guarded).
_PROTECTED_BASELINES = {
    "atomicity_compound_baseline.json",
    "enforcement_perimeter_baseline.json",
    "frozen_aspects_baseline.json",
}

# Pin files that control which domain the verification machinery targets.
_PROTECTED_PINS = {
    "active-domain",  # matches both spec/.runtime/active-domain and domains/.active-domain
}


def _deny(reason: str) -> None:
    print(
        json.dumps(
            {
                "hookSpecificOutput": {
                    "hookEventName": "PreToolUse",
                    "permissionDecision": "deny",
                    "permissionDecisionReason": reason,
                }
            }
        )
    )


def main() -> int:
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return 0

    file_path = (payload.get("tool_input") or {}).get("file_path") or ""
    normalized = file_path.replace("\\", "/")
    parts = [p for p in normalized.split("/") if p]

    # Rule 1: domains/*/graph.py
    if len(parts) >= 2 and parts[-1] == "graph.py" and "domains" in parts[:-1]:
        _deny(
            "Direct edits to domains/*/graph.py are prohibited "
            "(R-no-hand-edit-graph). Use tools/apply_proposal.py instead."
        )
        return 0

    # Rule 2: spec/tests/*_baseline.json
    if parts and parts[-1] in _PROTECTED_BASELINES:
        # Verify it's under a tests/ directory
        if "tests" in parts[:-1]:
            _deny(
                f"Direct edits to {parts[-1]} are prohibited "
                "(R-enforcement-perimeter-baselines-guarded). "
                "Use tools/update_baseline.py for sanctioned updates."
            )
            return 0

    # Rule 3: active-domain pin files
    if parts and parts[-1] in _PROTECTED_PINS:
        _deny(
            "Direct edits to the active-domain pin are prohibited "
            "(R-enforcement-perimeter-baselines-guarded). "
            "Use tools/create_domain.py --activate instead."
        )
        return 0

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
