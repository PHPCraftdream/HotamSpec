"""Canon: §Attention — the agent-agnostic registry of "attention codes".

RULE (R-attention-registry): every signal an agent is obliged to notice —
"here is what needs your attention right now" — is produced by a NAMED source
in a single registry (ATTENTION_SOURCES), and `collect(g, ...)` runs the
registry and returns a flat list of typed AttentionSignal records. No agent,
on any platform, has to remember where the signals live: it runs the core and
reads the list. The platform adapter under tools/ is merely ONE consumer of
this list, not its owner (R-attention-agent-agnostic-core).

RULE (R-attention-superset-of-diagnose): the registry has TWO kinds of source,
distinguished by what each reads:

  * GRAPH sources read ONLY the in-memory TensionGraph. They are pure and
    DETERMINISTIC: running twice on the same graph yields byte-identical
    signals. `diagnose_signals(g)` is the single graph source; it is the exact
    set gen_spec.py renders into the byte-stable LIVE-STATE via
    tools/what_now.diagnose (R-deterministic-generation).

  * RUNTIME-FS sources read the FILESYSTEM (spec/.runtime/*, tickets/, pending
    proposals). Their output depends on wall-clock and on-disk state, so it is
    NON-deterministic across runs. These sources are NEVER built into the core
    registry: they are INJECTED by the live consumer (tools/what_now, the
    platform adapter) through `collect(..., runtime_sources=...)`. This is the
    architectural
    guarantee that fs-signals can never leak into diagnose()/LIVE-STATE and
    break determinism — the core simply has no reference to them.

Therefore, for a live agent, `collect(g, runtime_sources=RUNTIME_FS_SOURCES)`
is a SUPERSET of `diagnose_signals(g)`: it adds the runtime bands on top of the
deterministic graph diagnosis. For the substrate (gen_spec / LIVE-STATE), only
`diagnose_signals(g)` — the deterministic subset — is ever consumed. The
superset feeds the agent; the subset feeds the substrate. That split is the
centerpiece of this module.

WHY a core (not just tool functions): the "what needs attention" signals were
scattered — half in diagnose(g), half as CLI-only bands owned by
tools/what_now.py. There was no single, agent-agnostic, importable-as-a-library
place an arbitrary agent could call. Lifting them into one stdlib-only core with
an explicit registry makes the sensorium a first-class, testable object and lets
what_now become one CONSUMER of the core rather than the owner of the bands
(R-attention-agent-agnostic-core, R-prefer-tool-over-hand).

This module imports ONLY the Python stdlib and hotam_spec itself — it knows
nothing of any agent platform or its adapters (enforced by
test_backend_neutral_scope's core-import scan, R-core-imports-stdlib-or-hotam-spec-only,
and by test_attention_core's platform-token scan).
"""

from __future__ import annotations

from collections.abc import Callable, Iterable, Sequence
from dataclasses import dataclass

from hotam_spec.conflict import ACKNOWLEDGED, DETECTED
from hotam_spec.graph import (
    TensionGraph,
    conflicts_on_assumption,
    dead_assumptions,
    entity_state_conflict_suspects,
    latent_connector_clusters,
    requirements_on_assumption,
    uncertain_assumptions,
)
from hotam_spec.invariants import all_violations
from hotam_spec.reflection import all_findings

# --- Priority bands (shared vocabulary; mirrored by tools/what_now.py) -------

P_REFLECTION = 0  # highest — operator self-readiness ranked above structural form
P_STRUCTURE = 1
P_DRIFT_FALLOUT = 2
P_CONFLICT_STALLED = 3
P_OPEN_ITEM = 4
P_LATENT_CONNECTOR = 5
P_RUNTIME = 6  # runtime-fs bands (pending proposals, tickets, revisit, audit)
P_ADVISORY = 7  # lowest — Finding.advisory=True: NEVER a gate (§Attention, A2)

