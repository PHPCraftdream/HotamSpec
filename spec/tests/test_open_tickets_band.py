"""Enforcer/lift for R-open-tickets-visible.

The what_now harness surfaces a CLI-only PENDING_PROPOSAL-band action summarising
open (non-done) on-disk tickets, and stays SILENT when there is no open ticket.
This band is filesystem-sourced and must NEVER enter diagnose() (determinism).
"""

from __future__ import annotations


import _ticket_store as ts  # noqa: E402
import ticket_create  # noqa: E402
import ticket_move  # noqa: E402
import what_now  # noqa: E402


def test_band_silent_when_no_open_tickets(tmp_path, monkeypatch):
    monkeypatch.setattr(ts, "TICKETS_DIR", tmp_path / "tickets")
    ts.ensure_layout()
    assert what_now.open_ticket_actions() == []


def test_band_reports_open_tickets_by_status(tmp_path, monkeypatch):
    monkeypatch.setattr(ts, "TICKETS_DIR", tmp_path / "tickets")
    ticket_create.main(["--title", "a", "--assignee", "om"])
    ticket_create.main(["--title", "b", "--assignee", "om"])
    ticket_move.main(["T-2", "in-progress"])
    actions = what_now.open_ticket_actions()
    assert len(actions) == 1
    msg = actions[0].imperative
    assert "open tickets: 2" in msg
    assert "backlog: 1" in msg and "in-progress: 1" in msg
    assert actions[0].target == "open-tickets"


def test_done_tickets_excluded_from_open_count(tmp_path, monkeypatch):
    monkeypatch.setattr(ts, "TICKETS_DIR", tmp_path / "tickets")
    ticket_create.main(["--title", "a", "--assignee", "om"])
    ticket_move.main(["T-1", "done"])
    assert what_now.open_ticket_actions() == []


def test_band_never_enters_diagnose(tmp_path, monkeypatch):
    # diagnose() must be graph-only; the ticket band is CLI-only. Guard by name.
    import inspect

    src = inspect.getsource(what_now.diagnose)
    assert "open_ticket_actions" not in src
