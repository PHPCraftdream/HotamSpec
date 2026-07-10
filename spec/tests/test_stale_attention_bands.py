"""Tests for etap E (task #107) new attention/runtime-fs sources.

Three ADVISORY, P6 PENDING_PROPOSAL-band sources added to
what_now.runtime_fs_sources() (mechanics half of the fleet-discipline audit,
lens-2-#3 / lens-4-#4):

  1. stale_ticket_actions      — non-done tickets older than STALE_TICKET_AGE_DAYS.
  2. stale_delegation_actions  — open DG-*.md delegations older than
                                  STALE_DELEGATION_AGE_DAYS.
  3. mutating_spawn_without_isolation_actions — thin attention wiring over
     spawn_log_isolation_status.compute_isolation_status() (previously not
     wired into the registry at all).

All three: (a) NOT part of diagnose(g) — filesystem-sourced, never enters
LIVE-STATE; (b) silent when there is nothing stale/flagged; (c) fire one
Action per offending record, P6 PENDING_PROPOSAL band (same band as every
other runtime-fs source), never a gate.
"""

from __future__ import annotations

import json


import _delegation_store as ds  # noqa: E402
import _ticket_store as ts  # noqa: E402
import delegate  # noqa: E402
import ticket_create  # noqa: E402
import ticket_move  # noqa: E402
import what_now  # noqa: E402


# --- stale_ticket_actions -----------------------------------------------


def test_stale_tickets_silent_when_none(tmp_path, monkeypatch):
    monkeypatch.setattr(ts, "TICKETS_DIR", tmp_path / "tickets")
    ticket_create.main(["--title", "fresh", "--assignee", "om"])
    assert what_now.stale_ticket_actions() == []


def test_stale_tickets_silent_within_threshold(tmp_path, monkeypatch):
    monkeypatch.setattr(ts, "TICKETS_DIR", tmp_path / "tickets")
    ticket_create.main(["--title", "recent", "--assignee", "om"])
    t = ts.load("T-1")
    created = t.header["created"]
    import datetime as _dt

    created_dt = _dt.datetime.strptime(created, "%Y-%m-%dT%H:%M:%SZ").replace(
        tzinfo=_dt.timezone.utc
    )
    now = created_dt.timestamp() + (what_now.STALE_TICKET_AGE_DAYS - 1) * 86400
    assert what_now.stale_ticket_actions(now=now) == []


def test_stale_tickets_fires_past_threshold(tmp_path, monkeypatch):
    monkeypatch.setattr(ts, "TICKETS_DIR", tmp_path / "tickets")
    ticket_create.main(["--title", "old", "--assignee", "om"])
    t = ts.load("T-1")
    created = t.header["created"]
    import datetime as _dt

    created_dt = _dt.datetime.strptime(created, "%Y-%m-%dT%H:%M:%SZ").replace(
        tzinfo=_dt.timezone.utc
    )
    now = created_dt.timestamp() + (what_now.STALE_TICKET_AGE_DAYS + 1) * 86400
    actions = what_now.stale_ticket_actions(now=now)
    assert [a.target for a in actions] == ["T-1"]
    assert "stale ticket T-1" in actions[0].imperative
    assert actions[0].priority == what_now.P_PENDING_PROPOSAL


def test_stale_tickets_excludes_done(tmp_path, monkeypatch):
    monkeypatch.setattr(ts, "TICKETS_DIR", tmp_path / "tickets")
    ticket_create.main(["--title", "old-done", "--assignee", "om"])
    ticket_move.main(["T-1", "done"])
    t = ts.load("T-1")
    created = t.header["created"]
    import datetime as _dt

    created_dt = _dt.datetime.strptime(created, "%Y-%m-%dT%H:%M:%SZ").replace(
        tzinfo=_dt.timezone.utc
    )
    now = created_dt.timestamp() + (what_now.STALE_TICKET_AGE_DAYS + 30) * 86400
    assert what_now.stale_ticket_actions(now=now) == []


def test_stale_tickets_never_enters_diagnose():
    import inspect

    src = inspect.getsource(what_now.diagnose)
    assert "stale_ticket_actions" not in src


# --- stale_delegation_actions --------------------------------------------


def _create_delegation(tmp_path, monkeypatch, *, date: str | None = None):
    monkeypatch.setattr(ds, "DELEGATIONS_DIR", tmp_path / "delegations")
    argv = [
        "create",
        "--to", "o46l",
        "--task", "do something",
        "--boundaries", "no commit",
        "--expected", "report",
    ]
    if date:
        argv.extend(["--date", date])
    delegate.main(argv)


