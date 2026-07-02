"""Canon: §Graph — the tension graph store and its traversal helpers.

The store IS the Python code (like dev-coin's params.py): a frozen TensionGraph
holding tuples of Axes, Stakeholders, Assumptions, Requirements and Conflicts;
edges are tuple-of-id fields on those objects; traversal is the plain functions
below. No database, no RDF — the graph instance the invariants, the generator
and the harness all read is the one assembled by the loader.

CONTENT-FREE FRAMEWORK: this module ships ZERO business data. Hotam-Spec is a blank
kit. Real domains populate `spec/content/graph.py` exposing `build_graph() ->
TensionGraph`; an empty `spec/content/` is the legitimate ship state ("no
content yet"). The example demo lives outside the framework in
`spec/tests/fixtures/seed.py` and is loaded only via the explicit `--demo` flag
of the tools or by the tests.

WHY traversal lives here as functions (not methods on a graph class doing logic):
keeps the ontology dataclasses pure data and the queries in one auditable place,
mirroring dev-coin where chain logic is module functions over frozen dataclasses.
"""

from __future__ import annotations

import importlib.util
import sys
from dataclasses import dataclass, field
from pathlib import Path

from hotam_spec.assumption import DEAD, Assumption
from hotam_spec.axis import Axis
from hotam_spec.conflict import Conflict
from hotam_spec.entity import EntityInstance, EntityType
from hotam_spec.operator import Operator
from hotam_spec.process import Goal, Process
from hotam_spec.requirement import Requirement
from hotam_spec.stakeholder import Stakeholder


@dataclass(frozen=True)
class TensionGraph:
    """Canon: §Graph — the whole requirement world as one frozen object.

    RULE: this is the single in-memory source of truth consumed by invariants,
    the generator and the harness. All collections are tuples (ordered,
    hashable, frozen); lookups go through the index helpers below.

    Fields:
      axes         — tuple of Axis (the controlled vocabulary for Conflict.axis).
      stakeholders — tuple of Stakeholder.
      assumptions  — tuple of Assumption.
      requirements — tuple of Requirement.
      conflicts    — tuple of Conflict.
      operators    — tuple of Operator (§Operator — the acting facets; M20).
      processes    — tuple of Process (§Process — opt-in behavioral aspect, M12).
      goals        — tuple of Goal (§Goal — first-class target-state type, M19).
      entity_types — tuple of EntityType (§Entity — domain-declared business concepts; M12).
                     WHY: allows a domain to declare lifecycle-bearing concepts (customer,
                     order, invoice) without framework code per entity — coverage
                     iterates g.entity_types.
      entities     — tuple of EntityInstance (§Entity — concrete in-graph instances).
                     WHY: a declared EntityType without instances is the legitimate
                     schema-only state; EntityInstance carries the per-instance
                     id/state/field_values for traversal and invariant checking.

    WHY frozen + tuples: determinism. The generator emits docs in graph order and
    the meta-test demands byte-for-byte stability; a mutable/unordered store would
    make regeneration non-deterministic and reintroduce drift.

    WHY axes live on the graph (not as a module constant): a domain owns its
    controlled vocabulary; the framework ships with none. Two domains may admit
    different axes, and the invariant `check_axis_in_registry` reads from this
    field so the per-domain vocabulary is the authority.

    WHY processes and goals default to empty tuple: they are opt-in aspects (M12).
    A domain that does not model processes pays nothing — is_empty() still works
    correctly with the extended set.
    """

    axes: tuple[Axis, ...] = field(default_factory=tuple)
    stakeholders: tuple[Stakeholder, ...] = field(default_factory=tuple)
    assumptions: tuple[Assumption, ...] = field(default_factory=tuple)
    requirements: tuple[Requirement, ...] = field(default_factory=tuple)
    conflicts: tuple[Conflict, ...] = field(default_factory=tuple)
    operators: tuple[Operator, ...] = field(default_factory=tuple)
    processes: tuple[Process, ...] = field(default_factory=tuple)
    goals: tuple[Goal, ...] = field(default_factory=tuple)
    entity_types: tuple[EntityType, ...] = field(default_factory=tuple)
    entities: tuple[EntityInstance, ...] = field(default_factory=tuple)

    def is_empty(self) -> bool:
        """Canon: §Graph — True iff no domain content has been loaded.

        RULE: empty iff every collection is empty. An empty graph is the
        legitimate ship state of the framework (no content under spec/content/).
        Includes the §Process, §Goal, and §Entity aspect collections.

        WHY: the harness and generator use this to render a calm "no content
        yet" message instead of an awkwardly empty roster.
        """
        return not (
            self.axes
            or self.stakeholders
            or self.assumptions
            or self.requirements
            or self.conflicts
            or self.operators
            or self.processes
            or self.goals
            or self.entity_types
            or self.entities
        )


