"""Tests: apply_proposal.py --batch stress-obkatka on 30+ mixed proposals (Etap F #1).

Adoption-prep context (docs/reviews/2026-07-10-lens-4-applicability.md #5, "batch-режим
apply_proposal существует (--batch), но на потоке из 32+15 узлов ни разу не обкатан"):
the steward's own adoption plan (PLAN-hotamspec-adoption.md, read as context, not
executed here) lands business domains in waves of 15-32 nodes. This module is the
first real exercise of --batch at that scale.

Two tests, two different costs:

  test_batch_write_only_scales_linearly_not_quadratically (FAST, always runs in T2):
    30 mixed proposals (Stakeholder/Axis/Assumption/Requirement/Conflict), WRITE-ONLY
    (apply(..., defer_verify=True), the exact path main()'s --batch loop now uses per
    item — see the fix below). No subprocess is spawned (gen_spec/pytest mocked out via
    the same patch.object(apply_proposal, "subprocess", ...) pattern as
    test_apply_proposal_gate_wiring.py). Measures wall-clock of the WRITE phase itself
    (repeated ast.parse() + string-splice per item) to confirm it is not secretly
    quadratic (each item re-parses the WHOLE graph.py source from disk — O(n) per item,
    O(n^2) total across n items — this test's timing assertion is generous enough to
    tolerate that up to the graph sizes this framework actually reaches; the DOMINANT
    cost this stress-test exists to catch is the regen/verify SUBPROCESS multiplication
    below, not AST parsing, which is µs-to-ms even for hundreds of nodes).

  test_batch_collapses_regen_and_verify_to_a_single_pass (FAST, always runs in T2):
    Regression guard for the REAL degradation this stress-test found: main()'s --batch
    loop used to call apply() N times with its DEFAULT (non-deferred) behavior — 2
    gen_spec.py subprocess calls (--docs-only pass + full pass) + 1 pytest subprocess
    call PER ITEM. Because a fresh-domain batch is almost entirely brand-new-node
    creation, tools/gate.py's select_tier1() fails closed to the T2 full suite for
    nearly every item (Stakeholder/Axis/Assumption targets are never even attempted —
    gate.select_tier1 only recognizes Requirement/Conflict targets, see gate.py's own
    docstring). Measured against this repo's own calibrated baseline
    (spec/.runtime/run-speed-baseline.json: ~60-75s per T2 run), the PRE-FIX behavior
    would cost a 30-item batch ~30-40 MINUTES of redundant, byte-identical regen+verify
    work — only the LAST regen+verify (over the fully-written graph) is load-bearing.
    The fix (apply(..., defer_verify=True) per item + ONE _regen_and_verify call after
    the loop) collapses N regen/verify passes to exactly 1. This test asserts the call
    COUNT (mocked subprocess.run), not wall-clock, so it stays fast and deterministic.

Both tests write into an isolated tmp_path copy of a MINIMAL but real build_graph()
shape (create_domain.py's own template — the exact AST contract §2 of this Etap
documents), never touching spec/content/graph.py or any real domain.

WHY NOT a real end-to-end subprocess run (like test_e2e_consumer_subprocess.py):
apply_proposal.py's verify tier shells out to `pytest -q` with
cwd=SPEC_ROOT (THIS repo's own spec/tests, by design — R-tiered-gate-not-a-commit-gate:
the verify step confirms the FRAMEWORK still passes its own tests after a consumer's
graph write, which is correct even for a foreign/consumer domain). But
tests/test_constitution.py hardcodes
`REPO_ROOT = Path(__file__).resolve().parents[2]` (this repo's own root, not
HOTAM_SPEC_PROJECT_ROOT-aware) while resolving the ACTIVE DOMAIN name from env/pin —
so a real T2 run with a foreign active-domain pin active looks for
`<this-repo>/domains/<foreign-domain-name>/docs/gen/FRAMEWORK-INVARIANTS.md`, which
does not exist, and fails. This is the KNOWN, already-tracked DOMAIN_COUPLED test class
(tests/conftest.py's own registry lists exactly this test) — not a new bug, and not in
scope to fix here (it is a pre-existing test-suite portability gap for RUNNING THE REAL
SUITE under a foreign domain pin, orthogonal to the batch degradation this module
exists to fix). Mocking subprocess.run (the same pattern already used by
test_apply_proposal_gate_wiring.py) sidesteps this landmine entirely while still
proving the call-count collapse the fix delivers.

A third test, test_batch_real_gen_spec_timing_e2e, measures REAL wall-clock cost of the
2 gen_spec.py subprocess calls the single post-batch _regen_and_verify pass makes (the
part of the pipeline that is safe to run for real against a foreign/consumer domain --
gen_spec.py already self-guards against cross-domain contamination, see
test_portability_w4_smoke_e2e.py's
test_consumer_gen_spec_never_touches_framework_thinking_or_tool_docs). It stubs ONLY the
final pytest verify call (the one step that is NOT foreign-domain-safe today, per the
DOMAIN_COUPLED landmine above) so the test stays both real AND repo-safe. Gated behind
HOTAM_SPEC_RUN_BATCH_STRESS=1 (the same skipif-gating shape as
test_e2e_consumer_subprocess.py's HOTAM_SPEC_RUN_E2E_SUBPROCESS, chosen so a real
subprocess-spawning test never silently taxes the default T2 run --
R-run-speed-guarded): run it explicitly with

    HOTAM_SPEC_RUN_BATCH_STRESS=1 .venv/Scripts/python.exe -m pytest \\
        spec/tests/test_apply_proposal_batch_stress.py -v -s -k real_gen_spec_timing
"""

