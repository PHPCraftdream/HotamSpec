"""Canon: §Proposal — suite-wide guard against runtime-log contamination.

Several tests call apply_proposal.apply(...) directly (test_apply_proposal_gate_wiring.py,
test_pending_proposal_archive.py, ...) to exercise the gate/verify/archive
wiring against a throw-away sample graph. Those calls reach _append_land_log(),
which — absent an explicit runtime_dir — appends to the LIVE
spec/.runtime/land-log.jsonl. The result: ~800 fixture rows (R-sample-target,
R-brand-new-node) drowning the ~80 real landings, so gate_status.py and any
human reading the trace see mostly noise (audit 2026-07-03).

This autouse fixture redirects apply_proposal._RUNTIME_DIR (and the derived
proposals dirs) into each test's tmp_path for the WHOLE suite, so no test can
write to the real runtime log by omission. A test that specifically wants to
assert real-log behavior can still pass an explicit runtime_dir to the API.
"""

from __future__ import annotations

import os
import sys
from pathlib import Path

import pytest

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_REPO_ROOT = _SPEC_ROOT.parent
_TOOLS = str(_SPEC_ROOT / "tools")
if _TOOLS not in sys.path:
    sys.path.insert(0, _TOOLS)
if str(_SPEC_ROOT / "src") not in sys.path:
    sys.path.insert(0, str(_SPEC_ROOT / "src"))

from hotam_spec.runtime_paths import runtime_dir as _runtime_dir  # noqa: E402


# --- Wave 17: framework-vs-domain test tiering (steward doctrine, verdict #8) ---
#
# Doctrine (steward, verbatim): "Бизнес всегда должен думать, что фреймворк
# работает. Быть рабочим — ответственность фреймворка. Его тесты прогоняются до
# всего отдельно. Если он меняется в ходе работы — обязан сделать все
# исправления для зелёных тестов."
#
# The framework must PROVE it works INDEPENDENTLY of which business domain is
# active. Tests split into two responsibility tiers:
#   * framework  — exercise hotam_spec.* mechanics; MUST stay green under ANY
#                  active domain (or none). Run first, separately:
#                  `-m framework` (equivalently `-m "not domain"`).
#   * domain     — assert the concrete content of the self-domain
#                  (hotam-spec-self): its atom count, specific R-/C-/A- anchors,
#                  its generated-doc bytes. Only green under the self-domain pin.
#
# The registry below is the single, auditable list of domain-coupled tests: a
# frozenset of (test-filename, test-function-name). Anything NOT listed is
# framework by default. Applied by pytest_collection_modifyitems. This keeps the
# cut centralized (one file, no per-test decorators scattered across the suite)
# and deterministic. Source of the list: the tests that redden under a foreign
# pin `HOTAM_SPEC_ACTIVE_DOMAIN=hotam-dev` (Wave 17 audit, C3).
DOMAIN_COUPLED: frozenset[tuple[str, str]] = frozenset(
    {
        ("test_bijection.py", "test_content_graph_bijection_clean"),
        ("test_constitution_gen.py", "test_constitution_includes_hard_boundary"),
        ("test_constitution_gen.py", "test_constitution_includes_super_rules"),
        (
            "test_constitution_gen.py",
            "test_constitution_lists_all_constitution_requirements",
        ),
        (
            "test_delegation_marker_honesty.py",
            "test_active_domain_delegation_markers_resolve",
        ),
        (
            "test_delegation_marker_honesty.py",
            "test_del_1_specifically_resolves_the_core_vs_aspect_conflict",
        ),
        ("test_docs_gen.py", "test_generated_docs_carry_reader_header"),
        ("test_entities_md.py", "test_entities_md_emitted_for_active_domain"),
        ("test_goal.py", "test_goal_burn_down_present"),
        ("test_history_gen.py", "test_history_contains_every_rejected_requirement"),
        ("test_history_gen.py", "test_history_contains_rationale_for_decided"),
        ("test_no_stale_m_table.py", "test_root_claude_md_links_to_decisions_md"),
        ("test_operator.py", "test_director_operator_present"),
        ("test_operator.py", "test_director_within_budget"),
        ("test_process.py", "test_pr_closed_loop_present"),
        ("test_recently_rejected.py", "test_recently_rejected_lists_known_rejections"),
        ("test_rejected_preserved.py", "test_graph_keeps_a_nonempty_rejected_history"),
        (
            "test_rejected_preserved.py",
            "test_every_history_rejected_id_still_in_graph_as_rejected",
        ),
    }
)


