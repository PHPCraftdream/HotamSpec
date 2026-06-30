"""Canon: §Proposal — mechanical writer for steward-approved JSON proposals.

Reads a steward-approved JSON proposal from a file path argument, validates it
against the proposal shape (ProposedConflictTransition, ProposedRequirement, or
ProposedRejection), locates the target node in spec/content/graph.py via AST,
applies the field changes via deterministic string replacement, regenerates docs
via gen_spec.py, and runs pytest -q to verify the change is structurally clean.
Optionally runs the P4 closure check to confirm the triggering diagnosis was
actually removed.

This is the FIRST OPERATOR ACTION TOOL: the AI operator emits a proposal
(see tensio/proposal.py); the steward approves out-of-band; then the AI calls
this tool to mechanically land the change. No free-text editing of the graph.

Supported proposal kinds:
  - ConflictTransition — move a Conflict lifecycle (DETECTED → DECIDED etc.)
  - Requirement — add or update a Requirement in the graph
  - Rejection — reject an existing Requirement (status → REJECTED)

Usage:
  uv run python tools/apply_proposal.py proposal.json
  uv run python tools/apply_proposal.py --dry-run proposal.json
  uv run python tools/apply_proposal.py --triggering-kind CONFLICT_STALLED proposal.json
  uv run python tools/apply_proposal.py --batch proposals_array.json

The JSON shapes:

  ProposedConflictTransition DECIDED:
  {
    "kind": "ConflictTransition",
    "conflict_id": "C-8600b1b8",
    "new_lifecycle": "DECIDED(... rationale text ...)",
    "decided_by": "domain-user",
    "revisit_marker": "REVISIT if ...",
    "derived": ["R-foo"]
  }

  ProposedRequirement (add or update):
  {
    "kind": "Requirement",
    "id": "R-foo",
    "claim": "The system shall ...",
    "owner": "framework-author",
    "status": "DRAFT",
    "why": "...",
    "assumptions": ["A-python-stack"],
    "enforcement": "ENFORCED",
    "enforced_by": ["check_foo"]
  }

  ProposedRejection:
  {
    "kind": "Rejection",
    "requirement_id": "R-foo",
    "reason": "REJECTED — REPLACES R-bar; see R-new"
  }

Exit codes:
  0 — success (write landed, tests green, and if --triggering-kind was supplied:
      closure confirmed — the action is no longer in the post-apply diagnosis).
  1 — failure (validation error, missing id, or pytest red).
  2 — not advanced (write landed, tests green, but the triggering action STILL
      appears in the post-apply diagnosis — the tick (P5) must NOT count this
      as progress; investigate before marking closed).
"""

from __future__ import annotations

import argparse
import ast
import json
import subprocess
import sys
from pathlib import Path

# --- Make tensio importable --------------------------------------------------

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_SRC = _SPEC_ROOT / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from tensio.conflict import conflict_identity  # noqa: E402
from tensio.proposal import (  # noqa: E402
    Proposal,
    ProposedConflictTransition,
    ProposedRejection,
    ProposedRequirement,
)

_CONTENT_GRAPH = _SPEC_ROOT / "content" / "graph.py"
_GEN_SPEC = Path(__file__).resolve().parent / "gen_spec.py"

# ---------------------------------------------------------------------------
# Validation helpers
# ---------------------------------------------------------------------------


def _validate_proposal(raw: dict) -> Proposal:
    """Canon: §Proposal — parse and validate a JSON dict into a Proposal variant.

    RULE: 'kind' must be one of 'ConflictTransition', 'Requirement', or
    'Rejection'. Each kind has its own required fields.

    Returns a Proposal (one of the three dataclass variants) or raises
    ValueError with a clear message.
    """
    kind = raw.get("kind", "")
    if kind == "ConflictTransition":
        return _validate_conflict_transition(raw)
    if kind == "Requirement":
        return _validate_requirement(raw)
    if kind == "Rejection":
        return _validate_rejection(raw)
    raise ValueError(
        f"Unsupported proposal kind '{kind}'. "
        f"Supported: 'ConflictTransition', 'Requirement', 'Rejection'."
    )


