// Origin: one-time bulk extraction by .scratch/selfspec-codegen (task #345, RAC-A) from
// domains/hotam-spec-self/graph.json. That program was a THROWAWAY (run once, never
// committed to git) and is NOT a regeneration path -- it went graph->Go, the OPPOSITE of
// today's sync direction. These literals are HAND-MAINTAINED Go source: edit them directly,
// then mirror Go->graph via `hotam sync-self` (dry-run by default; --confirm-hash to write).
// The "generated" framing describes this file's ORIGIN (task #345's one-time bulk pull from
// the pre-existing JSON graph), not an ongoing regeneration workflow.
//
// Topic: Crystal (CLAUDE.md rendering, template, tree).
//
// STRUCTURAL fields only: Claim, Why, Owner, Status, Relations, Assumptions,
// Enforcement, EnforcedBy, Enforceability, MTag, Summary, CreatedAt, SettledAt,
// BlockedOn, ImplementedBy, VerifiedBy, SourceRefs, DeclOrder. EVENT fields
// (History, GateSignoffs, LastReviewedAt, ReviewAfter, Evidence) are
// deliberately absent — MergeIntoGraph passes them through from the graph's
// existing node untouched.
package selfspec

import "github.com/PHPCraftdream/HotamSpec/internal/ontology"

var _ = Requirements.MustRegister("R-claude-md-budget-phi-cap", ontology.Requirement{
	ID:             "R-claude-md-budget-phi-cap",
	Claim:          "CLAUDE.md (the director's crystal) shall not exceed 1_000_000 / φ ≈ 618033 tokens; on approach, the operator crystallizes/splits rather than letting the crystal swell.",
	Owner:          "framework-author",
	Status:         "REJECTED",
	Why:            "REJECTED -- REPLACES by R-budget-measure + R-working-vs-substrate-budget (both SETTLED/ENFORCED) via CRYSTAL_CHARS. The phi-cap claim hardcoded a numeric ceiling (1_000_000 / phi ~= 618033 tokens) picked before the operator had any real measure of its own crystal size, and it named TOKENS while the operator's actual context arithmetic (check_operator_within_budget) counts CHARS against a host cap -- a unit mismatch that would have silently lied the moment anyone tried to wire a check against it. R-budget-measure already settles the honest unit (bytes/chars, not node counts, not a borrowed-constant token cap) and R-working-vs-substrate-budget already settles what the budget bounds (the WORKING store, leaving crystallized substrate free). check_operator_within_budget's CRYSTAL_CHARS measure (root CLAUDE.md char-length vs a declared host cap, currently 150000) is the real, live, already-ENFORCED mechanism; the phi-cap was a speculative alternate arithmetic that never had an enforcer and would have required reconciling two different budget units had it ever been built. R-claude-md-tree-of-crystals's trigger is retargeted from this dead cap onto the CRYSTAL_CHARS warn threshold in a separate proposal. — (was: The context-bounded-delegation law (R-context-bounded-delegation) applied to the operator's OWN body, not just the graph. BUILD-TRIGGER: CLAUDE.md crosses ~50% of the φ-cap (~309K tokens) — today it is ~7K (~1%), so a budget CHECK now would be machinery guarding a condition that cannot fire. The LIVE-STATE block already reports φ-headroom; the check + the REFLECTION P0 wiring land when headroom actually narrows.)",
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
	DeclOrder:      71,
})

