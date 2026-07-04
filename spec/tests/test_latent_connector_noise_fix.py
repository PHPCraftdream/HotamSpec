"""Tests for the P5 LATENT_CONNECTOR noise fix (task #94).

Before the fix, `latent_connector_suspects` flagged ANY pair of requirements
sharing >= 1 assumption id — including framework-generic assumptions (e.g.
A-prose-suffices, A-stakeholders-care) referenced by dozens of requirements.
That produced an O(n^2) false-positive explosion (3555 suspects on the real
meta-domain graph). The fix distinguishes SPECIFIC assumptions (referenced by
few requirements) from GENERIC ones (referenced by many) and only flags pairs
sharing a SPECIFIC assumption.

References: R-conflict-* (latent-connector heuristic is §Conflict), the real
number 3555 is the pre-fix suspect count on domains/hotam-spec-self/graph.py.
"""

from __future__ import annotations

import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

from hotam_spec.graph import (  # noqa: E402
    GENERIC_ASSUMPTION_THRESHOLD,
    TensionGraph,
    latent_connector_clusters,
    latent_connector_suspects,
    load_content_graph,
)
from hotam_spec.requirement import SETTLED, Requirement  # noqa: E402
from hotam_spec.stakeholder import Stakeholder  # noqa: E402

import what_now  # noqa: E402


def _pair_flagged(suspects, left_id: str, right_id: str) -> bool:
    left, right = sorted((left_id, right_id))
    return any(s.left == left and s.right == right for s in suspects)


def test_pair_sharing_only_generic_assumption_not_flagged() -> None:
    """A pair sharing ONLY a generic (widely-referenced) assumption is not flagged."""
    n_padding = GENERIC_ASSUMPTION_THRESHOLD + 3  # comfortably cross the threshold
    padding_reqs = tuple(
        Requirement(
            id=f"R-pad-{i}",
            claim=f"padding claim {i}",
            owner="ST-padder",
            status=SETTLED,
            assumptions=("A-generic",),
        )
        for i in range(n_padding)
    )
    r_a = Requirement(
        id="R-target-a",
        claim="target claim a",
        owner="ST-padder",
        status=SETTLED,
        assumptions=("A-generic",),
    )
    r_b = Requirement(
        id="R-target-b",
        claim="target claim b",
        owner="ST-padder",
        status=SETTLED,
        assumptions=("A-generic",),
    )
    g = TensionGraph(requirements=(*padding_reqs, r_a, r_b))

    suspects = latent_connector_suspects(g)

    assert not _pair_flagged(suspects, "R-target-a", "R-target-b")


def test_pair_sharing_specific_assumption_still_flagged() -> None:
    """A pair sharing a RARE assumption (referenced only by them) is still flagged."""
    r_a = Requirement(
        id="R-rare-a",
        claim="rare claim a",
        owner="ST-owner",
        status=SETTLED,
        assumptions=("A-rare",),
    )
    r_b = Requirement(
        id="R-rare-b",
        claim="rare claim b",
        owner="ST-owner",
        status=SETTLED,
        assumptions=("A-rare",),
    )
    g = TensionGraph(requirements=(r_a, r_b))

    suspects = latent_connector_suspects(g)

    assert _pair_flagged(suspects, "R-rare-a", "R-rare-b")


def test_pair_sharing_both_generic_and_specific_still_flagged() -> None:
    """A pair sharing one generic AND one specific assumption is flagged (specific alone suffices)."""
    n_padding = GENERIC_ASSUMPTION_THRESHOLD + 3
    padding_reqs = tuple(
        Requirement(
            id=f"R-pad2-{i}",
            claim=f"padding claim {i}",
            owner="ST-padder",
            status=SETTLED,
            assumptions=("A-generic2",),
        )
        for i in range(n_padding)
    )
    r_a = Requirement(
        id="R-mixed-a",
        claim="mixed claim a",
        owner="ST-owner",
        status=SETTLED,
        assumptions=("A-generic2", "A-rare2"),
    )
    r_b = Requirement(
        id="R-mixed-b",
        claim="mixed claim b",
        owner="ST-owner",
        status=SETTLED,
        assumptions=("A-generic2", "A-rare2"),
    )
    g = TensionGraph(requirements=(*padding_reqs, r_a, r_b))

    suspects = latent_connector_suspects(g)

    assert _pair_flagged(suspects, "R-mixed-a", "R-mixed-b")
    hit = next(s for s in suspects if {s.left, s.right} == {"R-mixed-a", "R-mixed-b"})
    assert "A-rare2" in hit.hint


def test_real_graph_suspect_count_dramatically_reduced(active_graph) -> None:
    """The real meta-domain graph's suspect count drops from 3555 to a small number.

    Pre-fix (unfiltered share-any-assumption heuristic) this was 3555 on the
    real domains/hotam-spec-self/graph.py. Empirically, after the specificity
    fix the count is in the low tens (observed: 22). We assert a generous
    upper bound rather than an exact literal so future graph edits don't
    spuriously break this test.
    """
    # Task #46, Measure 3: read the session-scoped active graph (frozen, shared
    # read-only) instead of rebuilding it per-test.
    g = active_graph
    suspects = latent_connector_suspects(g)
    assert len(suspects) < 100, (
        f"expected a small, specificity-filtered suspect count, got {len(suspects)}"
        " (pre-fix unfiltered heuristic produced 3555)"
    )


