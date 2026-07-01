"""Tests for tools/claude_md_diff_watch.py — CLAUDE.md diff-injection hook."""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "tools"))

from claude_md_diff_watch import _DIFF_LINE_CAP, build_payload  # noqa: E402


def test_first_run_creates_baseline_silently(tmp_path):
    claude_md = tmp_path / "CLAUDE.md"
    claude_md.write_text("line one\nline two\n", encoding="utf-8")
    snapshot = tmp_path / ".runtime" / "claude_md_snapshot.md"

    assert not snapshot.exists()

    additional, updated = build_payload(claude_md, snapshot)

    assert additional == ""
    assert not updated
    assert snapshot.exists()
    assert snapshot.read_text(encoding="utf-8") == "line one\nline two\n"


def test_no_change_emits_nothing(tmp_path):
    claude_md = tmp_path / "CLAUDE.md"
    content = "line one\nline two\n"
    claude_md.write_text(content, encoding="utf-8")
    snapshot = tmp_path / ".runtime" / "claude_md_snapshot.md"
    snapshot.parent.mkdir(parents=True)
    snapshot.write_text(content, encoding="utf-8")

    additional, updated = build_payload(claude_md, snapshot)

    assert additional == ""
    assert not updated


def test_change_emits_diff(tmp_path):
    claude_md = tmp_path / "CLAUDE.md"
    claude_md.write_text("line one\nline two\nline three\n", encoding="utf-8")
    snapshot = tmp_path / ".runtime" / "claude_md_snapshot.md"
    snapshot.parent.mkdir(parents=True)
    snapshot.write_text("line one\nline two\n", encoding="utf-8")

    additional, updated = build_payload(claude_md, snapshot)

    assert updated
    assert "CLAUDE.md changed since your last turn" in additional
    assert "+line three" in additional
    assert "---" in additional
    assert "+++" in additional


def test_large_change_emits_summary_not_full_diff(tmp_path):
    claude_md = tmp_path / "CLAUDE.md"
    old_lines = "\n".join(f"old line {i}" for i in range(200)) + "\n"
    new_lines = "\n".join(f"new line {i}" for i in range(200)) + "\n"
    claude_md.write_text(new_lines, encoding="utf-8")
    snapshot = tmp_path / ".runtime" / "claude_md_snapshot.md"
    snapshot.parent.mkdir(parents=True)
    snapshot.write_text(old_lines, encoding="utf-8")

    additional, updated = build_payload(claude_md, snapshot)

    assert updated
    assert "changed significantly" in additional
    assert "re-read the file" in additional
    # Must not dump the full diff.
    assert additional.count("\n") < _DIFF_LINE_CAP
    assert "old line 0" not in additional


def test_snapshot_updated_after_diff_shown(tmp_path):
    claude_md = tmp_path / "CLAUDE.md"
    claude_md.write_text("line one\nline two\nline three\n", encoding="utf-8")
    snapshot = tmp_path / ".runtime" / "claude_md_snapshot.md"
    snapshot.parent.mkdir(parents=True)
    snapshot.write_text("line one\nline two\n", encoding="utf-8")

    additional, updated = build_payload(claude_md, snapshot)
    assert updated

    # Subsequent identical run must be silent.
    additional2, updated2 = build_payload(claude_md, snapshot)
    assert additional2 == ""
    assert not updated2
    assert snapshot.read_text(encoding="utf-8") == claude_md.read_text(encoding="utf-8")
