"""Canon: §Harness — derives the prioritized next correct action from any graph state, making being-lost structurally impossible.

The harness: derive the next correct action from ANY graph state ("what now").

This is the centerpiece. dev-coin makes DRIFT structurally impossible (regen ==
committed). Hotam-Spec generalizes that to make BEING LOST structurally impossible:
an agent dropped into the repo in any state runs this tool and deterministically
gets a prioritized, typed list of next actions. It is the Diagnosis step of the
closed loop:

    State (graph + generated docs + test status)
      -> Diagnosis  (THIS tool: tools/what_now.py)
      -> Next-action (typed, prioritized, addressable)
      -> Action     (edit the graph in spec/content)
      -> regenerate (tools/gen_spec.py)
      -> State.

It aggregates, in priority order:
  P0 REFLECTION       — operator self-diagnosis: DRAFT-overhang (burn-down meter),
                        UNENFORCED-SETTLED debt, over-budget operators, dead-
                        assumption-on-enforcer, derived-but-unbuilt. Ranked ABOVE
                        P1 STRUCTURE because an operator that cannot see its own
                        state is worse than a malformed graph. The conditions are
                        named predicates in hotam_spec.reflection
                        (R-reflection-predicates-first-class). (§Reflection, M35)
  P1 STRUCTURE        — failing structural invariants (malformed form / dangling
                        refs / conflict missing axis|context|steward). A malformed
                        graph makes all softer diagnosis unreliable.
  P2 DRIFT_FALLOUT    — DEAD assumptions with live dependents: every Requirement
                        and Conflict resting on them to revisit (context drift,
                        invisibility #3). One dead assumption re-opens a cluster.
  P3 CONFLICT_STALLED — conflicts stuck DETECTED/ACKNOWLEDGED with no steward
                        resolution: a contradiction seen but not yet held.
  P4 OPEN_ITEM        — OPEN(question) requirements awaiting a steward decision.
  P5 LATENT_CONNECTOR — HEURISTIC: requirement pairs that SHOULD have a C-node but
                        don't, flagged "for AI review" (the deferred detector's
                        stub), rendered ONE action per shared-assumption CLUSTER
                        (graph.latent_connector_clusters; pair detail stays in
                        TENSIONS.md). Lowest priority because it is a suspicion,
                        not a proven defect, and the AI never acts on it
                        unilaterally.
  P6 PENDING_PROPOSAL — a proposal JSON file sits under spec/.runtime/proposals/
                        (or its pending/ sub-folder) awaiting the steward's
                        verdict; not landed yet, so not in applied/. Pure file
                        surfacing, NOT a graph diagnosis — no new node type
                        (R-presented-pending-decision-type).
  P7 ADVISORY         — LOWEST priority: findings whose predicate declared
                        itself Finding.advisory=True (e.g.
                        reflect_replaces_edge_migration,
                        reflect_all_members_rejected) — NEVER a gate/blocker,
                        surfaced for awareness only, ranked below even the
                        ephemeral PENDING_PROPOSAL band (§Attention, A2).

Run:
  python tools/what_now.py            # diagnose spec/content/ (your domain)
  python tools/what_now.py --demo     # diagnose the fixture demo graph
  python tools/what_now.py --report   # single advisory Tick report (was tools/tick.py)

Dependency-light (stdlib + the hotam_spec package). Deterministic ordering.
"""

from __future__ import annotations

import argparse
import json
import sys
from dataclasses import dataclass
from pathlib import Path

# hotam_spec lives in spec/src; make it importable whether run via uv or plain python.
_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from hotam_spec.graph import (  # noqa: E402
    CONTENT_GRAPH_FILE,
    TensionGraph,
    load_content_graph,
)
from hotam_spec import attention as _attention  # noqa: E402
from hotam_spec.attention import (  # noqa: E402
    UNCERTAIN_AGING_MIN_DEPENDENTS,
    AttentionSignal,
    AttentionSource,
    READS_RUNTIME_FS,
)
from hotam_spec.invariants import all_violations  # noqa: E402,F401
from hotam_spec.reflection import all_findings  # noqa: E402,F401
from hotam_spec.runtime_paths import runtime_dir as _runtime_dir  # noqa: E402