def test_what_now_p5_output_capped_with_disclosure() -> None:
    """render() caps P5 lines at p5_limit and prints a visible 'suppressed' disclosure."""
    actions = [
        what_now.Action(
            priority=what_now.P_LATENT_CONNECTOR,
            kind="LATENT_CONNECTOR",
            target=f"R-a{i}~R-b{i}",
            imperative=f"suspect pair {i}",
        )
        for i in range(30)
    ]
    out = what_now.render(actions, source_label="content", p5_limit=5)

    printed_p5_lines = [
        line for line in out.splitlines() if line.strip().startswith("[P5]")
    ]
    assert len(printed_p5_lines) == 5
    assert "suppressed" in out
    assert "25 more suppressed" in out
    assert "--p5-limit" in out or "audit_atomicity" in out


def test_what_now_p5_output_no_disclosure_when_under_cap() -> None:
    """When the P5 count is already under the cap, no disclosure line is printed."""
    actions = [
        what_now.Action(
            priority=what_now.P_LATENT_CONNECTOR,
            kind="LATENT_CONNECTOR",
            target="R-a~R-b",
            imperative="one suspect",
        )
    ]
    out = what_now.render(actions, source_label="content", p5_limit=20)
    assert "suppressed" not in out


# ---------------------------------------------------------------------------
# Clustering by shared-assumption signature
# (R-latent-connectors-cluster-by-assumption)
# ---------------------------------------------------------------------------


def _cluster_graph() -> TensionGraph:
    """Well-formed graph: 3 reqs share A-rare-x (3 pairs, 1 cluster) + 2 reqs
    share A-rare-y (1 pair, 1 cluster)."""
    sh = (Stakeholder(id="ST-owner", name="Owner", domain="d"),)
    reqs_x = tuple(
        Requirement(
            id=f"R-x{i}",
            claim=f"claim x{i}",
            owner="ST-owner",
            status=SETTLED,
            assumptions=("A-rare-x",),
        )
        for i in range(3)
    )
    reqs_y = tuple(
        Requirement(
            id=f"R-y{i}",
            claim=f"claim y{i}",
            owner="ST-owner",
            status=SETTLED,
            assumptions=("A-rare-y",),
        )
        for i in range(2)
    )
    return TensionGraph(stakeholders=sh, requirements=(*reqs_x, *reqs_y))


def test_clusters_group_pairs_by_signature() -> None:
    """3 requirements on one rare assumption = 3 pairs but exactly ONE cluster."""
    g = _cluster_graph()
    suspects = latent_connector_suspects(g)
    clusters = latent_connector_clusters(g)

    assert len(suspects) == 4  # C(3,2)=3 pairs on A-rare-x + 1 pair on A-rare-y
    assert len(clusters) == 2

    by_sig = {cl.assumptions: cl for cl in clusters}
    cx = by_sig[("A-rare-x",)]
    assert cx.requirements == ("R-x0", "R-x1", "R-x2")
    assert len(cx.pairs) == 3
    cy = by_sig[("A-rare-y",)]
    assert cy.requirements == ("R-y0", "R-y1")
    assert len(cy.pairs) == 1


def test_clusters_partition_suspects() -> None:
    """Every suspect pair lands in exactly one cluster (pair detail preserved)."""
    for g in (_cluster_graph(), load_content_graph()):
        suspects = latent_connector_suspects(g)
        clusters = latent_connector_clusters(g)
        cluster_pairs = [
            (p.left, p.right) for cl in clusters for p in cl.pairs
        ]
        assert sorted(cluster_pairs) == sorted((s.left, s.right) for s in suspects)
        assert len(cluster_pairs) == len(set(cluster_pairs))


def test_clusters_deterministic() -> None:
    """Same graph → identical cluster tuple (ordering is stable)."""
    g = _cluster_graph()
    assert latent_connector_clusters(g) == latent_connector_clusters(g)


def test_what_now_p5_one_line_per_cluster() -> None:
    """diagnose() emits ONE P5 latent-connector action per cluster, not per pair."""
    g = _cluster_graph()
    actions = what_now.diagnose(g)
    p5 = [
        a
        for a in actions
        if a.priority == what_now.P_LATENT_CONNECTOR
        and "[HEURISTIC, for AI review]" in a.imperative
    ]
    assert len(p5) == 2, f"expected one action per cluster, got {p5}"
    targets = {a.target for a in p5}
    assert targets == {"A-rare-x", "A-rare-y"}
    x_action = next(a for a in p5 if a.target == "A-rare-x")
    assert "3 requirements" in x_action.imperative
    assert "review the cluster" in x_action.imperative


def test_real_graph_p5_count_equals_cluster_count() -> None:
    """On the real domain graph the P5 band size is the cluster count (plus any
    entity-state suspects), not the pair count."""
    from hotam_spec.graph import entity_state_conflict_suspects

    g = load_content_graph()
    clusters = latent_connector_clusters(g)
    suspects = latent_connector_suspects(g)
    actions = what_now.diagnose(g)
    p5 = [a for a in actions if a.priority == what_now.P_LATENT_CONNECTOR]
    assert len(p5) == len(clusters) + len(entity_state_conflict_suspects(g))
    if len(suspects) > len(clusters):
        assert len(p5) < len(suspects), (
            "clustering must compress the P5 band when pairs share a signature"
        )