def pytest_configure(config: pytest.Config) -> None:
    """Register the two responsibility-tier markers (R-framework-suite-domain-independent)."""
    config.addinivalue_line(
        "markers",
        "framework: framework-tier test — green under ANY active domain (or none).",
    )
    config.addinivalue_line(
        "markers",
        "domain: domain-tier test — asserts concrete self-domain content.",
    )


def pytest_collection_modifyitems(
    config: pytest.Config, items: list[pytest.Item]
) -> None:
    """Tag every collected test `domain` (if in DOMAIN_COUPLED) else `framework`."""
    for item in items:
        fname = Path(str(item.fspath)).name
        func = getattr(item, "originalname", None) or item.name
        if (fname, func) in DOMAIN_COUPLED:
            item.add_marker(pytest.mark.domain)
        else:
            item.add_marker(pytest.mark.framework)


@pytest.fixture(autouse=True)
def _isolate_runtime_dir(tmp_path, monkeypatch):
    """Redirect apply_proposal's runtime dir into tmp_path for every test.

    Best-effort: if apply_proposal is not importable in some minimal env, the
    fixture is a no-op rather than a collection error.
    """
    try:
        import apply_proposal  # noqa: PLC0415
    except Exception:
        yield
        return

    runtime = tmp_path / ".runtime"
    proposals = runtime / "proposals"
    applied = proposals / "applied"
    monkeypatch.setattr(apply_proposal, "_RUNTIME_DIR", runtime, raising=False)
    monkeypatch.setattr(apply_proposal, "_PROPOSALS_DIR", proposals, raising=False)
    monkeypatch.setattr(
        apply_proposal, "_PROPOSALS_APPLIED_DIR", applied, raising=False
    )
    yield


# ---------------------------------------------------------------------------
# Test-suite speed-up (task #46, measures 1/3/4) — shared, read-only snapshots
# of the ONE gen_spec run and the ONE active graph, so the ~9 tests that used to
# each spawn `python tools/gen_spec.py` (~11s cold-start each) instead read a
# single in-process regeneration. DETERMINISM + ISOLATION are preserved:
#   * The regen runs UNDER THE SELF-HOST PIN (HOTAM_SPEC_ACTIVE_DOMAIN=
#     hotam-spec-self) so the resident root crystal is never bound to a foreign
#     env domain (R-root-crystal-follows-pin).
#   * gen_spec is byte-idempotent (that property is what the retained canary in
#     test_gen_spec_idempotency.py verifies), so regenerating an already-clean
#     working tree rewrites identical bytes — the tree stays clean. The
#     snapshot captures the fresh on-disk text AFTER that regen.
# ---------------------------------------------------------------------------


def _fresh_gen_spec_module():
    """Import tools/gen_spec.py in an ISOLATED module namespace (importlib), so
    the session regen never pollutes any other test's `import gen_spec`."""
    import importlib.util  # noqa: PLC0415

    gen_spec_path = _SPEC_ROOT / "tools" / "gen_spec.py"
    spec = importlib.util.spec_from_file_location("gen_spec_session", gen_spec_path)
    assert spec is not None and spec.loader is not None
    mod = importlib.util.module_from_spec(spec)
    sys.modules["gen_spec_session"] = mod
    try:
        spec.loader.exec_module(mod)  # type: ignore[union-attr]
    finally:
        sys.modules.pop("gen_spec_session", None)
    return mod


@pytest.fixture(scope="session")
def gen_spec_snapshot() -> dict[str, object]:
    """Run gen_spec ONCE in-process under the self-host pin and snapshot the
    generated bytes/text (Measure 1). Returned dict:

      * ``claude_md_text``  — root CLAUDE.md text, CRLF-normalized to \n.
      * ``claude_md_bytes`` — root CLAUDE.md raw bytes.
      * ``docs_gen``        — {filename: text} for domains/<self>/docs/gen/*.md.
      * ``gen_dir``         — the Path of that docs/gen directory.

    Consumer map/doc tests assert against THIS snapshot instead of spawning a
    subprocess to regenerate then re-read.
    """
    mod = _fresh_gen_spec_module()

    prev_env = os.environ.get("HOTAM_SPEC_ACTIVE_DOMAIN")
    os.environ["HOTAM_SPEC_ACTIVE_DOMAIN"] = "hotam-spec-self"
    prev_cwd = os.getcwd()
    try:
        os.chdir(_SPEC_ROOT)
        mod.main([])
    finally:
        os.chdir(prev_cwd)
        if prev_env is None:
            os.environ.pop("HOTAM_SPEC_ACTIVE_DOMAIN", None)
        else:
            os.environ["HOTAM_SPEC_ACTIVE_DOMAIN"] = prev_env

    claude_md = _REPO_ROOT / "CLAUDE.md"
    claude_md_bytes = claude_md.read_bytes()
    claude_md_text = claude_md_bytes.decode("utf-8").replace("\r\n", "\n")

    gen_dir = mod.GEN_DIR
    docs_gen: dict[str, str] = {}
    if gen_dir.exists():
        for p in sorted(gen_dir.glob("*.md")):
            docs_gen[p.name] = p.read_text(encoding="utf-8").replace("\r\n", "\n")

    return {
        "claude_md_text": claude_md_text,
        "claude_md_bytes": claude_md_bytes,
        "docs_gen": docs_gen,
        "gen_dir": gen_dir,
    }


