"""Canon: §Proposal — mechanical writer for steward-approved JSON proposals.

Reads a steward-approved JSON proposal from a file path argument, validates it
against the ProposedConflictTransition shape, locates the target Conflict node
in spec/content/graph.py via AST, applies the field changes via deterministic
string replacement, regenerates docs via gen_spec.py, and runs pytest -q to
verify the change is structurally clean.

This is the FIRST OPERATOR ACTION TOOL: the AI operator emits a proposal
(see tensio/proposal.py); the steward approves out-of-band; then the AI calls
this tool to mechanically land the change. No free-text editing of the graph.

SCOPE (P3): implements ONLY the ProposedConflictTransition → DECIDED path.
ProposedRequirement and ProposedRejection are deferred to P4+.

Usage:
  uv run python tools/apply_proposal.py proposal.json
  uv run python tools/apply_proposal.py --dry-run proposal.json

The JSON shape (ProposedConflictTransition DECIDED):
  {
    "kind": "ConflictTransition",
    "conflict_id": "C-8600b1b8",
    "new_lifecycle": "DECIDED(... rationale text ...)",
    "decided_by": "domain-user",
    "revisit_marker": "REVISIT if ...",
    "derived": ["R-foo"]
  }

The tool REFUSES if:
  - new_lifecycle starts with DECIDED but decided_by is empty.
  - conflict_id does not resolve in spec/content/graph.py.
  - pytest fails after the write (reports exit code; does NOT auto-revert).
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
from tensio.proposal import ProposedConflictTransition  # noqa: E402

_CONTENT_GRAPH = _SPEC_ROOT / "content" / "graph.py"
_GEN_SPEC = Path(__file__).resolve().parent / "gen_spec.py"

# ---------------------------------------------------------------------------
# Validation helpers
# ---------------------------------------------------------------------------


def _validate_proposal(raw: dict) -> ProposedConflictTransition:
    """Canon: §Proposal — parse and validate a JSON dict into ProposedConflictTransition.

    RULE: 'kind' must equal 'ConflictTransition'; 'conflict_id' and
    'new_lifecycle' are required strings. If new_lifecycle starts with DECIDED,
    decided_by must be non-empty.

    Returns a ProposedConflictTransition or raises ValueError with a clear
    message.
    """
    kind = raw.get("kind", "")
    if kind != "ConflictTransition":
        raise ValueError(
            f"Unsupported proposal kind '{kind}'. "
            f"P3 implements only 'ConflictTransition'."
        )
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
                # Remove intermediate lines, replace on first
                del lines[lineno + 1 : end_lineno - 1 + 1]
                # Now recompute after deletion
                line = lines[lineno]
                lines[lineno] = line[:col] + new_repr + ","
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
# Main apply logic
# ---------------------------------------------------------------------------


def apply(proposal: ProposedConflictTransition, *, dry_run: bool = False) -> int:
    """Canon: §Proposal — apply a validated ProposedConflictTransition to the graph.

    Steps:
      1. Read and parse spec/content/graph.py via ast.
      2. Locate the Conflict node whose computed id matches proposal.conflict_id.
      3. Replace/insert lifecycle, decided_by, revisit_marker, derived.
      4. If dry_run: print diff and return 0 without writing.
      5. Write the file, run gen_spec.py, run pytest -q.
      6. Return 0 on success, 1 on any failure.
    """
    source_text = _CONTENT_GRAPH.read_text(encoding="utf-8")
    original_lines = source_text.splitlines(keepends=True)
    tree = ast.parse(source_text)

    call_node = _find_conflict_call(tree, proposal.conflict_id)
    if call_node is None:
        print(
            f"ERROR: conflict_id '{proposal.conflict_id}' not found in "
            f"{_CONTENT_GRAPH}. No changes made.",
            file=sys.stderr,
        )
        return 1

    # Apply field replacements in order
    lines = list(original_lines)
    for field_name, new_value in [
        ("lifecycle", proposal.new_lifecycle),
        ("decided_by", proposal.decided_by),
        ("revisit_marker", proposal.revisit_marker),
        ("derived", proposal.derived),
    ]:
        # Skip empty optional fields that aren't already present
        if field_name in ("revisit_marker",) and not new_value:
            existing = _kwarg_line_col(call_node, field_name)
            if existing is None:
                continue  # don't insert empty revisit_marker
        if field_name == "derived" and not new_value:
            existing = _kwarg_line_col(call_node, field_name)
            if existing is None:
                continue  # don't insert empty derived
        lines = _replace_or_insert_field(lines, call_node, field_name, new_value)
        # Re-parse after each replacement to keep AST offsets accurate
        new_src = "".join(lines)
        tree = ast.parse(new_src)
        call_node = _find_conflict_call(tree, proposal.conflict_id)
        if call_node is None:
            print(
                f"ERROR: lost track of conflict '{proposal.conflict_id}' after "
                f"replacing field '{field_name}'. Aborting.",
                file=sys.stderr,
            )
            return 1

    if dry_run:
        diff = _render_diff(original_lines, lines, _CONTENT_GRAPH.name)
        print("=== DRY RUN — proposed diff ===")
        print(diff)
        print("=== (no file written) ===")
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

    print(
        f"\nSUMMARY:\n"
        f"  conflict : {proposal.conflict_id}\n"
        f"  decided_by: {proposal.decided_by or '(none)'}\n"
        f"  new_lifecycle: {proposal.new_lifecycle}\n"
        f"  tests: GREEN"
    )
    return 0


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------


def main(argv: list[str] | None = None) -> int:
    """Canon: §Proposal — CLI entry point for apply_proposal.py."""
    parser = argparse.ArgumentParser(
        description="Mechanically apply a steward-approved JSON proposal to spec/content/graph.py."
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print the proposed diff without writing any files.",
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

    try:
        proposal = _validate_proposal(raw)
    except ValueError as exc:
        print(f"ERROR: invalid proposal: {exc}", file=sys.stderr)
        return 1

    return apply(proposal, dry_run=args.dry_run)


if __name__ == "__main__":
    sys.exit(main())
