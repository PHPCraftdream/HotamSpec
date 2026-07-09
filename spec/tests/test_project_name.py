"""Enforcer for R-project-name-hotam-spec (closes M1).

The project's name shall be Hotam-Spec (display), hotam_spec (Python package),
hotam-spec (kebab-case for filesystem and PyPI).
"""

from __future__ import annotations

from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent

_SRC = SPEC_ROOT / "src"


def test_pyproject_name_is_kebab_hotam_spec() -> None:
    """pyproject [project].name is the kebab-case 'hotam-spec'."""
    pyproject = (SPEC_ROOT / "pyproject.toml").read_text(encoding="utf-8")
    assert 'name = "hotam-spec"' in pyproject, (
        "pyproject.toml must declare the PyPI/kebab name 'hotam-spec'"
    )


def test_python_package_is_snake_hotam_spec() -> None:
    """The importable Python package is 'hotam_spec' under spec/src/."""
    assert (_SRC / "hotam_spec" / "__init__.py").is_file()
    import hotam_spec  # noqa: PLC0415

    assert hotam_spec.__name__ == "hotam_spec"


def test_display_name_is_hotam_spec_in_root_claude_md() -> None:
    """Root CLAUDE.md displays the project as 'Hotam-Spec'."""
    claude_md = (REPO_ROOT / "CLAUDE.md").read_text(encoding="utf-8")
    assert "Hotam-Spec" in claude_md
