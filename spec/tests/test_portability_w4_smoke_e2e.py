"""Tests: portability W4 — end-to-end smoke-test via pytest tmp_path (§3.2, §3.3, §10).

This is the "consumer scenario" canary (§8-W4 acceptance). It verifies that
the framework, when pointed at a synthetic consumer project root via
``HOTAM_SPEC_PROJECT_ROOT``, correctly:

  - AC3 analog: writes the consumer's CLAUDE.md into the tmp_path, never
    touching the framework repo's own CLAUDE.md.
  - AC6 analog: no ``Path(__file__).resolve().parents[N]`` in the framework
    package computes a CONSUMER path — only intra-package paths.
  - AC9 analog: ``_ticket_store`` / ``_delegation_store`` write into the
    tmp_path consumer root, not the framework install dir.
  - AC10 analog: two calls with different env vars yield different roots
    (already covered by test_project_paths.py — confirmed green here as a
    co-located regression guard).

All writes happen inside ``tmp_path`` (auto-cleaned by pytest). Nothing
touches ``D:/ai_dev/prat/`` or any external directory.
"""

from __future__ import annotations

import importlib.util
import os
import re
import sys
from pathlib import Path

import pytest

SPEC_ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = SPEC_ROOT.parent
FRAMEWORK_CLAUDE_MD = REPO_ROOT / "CLAUDE.md"
_tools = str(SPEC_ROOT / "tools")
if _tools not in sys.path:
    sys.path.insert(0, _tools)

#: Modules that are ALLOWED to use Path(__file__).resolve().parents[N] — they
#: are the path-accessor modules themselves and compute INTRA-package paths
#: only (the framework's own spec/src/tests/tools, never consumer paths).
_LEGIT_PARENTS_MODULES = frozenset({
    "repo_paths.py",
    "project_paths.py",
    "runtime_paths.py",
    "template_loader.py",
})


# ===========================================================================
# Helper: scaffold a minimal valid consumer domain inside a given root.
# Reuses create_domain.py's scaffold function (not subprocess).
# ===========================================================================


def _scaffold_consumer_project(
    root: Path,
    domain_name: str = "synthetic-consumer",
) -> Path:
    """Create a minimal valid consumer layout inside ``root``.

    Layout: domains/.active-domain + domains/<name>/{manifest.py, graph.py}.
    Uses create_domain.scaffold() to produce a graph.py that gen_spec can load.
    Returns the domain directory path.
    """
    domains_dir = root / "domains"
    domains_dir.mkdir(parents=True, exist_ok=True)
    # Pin the synthetic domain as active.
    (domains_dir / ".active-domain").write_text(domain_name + "\n", encoding="utf-8")

    import create_domain as _cd  # noqa: PLC0415

    _cd._DOMAINS_ROOT = domains_dir  # type: ignore[attr-defined]
    try:
        rc = _cd.scaffold(
            name=domain_name,
            description="Synthetic consumer domain for portability smoke-test.",
            goals=["smoke-test passes in isolated tmp_path"],
            director_purpose="director of synthetic consumer domain",
            domains_root=domains_dir,
        )
    finally:
        _cd._DOMAINS_ROOT = None  # type: ignore[attr-defined]
    assert rc == 0, f"create_domain.scaffold failed with rc={rc}"
    return domains_dir / domain_name


def _load_gen_spec_isolated(module_name: str) -> object:
    """Load tools/gen_spec.py into a fresh module namespace.

    Each call gets its own module-level constants (REPO_ROOT, DOMAINS_ROOT,
    CLAUDE_MD) resolved against whatever env is active at exec time. This
    mirrors the pattern in test_gen_spec_idempotency.py.
    """
    gen_spec_path = SPEC_ROOT / "tools" / "gen_spec.py"
    spec = importlib.util.spec_from_file_location(module_name, gen_spec_path)
    assert spec is not None and spec.loader is not None
    mod = importlib.util.module_from_spec(spec)
    sys.modules[module_name] = mod
    spec.loader.exec_module(mod)  # type: ignore[union-attr]
    return mod


# ===========================================================================
# AC3 analog — consumer CLAUDE.md written to tmp_path, framework untouched
# ===========================================================================


