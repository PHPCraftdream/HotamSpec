"""Canon: §Invariants — surfaces Requirements with compound claims and check_* functions with compound conditions, both structural signals for decomposition.

Atomicity audit: detect compound claims and compound invariants.

Run:
  uv run python tools/audit_atomicity.py            # audit domain content
  uv run python tools/audit_atomicity.py --demo      # audit demo fixture

Output: AUDIT.md is written to the ACTIVE domain's docs/gen/ dir
(domains/<active>/docs/gen/AUDIT.md), resolved through the same
gen_spec._resolve_active_gen_dir() the generator itself uses -- not a fixed
top-level docs/gen/. A legacy top-level docs/gen/AUDIT.md would silently go
stale the moment a second domain becomes active; there is exactly one
resolver and one written location, shared with gen_spec.py.

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

import _bootstrap  # noqa: E402,F401  -- side effect: configures sys.path for hotam_spec + tools
SPEC_ROOT = Path(__file__).resolve().parents[1]  # .../spec
REPO_ROOT = SPEC_ROOT.parent  # .../HotamSpec

# GEN_DIR resolves through the SAME active-domain resolver gen_spec.py uses
# (gen_spec._resolve_active_gen_dir), so the audit is written to the ACTIVE
# domain's docs/gen/ (domains/<active>/docs/gen/AUDIT.md), never a stale
# top-level docs/gen/ that no longer corresponds to the graph being audited
# (R-active-domain-pin-not-alphabetical: one resolver, one answer, shared).
from gen_spec import _resolve_active_gen_dir  # noqa: E402

GEN_DIR = _resolve_active_gen_dir()

from hotam_spec.doc_readers import reader_line as _doc_reader_line  # noqa: E402
from hotam_spec.graph import (  # noqa: E402
    TensionGraph,
    active_domain_doc_readers,
    stakeholder_ids,
)
from hotam_spec.invariants import ALL_INVARIANTS  # noqa: E402
from _graph_loader import load_graph as _load_graph  # noqa: E402


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


_CLAIM_SCOPE_CLAUSE = re.compile(
    r"\s--\s(?:this\s+is|this\s+does\s+not|it\s+does\s+not|not\s+itself|"
    r"[\w\s-]{0,60}?\bapplies\s+only\b|[\w\s-]{0,60}?\bnever\s+substituted\b|"
    r"derivation\s+of|[\w\s-]{0,60}?\bdeferred\s+until\b)",
    re.IGNORECASE,
)


def _audit_claim(claim: str) -> tuple[str, str]:
    """Return (verdict, reason) for a requirement claim.

    RULE: a trailing ``-- this is/does not/never ...`` clause is a SCOPE
    DISCLAIMER on the one relation already stated (naming what the claim
    does NOT promise, e.g. R-commit-boundary-checkable's "-- this is the
    mechanically checkable SLICE ... it does not itself verify that a
    steward runs it"), not a second independent obligation -- it is exempt
    from the semicolon/em-dash compound signals below. WHY: the same
    false-positive class the invariant side already fixed for
    "N sub-rules" self-declarations (_declared_sub_rule_count) -- a single
    promise legitimately carries a boundary clause narrowing what it does
    NOT cover, and flagging that as COMPOUND would push authors to either
    delete honest disclaimers (making claims falsely appear unbounded) or
    split them into a fake sibling requirement that promises nothing new.
    """
    m = _CLAIM_SCOPE_CLAUSE.search(claim)
    if m:
        # A genuine scope disclaimer is TRAILING -- it narrows the one relation
        # already stated and carries no further obligation. If a fresh
        # obligation verb ("shall") survives AFTER the disclaimer marker, the
        # "-- this does not ..." is being used to smuggle a second independent
        # promise past the exemption; do NOT strip in that case so the
        # semicolon/em-dash/"and" signals below still catch it. This closes the
        # "obligation buried behind a disclaimer clause" hole while leaving the
        # legitimate trailing-disclaimer case (R-commit-boundary-checkable,
        # R-tiered-gate-not-a-commit-gate) exempt.
        tail = claim[m.start():]
        if not re.search(r"\bshall\b", tail, re.IGNORECASE):
            # Strip the disclaimer clause before running the compound signals so
            # a genuine second obligation earlier in the claim is still caught.
            claim = _CLAIM_SCOPE_CLAUSE.split(claim)[0]

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
    """Return list of distinct entity types walked by statement-level for-loops.

    Deliberately excludes comprehensions (list/set/dict) such as
    ``type_by_slug = {et.slug: et for et in g.entity_types}`` -- a
    comprehension building a lookup TABLE consumed by a single subsequent
    walk is not a second independent rule over that entity type, it is
    plumbing for the one rule the statement-level loop enforces. Counting it
    as a second "loop over an entity type" is a classifier false positive:
    the check still enforces exactly one relation, it just needs a helper
    index to do so efficiently. Only ``for x in g.<attr>:`` STATEMENTS (the
    ast.For form, which can carry a violation-producing body across
    multiple statements) signal a genuine second independent walk.
    """
    try:
        dedented = __import__("textwrap").dedent(source)
        tree = ast.parse(dedented)
    except SyntaxError:
        # Fall back to the old regex heuristic if the source can't be parsed
        # standalone (e.g. decorators referencing local names).
        pattern = re.compile(r"for\s+\w+\s+in\s+g\.(\w+)")
        return sorted(set(pattern.findall(source)))

    found: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.For):
            it = node.iter
            if (
                isinstance(it, ast.Attribute)
                and isinstance(it.value, ast.Name)
                and it.value.id == "g"
            ):
                found.add(it.attr)
    return sorted(found)


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


_SUB_RULE_MARKER = re.compile(
    r"\b(?:two|three|four|five|\d+)\s+sub-rules\b", re.IGNORECASE
)


def _declared_sub_rule_count(func: object) -> int:
    """Return how many independent sub-rules a check_*'s OWN docstring claims.

    Multiple ``Violation(...)`` call sites with distinct message text are not
    on their own evidence of a compound (multi-rule) check: a single relation
    ("X must resolve") legitimately fails in several distinct ways (missing
    docstring / missing RULE line / low overlap; stale row / bad kind /
    duplicate row / unclassified function) without the check enforcing more
    than one rule -- see check_bijection_r_to_enforcer's own docstring, which
    is exempted from compoundness at exactly 2 messages by explicitly
    documenting itself as "two sub-rules" ANDed under one bijection. The
    signal that separates "one relation, several failure branches" from
    "actually multiple independent rules bundled into one function" is
    whether the docstring itself claims more than one rule -- an author
    writing "RULE (...): N sub-rules ..." is self-declaring compoundness;
    a single "RULE: ..." paragraph describing one relation is not, no matter
    how many ways that one relation can fail.
    """
    doc = inspect.getdoc(func) or ""
    m = _SUB_RULE_MARKER.search(doc)
    if not m:
        return 1
    word = m.group(0).split()[0].lower()
    numerals = {"two": 2, "three": 3, "four": 4, "five": 5}
    if word in numerals:
        return numerals[word]
    try:
        return int(word)
    except ValueError:
        return 1


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

    # Check semantic conditions via AST -- only a compound signal when the
    # check's OWN docstring self-declares more than one sub-rule (see
    # _declared_sub_rule_count); otherwise several Violation messages are
    # failure-mode branches of the ONE relation the docstring's single RULE
    # line describes, not evidence of multiple bundled rules.
    try:
        # Dedent the source for parsing
        import textwrap

        dedented = textwrap.dedent(source)
        tree = ast.parse(dedented)
        n_conditions = _count_semantic_conditions(tree)
        declared_rules = _declared_sub_rule_count(func)
        if n_conditions > 2 and n_conditions > declared_rules:
            reasons.append(
                f"{n_conditions} distinct violation messages "
                f"(docstring declares {declared_rules} sub-rule"
                f"{'s' if declared_rules != 1 else ''})"
            )
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
    reader_stakeholder_ids: frozenset[str] | None = None,
) -> str:
    """Build the AUDIT.md content.

    RULE (R-doc-names-reader): stamps a `reader: <Stakeholder.id>` header
    line — AUDIT is a FRAMEWORK-INTERNAL doc (ROLE_FRAMEWORK_MAINTAINER, same
    family as FRAMEWORK-INVARIANTS.md/GLOSSARY.md) — mirroring gen_spec.py's
    build_* functions so every generated doc names its reader.
    """
    lines: list[str] = [_BANNER]
    reader = _doc_reader_line(
        "AUDIT", reader_stakeholder_ids or frozenset(), active_domain_doc_readers()
    )
    lines.append(f"{reader}\n\n")
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
    # Scope (R-requirement-claim-is-atomic): the atomicity ratchet governs
    # LIVE promises only. REJECTED requirements are kept forever as history
    # (R-rejected-preserved-not-deleted) -- their claim text is frozen
    # historical prose, never re-edited, so their compoundness is a fact
    # about the past, not debt anyone can (or should) burn down today;
    # flagging them COMPOUND would put dead prose on the same ratchet as a
    # currently-held promise. DRAFT requirements are "proposed, not yet
    # accepted into the canon" (requirement.py docstring) -- they are not a
    # promise yet either; DRAFT->SETTLED promotion is exactly the moment to
    # atomize a compound draft, so they are excluded from the ratchet here
    # and instead re-audited at PROMOTE time (not audit_atomicity's job).
    # SETTLED (including SETTLED/INHERENTLY_PROSE and SETTLED/OPEN(...)) ARE
    # live, currently-held promises -- INHERENTLY_PROSE only changes HOW a
    # requirement is verified (no check_* can ever confirm it), not WHETHER
    # it is a live promise the steward is answerable for, so it stays in
    # scope for atomicity same as any other SETTLED claim.
    claim_rows: list[tuple[str, str, str]] = []
    for r in sorted(g.requirements, key=lambda r: r.id):
        if r.status != "SETTLED" and not r.status.startswith("OPEN"):
            continue
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
    md = _build_audit_md(claim_rows, inv_rows, stakeholder_ids(g))
    out_path = GEN_DIR / "AUDIT.md"
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text(md, encoding="utf-8", newline="\n")
    print(f"\nwritten: {out_path}")


if __name__ == "__main__":
    main()
