"""Canon: §Domain — content graph of domain 'hotam-dev'.

hotam-dev models the development of the Hotam-Spec repository ITSELF as a
tension graph: waves, commits, spawns, and verification gates. Two
accountable parties carry the tensions: the human dev-steward (reviews,
approves, signs off waves/commits, the only one who may authorize a push)
and the pipeline-operator (the AI agent/conveyor that executes waves,
applies proposals, spawns sub-agents). One axis so far:
speed-vs-verification -- the real, observed tension between the T1 targeted
gate on every apply_proposal call and the mandatory full T2 pytest suite at
wave/commit boundaries (T2 has hit multi-minute timeouts in this repo).

Stakeholders, axes and assumptions are hand-seeded here (mirroring
hotam-spec-self's own build_graph()) because the proposal protocol has no
ProposedStakeholder / ProposedAxis / ProposedAssumption kind
(R-no-hand-edit-graph governs Requirement/Conflict/EntityType content,
landed via tools/apply_proposal.py, see spec/.runtime/proposals/).

`requirements = (...)` and `conflicts = (...)` are separate top-level
assignments inside build_graph() (not inline TensionGraph kwargs) because
tools/apply_proposal.py's AST locator (_find_requirements_tuple_end /
the conflict-tuple equivalent) requires that exact shape to append new
nodes mechanically -- this mirrors domains/hotam-spec-self/graph.py's own
structure.
"""

from hotam_spec.assumption import Assumption, HOLDS
from hotam_spec.axis import Axis
from hotam_spec.conflict import Conflict, conflict_identity
from hotam_spec.graph import TensionGraph
from hotam_spec.requirement import ENFORCED, PROSE, STRUCTURAL, Requirement
from hotam_spec.stakeholder import Stakeholder


