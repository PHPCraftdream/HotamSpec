"""Enforcer/lift for R-tool-ticket-comment + R-ticket-carries-history.

Covers commenting via the tool: the comment lands under ## Comments and a
"commented" History entry is appended.
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import _ticket_store as ts  # noqa: E402
import ticket_comment  # noqa: E402
import ticket_create  # noqa: E402


def test_comment_appends_and_records_history(tmp_path, monkeypatch):
    monkeypatch.setattr(ts, "TICKETS_DIR", tmp_path / "tickets")
    ticket_create.main(["--title", "t", "--assignee", "om"])
    ticket_comment.main(["T-1", "looks good", "--actor", "steward"])
    t = ts.load("T-1")
    assert "looks good" in t.body
    assert "steward: looks good" in t.body
    actions = [h["action"] for h in ts.parse_history(t)]
    assert actions == ["created", "commented"]
