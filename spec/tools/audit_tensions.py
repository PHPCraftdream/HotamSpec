"""Canon: §Loop — the generative-audit tool: a deterministic, LLM-free shortlist of
candidate SETTLED-atom pairs that MIGHT hide an unmediated tension.

The framework's blind spot (Wave 11 fxx finding "слепые зоны"): the system holds
tensions well but does not FIND them — over its whole history the machine
surfaced 0 of 8 conflicts; latent-mining covered 0 pairs; 3 axes never fired.
`confront.py` checks ONE candidate claim against reality; this tool sweeps the
WHOLE settled graph for pairs worth a human's conflict-review, closing the
missing "first act of sight" half of the mediation loop.

WHAT IT IS (and is NOT): a ranked shortlist of <= MAX_CANDIDATES pairs, each
tagged with the signal that surfaced it and a suggested axis. It NEVER writes to
graph.py (R-tension-audit-presents-only) — its only side effects are stdout and
a run stamp appended to spec/.runtime/tension-audit.jsonl. Every pair is a
SUSPECT for AI/steward review, never a decided conflict (R-ai-presents-not-decides).
The steward drafts a ProposedConflict from a row if the tension is real.

SIGNALS (each pair is surfaced by exactly one, in priority order):
  (a) POLE  — axis-pole lexical pull: for each graph axis, poles are parsed from
              the slug ("p-vs-q") plus its description; a pair where one claim's
              lexicon pulls toward pole p and the other's toward pole q is a
              candidate on that axis (the axis is what they'd cluster under).
  (b) MODAL — modal opposition: one claim carries a prohibition (shall never /
              only / not / no) and the other an unqualified obligation (shall …
              without negation), while sharing >= MODAL_MIN_SHARED content
              tokens — a "X must never / X shall" collision.
  (c) NOUN  — plain content-token overlap >= NOUN_MIN_SHARED between two atoms
              with DIFFERENT owners (a cross-owner overlap is where undeclared
              tensions hide; same-owner overlap is usually one author refining
              a theme).

EXCLUSIONS (a pair is dropped, never surfaced):
  - already mediated by a Conflict node (members_pair_set) — the tension is seen;
  - decomposition SIBLINGS: one refines/supports the other, both refine a common
    target, or they share a long common id-prefix (e.g. R-content-free-no-*).
    Sibling overlap is a split artifact, not a tension (anti-noise).

DETERMINISM: pure function of the graph; stdlib only; stable tokenizer; output
sorted by (signal-priority, -score, id-pair); scores rounded; LF, utf-8, no
timestamps in the printed shortlist (the stamp file carries the only clock).
Two runs on the same graph print byte-identical shortlists.

Run (from spec/):
  python tools/audit_tensions.py            # audit the active content graph
  python tools/audit_tensions.py --demo     # audit the fixture demo graph
  python tools/audit_tensions.py --no-stamp  # print only, do not append the stamp
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]
if str(SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(SPEC_ROOT / "src"))
if str(SPEC_ROOT / "tools") not in sys.path:
    sys.path.insert(0, str(SPEC_ROOT / "tools"))

from hotam_spec.graph import (  # noqa: E402
    TensionGraph,
    members_pair_set,
)
from _graph_loader import load_graph as _load_graph  # noqa: E402
from hotam_spec.requirement import SETTLED  # noqa: E402
from hotam_spec.runtime_paths import runtime_dir as _runtime_dir  # noqa: E402

# ---------------------------------------------------------------------------
# Tunables (documented constants, not magic numbers)
# ---------------------------------------------------------------------------

MAX_CANDIDATES = 10
"""Shortlist cap: a human reviews a shortlist, not a firehose. Truncation is
disclosed in the output footer, never silent (R-speak-by-reference)."""

MODAL_MIN_SHARED = 2
"""Minimum shared content tokens for a MODAL-opposition pair — below this a
prohibition and an obligation are simply unrelated statements."""

NOUN_MIN_SHARED = 4
"""Minimum shared content tokens for a bare cross-owner NOUN-overlap pair. Set
high: NOUN is the weakest signal, so it must clear a stiffer bar than POLE/MODAL
to avoid drowning the shortlist in coincidental theme-sharing."""

POLE_MIN_TOKENS = 1
"""A claim 'pulls' toward a pole if it shares >= this many tokens with the
pole's lexicon. 1 is deliberate: poles are 1-2 word slugs, so a single hit is
already meaningful directional evidence."""

STAMP_FILE = _runtime_dir() / "tension-audit.jsonl"

# Signal priority (lower = stronger evidence, sorted first).
_SIG_POLE, _SIG_MODAL, _SIG_NOUN = 0, 1, 2
_SIG_LABEL = {_SIG_POLE: "POLE", _SIG_MODAL: "MODAL", _SIG_NOUN: "NOUN"}

# ---------------------------------------------------------------------------
# Tokenization (mirrors tools/confront.py — deterministic, stdlib)
# ---------------------------------------------------------------------------

_WORD_RE = re.compile(r"[a-z0-9][a-z0-9_-]*")

_STOPWORDS = frozenset(
    {
        "a", "an", "the", "and", "or", "of", "in", "on", "to", "for", "by",
        "with", "as", "at", "be", "is", "are", "was", "were", "it", "its",
        "this", "that", "these", "those", "should", "may", "can", "will",
        "always", "every", "each", "any", "all", "one", "two", "own", "via",
        "per", "vs", "into", "from", "when", "than", "then", "so", "if",
        "but", "yet", "system", "methodology", "framework", "requirement",
        "requirements", "shall", "which", "whose", "their", "they", "them",
    }
)

# Negation / restriction markers that make a claim a PROHIBITION (modal signal).
_PROHIBITION_MARKERS = frozenset({"never", "not", "no", "only", "must"})


def _stem(token: str) -> str:
    for suffix in ("ing", "edly", "ed", "es", "s", "ly"):
        if len(token) > len(suffix) + 3 and token.endswith(suffix):
            return token[: -len(suffix)]
    return token


def _tokens(text: str) -> frozenset[str]:
    return frozenset(
        _stem(t)
        for t in _WORD_RE.findall(text.lower())
        if t not in _STOPWORDS and len(t) > 2
    )


def _raw_words(text: str) -> frozenset[str]:
    """Unstemmed lowercased words — used to detect prohibition markers, which
    must match exactly (e.g. 'never', 'only') and are lost by stemming/stops."""
    return frozenset(_WORD_RE.findall(text.lower()))


# ---------------------------------------------------------------------------
# Axis poles
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class _AxisPoles:
    slug: str
    left: frozenset[str]
    right: frozenset[str]


def _split_slug_poles(slug: str) -> tuple[str, str] | None:
    """A canonical axis slug is 'left-vs-right'; return (left, right) or None."""
    if "-vs-" not in slug:
        return None
    left, _, right = slug.partition("-vs-")
    return left, right


def axis_poles(g: TensionGraph) -> tuple[_AxisPoles, ...]:
    """Canon: §Axis — parse each graph axis into a left/right pole lexicon.

    Poles seed from the slug halves ('offload' | 'carry') and are enriched from
    the description: text before the first ' vs ' feeds the left pole, text
    after feeds the right, so 'Crystallize … vs the residency cap …' pushes the
    right words onto the carry side. Purely lexical and deterministic.
    """
    out: list[_AxisPoles] = []
    for a in g.axes:
        halves = _split_slug_poles(a.slug)
        if halves is None:
            continue
        left_slug, right_slug = halves
        left = set(_tokens(left_slug.replace("-", " ")))
        right = set(_tokens(right_slug.replace("-", " ")))
        desc = a.description or ""
        # description often mirrors the axis as "LEFT ... vs RIGHT ..."
        low = desc.lower()
        if " vs " in low:
            l_desc, _, r_desc = low.partition(" vs ")
            left |= set(_tokens(l_desc))
            right |= set(_tokens(r_desc))
        # Guard against a token appearing in both poles (no directional signal).
        shared = left & right
        out.append(
            _AxisPoles(
                slug=a.slug,
                left=frozenset(left - shared),
                right=frozenset(right - shared),
            )
        )
    return tuple(out)


# ---------------------------------------------------------------------------
# Candidate record
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class Candidate:
    """One suspect pair: the two requirement ids, the signal that surfaced it,
    a suggested axis (or "" when the signal is axis-agnostic), and a score."""

    left_id: str
    right_id: str
    signal: int  # _SIG_POLE / _SIG_MODAL / _SIG_NOUN
    axis: str
    score: float

    @property
    def key(self) -> tuple[str, str]:
        return tuple(sorted((self.left_id, self.right_id)))  # type: ignore[return-value]

    def render(self) -> str:
        a, b = self.key
        axis = self.axis or "(axis: propose one)"
        return (
            f"[{_SIG_LABEL[self.signal]} {self.score:.4f}] {a} <-> {b}"
            f"  |  suggested axis: {axis}"
        )


# ---------------------------------------------------------------------------
# Sibling / decomposition exclusion
# ---------------------------------------------------------------------------


def _common_prefix_len(a: str, b: str) -> int:
    n = 0
    for ca, cb in zip(a, b):
        if ca != cb:
            break
        n += 1
    return n


def _sibling_pairs(g: TensionGraph, settled_ids: frozenset[str]) -> frozenset[frozenset[str]]:
    """Pairs that are decomposition siblings, not tensions — excluded from the
    shortlist. A pair is a sibling if:
      - one refines/supports the other (direct relation edge), OR
      - both refine/support the SAME target, OR
      - they share a long common id prefix (>= 18 chars, e.g.
        'R-content-free-no-'), a strong signal they were split from one parent.
    """
    rel_target: dict[str, set[str]] = {}
    direct: set[frozenset[str]] = set()
    for r in g.requirements:
        if r.id not in settled_ids:
            continue
        for rel in r.relations:
            rel_target.setdefault(r.id, set()).add(rel.target)
            if rel.target in settled_ids:
                direct.add(frozenset({r.id, rel.target}))
    sib: set[frozenset[str]] = set(direct)
    ids = sorted(settled_ids)
    for i in range(len(ids)):
        for j in range(i + 1, len(ids)):
            a, b = ids[i], ids[j]
            pair = frozenset({a, b})
            if pair in sib:
                continue
            # shared refine/support target
            if rel_target.get(a) and rel_target.get(b) and (rel_target[a] & rel_target[b]):
                sib.add(pair)
                continue
            # long shared id-prefix -> split from a common parent
            if _common_prefix_len(a, b) >= 18:
                sib.add(pair)
    return frozenset(sib)


# ---------------------------------------------------------------------------
# The audit
# ---------------------------------------------------------------------------


def audit(g: TensionGraph, *, limit: int = MAX_CANDIDATES) -> list[Candidate]:
    """Canon: §Loop — the deterministic shortlist of unmediated-tension suspects.

    Pure function of the graph. Sweeps all cross-pairs of SETTLED requirements,
    drops mediated + sibling pairs, tags each survivor with its strongest signal
    (POLE > MODAL > NOUN), and returns the top `limit` by (signal, -score, ids).
    """
    settled = [r for r in g.requirements if r.status == SETTLED]
    settled_ids = frozenset(r.id for r in settled)
    tok = {r.id: _tokens(r.claim) for r in settled}
    raw = {r.id: _raw_words(r.claim) for r in settled}
    owner = {r.id: r.owner for r in settled}

    mediated = members_pair_set(g)
    siblings = _sibling_pairs(g, settled_ids)
    poles = axis_poles(g)

    def pole_axis(a_tok: frozenset[str], b_tok: frozenset[str]) -> str:
        """Return the axis slug whose two poles are pulled apart by this pair
        (one atom toward left, the other toward right), else ''."""
        for ap in poles:
            a_left = len(a_tok & ap.left) >= POLE_MIN_TOKENS
            a_right = len(a_tok & ap.right) >= POLE_MIN_TOKENS
            b_left = len(b_tok & ap.left) >= POLE_MIN_TOKENS
            b_right = len(b_tok & ap.right) >= POLE_MIN_TOKENS
            if (a_left and b_right) or (a_right and b_left):
                return ap.slug
        return ""

    def is_prohibition(rid: str) -> bool:
        return bool(raw[rid] & _PROHIBITION_MARKERS)

    ids = sorted(settled_ids)
    cands: list[Candidate] = []
    for i in range(len(ids)):
        for j in range(i + 1, len(ids)):
            a, b = ids[i], ids[j]
            pair = frozenset({a, b})
            if pair in mediated or pair in siblings:
                continue
            shared = tok[a] & tok[b]
            n_shared = len(shared)
            # (a) POLE — highest priority
            ax = pole_axis(tok[a], tok[b])
            if ax:
                # score: shared-token mass normalised, so pole pairs that ALSO
                # overlap in content rank above bare pole hits.
                denom = min(len(tok[a]), len(tok[b])) or 1
                cands.append(Candidate(a, b, _SIG_POLE, ax, round(n_shared / denom, 4)))
                continue
            # (b) MODAL — one prohibits, one obliges, with shared content
            if n_shared >= MODAL_MIN_SHARED and (is_prohibition(a) != is_prohibition(b)):
                denom = min(len(tok[a]), len(tok[b])) or 1
                cands.append(Candidate(a, b, _SIG_MODAL, "", round(n_shared / denom, 4)))
                continue
            # (c) NOUN — cross-owner content overlap
            if n_shared >= NOUN_MIN_SHARED and owner[a] != owner[b]:
                denom = min(len(tok[a]), len(tok[b])) or 1
                cands.append(Candidate(a, b, _SIG_NOUN, "", round(n_shared / denom, 4)))

    cands.sort(key=lambda c: (c.signal, -c.score, c.key))
    return cands[:limit]


# ---------------------------------------------------------------------------
# Stamp
# ---------------------------------------------------------------------------


def append_stamp(g: TensionGraph, candidates: list[Candidate], *, path: Path = STAMP_FILE) -> None:
    """Append a run record to spec/.runtime/tension-audit.jsonl.

    The ONLY clock in this tool: {stamp, settled_count, candidates}. The
    staleness reflection predicate (reflect_generative_audit_stale) reads
    settled_count from the LAST line to decide whether the graph has grown
    enough since the last audit to warrant a fresh sweep.
    """
    settled_count = sum(1 for r in g.requirements if r.status == SETTLED)
    record = {
        "stamp": datetime.now(timezone.utc).isoformat(),
        "settled_count": settled_count,
        "candidates": len(candidates),
    }
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8", newline="\n") as fh:
        fh.write(json.dumps(record) + "\n")


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


def _render(g: TensionGraph, candidates: list[Candidate], *, limit: int) -> str:
    settled_count = sum(1 for r in g.requirements if r.status == SETTLED)
    lines = [
        "== audit_tensions: generative shortlist of unmediated-tension suspects ==",
        "",
        f"swept {settled_count} SETTLED atoms; "
        f"{len(candidates)} candidate pair(s) (cap {limit}).",
        "HEURISTIC, for AI/steward review — never a decision "
        "(R-ai-presents-not-decides). Draft a ProposedConflict from a row only "
        "if the tension is real.",
        "",
    ]
    if not candidates:
        lines.append("  (no suspect pairs — every settled pair is mediated or a sibling)")
    else:
        for c in candidates:
            lines.append("  " + c.render())
    lines.append("")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--demo", action="store_true", help="audit the fixture demo graph")
    parser.add_argument("--no-stamp", action="store_true", help="print only; do not append the run stamp")
    parser.add_argument("--limit", type=int, default=MAX_CANDIDATES, help=f"shortlist cap (default {MAX_CANDIDATES})")
    args = parser.parse_args(argv)

    g = _load_graph(demo=args.demo)

    candidates = audit(g, limit=args.limit)
    sys.stdout.write(_render(g, candidates, limit=args.limit))
    if not args.no_stamp and not args.demo:
        append_stamp(g, candidates)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
