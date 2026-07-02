"""Tests for ENTITIES.md generation and check_entities_md_lists_all_types.

Six tests covering:
  1. ENTITIES.md is emitted for the active domain (hotam-spec-self) with opt-in placeholder.
  2. ENTITIES.md regenerates byte-identical (deterministic).
  3. build_entities_md renders correctly for a synthetic graph with one EntityType.
  4. check_entities_md_lists_all_types returns [] for a domain with no entity_types.
  5. check_entities_md_lists_all_types fires a Violation when a slug is missing from the file.
  6. CONSTITUTION block includes R-entity-<slug> when entity_types are present.
"""

from __future__ import annotations

import sys
from pathlib import Path

# Add tools and tests to path.
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
_TESTS = Path(__file__).resolve().parent
for _p in (_TOOLS, _TESTS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import gen_spec  # noqa: E402

# ---------------------------------------------------------------------------
# 1. ENTITIES.md exists for active domain (hotam-spec-self) with placeholder
# ---------------------------------------------------------------------------


def test_entities_md_emitted_for_active_domain() -> None:
    """domains/hotam-spec-self/docs/gen/ENTITIES.md exists with the empty-state placeholder."""
    path = gen_spec.ENTITIES_MD
    assert path.exists(), (
        f"ENTITIES.md not found at {path}: run `uv run python tools/gen_spec.py`."
    )
    text = path.read_text(encoding="utf-8")
    assert "§Entity aspect is opt-in" in text, (
        "ENTITIES.md for hotam-spec-self must contain the opt-in placeholder "
        "(the domain has no entity_types)"
    )


# ---------------------------------------------------------------------------
# 2. ENTITIES.md regenerates byte-identical
# ---------------------------------------------------------------------------


def test_entities_md_regenerates_byte_identical() -> None:
    """build_entities_md called twice on the same graph produces identical bytes."""
    g = gen_spec.load_content_graph()
    active = gen_spec._active_domain()
    domain_name = active.name if active is not None else ""
    first = gen_spec.build_entities_md(g, domain_name)
    second = gen_spec.build_entities_md(g, domain_name)
    assert first == second, "build_entities_md is not deterministic"


# ---------------------------------------------------------------------------
# 3. Synthetic graph with one EntityType — rendered correctly
# ---------------------------------------------------------------------------


def _make_synthetic_graph_with_entity():
    """Return a TensionGraph containing one EntityType('test-widget', …)."""
    # Import after path is set up.
    from hotam_spec.axis import Axis  # noqa: PLC0415
    from hotam_spec.entity import EntityField, EntityType  # noqa: PLC0415
    from hotam_spec.graph import TensionGraph  # noqa: PLC0415
    from hotam_spec.lifecycle import Lifecycle, State, Transition  # noqa: PLC0415
    from hotam_spec.stakeholder import Stakeholder  # noqa: PLC0415

    lc = Lifecycle(
        slug="widget-lc",
        states=(
            State("PENDING", "initial"),
            State("ACTIVE", "normal"),
            State("RETIRED", "terminal"),
        ),
        transitions=(
            Transition("PENDING", "ACTIVE", "activate"),
            Transition("ACTIVE", "RETIRED", "retire"),
        ),
    )
    et = EntityType(
        slug="test-widget",
        description="A synthetic widget for testing.",
        lifecycle=lc,
        fields=(
            EntityField("name", "string", required=True),
            EntityField("ref", "reference", required=False, ref_target="stakeholder"),
        ),
        why="test fixture",
    )
    return TensionGraph(
        axes=(Axis(slug="speed-vs-quality", description="speed or quality"),),
        stakeholders=(Stakeholder(id="product", name="Product", domain="test"),),
        entity_types=(et,),
    )


def test_entities_md_lists_types_when_present() -> None:
    """build_entities_md renders ## test-widget, Mermaid block, and fields table."""
    g = _make_synthetic_graph_with_entity()
    text = gen_spec.build_entities_md(g, "test-domain")

    assert "## test-widget" in text, "Missing ## test-widget section header"
    assert "```mermaid" in text, "Missing Mermaid block"
    assert "stateDiagram-v2" in text, "Missing stateDiagram-v2 directive"
    assert "[*] --> PENDING" in text, "Missing initial state arrow"
    assert "PENDING --> ACTIVE : activate" in text, "Missing activate transition"
    assert "ACTIVE --> RETIRED : retire" in text, "Missing retire transition"
    assert "| name |" in text or "name" in text, "Missing fields table"
    assert "| ref |" in text or "ref" in text, "Missing ref field"
    assert "### Covered by" in text, "Missing covered-by section"
    assert "check_entity_type_lifecycle_wellformed" in text, "Missing invariant name"


# ---------------------------------------------------------------------------
# 4. check_entities_md_lists_all_types — empty domain → []
# ---------------------------------------------------------------------------


def test_check_entities_md_lists_all_types_no_types() -> None:
    """A domain with no entity_types → check_entities_md_lists_all_types returns []."""
    from hotam_spec.graph import TensionGraph  # noqa: PLC0415
    from hotam_spec.invariants import check_entities_md_lists_all_types  # noqa: PLC0415

    g = TensionGraph()  # no entity_types
    # The check walks domains/ on disk; hotam-spec-self has no entity_types, so [] expected.
    violations = check_entities_md_lists_all_types(g)
    assert violations == [], (
        f"Expected no violations for empty domain, got: {violations}"
    )


# ---------------------------------------------------------------------------
# 5. check_entities_md_lists_all_types — missing slug fires Violation
# ---------------------------------------------------------------------------


def test_check_entities_md_lists_all_types_missing_slug(tmp_path: Path) -> None:
    """When a domain's graph.py declares an entity type but ENTITIES.md lacks the section,
    check_entities_md_lists_all_types fires one Violation."""
    from hotam_spec.invariants import (  # noqa: PLC0415
        check_entities_md_lists_all_types,
    )
    from hotam_spec.graph import TensionGraph  # noqa: PLC0415

    # Create a synthetic domain directory under tmp_path that mimics domains/<name>/.
    domain_dir = tmp_path / "domains" / "test-domain"
    gen_dir = domain_dir / "docs" / "gen"
    gen_dir.mkdir(parents=True)

    # Write a minimal graph.py with entity_types.
    graph_src = """\
from hotam_spec.axis import Axis
from hotam_spec.entity import EntityType
from hotam_spec.graph import TensionGraph
from hotam_spec.lifecycle import Lifecycle, State, Transition
from hotam_spec.stakeholder import Stakeholder

def build_graph():
    lc = Lifecycle(
        slug='foo-lc',
        states=(State('ACTIVE', 'initial'), State('CLOSED', 'terminal')),
        transitions=(Transition('ACTIVE', 'CLOSED', 'close'),),
    )
    et = EntityType(slug='foo', description='Foo entity.', lifecycle=lc)
    return TensionGraph(
        axes=(Axis(slug='s-vs-q', description='speed vs quality'),),
        stakeholders=(Stakeholder(id='owner', name='Owner', domain='test'),),
        entity_types=(et,),
    )
"""
    (domain_dir / "graph.py").write_text(graph_src, encoding="utf-8")

    # Write ENTITIES.md WITHOUT a ## foo section.
    (gen_dir / "ENTITIES.md").write_text(
        "# Entities\n\n_(nothing here)_\n", encoding="utf-8"
    )

    # Monkey-patch _DOMAINS_ROOT_FOR_ENTITY_CHECK to point at tmp_path/domains.
    import hotam_spec.invariants as _inv  # noqa: PLC0415

    orig = _inv._DOMAINS_ROOT_FOR_ENTITY_CHECK
    try:
        _inv._DOMAINS_ROOT_FOR_ENTITY_CHECK = tmp_path / "domains"
        violations = check_entities_md_lists_all_types(TensionGraph())
    finally:
        _inv._DOMAINS_ROOT_FOR_ENTITY_CHECK = orig

    assert len(violations) == 1, f"Expected 1 violation, got: {violations}"
    assert violations[0].target == "foo"
    assert "## foo" in violations[0].message


# ---------------------------------------------------------------------------
# 6. CONSTITUTION block includes R-entity-<slug> when entity_types present
# ---------------------------------------------------------------------------


def test_entity_constitution_section_appears_when_types_present() -> None:
    """build_framework_invariants appends R-entity-<slug> entries when entity_types exist.

    Phase 3 (task #9): entity-derived requirements are framework-plumbing —
    they relocated from the root CONSTITUTION block to
    docs/gen/FRAMEWORK-INVARIANTS.md (build_framework_invariants). The root
    CONSTITUTION block no longer carries this section.
    """
    g = _make_synthetic_graph_with_entity()
    invariants = gen_spec.build_framework_invariants(g)
    assert "R-entity-test-widget" in invariants, (
        "FRAMEWORK-INVARIANTS.md must include R-entity-test-widget when EntityType 'test-widget' exists"
    )
    assert "§Entity" in invariants, (
        "FRAMEWORK-INVARIANTS.md must reference §Entity for entity-derived R"
    )
    block = gen_spec._render_constitution_block(g)
    assert "R-entity-test-widget" not in block, (
        "root CONSTITUTION block must NOT include entity-derived entries — "
        "they are framework-plumbing (relocated to FRAMEWORK-INVARIANTS.md)"
    )


def test_entity_constitution_section_absent_when_no_types() -> None:
    """build_framework_invariants omits Entity-derived section when entity_types is empty."""
    from hotam_spec.graph import TensionGraph  # noqa: PLC0415

    g = TensionGraph()
    invariants = gen_spec.build_framework_invariants(g)
    assert "Entity-derived requirements" not in invariants, (
        "FRAMEWORK-INVARIANTS.md must not include entity-derived section for empty entity_types"
    )
    block = gen_spec._render_constitution_block(g)
    assert "Entity-derived requirements" not in block, (
        "root CONSTITUTION block must never include entity-derived section"
    )