from __future__ import annotations

import os
import time
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

import apply_proposal
from hotam_spec.proposal import (
    ProposedAssumption,
    ProposedAxis,
    ProposedConflict,
    ProposedRequirement,
    ProposedStakeholder,
)

# See the module docstring's "third test" paragraph: gated the same shape as
# test_e2e_consumer_subprocess.py's HOTAM_SPEC_RUN_E2E_SUBPROCESS — an explicit
# opt-in env var, not just a marker, since the fixed T2 invocation (`python -m
# pytest -q`, no `-m` filters) would never apply marker deselection anyway.
_RUN_BATCH_STRESS = os.environ.get("HOTAM_SPEC_RUN_BATCH_STRESS") == "1"
_BATCH_STRESS_SKIP_REASON = (
    "real-subprocess batch timing test (2 real gen_spec.py calls): run "
    "explicitly with HOTAM_SPEC_RUN_BATCH_STRESS=1"
)

# The exact minimal build_graph() shape create_domain.py scaffolds for every new
# consumer domain (§2 of Etap F's PROPOSAL-REFERENCE.md addition documents this
# contract): five separate top-level tuple assignments inside build_graph(), each
# locatable by tools/apply_proposal.py's AST-based _find_module_tuple_end().
_SCAFFOLD_GRAPH_SOURCE = '''\
"""Canon: §Domain — content graph of a synthetic stress-test domain."""

from __future__ import annotations

from hotam_spec.assumption import Assumption
from hotam_spec.axis import Axis
from hotam_spec.conflict import Conflict, conflict_identity
from hotam_spec.graph import TensionGraph
from hotam_spec.requirement import ENFORCED, PROSE, STRUCTURAL, Requirement
from hotam_spec.stakeholder import Stakeholder

# ENFORCED/PROSE/STRUCTURAL must be in scope: apply_proposal's writer emits
# `enforcement=PROSE` as a bare name (see tools/create_domain.py's own
# _GRAPH_PY_TEMPLATE comment for the full rationale — this fixture mirrors
# that template, including the fix this stress-test itself motivated).
_ = (
    Assumption,
    Axis,
    Conflict,
    conflict_identity,
    ENFORCED,
    PROSE,
    STRUCTURAL,
    Requirement,
    Stakeholder,
)


def build_graph() -> TensionGraph:
    stakeholders = (
    )
    axes = (
    )
    requirements = (
    )
    conflicts = (
    )
    assumptions = (
    )
    return TensionGraph(
        axes=axes,
        stakeholders=stakeholders,
        requirements=requirements,
        conflicts=conflicts,
        assumptions=assumptions,
    )
'''


