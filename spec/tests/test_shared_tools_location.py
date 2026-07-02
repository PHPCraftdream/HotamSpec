"""Enforcer for R-shared-tools-in-spec-tools.

Tools available to all agents shall live in `spec/tools/`.

Structural projection: (1) the canonical shared toolset — every tool the
mediation loop names — exists under spec/tools/; (2) no rival shared-tools
location exists at the repository root (a root-level tools/ directory would
fork the shared home); (3) every *.py directly under spec/tools/ is a plain
module (the shared home holds tools, not packages of hidden state).
"""

from __future__ import annotations

from pathlib import Path

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent
TOOLS_DIR = SPEC_ROOT / "tools"

#: The tools the operating seed (CLAUDE.md mediation loop) depends on by name.
_CANONICAL_SHARED_TOOLS = {
    "what_now.py",
    "gen_spec.py",
    "apply_proposal.py",
    "closure.py",
    "emit_cipher.py",
    "audit_atomicity.py",
    "create_agent.py",
    "spawn_agent.py",
}


def test_spec_tools_dir_holds_the_canonical_shared_toolset() -> None:
    """Every mediation-loop tool lives in spec/tools/ (R-shared-tools-in-spec-tools)."""
    assert TOOLS_DIR.is_dir(), f"missing shared tools home: {TOOLS_DIR}"
    present = {p.name for p in TOOLS_DIR.glob("*.py")}
    missing = _CANONICAL_SHARED_TOOLS - present
    assert not missing, (
        f"shared tools missing from spec/tools/: {sorted(missing)} — shared "
        f"tools must live in spec/tools/, not elsewhere"
    )


def test_no_rival_shared_tools_dir_at_repo_root() -> None:
    """No repo-root tools/ directory rivals spec/tools/ as the shared home."""
    rival = REPO_ROOT / "tools"
    assert not rival.exists(), (
        f"{rival} exists — shared tools must live under spec/tools/ only "
        "(per-agent private tools live in <agent>/tools/)"
    )


def test_spec_tools_entries_are_plain_modules() -> None:
    """Direct children of spec/tools/ are files (plain tool modules), not packages."""
    offenders = [
        p.name
        for p in TOOLS_DIR.iterdir()
        if p.is_dir() and p.name not in {"__pycache__"}
    ]
    assert not offenders, (
        f"unexpected directories inside spec/tools/: {offenders} — the shared "
        "home holds flat tool modules"
    )
