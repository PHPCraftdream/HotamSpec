"""Tests: portability G3 — REAL subprocess e2e via an isolated venv + editable install.

test_portability_w4_smoke_e2e.py already proves the resolver chain (project_paths,
runtime_paths, ticket/delegation stores) picks the right consumer root — but it
does so IN-PROCESS: same interpreter, same sys.path, direct function calls
(``gs.main([])``, ``_cd.scaffold(...)``). That style cannot see a whole class of
portability bugs:

  - entry points ([project.scripts] in pyproject.toml) only exist after a REAL
    ``pip install`` writes the console-script shims into <venv>/Scripts/. An
    in-process test importing ``hotam_spec.cli.create_domain`` directly never
    exercises that resolution.
  - a stray ``sys.path`` entry pointing at the framework's OWN checkout (e.g.
    because the test process itself was launched from spec/) can mask a
    missing dependency or a relative-import bug that would break a clean
    interpreter.
  - PATH / shebang wiring on Windows (.exe launcher wrapping the venv's
    python.exe) is itself part of "does pip install work", and has zero
    coverage from in-process calls.

This module trades speed for that missing coverage: it creates a throw-away
venv, does a real ``pip install -e <repo>/spec`` into it, then drives the
installed ``hotam-create-domain`` / ``hotam-what-now`` console scripts via
``subprocess.run`` from a synthetic consumer directory's cwd — a completely
separate process, interpreter, and sys.path from the pytest runner.

Cost: creating the venv + editable install is expensive (dozens of seconds to
low minutes, dominated by venv creation + pip's dependency resolution/metadata
build on Windows). This is inherent to the "real interpreter, real install"
guarantee the test buys — an in-process shortcut would silently reintroduce
the blind spot this test exists to close. The single test in this module is
therefore marked ``@pytest.mark.slow`` (registered in tests/conftest.py,
alongside the existing ``framework``/``domain`` tier markers) AND additionally
gated by ``skipif`` on an explicit env var — see the rationale right below the
marker registry comment further down this file for why ``skipif`` (not just
the marker) is what actually keeps it out of the default T2 run. Run it
explicitly with:

    HOTAM_SPEC_RUN_E2E_SUBPROCESS=1 .venv/Scripts/python.exe -m pytest spec/tests/test_e2e_consumer_subprocess.py -v -s

All writes happen inside pytest's ``tmp_path`` (auto-cleaned). Nothing touches
``D:/ai_dev/prat/`` or any directory outside tmp_path / this repo checkout.
The one assertion against the framework's OWN repo is read-only: a byte
snapshot of ``D:/dev/HotamSpec/CLAUDE.md`` taken before and compared after,
proving the consumer-side run never mutated the framework's installation.
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
FRAMEWORK_CLAUDE_MD = REPO_ROOT / "CLAUDE.md"

# gen_spec.py (run for real by `hotam-create-domain --activate` below) writes
# TWO kinds of docs: consumer-owned (root CLAUDE.md, domains/<name>/docs/gen/)
# — correctly anchored to the CONSUMER's project root by the R1-R6 chain — and
# framework-SHARED docs (spec/docs/thinking/*.md, spec/docs/tools/*.md), which
# are anchored to Path(__file__)-derived SPEC_ROOT (i.e. the framework's own
# physical install location) regardless of which project is active. Under an
# editable install that location IS this real repo checkout. Their content
# includes a `reader: <id>` header resolved from whatever domain happens to be
# active in the CALLING process — here, the freshly-created, stakeholder-less
# "my-shop" consumer domain — which legitimately differs from this repo's own
# committed reader bindings (resolved from its own hotam-spec-self domain).
# This is a pre-existing gen_spec.py cross-contamination gap (reproducible in
# plain self-hosting mode too: `HOTAM_SPEC_ACTIVE_DOMAIN=hotam-dev
# python tools/gen_spec.py` dirties the same files) — orthogonal to what this
# test exists to verify, and out of scope to fix here. So this test snapshots
# and restores those two directories itself, the same way it protects
# CLAUDE.md, rather than leaving the framework checkout dirtied by a passing
# test run.
_SHARED_DOC_DIRS = (
    SPEC_ROOT / "docs" / "thinking",
    SPEC_ROOT / "docs" / "tools",
)

# Generous but bounded — venv creation + editable install on a cold pip cache
# can take a while on Windows; a hung/broken install should still fail the
# test rather than block a CI runner indefinitely.
_INSTALL_TIMEOUT_S = 600
_CLI_TIMEOUT_S = 120

# This module has no `[tool.pytest.ini_options] markers` block to opt out of
# (the project registers markers dynamically in tests/conftest.py instead —
# see the `framework`/`domain` tier there), and the mandatory T2 command
# (CLAUDE.md: `python -m pytest -q`, no flags) does not pass `-m` filters. A
# real venv + real `pip install -e` costs tens of seconds to a few minutes —
# far above what's acceptable inside the T1/T2 fast path. So this single test
# opts itself OUT of the default run via `skipif`, gated on an explicit env
# var, rather than relying on marker deselection the fixed T2 invocation
# would never apply. Run it explicitly with:
#   HOTAM_SPEC_RUN_E2E_SUBPROCESS=1 .venv/Scripts/python.exe -m pytest \
#       spec/tests/test_e2e_consumer_subprocess.py -v -s
_RUN_E2E = os.environ.get("HOTAM_SPEC_RUN_E2E_SUBPROCESS") == "1"
_SKIP_REASON = (
    "slow e2e test (real venv + real `pip install -e`, ~tens of seconds to "
    "a few minutes): run explicitly with HOTAM_SPEC_RUN_E2E_SUBPROCESS=1"
)


def _venv_python(venv_dir: Path) -> Path:
    """Path to the venv's python executable (Windows layout: Scripts/python.exe)."""
    if sys.platform == "win32":
        return venv_dir / "Scripts" / "python.exe"
    return venv_dir / "bin" / "python"


