// Origin: one-time bulk extraction by .scratch/selfspec-codegen (task #345, RAC-A) from
// domains/hotam-spec-self/graph.json. That program was a THROWAWAY (run once, never
// committed to git) and is NOT a regeneration path -- it went graph->Go, the OPPOSITE of
// today's sync direction. These literals are HAND-MAINTAINED Go source: edit them directly,
// then mirror Go->graph via `hotam sync-self` (dry-run by default; --confirm-hash to write).
// The "generated" framing describes this file's ORIGIN (task #345's one-time bulk pull from
// the pre-existing JSON graph), not an ongoing regeneration workflow.
//
// Topic: Empty Content and Well-Formedness.
//
// STRUCTURAL fields only: Claim, Why, Owner, Status, Relations, Assumptions,
// Enforcement, EnforcedBy, Enforceability, MTag, Summary, CreatedAt, SettledAt,
// BlockedOn, ImplementedBy, VerifiedBy, SourceRefs, DeclOrder. EVENT fields
// (History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are
// deliberately absent — MergeIntoGraph passes them through from the graph's
// existing node untouched.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-empty-content-calm-banner", ontology.Requirement{
	ID:             "R-empty-content-calm-banner",
	Claim:          "When the active domain has no content yet (empty graph), `hotam what-now` shall render a calm 'no content yet'-style signal, not an error.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-empty-content-is-legitimate (calm-banner concern). D2 split decided by domain-user 2026-06-30. WHY: an error on missing content scares off new adopters who have not populated their domain yet. The calm behavior holds PARTIALLY: what-now over an EMPTY-but-present graph (0 nodes, graph.json loads successfully) returns 'none â€” graph clean' via formatSignals (cmd/hotam/what_now.go) â€” it does not error on an empty graph, which is the spirit of this requirement. However the 'missing graph.json' case (a freshly cloned framework with no graph.json at all) is NOT yet handled with a calm banner: loader.LoadGraph (internal/loader/loader.go) returns a read error and what-now propagates it rather than rendering a welcoming notice. The historical tools/what_now.py calm-banner path (test_main_empty_content_prints_calm_banner) is the reference design for implementing the missing-file calm path. The empty-graph case is covered structurally; the missing-file case remains an honest gap. ENFORCED 2026-07-13: TestEmptyContentCalmBanner in internal/diagnose/empty_content_test.go mechanically verifies this claim: DiagnoseSignals over an empty graph yields zero signals without panicking, and TopAction renders the calm 'none — graph clean' signal rather than an error. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-empty-content-is-legitimate"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestEmptyContentCalmBanner"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"cmd/hotam/what_now.go", "internal/loader/loader.go"},
	DeclOrder:      108,
})

