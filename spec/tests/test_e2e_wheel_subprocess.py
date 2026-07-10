"""Tests: Etap G — REAL subprocess e2e via an isolated venv + WHEEL install (non-editable).

This test closes the packaging gap identified by the external review (A.1):
``_path_setup.py`` self-documents that tools/ is NOT shipped in a non-editable
wheel, so every ``hotam-*`` CLI command fails with ``ModuleNotFoundError`` when
the package is ``pip install``'d from a ``.whl`` (not ``pip install -e``).

The existing ``test_e2e_consumer_subprocess.py`` covers the editable path
(``pip install -e <repo>/spec``); this test covers the **wheel** path:

  1. Build a real ``.whl`` from the repo via ``uv build --wheel``.
  2. Create a fresh venv, ``pip install <wheel.whl>`` (non-editable).
  3. Drive the **full QUICKSTART** (docs/QUICKSTART-CONSUMER.md) as real
     subprocesses from a synthetic consumer directory:
       - hotam-create-domain --activate
       - hotam-what-now  (empty graph)
       - 3x hotam-apply-proposal (stakeholders)
       - hotam-create-axis
       - 2x hotam-apply-proposal (requirements)
       - hotam-apply-proposal (conflict)
       - hotam-what-now  (conflict shows in diagnosis)
  4. Assert each step exits 0 and the expected artifacts exist.

Cost: building the wheel + creating the venv + pip install + 10 subprocesses.
On Windows this takes ~1-3 minutes. Gated by the same
``HOTAM_SPEC_RUN_E2E_SUBPROCESS=1`` env var as the editable e2e test.

Run explicitly with:
    HOTAM_SPEC_RUN_E2E_SUBPROCESS=1 .venv/Scripts/python.exe -m pytest spec/tests/test_e2e_wheel_subprocess.py -v -s

All writes happen inside pytest's ``tmp_path`` (auto-cleaned).
"""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
import venv
from pathlib import Path

import pytest

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent

_INSTALL_TIMEOUT_S = 600
_BUILD_TIMEOUT_S = 300
_CLI_TIMEOUT_S = 120

_RUN_E2E = os.environ.get("HOTAM_SPEC_RUN_E2E_SUBPROCESS") == "1"
_SKIP_REASON = (
    "slow e2e test (wheel build + real venv + real `pip install`, ~1-3 minutes): "
    "run explicitly with HOTAM_SPEC_RUN_E2E_SUBPROCESS=1"
)


def _venv_python(venv_dir: Path) -> Path:
    """Path to the venv's python executable."""
    if sys.platform == "win32":
        return venv_dir / "Scripts" / "python.exe"
    return venv_dir / "bin" / "python"


def _venv_script(venv_dir: Path, name: str) -> Path:
    """Path to a console-script entry point installed into the venv."""
    if sys.platform == "win32":
        return venv_dir / "Scripts" / f"{name}.exe"
    return venv_dir / "bin" / name


def _run_cli(
    venv_dir: Path,
    consumer_dir: Path,
    command: str,
    *args: str,
    env_extra: dict[str, str] | None = None,
) -> subprocess.CompletedProcess[str]:
    """Run a hotam-* CLI command in the consumer directory."""
    exe = _venv_script(venv_dir, command)
    child_env = dict(os.environ)
    # Strip any leftover HOTAM_SPEC_* and PYTHONPATH
    for key in list(child_env):
        if key.startswith("HOTAM_SPEC_"):
            del child_env[key]
    child_env.pop("PYTHONPATH", None)
    # Set explicit project root to avoid home-directory marker pollution
    child_env["HOTAM_SPEC_PROJECT_ROOT"] = str(consumer_dir)
    if env_extra:
        child_env.update(env_extra)
    return subprocess.run(
        [str(exe), *args],
        cwd=str(consumer_dir),
        env=child_env,
        capture_output=True,
        text=True,
        timeout=_CLI_TIMEOUT_S,
    )