def _validate_conflict_transition(raw: dict) -> ProposedConflictTransition:
    """Parse and validate a ConflictTransition proposal."""
    conflict_id = raw.get("conflict_id", "").strip()
    if not conflict_id:
        raise ValueError("'conflict_id' is required and must be non-empty.")
    new_lifecycle = raw.get("new_lifecycle", "").strip()
    if not new_lifecycle:
        raise ValueError("'new_lifecycle' is required and must be non-empty.")
    decided_by = raw.get("decided_by", "").strip()
    if new_lifecycle.startswith("DECIDED") and not decided_by:
        raise ValueError(
            "new_lifecycle starts with DECIDED but decided_by is empty. "
            "A DECIDED transition requires a human decider "
            "(R-decided-needs-human-signoff)."
        )
    revisit_marker = raw.get("revisit_marker", "")
    derived_raw = raw.get("derived", [])
    if not isinstance(derived_raw, list):
        raise ValueError("'derived' must be a list of R-id strings.")
    derived = tuple(str(x) for x in derived_raw)
    return ProposedConflictTransition(
        conflict_id=conflict_id,
        new_lifecycle=new_lifecycle,
        decided_by=decided_by,
        revisit_marker=revisit_marker if isinstance(revisit_marker, str) else "",
        derived=derived,
    )


def _validate_requirement(raw: dict) -> ProposedRequirement:
    """Parse and validate a Requirement proposal."""
    req_id = raw.get("id", "").strip()
    if not req_id:
        raise ValueError("'id' is required for a Requirement proposal.")
    claim = raw.get("claim", "").strip()
    if not claim:
        raise ValueError("'claim' is required and must be non-empty.")
    owner = raw.get("owner", "").strip()
    if not owner:
        raise ValueError("'owner' is required and must be non-empty.")
    status = raw.get("status", "").strip()
    if not status:
        raise ValueError("'status' is required and must be non-empty.")
    why = raw.get("why", "")
    assumptions_raw = raw.get("assumptions", [])
    if not isinstance(assumptions_raw, list):
        raise ValueError("'assumptions' must be a list of assumption id strings.")
    assumptions = tuple(str(x) for x in assumptions_raw)
    relations_raw = raw.get("relations", [])
    if not isinstance(relations_raw, list):
        raise ValueError("'relations' must be a list of [kind, target] pairs.")
    relations = tuple((str(r[0]), str(r[1])) for r in relations_raw)
    enforcement = raw.get("enforcement", "PROSE").strip()
    enforced_by_raw = raw.get("enforced_by", [])
    if not isinstance(enforced_by_raw, list):
        raise ValueError("'enforced_by' must be a list of strings.")
    enforced_by = tuple(str(x) for x in enforced_by_raw)
    m_tag = raw.get("m_tag", "")
    return ProposedRequirement(
        id=req_id,
        claim=claim,
        owner=owner,
        status=status,
        why=why if isinstance(why, str) else "",
        assumptions=assumptions,
        relations=relations,
        enforcement=enforcement,
        enforced_by=enforced_by,
        m_tag=m_tag if isinstance(m_tag, str) else "",
    )


def _validate_rejection(raw: dict) -> ProposedRejection:
    """Parse and validate a Rejection proposal."""
    requirement_id = raw.get("requirement_id", "").strip()
    if not requirement_id:
        raise ValueError("'requirement_id' is required for a Rejection proposal.")
    reason = raw.get("reason", "").strip()
    if not reason:
        raise ValueError("'reason' is required and must be non-empty.")
    return ProposedRejection(
        requirement_id=requirement_id,
        reason=reason,
    )


# ---------------------------------------------------------------------------
# AST-based requirement locator
# ---------------------------------------------------------------------------


