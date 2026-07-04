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

import sys
from pathlib import Path

import pytest

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_TOOLS = str(_SPEC_ROOT / "tools")
if _TOOLS not in sys.path:
    sys.path.insert(0, _TOOLS)


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
