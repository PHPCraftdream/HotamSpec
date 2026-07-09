"""Canon: §Reflection — the operator's P0 self-diagnosis conditions as named predicates.

RULE: every P0 REFLECTION condition the harness can raise MUST be a named,
pure, graph-only predicate in this module — draft-overhang, unenforced-settled,
over-budget-operators, dead-assumption-on-enforcer, derived-but-unbuilt,
implements-decay, replaces-edge-migration, all-members-rejected — composed by
tools/what_now.py via all_findings() in REFLECTION_PREDICATES order, never
re-inlined in tool code (R-reflection-predicates-first-class).

CONTRACT of each predicate: `reflect_*(graph) -> list[Finding]`. An EMPTY list
means the operator is ready on that condition. Each Finding names the offending
object id and an imperative message, so the harness (tools/what_now.py) turns
findings directly into P0 REFLECTION actions — the same shape §Invariants uses
for P1 STRUCTURE (check_* -> Violation -> action).

WHY a first-class module (mirror of §Invariants): the check_* layer diagnoses
the domain graph's structural form, but the operator's own readiness lived as
tool-inlined code — important-yet-invisible. Named predicates give each
self-diagnosis condition a stable, testable anchor and keep the harness a thin
renderer over substrate, for Findings exactly as for Violations.

WHY ranked P0 (above §Invariants P1 STRUCTURE): an operator that cannot see its
own state is worse than a malformed graph — self-diagnosis outranks domain
diagnosis (§Reflection, M35).

References:
  R-reflection-predicates-first-class — this module is that claim's body.
  R-crystallize-before-split / R-context-bounded-delegation — over-budget relief.
  R-stale-substrate — dead-assumption-on-enforcer is its live signal.
  R-working-vs-substrate-budget — the budget bounds the WORKING store only.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date as _date
from pathlib import Path

from hotam_spec.assumption import DEAD, IMPLEMENTS
from hotam_spec.conflict import DECIDED_PREFIX
from hotam_spec.graph import TensionGraph, requirement_by_id
from hotam_spec.requirement import DRAFT, ENFORCED, SETTLED

_REPO_ROOT = Path(__file__).resolve().parents[3]  # .../HotamSpec (mirrors invariants.py)

#: An IMPLEMENTS aspiration older than this (in days) without re-affirmation
#: fires the decay signal. 14 days = roughly two working weeks; short enough
#: that an aspiration forgotten between waves surfaces, long enough that a
#: wave-in-flight does not noise. The age is measured from created_at (or the
#: last decided_at transition that re-typed the assumption to IMPLEMENTS).
IMPLEMENTS_DECAY_DAYS = 14


@dataclass(frozen=True)
class Finding:
    """Canon: §Reflection — one operator self-diagnosis finding: which object, what to fix.

    Fields:
      condition  — the reflect_* predicate name that fired (the condition).
      target     — the object id to act on (Requirement/Operator id, or a
                   stable meter slug like 'burn-down' / 'enforcement-gradient').
      imperative — human-readable instruction, surfaced verbatim by the
                   harness as a P0 REFLECTION action.

    WHY a record (not a string): the harness needs `target` to build a typed,
    addressable next-action — the exact reason invariants.Violation is a
    record; `condition` anchors the finding to its predicate for tests.
    """

    condition: str
    target: str
    imperative: str


def graph_size(g: TensionGraph) -> int:
    """Canon: §Reflection — NODE_COUNT measure for the operator budget condition.

    RULE: size = |requirements| + |conflicts| + |assumptions|. This is the
    same NODE_COUNT metric check_operator_within_budget uses for operators
    whose context_budget.measure == NODE_COUNT (R-context-budget-rule) — the
    Reflection band reuses identical logic but surfaces it as a P0 advisory,
    not a P1 structural violation, so over-budget operators appear at the TOP
    of the action list before any structural noise. Operators measured by
    CRYSTAL_CHARS instead use _crystal_chars(), not this function — see
    reflect_over_budget_operators.
    """
    return len(g.requirements) + len(g.conflicts) + len(g.assumptions)


# ---------------------------------------------------------------------------
# The five self-diagnosis predicates (P0 REFLECTION band, §Reflection, M35)
# ---------------------------------------------------------------------------


def reflect_draft_overhang(g: TensionGraph) -> list[Finding]:
    """Canon: §Reflection — DRAFT-overhang: the burn-down meter (M35 SETTLED:DRAFT ratio).

    RULE: when at least one requirement is SETTLED and the DRAFT count reaches
    half the SETTLED count (draft_n >= settled_n / 2), fire ONE finding on the
    'burn-down' meter: promote DRAFTs toward ENFORCED before crystallizing
    more (R-crystallize-before-split, C-06e2d84e).

    WHY: a growing DRAFT pile is working knowledge wearing a requirement
    costume — claims minted faster than they are promoted. Past half the
    SETTLED mass the overhang itself becomes the top self-signal.
    """
    settled_n = sum(1 for r in g.requirements if r.status == SETTLED)
    draft_n = sum(1 for r in g.requirements if r.status == DRAFT)
    if settled_n > 0 and draft_n >= settled_n / 2:
        return [
            Finding(
                condition="reflect_draft_overhang",
                target="burn-down",
                imperative=(
                    f"DRAFT-overhang: {draft_n} DRAFT vs {settled_n} SETTLED"
                    " — promote DRAFTs toward ENFORCED before crystallizing"
                    " more (R-crystallize-before-split, C-06e2d84e)."
                ),
            )
        ]
    return []


def reflect_unenforced_settled(g: TensionGraph) -> list[Finding]:
    """Canon: §Reflection — UNENFORCED-SETTLED overhang: claimed but not guaranteed.

    RULE: when MORE THAN 5 SETTLED requirements are closeable debt
    (Requirement.is_closeable_debt(): ENFORCEABLE yet still PROSE/STRUCTURAL),
    fire ONE finding on the 'enforcement-gradient' meter pointing at
    docs/gen/UNENFORCED.md. INHERENTLY_PROSE requirements are honestly-labeled
    permanent discipline, not debt (R-enforceability-kind-declared).

    WHY a generous threshold (> 5): a handful of not-yet-enforced SETTLED
    claims is normal in-flight work; past that the soft context-debt
    compounds silently, so the meter surfaces it as a P0 self-signal.
    """
    n_unenforced = sum(
        1 for r in g.requirements if r.status == SETTLED and r.is_closeable_debt()
    )
    if n_unenforced > 5:
        return [
            Finding(
                condition="reflect_unenforced_settled",
                target="enforcement-gradient",
                imperative=(
                    f"{n_unenforced} SETTLED requirements are closeable debt"
                    " (ENFORCEABLE, still PROSE/STRUCTURAL)"
                    " — claimed but not guaranteed, soft context-debt."
                    " See docs/gen/UNENFORCED.md."
                ),
            )
        ]
    return []


def _crystal_chars() -> int:
    """Canon: §Reflection — CRYSTAL_CHARS measure: char length of root CLAUDE.md.

    RULE: mirrors invariants.check_operator_within_budget's CRYSTAL_CHARS
    branch exactly — the resident crystal (root CLAUDE.md) character count,
    or 0 if the file is absent (nothing resident yet; not a violation).
    """
    claude_md = _REPO_ROOT / "CLAUDE.md"
    return len(claude_md.read_text(encoding="utf-8")) if claude_md.exists() else 0


def reflect_over_budget_operators(g: TensionGraph) -> list[Finding]:
    """Canon: §Reflection — over-budget operators: crystallize first, then delegate.

    RULE: for each Operator whose context_budget.limit is positive, measure it
    by its OWN context_budget.measure — NODE_COUNT uses graph_size(g);
    CRYSTAL_CHARS uses the character length of the resident crystal (root
    CLAUDE.md) — exactly the same dispatch check_operator_within_budget uses
    (R-context-budget-rule). limit == 0 means unbounded (aspect off).

    WHY surfaced here as well as in check_operator_within_budget: the
    §Invariants form is a P1 structural violation; this predicate re-surfaces
    the same condition, measured the same way, as a P0 advisory so an
    over-budget operator appears at the TOP of the action list before any
    structural noise.
    """
    from hotam_spec.operator import CRYSTAL_CHARS, NODE_COUNT  # noqa: PLC0415

    node_size = graph_size(g)
    out: list[Finding] = []
    for op in g.operators:
        limit = op.context_budget.limit
        if limit <= 0:
            continue
        if op.context_budget.measure == CRYSTAL_CHARS:
            size = _crystal_chars()
            unit = "chars (CRYSTAL_CHARS measure)"
        else:
            size = node_size
            unit = "nodes (NODE_COUNT measure)"
        if size > limit:
            out.append(
                Finding(
                    condition="reflect_over_budget_operators",
                    target=op.id,
                    imperative=(
                        f"Operator '{op.id}' holds {size} {unit}"
                        f" > budget {limit}; crystallize first"
                        " (R-crystallize-before-split); if still over, delegate"
                        " a sub-domain (R-context-bounded-delegation)."
                    ),
                )
            )
    return out


def reflect_dead_assumption_on_enforcer(g: TensionGraph) -> list[Finding]:
    """Canon: §Reflection — DEAD-assumption-on-ENFORCER: the stale-substrate signal.

    RULE: for every ENFORCED requirement, for each of its assumptions whose
    status is DEAD, fire a finding on the requirement id (one per
    requirement-and-dead-assumption pair, in graph order): the enforcer may be
    enforcing a now-wrong premise (R-stale-substrate).

    WHY ENFORCED only: a PROSE/STRUCTURAL requirement resting on a dead
    assumption is ordinary P2 DRIFT_FALLOUT; an ENFORCED one has a live
    check/test actively guarding a premise that no longer holds — automation
    amplifying drift, which the operator must see first.
    """
    dead_ids = {a.id for a in g.assumptions if a.status == DEAD}
    out: list[Finding] = []
    if dead_ids:
        for r in g.requirements:
            if r.enforcement != ENFORCED:
                continue
            for aid in r.assumptions:
                if aid in dead_ids:
                    out.append(
                        Finding(
                            condition="reflect_dead_assumption_on_enforcer",
                            target=r.id,
                            imperative=(
                                f"R-stale-substrate signal: enforced requirement"
                                f" '{r.id}' rests on DEAD assumption '{aid}';"
                                " its enforcer may be enforcing a now-wrong premise."
                            ),
                        )
                    )
    return out


def reflect_derived_but_unbuilt(g: TensionGraph) -> list[Finding]:
    """Canon: §Reflection — derived-but-unbuilt: a decision recorded, its offspring never built.

    RULE: for each Conflict in DECIDED(...) lifecycle, for each id in its
    `derived` tuple that is absent from the graph or still DRAFT, fire a
    finding on the derived id — derived-but-unbuilt debt.

    WHY: a DECIDED conflict justifies itself partly through what it spawned
    (R-decided-conflict-justifies-itself); a derived requirement left
    DRAFT/absent means the decision's promised follow-through silently never
    landed — debt the operator, not the domain, owns.
    """
    draft_ids = {r.id for r in g.requirements if r.status == DRAFT}
    out: list[Finding] = []
    for c in g.conflicts:
        if not c.lifecycle.startswith(DECIDED_PREFIX):
            continue
        for derived_id in c.derived:
            derived_req = requirement_by_id(g, derived_id)
            if derived_req is None or derived_req.id in draft_ids:
                out.append(
                    Finding(
                        condition="reflect_derived_but_unbuilt",
                        target=derived_id,
                        imperative=(
                            f"DECIDED conflict '{c.id}' spawned '{derived_id}'"
                            " but it remains DRAFT/unbuilt"
                            " — derived-but-unbuilt debt."
                        ),
                    )
                )
    return out


def reflect_implements_decay(g: TensionGraph) -> list[Finding]:
    """Canon: §Reflection — IMPLEMENTS-decay: an aspiration aging without progress.

    RULE: for each Assumption whose status is IMPLEMENTS, if its age (measured
    from decided_at if known, else from created_at) exceeds IMPLEMENTS_DECAY_DAYS
    days, fire ONE finding per assumption: 're-affirm or downgrade'. The signal
    is advisory (P0 REFLECTION band surfaced via what_now), NEVER a gate/blocker.

    WHY IMPLEMENTS is the dangerous quiet corner (Ontology K2(c)): an aspiration
    by construction raises no UNCERTAIN-aging doubt and no DEAD-fallout — it is
    the legal way to record a striving and forget it forever. The two largest
    live assumptions (A-bootstrap-self-applies, A-most-knowledge-crystallizable)
    are IMPLEMENTS; without this predicate they are permanently invisible. The
    decay predicate restores honest aging: 'you wanted this N days ago; is it
    still a live striving, or has it silently become a dead hope?'.

    HONEST UNKNOWN SEMANTICS: an IMPLEMENTS assumption with NO known date (both
    decided_at and created_at are "") is a LEGACY node predating the timestamp
    layer — it has no honest age. Such a node MUST NOT fire (no false noise on
    the ~290 pre-timestamp nodes). The predicate only fires when an age is
    computable. decided_at (the last transition into IMPLEMENTS) takes
    precedence over created_at: re-typing an assumption to IMPLEMENTS resets
    the decay clock, so an aspiration actively worked on never ages out.
    """
    today = _date.today()
    out: list[Finding] = []
    for a in g.assumptions:
        if a.status != IMPLEMENTS:
            continue
        # Prefer decided_at (the last transition into IMPLEMENTS); fall back to
        # created_at. Both must be ISO YYYY-MM-DD or empty.
        stamp_str = a.decided_at or a.created_at
        if not stamp_str:
            continue  # unknown date — no honest age, do NOT fire
        try:
            stamp = _date.fromisoformat(stamp_str)
        except ValueError:
            continue  # malformed date — do NOT fire (defensive; never lie)
        age_days = (today - stamp).days
        if age_days > IMPLEMENTS_DECAY_DAYS:
            out.append(
                Finding(
                    condition="reflect_implements_decay",
                    target=a.id,
                    imperative=(
                        f"IMPLEMENTS aspiration '{a.id}' is {age_days} days old"
                        f" (last stamped {stamp_str}) without re-affirmation"
                        " — re-affirm (transition to HOLDS if achieved) or"
                        " downgrade (to DEAD if abandoned). An aspiration that"
                        " ages silently is the invisible corner IMPLEMENTS"
                        " created (Ontology K2(c))."
                    ),
                )
            )
    return out


# Marker reused from gen_spec.py's regex (em-dash, en-dash, --, -).
_REPLACES_PROSE_RE = __import__("re").compile(r"REJECTED\s*(?:—|–|--|-)\s*REPLACES")


def reflect_all_members_rejected(g: TensionGraph) -> list[Finding]:
    """Canon: §Reflection — all-members-REJECTED: a live conflict whose every party is dead.

    RULE: for each Conflict that is NOT itself in a terminal/archival state
    (its lifecycle does NOT start with DECIDED — the resolved state — and is
    NOT REVISIT_WHEN — the parked state), if EVERY one of its members is a
    REJECTED Requirement, fire ONE advisory finding on the conflict id: the
    tension's parties are all gone, yet the conflict is neither resolved nor
    parked. The steward should DECIDE it (mark the tension exhausted) or
    REVISIT_WHEN (park it) — a live DETECTED/ACKNOWLEDGED conflict with two
    dead members is a structural ghost (C-c3911f28 is the live example: both
    members are REJECTED, yet it was recorded as DECIDED, so it stays silent).

    WHY ADVISORY, not a hard invariant: the methodology holds that a conflict
    is closed only by a steward, never silently (R-decided-needs-human-signoff).
    A hard invariant firing on 'all members REJECTED' would either (a) force the
    conflict into a terminal state automatically (violating the hard boundary)
    or (b) block the graph green until the steward acts (holding the whole
    domain hostage to one historical node). The reflection signal instead
    HONESTLY SURFACES the ghost so the steward can decide its fate, without
    blocking. C-c3911f28 is the canonical case: a DECIDED conflict between two
    REJECTED requirements, both superseded — it is a legitimate 'tension
    exhausted' record, and a hard check would misfire on it. By excluding
    DECIDED/REVISIT_WHEN (the terminal/archival states) from the trigger, the
    predicate fires ONLY on the genuinely-ghosty DETECTED/ACKNOWLEDGED case.

    WHY members are resolved defensively: a dangling member id (not a known
    Requirement) is itself a P1 structural violation (check_no_dangling_conflict_
    refs); here we treat an unresolvable member as NOT-REJECTED so the predicate
    does not double-report a dangling id as a ghost. The structural check owns
    the dangle; this predicate owns the all-dead-but-live signal.
    """
    out: list[Finding] = []
    for c in g.conflicts:
        # Terminal/archival states: DECIDED (resolved) and REVISIT_WHEN (parked)
        # are steward outcomes — a conflict there with dead members is a resolved
        # historical record, not a ghost. Only DETECTED/ACKNOWLEDGED can ghost.
        if c.lifecycle.startswith(DECIDED_PREFIX):
            continue
        if c.lifecycle.startswith("REVISIT_WHEN"):
            continue
        if len(c.members) < 1:
            continue  # min-two-members is a separate structural check
        # Resolve members; treat unresolvable as NOT-rejected (don't double-report).
        statuses: list[str] = []
        for mid in c.members:
            r = requirement_by_id(g, mid)
            if r is None:
                statuses.append("UNKNOWN")
            else:
                statuses.append(r.status)
        if statuses and all(s == "REJECTED" for s in statuses):
            out.append(
                Finding(
                    condition="reflect_all_members_rejected",
                    target=c.id,
                    imperative=(
                        f"Conflict '{c.id}' is live ({c.lifecycle}) but ALL its"
                        f" members are REJECTED ({list(c.members)}). The tension's"
                        " parties are gone; DECIDE it (mark exhausted) or"
                        " REVISIT_WHEN (park) so the graph stops holding a ghost"
                        " connector. Advisory; never a gate (the steward closes a"
                        " conflict, never the harness — R-decided-needs-human-signoff)."
                    ),
                )
            )
    return out


def reflect_replaces_edge_migration(g: TensionGraph) -> list[Finding]:
    """Canon: §Reflection — replaces-edge-migration: REJECTED prose without a structural edge.

    RULE: for each REJECTED Requirement whose `why` contains a prose
    'REJECTED <dash> REPLACES' marker but which is NOT the target of any
    structural `replaces` Relation edge in the graph, fire ONE advisory finding
    (P0 REFLECTION band surfaced via what_now, NEVER a gate). The finding tells
    the operator to migrate that historical rejection onto a structural edge so
    the anti-relitigation relation becomes machine-traversable.

    WHY advisory and not a gate: the ~38 historical REJECTED nodes predate the
    structural replaces edge (introduced in the K1 ontology wave). Migrating them
    is a steward act (each requires confirming the successor and writing the edge
    via apply_proposal); forcing it as a hard invariant would block the graph
    until all 38 are hand-migrated. The predicate instead HONESTLY SURFACES the
    not-yet-migrated set so a steward can work through it incrementally
    (R-reflection-predicates-first-class — important-yet-invisible as a named
    predicate, never silently extinguished). Once an edge is added, the finding
    for that id goes silent — the migration ratchet only ever shrinks.

    WHY the prose marker is the trigger (not just any REJECTED): a REJECTED
    requirement with NO 'REPLACES' marker is an honest 'discarded, no successor'
    node — it has nothing to migrate. Only nodes that ALREADY CLAIM a replacement
    in prose but lack the structural twin are migration candidates.
    """
    from hotam_spec.graph import replaces_map  # noqa: PLC0415

    rmap = replaces_map(g)
    out: list[Finding] = []
    for r in g.requirements:
        if r.status != "REJECTED":
            continue
        if r.id in rmap:
            continue  # already has a structural replaces edge
        if not _REPLACES_PROSE_RE.search(r.why):
            continue  # no prose marker — not a migration candidate
        out.append(
            Finding(
                condition="reflect_replaces_edge_migration",
                target=r.id,
                imperative=(
                    f"REJECTED requirement '{r.id}' claims a REPLACES successor in"
                    " prose but has NO structural `replaces` edge — migrate it via"
                    " a ProposedRejection (with replaced_by) so the anti-relitigation"
                    " relation becomes machine-traversable"
                    " (R-rejected-preserved-not-deleted). Advisory; never a gate."
                ),
            )
        )
    return out


# ---------------------------------------------------------------------------
# Registry + single entry point (mirror of invariants.ALL_INVARIANTS)
# ---------------------------------------------------------------------------

REFLECTION_PREDICATES = (
    reflect_draft_overhang,
    reflect_unenforced_settled,
    reflect_over_budget_operators,
    reflect_dead_assumption_on_enforcer,
    reflect_derived_but_unbuilt,
    reflect_implements_decay,
    reflect_replaces_edge_migration,
    reflect_all_members_rejected,
)


def all_findings(g: TensionGraph) -> list[Finding]:
    """Canon: §Reflection — run every reflection predicate, in registry order.

    RULE: the harness's P0 REFLECTION band is exactly this list — one entry
    point so tests, the harness and any future gate read the same predicates
    in the same order (determinism; mirror of invariants.all_violations).

    WHY one entry point: a tool composing predicates piecemeal could silently
    drop a condition; consuming the registry whole makes omission structurally
    visible (the registry and the band cannot drift apart).
    """
    out: list[Finding] = []
    for predicate in REFLECTION_PREDICATES:
        out.extend(predicate(g))
    return out
