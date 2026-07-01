"""Hotam-Spec modeling itself — the meta-domain (the framework's own design).

Hotam-Spec eats its own dog food: the methodology's design is the FIRST domain
populated under spec/content/. This is the strongest possible stress test of
the framework — if Hotam-Spec cannot model its own lifecycle cleanly, there is a
hole in the framework.

Four stakeholders carry the tensions: the framework AUTHOR (who designs and
defends the framework), the AI AGENT (who lives the three roles and the hard
boundary), the DOMAIN USER (who will populate a real business domain), and a
FRAMEWORK REVIEWER (who stewards tensions between author and AI — by
construction not the owner of either side).

Nine axes in two families:
  - Original methodology axes (6): agent-autonomy-vs-human-control,
    framework-purity-vs-helpfulness, core-vs-aspect,
    apparatus-weight-vs-coverage, formalization-vs-prose,
    single-altitude-vs-multi-altitude.
  - Operator/context axes (3): offload-vs-carry,
    horizontal-vs-vertical-relief, sequential-vs-parallel.

Nine assumptions cover the Python stack, stakeholder engagement, prose
sufficiency, graph-in-memory size, content-free legitimacy, bootstrap
self-application, finite-context operators, compaction loss, and
knowledge crystallizability.

≈165 requirements in the current SETTLED/DRAFT/OPEN/REJECTED mix (task #76):
  SETTLED (121): achieved core + all atomization wave promotions.
    Task #76 promoted DRAFT→SETTLED: R-smoke-test, R-audit-atomicity-tool,
    R-requirement-claim-is-atomic, R-check-method-is-atomic,
    R-constituting-requirements-converge, R-tools-registry-generated.
    Also fixed: duplicate R-bijection-r-to-enforcer DRAFT renamed+REJECTED;
    R-content-layout-evolution m_tag cleared (was erroneously M8 on SETTLED).
  DRAFT   (14): deferred layers — delegation, spawn-log, phi-cap, backend,
    context-hook, private-tools, tree-of-crystals, measure-context-size.
  OPEN    (11): live M-decisions awaiting steward confirmation — M3/M5/
    M17/M18/M19/M20/M21/M22/M26/M28/M30 (M7 resolved in P6).
  REJECTED (3): design dead-ends kept for history per R-rejected-preserved-
    not-deleted (R-seed-in-src, R-rdf-store, R-axes-as-module-constant).

6 conflicts: 5 DECIDED (autonomy-vs-boundary C-186c4347, bootstrap-paradox
C-c3911f28, apparatus-weight-vs-coverage C-06e2d84e, horizontal-vs-vertical
C-d210d6d0, sequential-vs-parallel C-d4f3eadf) + 1 live DETECTED
(C-8600b1b8 on core-vs-aspect — the open front, keeps what_now P3 non-trivial).

M-decisions: M1–M31 catalogued in CLAUDE.md; OPEN requirements R-trust-anchor-
mechanism through R-uncrystallizable-automated mirror the corresponding rows.

Build by `hotam_spec.graph.load_content_graph()`; rendered into docs/gen/{REQUIREMENTS,
TENSIONS,OPEN}.md by tools/gen_spec.py; diagnosed by tools/what_now.py.
"""

from __future__ import annotations

from hotam_spec.assumption import HOLDS, UNCERTAIN, Assumption
from hotam_spec.axis import Axis
from hotam_spec.conflict import Conflict, conflict_identity
from hotam_spec.graph import TensionGraph
from hotam_spec.operator import ContextBudget, Operator
from hotam_spec.process import (
    Goal,
    Process,
    Step,
    TargetState,
    TARGET_KIND_GRAPH_PROPERTY,
    PROCESS_LIFECYCLE,
)
from hotam_spec.requirement import ENFORCED, PROSE, STRUCTURAL, Requirement
from hotam_spec.stakeholder import Stakeholder


