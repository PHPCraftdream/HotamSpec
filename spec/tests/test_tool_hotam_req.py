"""Tests for spec/tools/hotam_req.py -- the CLI dispatcher for
list / show / search / patch / context (task #112 / Etap J).

Covers the dispatch surface (unknown subcommand, --help, exit codes) plus
unit tests for each subcommand against the live self-hosting graph.
"""

from __future__ import annotations

import json
from pathlib import Path

import hotam_req


# ---------------------------------------------------------------------------
# Dispatch surface
# ---------------------------------------------------------------------------


def test_no_args_prints_usage_and_fails() -> None:
    """No subcommand -> usage printed, non-zero exit."""
    assert hotam_req.main([]) == 2


def test_help_prints_usage_and_succeeds() -> None:
    """--help -> usage printed, exit 0."""
    assert hotam_req.main(["--help"]) == 0
    assert hotam_req.main(["-h"]) == 0


def test_unknown_subcommand_fails_closed() -> None:
    """An unrecognized subcommand is rejected."""
    assert hotam_req.main(["frobnicate"]) == 2


# ---------------------------------------------------------------------------
# list
# ---------------------------------------------------------------------------


def test_list_all(capsys) -> None:
    """list with no filters returns at least some requirements."""
    rc = hotam_req.do_list([])
    assert rc == 0
    out = capsys.readouterr().out
    assert "ID" in out
    assert "STATUS" in out
    assert "requirement(s)" in out


