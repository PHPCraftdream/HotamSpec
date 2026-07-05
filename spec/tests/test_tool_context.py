"""Tests for spec/tools/context.py — the working-context measurement reader.

W3b (context-cipher): the drop-file bridge spec/.runtime/context.json →
tools/context.py → LIVE-STATE. The READER half is verified here; the PRODUCER
(a hook writing the stamp) is still deferred, so R-measure-context-size stays
DRAFT — these tests pin the contract a future hook must satisfy:

    spec/.runtime/context.json = {
      "ctx_pct": <float 0..100>,   # working-context fullness
      "model":   "<model id>",     # optional
      "stamp":   "<iso8601>"       # optional, when the measurement was taken
    }

Honesty rule: absent or unreadable stamp → UNMEASURED, never a guessed number.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import context  # noqa: E402


def test_absent_stamp_reads_unmeasured(tmp_path: Path, monkeypatch) -> None:
    """No spec/.runtime/context.json → measured=False and the honest UNMEASURED line."""
    monkeypatch.setattr(context, "_RUNTIME", tmp_path / "context.json")
    s = context.read_context()
    assert s.measured is False
    assert s.pct is None
    line = context.render_line()
    assert line.startswith("context: UNMEASURED")
    assert "R-unmeasured-cipher-names-host-boundary" in line
    # Honest boundary: no command-to-call is named; the host is not touched.
    assert "R-work-within-launch-dir" in line
    assert "--patch-global" not in line


def test_valid_stamp_reads_measured(tmp_path: Path, monkeypatch) -> None:
    """A well-formed stamp yields measured=True with pct/model/stamp echoed."""
    stamp_file = tmp_path / "context.json"
    stamp_file.write_text(
        json.dumps(
            {"ctx_pct": 42.5, "model": "claude-fable-5", "stamp": "2026-07-02T10:00:00"}
        ),
        encoding="utf-8",
    )
    monkeypatch.setattr(context, "_RUNTIME", stamp_file)
    s = context.read_context()
    assert s.measured is True
    assert s.pct == 42.5
    assert s.model == "claude-fable-5"
    assert s.stamp == "2026-07-02T10:00:00"
    line = context.render_line()
    assert line.startswith("context: 42%")
    assert "claude-fable-5" in line


def test_corrupt_stamp_reads_unmeasured(tmp_path: Path, monkeypatch) -> None:
    """Unparseable JSON → UNMEASURED (never a guessed or stale number)."""
    stamp_file = tmp_path / "context.json"
    stamp_file.write_text("{not json", encoding="utf-8")
    monkeypatch.setattr(context, "_RUNTIME", stamp_file)
    s = context.read_context()
    assert s.measured is False
    assert "unreadable" in s.note


def test_stamp_without_pct_renders_unmeasured_line(tmp_path: Path, monkeypatch) -> None:
    """A stamp lacking ctx_pct renders the UNMEASURED cipher (missing = unmeasured)."""
    stamp_file = tmp_path / "context.json"
    stamp_file.write_text(json.dumps({"model": "m"}), encoding="utf-8")
    monkeypatch.setattr(context, "_RUNTIME", stamp_file)
    line = context.render_line()
    assert "UNMEASURED" in line