def _find_requirement_call(tree: ast.AST, req_id: str) -> ast.Call | None:
    """Canon: §Proposal — locate the Requirement(...) AST call whose id matches.

    Walks the AST looking for ast.Call nodes whose function is 'Requirement'. For
    each, extracts the 'id' keyword arg (string literal only). Returns the matching
    node or None.
    """
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        if isinstance(func, ast.Name) and func.id != "Requirement":
            continue
        if isinstance(func, ast.Attribute) and func.attr != "Requirement":
            continue
        # Extract id= kwarg
        for kw in node.keywords:
            if kw.arg == "id" and isinstance(kw.value, ast.Constant):
                if kw.value.value == req_id:
                    return node  # type: ignore[return-value]
    return None


def _find_requirements_tuple_end(tree: ast.AST, source_lines: list[str]) -> int | None:
    """Find the line number (1-indexed) of the closing ')' of `requirements = (...)`.

    Looks for an assignment `requirements = (...)` inside `build_graph()` and
    returns the end_lineno of the Tuple node (the line with the closing paren).
    """
    for node in ast.walk(tree):
        if not isinstance(node, ast.FunctionDef):
            continue
        if node.name != "build_graph":
            continue
        for stmt in node.body:
            if not isinstance(stmt, ast.Assign):
                continue
            for target in stmt.targets:
                if isinstance(target, ast.Name) and target.id == "requirements":
                    # The value is a Tuple
                    val = stmt.value
                    end = getattr(val, "end_lineno", None)
                    return end
    return None


# ---------------------------------------------------------------------------
# AST-based conflict locator
# ---------------------------------------------------------------------------


def _find_conflict_call(tree: ast.AST, conflict_id: str) -> ast.Call | None:
    """Canon: §Proposal — locate the Conflict(...) AST call whose computed id matches.

    Walks the AST looking for ast.Call nodes whose function is 'Conflict'. For
    each, extracts 'axis' and 'context' keyword args (string literals only) and
    computes conflict_identity(axis, context). Returns the matching node or None.
    """
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        # Match: Conflict(...) — either bare name or attribute
        if isinstance(func, ast.Name) and func.id != "Conflict":
            continue
        if isinstance(func, ast.Attribute) and func.attr != "Conflict":
            continue
        if isinstance(func, ast.Name) and func.id != "Conflict":
            continue
        # Extract axis= and context= kwargs
        kwargs: dict[str, str] = {}
        for kw in node.keywords:
            if kw.arg in ("axis", "context") and isinstance(kw.value, ast.Constant):
                kwargs[kw.arg] = kw.value.value
        if "axis" not in kwargs or "context" not in kwargs:
            continue
        computed = conflict_identity(kwargs["axis"], kwargs["context"])
        if computed == conflict_id:
            return node  # type: ignore[return-value]
    return None


# ---------------------------------------------------------------------------
# Field replacement on source text
# ---------------------------------------------------------------------------


def _kwarg_line_col(call: ast.Call, field: str) -> tuple[int, int] | None:
    """Canon: §Proposal — return (lineno, col_offset) of a keyword arg's VALUE node.

    Returns None if the kwarg is not present.
    """
    for kw in call.keywords:
        if kw.arg == field:
            return (kw.value.lineno, kw.value.col_offset)
    return None


