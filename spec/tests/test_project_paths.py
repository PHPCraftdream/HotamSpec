"""Canon: §Graph — tests for hotam_spec.project_paths (R-project-root-not-hardcoded).

The R1–R6 resolution chain is the foundation of portability: it decides where
the consumer's data (domains/, tickets/, CLAUDE.md) lives. Six properties MUST
hold, each tested here in isolation using tmp_path + monkeypatch:

  R1 — env HOTAM_SPEC_PROJECT_ROOT wins over everything.
  R2 — env HOTAM_SPEC_DOMAINS_ROOT → project = its parent.
  R3 — filesystem markers in CWD (bottom-up).
  R4 — .hotam-spec-project marker file (bottom-up, up to 5 levels).
  R5 — pyproject.toml [tool.hotam-spec].project_root (relative path).
  R6 — self-hosting fallback (repo_paths.repo_root()).

Plus priority tests (R1 beats R2 beats R3...) and the uncertainty rule
(ProjectRootUnresolved carries a diagnostic).

IMPORTANT for tests on the REAL repo: the HotamSpec repo itself contains
domains/ and CLAUDE.md, so when CWD is the repo root, R3 fires (not R6).
Tests that need to isolate R6 must use tmp_path + monkeypatch.chdir to escape
the real-repo markers.
"""

from __future__ import annotations

import os
from pathlib import Path

import pytest

from hotam_spec import project_paths
from hotam_spec.project_paths import (
    ENV_DOMAINS_ROOT,
    ENV_PROJECT_ROOT,
    MARKER_FILENAME,
    ProjectRootUnresolved,
    project_root,
    project_root_or_raise,
)
from hotam_spec.repo_paths import repo_root


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch: pytest.MonkeyPatch) -> None:
    """Ensure both project-root env vars are unset for every test."""
    monkeypatch.delenv(ENV_PROJECT_ROOT, raising=False)
    monkeypatch.delenv(ENV_DOMAINS_ROOT, raising=False)


# ---------------------------------------------------------------------------
# R1 — env HOTAM_SPEC_PROJECT_ROOT
# ---------------------------------------------------------------------------

