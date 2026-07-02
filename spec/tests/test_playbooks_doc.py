"""Enforcer for R-active-loop-playbook-doc.

At least one band-specific playbook shall exist under docs/playbooks/
describing the agent's role for that band. Band-specific = its filename names
a what_now priority band (P0..P5 prefix or a band label).
"""

from __future__ import annotations

import re
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
PLAYBOOKS_DIR = REPO_ROOT / "docs" / "playbooks"

_BAND_FILE_RE = re.compile(
    r"^P[0-5]-|REFLECTION|STRUCTURE|DRIFT_FALLOUT|CONFLICT_STALLED|OPEN_ITEM|LATENT_CONNECTOR",
    re.IGNORECASE,
)


def test_playbooks_dir_exists() -> None:
    """docs/playbooks/ exists (R-active-loop-playbook-doc)."""
    assert PLAYBOOKS_DIR.is_dir(), f"missing playbooks dir: {PLAYBOOKS_DIR}"


def test_at_least_one_band_specific_playbook_exists() -> None:
    """At least one playbook file names a what_now band (e.g. P4-OPEN-ITEM.md)."""
    md_files = sorted(PLAYBOOKS_DIR.glob("*.md"))
    assert md_files, f"docs/playbooks/ holds no markdown playbooks: {PLAYBOOKS_DIR}"
    band_files = [p for p in md_files if _BAND_FILE_RE.match(p.stem)]
    assert band_files, (
        "no band-specific playbook found under docs/playbooks/ — expected at "
        f"least one file named after a what_now band; found: {[p.name for p in md_files]}"
    )


def test_band_playbook_is_nonempty_and_names_agent_role() -> None:
    """A band playbook has real content describing the agent's role for the band."""
    band_files = [
        p for p in sorted(PLAYBOOKS_DIR.glob("*.md")) if _BAND_FILE_RE.match(p.stem)
    ]
    assert band_files
    text = band_files[0].read_text(encoding="utf-8")
    assert len(text.strip()) > 100, f"{band_files[0].name} is (nearly) empty"
    assert re.search(r"agent|operator", text, re.IGNORECASE), (
        f"{band_files[0].name} does not describe the agent/operator role"
    )