def _replace_or_insert_field(
    source_lines: list[str],
    call: ast.Call,
    field: str,
    new_value: object,
) -> list[str]:
    """Canon: §Proposal — replace or insert a keyword arg in a Conflict(...) call.

    Strategy (deterministic string replacement):
      - If the field already exists as a kwarg, use AST line/col to locate the
        start of the value token; replace the old value with the Python repr of
        new_value using a targeted line edit.
      - If the field is absent, insert it as a new kwarg line just before the
        closing ')' of the Conflict call, indented to match siblings.

    This preserves existing formatting/indentation and avoids ast.unparse
    roundtrip reformatting.
    """
    lines = list(source_lines)

    # Try to find an existing kwarg
    for kw in call.keywords:
        if kw.arg != field:
            continue
        # Found: replace the value on its line
        val_node = kw.value
        lineno = val_node.lineno - 1  # 0-indexed
        line = lines[lineno]
        col = val_node.col_offset

        # Find the end of the value on this line (handle simple strings and tuples)
        # We use a "find the comma or close-paren after the value" heuristic.
        # For simplicity: rebuild from col to end of line, then re-glue.
        # We need the end col of the old value — check end_lineno/end_col_offset.
        end_lineno = getattr(val_node, "end_lineno", None)
        end_col = getattr(val_node, "end_col_offset", None)
        if end_lineno is not None and end_col is not None:
            if end_lineno - 1 == lineno:
                # Single-line value: replace col..end_col
                new_repr = _python_repr(new_value)
                lines[lineno] = line[:col] + new_repr + line[end_col:]
            else:
                # Multi-line value (e.g. long string): replace from col to end
                new_repr = _python_repr(new_value)
                # Grab the suffix after the value on the end line (e.g. ",\n")
                end_line = lines[end_lineno - 1]
                suffix = end_line[end_col:]
                # Remove from the line after start through the end line (inclusive)
                del lines[lineno + 1 : end_lineno - 1 + 1]
                # Now recompute after deletion
                line = lines[lineno]
                lines[lineno] = line[:col] + new_repr + suffix
        return lines

    # Field not present: insert before the closing ')' of the Conflict call.
    # Find the line of the last keyword arg (or the last member of the call)
    # and insert after it.
    end_lineno = getattr(call, "end_lineno", None)
    if end_lineno is None:
        raise RuntimeError(
            f"Cannot determine end line of Conflict call for field '{field}'"
        )

    # Find indentation from an existing kwarg line
    indent = "            "  # default: 12 spaces
    if call.keywords:
        # Find the indentation of the first keyword arg
        for kw in call.keywords:
            kw_linetext = lines[kw.value.lineno - 1]
            stripped = kw_linetext.lstrip()
            indent = kw_linetext[: len(kw_linetext) - len(stripped)]
            break

    new_repr = _python_repr(new_value)
    insert_line = f"{indent}{field}={new_repr},\n"
    # Insert before the line that contains only/mostly the closing paren
    insert_at = end_lineno - 1  # 0-indexed: the '),' or ')' line
    lines.insert(insert_at, insert_line)
    return lines


def _python_repr(value: object) -> str:
    """Canon: §Proposal — produce a Python-literal repr suitable for source insertion.

    Strings → double-quoted; empty tuples → (); tuples of strings → ("a", "b");
    empty string → "".
    """
    if isinstance(value, str):
        # Use double quotes, escape internal double quotes
        escaped = value.replace("\\", "\\\\").replace('"', '\\"')
        return f'"{escaped}"'
    if isinstance(value, tuple):
        if not value:
            return "()"
        items = ", ".join(_python_repr(v) for v in value)
        return f"({items},)" if len(value) == 1 else f"({items})"
    return repr(value)


# ---------------------------------------------------------------------------
# Diff rendering
# ---------------------------------------------------------------------------


def _render_diff(original: list[str], modified: list[str], label: str) -> str:
    """Canon: §Proposal — render a minimal unified-style diff between two line lists."""
    import difflib

    diff = list(
        difflib.unified_diff(
            original,
            modified,
            fromfile=f"a/{label}",
            tofile=f"b/{label}",
            lineterm="",
        )
    )
    return "\n".join(diff) if diff else "(no changes)"


# ---------------------------------------------------------------------------
# Requirement rendering
# ---------------------------------------------------------------------------


