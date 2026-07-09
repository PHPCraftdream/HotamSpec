"""Bijection test: DECISIONS.md, graph m_tags, and CLAUDE.md M-table stay in sync.

Three assertions enforcing the U5 anti-drift guarantee:

  1. Every Requirement with a non-empty `m_tag` appears in `docs/gen/DECISIONS.md`
     (regex over the generated text).

  2. Every M-row in CLAUDE.md that names an `R-…` anchor in its cell MUST point
     to a Requirement whose `m_tag` matches the M-number AND whose `id` matches
     the cited anchor.  CLAUDE.md stays a HUMAN-FRIENDLY VIEW; this test ensures
     every cross-anchor it claims actually resolves in the graph.

  3. No two Requirements share the same `m_tag` (also enforced by
     `check_m_tag_format`, verified here at the bijection level).

WHY not "every CLAUDE.md row must have a graph mirror": the M-table may contain
rows without an `m_tag` mirror (M1, M2, M4, … — prose-only decisions not yet
crystallized as Requirements).  That is M-table sparseness, not drift.  The
one-way direction (every m_tag req has a CLAUDE.md row) is what this test
checks in assertion 2; the graph side is the source of truth.
"""

from __future__ import annotations

import re
from pathlib import Path


import gen_spec  # noqa: E402

from hotam_spec.graph import load_content_graph  # noqa: E402

_REPO_ROOT = Path(__file__).resolve().parents[2]  # spec/tests -> spec -> HotamSpec
_CLAUDE_MD = _REPO_ROOT / "CLAUDE.md"

# Match M-table rows: | M<n> | ...
_M_ROW_RE = re.compile(r"^\| M(\d+)\s*\|")
# Extract R-… anchors from a cell (backtick-quoted)
_R_ANCHOR_RE = re.compile(r"`(R-[A-Za-z0-9_-]+)`")
# Match a DECISIONS.md row: | M<n> | `R-…` | ...
_DECISIONS_ROW_RE = re.compile(r"^\| M(\d+) \| `(R-[A-Za-z0-9_-]+)`")


def _read_claude_m_rows() -> dict[str, list[str]]:
    """Return {M-number-str: [R-id, ...]} for every M-row in CLAUDE.md citing an R-anchor."""
    result: dict[str, list[str]] = {}
    text = _CLAUDE_MD.read_text(encoding="utf-8")
    for line in text.splitlines():
        m = _M_ROW_RE.match(line)
        if not m:
            continue
        m_num = m.group(1)
        anchors = _R_ANCHOR_RE.findall(line)
        if anchors:
            result[m_num] = anchors
    return result


# ---------------------------------------------------------------------------
# Assertion 1: every graph m_tag appears in DECISIONS.md
# ---------------------------------------------------------------------------


def test_every_m_tag_req_appears_in_decisions_md() -> None:
    """Every Requirement with a non-empty m_tag appears as a row in DECISIONS.md.

    An empty `tagged` set is legitimate once every M-tagged OPEN item has been
    resolved to SETTLED/REJECTED (m_tag is OPEN-only per check_m_tag_open_only)
    -- e.g. R-partition-vs-border (M18) was the last OPEN M-tag; once REJECTED
    with REPLACES, the graph carries zero m_tags. That is the calm "no open
    M-decisions remain" state, not vacuous coverage -- the loop below still
    proves the bijection holds for whatever IS tagged (0 or more).
    """
    g = load_content_graph()
    tagged = [r for r in g.requirements if r.m_tag]

    decisions_text = gen_spec.build_decisions(g)
    missing: list[str] = []
    for r in tagged:
        # Look for the M-tag and requirement id in a DECISIONS.md table row
        pattern = re.compile(
            r"^\| " + re.escape(r.m_tag) + r" \| `" + re.escape(r.id) + r"`",
            re.MULTILINE,
        )
        if not pattern.search(decisions_text):
            missing.append(f"{r.m_tag} -> {r.id}")
    assert not missing, (
        "These m_tag requirements are missing from DECISIONS.md:\n"
        + "\n".join(missing)
        + "\nRun: uv run python tools/gen_spec.py"
    )


# ---------------------------------------------------------------------------
# Assertion 2: every CLAUDE.md R-anchor cross-reference resolves in the graph
# ---------------------------------------------------------------------------


def test_claude_md_r_anchors_resolve_in_graph_with_matching_m_tag() -> None:
    """Every R-anchor in a CLAUDE.md M-row names a Requirement with the matching m_tag.

    Checks both:
      (a) the R-id exists in the graph (dangling anchor check), and
      (b) the Requirement's m_tag matches the M-number in that row.
    """
    g = load_content_graph()
    req_by_id = {r.id: r for r in g.requirements}

    claude_rows = _read_claude_m_rows()
    if not claude_rows:
        # Root CLAUDE.md no longer carries the M-table (P19c removed it; the
        # canonical M-registry is now docs/gen/DECISIONS.md). The bijection is
        # enforced graph-side (Assertion 1) and at the DECISIONS.md level via
        # gen_spec; this assertion becomes a no-op until a CLAUDE.md surfaces
        # M-rows again.
        return

    bad: list[str] = []
    for m_num, anchors in claude_rows.items():
        expected_tag = f"M{m_num}"
        for rid in anchors:
            if rid not in req_by_id:
                bad.append(f"M{m_num}: `{rid}` not found in content graph (dangling)")
            elif req_by_id[rid].m_tag != expected_tag:
                actual = req_by_id[rid].m_tag or "(empty)"
                bad.append(
                    f"M{m_num}: `{rid}` exists but m_tag={actual!r} "
                    f"(expected {expected_tag!r})"
                )
    assert not bad, "CLAUDE.md M-table / graph m_tag mismatch:\n" + "\n".join(bad)


# ---------------------------------------------------------------------------
# Assertion 3: no two Requirements share an m_tag (bijection at graph level)
# ---------------------------------------------------------------------------


def test_no_duplicate_m_tags_in_graph() -> None:
    """No two Requirements in the graph share the same m_tag value."""
    g = load_content_graph()
    seen: dict[str, str] = {}  # tag -> first req id
    duplicates: list[str] = []
    for r in g.requirements:
        if not r.m_tag:
            continue
        if r.m_tag in seen:
            duplicates.append(f"{r.m_tag}: {seen[r.m_tag]} and {r.id}")
        else:
            seen[r.m_tag] = r.id
    assert not duplicates, "Duplicate m_tags found in graph:\n" + "\n".join(duplicates)
