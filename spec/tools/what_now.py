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

Run:
  uv run python tools/what_now.py            # diagnose spec/content/ (your domain)
  uv run python tools/what_now.py --demo     # diagnose the fixture demo graph

Dependency-light (stdlib + the hotam_spec package). Deterministic ordering.
"""

from __future__ import annotations

import argparse
import sys
from dataclasses import dataclass
from pathlib import Path

# hotam_spec lives in spec/src; make it importable whether run via uv or plain python.
_SRC = Path(__file__).resolve().parents[1] / "src"
if str(_SRC) not in sys.path:
    sys.path.insert(0, str(_SRC))

from hotam_spec.conflict import ACKNOWLEDGED, DETECTED  # noqa: E402
from hotam_spec.graph import (  # noqa: E402
    CONTENT_GRAPH_FILE,
    TensionGraph,
    conflicts_on_assumption,
    dead_assumptions,
    entity_state_conflict_suspects,
    latent_connector_clusters,
    load_content_graph,
    requirements_on_assumption,
)
from hotam_spec.invariants import all_violations  # noqa: E402
from hotam_spec.reflection import all_findings  # noqa: E402

# --- Priority bands (lower number = more urgent) ----------------------------

P_REFLECTION = 0  # highest — operator self-readiness ranked above structural form
P_STRUCTURE = 1
P_DRIFT_FALLOUT = 2
P_CONFLICT_STALLED = 3
P_OPEN_ITEM = 4
P_LATENT_CONNECTOR = 5

_BAND_LABEL = {
    P_REFLECTION: "REFLECTION",
    P_STRUCTURE: "STRUCTURE",
    P_DRIFT_FALLOUT: "DRIFT_FALLOUT",
    P_CONFLICT_STALLED: "CONFLICT_STALLED",
    P_OPEN_ITEM: "OPEN_ITEM",
    P_LATENT_CONNECTOR: "LATENT_CONNECTOR",
}


@dataclass(frozen=True)
class Action:
    """One typed, addressable next-action the agent can take.

    Fields:
      priority — band (0..5); lower is more urgent. P0=REFLECTION is the
                 operator self-readiness band; P1..P5 are domain diagnosis.
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


