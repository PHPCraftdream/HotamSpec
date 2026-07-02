"""Canon: §Loop — the CONFRONT step's tool: ranks a candidate claim's lexical overlap against SETTLED reality and REJECTED history before anything is written.

Mediation-loop step 3 (CONFRONT) mechanized: every other step already has a
tool (ORIENT: what_now/emit_cipher · LOCATE: gen_spec's indexes · TRANSLATE/
LAND: apply_proposal · verify: closure). This tool closes the gap: given a
candidate claim, it surfaces

  1. "possibly re-derives R-x (SETTLED)"  — the claim substantially overlaps
     an already-SETTLED claim (don't re-derive; cite it), and
  2. "possibly relitigates R-y (REJECTED)" — the claim substantially overlaps
     a REJECTED requirement's claim/why; the output names the replacement ids
     parsed from the 'REPLACES …' marker (anti-relitigation:
     R-rejected-preserved-not-deleted, RECENTLY-REJECTED discipline).

HEURISTIC, advisory only: lexical token/stem overlap (stdlib, deterministic),
not semantic proof. The operator/steward judges; the tool never blocks
(exit code 0 always). Writing nothing remains a valid CONFRONT outcome.

Run (from spec/):
  uv run python tools/confront.py "the framework shall ship no business data"
  echo "claim text" | uv run python tools/confront.py -
  uv run python tools/confront.py --demo "orders ship within 24 hours"

Deterministic: sorted output, fixed score formatting, LF, utf-8, no timestamps.
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path

# --- Make the hotam_spec package importable ------------------------------------

SPEC_ROOT = Path(__file__).resolve().parents[1]
if str(SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(SPEC_ROOT / "src"))

from hotam_spec.graph import TensionGraph, load_content_graph  # noqa: E402
from hotam_spec.requirement import REJECTED, SETTLED  # noqa: E402

# ---------------------------------------------------------------------------
# Tokenization (stdlib, deterministic)
# ---------------------------------------------------------------------------

_WORD_RE = re.compile(r"[a-z0-9][a-z0-9_-]*")

#: Words carrying no discriminating signal in requirement prose.
_STOPWORDS = frozenset(
    {
        "a", "an", "the", "and", "or", "not", "no", "of", "in", "on", "to",
        "for", "by", "with", "as", "at", "be", "is", "are", "was", "were",
        "it", "its", "this", "that", "these", "those", "shall", "must",
        "should", "may", "can", "will", "never", "always", "every", "each",
        "any", "all", "one", "two", "own", "via", "per", "vs", "into",
        "from", "when", "than", "then", "so", "if", "but", "only", "yet",
        "system", "methodology", "framework", "requirement", "requirements",
    }
)


def _stem(token: str) -> str:
    """Cheap deterministic stemmer: strip common English suffixes.

    Good enough to unify 'clusters/clustering/clustered' without pulling a
    dependency; intentionally conservative (short tokens untouched).
    """
    for suffix in ("ing", "edly", "ed", "es", "s", "ly"):
        if len(token) > len(suffix) + 3 and token.endswith(suffix):
            return token[: -len(suffix)]
    return token


def _tokens(text: str) -> frozenset[str]:
    """Lowercased, stopword-free, stemmed token set of a prose text."""
    return frozenset(
        _stem(t)
        for t in _WORD_RE.findall(text.lower())
        if t not in _STOPWORDS and len(t) > 2
    )


def _score(input_tokens: frozenset[str], target_tokens: frozenset[str]) -> float:
    """Overlap score in [0,1]: mean of input-containment and Jaccard.

    Containment (|I∩T|/|I|) catches a short claim re-deriving a long one;
    Jaccard tempers it so a single shared rare word does not dominate.
    Rounded to 4 places for deterministic rendering.
    """
    if not input_tokens or not target_tokens:
        return 0.0
    inter = len(input_tokens & target_tokens)
    containment = inter / len(input_tokens)
    jaccard = inter / len(input_tokens | target_tokens)
    return round((containment + jaccard) / 2, 4)


_REPLACES_ID_RE = re.compile(r"R-[a-z0-9][a-z0-9-]*")


def _replacements_from_why(why: str) -> tuple[str, ...]:
    """Extract replacement R-ids from a REJECTED requirement's 'REPLACES …' text.

    Reads the segment after the first 'REPLACES' marker up to the first
    sentence boundary and collects the R-ids named there, preserving first-seen
    order (the anti-relitigation pointer: cite these instead of re-deriving).
    """
    marker = why.find("REPLACES")
    if marker == -1:
        return ()
    segment = why[marker : marker + 400]
    seen: list[str] = []
    for rid in _REPLACES_ID_RE.findall(segment):
        if rid not in seen:
            seen.append(rid)
    return tuple(seen)


# ---------------------------------------------------------------------------
# Confrontation (pure)
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class Match:
    """One ranked confrontation hit."""

    rid: str
    score: float
    status: str  # SETTLED | REJECTED
    claim: str
    replaced_by: tuple[str, ...] = ()  # only for REJECTED


def confront(g: TensionGraph, text: str, *, min_score: float = 0.15) -> list[Match]:
    """Rank the graph's SETTLED claims and REJECTED history against `text`.

    Returns matches with score >= min_score, sorted by (-score, rid) —
    deterministic. SETTLED requirements are matched on their claim; REJECTED
    requirements on claim + why (the why carries the REPLACES pointer and the
    original rationale, which is where relitigation is usually visible).
    """
    input_tokens = _tokens(text)
    out: list[Match] = []
    for r in g.requirements:
        if r.status == SETTLED:
            score = _score(input_tokens, _tokens(r.claim))
            if score >= min_score:
                out.append(Match(rid=r.id, score=score, status=SETTLED, claim=r.claim))
        elif r.status == REJECTED:
            score = _score(input_tokens, _tokens(r.claim + " " + r.why))
            if score >= min_score:
                out.append(
                    Match(
                        rid=r.id,
                        score=score,
                        status=REJECTED,
                        claim=r.claim,
                        replaced_by=_replacements_from_why(r.why),
                    )
                )
    out.sort(key=lambda m: (-m.score, m.rid))
    return out


def render(matches: list[Match], *, text: str, top: int = 8) -> str:
    """Deterministic human-readable confrontation report (LF)."""
    head = text.strip().replace("\n", " ")
    if len(head) > 72:
        head = head[:69] + "..."
    lines: list[str] = [f"== confront: {head!r} =="]
    settled = [m for m in matches if m.status == SETTLED][:top]
    rejected = [m for m in matches if m.status == REJECTED][:top]

    lines.append("")
    lines.append("-- possibly re-derives (SETTLED — cite, don't re-derive) --")
    if settled:
        for m in settled:
            claim = m.claim if len(m.claim) <= 96 else m.claim[:93] + "..."
            lines.append(f"  {m.score:.4f}  {m.rid} — {claim}")
    else:
        lines.append("  (none above threshold)")

    lines.append("")
    lines.append("-- possibly relitigates (REJECTED — cite the replacement) --")
    if rejected:
        for m in rejected:
            repl = ", ".join(m.replaced_by) if m.replaced_by else "(no REPLACES marker)"
            lines.append(f"  {m.score:.4f}  {m.rid} — REJECTED, replaced by: {repl}")
    else:
        lines.append("  (none above threshold)")

    if not settled and not rejected:
        lines.append("")
        lines.append(
            "No substantial overlap found — likely novel; proceed to TRANSLATE "
            "(the heuristic is lexical: a semantic collision can still exist)."
        )
    lines.append("")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------


def _load_graph(*, demo: bool) -> TensionGraph:
    """Return the graph to confront against: demo fixture or domain content."""
    if demo:
        tests_dir = str(SPEC_ROOT / "tests")
        if tests_dir not in sys.path:
            sys.path.insert(0, tests_dir)
        from fixtures.seed import seed_graph  # noqa: PLC0415

        return seed_graph()
    return load_content_graph()


def main(argv: list[str] | None = None) -> int:
    """CONFRONT a candidate claim against the graph; print the ranked report."""
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument(
        "claim",
        nargs="*",
        help="candidate claim text ('-' or empty reads stdin)",
    )
    parser.add_argument(
        "--demo",
        action="store_true",
        help="confront against the fixture demo graph instead of domain content.",
    )
    parser.add_argument(
        "--top",
        type=int,
        default=8,
        help="max matches printed per section (default 8).",
    )
    parser.add_argument(
        "--min-score",
        type=float,
        default=0.15,
        help="overlap threshold below which matches are dropped (default 0.15).",
    )
    args = parser.parse_args(argv)

    words = [w for w in args.claim if w != "-"]
    text = " ".join(words).strip()
    if not text:
        text = sys.stdin.read().strip()
    if not text:
        print("ERROR: no claim text given (argv or stdin).", file=sys.stderr)
        return 1

    g = _load_graph(demo=args.demo)
    matches = confront(g, text, min_score=args.min_score)
    sys.stdout.write(render(matches, text=text, top=args.top))
    return 0


if __name__ == "__main__":
    sys.exit(main())