# ---------------------------------------------------------------------------
# Content loader — discovers domains/<name>/graph.py or spec/content/graph.py
# ---------------------------------------------------------------------------

#: Path to the user-content slot (legacy; used when no domains/ are present).
# graph.py lives at spec/src/hotam_spec/graph.py; parents[2] is `spec/`.
CONTENT_DIR = Path(__file__).resolve().parents[2] / "content"
CONTENT_GRAPH_FILE = CONTENT_DIR / "graph.py"
CONTENT_BUILDER_NAME = "build_graph"

#: Repo root (spec/src/hotam_spec/graph.py -> parents[3] = repo root)
_REPO_ROOT = Path(__file__).resolve().parents[3]
_DOMAINS_ROOT = _REPO_ROOT / "domains"

#: Pin file naming the default active domain when HOTAM_SPEC_ACTIVE_DOMAIN is
#: unset. Lives at domains/.active-domain — a COMMITTED, version-controlled
#: file (unlike spec/.runtime/, which is gitignored ephemera per
#: R-task-spawn-log-runtime) so the default is a deliberate, auditable
#: decision, not a local-only override. Mirrors gen_spec.py's
#: _ACTIVE_DOMAIN_PIN_FILE and apply_proposal.py's resolver exactly (same
#: repo path) so every tool that discovers the active domain agrees on the
#: same deterministic default (R-active-domain-pin-not-alphabetical).
_ACTIVE_DOMAIN_PIN_FILE = _REPO_ROOT / "domains" / ".active-domain"


def _active_domain_graph_file() -> Path | None:
    """Canon: §Graph — return path to the active domain's graph.py, or None.

    RULE: resolution order is (1) HOTAM_SPEC_ACTIVE_DOMAIN env var, (2)
    spec/.runtime/active-domain pin file, (3) the first
    domains/<name>/graph.py alphabetically. Returns None if domains/ is
    absent or empty (legitimate state: the framework has no domain yet).

    WHY env-var first, pin file second: the env var lets CI / test harnesses
    override the domain without mutating the filesystem
    (R-deterministic-generation); the pin file is the committed, deliberate
    default for everyday use — with >= 2 domains present, "first
    alphabetically" is an accident of naming, not a decision (see
    gen_spec.py::_select_active_domain_dir for the full WHY). Alphabetical
    stays as the last-resort fallback so a fresh repo with no pin file yet is
    never "lost" (R-agent-never-lost).
    """
    import os  # noqa: PLC0415

    env_domain = os.environ.get("HOTAM_SPEC_ACTIVE_DOMAIN", "").strip()
    if env_domain:
        candidate = _DOMAINS_ROOT / env_domain / "graph.py"
        if candidate.exists():
            return candidate
    if not _DOMAINS_ROOT.exists():
        return None
    domain_dirs = sorted(
        d for d in _DOMAINS_ROOT.iterdir() if d.is_dir() and not d.name.startswith("_")
    )
    if _ACTIVE_DOMAIN_PIN_FILE.exists():
        pinned = _ACTIVE_DOMAIN_PIN_FILE.read_text(encoding="utf-8").strip()
        if pinned:
            candidate = _DOMAINS_ROOT / pinned / "graph.py"
            if candidate.exists():
                return candidate
    for d in domain_dirs:
        gf = d / "graph.py"
        if gf.exists():
            return gf
    return None


