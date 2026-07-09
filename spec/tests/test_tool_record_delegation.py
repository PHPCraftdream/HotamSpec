"""Tests for spec/tools/record_delegation.py — delegation registry writer.

Covers:
  1. Positive: first record gets DEL-1, second gets DEL-2 (auto-increment).
  2. Negative: unknown steward is refused.
  3. Negative: empty verbatim / scope refused.
  4. Default date fills in today's ISO date.
"""

from __future__ import annotations

import json
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]


import pytest  # noqa: E402

import record_delegation  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402


def _g() -> TensionGraph:
    sh = (
        Stakeholder(id="domain-user", name="Domain User", domain="test"),
        Stakeholder(id="framework-author", name="Author", domain="test"),
    )
    return TensionGraph(stakeholders=sh)


# ---------------------------------------------------------------------------
# 1. Positive: record + auto-increment
# ---------------------------------------------------------------------------


def test_first_record_gets_del_1(tmp_path: Path) -> None:
    path = tmp_path / "delegations.jsonl"
    rc = record_delegation.record_delegation(
        steward="domain-user",
        verbatim="решай все вопросы в сторону совершенства",
        scope="campaign: test",
        date="2026-07-02",
        graph=_g(),
        delegations_path=path,
    )
    assert rc == 0
    lines = path.read_text(encoding="utf-8").splitlines()
    assert len(lines) == 1
    rec = json.loads(lines[0])
    assert rec["id"] == "DEL-1"
    assert rec["steward"] == "domain-user"
    assert rec["date"] == "2026-07-02"
    assert rec["scope"] == "campaign: test"


def test_second_record_increments_to_del_2(tmp_path: Path) -> None:
    path = tmp_path / "delegations.jsonl"
    record_delegation.record_delegation(
        steward="domain-user",
        verbatim="first delegation",
        scope="campaign: one",
        date="2026-07-01",
        graph=_g(),
        delegations_path=path,
    )
    rc = record_delegation.record_delegation(
        steward="framework-author",
        verbatim="second delegation",
        scope="campaign: two",
        date="2026-07-02",
        graph=_g(),
        delegations_path=path,
    )
    assert rc == 0
    lines = path.read_text(encoding="utf-8").splitlines()
    assert len(lines) == 2
    rec2 = json.loads(lines[1])
    assert rec2["id"] == "DEL-2"
    assert rec2["steward"] == "framework-author"


def test_increment_survives_gaps(tmp_path: Path) -> None:
    """Next id is max(existing)+1 even if the file has a gap or is unordered."""
    path = tmp_path / "delegations.jsonl"
    path.write_text(
        json.dumps({"id": "DEL-1", "steward": "domain-user", "verbatim": "x",
                    "date": "2026-01-01", "scope": "s"}) + "\n"
        + json.dumps({"id": "DEL-5", "steward": "domain-user", "verbatim": "y",
                      "date": "2026-01-02", "scope": "s"}) + "\n",
        encoding="utf-8",
    )
    rc = record_delegation.record_delegation(
        steward="domain-user",
        verbatim="new one",
        scope="campaign: three",
        date="2026-07-03",
        graph=_g(),
        delegations_path=path,
    )
    assert rc == 0
    lines = path.read_text(encoding="utf-8").splitlines()
    assert len(lines) == 3
    rec3 = json.loads(lines[2])
    assert rec3["id"] == "DEL-6"


# ---------------------------------------------------------------------------
# 2. Negative: unknown steward
# ---------------------------------------------------------------------------


def test_unknown_steward_refused(tmp_path: Path) -> None:
    path = tmp_path / "delegations.jsonl"
    rc = record_delegation.record_delegation(
        steward="not-a-real-stakeholder",
        verbatim="some delegation",
        scope="campaign: test",
        graph=_g(),
        delegations_path=path,
    )
    assert rc == 1
    assert not path.exists()


# ---------------------------------------------------------------------------
# 3. Negative: empty verbatim / scope / steward
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    "steward,verbatim,scope",
    [
        pytest.param("domain-user", "   ", "campaign: test", id="empty-verbatim"),
        pytest.param("domain-user", "some delegation", "   ", id="empty-scope"),
        pytest.param("   ", "some delegation", "campaign: test", id="empty-steward"),
    ],
)
def test_blank_required_field_refused(
    tmp_path: Path, steward: str, verbatim: str, scope: str
) -> None:
    path = tmp_path / "delegations.jsonl"
    rc = record_delegation.record_delegation(
        steward=steward,
        verbatim=verbatim,
        scope=scope,
        graph=_g(),
        delegations_path=path,
    )
    assert rc == 1


# ---------------------------------------------------------------------------
# 4. Default date
# ---------------------------------------------------------------------------


def test_default_date_is_today(tmp_path: Path) -> None:
    import datetime as _dt

    path = tmp_path / "delegations.jsonl"
    rc = record_delegation.record_delegation(
        steward="domain-user",
        verbatim="some delegation",
        scope="campaign: test",
        graph=_g(),
        delegations_path=path,
    )
    assert rc == 0
    rec = json.loads(path.read_text(encoding="utf-8").splitlines()[0])
    assert rec["date"] == _dt.date.today().isoformat()


# ---------------------------------------------------------------------------
# 5. Real seed record exists (DEL-1 in the active domain)
# ---------------------------------------------------------------------------


