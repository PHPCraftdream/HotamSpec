"""Tests for tools/audit_tensions.py — the generative-audit shortlist tool.

Guarantees (Wave 11, generation organ):
  1. A synthetic graph carrying a deliberate POLE-tension pair surfaces it,
     tagged POLE on the intended axis.
  2. A synthetic graph carrying a deliberate MODAL-opposition pair (one
     prohibition, one obligation, shared content) surfaces it, tagged MODAL.
  3. An empty / conflict-free graph does not crash and returns [] (vacuous).
  4. Two runs on the same graph are byte-identical in their CANDIDATES part
     (the printed shortlist); the only clock lives in the stamp file, which is
     excluded from the determinism assertion.
  5. Mediated pairs (already carried by a Conflict node) are excluded.
  6. Decomposition siblings (refine/support edge, shared refine-target, or long
     shared id-prefix) are excluded.
  7. The tool NEVER writes to any graph.py — its only side effects are stdout
     and the append-only stamp file (R-tension-audit-presents-only).
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

from hotam_spec.axis import Axis  # noqa: E402
from hotam_spec.conflict import Conflict, conflict_identity  # noqa: E402
from hotam_spec.graph import TensionGraph  # noqa: E402
from hotam_spec.requirement import ENFORCED, SETTLED, Relation, Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

import audit_tensions  # noqa: E402
from audit_tensions import _SIG_MODAL, _SIG_POLE, audit  # noqa: E402

# ---------------------------------------------------------------------------
# Shared builders
# ---------------------------------------------------------------------------

_SH = (
    Stakeholder(id="s-a", name="A", domain="x"),
    Stakeholder(id="s-b", name="B", domain="y"),
)


def _req(
    rid: str,
    claim: str,
    *,
    owner: str = "s-a",
    relations: tuple[Relation, ...] = (),
) -> Requirement:
    return Requirement(
        id=rid,
        claim=claim,
        owner=owner,
        status=SETTLED,
        enforcement=ENFORCED,
        enforced_by=(f"test_{rid}",),
        relations=relations,
    )


# ---------------------------------------------------------------------------
# 1. POLE signal
# ---------------------------------------------------------------------------


def test_pole_tension_is_found() -> None:
    """A pair pulling apart the two poles of an axis surfaces as POLE."""
    axes = (
        Axis(
            slug="latency-vs-completeness",
            description="fast partial latency answer vs slow complete full coverage",
        ),
    )
    reqs = (
        _req("R-fast", "The gateway shall return a fast low-latency partial answer."),
        _req(
            "R-thorough",
            "The auditor shall guarantee a complete full-coverage result.",
            owner="s-b",
        ),
    )
    g = TensionGraph(axes=axes, stakeholders=_SH, requirements=reqs)
    cands = audit(g)
    hit = [c for c in cands if c.key == ("R-fast", "R-thorough")]
    assert hit, "the pole-opposed pair must surface"
    assert hit[0].signal == _SIG_POLE
    assert hit[0].axis == "latency-vs-completeness"


# ---------------------------------------------------------------------------
# 2. MODAL signal
# ---------------------------------------------------------------------------


def test_modal_opposition_is_found() -> None:
    """One prohibition + one obligation sharing content surfaces as MODAL."""
    # No axis poles match, so the signal falls through to MODAL.
    axes = (Axis(slug="alpha-vs-beta", description="alpha thing vs beta thing"),)
    reqs = (
        _req(
            "R-prohibit",
            "The operator shall never delete a crystallized substrate record.",
        ),
        _req(
            "R-oblige",
            "The operator shall delete a crystallized substrate record on request.",
            owner="s-b",
        ),
    )
    g = TensionGraph(axes=axes, stakeholders=_SH, requirements=reqs)
    cands = audit(g)
    hit = [c for c in cands if c.key == ("R-oblige", "R-prohibit")]
    assert hit, "the prohibition/obligation pair must surface"
    assert hit[0].signal == _SIG_MODAL


# ---------------------------------------------------------------------------
# 3. Vacuous graph
# ---------------------------------------------------------------------------


def test_empty_graph_is_vacuous() -> None:
    """An empty graph does not crash and yields no candidates."""
    g = TensionGraph(axes=(), stakeholders=(), requirements=())
    assert audit(g) == []


def test_single_requirement_is_vacuous() -> None:
    """A one-atom graph has no pairs; no crash, no candidates."""
    g = TensionGraph(
        axes=(Axis(slug="a-vs-b", description="a vs b"),),
        stakeholders=_SH,
        requirements=(_req("R-lonely", "The system shall do exactly one thing."),),
    )
    assert audit(g) == []


# ---------------------------------------------------------------------------
# 4. Determinism (candidates part only — the stamp carries the only clock)
# ---------------------------------------------------------------------------


def test_two_runs_identical_candidates() -> None:
    """Two audits of the same graph produce identical candidate lists."""
    axes = (
        Axis(
            slug="latency-vs-completeness",
            description="fast partial latency vs slow complete coverage",
        ),
    )
    reqs = (
        _req("R-fast", "The gateway shall return a fast low-latency partial answer."),
        _req(
            "R-thorough",
            "The auditor shall guarantee a complete full-coverage result.",
            owner="s-b",
        ),
        _req(
            "R-prohibit",
            "The operator shall never delete a crystallized record.",
        ),
        _req(
            "R-oblige",
            "The operator shall delete a crystallized record when asked.",
            owner="s-b",
        ),
    )
    g = TensionGraph(axes=axes, stakeholders=_SH, requirements=reqs)
    first = audit(g)
    second = audit(g)
    assert first == second
    # And the rendered shortlist (no timestamps) is byte-identical.
    r1 = audit_tensions._render(g, first, limit=10)
    r2 = audit_tensions._render(g, second, limit=10)
    assert r1 == r2


# ---------------------------------------------------------------------------
# 5. Mediated pairs excluded
# ---------------------------------------------------------------------------


def test_mediated_pair_is_excluded() -> None:
    """A pair already carried by a Conflict node is never surfaced."""
    axes = (
        Axis(
            slug="latency-vs-completeness",
            description="fast partial latency vs slow complete coverage",
        ),
    )
    ctx = "the gateway wants speed while the auditor wants completeness"
    reqs = (
        _req("R-fast", "The gateway shall return a fast low-latency partial answer."),
        _req(
            "R-thorough",
            "The auditor shall guarantee a complete full-coverage result.",
            owner="s-b",
        ),
    )
    conflicts = (
        Conflict(
            id=conflict_identity("latency-vs-completeness", ctx),
            axis="latency-vs-completeness",
            context=ctx,
            members=("R-fast", "R-thorough"),
            steward="s-a",
            lifecycle="DETECTED",
        ),
    )
    g = TensionGraph(
        axes=axes, stakeholders=_SH, requirements=reqs, conflicts=conflicts
    )
    cands = audit(g)
    assert not [c for c in cands if c.key == ("R-fast", "R-thorough")], (
        "a mediated pair must be excluded from the shortlist"
    )


# ---------------------------------------------------------------------------
# 6. Sibling pairs excluded
# ---------------------------------------------------------------------------


def test_refine_siblings_excluded() -> None:
    """A pair joined by a refine/support edge is a decomposition sibling."""
    axes = (
        Axis(
            slug="latency-vs-completeness",
            description="fast partial latency vs slow complete coverage",
        ),
    )
    reqs = (
        _req("R-fast", "The gateway shall return a fast low-latency partial answer."),
        _req(
            "R-thorough",
            "The auditor shall guarantee a complete full-coverage result.",
            owner="s-b",
            relations=(Relation("refines", "R-fast"),),
        ),
    )
    g = TensionGraph(axes=axes, stakeholders=_SH, requirements=reqs)
    assert not [
        c for c in audit(g) if c.key == ("R-fast", "R-thorough")
    ], "a refine-sibling pair must be excluded"


def test_shared_prefix_siblings_excluded() -> None:
    """A pair with a long shared id-prefix is a split-from-one-parent sibling."""
    axes = (
        Axis(
            slug="latency-vs-completeness",
            description="fast partial latency vs slow complete coverage",
        ),
    )
    reqs = (
        _req(
            "R-content-free-no-latency",
            "The gateway shall return a fast low-latency partial answer.",
        ),
        _req(
            "R-content-free-no-completeness",
            "The auditor shall guarantee a complete full-coverage result.",
            owner="s-b",
        ),
    )
    g = TensionGraph(axes=axes, stakeholders=_SH, requirements=reqs)
    assert not audit(g), "long shared-prefix pair must be excluded as a sibling"


# ---------------------------------------------------------------------------
# 7. The tool never writes to graph.py
# ---------------------------------------------------------------------------


def test_audit_never_writes_graph_py() -> None:
    """audit() is a pure read: no domains/*/graph.py mutates during a run.

    Guards R-tension-audit-presents-only: the tool's only side effects are
    stdout and the append-only stamp; it never mutates the graph substrate.
    """
    import hashlib

    repo_root = Path(__file__).resolve().parents[2]
    graph_files = sorted(repo_root.glob("domains/*/graph.py"))
    assert graph_files, "expected at least one domain graph.py to guard"

    def _digest() -> dict[str, str]:
        return {
            str(p): hashlib.sha256(p.read_bytes()).hexdigest() for p in graph_files
        }

    before = _digest()
    axes = (Axis(slug="a-vs-b", description="a vs b"),)
    reqs = (
        _req("R-x", "The system shall be fast and low-latency partial."),
        _req("R-y", "The system shall be complete and full coverage.", owner="s-b"),
    )
    g = TensionGraph(axes=axes, stakeholders=_SH, requirements=reqs)
    audit(g)
    after = _digest()
    assert before == after, "audit() must not mutate any domain graph.py"
