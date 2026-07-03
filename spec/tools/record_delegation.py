"""Canon: §Stakeholder — records a new steward delegation into the active domain's
durable delegations.jsonl registry (R-trust-anchor-delegation-explicit-only).

WHY a committed file, not spec/.runtime/ ephemera: a delegation IS a trust-anchor
signature — the steward's own explicit act of handing decision authority to the
agent, per-case or for a declared campaign, "never implied or standing by default"
(R-trust-anchor-delegation-explicit-only). Trust-anchor signatures belong in
versioned substrate next to the graph they authorize decisions on, exactly like
domains/*/graph.py itself, NOT in spec/.runtime/ (gitignored, ephemeral tooling
output — R-task-spawn-log-runtime's directory choice does not apply here: that
log records TOOL invocations, this file records HUMAN authorization acts).

Record shape (one JSON object per line, append-only, never rewritten):
  {"id": "DEL-<n>", "steward": "<Stakeholder id>", "verbatim": "<exact wording>",
   "date": "YYYY-MM-DD", "scope": "<campaign/case>"}

RULE: id is auto-incremented (DEL-1, DEL-2, ...) from the highest existing id
in the file — never caller-supplied, so ids stay a stable, gapless append log.
steward MUST be an existing Stakeholder id in the active domain's graph.
verbatim and scope MUST be non-empty (an unlabeled or unscoped delegation
cannot be resolved back to a specific human act — R-speak-by-reference).
date defaults to today (ISO 8601, local) when omitted.

Usage:
  uv run python tools/record_delegation.py --steward domain-user \\
      --verbatim "exact wording of the delegation" \\
      --scope "campaign: description" [--date 2026-07-02]

Exit codes:
  0 — success (record appended).
  1 — failure (validation error, e.g. unknown steward, empty verbatim/scope).
"""

from __future__ import annotations

import argparse
import datetime as _dt
import json
import re
import sys
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_SRC = _SPEC_ROOT / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from hotam_spec.graph import TensionGraph, load_content_graph  # noqa: E402

_DOMAINS_ROOT = _SPEC_ROOT.parent / "domains"
_ID_RE = re.compile(r"^DEL-(\d+)$")


def _delegations_path(domain_dir: Path | None = None) -> Path:
    """Canon: §Stakeholder — resolve the active domain's delegations.jsonl path.

    Mirrors apply_proposal.py's _resolve_content_graph domain-selection intent
    but simplified: this tool operates against the SAME active domain the rest
    of the LAND pipeline targets, one directory up from graph.py.
    """
    if domain_dir is not None:
        return domain_dir / "delegations.jsonl"
    from hotam_spec.graph import _active_domain_graph_file  # noqa: PLC0415

    graph_file = _active_domain_graph_file()
    if graph_file is not None:
        return graph_file.parent / "delegations.jsonl"
    return _SPEC_ROOT / "content" / "delegations.jsonl"


def _read_records(path: Path) -> list[dict]:
    if not path.exists():
        return []
    out: list[dict] = []
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        out.append(json.loads(line))
    return out


def _next_id(records: list[dict]) -> str:
    """Canon: §Stakeholder — DEL-<n+1> where n is the highest existing DEL-<n>."""
    max_n = 0
    for rec in records:
        m = _ID_RE.match(rec.get("id", ""))
        if m:
            max_n = max(max_n, int(m.group(1)))
    return f"DEL-{max_n + 1}"


def record_delegation(
    *,
    steward: str,
    verbatim: str,
    scope: str,
    date: str = "",
    graph: TensionGraph | None = None,
    delegations_path: Path | None = None,
) -> int:
    """Validate and append a new delegation record. Returns exit code."""
    steward = steward.strip()
    if not steward:
        print("ERROR: --steward is required and must be non-empty.", file=sys.stderr)
        return 1

    g = graph if graph is not None else load_content_graph()
    stakeholder_ids = {s.id for s in g.stakeholders}
    if stakeholder_ids and steward not in stakeholder_ids:
        print(
            f"ERROR: steward '{steward}' is not a declared Stakeholder id in the "
            f"active domain's graph. Declared: {sorted(stakeholder_ids)}.",
            file=sys.stderr,
        )
        return 1

    verbatim = verbatim.strip()
    if not verbatim:
        print("ERROR: --verbatim is required and must be non-empty.", file=sys.stderr)
        return 1

    scope = scope.strip()
    if not scope:
        print("ERROR: --scope is required and must be non-empty.", file=sys.stderr)
        return 1

    if not date:
        date = _dt.date.today().isoformat()

    path = delegations_path if delegations_path is not None else _delegations_path()
    records = _read_records(path)
    new_id = _next_id(records)

    entry = {
        "id": new_id,
        "steward": steward,
        "verbatim": verbatim,
        "date": date,
        "scope": scope,
    }

    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8", newline="\n") as fh:
        fh.write(json.dumps(entry, ensure_ascii=False) + "\n")

    print(f"Recorded: {new_id} -> {path}")
    return 0


def main(argv: list[str] | None = None) -> int:
    """CLI entry point for record_delegation.py."""
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(
        description=(
            "Record a new steward delegation into the active domain's "
            "delegations.jsonl (R-trust-anchor-delegation-explicit-only)."
        )
    )
    parser.add_argument(
        "--steward",
        required=True,
        help="Stakeholder id of the steward granting the delegation.",
    )
    parser.add_argument(
        "--verbatim",
        required=True,
        help="Exact wording of the delegation.",
    )
    parser.add_argument(
        "--scope",
        required=True,
        help="Campaign or case this delegation applies to.",
    )
    parser.add_argument(
        "--date",
        default="",
        help="ISO 8601 date (default: today).",
    )
    args = parser.parse_args(argv)

    return record_delegation(
        steward=args.steward,
        verbatim=args.verbatim,
        scope=args.scope,
        date=args.date,
    )


if __name__ == "__main__":
    sys.exit(main())
