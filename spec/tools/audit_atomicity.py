"""Canon: §Invariants — surfaces Requirements with compound claims and check_* functions with compound conditions, both structural signals for decomposition.

Atomicity audit: detect compound claims and compound invariants.

Run:
  uv run python tools/audit_atomicity.py            # audit domain content
  uv run python tools/audit_atomicity.py --demo      # audit demo fixture

Deterministic: sorted output, LF newlines, utf-8, no timestamps.
"""

from __future__ import annotations

import argparse
import ast
import inspect
import re
import sys
from pathlib import Path

# --- Make the hotam_spec package importable ------------------------------------

SPEC_ROOT = Path(__file__).resolve().parents[1]  # .../spec
REPO_ROOT = SPEC_ROOT.parent  # .../HotamSpec
GEN_DIR = REPO_ROOT / "docs" / "gen"

if str(SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(SPEC_ROOT / "src"))

from hotam_spec.graph import TensionGraph, load_content_graph  # noqa: E402
from hotam_spec.invariants import ALL_INVARIANTS  # noqa: E402


# ---------------------------------------------------------------------------
# Graph loading (mirrors gen_spec.py pattern)
# ---------------------------------------------------------------------------


def _load_graph(*, demo: bool) -> TensionGraph:
    """Return the graph to audit: demo fixture or domain content."""
    if demo:
        tests_dir = str(SPEC_ROOT / "tests")
        if tests_dir not in sys.path:
            sys.path.insert(0, tests_dir)
        from fixtures.seed import seed_graph  # noqa: PLC0415

        return seed_graph()
    return load_content_graph()


# ---------------------------------------------------------------------------
# Claim atomicity heuristics
# ---------------------------------------------------------------------------

# Patterns that signal compound claims
_SEMICOLON = re.compile(r";\s+")
_DASH_ALSO = re.compile(r"\s—\salso\s")
# " and " connecting clauses (heuristic: followed by a verb-like word)
_AND_CLAUSE = re.compile(
    r",?\s+and\s+(?:shall|must|should|will|can|may|does|is|are|has|have|ensure|prevent|require|forbid|allow|enforce|guarantee|surface|make|keep|hold|carry|check|fire|run|return|produce|emit|write|read|load|store|create|delete|update|track|record|name|cite|anchor|model|declare|assign|mark|set|drive|derive|start|stop|block|close|open|reject|approve|resolve|decide|propose|settle|revisit)\b",
    re.IGNORECASE,
)
# " — " followed by a new verb (not a parenthetical)
_DASH_VERB = re.compile(
    r"\s—\s(?:shall|must|should|will|can|may|does|is|are|has|have|it|the|a|an|this|that|every|each|no|all|any|never|always|if|when|where|while|because|since|so|but|not|also|only|either|neither|both|without|within|under)\b",
    re.IGNORECASE,
)


def _audit_claim(claim: str) -> tuple[str, str]:
    """Return (verdict, reason) for a requirement claim."""
    # Semicolons splitting independent statements
    parts = _SEMICOLON.split(claim)
    if len(parts) > 1:
        return "COMPOUND", f"semicolon splits {len(parts)} segments"

    # " — also "
    if _DASH_ALSO.search(claim):
        return "COMPOUND", "' — also ' joins independent statements"

    # " and " connecting clauses with a verb
    matches = _AND_CLAUSE.findall(claim)
    if matches:
        return "COMPOUND", f"'and' connects clause with verb ({matches[0].strip()})"

    # Multiple sentences (period followed by capital letter, not abbreviations)
    sentences = re.split(r"\.\s+(?=[A-Z])", claim)
    if len(sentences) > 1:
        # Check if they have distinct subjects (very rough heuristic)
        return "COMPOUND", f"{len(sentences)} sentences detected"

    return "ATOMIC", ""


# ---------------------------------------------------------------------------
# Invariant atomicity heuristics
# ---------------------------------------------------------------------------


def _count_entity_loops(source: str) -> list[str]:
    """Return list of distinct entity types iterated in for-loops."""
    # Match patterns like "for x in g.requirements", "for x in g.conflicts", etc.
    pattern = re.compile(r"for\s+\w+\s+in\s+g\.(\w+)")
    return sorted(set(pattern.findall(source)))


def _count_semantic_conditions(tree: ast.Module) -> int:
    """Count distinct violation-producing branches (append calls with different messages)."""
    # Count distinct string literals in Violation(...) or out.append(Violation(...)) calls
    messages: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Call):
            # Look for Violation("check_name", target, "message") patterns
            if isinstance(node.func, ast.Name) and node.func.id == "Violation":
                if len(node.args) >= 3 and isinstance(
                    node.args[2], (ast.Constant, ast.JoinedStr)
                ):
                    if isinstance(node.args[2], ast.Constant) and isinstance(
                        node.args[2].value, str
                    ):
                        # Use first 40 chars as a semantic key
                        messages.add(node.args[2].value[:40])
                    else:
                        # f-string — use the first constant part
                        messages.add(f"fstring_{id(node.args[2])}")
    return len(messages)


