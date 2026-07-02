"""Canon: §Reflection — the operator's P0 self-diagnosis conditions as named predicates.

RULE: every P0 REFLECTION condition the harness can raise MUST be a named,
pure, graph-only predicate in this module — draft-overhang, unenforced-settled,
over-budget-operators, dead-assumption-on-enforcer, derived-but-unbuilt —
composed by tools/what_now.py via all_findings() in REFLECTION_PREDICATES
order, never re-inlined in tool code (R-reflection-predicates-first-class).

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
from pathlib import Path

from hotam_spec.assumption import DEAD
from hotam_spec.conflict import DECIDED_PREFIX
from hotam_spec.graph import TensionGraph, requirement_by_id
from hotam_spec.requirement import DRAFT, ENFORCED, SETTLED

_REPO_ROOT = Path(__file__).resolve().parents[3]  # .../HotamSpec (mirrors invariants.py)


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


# ---------------------------------------------------------------------------
# Registry + single entry point (mirror of invariants.ALL_INVARIANTS)
# ---------------------------------------------------------------------------

REFLECTION_PREDICATES = (
    reflect_draft_overhang,
    reflect_unenforced_settled,
    reflect_over_budget_operators,
    reflect_dead_assumption_on_enforcer,
    reflect_derived_but_unbuilt,
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
