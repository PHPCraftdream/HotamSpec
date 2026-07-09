"""Canon: §Graph — reusable CLAUDE.md sentinel-block operations.

RULE: the BEGIN/END sentinel-pair pattern (e.g. ``<!-- LIVE-STATE:BEGIN -->``
… ``<!-- LIVE-STATE:END -->``) is the structural backbone of every generated
CLAUDE.md — root and agent alike. Three operations recur:

  1. ``wrap_block(name, content)`` — wrap content in its sentinel pair.
  2. ``extract_block(text, name)`` — pull the inner text between sentinels.
  3. ``replace_block(text, name, content)`` — splice new content between
     sentinels, preserving the surrounding text byte-for-byte.

Before this module, these three operations were inlined ad-hoc in
``gen_spec.py`` (``_wrap()``, ``extract_live_state_block()``, and manual
``text.find(_X_BEGIN)`` / slicing repeated in
``_update_agent_shared_docs_block`` and the agent CONSTITUTION/OVERLAP
updater). Centralizing them here makes the sentinel contract explicit and
reusable — any new CLAUDE.md block renderer gets wrap/extract/replace for
free instead of re-implementing the string slicing.

The sentinel-name convention is fixed: ``<!-- <NAME>:BEGIN -->`` and
``<!-- <NAME>:END -->``, uppercase, hyphen-separated. Callers pass the NAME
(e.g. ``"LIVE-STATE"``, ``"CONSTITUTION"``); this module synthesizes the
full sentinel strings.

stdlib-only, no filesystem I/O (pure string functions), no imports of
domain-specific code.
"""

from __future__ import annotations


def begin_sentinel(name: str) -> str:
    """Return the ``<!-- <NAME>:BEGIN -->`` sentinel for a block name.

    Canon: §Graph — begin-sentinel synthesizer.
    """
    return f"<!-- {name}:BEGIN -->"


def end_sentinel(name: str) -> str:
    """Return the ``<!-- <NAME>:END -->`` sentinel for a block name.

    Canon: §Graph — end-sentinel synthesizer.
    """
    return f"<!-- {name}:END -->"


def wrap_block(name: str, content: str) -> str:
    """Wrap ``content`` in the ``<NAME>`` sentinel pair (BEGIN\\n<content>\\nEND).

    Canon: §Graph — sentinel-pair wrapper.

    The content is placed on its own line between the two sentinels,
    matching the historical ``_wrap()`` output exactly:

        <!-- NAME:BEGIN -->
        <content>
        <!-- NAME:END -->
    """
    return f"{begin_sentinel(name)}\n{content}\n{end_sentinel(name)}"


def extract_block(text: str, name: str) -> str | None:
    """Return the text between the ``<NAME>`` sentinels (excluding sentinels).

    Canon: §Graph — sentinel-bounded extraction.

    Strips leading/trailing newlines from the inner text. Returns ``None``
    if either sentinel is absent or the END sentinel precedes the BEGIN
    sentinel (malformed).
    """
    begin = begin_sentinel(name)
    end = end_sentinel(name)
    begin_pos = text.find(begin)
    end_pos = text.find(end)
    if begin_pos == -1 or end_pos == -1 or end_pos <= begin_pos:
        return None
    inner = text[begin_pos + len(begin) : end_pos]
    return inner.strip("\n")


def replace_block(text: str, name: str, content: str) -> str:
    """Splice ``content`` between the ``<NAME>`` sentinels in ``text``.

    Canon: §Graph — sentinel-bounded replacement.

    Preserves everything before BEGIN and after END byte-for-byte. The
    content is placed on its own line between the sentinels. Raises
    ``ValueError`` if either sentinel is absent — callers that need
    insert-if-absent semantics should check ``extract_block`` first and
    use ``insert_block_after``.
    """
    begin = begin_sentinel(name)
    end = end_sentinel(name)
    begin_pos = text.find(begin)
    end_pos = text.find(end)
    if begin_pos == -1 or end_pos == -1:
        raise ValueError(
            f"CLAUDE.md block '{name}' sentinels not found "
            f"('{begin}' / '{end}'). Manual corruption suspected."
        )
    before = text[: begin_pos + len(begin)]
    after = text[end_pos:]
    return before + "\n" + content + "\n" + after


def insert_block_after(text: str, after_name: str, name: str, content: str) -> str:
    """Insert a new ``<NAME>`` block immediately after the ``<after_name>`` END sentinel.

    Canon: §Graph — sentinel-block insertion.

    Used when a block may not exist yet (e.g. inserting an OVERLAP block
    after CONSTITUTION:END in an agent CLAUDE.md that was scaffolded before
    OVERLAP existed). The new block is placed on a new line after the
    ``after_name`` END sentinel. Raises ``ValueError`` if the ``after_name``
    END sentinel is absent.
    """
    end = end_sentinel(after_name)
    pos = text.find(end)
    if pos == -1:
        raise ValueError(
            f"Anchor block '{after_name}' END sentinel not found "
            f"('{end}'). Cannot insert '{name}' block."
        )
    insert_at = pos + len(end)
    block = f"\n\n{wrap_block(name, content)}"
    return text[:insert_at] + block + text[insert_at:]