def build_graph() -> TensionGraph:
    axes = (
        Axis(
            slug="speed-vs-verification",
            description=(
                "T1 targeted-enforcer gate on every apply_proposal call "
                "(fast, per-move) vs the mandatory full T2 pytest suite "
                "at wave/commit boundaries (slow, complete). T2 has hit "
                "multi-minute timeouts in this repo, creating real "
                "pressure to skip or shrink it -- which would undermine "
                "wave atomicity."
            ),
        ),
    )

    stakeholders = (
        Stakeholder(
            id="dev-steward",
            name="Dev steward",
            domain="reviews and approves waves/commits; sole authority to request a push",
        ),
        Stakeholder(
            id="pipeline-operator",
            name="Pipeline operator",
            domain="executes waves, applies proposals, spawns sub-agents",
        ),
    )

    assumptions = (
        Assumption(
            id="A-runtime-logs-append-only",
            statement=(
                "spec/.runtime/*.jsonl logs (land-log.jsonl, spawn-log.jsonl, "
                "boot-cite-log.jsonl) are append-only -- no tool ever rewrites "
                "or truncates a prior entry."
            ),
            status=HOLDS,
            owner="pipeline-operator",
        ),
        Assumption(
            id="A-single-steward-session",
            statement=(
                "At most one human dev-steward session drives waves at a time "
                "-- no concurrent multi-steward editing of the same graph."
            ),
            status=HOLDS,
            owner="dev-steward",
        ),
    )

    requirements = (
        Requirement(
            id="R-t1-gate-is-default",
            claim=(
                "tools/apply_proposal.py shall run the T1 targeted-enforcer "
                "gate by default on every individual proposal apply, "
                "deferring the full T2 suite to wave/commit boundaries."
            ),
            owner="pipeline-operator",
            status="SETTLED",
            why=(
                "Per-move full-suite verification is too slow for the "
                "conveyor's normal cadence (observed multi-minute T2 runs); "
                "T1 keeps the loop responsive while T2 remains the mandatory "
                "boundary gate (see R-wave-lands-atomically)."
            ),
            assumptions=("A-runtime-logs-append-only",),
            enforcement=ENFORCED,
            enforced_by=("test_tool_gate.py",),
        ),
        Requirement(
            id="R-wave-lands-atomically",
            claim=("A wave shall land as a whole with a green T2 (full pytest suite) run at its boundary before the next wave starts."),
            owner="pipeline-operator",
            status="SETTLED",
            why=("T1 alone (see R-t1-gate-is-default) only checks the targeted enforcer subset per move; without a mandatory full-suite gate at the wave boundary, cross-cutting regressions between moves in the same wave could slip through undetected. Enforcement is honestly STRUCTURAL: what_now/apply_proposal make the T2 boundary visible and addressable (gate_status.py, land-log.jsonl tier trace) but no single check_* fires automatically if a steward skips a wave-boundary T2 run by hand outside the tool."),
            assumptions=("A-runtime-logs-append-only", "A-single-steward-session",),
            enforcement=STRUCTURAL,
        ),
        Requirement(
            id="R-spawn-logged",
            claim=("Every sub-agent spawn shall be appended to spec/.runtime/spawn-log.jsonl."),
            owner="pipeline-operator",
            status="SETTLED",
            why=("Without a trace of every spawn, R-delegation-conclusions-only and R-task-spawn-log-runtime (hotam-spec-self) cannot be audited after the fact; the append-only log is the mechanical proof a spawn happened, with what kind/parent/child."),
            assumptions=("A-runtime-logs-append-only",),
            enforcement=ENFORCED,
            enforced_by=("test_tool_spawn_agent.py::test_spawn_log_written",),
        ),
        Requirement(
            id="R-land-leaves-trace",
            claim=("Every applied proposal shall append a trace entry to spec/.runtime/land-log.jsonl."),
            owner="pipeline-operator",
            status="SETTLED",
            why=("The land-log is the mechanical record of what tier (T1/T2) verified each applied proposal and whether pytest passed -- the basis for gate_status.py's commit-boundary answer (R-commit-boundary-checkable, hotam-spec-self); without it there is no auditable trail of what actually got verified before landing."),
            assumptions=("A-runtime-logs-append-only",),
            enforcement=ENFORCED,
            enforced_by=("test_apply_proposal_land_log.py::test_land_log_record_shape_t1",),
        ),
        Requirement(
            id="R-commit-follows-review",
            claim=("A commit shall land only after review of the diff by a human or an agent code-review step."),
            owner="dev-steward",
            status="SETTLED",
            why=("Review is the last human-in-the-loop check before a change becomes permanent history; this is a judgment call about diff quality and intent that no mechanical check can substitute for, so enforcement stays honestly PROSE."),
            assumptions=("A-single-steward-session",),
            enforcement=PROSE,
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-push-only-on-request",
            claim=("Push to remote shall occur only on the dev-steward's explicit request, never autonomously."),
            owner="dev-steward",
            status="SETTLED",
            why=("A push is externally visible and hard to undo cleanly (shared history); reserving it strictly to explicit steward request keeps the irreversible action under human authority, mirroring the project's global CLAUDE.md discipline. Not mechanically checkable from inside this repo (no push actually happens in tests), so PROSE."),
            assumptions=("A-single-steward-session",),
            enforcement=PROSE,
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-wave-strictly-sequential",
            claim=("Waves touching overlapping files or scopes shall run strictly sequentially, never concurrently."),
            owner="pipeline-operator",
            status="SETTLED",
            why=("Concurrent edits to the same files/scope would race and corrupt state (the graph, the docs, the .runtime logs); git history in this repo is evidence of sequential waves but is not itself a cheap mechanical gate, so enforcement stays honestly STRUCTURAL/PROSE rather than a fabricated git-log parser."),
            assumptions=("A-single-steward-session",),
            enforcement=STRUCTURAL,
        ),
    )

    conflicts = (
        Conflict(
            id=conflict_identity("speed-vs-verification", "T1 targeted-enforcer gate on every apply_proposal call vs mandatory full T2 pytest suite at wave/commit boundaries -- T2 runs have hit multi-minute timeouts in this repo (observed Wave 2), creating real pressure to skip or shrink T2, which would undermine R-wave-lands-atomically"),
            axis="speed-vs-verification",
            context="T1 targeted-enforcer gate on every apply_proposal call vs mandatory full T2 pytest suite at wave/commit boundaries -- T2 runs have hit multi-minute timeouts in this repo (observed Wave 2), creating real pressure to skip or shrink T2, which would undermine R-wave-lands-atomically",
            members=("R-t1-gate-is-default", "R-wave-lands-atomically"),
            steward="dev-steward",
            lifecycle="DETECTED",
            shared_assumption="A-runtime-logs-append-only",
        ),
    )

    return TensionGraph(
        axes=axes,
        stakeholders=stakeholders,
        assumptions=assumptions,
        requirements=requirements,
        conflicts=conflicts,
    )
