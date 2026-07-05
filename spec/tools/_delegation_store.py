"""Shared on-disk delegation engine: frontmatter parsing + mutation (private helper).

Canon: §Ticket (sibling pattern) — file-based delegation tracking.

RULE: delegations live in `delegations/DG-<n>.md` at the REPO ROOT (flat, no
status subfolders). Status is a field in the JSON header, not a folder.
WHY flat: delegations are simpler than tickets — only two states (open/done),
no kanban columns needed. The steward's verdict (2026-07-05): "делегировать все
задачи через файлы, и вести их историю в гите" — git history on the file IS
the audit trail, so the mechanism is deliberately minimal.

DESIGN DECISIONS mirror _ticket_store.py:
- FRONTMATTER: fenced ```json block (not HTML sentinels) per the steward's spec.
- HISTORY: ## Result section filled on close (not append-only History lines).
- STDLIB-ONLY (R-core-imports-stdlib-or-hotam-spec-only).
"""

from __future__ import annotations

import datetime as _dt
import json
import re
from dataclasses import dataclass
from pathlib import Path

# --- layout -----------------------------------------------------------------

REPO_ROOT = Path(__file__).resolve().parents[2]
DELEGATIONS_DIR = REPO_ROOT / "delegations"

VALID_STATUSES: tuple[str, ...] = ("open", "done")

_ID_RE = re.compile(r"^DG-(\d+)$")

_HEADER_RE = re.compile(
    r"^```json\s*\n(.*?)\n```", re.DOTALL
)


# --- value type -------------------------------------------------------------


@dataclass
class Delegation:
    """A parsed delegation: JSON header + markdown body, aware of its path."""

    header: dict
    body: str
    path: Path

    @property
    def id(self) -> str:
        return self.header["id"]

    @property
    def status(self) -> str:
        return self.header["status"]


# --- helpers ----------------------------------------------------------------


def now_stamp() -> str:
    """ISO-8601 UTC wall-clock stamp."""
    return _dt.datetime.now(_dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def ensure_layout() -> None:
    """Create delegations/ directory (idempotent)."""
    DELEGATIONS_DIR.mkdir(parents=True, exist_ok=True)


def _split_frontmatter(text: str) -> tuple[dict, str]:
    """Parse ```json header``` + body -> (header_dict, body_str)."""
    m = _HEADER_RE.match(text)
    if not m:
        raise ValueError("delegation file missing ```json header")
    header = json.loads(m.group(1))
    body = text[m.end():].lstrip("\n")
    return header, body


def _render(header: dict, body: str) -> str:
    """Render (header, body) back to the on-disk string."""
    hjson = json.dumps(header, indent=2, ensure_ascii=False)
    body = body.rstrip("\n") + "\n"
    return f"```json\n{hjson}\n```\n\n{body}"


def find_path(delegation_id: str) -> Path | None:
    """Locate a delegation file by id, or None."""
    p = DELEGATIONS_DIR / f"{delegation_id}.md"
    return p if p.exists() else None


def load(delegation_id: str) -> Delegation:
    path = find_path(delegation_id)
    if path is None:
        raise FileNotFoundError(f"no delegation {delegation_id!r} in delegations/")
    header, body = _split_frontmatter(path.read_text(encoding="utf-8"))
    return Delegation(header=header, body=body, path=path)


def save(delegation: Delegation) -> None:
    delegation.path.write_text(
        _render(delegation.header, delegation.body), encoding="utf-8"
    )


def all_ids() -> list[str]:
    """Every DG-<n> id present on disk."""
    ids: list[str] = []
    if not DELEGATIONS_DIR.exists():
        return ids
    for p in DELEGATIONS_DIR.glob("DG-*.md"):
        if _ID_RE.match(p.stem):
            ids.append(p.stem)
    return ids


def next_id() -> str:
    """Smallest-free DG-<n>: max existing numeric suffix + 1."""
    nums = [int(m.group(1)) for i in all_ids() if (m := _ID_RE.match(i))]
    return f"DG-{(max(nums) + 1) if nums else 1}"


def new_delegation(
    *,
    delegation_id: str,
    date: str,
    from_: str,
    to: str,
    task: str,
    boundaries: str,
    expected_return: str,
) -> Delegation:
    """Build an in-memory Delegation with initial body scaffold."""
    header = {
        "id": delegation_id,
        "date": date,
        "from": from_,
        "to": to,
        "task": task,
        "boundaries": boundaries,
        "expected_return": expected_return,
        "status": "open",
        "result": "",
        "result_commit": "",
    }
    body = (
        f"## Task\n\n{task}\n\n"
        f"## Result\n\n_(pending)_\n"
    )
    path = DELEGATIONS_DIR / f"{delegation_id}.md"
    return Delegation(header=header, body=body, path=path)
