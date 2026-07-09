"""Tests for tools/apply_proposal.py — ProposedStakeholder kind (§Proposal / §Stakeholder).

Covers (R-proposed-stakeholder-kind-exists — the stranger's first door):
  (a) validation happy path + rejection of missing fields.
  (b) ProposedStakeholder applies and appends a valid Stakeholder node.
  (c) a duplicate stakeholder id is rejected.
  (d) _validate_proposal dispatches the 'Stakeholder' kind.
"""

from __future__ import annotations

import ast

import pytest


import apply_proposal  # noqa: E402
from hotam_spec.proposal import ProposedStakeholder  # noqa: E402

_SAMPLE_SOURCE = '''\
from __future__ import annotations

from hotam_spec.stakeholder import Stakeholder


def build_graph():
    stakeholders = (
        Stakeholder(
            id="owner-a",
            name="Owner A",
            domain="a",
        ),
    )

    requirements = (
    )
'''


# ---------------------------------------------------------------------------
# (a) validation
# ---------------------------------------------------------------------------


def test_validate_stakeholder_happy_path() -> None:
    raw = {
        "kind": "Stakeholder",
        "id": "finance",
        "name": "Finance",
        "domain": "money",
        "why": "because",
    }
    proposal = apply_proposal._validate_stakeholder(raw)
    assert isinstance(proposal, ProposedStakeholder)
    assert proposal.id == "finance"
    assert proposal.target_anchor() == "finance"


@pytest.mark.parametrize(
    "raw,message_fragment",
    [
        ({"name": "Finance", "domain": "money"}, "'id' is required"),
        ({"id": "finance", "domain": "money"}, "'name' is required"),
        ({"id": "finance", "name": "Finance"}, "'domain' is required"),
    ],
)
def test_validate_stakeholder_rejects_bad_input(raw, message_fragment) -> None:
    with pytest.raises(ValueError, match=message_fragment):
        apply_proposal._validate_stakeholder(raw)


# ---------------------------------------------------------------------------
# (b) apply: appends a new Stakeholder; refuses duplicate id
# ---------------------------------------------------------------------------


def test_apply_stakeholder_appends_new_node() -> None:
    proposal = ProposedStakeholder(
        id="platform",
        name="Platform",
        domain="latency/SLA",
    )
    result = apply_proposal._apply_stakeholder_to_source(_SAMPLE_SOURCE, proposal)
    assert 'id="platform"' in result
    assert 'name="Platform"' in result
    assert 'domain="latency/SLA"' in result
    # Original stakeholder preserved.
    assert 'id="owner-a"' in result
    # Still parses as valid Python.
    ast.parse(result)


def test_apply_stakeholder_refuses_duplicate_id() -> None:
    proposal = ProposedStakeholder(
        id="owner-a",
        name="Trying to re-declare",
        domain="a",
    )
    with pytest.raises(RuntimeError, match="already exists"):
        apply_proposal._apply_stakeholder_to_source(_SAMPLE_SOURCE, proposal)


def test_apply_stakeholder_ensures_import_when_missing() -> None:
    # A graph source lacking the Stakeholder import still gets one injected.
    source_no_import = '''\
from __future__ import annotations


def build_graph():
    stakeholders = (
    )
'''
    proposal = ProposedStakeholder(id="finance", name="Finance", domain="money")
    result = apply_proposal._apply_stakeholder_to_source(source_no_import, proposal)
    assert "from hotam_spec.stakeholder import Stakeholder" in result
    ast.parse(result)


def test_validate_proposal_dispatches_stakeholder_kind() -> None:
    raw = {
        "kind": "Stakeholder",
        "id": "reviewer",
        "name": "Reviewer",
        "domain": "governance",
    }
    proposal = apply_proposal._validate_proposal(raw)
    assert isinstance(proposal, ProposedStakeholder)
