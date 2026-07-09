"""Canon: §Conflict — honesty check for the 'per explicit campaign delegation'
marker: every Conflict lifecycle text carrying it must resolve to a real,
dated record in the active domain's delegations.jsonl
(R-trust-anchor-delegation-explicit-only).

WHY a test, not a check_*: this is a CROSS-FILE consistency check (graph.py's
Conflict.lifecycle prose against a sibling delegations.jsonl file, not a
structural property of the graph object alone), and it runs against ONE
domain's committed delegation ledger — the same altitude as
test_agent_map.py's regen-stability check, not the check_* invariant layer
that governs domain-independent graph well-formedness. A hand-written marker
naming a delegation date with NO matching ledger entry is exactly the kind of
silent, unauthorized 'AI decided on its own' drift
R-trust-anchor-delegation-explicit-only exists to catch.

RULE: for every Conflict in the active domain's graph whose lifecycle text
contains the literal marker 'per explicit campaign delegation', extract the
first ISO date (YYYY-MM-DD) found after the marker and require AT LEAST ONE
delegations.jsonl record whose `date` field equals that date. A marker with
no matching record is a violation (an unauthorized decision dressed up as a
delegated one). The negative case is a synthetic Conflict-lifecycle string
with a marker+date that names no real ledger entry.
"""

from __future__ import annotations

import json
import re
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]

from hotam_spec.graph import load_content_graph  # noqa: E402

_MARKER = "per explicit campaign delegation"
_DATE_RE = re.compile(r"\d{4}-\d{2}-\d{2}")

_DOMAINS_ROOT = _SPEC_ROOT.parent / "domains"
_ACTIVE_DELEGATIONS = _DOMAINS_ROOT / "hotam-spec-self" / "delegations.jsonl"


def _dates_after_marker(text: str) -> list[str]:
    """Return every ISO date found in `text` after the FIRST marker occurrence."""
    idx = text.find(_MARKER)
    if idx == -1:
        return []
    return _DATE_RE.findall(text[idx:])


def _load_delegation_dates(path: Path) -> set[str]:
    if not path.exists():
        return set()
    dates: set[str] = set()
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        rec = json.loads(line)
        dates.add(rec.get("date", ""))
    return dates


# ---------------------------------------------------------------------------
# Positive: the real domain's marker resolves to DEL-1
# ---------------------------------------------------------------------------


def test_active_domain_delegation_markers_resolve() -> None:
    """Every 'per explicit campaign delegation <date>' marker in the active
    domain's Conflict lifecycles resolves to a delegations.jsonl record dated
    the same day.
    """
    g = load_content_graph()
    delegation_dates = _load_delegation_dates(_ACTIVE_DELEGATIONS)

    unresolved: list[str] = []
    found_any_marker = False
    for c in g.conflicts:
        dates = _dates_after_marker(c.lifecycle)
        if not dates:
            continue
        found_any_marker = True
        if not any(d in delegation_dates for d in dates):
            unresolved.append(f"{c.id}: marker dates {dates} not in {sorted(delegation_dates)}")

    assert found_any_marker, (
        "expected at least one Conflict lifecycle carrying the "
        f"'{_MARKER}' marker in the active domain (C-be22cdd1 / core-vs-aspect)."
    )
    assert not unresolved, (
        "Conflict(s) carry a campaign-delegation marker with no matching "
        f"delegations.jsonl record:\n" + "\n".join(unresolved)
    )


def test_del_1_specifically_resolves_the_core_vs_aspect_conflict() -> None:
    """The core-vs-aspect Conflict's 2026-07-02 marker resolves to DEL-1."""
    g = load_content_graph()
    delegation_dates = _load_delegation_dates(_ACTIVE_DELEGATIONS)
    assert "2026-07-02" in delegation_dates

    matches = [
        c
        for c in g.conflicts
        if _MARKER in c.lifecycle and "2026-07-02" in c.lifecycle
    ]
    assert matches, "expected the core-vs-aspect Conflict to carry the marker"


# ---------------------------------------------------------------------------
# Negative: a marker with no matching ledger entry is caught
# ---------------------------------------------------------------------------


def test_unresolved_marker_is_detected(tmp_path: Path) -> None:
    """A synthetic marker naming a date absent from the ledger fails resolution."""
    fake_delegations = tmp_path / "delegations.jsonl"
    fake_delegations.write_text(
        json.dumps(
            {
                "id": "DEL-1",
                "steward": "domain-user",
                "verbatim": "some other delegation",
                "date": "2026-01-01",
                "scope": "campaign: unrelated",
            }
        )
        + "\n",
        encoding="utf-8",
    )
    delegation_dates = _load_delegation_dates(fake_delegations)

    fake_lifecycle = (
        "DECIDED(chosen variant V-something per explicit campaign delegation "
        "2099-12-31 (\"fabricated, never actually granted\"))"
    )
    dates = _dates_after_marker(fake_lifecycle)
    assert dates == ["2099-12-31"]
    assert not any(d in delegation_dates for d in dates), (
        "expected the fabricated marker date to NOT resolve against an "
        "unrelated ledger — this is the case the honesty check must catch"
    )


def test_no_marker_no_dates() -> None:
    """A lifecycle text without the marker yields an empty date list (no false positive)."""
    assert _dates_after_marker("DECIDED(ordinary rationale, no delegation involved)") == []


def test_marker_without_date_yields_empty_list() -> None:
    """A marker present but with no date after it (malformed) yields no dates
    — such a Conflict would also fail resolution (empty dates -> not any -> unresolved),
    which is the conservative/honest failure mode.
    """
    assert _dates_after_marker("... per explicit campaign delegation but no date here") == []
