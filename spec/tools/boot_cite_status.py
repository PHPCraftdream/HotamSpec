"""Canon: §Operator — Stop-hook writer + reader that lexically checks whether the operator's first sentence cites a typed anchor.

R-boot-cite-in-first-sentence (SETTLED, PROSE) claims the operator shall cite
at least one of R-/C-/A-/OP-/GOAL-/§ in the first sentence of every
substantive reply. That claim was pure prose discipline -- nothing measured
whether it actually happened turn over turn. This tool makes the FORM of the
claim measurable (not its substance -- see HONESTY NOTE below).

WRITER (Stop hook): Claude Code's Stop hook receives a JSON payload on stdin
shaped like {"transcript_path": "...", "session_id": "...", ...} (the same
"transcript_path" field context_producer.py's sibling hooks already rely on
via PostToolUse/Stop payloads -- see spec/tools/context_producer.py's own
docstring for the precedent). transcript_path points at a JSONL file where
each line is one turn; assistant turns have `type == "assistant"` and
`message.content` is a list of content blocks, the relevant ones being
`{"type": "text", "text": "..."}`. This tool reads that file, takes the LAST
assistant text block's first sentence (split on '.', '!', '?', or newline --
whichever comes first), and checks it for a typed-anchor token via regex
(R-/C-/A-/OP-/GOAL- prefix followed by a hyphenated slug, or a bare '§').
Result is appended to spec/.runtime/boot-cite-log.jsonl as
{"stamp": <iso8601>, "cited": <bool>, "first_sentence_chars": <int>}.

HONESTY NOTE -- FORM, NOT SUBSTANCE (R-ai-presents-not-decides discipline
applied to self-measurement): this is a LEXICAL check. It proves the first
sentence CONTAINS an anchor-shaped token; it does NOT prove the citation is
correct, relevant, or that the reply actually confronted graph reality
before writing (R-boot-cite-in-first-sentence's real intent). A reply could
still game this check by prefixing a real-shaped-but-irrelevant anchor with
no bearing on the content. This tool measures the citation RITUAL, not the
citation's TRUTH -- exactly the gap the mediation loop's CONFRONT step
(tools/confront.py) exists to narrow, which this tool does not attempt to
replace.

ANCHOR SHAPE (tightened): a typed anchor must carry >=2 hyphen-separated
slug segments after its prefix (R-anchor-everything mints multi-word
kebab-case slugs, e.g. "R-anchor-everything", never a single bare word) --
with two narrow legitimate single-segment exceptions: hex-hash Conflict
ids (e.g. "C-8600b1b8") and lowercase single-word Operator ids (e.g.
"OP-director", matching hotam_spec.operator's naming convention). This
rejects English words that merely happen to glue onto a prefix by
coincidence and are NOT typed anchors: "R-squared", "C-suite", "OP-ED",
"GOAL-oriented", "A-list" all match a looser single-segment pattern but
must NOT count as a citation ("OP-ED" fails the Operator-id exception too
-- all-caps, not the lowercase convention) -- loosening the regex to
accept them would silently inflate the compliance rate with false
positives, the same honesty failure this tool exists to avoid. The bare
section-sign (§) path is untouched.

READER: given the log, answer "what fraction of the last N logged replies
cited an anchor in their first sentence?" -- a burn-down/compliance meter,
not a gate. An empty/absent log is vacuously 0/0 (undefined rate, reported
as such, never fabricated as 100% or 0%).

Run (from spec/):
  # writer (as a Stop hook, receiving the hook JSON payload on stdin):
  uv run python tools/boot_cite_status.py write --stdin-file payload.json
  cat payload.json | uv run python tools/boot_cite_status.py write

  # reader:
  uv run python tools/boot_cite_status.py read                    # last 20, human-readable
  uv run python tools/boot_cite_status.py read --last 50 --json   # machine-readable
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_DEFAULT_LOG_PATH = _SPEC_ROOT / ".runtime" / "boot-cite-log.jsonl"

# R-/C-/A-/OP-/GOAL- prefix followed by >=2 hyphen-separated slug segments
# (each segment >=1 alnum char), or a bare section-sign. Requiring two
# segments (not one) rejects a bare adjective/gerund glued to the prefix by
# coincidence -- "R-squared", "C-suite", "OP-ED", "GOAL-oriented", "A-list"
# all lexically match a looser single-segment pattern but are not typed
# anchors (R-anchor-everything mints multi-word kebab-case slugs like
# R-anchor-everything itself, never a single bare word). "R-anchor-everything"
# (3 segments) and "C-8600b1b8" (1 hex segment... but hex hash ids are the
# one legitimate single-segment case, see below) still need to resolve.
_ANCHOR_RE = re.compile(
    r"(?:\b(?:R|C|A|OP|GOAL)-[A-Za-z0-9]+-[A-Za-z0-9-]+)"  # >=2 hyphen segments
    r"|(?:\bC-[0-9a-f]{6,}\b)"  # hex-hash Conflict ids (single segment, legitimate)
    r"|(?:\bOP-[a-z][a-z0-9]{2,}\b)"  # single-word lowercase Operator ids (e.g. OP-director)
    r"|§"
)

_SENTENCE_SPLIT_RE = re.compile(r"[.!?\n]")


def first_sentence(text: str) -> str:
    """Canon: §Operator — return the first sentence of text (split on . ! ? or newline).

    Pure helper: if no sentence-terminator is found, the whole (stripped)
    text is treated as one sentence. Leading/trailing whitespace stripped.
    """
    text = text.strip()
    if not text:
        return ""
    m = _SENTENCE_SPLIT_RE.search(text)
    if m is None:
        return text
    return text[: m.start()].strip()


def cites_anchor(sentence: str) -> bool:
    """Canon: §Operator — True iff sentence lexically contains an anchor token.

    FORM CHECK ONLY -- see module docstring's HONESTY NOTE. Matches an
    R-/C-/A-/OP-/GOAL- prefixed slug or a bare section-sign (§)."""
    return _ANCHOR_RE.search(sentence) is not None


def _read_payload(stdin_file: str | None) -> dict:
    """Read and parse the hook JSON payload from stdin or a file. Returns {} on failure."""
    try:
        if stdin_file:
            raw = Path(stdin_file).read_text(encoding="utf-8")
        else:
            raw = sys.stdin.read()
        if not raw.strip():
            return {}
        data = json.loads(raw)
        return data if isinstance(data, dict) else {}
    except (OSError, json.JSONDecodeError):
        return {}


def last_assistant_text(transcript_path: Path) -> str:
    """Canon: §Operator — extract the last assistant text block from a transcript JSONL.

    Reads the transcript file line by line (append-order JSONL, one turn per
    line). Returns the `text` field of the LAST content block of type
    "text" belonging to the LAST line with `type == "assistant"`. Returns
    "" if the file is missing, unreadable, malformed, or contains no
    assistant text block -- fail-silent, matching context_producer.py's
    honest-absence convention.
    """
    try:
        raw_lines = transcript_path.read_text(encoding="utf-8").splitlines()
    except OSError:
        return ""

    last_text = ""
    for line in raw_lines:
        line = line.strip()
        if not line:
            continue
        try:
            entry = json.loads(line)
        except json.JSONDecodeError:
            continue
        if entry.get("type") != "assistant":
            continue
        message = entry.get("message") or {}
        content = message.get("content")
        if not isinstance(content, list):
            continue
        for block in content:
            if isinstance(block, dict) and block.get("type") == "text":
                text = block.get("text", "")
                if isinstance(text, str) and text.strip():
                    last_text = text
    return last_text


def _append_boot_cite_log(log_path: Path, stamp: str, cited: bool, first_sentence_chars: int) -> None:
    log_path.parent.mkdir(parents=True, exist_ok=True)
    entry = {
        "stamp": stamp,
        "cited": cited,
        "first_sentence_chars": first_sentence_chars,
    }
    with log_path.open("a", encoding="utf-8", newline="\n") as fh:
        fh.write(json.dumps(entry, ensure_ascii=False) + "\n")


def write_from_payload(
    payload: dict,
    log_path: Path | None = None,
    *,
    stamp: str | None = None,
) -> bool | None:
    """Canon: §Operator — the writer half: payload -> boot-cite-log entry.

    Returns the `cited` bool that was written, or None if no transcript
    could be read (nothing written -- fail-silent, mirrors
    context_producer.py's produce() honest-absence contract). `stamp`
    defaults to current UTC time; tests should pass a fixed value for
    determinism.
    """
    transcript_path_str = payload.get("transcript_path")
    if not transcript_path_str:
        return None
    transcript_path = Path(transcript_path_str)
    text = last_assistant_text(transcript_path)
    if not text:
        return None

    sentence = first_sentence(text)
    cited = cites_anchor(sentence)

    target = log_path if log_path is not None else _DEFAULT_LOG_PATH
    actual_stamp = stamp or datetime.now(timezone.utc).isoformat(timespec="seconds")
    _append_boot_cite_log(target, actual_stamp, cited, len(sentence))
    return cited


@dataclass(frozen=True)
class BootCiteStatusResult:
    """Canon: §Operator — the outcome of a boot-cite compliance read.

    Fields:
      total       — number of log records considered (last N, or all if fewer).
      cited_count — how many of those had cited=true.
      rate        — cited_count / total, or None if total == 0 (undefined, not 0).
      reason      — human-readable explanation.
    """

    total: int
    cited_count: int
    rate: float | None
    reason: str = ""


def _read_records(log_path: Path) -> list[dict]:
    if not log_path.exists():
        return []
    records: list[dict] = []
    for line in log_path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            records.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    return records


def compute_boot_cite_status(log_path: Path | None = None, *, last: int = 20) -> BootCiteStatusResult:
    """Canon: §Operator — what fraction of the last `last` logged replies cited an anchor?

    Empty/absent log -> total=0, rate=None (undefined, never fabricated).
    """
    target = log_path if log_path is not None else _DEFAULT_LOG_PATH
    records = _read_records(target)

    if not records:
        return BootCiteStatusResult(
            total=0,
            cited_count=0,
            rate=None,
            reason="boot-cite-log is empty or absent — rate undefined (R-empty-content-wellformed).",
        )

    window = records[-last:] if last > 0 else records
    total = len(window)
    cited_count = sum(1 for r in window if r.get("cited") is True)
    rate = cited_count / total if total else None

    return BootCiteStatusResult(
        total=total,
        cited_count=cited_count,
        rate=rate,
        reason=f"{cited_count}/{total} of the last {total} logged reply(ies) cited an anchor in their first sentence.",
    )


def main(argv: list[str] | None = None) -> int:
    """Canon: §Operator — CLI entry point: `write` (Stop-hook mode) or `read` (status mode)."""
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(
        description=(
            "Stop-hook writer + reader for R-boot-cite-measured: lexical "
            "first-sentence anchor-citation check."
        )
    )
    sub = parser.add_subparsers(dest="mode", required=True)

    write_p = sub.add_parser("write", help="Read hook payload, append a boot-cite-log entry.")
    write_p.add_argument("--stdin-file", default=None, help="Read payload from this file instead of stdin.")
    write_p.add_argument("--log-path", default=None, help="Override the boot-cite-log.jsonl path.")

    read_p = sub.add_parser("read", help="Print compliance rate over the last N logged replies.")
    read_p.add_argument("--last", type=int, default=20, help="Window size (default 20).")
    read_p.add_argument("--log-path", default=None, help="Override the boot-cite-log.jsonl path.")
    read_p.add_argument("--json", action="store_true", help="Print machine-readable JSON.")

    args = parser.parse_args(argv)

    if args.mode == "write":
        payload = _read_payload(args.stdin_file)
        log_path = Path(args.log_path) if args.log_path else None
        write_from_payload(payload, log_path)
        return 0

    # mode == "read"
    log_path = Path(args.log_path) if args.log_path else None
    result = compute_boot_cite_status(log_path, last=args.last)
    if args.json:
        print(
            json.dumps(
                {
                    "total": result.total,
                    "cited_count": result.cited_count,
                    "rate": result.rate,
                    "reason": result.reason,
                },
                ensure_ascii=False,
            )
        )
    else:
        rate_str = f"{result.rate:.0%}" if result.rate is not None else "UNDEFINED (no data)"
        print(f"boot-cite compliance (last {result.total}): {rate_str}")
        print(f"reason: {result.reason}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