BAND_LABEL = {
    P_REFLECTION: "REFLECTION",
    P_STRUCTURE: "STRUCTURE",
    P_DRIFT_FALLOUT: "DRIFT_FALLOUT",
    P_CONFLICT_STALLED: "CONFLICT_STALLED",
    P_OPEN_ITEM: "OPEN_ITEM",
    P_LATENT_CONNECTOR: "LATENT_CONNECTOR",
    P_RUNTIME: "PENDING_PROPOSAL",
    P_ADVISORY: "ADVISORY",
}

#: Canon: §Attention — UNCERTAIN-aging threshold (R-uncertain-assumptions-surface).
#: An UNCERTAIN assumption with at least this many dependent Requirements is a
#: standing review debt worth a P4 signal; below it the doubt is too local.
UNCERTAIN_AGING_MIN_DEPENDENTS = 5

#: Source-classification vocabulary: what a source reads.
READS_GRAPH = "graph"
READS_RUNTIME_FS = "runtime-fs"
READS_VALUES = frozenset({READS_GRAPH, READS_RUNTIME_FS})


@dataclass(frozen=True)
class AttentionSignal:
    """Canon: §Attention — one agent-agnostic "pay attention here" signal.

    Fields:
      source     — the ATTENTION_SOURCES id that produced this signal (e.g.
                   'diagnose', 'open-tickets') — so a consumer can tell which
                   sensor fired and whether it was graph or runtime-fs.
      priority   — band (0..6); lower is more urgent. See P_* constants.
      target     — the object id / meter slug to act on.
      message    — human-readable instruction, surfaced verbatim by any adapter.

    WHY plain data (no platform types): this is the border object every agent
    reads. It carries no platform specifics; a CLI, an adapter, or a foreign
    agent all consume the same record (R-attention-agent-agnostic-core).
    """

    source: str
    priority: int
    target: str
    message: str


@dataclass(frozen=True)
class AttentionSource:
    """Canon: §Attention — one named entry in the attention-code registry.

    Fields:
      id       — stable slug naming the sensor (appears in AttentionSignal.source).
      reads    — READS_GRAPH or READS_RUNTIME_FS: the determinism class. GRAPH
                 sources are pure/deterministic and safe for the substrate;
                 RUNTIME_FS sources read disk and are live-consumer-only.
      collect  — Callable[[TensionGraph], Iterable[AttentionSignal]].

    WHY the `reads` tag is first-class (not a comment): it is the machine-
    checkable guarantee behind R-attention-superset-of-diagnose. The core
    refuses to build a runtime-fs source into its deterministic registry; only
    graph sources may live in ATTENTION_SOURCES (asserted at import time).
    """

    id: str
    reads: str
    collect: Callable[[TensionGraph], Iterable[AttentionSignal]]


# ---------------------------------------------------------------------------
# The single GRAPH source — deterministic graph diagnosis (P0..P5).
# This is the exact set gen_spec/LIVE-STATE consume (via what_now.diagnose).
# ---------------------------------------------------------------------------