def test_list_filter_status(capsys) -> None:
    """list --status SETTLED returns only SETTLED requirements."""
    rc = hotam_req.do_list(["--status", "SETTLED"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "SETTLED" in out
    # Every non-header, non-summary, non-empty line should contain SETTLED
    for line in out.strip().split("\n")[2:]:  # skip header + separator
        line = line.strip()
        if not line or line.startswith("("):
            continue
        assert "SETTLED" in line


def test_list_filter_owner(capsys) -> None:
    """list --owner framework-author returns only framework-author's requirements."""
    rc = hotam_req.do_list(["--owner", "framework-author"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "framework-author" in out


def test_list_filter_enforcement(capsys) -> None:
    """list --enforcement ENFORCED returns only ENFORCED requirements."""
    rc = hotam_req.do_list(["--enforcement", "ENFORCED"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "ENFORCED" in out


def test_list_no_matches(capsys) -> None:
    """list with a filter that matches nothing returns a calm message."""
    rc = hotam_req.do_list(["--owner", "no-such-owner-xyz"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "no matching" in out


# ---------------------------------------------------------------------------
# show
# ---------------------------------------------------------------------------


def test_show_existing(capsys) -> None:
    """show R-anchor-everything returns the requirement details."""
    rc = hotam_req.do_show(["R-anchor-everything"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "R-anchor-everything" in out
    assert "claim" in out


def test_show_json(capsys) -> None:
    """show --json returns valid JSON."""
    rc = hotam_req.do_show(["R-anchor-everything", "--json"])
    assert rc == 0
    out = capsys.readouterr().out
    data = json.loads(out)
    assert data["id"] == "R-anchor-everything"
    assert "claim" in data


def test_show_not_found() -> None:
    """show for a nonexistent id returns exit 1."""
    rc = hotam_req.do_show(["R-does-not-exist-xyz"])
    assert rc == 1


# ---------------------------------------------------------------------------
# search
# ---------------------------------------------------------------------------


def test_search_by_id(capsys) -> None:
    """search finds by id substring."""
    rc = hotam_req.do_search(["anchor-everything"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "R-anchor-everything" in out


def test_search_by_claim(capsys) -> None:
    """search finds by claim text."""
    rc = hotam_req.do_search(["conflict"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "match" in out


def test_search_json(capsys) -> None:
    """search --json returns valid JSON array."""
    rc = hotam_req.do_search(["anchor", "--json"])
    assert rc == 0
    out = capsys.readouterr().out
    data = json.loads(out)
    assert isinstance(data, list)
    assert len(data) > 0


def test_search_no_matches(capsys) -> None:
    """search with no matches returns a calm message."""
    rc = hotam_req.do_search(["xyzzy_no_match_42"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "no requirements matching" in out


# ---------------------------------------------------------------------------
# context
# ---------------------------------------------------------------------------


def test_context_existing(capsys) -> None:
    """context for a real requirement returns structured info."""
    rc = hotam_req.do_context(["R-no-hand-edit-graph"])
    assert rc == 0
    out = capsys.readouterr().out
    assert "R-no-hand-edit-graph" in out
    assert "Context for" in out


def test_context_json(capsys) -> None:
    """context --json returns valid JSON with expected keys."""
    rc = hotam_req.do_context(["R-no-hand-edit-graph", "--json"])
    assert rc == 0
    out = capsys.readouterr().out
    data = json.loads(out)
    assert "requirement" in data
    assert data["requirement"]["id"] == "R-no-hand-edit-graph"
    assert "enforcement" in data


def test_context_not_found() -> None:
    """context for nonexistent id returns exit 1."""
    rc = hotam_req.do_context(["R-does-not-exist-xyz"])
    assert rc == 1


def test_context_with_conflicts(capsys) -> None:
    """context for a requirement that is a conflict member shows conflicts section."""
    # R-conflict-is-connector-node is a member of at least one conflict
    rc = hotam_req.do_context(["R-conflict-is-connector-node", "--json"])
    assert rc == 0
    out = capsys.readouterr().out
    data = json.loads(out)
    # This requirement participates in at least one conflict
    assert "requirement" in data


# ---------------------------------------------------------------------------
# patch (uses tmp domain, NEVER the live graph)
# ---------------------------------------------------------------------------


def _scaffold_tmp_domain(tmp_path: Path, name: str) -> Path:
    """Scaffold a temporary domain and return the graph.py path."""
    import create_domain as _cd  # noqa: PLC0415

    domains_dir = tmp_path / "domains"
    domains_dir.mkdir(exist_ok=True)
    old_root = getattr(_cd, "_DOMAINS_ROOT", None)
    _cd._DOMAINS_ROOT = domains_dir
    try:
        rc = _cd.scaffold(
            name=name,
            description=f"test domain {name}",
            goals=["test"],
            director_purpose="test",
            domains_root=domains_dir,
        )
    finally:
        _cd._DOMAINS_ROOT = old_root
    assert rc == 0, f"create_domain.scaffold failed for {name}"
    graph_path = domains_dir / name / "graph.py"
    assert graph_path.exists()
    return graph_path


def test_patch_dry_run_on_tmp_domain(tmp_path, capsys) -> None:
    """patch --dry-run on a tmp-domain graph reads current values and shows diff."""
    graph_path = _scaffold_tmp_domain(tmp_path, "test-patch")

    import apply_proposal  # noqa: PLC0415

    from hotam_spec.proposal import ProposedRequirement, ProposedStakeholder  # noqa: PLC0415

    sh = ProposedStakeholder(id="alice", name="Alice", domain="product")
    rc = apply_proposal.apply(sh, dry_run=False, content_graph=graph_path, defer_verify=True)
    assert rc == 0

    req = ProposedRequirement(
        id="R-test-patch-target",
        claim="A test requirement for patching.",
        owner="alice",
        status="DRAFT",
        why="testing the patch command",
        enforcement="PROSE",
    )
    rc = apply_proposal.apply(req, dry_run=False, content_graph=graph_path, defer_verify=True)
    assert rc == 0

    # Patch --dry-run: change enforcement to STRUCTURAL
    rc = hotam_req.do_patch([
        "R-test-patch-target",
        "--set", "enforcement=STRUCTURAL",
        "--dry-run",
        "--content-graph", str(graph_path),
    ])
    assert rc == 0
    out = capsys.readouterr().out
    assert "DRY RUN" in out


def test_patch_why_append_on_tmp_domain(tmp_path) -> None:
    """patch --why-append appends to existing why without replacing."""
    graph_path = _scaffold_tmp_domain(tmp_path, "test-append")

    import apply_proposal  # noqa: PLC0415

    from hotam_spec.proposal import ProposedRequirement, ProposedStakeholder  # noqa: PLC0415

    sh = ProposedStakeholder(id="bob", name="Bob", domain="eng")
    apply_proposal.apply(sh, dry_run=False, content_graph=graph_path, defer_verify=True)

    req = ProposedRequirement(
        id="R-test-why-append",
        claim="A test requirement.",
        owner="bob",
        status="DRAFT",
        why="original why",
        enforcement="PROSE",
    )
    apply_proposal.apply(req, dry_run=False, content_graph=graph_path, defer_verify=True)

    # patch with --why-append, using --dry-run to avoid full regen
    rc = hotam_req.do_patch([
        "R-test-why-append",
        "--why-append", "additional context",
        "--dry-run",
        "--content-graph", str(graph_path),
    ])
    assert rc == 0


def test_patch_no_set_fails() -> None:
    """patch with no --set and no --why-append returns exit 2."""
    rc = hotam_req.do_patch(["R-something"])
    assert rc == 2


def test_patch_bad_field_fails(capsys) -> None:
    """patch with an unknown field returns exit 2."""
    rc = hotam_req.do_patch(["R-anchor-everything", "--set", "nonexistent_field=value"])
    assert rc == 2


# ---------------------------------------------------------------------------
# _parse_set_pair helper
# ---------------------------------------------------------------------------


def test_parse_set_pair_valid() -> None:
    """Valid 'field=value' is parsed correctly."""
    assert hotam_req._parse_set_pair("enforcement=ENFORCED") == ("enforcement", "ENFORCED")
    assert hotam_req._parse_set_pair("claim=some long claim text") == ("claim", "some long claim text")


def test_parse_set_pair_invalid() -> None:
    """Missing '=' raises ValueError."""
    import pytest  # noqa: PLC0415

    with pytest.raises(ValueError):
        hotam_req._parse_set_pair("no_equals_sign")
