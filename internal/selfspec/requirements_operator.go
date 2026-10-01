// Origin: one-time bulk extraction by .scratch/selfspec-codegen (task #345, RAC-A) from
// domains/hotam-spec-self/graph.json. That program was a THROWAWAY (run once, never
// committed to git) and is NOT a regeneration path -- it went graph->Go, the OPPOSITE of
// today's sync direction. These literals are HAND-MAINTAINED Go source: edit them directly,
// then mirror Go->graph via `hotam sync-self` (dry-run by default; --confirm-hash to write).
// The "generated" framing describes this file's ORIGIN (task #345's one-time bulk pull from
// the pre-existing JSON graph), not an ongoing regeneration workflow.
//
// Topic: Operator.
//
// STRUCTURAL fields only: Claim, Why, Owner, Status, Relations, Assumptions,
// Enforcement, EnforcedBy, Enforceability, MTag, Summary, CreatedAt, SettledAt,
// BlockedOn, ImplementedBy, VerifiedBy, SourceRefs, DeclOrder. EVENT fields
// (History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are
// deliberately absent — MergeIntoGraph passes them through from the graph's
// existing node untouched.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-operator-acting-facet", ontology.Requirement{
	ID:             "R-operator-acting-facet",
	Claim:          "An Operator shall be a Stakeholder's ACTING facet: it owns a bounded DomainScope, carries a ContextBudget and capabilities, and may have a parent Operator.",
	Owner:          "framework-author",
	Status:         "REJECTED",
	Why:            "REJECTED — REPLACES by R-operator-is-frozen-dataclass + R-operator-references-stakeholder + R-operator-has-context-budget + R-operator-may-have-parent per atomicity discipline (R-requirement-claim-is-atomic). The original claim mixed four concerns: type identity, stakeholder reference, budget, hierarchy.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      29,
})

var _ = Requirements.MustRegister("R-operator-backend-protocol", ontology.Requirement{
	ID:             "R-operator-backend-protocol",
	Claim:          "The framework's tools shall talk to the acting agent through a single OperatorBackend protocol (get_context_state / request_resolver_approval / delegate), so the methodology does not hard-depend on which coding-agent or model drives it.",
	Owner:          "framework-author",
	Status:         "REJECTED",
	Why:            "REJECTED -- REPLACES by R-backend-scope (SETTLED). M37 resolver verdict 2026-07-02 (verbatim): «не важно -- каждый умный агент под себя допишет» (English: 'it doesn't matter -- every smart agent will adapt it for itself'). The resolver explicitly declined to build a designed OperatorBackend protocol (get_context_state / request_resolver_approval / delegate): the core surface (spec/tools/*.py CLIs, JSON Proposed* shapes, pytest verification) already has no dependency on any one agent's runtime and stays backend-neutral BY CONSTRUCTION, not by an abstraction layer built ahead of a second real backend. Building the protocol now would be exactly the speculative-engineering-against-hypothetical-backends R-speculative-aspects-frozen already warns against. R-backend-scope (SETTLED, PROSE) already carries this verdict verbatim in its why and is the surviving atom; R-operator-backend-protocol is rejected rather than left DRAFT so the graph does not carry a dead aspiration as open debt. — (was: Today tools/ implicitly assume Claude Code (Agent tool, Bash, chat-resolver). BUILD-TRIGGER: a SECOND concrete backend becomes real (CI runner, a different coding agent, or a programmatic resolver). Until then, abstracting for hypothetical backends is the big-bang-up-front antipattern (weight ∝ cost of an unnoticed conflict). See OPEN R-backend-scope (which backends are real?).)",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      70,
})

var _ = Requirements.MustRegister("R-operator-crystal-embeds-thinking", ontology.Requirement{
	ID:             "R-operator-crystal-embeds-thinking",
	Claim:          "The operator's CLAUDE.md shall embed the full content of its scope-relevant thinking documentation inline, not as markdown links, so the operator holds the methodology itself rather than a table of contents.",
	Owner:          "framework-author",
	Status:         "REJECTED",
	Why:            "REJECTED — REPLACES by R-operator-crystal-embeds-thinking-distilled: full-text embedding contradicted R-crystal-reload-by-reference and breached the 150k host limit (CLAUDE.md reached ~200k chars); the crystal now carries a RULE+WHY distillate + Tier-3 pointer instead. — (was: A link the operator must separately fetch is a re-derivation tax on every turn; inlining the methodology content means it is present in the loaded substrate from the first token (R-operator-prompt-from-substrate). Task #98 (A1) built the EMBEDDED-THINKING block; ENFORCED by test_embedded_thinking_sentinels_present (sentinels exist) and test_embedded_thinking_contains_full_topic_content (content is the full topic text, not a link).)",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"test_embedded_thinking_tools.py::test_embedded_thinking_sentinels_present", "test_embedded_thinking_tools.py::test_embedded_thinking_contains_full_topic_content"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-01",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      189,
})