def _venv_script(venv_dir: Path, name: str) -> Path:
    """Path to a console-script entry point installed into the venv.

    Windows installs ``<name>.exe`` launcher shims into Scripts/; POSIX
    installs a shebang script into bin/.
    """
    if sys.platform == "win32":
        return venv_dir / "Scripts" / f"{name}.exe"
    return venv_dir / "bin" / name


@pytest.mark.slow
@pytest.mark.skipif(not _RUN_E2E, reason=_SKIP_REASON)
def test_editable_install_consumer_cli_subprocess_e2e(tmp_path: Path) -> None:
    """Real venv + real `pip install -e`, drive the installed CLI as subprocesses.

    Steps (all against tmp_path, never the framework's own checkout):
      1. python -m venv tmp_path/venv
      2. <venv>/Scripts/python.exe -m pip install -e <repo>/spec
      3. cwd = tmp_path/consumer_project (a fresh, empty directory) with a
         .hotam-spec-project marker (the documented R4 bootstrap mechanism —
         see docs/QUICKSTART-CONSUMER.md and the comment at the marker
         write-out below for why it's required on the FIRST invocation)
      4. subprocess: hotam-create-domain --activate my-shop ...
      5. assert consumer_project/domains/my-shop/, CLAUDE.md, .active-domain
      6. assert D:/dev/HotamSpec/CLAUDE.md is byte-identical before/after
      7. subprocess: hotam-what-now from consumer_project, exit 0
    """
    # -- snapshot the framework's OWN CLAUDE.md before touching anything ---
    fw_claude_before = FRAMEWORK_CLAUDE_MD.read_bytes()

    # -- snapshot the framework's shared docs (see _SHARED_DOC_DIRS above) --
    shared_doc_backups: dict[Path, Path] = {}
    for shared_dir in _SHARED_DOC_DIRS:
        backup_dir = tmp_path / f"backup-{shared_dir.name}"
        shutil.copytree(shared_dir, backup_dir)
        shared_doc_backups[shared_dir] = backup_dir

    # -- 1. create an isolated venv ------------------------------------
    venv_dir = tmp_path / "venv"
    venv.EnvBuilder(with_pip=True, clear=True).create(venv_dir)
    venv_python = _venv_python(venv_dir)
    assert venv_python.exists(), f"venv python not found at {venv_python}"

    # -- 2. editable install of THIS repo's spec/ package ---------------
    install = subprocess.run(
        [str(venv_python), "-m", "pip", "install", "-e", str(SPEC_ROOT)],
        cwd=str(tmp_path),
        capture_output=True,
        text=True,
        timeout=_INSTALL_TIMEOUT_S,
    )
    assert install.returncode == 0, (
        "pip install -e failed:\n"
        f"--- stdout ---\n{install.stdout}\n--- stderr ---\n{install.stderr}"
    )

    create_domain_exe = _venv_script(venv_dir, "hotam-create-domain")
    what_now_exe = _venv_script(venv_dir, "hotam-what-now")
    assert create_domain_exe.exists(), (
        f"hotam-create-domain entry point not installed at {create_domain_exe}; "
        f"pip stdout:\n{install.stdout}"
    )
    assert what_now_exe.exists(), (
        f"hotam-what-now entry point not installed at {what_now_exe}"
    )

    # -- 3. a fresh consumer project directory, distinct from the venv --
    consumer_project = tmp_path / "consumer_project"
    consumer_project.mkdir()

    # docs/QUICKSTART-CONSUMER.md step 2 documents the bootstrap chicken-and-
    # egg explicitly: a brand-new project directory carries NO R1-R5 marker
    # yet (that's exactly what `hotam-create-domain` is about to create), so
    # the FIRST invocation needs an explicit marker. The doc's own suggestion
    # is `git init` — but `.git/` is deliberately NOT an R3 marker (a marker
    # would false-positive on every git repo on the machine), so the actual
    # supported bootstrap mechanism is the R4 `.hotam-spec-project` empty
    # marker file. This is also the regression guard for a real bug this test
    # caught: before hotam_spec.project_paths.project_root()'s R6 self-hosting
    # fallback was gated on CWD being inside the framework's own repo, a
    # marker-less consumer directory would silently resolve to the framework
    # install location instead of raising — see the R6 gate in project_paths.py
    # and the updated R6 tests in test_project_paths.py.
    (consumer_project / ".hotam-spec-project").write_text("", encoding="utf-8")

    # Isolate env: no leftover HOTAM_SPEC_* pointing elsewhere, and strip any
    # PYTHONPATH the parent test runner may carry (so sys.path in the child
    # process is determined ONLY by the venv + the subprocess's own cwd, the
    # exact condition a real end-user's shell would be in).
    child_env = dict(os.environ)
    for key in list(child_env):
        if key.startswith("HOTAM_SPEC_"):
            del child_env[key]
    child_env.pop("PYTHONPATH", None)

    # -- 4. real subprocess call of the installed console script --------
    # Wrapped in try/finally so the framework's shared docs are restored
    # (see _SHARED_DOC_DIRS above) even if an assertion below fails.
    try:
        create_result = subprocess.run(
            [
                str(create_domain_exe),
                "my-shop",
                "--description",
                "Synthetic e2e consumer domain.",
                "--goals",
                "prove subprocess e2e install works",
                "--director-purpose",
                "director of the synthetic e2e consumer domain",
                "--activate",
            ],
            cwd=str(consumer_project),
            env=child_env,
            capture_output=True,
            text=True,
            timeout=_CLI_TIMEOUT_S,
        )
    finally:
        for shared_dir, backup_dir in shared_doc_backups.items():
            shutil.rmtree(shared_dir)
            shutil.copytree(backup_dir, shared_dir)

    assert create_result.returncode == 0, (
        "hotam-create-domain subprocess failed:\n"
        f"--- stdout ---\n{create_result.stdout}\n--- stderr ---\n{create_result.stderr}"
    )

    # -- 5. consumer-side artifacts exist in the RIGHT place -------------
    domain_dir = consumer_project / "domains" / "my-shop"
    assert domain_dir.is_dir(), (
        f"{domain_dir} not created; stdout:\n{create_result.stdout}"
    )
    assert (domain_dir / "manifest.py").exists()
    assert (domain_dir / "graph.py").exists()

    consumer_claude_md = consumer_project / "CLAUDE.md"
    assert consumer_claude_md.exists(), (
        f"{consumer_claude_md} not created by --activate; "
        f"stdout:\n{create_result.stdout}"
    )

    active_domain_marker = consumer_project / "domains" / ".active-domain"
    assert active_domain_marker.exists()
    assert active_domain_marker.read_text(encoding="utf-8").strip() == "my-shop"

    # -- 6. the FRAMEWORK's own CLAUDE.md must be untouched --------------
    fw_claude_after = FRAMEWORK_CLAUDE_MD.read_bytes()
    assert fw_claude_before == fw_claude_after, (
        "The framework repo's own CLAUDE.md changed during the consumer "
        "subprocess run — the editable-install consumer scenario must NEVER "
        "write into the framework's own checkout."
    )

    # -- 7. hotam-what-now works from the consumer cwd -------------------
    what_now_result = subprocess.run(
        [str(what_now_exe)],
        cwd=str(consumer_project),
        env=child_env,
        capture_output=True,
        text=True,
        timeout=_CLI_TIMEOUT_S,
    )
    assert what_now_result.returncode == 0, (
        "hotam-what-now subprocess failed:\n"
        f"--- stdout ---\n{what_now_result.stdout}\n--- stderr ---\n{what_now_result.stderr}"
    )
    assert what_now_result.stdout.strip(), (
        "hotam-what-now produced no output from the consumer project"
    )