def active_domain_doc_readers() -> dict[str, str]:
    """Canon: §Graph / §Domain — the active domain's declared DOC_READERS binding.

    RULE: resolves the active domain directory the same way
    `_active_domain_graph_file` does (env var, else first domains/<name>/
    alphabetically), then imports its `manifest.py` and returns its
    `DOC_READERS` attribute (a `dict[role_hint, Stakeholder.id]`) if present,
    else `{}`. Never fabricates a binding — a domain that has not declared
    `DOC_READERS` yet gets an empty mapping, which resolve_reader() (in
    `hotam_spec.doc_readers`) treats as "unresolved", not a guess.

    WHY here, not in doc_readers.py: `doc_readers.py` is framework code and
    must stay content-free (R-content-free-no-business-data) — it cannot
    import any specific domain's manifest. `graph.py` already owns "find the
    active domain" (`_active_domain_graph_file`); this function is the same
    discovery walk applied to `manifest.py` instead of `graph.py`, keeping
    the domain-discovery logic in ONE place.
    """
    graph_file = _active_domain_graph_file()
    if graph_file is None:
        return {}
    manifest_py = graph_file.parent / "manifest.py"
    if not manifest_py.exists():
        return {}
    spec = importlib.util.spec_from_file_location(
        f"_manifest_doc_readers_{graph_file.parent.name}", manifest_py
    )
    if spec is None or spec.loader is None:
        return {}
    mod = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(mod)  # type: ignore[union-attr]
    except Exception:
        return {}
    bindings = getattr(mod, "DOC_READERS", None)
    if not isinstance(bindings, dict):
        return {}
    return {str(k): str(v) for k, v in bindings.items()}


def _load_graph_file(graph_file: Path) -> TensionGraph:
    """Canon: §Graph — load build_graph() from a graph file path.

    RULE: import the file, call build_graph(), validate the return type.
    WHY factored out: both load_content_graph() and _active_domain_graph_file()
    use the same import/validate pattern; one implementation avoids drift.
    """
    spec = importlib.util.spec_from_file_location(
        "hotam_spec_user_content_graph", graph_file
    )
    if spec is None or spec.loader is None:  # pragma: no cover — defensive
        return TensionGraph()
    module = importlib.util.module_from_spec(spec)
    # Make sure user content can `from hotam_spec.* import …` cleanly.
    src_dir = str(Path(__file__).resolve().parents[1])
    if src_dir not in sys.path:
        sys.path.insert(0, src_dir)
    spec.loader.exec_module(module)
    builder = getattr(module, CONTENT_BUILDER_NAME, None)
    if builder is None:
        raise RuntimeError(
            f"{graph_file} does not expose `{CONTENT_BUILDER_NAME}()`; "
            f"see CLAUDE.md §How to populate."
        )
    g = builder()
    if not isinstance(g, TensionGraph):
        raise TypeError(
            f"{graph_file}:{CONTENT_BUILDER_NAME}() must return a "
            f"TensionGraph, got {type(g).__name__}"
        )
    return g


def load_content_graph() -> TensionGraph:
    """Canon: §Graph — load the user's graph from domains/<name>/ or spec/content/.

    RULE: discovery order:
      1. HOTAM_SPEC_ACTIVE_DOMAIN env var → domains/<name>/graph.py
      2. spec/.runtime/active-domain pin file → domains/<name>/graph.py
      3. First domains/<name>/graph.py alphabetically.
      4. Legacy fallback: spec/content/graph.py (backward-compat).
      5. Nothing found → return empty TensionGraph (legitimate framework state).

    Never raise just because nothing is populated yet — emptiness is a legitimate
    state (R-empty-content-wellformed).

    WHY file-discovery with env-var override: the framework's "agent is never
    lost" property requires a deterministic location every tool agrees on; the
    env var lets CI pin a domain without filesystem mutation
    (R-deterministic-generation, R-agent-never-lost).
    """
    domain_graph = _active_domain_graph_file()
    if domain_graph is not None:
        return _load_graph_file(domain_graph)
    # Legacy fallback: spec/content/graph.py
    if CONTENT_GRAPH_FILE.exists():
        return _load_graph_file(CONTENT_GRAPH_FILE)
    return TensionGraph()