def diagnose_signals(g: TensionGraph) -> list[AttentionSignal]:
    """Canon: §Attention — the deterministic graph-only diagnosis (P0..P5, P7).

    RULE: pure and graph-only. Signals are emitted band by band in stable
    graph/id order, then a final stable sort by (priority, target, message)
    fixes the global order; running twice on the same graph yields the same
    list. This is the DETERMINISTIC SUBSET the substrate consumes — it reads no
    filesystem and no clock (R-attention-superset-of-diagnose,
    R-deterministic-generation).

    tools/what_now.diagnose() is a thin adapter over this function (mapping
    AttentionSignal -> its Action type), so the harness and the substrate share
    one body of graph-diagnosis logic instead of two copies.
    """
    out: list[AttentionSignal] = []

    # P0 REFLECTION — operator self-readiness (§Reflection). A Finding that
    # declares itself advisory (Finding.advisory=True — NEVER a gate) is
    # routed to the lowest-urgency P_ADVISORY band instead, so genuinely
    # actionable self-diagnosis stays at P0 without advisory noise mixed in
    # (§Attention, A2).
    for f in all_findings(g):
        priority = P_ADVISORY if f.advisory else P_REFLECTION
        out.append(AttentionSignal("diagnose", priority, f.target, f.imperative))

    # P1 STRUCTURE — failing structural invariants.
    for v in all_violations(g):
        out.append(
            AttentionSignal(
                "diagnose", P_STRUCTURE, v.target, f"[{v.invariant}] {v.message}"
            )
        )

    # P2 DRIFT_FALLOUT — DEAD assumptions with live dependents.
    for a in dead_assumptions(g):
        dep_reqs = requirements_on_assumption(g, a.id)
        dep_cons = conflicts_on_assumption(g, a.id)
        if not dep_reqs and not dep_cons:
            continue
        for r in dep_reqs:
            out.append(
                AttentionSignal(
                    "diagnose",
                    P_DRIFT_FALLOUT,
                    r.id,
                    f"assumption '{a.id}' is DEAD ({a.statement!r}); "
                    f"revisit requirement '{r.id}' which rests on it",
                )
            )
        for c in dep_cons:
            out.append(
                AttentionSignal(
                    "diagnose",
                    P_DRIFT_FALLOUT,
                    c.id,
                    f"assumption '{a.id}' is DEAD; revive conflict cluster "
                    f"'{c.id}' whose shared_assumption was '{a.id}'",
                )
            )

    # P3 CONFLICT_STALLED — conflicts with no steward resolution.
    for c in g.conflicts:
        if c.lifecycle == DETECTED:
            out.append(
                AttentionSignal(
                    "diagnose",
                    P_CONFLICT_STALLED,
                    c.id,
                    f"conflict '{c.id}' on axis '{c.axis}' is DETECTED with no "
                    f"steward movement; steward '{c.steward}' must ACKNOWLEDGE it",
                )
            )
        elif c.lifecycle == ACKNOWLEDGED:
            out.append(
                AttentionSignal(
                    "diagnose",
                    P_CONFLICT_STALLED,
                    c.id,
                    f"conflict '{c.id}' is ACKNOWLEDGED but undecided; steward "
                    f"'{c.steward}' must DECIDE (rationale) or set REVISIT_WHEN",
                )
            )

    # P4 OPEN_ITEM — OPEN(question) requirements.
    for r in g.requirements:
        if r.is_open():
            question = r.status[len("OPEN"):].strip().strip("()").strip()
            out.append(
                AttentionSignal(
                    "diagnose",
                    P_OPEN_ITEM,
                    r.id,
                    f"OPEN requirement '{r.id}' (owner '{r.owner}') awaits a "
                    f"decision: {question or '(no question stated)'}",
                )
            )

    # P4 OPEN_ITEM — HELD conflicts carrying variants: the steward must choose.
    for c in g.conflicts:
        if c.is_held():
            for v in c.variants:
                out.append(
                    AttentionSignal(
                        "diagnose",
                        P_OPEN_ITEM,
                        c.id,
                        f"choose a variant: '{v.id}' — {c.axis}",
                    )
                )

    # P4 OPEN_ITEM — UNCERTAIN-aging assumptions with high fan-out.
    for a in uncertain_assumptions(g):
        dep_reqs = requirements_on_assumption(g, a.id)
        if len(dep_reqs) < UNCERTAIN_AGING_MIN_DEPENDENTS:
            continue
        out.append(
            AttentionSignal(
                "diagnose",
                P_OPEN_ITEM,
                a.id,
                f"review assumption '{a.id}' ({a.statement!r}): still "
                f"UNCERTAIN with {len(dep_reqs)} dependent requirements — "
                f"resolve the doubt (transition to DEAD or re-affirm HOLDS) "
                f"or it drifts",
            )
        )

    # P5 LATENT_CONNECTOR — heuristic missing-connector suspects, clustered.
    for cl in latent_connector_clusters(g):
        sig = ", ".join(cl.assumptions)
        out.append(
            AttentionSignal(
                "diagnose",
                P_LATENT_CONNECTOR,
                ",".join(cl.assumptions),
                f"[HEURISTIC, for AI review] assumption(s) {sig} shared by "
                f"{len(cl.requirements)} requirements "
                f"({', '.join(cl.requirements)}) with no mediating Conflict "
                f"node — review the cluster as ONE item: consider splitting "
                f"the assumption or materializing a connector "
                f"({len(cl.pairs)} pair(s); detail: docs/gen/TENSIONS.md)",
            )
        )

    for s in entity_state_conflict_suspects(g):
        out.append(
            AttentionSignal(
                "diagnose",
                P_LATENT_CONNECTOR,
                f"{s.left}~{s.right}",
                f"[HEURISTIC, entity-state conflict] {s.hint}",
            )
        )

    out.sort(key=lambda s: (s.priority, s.target, s.message))
    return out


