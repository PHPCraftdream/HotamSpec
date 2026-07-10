"""Canon: §Requirement — CLI for browsing, searching, patching and contextualizing Requirements.

Five subcommands share one theme -- "give the operator fast, graph-backed access
to the requirement roster without hand-grepping 100+KB generated docs or reading
raw graph.py source":

  list   [--status S] [--owner O] [--enforcement E]  -- compact table from live graph
  show   R-x [--json]                                 -- full requirement details
  search "text" [--json]                              -- case-insensitive id/claim/why search
  patch  R-x --set field=value [--why-append "text"]  -- UX sugar over apply_proposal
  context R-x [--depth N] [--json]                    -- agent-ready context package

This is a thin dispatcher (review.py/land.py precedent). All reads come from
the LIVE TensionGraph via load_content_graph(), never from generated markdown.
The `patch` subcommand assembles a full ProposedRequirement JSON and routes it
through the existing apply_proposal.apply() mechanism -- it is PURE UX sugar,
no new write path, no hand-edit (R-no-hand-edit-graph).

Run (from spec/):
  python tools/hotam_req.py list --status SETTLED
  python tools/hotam_req.py show R-anchor-everything
  python tools/hotam_req.py search "conflict"
  python tools/hotam_req.py patch R-anchor-everything --set enforcement=ENFORCED --dry-run
  python tools/hotam_req.py context R-no-hand-edit-graph --json
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

import _bootstrap  # noqa: E402,F401  -- side effect: configures sys.path for hotam_spec + tools

from hotam_spec.graph import TensionGraph, _load_graph_file, load_content_graph
from hotam_spec.requirement import Requirement

_SUBCOMMANDS = ("list", "show", "search", "patch", "context")


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _load_graph() -> TensionGraph:
    """Load the active domain's graph (live, not cached)."""
    return load_content_graph()


def _load_graph_from_file(path: Path) -> TensionGraph:
    """Load a graph from a specific file path (for testing / --content-graph)."""
    return _load_graph_file(path)


def _find_requirement(g: TensionGraph, req_id: str) -> Requirement | None:
    """Return the Requirement with the given id, or None."""
    for r in g.requirements:
        if r.id == req_id:
            return r
    return None


def _enforcement_display(r: Requirement) -> str:
    """Short enforcement label."""
    return r.enforcement


def _status_display(r: Requirement) -> str:
    """Short status label (truncate long OPEN(...) questions for list view)."""
    s = r.status
    if s.startswith("OPEN(") and len(s) > 25:
        return s[:22] + "...)"
    return s


# ---------------------------------------------------------------------------
# list
# ---------------------------------------------------------------------------


def do_list(argv: list[str]) -> int:
    """List requirements: compact table of id / status / enforcement / owner."""
    parser = argparse.ArgumentParser(
        prog="hotam_req.py list",
        description="List requirements from the live graph.",
    )
    parser.add_argument("--status", default=None, help="Filter by status (SETTLED, DRAFT, OPEN, REJECTED).")
    parser.add_argument("--owner", default=None, help="Filter by owner (stakeholder id).")
    parser.add_argument("--enforcement", default=None, help="Filter by enforcement (PROSE, STRUCTURAL, ENFORCED).")
    args = parser.parse_args(argv)

    g = _load_graph()
    rows: list[Requirement] = []
    for r in g.requirements:
        if args.status is not None:
            target = args.status.upper()
            if target == "OPEN":
                if not r.status.startswith("OPEN"):
                    continue
            elif r.status.upper() != target:
                continue
        if args.owner is not None and r.owner != args.owner:
            continue
        if args.enforcement is not None and r.enforcement.upper() != args.enforcement.upper():
            continue
        rows.append(r)

    if not rows:
        print("(no matching requirements)")
        return 0

    # Column widths
    id_w = max(len(r.id) for r in rows)
    st_w = max(len(_status_display(r)) for r in rows)
    en_w = max(len(_enforcement_display(r)) for r in rows)
    ow_w = max(len(r.owner) for r in rows)
    id_w = max(id_w, 2)
    st_w = max(st_w, 6)
    en_w = max(en_w, 11)
    ow_w = max(ow_w, 5)

    header = f"{'ID':<{id_w}}  {'STATUS':<{st_w}}  {'ENFORCEMENT':<{en_w}}  {'OWNER':<{ow_w}}"
    print(header)
    print("-" * len(header))
    for r in rows:
        print(f"{r.id:<{id_w}}  {_status_display(r):<{st_w}}  {_enforcement_display(r):<{en_w}}  {r.owner:<{ow_w}}")
    print(f"\n({len(rows)} requirement(s))")
    return 0


# ---------------------------------------------------------------------------
# show
# ---------------------------------------------------------------------------