var _ = Requirements.MustRegister("R-empty-content-gen-notice", ontology.Requirement{
	ID:             "R-empty-content-gen-notice",
	Claim:          "When the active domain has no content yet (missing or genuinely empty graph), `hotam gen-spec` shall NOT fail, and shall write ZERO files under docs/gen/ (full profile; the lightweight consumer profile additionally writes no project-shared framework/ files) — no 'no content yet' notice rendered into a placeholder-filled docs/gen/*.md set.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-empty-content-is-legitimate (gen-notice concern). D2 split decided by domain-user 2026-06-30. WHY: failing on missing content would block the regen step of the closed loop for new adopters. ENFORCED 2026-07-13, REVISED 2026-07-26 (task #364): genSpec (cmd/hotam/gen_spec.go) still loads the domain graph via loadGraphOrEmpty, which detects os.IsNotExist on the missing graph.json specifically and substitutes an empty graph (&ontology.Graph{DomainDir: domainDir}) instead of propagating the raw read error — but the empty-graph BEHAVIOR itself changed: previously every Build* helper's g.IsEmpty() branch rendered the calm EmptyNotice (internal/generator/common.go) INTO docs/gen/*.md, so a freshly-scaffolded, unmodeled domain accumulated a directory full of calm-but-pointless placeholder files nobody asked for. Task #364 (business-domain feedback: a truly empty domain should produce NO generated artifacts at all) extended every remaining docs/gen/*.md projection (REQUIREMENTS/OPEN/UNENFORCED/FRAMEWORK-INVARIANTS/HISTORY/CONSTITUTION/TRACEABILITY/COVERAGE/REPO-MAP/AGENT-CONTEXT/live-state.md, plus the docs/gen/graph.json archival copy) with the SAME conditional-write pattern DECISIONS.md/ENTITIES.md/TENSIONS.md/PIPELINE.md/MODELS.md already used (each gated behind its own generator.XxxMDHasContent(g) predicate, all reducing to !g.IsEmpty() here) — and, in the SAME task, removed `hotam init`/`initDomain`'s own auto-seeded Stakeholder+Requirement, so a freshly scaffolded domain is now GENUINELY empty (0 nodes) rather than carrying one worked-example requirement. The missing-file case (this requirement's own anchor) and the empty-but-present case remain indistinguishable to an adopter with nothing modeled yet, so they still produce IDENTICAL output — just now that output is ZERO docs/gen/ files instead of a full calm-notice set. The discrimination between 'missing/empty, calm' and 'a real error' is unchanged and still mechanical: errors.Is(err, os.ErrNotExist) — a genuine error (a malformed graph.json decode failure, or a permissions error) still surfaces as a real error, enforced by the TestGenSpec_MissingGraph_MalformedStillErrors non-vacuity control. The PROJECT-shared framework/ output (GLOSSARY.md, tools/*.md/INDEX.md) is UNAFFECTED by any of this — it is written unconditionally, independent of domain content. The identical missing-file gap also exists in the what-now code path (R-empty-content-calm-banner's own why documents this); that is NOT this requirement's anchor and remains an honest open gap there — loadGraphOrEmpty is deliberately scoped to gen-spec, not shared via loadDomainGraph, to avoid expanding scope across every loader caller.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-empty-content-is-legitimate"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestGenSpec_MissingGraphRendersCalmNotice"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"cmd/hotam/gen_spec.go", "internal/loader/loader.go", "internal/generator/common.go", "cmd/hotam/init_cmd.go"},
	DeclOrder:      109,
})

var _ = Requirements.MustRegister("R-empty-content-is-legitimate", ontology.Requirement{
	ID:             "R-empty-content-is-legitimate",
	Claim:          "A freshly-cloned framework with no spec/content/graph.py shall be structurally well-formed; what_now renders a calm 'no content yet' banner and gen_spec emits the same notice.",
	Owner:          "domain-user",
	Status:         "REJECTED",
	Why:            "REJECTED — REPLACES split into R-empty-content-wellformed + R-empty-content-calm-banner + R-empty-content-gen-notice (D2, decided by domain-user 2026-06-30) — (was: An empty content slot is honest, not a defect. Adopters can see the framework working before they have anything to model.)",
	Assumptions:    []string{"A-content-free-honest"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"test_what_now.py::test_main_empty_content_prints_calm_banner", "test_docs_gen.py::test_empty_graph_renders_no_content_notice"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      7,
})

var _ = Requirements.MustRegister("R-empty-content-wellformed", ontology.Requirement{
	ID:             "R-empty-content-wellformed",
	Claim:          "A freshly-cloned framework with an empty graph shall pass all structural invariants â€” an empty graph is well-formed.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-empty-content-is-legitimate (well-formedness concern). D2 split decided by domain-user 2026-06-30. WHY: if an empty graph is malformed, the framework punishes adopters for having nothing to model yet. This holds structurally: loader.validateGraph (internal/loader/loader.go) iterates over each slice and checks per-element fields â€” an empty graph (zero axes, stakeholders, assumptions, requirements, conflicts, operators, goals, entity types, entities) has nothing to violate, so it loads successfully; likewise invariants.AllViolations (internal/invariants/all_violations.go) over an empty graph returns zero violations because every check iterates an empty slice. An empty graph is well-formed by construction. There is no dedicated empty-graph-wellformed test, but the property is exercised by the loader/generator test fixtures that build minimal graphs and assert zero violations (the historical test_empty_graph_is_wellformed asserted the same property). ENFORCED 2026-07-13: TestEmptyContentWellFormed in internal/invariants/empty_content_test.go mechanically verifies this claim: AllViolations over a completely empty ontology.Graph returns zero violations (an empty graph is well-formed by construction). settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-empty-content-is-legitimate"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestEmptyContentWellFormed"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/loader/loader.go", "internal/invariants/all_violations.go"},
	DeclOrder:      107,
})