var _ = Requirements.MustRegister("R-claude-md-consolidates-when-single-agent", ontology.Requirement{
	ID:             "R-claude-md-consolidates-when-single-agent",
	Claim:          "While a repository has exactly one domain and zero actively-spawned concurrent sub-agents, `hotam gen-spec` shall generate exactly one CLAUDE.md file at repository root containing all operator-prompt content.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Applies R-crystallize-before-split to file structure itself: don't split the operator-prompt into per-domain/per-agent files until a real second concurrent operator actually needs its own bounded crystal. Maintaining synchronized scaffold files that nobody reads is premature complexity. `hotam gen-spec` (internal/generator/claudemd.go) emits a single CLAUDE.md from the active domain's graph; the AGENT-MAP block (RenderAgentMapBlock) renders '_(no sub-operators yet)_' because no agent has been scaffolded yet (the agents/ infrastructure -- create-agent -- is Planned). ENFORCED 2026-07-13: TestRenderClaudeMDFromTemplate_SingleDomainConsolidatesToOneCrystal in internal/generator/claudemd_coverage_test.go mechanically verifies this claim against the generated crystal on the fixture graph. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-agent-has-own-crystal"}, {Kind: "replaces", Target: "R-domain-owns-claude-md"}, {Kind: "replaces", Target: "R-framework-claude-md-is-domain-free"}, {Kind: "replaces", Target: "R-root-claude-md-contains-domain-crystal"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestRenderClaudeMDFromTemplate_SingleDomainConsolidatesToOneCrystal"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-01",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/generator/claudemd.go", "cmd/hotam/gen_spec.go"},
	DeclOrder:      188,
})

