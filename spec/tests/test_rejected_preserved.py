"""Enforcer for R-rejected-preserved-not-deleted.

Requirements that are rejected shall be marked REJECTED and kept in the graph
for history, never deleted.

Machine-checkable projection: every requirement id that the committed
HISTORY.md lists under '## REJECTED requirements' must still resolve in the
CURRENT graph with status REJECTED. Combined with test_docs_gen.py's
byte-equality gate (committed docs == regeneration), silently deleting a
REJECTED node either breaks this test (id in committed HISTORY.md, gone from
graph) or forces a visible HISTORY.md diff in review. The graph is also
required to carry a non-empty REJECTED history (the meta-domain permanently
holds its dead ends — R-rejected-preserved-not-deleted).
"""

from __future__ import annotations

import re
from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]

from hotam_spec.graph import load_content_graph  # noqa: E402
from hotam_spec.requirement import REJECTED  # noqa: E402

_HISTORY_MD = (
    SPEC_ROOT.parent / "domains" / "hotam-spec-self" / "docs" / "gen" / "HISTORY.md"
)

_REJECTED_HEADING_RE = re.compile(r"^### `(R-[a-z0-9-]+)`", re.MULTILINE)


def _rejected_ids_in_history() -> list[str]:
    """Ids listed under '## REJECTED requirements' in the committed HISTORY.md."""
    text = _HISTORY_MD.read_text(encoding="utf-8")
    start = text.find("## REJECTED requirements")
    assert start != -1, "HISTORY.md lacks the '## REJECTED requirements' section"
    end = text.find("\n## ", start + 1)
    section = text[start:] if end == -1 else text[start:end]
    return _REJECTED_HEADING_RE.findall(section)


def test_graph_keeps_a_nonempty_rejected_history() -> None:
    """The meta-domain graph carries REJECTED requirements (history preserved)."""
    g = load_content_graph()
    rejected = [r for r in g.requirements if r.status == REJECTED]
    assert rejected, (
        "the meta-domain must keep its REJECTED requirements — an empty "
        "REJECTED set would mean history was deleted "
        "(R-rejected-preserved-not-deleted)"
    )


def test_every_history_rejected_id_still_in_graph_as_rejected() -> None:
    """Every id in HISTORY.md's REJECTED section resolves in the graph, status REJECTED."""
    g = load_content_graph()
    by_id = {r.id: r for r in g.requirements}
    history_ids = _rejected_ids_in_history()
    assert history_ids, "HISTORY.md lists no REJECTED requirements — unexpected"

    missing = [rid for rid in history_ids if rid not in by_id]
    assert not missing, (
        f"REJECTED requirements deleted from the graph but present in "
        f"committed HISTORY.md: {missing} (R-rejected-preserved-not-deleted)"
    )
    wrong_status = [
        rid for rid in history_ids if by_id[rid].status != REJECTED
    ]
    assert not wrong_status, (
        f"HISTORY.md lists these as REJECTED but their graph status differs: "
        f"{wrong_status}"
    )
