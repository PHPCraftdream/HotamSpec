"""Enforcer/lift for R-tool-ticket-move + R-ticket-carries-history.

Covers status transition via the tool: the file relocates between status
folders, the header status/updated fields change, and a "status: X→Y" History
line is appended (auto-history on every status move).
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
import ticket_create  # noqa: E402
import ticket_move  # noqa: E402


def _mk(tmp_path, monkeypatch):
    monkeypatch.setattr(ts, "TICKETS_DIR", tmp_path / "tickets")
    ticket_create.main(["--title", "t", "--assignee", "om"])


def test_move_relocates_file_and_records_history(tmp_path, monkeypatch):
    _mk(tmp_path, monkeypatch)
    old = ts.find_path("T-1")
    assert old.parent.name == "backlog"
    ticket_move.main(["T-1", "in-progress"])
    new = ts.find_path("T-1")
    assert new.parent.name == "in-progress"
    assert not old.exists()
    t = ts.load("T-1")
    assert t.status == "in-progress"
    actions = [h["action"] for h in ts.parse_history(t)]
    assert actions == ["created", "status"]
    detail = ts.parse_history(t)[-1]["detail"]
    assert detail == "backlog→in-progress"


def test_move_to_same_status_is_noop(tmp_path, monkeypatch):
    _mk(tmp_path, monkeypatch)
    ticket_move.main(["T-1", "backlog"])
    t = ts.load("T-1")
    assert [h["action"] for h in ts.parse_history(t)] == ["created"]