def test_r1_env_project_root_set(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R1: env var pointing at an existing directory → that directory."""
    target = tmp_path / "my-project"
    target.mkdir()
    monkeypatch.setenv(ENV_PROJECT_ROOT, str(target))
    monkeypatch.chdir(tmp_path)

    assert project_root() == target


def test_r1_env_project_root_nonexistent_returns_none_through_chain(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """R1 pointing at a non-existent path → R1 skipped, nothing else matches → None.

    Uses a clean tmp_path with NO markers and NO env for R2, so the chain falls
    to R6 which returns repo_root() — so this test verifies R1 validation only
    by checking that a BAD R1 value is NOT returned.
    """
    monkeypatch.setenv(ENV_PROJECT_ROOT, str(tmp_path / "does-not-exist"))
    monkeypatch.chdir(tmp_path)

    # R1 fails validation, chain falls through to R6 (repo_root).
    # The result is repo_root(), NOT the bad env value.
    result = project_root()
    assert result != tmp_path / "does-not-exist"


def test_r1_env_project_root_file_not_dir_skipped(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """R1 pointing at a file (not a directory) → skipped."""
    target = tmp_path / "not-a-dir.txt"
    target.write_text("hello", encoding="utf-8")
    monkeypatch.setenv(ENV_PROJECT_ROOT, str(target))
    monkeypatch.chdir(tmp_path)

    result = project_root()
    assert result != target


def test_r1_env_whitespace_stripped(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R1: surrounding whitespace is stripped before validation."""
    target = tmp_path / "ws-project"
    target.mkdir()
    monkeypatch.setenv(ENV_PROJECT_ROOT, f"  {target}  ")
    monkeypatch.chdir(tmp_path)

    assert project_root() == target


# ---------------------------------------------------------------------------
# R2 — env HOTAM_SPEC_DOMAINS_ROOT
# ---------------------------------------------------------------------------

def test_r2_env_domains_root_set(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R2: env var pointing at an existing domains/ → project = its parent."""
    project = tmp_path / "consumer"
    domains = project / "domains"
    domains.mkdir(parents=True)
    monkeypatch.setenv(ENV_DOMAINS_ROOT, str(domains))
    monkeypatch.chdir(tmp_path)

    assert project_root() == project


def test_r2_env_domains_root_nonexistent_skipped(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """R2 pointing at a non-existent domains/ → skipped."""
    monkeypatch.setenv(ENV_DOMAINS_ROOT, str(tmp_path / "no-such-domains"))
    monkeypatch.chdir(tmp_path)

    # Falls through to R6 (repo_root).
    result = project_root()
    assert result != tmp_path / "no-such-domains"


# ---------------------------------------------------------------------------
# R3 — filesystem markers in CWD
# ---------------------------------------------------------------------------

def test_r3_marker_domains_dir(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R3: CWD has domains/ → CWD is the project root."""
    (tmp_path / "domains").mkdir()
    monkeypatch.chdir(tmp_path)

    assert project_root() == tmp_path


def test_r3_marker_claude_md(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R3: CWD has CLAUDE.md → CWD is the project root."""
    (tmp_path / "CLAUDE.md").write_text("# Test", encoding="utf-8")
    monkeypatch.chdir(tmp_path)

    assert project_root() == tmp_path


def test_r3_marker_claude_dir(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R3: CWD has .claude/ → CWD is the project root."""
    (tmp_path / ".claude").mkdir()
    monkeypatch.chdir(tmp_path)

    assert project_root() == tmp_path


def test_r3_marker_tickets_dir(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R3: CWD has tickets/ → CWD is the project root."""
    (tmp_path / "tickets").mkdir()
    monkeypatch.chdir(tmp_path)

    assert project_root() == tmp_path


def test_r3_marker_delegations_dir(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R3: CWD has delegations/ → CWD is the project root."""
    (tmp_path / "delegations").mkdir()
    monkeypatch.chdir(tmp_path)

    assert project_root() == tmp_path


def test_r3_marker_pyproject_with_hotam_spec(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """R3: CWD has pyproject.toml with [tool.hotam-spec] → CWD is the project root."""
    (tmp_path / "pyproject.toml").write_text(
        '[tool.hotam-spec]\nsome_key = "value"\n', encoding="utf-8"
    )
    monkeypatch.chdir(tmp_path)

    assert project_root() == tmp_path


def test_r3_pyproject_without_hotam_spec_does_not_match(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """R3: pyproject.toml WITHOUT [tool.hotam-spec] → does NOT match (no false positive).

    The marker must be the hotam-spec table, not just any pyproject.toml.
    """
    (tmp_path / "pyproject.toml").write_text(
        '[project]\nname = "unrelated"\n', encoding="utf-8"
    )
    monkeypatch.chdir(tmp_path)

    # Falls through to R6, which is gated on CWD being inside the framework
    # repo (tmp_path is not) → None, not a silent framework-repo guess.
    assert project_root() is None


def test_r3_marker_in_parent_dir(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R3: marker is in a PARENT of CWD (bottom-up search)."""
    (tmp_path / "domains").mkdir()
    sub = tmp_path / "packages" / "myapp"
    sub.mkdir(parents=True)
    monkeypatch.chdir(sub)

    assert project_root() == tmp_path


# ---------------------------------------------------------------------------
# R4 — .hotam-spec-project marker file
# ---------------------------------------------------------------------------

def test_r4_marker_file_in_cwd(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R4: .hotam-spec-project file in CWD → CWD is the project root.

    This test ensures R4 fires when there are NO R3 markers (domains/, etc.)
    so it's the marker file that wins.
    """
    (tmp_path / MARKER_FILENAME).write_text("", encoding="utf-8")
    monkeypatch.chdir(tmp_path)

    assert project_root() == tmp_path


def test_r4_marker_file_in_parent(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R4: .hotam-spec-project file in a parent → parent is the project root."""
    (tmp_path / MARKER_FILENAME).write_text("", encoding="utf-8")
    sub = tmp_path / "sub" / "deep"
    sub.mkdir(parents=True)
    monkeypatch.chdir(sub)

    assert project_root() == tmp_path


# ---------------------------------------------------------------------------
# R5 — pyproject.toml [tool.hotam-spec].project_root
# ---------------------------------------------------------------------------

def test_r5_pyproject_project_root_resolves(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """R5 unit: _resolve_pyproject returns the project_root target directory.

    R5 is structurally dominated by R3 in the full chain (a pyproject.toml
    with [tool.hotam-spec] is itself an R3 marker, so R3 fires first). R5's
    independent value is tested here by calling _resolve_pyproject directly:
    given a pyproject.toml with [tool.hotam-spec].project_root pointing at an
    existing subdirectory, it returns that subdirectory resolved relative to
    the pyproject.toml's own location.
    """
    from hotam_spec.project_paths import _resolve_pyproject

    project = tmp_path / "target-dir"
    project.mkdir()
    pyproject = tmp_path / "pyproject.toml"
    pyproject.write_text(
        f'[tool.hotam-spec]\nproject_root = "target-dir"\n', encoding="utf-8"
    )
    monkeypatch.chdir(tmp_path)

    assert _resolve_pyproject(tmp_path, 5) == project


def test_r5_pyproject_project_root_nested(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """R5 unit: nested project_root resolves relative to pyproject location."""
    from hotam_spec.project_paths import _resolve_pyproject

    project = tmp_path / "nested" / "target"
    project.mkdir(parents=True)
    pyproject = tmp_path / "pyproject.toml"
    pyproject.write_text(
        f'[tool.hotam-spec]\nproject_root = "nested/target"\n', encoding="utf-8"
    )
    monkeypatch.chdir(tmp_path)

    assert _resolve_pyproject(tmp_path, 5) == project


# ---------------------------------------------------------------------------
# R6 — self-hosting fallback
# ---------------------------------------------------------------------------

def test_r6_fallback_when_nothing_else_matches(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """R6: when CWD is genuinely inside the framework repo → repo_paths.repo_root().

    R6 is gated on CWD being inside repo_root() — it must NEVER fire for an
    unrelated directory (e.g. a consumer's tmp_path), only for the true
    self-hosting case (working inside the HotamSpec checkout itself). This
    test simulates that by chdir-ing to a subdirectory it creates INSIDE the
    real repo_root(), with no other R1-R5 markers active there.
    """
    self_hosting_subdir = repo_root() / ".hotam-spec-r6-test-scratch"
    self_hosting_subdir.mkdir(exist_ok=True)
    try:
        monkeypatch.chdir(self_hosting_subdir)
        assert project_root() == repo_root()
    finally:
        # chdir away first — Windows refuses to rmdir a process's own CWD.
        monkeypatch.undo()
        self_hosting_subdir.rmdir()


def test_r6_does_not_fire_outside_the_framework_repo(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """R6 must NOT fire when CWD has no relation to the framework's own repo.

    This is the regression guard for the bug tests/test_e2e_consumer_
    subprocess.py caught: a brand-new, marker-less consumer directory
    (tmp_path here stands in for it) used to silently resolve to the
    framework's own repo_root() — R1-R5 all fail here (empty tmp_path,
    no env), and R6 must now return None instead of guessing.
    """
    monkeypatch.chdir(tmp_path)

    assert project_root() is None


# ---------------------------------------------------------------------------
# Priority tests
# ---------------------------------------------------------------------------

def test_priority_r1_beats_r2(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R1 wins over R2 when both are set."""
    r1_target = tmp_path / "r1-project"
    r1_target.mkdir()
    r2_domains = tmp_path / "r2-consumer" / "domains"
    r2_domains.mkdir(parents=True)

    monkeypatch.setenv(ENV_PROJECT_ROOT, str(r1_target))
    monkeypatch.setenv(ENV_DOMAINS_ROOT, str(r2_domains))
    monkeypatch.chdir(tmp_path)

    assert project_root() == r1_target


def test_priority_r1_beats_r3(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R1 wins over R3 (markers in CWD)."""
    r1_target = tmp_path / "explicit"
    r1_target.mkdir()
    (tmp_path / "domains").mkdir()  # R3 marker
    monkeypatch.setenv(ENV_PROJECT_ROOT, str(r1_target))
    monkeypatch.chdir(tmp_path)

    assert project_root() == r1_target


def test_priority_r2_beats_r3(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R2 wins over R3 (markers in CWD)."""
    r2_project = tmp_path / "r2-consumer"
    r2_domains = r2_project / "domains"
    r2_domains.mkdir(parents=True)
    (tmp_path / "domains").mkdir()  # R3 marker in CWD (tmp_path itself)

    monkeypatch.setenv(ENV_DOMAINS_ROOT, str(r2_domains))
    monkeypatch.chdir(tmp_path)

    assert project_root() == r2_project


def test_priority_r3_beats_r4(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R3 (domain/ marker in CWD) wins over R4 (marker file in CWD).

    Both markers are in the same directory; R3 fires first because it's
    earlier in the chain.
    """
    (tmp_path / "domains").mkdir()  # R3 marker
    (tmp_path / MARKER_FILENAME).write_text("", encoding="utf-8")  # R4 marker
    monkeypatch.chdir(tmp_path)

    # Both would return tmp_path, but we verify it's resolved (priority is
    # about which mechanism fires, and both agree on the same root here).
    assert project_root() == tmp_path


def test_priority_r3_beats_r5(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """R3 (markers in CWD) wins over R5 (pyproject project_root).

    CWD has both domains/ (R3 marker) AND a pyproject.toml with project_root
    pointing elsewhere → R3 wins (CWD is returned, not the pyproject target).
    """
    r5_target = tmp_path / "r5-target"
    r5_target.mkdir()
    (tmp_path / "domains").mkdir()  # R3 marker
    (tmp_path / "pyproject.toml").write_text(
        f'[tool.hotam-spec]\nproject_root = "r5-target"\n', encoding="utf-8"
    )
    monkeypatch.chdir(tmp_path)

    assert project_root() == tmp_path  # R3, not r5_target


# ---------------------------------------------------------------------------
# Uncertainty rule — ProjectRootUnresolved
# ---------------------------------------------------------------------------

def test_unresolved_diagnostic_content(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """ProjectRootUnresolved carries a diagnostic mentioning checked sources.

    We can't fully trigger None (R6 always returns repo_root), but we CAN
    test the diagnostic builder directly by mocking project_root to return
    None, and verify the diagnostic lists R1-R6 sources.
    """
    monkeypatch.chdir(tmp_path)
    monkeypatch.delenv(ENV_PROJECT_ROOT, raising=False)
    monkeypatch.delenv(ENV_DOMAINS_ROOT, raising=False)

    # Mock project_root to return None so project_root_or_raise raises.
    monkeypatch.setattr(project_paths, "project_root", lambda: None)

    with pytest.raises(ProjectRootUnresolved) as exc_info:
        project_root_or_raise()

    diag = exc_info.value.diagnostic
    assert ENV_PROJECT_ROOT in diag
    assert ENV_DOMAINS_ROOT in diag
    assert MARKER_FILENAME in diag
    assert "tool.hotam-spec" in diag or PYPROJECT_TABLE in diag


def test_project_root_or_raise_returns_path_when_resolved(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """project_root_or_raise returns the path (not raises) when resolution succeeds."""
    (tmp_path / "domains").mkdir()
    monkeypatch.chdir(tmp_path)

    assert project_root_or_raise() == tmp_path


def test_project_root_unresolved_is_runtime_error() -> None:
    """ProjectRootUnresolved subclasses RuntimeError (catchable by broad handlers)."""
    assert issubclass(ProjectRootUnresolved, RuntimeError)


# ---------------------------------------------------------------------------
# §8-W2 acceptance #2: env-var override REALLY switches root between calls.
# This is the direct verification that module-level resolver-result caches
# (the §3.3 problem — _DOMAINS_ROOT = _domains_root() computed ONCE at import)
# are gone: if any migrated module cached the root at import, two sequential
# calls with different env would return the SAME (stale) root and this test
# would fail. It MUST see two DIFFERENT roots.
# ---------------------------------------------------------------------------


def test_env_override_switches_root_between_calls(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """Two consecutive project_root() calls with different env → different roots.

    §3.3 / §8-W2 acceptance #2: the resolution MUST be re-evaluated on every
    call, not locked at import time. We set env A, call, then set env B, call
    again — the two results MUST differ and each MUST equal its env target.
    """
    dir_a = tmp_path / "project_a"
    dir_b = tmp_path / "project_b"
    dir_a.mkdir()
    dir_b.mkdir()

    monkeypatch.chdir(tmp_path)  # neutral CWD (no markers here)

    monkeypatch.setenv(ENV_PROJECT_ROOT, str(dir_a))
    root_a = project_root()
    assert root_a == dir_a.resolve()

    monkeypatch.setenv(ENV_PROJECT_ROOT, str(dir_b))
    root_b = project_root()
    assert root_b == dir_b.resolve()

    assert root_a != root_b, (
        "env override did not switch root between calls — a module-level "
        "resolver-result cache (§3.3) is still locking the first result"
    )


def test_domains_root_accessor_reflects_env_change_between_calls(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """domains_root() (the migrated accessor) re-resolves when env changes.

    §3.1/§3.3: repo_paths.domains_root() used to derive from _REPO_ROOT (a
    module-level constant computed once). After W2 it derives from
    project_root_or_raise(), so two calls with different HOTAM_SPEC_PROJECT_ROOT
    MUST return different domains roots. This catches a regression where
    domains_root is re-introduced as an import-time cache.
    """
    from hotam_spec.repo_paths import domains_root

    proj_a = tmp_path / "consumer_a"
    proj_b = tmp_path / "consumer_b"
    proj_a.mkdir()
    proj_b.mkdir()

    monkeypatch.chdir(tmp_path)  # neutral CWD

    monkeypatch.setenv(ENV_PROJECT_ROOT, str(proj_a))
    dr_a = domains_root()

    monkeypatch.setenv(ENV_PROJECT_ROOT, str(proj_b))
    dr_b = domains_root()

    assert dr_a == proj_a.resolve() / "domains"
    assert dr_b == proj_b.resolve() / "domains"
    assert dr_a != dr_b, (
        "domains_root() returned the same path after env change — it is "
        "cached at import (§3.3 regression)"
    )
