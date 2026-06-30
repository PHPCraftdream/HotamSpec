"""Seed tension graph fixture — the worked example, NOT framework content.

This is the small example world the tests and the `--demo` flag run against. It
is deliberately well-formed structurally yet carries a live methodology surface
so the harness has something to show:
  - one OPEN requirement (R-205) — an open hole to surface;
  - one Conflict still DETECTED with no steward movement (automation-vs-control);
  - one DEAD assumption (A-single-customer) with live dependents — context drift;
  - one latent-connector suspect — two requirements sharing an assumption with
    NO Conflict node materialized between them.

These are not bugs in the fixture; they are the methodology's working surface.

WHY a fixture (and not seed data in src/tensio/): Tensio is a CONTENT-FREE
framework. Business content (this file) is example data for the tests and the
opt-in `--demo` run; real domains live under `spec/content/`.
"""

from __future__ import annotations

from tensio.assumption import DEAD, Assumption
from tensio.axis import Axis
from tensio.conflict import Conflict, conflict_identity
from tensio.graph import TensionGraph
from tensio.requirement import Relation, Requirement
from tensio.stakeholder import Stakeholder


# --- Controlled vocabulary used by this fixture -----------------------------
# These are EXAMPLE axes. A real domain defines its own under spec/content/.

DEMO_AXES: tuple[Axis, ...] = (
    Axis(
        slug="latency-vs-completeness",
        description=(
            "Fast response now vs fully complete/validated result. Tightening "
            "latency tends to drop synchronous completeness, and vice versa."
        ),
    ),
    Axis(
        slug="cost-vs-flexibility",
        description=(
            "Cheap/simple/fixed implementation vs configurable/general one. "
            "Flexibility usually costs build, run, and reasoning budget."
        ),
    ),
    Axis(
        slug="privacy-vs-analytics",
        description=(
            "Minimizing data collection/retention vs maximizing data available "
            "for analytics, personalization, and audit."
        ),
    ),
    Axis(
        slug="consistency-vs-availability",
        description=(
            "Strong/synchronous correctness vs staying available and responsive "
            "under partition or load (the CAP tension, business-side)."
        ),
    ),
    Axis(
        slug="automation-vs-control",
        description=(
            "Automatic decisioning/throughput vs mandatory human review and "
            "override. Automation raises throughput, lowers human control."
        ),
    ),
)


def seed_graph() -> TensionGraph:
    """The canonical example tension graph (test fixture / `--demo` source).

    Well-formed against every structural invariant yet carries the four
    deliberately-planted surface signals (see module docstring). The harness
    must print a non-empty action list against this graph.
    """
    stakeholders = (
        Stakeholder(id="finance", name="Finance", domain="money / compliance"),
        Stakeholder(id="platform", name="Platform", domain="latency / SLA"),
        Stakeholder(id="product", name="Product", domain="customer experience"),
        Stakeholder(id="security", name="Security", domain="privacy / access"),
        Stakeholder(
            id="architecture",
            name="Architecture",
            domain="cross-cutting structure",
        ),
    )

    assumptions = (
        Assumption(
            id="A-single-customer",
            statement="Each account belongs to exactly one customer (no orgs).",
            status=DEAD,  # multi-user orgs shipped -> this is now FALSE
            owner="product",
            machine_check="account.org_users == 1",
        ),
        Assumption(
            id="A-sync-budget",
            statement="A request may spend up to 2s of synchronous work.",
            status="UNCERTAIN",
            owner="platform",
            machine_check="request.sync_budget_ms <= 2000",
        ),
        Assumption(
            id="A-eu-only",
            statement="All processed data subjects reside in the EU.",
            status="HOLDS",
            owner="security",
        ),
    )

    requirements = (
        Requirement(
            id="R-87",
            claim="The system shall return a payment decision in < 200 ms (p95).",
            owner="platform",
            status="SETTLED",
            why="Checkout abandonment rises sharply past 200ms; latency is revenue.",
            assumptions=("A-sync-budget",),
            relations=(Relation(kind="supports", target="R-90"),),
        ),
        Requirement(
            id="R-203",
            claim=(
                "The system shall run a full synchronous AML/compliance check "
                "before approving any payment."
            ),
            owner="finance",
            status="SETTLED",
            why="Regulatory: an approved payment must be screened, no async gap.",
            assumptions=("A-sync-budget",),
        ),
        Requirement(
            id="R-90",
            claim="The system shall personalize the checkout per returning customer.",
            owner="product",
            status="SETTLED",
            why="Personalization lifts conversion for known customers.",
            assumptions=("A-single-customer",),
        ),
        Requirement(
            id="R-150",
            claim=(
                "The system shall retain full per-customer transaction history "
                "for analytics."
            ),
            owner="product",
            status="SETTLED",
            why="Analytics and lifetime-value models need complete history.",
            assumptions=("A-single-customer",),
        ),
        Requirement(
            id="R-205",
            claim=(
                "The system shall let an account administrator act on behalf of "
                "other users in the same organization."
            ),
            owner="product",
            status="OPEN(which permission scopes may an admin assume?)",
            why=(
                "Multi-user orgs shipped; admins need delegated action. Scope of "
                "delegation is unresolved -> OPEN."
            ),
            assumptions=("A-eu-only",),
        ),
        Requirement(
            id="R-300",
            claim=(
                "The system shall pre-screen returning customers asynchronously "
                "and fast-path the synchronous check when a fresh clear exists."
            ),
            owner="architecture",
            status="DRAFT",
            why=(
                "Born from C-87x203 to dissolve latency-vs-completeness: move the "
                "heavy check off the hot path when a recent clearance is on file."
            ),
            assumptions=("A-sync-budget",),
        ),
    )

    c_axis = "latency-vs-completeness"
    c_ctx = "approving a payment at checkout"
    conflicts = (
        Conflict(
            id=conflict_identity(c_axis, c_ctx),
            axis=c_axis,
            context=c_ctx,
            members=("R-87", "R-203"),
            steward="architecture",
            lifecycle=(
                "DECIDED(fast-path the sync AML check via async pre-screening; "
                "see R-300)"
            ),
            decided_by="architecture",
            shared_assumption="A-sync-budget",
            derived=("R-300",),
            revisit_marker=(
                "REVISIT if A-sync-budget dies (sync budget changes invalidate "
                "the fast-path math)."
            ),
        ),
        Conflict(
            id=conflict_identity(
                "automation-vs-control", "acting inside a multi-user organization"
            ),
            axis="automation-vs-control",
            context="acting inside a multi-user organization",
            members=("R-90", "R-205"),
            steward="security",
            lifecycle="DETECTED",
            shared_assumption="A-single-customer",
            revisit_marker="",
        ),
    )

    return TensionGraph(
        axes=DEMO_AXES,
        stakeholders=stakeholders,
        assumptions=assumptions,
        requirements=requirements,
        conflicts=conflicts,
    )