#: Canon: §Attention — the framework registry. GRAPH sources ONLY (deterministic).
#: Runtime-fs sources are injected by the live consumer, never listed here, so
#: they can never leak into the deterministic substrate (R-attention-superset-of-diagnose).
ATTENTION_SOURCES: tuple[AttentionSource, ...] = (
    AttentionSource(id="diagnose", reads=READS_GRAPH, collect=diagnose_signals),
)

# Import-time guard: the framework registry admits deterministic graph sources
# ONLY. This is the structural spine of R-attention-superset-of-diagnose — a
# runtime-fs source added here (by mistake) would fail the build immediately.
assert all(s.reads == READS_GRAPH for s in ATTENTION_SOURCES), (
    "ATTENTION_SOURCES must contain graph-only sources; runtime-fs sources are "
    "injected via collect(runtime_sources=...) so they never leak into the "
    "deterministic substrate (R-attention-superset-of-diagnose)."
)


def collect(
    g: TensionGraph,
    *,
    runtime_sources: Sequence[AttentionSource] = (),
) -> list[AttentionSignal]:
    """Canon: §Attention — run the attention registry and return all signals.

    The core's built-in ATTENTION_SOURCES (graph-only, deterministic) always
    run. A live consumer (tools/what_now, the platform adapter, any agent) may
    pass `runtime_sources` — filesystem/clock-reading AttentionSources — to get the
    SUPERSET that also carries pending proposals, open tickets, stale audit and
    unread revisit markers.

    With no runtime_sources, `collect(g)` == `diagnose_signals(g)` (same
    signals, same order): the deterministic subset. With
    `runtime_sources=RUNTIME_FS_SOURCES`, it is the live superset for the agent.
    Signals are returned sorted by (priority, source, target, message) for a
    stable, scannable order (R-attention-superset-of-diagnose).

    Passing a READS_GRAPH-tagged source in runtime_sources is a programming
    error (graph sources belong in ATTENTION_SOURCES); it is rejected loudly.
    """
    for s in runtime_sources:
        if s.reads != READS_RUNTIME_FS:
            raise ValueError(
                f"runtime_sources must be READS_RUNTIME_FS; source {s.id!r} is "
                f"{s.reads!r}. Graph sources belong in ATTENTION_SOURCES."
            )
    sources: Iterable[AttentionSource] = (*ATTENTION_SOURCES, *runtime_sources)
    signals: list[AttentionSignal] = []
    for src in sources:
        signals.extend(src.collect(g))
    signals.sort(key=lambda s: (s.priority, s.source, s.target, s.message))
    return signals


def render_flat(signals: Sequence[AttentionSignal]) -> str:
    """Canon: §Attention — render signals as agent-agnostic plain text.

    One line per signal: `[P<n> <BAND>] <target>: <message>`. Pure text, no
    platform markup — any agent (or a platform adapter) injects this verbatim.
    Returns a calm one-liner when there is nothing to attend to.
    """
    if not signals:
        return "attention: clear — no signals.\n"
    lines = [f"attention: {len(signals)} signal(s) (lower P = more urgent)."]
    for s in signals:
        band = BAND_LABEL.get(s.priority, str(s.priority))
        lines.append(f"[P{s.priority} {band}] {s.target}: {s.message}")
    return "\n".join(lines) + "\n"


def signals_to_lines(signals: Iterable[AttentionSignal]) -> list[str]:
    """Canon: §Attention — the per-signal body lines (no header), for embedding."""
    out = []
    for s in signals:
        band = BAND_LABEL.get(s.priority, str(s.priority))
        out.append(f"[P{s.priority} {band}] {s.target}: {s.message}")
    return out