var _ = Requirements.MustRegister("R-operator-crystal-embeds-thinking-distilled", ontology.Requirement{
	ID:             "R-operator-crystal-embeds-thinking-distilled",
	Claim:          "Under the full gen profile (the self-hosting domains; the lightweight consumer profile omits the thinking block), the operator's CLAUDE.md shall embed one RULE sentence per thinking topic (sourced from Section.Canon, guarded via shortForm) plus a path link to domains/<domain>/docs/gen/thinking/<slug>.md, not a multi-line RULE+WHY distillate.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "REPLACES the Tier-1 multi-paragraph RULE+WHY distillation approach: even compressed RULE+WHY pairs consumed ~19k chars across 22 topics. The implemented design embeds one real RULE sentence per §-section, sourced from Section.Canon (internal/methodology/sections_data.go) -- the same registry BuildThinkingDocs (internal/generator/thinkingdocs.go) reads from, so Canon is the single source of truth for both the inlined RULE and the full thinking-doc on disk (domains/<domain>/docs/gen/thinking/<slug>.md). The thinking/*.md docs are themselves generated FROM this registry's Canon/Narrative/Why, so sourcing the inlined rule from Canon adds no new source of truth. Each Canon is guarded through shortForm(canon, \"\") (firstWholeSentence) as future-proofing against multi-sentence Canon drift (R-crystal-carries-short-form); today's 29 Canon values are single standalone sentences (2694 chars total, min 44 / max 178 / avg 93), so no truncation occurs and the short-form guard is a pure regression hedge. The rendered '- **§X** — <Canon>' block costs ~3.3k chars; the resident crystal moved 16458 -> 19335 chars (+2877), far below the 150000-char host cap (R-context-budget-rule, ENFORCED via ComputeCrystalCharCountFixpoint) and nowhere near the ~200k the OLD full-text embedding reached. Claim path/name fixes landed in this same proposal: corrected the Python-era 'short_form' name to the Go identifier 'shortForm', and the stale 'spec/docs/thinking/<slug>.md' path to 'domains/<domain>/docs/gen/thinking/<slug>.md' (matching the generator's path convention). ENFORCED 2026-07-13 via three tests in internal/generator/claudemd_coverage_test.go: TestRenderEmbeddedThinkingBlock_DistillsCanonPerSection (every registered section's short-formed Canon appears in the rendered block, bullet shape verified), TestRenderEmbeddedThinkingBlock_NoMidWordTruncation (no mid-word '...' stub for any current Canon value -- regression guard mirroring TestShortForm_NoMidWordEllipsis), TestRenderEmbeddedThinkingBlock_CrystalWithinBudget (crystal char-count fixpoint converges and stays under the 130000 warn threshold).",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-operator-crystal-embeds-thinking"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestRenderEmbeddedThinkingBlock_DistillsCanonPerSection", "TestRenderEmbeddedThinkingBlock_NoMidWordTruncation", "TestRenderEmbeddedThinkingBlock_CrystalWithinBudget"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/generator/claudemd.go", "internal/methodology/sections_data.go", "internal/generator/thinkingdocs.go"},
	DeclOrder:      193,
})

var _ = Requirements.MustRegister("R-operator-crystal-embeds-tools", ontology.Requirement{
	ID:             "R-operator-crystal-embeds-tools",
	Claim:          "The operator's CLAUDE.md shall embed the full content of its scope-relevant tool documentation inline, not as markdown links.",
	Owner:          "framework-author",
	Status:         "REJECTED",
	Why:            "REJECTED — REPLACES by R-operator-crystal-embeds-tools-distilled: full-text embedding contradicted R-crystal-reload-by-reference and breached the 150k host limit (CLAUDE.md reached ~200k chars); the crystal now carries a RULE+WHY distillate + Tier-3 pointer instead. — (was: Same rationale as R-operator-crystal-embeds-thinking applied to tool docs: an operator deciding whether to invoke apply_proposal.py should not have to fetch a separate file to learn its contract. ENFORCED by test_embedded_tools_sentinels_present (sentinels exist) and test_embedded_tools_contains_full_tool_content (content is the full tool doc text, not a link); test_embedded_blocks_regen_byte_identical guards against drift between the embedded copy and the regenerated source.)",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"test_embedded_thinking_tools.py::test_embedded_tools_sentinels_present", "test_embedded_thinking_tools.py::test_embedded_tools_contains_full_tool_content", "test_embedded_thinking_tools.py::test_embedded_blocks_regen_byte_identical"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-01",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      190,
})

var _ = Requirements.MustRegister("R-operator-crystal-embeds-tools-distilled", ontology.Requirement{
	ID:             "R-operator-crystal-embeds-tools-distilled",
	Claim:          "Under the full gen profile (the self-hosting domains; the lightweight consumer profile omits the tools block and writes no framework/tools/), the operator's CLAUDE.md shall embed an EMBEDDED-TOOLS block that reports the Implemented and Planned tool counts from the methodology.Tool registry and directs the operator to `hotam -h`, `hotam status --json`, `hotam req`, `hotam brief`, and docs/gen/tools/INDEX.md for on-demand detail — a compact pointer reference, not a per-tool distillation.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "SUPERSEDES the one-line-per-Implemented-tool distillation: now that agentic on-demand tool discovery (`hotam -h`, `hotam req`, `hotam brief` — tasks #124/#126 this session) makes the embedded per-tool list redundant with the same registry-derived information available on demand, the crystal's context-debt trends toward pointers-to-free-on-demand-detail rather than embedded distillation (review 5 / wave-5 @fx agent-UX consultation, plan item T-m / task #128). The counts are retained so an agent glancing at the crystal still knows roughly how much tool surface exists. The block is still rendered by RenderEmbeddedToolsBlock (internal/generator/claudemd.go) as a live projection of methodology.Tools.All() (internal/methodology/tools_data.go) — only the rendering shape changed from per-tool lines to a pointer-only one-liner, keeping the registry-derived, never-hand-maintained discipline intact (R-tools-registry-generated still holds: the block is rendered from the registry). The full per-tool detail (Canon, Purpose, Status sections) lives in docs/gen/tools/<command>.md, generated by BuildToolDocs (internal/generator/tooldocs.go). ENFORCED: TestRenderEmbeddedToolsBlock_PointerOneLiner in internal/generator/claudemd_coverage_test.go mechanically verifies this claim.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-operator-crystal-embeds-tools"}, {Kind: "replaces", Target: "R-repo-map-generated"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestRenderEmbeddedToolsBlock_PointerOneLiner"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/generator/claudemd.go", "internal/methodology/tool.go", "internal/generator/tooldocs.go"},
	DeclOrder:      194,
})

var _ = Requirements.MustRegister("R-operator-crystal-is-claude-md", ontology.Requirement{
	ID:             "R-operator-crystal-is-claude-md",
	Claim:          "Each operator's crystallized substrate shall be its own CLAUDE.md — an anchored map of its bounded sub-domain that it reloads BY REFERENCE rather than re-carrying; the director-operator's CLAUDE.md holds the overall graph and references each sub-operator's CLAUDE.md.",
	Owner:          "ai-agent",
	Status:         "REJECTED",
	Why:            "REJECTED — REPLACES split into R-crystal-is-claude-md + R-crystal-reload-by-reference + R-crystal-tree-hierarchy (wave 2, decided by framework-author 2026-06-30) — (was: REJECTED — REPLACES split into R-crystal-is-claude-md + R-crystal-reload-by-reference + R-crystal-tree-hierarchy (wave 2, decided by framework-author 2026-06-30) — (was: SETTLED (P7): the crystal exists as substrate. The Director's Map in CLAUDE.md indexes the whole graph and provides the anchored map for the director-operator. docs/gen/CONSTITUTION.md is the generated reconstitution from the laws — a fresh agent reading it reconstitutes as operator without relying on a session checkpoint. The discipline is structural via: the Director's Map is the crystal (CLAUDE.md); CONSTITUTION.md is generated from the SETTLED laws; the boot-sequence in §6 names the exact steps to reconstitute. Per the anchoring super-rule it cites code handles (R-/C-/§/file) so understanding is regained fast; the delegation hierarchy is therefore a TREE of CLAUDE.md crystals (exactly how Claude Code nests CLAUDE.md per directory), one per operator, each bounded by its context budget. Implementation: docs/gen/CONSTITUTION.md + CLAUDE.md.))",
	Assumptions:    []string{"A-compaction-loses-working", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "STRUCTURAL",
	EnforcedBy:     []string{"test_constitution_gen.py"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      43,
})

var _ = Requirements.MustRegister("R-operator-has-context-budget", ontology.Requirement{
	ID:             "R-operator-has-context-budget",
	Claim:          "An Operator shall carry a ContextBudget with a positive limit and a declared measure.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-operator-acting-facet (budget concern). check_operator_within_budget validates the budget.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-operator-acting-facet"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_operator_within_budget"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      32,
})

var _ = Requirements.MustRegister("R-operator-is-frozen-dataclass", ontology.Requirement{
	ID:             "R-operator-is-frozen-dataclass",
	Claim:          "An Operator shall be a dedicated struct type in internal/ontology/operator.go carrying the typed anchor 'OP-', with field mutations performed only through the proposal system.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-operator-acting-facet (type identity concern). Operator is a plain struct (internal/ontology/operator.go: fields ID, Stakeholder, Lifecycle, ContextBudget, Parent *string, Scope, Why, DeclOrder). Go has no frozen-dataclass language primitive (historically Python's @dataclass(frozen=True) provided one), so immutability is a DISCIPLINE, not a structurally-enforced guarantee: there are no setter methods, and field-level mutations are performed only through the proposal system (ProposedOperatorBudget.mutate in internal/proposal/mutate.go re-writes g.Operators[idx].ContextBudget). The typed-anchor discipline ('OP-' prefix) is enforced by check_typed_anchors_operator (internal/invariants/typed_anchors.go). OP-director is the first instance. FOUND ISSUE (not fixed): the claim's 'field mutations only through proposals' discipline is NOT structurally verified -- check_typed_anchors_operator only verifies the OP- prefix, not the immutability discipline. The enforcement/enforced_by are out of scope for this content batch; the enforcement is ENFORCED with check_typed_anchors_operator, which honestly tests only the prefix half. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-operator-acting-facet"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_typed_anchors_operator"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/ontology/operator.go", "internal/invariants/typed_anchors.go", "internal/proposal/mutate.go"},
	DeclOrder:      30,
})

var _ = Requirements.MustRegister("R-operator-may-have-parent", ontology.Requirement{
	ID:             "R-operator-may-have-parent",
	Claim:          "An Operator.parent shall reference another Operator.id or be None (root).",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-operator-acting-facet (hierarchy concern). Structural via the Operator.parent field type.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-operator-acting-facet"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_no_dangling_operator_refs"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-06-30",
	SourceRefs:     []string{},
	DeclOrder:      33,
})

var _ = Requirements.MustRegister("R-operator-not-self-approve", ontology.Requirement{
	ID:             "R-operator-not-self-approve",
	Claim:          "An Operator shall not resolver a Conflict in which its underlying Stakeholder owns one of the members.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "M36 — the reflexive twin of check_resolver_not_a_member_owner. An Operator is the acting facet of a Stakeholder; the resolver-distinct boundary applies through that facet so an Operator cannot self-ratify decisions on its own party's side. Structurally enforced.",
	Assumptions:    []string{"A-stakeholders-care"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_operator_resolver_not_self"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      35,
})

var _ = Requirements.MustRegister("R-operator-prompt-from-substrate", ontology.Requirement{
	ID:             "R-operator-prompt-from-substrate",
	Claim:          "The operator-prompt CLAUDE.md shall include a CONSTITUTION block (compact per-category summary of SETTLED requirements) generated deterministically from the active domain's graph.json, with the full id+claim roster in docs/gen/CONSTITUTION.md.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "Realizes the sensor-substrate inversion: consciousness (the operator-prompt) is GENERATED from the graph, not hand-maintained. Two layers carry this: (1) BuildConstitutionBlock (internal/generator/claudemd_constitutionindex.go) renders the CLAUDE.md CONSTITUTION sentinel block as a compact per-category SUMMARY (category label + count only, P2-1 compaction), generated from the active domain's SETTLED requirements; (2) BuildConstitution (internal/generator/constitution.go) renders the full docs/gen/CONSTITUTION.md catalog (all SETTLED requirements grouped by category with id + enforcement flag + claim). Both are pure functions of the in-memory graph, called by gen-spec (cmd/hotam/gen_spec.go). Enforced by TestBuildConstitution_ByteIdenticalToFixture (internal/generator/), which asserts the full catalog is byte-identical to its fixture -- proving deterministic generation from the graph. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-compaction-loses-working", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestBuildConstitution_ByteIdenticalToFixture"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/generator/claudemd_constitutionindex.go", "internal/generator/constitution.go", "cmd/hotam/gen_spec.go"},
	DeclOrder:      82,
})

var _ = Requirements.MustRegister("R-operator-prompt-loaded-at-session-start", ontology.Requirement{
	ID:             "R-operator-prompt-loaded-at-session-start",
	Claim:          "A SessionStart hook shall run `hotam gen-spec` before the operator's first turn of any session, ensuring root CLAUDE.md is current substrate-derived state.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Not yet implemented: the Go CLI (cmd/hotam) is a pure command-line tool with no host-hook / SessionStart integration, so there is no mechanism that automatically runs gen-spec at session boot. Closes the boot edge of the sensor-substrate inversion (R-operator-prompt-from-substrate): without SessionStart regen, the host may auto-load a stale CLAUDE.md whose DOMAIN-CRYSTAL / LIVE-STATE blocks reflect an older graph state. The equivalent would invoke `hotam gen-spec` from a host-side hook, but no such hook is wired. (Historical Python mechanism: a SessionStart hook in .claude/settings.local.json ran gen_spec.py.) PROSE (not ENFORCED): no enforcer exists. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"cmd/hotam/gen_spec.go"},
	DeclOrder:      182,
	BlockedOn:      "blocked on the setup_hooks tool (Planned) + a host SessionStart hook",
})

var _ = Requirements.MustRegister("R-operator-references-stakeholder", ontology.Requirement{
	ID:             "R-operator-references-stakeholder",
	Claim:          "An Operator.stakeholder shall reference an existing Stakeholder.id.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Atom of R-operator-acting-facet (stakeholder reference concern). check_no_dangling_ids validates the reference.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-operator-acting-facet"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_no_dangling_operator_refs"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      31,
})

var _ = Requirements.MustRegister("R-operator-type-vs-facet", ontology.Requirement{
	ID:             "R-operator-type-vs-facet",
	Claim:          "Operator shall be its own first-class struct type in internal/ontology/operator.go (not a Stakeholder facet), with typed anchor 'OP-', a ContextBudget, and an optional parent reference.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "M20. DECIDED 2026-06-30: Operator is a new type. Rationale: a Stakeholder facet cannot carry a ContextBudget, enforce check_operator_within_budget, or be referenced by Goal.owner -- all of which are live ENFORCED requirements. The clean separation (Stakeholder = party in internal/ontology/stakeholder.go, Operator = acting facet with budget + capabilities in internal/ontology/operator.go) prevents conflation at the single-altitude-vs-multi-altitude axis. Evidence: Operator struct (internal/ontology/operator.go) with fields including ContextBudget (ontology.ContextBudget) and Parent (*string); R-operator-is-frozen-dataclass SETTLED ENFORCED; check_typed_anchors_operator (internal/invariants/typed_anchors.go) live, verifying the OP- prefix. FOUND ISSUE (not fixed): the historical claim named 'frozen-dataclass' -- Go has no frozen-dataclass primitive, so immutability is a discipline (no setters; mutations only via ProposedOperatorBudget), not structurally enforced. check_typed_anchors_operator verifies only the OP- prefix, not the type-separation or immutability; the enforcement/enforced_by are out of scope for this content batch.",
	Assumptions:    []string{"A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_typed_anchors_operator"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/ontology/operator.go", "internal/ontology/stakeholder.go", "internal/invariants/typed_anchors.go"},
	DeclOrder:      62,
})
