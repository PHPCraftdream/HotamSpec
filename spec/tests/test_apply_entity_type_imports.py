"""Regression: _apply_entity_type_to_source injects entity/lifecycle imports.

Porción-1 bug: when a domain receives its FIRST EntityType, the writer must
inject `from hotam_spec.entity import EntityField, EntityType` and
`from hotam_spec.lifecycle import Lifecycle, State, Transition` — otherwise
build_graph() raises NameError on the new node's constructor. The old code
duplicated fragile substring-based import checks per module; the fix routes
both through the shared whole-name helper `_ensure_import`.

Oracle: the produced source must (1) compile, and (2) actually load via
build_graph() with no NameError — i.e. every symbol the rendered EntityType
references resolves.
"""

from __future__ import annotations

import sys
import textwrap
from pathlib import Path

_SRC = Path(__file__).resolve().parents[1] / "src"
_TOOLS = Path(__file__).resolve().parents[1] / "tools"
for _p in (_SRC, _TOOLS):
    if str(_p) not in sys.path:
        sys.path.insert(0, str(_p))

import apply_proposal  # noqa: E402


_BARE_GRAPH = textwrap.dedent(
    '''\
    from __future__ import annotations

    from hotam_spec.graph import TensionGraph
    from hotam_spec.requirement import Requirement
    from hotam_spec.stakeholder import Stakeholder


    def build_graph() -> TensionGraph:
        requirements = ()
        conflicts = ()
        return TensionGraph(
            requirements=requirements,
            conflicts=conflicts,
        )
    '''
)


def _widget_proposal():
    return apply_proposal._validate_entity_type(
        {
            "kind": "EntityType",
            "slug": "widget",
            "description": "a widget",
            "states": [["new", "initial"], ["done", "terminal"]],
            "transitions": [["new", "done", "finish"]],
            "fields": [["owner", "reference", True, "Stakeholder"]],
        }
    )


def _load_build_graph(source: str):
    """Compile `source` and execute build_graph(), returning the graph.

    Raises NameError (the exact porción-1 failure mode) if any symbol used by
    the rendered EntityType was not imported.
    """
    ns: dict = {}
    code = compile(source, "<synthetic-domain>", "exec")
    exec(code, ns)  # noqa: S102 — controlled test source
    return ns["build_graph"]()


def test_first_entity_type_injects_all_imports_and_loads():
    """A domain with NO entity/lifecycle imports loads after the first EntityType."""
    proposal = _widget_proposal()
    result = apply_proposal._apply_entity_type_to_source(
        _BARE_GRAPH, proposal, Path("synthetic/graph.py")
    )

    assert "from hotam_spec.entity import" in result
    assert "EntityType" in result and "EntityField" in result
    assert "from hotam_spec.lifecycle import" in result
    for sym in ("Lifecycle", "State", "Transition"):
        assert sym in result.split("def build_graph")[0]  # present in imports

    graph = _load_build_graph(result)
    assert len(graph.entity_types) == 1
    assert graph.entity_types[0].slug == "widget"


def test_partial_lifecycle_import_is_completed():
    """A domain importing only Lifecycle gains State+Transition (whole-name)."""
    partial = _BARE_GRAPH.replace(
        "from hotam_spec.graph import TensionGraph",
        "from hotam_spec.lifecycle import Lifecycle\n"
        "from hotam_spec.graph import TensionGraph",
    )
    result = apply_proposal._apply_entity_type_to_source(
        partial, _widget_proposal(), Path("synthetic/graph.py")
    )
    # State and Transition must be added; Lifecycle not duplicated.
    imports = result.split("def build_graph")[0]
    assert imports.count("Lifecycle") == 1  # not re-imported
    assert "State" in imports and "Transition" in imports
    graph = _load_build_graph(result)
    assert graph.entity_types[0].slug == "widget"


def test_ensure_import_whole_name_no_substring_false_positive():
    """_ensure_import treats names as whole tokens, not substrings."""
    src = (
        "from __future__ import annotations\n\n"
        "from hotam_spec.lifecycle import StateMachine\n"
    )
    out = apply_proposal._ensure_import(
        src, "hotam_spec.lifecycle", ["State", "Transition"]
    )
    # 'State' must be added even though 'StateMachine' contains it.
    names = out.split("import ")[-1].strip()
    tokens = {n.strip() for n in names.split(",")}
    assert "State" in tokens
    assert "Transition" in tokens
    assert "StateMachine" in tokens
