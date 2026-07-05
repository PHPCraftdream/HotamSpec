"""Tests for hotam_spec.text.short_form — the crystal's no-mid-word-cutoff guarantee."""

from __future__ import annotations

from hotam_spec.text import short_form


def test_first_sentence_from_multi() -> None:
    """Extract the first whole sentence from multi-sentence text."""
    text = "The system shall do X. It shall also do Y. And Z."
    assert short_form(text) == "The system shall do X."


def test_summary_overrides_text() -> None:
    """When summary is provided, it takes priority over text."""
    assert short_form("Long claim text here.", summary="Short.") == "Short."


def test_no_period_returns_entire_text() -> None:
    """Text without a sentence-ending period returns the full text."""
    text = "A claim with no period"
    assert short_form(text) == text


def test_empty_text() -> None:
    """Empty input returns empty output."""
    assert short_form("") == ""
    assert short_form("   ") == ""


def test_never_cuts_mid_word() -> None:
    """With max_chars, the result never cuts mid-word."""
    text = "The system shall provide fast responses."
    result = short_form(text, max_chars=20)
    # Should cut at word boundary, not mid-word
    assert not result.endswith("resp [...]")
    assert "[...]" in result
    # Every word before [...] should be complete
    words_before = result.replace(" [...]", "").split()
    original_words = text.split()
    for w in words_before:
        assert w in original_words or w.rstrip(".") in [
            ow.rstrip(".") for ow in original_words
        ]


def test_max_chars_with_short_text() -> None:
    """max_chars that fits the first sentence returns it unchanged."""
    text = "Short. And more."
    assert short_form(text, max_chars=200) == "Short."


def test_single_word_text_with_max_chars() -> None:
    """A single long word with max_chars still returns something meaningful."""
    text = "Supercalifragilisticexpialidocious"
    result = short_form(text, max_chars=10)
    # Should return the word + [...] since there's no space to cut at
    assert result == "Supercalifragilisticexpialidocious [...]"


def test_period_at_end_of_string() -> None:
    """A period at end-of-string (no trailing space) is a sentence boundary."""
    text = "The system shall do X."
    assert short_form(text) == "The system shall do X."


def test_period_followed_by_newline() -> None:
    """A period followed by newline is a sentence boundary."""
    text = "First sentence.\nSecond sentence."
    assert short_form(text) == "First sentence."


def test_summary_empty_string_not_used() -> None:
    """An empty summary string falls through to first-sentence logic."""
    text = "Claim one. Claim two."
    assert short_form(text, summary="") == "Claim one."
