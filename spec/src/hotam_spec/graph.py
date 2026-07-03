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

from hotam_spec.assumption import DEAD, UNCERTAIN, Assumption
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
    self_hosting: bool = False
    """Canon: §Domain — True iff this graph models the Hotam-Spec framework
    ITSELF (the hotam-spec-self domain), False for any ordinary business
    domain (R-domain-self-hosting-flag).

    RULE: populated at load time from the sibling manifest.py's SELF_HOSTING
    attribute (default False when absent). FRAMEWORK_SCOPED invariants
    (bijection over ALL_INVARIANTS, docstring/body coherence, rules-as-data
    classification, and the domain+agent filesystem walks) carry framework
    jurisdiction, not business-domain jurisdiction — they run only when
    self_hosting is True (invariants.all_violations gates them on this flag).

    WHY a graph field (not a call-site parameter): all_violations(g) takes
    only the graph; threading jurisdiction through a second parameter would
    change every call site (tests, gate.py, what_now.py) for one domain's
    concern. A field travels with the graph the same way axes/self_hosting
    already do, and is set once at the loader boundary (_load_graph_file),
    keeping the invariant functions themselves parameter-free.
    """

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


def domain_doc_readers(domain_dir: Path) -> dict[str, str]:
    """Canon: §Graph / §Domain — one SPECIFIC domain's declared DOC_READERS binding.

    RULE: import `domain_dir/manifest.py` and return its `DOC_READERS`
    attribute (a `dict[role_hint, Stakeholder.id]`) if present, else `{}`.
    Never fabricates a binding.

    WHY a per-domain variant (not only the env-resolved active_domain_doc_readers):
    the per-domain doc generator (gen_spec._process_domains) renders EACH
    domain's docs/gen/ from that domain's OWN graph; its `reader:` header must
    resolve from the SAME domain, not from whatever HOTAM_SPEC_ACTIVE_DOMAIN
    happens to be set to. Resolving through the env-active binding contaminated
    the self-host docs' reader with the transiently-active domain's (or an
    unresolved sentinel) whenever a proposal was landed for a non-pinned domain
    (R-root-crystal-follows-pin). This isolates reader resolution to the domain
    actually being rendered.
    """
    manifest_py = domain_dir / "manifest.py"
    if not manifest_py.exists():
        return {}
    spec = importlib.util.spec_from_file_location(
        f"_manifest_doc_readers_{domain_dir.name}", manifest_py
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


def active_domain_doc_readers() -> dict[str, str]:
    """Canon: §Graph / §Domain — the active domain's declared DOC_READERS binding.

    RULE: resolves the active domain directory the same way
    `_active_domain_graph_file` does (env var, else first domains/<name>/
    alphabetically), then delegates to `domain_doc_readers(domain_dir)`.
    Never fabricates a binding — a domain that has not declared
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
    bindings = domain_doc_readers(graph_file.parent)
    if not bindings:
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
    self_hosting = _manifest_self_hosting(graph_file.parent)
    if self_hosting != g.self_hosting:
        import dataclasses  # noqa: PLC0415

        g = dataclasses.replace(g, self_hosting=self_hosting)
    return g


def _manifest_self_hosting(domain_dir: Path) -> bool:
    """Canon: §Domain — read SELF_HOSTING off domains/<name>/manifest.py (default False).

    RULE: manifest.py is read directly (not via graph.py); a domain with no
    manifest.py (e.g. the legacy spec/content/ fallback) or no SELF_HOSTING
    attribute is NOT self-hosting (R-domain-self-hosting-flag). Only
    domains/hotam-spec-self/manifest.py sets SELF_HOSTING = True.

    WHY read here (not cached on TensionGraph construction by the domain's
    own graph.py): manifest.py is domain plumbing edited directly (not via
    apply_proposal, per the ЖЁСТКИЕ ПРАВИЛА); keeping the flag's source of
    truth in manifest.py — not duplicated into graph.py's build_graph() call
    sites — means a steward flips one file to change jurisdiction.
    """
    manifest_py = domain_dir / "manifest.py"
    if not manifest_py.exists():
        return False
    spec = importlib.util.spec_from_file_location(
        f"_manifest_self_hosting_{domain_dir.name}", manifest_py
    )
    if spec is None or spec.loader is None:
        return False
    mod = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(mod)  # type: ignore[union-attr]
    except Exception:
        return False
    return bool(getattr(mod, "SELF_HOSTING", False))


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
# Dependency traversal — depends_on chains and independent subgraphs
# ---------------------------------------------------------------------------


def dependency_chains(g: TensionGraph) -> tuple[tuple[str, ...], ...]:
    """Canon: §Graph — maximal depends_on chains, deepest-dependency first.

    RULE: build the directed graph whose edges are exactly the `depends_on`
    Relations between Requirements (A depends_on B => edge A -> B). Return every
    MAXIMAL path (a path whose start node has no incoming depends_on edge and
    whose end node has no outgoing depends_on edge), each rendered as a tuple of
    Requirement ids ordered from the most-depended-UPON leaf to the most-
    dependent root — i.e. reverse topological order, so a consumer can build /
    verify the chain front-to-back (R-dependency-drives-sequential: the chain is
    emitted in the order the work must actually happen, dependency before
    dependent). Chains of length < 2 (an isolated requirement with no depends_on
    edge in either direction) are omitted — a chain needs at least one real edge.
    Edges whose target is absent from the graph (dangling) are skipped here (they
    are a separate diagnosable defect, not this traversal's concern). The outer
    tuple is sorted for determinism (R-deterministic-generation). A cyclic
    depends_on component yields no chain from its cycle members (a cycle has no
    maximal linear path); acyclic tails attached to it are still surfaced.

    WHY reverse-topological (leaf-first): "sequential" means the dependency is
    done before the thing that depends on it; emitting leaf-first makes the tuple
    itself the execution order, so R-dependency-drives-sequential is satisfied by
    reading the chain left to right rather than by a separate sort at the call site.
    """
    # Adjacency: node -> set of nodes it depends_on (edge node -> dep).
    ids = {r.id for r in g.requirements}
    deps: dict[str, set[str]] = {r.id: set() for r in g.requirements}
    rdeps: dict[str, set[str]] = {r.id: set() for r in g.requirements}
    for r in g.requirements:
        for rel in r.relations:
            if rel.kind == "depends_on" and rel.target in ids:
                deps[r.id].add(rel.target)
                rdeps[rel.target].add(r.id)

    # Roots: nodes that nothing depends on (no incoming reverse edge, i.e. no
    # requirement lists them as a dependent) but which DO depend on something.
    roots = sorted(
        rid for rid in ids if not rdeps[rid] and deps[rid]
    )

    chains: set[tuple[str, ...]] = set()

    def _walk(root: str) -> None:
        # DFS every maximal path root -> ... -> leaf, guarding against cycles.
        stack: list[tuple[str, tuple[str, ...]]] = [(root, (root,))]
        while stack:
            node, path = stack.pop()
            children = sorted(deps[node])
            unseen = [c for c in children if c not in path]
            if not unseen:
                # leaf (or cycle-blocked): emit path leaf-first (reversed).
                if len(path) >= 2:
                    chains.add(tuple(reversed(path)))
                continue
            for c in unseen:
                stack.append((c, path + (c,)))

    for root in roots:
        _walk(root)

    return tuple(sorted(chains))


def independent_subgraphs(g: TensionGraph) -> tuple[tuple[str, ...], ...]:
    """Canon: §Graph — connected components over the depends_on relation.

    RULE: treat every `depends_on` Relation between two Requirements present in
    the graph as an UNDIRECTED edge and return the connected components — each a
    sorted tuple of Requirement ids — with the outer tuple sorted for
    determinism. A component with a single member (a requirement sharing no
    depends_on edge with any other) IS returned: it is a maximally-independent
    unit. Two components share no depends_on edge, so they are safe to work on in
    parallel (R-dependency-drives-parallel: disjoint components are the
    parallelizable slices).

    WHY undirected components (not the directed chains above): parallelism cares
    only about REACHABILITY through dependency edges, in either direction — if A
    depends_on B and C depends_on B, then A and C are in one component (they share
    B) and must NOT be assumed independent, even though there is no directed path
    between A and C. Union-find over undirected edges captures exactly that.
    """
    parent: dict[str, str] = {r.id: r.id for r in g.requirements}

    def _find(x: str) -> str:
        while parent[x] != x:
            parent[x] = parent[parent[x]]
            x = parent[x]
        return x

    def _union(a: str, b: str) -> None:
        ra, rb = _find(a), _find(b)
        if ra != rb:
            # Attach the lexicographically-larger root under the smaller for
            # deterministic component roots.
            hi, lo = (ra, rb) if ra > rb else (rb, ra)
            parent[hi] = lo

    ids = set(parent)
    for r in g.requirements:
        for rel in r.relations:
            if rel.kind == "depends_on" and rel.target in ids:
                _union(r.id, rel.target)

    comps: dict[str, list[str]] = {}
    for rid in ids:
        comps.setdefault(_find(rid), []).append(rid)

    return tuple(sorted(tuple(sorted(members)) for members in comps.values()))


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
    surface; a DEAD assumption is never silently dropped. The filter keys off
    exact status equality, so the volitional IMPLEMENTS status is naturally
    excluded — an aspiration is not a broken premise
    (R-assumption-implements-state).
    """
    return tuple(a for a in g.assumptions if a.status == DEAD)


def uncertain_assumptions(g: TensionGraph) -> tuple[Assumption, ...]:
    """Canon: §Graph — assumptions currently flipped to UNCERTAIN.

    RULE: an UNCERTAIN assumption is under question but not yet falsified — it is
    NOT fallout (nothing has died), but a high-fan-out UNCERTAIN premise is a
    standing review debt the harness surfaces (see what_now's UNCERTAIN-aging
    band, R-uncertain-assumptions-surface). A UNCERTAIN assumption that nothing
    rests on is invisible-and-harmless; one many requirements rest on is the
    largest silent question in the graph.

    WHY a peer of dead_assumptions(): DEAD lights up fallout (P2); UNCERTAIN
    lights up review pressure (P4). Both are pure status filters over g. The
    volitional IMPLEMENTS status is excluded by the exact-equality filter — an
    aspiration is not an unresolved doubt and must raise NO review-debt signal
    (R-assumption-implements-state).
    """
    return tuple(a for a in g.assumptions if a.status == UNCERTAIN)


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