def _render_requirement_source(proposal: ProposedRequirement, indent: str) -> str:
    """Render a Requirement(...) constructor call as source text.

    Uses the same indentation style as existing entries in the content graph.
    """
    inner = indent + "    "  # one extra level for kwargs
    lines: list[str] = []
    lines.append(f"{indent}Requirement(")
    lines.append(f'{inner}id="{proposal.id}",')
    # Claim: use parenthesized string for readability
    claim_escaped = proposal.claim.replace("\\", "\\\\").replace('"', '\\"')
    lines.append(f'{inner}claim=("{claim_escaped}"),')
    lines.append(f'{inner}owner="{proposal.owner}",')
    lines.append(f'{inner}status="{proposal.status}",')
    if proposal.why:
        why_escaped = proposal.why.replace("\\", "\\\\").replace('"', '\\"')
        lines.append(f'{inner}why=("{why_escaped}"),')
    if proposal.assumptions:
        items = ", ".join(f'"{a}"' for a in proposal.assumptions)
        lines.append(f"{inner}assumptions=({items},),")
    if proposal.relations:
        rel_items = ", ".join(
            f'Relation("{kind}", "{target}")' for kind, target in proposal.relations
        )
        lines.append(f"{inner}relations=({rel_items},),")
    # enforcement: use the constant name if it matches, else string
    enf = proposal.enforcement
    if enf in ("PROSE", "STRUCTURAL", "ENFORCED"):
        lines.append(f"{inner}enforcement={enf},")
    else:
        lines.append(f'{inner}enforcement="{enf}",')
    if proposal.enforced_by:
        items = ", ".join(f'"{e}"' for e in proposal.enforced_by)
        lines.append(f"{inner}enforced_by=({items},),")
    if proposal.m_tag:
        lines.append(f'{inner}m_tag="{proposal.m_tag}",')
    lines.append(f"{indent}),")
    return "\n".join(lines) + "\n"


# ---------------------------------------------------------------------------
# Apply: Requirement (add or update)
# ---------------------------------------------------------------------------


def _apply_requirement_to_source(
    source_text: str, proposal: ProposedRequirement
) -> str:
    """Apply a ProposedRequirement to graph source: add new or update existing."""
    tree = ast.parse(source_text)
    existing = _find_requirement_call(tree, proposal.id)

    if existing is not None:
        # UPDATE existing requirement fields
        lines = source_text.splitlines(keepends=True)
        call_node = existing
        for field_name, new_value in [
            ("claim", proposal.claim),
            ("owner", proposal.owner),
            ("status", proposal.status),
            ("why", proposal.why),
            ("assumptions", proposal.assumptions),
            ("enforcement", proposal.enforcement),
            ("enforced_by", proposal.enforced_by),
        ]:
            # Skip empty optional fields not already present
            if field_name in ("assumptions", "enforced_by") and not new_value:
                if _kwarg_line_col(call_node, field_name) is None:
                    continue
            lines = _replace_or_insert_field(lines, call_node, field_name, new_value)
            new_src = "".join(lines)
            tree = ast.parse(new_src)
            call_node = _find_requirement_call(tree, proposal.id)
            if call_node is None:
                raise RuntimeError(
                    f"Lost track of requirement '{proposal.id}' after "
                    f"replacing field '{field_name}'."
                )
        return "".join(lines)

    # ADD new requirement at end of requirements tuple
    lines = source_text.splitlines(keepends=True)
    tree = ast.parse(source_text)
    tuple_end = _find_requirements_tuple_end(tree, lines)
    if tuple_end is None:
        raise RuntimeError(
            "Cannot find `requirements = (...)` tuple in build_graph(). "
            "Is spec/content/graph.py well-formed?"
        )

    # Determine indentation from existing Requirement calls
    indent = "        "  # default: 8 spaces
    for node in ast.walk(tree):
        if not isinstance(node, ast.Call):
            continue
        func = node.func
        if isinstance(func, ast.Name) and func.id == "Requirement":
            line_text = lines[node.lineno - 1]
            stripped = line_text.lstrip()
            indent = line_text[: len(line_text) - len(stripped)]
            break

    new_req = _render_requirement_source(proposal, indent)
    # Insert before the closing ')' of the tuple (tuple_end is 1-indexed)
    insert_at = tuple_end - 1  # 0-indexed: the ')' line
    lines.insert(insert_at, new_req)

    result = "".join(lines)

    # Ensure Relation import exists if relations are used
    if proposal.relations:
        if "Relation" not in result.split("import")[0]:
            # Check if Relation is already imported
            if "Relation" not in source_text:
                result = result.replace(
                    "from tensio.requirement import",
                    "from tensio.requirement import Relation,",
                    1,
                )
    return result


