"""Canon: §Lifecycle — the generic state-machine value-type (framework keystone).

RULE: every modeled state machine (Requirement.status, Conflict.lifecycle,
and future Operator/Process lifecycles) MUST validate against a framework-
supplied Lifecycle constant via invariants.check_status_in_lifecycle.

Canon: §Lifecycle — this module ships the SHAPE (State / Transition / Lifecycle)
plus the two canonical framework instances (REQUIREMENT_STATUS_LIFECYCLE and
CONFLICT_LIFECYCLE). It is content-free: no business data lives here.

References:
  R-lifecycle-abstraction — introduces this keystone abstraction (DRY the two
    hand-rolled state machines; opens the door to behavioral aspects).
  R-statemachine-wellformedness — every modeled state machine is reachable,
    deterministic, and terminal (or explicitly cyclic); transition guards may
    rest on an Assumption (the behavioral drift seam).

WHY one module, two constants: Requirement.status and Conflict.lifecycle are
BOTH hand-rolled prefix-test state machines with the same shape. Generalizing
them into Lifecycle:
  - makes the stored strings the single source of truth (no parallel storage),
  - adds a structural invariant that validates stored values against canonical
    states without changing how they are stored or compared,
  - and establishes the keystone so that Operator.lifecycle / Goal.status /
    Process.lifecycle in later phases can reuse it — no parallel machinery.
"""

from __future__ import annotations

from dataclasses import dataclass, field

# ---------------------------------------------------------------------------
# State-kind constants
# ---------------------------------------------------------------------------

INITIAL = "initial"
NORMAL = "normal"
TERMINAL = "terminal"
QUIESCENT = "quiescent"  # decision-recorded; may re-open via REVISIT_WHEN

STATE_KINDS: frozenset[str] = frozenset({INITIAL, NORMAL, TERMINAL, QUIESCENT})


# ---------------------------------------------------------------------------
# Core value-types
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class State:
    """Canon: §Lifecycle — one named state in a Lifecycle.

    RULE: `kind` MUST be one of STATE_KINDS (initial | normal | terminal |
    quiescent). A Lifecycle MUST contain exactly one INITIAL state and,
    unless cyclic=True, at least one terminal/quiescent state reachable from
    the initial state.

    WHY quiescent vs terminal: DECIDED and REVISIT_WHEN are stable resting
    points with a recorded rationale — they are quiescent, not dead. They
    can be re-opened (the cyclic flag and a condition-fires transition cover
    this), whereas terminal states are truly final.
    """

    name: str
    kind: str = NORMAL  # one of STATE_KINDS
    why: str = ""

    def is_initial(self) -> bool:
        """Canon: §Lifecycle — True iff this state is the INITIAL state."""
        return self.kind == INITIAL

    def is_terminal(self) -> bool:
        """Canon: §Lifecycle — True iff this state is terminal or quiescent (a resting point)."""
        return self.kind in (TERMINAL, QUIESCENT)


@dataclass(frozen=True)
class Transition:
    """Canon: §Lifecycle — one src→dst edge on an event, with optional guard.

    RULE: `src` and `dst` MUST be names of States declared in the containing
    Lifecycle (enforced by check_lifecycle_wellformed). `guard` is human-
    readable prose; `guard_assumption` optionally names the Assumption id the
    guard rests on — this is the behavioral drift seam: when that Assumption
    dies, the guard may no longer hold.

    WHY guard_assumption: connecting a transition guard to an Assumption lets
    the same drift machinery (DRIFT_FALLOUT) surface behavioral stale states,
    not just data stale states.
    """

    src: str  # State.name
    dst: str  # State.name
    event: str  # the trigger ("acknowledge", "decide", "withdraw", …)
    guard: str = ""  # human-readable predicate
    guard_assumption: str | None = None  # Assumption id the guard rests on (drift seam)
    why: str = ""


@dataclass(frozen=True)
class Lifecycle:
    """Canon: §Lifecycle — finite set of named states + transitions + cyclic flag.

    Validation contract (used by check_status_in_lifecycle and similar):
      - states is non-empty; exactly one INITIAL; every transition endpoint
        resolves to a declared state; if cyclic=False, at least one
        terminal/quiescent state is reachable from the INITIAL via BFS.

    A 'prefix state' is one whose name is the prefix of the value string
    carried by the source-of-truth field — e.g. 'DECIDED' is the prefix of
    'DECIDED(rationale)'. For prefix states, Lifecycle.matches(value) performs
    a prefix test; for exact states it matches by full equality.

    RULE: `prefix_states` enumerates state names whose stored value carries
    an inline argument (e.g. 'OPEN(question)', 'DECIDED(rationale)',
    'REVISIT_WHEN(condition)'). This is the canonical record of which states
    use the inline-argument pattern.
    """

    slug: str
    states: tuple[State, ...]
    transitions: tuple[Transition, ...] = field(default_factory=tuple)
    cyclic: bool = False
    prefix_states: tuple[str, ...] = field(default_factory=tuple)
    # ^ state names whose stored value carries an inline argument
    # (e.g. "OPEN(question)", "DECIDED(rationale)", "REVISIT_WHEN(condition)").

    def state_names(self) -> frozenset[str]:
        """Canon: §Lifecycle — the set of all state names in this Lifecycle."""
        return frozenset(s.name for s in self.states)

    def initial(self) -> State:
        """Canon: §Lifecycle — return the unique INITIAL state, or raise ValueError."""
        for s in self.states:
            if s.is_initial():
                return s
        raise ValueError(f"{self.slug}: no initial state")

    def matches(self, value: str) -> State | None:
        """Canon: §Lifecycle — return the State a stored value belongs to, or None.

        RULE: exact match for non-prefix states; prefix match
        (value == name or value.startswith(name + "(")) for states listed in
        `prefix_states`. Returns None if no state matches — the caller
        (invariant) treats None as a violation.

        WHY: the stored value 'DECIDED(rationale)' belongs to the DECIDED state;
        'OPEN(question)' belongs to OPEN. The prefix test must be guarded by
        '(' to avoid false matches (e.g. 'DECIDED2' must NOT match 'DECIDED').
        """
        for s in self.states:
            if s.name in self.prefix_states:
                if value == s.name or value.startswith(s.name + "("):
                    return s
            elif value == s.name:
                return s
        return None


