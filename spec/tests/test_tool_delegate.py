"""Enforcer/lift for R-delegation-is-a-file: delegate.py create/close/show/list.

Covers: auto-id allocation, header shape, body scaffold, close mutation,
double-close exit 1, unknown-id exit 1, list/show read-only.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

_TOOLS = Path(__file__).resolve().parents[1] / "tools"
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))

import _delegation_store as ds  # noqa: E402
import delegate  # noqa: E402


def _redirect(tmp_path, monkeypatch):
    monkeypatch.setattr(ds, "DELEGATIONS_DIR", tmp_path / "delegations")


def _create_one(extra=None):
    argv = [
        "create",
        "--to", "o46l",
        "--task", "Build something",
        "--boundaries", "no commit",
        "--expected", "report",
    ]
    if extra:
        argv.extend(extra)
    delegate.main(argv)


# --- create -----------------------------------------------------------------


def test_create_allocates_id_and_writes_file(tmp_path, monkeypatch):
    _redirect(tmp_path, monkeypatch)
    _create_one()
    ids = ds.all_ids()
    assert ids == ["DG-1"]
    d = ds.load("DG-1")
    assert d.status == "open"
    assert d.header["to"] == "o46l"
    assert d.header["task"] == "Build something"
    assert d.header["boundaries"] == "no commit"
    assert d.header["expected_return"] == "report"
    assert d.header["result"] == ""


def test_ids_autoincrement(tmp_path, monkeypatch):
    _redirect(tmp_path, monkeypatch)
    _create_one()
    _create_one()
    assert sorted(ds.all_ids(), key=lambda i: int(i.split("-")[1])) == [
        "DG-1",
        "DG-2",
    ]


def test_frontmatter_roundtrips(tmp_path, monkeypatch):
    _redirect(tmp_path, monkeypatch)
    _create_one()
    raw = ds.find_path("DG-1").read_text(encoding="utf-8")
    assert raw.startswith("```json")
    assert "```\n" in raw
    header, body = ds._split_frontmatter(raw)
    assert header["id"] == "DG-1"
    assert "## Task" in body
    assert "## Result" in body


def test_body_contains_task_and_result_sections(tmp_path, monkeypatch):
    _redirect(tmp_path, monkeypatch)
    _create_one()
    d = ds.load("DG-1")
    assert "## Task" in d.body
    assert "## Result" in d.body
    assert "_(pending)_" in d.body


# --- close ------------------------------------------------------------------


def test_close_sets_done_and_result(tmp_path, monkeypatch):
    _redirect(tmp_path, monkeypatch)
    _create_one()
    delegate.main(["close", "DG-1", "--result", "All done"])
    d = ds.load("DG-1")
    assert d.status == "done"
    assert d.header["result"] == "All done"
    assert "All done" in d.body


def test_close_with_commit(tmp_path, monkeypatch):
    _redirect(tmp_path, monkeypatch)
    _create_one()
    delegate.main(["close", "DG-1", "--result", "ok", "--commit", "abc123"])
    d = ds.load("DG-1")
    assert d.header["result_commit"] == "abc123"


def test_double_close_exits_1(tmp_path, monkeypatch):
    _redirect(tmp_path, monkeypatch)
    _create_one()
    delegate.main(["close", "DG-1", "--result", "done"])
    with pytest.raises(SystemExit, match="1"):
        delegate.main(["close", "DG-1", "--result", "again"])


def test_unknown_id_exits_error(tmp_path, monkeypatch):
    _redirect(tmp_path, monkeypatch)
    ds.ensure_layout()
    with pytest.raises(FileNotFoundError):
        delegate.main(["close", "DG-99", "--result", "x"])


# --- show -------------------------------------------------------------------


def test_show_prints_header(tmp_path, monkeypatch, capsys):
    _redirect(tmp_path, monkeypatch)
    _create_one()
    delegate.main(["show", "DG-1"])
    out = capsys.readouterr().out
    assert "DG-1" in out
    assert "open" in out


def test_show_json(tmp_path, monkeypatch, capsys):
    _redirect(tmp_path, monkeypatch)
    _create_one()
    capsys.readouterr()  # drain create output
    delegate.main(["show", "DG-1", "--json"])
    out = capsys.readouterr().out
    parsed = json.loads(out)
    assert parsed["id"] == "DG-1"


# --- list -------------------------------------------------------------------


def test_list_shows_all(tmp_path, monkeypatch, capsys):
    _redirect(tmp_path, monkeypatch)
    _create_one()
    _create_one()
    delegate.main(["list"])
    out = capsys.readouterr().out
    assert "DG-1" in out
    assert "DG-2" in out
    assert "2 delegation(s)" in out


def test_list_filters_by_status(tmp_path, monkeypatch, capsys):
    _redirect(tmp_path, monkeypatch)
    _create_one()
    _create_one()
    delegate.main(["close", "DG-1", "--result", "done"])
    capsys.readouterr()  # drain prior output
    delegate.main(["list", "--status", "open"])
    out = capsys.readouterr().out
    assert "DG-1" not in out
    assert "DG-2" in out


def test_list_empty(tmp_path, monkeypatch, capsys):
    _redirect(tmp_path, monkeypatch)
    ds.ensure_layout()
    delegate.main(["list"])
    out = capsys.readouterr().out
    assert "no delegations" in out