# ---------------------------------------------------------------------------
# Indexing / lookup
# ---------------------------------------------------------------------------


def axis_slugs(g: TensionGraph) -> frozenset[str]:
    """Canon: §Axis — the set of admitted axis slugs on this graph.

    RULE: this set is the authority used by invariants.check_axis_in_registry —
    a Conflict.axis must be one of these. Per-domain vocabulary, no global state.

    WHY a function over `g.axes`: keeps the membership test in one place so the
    invariant, the generator and any consumer agree on what "a known axis" means.
    """
    return frozenset(a.slug for a in g.axes)


def stakeholder_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Graph — set of all Stakeholder ids (for dangling-ref checks)."""
    return frozenset(s.id for s in g.stakeholders)


def assumption_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Graph — set of all Assumption ids (for dangling-ref checks)."""
    return frozenset(a.id for a in g.assumptions)


def requirement_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Graph — set of all Requirement ids (for dangling-ref checks)."""
    return frozenset(r.id for r in g.requirements)


def operator_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Operator — set of all Operator ids (for dangling-ref checks).

    RULE: used by check_no_dangling_ids to verify Operator.parent references
    and by check_operator_steward_not_self to identify operator acting facets.
    """
    return frozenset(op.id for op in g.operators)


def process_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Process — set of all Process ids in the graph.

    RULE: used by check_typed_anchors to verify PR- prefix discipline and by
    tests to confirm a named process is present. Empty when the §Process aspect
    is not loaded (opt-in, M12).
    """
    return frozenset(p.id for p in g.processes)


def goal_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Goal — set of all Goal ids in the graph.

    RULE: used by check_typed_anchors to verify GOAL- prefix discipline and by
    check_goal_owner_is_operator to resolve Goal.owner references. Empty when
    the §Goal aspect is not loaded (opt-in, M12/M19).
    """
    return frozenset(go.id for go in g.goals)


def requirement_by_id(g: TensionGraph, rid: str) -> Requirement | None:
    """Canon: §Graph — Requirement with id `rid`, or None.

    WHY None-returning (not raising): callers in the harness tolerate dangling
    ids on purpose — a dangling member is itself a diagnosable issue surfaced as
    a next-action, not a crash.
    """
    for r in g.requirements:
        if r.id == rid:
            return r
    return None


def assumption_by_id(g: TensionGraph, aid: str) -> Assumption | None:
    """Canon: §Graph — Assumption with id `aid`, or None (see requirement_by_id)."""
    for a in g.assumptions:
        if a.id == aid:
            return a
    return None


# ---------------------------------------------------------------------------
# Drift traversal — assumptions and their dependents
# ---------------------------------------------------------------------------


def requirements_on_assumption(g: TensionGraph, aid: str) -> tuple[Requirement, ...]:
    """Canon: §Graph — Requirements that rest on assumption `aid`.

    RULE: a requirement depends on `aid` iff aid in requirement.assumptions.
    Used to walk invisibility #2 (hidden dependency) and #3 (context drift).

    WHY: when `aid` dies, these are exactly the requirements whose ground moved.
    """
    return tuple(r for r in g.requirements if aid in r.assumptions)


def conflicts_on_assumption(g: TensionGraph, aid: str) -> tuple[Conflict, ...]:
    """Canon: §Graph — Conflicts whose shared_assumption is `aid`.

    RULE: these are the cluster that INHERITS drift from `aid`. When `aid` dies,
    the whole cluster under it must revive at once — one trigger, one cluster.
    """
    return tuple(c for c in g.conflicts if c.shared_assumption == aid)


def dead_assumptions(g: TensionGraph) -> tuple[Assumption, ...]:
    """Canon: §Graph — assumptions currently flipped to DEAD.

    RULE: every DEAD assumption with live dependents is fallout the harness MUST
    surface; a DEAD assumption is never silently dropped.
    """
    return tuple(a for a in g.assumptions if a.status == DEAD)


# ---------------------------------------------------------------------------
# Conflict traversal — clustering and latent connectors
# ---------------------------------------------------------------------------


