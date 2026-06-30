"""Guard: root CLAUDE.md must not contain a hand-written M-table.

The canonical M-registry lives in domains/hotam-spec-self/docs/gen/DECISIONS.md
(generated). Any | M<N> row in the root CLAUDE.md is a stale duplicate.
"""

from pathlib import Path

ROOT_CLAUDE = Path(__file__).resolve().parents[2] / "CLAUDE.md"


def test_root_claude_md_has_no_m_table_rows() -> None:
    """Root CLAUDE.md contains no lines matching ^| M[0-9]."""
    text = ROOT_CLAUDE.read_text(encoding="utf-8")
    offending = [
        line
        for line in text.splitlines()
        if line.startswith("| M") and line[3].isdigit()
    ]
    assert offending == [], (
        f"Root CLAUDE.md still contains {len(offending)} hand-written M-table row(s). "
        "Remove them — the canonical registry is domains/hotam-spec-self/docs/gen/DECISIONS.md.\n"
        + "\n".join(offending[:5])
    )


def test_root_claude_md_links_to_decisions_md() -> None:
    """Root CLAUDE.md must reference the generated DECISIONS.md path."""
    text = ROOT_CLAUDE.read_text(encoding="utf-8")
    needle = "domains/hotam-spec-self/docs/gen/DECISIONS.md"
    assert needle in text, (
        f"Root CLAUDE.md does not link to {needle}. "
        "Add a pointer so readers know where the M-registry lives."
    )