@pytest.mark.slow
@pytest.mark.skipif(not _RUN_E2E, reason=_SKIP_REASON)
def test_wheel_install_full_quickstart_e2e(tmp_path: Path) -> None:
    """Wheel install + full QUICKSTART: create-domain, stakeholders, axis, requirements, conflict, what-now."""

    # -- 0-1. Build wheel via the atomic self-verifying builder ----------
    # build_wheel.py fuses populate + `uv build --wheel` + a member-count
    # self-check into one command that refuses to emit a wheel missing its
    # _tools/ scripts (R-wheel-build-atomic-verified). Using it here is what
    # keeps that requirement ENFORCED end-to-end.
    build_script = SPEC_ROOT / "scripts" / "build_wheel.py"
    wheel_dir = tmp_path / "dist"
    wheel_dir.mkdir()
    build_result = subprocess.run(
        [sys.executable, str(build_script), "--out-dir", str(wheel_dir)],
        cwd=str(SPEC_ROOT),
        capture_output=True,
        text=True,
        timeout=_BUILD_TIMEOUT_S,
    )
    assert build_result.returncode == 0, (
        f"build_wheel.py failed:\n{build_result.stdout}\n{build_result.stderr}"
    )
    wheels = list(wheel_dir.glob("*.whl"))
    assert len(wheels) == 1, f"Expected 1 wheel, got {len(wheels)}: {wheels}"
    wheel_path = wheels[0]

    # -- 2. Create venv + install wheel (non-editable) ------------------
    venv_dir = tmp_path / "venv"
    venv.EnvBuilder(with_pip=True, clear=True).create(venv_dir)
    venv_python = _venv_python(venv_dir)
    assert venv_python.exists()

    install = subprocess.run(
        [str(venv_python), "-m", "pip", "install", str(wheel_path)],
        capture_output=True,
        text=True,
        timeout=_INSTALL_TIMEOUT_S,
    )
    assert install.returncode == 0, (
        f"pip install wheel failed:\n{install.stdout}\n{install.stderr}"
    )

    # Verify entry points exist
    for cmd in ("hotam-create-domain", "hotam-what-now", "hotam-apply-proposal", "hotam-create-axis"):
        assert _venv_script(venv_dir, cmd).exists(), f"{cmd} not installed"

    # -- 3. Create consumer project directory ---------------------------
    consumer = tmp_path / "consumer_project"
    consumer.mkdir()
    (consumer / ".hotam-spec-project").write_text("", encoding="utf-8")

    # -- 4. hotam-create-domain --activate ------------------------------
    r = _run_cli(
        venv_dir, consumer,
        "hotam-create-domain", "my-shop",
        "--description", "My shop's contradictory requirements",
        "--goals", "hold the first tension;decide honestly",
        "--director-purpose", "steward my-shop",
        "--activate",
    )
    assert r.returncode == 0, (
        f"hotam-create-domain failed:\n--- stdout ---\n{r.stdout}\n--- stderr ---\n{r.stderr}"
    )
    assert (consumer / "domains" / "my-shop" / "graph.py").exists()
    assert (consumer / "domains" / ".active-domain").exists()
    assert (consumer / "CLAUDE.md").exists()

    # -- 5. hotam-what-now (empty graph) --------------------------------
    r = _run_cli(venv_dir, consumer, "hotam-what-now")
    assert r.returncode == 0, (
        f"hotam-what-now (empty) failed:\n{r.stdout}\n{r.stderr}"
    )

    # -- 6. Three stakeholders ------------------------------------------
    for i, (sid, sname, sdomain) in enumerate([
        ("alice", "Alice", "product"),
        ("bob", "Bob", "engineering"),
        ("carol", "Carol", "governance"),
    ], 1):
        proposal_file = consumer / f"sh{i}.json"
        proposal_file.write_text(
            f'{{"kind":"Stakeholder","id":"{sid}","name":"{sname}","domain":"{sdomain}"}}',
            encoding="utf-8",
        )
        r = _run_cli(venv_dir, consumer, "hotam-apply-proposal", str(proposal_file))
        assert r.returncode == 0, (
            f"stakeholder {sid} failed:\n--- stdout ---\n{r.stdout}\n--- stderr ---\n{r.stderr}"
        )

    # -- 7. Axis --------------------------------------------------------
    r = _run_cli(
        venv_dir, consumer,
        "hotam-create-axis", "speed-vs-rigor",
        "--description", "ship fast vs verify thoroughly",
    )
    assert r.returncode == 0, (
        f"create-axis failed:\n{r.stdout}\n{r.stderr}"
    )

    # -- 8. Two requirements --------------------------------------------
    for rid, claim, owner in [
        ("R-ship-fast", "Ship within one week.", "alice"),
        ("R-verify-all", "Verify every change before release.", "bob"),
    ]:
        proposal_file = consumer / f"{rid}.json"
        proposal_file.write_text(
            f'{{"kind":"Requirement","id":"{rid}","claim":"{claim}",'
            f'"owner":"{owner}","status":"SETTLED","enforcement":"PROSE"}}',
            encoding="utf-8",
        )
        r = _run_cli(venv_dir, consumer, "hotam-apply-proposal", str(proposal_file))
        assert r.returncode == 0, (
            f"requirement {rid} failed:\n--- stdout ---\n{r.stdout}\n--- stderr ---\n{r.stderr}"
        )

    # -- 9. Conflict ----------------------------------------------------
    conflict_file = consumer / "c1.json"
    conflict_file.write_text(
        '{"kind":"Conflict","axis":"speed-vs-rigor","context":"first release cadence",'
        '"members":["R-ship-fast","R-verify-all"],"steward":"carol"}',
        encoding="utf-8",
    )
    r = _run_cli(venv_dir, consumer, "hotam-apply-proposal", str(conflict_file))
    assert r.returncode == 0, (
        f"conflict failed:\n--- stdout ---\n{r.stdout}\n--- stderr ---\n{r.stderr}"
    )

    # -- 10. Final hotam-what-now (conflict should appear) ---------------
    r = _run_cli(venv_dir, consumer, "hotam-what-now")
    assert r.returncode == 0, (
        f"hotam-what-now (final) failed:\n{r.stdout}\n{r.stderr}"
    )
    # The conflict should be visible in the diagnosis
    assert "CONFLICT_STALLED" in r.stdout or "speed-vs-rigor" in r.stdout, (
        f"Final what-now should show the conflict, got:\n{r.stdout}"
    )

    # -- 11. Full CRUD scenario via shared helper -------------------------
    from _e2e_crud_helpers import run_crud_scenario

    def _run_cli_shortcut(command: str, *args: str) -> subprocess.CompletedProcess[str]:
        return _run_cli(venv_dir, consumer, command, *args)

    graph_py = consumer / "domains" / "my-shop" / "graph.py"
    run_crud_scenario(
        _run_cli_shortcut,
        consumer_dir=consumer,
        graph_py=graph_py,
    )
