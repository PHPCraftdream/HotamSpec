"""Tests for spec/tools/create_entity_type.py — EntityType scaffolder.

Uses tmp_path to isolate all graph edits from the real domain.
Tests call create_entity_type.scaffold() directly (no subprocess for unit tests)
or invoke the CLI as a subprocess for argument-parsing tests.
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

_SPEC_ROOT = Path(__file__).resolve().parents[1]
_TOOLS = _SPEC_ROOT / "tools"
_SRC = _SPEC_ROOT / "src"

if str(_TOOLS) not in sys.path:
    sys.path.insert(0, str(_TOOLS))
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

import apply_proposal  # noqa: E402
import create_entity_type  # noqa: E402


# ---------------------------------------------------------------------------
# Minimal graph.py template for tests
# ---------------------------------------------------------------------------

_MINIMAL_GRAPH = """\
from __future__ import annotations

from hotam_spec.graph import TensionGraph
from hotam_spec.stakeholder import Stakeholder


def build_graph() -> TensionGraph:
    stakeholders = (
        Stakeholder(id="owner", name="Owner", domain="test"),
    )
    return TensionGraph(
        stakeholders=stakeholders,
    )
"""

_GRAPH_WITH_EXISTING_ENTITY = """\
from __future__ import annotations

from hotam_spec.entity import EntityField, EntityType
from hotam_spec.graph import TensionGraph
from hotam_spec.lifecycle import Lifecycle, State, Transition
from hotam_spec.stakeholder import Stakeholder


def build_graph() -> TensionGraph:
    stakeholders = (
        Stakeholder(id="owner", name="Owner", domain="test"),
    )
    return TensionGraph(
        stakeholders=stakeholders,
        entity_types=(
        EntityType(
            slug="customer",
            description="A customer entity.",
            lifecycle=Lifecycle(
                slug="customer-lifecycle",
                states=(
                    State("ACTIVE", kind="initial"),
                    State("CLOSED", kind="quiescent"),
                ),
                transitions=(
                    Transition("ACTIVE", "CLOSED", event="close"),
                ),
            ),
        ),
        ),
    )