# Re-exported for backwards compatibility with tests that referenced these
# names on this module before the graph-diagnosis body moved into the
# hotam_spec.attention core (Wave 16). They remain the canonical source.
_ = (UNCERTAIN_AGING_MIN_DEPENDENTS, all_violations, all_findings)

# --- Priority bands (lower number = more urgent) ----------------------------

P_REFLECTION = 0  # highest — operator self-readiness ranked above structural form
P_STRUCTURE = 1
P_DRIFT_FALLOUT = 2
P_CONFLICT_STALLED = 3
P_OPEN_ITEM = 4
P_LATENT_CONNECTOR = 5
P_PENDING_PROPOSAL = 6
P_ADVISORY = 7  # lowest — Finding.advisory=True: NEVER a gate (§Attention, A2)

_BAND_LABEL = {
    P_REFLECTION: "REFLECTION",
    P_STRUCTURE: "STRUCTURE",
    P_DRIFT_FALLOUT: "DRIFT_FALLOUT",
    P_CONFLICT_STALLED: "CONFLICT_STALLED",
    P_OPEN_ITEM: "OPEN_ITEM",
    P_LATENT_CONNECTOR: "LATENT_CONNECTOR",
    P_PENDING_PROPOSAL: "PENDING_PROPOSAL",
    P_ADVISORY: "ADVISORY",
}


@dataclass(frozen=True)
class TickReport:
    """Canon: §Tick — one diagnostic cycle result (advisory, M32 conservative).

    Fields:
      cycle        — caller-supplied monotonic cycle counter.
      total_actions — count of actions diagnose() returned.
      band_counts  — {band_label: count} summary.
      top_action   — the highest-priority Action, or None if graph is clean.
      paused       — True when top_action requires steward attention (always
                     True under M32 conservative; False only on empty graph).
      paused_reason — human explanation of why the tick is paused.
      advisory     — the one-line human-readable advice for this cycle.

    WHY frozen: a tick report is a point-in-time snapshot; it should not
    be mutated after construction (R-anchor-everything applied to data).

    Formerly tools/tick.py (a thin wrapper adding no logic beyond
    diagnose()+render()); folded into what_now.py's --report flag so the
    closed-loop diagnostic driver and the next-action harness share one
    module (R-prefer-tool-over-hand / mechanical de-duplication).
    """

    cycle: int
    total_actions: int
    band_counts: dict[str, int]
    top_action: object | None  # Action or None
    paused: bool  # True if top action requires steward attention
    paused_reason: str
    advisory: str  # the human-readable advice line


def tick(cycle: int = 1) -> TickReport:
    """Canon: §Tick — one advisory tick: load, diagnose, classify, render advice.

    M32 (conservative): EVERY band pauses — the tick never auto-applies.
    Returns a TickReport; the caller decides whether to print/log/act.

    WHY load fresh each call: the substrate may have changed between ticks
    (e.g. an operator applied a proposal); re-loading ensures each cycle
    reflects the current graph, not a cached snapshot.
    """
    g = load_content_graph()
    actions = diagnose(g)
    band_counts: dict[str, int] = {}
    for a in actions:
        band_counts[a.kind] = band_counts.get(a.kind, 0) + 1
    if not actions:
        return TickReport(
            cycle=cycle,
            total_actions=0,
            band_counts={},
            top_action=None,
            paused=False,
            paused_reason="",
            advisory=(
                "TICK OK — no actions. The graph is well-formed and every "
                "contradiction is visible, stewarded, and up to date."
            ),
        )
    top = actions[0]
    # M32 conservative: EVERY band pauses. The tick is advisory.
    paused_reason = (
        f"M32 (conservative): tick is advisory. The {top.kind} action requires "
        f"a steward-approved proposal via apply_proposal.py; the tick does not "
        f"auto-apply."
    )
    # If REFLECTION actions are present, surface them explicitly — P0 lands first.
    reflection_n = sum(1 for a in actions if a.kind == "REFLECTION")
    reflection_note = (
        f" ({reflection_n} REFLECTION — operator self-readiness)"
        if reflection_n
        else ""
    )
    advisory = (
        f"TICK CYCLE {cycle}: {len(actions)} action(s){reflection_note};"
        f" top is [P{top.priority}] {top.kind} on `{top.target}`."
        f" Steward should review and submit a"
        f" proposal via apply_proposal.py (see docs/playbooks/)."
    )
    return TickReport(
        cycle=cycle,
        total_actions=len(actions),
        band_counts=band_counts,
        top_action=top,
        paused=True,
        paused_reason=paused_reason,
        advisory=advisory,
    )