var _ = Requirements.MustRegister("R-claude-md-current", ontology.Requirement{
	ID:             "R-claude-md-current",
	Claim:          "A domain's committed root or local CLAUDE.md, once the crystal convention is adopted for its project, shall keep its generated portion (everything up to and including the durable-notes marker line) byte-identical to a fresh hotam gen-spec render of the current graph, mechanically checked by check_domain_claude_md_current; content the operator authors below that marker line is outside this check's scope.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "External review P1: check_spec_md_current closed the staleness gap for docs/gen/SPEC.md, but CLAUDE.md -- the resident boot crystal every operator session actually reads first -- had no equivalent freshness invariant at all. check_domain_claude_md_current (internal/invariants/claude_md_current.go, real implementation wired in from cmd/hotam/claude_md_current_wiring.go via registry.Update -- the same pattern tool_wiring.go already establishes for methodology.Tools' Run field, needed here because internal/invariants must never import internal/generator, a real, mechanically-enforced import cycle) closes that gap by comparing the committed crystal's generated span against a fresh render on every all-violations pass, ignoring the durable-notes tail the template's own text invites operators to write below the marker line. This requirement is the self-hosting anchor (mirrors R-orientation-faq-answerable's and R-scenario-spec-obligations-mechanically-enforced's identical role for their own check_* gates) that satisfies check_bijection_r_to_enforcer for this new check.",
	Assumptions:    nil,
	Relations:      nil,
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"check_domain_claude_md_current"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-22",
	SettledAt:      "2026-07-22",
	SourceRefs:     nil,
	DeclOrder:      0,
})

var _ = Requirements.MustRegister("R-claude-md-live-state-generated", ontology.Requirement{
	ID:             "R-claude-md-live-state-generated",
	Claim:          "The live numeric state in CLAUDE.md (top action, debt counts, graph size, crystal headroom, context) shall be generated by gen_spec into a sentinel-delimited block, never hand-written.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Commit 36ceabd hand-wrote 'Today: 15 unenforced' into CLAUDE.md — the auto-loaded file — and it drifted to 16 within one phase. The U5 lesson (single source + generated mirror) applied to the operator's own crystal. gen_spec is the 'hook that updates it with the logic run'.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestBuildLiveState_RendersOnFixture"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      68,
})

var _ = Requirements.MustRegister("R-claude-md-template-driven", ontology.Requirement{
	ID:             "R-claude-md-template-driven",
	Claim:          "CLAUDE.md shall be generated by substituting <!-- mind --> and <!-- business --> placeholders in CLAUDE.md.template.txt with rendered content, preserving all other template text verbatim.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "The prior sentinel-surgery approach (editing ~10 separate BEGIN/END marker pairs scattered through an existing file) made it impossible for a human to add durable plain-text notes anywhere in CLAUDE.md without risking accidental clobbering by a future block-insertion bug. The template model inverts this: exactly two named placeholders are substituted; everything else in the template -- including any hand-written notes a human adds -- survives every regeneration verbatim. Simpler mental model, same anti-drift guarantee for the two generated zones. (Wave 1 seed-coherence pass: enforced_by's second entry 'test_regen_byte_identical' is ambiguous -- two test files each define a function with that bare name -- corrected to the specific file, caught by the new check_enforced_by_resolvable invariant.) ENFORCED 2026-07-13: TestRenderClaudeMDFromTemplate_SubstitutesPlaceholdersPreservesRest in internal/generator/claudemd_coverage_test.go mechanically verifies this claim against the generated crystal on the fixture graph.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestRenderClaudeMDFromTemplate_SubstitutesPlaceholdersPreservesRest"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-01",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      192,
})

var _ = Requirements.MustRegister("R-claude-md-tree-of-crystals", ontology.Requirement{
	ID:             "R-claude-md-tree-of-crystals",
	Claim:          "When the root CLAUDE.md's resident CRYSTAL_CHARS size approaches the operator's declared host cap, the operator shall move sections into nested <subdir>/CLAUDE.md crystals and keep only a heading + a when-to-read pointer in the root -- a tree of crystals, one per sub-domain.",
	Owner:          "framework-author",
	Status:         "DRAFT",
	Why:            "R-operator-crystal-is-claude-md made recursive: the delegation hierarchy is a tree of CLAUDE.md crystals (Claude Code natively loads nested CLAUDE.md by directory). Retargeted 2026-07-02 (Wave 2 burn-down): the original BUILD-TRIGGER named R-claude-md-budget-phi-cap, which is now REJECTED (dead hardcoded token ceiling, wrong unit -- see its rejection reason). The real, live, already-ENFORCED measure is check_operator_within_budget's CRYSTAL_CHARS arithmetic (root CLAUDE.md char-length vs the declared host cap, currently 150000 chars per LIVE-STATE) -- R-budget-measure + R-working-vs-substrate-budget. BUILD-TRIGGER (retargeted): the CRYSTAL_CHARS warn threshold in gen_spec's LIVE-STATE narrows meaningfully (today's resident crystal is well under half the cap) -- premature to build the splitting mechanism until headroom actually narrows against the SAME measure the operator already checks every turn.",
	Assumptions:    []string{"A-finite-context-operators", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-tree-of-crystals-cognitive-trigger"}},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      72,
})

var _ = Requirements.MustRegister("R-crystal-carries-mediation-loop", ontology.Requirement{
	ID:             "R-crystal-carries-mediation-loop",
	Claim:          "Under the full gen profile (the self-hosting domains; the lightweight consumer profile omits this block), root CLAUDE.md shall contain a generated MEDIATION-LOOP sentinel block rendering the six-step input-processing loop -- ORIENT, LOCATE, CONFRONT, TRANSLATE, PRESENT, LAND -- each step naming its real tool command.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "The loop is the operator's operating procedure for ANY input: it binds the role (guardian, hypothesis-checker) to tools that already exist -- `hotam what-now` (ORIENT), the Constitution index and generated docs (LOCATE), `hotam confront` + RECENTLY-REJECTED anti-relitigation scan (CONFRONT), Proposed* JSON in internal/proposal (TRANSLATE), resolver decision (PRESENT, R-ai-presents-not-decides), `hotam apply-proposal` then `hotam gen-spec` then `go test` then closure via `hotam what-now` / `hotam all-violations` (LAND, R-verify-closure-per-action). Naming real commands keeps the loop executable rather than aspirational; a pass that writes nothing is a valid conclusion. The block is emitted by RenderMediationLoopBlock (internal/generator/claudemd.go) as static text baked in claudemd_static.go. ENFORCED 2026-07-13: TestRenderMediationLoopBlock_NamesSixStepsAndRealTools in internal/generator/claudemd_sentinel_test.go mechanically verifies this claim against the generated crystal on the fixture graph.",
	Assumptions:    []string{"A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-agent-never-lost"}, {Kind: "refines", Target: "R-ai-presents-not-decides"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestRenderMediationLoopBlock_NamesSixStepsAndRealTools"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/generator/claudemd.go", "internal/generator/claudemd_static.go"},
	DeclOrder:      196,
})

var _ = Requirements.MustRegister("R-crystal-carries-recursion-seed", ontology.Requirement{
	ID:             "R-crystal-carries-recursion-seed",
	Claim:          "Under the full gen profile (the self-hosting domains; the lightweight consumer profile omits this block), root CLAUDE.md shall contain a generated OPERATOR-RECURSION sentinel block describing sub-operator spawning as this same seed narrowed to a sub-scope.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Recursion is a CAPABILITY of the sole operator, not a set of materialized files: while R-claude-md-consolidates-when-single-agent holds (one domain, zero active sub-agents), agent crystals must not exist, so the crystal carries the description of HOW to spawn -- the same Role/Loop seed with a narrower scope filter (R-sub-agent-crystal-triad) -- plus the conclusions-only return contract (R-delegation-conclusions-only). The block is emitted by RenderOperatorRecursionBlock (internal/generator/claudemd.go) with the active domain name interpolated into the spawn-path example. The spawn machinery itself (create-agent, spawn-agent commands) is Planned (internal/methodology/tools_data.go); the OPERATOR-RECURSION block documents the procedure for when the machinery is built. ENFORCED 2026-07-13: TestRenderOperatorRecursionBlock_SameSeedNarrowed in internal/generator/claudemd_sentinel_test.go mechanically verifies this claim against the generated crystal on the fixture graph. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{"A-finite-context-operators"},
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-context-bounded-delegation"}, {Kind: "refines", Target: "R-sub-agent-crystal-triad"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestRenderOperatorRecursionBlock_SameSeedNarrowed"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/generator/claudemd.go", "internal/methodology/tools_data.go"},
	DeclOrder:      197,
})

var _ = Requirements.MustRegister("R-crystal-carries-role-seed", ontology.Requirement{
	ID:             "R-crystal-carries-role-seed",
	Claim:          "Under the full gen profile (the self-hosting domains; the lightweight consumer profile omits this block), root CLAUDE.md shall contain a generated OPERATOR-ROLE sentinel block stating the operator's scope, the core idea (requirements as executable code: an atomic object with a method whose scenario test generates its text), the guardian-of-consistency role across spec, tests, and business intent, and the single generative law of the methodology.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Phase 2 of the crystal redesign (task #8): the operator's identity — requirements as executable code (each requirement an atomic object with a method; the scenario test that runs it also generates its text), guarded as spec↔tests↔business consistency under one generative law (a requirement proves itself and emits its own sentence; everything important-yet-invisible becomes a typed anchored node under a resolver; tension between requirements is a supporting mechanism, held open as a Conflict node and never quietly extinguished) — must be the FIRST resident content of the crystal. Generated (not hand-written template prose) to keep R-root-claude-md-is-sentinel-only true in claim, not only in test; parameterized by the active domain and SETTLED-atom count so the same seed narrows for future sub-operators (R-sub-agent-crystal-triad). ENFORCED 2026-07-13: TestRenderOperatorRoleBlock_CarriesScopeGuardianLaw in internal/generator/claudemd_sentinel_test.go mechanically verifies this claim against the generated crystal on the fixture graph.",
	Assumptions:    []string{"A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "refines", Target: "R-crystal-is-claude-md"}},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestRenderOperatorRoleBlock_CarriesScopeGuardianLaw"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-02",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      195,
})

var _ = Requirements.MustRegister("R-crystal-carries-short-form", ontology.Requirement{
	ID:             "R-crystal-carries-short-form",
	Claim:          "The crystal generator shall render every object using a meaningful short form (an explicit summary, else its first whole sentence) instead of mechanically truncating text mid-word.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Resolver verdict 2026-07-05: 'All that gets truncated must not be truncated but have a short version.' Mechanical truncation ([:96]+'...') creates mid-word stubs that look like knowledge but are illusions — the exact pattern this requirement prohibits. Implemented (task #103): the generator (internal/generator/claudemd.go) now renders short forms via shortForm(text, summary) — summary-priority (when a top signal's Target resolves to a Requirement carrying a non-empty Summary, that Summary is used), else firstWholeSentence(text), which splits on '.', '!', or '?' followed by whitespace or end-of-string and never cuts mid-word. The one violating call site — domainPulse's DOMAIN-MAP top-action pulse line, whose old runeTruncate form reduced '...See docs/gen/UNENFORCED.md.' to the mid-word stub 'See doc...' — is replaced; runeTruncate itself is deleted as now fully unused (its sole production caller was domainPulse; runeTruncateEllipsis is a separate, retained helper with a different single-'…' purpose). The Requirement.Summary field (internal/ontology/requirement.go) is consulted via summaryForTarget. Scope is deliberately tight: only the crystal-rendering site this requirement governs was converted; cmd/hotam/confront.go's runeTruncatePreview (a CLI diagnostic preview, not crystal rendering) is intentionally untouched. settled_at RESTORED 2026-07-13 from git HEAD 9af0176 (pre-corruption value) after a mutate.go bug this session silently reset it on content-only updates; see task #84.",
	Assumptions:    []string{},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestShortForm_SummaryPriority", "TestFirstWholeSentence_SentenceBoundary", "TestShortForm_NoMidWordEllipsis", "TestSummaryForTarget", "TestDomainPulse_SummaryPreferredWhenTargetResolves", "TestDomainPulse_LongMessageUsesFirstSentence"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-07-05",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/generator/claudemd.go", "internal/ontology/requirement.go"},
	DeclOrder:      262,
})

var _ = Requirements.MustRegister("R-crystal-is-claude-md", ontology.Requirement{
	ID:             "R-crystal-is-claude-md",
	Claim:          "Each operator's crystallized substrate shall be its own CLAUDE.md file.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "Atom of R-operator-crystal-is-claude-md (identity concern). CLAUDE.md is auto-loaded by the harness, making it the natural crystal location. RECLASSIFIED 2026-07-13: enforceability ENFORCEABLE -> INHERENTLY_PROSE (agent consultation, fm) -- this is a naming/identity design decision (the crystal file IS named CLAUDE.md, by construction of the generator), not a runtime property a test can independently verify without simply re-asserting the same decision (an assertion that a constant filename equals \"CLAUDE.md\" would be a vacuous test, not a proof of the concept).",
	Assumptions:    []string{"A-compaction-loses-working", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-operator-crystal-is-claude-md"}},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      141,
})

var _ = Requirements.MustRegister("R-crystal-reload-by-reference", ontology.Requirement{
	ID:             "R-crystal-reload-by-reference",
	Claim:          "An operator shall reload its crystal (CLAUDE.md) by reference rather than re-carrying it in working context.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "Atom of R-operator-crystal-is-claude-md (reload-by-reference concern). Re-carrying wastes context budget; reloading by reference is the offload instrument. RECLASSIFIED 2026-07-13: enforceability ENFORCEABLE -> INHERENTLY_PROSE (agent consultation, fm) -- this describes runtime behavior of a live agent's own working-context management, which is unobservable to this repository's test suite by construction: no test process can inspect what a session's context window is carrying versus re-fetching.",
	Assumptions:    []string{"A-compaction-loses-working", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-operator-crystal-is-claude-md"}},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "INHERENTLY_PROSE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      142,
})

var _ = Requirements.MustRegister("R-crystal-tree-hierarchy", ontology.Requirement{
	ID:             "R-crystal-tree-hierarchy",
	Claim:          "The delegation hierarchy shall be a tree of CLAUDE.md crystals, one per operator, each bounded by its context budget.",
	Owner:          "ai-agent",
	Status:         "SETTLED",
	Why:            "Atom of R-operator-crystal-is-claude-md (tree-hierarchy concern). The tree structure mirrors the delegation hierarchy and is natively supported by Claude Code nested CLAUDE.md loading.",
	Assumptions:    []string{"A-compaction-loses-working", "A-bootstrap-self-applies"},
	Relations:      []ontology.Relation{{Kind: "replaces", Target: "R-operator-crystal-is-claude-md"}},
	Enforcement:    "PROSE",
	EnforcedBy:     []string{},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{},
	DeclOrder:      143,
	BlockedOn:      "blocked on the create_agent/spawn_agent/invoke_agent tools (Planned) — no second operator exists to form a tree",
})

var _ = Requirements.MustRegister("R-root-claude-md-contains-domain-crystal", ontology.Requirement{
	ID:             "R-root-claude-md-contains-domain-crystal",
	Claim:          "Root CLAUDE.md shall embed the full content of the active domain's CLAUDE.md inside a DOMAIN-CRYSTAL sentinel block generated by gen_spec.py.",
	Owner:          "framework-author",
	Status:         "REJECTED",
	Why:            "REJECTED — REPLACES R-claude-md-consolidates-when-single-agent. This claim described embedding a SEPARATE domains/hotam-spec-self/CLAUDE.md inside root CLAUDE.md via a DOMAIN-CRYSTAL sentinel block. That separate file no longer exists (deleted in task #101's consolidation) — there is nothing left to embed, since the domain content is generated directly into the single root CLAUDE.md rather than composed from a nested crystal. The two-file DOMAIN-CRYSTAL composition pattern returns only when a second domain is created and root CLAUDE.md can no longer hold all domains inline. — (was: Closes the sensor-substrate gap: Claude Code auto-loads root CLAUDE.md on session start; embedding the domain's CLAUDE.md (the canonical entry point of the domain, and the base from which all sub-agents derive scoped versions) means the operator boots from substrate (R-operator-prompt-from-substrate) rather than from raw weights + session memory. The substrate writes the operator's prompt physically, not aspirationally.)",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"test_domain_crystal_contains_domains_claude_md_content"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "",
	SourceRefs:     []string{},
	DeclOrder:      180,
})

var _ = Requirements.MustRegister("R-root-claude-md-is-sentinel-only", ontology.Requirement{
	ID:             "R-root-claude-md-is-sentinel-only",
	Claim:          "The root CLAUDE.md (and its AGENTS.md/GEMINI.md siblings) shall be rendered by gen-spec from a template whose body is a minimal framework-identity header plus sentinel-bounded generated blocks, with no hand-written prose between sentinels.",
	Owner:          "framework-author",
	Status:         "SETTLED",
	Why:            "Substrate-generates-operator at root level. Hand-written prose drifts; the framework's mind is assembled by gen-spec from spec/docs/thinking/* (shared DRY source) and the domain graph, and the root is a thin shell pointing into both. The root CLAUDE.md is produced by generator.RenderClaudeMDFromTemplate (internal/generator/claudemd.go) when gen-spec is invoked with --claude-md pointing at the root path; the same bytes are also written to AGENTS.md and GEMINI.md (cmd/hotam/gen_spec.go). The body uses the sentinel-pair pattern from internal/generator/claudemd_sentinel.go (WrapBlock/ExtractBlock), so every generated region sits between e.g. <!-- LIVE-STATE:BEGIN --> ... <!-- LIVE-STATE:END --> markers and is rewritten wholesale on each regen, making hand-edits between sentinels structurally impossible to preserve. RESOLVED -- REPLACES the old hand-written prose in root CLAUDE.md (P19a). FOUND ISSUE (not fixed): no test or check_* verifies the on-disk root CLAUDE.md contains no hand-written prose between sentinels (the original structural enforcer, tests/test_root_claude_md_is_sentinel_only.py, was not carried over). The property holds today because the file is regenerated wholesale from the template, but the guard against a future hand-edit is absent; the requirement is STRUCTURAL with no enforced_by. ENFORCED 2026-07-13: TestRenderClaudeMDFromTemplate_NoProseBetweenSentinels in internal/generator/claudemd_coverage_test.go mechanically verifies this claim against the generated crystal on the fixture graph.",
	Assumptions:    []string{"A-python-stack"},
	Relations:      []ontology.Relation{},
	Enforcement:    "ENFORCED",
	EnforcedBy:     []string{"TestRenderClaudeMDFromTemplate_NoProseBetweenSentinels"},
	MTag:           "",
	Enforceability: "ENFORCEABLE",
	Summary:        "",
	CreatedAt:      "2026-06-30",
	SettledAt:      "2026-07-12",
	SourceRefs:     []string{"internal/generator/claudemd.go", "internal/generator/claudemd_sentinel.go", "cmd/hotam/gen_spec.go"},
	DeclOrder:      173,
})
