"""Canon: §Axis — controlled vocabulary of tension dimensions.

An Axis names the DIMENSION along which two requirements diverge inside a
Conflict (latency vs completeness; cost vs flexibility; privacy vs analytics).
The axis is not a property of either requirement — it is born only from their
meeting and lives on the connector node (see §Conflict).

WHY a controlled vocabulary: conflicts CLUSTER by axis — many C-nodes on one
axis are one unresolved ARCHITECTURAL choice, not ten local disputes. Clustering
only works if the axis is a normalized, shared slug rather than ad-hoc prose;
"latency-vs-completeness" and "speed_vs_full_check" must not split one cluster
into two. So Conflict.axis MUST be a slug present in the graph's `axes`
vocabulary (invariants.check_axis_in_registry).

WHY the vocabulary lives on TensionGraph.axes (not as a module constant): Hotam-Spec
is a CONTENT-FREE framework. The framework ships zero axes; each domain owns its
own vocabulary, declared on its graph in `spec/content/graph.py`. The framework
provides the Axis dataclass shape and the invariant; the slugs are the domain's.

OPEN (methodology) — DEFERRED AI-gatekeeper: a new axis should be admitted only
if no near-duplicate already exists. For now admission is manual editing of a
domain's `axes` tuple; the duplicate-detection gatekeeper is deferred (see
CLAUDE.md OPEN decisions and ROADMAP).
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Axis:
    """Canon: §Axis — one controlled-vocabulary entry: a normalized tension slug.

    RULE: Conflict.axis MUST equal the `slug` of some Axis in the graph's `axes`.
    Adding a new tension dimension in a domain = adding a row to that domain's
    `axes` tuple (so clustering stays sound).

    Fields:
      slug        — normalized identifier carried by Conflict.axis.
      description — what the two poles of this tension are.

    WHY frozen: an axis's meaning must not silently change under the conflicts
    that reference it; if the meaning shifts, that is a new axis (new slug).
    """

    slug: str
    description: str
