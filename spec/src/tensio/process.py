"""Canon: §Process — opt-in behavioral aspect (M12).

A Process is a Lifecycle + ordered Steps + the roles it requires + the
entities it drives. It is the richest contradiction surface (M12 /
R-process-aspect-first) because:
  - two processes driving one entity along incompatible state paths is
    the canonical hidden contradiction;
  - a step requiring a role no actor provides is a structural dead-end;
  - a method postcondition that violates an entity invariant is a real
    conflict surfaced as a Conflict node on a behavioral axis.

Entity is DEFERRED to a future aspect (M12); Process declares
`drives_entities: tuple[str, ...]` as forward-compat string references so
the Process aspect can ship before Entity. When Entity lands, the
invariant `check_process_drives_existing_entities` will activate (today
it is a no-op because g has no entities collection yet).

Goal is its OWN first-class type (M19): a target-state predicate + what it
targets. Distinct from a static Requirement claim because it carries a
MOVING TARGET that yields a Gap.

Goal CONFLICTS reuse the existing Conflict connector node on a goal-tension
axis (M23) — no new GoalConflict type. Keep "conflict is one node type".

References:
  R-process-aspect-first — this module IS the first behavioral aspect.
  R-goal-as-target-state — §Goal ships here as its own type (M19).
  R-statemachine-wellformedness — PROCESS_LIFECYCLE and GOAL_LIFECYCLE
    validate via check_lifecycle_wellformed (same keystone path as
    REQUIREMENT_STATUS_LIFECYCLE and CONFLICT_LIFECYCLE).
  R-task-vs-action-distinct-altitudes — Process.steps use prose `invokes`
    (forward-compat); the harness Action type is NOT merged here.
"""

from __future__ import annotations

from dataclasses import dataclass, field

from tensio.lifecycle import INITIAL, NORMAL, QUIESCENT, Lifecycle, State, Transition


# ---------------------------------------------------------------------------
# §Process — Step + Process dataclasses
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class Step:
    """Canon: §Process — one step of a Process, with the role it requires.

    RULE: a Step's `requires_role` MUST be in its Process.roles_required;
    this is enforced by check_process_roles_declared (no implicit role).
    A Step's `invokes` field is a forward-compat prose reference to an
    entity method (Entity aspect not yet shipped); today it is prose-only
    (R-task-vs-action-distinct-altitudes).

    WHY requires_role on each Step (not on the Process as a whole): a
    Process may need different roles at different steps — e.g. the operator
    diagnoses but the steward approves. Capturing the per-step demand lets
    the invariant check_process_roles_declared verify that every demanded
    role is actually declared in Process.roles_required (supply ≥ demand).
    """

    name: str  # the step slug (e.g. "validate", "approve", "fulfill")
    requires_role: str  # one of Process.roles_required
    invokes: str = ""  # forward-compat: "entity-slug.method" (prose for now)
    why: str = ""


@dataclass(frozen=True)
class Process:
    """Canon: §Process — a state-machine + ordered steps (opt-in behavioral aspect, M12).

    RULE: Process.id MUST start with 'PR-' (typed-anchor discipline,
    enforced by check_typed_anchors). Process.lifecycle MUST pass
    check_lifecycle_wellformed (enforced by check_process_lifecycle_wellformed).
    Every Step.requires_role MUST be in Process.roles_required (enforced by
    check_process_roles_declared — no implicit role).

    Fields:
      id              — typed anchor "PR-…".
      lifecycle       — the Process's own Lifecycle (uses the §Lifecycle
                        keystone; no parallel state machinery).
      steps           — ordered tuple of Step (the procedure's verbs).
      roles_required  — the union of all Step.requires_role values; must
                        be the actual roles the process needs. Listed
                        explicitly so check_process_roles_declared can
                        match supply against demand.
      drives_entities — forward-compat string refs (Entity not shipped yet).
                        When Entity aspect lands, these will be checked.
      why             — anti-relitigation prose.

    WHY opt-in aspect (M12): Lifecycle is core (P1, shipped); Process is
    the first OPT-IN behavioral surface. Domains that do not need process
    modeling pay nothing — TensionGraph.processes defaults to empty tuple.
    """

    id: str
    lifecycle: Lifecycle
    steps: tuple[Step, ...] = field(default_factory=tuple)
    roles_required: tuple[str, ...] = field(default_factory=tuple)
    drives_entities: tuple[str, ...] = field(default_factory=tuple)
    why: str = ""


# ---------------------------------------------------------------------------
# §Goal / §TargetState (M19: Goal is its own first-class type)
# ---------------------------------------------------------------------------

TARGET_KIND_GRAPH_PROPERTY = "GRAPH_PROPERTY"
TARGET_KIND_BUSINESS_STATE = "BUSINESS_STATE"
TARGET_KIND_ENTITY_STATE = "ENTITY_STATE"  # forward-compat (Entity not shipped)
TARGET_KINDS: frozenset[str] = frozenset(
    {
        TARGET_KIND_GRAPH_PROPERTY,
        TARGET_KIND_BUSINESS_STATE,
        TARGET_KIND_ENTITY_STATE,
    }
)