# ---------------------------------------------------------------------------
# Framework-supplied canonical lifecycles
# ---------------------------------------------------------------------------

REQUIREMENT_STATUS_LIFECYCLE = Lifecycle(
    slug="requirement-status",
    states=(
        State("DRAFT", kind=INITIAL, why="Proposed, not yet accepted into the canon."),
        State("SETTLED", kind=NORMAL, why="Accepted and currently held."),
        State(
            "OPEN",
            kind=NORMAL,
            why="Accepted-with-a-hole; carries a non-empty question.",
        ),
        State(
            "REJECTED",
            kind=TERMINAL,
            why="Withdrawn; kept for history (anti-relitigation).",
        ),
    ),
    transitions=(
        Transition(
            "DRAFT",
            "SETTLED",
            event="accept",
            why="A draft is reviewed and accepted as canon.",
        ),
        Transition(
            "DRAFT", "REJECTED", event="reject", why="A draft is reviewed and rejected."
        ),
        Transition(
            "DRAFT",
            "OPEN",
            event="accept-with-hole",
            why="Accepted but a specific question remains.",
        ),
        Transition(
            "SETTLED",
            "REJECTED",
            event="withdraw",
            why="A previously-held requirement is retired.",
        ),
        Transition(
            "SETTLED",
            "OPEN",
            event="reopen-question",
            why="A new hole emerged in a SETTLED requirement.",
        ),
        Transition(
            "OPEN",
            "SETTLED",
            event="resolve-question",
            why="The OPEN(question) was decided.",
        ),
        Transition(
            "OPEN",
            "REJECTED",
            event="reject-question",
            why="The OPEN requirement is withdrawn.",
        ),
    ),
    prefix_states=("OPEN",),
)

CONFLICT_LIFECYCLE = Lifecycle(
    slug="conflict-lifecycle",
    states=(
        State(
            "DETECTED",
            kind=INITIAL,
            why="Surfaced; node materialized; no steward action yet.",
        ),
        State(
            "ACKNOWLEDGED", kind=NORMAL, why="Steward accepts it is real and owns it."
        ),
        State(
            "DECIDED",
            kind=QUIESCENT,
            why="Resolved with recorded rationale and/or derived requirement.",
        ),
        State(
            "REVISIT_WHEN",
            kind=QUIESCENT,
            why="Parked with an explicit revisit condition (anti-relitigation).",
        ),
        State(
            "HELD",
            kind=QUIESCENT,
            why=(
                "Not resolvable by amending the member requirements; held open "
                "as a live tension carrying >=2 elaborated behavior variants for "
                "the steward to choose between. Entered only by human signoff "
                "(decided_by), mirroring DECIDED's signoff lock."
            ),
        ),
    ),
    transitions=(
        Transition(
            "DETECTED",
            "ACKNOWLEDGED",
            event="steward-acknowledge",
            why="Steward takes ownership of the tension.",
        ),
        Transition(
            "ACKNOWLEDGED",
            "DECIDED",
            event="steward-decide",
            guard="rationale or derived requirement recorded",
            why="Resolution lands.",
        ),
        Transition(
            "ACKNOWLEDGED",
            "REVISIT_WHEN",
            event="steward-park",
            guard="revisit condition recorded",
            why="Parked until the condition fires.",
        ),
        Transition(
            "ACKNOWLEDGED",
            "HELD",
            event="steward-hold",
            guard="decided_by recorded and >=2 variants attached",
            why=(
                "Steward records that the tension cannot be resolved by "
                "amending the members and holds it open with variants."
            ),
        ),
        Transition(
            "DECIDED",
            "DETECTED",
            event="condition-fires",
            guard="revisit_marker condition holds",
            why="The condition recorded at decide time triggered.",
        ),
        Transition(
            "REVISIT_WHEN",
            "DETECTED",
            event="condition-fires",
            guard="parked condition holds",
            why="The parked condition triggered.",
        ),
        Transition(
            "HELD",
            "DECIDED",
            event="steward-choose-variant",
            guard="rationale names the chosen variant",
            why="Steward chooses one held variant; the tension resolves.",
        ),
    ),
    prefix_states=("DECIDED", "REVISIT_WHEN", "HELD"),
    cyclic=True,  # DECIDED/REVISIT_WHEN/HELD ↔ DETECTED allowed
)