def render_tick(report: TickReport) -> str:
    """Canon: §Tick — render a TickReport as a human-readable LF string.

    Format: cycle header, band summary, top action, paused notice, advisory.
    Empty graph: cycle header + advisory only. Always ends with newline.

    WHY a separate render step: keeps tick() pure (returns data); lets
    tests assert on the TickReport fields without parsing text, while
    callers that want human output call render_tick().
    """
    lines = [f"== tick: cycle {report.cycle} =="]
    if report.total_actions == 0:
        lines.append("")
        lines.append(report.advisory)
        return "\n".join(lines) + "\n"
    lines.append("")
    lines.append(f"Bands: {dict(sorted(report.band_counts.items()))}")
    lines.append("")
    if report.top_action is not None:
        a = report.top_action
        lines.append(f"Top action: [P{a.priority}] {a.kind} on `{a.target}`")
        lines.append(f"  imperative: {a.imperative}")
    lines.append("")
    if report.paused:
        lines.append(f"PAUSED — {report.paused_reason}")
    lines.append("")
    lines.append(report.advisory)
    return "\n".join(lines) + "\n"


@dataclass(frozen=True)
class Action:
    """One typed, addressable next-action the agent can take.

    Fields:
      priority — band (0..7); lower is more urgent. P0=REFLECTION is the
                 operator self-readiness band; P1..P5 are domain diagnosis;
                 P6/P7 are runtime-fs/advisory bands (see module docstring).
      kind     — the band label (REFLECTION / STRUCTURE / DRIFT_FALLOUT / ...).
      target   — the object id to act on (Requirement/Conflict/Assumption id).
      imperative — human-readable instruction (what to do, in the imperative).

    WHY typed + targeted: an agent (or human) can act without re-deriving context
    — the id says where, the imperative says what, the band says how urgent.
    """

    priority: int
    kind: str
    target: str
    imperative: str


def _signal_to_action(s: AttentionSignal) -> Action:
    """Adapt a core AttentionSignal into the harness's Action shape.

    The band label is looked up from the signal's priority so what_now's
    rendering (which keys on Action.kind) is unchanged.
    """
    return Action(
        priority=s.priority,
        kind=_BAND_LABEL.get(s.priority, str(s.priority)),
        target=s.target,
        imperative=s.message,
    )


def diagnose(g: TensionGraph) -> list[Action]:
    """Derive the full prioritized next-action list from a graph state.

    Thin adapter over hotam_spec.attention.diagnose_signals(g): the graph-only,
    DETERMINISTIC diagnosis body now lives in the attention core (the single
    graph AttentionSource) so the harness and the substrate share one copy
    (R-attention-superset-of-diagnose). This function only maps AttentionSignal
    -> Action. Deterministic: the core sorts by (priority, target, message);
    the final stable sort here fixes (priority, kind, target).
    """
    actions = [_signal_to_action(s) for s in _attention.diagnose_signals(g)]
    actions.sort(key=lambda a: (a.priority, a.kind, a.target, a.imperative))
    return actions



