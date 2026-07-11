#!/usr/bin/env python3
"""One-shot canonical JSON export of the hotam-spec-self tension graph.

Preparation step for porting the HotamSpec framework to Go: reads the active
domain's graph (domains/hotam-spec-self/graph.py via the framework's OWN loader)
into memory and serializes EVERY node collection into ONE canonical JSON
document.

READ-ONLY: imports the graph and writes a single NEW file. Nothing under
domains/ or src/hotam_spec/ is created, mutated, or overwritten.

Run (from anywhere; venv recommended):
    spec/.venv/Scripts/python.exe spec/scripts/export_graph_to_json.py

Loading uses the SAME mechanism as the existing tools (tools/gen_spec.py,
tools/what_now.py): the public hotam_spec.graph.load_content_graph(), whose
domain resolution (env-var -> pin file -> alphabetical) is pinned to
hotam-spec-self here via HOTAM_SPEC_ACTIVE_DOMAIN (the resolver's highest-
priority tier). No hand-rolled importlib.

Serialization rules (for stable, readable future git-diffs):
  - top-level keys   = TensionGraph collection names, in declaration order;
  - each collection  = list of nodes sorted by identity (id, else slug);
  - node fields      = dataclass DECLARATION order (not alphabetical);
  - nested dataclass = recursed; tuple/list preserve order; frozenset/set
    become a sorted list; enum -> .value; str/int/float/bool/None unchanged.
"""

from __future__ import annotations

import dataclasses
import enum
import json
import os
import sys
from pathlib import Path

# --- bootstrap: make hotam_spec importable (this script lives in spec/scripts/) ---
_SPEC_ROOT = Path(__file__).resolve().parents[1]
_SRC = _SPEC_ROOT / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

# Pin the domain explicitly. HOTAM_SPEC_ACTIVE_DOMAIN is the resolver's
# highest-priority tier, so this deterministically targets hotam-spec-self
# regardless of the committed pin file or working directory.
TARGET_DOMAIN = "hotam-spec-self"
os.environ.setdefault("HOTAM_SPEC_ACTIVE_DOMAIN", TARGET_DOMAIN)

from hotam_spec.domain_resolution import resolve_active_domain  # noqa: E402
from hotam_spec.graph import TensionGraph, load_content_graph  # noqa: E402
from hotam_spec.repo_paths import domains_root, spec_root  # noqa: E402

OUT_DIR = spec_root() / ".runtime" / "graph-export"
OUT_FILE = OUT_DIR / f"{TARGET_DOMAIN}.graph.json"

# The node collections on TensionGraph, in the order they are declared on the
# dataclass (axes, stakeholders, assumptions, requirements, conflicts,
# operators, processes, goals, entity_types, entities).
COLLECTIONS: tuple[str, ...] = (
    "axes",
    "stakeholders",
    "assumptions",
    "requirements",
    "conflicts",
    "operators",
    "processes",
    "goals",
    "entity_types",
    "entities",
)


def _to_jsonable(obj):
    """Recursively convert a graph value into JSON-primitive types.

    - dataclass instance -> dict in field-DECLARATION order (via
      dataclasses.fields), recursing on each value;
    - frozenset/set -> sorted list (unordered, so sort for determinism);
    - tuple/list -> list PRESERVING order (semantically ordered: steps,
      history, relations, members, transitions, states, fields);
    - Enum -> .value;
    - str/int/float/bool/None -> unchanged;
    - Path -> str;
    - anything else -> str() (defensive; never crashes the export).
    """
    if obj is None or isinstance(obj, (str, int, float, bool)):
        return obj
    if isinstance(obj, enum.Enum):
        return obj.value
    if dataclasses.is_dataclass(obj) and not isinstance(obj, type):
        return {
            f.name: _to_jsonable(getattr(obj, f.name)) for f in dataclasses.fields(obj)
        }
    if isinstance(obj, (frozenset, set)):
        return sorted(_to_jsonable(x) for x in obj)
    if isinstance(obj, (tuple, list)):
        return [_to_jsonable(x) for x in obj]
    if isinstance(obj, Path):
        return str(obj)
    return str(obj)


def _identity_key(node) -> str:
    """Sort key for a node: its `id`, else `slug` (Axis/EntityType use slug)."""
    return getattr(node, "id", None) or getattr(node, "slug", None) or ""


def export_graph(g: TensionGraph) -> dict:
    """Serialize every collection of g into a {collection: [node, ...]} dict.

    Collections are keyed in TensionGraph declaration order; each node list is
    sorted by identity string. Node fields keep their dataclass declaration
    order.
    """
    payload: dict[str, list] = {}
    for name in COLLECTIONS:
        nodes = getattr(g, name)
        payload[name] = [_to_jsonable(n) for n in sorted(nodes, key=_identity_key)]
    return payload


def main() -> int:
    resolved = resolve_active_domain(domains_root())
    if resolved != TARGET_DOMAIN:
        print(
            f"WARNING: resolved active domain is {resolved!r}, "
            f"expected {TARGET_DOMAIN!r}",
            file=sys.stderr,
        )

    g = load_content_graph()
    if not g.self_hosting:
        print(
            f"WARNING: loaded graph is not self-hosting (self_hosting=False); "
            f"expected the {TARGET_DOMAIN} domain.",
            file=sys.stderr,
        )

    payload = export_graph(g)
    counts = {name: len(payload[name]) for name in COLLECTIONS}
    total = sum(counts.values())

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    with OUT_FILE.open("w", encoding="utf-8", newline="\n") as fh:
        json.dump(payload, fh, indent=2, ensure_ascii=False)
        fh.write("\n")

    # Self-check: the written file must parse back as valid JSON.
    with OUT_FILE.open("r", encoding="utf-8") as fh:
        json.load(fh)

    source_graph_py = domains_root() / TARGET_DOMAIN / "graph.py"
    core = counts["requirements"] + counts["conflicts"] + counts["assumptions"]
    print(f"exported domain : {resolved} (self_hosting={g.self_hosting})")
    print(f"source graph.py : {source_graph_py}")
    print(f"output file     : {OUT_FILE}")
    print("node counts by collection:")
    for name in COLLECTIONS:
        print(f"  {name:13s}: {counts[name]}")
    print(f"  {'TOTAL':13s}: {total}")
    print(f"  (req+conflict+assumption = {core}; project docs cite ~299)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