def _requirement_to_dict(r: Requirement) -> dict:
    """Serialize a Requirement to a plain dict (JSON-friendly)."""
    return {
        "id": r.id,
        "claim": r.claim,
        "owner": r.owner,
        "status": r.status,
        "why": r.why,
        "enforcement": r.enforcement,
        "enforceability": r.enforceability,
        "enforced_by": list(r.enforced_by),
        "assumptions": list(r.assumptions),
        "relations": [{"kind": rel.kind, "target": rel.target} for rel in r.relations],
        "m_tag": r.m_tag,
        "summary": r.summary,
        "created_at": r.created_at,
        "settled_at": r.settled_at,
    }


def do_show(argv: list[str]) -> int:
    """Show full details for a single requirement."""
    parser = argparse.ArgumentParser(
        prog="hotam_req.py show",
        description="Show a single requirement from the live graph.",
    )
    parser.add_argument("req_id", help="Requirement id (e.g. R-anchor-everything).")
    parser.add_argument("--json", dest="as_json", action="store_true", help="Machine-readable JSON output.")
    args = parser.parse_args(argv)

    g = _load_graph()
    r = _find_requirement(g, args.req_id)
    if r is None:
        print(f"error: requirement '{args.req_id}' not found in the active graph.", file=sys.stderr)
        return 1

    if args.as_json:
        print(json.dumps(_requirement_to_dict(r), indent=2, ensure_ascii=False))
        return 0

    # Human-readable
    print(f"id           : {r.id}")
    print(f"claim        : {r.claim}")
    print(f"owner        : {r.owner}")
    print(f"status       : {r.status}")
    print(f"enforcement  : {r.enforcement}")
    print(f"enforceability: {r.enforceability}")
    if r.enforced_by:
        print(f"enforced_by  : {', '.join(r.enforced_by)}")
    if r.why:
        print(f"why          : {r.why}")
    if r.assumptions:
        print(f"assumptions  : {', '.join(r.assumptions)}")
    if r.relations:
        for rel in r.relations:
            print(f"relation     : {rel.kind} -> {rel.target}")
    if r.m_tag:
        print(f"m_tag        : {r.m_tag}")
    if r.summary:
        print(f"summary      : {r.summary}")
    if r.created_at:
        print(f"created_at   : {r.created_at}")
    if r.settled_at:
        print(f"settled_at   : {r.settled_at}")
    return 0


# ---------------------------------------------------------------------------
# search
# ---------------------------------------------------------------------------


def do_search(argv: list[str]) -> int:
    """Case-insensitive search across id, claim, and why fields."""
    parser = argparse.ArgumentParser(
        prog="hotam_req.py search",
        description="Search requirements by text (id / claim / why).",
    )
    parser.add_argument("query", help="Search text (case-insensitive).")
    parser.add_argument("--json", dest="as_json", action="store_true", help="Machine-readable JSON output.")
    args = parser.parse_args(argv)

    g = _load_graph()
    q = args.query.lower()
    hits: list[Requirement] = []
    for r in g.requirements:
        if q in r.id.lower() or q in r.claim.lower() or q in r.why.lower():
            hits.append(r)

    if args.as_json:
        results = []
        for r in hits:
            results.append({
                "id": r.id,
                "status": r.status,
                "claim": r.claim[:200],
                "owner": r.owner,
            })
        print(json.dumps(results, indent=2, ensure_ascii=False))
        return 0

    if not hits:
        print(f"(no requirements matching '{args.query}')")
        return 0

    for r in hits:
        claim_short = r.claim[:100] + ("..." if len(r.claim) > 100 else "")
        print(f"{r.id}  [{r.status}]  {claim_short}")
    print(f"\n({len(hits)} match(es))")
    return 0


# ---------------------------------------------------------------------------
# patch
# ---------------------------------------------------------------------------


_PATCHABLE_FIELDS = frozenset({
    "claim", "owner", "status", "why", "enforcement", "enforceability",
    "m_tag", "summary",
})

_LIST_FIELDS = frozenset({
    "assumptions", "enforced_by",
})


def _parse_set_pair(raw: str) -> tuple[str, str]:
    """Parse 'field=value' into (field, value). Raises ValueError on bad syntax."""
    if "=" not in raw:
        raise ValueError(f"--set expects 'field=value', got '{raw}'")
    key, _, val = raw.partition("=")
    key = key.strip()
    val = val.strip()
    if not key:
        raise ValueError(f"--set field name is empty in '{raw}'")
    return key, val