"""


# ---------------------------------------------------------------------------
# Helper: run apply_proposal.apply() against a tmp graph.py
# ---------------------------------------------------------------------------


def _apply_entity_type(tmp_graph: Path, proposal_dict: dict) -> int:
    """Build a ProposedEntityType from raw dict and apply to tmp_graph."""
    proposal = apply_proposal._validate_entity_type(proposal_dict)
    source_text = tmp_graph.read_text(encoding="utf-8")
    try:
        new_source = apply_proposal._apply_entity_type_to_source(
            source_text, proposal, tmp_graph
        )
    except RuntimeError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    tmp_graph.write_text(new_source, encoding="utf-8")
    return 0


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_success(tmp_path: Path) -> None:
    """A complete valid spec inserts an EntityType into the graph."""
    graph_py = tmp_path / "graph.py"
    graph_py.write_text(_MINIMAL_GRAPH, encoding="utf-8")

    proposal_dict = {
        "slug": "order",
        "description": "A customer order.",
        "why": "Orders are the primary transaction type.",
        "states": [
            ["PENDING", "initial", ""],
            ["FULFILLED", "terminal", ""],
            ["CANCELLED", "terminal", ""],
        ],
        "transitions": [
            ["PENDING", "FULFILLED", "fulfill"],
            ["PENDING", "CANCELLED", "cancel"],
        ],
        "cyclic": False,
        "fields": [
            ["amount", "number", True, ""],
            ["status", "state", False, ""],
        ],
    }

    rc = _apply_entity_type(graph_py, proposal_dict)
    assert rc == 0

    result_text = graph_py.read_text(encoding="utf-8")
    assert 'slug="order"' in result_text
    assert "EntityType(" in result_text
    assert "PENDING" in result_text
    assert "FULFILLED" in result_text
    assert "from hotam_spec.entity import" in result_text
    assert "from hotam_spec.lifecycle import" in result_text


def test_refuses_duplicate_slug(tmp_path: Path) -> None:
    """Refuses to insert an EntityType whose slug already exists."""
    graph_py = tmp_path / "graph.py"
    graph_py.write_text(_GRAPH_WITH_EXISTING_ENTITY, encoding="utf-8")

    proposal_dict = {
        "slug": "customer",
        "description": "Duplicate customer.",
        "why": "Test duplicate.",
        "states": [["ACTIVE", "initial", ""]],
        "transitions": [],
        "cyclic": False,
        "fields": [],
    }

    rc = _apply_entity_type(graph_py, proposal_dict)
    assert rc == 1


def test_refuses_invalid_slug() -> None:
    """Slugs that are not kebab-case exit 1 from scaffold()."""
    valid_states = "ACTIVE:initial,CLOSED:quiescent"
    valid_transitions = "close:ACTIVE->CLOSED"

    bad_slugs = ["Customer", "with space", "with/slash"]
    for bad in bad_slugs:
        rc = create_entity_type.scaffold(
            slug=bad,
            description="A test entity.",
            states_str=valid_states,
            transitions_str=valid_transitions,
        )
        assert rc == 1, f"Expected rc=1 for slug={bad!r}, got {rc}"


def test_refuses_missing_args() -> None:
    """Missing --description / --states / --transitions each exit 1."""
    # Missing --description
    result = subprocess.run(
        [
            sys.executable,
            str(_TOOLS / "create_entity_type.py"),
            "test-entity",
            "--states",
            "ACTIVE:initial,CLOSED:quiescent",
            "--transitions",
            "close:ACTIVE->CLOSED",
        ],
        capture_output=True,
        text=True,
    )
    assert result.returncode != 0
    assert "--description" in result.stderr

    # Missing --states
    result = subprocess.run(
        [
            sys.executable,
            str(_TOOLS / "create_entity_type.py"),
            "test-entity",
            "--description",
            "A test entity.",
            "--transitions",
            "close:ACTIVE->CLOSED",
        ],
        capture_output=True,
        text=True,
    )
    assert result.returncode != 0
    assert "--states" in result.stderr

    # Missing --transitions
    result = subprocess.run(
        [
            sys.executable,
            str(_TOOLS / "create_entity_type.py"),
            "test-entity",
            "--description",
            "A test entity.",
            "--states",
            "ACTIVE:initial,CLOSED:quiescent",
        ],
        capture_output=True,
        text=True,
    )
    assert result.returncode != 0
    assert "--transitions" in result.stderr


def test_validates_state_machine() -> None:
    """--states without an initial, and --transitions with unknown endpoints exit 1."""
    # No initial state
    rc = create_entity_type.scaffold(
        slug="no-initial",
        description="Missing initial state.",
        states_str="ACTIVE:normal,CLOSED:quiescent",
        transitions_str="close:ACTIVE->CLOSED",
    )
    assert rc == 1

    # Transition references unknown state
    rc = create_entity_type.scaffold(
        slug="bad-transition",
        description="Bad transition endpoint.",
        states_str="ACTIVE:initial,CLOSED:quiescent",
        transitions_str="close:ACTIVE->UNKNOWN",
    )
    assert rc == 1


def test_r_tool_create_entity_type_in_constitution() -> None:
    """After running gen_spec, the domain CLAUDE.md mentions R-tool-create-entity-type."""
    # Run gen_spec to ensure the CLAUDE.md is up to date.
    result = subprocess.run(
        [sys.executable, str(_TOOLS / "gen_spec.py")],
        capture_output=True,
        text=True,
        cwd=str(_SPEC_ROOT),
    )
    assert result.returncode == 0, f"gen_spec.py failed: {result.stderr}"

    # Check the domain CLAUDE.md (where tool-derived requirements are projected).
    domains_root = _SPEC_ROOT.parent / "domains"
    claude_md_paths = list(domains_root.glob("*/CLAUDE.md"))
    assert claude_md_paths, "No domain CLAUDE.md found under domains/."

    found = False
    for claude_md in claude_md_paths:
        text = claude_md.read_text(encoding="utf-8")
        if "R-tool-create-entity-type" in text:
            found = True
            break

    assert found, (
        "R-tool-create-entity-type not found in any domains/*/CLAUDE.md. "
        "Check that create_entity_type.py's first docstring line matches "
        "'Canon: §Entity — <claim>'."
    )