def diagnose(g: TensionGraph) -> list[Action]:
    """Derive the full prioritized next-action list from a graph state.

    Deterministic: actions are emitted band by band, and within a band in stable
    graph/id order, then a final stable sort by (priority, kind, target) fixes
    the global order. Running twice on the same graph yields the same list.
    """
    actions: list[Action] = []

    # P0 REFLECTION — operator self-readiness (§Reflection, M35).
    # The five conditions live as named predicates in hotam_spec.reflection
    # (R-reflection-predicates-first-class); the harness maps each Finding
    # into a P0 Action exactly as it maps invariants Violations into P1 below.
    for f in all_findings(g):
        actions.append(
            Action(
                priority=P_REFLECTION,
                kind=_BAND_LABEL[P_REFLECTION],
                target=f.target,
                imperative=f.imperative,
            )
        )

    # P1 STRUCTURE — failing structural invariants.
    for v in all_violations(g):
        actions.append(
            Action(
                priority=P_STRUCTURE,
                kind=_BAND_LABEL[P_STRUCTURE],
                target=v.target,
                imperative=f"[{v.invariant}] {v.message}",
            )
        )

    # P2 DRIFT_FALLOUT — DEAD assumptions with live dependents.
    for a in dead_assumptions(g):
        dep_reqs = requirements_on_assumption(g, a.id)
        dep_cons = conflicts_on_assumption(g, a.id)
        if not dep_reqs and not dep_cons:
            continue  # a dead assumption with no dependents needs no revisit
        for r in dep_reqs:
            actions.append(
                Action(
                    priority=P_DRIFT_FALLOUT,
                    kind=_BAND_LABEL[P_DRIFT_FALLOUT],
                    target=r.id,
                    imperative=(
                        f"assumption '{a.id}' is DEAD ({a.statement!r}); "
                        f"revisit requirement '{r.id}' which rests on it"
                    ),
                )
            )
        for c in dep_cons:
            actions.append(
                Action(
                    priority=P_DRIFT_FALLOUT,
                    kind=_BAND_LABEL[P_DRIFT_FALLOUT],
                    target=c.id,
                    imperative=(
                        f"assumption '{a.id}' is DEAD; revive conflict cluster "
                        f"'{c.id}' whose shared_assumption was '{a.id}'"
                    ),
                )
            )

    # P3 CONFLICT_STALLED — conflicts with no steward resolution.
    for c in g.conflicts:
        if c.lifecycle == DETECTED:
            actions.append(
                Action(
                    priority=P_CONFLICT_STALLED,
                    kind=_BAND_LABEL[P_CONFLICT_STALLED],
                    target=c.id,
                    imperative=(
                        f"conflict '{c.id}' on axis '{c.axis}' is DETECTED with no "
                        f"steward movement; steward '{c.steward}' must ACKNOWLEDGE it"
                    ),
                )
            )
        elif c.lifecycle == ACKNOWLEDGED:
            actions.append(
                Action(
                    priority=P_CONFLICT_STALLED,
                    kind=_BAND_LABEL[P_CONFLICT_STALLED],
                    target=c.id,
                    imperative=(
                        f"conflict '{c.id}' is ACKNOWLEDGED but undecided; steward "
                        f"'{c.steward}' must DECIDE (rationale) or set REVISIT_WHEN"
                    ),
                )
            )

    # P4 OPEN_ITEM — OPEN(question) requirements.
    for r in g.requirements:
        if r.is_open():
            question = r.status[len("OPEN") :].strip().strip("()").strip()
            actions.append(
                Action(
                    priority=P_OPEN_ITEM,
                    kind=_BAND_LABEL[P_OPEN_ITEM],
                    target=r.id,
                    imperative=(
                        f"OPEN requirement '{r.id}' (owner '{r.owner}') awaits a "
                        f"decision: {question or '(no question stated)'}"
                    ),
                )
            )

    # P5 LATENT_CONNECTOR — heuristic missing-connector suspects (for AI
    # review), CLUSTERED by shared-assumption signature: one action per
    # cluster, not C(n,2) pair lines (R-latent-connectors-cluster-by-assumption;
    # pair-level detail lives in latent_connector_clusters(...).pairs and in
    # the TENSIONS.md suspect table).
    for cl in latent_connector_clusters(g):
        sig = ", ".join(cl.assumptions)
        actions.append(
            Action(
                priority=P_LATENT_CONNECTOR,
                kind=_BAND_LABEL[P_LATENT_CONNECTOR],
                target=",".join(cl.assumptions),
                imperative=(
                    f"[HEURISTIC, for AI review] assumption(s) {sig} shared by "
                    f"{len(cl.requirements)} requirements "
                    f"({', '.join(cl.requirements)}) with no mediating Conflict "
                    f"node — review the cluster as ONE item: consider splitting "
                    f"the assumption or materializing a connector "
                    f"({len(cl.pairs)} pair(s); detail: docs/gen/TENSIONS.md)"
                ),
            )
        )

    for s in entity_state_conflict_suspects(g):
        actions.append(
            Action(
                priority=P_LATENT_CONNECTOR,
                kind=_BAND_LABEL[P_LATENT_CONNECTOR],
                target=f"{s.left}~{s.right}",
                imperative=(f"[HEURISTIC, entity-state conflict] {s.hint}"),
            )
        )

    actions.sort(key=lambda a: (a.priority, a.kind, a.target, a.imperative))
    return actions


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
        "`uv run python tools/gen_spec.py` -> `uv run pytest -q` -> re-run me."
    )
    return "\n".join(lines) + "\n"


# --- Entry point ------------------------------------------------------------


def _load_graph(*, demo: bool) -> tuple[TensionGraph, str]:
    """Return (graph, source_label) per the --demo flag.

    --demo loads the fixture seed (explicit opt-in to the example); default loads
    spec/content/ (the user's domain), which may be empty in a fresh framework.
    """
    if demo:
        # Fixture lives outside src; add spec/tests to the import path.
        tests_dir = str(Path(__file__).resolve().parents[1] / "tests")
        if tests_dir not in sys.path:
            sys.path.insert(0, tests_dir)
        from fixtures.seed import seed_graph  # noqa: PLC0415

        return seed_graph(), "demo fixture"
    return load_content_graph(), "content"


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
    args = parser.parse_args(argv)
    g, label = _load_graph(demo=args.demo)
    if g.is_empty() and not args.demo:
        sys.stdout.write(_EMPTY_GRAPH_BANNER)
        return
    actions = diagnose(g)
    sys.stdout.write(render(actions, source_label=label, p5_limit=args.p5_limit))


if __name__ == "__main__":
    main()
