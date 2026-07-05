"""Canon: §Requirement — text helpers for crystal rendering (stdlib-only).

RULE (R-crystal-carries-short-form): the crystal generator shall never
mechanically truncate text mid-word; every rendered object carries a
meaningful short form.  short_form() is the single implementation of that
guarantee.

WHY a separate module (not inlined in gen_spec): the helper is stdlib-only,
unit-testable in isolation, and reusable by any consumer that needs a
human-readable abbreviation of a claim or description.
"""

from __future__ import annotations

import re as _re

# Sentence boundary: period followed by a space, newline, or end-of-string.
_SENTENCE_END = _re.compile(r"\.\s|\.\Z")


def short_form(text: str, summary: str = "", max_chars: int | None = None) -> str:
    """Canon: §Requirement — return a meaningful short form of *text*, never cutting mid-word.

    RULE: if *summary* is non-empty, return it verbatim (the author's own
    abbreviation takes priority).  Otherwise extract the first whole sentence
    of *text* (boundary: '. ' or '.\\n' or '.' at end-of-string).  If *text*
    contains no sentence-ending period, return the entire text.

    When *max_chars* is given and the result exceeds it, trim to the last
    whole word that fits and append ' [...]'.  A *max_chars* that is too small
    to hold even one word returns the first word plus ' [...]'.

    The function NEVER produces a mid-word cutoff.
    """
    if summary:
        return summary

    text = text.strip()
    if not text:
        return ""

    # Extract first sentence.
    m = _SENTENCE_END.search(text)
    if m is not None:
        result = text[: m.start() + 1]  # include the period
    else:
        result = text  # no period -- return entire text

    if max_chars is not None and len(result) > max_chars:
        # Trim to last whole word boundary.
        truncated = result[:max_chars]
        last_space = truncated.rfind(" ")
        if last_space > 0:
            result = truncated[:last_space] + " [...]"
        else:
            # Single long word -- take the first word of the original.
            first_space = result.find(" ")
            if first_space > 0:
                result = result[:first_space] + " [...]"
            else:
                result = result + " [...]"

    return result
