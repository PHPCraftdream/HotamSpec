"""Tests for spec/tools/create_domain.py — domain directory scaffolder.

Uses tmp_path to isolate all file creation from the real domains/ directory.
The director agent is created via a real subprocess call to create_agent.py
(no mocking needed — both tools are pure file-writers with no side-effects).
"""

from __future__ import annotations

import sys
from pathlib import Path

_TOOLS = Path(__file__).resolve().parents[1] / "tools"
if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))

import create_domain  # noqa: E402


# ---------------------------------------------------------------------------
# Helper
# ---------------------------------------------------------------------------


def _scaffold(
    tmp_path: Path,
    name: str = "test-domain",
    description: str = "A test domain.",
    goals: str = "Goal one;Goal two",
    director_purpose: str = "Stewards the test domain.",
) -> int:
    """Invoke scaffold() with tmp_path as domains_root."""
    return create_domain.scaffold(
        name=name,
        description=description,
        goals=create_domain._goals_from_str(goals),
        director_purpose=director_purpose,
        domains_root=tmp_path,
    )


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_creates_required_files(tmp_path: Path) -> None:
    """All required paths exist after a successful scaffold."""
    rc = _scaffold(tmp_path, name="my-domain")
    assert rc == 0

    d = tmp_path / "my-domain"
    assert (d / "manifest.py").is_file()
    assert (d / "graph.py").is_file()
    assert (d / "CLAUDE.md").is_file()
    assert (d / "tools").is_dir()
    assert (d / "agents").is_dir()
    assert (d / "docs" / "gen").is_dir()


def test_refuses_existing_domain(tmp_path: Path) -> None:
    """Returns exit code 1 if domain directory already exists."""
    (tmp_path / "already-exists").mkdir()
    rc = _scaffold(tmp_path, name="already-exists")
    assert rc == 1


def test_refuses_invalid_name(tmp_path: Path) -> None:
    """Returns exit code 1 for names that fail validation."""
    invalid_names = ["Foo", "with space", "with/slash"]
    for bad in invalid_names:
        rc = _scaffold(tmp_path, name=bad)
        assert rc == 1, f"Expected rc=1 for name={bad!r}, got {rc}"


def test_refuses_missing_args(tmp_path: Path) -> None:
    """main() returns exit code 1 when required args are omitted."""
    import subprocess

    # Missing --description
    result = subprocess.run(
        [
            sys.executable,
            str(_TOOLS / "create_domain.py"),
            "some-domain",
            "--goals",
            "G1",
            "--director-purpose",
            "p",
        ],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 1
    assert "--description" in result.stderr

    # Missing --goals
    result = subprocess.run(
        [
            sys.executable,
            str(_TOOLS / "create_domain.py"),
            "some-domain",
            "--description",
            "D",
            "--director-purpose",
            "p",
        ],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 1
    assert "--goals" in result.stderr

    # Missing --director-purpose
    result = subprocess.run(
        [
            sys.executable,
            str(_TOOLS / "create_domain.py"),
            "some-domain",
            "--description",
            "D",
            "--goals",
            "G1",
        ],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 1
    assert "--director-purpose" in result.stderr


def test_manifest_fields_present(tmp_path: Path) -> None:
    """manifest.py has non-empty ID, DESCRIPTION, GOALS, DIRECTOR."""
    rc = _scaffold(
        tmp_path,
        name="manifest-domain",
        description="Testing manifest fields.",
        goals="Track tensions;Resolve conflicts",
    )
    assert rc == 0

    manifest_text = (tmp_path / "manifest-domain" / "manifest.py").read_text(
        encoding="utf-8"
    )
    assert 'ID = "manifest-domain"' in manifest_text
    assert 'DESCRIPTION = "Testing manifest fields."' in manifest_text
    assert '"Track tensions"' in manifest_text
    assert '"Resolve conflicts"' in manifest_text
    assert 'DIRECTOR = "director"' in manifest_text


def test_director_agent_created(tmp_path: Path) -> None:
    """Director agent directory is fully scaffolded inside agents/director/."""
    rc = _scaffold(tmp_path, name="director-domain")
    assert rc == 0

    director = tmp_path / "director-domain" / "agents" / "director"
    assert (director / "CLAUDE.md").is_file()
    assert (director / "scope.py").is_file()
    assert (director / "tools").is_dir()
    assert (director / "agents").is_dir(), "director must have recursive agents/ subdir"


def test_director_scope_is_empty_tuple(tmp_path: Path) -> None:
    """Director agent's SCOPE is () — meaning whole domain in scope."""
    rc = _scaffold(tmp_path, name="scope-domain")
    assert rc == 0

    scope_text = (
        tmp_path / "scope-domain" / "agents" / "director" / "scope.py"
    ).read_text(encoding="utf-8")
    assert "SCOPE = ()" in scope_text