@dataclass(frozen=True)
class TargetState:
    """Canon: §Goal — a desired-state PREDICATE + what it targets.

    Canon: §Goal — Target state: the desired-state predicate carried by a
    Goal (kind in TARGET_KINDS). This IS the glossary's 'Target state' entry
    made structural — a kind discriminant + predicate + optional named target.

    RULE: kind MUST be in TARGET_KINDS; predicate is prose for now
    (machine-checkable later, like Assumption.machine_check). The `target`
    field names the graph property, business-state predicate, or (forward-
    compat) entity id being targeted.

    WHY a separate TargetState (not a plain string on Goal): the kind
    discriminant lets the invariant check_goal_target_kind_known verify the
    type vocabulary without parsing the predicate. It also future-proofs the
    Goal for machine-checkable predicates — the same seam as
    Assumption.machine_check.
    """

    kind: str  # one of TARGET_KINDS
    predicate: str  # prose or machine-checkable predicate
    target: str = ""  # optional: the named entity/graph-property targeted


@dataclass(frozen=True)
class Goal:
    """Canon: §Goal — a target the operator pursues (M19, distinct from §Requirement).

    RULE: Goal.id MUST start with 'GOAL-' (typed-anchor discipline, enforced
    by check_typed_anchors). Goal.owner MUST be a known Operator.id (enforced
    by check_goal_owner_is_operator). Goal.target_state.kind MUST be in
    TARGET_KINDS (enforced by check_goal_target_kind_known). Goal.lifecycle
    MUST match GOAL_LIFECYCLE (enforced by check_status_in_lifecycle).

    Fields:
      id           — typed anchor "GOAL-…".
      owner        — Operator.id pursuing this goal.
      target_state — the desired state predicate + what it targets.
      lifecycle    — Goal lifecycle (ACTIVE/MET/ABANDONED) using §Lifecycle.
      why          — rationale (anti-relitigation).

    WHY a separate type from §Requirement (M19): a Requirement is what the
    system SHALL satisfy (static claim). A Goal is the MOVING TARGET that
    gives a Process direction and yields a measurable Gap = (target_state -
    current state). Conflating them loses the Gap — the very distance that
    tells the Process what work remains.

    WHY Goal CONFLICTS reuse Conflict (M23): a Goal conflict is two Goals with
    incompatible TargetStates over the same target. The existing Conflict node
    handles this cleanly on a goal-tension axis — no new GoalConflict type.
    Keep "conflict is one node type" (R-conflict-is-connector-node).
    """

    id: str
    owner: str  # Operator.id pursuing this goal
    target_state: TargetState
    lifecycle: str = "ACTIVE"
    why: str = ""


# ---------------------------------------------------------------------------
# Canonical lifecycles (using the §Lifecycle keystone)
# ---------------------------------------------------------------------------

PROCESS_LIFECYCLE = Lifecycle(
    slug="process-lifecycle",
    states=(
        State("READY", kind=INITIAL, why="Process declared; not yet running."),
        State("RUNNING", kind=NORMAL, why="Currently executing a step."),
        State("BLOCKED", kind=NORMAL, why="Awaiting a role or capability."),
        State("DONE", kind=QUIESCENT, why="All steps completed successfully."),
        State("ABANDONED", kind=QUIESCENT, why="Steward retired the process."),
    ),
    transitions=(
        Transition("READY", "RUNNING", event="start", why="Process begins execution."),
        Transition(
            "RUNNING", "BLOCKED", event="block", why="Step cannot proceed; waiting."
        ),
        Transition("RUNNING", "DONE", event="complete", why="All steps finished."),
        Transition(
            "RUNNING", "ABANDONED", event="abandon", why="Steward retires the process."
        ),
        Transition(
            "BLOCKED", "RUNNING", event="unblock", why="Blocker resolved; resume."
        ),
        Transition(
            "BLOCKED",
            "ABANDONED",
            event="abandon",
            why="Steward retires while blocked.",
        ),
    ),
    cyclic=False,
)

GOAL_LIFECYCLE = Lifecycle(
    slug="goal-lifecycle",
    states=(
        State("ACTIVE", kind=INITIAL, why="Operator actively pursuing this target."),
        State("MET", kind=QUIESCENT, why="Target_state.predicate now holds."),
        State("ABANDONED", kind=QUIESCENT, why="Steward retired the goal."),
    ),
    transitions=(
        Transition(
            "ACTIVE", "MET", event="target-reached", why="The predicate now holds."
        ),
        Transition(
            "ACTIVE", "ABANDONED", event="abandon", why="Steward retires the goal."
        ),
    ),
    cyclic=False,
)