def pending_proposal_actions(*, now: float | None = None) -> list[Action]:
    """Canon: §Harness — P6 PENDING_PROPOSAL band: proposal files awaiting a verdict.

    RULE (R-presented-pending-decision-type): NOT part of diagnose(g) — this
    reads the FILESYSTEM (spec/.runtime/proposals/), not the graph, and its
    'N days' age is wall-clock-relative, so it is NON-DETERMINISTIC across
    runs. gen_spec.py's generated docs must be byte-stable
    (R-deterministic-generation), so this band is surfaced ONLY by the
    interactive CLI (main(), below) and is never fed into diagnose() or any
    generated doc.

    `now` (unix seconds) defaults to time.time(); tests pass a fixed value for
    determinism of THIS function's own unit tests.
    """
    import time as _time  # noqa: PLC0415

    _tools = Path(__file__).resolve().parent
    if str(_tools) not in sys.path:
        sys.path.insert(0, str(_tools))
    import apply_proposal as _apply_proposal  # noqa: PLC0415

    ref_time = now if now is not None else _time.time()
    actions: list[Action] = []
    for p in _apply_proposal.pending_proposal_files():
        age_days = max(0, int((ref_time - p.stat().st_mtime) // 86400))
        actions.append(
            Action(
                priority=P_PENDING_PROPOSAL,
                kind=_BAND_LABEL[P_PENDING_PROPOSAL],
                target=p.name,
                imperative=(
                    f"presented, awaiting steward: {p.name}, {age_days} day(s)"
                ),
            )
        )
    return actions


TENSION_AUDIT_STAMP = (
    _runtime_dir() / "tension-audit.jsonl"
)
"""Canon: §Harness — append-only run stamp written by tools/audit_tensions.py."""

GENERATIVE_AUDIT_STALE_DELTA = 10
"""Canon: §Harness — net-new SETTLED atoms since the last generative sweep that
make the audit stale. Generous: a handful of new atoms rarely hides a fresh
cross-tension, but past this the un-swept surface compounds silently."""

REVISIT_STALE_PERCENT = 5
"""Canon: §Harness — percentage growth of SETTLED atoms since the last revisit
evaluation that triggers re-evaluation. At 5%, a 200-atom graph needs +10 new
atoms to re-fire, a 400-atom graph needs +20. This replaces the old absolute
threshold (GENERATIVE_AUDIT_STALE_DELTA=10) which re-fired every ~1.5 days at
normal development pace (~8 atoms/day), causing noise/habituation."""


def _last_audit_settled_count() -> int | None:
    """settled_count from the LAST line of tension-audit.jsonl, or None if absent."""
    if not TENSION_AUDIT_STAMP.exists():
        return None
    last: int | None = None
    for line in TENSION_AUDIT_STAMP.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            rec = json.loads(line)
        except json.JSONDecodeError:
            continue
        val = rec.get("settled_count")
        if isinstance(val, int):
            last = val
    return last


def generative_audit_staleness_actions(g: TensionGraph) -> list[Action]:
    """Canon: §Harness — CLI-only band: the generative tension audit is stale.

    RULE (R-tension-audit-staleness-visible): NOT part of diagnose(g). Like P6
    PENDING_PROPOSAL, this band reads the FILESYSTEM (spec/.runtime/tension-audit.jsonl),
    not the graph, so it is kept OUT of diagnose() — which gen_spec.py renders
    into the byte-stable LIVE-STATE (R-deterministic-generation) and which every
    graph-generic test exercises on synthetic graphs. Coupling a filesystem read
    into diagnose() would both break that determinism contract and pollute
    unrelated graph tests, so the staleness signal is surfaced ONLY by the
    interactive CLI (main(), below), exactly as pending_proposal_actions is.

    Fires ONE action on the 'generative-audit' meter when the audit has NEVER
    run (no stamp) or the live graph has grown by more than
    GENERATIVE_AUDIT_STALE_DELTA net-new SETTLED atoms since the last recorded
    sweep: run `python tools/audit_tensions.py` to re-sweep.

    WHY it exists: the framework holds tensions well but historically did not
    FIND them (0/8 machine-surfaced). Left to memory the first act of sight
    lapses as the graph grows; this makes "you have not swept lately" an
    addressable action rather than an invisible lapse.
    """
    now = sum(1 for r in g.requirements if r.status == "SETTLED")
    then = _last_audit_settled_count()
    if then is None:
        msg = (
            f"generative tension audit has NEVER run ({now} SETTLED atoms unswept)"
            " — run `python tools/audit_tensions.py` to surface"
            " unmediated-tension suspects (R-tension-audit-staleness-visible)."
        )
    elif now - then > GENERATIVE_AUDIT_STALE_DELTA:
        msg = (
            f"generative tension audit is stale: {now} SETTLED now vs {then} at"
            f" the last sweep (+{now - then} > {GENERATIVE_AUDIT_STALE_DELTA})"
            " — re-run `python tools/audit_tensions.py`"
            " (R-tension-audit-staleness-visible)."
        )
    else:
        return []
    return [
        Action(
            priority=P_PENDING_PROPOSAL,
            kind=_BAND_LABEL[P_PENDING_PROPOSAL],
            target="generative-audit",
            imperative=msg,
        )
    ]


REVISIT_EVAL_FILE = (
    _runtime_dir() / "revisit-eval.jsonl"
)
"""Canon: §Harness — append-only revisit-marker evaluation log written by
tools/mark_revisit_evaluated.py."""


def _last_revisit_evaluations() -> dict[str, int]:
    """Map each evaluated conflict id -> settled_count at its LAST evaluation."""
    out: dict[str, int] = {}
    if not REVISIT_EVAL_FILE.exists():
        return out
    for line in REVISIT_EVAL_FILE.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            rec = json.loads(line)
        except json.JSONDecodeError:
            continue
        cid = rec.get("conflict")
        sc = rec.get("settled_count")
        if isinstance(cid, str) and isinstance(sc, int):
            out[cid] = sc  # later lines overwrite -> last wins
    return out


def revisit_marker_actions(g: TensionGraph) -> list[Action]:
    """Canon: §Harness — CLI-only band: DECIDED revisit_markers awaiting evaluation.

    RULE: NOT part of diagnose(g). Like the pending-proposal and generative-audit
    bands, this reads the FILESYSTEM (spec/.runtime/revisit-eval.jsonl), which
    gen_spec must never render (R-deterministic-generation) and which graph-generic
    tests must not be polluted by, so the signal is surfaced ONLY at the CLI.

    For each Conflict carrying a non-empty revisit_marker, fire one action when
    the marker has NEVER been evaluated, or the live graph has grown by more than
    REVISIT_STALE_PERCENT % of SETTLED atoms since its last evaluation
    (mark_revisit_evaluated.py records the evaluation). Emitted in stable graph
    order, one line per stale marker.

    WHY percentage (not absolute): the old threshold (GENERATIVE_AUDIT_STALE_DELTA=10)
    re-fired every ~1.5 days at normal development pace (~8 atoms/day), causing
    attention noise. A 5% threshold scales with graph size: a 200-atom graph
    needs +10 new SETTLED atoms, a 400-atom graph needs +20. This keeps revisit
    signals meaningful instead of habitual.

    WHY: a revisit_marker names the CONDITION under which a DECIDED conflict
    should be re-opened, but nothing tracked whether anyone LOOKED again. An
    unread trigger lets the decision silently ossify while its stated revisit
    condition may already have come true (R-revisit-markers-evaluated).
    """
    now = sum(1 for r in g.requirements if r.status == "SETTLED")
    last_eval = _last_revisit_evaluations()
    # Percentage-based threshold: re-fire when growth exceeds REVISIT_STALE_PERCENT%
    # of current SETTLED count, with a floor of 10 atoms so very small graphs
    # don't re-fire on every single atom.
    threshold = max(10, now * REVISIT_STALE_PERCENT // 100)
    out: list[Action] = []
    for c in g.conflicts:
        if not c.revisit_marker:
            continue
        then = last_eval.get(c.id)
        if then is None:
            reason = "never evaluated"
        elif now - then > threshold:
            reason = f"last evaluated at {then} SETTLED, now {now} (+{now - then}, threshold {threshold})"
        else:
            continue
        out.append(
            Action(
                priority=P_PENDING_PROPOSAL,
                kind=_BAND_LABEL[P_PENDING_PROPOSAL],
                target=c.id,
                imperative=(
                    f"evaluate revisit marker ({reason}): {c.revisit_marker}"
                    " — then `python tools/mark_revisit_evaluated.py"
                    f" {c.id}` (R-revisit-markers-evaluated)."
                ),
            )
        )
    return out


def open_ticket_actions() -> list[Action]:
    """Canon: §Harness — CLI-only band: on-disk tickets awaiting work, by status.

    RULE (R-open-tickets-visible): NOT part of diagnose(g). Like the pending-
    proposal, generative-audit and revisit-marker bands, this reads the
    FILESYSTEM (tickets/ at the repo root), which gen_spec must never render
    (R-deterministic-generation) and which graph-generic tests must not be
    polluted by, so the signal is surfaced ONLY at the interactive CLI.

    Fires ONE action summarising the non-terminal ticket load (everything not in
    tickets/done/), broken down by status, so the steward's on-disk queue is
    visible in the same pulse as graph debt. Emits nothing when there is no open
    ticket.

    WHY: the steward's backlog moved out of the chat and onto disk (the ticket
    engine). Without a pulse band the queue would be invisible until someone
    remembered to `ls tickets/` — exactly the being-lost the harness exists to
    prevent (R-agent-never-lost).
    """
    _tools = Path(__file__).resolve().parent
    if str(_tools) not in sys.path:
        sys.path.insert(0, str(_tools))
    try:
        import _ticket_store as _ts  # noqa: PLC0415
    except Exception:
        return []
    counts = _ts.counts_by_status()
    open_total = sum(v for k, v in counts.items() if k != "done")
    if open_total == 0:
        return []
    breakdown = ", ".join(
        f"{k}: {counts[k]}" for k in _ts.STATUSES if k != "done" and counts[k]
    )
    return [
        Action(
            priority=P_PENDING_PROPOSAL,
            kind=_BAND_LABEL[P_PENDING_PROPOSAL],
            target="open-tickets",
            imperative=(
                f"open tickets: {open_total} ({breakdown})"
                " — `python tools/ticket_list.py` (R-open-tickets-visible)."
            ),
        )
    ]


# --- Runtime-fs attention sources (the injected superset half) --------------


def _actions_to_signals(source_id: str, actions: list[Action]) -> list[AttentionSignal]:
    """Adapt the legacy Action-returning fs-band functions into AttentionSignals
    tagged with their source id, so they can be injected into
    hotam_spec.attention.collect() as runtime-fs sources."""
    return [
        AttentionSignal(
            source=source_id,
            priority=a.priority,
            target=a.target,
            message=a.imperative,
        )
        for a in actions
    ]


def runtime_fs_sources() -> tuple[AttentionSource, ...]:
    """Canon: §Attention — the RUNTIME-FS half of the attention registry.

    These four sources read the FILESYSTEM (spec/.runtime/*, tickets/, pending
    proposals), so they are NON-deterministic and MUST NOT flow into diagnose()
    / LIVE-STATE. They live here in the tool (not in the stdlib-only core) and
    are INJECTED into hotam_spec.attention.collect(runtime_sources=...) by the
    live consumer (this CLI, the Claude hook, any agent). That injection seam is
    the architectural guarantee behind R-attention-superset-of-diagnose: the
    deterministic core has no reference to them, so they can never leak into the
    substrate.

    Each is tagged READS_RUNTIME_FS; collect() rejects any READS_GRAPH source
    passed as a runtime source.
    """
    return (
        AttentionSource(
            id="pending-proposals",
            reads=READS_RUNTIME_FS,
            collect=lambda g: _actions_to_signals(
                "pending-proposals", pending_proposal_actions()
            ),
        ),
        AttentionSource(
            id="generative-audit",
            reads=READS_RUNTIME_FS,
            collect=lambda g: _actions_to_signals(
                "generative-audit", generative_audit_staleness_actions(g)
            ),
        ),
        AttentionSource(
            id="revisit-markers",
            reads=READS_RUNTIME_FS,
            collect=lambda g: _actions_to_signals(
                "revisit-markers", revisit_marker_actions(g)
            ),
        ),
        AttentionSource(
            id="open-tickets",
            reads=READS_RUNTIME_FS,
            collect=lambda g: _actions_to_signals(
                "open-tickets", open_ticket_actions()
            ),
        ),
    )


def collect_attention(g: TensionGraph) -> list[AttentionSignal]:
    """Canon: §Attention — the LIVE superset for an agent/hook consumer.

    Runs the framework's deterministic graph diagnosis PLUS the injected
    runtime-fs bands — the full "pay attention here" list for a living agent.
    This is what tools/attention.py (the CLI) and tools/attention_hook.py (the
    Claude adapter) call. gen_spec / LIVE-STATE never call this — they call
    diagnose() (the deterministic subset) only.
    """
    return _attention.collect(g, runtime_sources=runtime_fs_sources())


# --- Rendering --------------------------------------------------------------

_EMPTY_GRAPH_BANNER = (
    "== what_now: no content yet ==\n"
    "\n"
    f"No domain content under {CONTENT_GRAPH_FILE} — the framework is blank.\n"
    "\n"
    "Populate a domain (see CLAUDE.md §How to populate):\n"
    "  1. create spec/content/graph.py exposing `build_graph() -> TensionGraph`;\n"
    "  2. declare at least one Stakeholder and one Requirement (and the axes\n"
    "     vocabulary this domain admits);\n"
    "  3. re-run me, then `tools/gen_spec.py`, then `pytest -q`.\n"
    "\n"
    "To see the worked example fixture instead, run me with `--demo`.\n"
)


DEFAULT_P5_LIMIT = 20
"""Canon: §Harness — default cap on printed P5 LATENT_CONNECTOR lines.

P5 is a HEURISTIC suspect list (share-a-specific-assumption), not a hard
diagnosis; on a large graph it can still run to dozens of entries. Capping
the printed list keeps `what_now` output scannable while an honest
disclosure line reports the count suppressed, so the cap never silently
hides debt (R-speak-by-reference)."""


def render(
    actions: list[Action],
    *,
    source_label: str = "content",
    p5_limit: int = DEFAULT_P5_LIMIT,
) -> str:
    """Render the action list as a deterministic, human-readable report (LF).

    `source_label` is shown in the header ('content' vs 'demo') so an agent
    reading the output instantly sees which graph was diagnosed. `p5_limit`
    caps the number of P5 LATENT_CONNECTOR lines printed; when truncated, an
    honest disclosure line names the count suppressed and how to see the
    full list (never silently drop debt — R-speak-by-reference).
    """
    lines: list[str] = []
    lines.append(f"== what_now: prioritized next actions ({source_label}) ==")
    lines.append("")
    if not actions:
        lines.append("No open actions. The tension graph is well-formed and every")
        lines.append("contradiction is visible, stewarded, and up to date.")
        return "\n".join(lines) + "\n"

    lines.append(f"{len(actions)} action(s). Lower priority number = more urgent.")
    lines.append("")
    current_band: int | None = None
    p5_printed = 0
    p5_total = sum(1 for a in actions if a.priority == P_LATENT_CONNECTOR)
    for a in actions:
        if a.priority == P_LATENT_CONNECTOR:
            if p5_printed >= p5_limit:
                continue
            p5_printed += 1
        if a.priority != current_band:
            current_band = a.priority
            lines.append(f"--- P{a.priority} {_BAND_LABEL[a.priority]} ---")
        lines.append(f"  [P{a.priority}] {a.target}: {a.imperative}")
    if p5_total > p5_limit:
        suppressed = p5_total - p5_limit
        lines.append(
            f"  ... {suppressed} more suppressed (run tools/audit_atomicity.py "
            "or increase --p5-limit for full list)"
        )
    lines.append("")
    lines.append(
        "Loop: pick the top action -> edit spec/content -> "
        "`python tools/gen_spec.py` -> `python -m pytest -q` -> re-run me."
    )
    return "\n".join(lines) + "\n"


# --- Entry point ------------------------------------------------------------


def _load_graph(*, demo: bool) -> tuple[TensionGraph, str]:
    """Return (graph, source_label) per the --demo flag.

    --demo loads the fixture seed (explicit opt-in to the example); default loads
    spec/content/ (the user's domain), which may be empty in a fresh framework.
    Delegates to the shared _graph_loader (R-shared-tools-in-spec-tools) —
    the same demo/content branch attention.py, audit_atomicity.py,
    confront.py and audit_tensions.py use.
    """
    from _graph_loader import load_graph_with_label  # noqa: PLC0415

    return load_graph_with_label(demo=demo)


def main(argv: list[str] | None = None) -> None:
    """Diagnose the configured graph and print the prioritized next-action list."""
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument(
        "--demo",
        action="store_true",
        help="diagnose the fixture demo graph instead of spec/content/.",
    )
    parser.add_argument(
        "--p5-limit",
        type=int,
        default=DEFAULT_P5_LIMIT,
        help=(
            "cap on printed P5 LATENT_CONNECTOR lines "
            f"(default {DEFAULT_P5_LIMIT}); truncation is disclosed, never silent."
        ),
    )
    parser.add_argument(
        "--report",
        action="store_true",
        help=(
            "print a single advisory Tick report instead of the full action "
            "list (formerly tools/tick.py): load, diagnose, classify, and "
            "surface the top action. Always exits 0 — advisory, not a gate."
        ),
    )
    parser.add_argument(
        "--cycle",
        type=int,
        default=1,
        help="cycle counter for --report (caller-supplied; default 1).",
    )
    args = parser.parse_args(argv)
    if args.report:
        report = tick(cycle=args.cycle)
        sys.stdout.write(render_tick(report))
        return
    g, label = _load_graph(demo=args.demo)
    if g.is_empty() and not args.demo:
        sys.stdout.write(_EMPTY_GRAPH_BANNER)
        return
    actions = diagnose(g)
    sys.stdout.write(render(actions, source_label=label, p5_limit=args.p5_limit))
    # P6 PENDING_PROPOSAL — CLI-only, filesystem-sourced, non-deterministic
    # (age-in-days); never fed into diagnose()/render() so generated docs
    # (which call diagnose() via gen_spec.py) stay byte-stable. See
    # pending_proposal_actions() docstring / R-presented-pending-decision-type.
    # Both bands below are CLI-only (filesystem-sourced, non-deterministic) and
    # are never fed into diagnose()/render() so generated docs stay byte-stable.
    cli_only = (
        pending_proposal_actions()
        + generative_audit_staleness_actions(g)
        + revisit_marker_actions(g)
        + open_ticket_actions()
    )
    if cli_only:
        sys.stdout.write(f"\n--- P{P_PENDING_PROPOSAL} PENDING_PROPOSAL ---\n")
        for a in cli_only:
            sys.stdout.write(f"  [P{a.priority}] {a.target}: {a.imperative}\n")


if __name__ == "__main__":
    main()
