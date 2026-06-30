"""Sync test: every R-anchor cited in the CLAUDE.md M-table resolves in the graph.

Enforces `R-anchor-everything` for the M-table cross-references added in
U5: an R-… id written in the M-decision cell must name a Requirement that
actually exists in load_content_graph(). This is a one-way check (NOT yet
bijection — full bijection is a Batch-B item).

WHY not a manual list: the test parses the actual CLAUDE.md at runtime, so
adding a new M-row with an R-anchor automatically adds a new assertion.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from tensio.graph import load_content_graph  # noqa: E402

_REPO_ROOT = Path(__file__).resolve().parents[2]  # spec/tests -> spec -> HotamSpec
_CLAUDE_MD = _REPO_ROOT / "CLAUDE.md"

# Match M-table rows: | M<n> | ... |
_M_ROW_RE = re.compile(r"^\| M\d+\s*\|")
# Extract all R-… tokens from a cell
_R_ANCHOR_RE = re.compile(r"`(R-[A-Za-z0-9_-]+)`")


def _parse_m_table_r_anchors() -> dict[str, list[str]]:
    """Return {M-row-label: [R-id, ...]} for every M-row mentioning an R-anchor."""
    result: dict[str, list[str]] = {}
    text = _CLAUDE_MD.read_text(encoding="utf-8")
    for line in text.splitlines():
        if not _M_ROW_RE.match(line):
            continue
        cols = [c.strip() for c in line.split("|") if c.strip()]
        if not cols:
            continue
        label = cols[0]  # e.g. "M3"
        anchors = _R_ANCHOR_RE.findall(line)
        if anchors:
            result[label] = anchors
    return result


def test_m_table_r_anchors_resolve_in_content_graph() -> None:
    """Every R-anchor in the M-table names a real Requirement in the content graph."""
    g = load_content_graph()
    req_ids = {r.id for r in g.requirements}
    mapping = _parse_m_table_r_anchors()
    assert mapping, "M-table parse found no R-anchors — check regex or CLAUDE.md path"
    bad: list[str] = []
    for m_label, anchors in mapping.items():
        for rid in anchors:
            if rid not in req_ids:
                bad.append(f"{m_label}: `{rid}` not found in content graph")
    assert not bad, "M-table cites R-anchors that don't exist:\n" + "\n".join(bad)
