"""Deterministic PreToolUse guard: deny direct Edit/Write on domains/*/graph.py.

Reads the PreToolUse JSON payload from stdin and, if tool_input.file_path
matches a domain's graph.py (not docs/gen/*, not manifest.py, not a
scope.py), emits a PreToolUse deny decision per R-no-hand-edit-graph.
Silent (no output) for everything else, which Claude Code treats as allow.
"""

import json
import sys


def main() -> int:
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return 0

    file_path = (payload.get("tool_input") or {}).get("file_path") or ""
    normalized = file_path.replace("\\", "/")
    parts = [p for p in normalized.split("/") if p]

    if len(parts) >= 2 and parts[-1] == "graph.py" and "domains" in parts[:-1]:
        print(
            json.dumps(
                {
                    "hookSpecificOutput": {
                        "hookEventName": "PreToolUse",
                        "permissionDecision": "deny",
                        "permissionDecisionReason": (
                            "Direct edits to domains/*/graph.py are prohibited "
                            "(R-no-hand-edit-graph). Use tools/apply_proposal.py instead."
                        ),
                    }
                }
            )
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
