"""Tests: portability W3 — package data via importlib.resources (§3.4, §3.6).

Guarantees that CLAUDE.md.template.txt and enforcer_map.json are reachable
through importlib.resources (PEP 391) — the stdlib accessor that works
identically for editable, wheel, and vendor-copy installs. A plain Path()
read would work in editable mode but fail in a wheel; importlib.resources
is the install-method-agnostic contract.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

from importlib.resources import files

SPEC_ROOT = Path(__file__).resolve().parents[1]
_tools = str(SPEC_ROOT / "tools")
if _tools not in sys.path:
    sys.path.insert(0, _tools)


# ===========================================================================
# §3.4 — template in package data, reachable via importlib.resources
# ===========================================================================


def test_template_reachable_via_importlib_resources() -> None:
    """The packaged template is readable through importlib.resources.files()."""
    template_path = Path(str(files("hotam_spec") / "_templates" / "claude_md.template.txt"))
    assert template_path.is_file(), (
        f"Packaged template not found via importlib.resources at {template_path}. "
        "The file must ship inside the wheel (hotam_spec/_templates/claude_md.template.txt)."
    )
    text = template_path.read_text(encoding="utf-8")
    lines = [ln.strip() for ln in text.split("\n")]
    assert "<!-- mind -->" in lines, "Packaged template missing '<!-- mind -->' placeholder"
    assert "<!-- business -->" in lines, "Packaged template missing '<!-- business -->' placeholder"


def test_template_loader_reads_packaged_template() -> None:
    """template_loader.read_claude_md_template() returns the packaged template text."""
    from hotam_spec.template_loader import _packaged_template_path, read_claude_md_template

    packaged = _packaged_template_path()
    assert packaged.is_file(), f"Packaged template path does not exist: {packaged}"
    text = read_claude_md_template()
    assert "<!-- mind -->" in text
    assert "<!-- business -->" in text


def test_template_override_takes_priority(tmp_path, monkeypatch) -> None:
    """A consumer override at project_root()/CLAUDE.md.template.txt wins."""
    from hotam_spec import template_loader

    # Create a fake override template.
    override_content = "# override template\n<!-- mind -->\n<!-- business -->\noverride-marker\n"
    override_file = tmp_path / "CLAUDE.md.template.txt"
    override_file.write_text(override_content, encoding="utf-8")

    # Force project_root() to resolve to tmp_path (R1 env var).
    monkeypatch.setenv("HOTAM_SPEC_PROJECT_ROOT", str(tmp_path))
    # Clear any cached resolution.
    import hotam_spec.project_paths  # noqa: F401

    effective = template_loader.claude_md_template_path()
    assert effective == override_file, (
        f"Override did not take priority: expected {override_file}, got {effective}"
    )
    text = template_loader.read_claude_md_template()
    assert "override-marker" in text, "Override template content not read correctly"


def test_template_falls_back_to_packaged_without_override(monkeypatch, tmp_path) -> None:
    """With no override present, the packaged template is used."""
    from hotam_spec import template_loader

    # Point project_root at an empty tmp_path (no CLAUDE.md.template.txt there).
    monkeypatch.setenv("HOTAM_SPEC_PROJECT_ROOT", str(tmp_path))
    effective = template_loader.claude_md_template_path()
    packaged = template_loader._packaged_template_path()
    assert effective == packaged, (
        f"Expected packaged fallback {packaged}, got {effective}"
    )


# ===========================================================================
# §3.6 — enforcer_map.json in package data, reachable via importlib.resources
# ===========================================================================


def test_enforcer_map_reachable_via_importlib_resources() -> None:
    """The packaged enforcer_map.json is readable through importlib.resources."""
    map_path = Path(str(files("hotam_spec") / "_data" / "enforcer_map.json"))
    assert map_path.is_file(), (
        f"Packaged enforcer_map.json not found via importlib.resources at {map_path}. "
        "The file must ship inside the wheel (hotam_spec/_data/enforcer_map.json)."
    )
    data = json.loads(map_path.read_text(encoding="utf-8"))
    assert "func_index" in data, "enforcer_map.json missing 'func_index' key"
    assert "check_map" in data, "enforcer_map.json missing 'check_map' key"
    assert isinstance(data["func_index"], dict)
    assert isinstance(data["check_map"], dict)
    assert len(data["func_index"]) > 0, "enforcer_map.json func_index is empty"


def test_load_packaged_scan_returns_snapshot() -> None:
    """_load_packaged_scan() reads the package-data snapshot correctly."""
    from hotam_spec.enforcer_resolution import _load_packaged_scan

    scan = _load_packaged_scan()
    assert scan is not None, "Packaged scan returned None — enforcer_map.json missing or corrupt"
    assert len(scan.func_index) > 0, "Packaged scan func_index is empty"
    assert len(scan.check_map) > 0, "Packaged scan check_map is empty"


def test_packaged_scan_matches_live_scan() -> None:
    """The packaged snapshot agrees with a fresh live-scan of spec/tests/.

    This is the consistency guarantee: the build-time snapshot must reflect
    the same mapping the live resolver would produce. If this fails, the
    snapshot is stale and must be regenerated via tools/gen_enforcer_map.py.
    """
    from hotam_spec.enforcer_resolution import _build_scan_uncached, _load_packaged_scan
    from hotam_spec.repo_paths import tests_root

    tests_dir = tests_root()
    if not tests_dir.exists():
        return  # not in self-hosting mode — skip the consistency check

    live = _build_scan_uncached(tests_dir)
    packaged = _load_packaged_scan()
    assert packaged is not None

    # The live scan may have MORE entries if tests were added since the
    # snapshot was built — that's fine (the snapshot is a fallback). But
    # every entry IN the snapshot must agree with the live scan.
    for name, file_rel in packaged.func_index.items():
        live_val = live.func_index.get(name)
        # None means ambiguous in one but not the other — flag it.
        assert live_val == file_rel, (
            f"Snapshot mismatch for test func '{name}': "
            f"snapshot={file_rel}, live={live_val}. "
            "Regenerate: .venv/Scripts/python.exe tools/gen_enforcer_map.py"
        )
    for check, files in packaged.check_map.items():
        live_files = live.check_map.get(check, [])
        assert set(files) == set(live_files), (
            f"Snapshot mismatch for check '{check}': "
            f"snapshot={sorted(files)}, live={sorted(live_files)}. "
            "Regenerate: .venv/Scripts/python.exe tools/gen_enforcer_map.py"
        )


# ===========================================================================
# §4.1 — CLI wrappers importable (entry points will work after pip install)
# ===========================================================================


def test_cli_wrappers_importable() -> None:
    """All CLI wrapper modules import successfully (entry-point readiness)."""
    import importlib

    # Core mediation-loop wrappers (mandatory per W3).
    core = ["gen_spec", "what_now", "apply_proposal", "create_domain"]
    for name in core:
        mod = importlib.import_module(f"hotam_spec.cli.{name}")
        assert hasattr(mod, "main"), f"hotam_spec.cli.{name} missing main()"

    # Subcommand CLIs.
    for name in ["ticket", "delegation"]:
        mod = importlib.import_module(f"hotam_spec.cli.{name}")
        assert hasattr(mod, "main"), f"hotam_spec.cli.{name} missing main()"