def do_patch(argv: list[str]) -> int:
    """Patch a requirement: read current state, apply --set overrides, run apply_proposal."""
    parser = argparse.ArgumentParser(
        prog="hotam_req.py patch",
        description="Patch a requirement via apply_proposal (UX sugar, no new write path).",
    )
    parser.add_argument("req_id", help="Requirement id (e.g. R-anchor-everything).")
    parser.add_argument(
        "--set", dest="sets", action="append", default=[],
        metavar="field=value",
        help="Set a field value. Repeatable. Fields: claim, owner, status, why, enforcement, enforceability, m_tag, summary, assumptions, enforced_by.",
    )
    parser.add_argument(
        "--why-append", default=None,
        help="Append text to the existing 'why' (does NOT replace it).",
    )
    parser.add_argument("--dry-run", action="store_true", help="Show diff without writing.")
    parser.add_argument(
        "--content-graph", default=None,
        help="Override the graph.py path (for testing).",
    )
    args = parser.parse_args(argv)

    if not args.sets and args.why_append is None:
        print("error: at least one --set or --why-append is required.", file=sys.stderr)
        return 2

    # Load graph (or override path for testing)
    content_graph_path = Path(args.content_graph) if args.content_graph else None
    if content_graph_path is not None:
        g = _load_graph_from_file(content_graph_path)
    else:
        g = _load_graph()
    r = _find_requirement(g, args.req_id)
    if r is None:
        print(f"error: requirement '{args.req_id}' not found in the active graph.", file=sys.stderr)
        return 1

    # Start from current values
    proposal_dict: dict = {
        "kind": "Requirement",
        "id": r.id,
        "claim": r.claim,
        "owner": r.owner,
        "status": r.status,
        "why": r.why,
        "assumptions": list(r.assumptions),
        "relations": [[rel.kind, rel.target] for rel in r.relations],
        "enforcement": r.enforcement,
        "enforced_by": list(r.enforced_by),
        "m_tag": r.m_tag,
        "enforceability": r.enforceability,
        "summary": r.summary,
        "created_at": r.created_at,
        "settled_at": r.settled_at,
    }

    # Apply --set overrides
    for raw_set in args.sets:
        try:
            field, value = _parse_set_pair(raw_set)
        except ValueError as e:
            print(f"error: {e}", file=sys.stderr)
            return 2
        if field in _PATCHABLE_FIELDS:
            proposal_dict[field] = value
        elif field in _LIST_FIELDS:
            # Accept comma-separated list
            proposal_dict[field] = [v.strip() for v in value.split(",") if v.strip()]
        elif field == "relations":
            # Accept kind:target,kind:target format
            rels = []
            for pair in value.split(","):
                pair = pair.strip()
                if ":" not in pair:
                    print(f"error: relations format is 'kind:target[,kind:target,...]', got '{pair}'", file=sys.stderr)
                    return 2
                k, _, t = pair.partition(":")
                rels.append([k.strip(), t.strip()])
            proposal_dict["relations"] = rels
        else:
            print(f"error: unknown patchable field '{field}'. Allowed: {', '.join(sorted(_PATCHABLE_FIELDS | _LIST_FIELDS | {'relations'}))}.", file=sys.stderr)
            return 2

    # Apply --why-append
    if args.why_append is not None:
        existing_why = proposal_dict["why"]
        if existing_why:
            proposal_dict["why"] = existing_why + " " + args.why_append
        else:
            proposal_dict["why"] = args.why_append

    # Route through apply_proposal
    import apply_proposal  # noqa: PLC0415  -- lives in tools/, not a package

    try:
        proposal = apply_proposal._validate_requirement(proposal_dict)
    except ValueError as e:
        print(f"error: invalid proposal: {e}", file=sys.stderr)
        return 1

    return apply_proposal.apply(
        proposal,
        dry_run=args.dry_run,
        content_graph=content_graph_path,
    )


# ---------------------------------------------------------------------------
# context
# ---------------------------------------------------------------------------


