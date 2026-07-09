"""Enforcer/lift for R-tool-ticket-edit + R-ticket-carries-history.

Covers editing title/body via the tool: the new text is written AND the prior
text is snapshotted into a "text changed" History entry (the text-change trail
the steward asked for).
"""

from __future__ import annotations


import _ticket_store as ts  # noqa: E402
import ticket_create  # noqa: E402
import ticket_edit  # noqa: E402


def test_edit_title_snapshots_old_into_history(tmp_path, monkeypatch):
    monkeypatch.setattr(ts, "TICKETS_DIR", tmp_path / "tickets")
    ticket_create.main(["--title", "Old title", "--assignee", "om"])
    ticket_edit.main(["T-1", "--title", "New title"])
    t = ts.load("T-1")
    assert t.header["title"] == "New title"
    hist = ts.parse_history(t)
    assert hist[-1]["action"] == "text changed"
    assert "Old title" in hist[-1]["detail"]


def test_edit_body_preserves_old_text_verbatim(tmp_path, monkeypatch):
    monkeypatch.setattr(ts, "TICKETS_DIR", tmp_path / "tickets")
    ticket_create.main(
        ["--title", "T", "--assignee", "om", "--description", "original body"]
    )
    ticket_edit.main(["T-1", "--body", "brand new body"])
    t = ts.load("T-1")
    assert "brand new body" in t.body
    hist = ts.parse_history(t)
    assert hist[-1]["action"] == "text changed"
    assert "original body" in hist[-1]["detail"]
    # comments/history sections survive the edit
    assert ts.COMMENTS_HEADING in t.body and ts.HISTORY_HEADING in t.body