def conflicts_by_axis(g: TensionGraph) -> dict[str, tuple[Conflict, ...]]:
    """Canon: §Conflict — conflicts grouped by axis (the CLUSTERS).

    RULE: a cluster = all conflicts sharing one axis. A cluster of size > 1 is an
    unresolved ARCHITECTURAL choice, not N local disputes — rendered as such in
    TENSIONS.md. Keys are emitted in first-seen graph order for determinism.

    WHY a node-graph reveals this and an edge-list cannot: clustering needs the
    axis to be a shared, normalized property of a node; edges between pairs carry
    no common axis to group on.
    """
    out: dict[str, list[Conflict]] = {}
    for c in g.conflicts:
        out.setdefault(c.axis, []).append(c)
    return {axis: tuple(cs) for axis, cs in out.items()}


def members_pair_set(g: TensionGraph) -> frozenset[frozenset[str]]:
    """Canon: §Conflict — the set of unordered member pairs already mediated.

    RULE: for every conflict, every pair of its members is "already connected".
    The latent-connector heuristic skips pairs already in this set (they have a
    C-node), hunting only pairs that SHOULD have one but don't.

    WHY pairs (not whole member tuples): a latent suspect is a 2-requirement
    tension; a conflict with 3 members already mediates all 3 of its pairs.
    """
    pairs: set[frozenset[str]] = set()
    for c in g.conflicts:
        ms = list(c.members)
        for i in range(len(ms)):
            for j in range(i + 1, len(ms)):
                pairs.add(frozenset({ms[i], ms[j]}))
    return frozenset(pairs)


GENERIC_ASSUMPTION_THRESHOLD = 8
"""Canon: §Conflict — an assumption referenced by >= this many requirements is
GENERIC (framework-wide scaffolding, e.g. A-python-stack), not a specific shared
premise. Pairs sharing ONLY generic assumptions are noise, not suspects."""


def assumption_reference_counts(g: TensionGraph) -> dict[str, int]:
    """Canon: §Conflict — how many non-REJECTED requirements reference each assumption.

    RULE: count, per assumption id, the number of distinct non-REJECTED
    requirements naming it in Requirement.assumptions. Used to distinguish a
    SPECIFIC assumption (referenced by a few requirements — a real shared
    premise) from a GENERIC one (referenced by dozens — framework scaffolding
    that any two requirements would trivially "share").
    """
    from hotam_spec.requirement import REJECTED  # noqa: PLC0415

    counts: dict[str, int] = {}
    for r in g.requirements:
        if r.status == REJECTED:
            continue
        for a_id in r.assumptions:
            counts[a_id] = counts.get(a_id, 0) + 1
    return counts


@dataclass(frozen=True)
class LatentSuspect:
    """Canon: §Conflict — a requirement pair that SHOULD have a C-node but doesn't.

    RULE (HEURISTIC, not a proof): two SETTLED/OPEN requirements that share at
    least one SPECIFIC assumption (referenced by fewer than
    GENERIC_ASSUMPTION_THRESHOLD requirements) and are NOT already mediated by
    any conflict are flagged "for AI review". Pairs whose ONLY shared
    assumption(s) are GENERIC (framework-wide scaffolding referenced by many
    requirements) are not flagged — sharing a generic assumption is not a
    signal of hidden tension, it is noise. This is a stub for the deferred
    detector (spec-stack layer 4/5); it is explicitly a suspicion, never an
    auto-materialized conflict.

    Fields:
      left, right — the two Requirement ids (sorted for stable identity).
      hint        — why they were flagged (the shared specific signal).

    WHY only a heuristic now: the real detector ("find the missing connector")
    needs Hypothesis-generated tuples / Z3 joint-violation models; both DEFERRED.
    The hard boundary holds — a suspect is presented to a human, never resolved.
    """

    left: str
    right: str
    hint: str