def do_context(argv: list[str]) -> int:
    """Build an agent-ready context package for a requirement."""
    parser = argparse.ArgumentParser(
        prog="hotam_req.py context",
        description="Agent-ready context package: requirement + owner + assumptions + relations + conflicts + enforcement.",
    )
    parser.add_argument("req_id", help="Requirement id (e.g. R-no-hand-edit-graph).")
    parser.add_argument("--depth", type=int, default=1, help="Relation traversal depth (default 1, reserved).")
    parser.add_argument("--json", dest="as_json", action="store_true", help="Machine-readable JSON output.")
    args = parser.parse_args(argv)

    g = _load_graph()
    r = _find_requirement(g, args.req_id)
    if r is None:
        print(f"error: requirement '{args.req_id}' not found in the active graph.", file=sys.stderr)
        return 1

    # Build context package
    ctx: dict = {"requirement": _requirement_to_dict(r)}

    # Owner stakeholder
    owner_sh = None
    for sh in g.stakeholders:
        if sh.id == r.owner:
            owner_sh = sh
            break
    if owner_sh is not None:
        ctx["owner_stakeholder"] = {
            "id": owner_sh.id,
            "name": owner_sh.name,
            "domain": owner_sh.domain,
        }

    # Assumptions with statuses
    if r.assumptions:
        a_list = []
        for a_id in r.assumptions:
            for a in g.assumptions:
                if a.id == a_id:
                    a_list.append({
                        "id": a.id,
                        "statement": a.statement,
                        "status": a.status,
                        "owner": a.owner,
                    })
                    break
        ctx["assumptions"] = a_list

    # Relations in BOTH directions
    outgoing = []
    for rel in r.relations:
        outgoing.append({"kind": rel.kind, "target": rel.target})
    incoming = []
    for other in g.requirements:
        if other.id == r.id:
            continue
        for rel in other.relations:
            if rel.target == r.id:
                incoming.append({"kind": rel.kind, "from": other.id})
    if outgoing:
        ctx["relations_outgoing"] = outgoing
    if incoming:
        ctx["relations_incoming"] = incoming

    # Conflicts where this requirement is a member
    member_conflicts = []
    for c in g.conflicts:
        if r.id in c.members:
            member_conflicts.append({
                "id": c.id,
                "axis": c.axis,
                "context": c.context,
                "lifecycle": c.lifecycle,
                "steward": c.steward,
                "members": list(c.members),
            })
    if member_conflicts:
        ctx["conflicts"] = member_conflicts

    # Enforcement details
    ctx["enforcement"] = {
        "level": r.enforcement,
        "enforceability": r.enforceability,
        "enforced_by": list(r.enforced_by),
        "is_closeable_debt": r.is_closeable_debt(),
    }

    if args.as_json:
        print(json.dumps(ctx, indent=2, ensure_ascii=False))
        return 0

    # Human-readable
    print(f"=== Context for {r.id} ===")
    print(f"claim        : {r.claim}")
    print(f"owner        : {r.owner}", end="")
    if owner_sh:
        print(f" ({owner_sh.name}, {owner_sh.domain})")
    else:
        print()
    print(f"status       : {r.status}")
    print(f"enforcement  : {r.enforcement} ({r.enforceability})")
    if r.enforced_by:
        print(f"enforced_by  : {', '.join(r.enforced_by)}")
    if r.why:
        print(f"why          : {r.why}")

    if r.assumptions:
        print(f"\n--- Assumptions ({len(ctx.get('assumptions', []))} found) ---")
        for a in ctx.get("assumptions", []):
            print(f"  {a['id']}  [{a['status']}]  {a['statement'][:80]}")

    if outgoing:
        print(f"\n--- Outgoing relations ---")
        for rel in outgoing:
            print(f"  {rel['kind']} -> {rel['target']}")
    if incoming:
        print(f"\n--- Incoming relations ---")
        for rel in incoming:
            print(f"  {rel['kind']} <- {rel['from']}")

    if member_conflicts:
        print(f"\n--- Conflicts ({len(member_conflicts)}) ---")
        for c in member_conflicts:
            print(f"  {c['id']}  [{c['lifecycle']}]  axis={c['axis']}  steward={c['steward']}")

    return 0


# ---------------------------------------------------------------------------
# Dispatcher (review.py / land.py precedent)
# ---------------------------------------------------------------------------

_DISPATCH = {
    "list": do_list,
    "show": do_show,
    "search": do_search,
    "patch": do_patch,
    "context": do_context,
}


def main(argv: list[str] | None = None) -> int:
    """Canon: §Requirement — dispatch to list / show / search / patch / context."""
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    raw = sys.argv[1:] if argv is None else list(argv)

    if not raw or raw[0] in ("-h", "--help"):
        print("usage: hotam_req.py <subcommand> [args]")
        print("subcommands: " + ", ".join(_SUBCOMMANDS))
        print()
        print("  list   [--status S] [--owner O] [--enforcement E]  compact table from live graph")
        print("  show   R-x [--json]                                full requirement details")
        print("  search \"text\" [--json]                              case-insensitive search")
        print("  patch  R-x --set field=value [--why-append TEXT]    patch via apply_proposal")
        print("  context R-x [--depth N] [--json]                   agent-ready context package")
        return 0 if raw and raw[0] in ("-h", "--help") else 2

    sub, rest = raw[0], raw[1:]
    handler = _DISPATCH.get(sub)
    if handler is None:
        print(f"error: unknown subcommand '{sub}'", file=sys.stderr)
        print("available: " + ", ".join(_SUBCOMMANDS), file=sys.stderr)
        return 2
    return handler(rest)


if __name__ == "__main__":
    sys.exit(main())