@pytest.fixture(scope="session")
def active_graph():
    """Build the active-domain content graph ONCE per session (Measure 3).

    The graph is a frozen dataclass tree (R-graph frozen + tuples), so sharing
    it read-only across tests is safe. Tests that MUTATE or rebuild a per-
    scenario graph keep constructing their own — they do not take this fixture.
    """
    from hotam_spec.graph import load_content_graph  # noqa: PLC0415

    return load_content_graph()


# ---------------------------------------------------------------------------
# Run-speed guard: record wall-clock duration of every pytest session
# (R-run-speed-guarded).  Data lands in .runtime/run-durations.jsonl (per-
# machine, gitignored).  On the 5th record, a baseline (mean×1.2) is
# written to .runtime/run-speed-baseline.json.  The separate test
# test_run_speed_guard.py reads the PREVIOUS run's data and fails if the
# duration exceeded the baseline — lag-1 design keeps the guard within
# normal pytest semantics.
# ---------------------------------------------------------------------------

_SESSION_START: float | None = None

# Minimum number of tests collected to consider a run "full suite".
# Partial runs (single file, -k filter) must NOT pollute the speed journal
# because their short durations would corrupt the baseline and cause false
# failures on subsequent full runs (R-run-speed-guarded).
_FULL_SUITE_THRESHOLD = 100


def pytest_sessionstart(session: pytest.Session) -> None:
    """Record wall-clock start of the pytest session."""
    import time  # noqa: PLC0415

    global _SESSION_START  # noqa: PLW0603
    _SESSION_START = time.monotonic()


def pytest_sessionfinish(session: pytest.Session, exitstatus: int) -> None:
    """Record session duration and maintain the speed baseline.

    Only records when the session collected >= _FULL_SUITE_THRESHOLD tests,
    so partial runs (agents running ``pytest tests/test_x.py``, ``-k foo``,
    etc.) never corrupt the calibration journal.
    """
    import json as _json  # noqa: PLC0415
    import time  # noqa: PLC0415

    if _SESSION_START is None:
        return

    # Guard: skip recording for partial runs.
    collected = getattr(session, "testscollected", 0)
    if collected < _FULL_SUITE_THRESHOLD:
        return

    duration = time.monotonic() - _SESSION_START

    runtime_dir = _runtime_dir()
    runtime_dir.mkdir(parents=True, exist_ok=True)

    journal = runtime_dir / "run-durations.jsonl"
    baseline_file = runtime_dir / "run-speed-baseline.json"

    # Append this run's duration
    try:
        with open(journal, "a", encoding="utf-8") as f:
            f.write(_json.dumps({"duration_s": round(duration, 3)}) + "\n")
    except OSError:
        return  # best-effort, never fail the suite for logging

    # Read all durations to check calibration
    try:
        durations: list[float] = []
        for line in journal.read_text(encoding="utf-8").splitlines():
            line = line.strip()
            if line:
                durations.append(_json.loads(line)["duration_s"])
    except (OSError, KeyError, _json.JSONDecodeError):
        return

    # Calibrate baseline on 5th record (or later if baseline missing)
    if len(durations) >= 5 and not baseline_file.exists():
        first5 = durations[:5]
        baseline = sum(first5) / len(first5) * 1.2
        try:
            baseline_file.write_text(
                _json.dumps(
                    {"baseline_s": round(baseline, 3), "calibrated_from": first5}
                ),
                encoding="utf-8",
            )
        except OSError:
            pass  # best-effort