def _audit_invariant(func: object) -> tuple[str, str]:
    """Return (verdict, reason) for a check_* function."""
    try:
        source = inspect.getsource(func)  # type: ignore[arg-type]
    except (OSError, TypeError):
        return "ATOMIC", "could not inspect source"

    reasons: list[str] = []

    # Check entity type loops
    entity_types = _count_entity_loops(source)
    if len(entity_types) > 1:
        reasons.append(
            f"loops over {len(entity_types)} entity types: {', '.join(entity_types)}"
        )

    # Check semantic conditions via AST
    try:
        # Dedent the source for parsing
        import textwrap

        dedented = textwrap.dedent(source)
        tree = ast.parse(dedented)
        n_conditions = _count_semantic_conditions(tree)
        if n_conditions > 2:
            reasons.append(f"{n_conditions} distinct violation messages")
    except SyntaxError:
        pass

    if reasons:
        return "COMPOUND", "; ".join(reasons)
    return "ATOMIC", ""


# ---------------------------------------------------------------------------
# Output
# ---------------------------------------------------------------------------

_BANNER = "<!-- Generated by tools/audit_atomicity.py — DO NOT EDIT BY HAND -->\n\n"


def _build_audit_md(
    claim_rows: list[tuple[str, str, str]],
    inv_rows: list[tuple[str, str, str]],
) -> str:
    """Build the AUDIT.md content."""
    lines: list[str] = [_BANNER]
    lines.append("# Atomicity Audit\n\n")

    # Requirements section
    lines.append("## Requirement Claims\n\n")
    lines.append("| Requirement | Verdict | Reason |\n")
    lines.append("|---|---|---|\n")
    for rid, verdict, reason in claim_rows:
        reason_esc = reason.replace("|", "\\|") if reason else ""
        lines.append(f"| `{rid}` | {verdict} | {reason_esc} |\n")

    lines.append("\n## check_* Invariants\n\n")
    lines.append("| Invariant | Verdict | Reason |\n")
    lines.append("|---|---|---|\n")
    for name, verdict, reason in inv_rows:
        reason_esc = reason.replace("|", "\\|") if reason else ""
        lines.append(f"| `{name}` | {verdict} | {reason_esc} |\n")

    return "".join(lines)


def main(argv: list[str] | None = None) -> None:
    """Audit atomicity of requirements and invariants."""
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument(
        "--demo",
        action="store_true",
        help="audit the fixture demo graph instead of domain content.",
    )
    args = parser.parse_args(argv)
    g = _load_graph(demo=args.demo)

    # --- Audit requirement claims ---
    claim_rows: list[tuple[str, str, str]] = []
    for r in sorted(g.requirements, key=lambda r: r.id):
        verdict, reason = _audit_claim(r.claim)
        claim_rows.append((r.id, verdict, reason))

    # --- Audit check_* invariants ---
    inv_rows: list[tuple[str, str, str]] = []
    for func in ALL_INVARIANTS:
        name = func.__name__
        if not name.startswith("check_"):
            continue
        verdict, reason = _audit_invariant(func)
        inv_rows.append((name, verdict, reason))
    inv_rows.sort(key=lambda t: t[0])

    # --- Print summary ---
    compound_claims = sum(1 for _, v, _ in claim_rows if v == "COMPOUND")
    compound_invs = sum(1 for _, v, _ in inv_rows if v == "COMPOUND")
    total_claims = len(claim_rows)
    total_invs = len(inv_rows)

    print("=== Atomicity Audit ===\n")
    print(f"Requirements: {compound_claims}/{total_claims} COMPOUND")
    for rid, verdict, reason in claim_rows:
        marker = "!!" if verdict == "COMPOUND" else "  "
        print(f"  {marker} {rid}: {verdict}" + (f" — {reason}" if reason else ""))

    print(f"\nInvariants: {compound_invs}/{total_invs} COMPOUND")
    for name, verdict, reason in inv_rows:
        marker = "!!" if verdict == "COMPOUND" else "  "
        print(f"  {marker} {name}: {verdict}" + (f" — {reason}" if reason else ""))

    # --- Write AUDIT.md ---
    md = _build_audit_md(claim_rows, inv_rows)
    out_path = GEN_DIR / "AUDIT.md"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(md, encoding="utf-8", newline="\n")
    print(f"\nwritten: {out_path}")


if __name__ == "__main__":
    main()
