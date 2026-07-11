"""Tests for gen_spec.build_graph_json — the READ-ONLY machine-readable snapshot.

graph.json is a GENERATED export of the graph nodes (Canon: §Graph). The single
source of truth stays graph.py (storage = the Python code itself,
R-per-node-json-store REJECTED); this snapshot only lets a non-Python reader
consume the node roster. These tests pin: (1) it is valid JSON, (2) it carries
the expected top-level structure including a flat node_ids list, (3) node_ids
matches the nodes actually in the graph, (4) generation is deterministic.
"""

from __future__ import annotations

import json

import gen_spec  # noqa: E402

from fixtures.seed import seed_graph  # noqa: E402


def _load(g) -> dict:
    """Build graph.json for graph g and parse it back into a dict."""
    text = gen_spec.build_graph_json(g)
    return json.loads(text)


def test_graph_json_is_valid_json_with_expected_structure():
    g = seed_graph()
    data = _load(g)
    for key in ("requirements", "conflicts", "assumptions", "stakeholders", "node_ids"):
        assert key in data, f"graph.json missing top-level key {key!r}"
    assert isinstance(data["node_ids"], list)
    # Every requirement row carries the stable primitive fields.
    for r in data["requirements"]:
        assert set(r).issuperset({"id", "claim", "owner", "status"})


def test_graph_json_node_ids_match_graph_nodes():
    g = seed_graph()
    data = _load(g)
    expected = sorted(
        [r.id for r in g.requirements]
        + [c.id for c in g.conflicts]
        + [a.id for a in g.assumptions]
        + [s.id for s in g.stakeholders]
    )
    assert data["node_ids"] == expected
    # node_ids is sorted (deterministic ordering).
    assert data["node_ids"] == sorted(data["node_ids"])


def test_graph_json_is_deterministic():
    g = seed_graph()
    assert gen_spec.build_graph_json(g) == gen_spec.build_graph_json(g)


def test_graph_json_ends_with_lf():
    g = seed_graph()
    assert gen_spec.build_graph_json(g).endswith("\n")


def test_committed_graph_json_matches_regeneration():
    """The committed domains/<active>/docs/gen/graph.json equals a fresh export.

    Mirrors the anti-drift meta-test for the .md files: the on-disk snapshot is
    generated, so it must match a fresh build of the active domain's graph.
    """
    from hotam_spec.graph import load_content_graph  # noqa: PLC0415

    g = load_content_graph()
    committed = gen_spec.GEN_DIR / "graph.json"
    if not committed.exists():
        # Active domain has no graph.json committed (e.g. an empty framework);
        # nothing to compare against.
        return
    on_disk = committed.read_text(encoding="utf-8").replace("\r\n", "\n")
    assert on_disk == gen_spec.build_graph_json(g)