# ---------------------------------------------------------------------------
# Apply: Rejection
# ---------------------------------------------------------------------------


def _apply_rejection_to_source(source_text: str, proposal: ProposedRejection) -> str:
    """Apply a ProposedRejection: set status to REJECTED, prepend reason to why."""
    tree = ast.parse(source_text)
    call_node = _find_requirement_call(tree, proposal.requirement_id)
    if call_node is None:
        raise RuntimeError(
            f"Requirement '{proposal.requirement_id}' not found in {_CONTENT_GRAPH}."
        )

    lines = source_text.splitlines(keepends=True)

    # Set status to "REJECTED"
    lines = _replace_or_insert_field(lines, call_node, "status", "REJECTED")
    new_src = "".join(lines)
    tree = ast.parse(new_src)
    call_node = _find_requirement_call(tree, proposal.requirement_id)
    if call_node is None:
        raise RuntimeError(
            f"Lost track of requirement '{proposal.requirement_id}' after "
            f"setting status."
        )

    # Prepend rejection reason to why
    existing_why = ""
    for kw in call_node.keywords:
        if kw.arg == "why" and isinstance(kw.value, ast.Constant):
            existing_why = kw.value.value
            break

    new_why = proposal.reason
    if existing_why:
        new_why = f"{proposal.reason} — (was: {existing_why})"

    lines = _replace_or_insert_field(lines, call_node, "why", new_why)
    return "".join(lines)


# ---------------------------------------------------------------------------
# Apply: ConflictTransition
# ---------------------------------------------------------------------------


def _apply_conflict_transition(
    source_text: str, proposal: ProposedConflictTransition
) -> list[str]:
    """Apply a ConflictTransition to graph source. Returns modified lines."""
    tree = ast.parse(source_text)
    call_node = _find_conflict_call(tree, proposal.conflict_id)
    if call_node is None:
        raise RuntimeError(
            f"conflict_id '{proposal.conflict_id}' not found in "
            f"{_CONTENT_GRAPH}. No changes made."
        )

    lines = source_text.splitlines(keepends=True)
    for field_name, new_value in [
        ("lifecycle", proposal.new_lifecycle),
        ("decided_by", proposal.decided_by),
        ("revisit_marker", proposal.revisit_marker),
        ("derived", proposal.derived),
    ]:
        if field_name in ("revisit_marker",) and not new_value:
            existing = _kwarg_line_col(call_node, field_name)
            if existing is None:
                continue
        if field_name == "derived" and not new_value:
            existing = _kwarg_line_col(call_node, field_name)
            if existing is None:
                continue
        lines = _replace_or_insert_field(lines, call_node, field_name, new_value)
        new_src = "".join(lines)
        tree = ast.parse(new_src)
        call_node = _find_conflict_call(tree, proposal.conflict_id)
        if call_node is None:
            raise RuntimeError(
                f"Lost track of conflict '{proposal.conflict_id}' after "
                f"replacing field '{field_name}'."
            )
    return list(lines)


# ---------------------------------------------------------------------------
# Main apply logic
# ---------------------------------------------------------------------------


