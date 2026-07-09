"""Canon: §Graph — tests for hotam_spec.claude_md sentinel-block helpers.

The wrap/extract/replace/insert operations are the structural backbone of
every generated CLAUDE.md. These tests pin their byte-exact output so a
silent change to the sentinel format is caught immediately.
"""

from __future__ import annotations

import pytest

from hotam_spec.claude_md import (
    begin_sentinel,
    end_sentinel,
    extract_block,
    insert_block_after,
    replace_block,
    wrap_block,
)


def test_begin_sentinel_format() -> None:
    """begin_sentinel(name) produces <!-- NAME:BEGIN -->."""
    assert begin_sentinel("LIVE-STATE") == "<!-- LIVE-STATE:BEGIN -->"


def test_end_sentinel_format() -> None:
    """end_sentinel(name) produces <!-- NAME:END -->."""
    assert end_sentinel("CONSTITUTION") == "<!-- CONSTITUTION:END -->"


def test_wrap_block_format() -> None:
    """wrap_block places content between sentinels on its own line."""
    result = wrap_block("FOO", "inner text")
    assert result == "<!-- FOO:BEGIN -->\ninner text\n<!-- FOO:END -->"


def test_extract_block_returns_inner_text() -> None:
    """extract_block returns the text between sentinels, stripped."""
    text = "<!-- FOO:BEGIN -->\nhello world\n<!-- FOO:END -->"
    assert extract_block(text, "FOO") == "hello world"


def test_extract_block_strips_leading_trailing_newlines() -> None:
    """extract_block strips leading/trailing \\n from the inner text."""
    text = "<!-- FOO:BEGIN -->\n\n\nhello\n\n\n<!-- FOO:END -->"
    assert extract_block(text, "FOO") == "hello"


def test_extract_block_returns_none_when_missing() -> None:
    """extract_block returns None when either sentinel is absent."""
    assert extract_block("no sentinels here", "FOO") is None


def test_extract_block_returns_none_when_end_before_begin() -> None:
    """extract_block returns None when END precedes BEGIN (malformed)."""
    text = "<!-- FOO:END -->\nhello\n<!-- FOO:BEGIN -->"
    assert extract_block(text, "FOO") is None


def test_replace_block_preserves_surrounding_text() -> None:
    """replace_block splices new content, preserving before/after."""
    text = "before\n<!-- FOO:BEGIN -->\nold\n<!-- FOO:END -->\nafter"
    result = replace_block(text, "FOO", "new")
    assert result == "before\n<!-- FOO:BEGIN -->\nnew\n<!-- FOO:END -->\nafter"


def test_replace_block_raises_when_sentinel_missing() -> None:
    """replace_block raises ValueError when sentinels are absent."""
    with pytest.raises(ValueError, match="FOO"):
        replace_block("no sentinels", "FOO", "new")


def test_insert_block_after_places_new_block() -> None:
    """insert_block_after inserts a new block right after the anchor END."""
    text = "<!-- ANCHOR:BEGIN -->\nstuff\n<!-- ANCHOR:END -->\ntail"
    result = insert_block_after(text, "ANCHOR", "NEW", "new content")
    assert "<!-- ANCHOR:END -->" in result
    assert "<!-- NEW:BEGIN -->\nnew content\n<!-- NEW:END -->" in result
    assert result.endswith("tail")


def test_insert_block_after_raises_when_anchor_missing() -> None:
    """insert_block_after raises ValueError when anchor END is absent."""
    with pytest.raises(ValueError, match="ANCHOR"):
        insert_block_after("no anchor", "ANCHOR", "NEW", "content")


def test_roundtrip_wrap_extract() -> None:
    """wrap then extract returns the original content."""
    content = "some multi\nline\ntext"
    wrapped = wrap_block("BLOCK", content)
    assert extract_block(wrapped, "BLOCK") == content
