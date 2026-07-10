"""Canon: §Axis — scaffolds a new Axis into the active domain's controlled-vocabulary
`axes` tuple via a MANDATORY confront-style similarity gate.

WHY a gatekeeper CLI, not a bare apply_proposal shortcut: an axis is the ONE
structural place many local Conflict disputes cluster into a single unresolved
ARCHITECTURAL choice (R-axis-controlled-vocab). Admitting a near-duplicate axis
silently FORKS one cluster into two ("latency-vs-completeness" vs
"speed-vs-full-check" would otherwise never merge) — exactly the kind of
important-yet-invisible drift R-anchor-everything forbids. R-axis-gatekeeper-policy
requires duplicate gatekeeping to be a MANDATORY part of the axis-creation path,
not a separately-switched feature: "a privatnik is born with a door."

Mechanism: reuses confront.py's lexical token/stem overlap heuristic (same
deterministic, stdlib-only scoring as the CONFRONT loop step), scored against
EVERY existing Axis's slug+description tokens in the active domain's graph. A
score at or above --threshold (default 0.35 — axis descriptions are short, so
containment/Jaccard overlap is a stronger signal than for full requirement
prose) refuses with exit code 1, naming the nearest existing axis. --force-new
"<justification>" overrides the refusal; the justification is folded into the
Axis proposal's `why` field so the override is recorded, never silent
(R-speak-by-reference; mirrors R-decided-needs-human-signoff's discipline of
never letting an override vanish into free text).

Usage:
  python tools/create_axis.py <slug> --description "..."
  python tools/create_axis.py <slug> --description "..." --dry-run
  python tools/create_axis.py <slug> --description "..." --force-new "justification text"

Exit codes:
  0 — success (Axis landed, docs regenerated, tests green) or a --dry-run print.
  1 — failure (validation error, gatekeeper refusal, or apply_proposal failed).
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path

_SLUG_RE = re.compile(r"^[a-z][a-z0-9-]*$")

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_SRC = _SPEC_ROOT / "src"
_TOOLS = Path(__file__).resolve().parent
for _p in (_SRC, _TOOLS):
    if _p.is_dir() and str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import confront  # noqa: E402
from hotam_spec.graph import TensionGraph, load_content_graph  # noqa: E402

_APPLY_PROPOSAL = _TOOLS / "apply_proposal.py"

#: Axis descriptions are short (one or two sentences); a lower bar than
#: confront's default 0.15 catches near-duplicates without the noise a full
#: requirement-length threshold would miss on such short text.
_DEFAULT_THRESHOLD = 0.35


def _axis_matches(
    g: TensionGraph, slug: str, description: str, *, threshold: float
) -> list[tuple[float, str, str]]:
    """Score `slug description` against every existing Axis's `slug description`.

    Returns (score, existing_slug, existing_description) triples with
    score >= threshold, sorted by (-score, existing_slug) — deterministic.
    Reuses confront's tokenizer/scorer (same lexical heuristic as the
    CONFRONT loop step) rather than reinventing scoring for axes.
    """
    input_tokens = confront._tokens(f"{slug} {description}")
    out: list[tuple[float, str, str]] = []
    for axis in g.axes:
        target_tokens = confront._tokens(f"{axis.slug} {axis.description}")
        score = confront._score(input_tokens, target_tokens)
        if score >= threshold:
            out.append((score, axis.slug, axis.description))
    out.sort(key=lambda t: (-t[0], t[1]))
    return out


def scaffold(
    slug: str,
    description: str,
    *,
    threshold: float = _DEFAULT_THRESHOLD,
    force_new: str = "",
    dry_run: bool = False,
    graph: TensionGraph | None = None,
) -> int:
    """Validate args, run the MANDATORY gatekeeper, build a ProposedAxis JSON,
    call apply_proposal. Returns exit code.
    """
    if not slug or not _SLUG_RE.match(slug):
        print(
            f"ERROR: slug '{slug}' must be kebab-case (lowercase letters, digits, "
            "hyphens, starting with a letter).",
            file=sys.stderr,
        )
        return 1

    if not description.strip():
        print(
            "ERROR: --description is required and must be non-empty.", file=sys.stderr
        )
        return 1

    g = graph if graph is not None else load_content_graph()

    # Exact-slug duplicate is always a hard refusal — no --force-new escape
    # hatch for this one (it isn't a "possible" duplicate, it IS the same
    # axis; re-declaring it is a Rejection/edit scenario, not axis creation).
    existing_slugs = {a.slug for a in g.axes}
    if slug in existing_slugs:
        print(
            f"ERROR: Axis '{slug}' already exists in the active domain's axes "
            f"tuple. Re-declaring an existing slug is not axis CREATION.",
            file=sys.stderr,
        )
        return 1

    matches = _axis_matches(g, slug, description, threshold=threshold)

    why = ""
    if matches and not force_new:
        top_score, top_slug, top_desc = matches[0]
        print(
            "ERROR: candidate axis is a likely near-duplicate of an existing "
            "axis (R-axis-gatekeeper-policy — mandatory gatekeeping, not "
            "optional). Nearest existing axis:\n"
            f"  {top_score:.4f}  {top_slug} — {top_desc}\n"
            "If this is genuinely a NEW tension dimension, re-run with "
            '--force-new "<justification for why this is distinct>".',
            file=sys.stderr,
        )
        return 1
    if matches and force_new:
        top_score, top_slug, _top_desc = matches[0]
        why = (
            f"--force-new override of axis-gatekeeper similarity match "
            f"(top: {top_slug} score={top_score:.4f}). Justification: "
            f"{force_new}"
        )
    elif force_new:
        # --force-new supplied but nothing matched — record it anyway so the
        # override intent is never silently dropped even when it turned out
        # to be unnecessary.
        why = f"--force-new supplied (no similarity match found). Justification: {force_new}"

    proposal = {
        "kind": "Axis",
        "slug": slug,
        "description": description,
        "why": why or f"New tension-dimension axis '{slug}' declared via create_axis.",
    }

    if dry_run:
        print("=== DRY RUN — ProposedAxis (not written) ===")
        print(json.dumps(proposal, indent=2, ensure_ascii=False))
        return 0

    with tempfile.NamedTemporaryFile(
        mode="w", suffix=".json", delete=False, encoding="utf-8"
    ) as tmp:
        json.dump(proposal, tmp, ensure_ascii=False)
        tmp_path = tmp.name

    try:
        result = subprocess.run(
            [sys.executable, str(_APPLY_PROPOSAL), tmp_path],
            capture_output=False,
        )
        return result.returncode
    finally:
        Path(tmp_path).unlink(missing_ok=True)


def main(argv: list[str] | None = None) -> int:
    """CLI entry point for create_axis.py."""
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(
        description=(
            "Scaffold a new Axis into the active domain's controlled-vocabulary "
            "axes tuple, gated by a MANDATORY confront-style similarity check "
            "(R-axis-gatekeeper-policy)."
        )
    )
    parser.add_argument(
        "slug",
        help=(
            "Kebab-case axis slug (e.g. 'cost-vs-flexibility'). "
            "Lowercase letters, digits, hyphens only, starting with a letter."
        ),
    )
    parser.add_argument(
        "--description",
        required=True,
        help="What the two poles of this tension are.",
    )
    parser.add_argument(
        "--threshold",
        type=float,
        default=_DEFAULT_THRESHOLD,
        help=f"Similarity refusal threshold (default {_DEFAULT_THRESHOLD}).",
    )
    parser.add_argument(
        "--force-new",
        metavar="JUSTIFICATION",
        default="",
        help=(
            "Override a gatekeeper refusal with a recorded justification "
            "(never silent — folded into the proposal's why field)."
        ),
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print the ProposedAxis JSON without applying it.",
    )
    args = parser.parse_args(argv)

    return scaffold(
        slug=args.slug,
        description=args.description,
        threshold=args.threshold,
        force_new=args.force_new,
        dry_run=args.dry_run,
    )


if __name__ == "__main__":
    sys.exit(main())