def apply(
    proposal: Proposal,
    *,
    dry_run: bool = False,
    triggering_kind: str | None = None,
) -> int:
    """Canon: §Proposal — apply a validated Proposal to the graph.

    Dispatches to the appropriate handler based on proposal type:
      - ProposedConflictTransition: locate Conflict node, replace fields
      - ProposedRequirement: add new or update existing Requirement
      - ProposedRejection: set status to REJECTED, prepend reason

    Steps:
      1. Read spec/content/graph.py.
      2. Apply the proposal (type-dispatched).
      3. If dry_run: print diff and return 0 without writing.
      4. Write the file, run gen_spec.py, run pytest -q.
      5. If triggering_kind supplied: run closure.check_closure; return 2 if not advanced.
      6. Return 0 on success, 1 on any non-closure failure.

    `triggering_kind` is the band/kind string of the original action this proposal
    was meant to close (e.g. "CONFLICT_STALLED", "OPEN_ITEM", "STRUCTURE",
    "DRIFT_FALLOUT"). If None, the closure check is skipped (backward-compat for
    P3 unit tests that do not supply a triggering kind).
    """
    source_text = _CONTENT_GRAPH.read_text(encoding="utf-8")
    original_lines = source_text.splitlines(keepends=True)

    try:
        if isinstance(proposal, ProposedConflictTransition):
            lines = _apply_conflict_transition(source_text, proposal)
        elif isinstance(proposal, ProposedRequirement):
            new_source = _apply_requirement_to_source(source_text, proposal)
            lines = new_source.splitlines(keepends=True)
        elif isinstance(proposal, ProposedRejection):
            new_source = _apply_rejection_to_source(source_text, proposal)
            lines = new_source.splitlines(keepends=True)
        else:
            print(
                f"ERROR: unhandled proposal type {type(proposal).__name__}",
                file=sys.stderr,
            )
            return 1
    except RuntimeError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1

    if dry_run:
        diff = _render_diff(original_lines, lines, _CONTENT_GRAPH.name)
        print("=== DRY RUN — proposed diff ===")
        print(diff)
        print("=== (no file written) ===")
        if triggering_kind is not None:
            # In dry-run mode, we cannot truly check closure (no file was written),
            # but we emit the section so callers can verify the flag is wired.
            print(
                f"\n=== CLOSURE CHECK (dry-run, not authoritative) ===\n"
                f"  triggering_kind : {triggering_kind}\n"
                f"  target          : {proposal.target_anchor()}\n"
                f"  note            : (dry-run — run without --dry-run for real closure check)\n"
                f"=== END CLOSURE CHECK ==="
            )
        return 0

    # Write
    new_source = "".join(lines)
    _CONTENT_GRAPH.write_text(new_source, encoding="utf-8")
    print(f"Written: {_CONTENT_GRAPH}")

    # Regen
    regen_result = subprocess.run(
        [sys.executable, str(_GEN_SPEC)],
        capture_output=True,
        text=True,
    )
    if regen_result.returncode != 0:
        print("ERROR: gen_spec.py failed:", file=sys.stderr)
        print(regen_result.stderr, file=sys.stderr)
        return 1
    print("gen_spec.py: OK")

    # Verify
    pytest_result = subprocess.run(
        [sys.executable, "-m", "pytest", "-q", str(_SPEC_ROOT / "tests")],
        capture_output=True,
        text=True,
        cwd=str(_SPEC_ROOT),
    )
    print(pytest_result.stdout)
    if pytest_result.returncode != 0:
        print(
            "ERROR: pytest failed after apply. File written but tests are red.",
            file=sys.stderr,
        )
        print(pytest_result.stderr, file=sys.stderr)
        print(
            "NOTE: auto-revert is not implemented in P3. "
            "Inspect the diff and revert manually if needed.",
            file=sys.stderr,
        )
        return 1

    # Closure check (P4 feedback edge) — only when --triggering-kind is supplied.
    if triggering_kind is not None:
        import closure  # noqa: PLC0415  (lives in tools/, not a package)

        result = closure.check_closure(proposal, triggering_kind)
        print(
            f"\n=== CLOSURE CHECK ===\n"
            f"  advanced        : {result.advanced}\n"
            f"  target          : {result.target}\n"
            f"  triggering_kind : {result.triggering_kind}\n"
            f"  still_open      : {result.still_open_count}\n"
            f"  note            : {result.note}\n"
            f"=== END CLOSURE CHECK ==="
        )
        if not result.advanced:
            print(
                "\nERROR: closure FAILED — the triggering action is STILL in the "
                "post-apply diagnosis. The write landed (tests green) but the "
                "action did NOT advance. Investigate before marking closed.",
                file=sys.stderr,
            )
            return 2

    summary_target = proposal.target_anchor()
    summary_kind = type(proposal).__name__
    print(
        f"\nSUMMARY:\n"
        f"  kind   : {summary_kind}\n"
        f"  target : {summary_target}\n"
        f"  tests  : GREEN"
    )
    if isinstance(proposal, ProposedConflictTransition):
        print(
            f"  decided_by   : {proposal.decided_by or '(none)'}\n"
            f"  new_lifecycle: {proposal.new_lifecycle}"
        )
    if triggering_kind is not None:
        print(f"  closure: ADVANCED (action {summary_target!r} closed)")
    return 0


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------