def test_seed_delegation_del_1_exists() -> None:
    """The committed domains/hotam-spec-self/delegations.jsonl carries DEL-1."""
    domains_root = _SPEC_ROOT.parent / "domains"
    path = domains_root / "hotam-spec-self" / "delegations.jsonl"
    assert path.exists(), f"expected {path} to exist"
    lines = [ln for ln in path.read_text(encoding="utf-8").splitlines() if ln.strip()]
    assert lines, "expected at least one delegation record"
    rec = json.loads(lines[0])
    assert rec["id"] == "DEL-1"
    assert rec["steward"] == "domain-user"
    assert rec["date"] == "2026-07-02"
    assert "campaign" in rec["scope"]


# ---------------------------------------------------------------------------
# 6. status field + close (revoke) mechanic
# ---------------------------------------------------------------------------


def test_new_record_is_born_active(tmp_path: Path) -> None:
    path = tmp_path / "delegations.jsonl"
    record_delegation.record_delegation(
        steward="domain-user",
        verbatim="v",
        scope="campaign: s",
        date="2026-07-03",
        graph=_g(),
        delegations_path=path,
    )
    rec = json.loads(path.read_text(encoding="utf-8").splitlines()[0])
    assert rec["status"] == "active"
    assert rec["closed_date"] == ""


def test_close_flips_status_and_stamps_date(tmp_path: Path) -> None:
    path = tmp_path / "delegations.jsonl"
    record_delegation.record_delegation(
        steward="domain-user", verbatim="v", scope="campaign: s",
        date="2026-07-03", graph=_g(), delegations_path=path,
    )
    rc = record_delegation.close_delegation(
        delegation_id="DEL-1", date="2026-07-04", delegations_path=path
    )
    assert rc == 0
    rec = json.loads(path.read_text(encoding="utf-8").splitlines()[0])
    assert rec["status"] == "closed"
    assert rec["closed_date"] == "2026-07-04"
    # verbatim/scope/date trail preserved untouched
    assert rec["verbatim"] == "v"
    assert rec["scope"] == "campaign: s"
    assert rec["date"] == "2026-07-03"


def test_close_unknown_id_refused(tmp_path: Path) -> None:
    path = tmp_path / "delegations.jsonl"
    record_delegation.record_delegation(
        steward="domain-user", verbatim="v", scope="campaign: s",
        graph=_g(), delegations_path=path,
    )
    rc = record_delegation.close_delegation(
        delegation_id="DEL-99", delegations_path=path
    )
    assert rc == 1


def test_close_already_closed_refused(tmp_path: Path) -> None:
    path = tmp_path / "delegations.jsonl"
    record_delegation.record_delegation(
        steward="domain-user", verbatim="v", scope="campaign: s",
        graph=_g(), delegations_path=path,
    )
    record_delegation.close_delegation(delegation_id="DEL-1", delegations_path=path)
    rc = record_delegation.close_delegation(
        delegation_id="DEL-1", delegations_path=path
    )
    assert rc == 1


def test_close_backfills_legacy_records_status(tmp_path: Path) -> None:
    """A pre-status legacy record gains an explicit status on the next close."""
    path = tmp_path / "delegations.jsonl"
    path.write_text(
        json.dumps({"id": "DEL-1", "steward": "domain-user", "verbatim": "x",
                    "date": "2026-01-01", "scope": "s"}) + "\n"
        + json.dumps({"id": "DEL-2", "steward": "domain-user", "verbatim": "y",
                      "date": "2026-01-02", "scope": "s"}) + "\n",
        encoding="utf-8",
    )
    record_delegation.close_delegation(
        delegation_id="DEL-2", date="2026-07-04", delegations_path=path
    )
    recs = [json.loads(ln) for ln in path.read_text(encoding="utf-8").splitlines()]
    assert recs[0]["status"] == "active"  # backfilled
    assert recs[1]["status"] == "closed"


def test_seed_del_1_is_closed_campaign_preserved() -> None:
    """The committed DEL-1 carries the status lifecycle field.

    DEL-1 was the standing campaign delegation ('реши все задачи...'). The
    steward closed it on 2026-07-05 (campaign over), so every conflict
    resolution again requires an explicit signature. The record is NOT
    deleted -- it is flipped to 'closed' with a closed_date, preserving the
    trace (mirroring R-rejected-preserved-not-deleted).
    """
    domains_root = _SPEC_ROOT.parent / "domains"
    path = domains_root / "hotam-spec-self" / "delegations.jsonl"
    rec = json.loads(
        [ln for ln in path.read_text(encoding="utf-8").splitlines() if ln.strip()][0]
    )
    assert rec["id"] == "DEL-1"
    assert rec["status"] == "closed"
    assert rec.get("closed_date", "") != ""


def test_cli_close_end_to_end(tmp_path: Path, monkeypatch) -> None:
    """main(--close DEL-1) resolves through the CLI and flips status."""
    path = tmp_path / "delegations.jsonl"
    record_delegation.record_delegation(
        steward="domain-user", verbatim="v", scope="campaign: s",
        graph=_g(), delegations_path=path,
    )
    monkeypatch.setattr(record_delegation, "_delegations_path", lambda *a, **k: path)
    rc = record_delegation.main(["--close", "DEL-1", "--date", "2026-07-05"])
    assert rc == 0
    rec = json.loads(path.read_text(encoding="utf-8").splitlines()[0])
    assert rec["status"] == "closed" and rec["closed_date"] == "2026-07-05"
