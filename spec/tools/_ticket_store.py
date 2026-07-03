"""Shared on-disk ticket engine: frontmatter parsing + mutation + History (private helper).

This is a `_`-prefixed helper (NOT projected to an R-tool-* requirement by
gen_spec.py — leading-underscore files are skipped). It carries the load-bearing
mechanics that every ticket_*.py tool delegates to, so the encapsulation
demanded by the steward's verdict ("создание тикета, движение по статусам,
авто-история должно быть инкапсулировано в инструментах") lives in exactly one
place instead of being copy-pasted across six CLIs.

DESIGN DECISIONS (RULE / WHY):

- LOCATION: tickets live in `tickets/<status>/T-<n>.md` at the REPO ROOT, not
  under spec/.runtime/. WHY: .runtime/ is git-ignored ephemera
  (R-task-spawn-log-runtime); tickets are durable work items that must survive
  and be reviewed across sessions, so they are committed substrate. Status is
  encoded by the SUBFOLDER the file sits in — moving between statuses is a file
  move, giving a status change a visible git-level footprint.

- STATUS SET: backlog / in-progress / review / done / blocked. WHY these five:
  they are the minimal columns of a working kanban — a queue (backlog), active
  work (in-progress), a hand-off gate (review), a terminal (done), and an
  off-flow park (blocked). Fewer would collapse review or blocked into prose;
  more would invent ceremony the single-human steward does not yet need.

- FRONTMATTER FORMAT: a JSON object between two sentinel lines
  (`<!-- ticket:begin -->` / `<!-- ticket:end -->`), followed by the Markdown
  body. WHY JSON-between-sentinels and NOT YAML: the framework core is stdlib-
  only (R-core-imports-stdlib-or-hotam-spec-only) and ships no YAML parser;
  hand-rolling a YAML subset parser is a silent-bug farm. `json` is in the
  stdlib, is exact, and round-trips losslessly. The sentinels keep the machine
  header unambiguously separate from the human body so a parser never has to
  guess where the frontmatter ends.

- HISTORY IS APPEND-ONLY AND TOOL-WRITTEN: every mutation appends one line to the
  `## History` section in a fixed, machine-recognisable format
  (`- <stamp> · <actor> · <action> · <detail>`). Hand-editing the header or
  History is out-of-contract (R-ticket-mutation-via-tools-only); the tools are
  the only writers. `text changed` history entries snapshot the PRIOR title/body
  so the edit trail the steward asked for ("история изменения текста") is never
  lost.

- TIME STAMPS: mutations are run by a human/agent in real wall-clock time, so an
  ISO-8601 UTC stamp is written (like spec/.runtime/spawn-log.jsonl /
  land-log.jsonl). Tickets commit with real time; tests therefore assert on
  STRUCTURE (a History line was appended, its action/detail) and never compare
  the timestamp byte-for-byte.
"""

from __future__ import annotations

import datetime as _dt
import json
import re
from dataclasses import dataclass
from pathlib import Path

# --- layout -----------------------------------------------------------------

REPO_ROOT = Path(__file__).resolve().parents[2]
TICKETS_DIR = REPO_ROOT / "tickets"

STATUSES: tuple[str, ...] = ("backlog", "in-progress", "review", "done", "blocked")
"""The canonical status columns (= subfolders under tickets/)."""

INITIAL_STATUS = "backlog"

BEGIN_SENTINEL = "<!-- ticket:begin -->"
END_SENTINEL = "<!-- ticket:end -->"

HISTORY_HEADING = "## History"
COMMENTS_HEADING = "## Comments"

_ID_RE = re.compile(r"^T-(\d+)$")
_HISTORY_LINE_RE = re.compile(
    r"^- (?P<stamp>\S+) · (?P<actor>[^·]+?) · (?P<action>[^·]+?)(?: · (?P<detail>.*))?$"
)

LINK_PREFIXES = ("R-", "C-", "A-", "DEL-", "OP-", "GOAL-", "T-")
"""Anchor prefixes a ticket's `links` may reference (R-anchor-everything)."""


# --- value type -------------------------------------------------------------