def _latent_pair_records(
    g: TensionGraph,
) -> list[tuple[int, tuple[str, ...], str, str]]:
    """Shared pair scan behind latent_connector_suspects / latent_connector_clusters.

    Returns (min_ref_count, specific-assumption signature (sorted), left, right)
    per unmediated pair, sorted by (min_ref_count, left, right). One
    implementation keeps the suspect list and its clustering byte-consistent.
    """
    from hotam_spec.requirement import REJECTED  # noqa: PLC0415

    already = members_pair_set(g)
    ref_counts = assumption_reference_counts(g)
    reqs = [r for r in g.requirements if r.status != REJECTED]
    records: list[tuple[int, tuple[str, ...], str, str]] = []
    for i in range(len(reqs)):
        for j in range(i + 1, len(reqs)):
            a, b = reqs[i], reqs[j]
            shared = set(a.assumptions) & set(b.assumptions)
            if not shared:
                continue
            specific = {
                a_id
                for a_id in shared
                if ref_counts.get(a_id, 0) < GENERIC_ASSUMPTION_THRESHOLD
            }
            if not specific:
                continue
            if frozenset({a.id, b.id}) in already:
                continue
            left, right = sorted((a.id, b.id))
            min_count = min(ref_counts.get(a_id, 0) for a_id in specific)
            records.append((min_count, tuple(sorted(specific)), left, right))
    records.sort(key=lambda rec: (rec[0], rec[2], rec[3]))
    return records


def latent_connector_suspects(g: TensionGraph) -> tuple[LatentSuspect, ...]:
    """Canon: §Conflict — HEURISTIC hunt for missing connector nodes.

    RULE: flag any pair of non-REJECTED requirements that (a) share >= 1
    SPECIFIC assumption id (referenced by fewer than
    GENERIC_ASSUMPTION_THRESHOLD requirements — see assumption_reference_counts)
    and (b) are not already mediated by a conflict (members_pair_set). Pairs
    whose only shared assumptions are GENERIC (>= threshold references) are
    not flagged: sharing framework-wide scaffolding is not a hidden-tension
    signal, it is O(n^2) noise. Output is sorted by specificity (lowest shared
    reference count first, i.e. the rarest/most-suspicious pairs surface
    first), then by (left, right) for determinism.

    WHY share-a-specific-assumption is the cheap signal: two requirements
    interpreting one RARE assumption differently is the commonest hidden root
    of a real conflict (Conflict.shared_assumption). It is a SUSPICION the
    AI/steward must judge — deliberately stronger than "violated invariant"
    because it points at the not-yet-recorded. Real semantic detection is
    deferred (see CLAUDE.md / ROADMAP).
    """
    return tuple(
        LatentSuspect(
            left=left,
            right=right,
            hint="shares assumption(s): " + ", ".join(signature),
        )
        for _, signature, left, right in _latent_pair_records(g)
    )


@dataclass(frozen=True)
class LatentCluster:
    """Canon: §Conflict — latent suspects grouped by their shared-assumption signature.

    RULE: cluster key = the exact set of SPECIFIC assumptions a suspect pair
    shares (its signature). All pairs carrying one signature are ONE review
    item: N requirements standing on one specific assumption with no mediating
    conflict is one architectural question (usually: is that assumption really
    two assumptions?), not C(N,2) independent pair disputes.

    Fields:
      assumptions  — the signature, sorted for stable identity.
      requirements — sorted union of the member requirement ids of all pairs.
      pairs        — the pair-level detail (LatentSuspect records), preserved
                     for the verbose path (TENSIONS.md renders pairs).

    WHY clustering, not a threshold shift: raising
    GENERIC_ASSUMPTION_THRESHOLD only moves the noise cliff; grouping by
    signature keeps every suspect pair visible while the review surface
    matches the size of the actual decision space.
    """

    assumptions: tuple[str, ...]
    requirements: tuple[str, ...]
    pairs: tuple[LatentSuspect, ...]