def test_stale_delegations_silent_when_none(tmp_path, monkeypatch):
    _create_delegation(tmp_path, monkeypatch)
    assert what_now.stale_delegation_actions() == []


def test_stale_delegations_silent_within_threshold(tmp_path, monkeypatch):
    _create_delegation(tmp_path, monkeypatch, date="2026-07-01")
    import datetime as _dt

    created_dt = _dt.datetime(2026, 7, 1, tzinfo=_dt.timezone.utc)
    now = created_dt.timestamp() + (what_now.STALE_DELEGATION_AGE_DAYS - 1) * 86400
    assert what_now.stale_delegation_actions(now=now) == []


def test_stale_delegations_fires_past_threshold(tmp_path, monkeypatch):
    _create_delegation(tmp_path, monkeypatch, date="2026-07-01")
    import datetime as _dt

    created_dt = _dt.datetime(2026, 7, 1, tzinfo=_dt.timezone.utc)
    now = created_dt.timestamp() + (what_now.STALE_DELEGATION_AGE_DAYS + 1) * 86400
    actions = what_now.stale_delegation_actions(now=now)
    assert [a.target for a in actions] == ["DG-1"]
    assert "stale delegation DG-1" in actions[0].imperative
    assert actions[0].priority == what_now.P_PENDING_PROPOSAL


def test_stale_delegations_excludes_done(tmp_path, monkeypatch):
    _create_delegation(tmp_path, monkeypatch, date="2026-07-01")
    delegate.main(["close", "DG-1", "--result", "finished"])
    import datetime as _dt

    created_dt = _dt.datetime(2026, 7, 1, tzinfo=_dt.timezone.utc)
    now = created_dt.timestamp() + (what_now.STALE_DELEGATION_AGE_DAYS + 30) * 86400
    assert what_now.stale_delegation_actions(now=now) == []


def test_stale_delegations_never_enters_diagnose():
    import inspect

    src = inspect.getsource(what_now.diagnose)
    assert "stale_delegation_actions" not in src


# --- mutating_spawn_without_isolation_actions -----------------------------


def test_spawn_isolation_silent_when_log_absent(tmp_path, monkeypatch):
    import spawn_log_isolation_status as sl

    monkeypatch.setattr(sl, "_DEFAULT_LOG_PATH", tmp_path / "spawn-log.jsonl")
    assert what_now.mutating_spawn_without_isolation_actions() == []


def test_spawn_isolation_silent_when_clean(tmp_path, monkeypatch):
    import spawn_log_isolation_status as sl

    log_path = tmp_path / "spawn-log.jsonl"
    log_path.write_text(
        json.dumps(
            {
                "stamp": "2026-07-09T00:00:00Z",
                "agent": "x",
                "task_first_line": "t",
                "prompt_chars": 0,
                "isolation": "worktree",
                "mutating": True,
            }
        )
        + "\n",
        encoding="utf-8",
    )
    monkeypatch.setattr(sl, "_DEFAULT_LOG_PATH", log_path)
    assert what_now.mutating_spawn_without_isolation_actions() == []


def test_spawn_isolation_fires_when_flagged(tmp_path, monkeypatch):
    import spawn_log_isolation_status as sl

    log_path = tmp_path / "spawn-log.jsonl"
    log_path.write_text(
        json.dumps(
            {
                "stamp": "2026-07-09T00:00:00Z",
                "agent": "x",
                "task_first_line": "t",
                "prompt_chars": 0,
                "isolation": "shared",
                "mutating": True,
            }
        )
        + "\n",
        encoding="utf-8",
    )
    monkeypatch.setattr(sl, "_DEFAULT_LOG_PATH", log_path)
    actions = what_now.mutating_spawn_without_isolation_actions()
    assert len(actions) == 1
    assert actions[0].target == "spawn-log-isolation"
    assert "2026-07-09T00:00:00Z" in actions[0].imperative
    assert actions[0].priority == what_now.P_PENDING_PROPOSAL


def test_spawn_isolation_never_enters_diagnose():
    import inspect

    src = inspect.getsource(what_now.diagnose)
    assert "mutating_spawn_without_isolation_actions" not in src


# --- registry wiring -------------------------------------------------------


def test_all_three_sources_registered_in_runtime_fs_sources():
    ids = {s.id for s in what_now.runtime_fs_sources()}
    assert {"stale-tickets", "stale-delegations", "spawn-log-isolation"} <= ids


def test_runtime_fs_sources_all_tagged_runtime_fs():
    from hotam_spec.attention import READS_RUNTIME_FS

    for s in what_now.runtime_fs_sources():
        assert s.reads == READS_RUNTIME_FS
