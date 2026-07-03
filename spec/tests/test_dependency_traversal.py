"""Enforcers for R-dependency-drives-sequential and R-dependency-drives-parallel.

These tests are the ENFORCED backing for the two dependency requirements: they
check the CLAIM, not merely that the traversal runs.

  - R-dependency-drives-sequential: dependency CHAINS are emitted in the order
    the work must happen — dependency BEFORE dependent (reverse-topological /
    leaf-first), so processing a chain left-to-right is a valid sequential order.
  - R-dependency-drives-parallel: DISJOINT connected components share no
    depends_on edge, so they are identified as parallelizable slices.

Synthetic graphs make the assertions deterministic and independent of any live
domain's content; one positive check runs against the live hotam-dev graph
(the domain that actually carries depends_on edges).
"""

from __future__ import annotations

import os
import sys
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from hotam_spec.graph import (  # noqa: E402
    TensionGraph,
    dependency_chains,
    independent_subgraphs,
    load_content_graph,
)
from hotam_spec.requirement import Relation, Requirement  # noqa: E402


def _req(rid: str, deps: tuple[str, ...] = ()) -> Requirement:
    return Requirement(
        id=rid,
        claim=f"claim {rid}",
        owner="o",
        status="SETTLED",
        relations=tuple(Relation(kind="depends_on", target=t) for t in deps),
    )


def _graph(*reqs: Requirement) -> TensionGraph:
    return TensionGraph(requirements=tuple(reqs))


# ---------------------------------------------------------------------------
# R-dependency-drives-sequential
# ---------------------------------------------------------------------------


def test_chain_is_emitted_dependency_before_dependent():
    """A depends_on B depends_on C ==> chain (C, B, A): dep first, then dependent."""
    g = _graph(_req("R-a", ("R-b",)), _req("R-b", ("R-c",)), _req("R-c"))
    chains = dependency_chains(g)
    assert chains == (("R-c", "R-b", "R-a"),)
    # The claim: every dependency precedes its dependent in the emitted order.
    (chain,) = chains
    pos = {rid: i for i, rid in enumerate(chain)}
    # R-a depends_on R-b -> R-b must come first; R-b depends_on R-c -> R-c first.
    assert pos["R-b"] < pos["R-a"]
    assert pos["R-c"] < pos["R-b"]


def test_branching_chains_each_maximal_and_ordered():
    """A branch (B depended on by both A and D) yields two ordered maximal chains."""
    g = _graph(
        _req("R-a", ("R-b",)),
        _req("R-d", ("R-b",)),
        _req("R-b", ("R-c",)),
        _req("R-c"),
    )
    chains = dependency_chains(g)
    assert ("R-c", "R-b", "R-a") in chains
    assert ("R-c", "R-b", "R-d") in chains
    for chain in chains:
        pos = {rid: i for i, rid in enumerate(chain)}
        assert pos["R-c"] < pos["R-b"]


def test_isolated_requirement_yields_no_chain():
    """A requirement with no depends_on edge is not a chain (needs >= 1 edge)."""
    assert dependency_chains(_graph(_req("R-solo"))) == ()


def test_cycle_yields_no_linear_chain():
    """A depends_on cycle has no maximal linear path -> no chain, no infinite loop."""
    g = _graph(_req("R-x", ("R-y",)), _req("R-y", ("R-x",)))
    # Neither node is a root (each is depended-upon), so no chain is emitted;
    # the important property is termination without error.
    assert dependency_chains(g) == ()


def test_dangling_dependency_target_is_skipped():
    """A depends_on edge to an absent id does not crash and forms no chain."""
    assert dependency_chains(_graph(_req("R-a", ("R-missing",)))) == ()


# ---------------------------------------------------------------------------
# R-dependency-drives-parallel
# ---------------------------------------------------------------------------


def test_disjoint_components_are_parallelizable():
    """Two graphs with no shared depends_on edge are separate components."""
    g = _graph(
        _req("R-a", ("R-b",)),
        _req("R-b"),
        _req("R-c", ("R-d",)),
        _req("R-d"),
    )
    comps = independent_subgraphs(g)
    assert ("R-a", "R-b") in comps
    assert ("R-c", "R-d") in comps
    # The parallel claim: no depends_on edge crosses two distinct components.
    comp_of = {rid: i for i, c in enumerate(comps) for rid in c}
    for r in g.requirements:
        for rel in r.relations:
            if rel.kind == "depends_on" and rel.target in comp_of:
                assert comp_of[r.id] == comp_of[rel.target]


def test_shared_dependency_merges_into_one_component():
    """A and C both depending on B are ONE component (not parallelizable)."""
    g = _graph(_req("R-a", ("R-b",)), _req("R-c", ("R-b",)), _req("R-b"))
    comps = independent_subgraphs(g)
    assert comps == (("R-a", "R-b", "R-c"),)


def test_isolated_requirement_is_its_own_component():
    g = _graph(_req("R-solo"))
    assert independent_subgraphs(g) == (("R-solo",),)


# ---------------------------------------------------------------------------
# Live positive check — the domain that actually carries depends_on edges.
# ---------------------------------------------------------------------------


def test_live_hotam_dev_has_dependency_structure():
    """hotam-dev carries a real dependency chain and independent units."""
    prev = os.environ.get("HOTAM_SPEC_ACTIVE_DOMAIN")
    os.environ["HOTAM_SPEC_ACTIVE_DOMAIN"] = "hotam-dev"
    try:
        g = load_content_graph()
    finally:
        if prev is None:
            os.environ.pop("HOTAM_SPEC_ACTIVE_DOMAIN", None)
        else:
            os.environ["HOTAM_SPEC_ACTIVE_DOMAIN"] = prev

    chains = dependency_chains(g)
    assert chains, "hotam-dev must carry at least one depends_on chain"
    # Every chain is ordered dependency-before-dependent.
    ids = {r.id for r in g.requirements}
    for chain in chains:
        pos = {rid: i for i, rid in enumerate(chain)}
        for r in g.requirements:
            if r.id not in pos:
                continue
            for rel in r.relations:
                if rel.kind == "depends_on" and rel.target in pos:
                    assert pos[rel.target] < pos[r.id]

    comps = independent_subgraphs(g)
    # Union of components covers exactly the requirement set (partition).
    covered = {rid for c in comps for rid in c}
    assert covered == ids
    # At least two components (there ARE parallelizable, disjoint units).
    assert len(comps) >= 2