@dataclass
class Ticket:
    """A parsed ticket: machine header + human body, aware of its on-disk path."""

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
    """ISO-8601 UTC wall-clock stamp (real time; tests assert structure, not value)."""
    return _dt.datetime.now(_dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def status_dir(status: str) -> Path:
    if status not in STATUSES:
        raise ValueError(f"unknown status {status!r}; known: {', '.join(STATUSES)}")
    return TICKETS_DIR / status


def ensure_layout() -> None:
    """Create tickets/ and every status subfolder (idempotent)."""
    for s in STATUSES:
        (TICKETS_DIR / s).mkdir(parents=True, exist_ok=True)


def _split_frontmatter(text: str) -> tuple[dict, str]:
    """Parse `<!-- ticket:begin -->{json}<!-- ticket:end -->\\n<body>` → (header, body)."""
    if not text.startswith(BEGIN_SENTINEL):
        raise ValueError("ticket file missing begin sentinel")
    end = text.find(END_SENTINEL)
    if end == -1:
        raise ValueError("ticket file missing end sentinel")
    raw = text[len(BEGIN_SENTINEL) : end].strip()
    header = json.loads(raw)
    body = text[end + len(END_SENTINEL) :].lstrip("\n")
    return header, body


def _render(header: dict, body: str) -> str:
    """Render (header, body) back to the on-disk string (deterministic key order)."""
    hjson = json.dumps(header, indent=2, ensure_ascii=False, sort_keys=True)
    body = body.rstrip("\n") + "\n"
    return f"{BEGIN_SENTINEL}\n{hjson}\n{END_SENTINEL}\n\n{body}"


def find_path(ticket_id: str) -> Path | None:
    """Locate a ticket file by id across all status folders, or None."""
    for s in STATUSES:
        p = TICKETS_DIR / s / f"{ticket_id}.md"
        if p.exists():
            return p
    return None


def load(ticket_id: str) -> Ticket:
    path = find_path(ticket_id)
    if path is None:
        raise FileNotFoundError(f"no ticket {ticket_id!r} in tickets/")
    header, body = _split_frontmatter(path.read_text(encoding="utf-8"))
    return Ticket(header=header, body=body, path=path)


def save(ticket: Ticket) -> None:
    ticket.path.write_text(_render(ticket.header, ticket.body), encoding="utf-8")


def all_ids() -> list[str]:
    """Every T-<n> id present on disk (any status)."""
    ids: list[str] = []
    for s in STATUSES:
        d = TICKETS_DIR / s
        if not d.exists():
            continue
        for p in d.glob("T-*.md"):
            if _ID_RE.match(p.stem):
                ids.append(p.stem)
    return ids


def next_id() -> str:
    """Smallest-free T-<n>: max existing numeric suffix + 1 (starts at T-1)."""
    nums = [int(m.group(1)) for i in all_ids() if (m := _ID_RE.match(i))]
    return f"T-{(max(nums) + 1) if nums else 1}"


# --- section (History / Comments) mutation ----------------------------------


def append_history(ticket: Ticket, *, actor: str, action: str, detail: str = "") -> None:
    """Append one machine-recognisable History line. The ONLY History writer."""
    line = f"- {now_stamp()} · {actor} · {action}"
    if detail:
        line += f" · {detail}"
    ticket.body = _append_under_heading(ticket.body, HISTORY_HEADING, line)


def append_comment(ticket: Ticket, *, actor: str, text: str) -> None:
    """Append a stamped comment under ## Comments."""
    line = f"- {now_stamp()} · {actor}: {text}"
    ticket.body = _append_under_heading(ticket.body, COMMENTS_HEADING, line)


def _append_under_heading(body: str, heading: str, line: str) -> str:
    """Append `line` at the end of the block owned by `heading`, creating it if absent."""
    lines = body.rstrip("\n").split("\n")
    # Match the LAST occurrence: the real section scaffold (## Comments / ## History)
    # is always emitted AFTER the description by new_ticket(), so a literal heading
    # inside the human description must never capture appended machine lines.
    idx = next(
        (i for i in range(len(lines) - 1, -1, -1) if lines[i].strip() == heading),
        None,
    )
    if idx is None:
        return body.rstrip("\n") + f"\n\n{heading}\n\n{line}\n"
    # find end of this section = next '## ' heading or EOF
    end = len(lines)
    for j in range(idx + 1, len(lines)):
        if lines[j].startswith("## "):
            end = j
            break
    block = lines[:end]
    while block and block[-1].strip() == "":
        block.pop()
    block.append(line)
    return "\n".join(block + [""] + lines[end:]).rstrip("\n") + "\n"


def parse_history(ticket: Ticket) -> list[dict]:
    """Structured view of the History lines (for tests / ticket_show)."""
    out: list[dict] = []
    lines = ticket.body.split("\n")
    # Read the LAST ## History block (the real scaffold section), consistent with
    # _append_under_heading, so a literal '## History' in the description is ignored.
    start = next(
        (i for i in range(len(lines) - 1, -1, -1) if lines[i].strip() == HISTORY_HEADING),
        None,
    )
    if start is None:
        return out
    for ln in lines[start + 1 :]:
        if ln.startswith("## "):
            break
        if (m := _HISTORY_LINE_RE.match(ln)):
            out.append(
                {
                    "stamp": m.group("stamp"),
                    "actor": m.group("actor").strip(),
                    "action": m.group("action").strip(),
                    "detail": (m.group("detail") or "").strip(),
                }
            )
    return out


def new_ticket(
    *, ticket_id: str, title: str, assignee: str, links: list[str], description: str
) -> Ticket:
    """Build an in-memory Ticket with the initial body scaffold (no History yet)."""
    header = {
        "id": ticket_id,
        "title": title,
        "assignee": assignee,
        "status": INITIAL_STATUS,
        "created": now_stamp(),
        "updated": now_stamp(),
        "links": links,
    }
    body = (
        f"# {title}\n\n"
        f"{description.rstrip() or '_(no description)_'}\n\n"
        f"{COMMENTS_HEADING}\n\n"
        f"{HISTORY_HEADING}\n"
    )
    path = status_dir(INITIAL_STATUS) / f"{ticket_id}.md"
    return Ticket(header=header, body=body, path=path)


def counts_by_status() -> dict[str, int]:
    """Number of tickets per status folder (for the what_now pulse band)."""
    out: dict[str, int] = {}
    for s in STATUSES:
        d = TICKETS_DIR / s
        out[s] = len(list(d.glob("T-*.md"))) if d.exists() else 0
    return out