def _build_mixed_batch(n_per_kind: int = 6) -> list[dict]:
    """A realistic mixed adoption-wave batch: Stakeholder + Axis + Assumption +
    Requirement + Conflict interleaved, 30+ items total (n_per_kind=6 * 5 kinds
    + interleaving Conflicts referencing earlier Requirements = 30+ items,
    matching the order of magnitude of the steward's own adoption plan (prat:
    32 nodes, gpsm: 15 — PLAN-hotamspec-adoption.md, read as context only).
    """
    items: list[dict] = []
    stakeholder_ids = [f"stress-sh-{i}" for i in range(n_per_kind)]
    axis_slugs = [f"stress-axis-{i}" for i in range(n_per_kind)]
    req_ids = [f"R-stress-{i}" for i in range(n_per_kind * 2)]

    for i, sid in enumerate(stakeholder_ids):
        items.append(
            {
                "kind": "Stakeholder",
                "id": sid,
                "name": f"Stress Stakeholder {i}",
                "domain": "stress-probe",
                "why": "synthetic batch stress item",
            }
        )
    for i, slug in enumerate(axis_slugs):
        items.append(
            {
                "kind": "Axis",
                "slug": slug,
                "description": f"synthetic axis {i} for batch stress",
                "why": "synthetic batch stress item",
            }
        )
    for i in range(n_per_kind):
        items.append(
            {
                "kind": "Assumption",
                "id": f"A-stress-{i}",
                "statement": f"synthetic assumption {i} holds for the stress probe",
                "status": "HOLDS",
                "owner": stakeholder_ids[i % n_per_kind],
                "why": "synthetic batch stress item",
            }
        )
    for i, rid in enumerate(req_ids):
        items.append(
            {
                "kind": "Requirement",
                "id": rid,
                "claim": f"the system shall do synthetic stress thing {i}",
                "owner": stakeholder_ids[i % n_per_kind],
                "status": "DRAFT",
                "why": "synthetic batch stress item",
            }
        )
    # A handful of Conflicts, each connecting two of the just-created Requirements
    # (members must be pre-existing R-... ids — R-conflict-min-two-members). The
    # steward must NOT own either member (R-steward-distinct-from-owners) — the
    # Requirement loop above assigns owner=stakeholder_ids[i % n_per_kind] where i
    # is the req_ids INDEX, so member owners are computed the same way here and the
    # steward is picked as the first stakeholder id that owns NEITHER member.
    for i in range(n_per_kind // 2):
        member_a, member_b = req_ids[2 * i], req_ids[2 * i + 1]
        owner_a = stakeholder_ids[(2 * i) % n_per_kind]
        owner_b = stakeholder_ids[(2 * i + 1) % n_per_kind]
        steward = next(
            sid for sid in stakeholder_ids if sid not in (owner_a, owner_b)
        )
        items.append(
            {
                "kind": "Conflict",
                "axis": axis_slugs[i % n_per_kind],
                "context": f"synthetic stress conflict context {i}",
                "members": [member_a, member_b],
                "steward": steward,
                "why": "synthetic batch stress item",
            }
        )
    return items


def _batch_item_anchor(item: dict) -> str:
    """The literal source-text needle to look for post-write, per proposal kind.

    Stakeholder/Assumption/Requirement carry an explicit `id`; Axis carries
    `slug`; Conflict has NO caller-supplied id (the writer computes
    conflict_identity(axis, context) — R-stable-conflict-identity), so its
    presence is instead confirmed via its (axis, context) string pair, both
    of which are rendered verbatim into the written Conflict(...) call.
    """
    if item["kind"] == "Conflict":
        return item["context"]
    return item.get("id") or item.get("slug")


def _fake_subprocess_run(calls: list[list[str]]):
    """subprocess.run replacement: records argv, always reports success, never
    actually spawns gen_spec.py or pytest (keeps this test fast + repo-safe)."""

    def _run(args, **kwargs):  # noqa: ANN001, ANN003
        calls.append([str(a) for a in args])
        result = MagicMock()
        result.returncode = 0
        result.stdout = ""
        result.stderr = ""
        return result

    return MagicMock(side_effect=_run)


def test_batch_write_only_scales_linearly_not_quadratically(tmp_path: Path) -> None:
    """30+ mixed proposals, WRITE-ONLY (defer_verify=True): confirms the write
    phase (ast.parse + splice per item, no subprocess) does not blow up
    superlinearly at this scale — a generous ceiling, not a tight benchmark
    (the point of this stress-test is the subprocess collapse below; this
    test exists so a future O(n^2) regression in the AST-splice path itself
    would still be caught)."""
    graph_file = tmp_path / "graph.py"
    graph_file.write_text(_SCAFFOLD_GRAPH_SOURCE, encoding="utf-8")

    batch = _build_mixed_batch(n_per_kind=6)
    assert len(batch) >= 30, f"expected a 30+-item batch, got {len(batch)}"

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = graph_file
    try:
        t0 = time.time()
        for item in batch:
            proposal = apply_proposal._validate_proposal(item)
            rc = apply_proposal.apply(proposal, defer_verify=True, proposal_file=None)
            assert rc == 0, f"item {item.get('kind')}/{item.get('id', item.get('slug'))} failed rc={rc}"
        elapsed = time.time() - t0
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    # Generous ceiling: even a naive O(n^2) re-parse of a FEW HUNDRED lines of
    # source (this graph never grows past ~30 nodes in this test) would still
    # finish in well under a second; 10s leaves headroom for slow CI hosts
    # while still catching a genuine algorithmic blowup (e.g. accidentally
    # re-reading/re-parsing the whole graph N times per item instead of once).
    assert elapsed < 10.0, (
        f"write-only phase for {len(batch)} items took {elapsed:.2f}s — "
        "investigate for accidental superlinear behavior in the AST-splice path."
    )

    final_source = graph_file.read_text(encoding="utf-8")
    for item in batch:
        anchor = _batch_item_anchor(item)
        assert anchor in final_source, f"{anchor!r} missing from final graph.py"


def test_batch_collapses_regen_and_verify_to_a_single_pass(tmp_path: Path) -> None:
    """Regression guard: main()'s --batch loop must call the regen+verify
    subprocess pair exactly ONCE for the whole batch, not once per item.

    Drives apply_proposal.main(["--batch", str(batch_file)]) end-to-end (the
    real CLI entry point, not the internal apply() helper) with
    subprocess.run mocked out, so gen_spec.py --docs-only / gen_spec.py /
    pytest -q never actually spawn. Before the fix, a 30-item batch issued 30
    x (2 gen_spec calls + 1 pytest call) = 90 subprocess.run invocations, each
    standing in for a real ~60-75s T2 run
    (spec/.runtime/run-speed-baseline.json) when the gate fails closed (which
    it does for brand-new nodes — the overwhelming majority of a fresh-domain
    batch). After the fix: exactly 2 gen_spec calls + 1 pytest call for the
    ENTIRE batch, regardless of item count.
    """
    graph_file = tmp_path / "graph.py"
    graph_file.write_text(_SCAFFOLD_GRAPH_SOURCE, encoding="utf-8")

    batch = _build_mixed_batch(n_per_kind=6)
    assert len(batch) >= 30

    import json as _json

    batch_file = tmp_path / "batch.json"
    batch_file.write_text(_json.dumps(batch), encoding="utf-8")

    real_graph = apply_proposal._CONTENT_GRAPH
    apply_proposal._CONTENT_GRAPH = graph_file
    calls: list[list[str]] = []
    try:
        with patch.object(
            apply_proposal, "subprocess", MagicMock(run=_fake_subprocess_run(calls))
        ):
            rc = apply_proposal.main(["--batch", str(batch_file)])
    finally:
        apply_proposal._CONTENT_GRAPH = real_graph

    assert rc == 0, f"batch main() failed rc={rc}"

    gen_spec_calls = [c for c in calls if any("gen_spec.py" in a for a in c)]
    # pytest is invoked as `[sys.executable, "-m", "pytest", "-q", ...]` — "-m" a
    # module, not an argv token literally spelled "pytest"; match on the "-m"
    # "pytest" pair instead of a single-token check.
    pytest_calls = [c for c in calls if "pytest" in c]

    assert len(gen_spec_calls) == 2, (
        f"expected exactly 2 gen_spec.py subprocess calls for the WHOLE "
        f"{len(batch)}-item batch (1x --docs-only + 1x full regen, run ONCE "
        f"after all items are written), got {len(gen_spec_calls)}: "
        f"{gen_spec_calls}"
    )
    assert len(pytest_calls) == 1, (
        f"expected exactly 1 pytest subprocess call for the WHOLE "
        f"{len(batch)}-item batch (run ONCE after all items are written), "
        f"got {len(pytest_calls)}: {pytest_calls}"
    )

    final_source = graph_file.read_text(encoding="utf-8")
    for item in batch:
        anchor = _batch_item_anchor(item)
        assert anchor in final_source, f"{anchor!r} missing from final graph.py"


@pytest.mark.slow
@pytest.mark.skipif(not _RUN_BATCH_STRESS, reason=_BATCH_STRESS_SKIP_REASON)
def test_batch_real_gen_spec_timing_e2e(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """REAL end-to-end timing: a 30+-item batch through main(), with the two
    gen_spec.py regen passes running FOR REAL (subprocess, not mocked) against
    a synthetic consumer domain scaffolded by create_domain.py — only the
    final pytest verify subprocess is stubbed (see the module docstring's
    DOMAIN_COUPLED landmine paragraph for why: apply_proposal.py's T2 verify
    tier is not yet safe to run for real against a foreign active-domain pin;
    that is a pre-existing, separately-tracked gap, not something this batch
    fix introduces or is meant to close).

    Reports the measured wall-clock for the WRITE+REGEN phase to stdout (run
    with -s to see it) so a human can sanity-check the fix's real-world
    payoff, not just the mocked call-count assertion above.
    """
    consumer_root = tmp_path / "consumer"
    consumer_root.mkdir()
    domains_dir = consumer_root / "domains"
    domains_dir.mkdir()
    (domains_dir / ".active-domain").write_text(
        "batch-stress-consumer\n", encoding="utf-8"
    )

    import create_domain as _cd  # noqa: PLC0415

    _cd._DOMAINS_ROOT = domains_dir
    try:
        rc = _cd.scaffold(
            name="batch-stress-consumer",
            description="Synthetic consumer domain for --batch stress timing.",
            goals=["batch stress e2e timing passes"],
            director_purpose="director of batch-stress-consumer",
            domains_root=domains_dir,
        )
    finally:
        _cd._DOMAINS_ROOT = None  # type: ignore[attr-defined]
    assert rc == 0, "create_domain.scaffold failed"

    monkeypatch.setenv("HOTAM_SPEC_PROJECT_ROOT", str(consumer_root))
    monkeypatch.chdir(consumer_root)

    import apply_proposal as _ap  # fresh import path resolution under the new env

    real_content_graph = _ap._CONTENT_GRAPH
    _ap._CONTENT_GRAPH = _ap._resolve_content_graph()
    assert _ap._CONTENT_GRAPH != real_content_graph, (
        "content graph did not re-resolve to the synthetic consumer domain"
    )

    batch = _build_mixed_batch(n_per_kind=6)
    assert len(batch) >= 30

    import json as _json

    batch_file = tmp_path / "batch.json"
    batch_file.write_text(_json.dumps(batch), encoding="utf-8")

    real_subprocess_run = _ap.subprocess.run

    def _stub_only_pytest(args, **kwargs):  # noqa: ANN001, ANN003
        argv = [str(a) for a in args]
        if "pytest" in argv:
            result = MagicMock()
            result.returncode = 0
            result.stdout = ""
            result.stderr = ""
            return result
        return real_subprocess_run(args, **kwargs)

    try:
        t0 = time.time()
        with patch.object(_ap, "subprocess", MagicMock(run=_stub_only_pytest)):
            rc = _ap.main(["--batch", str(batch_file)])
        elapsed = time.time() - t0
    finally:
        _ap._CONTENT_GRAPH = real_content_graph

    assert rc == 0, f"batch main() failed rc={rc}"
    print(
        f"\n[batch-stress-e2e] {len(batch)} items, 2 REAL gen_spec.py passes "
        f"(pytest verify stubbed): {elapsed:.2f}s wall-clock "
        f"({elapsed / len(batch):.3f}s/item average — but this is a FIXED "
        f"2-call cost regardless of item count post-fix, not a per-item cost)."
    )
    # Two real gen_spec.py subprocess passes for a small synthetic domain
    # should comfortably finish well under a minute; a much larger number
    # would indicate the fix regressed (e.g. a stray per-item regen call
    # reintroduced). Generous ceiling for slow CI hosts.
    assert elapsed < 90.0, (
        f"real gen_spec-only batch pass took {elapsed:.2f}s — investigate "
        "for a reintroduced per-item regen call."
    )
