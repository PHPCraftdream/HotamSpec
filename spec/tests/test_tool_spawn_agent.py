"""Tests for spec/tools/spawn_agent.py (R-tool-spawn-agent).

Verifies the CLI surface: refuse on unknown agent, refuse on missing CLAUDE.md,
refuse on missing --stamp, compose the correct prompt, write spawn-log.jsonl,
and produce deterministic output.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

SPEC_ROOT = Path(__file__).resolve().parents[1]
TOOLS_DIR = SPEC_ROOT / "tools"
SPAWN_AGENT = TOOLS_DIR / "spawn_agent.py"

# Make the tool importable for monkeypatching.
if str(TOOLS_DIR) not in sys.path:
    sys.path.insert(0, str(TOOLS_DIR))


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_agent(tmp_path: Path, name: str, *, with_claude_md: bool = True) -> Path:
    """Create a minimal agent directory under tmp_path/agents/<name>/."""
    agent_dir = tmp_path / "agents" / name
    agent_dir.mkdir(parents=True)
    if with_claude_md:
        (agent_dir / "CLAUDE.md").write_text(
            f"# Agent {name}\nMARKER-XYZ\nsome crystal content\n",
            encoding="utf-8",
        )
    return agent_dir


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_composite_prompt_contains_crystal_and_task(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture
) -> None:
    """stdout contains the agent's CLAUDE.md content and the --task text."""
    import spawn_agent

    agent_dir = _make_agent(tmp_path, "test-agent")
    runtime_dir = tmp_path / ".runtime"

    monkeypatch.setattr(spawn_agent, "_DOMAINS_ROOT", tmp_path)
    monkeypatch.setattr(spawn_agent, "_LEGACY_AGENTS_ROOT", tmp_path / "agents")
    monkeypatch.setattr(spawn_agent, "_RUNTIME_DIR", runtime_dir)

    rc = spawn_agent.main(
        [
            str(agent_dir),
            "--task",
            "do the thing",
            "--stamp",
            "2026-01-01T00:00:00Z",
        ]
    )
    assert rc == 0
    captured = capsys.readouterr()
    assert "MARKER-XYZ" in captured.out
    assert "do the thing" in captured.out
    assert "----- CRYSTAL BEGIN -----" in captured.out
    assert "----- CRYSTAL END -----" in captured.out
    assert "## Your task" in captured.out