var _ = Requirements.MustRegister("R-open-states-question", ontology.Requirement{
	ID:             "R-open-states-question",
	Claim:          "Every requirement whose status begins with 'OPEN' shall carry a non-empty question of the form OPEN(<question>).",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "An OPEN with no question is a hole no one can act on. The harness surfaces OPEN items by their question; emptiness defeats the point of marking the requirement open at all.",
	Assumptions:    []string{"A-text-grounded-in-models"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_open_has_question"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-06-30",
	SourceRefs:     []string{},
	DeclOrder:      8,
})

var _ = Requirements.MustRegister("R-uncrystallizable-automated", ontology.Requirement{
	ID:             "R-uncrystallizable-automated",
	Claim:          "Detection of 'uncrystallizable knowledge = missing type' is human judgment: the operator records the candidate in the graph as an OPEN requirement (or a DRAFT), and the resolver decides whether to add the ontology type.",
	Owner:          "framework-reviewer",
	Status:         "SETTLED",
	Why:            "M30. DECIDED 2026-06-30: human judgment, not automated. Rationale: R-uncrystallizable-is-missing-type (SETTLED P6) already establishes that the operator records the signal as a node; the §Conscience property-sweep surfaces the meta-signal structurally when a class of contradictions cannot be expressed by existing critical-core invariants. Automating the type-creation decision would violate R-ai-presents-not-decides. The whole audit-backlog-residue checkpoint pattern + the DRAFT queue IS the recording mechanism. The resolver is the decider; the graph is the recorder. The §Conscience concept exists as methodology prose (internal/methodology/sections_data.go: §Conscience section, noting 'go test ./internal/invariants/...' as the conceptual sweep over the critical-core invariants named in internal/generator/constitution.go criticalCoreNames). NOT YET IMPLEMENTED: a property-test sweep over the critical core to flag a class of contradictions no existing invariant can express (the §Conscience concept-map entry reflects this: 0 checks, tests counted from static data, no mapped definition). (Historical note: the original test_conscience.py was a Hypothesis-based property test over CRITICAL_CORE_INVARIANTS; it was not carried forward.) The human-judgment core of this claim is unchanged and inherently unautomatable: the operator records the signal as a graph node, the resolver decides. STRUCTURAL / INHERENTLY_PROSE: the claim's own nature is a human-judgment decision boundary that no machine check_* can honestly enforce (R-enforceability-kind-declared).",
	Assumptions:    []string{"A-most-knowledge-crystallizable"},
	Relations:      []ontology.Relation{},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/methodology/sections_data.go", "internal/generator/constitution.go"},
	DeclOrder:      67,
})

var _ = Requirements.MustRegister("R-uncrystallizable-is-missing-type", ontology.Requirement{
	ID:             "R-uncrystallizable-is-missing-type",
	Claim:          "Knowledge an operator cannot crystallize as any existing node shall be RECORDED as a candidate missing ontology type for resolver review (not auto-acted).",
	Owner:          "framework-reviewer",
	Status:         "SETTLED",
	Why:            "SETTLED (P6): the meta-signal surface exists — when the §Conscience Hypothesis property-sweep finds a class of contradictions that no existing critical-core invariant can express, the property-test failure IS the recording mechanism (a clear, machine-visible meta-signal that a new type is needed). The resolver still decides whether to add the type; the recording itself is now structural, not manual.",
	Assumptions:    []string{"A-most-knowledge-crystallizable"},
	Relations:      []ontology.Relation{},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      57,
})
