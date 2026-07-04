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

from hotam_spec.assumption import HOLDS, UNCERTAIN, Assumption, IMPLEMENTS, DEAD
from hotam_spec.axis import Axis
from hotam_spec.conflict import Conflict, conflict_identity, Variant
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
from hotam_spec.requirement import Relation, ENFORCED, PROSE, STRUCTURAL, Requirement
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
            statement="For the bulk of requirements, EARS-style prose claims plus structural invariants suffice; formal predicates are reserved for the critical core. — [DEAD] Steward verdict 2026-07-03 (V2), verbatim: «кажется, теста никогда не достаточно. У всего должны быть модели, модели должны иметь простые методы (которые легко читаются (пару строчек), и текстовое поисание, по которому легко проверить, что имплементация метода соотвествует описанию. И должны быть тесты, которые вызывают эти методы. Еще должны быть стейты ... это мы в самом начале собуждали. Тексты сами по себе - это воздух без зелми». Prose alone never sufficed: a claim is trustworthy only when grounded in a model (short readable methods + description checkable against implementation + tests invoking them + states). REPLACED by the HOLDS assumption A-text-grounded-in-models; all 37 dependent requirements were re-linked onto it before this kill.",
            status=DEAD,
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
            statement="The framework can model its own design coherently — Hotam-Spec's own methodology fits its own ontology with no special-casing. — [IMPLEMENTS] Steward verdict 2026-07-03 (verbatim): 'нужно еще один статус - IMPLEMENTS - значит, что мы пытаемся это сделать, мы стремимся к этому, хотим этого'. A-bootstrap-self-applies is not a fact under question but an aspiration we strive toward — re-typed from UNCERTAIN (58 dependents, top pulse line) to the volitional IMPLEMENTS род rather than re-affirmed HOLDS. Changes род and drops the P4 doubt signal honestly.",
            status=IMPLEMENTS,
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
            statement="Most knowledge can be expressed as a node; where it cannot, that resistance is itself a signal of a missing ontology type. — [IMPLEMENTS] Steward verdict 2026-07-03 (V3), variant B verbatim: «Б - мы к этому стремимся, при нахождении несоответствия пытаемся исправить на месте сами или через агента-руку». This is not a fact-claim about the world but a VOLITIONAL striving (R-assumption-implements-state): most knowledge is not KNOWN to be crystallizable, but the operator STRIVES to crystallize it and, on finding a mismatch, repairs it in place itself or via a hand-agent. Re-typing UNCERTAIN → IMPLEMENTS retires the standing P4 doubt signal and records the aspiration.",
            status=IMPLEMENTS,
            owner="framework-reviewer",
        ),
        Assumption(
            id="A-conflict-is-a-node-not-an-edge",
            statement="A contradiction can only carry its shared knowledge (axis, context, shared root) if it is modeled as a first-class NODE; a `conflicts_with` edge between requirements holds nothing and cannot be clustered or stewarded.",
            status=HOLDS,
            owner="framework-author",
        ),
        Assumption(
            id="A-agent-code-imports-framework-directionally",
            statement="The dependency arrow between agent code and the framework body runs one way only: an agent's code may import hotam_spec.* as shared infrastructure, but hotam_spec.* (and spec/tools/*.py) must never import back from any agent's private tools/ directory.",
            status=HOLDS,
            owner="framework-author",
        ),
        Assumption(
            id="A-framework-shared-infra-no-owner",
            statement="The framework body (hotam_spec.*) is shared infrastructure any agent's code may import; it is owned by no single agent -- ownership is a governance convention, not a per-agent claim.",
            status=HOLDS,
            owner="framework-author",
        ),
        Assumption(
            id="A-text-grounded-in-models",
            statement="Text alone is air without ground: every claim is grounded in a model that has short readable methods, a textual description checkable against the method's implementation, tests that invoke those methods, and states (lifecycles). A requirement resting on this assumption is trustworthy only insofar as such a grounding model exists behind it.",
            status=HOLDS,
            owner="domain-user",
        ),
        Assumption(
            id="A-single-human-wears-all-hats",
            statement="In the current phase all stakeholder roles (domain-user/framework-reviewer/framework-author in self, dev-steward/pipeline-operator in hotam-dev) are worn by one human; role-separation checks guard structure, not real independence",
            status=HOLDS,
            owner="domain-user",
        ),
    )

    requirements = (
        # --- SETTLED — the achieved core -----------------------------------
        Requirement(
            id="R-agent-never-lost",
            claim=(
                "The system shall let an agent dropped into the repo in any state, at any moment, deterministically derive the next correct action via tools/what_now.py."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "The centerpiece. Generalizes dev-coin's 'drift is structurally impossible' to 'being lost is structurally impossible'."
            ),
            assumptions=("A-stakeholders-care",),
            enforcement="ENFORCED",
            enforced_by=("test_what_now.py",),
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
                "A contradiction shall be modeled as a first-class Conflict NODE carrying axis + context + shared_assumption + steward, never as a `conflicts_with` edge between requirements."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The central ontological insight. An edge holds nothing; a node holds knowledge belonging to neither party (the axis, the context, the shared root) -- that is what makes contradictions first-class and clusterable. REPOINTED 2026-07-02 (Wave 7 move 4, P5 latent-connector cluster fix): assumptions moved from A-content-free-honest (an over-broad, unrelated content-freeness assumption shared by coincidence with two other unrelated requirements, producing a false 3-way latent-connector cluster) to A-conflict-is-a-node-not-an-edge, which names this requirement's actual premise."
            ),
            assumptions=("A-conflict-is-a-node-not-an-edge",),
            enforcement="ENFORCED",
            enforced_by=("check_conflict_has_axis", "check_conflict_has_context", "check_conflict_has_steward", "test_invariants.py::test_conflicts_with_is_not_a_relation_kind"),
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
                "Every requirement whose status begins with 'OPEN' shall carry a non-empty question of the form OPEN(<question>)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "An OPEN with no question is a hole no one can act on. The harness surfaces OPEN items by their question; emptiness defeats the point of marking the requirement open at all."
            ),
            assumptions=("A-text-grounded-in-models",),
            enforcement="ENFORCED",
            enforced_by=("check_open_has_question",),
        ),
        Requirement(
            id="R-rejected-preserved-not-deleted",
            claim=(
                "Requirements that are rejected shall be marked REJECTED and kept in the graph for history, never deleted."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Anti-relitigation. Without preserved REJECTED, the same dead ideas re-surface every quarter. The historian role depends on this preservation."
            ),
            assumptions=("A-stakeholders-care",),
            enforcement="ENFORCED",
            enforced_by=("test_rejected_preserved.py",),
        ),
        Requirement(
            id="R-axis-controlled-vocab",
            claim=(
                "Every Conflict.axis shall be the slug of an Axis declared in the graph's `axes` tuple."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Conflicts CLUSTER by axis — many C-nodes on one axis = one architectural choice. Free-text axes would fragment the cluster and hide the clustering signal."
            ),
            assumptions=("A-text-grounded-in-models",),
            enforcement="ENFORCED",
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
                "Every decision shall be personally signed by the human steward -- today: a decided_by: Stakeholder.id field on the DECIDED Conflict plus git commit authorship."
            ),
            owner="framework-author",
            status=(
                "SETTLED"
            ),
            why=(
                "M5, REPLACES split into R-trust-anchor-mechanism + R-trust-anchor-delegation-explicit-only per atomicity discipline (R-requirement-claim-is-atomic). Steward verdict 2026-07-02 (verbatim): «человек обязан сам подписать, если только явно не делегирует это в каждом случае или на кампанию вперед» (English: 'the human is obliged to sign personally, unless he explicitly delegates it, either per-case or for a campaign in advance'). This atom lands the DEFAULT (personal signature) half: R-decided-needs-human-signoff's decided_by field plus git commit authorship (the repo's own author trail) already implement the mechanism the OPEN question asked about (PGP/SSH/web-of-trust cadence) -- the steward's answer is that the existing decided_by + commit-author pairing IS the anchor; no additional cryptographic layer is required today. Cadence is 'per decision' (every DECIDED Conflict), not periodic -- the steward signs each contradiction resolution as it happens, which is stronger than 'quarterly/per-PR' polling. CORRECTION: enforced_by originally cited check_decided_has_decided_by, a thin non-registered delegator wrapping atomic sub-checks (see hotam_spec/invariants.py ~line 3415); the bijection check requires a real ALL_INVARIANTS member, so enforced_by is corrected to the two atomic checks that actually guard decided_by presence and validity."
            ),
            assumptions=("A-stakeholders-care", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("check_decided_has_nonempty_decided_by", "check_decided_by_is_known_stakeholder"),
        ),
        Requirement(
            id="R-critical-core-scope",
            claim=(
                "The set of requirement domains warranting the deferred formal layers (Z3 conflict-detector, Quint temporal, mutation testing) shall be declared."
            ),
            owner="domain-user",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-critical-core-methodology + R-critical-core-per-domain (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-critical-core-methodology + R-critical-core-per-domain (wave 2, decided by framework-author 2026-06-30) — (was: M7 resolved (P6 — §Conscience): the critical core for the methodology's OWN domain is the six invariants in CRITICAL_CORE_INVARIANTS — check_steward_not_a_member_owner, check_operator_steward_not_self, check_decided_has_decided_by, check_typed_anchors, check_no_dangling_ids, check_open_has_question. These six guard every path by which a contradiction could be introduced without being seen. Business-domain 'critical core' (money / access / SLA) is a separate per-domain calibration; the framework's own methodology critical core is now declared and property-tested via test_conscience.py.))"
            ),
            assumptions=("A-text-grounded-in-models",),
            enforcement="ENFORCED",
            enforced_by=("test_conscience.py",),
        ),
        Requirement(
            id="R-axis-gatekeeper-policy",
            claim=(
                "Axis-duplicate gatekeeping shall be a mandatory part of the axis-creation path (confront-style similarity check at creation time, tools/create_axis.py), refusing near-duplicate axes unless overridden by an explicit --force-new justification."
            ),
            owner="ai-agent",
            status=(
                "SETTLED"
            ),
            why=(
                "M3. Steward accepted the operator's recommendation 2026-07-02: the gatekeeper is born WITH the door -- it is not an optional toggle to switch on later, it is built into the axis-creation tool's own admission path from the moment that tool exists, exactly as create_entity_type.py validates its inputs before writing. tools/create_axis.py now exists: it reuses confront.py's lexical token/stem overlap scorer against every existing Axis's slug+description, refuses (exit 1) on a score >= threshold (default 0.35, naming the nearest existing axis), and admits only via --force-new '<justification>' which is folded into the landed proposal's why field (never a silent override). Exact-slug duplicates are refused unconditionally (no --force-new escape -- that is re-declaration, not creation). ENFORCED by spec/tests/test_tool_create_axis.py covering: positive (novel axis passes), negative (near-duplicate refused, exit != 0, nearest match named), force-new override path (justification recorded), exact-slug duplicate always refused, and the apply-side writer (_apply_axis_to_source: insert + duplicate-slug refusal)."
            ),
            assumptions=(),
            enforcement="ENFORCED",
            enforced_by=("tests/test_tool_create_axis.py::test_novel_axis_passes_gatekeeper", "tests/test_tool_create_axis.py::test_near_duplicate_refused", "tests/test_tool_create_axis.py::test_near_duplicate_refusal_names_nearest", "tests/test_tool_create_axis.py::test_exact_slug_duplicate_always_refused_even_with_force", "tests/test_tool_create_axis.py::test_force_new_overrides_refusal", "tests/test_tool_create_axis.py::test_force_new_justification_recorded", "tests/test_tool_create_axis.py::test_writer_inserts_new_axis", "tests/test_tool_create_axis.py::test_writer_refuses_duplicate_slug"),
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
                "M8 + M9. DECIDED 2026-06-30: P17 implemented the multi-domain layout (domains/<name>/graph.py + manifest.py + agents/director/) making the 'one file or split?' question moot -- the answer is per-domain directories, each owning its own graph.py, with gen_spec discovering all of them. Single-file spec/content/graph.py is superseded by this layout. Evidence: domains/hotam-spec-self/graph.py, spec/tools/gen_spec.py load_content_graph, R-domain-owns-graph-py SETTLED. (Wave 1 seed-coherence pass: enforced_by's second entry was the bare requirement id 'R-domain-owns-graph-py' rather than a resolvable test/check reference -- corrected to name the actual test that guards the domain-owns-graph-py claim, caught by the new check_enforced_by_resolvable invariant.)"
            ),
            assumptions=("A-bootstrap-self-applies", "A-graph-fits-memory"),
            enforcement="ENFORCED",
            enforced_by=("check_domain_manifest_exists_and_importable", "test_tool_create_domain.py::test_creates_required_files"),
        ),
        # --- DRAFT — proposed next-steps -----------------------------------
        Requirement(
            id="R-active-loop-playbooks",
            claim=(
                "Each what_now priority band shall have a documented agent PLAYBOOK plus a tools/apply_proposal.py that mechanically applies a steward-approved JSON proposal to spec/content/."
            ),
            owner="ai-agent",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES by R-active-loop-protocol + R-active-loop-apply-tool + R-active-loop-playbook-doc per atomicity discipline (R-requirement-claim-is-atomic). The original claim mixed three concerns: data-model, tool, documentation."
            ),
            assumptions=("A-stakeholders-care", "A-text-grounded-in-models"),
            enforcement="PROSE",
            enforced_by=(),
        ),
        Requirement(
            id="R-active-loop-protocol",
            claim=(
                "Three Proposed* dataclass types (ProposedRequirement, ProposedConflictTransition, ProposedRejection) shall exist as the protocol for steward-approved operator changes."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Atom of R-active-loop-playbooks (data-model concern). hotam_spec/proposal.py defines the three types."
            ),
            assumptions=("A-stakeholders-care", "A-text-grounded-in-models"),
            enforcement="ENFORCED",
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
                "At least one band-specific playbook shall exist under docs/playbooks/ describing the agent's role for that band."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Atom of R-active-loop-playbooks (documentation concern). docs/playbooks/P4-OPEN-ITEM.md is the first band playbook."
            ),
            assumptions=(),
            enforcement="ENFORCED",
            enforced_by=("test_playbooks_doc.py",),
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
                "A controlled vocabulary of methodology terms shall be generated under docs/gen/GLOSSARY.md, with a sync test that fails on undefined or unused terms."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-glossary-generated + R-glossary-sync-fails-dead + R-glossary-sync-fails-unused + R-glossary-drift-stable (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-glossary-generated + R-glossary-sync-fails-dead + R-glossary-sync-fails-unused + R-glossary-drift-stable (wave 2, decided by framework-author 2026-06-30) — (was: Terminology drift is its own kind of invisibility — 'axis' / 'dimension', 'steward' / 'owner', 'conflict' / 'tension' will fragment without it. Now ENFORCED: test_glossary_sync.py fires on any dead vocab or invented §-token, and test_docs_gen.py::test_glossary_md_up_to_date keeps GLOSSARY.md byte-stable.))"
            ),
            assumptions=("A-text-grounded-in-models", "A-python-stack"),
            enforcement="ENFORCED",
            enforced_by=("test_glossary_sync.py", "test_docs_gen.py::test_glossary_md_up_to_date"),
        ),
        Requirement(
            id="R-history-from-rejected-markers",
            claim=(
                "docs/gen/HISTORY.md shall be generated from REJECTED markers in requirement WHY blocks and from DECIDED/REVISIT_WHEN lifecycle states on Conflicts."
            ),
            owner="ai-agent",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-history-generated-from-rejected + R-history-generated-from-decided (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-history-generated-from-rejected + R-history-generated-from-decided (wave 2, decided by framework-author 2026-06-30) — (was: The historian artifact is now real: build_history() in tools/gen_spec.py materializes REJECTED requirements and DECIDED/REVISIT_WHEN conflicts into docs/gen/HISTORY.md. Anti-drift enforced by test_history_md_up_to_date; content coverage enforced by test_history_gen.py.))"
            ),
            assumptions=("A-text-grounded-in-models",),
            enforcement="ENFORCED",
            enforced_by=("test_history_gen.py", "test_docs_gen.py::test_history_md_up_to_date"),
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
                "hotam_spec.process shall be the FIRST opt-in behavioral aspect — Lifecycle + Steps + roles_required + drives_entities — added after the keystone Lifecycle abstraction lands."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES split into R-process-types-exist + R-process-opt-in + R-process-lifecycle-wellformed-aspect + R-process-roles-declared-aspect + R-process-goal-owner-is-operator-aspect + R-process-typed-anchors-extended (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-process-types-exist + R-process-opt-in + R-process-lifecycle-wellformed-aspect + R-process-roles-declared-aspect + R-process-goal-owner-is-operator-aspect + R-process-typed-anchors-extended (wave 2, decided by framework-author 2026-06-30) — (was: SETTLED (P9): hotam_spec/process.py ships Process + Step + Goal + TargetState + PROCESS_LIFECYCLE + GOAL_LIFECYCLE. The §Process aspect is opt-in (TensionGraph.processes defaults to empty). PR-closed-loop instantiates ONE worked example at the meta-domain level. Three new invariants enforce the behavioral surface: check_process_lifecycle_wellformed, check_process_roles_declared, and check_goal_owner_is_operator. check_typed_anchors extended for PR- and GOAL- prefixes. M12 resolved: Lifecycle is core; Process is the first opt-in aspect that proves the keystone supports new aspects without parallel machinery.))"
            ),
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("test_process.py", "check_process_lifecycle_wellformed", "check_process_roles_declared", "check_typed_anchors"),
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
                "An operator's RESIDENT working set shall not exceed its context budget (measured by budget.measure — for CRYSTAL_CHARS the char-length of root CLAUDE.md vs the host cap), with any excess flagged as a structural OVERLOADED contradiction by the harness."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Built (P2): check_operator_within_budget fires when the operator's budget.measure exceeds limit. History: NODE_COUNT originally measured the crystallized SUBSTRATE (requirements+conflicts+assumptions), which R-working-vs-substrate-budget declares free — this falsely flagged operators as near-OVERLOADED for the very act of crystallizing or keeping REJECTED history. OP-director moved to CRYSTAL_CHARS (limit=150000, the observed host char cap) — the RESIDENT crystal (root CLAUDE.md) is now the thing actually metered, per R-working-vs-substrate-budget. DomainScope narrowing deferred to P5+."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("check_operator_within_budget", "test_operator.py::test_check_operator_within_budget_fires", "test_operator.py::test_director_within_budget", "test_operator.py::test_check_operator_within_budget_crystal_chars_fires", "test_operator.py::test_check_operator_within_budget_crystal_chars_green_when_under"),
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
                "The methodology shall relieve an over-budget operator by splitting its domain into a bounded sub-domain owned by a spawned sub-operator (the horizontal lever)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "SETTLED (P8): the P8 REFLECTION band fires 'over-budget' → crystallize first → if still over, delegate. The signal path is structural: check_operator_within_budget (P1) detects the breach; the REFLECTION band (P0, tools/what_now.py::P_REFLECTION) names the path; docs/playbooks/ documents the procedure. DomainScope narrowing (per-operator sub-graph) remains a later phase but the SIGNAL — over-budget → delegate — exists today. Makes the methodology scale-free; generalizes 'agent never lost' to 'agent never overloaded'. Implementation: tools/what_now.py::P_REFLECTION + docs/playbooks/. Wave 1 mechanical-honesty pass (2026-07-02): promoted STRUCTURAL to ENFORCED -- reflect_over_budget_operators (hotam_spec.reflection) already names the crystallize-then-delegate sequence in its Finding.imperative; a new synthetic-graph unit test (test_reflect_over_budget_operators_names_crystallize_then_delegate) proves the ordered sequence is actually rendered, not merely claimed in prose."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("check_operator_within_budget", "test_reflection.py::test_reflect_over_budget_operators_names_crystallize_then_delegate"),
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
                "A Transition.guard may name an Assumption it rests on (drift seam) — when that Assumption dies, the guard is surfaced."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of R-statemachine-wellformedness (guard-on-assumption concern). Structural via Transition.guard_assumption field. Wave 1 mechanical-honesty pass (2026-07-02): promoted to ENFORCED by the new check_transition_guard_assumption_resolves invariant, which fires when a non-empty guard_assumption fails to resolve in assumption_ids(g) (dangling-ref family, RULES_AS_DATA_TABLE: TABLE_DRIVEN) -- plus a drift-fallout unit test proving a DEAD assumption referenced only via guard_assumption is still discoverable through graph.dead_assumptions(), never invisibly stale."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement="ENFORCED",
            enforced_by=("check_transition_guard_assumption_resolves", "test_entity_invariants.py::test_transition_guard_assumption_resolves_clean", "test_entity_invariants.py::test_transition_guard_assumption_resolves_dangling_fires", "test_entity_invariants.py::test_transition_guard_assumption_dead_assumption_still_visible_in_dependents"),
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
                "After an applied proposal lands (write + regen + pytest pass), the system shall verify the action that triggered the proposal is no longer present in the post-apply what_now diagnosis."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "P4 -- the feedback edge that makes Drive (P5) safe to automate. Without per-action closure, an apply can technically land (tests green) yet the same diagnosis re-surface -- the tick would spin without advancing. Structural answer: closure.check_closure asserts no Action with the original (kind, target) pair remains. (Wave 1 seed-coherence pass: enforced_by's second entry named a tool source path 'tools/closure.py::check_closure' rather than a test node-id -- corrected to the test file that already covers check_closure, caught by the new check_enforced_by_resolvable invariant.)"
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("test_closure.py",),
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
                "SETTLED (P5): the references-not-content discipline is now structurally bound. check_section_anchors_known ensures every SS-anchor cited in framework docstrings resolves in the glossary -- an operator that invents a SS-token immediately fires a P1 STRUCTURE violation. test_glossary_sync.py provides the test-time mirror. docs/playbooks/ mandates that every proposal cites the R-/C-/SS anchor it acts on; test_playbooks_doc.py is the test-time mirror of that doc's presence and content. The SSTick advisory output itself cites anchor ids in every action (target field). Together these make reference-not-content structurally visible and machine-checked. (Wave 1 seed-coherence pass: enforced_by's third entry was a bare doc path 'docs/playbooks/' rather than a resolvable test reference -- corrected to name the test file that guards that doc, caught by the new check_enforced_by_resolvable invariant.)"
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement="ENFORCED",
            enforced_by=("test_glossary_sync.py", "check_section_anchors_known", "test_playbooks_doc.py"),
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
                "Knowledge an operator cannot crystallize as any existing node shall be RECORDED as a candidate missing ontology type for steward review (not auto-acted)."
            ),
            owner="framework-reviewer",
            status="SETTLED",
            why=(
                "SETTLED (P6): the meta-signal surface exists — when the §Conscience Hypothesis property-sweep finds a class of contradictions that no existing critical-core invariant can express, the property-test failure IS the recording mechanism (a clear, machine-visible meta-signal that a new type is needed). The steward still decides whether to add the type; the recording itself is now structural, not manual."
            ),
            assumptions=("A-most-knowledge-crystallizable",),
            enforcement="STRUCTURAL",
            enforced_by=("test_conscience.py", "CRITICAL_CORE_INVARIANTS"),
            enforceability="INHERENTLY_PROSE",
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
                "An operator's context budget shall measure the SIZE of its resident business content in bytes/chars (tokens when the host exposes them), not node counts."
            ),
            owner="framework-author",
            status=(
                "SETTLED"
            ),
            why=(
                "M17. Steward verdict 2026-07-02 (verbatim): «наверно, это размер бизнес-требований. Лучше в байтах или в токенах» (English: 'probably it's the size of the business requirements. Better in bytes or in tokens'). Landed reality already matches: hotam_spec.operator.CRYSTAL_CHARS ('measure = len(root CLAUDE.md) in characters') is the current byte-approximation of resident business content, wired into check_operator_within_budget and used by the live meta-domain operator (context_budget=ContextBudget(limit=150000, measure=\"CRYSTAL_CHARS\")). TOKEN_ESTIMATE is declared in BUDGET_MEASURES as the token-exact seam the steward names ('в токенах') for when the host exposes token counts, but is deferred (not measured yet) since no host API is wired. NODE_COUNT (the original M17 default, size = |requirements|+|conflicts|+|assumptions|) is DEMOTED to a legacy/backward-compat measure: it counts the crystallized SUBSTRATE, which R-working-vs-substrate-budget declares free, so it over-penalizes crystallizing; it remains selectable only for operators/tests that opted into it before CRYSTAL_CHARS existed. No wording change needed elsewhere: check_operator_within_budget's docstring already states this hierarchy verbatim."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("check_operator_within_budget", "test_operator.py"),
        ),
        Requirement(
            id="R-partition-vs-border",
            claim=(
                "Operator sub-domains shall relate to the parent graph by a single declared discipline (strict partition or declared-border overlap)."
            ),
            owner="framework-author",
            status=(
                "REJECTED"
            ),
            why=(
                "REJECTED -- REPLACES R-scope-is-projection + R-scope-overlap-generated + R-overlap-single-presenter. The open question ('strict partition, or overlap on explicitly-declared delegation borders?') presupposed a binary; the actual answer is a third option -- a PROJECTION. Operator sub-domains are neither a strict partition (which would forbid any shared node) nor an ad-hoc declared-border overlap (which would need hand-maintained edge lists); they are a computed, always-current id-set VIEW over the one shared graph (hotam_spec.scope_projection.project_scope), and any overlap between two operators' views is itself computed and rendered visibly (scope_overlap + the generator's OVERLAP block) rather than declared by hand or forbidden outright. A contested node under such an overlap resolves to exactly one deterministic presenter (R-overlap-single-presenter), closing the ambiguity the OPEN question was protecting against without adopting either extreme it named. -- (was: M18. Delegation (R-context-bounded-delegation) needs to know whether shared objects are forbidden (partition) or first-class borders; the two give different drift behavior.)"
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="PROSE",
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
                "Operator epistemics (observations, beliefs, reasoning) shall live in the working dialogue, crystallized into the substrate only on request."
            ),
            owner="framework-reviewer",
            status=(
                "SETTLED"
            ),
            why=(
                "M21. Steward verdict 2026-07-02 (verbatim): «нужно думать в чате. И по просьбе в чате записывать свои мысли» (English: 'need to think in chat. And on request in chat, write down one's thoughts'). This settles the M21 question against modeling belief-vs-reality drift as new first-class types: an operator's live reasoning stays in the session dialogue (the mediation loop's CONFRONT/TRANSLATE steps), and is crystallized into a typed node ONLY when explicitly asked to (mirrors R-crystallize-knowledge-to-code's on-demand discipline). REQUALIFIED 2026-07-02 (Wave 7 move 2 honesty pass): previously carried default enforceability=ENFORCEABLE, which listed it as closeable debt in UNENFORCED.md -- but no check_* can ever verify 'an operator kept its reasoning in the dialogue and crystallized only on request' as a property of the committed graph (it is a fact about a conversation, not the graph state). This is the same honesty class as R-initiator-supplies-domain-content: a real, permanent dialogue discipline that is INHERENTLY_PROSE, not ENFORCEABLE-but-unbuilt. RE-ATOMIZED Wave 8 move 2 (2026-07-03): audit_atomicity.py flagged the prior claim COMPOUND -- it also asserted 'Assumption remains the only belief-carrying node type -- no separate Observation/Evidence types', which duplicates R-no-observation-type (SETTLED, ENFORCED) verbatim; that mechanically-checkable half already lives there per R-no-observation-type's own why field ('Split out as its own atom so the mechanical slice can honestly reach ENFORCED while the parent claim stays STRUCTURAL'). Trimmed this claim to the dialogue-discipline half only (the genuinely INHERENTLY_PROSE half with no duplicate owner) -- R-no-observation-type remains the sole, undiluted owner of the negative-existence half via its own relations. No semantic narrowing of what the operator actually promises: both halves are still live and enforced exactly as before, now under two non-overlapping atoms instead of one compound one; this is a wording-only re-atomization of the SAME id (not a rejection+new-id split -- both halves already had a distinct home, one pre-existing (R-no-observation-type) and one here, so no new sibling atom is needed)."
            ),
            assumptions=("A-most-knowledge-crystallizable",),
            enforcement="STRUCTURAL",
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-rules-as-data",
            claim=(
                "Regular invariant families (homogeneous per-entity structural checks such as dangling-refs, typed-anchors, and lifecycle-membership) shall be CLASSIFIED as table-driven data (RULES_AS_DATA_TABLE) with a coherence invariant distinguishing them from irreducibly bespoke invariants (identity derivation, cross-entity bijections, docstring/body coherence, budget arithmetic) -- derivation of check_* function bodies from the table is deferred until it can coexist with R-method-matches-docstring's per-function inspectable-source requirement."
            ),
            owner="framework-reviewer",
            status=(
                "SETTLED"
            ),
            why=(
                "M22, decided 2026-07-02 by steward verdict HYBRID (criterion: whatever is most convenient for the agents). Landed reality: RULES_AS_DATA_TABLE (31 TABLE_DRIVEN / 32 BESPOKE rows) plus check_rules_as_data_classification_coherent classify every check_* by family; the check_* bodies themselves remain hand-written, NOT derived from the table. Empirically verified blocker to body-derivation: check_method_matches_docstring (R-method-matches-docstring) calls inspect.getsource(fn) on every check_* to enforce literal, inspectable Violation(...) messages -- functions produced by a table-driven factory are closures, and inspect.getsource raises OSError on them (no source file to point at). The alternative, exec()-based codegen from the table, would satisfy inspect.getsource mechanically but produce opaque, non-diffable bodies -- LESS convenient for an agent auditing behavior, which is the steward's own stated criterion. So classification-as-data landed; derivation-of-bodies is deliberately deferred, not abandoned, until a mechanism exists that keeps generated check_* functions individually inspectable. Wave 8 move 2 atomicity pass: audit_atomicity.py flagged the claim's semicolon COMPOUND -- inspection shows the clause after the semicolon is a scope/status disclaimer about what this ONE classification promise deliberately does NOT yet cover (body derivation), not a second independent obligation; the claim never promised body-derivation, it explicitly defers it. Reworded the semicolon to the codebase's existing '--' scope-disclaimer convention so audit_atomicity's _CLAIM_SCOPE_CLAUSE exemption recognizes it -- wording/classifier-alignment fix, no semantic change, not a split."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement="ENFORCED",
            enforced_by=("check_rules_as_data_classification_coherent",),
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
                "Detection of 'uncrystallizable knowledge = missing type' is human judgment: the operator records the candidate in the graph as an OPEN requirement (or a DRAFT), and the steward decides whether to add the ontology type."
            ),
            owner="framework-reviewer",
            status="SETTLED",
            why=(
                "M30. DECIDED 2026-06-30: human judgment, not automated. Rationale: R-uncrystallizable-is-missing-type (SETTLED P6) already establishes that the operator records the signal as a node; the §Conscience property-sweep (test_conscience.py) surfaces the meta-signal structurally when a class of contradictions cannot be expressed by existing critical-core invariants. Automating the type-creation decision would violate R-ai-presents-not-decides. The whole audit-backlog-residue checkpoint pattern + the DRAFT queue IS the recording mechanism. The steward is the decider; the graph is the recorder. Evidence: R-uncrystallizable-is-missing-type SETTLED STRUCTURAL; test_conscience.py; R-ai-presents-not-decides SETTLED."
            ),
            assumptions=("A-most-knowledge-crystallizable",),
            enforcement="STRUCTURAL",
            enforced_by=("test_conscience.py", "CRITICAL_CORE_INVARIANTS"),
            enforceability="INHERENTLY_PROSE",
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
            status="REJECTED",
            why=(
                "REJECTED -- REPLACES by R-backend-scope (SETTLED). M37 steward verdict 2026-07-02 (verbatim): «не важно -- каждый умный агент под себя допишет» (English: 'it doesn't matter -- every smart agent will adapt it for itself'). The steward explicitly declined to build a designed OperatorBackend protocol (get_context_state / request_steward_approval / delegate): the core surface (spec/tools/*.py CLIs, JSON Proposed* shapes, pytest verification) already has no dependency on any one agent's runtime and stays backend-neutral BY CONSTRUCTION, not by an abstraction layer built ahead of a second real backend. Building the protocol now would be exactly the speculative-engineering-against-hypothetical-backends R-speculative-aspects-frozen already warns against. R-backend-scope (SETTLED, PROSE) already carries this verdict verbatim in its why and is the surviving atom; R-operator-backend-protocol is rejected rather than left DRAFT so the graph does not carry a dead aspiration as open debt. — (was: Today tools/ implicitly assume Claude Code (Agent tool, Bash, chat-steward). BUILD-TRIGGER: a SECOND concrete backend becomes real (CI runner, a different coding agent, or a programmatic steward). Until then, abstracting for hypothetical backends is the big-bang-up-front antipattern (weight ∝ cost of an unnoticed conflict). See OPEN R-backend-scope (which backends are real?).)"
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
            status="REJECTED",
            why=(
                "REJECTED -- REPLACES by R-budget-measure + R-working-vs-substrate-budget (both SETTLED/ENFORCED) via CRYSTAL_CHARS. The phi-cap claim hardcoded a numeric ceiling (1_000_000 / phi ~= 618033 tokens) picked before the operator had any real measure of its own crystal size, and it named TOKENS while the operator's actual context arithmetic (check_operator_within_budget) counts CHARS against a host cap -- a unit mismatch that would have silently lied the moment anyone tried to wire a check against it. R-budget-measure already settles the honest unit (bytes/chars, not node counts, not a borrowed-constant token cap) and R-working-vs-substrate-budget already settles what the budget bounds (the WORKING store, leaving crystallized substrate free). check_operator_within_budget's CRYSTAL_CHARS measure (root CLAUDE.md char-length vs a declared host cap, currently 150000) is the real, live, already-ENFORCED mechanism; the phi-cap was a speculative alternate arithmetic that never had an enforcer and would have required reconciling two different budget units had it ever been built. R-claude-md-tree-of-crystals's trigger is retargeted from this dead cap onto the CRYSTAL_CHARS warn threshold in a separate proposal. — (was: The context-bounded-delegation law (R-context-bounded-delegation) applied to the operator's OWN body, not just the graph. BUILD-TRIGGER: CLAUDE.md crosses ~50% of the φ-cap (~309K tokens) — today it is ~7K (~1%), so a budget CHECK now would be machinery guarding a condition that cannot fire. The LIVE-STATE block already reports φ-headroom; the check + the REFLECTION P0 wiring land when headroom actually narrows.)"
            ),
            assumptions=("A-finite-context-operators",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-claude-md-tree-of-crystals",
            claim=(
                "When the root CLAUDE.md's resident CRYSTAL_CHARS size approaches the operator's declared host cap, the operator shall move sections into nested <subdir>/CLAUDE.md crystals and keep only a heading + a when-to-read pointer in the root -- a tree of crystals, one per sub-domain."
            ),
            owner="framework-author",
            status="DRAFT",
            why=(
                "R-operator-crystal-is-claude-md made recursive: the delegation hierarchy is a tree of CLAUDE.md crystals (Claude Code natively loads nested CLAUDE.md by directory). Retargeted 2026-07-02 (Wave 2 burn-down): the original BUILD-TRIGGER named R-claude-md-budget-phi-cap, which is now REJECTED (dead hardcoded token ceiling, wrong unit -- see its rejection reason). The real, live, already-ENFORCED measure is check_operator_within_budget's CRYSTAL_CHARS arithmetic (root CLAUDE.md char-length vs the declared host cap, currently 150000 chars per LIVE-STATE) -- R-budget-measure + R-working-vs-substrate-budget. BUILD-TRIGGER (retargeted): the CRYSTAL_CHARS warn threshold in gen_spec's LIVE-STATE narrows meaningfully (today's resident crystal is well under half the cap) -- premature to build the splitting mechanism until headroom actually narrows against the SAME measure the operator already checks every turn."
            ),
            assumptions=("A-finite-context-operators", "A-bootstrap-self-applies"),
            enforcement="PROSE",
            enforced_by=(),
        ),
        Requirement(
            id="R-subagent-gets-its-claude-md",
            claim=(
                "A delegated sub-operator shall receive its OWN crystal, a CLAUDE.md generated from its sub-domain."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "The Delegation.returns conclusions-only contract (R-delegation-conclusions-only) made concrete for sub-operators. BUILD-TRIGGER: spawn_agent built -- NOW FIRES (P22.C). Promoted DRAFT->SETTLED on P22.C: spawn_agent tool exists at spec/tools/spawn_agent.py, composing per-agent CLAUDE.md into the subagent prompt before dispatch. RE-ATOMIZED Wave 8 move 2 (2026-07-03): audit_atomicity.py flagged the prior claim COMPOUND ('and' joining 'shall receive its OWN crystal' with 'and return CONCLUSIONS only, never raw context') -- the second half duplicates R-delegation-conclusions-only verbatim ('the sub-operator shall return only CONCLUSIONS with shared objects declared as an explicit border, never raw detail'), which already SETTLED that half independently and generically (for any delegated sub-operator, not just ones with a CLAUDE.md). Trimmed this claim to the crystal-provisioning half only -- the concrete, spawn_agent-specific promise this atom's own enforcer (test_composite_prompt_contains_crystal_and_task) actually verifies. R-delegation-conclusions-only remains the sole owner of the return-conclusions-only half. No semantic narrowing: the return-conclusions-only guarantee is still live, enforced exactly as before, just no longer restated redundantly here."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("test_composite_prompt_contains_crystal_and_task",),
        ),
        Requirement(
            id="R-backend-scope",
            claim=(
                "The framework names no target backends: the core (graph/JSON proposals/CLI/pytest) stays backend-neutral by construction, and adapting the skin is the adopting agent's own concern."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "M37. Steward verdict 2026-07-02 (verbatim): «не важно -- каждый умный агент под себя допишет» (English: 'it doesn't matter -- every smart agent will adapt it for itself'). This settles M37 by declining to name concrete backends (CI runner / alternate coding agent / programmatic or human steward): the core surface (spec/tools/*.py CLIs, JSON Proposed* shapes, pytest verification) already has no dependency on any one agent's runtime, so it stays backend-neutral by construction rather than by a designed OperatorBackend protocol. R-operator-backend-protocol remains gated/unbuilt -- the steward's answer is that building it now would be speculative engineering against hypothetical backends R-speculative-aspects-frozen already warns against, not that the protocol is wrong in principle. REQUALIFIED 2026-07-02 (Wave 7 move 2 honesty pass): previously carried default enforceability=ENFORCEABLE, which listed it as closeable debt in UNENFORCED.md -- but no check_* can ever verify 'no target backend is named' or 'adapting the skin is the adopting agent's own concern' as a runtime property of the committed graph (it is a design-stance/non-decision about scope, not a structural fact). This is the same honesty class as R-initiator-supplies-domain-content and R-observation-evidence-scope: a real, permanent positional discipline that is INHERENTLY_PROSE, not ENFORCEABLE-but-unbuilt. The measurable SLICE of this claim already reached ENFORCED separately: R-core-imports-stdlib-or-hotam-spec-only (SETTLED, ENFORCED) mechanically verifies the CONSTRUCTION half -- an AST import scan proving spec/src/hotam_spec/*.py imports nothing beyond stdlib + itself, so no backend dependency has silently crept into the core -- that narrower, structural half is where the machine-checkable guarantee correctly lives; R-backend-scope itself states the broader naming/positional stance, which stays INHERENTLY_PROSE. Requalifying (not fabricating an enforcer) keeps the burn-down meter honest -- the debt this requirement represented was never real closeable debt in the first place."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="PROSE",
            enforced_by=(),
            enforceability="INHERENTLY_PROSE",
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
                "Scoping rule, structurally enforced by file layout. SETTLED — already true today."
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_shared_tools_location.py",),
        ),
        Requirement(
            id="R-docs-generated-from-requirements",
            claim=(
                "Per-topic narrative files under `docs/methodology/atoms/<topic>.md` shall be generated from SETTLED requirements grouped by topic, with hand-edits forbidden by a meta-test."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Director asked: build a system that generates topic-grouped descriptions in the docs folder. BUILD: this phase. Subdirectory `docs/methodology/atoms/` keeps generated files cleanly separate from the existing hand-written `docs/methodology/README.md`. (Wave 1 seed-coherence pass: enforced_by's second entry named a tool source path 'tools/gen_spec.py::build_methodology_atoms' rather than a test node-id -- corrected to the test that already exercises the per-topic builder functions, caught by the new check_enforced_by_resolvable invariant.)"
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_docs_gen.py::test_methodology_atoms_up_to_date",),
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
                "Every Requirement.m_tag (when non-empty) shall match `^M[1-9][0-9]*$`, be unique across the graph, and appear only on OPEN requirements."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The M-decision registry (DECISIONS.md) bijection depends on m_tag discipline. Was orphan policy; now claimed. Atomic — one concern (M-tag well-formedness) with three sub-rules all in one check_*; if we later split the check, this R splits with it."
            ),
            assumptions=("A-text-grounded-in-models",),
            enforcement="ENFORCED",
            enforced_by=("check_m_tag_valid_format", "check_m_tag_unique", "check_m_tag_open_only"),
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
                "SETTLED (BUILD-TRIGGER fired): R-operator-prompt-from-substrate has landed -- gen_spec generates the CONSTITUTION block from SETTLED requirements, making the constituting set explicit and machine-known. PROMOTED STRUCTURAL->ENFORCED (Wave 10, closeable->0): check_constituting_not_in_unresolved_conflict is the machine-checkable face of pairwise consistency -- no unresolved Conflict (DETECTED/ACKNOWLEDGED) may hold two SETTLED constituting atoms while the CONSTITUTION presents both as settled truth. Scoped to the self-host graph (the one that DEFINES this atom): a business domain's DETECTED conflict with SETTLED members is a NORMAL held tension awaiting its steward (e.g. hotam-dev C-ec1ec532), NOT incoherence, and those atoms do not compose the operator-prompt -- so the demand binds only to the self-host constitution index (honest scope per this atom's own claim 'composing the operator-prompt')."
            ),
            assumptions=("A-bootstrap-self-applies",),
            enforcement="ENFORCED",
            enforced_by=("check_constituting_not_in_unresolved_conflict", "test_invariants.py::test_constituting_convergence_fires_on_self_host_detected"),
        ),
        Requirement(
            id="R-requirement-claim-is-atomic",
            claim=(
                "Each `Requirement.claim` shall assert exactly one concern, with conjunctions of distinct concerns decomposed into separate requirements."
            ),
            owner="framework-reviewer",
            status="SETTLED",
            why=(
                "SETTLED (BUILD-TRIGGER fired): tools/audit_atomicity.py exists and surfaces Requirements with compound claims as a deterministic audit signal (docs/gen/AUDIT.md). Waves 1-3 of atomization applied the discipline to the meta-domain, decomposing compound requirements into single-concern atoms. ENFORCED Wave 8 move 2 (2026-07-03): audit_atomicity.py's requirement-claim audit was rescoped to LIVE promises only (status SETTLED or OPEN(...) -- REJECTED is frozen history, DRAFT is not yet a promise, see audit_atomicity.py's own RULE/WHY), which shrank the atomicity_compound_baseline.json ratchet baseline from 21 stale/misscoped ids down to 6 genuine live compounds, then to 0 after this wave's splits (R-observation-evidence-scope, R-subagent-gets-its-claude-md, R-land-tier-trace 3-way) and classifier-alignment rewords (R-tiered-gate-not-a-commit-gate, R-rules-as-data, both false-positive semicolon/scope-clause claims). With the baseline empty, test_atomicity_ratchet.py::test_no_new_compound_requirements_beyond_baseline is now a STRICT zero-COMPOUND gate for every live SETTLED/OPEN requirement claim -- the exact mechanical enforcer this requirement always needed. R-atomicity-ratchet-no-growth (SETTLED, ENFORCED) remains the permanent, more general growth-direction mechanism (tolerates future HONEST debt without red-lining CI); this atom's own enforced_by now points directly at the strict test that is meaningful precisely because the baseline is empty."
            ),
            assumptions=("A-text-grounded-in-models",),
            enforcement="ENFORCED",
            enforced_by=("test_atomicity_ratchet.py::test_no_new_compound_requirements_beyond_baseline",),
        ),
        Requirement(
            id="R-check-method-is-atomic",
            claim=(
                "Each `check_*` invariant shall enforce exactly one rule, with multi-rule enforcers split into separate `check_*` functions."
            ),
            owner="framework-reviewer",
            status="SETTLED",
            why=(
                "SETTLED (BUILD-TRIGGER fired): tools/audit_atomicity.py flags check_* functions with compound conditions in docs/gen/AUDIT.md (same wave as R-requirement-claim-is-atomic). The tool walks invariants.py and reports multi-rule families, making the discipline machine-auditable. ENFORCED Wave 8 move 2 (2026-07-03): the check_* (invariant) side of the atomicity_compound_baseline.json ratchet baseline was already empty coming into this wave (porция 1's AST-loop + N-sub-rules classifier fixes burned it down to 0 with no false positives remaining). With the baseline empty, test_atomicity_ratchet.py::test_no_new_compound_invariants_beyond_baseline is now a STRICT zero-COMPOUND gate for every registered check_* function -- the exact mechanical enforcer this requirement always needed. R-atomicity-ratchet-no-growth (SETTLED, ENFORCED) remains the permanent, more general growth-direction mechanism; this atom's own enforced_by now points directly at the strict test that is meaningful precisely because the baseline is empty."
            ),
            assumptions=("A-text-grounded-in-models",),
            enforcement="ENFORCED",
            enforced_by=("test_atomicity_ratchet.py::test_no_new_compound_invariants_beyond_baseline",),
        ),
        Requirement(
            id="R-bijection-r-to-enforcer-draft",
            claim=(
                "Each ENFORCED Requirement shall name exactly one enforcer in its `enforced_by` after atomization is complete."
            ),
            owner="framework-reviewer",
            status="REJECTED",
            why=(
                "REJECTED — SUPERSEDED by R-bijection-r-to-enforcer SETTLED (wave 3 outcome). The SETTLED version generalizes this claim: every SETTLED/ENFORCED requirement must name an existing check_* in ALL_INVARIANTS or a real test_*, enforced by check_bijection_r_to_enforcer. The original id was duplicated with the SETTLED version; renamed to R-bijection-r-to-enforcer-draft for history preservation."
            ),
            assumptions=("A-text-grounded-in-models",),
            enforcement="PROSE",
            enforced_by=(),
        ),
        Requirement(
            id="R-agent-is-a-directory",
            claim=(
                "A domain-agent shall be represented as a directory at `spec/agents/<name>/`."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "The user's clarification today: agent = folder with own logic, not sh-invocation. BUILD-TRIGGER: a real second operator (beyond OP-director) needs to be instantiated. Promoted DRAFT→SETTLED on first instantiation: spec/agents/framework-agent/ exists as concrete evidence."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("check_agent_has_agents_subdir", "check_agent_has_docs_subdir", "test_tool_create_agent.py"),
        ),
        Requirement(
            id="R-agent-has-own-crystal",
            claim=(
                "Each domain-agent shall carry its own `CLAUDE.md` file as its "
                "operator-prompt crystal."
            ),
            owner="ai-agent",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES R-sub-agent-crystal-triad + R-claude-md-consolidates-when-single-agent. The unconditional claim 'each domain-agent shall carry its own CLAUDE.md file' is now false: the domains/hotam-spec-self/agents/ scaffold (director + framework-agent) was deleted in task #101 down to a bare scope.py identity marker, with zero actively-spawned sub-agents holding their own CLAUDE.md. R-agent-has-own-crystal was PROSE, promoted DRAFT->SETTLED on the now-deleted framework-agent scaffold; that evidence no longer exists. R-sub-agent-crystal-triad records the governing content-shape rule for WHEN a real sub-agent is spawned; R-claude-md-consolidates-when-single-agent records that until then, one file suffices. — (was: The agent's prompt is independent of the director's. BUILD-TRIGGER: same as R-agent-is-a-directory. Promoted DRAFT→SETTLED on first instantiation: spec/agents/framework-agent/CLAUDE.md generated and populated.)"
            ),
            assumptions=("A-finite-context-operators", "A-compaction-loses-working"),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-agent-has-own-tools-dir",
            claim=(
                "Each domain-agent shall carry a `tools/` subdirectory holding its private tools."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Scoping the agent's available actions — private tools are not exposed to other agents. BUILD-TRIGGER: same as R-agent-is-a-directory. Promoted DRAFT→SETTLED on first instantiation: spec/agents/framework-agent/tools/ exists as concrete evidence."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("check_agent_has_tools_subdir", "test_tool_create_agent.py::test_creates_required_files", "test_invariants.py::test_check_agent_has_tools_subdir_fires_on_missing_tools"),
        ),
        Requirement(
            id="R-agent-imports-framework",
            claim=(
                "An agent's code shall import the framework body (`hotam_spec.*`) as "
                "shared infrastructure; the framework body is owned by no single agent."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED -- REPLACES split into R-agent-code-imports-framework + R-framework-owned-by-no-agent (Wave 2 burn-down, decided by framework-author 2026-07-02) per atomicity discipline (R-requirement-claim-is-atomic). audit_atomicity.py flagged this claim COMPOUND (semicolon splits 2 segments): (1) an agent's code shall import hotam_spec.* as shared infrastructure, and (2) the framework body is owned by no single agent. These are two distinct concerns -- (1) is a mechanically checkable import-direction fact about agent source files, (2) is a non-code ownership/governance stance about the framework body. Splitting lets (1) reach a real enforcer (a static AST import-direction scan, mirroring R-core-imports-stdlib-or-hotam-spec-only's pattern) without diluting it with the unenforceable ownership half. — (was: Keeps the framework content-free (R-content-free-framework) while letting agents specialize. BUILD-TRIGGER: same as R-agent-is-a-directory.)"
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
            status="REJECTED",
            why=(
                "REJECTED -- REPLACES split into R-task-spawn-is-a-hand + R-task-spawn-no-cross-invocation-persistence (Wave 2 burn-down, decided by ai-agent 2026-07-02) per atomicity discipline (R-requirement-claim-is-atomic). audit_atomicity.py flagged this claim COMPOUND ('and' connects clause with verb, 'and does'): (1) a task-agent invocation IS a hand (a classification claim), and (2) it returns conclusions and does not persist between invocations (a behavioral claim). Splitting separates the naming/classification half from the persistence-behavior half so each can be honestly graded on its own enforceability -- the classification is discipline/naming, the non-persistence half is closer to (but not fully) checkable via the spawn-log's structure. — (was: The user's distinction today: hands vs agents. BUILD-TRIGGER: D3's spawn-log writer exists — the log is the structural recording of this ephemeral act.)"
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
                "The spawn_agent tool shall append a spawn-log entry to spec/.runtime/spawn-log.jsonl -- with parent, child kind, task subject, and stamp -- on every invocation."
            ),
            owner="ai-agent",
            status="SETTLED",
            why=(
                "Ephemera, not committed substrate -- same altitude as spec/.runtime/context.json. BUILD-TRIGGER: spawn-log infrastructure built -- NOW FIRES (P22.C). Promoted DRAFT->SETTLED on P22.C. CLAIM NARROWED (Wave 10 move 2, honesty of the spawn seam): the prior claim ('Task-agent invocations shall be appended...') asserted COVERAGE of every real invocation, but its enforcer test_spawn_log_written proves only the MECHANISM -- that the tool appends a well-formed row when invoked. Narrowing the claim to the tool-mechanism it actually proves keeps this atom honestly ENFORCED (no claimed-but-unguaranteed coverage). The coverage claim (every real host spawn actually leaves a trace) now lives, honestly STRUCTURAL, as hotam-dev's R-host-spawn-leaves-trace (refining R-spawn-logged), enabled by spawn_agent.py --log-only. audit 2026-07-03: the real spawn-log was empty despite ~30+ host spawns -- exactly the coverage gap the narrowed claim no longer over-promises."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("test_spawn_log_written", "test_tool_spawn_agent.py::test_log_only_writes_row_without_composing_prompt"),
        ),
        Requirement(
            id="R-tools-registry-generated",
            claim=(
                "The list of available tools shall be generated by scanning `spec/tools/*.py` (and per-agent `spec/agents/<name>/tools/*.py`), never hand-maintained."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "SETTLED (BUILD-TRIGGER fired): gen_spec.py auto-projects R-tool-* requirements from Canon docstrings in spec/tools/*.py (lines ~220-266 and ~1614-1789 in gen_spec.py). The REPO-MAP and AGENT-MAP blocks in CLAUDE.md include tool entries generated from the filesystem scan, never hand-maintained. The docs-as-code pattern applied to tool inventories — a new tool without a Canon docstring simply won't appear, making drift structurally visible."
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_repo_map.py::test_repo_map_complete", "test_repo_map.py::test_repo_map_tool_xref_present"),
        ),
        Requirement(
            id="R-private-tools-in-agent-folder",
            claim=(
                "Tools available only to one agent shall live under that agent's tools/ subdirectory."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Counterpart to R-shared-tools-in-spec-tools -- private scope. BUILD-TRIGGER fired: R-agent-is-a-directory and R-agent-has-own-tools-dir have both landed ENFORCED and create_agent.py always scaffolds a tools/ subdir per agent. Promoted DRAFT->SETTLED reusing check_agent_has_tools_subdir (the same structural check already proves every agent directory carries its own tools/ subdir, which IS the location this claim requires private tools to live in) rather than fabricating a second near-duplicate check -- the location constraint and the subdir-existence constraint are the same structural fact viewed from two requirement angles."
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("check_agent_has_tools_subdir",),
        ),
        Requirement(
            id="R-tree-of-crystals-cognitive-trigger",
            claim=(
                "Tree-of-crystals delegation shall fire when a sub-domain's detail "
                "granularity exceeds the director's altitude, independently of the "
                "φ-cap size trigger."
            ),
            owner="framework-author",
            status="REJECTED",
            why=(
                "REJECTED — REPLACES R-claude-md-tree-of-crystals: the cognitive-trigger framing rested on the already-REJECTED φ-cap notion; the only real delegated trigger that survives is the CRYSTAL_CHARS warn threshold on the resident crystal (contextbudget), which is already first-class. No separate cognitive-trigger node is needed. — (was: Second trigger besides R-claude-md-budget-phi-cap: cognitive load, not token load. BUILD-TRIGGER: a heuristic detector exists (planned: count of distinct concerns per sub-domain crossing a threshold).)"
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
            status="REJECTED",
            why=(
                "REJECTED — REPLACES the point-installer requirements (R-tool-setup-context-hook et al.): the envisioned single setup_claude.py that would generate settings was never built; the surgical, per-concern installers (setup_context_hook.py --patch-global and friends) won on the ground — configuration is applied one concern at a time, opt-in and user-run, not by one settings-generating entry point. — (was: Single source of truth: Python file generates the JSON; meta-test enforces equality. BUILD-TRIGGER: the prerequisite atomization (R-requirement-claim-is-atomic and R-check-method-is-atomic) has landed — without it, setup_claude.py easily slides back into compoundness.)"
            ),
            assumptions=("A-python-stack",),
            enforcement=PROSE,
            enforced_by=(),
        ),
        Requirement(
            id="R-audit-atomicity-tool",
            claim=(
                "An audit of substrate atomicity (compound requirements + compound check_* invariants + R↔enforcer bijection + orphan analysis) shall be performed by a deterministic tool `spec/tools/audit_atomicity.py`, not by a one-off hand invocation."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "SETTLED (BUILD-TRIGGER fired): spec/tools/audit_atomicity.py exists and was used in atomization waves 1-3, emitting docs/gen/AUDIT.md deterministically. The verdict checkpoint (docs/checkpoints/framework-agent-audit-verdict.md) is now superseded by the tool's output. R-prefer-tool-over-hand is now honored for atomicity audits. STRUCTURAL because the tool's output is advisory (P0 REFLECTION); no blocking invariant yet."
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_tool_audit_atomicity.py",),
        ),
        Requirement(
            id="R-context-hook-piggybacks-cah-stamp",
            claim=(
                "The project-local hook (context_producer.py, wired via PostToolUse + Stop in .claude/settings.local.json) shall read a context-cache.json written by the global statusline script and write spec/.runtime/context.json."
            ),
            owner="framework-author",
            status="DRAFT",
            why=(
                "Design refined by the 2026-07-02 --patch-global investigation: the global cah-stamp/cah-status pipeline had NO existing on-disk cache for ctx_pct (rate-limits.json holds only 5h/weekly/effort, never context %) -- so this cannot honestly 'read the global cah-stamp cache' as originally worded. Instead spec/tools/setup_context_hook.py --patch-global (a separate, user-run, opt-in tool -- never applied automatically) surgically patches the global cah-status.js to ALSO write a sibling ~/.claude/cah-bin/cache/context-cache.json = {ctx_pct, model, stamp}; context_producer.py now reads that path (CAH_CONTEXT_CACHE override) as a fallback when its stdin payload lacks ctx_pct. Stays DRAFT: the patch exists and is tested hermetically, but no user has yet run --patch-global --apply against the real global script, so the end-to-end chain is not yet proven live. BUILD-TRIGGER unchanged: R-setup-claude-generates-settings has landed."
            ),
            assumptions=("A-finite-context-operators",),
            enforcement="PROSE",
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
            assumptions=("A-text-grounded-in-models", "A-python-stack"),
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
            assumptions=("A-text-grounded-in-models", "A-python-stack"),
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
            assumptions=("A-text-grounded-in-models", "A-python-stack"),
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
            assumptions=("A-text-grounded-in-models", "A-python-stack"),
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
            assumptions=("A-text-grounded-in-models",),
            enforcement="ENFORCED",
            enforced_by=("test_history_gen.py", "test_docs_gen.py::test_history_md_up_to_date"),
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
            assumptions=("A-text-grounded-in-models",),
            enforcement="ENFORCED",
            enforced_by=("test_history_gen.py", "test_docs_gen.py::test_history_md_up_to_date"),
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
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
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
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
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
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
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
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
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
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
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
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
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
                "Atom of §Entity aspect (lifecycle wellformedness concern). An EntityType with an invalid lifecycle is structurally broken — check_entity_type_lifecycle_wellformed reuses check_lifecycle_wellformed (the §Lifecycle keystone) for zero-parallel machinery (M12)."
            ),
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
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
                "Atom of §Entity aspect (state validity concern). An instance with an unknown state is structurally invalid — mirrors check_requirement_status_in_lifecycle for requirements."
            ),
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
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
                "Atom of §Entity aspect (required-fields concern). A missing required field violates the declared EntityType schema and makes downstream traversal unreliable."
            ),
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
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
                "Atom of §Entity aspect (typed-anchor concern). Encodes both type and entity kind in the id, enabling unambiguous cross-reference (R-anchor-everything)."
            ),
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
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
                "Atom of §Entity aspect (referential integrity concern). A dangling reference field is the entity-level equivalent of a dangling Conflict member."
            ),
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("check_entity_instance_refs_resolve",),
        ),
        Requirement(
            id="R-entity-field-kind-known",
            claim=(
                "Every EntityField.kind shall be in ENTITY_FIELD_KINDS (string | number | enum | reference | state)."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of §Entity aspect (field-kind concern). An unknown kind breaks the discriminant for kind-specific invariants and future machine-checkable field validation."
            ),
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
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
                "Atom of §Entity aspect (typed-anchors concern). Without prefix validation, EntityInstance nodes escape the anchoring discipline (R-anchor-everything)."
            ),
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("check_typed_anchors_entity",),
        ),
        Requirement(
            id="R-process-drives-existing-entities",
            claim=(
                "Every entity slug referenced in a Process.drives_entities shall resolve to a declared EntityType slug in g.entity_types."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of §Process/§Entity coupling (referential integrity concern). A Process that references undeclared entity types is structurally broken — the coupling between the behavioral aspect and the entity aspect must be referentially sound."
            ),
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("check_process_drives_existing_entities",),
        ),
        Requirement(
            id="R-step-invokes-known-transition",
            claim=(
                "Every Step.transition (when non-empty) shall name a transition event declared in the driven EntityType.lifecycle."
            ),
            owner="framework-author",
            status="SETTLED",
            why=(
                "Atom of §Process/§Entity coupling (step-transition concern). A Step that invokes an undeclared transition is a structural dead-end — the lifecycle machine cannot process it."
            ),
            assumptions=("A-text-grounded-in-models", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
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
            enforcement="ENFORCED",
            enforced_by=("test_dependency_traversal.py::test_disjoint_components_are_parallelizable",),
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
            enforcement="ENFORCED",
            enforced_by=("test_dependency_traversal.py::test_chain_is_emitted_dependency_before_dependent",),
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
            enforcement="ENFORCED",
            enforced_by=("test_framework_claude_md_purity.py::test_exactly_one_claude_md_in_repo",),
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
            enforcement="ENFORCED",
            enforced_by=("test_embedded_thinking_tools.py",),
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
            assumptions=("A-text-grounded-in-models",),
            enforcement="ENFORCED",
            enforced_by=("test_conscience.py", "check_no_dangling_assumption_owner", "check_no_dangling_requirement_owner", "check_no_dangling_requirement_assumptions", "check_no_dangling_requirement_relations", "check_no_dangling_conflict_refs"),
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
            assumptions=("A-text-grounded-in-models",),
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
                "Each sub-operator needs an operator-prompt scoped to its domain -- the framework-agent sees R-check-* and R-bijection-*, the finance-agent sees R-finance-*, etc. A single global CLAUDE.md would overload sub-agents with irrelevant requirements and dilute their focus. Per-agent generation enforces the bounded-context discipline (R-context-bounded-delegation) structurally. (Wave 1 seed-coherence pass: enforced_by named a bare 'test_agent_scoped_constitution' which is not a function or file -- corrected to the .py file that actually covers this claim, caught by the new check_enforced_by_resolvable invariant.)"
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_agent_scoped_constitution.py",),
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
                "The operator needs an automatic map of delegated authority -- who stewards what. Hand-maintained agent registries drift. PURPOSE (machine-readable in scope.py per R-agent-declares-purpose) + SCOPE (the filter) + atoms-count (the load) + tool counts (the capability) together give the director a one-glance view of the delegation graph without grep. (Wave 1 seed-coherence pass: enforced_by named a bare 'test_agent_map_complete' which is not a function or file -- corrected to the .py file that actually covers the AGENT-MAP block, caught by the new check_enforced_by_resolvable invariant.)"
            ),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_agent_map.py",),
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
            status="REJECTED",
            why=(
                "REJECTED — REPLACES R-claude-md-consolidates-when-single-agent. The unconditional claim 'each domains/<name>/CLAUDE.md is the domain-scoped operator-prompt' no longer holds: domains/hotam-spec-self/CLAUDE.md was deleted in the P22.C consolidation (task #101) and gen_spec.py now generates exactly one CLAUDE.md at repo root containing the domain content inline. Per-domain CLAUDE.md files return only when a second domain makes a shared root file overloaded. — (was: The CLAUDE.md is the director-operator's crystallized substrate (R-operator-crystal-is-claude-md). For a domain director this means the three-cipher pulse, top action, and debt figures must be domain-local, not framework-global. Generation by gen_spec.py prevents hand-written drift (R-drift-structurally-impossible). ENFORCED in P17 by test_domain_claude_md_has_all_5_blocks.)"
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
            status="REJECTED",
            why=(
                "REJECTED — REPLACES R-claude-md-consolidates-when-single-agent. The claim 'root CLAUDE.md shall contain only framework-level content' contradicts the current single-file design: root CLAUDE.md now deliberately embeds the active domain's CONSTITUTION, EMBEDDED-THINKING, and EMBEDDED-TOOLS content directly (task #101), not merely a DOMAIN-MAP index. Domain-purity-of-root was the correct discipline under the old two-file (root + per-domain) model; under the consolidated single-file model it is superseded by the discipline that domain content lives in clearly sentinel-delimited generated blocks within the one file, per R-claude-md-consolidates-when-single-agent. — (was: Mixing domain atoms into the framework CLAUDE.md recreates the single-file coupling that domain isolation is designed to break. The DOMAIN-MAP block is a generated index (R-domain-map-generated), not inline content; every domain-specific atom stays inside domains/<name>/. ENFORCED in P17 by test_framework_claude_md_no_domain_atoms.)"
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
            why=("Substrate-generates-operator at root level. Hand-written prose drifts; the framework's mind is assembled by gen_spec from spec/docs/thinking/* (shared DRY source) and the domain CLAUDE.md (per-domain knowledge). The root is a thin shell pointing into both. RESOLVED -- REPLACES the old hand-written prose in root CLAUDE.md (P19a). (Wave 1 seed-coherence pass: enforced_by named a bare 'test_root_claude_md_is_sentinel_only' which is not a function or file -- corrected to the .py file that actually covers this claim, caught by the new check_enforced_by_resolvable invariant.)"),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_root_claude_md_is_sentinel_only.py",),
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
            claim=("Each EntityType in the active domain's graph shall be projected as R-entity-<slug> in the domain's FRAMEWORK-INVARIANTS.md, with enforced_by listing the check_entity_* family covering it."),
            owner="framework-author",
            status="SETTLED",
            why=("Mirrors R-tool-is-its-own-requirement at the entity layer: an EntityType IS its own requirement; deleting it removes the R; changing its description changes the claim. Eliminates the prose gap between 'R about entity behavior' and 'EntityType implementing it'. Unfrozen per DECIDED C-be22cdd1 / V-unfreeze-entity-projection (2026-07-02): check_entity_type_constitution_projection (new sibling of check_entities_md_lists_all_types) now structurally verifies the projection is not silently omitted from FRAMEWORK-INVARIANTS.md, closing the honesty gap between the STRUCTURAL claim and mechanical guarantee. Dormant-but-real: 0 entity_types in the active domain today makes the check vacuously pass, not falsely enforced -- the guard is live and will fire the moment a domain declares an EntityType without projecting it. Wording precision (Wave 7 move 3a, no semantic change): claim and why now name the actual projection target, FRAMEWORK-INVARIANTS.md, not CONSTITUTION.md -- check_entity_type_constitution_projection's own docstring has always verified FRAMEWORK-INVARIANTS.md (entity-derived requirements are framework-plumbing, relocated out of CONSTITUTION.md by gen_spec.py's build_framework_invariants/_render_constitution_block split); the claim text simply had not caught up to that landed reality."),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("check_entity_type_constitution_projection",),
        ),
        Requirement(
            id="R-entity-is-declarative",
            claim=("The framework shall supply no built-in EntityType values — all entity types are declared by domains in build_graph()."),
            owner="framework-author",
            status="SETTLED",
            why=("The content-free contract (R-content-free-no-business-data) extends to the entity layer: entity types are domain knowledge, not framework knowledge. Supplying built-in types would violate the blank-kit invariant and couple the framework to a particular domain model."),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_content_free.py::test_no_domain_instances_in_tensio_src",),
        ),
        Requirement(
            id="R-entity-reuses-lifecycle",
            claim=("Each EntityType.lifecycle shall be a Lifecycle value (the §Lifecycle keystone) with no parallel state machinery introduced."),
            owner="framework-author",
            status="SETTLED",
            why=("The §Lifecycle keystone was introduced precisely so that every stateful aspect (Process, Entity, Goal, Operator) reuses one mechanism. Parallel state machinery would fracture the invariant surface and double the maintenance burden for each new aspect. Wave 1 mechanical-honesty pass (2026-07-02): promoted STRUCTURAL to ENFORCED -- the demo fixture's second EntityType (invoice, tests/fixtures/seed.py) validates through the exact same check_lifecycle_wellformed BFS machinery as the first (customer), proven directly by test_demo_fixture_has_two_entity_types_and_both_pass_all_entity_checks, which asserts check_lifecycle_wellformed(invoice_type.lifecycle) == [] using the shared §Lifecycle keystone, no parallel state machinery."),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("check_entity_type_lifecycle_wellformed", "test_demo_fixture.py::test_demo_fixture_has_two_entity_types_and_both_pass_all_entity_checks"),
        ),
        Requirement(
            id="R-entity-checks-by-iteration",
            claim=("The check_entity_* invariant family shall cover every declared EntityType by iterating g.entity_types, requiring no new check_* code per additional type."),
            owner="framework-author",
            status="SETTLED",
            why=("Parametric iteration is what makes the entity aspect scale: one invariant covers all types today and tomorrow. Per-type invariants would grow the check surface linearly and force framework edits on every domain addition — the inverse of the content-free design. Wave 1 mechanical-honesty pass (2026-07-02): promoted STRUCTURAL to ENFORCED -- the demo fixture (tests/fixtures/seed.py) now declares a SECOND EntityType (invoice, alongside customer), and test_demo_fixture_has_two_entity_types_and_both_pass_all_entity_checks runs the full check_entity_* family against both types with zero per-type code changes needed, proving the iteration claim end-to-end rather than by inspection of the source alone."),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_demo_fixture.py::test_demo_fixture_has_two_entity_types_and_both_pass_all_entity_checks",),
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
            status="REJECTED",
            why=("REJECTED — REPLACES R-claude-md-consolidates-when-single-agent. This claim described embedding a SEPARATE domains/hotam-spec-self/CLAUDE.md inside root CLAUDE.md via a DOMAIN-CRYSTAL sentinel block. That separate file no longer exists (deleted in task #101's consolidation) — there is nothing left to embed, since the domain content is generated directly into the single root CLAUDE.md rather than composed from a nested crystal. The two-file DOMAIN-CRYSTAL composition pattern returns only when a second domain is created and root CLAUDE.md can no longer hold all domains inline. — (was: Closes the sensor-substrate gap: Claude Code auto-loads root CLAUDE.md on session start; embedding the domain's CLAUDE.md (the canonical entry point of the domain, and the base from which all sub-agents derive scoped versions) means the operator boots from substrate (R-operator-prompt-from-substrate) rather than from raw weights + session memory. The substrate writes the operator's prompt physically, not aspirationally.)"),
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
            enforcement="ENFORCED",
            enforced_by=("test_project_name.py",),
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
            enforcement="STRUCTURAL",
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-claude-md-consolidates-when-single-agent",
            claim=("While a repository has exactly one domain and zero actively-spawned concurrent sub-agents, gen_spec.py shall generate exactly one CLAUDE.md file at repository root containing all operator-prompt content."),
            owner="framework-author",
            status="SETTLED",
            why=("Applies R-crystallize-before-split to file structure itself: don't split the operator-prompt into per-domain/per-agent files until a real second concurrent operator actually needs its own bounded crystal. Maintaining synchronized scaffold files that nobody reads (the deleted domains/hotam-spec-self/CLAUDE.md and agents/director/agents/framework-agent/ tree, task #101) is premature complexity. ENFORCED by test_framework_claude_md_purity.py::test_exactly_one_claude_md_in_repo, which fails the moment a second CLAUDE.md file reappears anywhere in the repo."),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_framework_claude_md_purity.py::test_exactly_one_claude_md_in_repo",),
        ),
        Requirement(
            id="R-operator-crystal-embeds-thinking",
            claim=("The operator's CLAUDE.md shall embed the full content of its scope-relevant thinking documentation inline, not as markdown links, so the operator holds the methodology itself rather than a table of contents."),
            owner="framework-author",
            status="REJECTED",
            why=("REJECTED — REPLACES by R-operator-crystal-embeds-thinking-distilled: full-text embedding contradicted R-crystal-reload-by-reference and breached the 150k host limit (CLAUDE.md reached ~200k chars); the crystal now carries a RULE+WHY distillate + Tier-3 pointer instead. — (was: A link the operator must separately fetch is a re-derivation tax on every turn; inlining the methodology content means it is present in the loaded substrate from the first token (R-operator-prompt-from-substrate). Task #98 (A1) built the EMBEDDED-THINKING block; ENFORCED by test_embedded_thinking_sentinels_present (sentinels exist) and test_embedded_thinking_contains_full_topic_content (content is the full topic text, not a link).)"),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_embedded_thinking_tools.py::test_embedded_thinking_sentinels_present", "test_embedded_thinking_tools.py::test_embedded_thinking_contains_full_topic_content",),
        ),
        Requirement(
            id="R-operator-crystal-embeds-tools",
            claim=("The operator's CLAUDE.md shall embed the full content of its scope-relevant tool documentation inline, not as markdown links."),
            owner="framework-author",
            status="REJECTED",
            why=("REJECTED — REPLACES by R-operator-crystal-embeds-tools-distilled: full-text embedding contradicted R-crystal-reload-by-reference and breached the 150k host limit (CLAUDE.md reached ~200k chars); the crystal now carries a RULE+WHY distillate + Tier-3 pointer instead. — (was: Same rationale as R-operator-crystal-embeds-thinking applied to tool docs: an operator deciding whether to invoke apply_proposal.py should not have to fetch a separate file to learn its contract. ENFORCED by test_embedded_tools_sentinels_present (sentinels exist) and test_embedded_tools_contains_full_tool_content (content is the full tool doc text, not a link); test_embedded_blocks_regen_byte_identical guards against drift between the embedded copy and the regenerated source.)"),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_embedded_thinking_tools.py::test_embedded_tools_sentinels_present", "test_embedded_thinking_tools.py::test_embedded_tools_contains_full_tool_content", "test_embedded_thinking_tools.py::test_embedded_blocks_regen_byte_identical",),
        ),
        Requirement(
            id="R-sub-agent-crystal-triad",
            claim=("Every sub-agent's CLAUDE.md shall contain three parts: scope-filtered embedded methodology thinking, a reference to its parent/senior agent, and its own scope-filtered domain business (CONSTITUTION)."),
            owner="framework-author",
            status="SETTLED",
            why=("Records the director's explicit framing for the FUTURE shape of a real second agent: a sub-agent inherits (1) base methodology thinking so it can reason like any operator, (2) a connection to its senior/parent agent so it knows where to escalate and whose scope it is bounded within, and (3) its own scope-filtered domain business so it acts only within its delegated concern. create_agent.py and gen_spec.py already implement this triad (SCOPE-filtered CONSTITUTION generation, AGENT-MAP parent linkage, embedded thinking/tools) — this rule is recorded now, while zero sub-agents are actively spawned, so agent-spawning stays disciplined later. Sibling rule R-claude-md-consolidates-when-single-agent governs WHEN this per-file triad model activates: only once N>1 concurrently-active agents exist; until then the triad content lives merged inside the single root CLAUDE.md."),
            assumptions=("A-python-stack", "A-finite-context-operators"),
            enforcement="ENFORCED",
            enforced_by=("test_agent_scoped_constitution.py", "test_tool_spawn_agent.py"),
        ),
        Requirement(
            id="R-claude-md-template-driven",
            claim=("CLAUDE.md shall be generated by substituting <!-- mind --> and <!-- business --> placeholders in CLAUDE.md.template.txt with rendered content, preserving all other template text verbatim."),
            owner="framework-author",
            status="SETTLED",
            why=("The prior sentinel-surgery approach (editing ~10 separate BEGIN/END marker pairs scattered through an existing file) made it impossible for a human to add durable plain-text notes anywhere in CLAUDE.md without risking accidental clobbering by a future block-insertion bug. The template model inverts this: exactly two named placeholders are substituted; everything else in the template -- including any hand-written notes a human adds -- survives every regeneration verbatim. Simpler mental model, same anti-drift guarantee for the two generated zones. (Wave 1 seed-coherence pass: enforced_by's second entry 'test_regen_byte_identical' is ambiguous -- two test files each define a function with that bare name -- corrected to the specific file, caught by the new check_enforced_by_resolvable invariant.)"),
            assumptions=("A-python-stack",),
            enforcement="ENFORCED",
            enforced_by=("test_hand_written_note_in_template_survives_regen", "test_claude_md_template.py::test_regen_byte_identical"),
        ),
        Requirement(
            id="R-operator-crystal-embeds-thinking-distilled",
            claim=("The operator's CLAUDE.md shall embed a compressed RULE+WHY distillation of each scope-relevant thinking topic inline (Tier 1), each with a pointer to its full text at spec/docs/thinking/<slug>.md (Tier 3), not the full body carried verbatim in working context."),
            owner="framework-author",
            status="SETTLED",
            why=("REPLACES R-operator-crystal-embeds-thinking: full-text embedding contradicted R-crystal-reload-by-reference (an operator shall reload its crystal by reference rather than re-carrying it in working context) -- it re-carried the entire verbatim thinking corpus in working context instead of referencing it -- and it breached the 150k-char host limit (root CLAUDE.md measured ~197,916 chars with all 22 spec/docs/thinking/*.md and 14 spec/docs/tools/*.md embedded verbatim). The distillate keeps the reasoning that matters (RULE + WHY, not a bare table of contents) small enough to carry for every scope-relevant topic, while the full text stays on disk and is Tier-3-referenced by path, loaded only when actually needed."),
            assumptions=("A-python-stack",),
            relations=(Relation("refines", "R-crystal-reload-by-reference"),),
            enforcement=ENFORCED,
            enforced_by=("test_embedded_thinking_tools.py::test_embedded_thinking_contains_distilled_topic_content", "test_embedded_thinking_tools.py::test_embedded_thinking_block_has_tier3_reference", "test_embedded_thinking_tools.py::test_embedded_thinking_block_is_bounded",),
        ),
        Requirement(
            id="R-operator-crystal-embeds-tools-distilled",
            claim=("The operator's CLAUDE.md shall embed a compressed RULE+WHY distillation of each scope-relevant tool's documentation inline (Tier 1), each with a pointer to its full text at spec/docs/tools/<basename>.md (Tier 3), not the full body carried verbatim in working context."),
            owner="framework-author",
            status="SETTLED",
            why=("REPLACES R-operator-crystal-embeds-tools: full-text embedding contradicted R-crystal-reload-by-reference and breached the 150k-char host limit (root CLAUDE.md measured ~197,916 chars with the full tool-doc corpus embedded verbatim). The distillate keeps the RULE + WHY reasoning for every scope-relevant tool while the full doc stays on disk, Tier-3-referenced by path."),
            assumptions=("A-python-stack",),
            relations=(Relation("refines", "R-crystal-reload-by-reference"),),
            enforcement=ENFORCED,
            enforced_by=("test_embedded_thinking_tools.py::test_embedded_tools_contains_distilled_tool_content", "test_embedded_thinking_tools.py::test_embedded_thinking_block_is_bounded",),
        ),
        Requirement(
            id="R-crystal-carries-role-seed",
            claim=("Root CLAUDE.md shall contain a generated OPERATOR-ROLE sentinel block stating the operator's scope, the guardian-of-consistency role across spec, tests, and business intent, and the single generative law of the methodology."),
            owner="framework-author",
            status="SETTLED",
            why=("Phase 2 of the crystal redesign (task #8): the operator's identity — guardian of spec↔tests↔business consistency under one generative law (everything important-yet-invisible becomes a typed anchored node under a steward; tension is held, never quietly extinguished) — must be the FIRST resident content of the crystal. Generated (not hand-written template prose) to keep R-root-claude-md-is-sentinel-only true in claim, not only in test; parameterized by the active domain and SETTLED-atom count so the same seed narrows for future sub-operators (R-sub-agent-crystal-triad)."),
            assumptions=("A-bootstrap-self-applies",),
            relations=(Relation("refines", "R-crystal-is-claude-md"),),
            enforcement=ENFORCED,
            enforced_by=("test_operator_seed.py::test_role_block_states_scope_and_law",),
        ),
        Requirement(
            id="R-crystal-carries-mediation-loop",
            claim=("Root CLAUDE.md shall contain a generated MEDIATION-LOOP sentinel block rendering the six-step input-processing loop — ORIENT, LOCATE, CONFRONT, TRANSLATE, PRESENT, LAND — each step naming its real tool command."),
            owner="framework-author",
            status="SETTLED",
            why=("The loop is the operator's operating procedure for ANY input: it binds the role (guardian, hypothesis-checker) to tools that already exist — what_now.py (ORIENT), the Constitution index and generated docs (LOCATE), RECENTLY-REJECTED anti-relitigation scan (CONFRONT), Proposed* JSON (TRANSLATE), steward decision (PRESENT, R-ai-presents-not-decides), apply_proposal → gen_spec → pytest → closure (LAND, R-verify-closure-per-action). Naming real commands keeps the loop executable rather than aspirational; a pass that writes nothing is a valid conclusion."),
            assumptions=("A-bootstrap-self-applies",),
            relations=(Relation("refines", "R-agent-never-lost"), Relation("supports", "R-ai-presents-not-decides"),),
            enforcement=ENFORCED,
            enforced_by=("test_operator_seed.py::test_mediation_loop_names_real_tools",),
        ),
        Requirement(
            id="R-crystal-carries-recursion-seed",
            claim=("Root CLAUDE.md shall contain a generated OPERATOR-RECURSION sentinel block describing sub-operator spawning as this same seed narrowed to a sub-scope, naming the create_agent → gen_spec → spawn_agent path."),
            owner="framework-author",
            status="SETTLED",
            why=("Recursion is a CAPABILITY of the sole operator, not a set of materialized files: while R-claude-md-consolidates-when-single-agent holds (one domain, zero active sub-agents), agent crystals must not exist, so the crystal carries the description of HOW to spawn — the same Role/Loop seed with a narrower scope filter (R-sub-agent-crystal-triad) — plus the real machinery path (create_agent.py, gen_spec.py, spawn_agent.py with --stamp, spawn-log per R-task-spawn-log-runtime) and the conclusions-only return contract (R-delegation-conclusions-only)."),
            assumptions=("A-finite-context-operators",),
            relations=(Relation("refines", "R-context-bounded-delegation"), Relation("supports", "R-sub-agent-crystal-triad"),),
            enforcement=ENFORCED,
            enforced_by=("test_operator_seed.py::test_recursion_block_names_spawn_path",),
        ),
        Requirement(
            id="R-constitution-is-index",
            claim=("The CONSTITUTION block in root CLAUDE.md shall render each SETTLED requirement as a one-line index entry — id, claim truncated to at most 96 characters, single-character enforcement flag — with a block-level pointer to the full roster in the domain's docs/gen/REQUIREMENTS.md."),
            owner="framework-author",
            status="SETTLED",
            why=("The crystal is a seed plus indexes, not a catalog (R-crystal-reload-by-reference): full claims already live in the generated roster (docs/gen/REQUIREMENTS.md) and enforcement detail in docs/gen/UNENFORCED.md, so carrying them verbatim doubled the block (~38k chars of the 85k crystal). The index keeps every SETTLED anchor plus enough claim text resident for the CONFRONT scan while cutting ~16k chars, and is orthogonal to Phase 3: relocating framework-plumbing atoms changes WHICH ids are listed, not the line format. R-operator-prompt-from-substrate stays true — all SETTLED requirements, grouped by category, generated deterministically."),
            assumptions=("A-finite-context-operators", "A-compaction-loses-working",),
            relations=(Relation("refines", "R-operator-prompt-from-substrate"), Relation("refines", "R-crystal-reload-by-reference"),),
            enforcement=ENFORCED,
            enforced_by=("test_constitution.py::test_constitution_is_index", "test_constitution.py::test_constitution_lists_all_settled",),
        ),
        Requirement(
            id="R-constitution-separates-plumbing",
            claim=("The CONSTITUTION index in root CLAUDE.md shall render only business and discipline atoms, relocating framework-plumbing atoms to a generated docs/gen/FRAMEWORK-INVARIANTS.md named by an in-block pointer, with the partition total equal to all SETTLED atoms."),
            owner="framework-author",
            status="SETTLED",
            why=("hotam-spec-self is the framework modeling itself, so a majority of its SETTLED requirements are internal guarantees of the framework's own machinery (Entity/Agent/Domain/Process/Operator-internals/Lifecycle-keystone/Generator/bijection/anchor mechanics/CLAUDE.md machinery) rather than business claims the operator mediates as reality. Phase 3 (task #9) relocates those atoms out of the resident CONSTITUTION index into a generated FRAMEWORK-INVARIANTS.md, reachable by pointer, so the operator's resident index reflects what it actually mediates. This is presentational only: no atom's status changes and no atom is dropped from the full REQUIREMENTS.md roster (kept, not deleted, mirroring R-rejected-preserved-not-deleted's anti-loss discipline)."),
            enforcement=ENFORCED,
            enforced_by=("test_constitution.py::test_constitution_partitions_all_settled", "test_constitution.py::test_constitution_pointer_to_framework_invariants", "test_constitution.py::test_framework_invariants_md_up_to_date",),
        ),
        Requirement(
            id="R-speculative-aspects-frozen",
            claim=("The Entity aspect, multi-domain federation, and sub-agent recursion machinery shall receive no inward development while frozen, unfreezing only when a real business domain demonstrates concrete need."),
            owner="framework-author",
            status="SETTLED",
            why=("Built ahead of demand -- 0 entity_types/entities, exactly 1 domain, 0 active sub-agents against 12+10+8 atoms of supporting machinery: classic speculative generality (96% of that surface inert). Frozen by steward 2026-07-02 after audit. Code/tests/atoms are PRESERVED (in the spirit of R-rejected-preserved-not-deleted), relocated into docs/gen/FRAMEWORK-INVARIANTS.md under R-constitution-separates-plumbing. Unfreeze trigger: Phase 5 (a real business domain). Note: the natural home for this freeze is C-8600b1b8 (core-vs-aspect, ACKNOWLEDGED, revisit_marker already reads 'REVISIT when a second opt-in behavioral aspect (Entity or Task) is proposed'). This conflict is now addressable by ConflictTransition proposals -- R-conflict-addressing-resolves-variables (landed 2026-07-02) taught tools/apply_proposal.py's _find_conflict_call to resolve axis/context kwargs bound through simple string-variable assignments (not just literals), which is how all six Conflict nodes in this graph bind axis/context (c1_axis..c6_axis / c1_ctx..c6_ctx). The freeze itself remains open pending steward decision on when to formally move C-8600b1b8 to DECIDED; do not confuse 'now addressable' with 'already resolved.' Wave 1 mechanical-honesty pass (2026-07-02): promoted STRUCTURAL to ENFORCED -- a sha256 hash-baseline test (tests/frozen_aspects_baseline.json + test_frozen_aspects_snapshot.py) now makes an inward edit to any frozen file (src/hotam_spec/entity.py; tools/create_agent.py, spawn_agent.py, invoke_agent.py; tools/create_domain.py) fail RED, with the failure message stating explicitly that unfreezing requires a recorded steward act (regenerating the baseline). Prose discipline alone could not detect a silent violation; the hash guard can."),
            enforcement="ENFORCED",
            enforced_by=("test_frozen_aspects_snapshot.py::test_frozen_aspect_files_unchanged_since_baseline", "test_frozen_aspects_snapshot.py::test_baseline_covers_all_three_named_frozen_surfaces"),
        ),
        Requirement(
            id="R-reflection-predicates-first-class",
            claim=("The operator's self-diagnosis conditions shall be named predicate functions in hotam_spec.reflection imported by the what_now harness, never inlined in tool code."),
            owner="framework-author",
            status="SETTLED",
            why=("Concept-Map gap closed: §Reflection was the one core concept with no home in src/ — the five P0 self-diagnosis conditions (draft-overhang, unenforced-settled, over-budget-operators, dead-assumption-on-enforcer, derived-but-unbuilt) lived inline in tools/what_now.py:diagnose(), important-yet-invisible substrate (generative law). Extraction mirrors the §Invariants pattern: reflect_* predicates return Finding records (condition/target/imperative) and the harness composes hotam_spec.reflection.all_findings() exactly as it composes invariants.all_violations() — the substrate owns the diagnosis vocabulary, the tool only renders. Behavior-preserving: what_now CLI output byte-identical before/after extraction."),
            relations=(Relation("supports", "R-agent-never-lost"),),
            enforcement=ENFORCED,
            enforced_by=("test_reflection.py::test_what_now_sources_reflection_predicates_from_module", "test_reflection.py::test_diagnose_p0_equals_reflection_findings",),
        ),
        Requirement(
            id="R-conflict-addressing-resolves-variables",
            claim=("tools/apply_proposal.py shall resolve a Conflict's axis and context kwargs through simple string-variable assignments as well as string literals when locating the Conflict node a proposal addresses."),
            owner="framework-author",
            status="SETTLED",
            why=("The C-8600b1b8 lesson: all six Conflict nodes in domains/hotam-spec-self/graph.py bind axis/context to local variables (c1_axis..c6_axis / c1_ctx..c6_ctx), so _find_conflict_call's literal-only matching made every existing Conflict unaddressable and the standing P3 CONFLICT_STALLED action mechanically unlandable. Fixed by folding module/function-level `name = \"literal\"` assignments (_collect_string_assignments) with ambiguous rebindings dropped conservatively; literal support unchanged. This repairs the ACT half of the mediation loop for conflict transitions (R-active-loop-apply-tool)."),
            enforcement=ENFORCED,
            enforced_by=("test_apply_proposal.py::test_find_conflict_call_resolves_variable_bound_kwargs", "test_apply_proposal.py::test_real_domain_conflict_c8600b1b8_is_addressable", "test_apply_proposal.py::test_all_real_domain_conflicts_are_addressable",),
        ),
        Requirement(
            id="R-proposed-conflict-kind-exists",
            claim=("The proposal protocol shall include a ProposedConflict kind (kind='Conflict') that materializes a new Conflict node in the active domain's graph via tools/apply_proposal.py."),
            owner="framework-author",
            status="SETTLED",
            why=("Closes recorded spec-tool drift: C-186c4347's DECIDED rationale already promised 'ProposedConflict' while only transitions existed, leaving surfaced tensions without a mechanical creation path (the loop could move conflicts but never materialize one). The writer computes id via conflict_identity(axis, context), never caller-supplied (R-stable-conflict-identity); requires the axis to already exist in the graph's axes tuple (R-axis-controlled-vocab; admitting a new axis is a separate act); requires >= 2 distinct existing members (R-conflict-min-two-members); refuses a steward who owns any member (R-steward-distinct-from-owners); lifecycle always starts DETECTED. Extends R-active-loop-protocol's floor exactly as EntityType and OperatorBudget did."),
            enforcement=ENFORCED,
            enforced_by=("test_apply_proposal.py::test_apply_conflict_creates_detected_node", "test_apply_proposal.py::test_apply_conflict_written_graph_loads_and_passes_shape", "test_proposal.py::test_proposed_conflict_target_anchor_is_computed_identity",),
        ),
        Requirement(
            id="R-latent-connectors-cluster-by-assumption",
            claim=("The what_now harness shall render latent-connector suspects as one P5 action per shared-assumption cluster rather than one action per requirement pair."),
            owner="framework-author",
            status="SETTLED",
            why=("Verified noise shape: 22 P5 lines were 21 pairs sharing A-most-knowledge-crystallizable (functional ref-count 7, just under GENERIC_ASSUMPTION_THRESHOLD=8 because the 8th referencing requirement is REJECTED) plus 1 genuinely distinct candidate (the A-content-free-honest pair) drowned by them. Clustering by the pairs' specific-shared-assumption signature (graph.latent_connector_clusters) renders the review surface at the size of the decision space while keeping every pair visible (LatentCluster.pairs; TENSIONS.md table). A threshold shift was rejected: it only moves the noise cliff (see test_latent_connector_noise_fix.py history)."),
            enforcement=ENFORCED,
            enforced_by=("test_latent_connector_noise_fix.py::test_what_now_p5_one_line_per_cluster", "test_latent_connector_noise_fix.py::test_clusters_partition_suspects", "test_latent_connector_noise_fix.py::test_real_graph_p5_count_equals_cluster_count",),
        ),
        Requirement(
            id="R-presented-pending-decision-type",
            claim=("The presented-awaiting-decision state shall live in a dedicated folder (spec/.runtime/proposals/pending/ vs applied/), surfaced by the harness -- not as a new graph node type."),
            owner="framework-reviewer",
            status="SETTLED",
            why=("Steward verdict 2026-07-02 (verbatim): «да, нужно. наверно такие вещи нужно вести в отдельной папке» (English: 'yes, it's needed. probably such things should be tracked in a separate folder'). Landed: spec/.runtime/proposals/ now splits into pending/ (proposal awaiting steward verdict) and applied/ (landed proposal, moved there by tools/apply_proposal.py's apply() on a successful land -- write+regen+verify-tier green, closure advanced when --triggering-kind supplied). Backward compat preserved: files directly under proposals/ (the historical flat layout predating this split) are still treated as pending by pending_proposal_files(). tools/what_now.py gains a P6 PENDING_PROPOSAL band (pending_proposal_actions()) that lists each pending file with its age in days -- CLI-only (main()), deliberately NOT wired into diagnose(g) because diagnose() is pure-over-the-graph and gen_spec.py's generated docs must stay byte-stable (R-deterministic-generation); a filesystem-mtime-derived 'N days' figure would break that. No new graph node type was added -- Presented/Pending stays tooling ephemera in .runtime/ (gitignored), never crystallized substrate."),
            enforcement="ENFORCED",
            enforced_by=("tests/test_pending_proposal_archive.py::test_pending_sees_flat_layout_files", "tests/test_pending_proposal_archive.py::test_pending_sees_pending_subfolder", "tests/test_pending_proposal_archive.py::test_pending_excludes_applied_subfolder", "tests/test_pending_proposal_archive.py::test_apply_success_archives_proposal_file", "tests/test_pending_proposal_archive.py::test_apply_failure_does_not_archive", "tests/test_pending_proposal_archive.py::test_pending_proposal_actions_one_per_file", "tests/test_pending_proposal_archive.py::test_pending_proposal_actions_age_in_days", "tests/test_pending_proposal_archive.py::test_diagnose_never_includes_pending_band"),
        ),
        Requirement(
            id="R-land-gate-tier-selector",
            claim=("The per-proposal LAND verify step (tools/apply_proposal.py) shall, by default, run the T1 targeted-enforcer pytest subset resolved by tools/gate.py from the proposal's target enforced_by tuple (plus a small always-run determinism+smoke baseline), instead of the full pytest suite."),
            owner="framework-author",
            status="SETTLED",
            why=("The full pytest suite paid a per-proposal LAND tax of roughly 100-300s dominated by Windows subprocess-spawn overhead (measured baseline: 614 tests / 298.76s wall; 63% of that mechanically eliminated by an unrelated in-process --help fix, not this requirement). Most proposals touch exactly one Requirement or Conflict whose enforced_by tuple already names the exact enforcer(s) that guard it -- the graph IS the test-impact map. Running only that targeted subset removes the redundant re-verification of unrelated tests on every single LAND without weakening what R-verify-closure-per-action actually checks for a targeted change, because R-land-gate-tier-selector-fails-closed guarantees the selector never narrows below what a full run would have exercised for THIS target."),
            assumptions=("A-python-stack",),
            relations=(Relation("refines", "R-verify-closure-per-action"),),
            enforcement=ENFORCED,
            enforced_by=("test_tool_gate.py", "test_apply_proposal_gate_wiring.py::test_apply_uses_t1_targeted_selection_when_gate_confident",),
        ),
        Requirement(
            id="R-land-gate-tier-selector-fails-closed",
            claim=("The LAND tiering decision (tools/apply_proposal.py::apply, consulting tools/gate.py's T1 selector) shall fall back to the T2 full suite on ANY selection uncertainty or blast-radius-unbounded case -- an unknown/new target, an empty enforced_by tuple, a Conflict target (no per-instance enforced_by), any single enforced_by entry gate.py cannot resolve to a concrete pytest node-id, a ProposedRejection (a rejected atom's own enforced_by does not bound its removal blast radius), or the creation of a brand-new Requirement or Conflict node (a new node's enforced_by, if any, is steward-supplied and unverified) -- never returning a partial or best-effort subset in an uncertain or unbounded case."),
            owner="framework-author",
            status="SETTLED",
            why=("A tier selector that silently narrows verification under uncertainty would weaken R-verify-closure-per-action exactly where it matters most -- new nodes, rejections, and framework-code changes are precisely the changes least covered by an existing, trustworthy enforced_by tuple. Fail-closed-to-full-suite on ANY uncertainty or unbounded-blast-radius case (not merely gate.py's own resolution failures) keeps the honesty property end to end: T1 only ever runs when the FULL apply() decision -- not just gate.py's internal mapping -- can name the SAME enforcers a full run would have exercised for this specific, pre-existing target. Widened 2026-07-02 (honesty wave) after review found apply() let Rejections and new-node creations reach gate.select_tier1 even though neither case's target enforced_by bounds the real blast radius (a rejected atom's blast radius is its dependents, not itself; a brand-new node has no verified enforced_by at all)."),
            assumptions=("A-python-stack",),
            relations=(Relation("refines", "R-land-gate-tier-selector"),),
            enforcement="ENFORCED",
            enforced_by=("test_tool_gate.py::test_fails_closed_on_unresolvable_entry", "test_tool_gate.py::test_fails_closed_on_partially_unresolvable_entries", "test_tool_gate.py::test_fails_closed_on_empty_enforced_by", "test_tool_gate.py::test_fails_closed_on_unknown_target", "test_tool_gate.py::test_fails_closed_on_conflict_target", "test_apply_proposal_gate_wiring.py::test_apply_falls_back_to_full_suite_when_gate_uncertain", "test_apply_proposal_gate_wiring.py::test_apply_rejection_always_uses_full_suite", "test_apply_proposal_gate_wiring.py::test_apply_new_requirement_creation_always_uses_full_suite"),
        ),
        Requirement(
            id="R-tiered-gate-not-a-commit-gate",
            claim=("The full pytest suite (T2) shall remain the mandatory verification gate at wave and commit boundaries -- the T1 targeted-enforcer tier applies only to the per-proposal LAND step inside apply_proposal.py and is never substituted for the full-suite run a steward or wave-closing agent performs before committing."),
            owner="framework-author",
            status="SETTLED",
            why=("Tiering exists to relieve the redundant per-proposal tax, not to weaken the wave/commit-boundary guarantee that the WHOLE graph is still structurally sound after a batch of changes -- a targeted T1 pass on proposal N does not prove proposal N did not regress something proposal N-3 touched. This is inherently a discipline/procedural claim about WHEN each tier is invoked (spec/CLAUDE.md's Mediation loop step 6 LAND, vs. a separate steward-run `uv run pytest -q` at commit time) rather than a graph-checkable structural property, so it is marked STRUCTURAL rather than claiming a machine enforcer that cannot honestly exist for a human-invoked boundary. Requalified INHERENTLY_PROSE 2026-07-02 (honesty wave): the claim's own why already argues no machine check_* can honestly enforce a human-invoked commit-time boundary, yet enforceability defaulted to ENFORCEABLE, so it counted as closeable debt in UNENFORCED.md/reflect_unenforced_settled -- a self-contradiction between the claim's stated nature and its declared kind (R-enforceability-kind-declared). Wave 8 move 2 atomicity pass: the claim's own semicolon was flagged COMPOUND by audit_atomicity.py -- inspection shows the clause after the semicolon is NOT a second independent obligation, it is a scope/boundary clause of the SAME rule (defining exactly how far the T1 tier is allowed to reach, i.e. never further than LAND, never a substitute for T2). Reworded the semicolon to the codebase's existing '--' scope-disclaimer convention (see R-commit-boundary-checkable) and extended audit_atomicity._audit_claim with a _CLAIM_SCOPE_CLAUSE exemption mirroring the invariant side's existing 'N sub-rules' self-declaration exemption -- same false-positive class, same fix shape. No semantic change to the promise; this is a wording/classifier-alignment fix, not a split (R-requirement-claim-is-atomic governs live obligations, and this claim always asserted exactly one)."),
            assumptions=("A-python-stack",),
            relations=(Relation("refines", "R-verify-closure-per-action"),),
            enforcement="STRUCTURAL",
            enforced_by=("tools/apply_proposal.py::apply", "tools/gate.py"),
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-doc-names-reader",
            claim=("Every generated doc shall name its reader as an existing Stakeholder id in its header."),
            owner="framework-author",
            status="SETTLED",
            why=("Stage E: a generated doc with no named reader is an important-yet-invisible artifact -- nobody is accountable for reading it, so drift between the doc and its audience's actual needs goes unnoticed (the generative law applied to doc plumbing, not just graph nodes). hotam_spec.doc_readers.DOC_READER_ROLES maps each doc kind to a portable ROLE hint (operator / domain-steward / framework-maintainer); gen_spec.py resolves that hint against an EXPLICIT role -> Stakeholder.id binding the active domain declares in its own manifest.py (the DOC_READERS dict, read via hotam_spec.graph.active_domain_doc_readers()) and stamps a `reader: <id>` line into every generated doc's header (docs/gen/*.md, spec/docs/{thinking,tools}/*.md, docs/methodology/atoms/*.md). Resolution NEVER scans stakeholder ids for a role-shaped substring (R-doc-readers-declared-not-guessed) -- a stakeholder id that happens to contain a role word (e.g. a 'travel-agent' stakeholder in some future business domain) can no longer be silently captured as the reader of operator-facing docs; only a binding the domain author wrote down on purpose counts. check_doc_reader_resolves_to_stakeholder guards the mapping itself: a declared binding whose id is absent from the graph's Stakeholders, or a role with no binding at all, fires a Violation, mirroring the dangling-ref family applied to doc plumbing. Aspect-gated (no-op) for domains that have declared no DOC_READERS binding at all, mirroring the Process/Goal/Entity aspect-gating precedent."),
            relations=(Relation("supports", "R-anchor-everything"), Relation("supports", "R-drift-structurally-impossible"),),
            enforcement="ENFORCED",
            enforced_by=("check_doc_reader_resolves_to_stakeholder", "test_invariants.py::test_check_doc_reader_resolves_to_stakeholder_registered", "test_invariants.py::test_check_doc_reader_green_when_all_roles_resolve", "test_invariants.py::test_check_doc_reader_fires_on_partial_adoption", "test_invariants.py::test_check_doc_reader_fires_on_dangling_bound_id", "test_invariants.py::test_check_doc_reader_travel_agent_regression", "test_docs_gen.py::test_generated_docs_carry_reader_header"),
        ),
        Requirement(
            id="R-initiator-supplies-domain-content",
            claim=("An agent shall receive its domain content from its initiator at boot and crystallize it into the domain code-spec."),
            owner="domain-user",
            status="SETTLED",
            why=("Resolves C-8600b1b8 (core-vs-aspect: R-content-free-framework vs R-agent-never-lost, shared assumption A-prose-suffices). Steward decision (domain-user, 2026-07-02), verbatim: «Агент должен получать от инициатора контент о своей области и должен его кристаллизовать в код-спеке». The framework itself stays content-free (R-content-free-framework unbroken -- it ships no business data) AND an agent dropped into a domain is never lost (R-agent-never-lost unbroken) because the INITIATOR (human steward or calling process) supplies domain content at boot time, and the agent's job is to crystallize that supplied content into the domain's code-spec (graph.py) via the existing proposal pipeline, not to invent it nor to find it absent. This is narrower than R-crystallize-knowledge-to-code (which covers crystallizing an operator's own working knowledge in general, source-agnostic); this requirement pins down the SOURCE of domain content specifically as the initiator-at-boot, closing the core-vs-aspect tension. REQUALIFIED 2026-07-02 (Wave 2 honesty pass): previously carried default enforceability=ENFORCEABLE, which listed it as closeable debt in UNENFORCED.md -- but no check_* can ever verify 'the initiator supplied content at boot' as a runtime event (it is a fact about a conversation/process outside the graph's own reach, not a property of the committed graph state). This is the same honesty class as R-ai-presents-not-decides and R-two-altitude-ontology: a real, permanent discipline that is INHERENTLY_PROSE, not ENFORCEABLE-but-unbuilt. Requalifying (not fabricating an enforcer) keeps the burn-down meter honest -- the debt this requirement represented was never real closeable debt in the first place."),
            assumptions=("A-text-grounded-in-models",),
            enforcement="STRUCTURAL",
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-enforced-by-resolvable",
            claim=("Every SETTLED/ENFORCED requirement's enforced_by entries shall resolve to a concrete pytest node-id via the shared enforcer-resolution algorithm."),
            owner="framework-author",
            status="SETTLED",
            why=("Wave 1 (seed coherence audit): check_enforced_names_invariant only checks that enforced_by is NON-EMPTY -- a real typo in an entry (a renamed test file, a stale check_* name) passes that check silently and is only discovered when someone runs tools/gate.py by hand and it fails closed to the full suite. That is an honest but SILENT tax on the T1 tiered gate (R-verify-closure-per-action): the graph claims a targeted enforcer exists when it does not, and the steward never sees this as debt. check_enforced_by_resolvable makes the debt visible: it greps tests/*.py to build the check_* -> test-file map (the same map gen_spec.py's CONCEPT-MAP block and tools/gate.py's T1 selector both build) and resolves every enforced_by entry against it, firing a Violation naming the exact unresolvable string. The resolution algorithm itself was extracted out of tools/gate.py into spec/src/hotam_spec/enforcer_resolution.py so this invariant (which lives in src/ and must not import from tools/) and gate.py (a tool) share ONE implementation rather than two hand-synced copies (R-prefer-tool-over-hand). Filesystem-grep-based, in-process, no subprocess spawn -- measured well under a second on this repo's ~180+ test files; not yet cached because the suite has not grown to a size where that matters."),
            relations=(Relation("supports", "R-verify-closure-per-action"), Relation("supports", "R-enforcement-first-class"),),
            enforcement=ENFORCED,
            enforced_by=("check_enforced_by_resolvable", "test_invariants.py::test_enforced_by_resolvable_registered", "test_invariants.py::test_enforced_by_typo_fires", "test_invariants.py::test_enforced_by_unknown_check_name_fires", "test_invariants.py::test_enforced_by_real_names_pass", "test_invariants.py::test_enforced_by_resolvable_green_on_seed",),
        ),
        Requirement(
            id="R-land-tier-trace",
            claim=("Every applied proposal that reaches the LAND verify step shall append its verification tier (T1 targeted or T2 full-suite), selected pytest node-ids (or the literal 'full'), and pytest/closure outcome to spec/.runtime/land-log.jsonl, written AFTER the verify step so the record states what actually ran."),
            owner="framework-author",
            status="SETTLED",
            why=("R-land-gate-tier-selector introduced the T1/T2 tiered LAND gate but left its own operation invisible -- there was no way to answer, after the fact, which tier a given land actually used. Mirrors R-task-spawn-log-runtime's spawn-log.jsonl precedent (same .runtime/ directory, same append-only JSONL discipline, same gitignored-not-committed-substrate status): a runtime trace, not generated docs, because its truth value depends on wall-clock events, not on the graph. Making the log write happen strictly AFTER the verify step (not before) is the load-bearing property -- a record must describe what was actually verified, never a plan that could still fail. RE-ATOMIZED Wave 8 move 2 (2026-07-03): audit_atomicity.py flagged the prior claim COMPOUND (2 semicolons, 3 segments) -- inspection confirmed 3 genuinely independent behavioral rules of the land-log mechanism, each with its own dedicated enforcer tests: (1) record shape+timing (kept here), (2) dry-run proposals never write a record (now R-land-tier-trace-skips-dry-run), (3) a broken/unwritable log location is best-effort/warn-only, never failing an otherwise-green apply (now R-land-tier-trace-best-effort). Split three-ways per R-requirement-claim-is-atomic; this atom now carries only the record-shape+timing promise, with enforced_by trimmed to the tests that verify exactly that."),
            assumptions=("A-python-stack",),
            relations=(Relation("refines", "R-land-gate-tier-selector"),),
            enforcement="ENFORCED",
            enforced_by=("test_apply_proposal_land_log.py::test_land_log_record_shape_t1", "test_apply_proposal_land_log.py::test_land_log_record_shape_t2", "test_apply_proposal_land_log.py::test_land_log_records_closure_exit", "test_apply_proposal_land_log.py::test_land_log_records_closure_exit_2_on_not_advanced"),
        ),
        Requirement(
            id="R-commit-boundary-checkable",
            claim=("tools/gate_status.py shall answer, from spec/.runtime/land-log.jsonl, whether a full T2 verification has landed at-or-after the most recent T1-gated land, exiting 0 (boundary satisfied) or 1 (boundary not satisfied, printing the unverified T1-gated targets) -- this is the mechanically checkable SLICE of R-tiered-gate-not-a-commit-gate's claim; it does not itself verify that a steward runs it, nor detect an imminent commit, nor replace R-tiered-gate-not-a-commit-gate's human-invoked procedural discipline."),
            owner="framework-author",
            status="SETTLED",
            why=("R-tiered-gate-not-a-commit-gate is honestly INHERENTLY_PROSE: no check_* can force a human to run the full suite before `git commit`. But the trace introduced by R-land-tier-trace makes ONE part of that claim mechanically answerable after the fact -- whether the log shows a covering T2 run. Splitting this into its own atom (rather than flipping R-tiered-gate-not-a-commit-gate to ENFORCED) keeps both claims honest: the parent claim stays INHERENTLY_PROSE because it genuinely cannot be machine-verified end to end (the human-invocation half is unreachable by any test); this new atom claims only the reachable half, with real enforced_by tests -- avoiding the exact self-contradiction (ENFORCEABLE default vs. an admittedly-unenforceable claim) the 2026-07-02 honesty wave requalified the parent to fix."),
            assumptions=("A-python-stack",),
            relations=(Relation("refines", "R-tiered-gate-not-a-commit-gate"), Relation("refines", "R-land-tier-trace"),),
            enforcement=ENFORCED,
            enforced_by=("test_tool_gate_status.py::test_t1_then_t2_is_satisfied", "test_tool_gate_status.py::test_t2_then_t1_is_not_satisfied", "test_tool_gate_status.py::test_only_t1_records_never_t2_is_not_satisfied", "test_tool_gate_status.py::test_only_t2_records_is_satisfied", "test_tool_gate_status.py::test_mixed_only_t1_after_last_t2_are_unverified", "test_tool_gate_status.py::test_cli_exit_0_on_satisfied", "test_tool_gate_status.py::test_cli_exit_1_on_not_satisfied",),
        ),
        Requirement(
            id="R-unmeasured-cipher-names-user-action",
            claim=("While the context cipher is UNMEASURED, the generated LIVE-STATE shall name the exact user-run command that activates measurement."),
            owner="ai-agent",
            status="SETTLED",
            why=("R-ai-presents-not-decides means the operator cannot self-install a hook that touches the user's GLOBAL ~/.claude config -- that is a steward decision (spec/tools/setup_context_hook.py --patch-global --apply). Leaving the UNMEASURED cipher as a bare status word made the gap invisible: the operator knew context was unmeasured but never told the user what to run to fix it, so the gap persisted turn after turn with no forcing function. tools/context.py's render_line() now embeds the exact command in the UNMEASURED line itself, and gen_spec's LIVE-STATE + emit_cipher's pulse both surface it verbatim (R-boot-cite-in-first-sentence then forces the operator to cite it every turn until the user runs the command and the cipher measures for real)."),
            assumptions=("A-finite-context-operators",),
            enforcement=ENFORCED,
            enforced_by=("test_tool_context.py::test_absent_stamp_reads_unmeasured", "test_tool_context.py::test_stamp_without_pct_renders_unmeasured_line",),
        ),
        Requirement(
            id="R-trust-anchor-delegation-explicit-only",
            claim=("Delegation of the steward's personal-signature duty to an agent shall be valid ONLY when granted EXPLICITLY -- per-case or for a declared campaign in advance -- never implied or standing by default, recorded durably in domains/<name>/delegations.jsonl and resolvable from any Conflict lifecycle marker that cites it."),
            owner="framework-author",
            status="SETTLED",
            why=("M5, REPLACES split into R-trust-anchor-mechanism + R-trust-anchor-delegation-explicit-only per atomicity discipline (R-requirement-claim-is-atomic). Steward verdict 2026-07-02 (verbatim): «человек обязан сам подписать, если только явно не делегирует это в каждом случае или на кампанию вперед» (English: 'the human is obliged to sign personally, unless he explicitly delegates it, either per-case or for a campaign in advance'). This atom lands the EXCEPTION half: delegation is not the default and is not implicit from e.g. an agent being given repo write access -- it requires an explicit steward grant, scoped either to one case or to a named campaign declared in advance. The standing 'delegation grant' object now exists: domains/hotam-spec-self/delegations.jsonl -- a durable, COMMITTED (not .runtime/) append-only JSONL ledger of {id, steward, verbatim, date, scope} records, each one a trust-anchor signature next to the graph it authorizes. tools/record_delegation.py appends new records (auto-incrementing DEL-<n>, refusing an unknown steward or empty verbatim/scope). The seed record DEL-1 (steward domain-user, date 2026-07-02, scope 'campaign: session pipeline decisions (C-be22cdd1 HELD + DECIDED)') anchors the verbatim delegation that authorized the core-vs-aspect Conflict's DECIDED transition. ENFORCED by spec/tests/test_delegation_marker_honesty.py: every Conflict.lifecycle text carrying the literal marker 'per explicit campaign delegation <date>' must resolve to a delegations.jsonl record dated the same day -- an unresolved marker (a date naming no ledger entry) is a caught violation, never a silent pass. spec/tests/test_tool_record_delegation.py covers the writer: auto-increment (including gap-tolerant), unknown-steward refusal, empty verbatim/scope refusal, default-date fill-in."),
            assumptions=("A-stakeholders-care", "A-bootstrap-self-applies"),
            enforcement="ENFORCED",
            enforced_by=("tests/test_delegation_marker_honesty.py::test_active_domain_delegation_markers_resolve", "tests/test_delegation_marker_honesty.py::test_del_1_specifically_resolves_the_core_vs_aspect_conflict", "tests/test_delegation_marker_honesty.py::test_unresolved_marker_is_detected", "tests/test_tool_record_delegation.py::test_first_record_gets_del_1", "tests/test_tool_record_delegation.py::test_second_record_increments_to_del_2", "tests/test_tool_record_delegation.py::test_increment_survives_gaps", "tests/test_tool_record_delegation.py::test_unknown_steward_refused", "tests/test_tool_record_delegation.py::test_empty_verbatim_refused", "tests/test_tool_record_delegation.py::test_empty_scope_refused", "tests/test_tool_record_delegation.py::test_seed_delegation_del_1_exists"),
        ),
        Requirement(
            id="R-unresolvable-conflict-carries-variants",
            claim=("A Conflict unresolvable by amending its member requirements shall carry at least two elaborated behavior variants; the operator presents the variants, the steward chooses."),
            owner="framework-reviewer",
            status="REJECTED",
            why=("REJECTED -- REPLACES split+answered by R-conflict-held-state + R-held-carries-variants + R-variant-choice-is-decision + R-unresolvable-classified-by-human (Wave 3, decided by framework-reviewer 2026-07-02). Both OPEN design questions are now answered: (1) attachment shape -- Variant is a payload field on Conflict.variants (frozen dataclass, anti-RDF, NOT a new graph node), landed via R-held-carries-variants; (2) when a conflict is classified unresolvable-by-members -- this is a human judgment recorded via the HELD lifecycle's decided_by signoff lock (check_held_has_decided_by family), never an AI inference, landed via R-unresolvable-classified-by-human. The original claim's three concerns (variants attach to Conflict; operator presents; steward chooses) are now separately atomized: R-held-carries-variants (the payload shape), R-conflict-held-state (the lifecycle state that carries it), and R-variant-choice-is-decision (the steward's choice moving HELD to DECIDED). REJECTED rather than promoted to SETTLED because the original claim's own OPEN(question) is a compound design-question node whose answer is now four separate, atomic, honestly-graded requirements -- keeping the original as a live SETTLED atom would duplicate their claims (R-requirement-claim-is-atomic). — (was: Steward-approved draft, his idea verbatim 2026-07-02: «ось превращается в сущность, если невозможно разрешить противоречие через изменения в конфликтующих сторонах. Возможно нужно порождать варианты поведения -- т.е не один вариант, а два у каждой сущности. Т.к главное чтобы модель смогла это увидеть, а решать уже с пользователем» (English: 'the axis turns into an entity if the contradiction cannot be resolved through changes in the conflicting parties. Perhaps it is necessary to generate behavior variants -- i.e. not one variant, but two for each entity. Because the main thing is that the model be able to SEE this, and the deciding is then done with the user'). Design not yet done -- recorded as the candidate-missing-capability per R-uncrystallizable-is-missing-type: no code, no new type until the design questions in the OPEN status are answered (attachment shape on Conflict; the unresolvable-by-members classification test).)"),
            enforcement=PROSE,
        ),
        Requirement(
            id="R-no-observation-type",
            claim=("hotam_spec shall define no Observation or Evidence class anywhere in its package -- Assumption remains the ontology's sole belief-carrying node type."),
            owner="framework-reviewer",
            status="SETTLED",
            why=("Wave 1 mechanical-honesty pass (2026-07-02): R-observation-evidence-scope (STRUCTURAL, M21 steward verdict) makes a broader claim -- operator epistemics live in the working dialogue, crystallized only on request -- that is not fully machine-checkable (no static scan can verify 'lives in the dialogue'). But its NEGATIVE half -- no separate Observation/Evidence type exists -- IS mechanically checkable: a static AST scan of spec/src/hotam_spec/*.py for a class named Observation or Evidence. Split out as its own atom (R-commit-boundary-checkable's slice-of-a-broader-claim pattern) so the mechanical slice can honestly reach ENFORCED while the parent claim (R-observation-evidence-scope) stays STRUCTURAL, undiluted by a partial enforcer."),
            assumptions=("A-most-knowledge-crystallizable",),
            relations=(Relation("refines", "R-observation-evidence-scope"),),
            enforcement=ENFORCED,
            enforced_by=("test_no_observation_type_scope.py::test_no_observation_or_evidence_class_defined_anywhere_in_hotam_spec", "test_no_observation_type_scope.py::test_assumption_is_the_only_belief_carrying_dataclass_by_convention",),
        ),
        Requirement(
            id="R-core-imports-stdlib-or-hotam-spec-only",
            claim=("Every top-level import in spec/src/hotam_spec/*.py shall resolve to the Python standard library or hotam_spec itself -- no third-party backend/runtime dependency."),
            owner="framework-author",
            status="SETTLED",
            why=("Wave 1 mechanical-honesty pass (2026-07-02): R-backend-scope (PROSE, M37 steward verdict) makes the broader design-stance claim that the core stays backend-neutral 'by construction' and names no target backends -- a claim that is largely a prose non-decision (declining to build an OperatorBackend protocol), not itself machine-checkable end-to-end. But the CONSTRUCTION half of 'by construction' IS mechanically checkable: an AST import scan proving spec/src/hotam_spec/*.py imports nothing beyond stdlib + itself, so no backend dependency has silently crept into the core. Split out as its own atom (same slice pattern as R-no-observation-type / R-commit-boundary-checkable) so this narrow, mechanical guarantee can honestly reach ENFORCED while R-backend-scope itself remains PROSE."),
            assumptions=("A-finite-context-operators",),
            relations=(Relation("refines", "R-backend-scope"),),
            enforcement=ENFORCED,
            enforced_by=("test_backend_neutral_scope.py::test_hotam_spec_core_imports_stdlib_or_self_only",),
        ),
        Requirement(
            id="R-agent-code-imports-framework",
            claim=("An agent's code shall import the framework body (hotam_spec.*) as shared infrastructure, and hotam_spec.* itself shall never import back from any agent's private tools/ directory."),
            owner="framework-author",
            status="SETTLED",
            why=("First half of the split R-agent-imports-framework (R-requirement-claim-is-atomic). Mechanically checkable: a static AST scan (mirroring test_backend_neutral_scope.py's R-core-imports-stdlib-or-hotam-spec-only pattern) verifies the dependency arrow points one way only -- hotam_spec.* and spec/tools/*.py never import a module that lives under any agent's private tools/ dir. Today no agent has private tools yet (only the director stub exists with no tools/ dir), so the scan is vacuously green; it fires the moment a real agent with private tools is spawned and the direction is ever reversed. Promoted DRAFT->SETTLED with a real enforcer landing in the same wave (test_agent_import_direction.py), not left as claimed-but-unguaranteed debt. REPOINTED 2026-07-02 (Wave 7 move 4, P5 latent-connector cluster fix): assumptions moved from A-content-free-honest (an over-broad, unrelated content-freeness assumption shared by coincidence with two other unrelated requirements, producing a false 3-way latent-connector cluster) to A-agent-code-imports-framework-directionally, which names this requirement's actual premise."),
            assumptions=("A-agent-code-imports-framework-directionally",),
            enforcement="ENFORCED",
            enforced_by=("test_agent_import_direction.py::test_framework_body_never_imports_from_an_agent_tools_dir", "test_agent_import_direction.py::test_shared_tools_never_import_from_an_agent_tools_dir"),
        ),
        Requirement(
            id="R-task-spawn-is-a-hand",
            claim=("A task-agent invocation (a sh/Agent-tool call) is a hand -- a one-shot delegated act, not a standing sub-operator."),
            owner="ai-agent",
            status="SETTLED",
            why=("First half of the split R-task-spawn-is-ephemeral (R-requirement-claim-is-atomic). The user's distinction (hands vs agents): a hand executes one task and reports back, distinct from a domain-delegation sub-operator (R-context-bounded-delegation) which owns a persistent sub-domain. This is a naming/classification discipline -- no check_* can verify 'this call was conceptually a hand', so it stays STRUCTURAL, carried by the spawn-log's own shape (one entry per invocation, no operator-id field implying persistence) rather than a dedicated enforcer."),
            assumptions=("A-finite-context-operators",),
            enforcement=STRUCTURAL,
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-atomicity-ratchet-no-growth",
            claim=("The set of requirement claims and check_* invariants flagged COMPOUND by tools/audit_atomicity.py's classification functions shall never grow beyond the frozen baseline recorded in spec/tests/atomicity_compound_baseline.json."),
            owner="framework-reviewer",
            status="SETTLED",
            why=("R-requirement-claim-is-atomic and R-check-method-is-atomic are both honestly STRUCTURAL -- 'decompose into separate requirements/functions' is human judgment no check_* can force wholesale, and burning down the 27 pre-existing compound atoms (21 requirements + 6 check_*) in one wave is out of scope. But the DIRECTION of the debt (is it growing or merely being carried) IS mechanically checkable via a ratchet: test_atomicity_ratchet.py re-derives the live COMPOUND set from audit_atomicity's own _audit_claim/_audit_invariant functions and fails RED the instant a COMPOUND id appears that is not already in the frozen baseline. This is the R-commit-boundary-checkable slice-of-a-broader-claim pattern applied to atomicity: the parents stay STRUCTURAL (the full claim -- eventually zero compound atoms -- is not machine-verifiable end to end), while this narrower atom (no NEW compound debt) has a real, always-green-until-violated enforcer landing in the same wave."),
            assumptions=("A-text-grounded-in-models",),
            relations=(Relation("refines", "R-requirement-claim-is-atomic"), Relation("refines", "R-check-method-is-atomic"),),
            enforcement="ENFORCED",
            enforced_by=("test_atomicity_ratchet.py::test_no_new_compound_requirements_beyond_baseline", "test_atomicity_ratchet.py::test_no_new_compound_invariants_beyond_baseline"),
        ),
        Requirement(
            id="R-framework-owned-by-no-agent",
            claim=("The framework body (`hotam_spec.*`) shall be owned by no single agent -- it is shared infrastructure any agent's code may import."),
            owner="framework-author",
            status="SETTLED",
            why=("Second half of the split R-agent-imports-framework (R-requirement-claim-is-atomic). This is a non-code ownership/governance stance about the framework body -- distinct from the mechanically-checkable import-direction fact already split out as R-agent-code-imports-framework (ENFORCED via test_agent_import_direction.py). No check_* can verify 'owned by no single agent' -- ownership is a governance convention (no per-agent CODEOWNERS-style claim exists anywhere in the repo, and none should), so this stays honestly STRUCTURAL/INHERENTLY_PROSE, carried by the fact that hotam_spec/ has exactly one owner field (framework-author, the framework's own steward) and no agent directory declares or claims exclusive ownership of any hotam_spec module. Landed as the missing REPLACES target flagged in the Wave 2 review (a REJECTED atom pointed at this id before it existed). REPOINTED 2026-07-02 (Wave 7 move 4, P5 latent-connector cluster fix): assumptions moved from A-content-free-honest (an over-broad, unrelated content-freeness assumption shared by coincidence with two other unrelated requirements, producing a false 3-way latent-connector cluster) to A-framework-shared-infra-no-owner, which names this requirement's actual premise."),
            assumptions=("A-framework-shared-infra-no-owner",),
            enforcement="STRUCTURAL",
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-conflict-held-state",
            claim=("Conflict.lifecycle shall admit a HELD(reason) state, entered only via a human-signed ConflictTransition, for tensions not resolvable by amending the member requirements."),
            owner="framework-reviewer",
            status="SETTLED",
            why=("Closes the design question in R-unresolvable-conflict-carries-variants (steward verdict, verbatim 2026-07-02): the axis 'turns into an entity if the contradiction cannot be resolved through changes in the conflicting parties'. HELD is that resting point, added to CONFLICT_LIFECYCLE as a QUIESCENT state (mirrors DECIDED/REVISIT_WHEN's shape exactly -- prefix-parsed, cyclic back to DETECTED on condition-fires) rather than a new node type -- the conflict itself does not change identity, only its lifecycle value. Entry requires the same signoff lock as DECIDED (check_held_has_decided_by, R-decided-needs-human-signoff applied to HELD) so an AI cannot silently classify a tension as unresolvable-by-members."),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("check_conflict_lifecycle_in_lifecycle", "check_canonical_lifecycles_wellformed",),
        ),
        Requirement(
            id="R-held-carries-variants",
            claim=("A HELD Conflict shall carry at least two elaborated behavior Variants (id, behavior, implies, costs) as a payload field, not as new graph nodes."),
            owner="framework-reviewer",
            status="SETTLED",
            why=("Answers the attachment-shape half of R-unresolvable-conflict-carries-variants's open question ('payload field vs derived Proposed* pair'): Variant is a frozen dataclass living in Conflict.variants (payload), never a graph.py node -- anti-RDF, mirrors why axis/context/shared_assumption are Conflict fields and not their own node types. check_held_has_min_two_variants enforces len(set(variant ids)) >= 2 whenever lifecycle starts with HELD; check_typed_anchors_variant enforces the V- prefix on every Variant id (R-anchor-everything extended to the new payload type) so a variant can be cited by reference (R-speak-by-reference) exactly like any other typed anchor."),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("check_held_has_min_two_variants", "check_typed_anchors_variant", "check_held_has_nonempty_decided_by", "check_held_by_is_known_stakeholder", "check_held_by_not_member_owner",),
        ),
        Requirement(
            id="R-variant-choice-is-decision",
            claim=("A derived Requirement shall be spawned from a HELD Conflict only after the steward's ConflictTransition names the chosen Variant, moving the conflict from HELD to DECIDED."),
            owner="framework-reviewer",
            status="SETTLED",
            why=("The HELD to DECIDED transition (steward-choose-variant, guard: 'rationale names the chosen variant') is where R-decided-needs-human-signoff and R-ai-presents-not-decides meet the variant machinery: the AI presents N elaborated variants, but ONLY the steward's DECIDED rationale selects which one becomes real (a derived= requirement). This is honestly STRUCTURAL, not fully mechanically checkable -- no check_* can verify the DECIDED rationale prose actually names one of the HELD conflict's variant ids (that is a semantic match, not a structural one); check_decided_has_rationale_or_derived enforces only that SOME rationale or derived requirement exists, the narrower claim that it names the chosen variant is carried by prose discipline and steward review, same honesty pattern as R-task-spawn-is-a-hand."),
            assumptions=("A-stakeholders-care",),
            enforcement=STRUCTURAL,
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-unresolvable-classified-by-human",
            claim=("Classifying a Conflict as unresolvable-by-amending-its-members shall be a human judgment, never an automated AI inference."),
            owner="framework-reviewer",
            status="SETTLED",
            why=("Answers the second open design question in R-unresolvable-conflict-carries-variants ('when is a conflict classified unresolvable-by-members?'): there is no proposed heuristic or detector for this -- it is the steward's call, made structural exactly the way R-uncrystallizable-automated makes 'missing type' detection a recorded human judgment rather than an algorithm. The AI's role is limited to PRESENTING elaborated variants once the steward has already decided the tension will not yield to amending R-a or R-b (R-ai-presents-not-decides); the ConflictTransition to HELD requires decided_by (check_held_has_decided_by) precisely so this classification act itself is never silent or AI-authored."),
            assumptions=("A-stakeholders-care",),
            enforcement=STRUCTURAL,
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-scope-is-projection",
            claim=("An operator's sub-domain shall be a computed PROJECTION (an id-set view derived by prefix match over the shared TensionGraph), never a copy of any node."),
            owner="framework-author",
            status="SETTLED",
            why=("M18/P-scope. Resolves R-partition-vs-border's open question by rejecting both extremes: a strict partition forbids any shared object (loses the ability to model deliberate cross-cutting delegation); an ad-hoc declared-border overlap invites hand-maintained edge lists that drift from the graph. hotam_spec.scope_projection.project_scope(g, prefixes) computes a ScopeView (sorted id-tuples for Requirements/Conflicts/Assumptions/Axes) purely from the graph, on demand, with the exact prefix-match discipline tools/gen_spec.py already uses for per-agent CONSTITUTION digests (id.startswith(p) for any p in prefixes) -- so the projection can never fork from the single writer (R-no-hand-edit-graph). Evidence: spec/src/hotam_spec/scope_projection.py:ScopeView/project_scope; Operator.scope field (hotam_spec/operator.py) carries the declaring prefix tuple."),
            assumptions=("A-finite-context-operators",),
            enforcement="ENFORCED",
            enforced_by=("test_scope_projection.py::test_project_scope_selects_by_prefix", "test_scope_projection.py::test_scope_view_matches_gen_spec_prefix_rule_directly"),
        ),
        Requirement(
            id="R-scope-overlap-generated",
            claim=("When two operators' scope projections share a node or axis, the overlap shall be computed and printed into each affected operator's crystal, never hidden or silently merged."),
            owner="framework-author",
            status="SETTLED",
            why=("M18/P-scope. hotam_spec.scope_projection.scope_overlap(view_a, view_b) computes the sorted set-intersection of two ScopeViews; tools/gen_spec.py::_regenerate_agent_constitutions renders one OVERLAP block per agent (via _render_overlap_block) against every OTHER discovered agent's SCOPE, written between OVERLAP:BEGIN/END sentinels immediately after CONSTITUTION:END. With the CURRENT meta-domain (one OP-director, SCOPE=()), the OVERLAP block is legitimately empty on every agent -- R-empty-content-wellformed's calm-empty discipline, not suppression: the sentinel pair is always present so the state ('no scope overlap') is visible rather than omitted. Evidence: spec/src/hotam_spec/scope_projection.py:scope_overlap/overlap_node_ids; spec/tools/gen_spec.py:_render_overlap_block/_regenerate_agent_constitutions (OVERLAP sentinels)."),
            assumptions=("A-finite-context-operators",),
            enforcement=ENFORCED,
            enforced_by=("test_scope_projection.py::test_scope_overlap_finds_shared_conflict_and_assumption", "test_scope_projection.py::test_scope_overlap_disjoint_scopes_is_empty",),
        ),
        Requirement(
            id="R-overlap-single-presenter",
            claim=("Every node contested by two or more operators' overlapping scope projections shall resolve to exactly one deterministic presenter."),
            owner="framework-author",
            status="SETTLED",
            why=("M18/P-scope. hotam_spec.scope_projection.presenter_for_node(node_id, operator_ids) returns the LEXICOGRAPHICALLY FIRST operator id among the operators contesting a node -- a stable, total, deterministic rule that does not depend on graph declaration order (which shifts under unrelated source reformatting) or on an arbitrary parent-hierarchy walk. invariants.check_scoped_node_has_single_presenter walks every pair of operators with a non-empty `scope`, computes their overlap via scope_projection, and would fire a Violation only if presenter_for_node ever returned None for a non-empty contesting set -- which cannot happen, making single-presentership PROVABLE rather than merely asserted. Evidence: spec/src/hotam_spec/scope_projection.py:presenter_for_node; spec/src/hotam_spec/invariants.py:check_scoped_node_has_single_presenter (registered in ALL_INVARIANTS + RULES_AS_DATA_TABLE as BESPOKE)."),
            assumptions=("A-finite-context-operators",),
            enforcement=ENFORCED,
            enforced_by=("check_scoped_node_has_single_presenter", "test_scope_projection.py::test_two_operators_overlapping_scope_resolves_to_one_presenter",),
        ),
        Requirement(
            id="R-spawn-log-carries-isolation",
            claim=("Every spawn-log.jsonl entry written by tools/spawn_agent.py shall carry isolation (worktree|shared) and mutating (bool) fields, defaulting to shared/false when the caller omits --isolation/--mutating."),
            owner="ai-agent",
            status="SETTLED",
            why=("Wave 5 (measurable slices of discipline): the spawn-log already recorded WHO/WHAT/WHEN (R-task-spawn-log-runtime) but not the isolation posture under which a sub-agent ran, so a parallel-mutating-agent hazard had no trace at all. spawn_agent.py's freeze under R-speculative-aspects-frozen was PARTIALLY lifted by explicit steward act (GO given for Wave 5) to add these two additive CLI flags (--isolation, --mutating) and log fields, backward-compatible with every pre-existing caller. This is the writer half; the honest reader half (R-parallel-mutating-agents-use-worktree) is a separate atom because it can only check log-internal consistency, not runtime concurrency."),
            assumptions=("A-finite-context-operators",),
            enforcement=ENFORCED,
            enforced_by=("test_tool_spawn_agent.py::test_spawn_log_carries_isolation_and_mutating_fields",),
        ),
        Requirement(
            id="R-boot-cite-measured",
            claim=("A Stop hook shall lexically check whether the first sentence of the operator's last reply in the transcript contains a typed anchor (R-/C-/A-/OP-/GOAL-/section-sign), logging the result to spec/.runtime/boot-cite-log.jsonl, checked as a form-level (not substance-level) signal."),
            owner="ai-agent",
            status="SETTLED",
            why=("R-boot-cite-in-first-sentence (PROSE) has never had any mechanical trace of compliance. tools/boot_cite_status.py's writer half reads the Stop hook's transcript_path payload, extracts the last assistant text block's first sentence, and lexically tests it for an anchor token; the reader half (compute_boot_cite_status) answers what fraction of the last N logged replies complied. HONESTY BOUNDARY, explicit in the tool docstring: this measures the citation RITUAL (a token-shaped string appears), never the citation's TRUTH (that the anchor is relevant or that graph reality was actually confronted) -- R-boot-cite-in-first-sentence itself stays PROSE/STRUCTURAL; this atom only claims the measurable slice exists and is tested."),
            assumptions=("A-finite-context-operators",),
            enforcement=ENFORCED,
            enforced_by=("test_tool_boot_cite_status.py::test_write_from_payload_cited_true", "test_tool_boot_cite_status.py::test_compute_status_mixed_and_windowed",),
        ),
        Requirement(
            id="R-domain-self-hosting-flag",
            claim=("Framework-jurisdiction invariants (FRAMEWORK_SCOPED_INVARIANTS) shall run only against a graph whose manifest declares SELF_HOSTING=True."),
            owner="framework-author",
            status="SETTLED",
            why=("Wave 7 docrystallization of the gap named by the Wave 6/prior agent's landed bijection work: TensionGraph.self_hosting (graph.py) is populated at load time from the sibling manifest.py's SELF_HOSTING constant (default False); invariants.all_violations gates FRAMEWORK_SCOPED_INVARIANTS (the 12 functions covering domain/agent/tool-plumbing structural checks that only make sense against the framework's OWN self-model, e.g. filesystem walks over spec/tools/, spec/agents/) on this flag so they fire ONLY when g.self_hosting is True -- domains/hotam-spec-self/manifest.py sets SELF_HOSTING = True (it IS the framework modeling itself); any other business domain (e.g. domains/hotam-dev/) defaults to SELF_HOSTING = False and is correctly exempt from framework-internal plumbing checks that have no bearing on business content. Without this gate, framework-jurisdiction checks would fire as phantom P1 violations against business domains that never claim to model the framework itself. Evidence: spec/src/hotam_spec/graph.py TensionGraph.self_hosting field + _manifest_self_hosting() loader; spec/src/hotam_spec/invariants.py FRAMEWORK_SCOPED_INVARIANTS set + all_violations gate (`if check.__name__ in framework_scoped and not g.self_hosting`); domains/hotam-spec-self/manifest.py SELF_HOSTING = True."),
            assumptions=("A-bootstrap-self-applies",),
            enforcement=ENFORCED,
            enforced_by=("test_framework_scoped_invariants_skipped_when_not_self_hosting", "test_framework_scoped_invariants_run_when_self_hosting", "test_hotam_dev_pulse_has_no_framework_scoped_violations", "test_hotam_spec_self_pulse_unchanged_by_self_hosting_gate", "test_synthetic_non_self_domain_with_framework_checks_not_flagged",),
        ),
        Requirement(
            id="R-land-tier-trace-skips-dry-run",
            claim=("A dry-run proposal shall never write a spec/.runtime/land-log.jsonl record."),
            owner="framework-author",
            status="SETTLED",
            why=("Split from R-land-tier-trace (Wave 8 move 2 atomicity pass, 2026-07-03) -- a distinct behavioral rule of the same land-log mechanism: a --dry-run apply never reaches a real verify step, so a log entry would misleadingly claim a tier ran when nothing was actually landed. This is the exemption half of the trace contract, independently enforced by test_dry_run_writes_no_log."),
            assumptions=("A-python-stack",),
            relations=(Relation("refines", "R-land-tier-trace"),),
            enforcement=ENFORCED,
            enforced_by=("test_apply_proposal_land_log.py::test_dry_run_writes_no_log",),
        ),
        Requirement(
            id="R-land-tier-trace-best-effort",
            claim=("A broken or unwritable spec/.runtime/land-log.jsonl location shall never fail an otherwise-green apply -- the write is best-effort, warn only."),
            owner="framework-author",
            status="SETTLED",
            why=("Split from R-land-tier-trace (Wave 8 move 2 atomicity pass, 2026-07-03) -- a distinct behavioral rule of the same land-log mechanism: the trace is diagnostic, not load-bearing for correctness, so a filesystem problem writing the log must never turn a successful proposal apply into a failure. Independently enforced by test_land_log_write_failure_is_best_effort."),
            assumptions=("A-python-stack",),
            relations=(Relation("refines", "R-land-tier-trace"),),
            enforcement=ENFORCED,
            enforced_by=("test_apply_proposal_land_log.py::test_land_log_write_failure_is_best_effort",),
        ),
        Requirement(
            id="R-domain-map-shows-pulse",
            claim=("The root CLAUDE.md DOMAIN-MAP block shall carry, for every domain, an 'open actions' line stating that domain's open-action count and top action, and emit_cipher shall surface the aggregate open-action count of all non-pinned domains in the injected pulse."),
            owner="framework-author",
            status="SETTLED",
            why=("Before this atom the root LIVE-STATE cipher diagnosed only the PINNED self-host domain, so a business domain's DETECTED conflict (e.g. hotam-dev C-ec1ec532 on axis speed-vs-verification) was INVISIBLE from the root crystal and the banner 'every contradiction is visible' was false at the altitude of the whole repo. gen_spec._domain_pulse runs what_now.diagnose per domain into the DOMAIN-MAP (the first eye); emit_cipher._other_domains_open sums the non-pinned domains' open actions into the pulse (the second eye). Both are generated, so they cannot drift from real graph state (R-deterministic-generation). Refines R-domain-map-generated."),
            assumptions=("A-python-stack",),
            enforcement=ENFORCED,
            enforced_by=("test_framework_claude_md_purity.py::test_domain_map_shows_pulse_per_domain", "test_framework_claude_md_purity.py::test_emit_cipher_aggregates_other_domain_open_actions",),
        ),
        Requirement(
            id="R-tension-audit-shortlist-tool",
            claim=("A tool tools/audit_tensions.py shall emit a deterministic, LLM-free shortlist of SETTLED requirement pairs that might hide an unmediated tension."),
            owner="framework-author",
            status="SETTLED",
            why=("The framework held tensions well but historically did not FIND them (0 of 8 conflicts machine-surfaced over its whole history; latent-mining covered 0 pairs; 3 axes never fired). confront.py checks ONE candidate claim against reality; this tool closes the missing first-act-of-sight half of the mediation loop by sweeping the WHOLE settled graph. Each surfaced pair is tagged with the signal that found it (POLE axis-pole pull / MODAL prohibition-vs-obligation / NOUN cross-owner overlap) and a suggested axis, so a human can review the shortlist. Deterministic and stdlib-only so the shortlist is reproducible and gate-safe."),
            enforcement="ENFORCED",
            enforced_by=("tests/test_tool_audit_tensions.py::test_pole_tension_is_found", "tests/test_tool_audit_tensions.py::test_modal_opposition_is_found", "tests/test_tool_audit_tensions.py::test_empty_graph_is_vacuous", "tests/test_tool_audit_tensions.py::test_two_runs_identical_candidates", "tests/test_tool_audit_tensions.py::test_mediated_pair_is_excluded", "tests/test_tool_audit_tensions.py::test_refine_siblings_excluded"),
        ),
        Requirement(
            id="R-tension-audit-presents-only",
            claim=("tools/audit_tensions.py shall never mutate any domain graph.py: its only outputs are a printed shortlist and an append-only run stamp, and every surfaced pair is a SUSPECT for AI/steward review, never a decided conflict."),
            owner="framework-author",
            status="SETTLED",
            why=("The generative-audit tool performs the CONFRONT-adjacent first act of sight, but sight is not decision (R-ai-presents-not-decides). If the sweep could write a Conflict node it would be deciding a tension exists rather than presenting a candidate; the steward must draft any ProposedConflict from a row. Keeping the tool read-only over the graph preserves the present-never-decide boundary at the generation organ."),
            relations=(Relation("depends_on", "R-tension-audit-shortlist-tool"),),
            enforcement=ENFORCED,
            enforced_by=("tests/test_tool_audit_tensions.py::test_audit_never_writes_graph_py",),
        ),
        Requirement(
            id="R-tension-audit-staleness-visible",
            claim=("The what_now harness shall surface a CLI-only action on the 'generative-audit' meter when the tension audit has never run or the live SETTLED graph has grown by more than GENERATIVE_AUDIT_STALE_DELTA atoms since the last recorded sweep."),
            owner="framework-author",
            status="SETTLED",
            why=("The first act of sight lapses if left to memory as the graph grows. The staleness signal reads the settled_count integer from the last append-only tension-audit.jsonl stamp -- a filesystem read. It is kept CLI-only (like the P6 PENDING_PROPOSAL band), NOT part of diagnose(g): diagnose is rendered by gen_spec into the byte-stable LIVE-STATE (R-deterministic-generation) and is exercised by every graph-generic test on synthetic graphs, so coupling a filesystem read into it would both break determinism and pollute unrelated tests. Surfacing staleness only at the interactive CLI makes 'you have not swept lately' an addressable action without corrupting the pure graph diagnosis."),
            relations=(Relation("depends_on", "R-tension-audit-shortlist-tool"),),
            enforcement=ENFORCED,
            enforced_by=("tests/test_what_now.py::test_staleness_never_run_fires", "tests/test_what_now.py::test_staleness_fresh_is_silent", "tests/test_what_now.py::test_staleness_after_growth_fires", "tests/test_what_now.py::test_staleness_never_enters_diagnose",),
        ),
        Requirement(
            id="R-revisit-markers-evaluated",
            claim=("The what_now harness shall surface a CLI-only action for each DECIDED conflict whose revisit_marker has never been evaluated or was last evaluated more than the staleness delta of SETTLED atoms ago."),
            owner="framework-author",
            status="SETTLED",
            why=("A DECIDED conflict's revisit_marker names the CONDITION under which the decision should be re-opened (the historian's anti-relitigation trigger), but nothing tracked whether anyone LOOKED at it again -- an unread trigger lets the decision silently ossify while its stated revisit condition may already have come true. tools/mark_revisit_evaluated.py appends a {stamp, conflict, settled_count} record to spec/.runtime/revisit-eval.jsonl when the steward evaluates a marker; the harness reads the last evaluation per conflict and re-surfaces the marker as 'evaluate revisit marker' when never evaluated or stale by more than GENERATIVE_AUDIT_STALE_DELTA SETTLED atoms. The band is CLI-only (never in diagnose(g)) for the same determinism reason as the pending-proposal and generative-audit bands: it reads the filesystem, which gen_spec must not render. Evaluating a marker is an OBSERVATION; re-opening the conflict is a separate ProposedConflict the steward drafts (R-ai-presents-not-decides)."),
            enforcement=ENFORCED,
            enforced_by=("tests/test_tool_mark_revisit_evaluated.py::test_append_evaluation_writes_record", "tests/test_tool_mark_revisit_evaluated.py::test_revisit_band_fires_when_never_evaluated", "tests/test_tool_mark_revisit_evaluated.py::test_revisit_band_silent_after_evaluation", "tests/test_tool_mark_revisit_evaluated.py::test_revisit_band_refires_after_growth", "tests/test_tool_mark_revisit_evaluated.py::test_revisit_band_never_enters_diagnose",),
        ),
        Requirement(
            id="R-assumption-transition-kind-exists",
            claim=("The proposal protocol shall include a ProposedAssumptionTransition kind (kind='AssumptionTransition') that changes an existing Assumption's status (HOLDS/UNCERTAIN/DEAD) in place via tools/apply_proposal.py, appending the reason to its statement and never deleting the node."),
            owner="framework-author",
            status="SETTLED",
            why=("Assumption drift is the declared root of the methodology (§Assumption — 'the root of context drift'), yet before this atom the graph had no kill-path: ProposedAssumption was add-only and the _graph_guard hand-edit lock blocked any status change, so a DEAD assumption's cluster-wide fallout (requirements_on_assumption / what_now P2 DRIFT_FALLOUT) could never actually fire — 0 status transitions in the whole project history. The writer (_apply_assumption_transition) UPDATES the existing Assumption(...) call's status= field to the bare constant and APPENDS '— [STATUS] reason' to its statement so the falsification trail survives (mirror of R-rejected-preserved-not-deleted). Signoff asymmetry, decided honestly against thinking/assumption.md: DEAD (kills a premise, cluster-wide fallout) and HOLDS (re-affirms, SILENCES the review signal) both REQUIRE a decided_by Stakeholder — the same altitude as closing a Conflict (R-decided-needs-human-signoff, R-ai-presents-not-decides); UNCERTAIN merely RAISES a question (adds a P4 review signal, removes none), which is exactly the operator's PRESENT step, so the agent may present it alone. The transition fails closed to the T2 full suite because an assumption carries no enforced_by to bound its fallout."),
            relations=(Relation("supports", "R-conflict-is-connector-node"),),
            enforcement=ENFORCED,
            enforced_by=("test_assumption_transition.py::test_validate_uncertain_needs_no_signoff", "test_assumption_transition.py::test_validate_dead_and_holds_require_signoff", "test_assumption_transition.py::test_holds_uncertain_dead_cycle", "test_assumption_transition.py::test_transition_missing_assumption_is_refused", "test_assumption_transition.py::test_dead_transition_surfaces_drift_fallout",),
        ),
        Requirement(
            id="R-machine-check-syntactic",
            claim=("Every non-empty Assumption.machine_check shall be a well-formed Python expression (compilable in eval mode), never free prose."),
            owner="framework-author",
            status="SETTLED",
            why=("The machine_check field was carried by 2 of 12 assumptions ('python.version >= (3, 12)', 'len(graph.requirements) + len(graph.conflicts) < 10_000') and had NEVER been checked in any way — pure invisible substrate. check_assumption_machine_checks_syntactic compiles each non-empty machine_check in eval mode; a SyntaxError is a Violation. The honesty boundary is deliberate and documented (R-uncrystallizable-automated): it does NOT execute or assert-true the formula, because the two real formulas evaluate against DIFFERENT, not-yet-materialized namespaces ('python' is not a defined object as written; 'graph' expects a binding), and §Assumption states machine_check is 'carried but not run' (spec-stack layers 4/5 deferred). What is guaranteed structurally without inventing semantics is that the recorded formula is a compilable expression — the seam the deferred Z3/Hypothesis layers attach to — rather than prose masquerading as machine_check. Promoting to real execution is a separate later atom once a domain supplies the evaluation namespace."),
            relations=(Relation("supports", "R-stale-substrate"),),
            enforcement=ENFORCED,
            enforced_by=("check_assumption_machine_checks_syntactic", "test_assumption_machine_check.py::test_registered_in_all_invariants", "test_assumption_machine_check.py::test_compilable_formula_passes", "test_assumption_machine_check.py::test_python_version_formula_passes", "test_assumption_machine_check.py::test_empty_machine_check_is_skipped", "test_assumption_machine_check.py::test_prose_formula_fires_violation", "test_assumption_machine_check.py::test_real_domain_machine_checks_are_syntactic",),
        ),
        Requirement(
            id="R-uncertain-assumptions-surface",
            claim=("The what_now harness shall surface every UNCERTAIN assumption carrying at least UNCERTAIN_AGING_MIN_DEPENDENTS dependent requirements as one P4 OPEN_ITEM action asking the steward to resolve the doubt."),
            owner="framework-author",
            status="SETTLED",
            why=("Blind spot found in the fxx assumption-life audit (2026-07-03): DEAD lights up P2 DRIFT_FALLOUT and HOLDS is calm, but an UNCERTAIN assumption aged invisibly — nowhere in the harness — even though the three real UNCERTAIN assumptions carry 58 / 37 / 9 dependent requirements (A-bootstrap-self-applies is the single largest silent question in the graph). uncertain_assumptions(g) is a pure status filter (peer of dead_assumptions); the K=UNCERTAIN_AGING_MIN_DEPENDENTS=5 threshold (chosen against the real graph so all three high-fan-out questions clear it and low-fan-out doubt stays quiet) gates one P4 action per aged assumption, pointing the steward at the AssumptionTransition kill-path (DEAD) or re-affirmation (HOLDS). Graph-only and deterministic, so it lives in what_now.diagnose(g) exactly like the P2 DEAD-fallout band, not as a CLI-only filesystem band."),
            relations=(Relation("supports", "R-agent-never-lost"),),
            enforcement=ENFORCED,
            enforced_by=("test_uncertain_aging.py::test_uncertain_assumptions_filter", "test_uncertain_aging.py::test_below_threshold_is_silent", "test_uncertain_aging.py::test_at_threshold_surfaces_one_p4_action", "test_uncertain_aging.py::test_holds_assumption_never_ages", "test_uncertain_aging.py::test_real_graph_surfaces_three_uncertain_assumptions",),
        ),
        Requirement(
            id="R-proposed-stakeholder-kind-exists",
            claim=("The proposal protocol shall include a ProposedStakeholder kind (kind='Stakeholder') that materializes a new Stakeholder node in the active domain's stakeholders tuple via tools/apply_proposal.py, refusing a duplicate id."),
            owner="framework-author",
            status="SETTLED",
            why=("Wave 13 (the stranger's door). A newcomer who clones the repo for their own business domain is locked out at the first Conflict: check_steward_not_a_member_owner requires a steward who is NOT the owner of any member Requirement, i.e. at least two distinct Stakeholders must exist before any tension can be held. Yet every Requirement, Axis and Assumption already had a Proposed* door while Stakeholder did not, trapping the newcomer between R-no-hand-edit-graph (the graph is writable only through apply_proposal) and the absence of a door. ProposedStakeholder(id, name, domain) is that missing door: a frozen dataclass in proposal.py with kind='Stakeholder', a _validate_stakeholder parser (id/name/domain all required), and an _apply_stakeholder_to_source writer that appends a Stakeholder(...) into the stakeholders = (...) tuple, ensuring the import, and refusing a duplicate id as a re-declaration. Models the exact ProposedAxis/ProposedAssumption pattern (append-to-tuple, exact-id-uniqueness); extends R-active-loop-protocol's floor exactly as EntityType, OperatorBudget, Axis and Assumption did."),
            enforcement=ENFORCED,
            enforced_by=("test_tool_apply_proposal_stakeholder.py::test_validate_proposal_dispatches_stakeholder_kind", "test_tool_apply_proposal_stakeholder.py::test_apply_stakeholder_appends_new_node", "test_tool_apply_proposal_stakeholder.py::test_apply_stakeholder_refuses_duplicate_id",),
        ),
        Requirement(
            id="R-sensorium-committed",
            claim=("The universal sensorium hooks (SessionStart/PostCompact gen_spec, UserPromptSubmit emit_cipher+claude_md_diff_watch, PreToolUse graph-guard, Stop context_producer+boot_cite) shall live in a committed project settings.json generated by tools/setup_hooks.py with commands portable via $CLAUDE_PROJECT_DIR, never only in the personal git-ignored settings.local.json."),
            owner="framework-author",
            status="SETTLED",
            why=("Wave 13 (the stranger's door C). The framework's 'agent is never lost' pulse (R-agent-never-lost) and its hand-edit guard (R-no-hand-edit-graph) are enforced entirely through Claude Code hooks — yet those hooks lived ONLY in the operator's personal, git-ignored .claude/settings.local.json, hardcoded to one machine's absolute path (D:/dev/HotamSpec). A stranger who clones the repo for their own business domain therefore inherits NO sensorium: no SessionStart/PostCompact regen, no UserPromptSubmit cipher, and — most dangerously — no PreToolUse _graph_guard, so R-no-hand-edit-graph becomes structurally invisible for them. tools/setup_hooks.py generates the COMMITTABLE <repo>/.claude/settings.json (distinct from the personal settings.local.json, which it never touches) with every hook command anchored on $CLAUDE_PROJECT_DIR — the repo root Claude Code exports to every hook — so a fresh clone gets a working sensorium on any machine with zero edits. Dry-run by default (prints the plan), --apply writes with a timestamped backup. Because Claude Code MERGES settings.json + settings.local.json, --apply also reports the now-redundant personal hook commands the user may prune by hand (matched by tool filename) to avoid a double run; it never edits the private file."),
            enforcement=ENFORCED,
            enforced_by=("test_tool_setup_hooks.py::test_build_settings_has_universal_events", "test_tool_setup_hooks.py::test_build_settings_commands_are_portable", "test_tool_setup_hooks.py::test_build_settings_covers_every_universal_tool", "test_tool_setup_hooks.py::test_dry_run_writes_nothing",),
        ),
        Requirement(
            id="R-assumption-implements-state",
            claim=("An Assumption's status field shall admit a fourth value IMPLEMENTS denoting a VOLITIONAL aspiration (a claim we strive to make true), categorically distinct from the three epistemic fact-claim statuses HOLDS/UNCERTAIN/DEAD."),
            owner="framework-author",
            status="SETTLED",
            why=("Steward verdict (2026-07-03, verbatim): 'нужно еще один статус - IMPLEMENTS - значит, что мы пытаемся это сделать, мы стремимся к этому, хотим этого'. Context: A-bootstrap-self-applies sat UNCERTAIN with 58 dependents — the single largest silent question in the graph and the top pulse line. Rather than re-affirm it HOLDS (a factual lie: it is NOT yet true), the steward introduced a fourth род of status: the aspiration. HOLDS/UNCERTAIN/DEAD are epistemic (they answer 'is this true?'); IMPLEMENTS is volitional (it answers 'do we want this to become true, and are we working toward it?'). Three consequences follow directly from its non-epistemic nature and are guaranteed by the existing status-keyed filters that use exact equality: (a) the UNCERTAIN-aging predicate (graph.uncertain_assumptions) does NOT touch it — an aspiration is not an unresolved doubt, so it raises no P4 review-debt; (b) it is NEVER DEAD-fallout (graph.dead_assumptions, reflection.reflect_dead_assumption_on_enforcer) — an aspiration is not a broken premise; (c) legitimate transitions and their signoff (the full table lives in proposal.ProposedAssumptionTransition): IMPLEMENTS to HOLDS ('achieved, became fact'), IMPLEMENTS to DEAD ('abandoned the striving'), UNCERTAIN to IMPLEMENTS ('we understood this is not a fact but a goal') and HOLDS to IMPLEMENTS ('declared fact too early') — ALL four require a human decided_by. Agent entry WITHOUT signoff remains UNCERTAIN-only. The UNCERTAIN to IMPLEMENTS move both CHANGES the род of the claim and REMOVES the live P4 doubt signal, so by the Wave-12 asymmetry (a transition that reduces live signal needs a named human) it carries the signoff lock, unlike the signal-raising move to plain UNCERTAIN. IMPLEMENTS is added to hotam_spec.assumption.ASSUMPTION_STATES and enforced first-class by check_assumption_status_valid (a per-Assumption set-membership invariant registered in ALL_INVARIANTS and RULES_AS_DATA_TABLE as TABLE_DRIVEN); apply_proposal._validate_assumption_transition adds IMPLEMENTS to the decided_by-required set (DEAD, HOLDS, IMPLEMENTS)."),
            relations=(Relation("supports", "R-conflict-is-connector-node"),),
            enforcement=ENFORCED,
            enforced_by=("check_assumption_status_valid", "test_assumption_transition.py::test_implements_is_valid_assumption_state", "test_assumption_transition.py::test_implements_requires_signoff", "test_assumption_transition.py::test_implements_transition_directions_apply", "test_assumption_transition.py::test_implements_status_valid_invariant", "test_assumption_transition.py::test_implements_neither_ages_nor_falls_out",),
        ),
        Requirement(
            id="R-user-request-decomposed-to-tickets",
            claim=("The operator shall, immediately on receiving a user request, decompose it into tickets in the dialogue and ask the addressee -- session tasks or the ticket engine -- before beginning any work."),
            owner="ai-agent",
            status="SETTLED",
            why=("Steward verdict 2026-07-03 (verbatim): 'всё, что просит пользователь - сразу декомпоизровать на тикеты в чате и справшить куда их оптравить - в таски сессии или в движок тикетов'. Decomposition-before-work is a behavioral discipline of the operator: it makes the unit of work explicit and routable BEFORE effort is spent, so a multi-part request is never silently collapsed into one undifferentiated action (R-anchor-everything at the work-item altitude; R-ai-presents-not-decides -- the routing choice is the steward's, presented not assumed). It is machine-unverifiable (INHERENTLY_PROSE): whether the operator actually paused to decompose and ask lives in the dialogue, not the graph."),
            enforcement=STRUCTURAL,
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-ticket-engine-on-disk",
            claim=("Work items shall be tracked as durable on-disk tickets under tickets/<status>/T-<n>.md, each a JSON-frontmatter header plus a Markdown body, created and moved between status folders by the ticket_* tools."),
            owner="framework-author",
            status="SETTLED",
            why=("The steward's queue lived in chat and evaporated between sessions. A ticket is a committed file whose status is its folder, so a status change is a visible file move with a git footprint. JSON-between-sentinels frontmatter keeps the machine header exact under a stdlib-only parser (R-core-imports-stdlib-or-hotam-spec-only) instead of a hand-rolled YAML subset. Enforced by the engine tests exercising create + move (tests/test_tool_ticket_create.py, tests/test_tool_ticket_move.py)."),
            assumptions=("A-text-grounded-in-models",),
            enforcement=ENFORCED,
            enforced_by=("test_tool_ticket_create.py", "test_tool_ticket_move.py",),
        ),
        Requirement(
            id="R-ticket-carries-history",
            claim=("Every ticket shall carry an append-only ## History section in which each mutation (create, status move, comment, text change) records one machine-recognisable entry, with a text change snapshotting the prior text."),
            owner="framework-author",
            status="SETTLED",
            why=("The steward asked that status history AND text-change history be kept automatically. Every ticket_* mutator appends exactly one History line; ticket_edit snapshots the OLD title/body into that line so the edit trail survives the edit. Enforced by the mutation tests asserting each action appends its History entry (tests/test_tool_ticket_create.py, tests/test_tool_ticket_move.py, tests/test_tool_ticket_comment.py, tests/test_tool_ticket_edit.py)."),
            assumptions=("A-text-grounded-in-models",),
            enforcement=ENFORCED,
            enforced_by=("test_tool_ticket_create.py", "test_tool_ticket_move.py", "test_tool_ticket_comment.py", "test_tool_ticket_edit.py",),
        ),
        Requirement(
            id="R-ticket-mutation-via-tools-only",
            claim=("A ticket's frontmatter header and History shall be changed only through the ticket_* tools, never by hand-editing the file."),
            owner="framework-author",
            status="SETTLED",
            why=("The encapsulation the steward demanded: creation, status movement and auto-history must be owned by the tools so the History trail is never skipped. This is INHERENTLY_PROSE discipline, not closeable debt: a filesystem can always be hand-edited and no test can prove authorship of a line. Its enforceable teeth live in R-ticket-carries-history (each mutation demonstrably appends History); this atom names the discipline that keeps hands off the header."),
            assumptions=("A-text-grounded-in-models",),
            enforcement=STRUCTURAL,
            enforceability="INHERENTLY_PROSE",
        ),
        Requirement(
            id="R-open-tickets-visible",
            claim=("The what_now harness shall surface a CLI-only band summarising open (non-done) on-disk tickets broken down by status, read from the filesystem and never fed into diagnose()."),
            owner="framework-author",
            status="SETTLED",
            why=("The steward's backlog moved onto disk; without a pulse band the queue is invisible until someone remembers to ls tickets/ (R-agent-never-lost). Like the pending-proposal / revisit-marker bands it reads the filesystem, so it stays OUT of diagnose() to keep generated docs byte-stable (R-deterministic-generation). Enforced by tests/test_open_tickets_band.py (reports open tickets by status; silent when none; excluded from diagnose)."),
            assumptions=("A-text-grounded-in-models",),
            enforcement=ENFORCED,
            enforced_by=("test_open_tickets_band.py",),
        ),
        Requirement(
            id="R-attention-registry",
            claim=("An agent-agnostic attention-code registry (hotam_spec.attention.ATTENTION_SOURCES) shall exist whose collect() runs every registered source and returns typed AttentionSignal records, with the graph-source diagnosis deterministic across runs."),
            owner="framework-author",
            status="SETTLED",
            why=("The steward's verdict (2026-07-03): the 'pay attention here' signals were scattered -- half in what_now.diagnose(g), half as CLI-only bands OWNED by tools/what_now.py -- with no single, agent-agnostic, importable-as-a-library place an arbitrary agent could call to learn what needs attention. This lifts them into ONE stdlib-only core (src/hotam_spec/attention.py) with an explicit registry of named sources, each an AttentionSource(id, reads, collect). collect(g) returns typed AttentionSignal(source, priority, target, message) the substrate and any agent share. The graph-source diagnosis (diagnose_signals) is deterministic (byte-identical across runs) so it can feed the byte-stable substrate. Enforced by tests/test_attention_core.py (registry non-empty and graph-only; diagnose_signals deterministic; collect(g) == diagnose_signals(g) with no runtime sources)."),
            assumptions=("A-text-grounded-in-models",),
            enforcement=ENFORCED,
            enforced_by=("test_attention_core.py",),
        ),
        Requirement(
            id="R-attention-agent-agnostic-core",
            claim=("The attention core (hotam_spec.attention) shall name no agent-platform token (Claude/Anthropic/hook/model name) so a platform adapter is one consumer, never the owner."),
            owner="framework-author",
            status="SETTLED",
            why=("The steward's verdict (2026-07-03): 'make the solution universal for any agent, and at the seam make an API for Claude'. The core must therefore know nothing of any one agent platform -- the platform adapter (tools/attention_hook.py) is a thin tool wrapper, not part of the core. This is the same backend-neutrality stance as R-core-imports-stdlib-or-hotam-spec-only, extended with a claim-specific platform-token scan. Enforced by tests/test_attention_core.py::test_core_names_no_platform_token (a source scan of src/hotam_spec/attention.py rejects any claude/anthropic/hook/model token) and covered structurally by test_backend_neutral_scope's core-import scan."),
            assumptions=("A-text-grounded-in-models",),
            relations=(Relation("refines", "R-backend-scope"),),
            enforcement="ENFORCED",
            enforced_by=("test_attention_core.py::test_core_names_no_platform_token",),
        ),
        Requirement(
            id="R-attention-superset-of-diagnose",
            claim=("The live attention list attention.collect(g) shall be a superset of the deterministic graph subset diagnose_signals(g), equal to it exactly when no runtime-fs sources are injected."),
            owner="framework-author",
            status="SETTLED",
            why=("The honest split at the centre of the attention core: the SUBSTRATE (gen_spec / LIVE-STATE) must stay byte-stable (R-deterministic-generation), so it consumes ONLY the deterministic graph diagnosis; the LIVE AGENT wants more -- pending proposals, open tickets, stale audit, unread revisit markers -- which read the filesystem and are non-deterministic. Rather than couple those into diagnose() (breaking determinism and polluting graph tests, the failure the Wave 11/15 guards prevent), the filesystem sources are INJECTED by the live consumer via collect(runtime_sources=...). ATTENTION_SOURCES holds graph-only sources (asserted at import time); what_now.runtime_fs_sources() supplies the fs half. Thus collect(agent) is a superset of diagnose(substrate). Enforced by tests/test_attention_core.py (collect==diagnose with no runtime; collect is a strict superset when injected; a graph source rejected as runtime; ATTENTION_SOURCES graph-only) and by the existing diagnose-exclusion guards test_open_tickets_band.py / test_tool_mark_revisit_evaluated.py."),
            assumptions=("A-text-grounded-in-models",),
            relations=(Relation("refines", "R-attention-registry"),),
            enforcement="ENFORCED",
            enforced_by=("test_attention_core.py",),
        ),
        Requirement(
            id="R-attention-claude-adapter",
            claim=("The committed sensorium generator (tools/setup_hooks.py) shall wire the Claude attention adapter (tools/attention_hook.py) onto UserPromptSubmit, and that adapter shall delegate to the attention core rather than re-implement sensing."),
            owner="framework-author",
            status="SETTLED",
            why=("The steward's verdict (2026-07-03): 'at the seam make an API for Claude'. The universal core needs a Claude-specific seam that injects the attention list into the agent's context each turn. tools/attention_hook.py is that adapter -- a thin UserPromptSubmit hook that calls attention.collect() with the runtime-fs sources injected and prints render_flat() to stdout (which Claude Code injects into context). setup_hooks.py (the committed, portable sensorium, R-sensorium-committed) wires it so a fresh clone gets the pulse with zero edits. Enforced by tests/test_attention_claude_adapter.py (build_settings() emits attention_hook.py on UserPromptSubmit; the adapter imports the core, not its own sensing)."),
            assumptions=("A-text-grounded-in-models",),
            relations=(Relation("refines", "R-sensorium-committed"),),
            enforcement=ENFORCED,
            enforced_by=("test_attention_claude_adapter.py",),
        ),
        Requirement(
            id="R-framework-suite-tiered",
            claim=("The test suite shall partition every collected test into exactly one of two responsibility tiers -- `framework` (exercising hotam_spec.* mechanics) or `domain` (asserting concrete self-domain content) -- via the DOMAIN_COUPLED registry in spec/tests/conftest.py, so the framework tier is a separately selectable, self-contained subset."),
            owner="framework-author",
            status="SETTLED",
            why=("Steward doctrine (verdict #8, verbatim): being-working is the framework's OWN responsibility, its tests run 'до всего отдельно' -- first and separately. That is only possible if the framework tests are a named, selectable subset. Before Wave 17 the suite hard-coded self-domain content (C3): under a foreign active-domain pin (HOTAM_SPEC_ACTIVE_DOMAIN=hotam-dev) 18 tests reddened because they assume self-domain atoms/structure. Tagging each test framework-vs-domain (centralized, auditable registry; conftest pytest_collection_modifyitems) makes the framework tier `-m framework` selectable and the domain tier `-m domain` isolated to the self-domain pin."),
            enforcement=STRUCTURAL,
            enforced_by=("test_framework_domain_tiering.py::test_every_test_is_tiered", "test_framework_domain_tiering.py::test_tiers_partition_the_suite",),
        ),
        Requirement(
            id="R-framework-suite-domain-independent",
            claim=("The framework tier (`pytest -m framework`) shall pass green under ANY active domain, or none, independent of which business domain is pinned."),
            owner="framework-author",
            status="SETTLED",
            why=("Steward doctrine (verdict #8, verbatim): 'Бизнес всегда должен думать, что фреймворк работает. Быть рабочим — ответственность фреймворка.' The framework must PROVE its health without depending on a particular business domain's content. This is the DIRECTIONAL guarantee the tiering (R-framework-suite-tiered) exists to serve. PROSE-enforced (not a single-run pytest invariant): proving it requires re-running `-m framework` under a FOREIGN pin (a nested pytest inside one run would recurse); the proof is executed at wave/commit boundary -- Wave 17 demonstrated 982 passed / 18 deselected under HOTAM_SPEC_ACTIVE_DOMAIN=hotam-dev."),
            enforcement=PROSE,
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
            lifecycle="DECIDED(The framework ships content-free and the agent still never gets lost: the initiator supplies the agent its domain content at boot, and the agent crystallizes that content into the domain code-spec. Агент должен получать от инициатора контент о своей области и должен его кристаллизовать в код-спеке. Decided by domain-user, 2026-07-02.)",
            shared_assumption="A-text-grounded-in-models",
            revisit_marker=(
                "REVISIT when a fifth opt-in aspect is proposed OR the first inter-aspect conflict is observed — at that point the core-vs-aspect boundary must be formally re-decided. Boundary re-affirmed at three aspects (2026-07-03): no inter-aspect conflict observed; alarm re-armed. shared_assumption re-pointed from the now-DEAD A-prose-suffices onto its live replacement A-text-grounded-in-models (V2)."
            ),
            decided_by="domain-user",
            derived=(),
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
        Conflict(
            id=conflict_identity("core-vs-aspect", "R-speculative-aspects-frozen freezes the Entity aspect (no inward development until a real business domain demonstrates concrete need), while R-entity-derived-requirement's own enforcement expects EntityType declarations to keep projecting into the domain's CLAUDE.md CONSTITUTION block -- new domain content under the Entity aspect is exactly the kind of inward development the freeze forbids, yet the aspect's enforceability claim presumes it stays live enough to receive new EntityType projections as domains populate it."),
            axis="core-vs-aspect",
            context="R-speculative-aspects-frozen freezes the Entity aspect (no inward development until a real business domain demonstrates concrete need), while R-entity-derived-requirement's own enforcement expects EntityType declarations to keep projecting into the domain's CLAUDE.md CONSTITUTION block -- new domain content under the Entity aspect is exactly the kind of inward development the freeze forbids, yet the aspect's enforceability claim presumes it stays live enough to receive new EntityType projections as domains populate it.",
            members=("R-speculative-aspects-frozen", "R-entity-derived-requirement"),
            steward="framework-reviewer",
            lifecycle="DECIDED(chosen variant V-unfreeze-entity-projection per explicit campaign delegation 2026-07-02 (\"все вопросы решай в сторону совершенства\"))",
            decided_by="domain-user",
            variants=(),
        ),
        Conflict(
            id=conflict_identity("offload-vs-carry", "every newly SETTLED atom adds resident weight to the operator crystal: R-operator-prompt-from-substrate + R-constitution-is-index project ALL SETTLED requirements into root CLAUDE.md (~64k chars at 198 atoms), while R-budget-measure caps that same crystal at 130000 warn / 150000 hard (CRYSTAL_CHARS) -- crystallization pressure and the residency cap collide monotonically as the graph grows, with no eviction mechanic beyond tiered distillation"),
            axis="offload-vs-carry",
            context="every newly SETTLED atom adds resident weight to the operator crystal: R-operator-prompt-from-substrate + R-constitution-is-index project ALL SETTLED requirements into root CLAUDE.md (~64k chars at 198 atoms), while R-budget-measure caps that same crystal at 130000 warn / 150000 hard (CRYSTAL_CHARS) -- crystallization pressure and the residency cap collide monotonically as the graph grows, with no eviction mechanic beyond tiered distillation",
            members=("R-operator-prompt-from-substrate", "R-budget-measure"),
            steward="framework-reviewer",
            lifecycle="DECIDED(DECIDED by tree-of-links law: the root instruction holds only links; when a level is full, links descend to second-level docs and deeper -- growth is unbounded because eviction is structural. Steward verdict 2026-07-03 (V4), verbatim: «у нас есть файлы, на которые только ссылкается коренвая инстуркуция. Если корневая инсррукуция полна ссылками до предела, то нужно писать ссылки в доках второго уровня и тд». The crystallization-pressure vs residency-cap collision (R-operator-prompt-from-substrate vs R-budget-measure) is resolved not by evicting knowledge but by making the resident crystal a tree of links: the root CLAUDE.md carries references, and when it saturates, references cascade into second-level docs (and deeper), so total addressable substrate grows without bound while the RESIDENT char-count stays under the cap. Decided by domain-user, 2026-07-03.)",
            decided_by="domain-user",
            shared_assumption="A-finite-context-operators",
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
            context_budget=ContextBudget(limit=150000, measure="CRYSTAL_CHARS"),
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