def test_refuses_unknown_agent(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """Exit 1 when the agent path cannot be resolved."""
    import spawn_agent

    monkeypatch.setattr(spawn_agent, "_DOMAINS_ROOT", tmp_path)
    monkeypatch.setattr(spawn_agent, "_LEGACY_AGENTS_ROOT", tmp_path / "agents")
    monkeypatch.setattr(spawn_agent, "_RUNTIME_DIR", tmp_path / ".runtime")

    rc = spawn_agent.main(
        [
            "nonexistent/path/agent",
            "--task",
            "some task",
            "--stamp",
            "2026-01-01T00:00:00Z",
        ]
    )
    assert rc == 1


def test_refuses_missing_claude_md(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """Exit 1 when agent dir exists but has no CLAUDE.md."""
    import spawn_agent

    agent_dir = _make_agent(tmp_path, "no-crystal-agent", with_claude_md=False)

    monkeypatch.setattr(spawn_agent, "_DOMAINS_ROOT", tmp_path)
    monkeypatch.setattr(spawn_agent, "_LEGACY_AGENTS_ROOT", tmp_path / "agents")
    monkeypatch.setattr(spawn_agent, "_RUNTIME_DIR", tmp_path / ".runtime")

    rc = spawn_agent.main(
        [
            str(agent_dir),
            "--task",
            "some task",
            "--stamp",
            "2026-01-01T00:00:00Z",
        ]
    )
    assert rc == 1


def test_refuses_missing_stamp(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """Exit 1 when --stamp is omitted."""
    import spawn_agent

    agent_dir = _make_agent(tmp_path, "stamped-agent")

    monkeypatch.setattr(spawn_agent, "_DOMAINS_ROOT", tmp_path)
    monkeypatch.setattr(spawn_agent, "_LEGACY_AGENTS_ROOT", tmp_path / "agents")
    monkeypatch.setattr(spawn_agent, "_RUNTIME_DIR", tmp_path / ".runtime")

    rc = spawn_agent.main(
        [
            str(agent_dir),
            "--task",
            "some task",
            # no --stamp
        ]
    )
    assert rc == 1


def test_spawn_log_written(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture
) -> None:
    """After success the spawn-log.jsonl entry exists with correct fields."""
    import spawn_agent

    agent_dir = _make_agent(tmp_path, "log-agent")
    runtime_dir = tmp_path / ".runtime"

    monkeypatch.setattr(spawn_agent, "_DOMAINS_ROOT", tmp_path)
    monkeypatch.setattr(spawn_agent, "_LEGACY_AGENTS_ROOT", tmp_path / "agents")
    monkeypatch.setattr(spawn_agent, "_RUNTIME_DIR", runtime_dir)

    rc = spawn_agent.main(
        [
            str(agent_dir),
            "--task",
            "first line of task\nsecond line",
            "--stamp",
            "2026-06-29T12:00:00Z",
        ]
    )
    assert rc == 0

    log_path = runtime_dir / "spawn-log.jsonl"
    assert log_path.exists(), "spawn-log.jsonl should have been created"

    entries = [
        json.loads(line)
        for line in log_path.read_text(encoding="utf-8").splitlines()
        if line.strip()
    ]
    assert len(entries) == 1
    entry = entries[0]
    assert entry["stamp"] == "2026-06-29T12:00:00Z"
    assert entry["task_first_line"] == "first line of task"
    assert isinstance(entry["prompt_chars"], int)
    assert entry["prompt_chars"] > 0
    assert "agent" in entry
    # R-spawn-log-carries-isolation: default fields when --isolation/--mutating omitted.
    assert entry["isolation"] == "shared"
    assert entry["mutating"] is False


def test_spawn_log_carries_isolation_and_mutating_fields(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture
) -> None:
    """--isolation/--mutating flags are recorded verbatim on the log entry
    (R-spawn-log-carries-isolation)."""
    import spawn_agent

    agent_dir = _make_agent(tmp_path, "iso-agent")
    runtime_dir = tmp_path / ".runtime"

    monkeypatch.setattr(spawn_agent, "_DOMAINS_ROOT", tmp_path)
    monkeypatch.setattr(spawn_agent, "_LEGACY_AGENTS_ROOT", tmp_path / "agents")
    monkeypatch.setattr(spawn_agent, "_RUNTIME_DIR", runtime_dir)

    rc = spawn_agent.main(
        [
            str(agent_dir),
            "--task",
            "mutating task",
            "--stamp",
            "2026-07-02T00:00:00Z",
            "--isolation",
            "worktree",
            "--mutating",
        ]
    )
    assert rc == 0

    log_path = runtime_dir / "spawn-log.jsonl"
    entries = [
        json.loads(line)
        for line in log_path.read_text(encoding="utf-8").splitlines()
        if line.strip()
    ]
    assert len(entries) == 1
    assert entries[0]["isolation"] == "worktree"
    assert entries[0]["mutating"] is True


def test_composite_prompt_deterministic(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture
) -> None:
    """Same inputs → identical stdout bytes on two separate invocations."""
    import spawn_agent

    agent_dir = _make_agent(tmp_path, "det-agent")
    runtime_dir = tmp_path / ".runtime"

    monkeypatch.setattr(spawn_agent, "_DOMAINS_ROOT", tmp_path)
    monkeypatch.setattr(spawn_agent, "_LEGACY_AGENTS_ROOT", tmp_path / "agents")
    monkeypatch.setattr(spawn_agent, "_RUNTIME_DIR", runtime_dir)

    argv = [
        str(agent_dir),
        "--task",
        "determinism test",
        "--stamp",
        "2026-01-01T00:00:00Z",
    ]

    rc1 = spawn_agent.main(argv)
    out1 = capsys.readouterr().out

    rc2 = spawn_agent.main(argv)
    out2 = capsys.readouterr().out

    assert rc1 == 0
    assert rc2 == 0
    assert out1 == out2, "Composite prompt must be byte-stable across runs"


def test_log_only_writes_row_without_composing_prompt(
    tmp_path: Path, monkeypatch, capsys
) -> None:
    """--log-only appends a spawn-log row and prints NO crystal prompt, even for
    an agent that has NO on-disk CLAUDE.md (R-host-spawn-leaves-trace)."""
    import spawn_agent

    runtime_dir = tmp_path / ".runtime"
    monkeypatch.setattr(spawn_agent, "_DOMAINS_ROOT", tmp_path)
    monkeypatch.setattr(spawn_agent, "_LEGACY_AGENTS_ROOT", tmp_path / "agents")
    monkeypatch.setattr(spawn_agent, "_RUNTIME_DIR", runtime_dir)

    rc = spawn_agent.main(
        [
            "oh-fleet-worker",  # a logical host agent, no on-disk crystal
            "--task",
            "wave 10 move 2 audit\nsecond line ignored",
            "--stamp",
            "2026-07-03T00:00:00Z",
            "--log-only",
            "--mutating",
            "--isolation",
            "worktree",
        ]
    )
    assert rc == 0
    captured = capsys.readouterr()
    assert captured.out.strip() == "", "log-only must not print a crystal prompt"

    log_path = runtime_dir / "spawn-log.jsonl"
    assert log_path.exists()
    rows = [json.loads(ln) for ln in log_path.read_text().splitlines() if ln.strip()]
    assert len(rows) == 1
    row = rows[0]
    assert row["agent"] == "oh-fleet-worker"
    assert row["task_first_line"] == "wave 10 move 2 audit"
    assert row["prompt_chars"] == 0
    assert row["isolation"] == "worktree"
    assert row["mutating"] is True


def test_log_only_still_requires_stamp(tmp_path: Path, monkeypatch) -> None:
    """--log-only does not exempt --stamp (determinism still applies)."""
    import spawn_agent

    monkeypatch.setattr(spawn_agent, "_DOMAINS_ROOT", tmp_path)
    monkeypatch.setattr(spawn_agent, "_LEGACY_AGENTS_ROOT", tmp_path / "agents")
    monkeypatch.setattr(spawn_agent, "_RUNTIME_DIR", tmp_path / ".runtime")

    rc = spawn_agent.main(["some-agent", "--task", "t", "--log-only"])
    assert rc == 1


def test_r_tool_spawn_agent_in_framework_invariants(tmp_path: Path) -> None:
    """R-tool-spawn-agent appears in FRAMEWORK-INVARIANTS.md after gen_spec.

    Tool-derived requirements (Canon: §Agent — ...) are projected into the
    FRAMEWORK-INVARIANTS.md (relocated from root CLAUDE.md crystal).
    """
    import gen_spec as _gs  # noqa: PLC0415
    fi_path = _gs.GEN_DIR / "FRAMEWORK-INVARIANTS.md"
    if not fi_path.exists():
        pytest.skip("FRAMEWORK-INVARIANTS.md not yet generated — run gen_spec.py first")
    text = fi_path.read_text(encoding="utf-8")
    assert "R-tool-spawn-agent" in text, (
        "R-tool-spawn-agent must appear in FRAMEWORK-INVARIANTS.md after gen_spec. "
        "The Canon docstring in spawn_agent.py triggers auto-projection via "
        "R-tools-registry-generated (R-tool-is-its-own-requirement)."
    )