def main(argv: list[str] | None = None) -> int:
    """Canon: §Proposal — CLI entry point for apply_proposal.py.

    Exit codes:
      0 — success (write landed, tests green, closure confirmed if --triggering-kind).
      1 — failure (validation, missing id, or pytest red).
      2 — not advanced (write+tests green, but triggering action STILL in diagnosis).
    """
    parser = argparse.ArgumentParser(
        description=(
            "Mechanically apply a steward-approved JSON proposal to spec/content/graph.py. "
            "Optionally verify P4 closure: that the triggering diagnosis was removed."
        )
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print the proposed diff without writing any files.",
    )
    parser.add_argument(
        "--triggering-kind",
        metavar="KIND",
        default=None,
        help=(
            "The what_now band/kind of the action this proposal was meant to close "
            "(e.g. CONFLICT_STALLED, OPEN_ITEM, STRUCTURE, DRIFT_FALLOUT). "
            "When supplied, runs closure.check_closure after pytest passes. "
            "If the action still appears in the post-apply diagnosis, exit code 2 "
            "is returned (distinguishable from pytest failures which exit 1). "
            "Omit to skip the closure check (backward-compatible with P3 tests)."
        ),
    )
    parser.add_argument(
        "--batch",
        action="store_true",
        help=(
            "Treat the JSON file as an array of proposals and apply them "
            "sequentially. Critical for atomized wave proposals."
        ),
    )
    parser.add_argument(
        "proposal_file",
        help="Path to the steward-approved JSON proposal file.",
    )
    args = parser.parse_args(argv)

    proposal_path = Path(args.proposal_file)
    if not proposal_path.exists():
        print(f"ERROR: proposal file not found: {proposal_path}", file=sys.stderr)
        return 1

    try:
        raw = json.loads(proposal_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        print(f"ERROR: invalid JSON in {proposal_path}: {exc}", file=sys.stderr)
        return 1

    if args.batch:
        if not isinstance(raw, list):
            print(
                "ERROR: --batch expects a JSON array of proposals.",
                file=sys.stderr,
            )
            return 1
        for i, item in enumerate(raw):
            print(f"\n--- Proposal {i + 1}/{len(raw)} ---")
            try:
                proposal = _validate_proposal(item)
            except ValueError as exc:
                print(f"ERROR: invalid proposal #{i + 1}: {exc}", file=sys.stderr)
                return 1
            rc = apply(
                proposal,
                dry_run=args.dry_run,
                triggering_kind=args.triggering_kind,
            )
            if rc != 0:
                print(
                    f"ERROR: proposal #{i + 1} failed with exit code {rc}. "
                    f"Stopping batch.",
                    file=sys.stderr,
                )
                return rc
        return 0

    try:
        proposal = _validate_proposal(raw)
    except ValueError as exc:
        print(f"ERROR: invalid proposal: {exc}", file=sys.stderr)
        return 1

    return apply(proposal, dry_run=args.dry_run, triggering_kind=args.triggering_kind)


if __name__ == "__main__":
    sys.exit(main())