def latent_connector_clusters(g: TensionGraph) -> tuple[LatentCluster, ...]:
    """Canon: §Conflict — group latent-connector suspects by shared-assumption signature.

    RULE: every suspect pair from latent_connector_suspects belongs to exactly
    ONE cluster, keyed by its specific-shared-assumption signature; the cluster
    carries the sorted union of its pairs' requirement ids plus the pair
    records themselves. Clusters are sorted by (min reference count across the
    cluster, signature) — rarest/most-suspicious first — and pairs inside a
    cluster keep the suspect ordering. Deterministic: same graph, same tuple.

    WHY: 21 pair lines over one shared assumption drown the one real question
    ('split this assumption?') and any genuinely distinct suspect next to
    them; the harness renders ONE P5 action per cluster (tools/what_now.py)
    while TENSIONS.md keeps the full pair table.
    """
    records = _latent_pair_records(g)
    grouped: dict[tuple[str, ...], list[tuple[int, str, str]]] = {}
    for min_count, signature, left, right in records:
        grouped.setdefault(signature, []).append((min_count, left, right))
    clusters: list[tuple[int, LatentCluster]] = []
    for signature, entries in grouped.items():
        member_ids: set[str] = set()
        pairs: list[LatentSuspect] = []
        for _, left, right in entries:
            member_ids.update((left, right))
            pairs.append(
                LatentSuspect(
                    left=left,
                    right=right,
                    hint="shares assumption(s): " + ", ".join(signature),
                )
            )
        cluster_min = min(mc for mc, _, _ in entries)
        clusters.append(
            (
                cluster_min,
                LatentCluster(
                    assumptions=signature,
                    requirements=tuple(sorted(member_ids)),
                    pairs=tuple(pairs),
                ),
            )
        )
    clusters.sort(key=lambda pair: (pair[0], pair[1].assumptions))
    return tuple(c for _, c in clusters)


def entity_type_slugs(g: TensionGraph) -> frozenset[str]:
    """Canon: §Entity — set of all declared EntityType slugs (for ref-resolution)."""
    return frozenset(et.slug for et in g.entity_types)


def entity_ids(g: TensionGraph) -> frozenset[str]:
    """Canon: §Entity — set of all EntityInstance ids (for dangling-ref checks)."""
    return frozenset(e.id for e in g.entities)


def entity_state_conflict_suspects(g: TensionGraph) -> tuple[LatentSuspect, ...]:
    """Canon: §Process / §Entity / §Conflict — HEURISTIC: two processes driving one entity into mutually-exclusive resting states.

    RULE (HEURISTIC, not a proof): for each EntityType, find Process pairs whose
    Steps invoke Lifecycle transitions leading to DIFFERENT terminal/quiescent
    states. Flag as LatentSuspect for AI review. The hard boundary holds: NEVER
    auto-materialize a Conflict — only surface as suspicion.

    WHY this is M16 made structural: two processes driving one entity along
    incompatible state paths is the canonical hidden contradiction Hotam-Spec was
    designed to surface. Until P21, Entity was deferred; now this detector turns
    the abstract description into a real next-action for the harness.
    """
    type_by_slug = {et.slug: et for et in g.entity_types}
    suspects: list[LatentSuspect] = []

    def process_destinations(p: object, slug: str) -> set[str]:
        """Set of resting destination states this process drives the named entity into."""
        et = type_by_slug.get(slug)
        if et is None:
            return set()
        transitions_by_event = {t.event: t for t in et.lifecycle.transitions}
        terminal_or_quiescent = {s.name for s in et.lifecycle.states if s.is_terminal()}
        dests: set[str] = set()
        for step in p.steps:  # type: ignore[union-attr]
            if not step.invokes or "." not in step.invokes:
                continue
            s, _, event = step.invokes.partition(".")
            if s != slug:
                continue
            t = transitions_by_event.get(event)
            if t and t.dst in terminal_or_quiescent:
                dests.add(t.dst)
        return dests

    for et in g.entity_types:
        slug = et.slug
        ps = [p for p in g.processes if slug in p.drives_entities]
        for i in range(len(ps)):
            for j in range(i + 1, len(ps)):
                a, b = ps[i], ps[j]
                da, db = process_destinations(a, slug), process_destinations(b, slug)
                if not da or not db:
                    continue
                if da.isdisjoint(db):
                    left, right = sorted((a.id, b.id))
                    hint = (
                        f"both drive entity '{slug}' but to disjoint resting states: "
                        f"{sorted(da)} vs {sorted(db)} — likely conflict on axis "
                        f"behavioral-{slug}-state"
                    )
                    suspects.append(LatentSuspect(left=left, right=right, hint=hint))
    suspects.sort(key=lambda s: (s.left, s.right))
    return tuple(suspects)
