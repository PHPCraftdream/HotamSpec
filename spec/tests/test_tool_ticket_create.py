"""Enforcer/lift for R-tool-ticket-create + R-ticket-engine-on-disk + R-ticket-carries-history.

Covers ticket creation via the tool: auto-id, initial backlog status, the file
landing in tickets/backlog/, a well-formed frontmatter header, and the mandatory
first "created" History entry (proving mutations auto-write History).
"""

from __future__ import annotations


import _ticket_store as ts  # noqa: E402
import ticket_create  # noqa: E402


def _redirect(tmp_path, monkeypatch):
    monkeypatch.setattr(ts, "TICKETS_DIR", tmp_path / "tickets")


def test_create_allocates_id_status_and_history(tmp_path, monkeypatch):
    _redirect(tmp_path, monkeypatch)
    ticket_create.main(
        ["--title", "First", "--assignee", "om", "--link", "R-x", "--description", "d"]
    )
    ids = ts.all_ids()
    assert ids == ["T-1"]
    t = ts.load("T-1")
    assert t.status == "backlog"
    assert t.path == ts.status_dir("backlog") / "T-1.md"
    assert t.header["title"] == "First"
    assert t.header["assignee"] == "om"
    assert t.header["links"] == ["R-x"]
    hist = ts.parse_history(t)
    assert len(hist) == 1
    assert hist[0]["action"] == "created"


def test_ids_autoincrement(tmp_path, monkeypatch):
    _redirect(tmp_path, monkeypatch)
    ticket_create.main(["--title", "a", "--assignee", "om"])
    ticket_create.main(["--title", "b", "--assignee", "om"])
    assert sorted(ts.all_ids(), key=lambda i: int(i.split("-")[1])) == ["T-1", "T-2"]


def test_frontmatter_roundtrips(tmp_path, monkeypatch):
    _redirect(tmp_path, monkeypatch)
    ticket_create.main(["--title", "Round", "--assignee", "steward"])
    raw = ts.find_path("T-1").read_text(encoding="utf-8")
    assert raw.startswith(ts.BEGIN_SENTINEL)
    assert ts.END_SENTINEL in raw
    header, body = ts._split_frontmatter(raw)
    assert header["id"] == "T-1"
    assert ts.COMMENTS_HEADING in body and ts.HISTORY_HEADING in body