def build_graph() -> TensionGraph:
    """Hotam-Spec's own design as a TensionGraph (the meta-domain)."""
    stakeholders = (
        Stakeholder(
            id="framework-author",
            name="Framework author",
            domain="framework integrity, direction, philosophical premises",
        ),
        Stakeholder(
            id="ai-agent",
            name="AI agent",
            domain="the three roles (Detector / Socratic / Historian) and the hard boundary",
        ),
        Stakeholder(
            id="domain-user",
            name="Domain user",
            domain="a practitioner populating their business domain under spec/content/",
        ),
        Stakeholder(
            id="framework-reviewer",
            name="Framework reviewer",
            domain="independent stewardship of tensions between author and AI",
        ),
    )

    axes = (
        Axis(
            slug="agent-autonomy-vs-human-control",
            description=(
                "How far the AI agent acts vs how strictly it presents/asks. "
                "Autonomy makes the loop fast; human control keeps invisibility "
                "from being AI-created."
            ),
        ),
        Axis(
            slug="framework-purity-vs-helpfulness",
            description=(
                "Content-free shipping (zero business data in src/hotam_spec) vs "
                "out-of-the-box utility for a fresh adopter. Purity is honest; "
                "helpfulness lowers adoption cost."
            ),
        ),
        Axis(
            slug="core-vs-aspect",
            description=(
                "What stays in the minimal framework core vs what becomes an "
                "opt-in pluggable aspect. Core costs every domain; aspects cost "
                "only those who load them."
            ),
        ),
        Axis(
            slug="apparatus-weight-vs-coverage",
            description=(
                "Heavy formal machinery (Z3 / Quint / mutation testing) catches "
                "more contradictions but slows the loop. Calibration rule: "
                "weight of apparatus ∝ cost of an unnoticed conflict."
            ),
        ),
        Axis(
            slug="formalization-vs-prose",
            description=(
                "Machine-checkable predicate (deterministic, narrow) vs EARS / "
                "free-prose claim (broad, ambiguous). Most claims are prose; "
                "the critical core is formalized."
            ),
        ),
        Axis(
            slug="single-altitude-vs-multi-altitude",
            description=(
                "Conflating the methodology's own concepts with the modeled "
                "domain's (Task-vs-Action; Conflict-as-methodology-node vs "
                "Conflict-as-business-event). Two altitudes must stay separable."
            ),
        ),
        Axis(
            slug="offload-vs-carry",
            description=(
                "Crystallize knowledge into the free substrate (graph + "
                "invariants + generated docs) vs hold it in expensive working "
                "context. Substrate knowledge is enforced/regenerable/addressable, "
                "so it does not count against an operator's context budget."
            ),
        ),
        Axis(
            slug="horizontal-vs-vertical-relief",
            description=(
                "Relieve operator context pressure by delegating/splitting the "
                "domain (horizontal) vs by crystallizing knowledge into the "
                "substrate (vertical). Splitting is for irreducible size; "
                "crystallizing is for un-offloaded knowledge."
            ),
        ),
        Axis(
            slug="sequential-vs-parallel",
            description=(
                "Coupled work (dependency edges between requirements/operators/"
                "entities) must be processed sequentially; independent sub-graphs "
                "can be delegated to parallel sub-operators. The dependency-graph "
                "topology — not a guess — decides which, and domains are split "
                "along lines of independence."
            ),
        ),
    )

    assumptions = (
        Assumption(
            id="A-python-stack",
            statement="The framework runs on Python 3.12+ with uv + ruff + pytest + hypothesis.",
            status=HOLDS,
            owner="framework-author",
            machine_check="python.version >= (3, 12)",
        ),
        Assumption(
            id="A-stakeholders-care",
            statement="At least two distinct human stakeholders exist who are willing to steward conflicts.",
            status=HOLDS,
            owner="framework-author",
        ),
        Assumption(
            id="A-prose-suffices",
            statement="For the bulk of requirements, EARS-style prose claims plus structural invariants suffice; formal predicates are reserved for the critical core.",
            status=UNCERTAIN,
            owner="ai-agent",
        ),
        Assumption(
            id="A-graph-fits-memory",
            statement="The whole tension graph fits in one Python process; streaming/persistence is not required.",
            status=HOLDS,
            owner="framework-author",
            machine_check="len(graph.requirements) + len(graph.conflicts) < 10_000",
        ),
        Assumption(
            id="A-content-free-honest",
            statement="An empty spec/content/ is a legitimate ship state — the framework's value is real even before any domain is populated.",
            status=HOLDS,
            owner="framework-author",
        ),
        Assumption(
            id="A-bootstrap-self-applies",
            statement="The framework can model its own design coherently — Hotam-Spec's own methodology fits its own ontology with no special-casing.",
            status=UNCERTAIN,
            owner="framework-reviewer",
        ),
        Assumption(
            id="A-finite-context-operators",
            statement="Operators are finite-context agents; an operator's problem domain must fit its context.",
            status=HOLDS,
            owner="framework-author",
        ),
        Assumption(
            id="A-compaction-loses-working",
            statement="Knowledge living only in working context is lost on context auto-compaction; only the crystallized substrate survives.",
            status=HOLDS,
            owner="ai-agent",
        ),
        Assumption(
            id="A-most-knowledge-crystallizable",
            statement="Most knowledge can be expressed as a node; where it cannot, that resistance is itself a signal of a missing ontology type.",
            status=UNCERTAIN,
            owner="framework-reviewer",
        ),
    )

    requirements = (
        # --- SETTLED — the achieved core -----------------------------------
        Requirement(
            id="R-agent-never-lost",
            claim=(
                "The system shall let an agent dropped into the repo in any state, "
                "at any moment, deterministically derive the next correct action "
                "via tools/what_now.py."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "The centerpiece. Generalizes dev-coin's 'drift is structurally "
                "impossible' to 'being lost is structurally impossible'."
            ),
            assumptions=("A-stakeholders-care",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-drift-structurally-impossible",
            claim=(
                "The generated docs/gen/*.md shall equal the regeneration of the "
                "current spec/content + framework docstrings, byte-for-byte."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Anti-drift meta-test (tests/test_docs_gen.py) — direct lift of "
                "dev-coin's pattern. The human layer cannot be hand-edited."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=(
                "test_docs_gen.py::test_requirements_md_up_to_date",
                "test_docs_gen.py::test_tensions_md_up_to_date",
                "test_docs_gen.py::test_open_md_up_to_date",
                "test_docs_gen.py::test_unenforced_md_up_to_date",
            ),
        ),
        Requirement(
            id="R-conflict-is-connector-node",
            claim=(
                "A contradiction shall be modeled as a first-class Conflict NODE "
                "carrying axis + context + shared_assumption + steward, never as "
                "a `conflicts_with` edge between requirements."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The central ontological insight. An edge holds nothing; a node "
                "holds knowledge belonging to neither party (the axis, the "
                "context, the shared root) — that is what makes contradictions "
                "first-class and clusterable."
            ),
            assumptions=("A-content-free-honest",),
            enforcement=STRUCTURAL,
            enforced_by=(),
        ),
        Requirement(
            id="R-content-free-framework",
            claim=(
                "spec/src/hotam_spec/ shall contain ZERO business content — no "
                "example requirements, no example axes, no seed graph."
            ),
            owner="framework-author",
            status="REJECTED",
            why="REJECTED — REPLACES split into R-content-free-no-business-data + R-content-free-no-examples + R-content-free-no-seed-graph (D1, decided by domain-user 2026-06-30) — (was: Hotam-Spec is a blank kit. Business content lives under spec/content/; the worked example is a test fixture. REPLACES the earlier design where seed data lived in src/hotam_spec/graph.py.)",
            assumptions=("A-content-free-honest",),
            enforcement=ENFORCED,
            enforced_by=("test_content_free.py",),
        ),
        Requirement(
            id="R-deterministic-generation",
            claim=(
                "tools/gen_spec.py shall produce byte-stable LF utf-8 output "
                "with no timestamps or randomness — two runs over an unchanged "
                "graph yield identical bytes."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Determinism is what makes the anti-drift meta-test possible. "
                "Without it, regeneration would never equal the committed file."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_docs_gen.py::test_generator_is_deterministic",),
        ),
        Requirement(
            id="R-ai-presents-not-decides",
            claim=(
                "The AI agent shall NEVER close a Conflict silently -- it presents with justification and defers every resolution to the human steward."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "The hard boundary. If the AI resolves contradictions itself, invisibility returns — now AI-created. Made structural by check_steward_not_a_member_owner."
            ),
            assumptions=("A-stakeholders-care",),
            enforcement="STRUCTURAL",
            enforced_by=(),
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-steward-distinct-from-owners",
            claim=(
                "Every Conflict's steward shall be a Stakeholder who is NOT the "
                "owner of any of the conflict's members."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "A tension lives BETWEEN stakeholders; if the steward owned a "
                "side, the tension would be judged by an interested party and "
                "quietly resolved in their favor. The structural twin of "
                "R-ai-presents-not-decides."
            ),
            assumptions=("A-stakeholders-care",),
            enforcement=ENFORCED,
            enforced_by=("check_steward_not_a_member_owner",),
        ),
        Requirement(
            id="R-empty-content-is-legitimate",
            claim=(
                "A freshly-cloned framework with no spec/content/graph.py shall "
                "be structurally well-formed; what_now renders a calm 'no "
                "content yet' banner and gen_spec emits the same notice."
            ),
            owner="domain-user",
            status="REJECTED",
            why="REJECTED — REPLACES split into R-empty-content-wellformed + R-empty-content-calm-banner + R-empty-content-gen-notice (D2, decided by domain-user 2026-06-30) — (was: An empty content slot is honest, not a defect. Adopters can see the framework working before they have anything to model.)",
            assumptions=("A-content-free-honest",),
            enforcement=ENFORCED,
            enforced_by=(
                "test_what_now.py::test_main_empty_content_prints_calm_banner",
                "test_docs_gen.py::test_empty_graph_renders_no_content_notice",
            ),
        ),
        Requirement(
            id="R-open-states-question",
            claim=(
                "Every requirement whose status begins with 'OPEN' shall carry a "
                "non-empty question of the form OPEN(<question>)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "An OPEN with no question is a hole no one can act on. The "
                "harness surfaces OPEN items by their question; emptiness "
                "defeats the point of marking the requirement open at all."
            ),
            assumptions=("A-prose-suffices",),
            enforcement=ENFORCED,
            enforced_by=("check_open_has_question",),
        ),
        Requirement(
            id="R-rejected-preserved-not-deleted",
            claim=(
                "Requirements that are rejected shall be marked REJECTED and "
                "kept in the graph for history, never deleted."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Anti-relitigation. Without preserved REJECTED, the same dead "
                "ideas re-surface every quarter. The historian role depends on "
                "this preservation."
            ),
            assumptions=("A-stakeholders-care",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-axis-controlled-vocab",
            claim=(
                "Every Conflict.axis shall be the slug of an Axis declared in "
                "the graph's `axes` tuple."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Conflicts CLUSTER by axis — many C-nodes on one axis = one "
                "architectural choice. Free-text axes would fragment the "
                "cluster and hide the clustering signal."
            ),
            assumptions=("A-prose-suffices",),
            enforcement=ENFORCED,
            enforced_by=("check_axis_in_registry",),
        ),
        Requirement(
            id="R-stable-conflict-identity",
            claim=(
                "A Conflict's id shall equal conflict_identity(axis, context) — "
                "the deterministic hash of its tension, not its members."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Identity from the tension itself lets the node survive renaming "
                "or splitting of its member requirements — only its edges update."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("check_conflict_id_matches_identity",),
        ),
        Requirement(
            id="R-two-altitude-ontology",
            claim=(
                "The methodology shall use ONE ontology at two altitudes: operator is to the methodology as actor is to the business (the methodology plane is the business plane applied reflexively)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Realized in THIS meta-domain — Hotam-Spec modeling its own design IS the proof that one ontology serves both altitudes. D3 (decided by domain-user 2026-06-30): downgraded STRUCTURAL→PROSE — no structural enforcer exists; the claim is discipline, not check."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement="PROSE",
            enforced_by=(),
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-boot-from-substrate",
            claim=(
                "The operator shall begin every new turn by re-loading three facts "
                "from the substrate — current context %, the top what_now action, "
                "and the SETTLED-DRAFT-UNENFORCED ratio — and cite at least one of "
                "them in the first sentence of any substantive reply."
            ),
            owner="ai-agent",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-boot-reload-three-facts + R-boot-cite-in-first-sentence (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-boot-reload-three-facts + R-boot-cite-in-first-sentence (wave 2, decided by framework-author 2026-06-30) — (was: Without this, the operator knows the spec but lives in session memory; CLAUDE.md is the only file the harness auto-loads, so the boot ritual MUST live there (not in CONSTITUTION.md, which is referenceable but not auto-loaded). This is the structural fix for 'knows the spec vs lives by it'.))"
            ),
            assumptions=("A-compaction-loses-working", "A-bootstrap-self-applies"),
            enforcement=STRUCTURAL,
            enforced_by=(),
        ),
        # --- OPEN(question) — live methodology decisions M1–M9 ---------------
        Requirement(
            id="R-trust-anchor-mechanism",
            claim=(
                "The methodology shall be externally anchored by a periodic "
                "stakeholder cryptographic signature on the tension map per "
                "domain — to ground the internal loop in a living human."
            ),
            owner="framework-author",
            status=(
                "OPEN(what signature mechanism (PGP/SSH/web of trust) and "
                "cadence (quarterly/per-PR/on-domain-change) anchor the loop?)"
            ),
            why=(
                "M5. The internal loop is self-referential; without an external "
                "anchor, the graph eventually drifts from the real organization. "
                "Mechanism and cadence pending."
            ),
            assumptions=("A-stakeholders-care", "A-bootstrap-self-applies"),
            m_tag="M5",
        ),
        Requirement(
            id="R-critical-core-scope",
            claim=(
                "The set of requirement domains warranting the deferred formal "
                "layers (Z3 conflict-detector, Quint temporal, mutation "
                "testing) shall be declared."
            ),
            owner="domain-user",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-critical-core-methodology + R-critical-core-per-domain (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-critical-core-methodology + R-critical-core-per-domain (wave 2, decided by framework-author 2026-06-30) — (was: M7 resolved (P6 — §Conscience): the critical core for the methodology's OWN domain is the six invariants in CRITICAL_CORE_INVARIANTS — check_steward_not_a_member_owner, check_operator_steward_not_self, check_decided_has_decided_by, check_typed_anchors, check_no_dangling_ids, check_open_has_question. These six guard every path by which a contradiction could be introduced without being seen. Business-domain 'critical core' (money / access / SLA) is a separate per-domain calibration; the framework's own methodology critical core is now declared and property-tested via test_conscience.py.))"
            ),
            assumptions=("A-prose-suffices",),
            enforcement=ENFORCED,
            enforced_by=("test_conscience.py",),
        ),
        Requirement(
            id="R-axis-gatekeeper-policy",
            claim=(
                "The admission policy for a new axis slug shall be machine-"
                "checked against duplicate detection by the AI gatekeeper."
            ),
            owner="ai-agent",
            status=(
                "OPEN(when do we switch on the AI duplicate-gatekeeper — "
                "immediately, on first ambiguous slug, or only above N axes?)"
            ),
            why=(
                "M3. Manual editing scales until two stewards add near-duplicate "
                "axes that fragment a cluster. Then the gatekeeper earns its "
                "place."
            ),
            assumptions=("A-prose-suffices",),
            m_tag="M3",
        ),
        Requirement(
            id="R-content-layout-evolution",
            claim=(
                "Domain content shall live in per-domain directories under domains/<name>/graph.py, with multi-domain federation implemented via the domains/ layout introduced in P17."
            ),
            owner="framework-author",
            status=(
                "SETTLED"
            ),
            why=(
                "M8 + M9. DECIDED 2026-06-30: P17 implemented the multi-domain layout (domains/<name>/graph.py + manifest.py + agents/director/) making the 'one file or split?' question moot — the answer is per-domain directories, each owning its own graph.py, with gen_spec discovering all of them. Single-file spec/content/graph.py is superseded by this layout. Evidence: domains/hotam-spec-self/graph.py, spec/tools/gen_spec.py load_content_graph, R-domain-owns-graph-py SETTLED."
            ),
            assumptions=("A-bootstrap-self-applies", "A-graph-fits-memory"),
            enforcement="ENFORCED",
            enforced_by=("check_domain_manifest_exists_and_importable", "R-domain-owns-graph-py"),
            m_tag="",
        ),
        # --- DRAFT — proposed next-steps -----------------------------------
        Requirement(
            id="R-active-loop-playbooks",
            claim=(
                "Each what_now priority band shall have a documented agent "
                "PLAYBOOK plus a tools/apply_proposal.py that mechanically "
                "applies a steward-approved JSON proposal to spec/content/."
            ),
            owner="ai-agent",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES by R-active-loop-protocol + "
                "R-active-loop-apply-tool + R-active-loop-playbook-doc per "
                "atomicity discipline (R-requirement-claim-is-atomic). The "
                "original claim mixed three concerns: data-model, tool, "
                "documentation."
            ),
            assumptions=("A-stakeholders-care", "A-prose-suffices"),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-active-loop-protocol",
            claim=(
                "Three Proposed* dataclass types (ProposedRequirement, "
                "ProposedConflictTransition, ProposedRejection) shall exist as "
                "the protocol for steward-approved operator changes."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Atom of R-active-loop-playbooks (data-model concern). "
                "hotam_spec/proposal.py defines the three types."
            ),
            assumptions=("A-stakeholders-care", "A-prose-suffices"),
            enforcement=ENFORCED,
            enforced_by=("test_proposal.py",),
        ),
        Requirement(
            id="R-active-loop-apply-tool",
            claim=(
                "A tool tools/apply_proposal.py shall consume an approved "
                "Proposed* JSON and mechanically apply the change to "
                "spec/content/."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Atom of R-active-loop-playbooks (tool concern). "
                "tools/apply_proposal.py lands a steward-approved JSON "
                "proposal into spec/content/graph.py and runs the "
                "regen+verify pipeline."
            ),
            assumptions=(),
            enforcement=ENFORCED,
            enforced_by=("test_apply_proposal.py",),
        ),
        Requirement(
            id="R-active-loop-playbook-doc",
            claim=(
                "At least one band-specific playbook shall exist under "
                "docs/playbooks/ describing the agent's role for that band."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Atom of R-active-loop-playbooks (documentation concern). "
                "docs/playbooks/P4-OPEN-ITEM.md is the first band playbook."
            ),
            assumptions=(),
            enforcement=STRUCTURAL,
            enforced_by=(),
        ),
        Requirement(
            id="R-decided-needs-human-signoff",
            claim=(
                "A Conflict in DECIDED(...) lifecycle shall carry a "
                "decided_by: Stakeholder.id field (later: a cryptographic "
                "signature) — enforced by a new invariant."
            ),
            owner="framework-reviewer",
            status="SETTLED",
            why=(
                "SETTLED (P3): Conflict.decided_by field added; "
                "check_decided_has_decided_by fires when lifecycle starts "
                "with DECIDED but decided_by is empty or owned by a member. "
                "Makes the hard boundary structural at the decision moment — "
                "the AI cannot silently write DECIDED without naming a human "
                "decider who is outside the conflict's members."
            ),
            assumptions=("A-stakeholders-care",),
            enforcement=ENFORCED,
            enforced_by=(
                "check_decided_has_nonempty_decided_by",
                "check_decided_by_is_known_stakeholder",
                "check_decided_by_not_member_owner",
            ),
        ),
        Requirement(
            id="R-glossary-sync-test",
            claim=(
                "A controlled vocabulary of methodology terms shall be "
                "generated under docs/gen/GLOSSARY.md, with a sync test that "
                "fails on undefined or unused terms."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-glossary-generated + R-glossary-sync-fails-dead + R-glossary-sync-fails-unused + R-glossary-drift-stable (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-glossary-generated + R-glossary-sync-fails-dead + R-glossary-sync-fails-unused + R-glossary-drift-stable (wave 2, decided by framework-author 2026-06-30) — (was: Terminology drift is its own kind of invisibility — 'axis' / 'dimension', 'steward' / 'owner', 'conflict' / 'tension' will fragment without it. Now ENFORCED: test_glossary_sync.py fires on any dead vocab or invented §-token, and test_docs_gen.py::test_glossary_md_up_to_date keeps GLOSSARY.md byte-stable.))"
            ),
            assumptions=("A-prose-suffices", "A-python-stack"),
            enforcement=ENFORCED,
            enforced_by=(
                "test_glossary_sync.py",
                "test_docs_gen.py::test_glossary_md_up_to_date",
            ),
        ),
        Requirement(
            id="R-history-from-rejected-markers",
            claim=(
                "docs/gen/HISTORY.md shall be generated from REJECTED markers "
                "in requirement WHY blocks and from DECIDED/REVISIT_WHEN "
                "lifecycle states on Conflicts."
            ),
            owner="ai-agent",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-history-generated-from-rejected + R-history-generated-from-decided (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-history-generated-from-rejected + R-history-generated-from-decided (wave 2, decided by framework-author 2026-06-30) — (was: The historian artifact is now real: build_history() in tools/gen_spec.py materializes REJECTED requirements and DECIDED/REVISIT_WHEN conflicts into docs/gen/HISTORY.md. Anti-drift enforced by test_history_md_up_to_date; content coverage enforced by test_history_gen.py.))"
            ),
            assumptions=("A-prose-suffices",),
            enforcement=ENFORCED,
            enforced_by=(
                "test_history_gen.py",
                "test_docs_gen.py::test_history_md_up_to_date",
            ),
        ),
        Requirement(
            id="R-smoke-test",
            claim=(
                "spec/tests/test_smoke.py shall provide one fast end-to-end "
                "signal that the framework is healthy — load content, run all "
                "invariants, run the harness, regenerate docs."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "SETTLED (BUILD-TRIGGER fired): spec/tests/test_smoke.py exists "
                "and provides the one-signal health check — load content + all "
                "invariants + harness + regen in a single test. An agent after "
                "a change should not need to remember the full test count or "
                "layout — one smoke = one signal."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("tests/test_smoke.py",),
        ),
        Requirement(
            id="R-lifecycle-abstraction",
            claim=(
                "A generic hotam_spec.lifecycle (State / Transition / Lifecycle) "
                "shall be introduced; Requirement.status and Conflict.lifecycle "
                "shall validate against framework-supplied Lifecycle constants."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-lifecycle-type-exists + R-lifecycle-validates-requirement + R-lifecycle-validates-conflict (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-lifecycle-type-exists + R-lifecycle-validates-requirement + R-lifecycle-validates-conflict (wave 2, decided by framework-author 2026-06-30) — (was: Built: hotam_spec/lifecycle.py ships REQUIREMENT_STATUS_LIFECYCLE and CONFLICT_LIFECYCLE; check_status_in_lifecycle validates stored values against them on every invariant run (P1).))"
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("check_status_in_lifecycle", "test_lifecycle.py"),
        ),
        Requirement(
            id="R-process-aspect-first",
            claim=(
                "hotam_spec.process shall be the FIRST opt-in behavioral aspect — "
                "Lifecycle + Steps + roles_required + drives_entities — added "
                "after the keystone Lifecycle abstraction lands."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-process-types-exist + R-process-opt-in + R-process-lifecycle-wellformed-aspect + R-process-roles-declared-aspect + R-process-goal-owner-is-operator-aspect + R-process-typed-anchors-extended (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-process-types-exist + R-process-opt-in + R-process-lifecycle-wellformed-aspect + R-process-roles-declared-aspect + R-process-goal-owner-is-operator-aspect + R-process-typed-anchors-extended (wave 2, decided by framework-author 2026-06-30) — (was: SETTLED (P9): hotam_spec/process.py ships Process + Step + Goal + TargetState + PROCESS_LIFECYCLE + GOAL_LIFECYCLE. The §Process aspect is opt-in (TensionGraph.processes defaults to empty). PR-closed-loop instantiates ONE worked example at the meta-domain level. Three new invariants enforce the behavioral surface: check_process_lifecycle_wellformed, check_process_roles_declared, and check_goal_owner_is_operator. check_typed_anchors extended for PR- and GOAL- prefixes. M12 resolved: Lifecycle is core; Process is the first opt-in aspect that proves the keystone supports new aspects without parallel machinery.))"
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement=ENFORCED,
            enforced_by=(
                "test_process.py",
                "check_process_lifecycle_wellformed",
                "check_process_roles_declared",
                "check_typed_anchors",
            ),
        ),
        Requirement(
            id="R-task-vs-action-distinct-altitudes",
            claim=(
                "The methodology's Task node type (a modeled work item) and the harness's Action (a fix-the-graph instruction) shall remain distinct types at distinct altitudes — never merged."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "SETTLED (P9): the discipline is structural by omission. Hotam-Spec's framework has NO Task type — only the harness Action (hotam_spec.what_now.Action). Process.steps carry a forward-compat prose `invokes` field (not a Task type) so the behavioral altitude stays separable from the harness altitude. The two are typed differently by construction: Action is the harness's typed instruction; any future Task would be a domain-modeled work item under the §Process aspect. The altitudes cannot collapse because they live in different namespaces. Implementation: hotam_spec.what_now.Action + docs/gen/CONSTITUTION.md + docs/playbooks/."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement="STRUCTURAL",
            enforced_by=(),
            enforceability="INHERENTLY_PROSE",
        ),
        # --- DRAFT — operators / budget / delegation / goals (dossier 2) -----
        Requirement(
            id="R-operator-acting-facet",
            claim=(
                "An Operator shall be a Stakeholder's ACTING facet: it owns a "
                "bounded DomainScope, carries a ContextBudget and capabilities, "
                "and may have a parent Operator."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES by R-operator-is-frozen-dataclass + "
                "R-operator-references-stakeholder + R-operator-has-context-budget + "
                "R-operator-may-have-parent per atomicity discipline "
                "(R-requirement-claim-is-atomic). The original claim mixed four "
                "concerns: type identity, stakeholder reference, budget, hierarchy."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-operator-is-frozen-dataclass",
            claim=(
                "An Operator shall be a frozen dataclass in hotam_spec.operator "
                "with typed anchor 'OP-'."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-operator-acting-facet (type identity concern). "
                "hotam_spec.operator.Operator is a frozen dataclass; OP-director "
                "is the first instance."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=ENFORCED,
            enforced_by=("check_typed_anchors_operator", "test_operator.py"),
        ),
        Requirement(
            id="R-operator-references-stakeholder",
            claim=(
                "An Operator.stakeholder shall reference an existing Stakeholder.id."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-operator-acting-facet (stakeholder reference concern). "
                "check_no_dangling_ids validates the reference."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=ENFORCED,
            enforced_by=("check_no_dangling_operator_refs", "test_operator.py"),
        ),
        Requirement(
            id="R-operator-has-context-budget",
            claim=(
                "An Operator shall carry a ContextBudget with a positive limit "
                "and a declared measure."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-operator-acting-facet (budget concern). "
                "check_operator_within_budget validates the budget."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=ENFORCED,
            enforced_by=("check_operator_within_budget", "test_operator.py"),
        ),
        Requirement(
            id="R-operator-may-have-parent",
            claim=(
                "An Operator.parent shall reference another Operator.id or be None (root)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-operator-acting-facet (hierarchy concern). Structural via the Operator.parent field type."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("check_no_dangling_operator_refs",),
        ),
        Requirement(
            id="R-context-budget-rule",
            claim=(
                "An operator's owned domain shall not exceed its context budget (size(domain) <= budget.limit), with any excess flagged as a structural OVERLOADED contradiction by the harness."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Built (P2): check_operator_within_budget fires when NODE_COUNT exceeds limit. OP-director budget set to 200 to cover the meta-domain. DomainScope narrowing deferred to P5+."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("check_operator_within_budget", "test_operator.py::test_check_operator_within_budget_fires", "test_operator.py::test_director_within_budget"),
        ),
        Requirement(
            id="R-operator-not-self-approve",
            claim=(
                "An Operator shall not steward a Conflict in which its underlying "
                "Stakeholder owns one of the members."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "M36 — the reflexive twin of check_steward_not_a_member_owner. An "
                "Operator is the acting facet of a Stakeholder; the steward-distinct "
                "boundary applies through that facet so an Operator cannot self-"
                "ratify decisions on its own party's side. Structurally enforced."
            ),
            assumptions=("A-stakeholders-care",),
            enforcement=ENFORCED,
            enforced_by=(
                "check_operator_steward_not_self",
                "test_operator.py::test_check_operator_steward_not_self_fires",
            ),
            m_tag="",
        ),
        Requirement(
            id="R-delegation-conclusions-only",
            claim=(
                "When an operator delegates a sub-domain, the sub-operator shall return only CONCLUSIONS with shared objects declared as an explicit border, never raw detail."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "SETTLED (P8): the proposal protocol (P3) carries CONCLUSIONS (rationale + derived requirements) not raw context detail — the apply_proposal tool's narrow API surface IS the contract. The parent keeps the conclusion-as-proposal, not the file-dump of working context. Returning raw detail would re-import the sub-domain's whole context into the parent's budget, defeating the delegation. Implementation: spec/src/hotam_spec/proposal.py + tools/apply_proposal.py + docs/playbooks/."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="STRUCTURAL",
            enforced_by=(),
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-goal-as-target-state",
            claim=(
                "A Goal shall be a desired target-state predicate; the Gap = "
                "(Goal - current state) is the work that drives a Process."
            ),
            owner="domain-user",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES by R-goal-is-first-class-type + "
                "R-goal-target-kind-known + R-goal-owner-is-operator per "
                "atomicity discipline (R-requirement-claim-is-atomic). The "
                "original claim was mostly atomic but its enforced_by tuple "
                "covered three distinct rules."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-goal-is-first-class-type",
            claim=(
                "Goal shall be its own frozen dataclass type (not a Requirement "
                "facet) with typed anchor 'GOAL-'."
            ),
            owner="domain-user",
            status="SETTLED",
            why=(
                "Atom of R-goal-as-target-state (type identity concern). "
                "hotam_spec/process.py defines Goal as a frozen dataclass with "
                "GOAL_LIFECYCLE. M19 resolved."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("test_goal.py", "check_typed_anchors_goal"),
        ),
        Requirement(
            id="R-goal-target-kind-known",
            claim=("Goal.target_state.kind shall be one of the declared TARGET_KINDS."),
            owner="domain-user",
            status="SETTLED",
            why=(
                "Atom of R-goal-as-target-state (target-kind concern). "
                "check_goal_target_kind_known validates."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("check_goal_target_kind_known",),
        ),
        Requirement(
            id="R-goal-owner-is-operator",
            claim=("Goal.owner shall reference an existing Operator.id."),
            owner="domain-user",
            status="SETTLED",
            why=(
                "Atom of R-goal-as-target-state (ownership concern). "
                "check_goal_owner_is_operator and check_no_dangling_ids validate."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("check_goal_owner_is_operator", "check_no_dangling_operator_refs"),
        ),
        Requirement(
            id="R-context-bounded-delegation",
            claim=(
                "The methodology shall relieve an over-budget operator by splitting "
                "its domain into a bounded sub-domain owned by a spawned "
                "sub-operator (the horizontal lever)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "SETTLED (P8): the P8 REFLECTION band fires 'over-budget' → "
                "crystallize first → if still over, delegate. The signal path is "
                "structural: check_operator_within_budget (P1) detects the breach; "
                "the REFLECTION band (P0, tools/what_now.py::P_REFLECTION) names "
                "the path; docs/playbooks/ documents the procedure. DomainScope "
                "narrowing (per-operator sub-graph) remains a later phase but the "
                "SIGNAL — over-budget → delegate — exists today. Makes the "
                "methodology scale-free; generalizes 'agent never lost' to 'agent "
                "never overloaded'. Implementation: tools/what_now.py::P_REFLECTION + "
                "docs/playbooks/."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=STRUCTURAL,
            enforced_by=("check_operator_within_budget",),
        ),
        Requirement(
            id="R-dependency-graph-parallelism",
            claim=(
                "The system shall track the dependency network between "
                "requirements/operators/entities (building on Requirement.relations "
                "depends_on/supports/refines) so that independent sub-graphs may be "
                "delegated to PARALLEL sub-operators while dependency chains are "
                "processed SEQUENTIALLY."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-dependency-tracked + R-dependency-drives-parallel + R-dependency-drives-sequential (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-dependency-tracked + R-dependency-drives-parallel + R-dependency-drives-sequential (wave 2, decided by framework-author 2026-06-30) — (was: SETTLED (P8): Requirement.relations (depends_on/supports/refines) is the live dependency network; the U‖/A‖/B‖ parallel commits demonstrate the principle operationally — independent sub-graphs ran in parallel, dependency chains ran sequentially. Parallel-vs-sequential is decided by the dependency topology (independent components vs chains), not guessed; this makes delegation sound. Implementation: hotam_spec.requirement.Relation + docs/playbooks/ + tools/what_now.py.))"
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=STRUCTURAL,
            enforced_by=(),
        ),
        Requirement(
            id="R-operator-crystal-is-claude-md",
            claim=(
                "Each operator's crystallized substrate shall be its own CLAUDE.md "
                "— an anchored map of its bounded sub-domain that it reloads BY "
                "REFERENCE rather than re-carrying; the director-operator's "
                "CLAUDE.md holds the overall graph and references each "
                "sub-operator's CLAUDE.md."
            ),
            owner="ai-agent",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-crystal-is-claude-md + R-crystal-reload-by-reference + R-crystal-tree-hierarchy (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-crystal-is-claude-md + R-crystal-reload-by-reference + R-crystal-tree-hierarchy (wave 2, decided by framework-author 2026-06-30) — (was: SETTLED (P7): the crystal exists as substrate. The Director's Map in CLAUDE.md indexes the whole graph and provides the anchored map for the director-operator. docs/gen/CONSTITUTION.md is the generated reconstitution from the laws — a fresh agent reading it reconstitutes as operator without relying on a session checkpoint. The discipline is structural via: the Director's Map is the crystal (CLAUDE.md); CONSTITUTION.md is generated from the SETTLED laws; the boot-sequence in §6 names the exact steps to reconstitute. Per the anchoring super-rule it cites code handles (R-/C-/§/file) so understanding is regained fast; the delegation hierarchy is therefore a TREE of CLAUDE.md crystals (exactly how Claude Code nests CLAUDE.md per directory), one per operator, each bounded by its context budget. Implementation: docs/gen/CONSTITUTION.md + CLAUDE.md.))"
            ),
            assumptions=("A-compaction-loses-working", "A-bootstrap-self-applies"),
            enforcement=STRUCTURAL,
            enforced_by=("test_constitution_gen.py",),
        ),
        # --- DRAFT — behavioral aspects (dossier 1) --------------------------
        Requirement(
            id="R-statemachine-wellformedness",
            claim=(
                "Every modeled state machine shall be reachable, deterministic, "
                "and terminal (or explicitly cyclic); a transition guard may rest "
                "on an Assumption (the behavioral drift seam)."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES by R-statemachine-reachable + "
                "R-statemachine-deterministic + R-statemachine-terminal-or-cyclic + "
                "R-statemachine-guard-on-assumption per atomicity discipline "
                "(R-requirement-claim-is-atomic). The original claim mixed four "
                "concerns: reachability, determinism, termination, guard-on-assumption."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-statemachine-reachable",
            claim=(
                "Every state in a canonical Lifecycle shall be reachable from "
                "the initial state."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-statemachine-wellformedness (reachability concern). "
                "check_lifecycle_wellformed and check_canonical_lifecycles_wellformed "
                "validate reachability for all framework lifecycles."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("check_canonical_lifecycles_wellformed",),
        ),
        Requirement(
            id="R-statemachine-deterministic",
            claim=(
                "A Lifecycle's transitions shall be deterministic — no two "
                "transitions with the same (src, event) and overlapping guards."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-statemachine-wellformedness (determinism concern). "
                "check_lifecycle_wellformed validates determinism."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("check_canonical_lifecycles_wellformed",),
        ),
        Requirement(
            id="R-statemachine-terminal-or-cyclic",
            claim=(
                "Every non-cyclic Lifecycle shall reach at least one terminal/"
                "quiescent state."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-statemachine-wellformedness (termination concern). "
                "check_lifecycle_wellformed validates terminal reachability."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("check_canonical_lifecycles_wellformed",),
        ),
        Requirement(
            id="R-statemachine-guard-on-assumption",
            claim=(
                "A Transition.guard may name an Assumption it rests on (drift "
                "seam) — when that Assumption dies, the guard is surfaced."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-statemachine-wellformedness (guard-on-assumption concern). "
                "Structural via Transition.guard_assumption field."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=STRUCTURAL,
            enforced_by=(),
        ),
        # --- DRAFT — crystallization + anchoring super-rules (dossier 3) -----
        Requirement(
            id="R-crystallize-knowledge-to-code",
            claim=(
                "An operator shall continuously crystallize working knowledge into requirement-code (the substrate) as the offload instrument, since crystallized knowledge does not count against context."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "SETTLED (P4): the act of crystallization is now structurally supported. Every codified knowledge-piece flows through the proposal → approve → apply → verify-closure pipeline (tools/apply_proposal.py + tools/closure.py). The closure check makes crystallization audit-able: each applied proposal must prove it removed the triggering diagnosis, so the discipline is not merely claimed but structurally enforced at the feedback edge. STRUCTURAL (not ENFORCED) because WHAT to crystallize remains a steward call; the pipeline + closure assert HOW it is done. Implementation: tools/apply_proposal.py + tools/closure.py + docs/playbooks/."
            ),
            assumptions=("A-compaction-loses-working",),
            enforcement="STRUCTURAL",
            enforced_by=(),
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-verify-closure-per-action",
            claim=(
                "After an applied proposal lands (write + regen + pytest pass), the "
                "system shall verify the action that triggered the proposal is "
                "no longer present in the post-apply what_now diagnosis."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "P4 — the feedback edge that makes Drive (P5) safe to automate. "
                "Without per-action closure, an apply can technically land (tests "
                "green) yet the same diagnosis re-surface — the tick would spin "
                "without advancing. Structural answer: closure.check_closure asserts "
                "no Action with the original (kind, target) pair remains."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=ENFORCED,
            enforced_by=("test_closure.py", "tools/closure.py::check_closure"),
        ),
        Requirement(
            id="R-anchor-everything",
            claim=(
                "Every object shall carry a stable, short, typed anchor (prefix "
                "names the kind: R-/C-/A-/OP-/GOAL-/...)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "SETTLED (P5): structurally enforced via three independent checks. "
                "check_typed_anchors fires when any R-/A-/C-/OP- id lacks its "
                "typed prefix. check_section_anchors_known fires when any §-token "
                "in framework docstrings is absent from the glossary — an unresolved "
                "anchor. test_glossary_sync.py cross-checks the same invariant at "
                "test-time. Together these three make the anchor discipline "
                "machine-checkable at every invariant run."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=(
                "check_typed_anchors_requirement",
                "check_typed_anchors_assumption",
                "check_typed_anchors_conflict",
                "check_typed_anchors_operator",
                "check_typed_anchors_process",
                "check_typed_anchors_goal",
                "check_typed_anchors_entity",
                "check_section_anchors_known",
                "test_glossary_sync.py",
            ),
        ),
        Requirement(
            id="R-speak-by-reference",
            claim=(
                "An operator shall communicate by reference, ensuring every assertion cites at least one concrete anchor in the info-space."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "SETTLED (P5): the references-not-content discipline is now "
                "structurally bound. check_section_anchors_known ensures every "
                "§-anchor cited in framework docstrings resolves in the glossary — "
                "an operator that invents a §-token immediately fires a P1 "
                "STRUCTURE violation. test_glossary_sync.py provides the test-time "
                "mirror. docs/playbooks/ mandates that every proposal cites the "
                "R-/C-/§ anchor it acts on. The §Tick advisory output itself "
                "cites anchor ids in every action (target field). Together these "
                "make reference-not-content structurally visible and machine-checked."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=(
                "test_glossary_sync.py",
                "check_section_anchors_known",
                "docs/playbooks/",
            ),
        ),
        Requirement(
            id="R-crystallize-before-split",
            claim=(
                "On overload, an operator shall crystallize first, re-measure, and delegate (split) only if still over budget."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "SETTLED (P7): the order discipline is structurally bound. The apply_proposal protocol crystallizes via Proposal types; the closure check verifies advancement before any split is even considered; the constitution §4 (super-rules) names the ORDER explicitly. Splitting is for irreducible size, crystallizing is for un-offloaded knowledge; delegation is the lever of last resort. Splitting before crystallizing fragments knowledge that could have been freed in place. Implementation: tools/apply_proposal.py + tools/closure.py + docs/gen/CONSTITUTION.md."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="STRUCTURAL",
            enforced_by=(),
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-working-vs-substrate-budget",
            claim=(
                "The context budget shall bound only the WORKING store of active uncrystallized knowledge, leaving the crystallized substrate free and unbounded."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "SETTLED (P8): the P8 REFLECTION band emits the over-budget Action sourced FROM the operator's budget field, measuring only the live graph nodes (requirements+conflicts+assumptions) — the substrate itself is never counted. Bounding the substrate would punish the very act — crystallizing — that the budget rewards. Only un-offloaded working knowledge competes for context, so only it is metered. Implementation: tools/what_now.py + tools/tick.py."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("test_reflection.py",),
        ),
        Requirement(
            id="R-enforcement-gradient",
            claim=(
                "A requirement shall carry an enforcement level PROSE | STRUCTURAL "
                "| ENFORCED, and ENFORCED requirements shall name their enforcing "
                "invariant/test."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-enforcement-levels-declared + R-enforced-names-enforcer (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-enforcement-levels-declared + R-enforced-names-enforcer (wave 2, decided by framework-author 2026-06-30) — (was: Makes 'how deeply crystallized' measurable; pushes knowledge down toward enforced reflexes. A PROSE requirement is a wish; an ENFORCED one is a guarantee — naming the enforcer is what makes the difference auditable. When DRAFT >= SETTLED/2, the REFLECTION band fires on `burn-down` (M35: SETTLED:DRAFT ratio + UNENFORCED count). Promote, don't accrue.))"
            ),
            assumptions=("A-most-knowledge-crystallizable",),
            enforcement=ENFORCED,
            enforced_by=(
                "check_enforced_names_invariant",
                "test_docs_gen.py::test_unenforced_md_up_to_date",
            ),
        ),
        Requirement(
            id="R-requirement-enforced",
            claim=(
                "A SETTLED requirement that names no enforcing invariant or test is UNENFORCED (claimed-but-not-guaranteed, soft context-debt)."
            ),
            owner="framework-reviewer",
            status="SETTLED",
            why=(
                "A SETTLED promise with no enforcer is not actually offloaded — "
                "the operator must still watch it by hand. UNENFORCED marks the "
                "gap between a claim and its reflex so it can be closed."
            ),
            assumptions=("A-most-knowledge-crystallizable",),
            enforcement=ENFORCED,
            enforced_by=(
                "check_enforced_names_invariant",
                "test_docs_gen.py::test_unenforced_md_up_to_date",
            ),
        ),
        Requirement(
            id="R-uncrystallizable-is-missing-type",
            claim=(
                "Knowledge an operator cannot crystallize as any existing node "
                "shall be RECORDED as a candidate missing ontology type for steward "
                "review (not auto-acted)."
            ),
            owner="framework-reviewer",
            status="SETTLED",
            why=(
                "SETTLED (P6): the meta-signal surface exists — when the "
                "§Conscience Hypothesis property-sweep finds a class of "
                "contradictions that no existing critical-core invariant can "
                "express, the property-test failure IS the recording mechanism "
                "(a clear, machine-visible meta-signal that a new type is needed). "
                "The steward still decides whether to add the type; the recording "
                "itself is now structural, not manual."
            ),
            assumptions=("A-most-knowledge-crystallizable",),
            enforcement=STRUCTURAL,
            enforced_by=("test_conscience.py", "CRITICAL_CORE_INVARIANTS"),
        ),
        Requirement(
            id="R-stale-substrate",
            claim=(
                "Crystallized knowledge whose enforcing assumption has died shall be surfaced as stale (enforced-but-wrong, a bad habit)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "SETTLED (P6): the structural path now exists. The §Conscience critical-core sweep (test_conscience.py::test_real_meta_domain_passes_critical_core) flags any critical-core invariant that fires on the live meta-domain — including one resting on a DEAD assumption. Today the meta-domain has zero DEAD assumptions, so no stale substrate fires; the structural detection path is in place for when one does."
            ),
            assumptions=("A-compaction-loses-working",),
            enforcement="ENFORCED",
            enforced_by=("test_reflection.py::test_reflection_emits_dead_assumption_enforcer", "test_conscience.py::test_real_meta_domain_passes_critical_core"),
        ),
        # --- OPEN(question) — load-bearing open decisions (M17–M31) ----------
        Requirement(
            id="R-budget-measure",
            claim=(
                "The context budget shall be measured by a single declared metric "
                "so size(domain) <= budget.limit is computable."
            ),
            owner="framework-author",
            status=(
                "OPEN(how is context budget measured — node-count, token-estimate, "
                "complexity, or operator-self-reported working set?)"
            ),
            why=(
                "M17. The budget rule (R-context-budget-rule) is only structural "
                "once the metric is fixed; until then 'over budget' is a judgment, "
                "not a check."
            ),
            assumptions=("A-finite-context-operators",),
            m_tag="M17",
        ),
        Requirement(
            id="R-partition-vs-border",
            claim=(
                "Operator sub-domains shall relate to the parent graph by a single "
                "declared discipline (strict partition or declared-border overlap)."
            ),
            owner="framework-author",
            status=(
                "OPEN(do operator sub-domains strictly partition the graph, or "
                "overlap on explicitly-declared delegation borders?)"
            ),
            why=(
                "M18. Delegation (R-context-bounded-delegation) needs to know "
                "whether shared objects are forbidden (partition) or first-class "
                "borders; the two give different drift behavior."
            ),
            assumptions=("A-finite-context-operators",),
            m_tag="M18",
        ),
        Requirement(
            id="R-goal-type-vs-facet",
            claim=(
                "Goal shall be its own first-class frozen-dataclass type (not a "
                "Requirement facet), with typed anchor 'GOAL-' and its own "
                "GOAL_LIFECYCLE."
            ),
            owner="domain-user",
            status="SETTLED",
            why=(
                "M19. DECIDED 2026-06-30 (already recorded in old M-table as "
                "DECIDED P9): Goal is a new type in hotam_spec.process. Rationale: the "
                "Gap = (Goal - current state) is semantically distinct from a static "
                "Requirement claim; a Requirement facet would lose that target-state "
                "semantics and the burn-down-to-zero pattern. R-goal-is-first-class-"
                "type is already SETTLED and enforced by test_goal.py + "
                "check_typed_anchors_goal. Evidence: spec/src/hotam_spec/process.py:Goal "
                "frozen dataclass + GOAL_LIFECYCLE; R-goal-is-first-class-type "
                "SETTLED ENFORCED."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("test_goal.py", "check_typed_anchors_goal"),
        ),
        Requirement(
            id="R-operator-type-vs-facet",
            claim=(
                "Operator shall be its own first-class frozen-dataclass type in "
                "hotam_spec.operator (not a Stakeholder facet), with typed anchor 'OP-', "
                "a ContextBudget, and an optional parent reference."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "M20. DECIDED 2026-06-30: Operator is a new type. Rationale: a "
                "Stakeholder facet cannot carry a ContextBudget, enforce "
                "check_operator_within_budget, or be referenced by Goal.owner — all "
                "of which are live ENFORCED requirements. The clean separation "
                "(Stakeholder = party, Operator = acting facet with budget + "
                "capabilities) prevents conflation at the single-altitude-vs-multi-"
                "altitude axis. Evidence: spec/src/hotam_spec/operator.py:Operator "
                "frozen dataclass; R-operator-is-frozen-dataclass SETTLED ENFORCED; "
                "check_typed_anchors_operator live."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("check_typed_anchors_operator", "test_operator.py"),
        ),
        Requirement(
            id="R-observation-evidence-scope",
            claim=(
                "The methodology shall decide whether an operator's BELIEF about "
                "business state and its drift from reality (Observation/Evidence) "
                "is in scope."
            ),
            owner="framework-reviewer",
            status=(
                "OPEN(does the methodology model an operator's BELIEF about "
                "business state and its drift from reality (Observation/Evidence), "
                "or is that out of scope as epistemics-creep?)"
            ),
            why=(
                "M21. Modeling belief-vs-reality drift would extend the drift "
                "machinery to epistemics; it may be powerful or it may be scope "
                "creep beyond requirement contradiction."
            ),
            assumptions=("A-most-knowledge-crystallizable",),
            m_tag="M21",
        ),
        Requirement(
            id="R-rules-as-data",
            claim=(
                "The methodology's own rules/invariants shall be either first-class "
                "data it reasons about or remain code checks plus meta-domain "
                "requirements — one stance, declared."
            ),
            owner="framework-reviewer",
            status=(
                "OPEN(do the methodology's own rules/invariants become first-class "
                "data the methodology reasons about, or stay as code check_* plus "
                "meta-domain requirements?)"
            ),
            why=(
                "M22. Promoting rules to data deepens the reflexive bootstrap "
                "(R-two-altitude-ontology) but risks an infinite regress; staying "
                "as code keeps the framework grounded."
            ),
            assumptions=("A-bootstrap-self-applies",),
            m_tag="M22",
        ),
        Requirement(
            id="R-enforcement-first-class",
            claim=(
                "The enforcement level (PROSE / STRUCTURAL / ENFORCED) shall be a first-class Requirement field with enforced_by anchors naming the check_* or test_* that guarantees the claim."
            ),
            owner="framework-author",
            status=(
                "SETTLED"
            ),
            why=(
                "M26. DECIDED 2026-06-30: Requirement.enforcement is a first-class field (not a derived report) since P6. The ENFORCEMENT_LEVELS constant (PROSE/STRUCTURAL/ENFORCED) is declared in hotam_spec/requirement.py; check_enforced_names_invariant validates every ENFORCED requirement names a real enforcer. A derived report would be inconsistent with check_bijection_r_to_enforcer which requires the field to be authoritative. Evidence: spec/src/hotam_spec/requirement.py:ENFORCEMENT_LEVELS + PROSE/STRUCTURAL/ENFORCED constants; check_enforced_names_invariant in invariants.py; R-enforcement-levels-declared SETTLED."
            ),
            assumptions=("A-most-knowledge-crystallizable",),
            enforcement="ENFORCED",
            enforced_by=("check_enforced_names_invariant", "check_bijection_r_to_enforcer"),
        ),
        Requirement(
            id="R-anchor-taxonomy",
            claim=(
                "The typed-anchor prefix set (R-/C-/A-/OP-/GOAL-/PR-/§) is frozen, with Axis.slug staying bare because axes are identified by slug within the graph's axes tuple rather than globally."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "M28. DECIDED 2026-06-30: the prefix set is frozen by the "
                "check_typed_anchors_* family — one invariant per node type enforces "
                "the exact prefix. check_typed_anchors_requirement (R-), "
                "check_typed_anchors_assumption (A-), check_typed_anchors_conflict "
                "(C-), check_typed_anchors_operator (OP-), check_typed_anchors_process "
                "(PR-), check_typed_anchors_goal (GOAL-) are all live in "
                "invariants.ALL_INVARIANTS. Axis.slug is bare because "
                "check_axis_in_registry validates by exact slug match within the "
                "graph; a prefix would introduce redundancy. §-anchors are validated "
                "by check_section_anchors_known against the glossary. The full set: "
                "R-/C-/A-/OP-/GOAL-/PR-/§. Evidence: spec/src/hotam_spec/invariants.py "
                "check_typed_anchors_* functions; R-anchor-everything SETTLED ENFORCED."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=(
                "check_typed_anchors_requirement",
                "check_typed_anchors_assumption",
                "check_typed_anchors_conflict",
                "check_typed_anchors_operator",
                "check_typed_anchors_process",
                "check_typed_anchors_goal",
                "check_typed_anchors_entity",
                "check_section_anchors_known",
            ),
        ),
        Requirement(
            id="R-uncrystallizable-automated",
            claim=(
                "Detection of 'uncrystallizable knowledge = missing type' is human "
                "judgment: the operator records the candidate in the graph as an OPEN "
                "requirement (or a DRAFT), and the steward decides whether to add "
                "the ontology type."
            ),
            owner="framework-reviewer",
            status="SETTLED",
            why=(
                "M30. DECIDED 2026-06-30: human judgment, not automated. Rationale: "
                "R-uncrystallizable-is-missing-type (SETTLED P6) already establishes "
                "that the operator records the signal as a node; the §Conscience "
                "property-sweep (test_conscience.py) surfaces the meta-signal "
                "structurally when a class of contradictions cannot be expressed by "
                "existing critical-core invariants. Automating the type-creation "
                "decision would violate R-ai-presents-not-decides. The whole "
                "audit-backlog-residue checkpoint pattern + the DRAFT queue IS the "
                "recording mechanism. The steward is the decider; the graph is the "
                "recorder. Evidence: R-uncrystallizable-is-missing-type SETTLED "
                "STRUCTURAL; test_conscience.py; R-ai-presents-not-decides SETTLED."
            ),
            assumptions=("A-most-knowledge-crystallizable",),
            enforcement=STRUCTURAL,
            enforced_by=("test_conscience.py", "CRITICAL_CORE_INVARIANTS"),
        ),
        # --- P10a: generated LIVE-STATE block in CLAUDE.md -------------------
        Requirement(
            id="R-claude-md-live-state-generated",
            claim=(
                "The live numeric state in CLAUDE.md (top action, debt counts, graph "
                "size, crystal headroom, context) shall be generated by gen_spec into a "
                "sentinel-delimited block, never hand-written."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Commit 36ceabd hand-wrote 'Today: 15 unenforced' into CLAUDE.md — the "
                "auto-loaded file — and it drifted to 16 within one phase. The U5 lesson "
                "(single source + generated mirror) applied to the operator's own "
                "crystal. gen_spec is the 'hook that updates it with the logic run'."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_docs_gen.py::test_claude_md_live_state_up_to_date",),
        ),
        Requirement(
            id="R-measure-context-size",
            claim=(
                "The operator's working-context fullness shall be MEASURED from a "
                "runtime stamp, not estimated, so the three-cipher pulse cites a real "
                "number."
            ),
            owner="ai-agent",
            status="DRAFT",
            why=(
                "tools/context.py reads spec/.runtime/context.json; the producing hook "
                "lives in the user's global ~/.claude settings (cah-stamp emits "
                "context %) — installing it is a STEWARD decision outside the framework "
                "body, so this stays DRAFT until the hook is approved and wired."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=("tools/context.py",),
        ),
        # --- DRAFT/OPEN — P10c: deferred backend + budget + crystal-tree -----
        Requirement(
            id="R-operator-backend-protocol",
            claim=(
                "The framework's tools shall talk to the acting agent through a single "
                "OperatorBackend protocol (get_context_state / request_steward_approval "
                "/ delegate), so the methodology does not hard-depend on which "
                "coding-agent or model drives it."
            ),
            owner="framework-author",
            status="DRAFT",
            why=(
                "Today tools/ implicitly assume Claude Code (Agent tool, Bash, chat-"
                "steward). BUILD-TRIGGER: a SECOND concrete backend becomes real (CI "
                "runner, a different coding agent, or a programmatic steward). Until "
                "then, abstracting for hypothetical backends is the big-bang-up-front "
                "antipattern (weight ∝ cost of an unnoticed conflict). See OPEN "
                "R-backend-scope (which backends are real?)."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-claude-md-budget-phi-cap",
            claim=(
                "CLAUDE.md (the director's crystal) shall not exceed 1_000_000 / φ ≈ "
                "618033 tokens; on approach, the operator crystallizes/splits rather "
                "than letting the crystal swell."
            ),
            owner="framework-author",
            status="DRAFT",
            why=(
                "The context-bounded-delegation law (R-context-bounded-delegation) "
                "applied to the operator's OWN body, not just the graph. BUILD-TRIGGER: "
                "CLAUDE.md crosses ~50% of the φ-cap (~309K tokens) — today it is ~7K "
                "(~1%), so a budget CHECK now would be machinery guarding a condition "
                "that cannot fire. The LIVE-STATE block already reports φ-headroom; the "
                "check + the REFLECTION P0 wiring land when headroom actually narrows."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-claude-md-tree-of-crystals",
            claim=(
                "When the root CLAUDE.md approaches its φ-cap, the operator shall move "
                "sections into nested <subdir>/CLAUDE.md crystals and keep only a "
                "heading + a when-to-read pointer in the root — a tree of crystals, one "
                "per sub-domain."
            ),
            owner="framework-author",
            status="DRAFT",
            why=(
                "R-operator-crystal-is-claude-md made recursive: the delegation "
                "hierarchy is a tree of CLAUDE.md crystals (Claude Code natively loads "
                "nested CLAUDE.md by directory). BUILD-TRIGGER: R-claude-md-budget-phi-"
                "cap fires (the root crystal nears the cap). Blocked-by that trigger; "
                "premature today."
            ),
            assumptions=("A-finite-context-operators", "A-bootstrap-self-applies"),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-subagent-gets-its-claude-md",
            claim=(
                "A delegated sub-operator shall receive its OWN crystal (a CLAUDE.md generated from its sub-domain) and return CONCLUSIONS only, never raw context, to the root operator."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "The Delegation.returns conclusions-only contract (R-delegation-conclusions-only) made concrete for sub-operators. BUILD-TRIGGER: spawn_agent built — NOW FIRES (P22.C). Promoted DRAFT->SETTLED on P22.C: spawn_agent tool exists at spec/tools/spawn_agent.py, composing per-agent CLAUDE.md into the subagent prompt before dispatch."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("test_composite_prompt_contains_crystal_and_task",),
        ),
        Requirement(
            id="R-backend-scope",
            claim=(
                "Which alternative operator backends are real enough to design the "
                "OperatorBackend protocol against?"
            ),
            owner="framework-author",
            status="OPEN(which backends beyond Claude Code are real targets — CI runner / a different coding agent / a programmatic or human steward — so the protocol is designed against concrete cases, not hypotheticals?)",
            why=(
                "Gates R-operator-backend-protocol. The protocol must be shaped by real "
                "backends or it over-engineers. Steward names the targets; until then "
                "the protocol stays DRAFT and unbuilt."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=(),
            m_tag="M37",
        ),
        # --- SETTLED — P11 new: convergence, atomicity, agent-directory, tools, docs ---
        Requirement(
            id="R-prefer-tool-over-hand",
            claim=(
                "The operator shall prefer a reusable tool over performing the same action by hand, with one-off acts permitted only for genuine bootstrap or single-occurrence events."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Today's third architectural principle. Cannot be algorithmically enforced (no AST detection of 'you did it by hand'); STRUCTURAL via prose discipline in the operator-prompt + a generated discipline doc. Use SETTLED (not DRAFT) — the principle is now in force; the structural enforcement is the prose."
            ),
            assumptions=("A-stakeholders-care",),
            enforcement="STRUCTURAL",
            enforced_by=("CLAUDE.md§Operator boot ritual", "docs/methodology/discipline.md"),
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-shared-tools-in-spec-tools",
            claim=("Tools available to all agents shall live in `spec/tools/`."),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Scoping rule, structurally enforced by file layout. SETTLED — "
                "already true today."
            ),
            assumptions=("A-python-stack",),
            enforcement=STRUCTURAL,
            enforced_by=("file layout", "docs/methodology/discipline.md"),
        ),
        Requirement(
            id="R-docs-generated-from-requirements",
            claim=(
                "Per-topic narrative files under `docs/methodology/atoms/<topic>.md` shall be generated from SETTLED requirements grouped by topic, with hand-edits forbidden by a meta-test."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Director asked: 'спеку-доку, которые будут генерировать описания "
                "в папке docs'. BUILD: this phase. Subdirectory "
                "`docs/methodology/atoms/` keeps generated files cleanly separate "
                "from the existing hand-written `docs/methodology/README.md`."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=(
                "test_docs_gen.py::test_methodology_atoms_up_to_date",
                "tools/gen_spec.py::build_methodology_atoms",
            ),
        ),
        # --- SETTLED — orphan-check anchoring (framework-agent audit) ----------
        Requirement(
            id="R-conflict-structurally-visible",
            claim=(
                "Every Conflict node shall carry a non-empty axis, context, "
                "and steward."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "These three fields are the knowledge that makes the "
                "contradiction visible (R-conflict-is-connector-node). A "
                "Conflict missing any of them is an invisible contradiction. "
                "Atomized claim — one structural rule with three required "
                "fields, all enforced by the same check_*."
            ),
            assumptions=("A-stakeholders-care",),
            enforcement=ENFORCED,
            enforced_by=(
                "check_conflict_has_axis",
                "check_conflict_has_context",
                "check_conflict_has_steward",
            ),
        ),
        Requirement(
            id="R-conflict-min-two-members",
            claim=(
                "Every Conflict node shall contain at least two distinct "
                "Requirement ids in its members tuple."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "A connector node connects at least two parties. Single-member "
                "'conflicts' are degenerate. Was previously an orphan check "
                "(no R claimed it); now anchored."
            ),
            assumptions=("A-stakeholders-care",),
            enforcement=ENFORCED,
            enforced_by=("check_conflict_min_two_members",),
        ),
        Requirement(
            id="R-decided-conflict-justifies-itself",
            claim=(
                "Every Conflict in DECIDED lifecycle shall carry either a "
                "non-empty rationale in DECIDED(...) or at least one derived "
                "Requirement."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Anti-relitigation — a DECIDED conflict without recorded "
                "reasoning gets re-litigated. Was orphan check; now claimed. "
                "Distinct from R-decided-needs-human-signoff (about decided_by "
                "attribution) — this is about the resolution's justification."
            ),
            assumptions=("A-stakeholders-care",),
            enforcement=ENFORCED,
            enforced_by=("check_decided_has_rationale_or_derived",),
        ),
        Requirement(
            id="R-m-tag-format-valid",
            claim=(
                "Every Requirement.m_tag (when non-empty) shall match "
                "`^M[1-9][0-9]*$`, be unique across the graph, and appear "
                "only on OPEN requirements."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The M-decision registry (DECISIONS.md) bijection depends on "
                "m_tag discipline. Was orphan policy; now claimed. Atomic — "
                "one concern (M-tag well-formedness) with three sub-rules all "
                "in one check_*; if we later split the check, this R splits "
                "with it."
            ),
            assumptions=("A-prose-suffices",),
            enforcement=ENFORCED,
            enforced_by=(
                "check_m_tag_valid_format",
                "check_m_tag_unique",
                "check_m_tag_open_only",
            ),
        ),
        # --- DRAFT — P11 new: convergence, atomicity, agent-dir, delegation, tools ---
        Requirement(
            id="R-operator-prompt-from-substrate",
            claim=(
                "The operator-prompt CLAUDE.md shall include a CONSTITUTION block "
                "listing all SETTLED requirements grouped by category, generated "
                "deterministically from spec/content/graph.py."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Realizes the sensor-substrate inversion: consciousness (the "
                "operator-prompt) is GENERATED from code. The atomized SETTLEDs "
                "are now the actual constitution; gen_spec emits them into CLAUDE.md "
                "between CONSTITUTION sentinels."
            ),
            assumptions=("A-compaction-loses-working", "A-bootstrap-self-applies"),
            enforcement=ENFORCED,
            enforced_by=("test_constitution_block_generated",),
        ),
        Requirement(
            id="R-constituting-requirements-converge",
            claim=(
                "The set of SETTLED requirements composing the operator-prompt shall be pairwise consistent on declared axes, with structural contradictions between constituting atoms forbidden."
            ),
            owner="framework-reviewer",
            status="SETTLED",
            why=(
                "SETTLED (BUILD-TRIGGER fired): R-operator-prompt-from-substrate "
                "has landed — gen_spec generates the CONSTITUTION block from SETTLED "
                "requirements, making the constituting set explicit and machine-known. "
                "Convergence is structurally expressed by the fact that all SETTLED "
                "atoms pass the conflict invariants before any is emitted into the "
                "constitution block; an atom that introduced a new structural violation "
                "would be caught by pytest before it could be promoted. Pair detection "
                "via the `axes` discipline at the meta-domain altitude is the next "
                "layer; STRUCTURAL enforcement covers the achieved level today."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=STRUCTURAL,
            enforced_by=(),
        ),
        Requirement(
            id="R-requirement-claim-is-atomic",
            claim=(
                "Each `Requirement.claim` shall assert exactly one concern, with conjunctions of distinct concerns decomposed into separate requirements."
            ),
            owner="framework-reviewer",
            status="SETTLED",
            why=(
                "SETTLED (BUILD-TRIGGER fired): tools/audit_atomicity.py exists and "
                "surfaces Requirements with compound claims as a deterministic audit "
                "signal (docs/gen/AUDIT.md). Waves 1-3 of atomization applied the "
                "discipline to the meta-domain, decomposing compound requirements "
                "into single-concern atoms. The tool is the machine-readable enforcer "
                "of the discipline; STRUCTURAL because the check is advisory (P0 "
                "REFLECTION) rather than a P1 invariant that blocks pytest."
            ),
            assumptions=("A-prose-suffices",),
            enforcement=STRUCTURAL,
            enforced_by=("tools/audit_atomicity.py",),
        ),
        Requirement(
            id="R-check-method-is-atomic",
            claim=(
                "Each `check_*` invariant shall enforce exactly one rule, with multi-rule enforcers split into separate `check_*` functions."
            ),
            owner="framework-reviewer",
            status="SETTLED",
            why=(
                "SETTLED (BUILD-TRIGGER fired): tools/audit_atomicity.py flags "
                "check_* functions with compound conditions in docs/gen/AUDIT.md "
                "(same wave as R-requirement-claim-is-atomic). The tool walks "
                "invariants.py and reports multi-rule families, making the "
                "discipline machine-auditable. STRUCTURAL because the check is "
                "advisory (P0 REFLECTION audit output) not a P1 blocking invariant."
            ),
            assumptions=("A-prose-suffices",),
            enforcement=STRUCTURAL,
            enforced_by=("tools/audit_atomicity.py",),
        ),
        Requirement(
            id="R-bijection-r-to-enforcer-draft",
            claim=(
                "Each ENFORCED Requirement shall name exactly one enforcer in its "
                "`enforced_by` after atomization is complete."
            ),
            owner="framework-reviewer",
            status="REJECTED",
            why=(
                "REJECTED — SUPERSEDED by R-bijection-r-to-enforcer SETTLED (wave 3 "
                "outcome). The SETTLED version generalizes this claim: every "
                "SETTLED/ENFORCED requirement must name an existing check_* in "
                "ALL_INVARIANTS or a real test_*, enforced by check_bijection_r_to_enforcer. "
                "The original id was duplicated with the SETTLED version; renamed "
                "to R-bijection-r-to-enforcer-draft for history preservation."
            ),
            assumptions=("A-prose-suffices",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-agent-is-a-directory",
            claim=(
                "A domain-agent shall be represented as a directory at "
                "`spec/agents/<name>/`."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The user's clarification today: agent = folder with own logic, "
                "not sh-invocation. BUILD-TRIGGER: a real second operator (beyond "
                "OP-director) needs to be instantiated. "
                "Promoted DRAFT→SETTLED on first instantiation: "
                "spec/agents/framework-agent/ exists as concrete evidence."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-agent-has-own-crystal",
            claim=(
                "Each domain-agent shall carry its own `CLAUDE.md` file as its "
                "operator-prompt crystal."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "The agent's prompt is independent of the director's. "
                "BUILD-TRIGGER: same as R-agent-is-a-directory. "
                "Promoted DRAFT→SETTLED on first instantiation: "
                "spec/agents/framework-agent/CLAUDE.md generated and populated."
            ),
            assumptions=("A-finite-context-operators", "A-compaction-loses-working"),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-agent-has-own-tools-dir",
            claim=(
                "Each domain-agent shall carry a `tools/` subdirectory holding its "
                "private tools."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Scoping the agent's available actions — private tools are not "
                "exposed to other agents. BUILD-TRIGGER: same as R-agent-is-a-directory. "
                "Promoted DRAFT→SETTLED on first instantiation: "
                "spec/agents/framework-agent/tools/ exists as concrete evidence."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-agent-imports-framework",
            claim=(
                "An agent's code shall import the framework body (`hotam_spec.*`) as "
                "shared infrastructure; the framework body is owned by no single agent."
            ),
            owner="framework-author",
            status="DRAFT",
            why=(
                "Keeps the framework content-free (R-content-free-framework) while "
                "letting agents specialize. BUILD-TRIGGER: same as R-agent-is-a-directory."
            ),
            assumptions=("A-content-free-honest",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-task-spawn-is-ephemeral",
            claim=(
                "A task-agent invocation (a sh/Agent-tool call) is a hand: it "
                "returns conclusions and does not persist between invocations."
            ),
            owner="ai-agent",
            status="DRAFT",
            why=(
                "The user's distinction today: hands vs agents. BUILD-TRIGGER: "
                "D3's spawn-log writer exists — the log is the structural recording "
                "of this ephemeral act."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-domain-delegation-persists",
            claim=(
                "A domain-delegation shall persist as a directory + a substrate node "
                "(`Delegation`)."
            ),
            owner="framework-author",
            status="DRAFT",
            why=(
                "Distinct from task-spawn: domain-delegation is recorded in the "
                "graph and stewardable. BUILD-TRIGGER: the `Delegation` node type "
                "(R-domain-delegation-as-node) is built."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-task-spawn-log-runtime",
            claim=(
                "Task-agent invocations shall be appended to "
                "`spec/.runtime/spawn-log.jsonl` with parent, child kind, task "
                "subject, and stamp."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Ephemera, not committed substrate — same altitude as "
                "`spec/.runtime/context.json`. BUILD-TRIGGER: spawn-log infrastructure "
                "built — NOW FIRES (P22.C). Promoted DRAFT->SETTLED on P22.C: "
                "spawn_agent tool writes spawn-log.jsonl entries on every invocation."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=ENFORCED,
            enforced_by=("test_spawn_log_written",),
        ),
        Requirement(
            id="R-tools-registry-generated",
            claim=(
                "The list of available tools shall be generated by scanning "
                "`spec/tools/*.py` (and per-agent `spec/agents/<name>/tools/*.py`), "
                "never hand-maintained."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "SETTLED (BUILD-TRIGGER fired): gen_spec.py auto-projects R-tool-* "
                "requirements from Canon docstrings in spec/tools/*.py (lines ~220-266 "
                "and ~1614-1789 in gen_spec.py). The REPO-MAP and AGENT-MAP blocks in "
                "CLAUDE.md include tool entries generated from the filesystem scan, "
                "never hand-maintained. The docs-as-code pattern applied to tool "
                "inventories — a new tool without a Canon docstring simply won't "
                "appear, making drift structurally visible."
            ),
            assumptions=("A-python-stack",),
            enforcement=STRUCTURAL,
            enforced_by=("test_repo_map_complete",),
        ),
        Requirement(
            id="R-private-tools-in-agent-folder",
            claim=(
                "Tools available only to one agent shall live under that agent's "
                "`tools/` subdirectory."
            ),
            owner="framework-author",
            status="DRAFT",
            why=(
                "Counterpart to R-shared-tools-in-spec-tools — private scope. "
                "BUILD-TRIGGER: R-agent-is-a-directory and R-agent-has-own-tools-dir "
                "have landed; a real agent has private tools."
            ),
            assumptions=("A-python-stack",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-tree-of-crystals-cognitive-trigger",
            claim=(
                "Tree-of-crystals delegation shall fire when a sub-domain's detail "
                "granularity exceeds the director's altitude, independently of the "
                "φ-cap size trigger."
            ),
            owner="framework-author",
            status="DRAFT",
            why=(
                "Second trigger besides R-claude-md-budget-phi-cap: cognitive load, "
                "not token load. BUILD-TRIGGER: a heuristic detector exists (planned: "
                "count of distinct concerns per sub-domain crossing a threshold)."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-domain-delegation-as-node",
            claim=(
                "A domain-delegation shall be recorded as a `Delegation` substrate "
                "node with fields parent_op, child_op, scope, border, "
                "returns_contract, crystal_path."
            ),
            owner="framework-author",
            status="DRAFT",
            why=(
                "Makes the delegation persistent and stewardable, unlike task-spawn "
                "ephemera. BUILD-TRIGGER: R-agent-is-a-directory through "
                "R-agent-imports-framework have landed (agents exist as directories) "
                "AND a first real delegation is performed."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-setup-claude-generates-settings",
            claim=(
                "The file `.claude/settings.json` shall be generated by "
                "`spec/tools/setup_claude.py` deterministically; hand-edits are "
                "forbidden by a meta-test."
            ),
            owner="framework-author",
            status="DRAFT",
            why=(
                "Single source of truth: Python file generates the JSON; meta-test "
                "enforces equality. BUILD-TRIGGER: the prerequisite atomization "
                "(R-requirement-claim-is-atomic and R-check-method-is-atomic) has "
                "landed — without it, setup_claude.py easily slides back into "
                "compoundness."
            ),
            assumptions=("A-python-stack",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-audit-atomicity-tool",
            claim=(
                "An audit of substrate atomicity (compound requirements + "
                "compound check_* invariants + R↔enforcer bijection + orphan "
                "analysis) shall be performed by a deterministic tool "
                "`spec/tools/audit_atomicity.py`, not by a one-off hand "
                "invocation."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "SETTLED (BUILD-TRIGGER fired): spec/tools/audit_atomicity.py "
                "exists and was used in atomization waves 1-3, emitting "
                "docs/gen/AUDIT.md deterministically. The verdict checkpoint "
                "(docs/checkpoints/framework-agent-audit-verdict.md) is now "
                "superseded by the tool's output. R-prefer-tool-over-hand is "
                "now honored for atomicity audits. STRUCTURAL because the "
                "tool's output is advisory (P0 REFLECTION); no blocking "
                "invariant yet."
            ),
            assumptions=("A-python-stack",),
            enforcement=STRUCTURAL,
            enforced_by=("tools/audit_atomicity.py",),
        ),
        Requirement(
            id="R-context-hook-piggybacks-cah-stamp",
            claim=(
                "The PostToolUse + Stop hook in `.claude/settings.json` shall read "
                "the global cah-stamp cache and write `spec/.runtime/context.json`."
            ),
            owner="framework-author",
            status="DRAFT",
            why=(
                "Honest ctx_pct stays null (deferred); the 5h/weekly/effort fields "
                "from cah-stamp are populated. BUILD-TRIGGER: "
                "R-setup-claude-generates-settings has landed."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        # --- REJECTED — design dead-ends preserved for history ---------------
        Requirement(
            id="R-seed-in-src",
            claim=(
                "The framework shall ship with a seed graph baked into "
                "spec/src/hotam_spec/graph.py so the demo runs without setup."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES by R-content-free-framework. The framework "
                "must ship blank; the seed graph leaked business content (R-87, "
                "A-single-customer, axes like latency-vs-completeness) into the "
                "framework package, breaking the framework / content split."
            ),
            assumptions=(),
        ),
        Requirement(
            id="R-rdf-store",
            claim=(
                "The tension graph shall be persisted in an RDF triplestore "
                "with SHACL shapes and SPARQL traversal."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES by storage = the Python code itself. "
                "RDF adds a heavy parallel substrate over what frozen "
                "dataclasses + plain-function traversal already do; SHACL "
                "duplicates the check_* invariants; SPARQL is unnecessary at "
                "the in-memory graph sizes we serve."
            ),
            assumptions=(),
        ),
        Requirement(
            id="R-axes-as-module-constant",
            claim=(
                "The controlled vocabulary of axes shall live as a module-"
                "level REGISTRY in hotam_spec.axis."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES by per-graph TensionGraph.axes. A "
                "module-level registry bakes a specific business vocabulary "
                "(latency-vs-completeness, privacy-vs-analytics, ...) into "
                "the content-free framework. Each domain owns its own "
                "vocabulary."
            ),
            assumptions=(),
        ),
        Requirement(
            id="R-content-free-no-business-data",
            claim=(
                "The framework spec/src/hotam_spec/ shall ship no business data (no example requirements, no example axes, no business stakeholders)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-content-free-framework (no-business-data concern). D1 split decided by domain-user 2026-06-30. WHY: business data in the framework source would couple it to a specific domain, violating content-free neutrality."
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_content_free.py::test_no_domain_instances_in_tensio_src",),
        ),
        Requirement(
            id="R-content-free-no-examples",
            claim=(
                "The framework shall not include illustrative example Requirement(...) calls in its source modules, keeping worked examples in spec/tests/fixtures/seed.py loaded only via --demo."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-content-free-framework (no-examples concern). D1 split decided by domain-user 2026-06-30. WHY: example data in src/ drifts from the fixture and misleads adopters into thinking it is real content."
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_content_free.py::test_no_domain_instances_in_tensio_src",),
        ),
        Requirement(
            id="R-content-free-no-seed-graph",
            claim=(
                "The framework shall not embed a seed TensionGraph -- load_content_graph() discovers the user's graph by convention from spec/content/graph.py:build_graph()."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-content-free-framework (no-seed-graph concern). D1 split decided by domain-user 2026-06-30. WHY: a baked-in seed graph forces every adopter to delete example data before starting, and risks silent merge conflicts."
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_content_free.py::test_no_domain_instances_in_tensio_src",),
        ),
        Requirement(
            id="R-empty-content-wellformed",
            claim=(
                "A freshly-cloned framework with no spec/content/graph.py shall pass all structural invariants — an empty graph is well-formed."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-empty-content-is-legitimate (well-formedness concern). D2 split decided by domain-user 2026-06-30. WHY: if an empty graph is malformed, the framework punishes adopters for having nothing to model yet."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_invariants.py::test_empty_graph_is_wellformed",),
        ),
        Requirement(
            id="R-empty-content-calm-banner",
            claim=(
                "When spec/content/graph.py is absent, tools/what_now.py shall render a calm 'no content yet' banner, not an error."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-empty-content-is-legitimate (calm-banner concern). D2 split decided by domain-user 2026-06-30. WHY: an error on missing content scares off new adopters who have not populated their domain yet."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=(
                "test_what_now.py::test_main_empty_content_prints_calm_banner",
            ),
        ),
        Requirement(
            id="R-empty-content-gen-notice",
            claim=(
                "When spec/content/graph.py is absent, tools/gen_spec.py shall emit a 'no content yet' notice into docs/gen/*.md, not fail."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-empty-content-is-legitimate (gen-notice concern). D2 split decided by domain-user 2026-06-30. WHY: failing on missing content would block the regen step of the closed loop for new adopters."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=(
                "test_docs_gen.py::test_empty_graph_renders_no_content_notice",
            ),
        ),
        Requirement(
            id="R-boot-reload-three-facts",
            claim=(
                "The operator shall begin every new turn by re-loading three facts from the substrate: current context %, the top what_now action, and the SETTLED-DRAFT-UNENFORCED ratio."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Atom of R-boot-from-substrate (WHAT to load). Without re-loading from the substrate, the operator lives in session memory and drifts from the graph's live state."
            ),
            assumptions=("A-compaction-loses-working", "A-bootstrap-self-applies"),
            enforcement=ENFORCED,
            enforced_by=("test_reflection.py",),
        ),
        Requirement(
            id="R-boot-cite-in-first-sentence",
            claim=(
                "The operator shall cite at least one of the three substrate facts in the first sentence of any substantive reply."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Atom of R-boot-from-substrate (WHEN to cite). Citing anchors the reply in the live substrate, proving the operator actually loaded it and is not parroting from memory."
            ),
            assumptions=("A-compaction-loses-working", "A-bootstrap-self-applies"),
            enforcement="PROSE",
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-glossary-generated",
            claim=(
                "A controlled vocabulary of methodology terms shall be generated under docs/gen/GLOSSARY.md."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-glossary-sync-test (generation concern). Without a generated glossary, terminology drift is invisible."
            ),
            assumptions=("A-prose-suffices", "A-python-stack"),
            enforcement="ENFORCED",
            enforced_by=("test_docs_gen.py::test_glossary_md_up_to_date",),
        ),
        Requirement(
            id="R-glossary-sync-fails-dead",
            claim=(
                "The glossary sync test shall fail when a defined vocabulary term is not used anywhere in the framework."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-glossary-sync-test (dead-term concern). Dead terms accumulate noise and mislead operators."
            ),
            assumptions=("A-prose-suffices", "A-python-stack"),
            enforcement="ENFORCED",
            enforced_by=("test_glossary_sync.py",),
        ),
        Requirement(
            id="R-glossary-sync-fails-unused",
            claim=(
                "The glossary sync test shall fail when a section-anchor token used in the framework is absent from the glossary."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-glossary-sync-test (undefined-term concern). An undefined section-anchor is an unresolved reference that breaks speak-by-reference."
            ),
            assumptions=("A-prose-suffices", "A-python-stack"),
            enforcement="ENFORCED",
            enforced_by=("test_glossary_sync.py",),
        ),
        Requirement(
            id="R-glossary-drift-stable",
            claim=(
                "The committed docs/gen/GLOSSARY.md shall equal the regeneration of the current graph byte-for-byte."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-glossary-sync-test (drift-stability concern). Same anti-drift discipline as R-drift-structurally-impossible applied to the glossary."
            ),
            assumptions=("A-prose-suffices", "A-python-stack"),
            enforcement="ENFORCED",
            enforced_by=("test_docs_gen.py::test_glossary_md_up_to_date",),
        ),
        Requirement(
            id="R-history-generated-from-rejected",
            claim=(
                "docs/gen/HISTORY.md shall include entries generated from REJECTED markers in requirement WHY blocks."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Atom of R-history-from-rejected-markers (REJECTED source stream). REJECTED requirements are the historian's primary source for design dead-ends."
            ),
            assumptions=("A-prose-suffices",),
            enforcement="ENFORCED",
            enforced_by=(
                "test_history_gen.py",
                "test_docs_gen.py::test_history_md_up_to_date",
            ),
        ),
        Requirement(
            id="R-history-generated-from-decided",
            claim=(
                "docs/gen/HISTORY.md shall include entries generated from DECIDED and REVISIT_WHEN lifecycle states on Conflicts."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Atom of R-history-from-rejected-markers (DECIDED/REVISIT source stream). Conflict decisions are the historian's primary source for resolved tensions."
            ),
            assumptions=("A-prose-suffices",),
            enforcement="ENFORCED",
            enforced_by=(
                "test_history_gen.py",
                "test_docs_gen.py::test_history_md_up_to_date",
            ),
        ),
        Requirement(
            id="R-lifecycle-type-exists",
            claim=(
                "A generic hotam_spec.lifecycle module shall define State, Transition, and Lifecycle types."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-lifecycle-abstraction (type-existence concern). The Lifecycle type is the keystone that Requirement.status and Conflict.lifecycle validate against."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement="ENFORCED",
            enforced_by=("test_lifecycle.py",),
        ),
        Requirement(
            id="R-lifecycle-validates-requirement",
            claim=(
                "Requirement.status shall validate against the framework-supplied REQUIREMENT_STATUS_LIFECYCLE."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-lifecycle-abstraction (requirement validation concern). check_status_in_lifecycle validates Requirement statuses on every invariant run."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement="ENFORCED",
            enforced_by=("check_requirement_status_in_lifecycle",),
        ),
        Requirement(
            id="R-lifecycle-validates-conflict",
            claim=(
                "Conflict.lifecycle shall validate against the framework-supplied CONFLICT_LIFECYCLE."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-lifecycle-abstraction (conflict validation concern). check_status_in_lifecycle validates Conflict lifecycles on every invariant run."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement="ENFORCED",
            enforced_by=("check_conflict_lifecycle_in_lifecycle",),
        ),
        Requirement(
            id="R-lifecycle-validates-operator",
            claim=(
                "Operator.lifecycle shall validate against the framework-supplied OPERATOR_LIFECYCLE."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-lifecycle-abstraction (operator validation concern). check_operator_lifecycle_in_lifecycle validates Operator lifecycles on every invariant run."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement="ENFORCED",
            enforced_by=("check_operator_lifecycle_in_lifecycle",),
        ),
        Requirement(
            id="R-lifecycle-validates-goal",
            claim=(
                "Goal.lifecycle shall validate against the framework-supplied GOAL_LIFECYCLE."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-lifecycle-abstraction (goal validation concern). check_goal_lifecycle_in_lifecycle validates Goal lifecycles on every invariant run."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement="ENFORCED",
            enforced_by=("check_goal_lifecycle_in_lifecycle",),
        ),
        Requirement(
            id="R-process-types-exist",
            claim=(
                "hotam_spec.process shall define Process, Step, Goal, TargetState, PROCESS_LIFECYCLE, and GOAL_LIFECYCLE types."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-process-aspect-first (type-existence concern). These types are the behavioral surface of the first opt-in aspect."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("test_process.py",),
        ),
        Requirement(
            id="R-process-opt-in",
            claim=(
                "The Process aspect shall be opt-in: TensionGraph.processes defaults to an empty tuple."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-process-aspect-first (opt-in concern). Core cost must not be imposed on domains that do not model processes."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("test_process.py",),
        ),
        Requirement(
            id="R-process-lifecycle-wellformed-aspect",
            claim=(
                "Every Process node shall have a well-formed lifecycle validated by check_process_lifecycle_wellformed."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-process-aspect-first (lifecycle wellformedness concern). A Process with an invalid lifecycle is structurally broken."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("check_process_lifecycle_wellformed",),
        ),
        Requirement(
            id="R-process-roles-declared-aspect",
            claim=(
                "Every role referenced in a Process step shall be declared in the Process roles_required tuple."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-process-aspect-first (roles-declared concern). Undeclared roles are dangling references in the behavioral model."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("check_process_roles_declared",),
        ),
        Requirement(
            id="R-process-goal-owner-is-operator-aspect",
            claim=(
                "Every Goal.owner shall reference an existing Operator.id, validated by check_goal_owner_is_operator."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-process-aspect-first (goal-owner concern). A Goal without a valid operator owner is unstewardable."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("check_goal_owner_is_operator",),
        ),
        Requirement(
            id="R-process-typed-anchors-extended",
            claim=(
                "check_typed_anchors shall validate PR- and GOAL- prefixes for Process and Goal nodes."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-process-aspect-first (typed-anchors concern). Without prefix validation, Process and Goal nodes escape the anchoring discipline."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("check_typed_anchors_process", "check_typed_anchors_goal"),
        ),
        Requirement(
            id="R-entity-type-lifecycle-wellformed",
            claim=(
                "Every EntityType.lifecycle shall be a well-formed Lifecycle validated by check_entity_type_lifecycle_wellformed."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of §Entity aspect (lifecycle wellformedness concern). An EntityType with "
                "an invalid lifecycle is structurally broken — check_entity_type_lifecycle_wellformed "
                "reuses check_lifecycle_wellformed (the §Lifecycle keystone) for zero-parallel machinery (M12)."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement=ENFORCED,
            enforced_by=("check_entity_type_lifecycle_wellformed",),
        ),
        Requirement(
            id="R-entity-instance-state-in-lifecycle",
            claim=(
                "Every EntityInstance.state shall be a valid state in the corresponding EntityType.lifecycle."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of §Entity aspect (state validity concern). An instance with an unknown state "
                "is structurally invalid — mirrors check_requirement_status_in_lifecycle for requirements."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement=ENFORCED,
            enforced_by=("check_entity_instance_state_in_lifecycle",),
        ),
        Requirement(
            id="R-entity-instance-required-fields",
            claim=(
                "Every EntityInstance shall provide values for all EntityFields with required=True."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of §Entity aspect (required-fields concern). A missing required field violates "
                "the declared EntityType schema and makes downstream traversal unreliable."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement=ENFORCED,
            enforced_by=("check_entity_instance_required_fields",),
        ),
        Requirement(
            id="R-entity-instance-id-prefix",
            claim=(
                "Every EntityInstance.id shall start with 'ENT-<entity_type>-' (typed-anchor discipline)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of §Entity aspect (typed-anchor concern). Encodes both type and entity kind in "
                "the id, enabling unambiguous cross-reference (R-anchor-everything)."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement=ENFORCED,
            enforced_by=("check_entity_instance_id_prefix", "check_typed_anchors_entity"),
        ),
        Requirement(
            id="R-entity-instance-refs-resolve",
            claim=(
                "Every EntityInstance reference field shall resolve in the graph according to its ref_target."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of §Entity aspect (referential integrity concern). A dangling reference field "
                "is the entity-level equivalent of a dangling Conflict member."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement=ENFORCED,
            enforced_by=("check_entity_instance_refs_resolve",),
        ),
        Requirement(
            id="R-entity-field-kind-known",
            claim=(
                "Every EntityField.kind shall be in ENTITY_FIELD_KINDS "
                "(string | number | enum | reference | state)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of §Entity aspect (field-kind concern). An unknown kind breaks the discriminant "
                "for kind-specific invariants and future machine-checkable field validation."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement=ENFORCED,
            enforced_by=("check_entity_field_kind_known",),
        ),
        Requirement(
            id="R-entity-typed-anchors",
            claim=(
                "check_typed_anchors shall validate the ENT- prefix for EntityInstance nodes."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of §Entity aspect (typed-anchors concern). Without prefix validation, "
                "EntityInstance nodes escape the anchoring discipline (R-anchor-everything)."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement=ENFORCED,
            enforced_by=("check_typed_anchors_entity",),
        ),
        Requirement(
            id="R-process-drives-existing-entities",
            claim=(
                "Every entity slug referenced in a Process.drives_entities shall resolve to a "
                "declared EntityType slug in g.entity_types."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of §Process/§Entity coupling (referential integrity concern). A Process "
                "that references undeclared entity types is structurally broken — the coupling "
                "between the behavioral aspect and the entity aspect must be referentially sound."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement=ENFORCED,
            enforced_by=("check_process_drives_existing_entities",),
        ),
        Requirement(
            id="R-step-invokes-known-transition",
            claim=(
                "Every Step.transition (when non-empty) shall name a transition event declared "
                "in the driven EntityType.lifecycle."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of §Process/§Entity coupling (step-transition concern). A Step that "
                "invokes an undeclared transition is a structural dead-end — the lifecycle "
                "machine cannot process it."
            ),
            assumptions=("A-prose-suffices", "A-bootstrap-self-applies"),
            enforcement=ENFORCED,
            enforced_by=("check_step_invokes_known_transition",),
        ),
        Requirement(
            id="R-dependency-tracked",
            claim=(
                "The system shall track the dependency network between requirements via Requirement.relations (depends_on, supports, refines)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-dependency-graph-parallelism (tracking concern). Relations are the data that makes dependency-driven delegation possible."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("check_no_dangling_requirement_relations",),
        ),
        Requirement(
            id="R-dependency-drives-parallel",
            claim=(
                "Independent sub-graphs in the dependency network may be delegated to parallel sub-operators."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-dependency-graph-parallelism (parallel concern). Independent components can run concurrently without coordination overhead."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="STRUCTURAL",
        ),
        Requirement(
            id="R-dependency-drives-sequential",
            claim=("Dependency chains in the network shall be processed sequentially."),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-dependency-graph-parallelism (sequential concern). Coupled requirements need ordering to avoid stale inputs."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="STRUCTURAL",
        ),
        Requirement(
            id="R-crystal-is-claude-md",
            claim=(
                "Each operator's crystallized substrate shall be its own CLAUDE.md file."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Atom of R-operator-crystal-is-claude-md (identity concern). CLAUDE.md is auto-loaded by the harness, making it the natural crystal location."
            ),
            assumptions=("A-compaction-loses-working", "A-bootstrap-self-applies"),
            enforcement="STRUCTURAL",
        ),
        Requirement(
            id="R-crystal-reload-by-reference",
            claim=(
                "An operator shall reload its crystal (CLAUDE.md) by reference rather than re-carrying it in working context."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Atom of R-operator-crystal-is-claude-md (reload-by-reference concern). Re-carrying wastes context budget; reloading by reference is the offload instrument."
            ),
            assumptions=("A-compaction-loses-working", "A-bootstrap-self-applies"),
            enforcement="STRUCTURAL",
        ),
        Requirement(
            id="R-crystal-tree-hierarchy",
            claim=(
                "The delegation hierarchy shall be a tree of CLAUDE.md crystals, one per operator, each bounded by its context budget."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Atom of R-operator-crystal-is-claude-md (tree-hierarchy concern). The tree structure mirrors the delegation hierarchy and is natively supported by Claude Code nested CLAUDE.md loading."
            ),
            assumptions=("A-compaction-loses-working", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("test_constitution_gen.py",),
        ),
        Requirement(
            id="R-enforcement-levels-declared",
            claim=(
                "A requirement shall carry an enforcement level from the set PROSE, STRUCTURAL, ENFORCED."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-enforcement-gradient (levels-declared concern). The three levels make 'how deeply crystallized' measurable and auditable."
            ),
            assumptions=("A-most-knowledge-crystallizable",),
            enforcement="ENFORCED",
            enforced_by=("check_enforced_names_invariant",),
        ),
        Requirement(
            id="R-enforced-names-enforcer",
            claim=(
                "An ENFORCED requirement shall name its enforcing invariant or test in enforced_by."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-enforcement-gradient (names-enforcer concern). Naming the enforcer is what makes the guarantee auditable rather than merely claimed."
            ),
            assumptions=("A-most-knowledge-crystallizable",),
            enforcement="ENFORCED",
            enforced_by=(
                "check_enforced_names_invariant",
                "test_docs_gen.py::test_unenforced_md_up_to_date",
            ),
        ),
        Requirement(
            id="R-critical-core-methodology",
            claim=(
                "The methodology's own critical core shall be the six invariants in CRITICAL_CORE_INVARIANTS, property-tested by test_conscience.py."
            ),
            owner="domain-user",
            status="SETTLED",
            why=(
                "Atom of R-critical-core-scope (methodology scope). M7 resolved: these six guard every path by which a contradiction could be introduced without being seen."
            ),
            assumptions=("A-prose-suffices",),
            enforcement="ENFORCED",
            enforced_by=(
                "test_conscience.py",
                "check_no_dangling_assumption_owner",
                "check_no_dangling_requirement_owner",
                "check_no_dangling_requirement_assumptions",
                "check_no_dangling_requirement_relations",
                "check_no_dangling_conflict_refs",
            ),
        ),
        Requirement(
            id="R-critical-core-per-domain",
            claim=(
                "Business-domain critical core (money, access, SLA) shall be a separate per-domain calibration, not framework-imposed."
            ),
            owner="domain-user",
            status="SETTLED",
            why=(
                "Atom of R-critical-core-scope (per-domain scope). Each domain has its own cost-of-unnoticed-conflict profile; the framework must not impose the methodology's own critical core onto business domains."
            ),
            assumptions=("A-prose-suffices",),
            enforcement="PROSE",
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-bijection-r-to-enforcer",
            claim=(
                "Every SETTLED/ENFORCED requirement shall name an existing "
                "check_* in hotam_spec.invariants.ALL_INVARIANTS or a real test_* "
                "in spec/tests/."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Substrate-generates-operator (R-operator-prompt-from-substrate) "
                "requires that each atomic claim point to its actual machine "
                "verifier. Unresolvable enforcer names break the bijection "
                "between claim and check, hiding compoundness. WAVE 3 outcome."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("check_bijection_r_to_enforcer",),
        ),
        Requirement(
            id="R-tool-is-its-own-requirement",
            claim=(
                "Every tool in spec/tools/ whose module docstring opens with 'Canon: §<topic> — <claim>' shall be projected into a SETTLED requirement R-tool-<basename> with that claim text, enforced by spec/tests/test_tool_<basename>.py when it exists."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The sensor-substrate inversion taken one step deeper: not only does code generate the operator's prompt, the tool IS its own requirement. The docstring is the claim, the body is the check, the test is the enforcer. Removing the tool removes the R; lying in the docstring is caught by the test failing. This eliminates the prose gap between 'R written in graph' and 'tool implementing R'."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_tool_derived_requirements.py",),
        ),
        Requirement(
            id="R-agent-scoped-constitution",
            claim=(
                "For each spec/agents/<name>/ directory, gen_spec.py shall regenerate that agent's CLAUDE.md CONSTITUTION block filtered by the agent's SCOPE tuple of R-id prefixes."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Each sub-operator needs an operator-prompt scoped to its domain — the framework-agent sees R-check-* and R-bijection-*, the finance-agent sees R-finance-*, etc. A single global CLAUDE.md would overload sub-agents with irrelevant requirements and dilute their focus. Per-agent generation enforces the bounded-context discipline (R-context-bounded-delegation) structurally."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_agent_scoped_constitution",),
        ),
        Requirement(
            id="R-repo-map-generated",
            claim=(
                "CLAUDE.md shall contain a REPO-MAP block listing every spec/src/hotam_spec/*.py, spec/tools/*.py, docs/gen/*.md, and spec/content/*.py with a one-line role from its module docstring or front matter."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The operator needs a map of the substrate to navigate without grep. Hand-written file lists in CLAUDE.md drift from reality (the 'Files (layers)' section already has). Generating the map from the filesystem + module docstrings makes drift structurally impossible (new file without map entry = red test) and removes a hand-written section from CLAUDE.md. Realizes R-repo-map-generated as the next layer of substrate-generates-operator."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_repo_map_complete",),
        ),
        Requirement(
            id="R-agent-declares-purpose",
            claim=(
                "Every spec/agents/<name>/scope.py shall define a non-empty module-level constant PURPOSE describing what the agent stewards in one line."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "An agent without a declared purpose is invisible to the operator-prompt — AGENT-MAP can't render its responsibility. PURPOSE in scope.py is machine-readable (vs README which is prose); placing it next to SCOPE keeps the agent's contract in one file. Enforced structurally so the absence of PURPOSE = missing operator visibility = red test, not silent gap."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_every_agent_declares_purpose",),
        ),
        Requirement(
            id="R-agent-map-generated",
            claim=(
                "CLAUDE.md shall contain an AGENT-MAP block listing every spec/agents/<name>/ with its PURPOSE, SCOPE prefixes, count of SETTLED atoms in scope, count of private and shared tools, and crystal path."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The operator needs an automatic map of delegated authority — who stewards what. Hand-maintained agent registries drift. PURPOSE (machine-readable in scope.py per R-agent-declares-purpose) + SCOPE (the filter) + atoms-count (the load) + tool counts (the capability) together give the director a one-glance view of the delegation graph without grep."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_agent_map_complete",),
        ),
        Requirement(
            id="R-domain-is-a-directory",
            claim=(
                "A business domain is represented as a directory at `domains/<name>/`."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Separates framework code (spec/, shared, immutable from any domain's view) from business content (domains/<name>/, per-business). Enables tools/create_domain to scaffold new businesses without touching the framework body. Builds on the agent-as-directory pattern: just as agents are directories with their own CLAUDE.md+tools+agents, domains are directories with their own graph+tools+agents+docs+CLAUDE.md."
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("check_domain_manifest_exists_and_importable", "test_tool_create_domain.py::test_creates_required_files"),
        ),
        Requirement(
            id="R-domain-has-manifest",
            claim=(
                "Every `domains/<name>/` directory contains `manifest.py` defining top-level constants ID, DESCRIPTION, GOALS, DIRECTOR."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The manifest is the machine-readable identity of the domain: it lets create_domain, gen_spec, and the root DOMAIN-MAP block discover a domain's metadata without loading its full graph. GOALS and DIRECTOR are required so any operator can reconstitute context from the manifest alone (R-agent-never-lost). ENFORCED via check_domain_manifest_valid (task #64)."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=(
                "check_domain_manifest_exists_and_importable",
                "check_domain_manifest_id_matches_dirname",
                "check_domain_manifest_description_nonempty",
                "check_domain_manifest_goals_nonempty",
                "check_domain_manifest_director_nonempty",
            ),
        ),
        Requirement(
            id="R-domain-declares-director",
            claim=(
                "Every domain's `manifest.py` DIRECTOR constant must resolve to a real agent directory at `domains/<name>/agents/<DIRECTOR>/`."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "A domain without a reachable director is an orphaned subtree: no operator can be loaded (R-agent-never-lost violated). The structural check prevents dangling DIRECTOR strings the same way check_dangling_refs prevents dangling assumption ids. ENFORCED via check_domain_director_exists (task #64)."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("check_domain_director_exists",),
        ),
        Requirement(
            id="R-domain-owns-graph-py",
            claim=(
                "Each `domains/<name>/` owns its `graph.py` defining `build_graph() -> TensionGraph`."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Isolates each domain's requirement graph from all others. The single-file convention (`build_graph()`) is inherited from spec/content/graph.py and load_content_graph(), so domain tooling reuses the same loader with a path argument. Cross-domain references are explicitly forbidden: a requirement is local or it is a shared framework node."
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("check_domain_manifest_exists_and_importable", "test_tool_create_domain.py::test_creates_required_files"),
        ),
        Requirement(
            id="R-domain-owns-docs-gen",
            claim=(
                "Each `domains/<name>/docs/gen/` shall hold only the markdown generated from that domain's graph, with no cross-domain doc files."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Generated docs are the human-readable shadow of the graph (R-deterministic-generation). Keeping them inside the domain directory ensures each operator's boot sequence reads only its own REQUIREMENTS.md/TENSIONS.md/OPEN.md, not a mixed-domain dump. The anti-drift meta-test must be domain-scoped accordingly."
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_tool_create_domain.py::test_creates_required_files",),
        ),
        Requirement(
            id="R-domain-owns-tools-and-agents",
            claim=(
                "Each `domains/<name>/` shall contain both a `tools/` directory (private tools) and an `agents/` directory (sub-operators), even if empty."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "tools/ holds domain-specific scripts (e.g. create_domain, gen_spec variants) that must not pollute the shared spec/tools/. agents/ is the recursive sub-operator tree (R-agent-is-recursive-director). Requiring both to exist even when empty makes scaffolding deterministic and avoids FileNotFoundError in tooling that expects the layout."
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_tool_create_domain.py::test_creates_required_files",),
        ),
        Requirement(
            id="R-domain-owns-claude-md",
            claim=(
                "Each `domains/<name>/CLAUDE.md` is the domain-scoped operator-prompt, generated by `gen_spec.py`."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The CLAUDE.md is the director-operator's crystallized substrate (R-operator-crystal-is-claude-md). For a domain director this means the three-cipher pulse, top action, and debt figures must be domain-local, not framework-global. Generation by gen_spec.py prevents hand-written drift (R-drift-structurally-impossible). ENFORCED in P17 by test_domain_claude_md_has_all_5_blocks."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_domain_claude_md_has_all_5_blocks",),
        ),
        Requirement(
            id="R-framework-claude-md-is-domain-free",
            claim=(
                "The root `CLAUDE.md` shall contain only framework-level content, referencing domains exclusively through the DOMAIN-MAP block."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Mixing domain atoms into the framework CLAUDE.md recreates the single-file coupling that domain isolation is designed to break. The DOMAIN-MAP block is a generated index (R-domain-map-generated), not inline content; every domain-specific atom stays inside domains/<name>/. ENFORCED in P17 by test_framework_claude_md_no_domain_atoms."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_framework_claude_md_no_domain_atoms",),
        ),
        Requirement(
            id="R-domain-map-generated",
            claim=(
                "The root `CLAUDE.md` shall contain a DOMAIN-MAP block listing every `domains/<name>/` with id, description, goals, director, path, atoms-count."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The framework operator must be able to discover all domains without reading each domain's graph. The DOMAIN-MAP block is generated by gen_spec.py from manifest.py files, so it cannot drift from the actual directory layout (R-deterministic-generation). atoms-count gives the operator a quick load-budget estimate (R-context-budget-rule). ENFORCED in P17 by test_framework_claude_md_has_domain_map."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_framework_claude_md_has_domain_map",),
        ),
        Requirement(
            id="R-director-agent-required-per-domain",
            claim=(
                "Every domain must contain a `director` agent at `domains/<name>/agents/director/` as the entry point operator."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The director is the domain's R-operator-acting-facet: it holds the crystal (CLAUDE.md), runs the boot sequence, and is the single entry point for any orchestrator. Without a director agent the domain has no defined operator and violates R-agent-never-lost. The name `director` is conventional, not arbitrary — it mirrors the OP-director role at the framework level."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=(
                "check_domain_director_exists",
                "test_tool_create_domain.py::test_director_agent_created",
            ),
        ),
        Requirement(
            id="R-agent-is-recursive-director",
            claim=(
                "Every agent at `spec/agents/<a>/` or `domains/*/agents/<a>/` shall be a director of its SCOPE containing its own `agents/` subdirectory, with the recursion terminating at an empty leaf `agents/` folder."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Recursive directory structure encodes the delegation hierarchy (R-delegation-conclusions-only, R-dependency-graph-parallelism): each agent can spawn sub-agents in its own agents/ without touching sibling or parent directories. The empty-leaf convention makes the recursion's base case structurally explicit — a leaf agent is an agent that has no sub-agents, represented as an empty directory rather than a missing one. ENFORCED via check_agent_has_agents_subdir (task #64)."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("check_agent_has_agents_subdir",),
        ),
        Requirement(
            id="R-framework-shared-docs-generated",
            claim=(
                "The framework shall generate spec/docs/thinking/*.md and spec/docs/tools/*.md deterministically from framework module docstrings and tool docstrings plus argparse --help output."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Shared docs are the single authoritative reference for framework thinking and tool usage across all agents and domains. Generating them deterministically from docstrings and --help ensures they cannot drift from the code (R-drift-structurally-impossible). The STRUCTURAL enforcement label means enforcement checks arrive in task #64 once the generator is built."
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_domain_isolation_p17.py::test_shared_thinking_docs_generated", "test_domain_isolation_p17.py::test_shared_tool_docs_generated"),
        ),
        Requirement(
            id="R-shared-tool-doc-from-docstring-and-help",
            claim=(
                "Each spec/docs/tools/<basename>.md shall be built from the tool module docstring and its argparse --help output — no hand-written content between sentinels."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "A tool doc that diverges from its --help output is worse than no doc — it misleads operators. Deriving both sections from the single source (module docstring + argparse) eliminates the divergence class entirely. The sentinel pattern mirrors how CONSTITUTION blocks are generated: machine-written between markers, never hand-edited."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_domain_isolation_p17.py::test_shared_tool_docs_content",),
        ),
        Requirement(
            id="R-shared-thinking-doc-from-canon-sections",
            claim=(
                "Each spec/docs/thinking/<topic-slug>.md shall be generated as the union of all framework docstrings carrying Canon: §<Topic> markers, never hand-written."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The Canon: §Topic markers in framework docstrings are the authoritative source for each thinking topic; aggregating them into a single file makes cross-module rationale visible without repeating it. Hand-writing the thinking docs would immediately drift from the annotated code, violating R-drift-structurally-impossible. The generator collects all §Topic-marked docstrings in one pass, identical to how gen_spec collects CONSTITUTION sections."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_domain_isolation_p17.py::test_shared_thinking_docs_content",),
        ),
        Requirement(
            id="R-agent-references-shared-docs",
            claim=(
                "Each agent CLAUDE.md shall contain a SHARED-DOCS block listing relative paths to spec/docs/thinking/*.md (all) and spec/docs/tools/*.md (filtered by SCOPE), without duplicating their content."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Duplicating shared framework content into each agent crystal guarantees drift — the copies diverge the moment any framework docstring changes. A SHARED-DOCS reference block keeps each agent crystal thin while granting operators access to the full framework reasoning on demand. The SCOPE filter means agents only reference tool docs for tools they actually use, keeping the block proportionate to the agent's responsibility."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_domain_isolation_p17.py::test_agent_shared_docs_block_present",),
        ),
        Requirement(
            id="R-agent-has-docs-dir",
            claim=(
                "Every agent at spec/agents/<a>/ or domains/*/agents/<a>/ (including recursively-nested sub-agents) shall contain a docs/ subdirectory for the agent private notes, separate from any generated content."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Agents accumulate private reasoning — checkpoints, open questions, steward notes — that must not mix with generated content or the parent operator crystal. A dedicated docs/ directory provides a stable, predictable location that survives crystal regeneration. The scaffold creates docs/.gitkeep so the directory is tracked even when empty, matching the same pattern used for tools/ and agents/ subdirs. ENFORCED via check_agent_has_docs_subdir (task #64)."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("check_agent_has_docs_subdir",),
        ),
        Requirement(
            id="R-domain-has-docs-dir",
            claim=(
                "Every domains/<name>/ shall contain a docs/ directory which wraps the generated docs/gen/ plus any hand-written domain material."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The docs/ wrapper cleanly separates machine-generated output (docs/gen/) from hand-written domain material without requiring two separate top-level directories. Domain operators need a place to put domain-level notes, ADRs, and glossaries that are domain-specific and not governed by the framework generator. Keeping everything under docs/ mirrors the spec/docs/ structure established for the framework level."
            ),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_tool_create_domain.py::test_creates_docs_dir",),
        ),
        Requirement(
            id="R-no-hand-edit-graph",
            claim=("Changes to domains/*/graph.py shall be made only through tools/apply_proposal.py, with direct hand-edits prohibited outside of bootstrap events."),
            owner="framework-author",
            status="SETTLED",
            why=("The closed loop's ACT half goes through apply_proposal (R-active-loop-apply-tool). A deterministic PreToolUse command-hook (not LLM-judgment) blocks direct Edit/Write on domains/*/graph.py, denying with a clear redirect to apply_proposal.py. Hardened from prompt-type judgment to command-type determinism to close an adversarial-bypass gap."),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_pretooluse_graph_guard_denies_graph_py",),
        ),
        Requirement(
            id="R-method-matches-docstring",
            claim=("Each check_* function in hotam_spec.invariants.ALL_INVARIANTS shall have a docstring whose RULE line shares non-trivial lexical overlap with its body's Violation messages."),
            owner="framework-author",
            status="SETTLED",
            why=("The audit principle 'docstring is the description, body is the verification' is verifiable structurally: if the RULE says one thing and the violation messages say another, drift is happening. The Jaccard threshold is heuristic but catches gross mismatch; refinement comes from running it."),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("check_method_matches_docstring",),
        ),
        Requirement(
            id="R-root-claude-md-is-sentinel-only",
            claim=("The root CLAUDE.md shall contain only a minimal framework-identity header plus sentinel-bounded generated blocks, with no hand-written prose between sentinels."),
            owner="framework-author",
            status="SETTLED",
            why=("Substrate-generates-operator at root level. Hand-written prose drifts; the framework's mind is assembled by gen_spec from spec/docs/thinking/* (shared DRY source) and the domain CLAUDE.md (per-domain knowledge). The root is a thin shell pointing into both. RESOLVED — REPLACES the old hand-written prose in root CLAUDE.md (P19a)."),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_root_claude_md_is_sentinel_only",),
        ),
        Requirement(
            id="R-entities-md-generated",
            claim=("domains/<name>/docs/gen/ENTITIES.md shall be generated from the active domain's graph by gen_spec.py, listing every EntityType with its lifecycle Mermaid diagram, fields, covering check_entity_* invariants, and instances."),
            owner="framework-author",
            status="SETTLED",
            why=("Substrate-generates-operator extended to the entity layer. ENTITIES.md is the per-domain entity registry; emitter writes it deterministically (LF, utf-8) from build_graph(). check_entities_md_lists_all_types enforces no EntityType is silently omitted. Anti-drift via existing test_docs_gen byte-identical regen."),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("check_entities_md_lists_all_types",),
        ),
        Requirement(
            id="R-entity-derived-requirement",
            claim=("Each EntityType in the active domain's graph shall be projected as R-entity-<slug> in the domain's CLAUDE.md CONSTITUTION block, with enforced_by listing the check_entity_* family covering it."),
            owner="framework-author",
            status="SETTLED",
            why=("Mirrors R-tool-is-its-own-requirement at the entity layer: an EntityType IS its own requirement; deleting it removes the R; changing its description changes the claim. Eliminates the prose gap between 'R about entity behavior' and 'EntityType implementing it'."),
            assumptions=("A-python-stack",),
            enforcement=STRUCTURAL,
        ),
        Requirement(
            id="R-entity-is-declarative",
            claim=("The framework shall supply no built-in EntityType values — all entity types are declared by domains in build_graph()."),
            owner="framework-author",
            status="SETTLED",
            why=("The content-free contract (R-content-free-no-business-data) extends to the entity layer: entity types are domain knowledge, not framework knowledge. Supplying built-in types would violate the blank-kit invariant and couple the framework to a particular domain model."),
            assumptions=("A-python-stack",),
            enforcement=STRUCTURAL,
        ),
        Requirement(
            id="R-entity-reuses-lifecycle",
            claim=("Each EntityType.lifecycle shall be a Lifecycle value (the §Lifecycle keystone) with no parallel state machinery introduced."),
            owner="framework-author",
            status="SETTLED",
            why=("The §Lifecycle keystone was introduced precisely so that every stateful aspect (Process, Entity, Goal, Operator) reuses one mechanism. Parallel state machinery would fracture the invariant surface and double the maintenance burden for each new aspect."),
            assumptions=("A-python-stack",),
            enforcement=STRUCTURAL,
            enforced_by=("check_entity_type_lifecycle_wellformed",),
        ),
        Requirement(
            id="R-entity-checks-by-iteration",
            claim=("The check_entity_* invariant family shall cover every declared EntityType by iterating g.entity_types, requiring no new check_* code per additional type."),
            owner="framework-author",
            status="SETTLED",
            why=("Parametric iteration is what makes the entity aspect scale: one invariant covers all types today and tomorrow. Per-type invariants would grow the check surface linearly and force framework edits on every domain addition — the inverse of the content-free design."),
            assumptions=("A-python-stack",),
            enforcement=STRUCTURAL,
        ),
        Requirement(
            id="R-entity-state-conflict-surfaced",
            claim=("Two processes driving one EntityType to disjoint terminal or quiescent states shall surface as a P5 LATENT_CONNECTOR action via entity_state_conflict_suspects()."),
            owner="framework-author",
            status="SETTLED",
            why=("Process-induced state conflicts on a shared entity are a category of hidden dependency (the third invisibility the methodology surfaces). Routing them to P5 keeps them visible without premature closure — the detector returns suspects, never decisions."),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_demo_fixture.py::test_demo_fixture_what_now_emits_p5_entity_state_conflict",),
        ),
        Requirement(
            id="R-root-claude-md-contains-domain-crystal",
            claim=("Root CLAUDE.md shall embed the full content of the active domain's CLAUDE.md inside a DOMAIN-CRYSTAL sentinel block generated by gen_spec.py."),
            owner="framework-author",
            status="SETTLED",
            why=("Closes the sensor-substrate gap: Claude Code auto-loads root CLAUDE.md on session start; embedding the domain's CLAUDE.md (the canonical entry point of the domain, and the base from which all sub-agents derive scoped versions) means the operator boots from substrate (R-operator-prompt-from-substrate) rather than from raw weights + session memory. The substrate writes the operator's prompt physically, not aspirationally."),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_domain_crystal_contains_domains_claude_md_content",),
        ),
        Requirement(
            id="R-recently-rejected-surfaced",
            claim=("Root CLAUDE.md shall contain a RECENTLY-REJECTED sentinel block listing every REJECTED requirement whose why contains 'REJECTED — REPLACES' to surface anti-relitigation evidence before re-derivation."),
            owner="framework-author",
            status="SETTLED",
            why=("Anti-relitigation discipline made structural: when the operator considers proposing an architectural change, the substrate puts previously-rejected proposals in front of it. Eliminates re-deriving rejected claims without citation of the replacement."),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_recently_rejected_lists_known_rejections",),
        ),
        Requirement(
            id="R-operator-prompt-loaded-at-session-start",
            claim=("A SessionStart hook in .claude/settings.local.json shall run gen_spec.py before the operator's first turn of any session, ensuring root CLAUDE.md is current substrate-derived state."),
            owner="framework-author",
            status="SETTLED",
            why=("Closes the boot edge of the sensor-substrate inversion. Without SessionStart regen, Claude Code may auto-load a stale CLAUDE.md whose DOMAIN-CRYSTAL block reflects an older graph state. The hook ensures the substrate writes the operator's prompt at every session boot — physically, not aspirationally (R-operator-prompt-from-substrate)."),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_session_start_hook_runs_gen_spec",),
        ),
        Requirement(
            id="R-three-cipher-pulse-structurally-injected",
            claim=("A UserPromptSubmit hook in .claude/settings.local.json shall extract the three-cipher pulse from the LIVE-STATE block and inject it as additionalContext into every user turn."),
            owner="framework-author",
            status="SETTLED",
            why=("Closes the per-turn edge of the inversion. The three-cipher pulse (top action / debt / context) is required by the boot ritual to be cited in the first sentence of every substantive reply (R-boot-cite-in-first-sentence); the hook enforces structural presence so the operator cannot forget. emit_cipher.py reads the LIVE-STATE sentinels of the just-regenerated root CLAUDE.md and emits a JSON hookSpecificOutput consumed by Claude Code."),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_user_prompt_submit_hook_emits_cipher",),
        ),
        Requirement(
            id="R-post-compact-regen-from-substrate",
            claim=("A PostCompact hook in .claude/settings.local.json shall run gen_spec.py after every auto-compact so the post-compact prompt reload reads fresh substrate-derived CLAUDE.md, not the stale pre-compact version."),
            owner="framework-author",
            status="SETTLED",
            why=("Auto-compact rewrites session context but does not re-read CLAUDE.md unless triggered. Without this hook, the operator post-compact runs on summary + stale CLAUDE.md. PostCompact regen ensures the substrate-derived prompt is the authoritative reload point after any compaction event."),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_post_compact_hook_runs_gen_spec",),
        ),
        Requirement(
            id="R-project-name-hotam-spec",
            claim=("The project's name shall be Hotam-Spec (display), hotam_spec (Python package), hotam-spec (kebab-case for filesystem and PyPI), closing M1."),
            owner="framework-author",
            status="SETTLED",
            why=("M1 (package name) was OPEN since the framework's first incarnation as 'tensio'. The rename to Hotam-Spec aligns the project with its repository name and the user's chosen identity. Convention: 'Hotam-Spec' in prose (capitalized hyphen), 'hotam_spec' snake_case in Python source (imports/identifiers), 'hotam-spec' kebab-case for filesystem and PyPI. Renames completed in three sequential passes (#89 package, #90 domain, #91 prose)."),
            assumptions=("A-python-stack",),
            enforcement=STRUCTURAL,
        ),
        Requirement(
            id="R-enforceability-kind-declared",
            claim=("A requirement shall carry an enforceability kind from the set ENFORCEABLE or INHERENTLY_PROSE, distinguishing real closeable debt from permanent discipline."),
            owner="framework-author",
            status="SETTLED",
            why=("The enforcement-gradient P0 REFLECTION conflated two categories: requirements that could have a check_* but don't yet (real debt) and requirements that are fundamentally judgment calls no check_* could ever verify (permanent discipline). Without this distinction the debt metric never converges -- inherent-prose rules were counted as debt forever. Splitting the two makes the P0 REFLECTION an honest, closeable signal."),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("check_enforceability_kind_known",),
        ),
        Requirement(
            id="R-parallel-mutating-agents-use-worktree",
            claim=("Parallel Agent-tool invocations that mutate tracked repository files shall use isolation:'worktree' unless their target files are provably disjoint and no history-rewriting git operation is planned during their execution window."),
            owner="framework-author",
            status="SETTLED",
            why=("Incident 2026-06-30/07-01: two parallel sm-agents (P5-noise-fix and enforceability-flag tasks) both touched overlapping domains/hotam-spec-self/graph.py and spec/src/hotam_spec/invariants.py territory in one shared working tree. A subsequent git filter-repo hard-reset (run by the director to purge .idea/__pycache__ from committed history) wiped the P5-noise-fix agent's uncommitted work because it had not yet been committed and was not isolated in a worktree. The fix had to be redone from scratch. This requirement crystallizes the lesson: parallel mutating agents belong in isolated worktrees, or the director must guarantee no history-rewrite runs while their work is uncommitted."),
            assumptions=("A-python-stack",),
            enforcement=STRUCTURAL,
        ),
        Requirement(
            id="R-claude-md-template-driven",
            claim=("CLAUDE.md shall be generated by substituting <!-- mind --> and <!-- business --> placeholders in CLAUDE.md.template.txt with rendered content, preserving all other template text verbatim."),
            owner="framework-author",
            status="SETTLED",
            why=("The prior sentinel-surgery approach (editing ~10 separate BEGIN/END marker pairs scattered through an existing file) made it impossible for a human to add durable plain-text notes anywhere in CLAUDE.md without risking accidental clobbering by a future block-insertion bug. The template model inverts this: exactly two named placeholders are substituted; everything else in the template -- including any hand-written notes a human adds -- survives every regeneration verbatim. Simpler mental model, same anti-drift guarantee for the two generated zones."),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_hand_written_note_in_template_survives_regen", "test_regen_byte_identical",),
        ),
    )

    # --- Live conflict NODES ----------------------------------------------
    # C1: the autonomy-vs-boundary tension inside the AI agent — the AI both
    # promises 'agent never lost' (must act effectively) and 'never closes
    # silently' (must not act on conflict resolution). DECIDED with a derived
    # requirement (R-active-loop-playbooks) that has not landed yet.
    c1_axis = "agent-autonomy-vs-human-control"
    c1_ctx = "the agent develops requirements, integrates new ones, finds contradictions, proposes resolutions, formalizes back into code, runs tests"

    # C2: the bootstrap-paradox — Hotam-Spec modeling its OWN design in
    # spec/content/. Resolved by treating the meta-domain as content like any
    # other; the framework code stays empty of business data.
    c2_axis = "framework-purity-vs-helpfulness"
    c2_ctx = "the methodology's own design needs to be modeled to demonstrate the framework end-to-end"

    # C3: aspect-weight — adding behavioral aspects (Lifecycle / Process /
    # Entity / Task) extends what the agent can see, at the cost of more
    # framework surface. DETECTED — no steward decision yet.
    c3_axis = "core-vs-aspect"
    c3_ctx = "extending the framework to surface behavioral contradictions (dead states, two processes one entity)"

    # C4: apparatus-weight — crystallizing the full accumulated design vs keeping
    # the framework minimal. The very tension that JUSTIFIES this crystallization.
    # DECIDED: record the design as DRAFT/OPEN requirements (coverage without
    # adding apparatus weight to src/hotam_spec).
    c4_axis = "apparatus-weight-vs-coverage"
    c4_ctx = "crystallizing the full accumulated design into the methodology vs keeping the framework minimal"

    # C5: horizontal-vs-vertical relief — an operator near its context budget must
    # choose how to relieve pressure. DECIDED: crystallize-before-split; delegation
    # is the lever of last resort.
    c5_axis = "horizontal-vs-vertical-relief"
    c5_ctx = (
        "an operator approaching its context budget must choose how to relieve pressure"
    )

    # C6: sequential-vs-parallel — when an over-budget operator domain is split
    # for parallel sub-operators, some sub-parts are coupled by dependencies and
    # cannot run in parallel. DECIDED: the dependency graph decides — parallelize
    # independent components, sequence coupled chains; cut along lines of
    # independence, never arbitrarily.
    c6_axis = "sequential-vs-parallel"
    c6_ctx = (
        "splitting an over-budget operator domain for parallel sub-operators "
        "when some sub-parts are coupled by dependencies"
    )

    conflicts = (
        Conflict(
            id=conflict_identity(c1_axis, c1_ctx),
            axis=c1_axis,
            context=c1_ctx,
            members=("R-agent-never-lost", "R-ai-presents-not-decides"),
            steward="framework-author",
            lifecycle=(
                "DECIDED(structured proposal protocol — the AI emits "
                "ProposedRequirement / ProposedConflict / ProposedResolution "
                "as JSON; the human steward approves; tools/apply_proposal.py "
                "mechanically writes the change into spec/content/; see "
                "derived R-active-loop-playbooks)"
            ),
            decided_by="framework-author",
            shared_assumption="A-stakeholders-care",
            derived=("R-active-loop-playbooks",),
            revisit_marker=(
                "REVISIT if domain-users report the playbook overhead negates "
                "the harness's directness (the loop becomes slower than free "
                "manual editing) — then re-calibrate band-by-band."
            ),
        ),
        Conflict(
            id=conflict_identity(c2_axis, c2_ctx),
            axis=c2_axis,
            context=c2_ctx,
            members=("R-content-free-framework", "R-empty-content-is-legitimate"),
            steward="framework-reviewer",
            lifecycle=(
                "DECIDED(the meta-domain lives in spec/content/graph.py exactly "
                "as any user's domain would; the framework code stays empty of "
                "business data; the worked-example fixture stays under "
                "spec/tests/fixtures/. The framework's own design is content "
                "for the methodology's reference domain.)"
            ),
            decided_by="framework-reviewer",
            shared_assumption="A-content-free-honest",
            revisit_marker=(
                "REVISIT if a fresh framework clone needs the meta-domain to "
                "self-bootstrap (cf. M8 content-layout evolution)."
            ),
        ),
        Conflict(
            id=conflict_identity(c3_axis, c3_ctx),
            axis=c3_axis,
            context=c3_ctx,
            members=("R-content-free-framework", "R-agent-never-lost"),
            steward="domain-user",
            lifecycle="ACKNOWLEDGED",
            shared_assumption="A-prose-suffices",
            revisit_marker=(
                "REVISIT when a second opt-in behavioral aspect (Entity or Task) "
                "is proposed — at that point the core-vs-aspect boundary must be "
                "formally decided."
            ),
        ),
        Conflict(
            id=conflict_identity(c4_axis, c4_ctx),
            axis=c4_axis,
            context=c4_ctx,
            members=("R-crystallize-knowledge-to-code", "R-content-free-framework"),
            steward="framework-reviewer",
            lifecycle=(
                "DECIDED(crystallize the design as DRAFT/OPEN requirements — "
                "recorded but UNBUILT; the status itself marks them "
                "proposed-not-built, so coverage rises without adding apparatus "
                "weight to src/hotam_spec. The substrate grows; the framework code "
                "stays minimal.)"
            ),
            decided_by="framework-reviewer",
            shared_assumption="A-content-free-honest",
            derived=(),
            revisit_marker=(
                "REVISIT if the DRAFT backlog grows faster than it is built — "
                "then prune or promote."
            ),
        ),
        Conflict(
            id=conflict_identity(c5_axis, c5_ctx),
            axis=c5_axis,
            context=c5_ctx,
            members=(
                "R-context-bounded-delegation",
                "R-crystallize-knowledge-to-code",
            ),
            steward="domain-user",
            lifecycle=(
                "DECIDED(crystallize-before-split — the operator crystallizes "
                "first and re-measures (see R-crystallize-before-split); "
                "delegation/splitting is the vertical lever of last resort, used "
                "only when knowledge is irreducible and the operator is still over "
                "budget.)"
            ),
            decided_by="domain-user",
            shared_assumption="A-finite-context-operators",
            derived=("R-crystallize-before-split",),
            revisit_marker="",
        ),
        Conflict(
            id=conflict_identity(c6_axis, c6_ctx),
            axis=c6_axis,
            context=c6_ctx,
            members=(
                "R-context-bounded-delegation",
                "R-dependency-graph-parallelism",
            ),
            steward="framework-reviewer",
            lifecycle=(
                "DECIDED(the dependency graph decides — parallelize independent "
                "components, sequence coupled chains; cut the domain along lines "
                "of independence, never arbitrarily.)"
            ),
            decided_by="framework-reviewer",
            shared_assumption="A-finite-context-operators",
            derived=(),
            revisit_marker="",
        ),
    )

    operators = (
        Operator(
            id="OP-director",
            stakeholder="framework-author",
            lifecycle="ACTIVE",
            # Generous initial budget — the meta-domain is ~80 nodes; this is a
            # real acting operator (the human director). 200 keeps headroom while
            # the budget invariant lives (check_operator_within_budget). Token-
            # estimate is deferred behind the measure seam (M17).
            context_budget=ContextBudget(limit=220, measure="NODE_COUNT"),
            parent=None,
            why=(
                "The director-operator: the human Framework author acting on the "
                "meta-domain. Reads CLAUDE.md as its crystal (R-operator-crystal-is-"
                "claude-md), runs the closed loop (R-agent-never-lost), proposes "
                "changes (R-ai-presents-not-decides), and ratifies steward "
                "decisions. The first instantiated Operator — the operator exists "
                "AS DATA in the graph it operates."
            ),
        ),
    )

    # --- P9: ONE worked Process + ONE worked Goal on the meta-domain ----------
    # The methodology has its OWN process: the closed loop (State→Diagnosis→
    # Action→Regenerate→State). It also has goals — e.g. "all SETTLED
    # requirements are ENFORCED". This is the meta-domain modeling its
    # behavioral surface, just as it modeled its requirements surface.

    processes = (
        Process(
            id="PR-closed-loop",
            lifecycle=PROCESS_LIFECYCLE,
            steps=(
                Step(
                    name="diagnose",
                    requires_role="operator",
                    invokes="",
                    why="Run what_now.diagnose(g) → typed actions.",
                ),
                Step(
                    name="propose",
                    requires_role="operator",
                    invokes="",
                    why="Emit a Proposal for the top action.",
                ),
                Step(
                    name="approve",
                    requires_role="steward",
                    invokes="",
                    why="Steward review + decided_by assignment.",
                ),
                Step(
                    name="apply",
                    requires_role="operator",
                    invokes="",
                    why="apply_proposal.py mechanically lands the change.",
                ),
                Step(
                    name="regenerate",
                    requires_role="operator",
                    invokes="",
                    why="gen_spec.py refreshes docs/gen/.",
                ),
                Step(
                    name="verify",
                    requires_role="operator",
                    invokes="",
                    why="closure.check_closure asserts advancement.",
                ),
            ),
            roles_required=("operator", "steward"),
            drives_entities=(),  # forward-compat: Entity aspect not yet shipped
            why=(
                "The methodology's own process modeled as a Process node — eating "
                "its own dog food at the behavioral altitude "
                "(R-process-aspect-first)."
            ),
        ),
    )

    goals = (
        Goal(
            id="GOAL-burn-down-zero",
            owner="OP-director",
            target_state=TargetState(
                kind=TARGET_KIND_GRAPH_PROPERTY,
                predicate=(
                    "count(r for r in g.requirements if r.status==SETTLED and "
                    "r.enforcement!=ENFORCED) == 0"
                ),
                target="enforcement-gradient",
            ),
            lifecycle="ACTIVE",
            why=(
                "M35 burn-down meter as an ACTIVE goal owned by the director: "
                "every SETTLED requirement should reach ENFORCED. Today several "
                "are still PROSE/STRUCTURAL — the REFLECTION band fires on this. "
                "Goal stays ACTIVE until the count hits 0 (then MET)."
            ),
        ),
    )

    return TensionGraph(
        axes=axes,
        stakeholders=stakeholders,
        assumptions=assumptions,
        requirements=requirements,
        conflicts=conflicts,
        operators=operators,
        processes=processes,
        goals=goals,
    )