def test_consumer_gen_spec_writes_to_project_root_not_framework(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """gen_spec, with HOTAM_SPEC_PROJECT_ROOT=tmp_path, writes CLAUDE.md there.

    AC3: the consumer's CLAUDE.md is produced inside the synthetic project
    root. The framework's own CLAUDE.md (D:/dev/HotamSpec/CLAUDE.md) is NOT
    modified — verified by snapshotting its bytes before and after.
    """
    consumer_root = tmp_path / "consumer"
    consumer_root.mkdir()
    _scaffold_consumer_project(consumer_root)

    # Snapshot framework CLAUDE.md before.
    fw_before = FRAMEWORK_CLAUDE_MD.read_bytes()

    monkeypatch.setenv("HOTAM_SPEC_PROJECT_ROOT", str(consumer_root))
    monkeypatch.chdir(consumer_root)

    gs = _load_gen_spec_isolated("gen_spec_smoke_consumer")
    try:
        gs.main([])  # type: ignore[attr-defined]
    finally:
        sys.modules.pop("gen_spec_smoke_consumer", None)

    # Consumer CLAUDE.md was created.
    consumer_claude = consumer_root / "CLAUDE.md"
    assert consumer_claude.exists(), (
        f"Consumer CLAUDE.md not written at {consumer_claude}"
    )

    # Framework CLAUDE.md is byte-identical (untouched).
    fw_after = FRAMEWORK_CLAUDE_MD.read_bytes()
    assert fw_before == fw_after, (
        "Framework CLAUDE.md was modified during consumer gen_spec run — "
        "the framework must NEVER touch its own CLAUDE.md when "
        "HOTAM_SPEC_PROJECT_ROOT points elsewhere."
    )


# ===========================================================================
# AC6 analog — no Path(__file__).resolve().parents[N] computing consumer paths
# ===========================================================================


def test_no_framework_module_uses_parents_for_consumer_paths() -> None:
    """No spec/src/hotam_spec/*.py uses parents[N] to compute consumer paths.

    AC6: ``Path(__file__).resolve().parents[N]`` is only legitimate for
    INTRA-package paths (the path-accessor modules themselves). Any other
    module using it is a portability violation — it would break under
    pip-install (where __file__ is inside site-packages, not the consumer repo).

    This is a static scan: it greps each .py file for the pattern and reports
    violations. The known path-accessor modules are whitelisted.
    """
    pkg_dir = SPEC_ROOT / "src" / "hotam_spec"
    pattern = re.compile(r"Path\(__file__\)\.resolve\(\)\.parents\[")

    violations: list[str] = []
    for py_file in sorted(pkg_dir.glob("*.py")):
        if py_file.name in _LEGIT_PARENTS_MODULES:
            continue
        text = py_file.read_text(encoding="utf-8")
        for i, line in enumerate(text.splitlines(), 1):
            if pattern.search(line):
                violations.append(f"{py_file.name}:{i}: {line.strip()}")

    assert not violations, (
        "AC6 violation: the following framework package modules use "
        "Path(__file__).resolve().parents[N] (only path-accessor modules "
        "repo_paths.py / project_paths.py / runtime_paths.py / template_loader.py "
        f"may do so, for intra-package paths): {violations}"
    )


# ===========================================================================
# AC9 analog — _ticket_store / _delegation_store write to consumer root
# ===========================================================================


def test_ticket_store_writes_to_consumer_root(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """_ticket_store creates tickets/ under the consumer project root.

    AC9: tickets are CONSUMER data. With HOTAM_SPEC_PROJECT_ROOT=tmp_path,
    the ticket store must write to tmp_path/tickets, NOT the framework's
    own directory.
    """
    consumer_root = tmp_path / "consumer-tickets"
    consumer_root.mkdir()

    monkeypatch.setenv("HOTAM_SPEC_PROJECT_ROOT", str(consumer_root))
    monkeypatch.chdir(consumer_root)

    import _ticket_store  # noqa: PLC0415

    _ticket_store.REPO_ROOT = None  # type: ignore[attr-defined]
    _ticket_store.TICKETS_DIR = None  # type: ignore[attr-defined]

    tickets_dir = _ticket_store._tickets_dir()
    assert str(tickets_dir).startswith(str(consumer_root.resolve())), (
        f"tickets dir {tickets_dir} is NOT under consumer root {consumer_root} — "
        "the ticket store is writing to the wrong location."
    )
    assert tickets_dir.name == "tickets"


def test_delegation_store_writes_to_consumer_root(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """_delegation_store resolves delegations/ under the consumer project root.

    AC9: delegations are CONSUMER data. With HOTAM_SPEC_PROJECT_ROOT=tmp_path,
    the delegation store must resolve tmp_path/delegations.
    """
    consumer_root = tmp_path / "consumer-delegations"
    consumer_root.mkdir()

    monkeypatch.setenv("HOTAM_SPEC_PROJECT_ROOT", str(consumer_root))
    monkeypatch.chdir(consumer_root)

    import _delegation_store  # noqa: PLC0415

    _delegation_store.REPO_ROOT = None  # type: ignore[attr-defined]
    _delegation_store.DELEGATIONS_DIR = None  # type: ignore[attr-defined]

    delegations_dir = _delegation_store._delegations_dir()
    assert str(delegations_dir).startswith(str(consumer_root.resolve())), (
        f"delegations dir {delegations_dir} is NOT under consumer root {consumer_root}"
    )
    assert delegations_dir.name == "delegations"


# ===========================================================================
# AC10 analog — env override switches root between calls (regression guard)
# ===========================================================================


def test_runtime_dir_env_override_switches_between_calls(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """runtime_dir() with HOTAM_SPEC_RUNTIME_DIR env yields different dirs.

    §3.2 variant 4-C: the runtime dir must be re-evaluated on every call.
    Two calls with different env values MUST return different paths — if a
    module-level cache locked the first result, this test would fail.
    """
    from hotam_spec.runtime_paths import runtime_dir

    rt_a = tmp_path / "runtime_a"
    rt_b = tmp_path / "runtime_b"
    rt_a.mkdir()
    rt_b.mkdir()

    monkeypatch.setenv("HOTAM_SPEC_RUNTIME_DIR", str(rt_a))
    result_a = runtime_dir()
    assert result_a == rt_a.resolve()

    monkeypatch.setenv("HOTAM_SPEC_RUNTIME_DIR", str(rt_b))
    result_b = runtime_dir()
    assert result_b == rt_b.resolve()

    assert result_a != result_b, (
        "runtime_dir() did not switch between calls — env override is being "
        "cached at module level (§3.2 violation)."
    )


def test_runtime_dir_env_takes_priority_over_default(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """HOTAM_SPEC_RUNTIME_DIR wins over the default project_root/.hotam-spec/runtime.

    §3.2 variant 4-C: env is the highest-priority resolution link.
    """
    from hotam_spec.runtime_paths import runtime_dir

    consumer_root = tmp_path / "consumer-rt"
    consumer_root.mkdir()
    env_rt = tmp_path / "env-runtime"
    env_rt.mkdir()

    monkeypatch.setenv("HOTAM_SPEC_PROJECT_ROOT", str(consumer_root))
    monkeypatch.setenv("HOTAM_SPEC_RUNTIME_DIR", str(env_rt))

    result = runtime_dir()
    assert result == env_rt.resolve(), (
        f"Expected env override {env_rt}, got {result}"
    )


def test_runtime_dir_default_is_under_project_root(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """Without env, runtime_dir() = project_root / .hotam-spec / runtime.

    §3.2 variant 4-C default: the consumer's runtime lives under their
    project root, not inside the framework package. This test uses a fresh
    tmp_path as the consumer root. The self-hosting legacy fallback to
    spec/.runtime/ only triggers when the legacy dir has data AND the new
    dir is empty — we mock the legacy check so it reports "no data",
    simulating a clean consumer environment.
    """
    from hotam_spec import runtime_paths

    consumer_root = tmp_path / "consumer-default"
    consumer_root.mkdir()

    monkeypatch.setenv("HOTAM_SPEC_PROJECT_ROOT", str(consumer_root))
    monkeypatch.delenv("HOTAM_SPEC_RUNTIME_DIR", raising=False)
    # Also ensure we're not in a CWD that has markers (so R3 doesn't fire).
    monkeypatch.chdir(tmp_path)

    # Simulate a clean consumer environment: no legacy spec/.runtime data.
    monkeypatch.setattr(runtime_paths, "_legacy_has_data", lambda: False)

    result = runtime_paths.runtime_dir()
    expected = consumer_root.resolve() / ".hotam-spec" / "runtime"
    assert result == expected, (
        f"Expected default {expected}, got {result}"
    )


def test_runtime_dir_legacy_fallback_preserves_self_hosting_data(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    """When the new default is empty but legacy spec/.runtime/ has data, use legacy.

    §3.2 migration: the self-hosting repo has accumulated calibration data
    in spec/.runtime/ (run-speed-baseline.json, land-log.jsonl, etc.).
    Moving the runtime to .hotam-spec/runtime would LOSE this data if done
    naively. The legacy fallback transparently keeps using spec/.runtime/
    until the operator explicitly migrates files or sets the env var.
    """
    from hotam_spec import runtime_paths

    consumer_root = tmp_path / "consumer-legacy"
    consumer_root.mkdir()

    monkeypatch.setenv("HOTAM_SPEC_PROJECT_ROOT", str(consumer_root))
    monkeypatch.delenv("HOTAM_SPEC_RUNTIME_DIR", raising=False)
    monkeypatch.chdir(tmp_path)

    # Simulate: legacy dir HAS data, new dir is empty.
    monkeypatch.setattr(runtime_paths, "_legacy_has_data", lambda: True)

    legacy_mock = tmp_path / "fake-legacy-runtime"
    legacy_mock.mkdir()
    (legacy_mock / "run-speed-baseline.json").write_text('{"baseline_s": 1.0}', encoding="utf-8")
    monkeypatch.setattr(runtime_paths, "_legacy_runtime_dir", lambda: legacy_mock)

    result = runtime_paths.runtime_dir()
    assert result == legacy_mock, (
        f"Legacy fallback should return {legacy_mock}, got {result}. "
        "The self-hosting data preservation fallback is not working."
    )
