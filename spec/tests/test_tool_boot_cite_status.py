"""Tests for spec/tools/boot_cite_status.py (R-boot-cite-measured).

Verifies the pure helpers (first_sentence, cites_anchor), the writer
(last_assistant_text + write_from_payload) against synthetic transcript
JSONL fixtures, the reader (compute_boot_cite_status), and the CLI surface.
Hermetic tmp_path throughout — never touches a real transcript or the real
spec/.runtime/boot-cite-log.jsonl.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

TOOLS_DIR = Path(__file__).resolve().parents[1] / "tools"
if str(TOOLS_DIR) not in sys.path:
    sys.path.insert(0, str(TOOLS_DIR))

import boot_cite_status as bcs  # noqa: E402


# ---------------------------------------------------------------------------
# first_sentence
# ---------------------------------------------------------------------------


def test_first_sentence_splits_on_period() -> None:
    assert bcs.first_sentence("Per R-anchor-everything, we proceed. Then more.") == (
        "Per R-anchor-everything, we proceed"
    )


def test_first_sentence_splits_on_newline() -> None:
    assert bcs.first_sentence("First line\nSecond line") == "First line"


def test_first_sentence_no_terminator_returns_whole_text() -> None:
    assert bcs.first_sentence("  no terminator here  ") == "no terminator here"


def test_first_sentence_empty() -> None:
    assert bcs.first_sentence("   ") == ""


# ---------------------------------------------------------------------------
# cites_anchor
# ---------------------------------------------------------------------------


def test_cites_anchor_r_prefix() -> None:
    assert bcs.cites_anchor("Per R-anchor-everything this holds") is True


def test_cites_anchor_section_sign() -> None:
    assert bcs.cites_anchor("See §Conflict for the model") is True


def test_cites_anchor_conflict_prefix() -> None:
    assert bcs.cites_anchor("Regarding C-8600b1b8 the steward decided") is True


def test_cites_anchor_no_anchor() -> None:
    assert bcs.cites_anchor("I think this looks fine, let's proceed") is False


def test_cites_anchor_goal_and_op_prefix() -> None:
    assert bcs.cites_anchor("GOAL-burn-down tracks this") is True
    assert bcs.cites_anchor("OP-director owns this") is True


def test_cites_anchor_rejects_r_squared() -> None:
    assert bcs.cites_anchor("The R-squared value looks good") is False


def test_cites_anchor_rejects_c_suite() -> None:
    assert bcs.cites_anchor("This is a C-suite decision") is False


def test_cites_anchor_rejects_op_ed() -> None:
    assert bcs.cites_anchor("Read the OP-ED in the paper") is False


def test_cites_anchor_rejects_goal_oriented() -> None:
    assert bcs.cites_anchor("A GOAL-oriented approach works well") is False


def test_cites_anchor_rejects_a_list() -> None:
    assert bcs.cites_anchor("This is an A-list requirement") is False


# ---------------------------------------------------------------------------
# last_assistant_text
# ---------------------------------------------------------------------------


def _transcript_line(kind: str, texts: list[str]) -> str:
    return json.dumps(
        {
            "type": kind,
            "message": {"content": [{"type": "text", "text": t} for t in texts]},
        }
    )


def test_last_assistant_text_picks_last_assistant_turn(tmp_path: Path) -> None:
    transcript = tmp_path / "t.jsonl"
    transcript.write_text(
        "\n".join(
            [
                _transcript_line("user", ["hello"]),
                _transcript_line("assistant", ["Per R-x-anchor this is the first reply."]),
                _transcript_line("user", ["ok next"]),
                _transcript_line("assistant", ["Second reply with no anchor."]),
            ]
        ),
        encoding="utf-8",
    )
    assert bcs.last_assistant_text(transcript) == "Second reply with no anchor."


def test_last_assistant_text_missing_file(tmp_path: Path) -> None:
    assert bcs.last_assistant_text(tmp_path / "nope.jsonl") == ""


def test_last_assistant_text_no_assistant_turns(tmp_path: Path) -> None:
    transcript = tmp_path / "t.jsonl"
    transcript.write_text(_transcript_line("user", ["hi"]), encoding="utf-8")
    assert bcs.last_assistant_text(transcript) == ""


def test_last_assistant_text_multiple_blocks_takes_last_text_block(tmp_path: Path) -> None:
    transcript = tmp_path / "t.jsonl"
    transcript.write_text(
        _transcript_line("assistant", ["thinking-ish first block", "final answer block"]),
        encoding="utf-8",
    )
    assert bcs.last_assistant_text(transcript) == "final answer block"


# ---------------------------------------------------------------------------
# write_from_payload
# ---------------------------------------------------------------------------


def test_write_from_payload_cited_true(tmp_path: Path) -> None:
    transcript = tmp_path / "t.jsonl"
    transcript.write_text(
        _transcript_line("assistant", ["Per R-anchor-everything we proceed."]),
        encoding="utf-8",
    )
    log_path = tmp_path / "boot-cite-log.jsonl"
    result = bcs.write_from_payload(
        {"transcript_path": str(transcript)}, log_path, stamp="2026-07-02T00:00:00Z"
    )
    assert result is True
    entries = [json.loads(l) for l in log_path.read_text(encoding="utf-8").splitlines()]
    assert len(entries) == 1
    assert entries[0]["cited"] is True
    assert entries[0]["stamp"] == "2026-07-02T00:00:00Z"
    assert isinstance(entries[0]["first_sentence_chars"], int)


def test_write_from_payload_cited_false(tmp_path: Path) -> None:
    transcript = tmp_path / "t.jsonl"
    transcript.write_text(
        _transcript_line("assistant", ["No anchor here at all, just prose."]),
        encoding="utf-8",
    )
    log_path = tmp_path / "boot-cite-log.jsonl"
    result = bcs.write_from_payload(
        {"transcript_path": str(transcript)}, log_path, stamp="2026-07-02T00:01:00Z"
    )
    assert result is False
    entries = [json.loads(l) for l in log_path.read_text(encoding="utf-8").splitlines()]
    assert entries[0]["cited"] is False


def test_write_from_payload_no_transcript_path_writes_nothing(tmp_path: Path) -> None:
    log_path = tmp_path / "boot-cite-log.jsonl"
    result = bcs.write_from_payload({}, log_path)
    assert result is None
    assert not log_path.exists()


def test_write_from_payload_missing_transcript_file_writes_nothing(tmp_path: Path) -> None:
    log_path = tmp_path / "boot-cite-log.jsonl"
    result = bcs.write_from_payload(
        {"transcript_path": str(tmp_path / "absent.jsonl")}, log_path
    )
    assert result is None
    assert not log_path.exists()


def test_write_from_payload_appends(tmp_path: Path) -> None:
    transcript = tmp_path / "t.jsonl"
    log_path = tmp_path / "boot-cite-log.jsonl"

    transcript.write_text(_transcript_line("assistant", ["R-foo-bar cited."]), encoding="utf-8")
    bcs.write_from_payload({"transcript_path": str(transcript)}, log_path, stamp="s1")

    transcript.write_text(_transcript_line("assistant", ["no anchor."]), encoding="utf-8")
    bcs.write_from_payload({"transcript_path": str(transcript)}, log_path, stamp="s2")

    entries = [json.loads(l) for l in log_path.read_text(encoding="utf-8").splitlines()]
    assert len(entries) == 2
    assert entries[0]["cited"] is True
    assert entries[1]["cited"] is False


# ---------------------------------------------------------------------------
# compute_boot_cite_status
# ---------------------------------------------------------------------------


def _write_log(path: Path, entries: list[dict]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="\n") as fh:
        for e in entries:
            fh.write(json.dumps(e) + "\n")


def test_compute_status_empty_log_is_undefined(tmp_path: Path) -> None:
    result = bcs.compute_boot_cite_status(tmp_path / "absent.jsonl")
    assert result.total == 0
    assert result.rate is None


def test_compute_status_all_cited(tmp_path: Path) -> None:
    log_path = tmp_path / "log.jsonl"
    _write_log(
        log_path,
        [{"stamp": f"s{i}", "cited": True, "first_sentence_chars": 10} for i in range(3)],
    )
    result = bcs.compute_boot_cite_status(log_path)
    assert result.total == 3
    assert result.cited_count == 3
    assert result.rate == 1.0


def test_compute_status_mixed_and_windowed(tmp_path: Path) -> None:
    log_path = tmp_path / "log.jsonl"
    entries = [{"stamp": f"s{i}", "cited": (i % 2 == 0), "first_sentence_chars": 5} for i in range(10)]
    _write_log(log_path, entries)
    result = bcs.compute_boot_cite_status(log_path, last=4)
    assert result.total == 4
    # last 4 entries: indices 6,7,8,9 -> cited True,False,True,False -> 2/4
    assert result.cited_count == 2
    assert result.rate == 0.5


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


def test_main_write_then_read(tmp_path: Path, capsys) -> None:
    transcript = tmp_path / "t.jsonl"
    transcript.write_text(_transcript_line("assistant", ["R-x-cited here."]), encoding="utf-8")
    payload_path = tmp_path / "payload.json"
    payload_path.write_text(json.dumps({"transcript_path": str(transcript)}), encoding="utf-8")
    log_path = tmp_path / "log.jsonl"

    rc = bcs.main(["write", "--stdin-file", str(payload_path), "--log-path", str(log_path)])
    assert rc == 0
    assert log_path.exists()

    rc2 = bcs.main(["read", "--log-path", str(log_path), "--json"])
    assert rc2 == 0
    payload = json.loads(capsys.readouterr().out)
    assert payload["total"] == 1
    assert payload["cited_count"] == 1
    assert payload["rate"] == 1.0
