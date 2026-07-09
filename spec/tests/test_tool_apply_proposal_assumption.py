"""Tests for tools/apply_proposal.py — ProposedAssumption kind (§Proposal / §Assumption).

Covers:
  (a) ProposedAssumption applies and appends a valid Assumption node.
  (b) a duplicate assumption id is rejected.
  (c) validation rejects bad status / missing fields.
  (d) after the real repointing (Wave 7 move 4), the former 3-way
      A-content-free-honest latent-connector cluster is gone from
      latent_connector_clusters(g) on the real hotam-spec-self domain graph.
"""

from __future__ import annotations


import pytest


import apply_proposal  # noqa: E402
from hotam_spec.proposal import ProposedAssumption  # noqa: E402

_SAMPLE_SOURCE = '''\
from __future__ import annotations

from hotam_spec.assumption import Assumption, HOLDS, UNCERTAIN


def build_graph():
    assumptions = (
        Assumption(
            id="A-existing",
            statement="Something already believed.",
            status=HOLDS,
            owner="framework-author",
        ),
    )

    requirements = (
    )
'''


# ---------------------------------------------------------------------------
# (a) validation happy path
# ---------------------------------------------------------------------------


def test_validate_assumption_happy_path() -> None:
    raw = {
        "kind": "Assumption",
        "id": "A-foo",
        "statement": "A falsifiable belief.",
        "status": "HOLDS",
        "owner": "framework-author",
        "why": "because",
    }
    proposal = apply_proposal._validate_assumption(raw)
    assert isinstance(proposal, ProposedAssumption)
    assert proposal.id == "A-foo"
    assert proposal.target_anchor() == "A-foo"


@pytest.mark.parametrize(
    "raw,message_fragment",
    [
        ({"statement": "x", "status": "HOLDS", "owner": "o"}, "'id' is required"),
        (
            {"id": "not-a-anchor", "statement": "x", "status": "HOLDS", "owner": "o"},
            "must start with 'A-'",
        ),
        ({"id": "A-foo", "status": "HOLDS", "owner": "o"}, "'statement' is required"),
        (
            {"id": "A-foo", "statement": "x", "status": "BOGUS", "owner": "o"},
            "'status' must be one of",
        ),
        (
            {"id": "A-foo", "statement": "x", "status": "HOLDS"},
            "'owner' is required",
        ),
    ],
)
def test_validate_assumption_rejects_bad_input(raw, message_fragment) -> None:
    with pytest.raises(ValueError, match=message_fragment):
        apply_proposal._validate_assumption(raw)


# ---------------------------------------------------------------------------
# (b) apply: appends a new Assumption; refuses duplicate id
# ---------------------------------------------------------------------------


def test_apply_assumption_appends_new_node() -> None:
    proposal = ProposedAssumption(
        id="A-new-one",
        statement="A brand new falsifiable belief.",
        status="UNCERTAIN",
        owner="framework-reviewer",
    )
    result = apply_proposal._apply_assumption_to_source(_SAMPLE_SOURCE, proposal)
    assert 'id="A-new-one"' in result
    assert "UNCERTAIN" in result
    # Original assumption preserved.
    assert 'id="A-existing"' in result
    # Still parses as valid Python.
    import ast

    ast.parse(result)


def test_apply_assumption_refuses_duplicate_id() -> None:
    proposal = ProposedAssumption(
        id="A-existing",
        statement="Trying to re-declare.",
        status="HOLDS",
        owner="framework-author",
    )
    with pytest.raises(RuntimeError, match="already exists"):
        apply_proposal._apply_assumption_to_source(_SAMPLE_SOURCE, proposal)


def test_validate_proposal_dispatches_assumption_kind() -> None:
    raw = {
        "kind": "Assumption",
        "id": "A-bar",
        "statement": "stmt",
        "status": "DEAD",
        "owner": "framework-author",
    }
    proposal = apply_proposal._validate_proposal(raw)
    assert isinstance(proposal, ProposedAssumption)


# ---------------------------------------------------------------------------
# (d) real-graph regression: the former A-content-free-honest 3-way cluster
#     (R-agent-code-imports-framework, R-conflict-is-connector-node,
#     R-framework-owned-by-no-agent) is gone.
# ---------------------------------------------------------------------------


def test_content_free_honest_latent_cluster_is_resolved_on_real_domain() -> None:
    from hotam_spec.graph import latent_connector_clusters, load_content_graph
    from hotam_spec.requirement import REJECTED

    g = load_content_graph()

    # No non-REJECTED requirement still points at the over-broad assumption.
    referrers = [
        r.id
        for r in g.requirements
        if r.status != REJECTED and "A-content-free-honest" in r.assumptions
    ]
    assert referrers == [], (
        f"A-content-free-honest should no longer be referenced by any "
        f"non-REJECTED requirement (Wave 7 move 4); found: {referrers}"
    )

    # The specific 3-way cluster is gone from latent_connector_clusters.
    clusters = latent_connector_clusters(g)
    stale_clusters = [c for c in clusters if "A-content-free-honest" in c.assumptions]
    assert stale_clusters == [], (
        f"P5 latent-connector cluster for A-content-free-honest should be "
        f"resolved; still present: {stale_clusters}"
    )

    # The three previously-clustered requirements each carry their own,
    # narrower assumption now.
    by_id = {r.id: r for r in g.requirements}
    assert by_id["R-conflict-is-connector-node"].assumptions == (
        "A-conflict-is-a-node-not-an-edge",
    )
    assert by_id["R-agent-code-imports-framework"].assumptions == (
        "A-agent-code-imports-framework-directionally",
    )
    assert by_id["R-framework-owned-by-no-agent"].assumptions == (
        "A-framework-shared-infra-no-owner",
    )
